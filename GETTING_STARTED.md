# Getting Started with Notification Service

## What We Built

A production-grade, self-hostable notification service with:

✅ **Plugin-based provider architecture** - Add email, SMS, push notifications easily
✅ **RabbitMQ message queue** - For reliable async processing  
✅ **PostgreSQL database** - Audit trail and status tracking
✅ **At-least-once & at-most-once delivery** - Choose your reliability level
✅ **Automatic retries** - Exponential backoff with dead letter queues
✅ **Idempotency** - Prevent duplicate sends
✅ **Single binary deployment** - No runtime dependencies except DB + queue

---

## 1. Understanding Go Plugin Architecture

### The Problem: How is Go Different from Python?

**Python approach:**
```python
# pip install wheel-file
# Python automatically discovers and imports modules
from notification_providers import gmail_provider
```

**Go approach:**
Go doesn't have dynamic module loading like Python. Instead, we use **interface-based plugin registration**:

### The Solution: Interface + Registry Pattern

```
┌─────────────────────────────────────────┐
│   Provider Interface (pkg/providers)    │
│  ┌─────────────────────────────────┐  │
│  │ type Provider interface {        │  │
│  │   Name()                         │  │
│  │   Send()                         │  │
│  │   Initialize()                   │  │
│  │   Health()                       │  │
│  │ }                                │  │
│  └─────────────────────────────────┘  │
└─────────────────────────────────────────┘
                  ▲
        ┌─────────┼─────────┐
        │         │         │
   ┌────────┐ ┌────────┐ ┌────────┐
   │ SMTP   │ │  SMS   │ │ Push   │
   │Provider│ │Provider│ │Provider│
   └────────┘ └────────┘ └────────┘
        │         │         │
        └─────────┼─────────┘
                  │
        ┌─────────▼─────────┐
        │ Registry Map      │
        │ "smtp" → factory  │
        │ "sms" → factory   │
        │ "push" → factory  │
        └───────────────────┘
```

### Key Difference: init() Functions

Each provider registers itself automatically:

**SMTP Provider (pkg/providers/smtp.go):**
```go
package providers

type SMTPProvider struct {
    Host, Port, Username, Password string
}

func (p *SMTPProvider) Name() string { return "smtp" }
func (p *SMTPProvider) Send(...) {...}
func (p *SMTPProvider) Initialize(...) {...}
func (p *SMTPProvider) Health() {...}

// Magic happens here!
func init() {
    Register("smtp", NewSMTPProvider)
}
```

When you `import "pkg/providers"`, the `init()` function runs and registers "smtp" in the registry. No manual work needed!

### Adding a New Provider (3 Steps)

**Step 1: Create file `pkg/providers/myservice.go`**
```go
package providers

type MyServiceProvider struct {
    APIKey string
}

func (p *MyServiceProvider) Name() string { return "myservice" }
func (p *MyServiceProvider) Send(ctx context.Context, req *NotificationRequest) (*NotificationResponse, error) {
    // Your implementation
}
// ... implement other methods ...

func init() {
    Register("myservice", NewMyServiceProvider)
}
```

**Step 2: Rebuild**
```bash
go build ./cmd/server
go build ./cmd/worker
```

**Step 3: Configure in DB**
```sql
INSERT INTO provider_configs (provider, config, is_active) VALUES 
('myservice', '{"api_key":"your-key"}', true);
```

That's it! The worker automatically loads and uses your provider.

---

## 2. Project Structure

```
notifications/
├── cmd/
│   ├── server/
│   │   └── main.go           ← REST API server
│   └── worker/
│       └── main.go           ← Message processor
│
├── pkg/
│   ├── providers/
│   │   ├── interface.go      ← Provider interface definition
│   │   ├── smtp.go           ← Email provider (plugin example)
│   │   ├── sms.go            ← SMS provider (plugin example)
│   │   └── errors.go
│   │
│   ├── models/
│   │   └── notification.go   ← Database models
│   │
│   ├── queue/
│   │   └── queue.go          ← RabbitMQ client
│   │
│   └── database/
│       └── db.go             ← PostgreSQL client
│
├── internal/
│   └── handlers/
│       └── notification.go   ← HTTP route handlers
│
├── docker-compose.yml        ← Local dev environment
├── go.mod & go.sum          ← Dependency management
└── README.md
```

