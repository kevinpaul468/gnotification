package queue

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Message represents a message in the queue
type Message struct {
	ID       string                 `json:"id"`
	AppID    string                 `json:"app_id"`
	Provider string                 `json:"provider"`
	Recipient string                `json:"recipient"`
	Subject  string                 `json:"subject,omitempty"`
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Retries  int                    `json:"retries"`
	DeliveryMode string              `json:"delivery_mode"`
}

// Queue manages RabbitMQ connections and operations
type Queue struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	dsn     string
}

// NewQueue creates a new queue instance
func NewQueue(dsn string) (*Queue, error) {
	q := &Queue{dsn: dsn}
	if err := q.Connect(); err != nil {
		return nil, err
	}
	return q, nil
}

// Connect establishes connection to RabbitMQ
func (q *Queue) Connect() error {
	var err error
	q.conn, err = amqp.Dial(q.dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	q.channel, err = q.conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open channel: %w", err)
	}

	return nil
}

// DeclareQueues sets up all required queues and exchanges
func (q *Queue) DeclareQueues(ctx context.Context) error {
	// Declare notification queue with persistence
	_, err := q.channel.QueueDeclare(
		"notifications",     // name
		true,                // durable - survives restart
		false,               // delete when unused
		false,               // exclusive
		false,               // no-wait
		amqp.Table{
			"x-message-ttl": 86400000, // 24 hours
		},
	)
	if err != nil {
		return fmt.Errorf("failed to declare notifications queue: %w", err)
	}

	// Declare dead letter queue for failed notifications
	_, err = q.channel.QueueDeclare(
		"notifications-dlq", // name
		true,                // durable
		false,               // delete when unused
		false,               // exclusive
		false,               // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to declare DLQ: %w", err)
	}

	// Declare retry queue
	_, err = q.channel.QueueDeclare(
		"notifications-retry",
		true,
		false,
		false,
		false,
		amqp.Table{
			"x-message-ttl": 300000, // 5 minutes
			"x-dead-letter-exchange": "",
			"x-dead-letter-routing-key": "notifications",
		},
	)
	if err != nil {
		return fmt.Errorf("failed to declare retry queue: %w", err)
	}

	return nil
}

// Publish sends a message to the queue
func (q *Queue) Publish(ctx context.Context, msg *Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	err = q.channel.PublishWithContext(
		ctx,
		"",               // exchange
		"notifications", // routing key
		false,            // mandatory
		false,            // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent, // persist to disk
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

// Consume returns a channel to consume messages
func (q *Queue) Consume(ctx context.Context, queueName string, prefetch int) (<-chan amqp.Delivery, error) {
	err := q.channel.Qos(
		prefetch, // prefetch count
		0,        // prefetch size
		false,    // global
	)
	if err != nil {
		return nil, fmt.Errorf("failed to set QoS: %w", err)
	}

	msgs, err := q.channel.ConsumeWithContext(
		ctx,
		queueName, // queue name
		"",        // consumer tag
		false,     // auto-ack (we'll ack manually)
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)
	if err != nil {
		return nil, fmt.Errorf("failed to consume: %w", err)
	}

	return msgs, nil
}

// PublishToRetry publishes a message to the retry queue
func (q *Queue) PublishToRetry(ctx context.Context, msg *Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	err = q.channel.PublishWithContext(
		ctx,
		"",                      // exchange
		"notifications-retry", // routing key
		false,                   // mandatory
		false,                   // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
	return err
}

// PublishToDLQ publishes a message to the dead letter queue
func (q *Queue) PublishToDLQ(ctx context.Context, msg *Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	err = q.channel.PublishWithContext(
		ctx,
		"",                   // exchange
		"notifications-dlq", // routing key
		false,                // mandatory
		false,                // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
	return err
}

// Close closes the queue connection
func (q *Queue) Close() error {
	if q.channel != nil {
		q.channel.Close()
	}
	if q.conn != nil {
		return q.conn.Close()
	}
	return nil
}
