# Quick Reference

## Building

```bash
# Server (REST API)
go build -o bin/notification-server ./cmd/server

# Worker (Message processor)
go build -o bin/notification-worker ./cmd/worker

# Or use docker-compose
docker-compose up -d
```

## Running

### With Docker Compose
```bash
docker-compose up -d                    # Start all services
docker-compose logs -f                  # Watch logs
docker-compose down                     # Stop all services
```

### Locally
```bash
export DATABASE_URL="postgres://localhost/notifications"
export RABBITMQ_URL="amqp://localhost:5672/"

# Terminal 1: API Server
go run ./cmd/server/main.go

# Terminal 2: Worker
go run ./cmd/worker/main.go
```

## API Endpoints

### Send Notification
```
POST /notifications/send
Content-Type: application/json

{
  "provider": "smtp",
  "recipient": "user@example.com",
  "subject": "Hello",
  "content": "Message body",
  "delivery_mode": "at_least_once",
  "idempotency_key": "unique-id"
}

Response: 202 Accepted
{
  "id": "uuid",
  "status": "pending"
}
```

### Check Status
```
GET /notifications/{id}

Response: 200 OK
{
  "id": "uuid",
  "status": "sent|pending|failed|delivered",
  "provider": "smtp",
  "recipient": "user@example.com",
  "created_at": "2024-05-22T14:23:04Z",
  "delivered_at": "2024-05-22T14:23:10Z"
}
```

### Health Check
```
GET /health

Response: 200 OK
{
  "status": "ok"
}
```

## Database

### Connect
```bash
docker exec notifications_db psql -U postgres -d notifications
```

### Key Tables
```sql
-- All notifications (with status)
SELECT * FROM notifications ORDER BY created_at DESC LIMIT 10;

-- Failed notifications
SELECT * FROM failed_notifications ORDER BY created_at DESC;

-- Provider configs
SELECT provider, config, is_active FROM provider_configs;
```

## RabbitMQ UI

- URL: http://localhost:15672
- Username: guest
- Password: guest

Check queue depths and message counts.

## Environment Variables

```bash
DATABASE_URL=postgres://localhost/notifications
RABBITMQ_URL=amqp://localhost:5672/
PORT=8080
ENVIRONMENT=development
```

## Adding a New Provider

1. Create `pkg/providers/myprovider.go`
2. Implement the `Provider` interface
3. Add `func init() { Register("name", NewMyProvider) }`
4. Build: `go build ./cmd/server && go build ./cmd/worker`
5. Configure in database
6. Done!

## Common Tasks

### Configure SMTP
```sql
INSERT INTO provider_configs (id, provider, config, is_active)
VALUES (
  'cfg-smtp',
  'smtp',
  '{"host":"smtp.gmail.com","port":587,"username":"email@gmail.com","password":"APP_PASSWORD","from":"noreply@example.com"}',
  true
);
```

### Send Test Email
```bash
curl -X POST http://localhost:8080/notifications/send \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "smtp",
    "recipient": "test@example.com",
    "subject": "Test",
    "content": "This is a test",
    "delivery_mode": "at_least_once"
  }'
```

### View Active Providers
```sql
SELECT provider, is_active FROM provider_configs WHERE is_active = true;
```

### Clear Failed Messages
```sql
DELETE FROM failed_notifications WHERE created_at < NOW() - INTERVAL '7 days';
```

## Troubleshooting

### Worker not starting
```bash
# Check logs
docker-compose logs worker

# Verify RabbitMQ
curl -u guest:guest http://localhost:15672/api/connections

# Verify PostgreSQL
psql -U postgres -h localhost -d notifications -c "SELECT 1"
```

### Notifications stuck as pending
```sql
-- Check DB for errors
SELECT * FROM notifications WHERE status = 'pending' ORDER BY created_at DESC;

-- Check failed queue
SELECT COUNT(*) FROM failed_notifications;
```

### SMTP auth fails
- Use app-specific passwords for Gmail
- Check credentials in provider_configs
- Ensure "Allow less secure apps" is enabled (if not using app password)

## Performance Tips

### Scale Workers
Run multiple worker instances for faster processing:
```bash
for i in {1..5}; do
  go run ./cmd/worker/main.go &
done
```

### Monitor Queue Depth
```bash
# RabbitMQ UI: http://localhost:15672
# Or via API:
curl -u guest:guest http://localhost:15672/api/queues
```

### Batch Operations
The service is optimized for high throughput with:
- RabbitMQ prefetch: 10 messages per worker
- Connection pooling in PostgreSQL
- Goroutine-based concurrency in Go

## Files Overview

| File | Purpose |
|------|---------|
| `cmd/server/main.go` | REST API server |
| `cmd/worker/main.go` | Message processor |
| `pkg/providers/` | Provider implementations |
| `pkg/database/` | PostgreSQL client |
| `pkg/queue/` | RabbitMQ client |
| `pkg/models/` | Database models |
| `internal/handlers/` | HTTP route handlers |

## Documentation

- [README.md](./README.md) - Full documentation
- [GETTING_STARTED.md](./GETTING_STARTED.md) - Detailed tutorial
- [PLUGIN_ARCHITECTURE.md](./PLUGIN_ARCHITECTURE.md) - Plugin system explained