---

## 3. Quick Start

### Option A: Docker Compose (Easiest)

```bash
cd /home/kevin/sources/swecha/notifications
docker-compose up -d

# Check services are running
docker-compose ps

# View logs
docker-compose logs -f server
docker-compose logs -f worker
```

Services running:
- **API**: http://localhost:8080
- **RabbitMQ UI**: http://localhost:15672 (guest/guest)
- **PostgreSQL**: localhost:5432

### Option B: Local Development

```bash
# Prerequisites
brew install postgresql rabbitmq golang

# Start services
brew services start postgresql
brew services start rabbitmq

# Create database
createdb notifications

# Set environment
export DATABASE_URL="postgres://localhost/notifications"
export RABBITMQ_URL="amqp://localhost:5672/"

# Run server
go run ./cmd/server/main.go

# In another terminal, run worker
go run ./cmd/worker/main.go
```

---

## 4. Testing the Service

### Test 1: Check Health

```bash
curl http://localhost:8080/health
# Response: {"status":"ok"}
```

### Test 2: Configure SMTP Provider

First, insert a provider config (using Gmail as example):

```bash
# Create a database connection
psql -U postgres notifications

# Insert SMTP config
INSERT INTO provider_configs (id, provider, config, is_active) 
VALUES (
    'cfg-smtp-1',
    'smtp',
    '{"host":"smtp.gmail.com","port":587,"username":"your-email@gmail.com","password":"YOUR_APP_PASSWORD","from":"noreply@example.com"}',
    true
);
```

### Test 3: Send a Notification

```bash
curl -X POST http://localhost:8080/notifications/send \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "smtp",
    "recipient": "your-email@example.com",
    "subject": "Test Notification",
    "content": "This is a test from the notification service",
    "delivery_mode": "at_least_once",
    "idempotency_key": "test-123"
  }'

# Response:
# {"id":"550e8400-e29b-41d4-a716-446655440000","status":"pending"}
```

### Test 4: Check Status

```bash
curl http://localhost:8080/notifications/550e8400-e29b-41d4-a716-446655440000

# Response:
# {
#   "id":"550e8400-...",
#   "status":"sent",
#   "provider":"smtp",
#   "recipient":"your-email@example.com",
#   ...
# }
```

---

## 5. Adding Your First Custom Provider

Let's add a simple Slack provider:

### Create `pkg/providers/slack.go`

```go
package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type SlackProvider struct {
	WebhookURL string
}

func NewSlackProvider(config map[string]interface{}) (Provider, error) {
	p := &SlackProvider{}
	if err := p.parseConfig(config); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *SlackProvider) parseConfig(config map[string]interface{}) error {
	url, ok := config["webhook_url"].(string)
	if !ok || url == "" {
		return fmt.Errorf("%w: missing 'webhook_url'", ErrInvalidConfig)
	}
	p.WebhookURL = url
	return nil
}

func (p *SlackProvider) Name() string {
	return "slack"
}

func (p *SlackProvider) Initialize(config map[string]interface{}) error {
	return p.parseConfig(config)
}

func (p *SlackProvider) Send(ctx context.Context, req *NotificationRequest) (*NotificationResponse, error) {
	payload := map[string]string{
		"text": req.Content,
	}

	body, _ := json.Marshal(payload)
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", p.WebhookURL, bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return &NotificationResponse{
			Success:     true,
			ProviderRef: "slack-" + req.ID,
		}, nil
	}

	return &NotificationResponse{
		Success:      false,
		ErrorMessage: fmt.Sprintf("Slack API error: %d", resp.StatusCode),
	}, nil
}

func (p *SlackProvider) Health() error {
	return nil
}

// Register the provider
func init() {
	Register("slack", NewSlackProvider)
}
```

### Rebuild and Deploy

```bash
go build -o bin/notification-server ./cmd/server
go build -o bin/notification-worker ./cmd/worker
```

### Configure in Database

```sql
INSERT INTO provider_configs (id, provider, config, is_active)
VALUES (
    'cfg-slack-1',
    'slack',
    '{"webhook_url":"https://hooks.slack.com/services/YOUR/WEBHOOK/URL"}',
    true
);
```

### Test It

```bash
curl -X POST http://localhost:8080/notifications/send \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "slack",
    "recipient": "channel-id",
    "content": "Hello from Slack provider!",
    "delivery_mode": "at_least_once"
  }'
```

