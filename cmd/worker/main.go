package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/swecha/notifications/pkg/database"
	"github.com/swecha/notifications/pkg/models"
	"github.com/swecha/notifications/pkg/providers"
	"github.com/swecha/notifications/pkg/queue"
)

type NotificationWorker struct {
	db        *database.DB
	queue     *queue.Queue
	providers map[string]providers.Provider
	maxRetries int
}

// NewWorker creates a new worker instance
func NewWorker(db *database.DB, q *queue.Queue) *NotificationWorker {
	return &NotificationWorker{
		db:         db,
		queue:      q,
		providers:  make(map[string]providers.Provider),
		maxRetries: 5,
	}
}

// InitializeProviders loads all active providers from config
func (w *NotificationWorker) InitializeProviders(ctx context.Context) error {
	configs, err := w.db.GetActiveProviderConfigs()
	if err != nil {
		return fmt.Errorf("failed to fetch provider configs: %w", err)
	}

	for _, cfg := range configs {
		var configMap map[string]interface{}
		if err := json.Unmarshal([]byte(cfg.Config), &configMap); err != nil {
			log.Printf("failed to parse config for %s: %v", cfg.Provider, err)
			continue
		}

		provider, err := providers.Create(cfg.Provider, configMap)
		if err != nil {
			log.Printf("failed to create provider %s: %v", cfg.Provider, err)
			continue
		}

		w.providers[cfg.Provider] = provider
		log.Printf("initialized provider: %s", cfg.Provider)
	}

	return nil
}

// Start begins processing messages
func (w *NotificationWorker) Start(ctx context.Context) error {
	msgs, err := w.queue.Consume(ctx, "notifications", 10)
	if err != nil {
		return err
	}

	log.Println("Worker started, waiting for messages...")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg := <-msgs:
			w.processMessage(ctx, msg)
		}
	}
}

// processMessage handles a single message
func (w *NotificationWorker) processMessage(ctx context.Context, delivery amqp.Delivery) {
	var msg queue.Message
	if err := json.Unmarshal(delivery.Body, &msg); err != nil {
		log.Printf("failed to unmarshal message: %v", err)
		delivery.Ack(false)
		return
	}

	log.Printf("Processing message %s to provider %s", msg.ID, msg.Provider)

	provider, exists := w.providers[msg.Provider]
	if !exists {
		log.Printf("provider %s not found", msg.Provider)
		w.handleFailedMessage(ctx, &msg, "Provider not found")
		delivery.Ack(false)
		return
	}

	// Create send request
	req := &providers.NotificationRequest{
		ID:        msg.ID,
		Recipient: msg.Recipient,
		Subject:   msg.Subject,
		Content:   msg.Content,
		Metadata:  msg.Metadata,
	}

	// Send notification with timeout
	sendCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	resp, err := provider.Send(sendCtx, req)
	cancel()

	if err != nil {
		log.Printf("send error: %v", err)
		w.handleRetry(ctx, &msg, err.Error(), delivery)
		return
	}

	if !resp.Success {
		log.Printf("send failed: %s", resp.ErrorMessage)
		w.handleRetry(ctx, &msg, resp.ErrorMessage, delivery)
		return
	}

	// Mark as sent
	if err := w.db.UpdateNotificationStatus(msg.ID, models.StatusSent, resp.ProviderRef); err != nil {
		log.Printf("failed to update notification status: %v", err)
	}

	log.Printf("message %s sent successfully", msg.ID)
	delivery.Ack(false)
}

// handleRetry decides whether to retry or move to DLQ
func (w *NotificationWorker) handleRetry(ctx context.Context, msg *queue.Message, errMsg string, delivery amqp.Delivery) {
	msg.Retries++

	// at_most_once: don't retry
	if msg.DeliveryMode == models.AtMostOnce {
		w.handleFailedMessage(ctx, msg, errMsg)
		delivery.Ack(false)
		return
	}

	// at_least_once: retry with backoff
	if msg.Retries < w.maxRetries {
		backoffSeconds := (1 << uint(msg.Retries)) * 5 // exponential backoff: 5, 10, 20, 40, 80
		log.Printf("retrying message %s in %d seconds (attempt %d/%d)", msg.ID, backoffSeconds, msg.Retries, w.maxRetries)
		w.queue.PublishToRetry(ctx, msg)
	} else {
		w.handleFailedMessage(ctx, msg, errMsg)
	}

	delivery.Ack(false)
}

// handleFailedMessage moves message to DLQ
func (w *NotificationWorker) handleFailedMessage(ctx context.Context, msg *queue.Message, errMsg string) {
	log.Printf("moving message %s to DLQ: %s", msg.ID, errMsg)

	// Update DB
	fn := &models.FailedNotification{
		ID:             fmt.Sprintf("%s-dlq-%d", msg.ID, time.Now().Unix()),
		NotificationID: msg.ID,
		Reason:         errMsg,
		Attempts:       msg.Retries,
		LastError:      errMsg,
	}
	w.db.CreateFailedNotification(fn)

	// Update notification status
	w.db.UpdateNotificationError(msg.ID, errMsg, msg.Retries)

	// Publish to DLQ
	w.queue.PublishToDLQ(ctx, msg)
}

func main() {
	// Load config
	rabbitmqDSN := os.Getenv("RABBITMQ_URL")
	if rabbitmqDSN == "" {
		rabbitmqDSN = "amqp://guest:guest@localhost:5672/"
	}

	dbDSN := os.Getenv("DATABASE_URL")
	if dbDSN == "" {
		log.Fatal("DATABASE_URL env var not set")
	}

	// Connect to DB
	db, err := database.NewDB(dbDSN)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	// Connect to queue
	q, err := queue.NewQueue(rabbitmqDSN)
	if err != nil {
		log.Fatalf("failed to connect to queue: %v", err)
	}
	defer q.Close()

	// Setup queues
	ctx := context.Background()
	if err := q.DeclareQueues(ctx); err != nil {
		log.Fatalf("failed to declare queues: %v", err)
	}

	// Create and initialize worker
	worker := NewWorker(db, q)
	if err := worker.InitializeProviders(ctx); err != nil {
		log.Fatalf("failed to initialize providers: %v", err)
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("received signal: %v", sig)
		os.Exit(0)
	}()

	// Start processing
	if err := worker.Start(ctx); err != nil {
		log.Fatalf("worker error: %v", err)
	}
}