---

## 6. Understanding the Message Flow

```
1. APP SENDS REQUEST
   POST /notifications/send
   {provider, recipient, content, delivery_mode, idempotency_key}
         │
         ▼
2. API SERVER
   ✓ Validates request
   ✓ Creates notification record in DB (status: pending)
   ✓ Publishes message to RabbitMQ
   Returns: 202 Accepted with notification ID
         │
         ▼
3. RABBITMQ QUEUE
   Message sits in "notifications" queue
   Persistence: survives restarts
         │
         ▼
4. WORKER PROCESS
   ✓ Consumes message from queue
   ✓ Looks up provider from registry
   ✓ Loads provider config from DB
   ✓ Calls provider.Send()
         │
         ├─ SUCCESS ──▶ Updates DB (status: sent) ──▶ ACKs message
         │
         └─ FAILURE ──▶ Decides:
                        ├─ at_most_once: move to DLQ, ACK
                        └─ at_least_once: exponential backoff retry
                                │
                                ├─ Retry succeeded: sent
                                └─ Max retries exceeded: DLQ

5. DEAD LETTER QUEUE
   Failed notifications tracked in:
   - notifications.status = "failed"
   - failed_notifications table
   - Operator can investigate and replay
```

---

## 7. Delivery Modes Explained

### At-Least-Once (Guaranteed)

- ✅ Message will be delivered (possibly multiple times)
- ✅ Automatic retries with exponential backoff
- ✅ Best for: Important notifications (password resets, payments)

```
Try #1: immediate
Try #2: 5s later
Try #3: 10s later
Try #4: 20s later
Try #5: 40s later
DLQ:    80s later
```

### At-Most-Once (Fire & Forget)

- ⚡ Fast, no retries
- ❌ May fail silently
- ✅ Best for: Non-critical notifications (analytics, tracking)

---

## 8. Architecture Benefits

### Why Go + Interfaces Instead of Python + Wheels?

| Feature | Go Interfaces | Python Wheels |
|---------|---------------|---------------|
| **Startup** | < 1 second | 2-5 seconds |
| **Memory** | 30-50MB | 100-200MB |
| **Type Safety** | ✅ Compile-time | ❌ Runtime errors |
| **Single Binary** | ✅ Yes | ❌ Need Python runtime |
| **Concurrency** | ✅ Goroutines (millions) | ⚠️ Limited (GIL) |
| **Deployment** | ✅ Copy binary | ⚠️ pip install needed |

### Why RabbitMQ Instead of Redis?

| Feature | RabbitMQ | Redis |
|---------|----------|-------|
| **Persistence** | ✅ Disk-based | ❌ In-memory (risky) |
| **Dead Letter Queue** | ✅ Built-in | ❌ Manual implementation |
| **Reliability** | ✅ At-least-once by default | ⚠️ Need extra config |
| **Clustering** | ✅ Native | ⚠️ Sentinel needed |

---

## 9. Production Deployment

### Docker

```bash
docker build -f Dockerfile.server -t notification-service:latest .
docker push your-registry/notification-service:latest
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: notification-service
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: api
        image: notification-service:latest
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: db-creds
              key: url
        - name: RABBITMQ_URL
          valueFrom:
            secretKeyRef:
              name: rabbitmq-creds
              key: url
        ports:
        - containerPort: 8080
      - name: worker
        image: notification-service:latest
        command: ["/notification-worker"]
        env:
        # ... same env vars
```

### Environment Variables

```bash
# Required
DATABASE_URL=postgres://user:pass@host:5432/db
RABBITMQ_URL=amqp://user:pass@host:5672/

# Optional
PORT=8080
ENVIRONMENT=production
```

---

## 10. Next Steps

1. **Try the demo**: Run `docker-compose up -d` and test with curl
2. **Add your provider**: Create a new provider file and rebuild
3. **Set up authentication**: Add API key validation in handlers
4. **Add health checks**: Implement provider health endpoints
5. **Set up monitoring**: Add Prometheus metrics
6. **Configure alerting**: Alert on DLQ messages

---

## References

- [Provider Interface](./pkg/providers/interface.go)
- [Plugin Architecture Guide](./PLUGIN_ARCHITECTURE.md)
- [API Documentation](./README.md)
