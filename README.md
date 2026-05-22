# Notification Service

A self-hostable notification service supporting multiple providers (email, SMS, push notifications) with a plugin architecture.

## Features

- ✅ Plugin-based provider architecture (easily add email, SMS, push providers)
- ✅ Multiple delivery guarantees (at-least-once, at-most-once)
- ✅ RabbitMQ message queue with dead letter queues
- ✅ Automatic retry with exponential backoff
- ✅ Idempotency support (prevent duplicate sends)
- ✅ Built-in audit trail (tracks all sends)
- ✅ Provider health checks
- ✅ Self-hostable (single binary + PostgreSQL + RabbitMQ)

## Architecture

```
Your Apps → Notification API → RabbitMQ Queue
                                    ↓
                          Worker Pool (multiple instances)
                                    ↓
                    ┌───────────────┼───────────────┐
                    ↓               ↓               ↓
              SMTP Provider    SMS Provider    Push Provider
                    ↓               ↓               ↓
              Gmail/SMTP      Twilio/AWS SNS   Firebase/etc
```

## Quick Start

### 1. Prerequisites

- Docker & Docker Compose
- Go 1.21+ (for local development)
- PostgreSQL 14+
- RabbitMQ 3.12+

### 2. Setup with Docker Compose

```bash
docker-compose up -d
```

This starts:
- PostgreSQL (port 5432)
- RabbitMQ (port 5672, UI on 15672)
- Notification Server (port 8080)
- Notification Worker (processes messages)

### 3. Configure Providers

Create a provider config:

```bash
curl -X POST http://localhost:8080/admin/providers/config \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "smtp",
    "config": {
      "host": "smtp.gmail.com",
      "port": 587,
      "username": "your-email@gmail.com",
      "password": "your-app-password",
      "from": "noreply@example.com"
    },
    "is_active": true
  }'
```

### 4. Send a Notification

```bash
curl -X POST http://localhost:8080/notifications/send \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "provider": "smtp",
    "recipient": "user@example.com",
    "subject": "Welcome!",
    "content": "Hello from notification service",
    "delivery_mode": "at_least_once",
    "idempotency_key": "unique-request-id-123"
  }'
```

Response:
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "pending"
}
```

### 5. Check Status

```bash
curl http://localhost:8080/notifications/550e8400-e29b-41d4-a716-446655440000
```

## API Endpoints

### Send Notification

**POST** `/notifications/send`

Request:
```json
{
  "provider": "smtp|sms|push|...",
  "recipient": "email@example.com or phone or user_id",
  "subject": "Email subject (optional)",
  "content": "Message content",
  "delivery_mode": "at_least_once|at_most_once",
  "idempotency_key": "unique-key-for-dedup (optional)",
  "metadata": { "custom_field": "value" }
}
```

Response: `202 Accepted`
```json
{
  "id": "notification-uuid",
  "status": "pending"
}
```

### Get Notification Status

**GET** `/notifications/{id}`

Response:
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "app_id": "my-app",
  "provider": "smtp",
  "status": "sent|pending|failed|delivered",
  "recipient": "user@example.com",
  "subject": "Welcome!",
  "content": "Hello from notification service",
  "delivery_mode": "at_least_once",
  "provider_ref": "message-id-from-smtp",
  "error_message": "",
  "retry_count": 0,
  "created_at": "2024-05-22T14:23:04Z",
  "delivered_at": "2024-05-22T14:23:10Z"
}
```

### Health Check

**GET** `/health`

Response:
```json
{
  "status": "ok"
}
```

## Configuration

### Environment Variables

```bash
# Database
DATABASE_URL="postgres://user:password@localhost:5432/notifications"

# RabbitMQ
RABBITMQ_URL="amqp://guest:guest@localhost:5672/"

# Server
PORT=8080
ENVIRONMENT=production
```

### Provider Configuration

Store provider configs in `provider_configs` table:

```sql
INSERT INTO provider_configs (id, provider, config, is_active)
VALUES (
  'cfg-smtp-1',
  'smtp',
  '{"host":"smtp.gmail.com","port":587,"username":"...","password":"...","from":"..."}',
  true
);
```

## Plugin Architecture

Adding a new provider is simple! See [PLUGIN_ARCHITECTURE.md](./PLUGIN_ARCHITECTURE.md) for detailed guide.

### Example: Add Telegram Provider

1. Create `pkg/providers/telegram.go`
2. Implement `Provider` interface
3. Add `init()` function to register
4. Rebuild: `go build ./cmd/server && go build ./cmd/worker`
5. Configure in DB
6. Done! ✨

## Building from Source

### Server Binary

```bash
go build -o bin/notification-server ./cmd/server
./bin/notification-server
```

### Worker Binary

```bash
go build -o bin/notification-worker ./cmd/worker
./bin/notification-worker
```

## Database Schema

### Notifications Table

Tracks all notification attempts:
```sql
CREATE TABLE notifications (
    id UUID PRIMARY KEY,
    app_id VARCHAR(255),
    provider VARCHAR(50),
    status VARCHAR(20),
    recipient TEXT,
    subject TEXT,
    content TEXT,
    delivery_mode VARCHAR(20),
    provider_ref VARCHAR(255),
    error_message TEXT,
    retry_count INT,
    idempotency_key VARCHAR(255) UNIQUE,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    delivered_at TIMESTAMP
);
```

### Failed Notifications Table

Dead letter queue entries:
```sql
CREATE TABLE failed_notifications (
    id UUID PRIMARY KEY,
    notification_id UUID,
    reason TEXT,
    next_retry_at TIMESTAMP,
    attempts INT,
    last_error TEXT,
    created_at TIMESTAMP
);
```

### Provider Configs Table

```sql
CREATE TABLE provider_configs (
    id VARCHAR(255) PRIMARY KEY,
    provider VARCHAR(50),
    config JSONB,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```

## Delivery Guarantees

### At-Least-Once (Default)

✅ Guaranteed delivery. Message retried up to 5 times with exponential backoff.

```
Attempt 1: immediate
Attempt 2: after 5s
Attempt 3: after 10s
Attempt 4: after 20s
Attempt 5: after 40s
DLQ: after 80s
```

### At-Most-Once

⚡ Fire and forget. No retries if failed.

Use for non-critical notifications (analytics, tracking).

## Monitoring

### Check RabbitMQ Queue

```bash
curl http://localhost:15672/api/queues
# Username: guest, Password: guest
```

### Failed Messages

```bash
curl http://localhost:8080/admin/failed-notifications
```

## Scaling

### Multiple Workers

Run multiple worker instances for parallel processing:

```bash
./bin/notification-worker &
./bin/notification-worker &
./bin/notification-worker &
```

RabbitMQ automatically distributes messages across workers.

### Load Testing

```bash
# Send 1000 notifications
for i in {1..1000}; do
  curl -X POST http://localhost:8080/notifications/send \
    -H "Content-Type: application/json" \
    -d "{\"provider\":\"smtp\",\"recipient\":\"test$i@example.com\",\"content\":\"Test $i\"}"
done
```

## Troubleshooting

### Worker not processing messages

Check database connection:
```bash
DATABASE_URL=... go run cmd/worker/main.go
```

Check RabbitMQ connection:
```bash
# Connect to RabbitMQ container
docker exec notifications_rabbitmq rabbitmqctl status
```

### Notifications stuck in pending

Check failed notifications table:
```bash
docker exec notifications_db psql -U postgres -d notifications -c "SELECT * FROM failed_notifications ORDER BY created_at DESC LIMIT 10;"
```

### SMTP authentication fails

Verify credentials and test manually:
```bash
telnet smtp.gmail.com 587
```

For Gmail, use [app passwords](https://support.google.com/accounts/answer/185833), not your regular password.

## Production Deployment

### Docker

```bash
docker build -t notification-service:latest .
docker run -e DATABASE_URL=... -e RABBITMQ_URL=... -p 8080:8080 notification-service
```

### Kubernetes

See `k8s/` directory for Kubernetes manifests.

### Systemd Service

```bash
sudo cp notification-server.service /etc/systemd/system/
sudo systemctl enable notification-server
sudo systemctl start notification-server
```

## Contributing

1. Add provider in `pkg/providers/`
2. Implement `Provider` interface
3. Add tests in `pkg/providers/*_test.go`
4. Update documentation
5. Submit PR

## License

MIT

## Support

- Issues: GitHub Issues
- Discussions: GitHub Discussions
- Docs: See `PLUGIN_ARCHITECTURE.md` for extending with new providers

---

## 🎛️ Admin Dashboard

Access the admin dashboard at: **`http://localhost:8080/admin`**

### Features

📋 **Dashboard**
- View real-time statistics (total, pending, sent, failed notifications)
- Monitor success rate and average delivery time
- Track active providers and API keys

🔌 **Provider Management**
- Configure SMTP (email), SMS, and other providers
- Enable/disable providers without code changes
- Store service credentials securely in database
- Support for unlimited provider configurations

🔑 **API Key Management**
- Generate API keys for each application
- Track API key usage and last access time
- Revoke compromised keys instantly
- Each app gets isolated access

📨 **Notification Monitoring**
- View all notifications with full history
- Filter by status (pending, sent, failed, delivered)
- Search by provider, app ID, or recipient
- Monitor retry attempts and error messages

### Quick Start

1. **Start the service:**
   ```bash
   docker-compose up -d
   ```

2. **Open admin panel:**
   ```
   http://localhost:8080/admin
   ```

3. **Add a provider (SMTP example):**
   - Go to Providers tab
   - Select SMTP
   - Enter Gmail credentials:
     ```json
     {
       "host": "smtp.gmail.com",
       "port": 587,
       "username": "your-email@gmail.com",
       "password": "app-password",
       "from": "noreply@example.com",
       "tls": true
     }
     ```
   - Click Save Configuration

4. **Create an API key:**
   - Go to API Keys tab
   - Enter App ID: `my-app`
   - Click Generate API Key
   - Copy the key (shown once only!)

5. **Send a notification:**
   ```bash
   curl -X POST http://localhost:8080/notifications/send \
     -H "Content-Type: application/json" \
     -d '{
       "provider": "smtp",
       "recipient": "user@example.com",
       "subject": "Hello",
       "content": "Test notification",
       "delivery_mode": "at_least_once"
     }'
   ```

### Admin API Endpoints

All endpoints return JSON. No authentication required (add as needed).

#### Dashboard
- `GET /admin/stats` - Get dashboard statistics

#### API Keys
- `POST /admin/api-keys` - Create new API key
- `GET /admin/api-keys` - List all API keys
- `DELETE /admin/api-keys` - Revoke an API key

#### Provider Configs
- `POST /admin/provider-configs` - Save provider configuration
- `GET /admin/provider-configs` - List all configurations
- `DELETE /admin/provider-configs` - Delete configuration

#### Notifications
- `GET /admin/notifications` - List notifications (with filters)
- `GET /admin/providers/available` - List registered providers

### Configuration via Admin UI

Instead of manually inserting into the database, use the admin UI to:

1. **Add new SMTP provider:**
   - Providers → Add Configuration
   - Select "SMTP (Email)"
   - Paste JSON config
   - Toggle Active
   - Save

2. **Add new SMS provider:**
   - Same process with SMS config
   - Include API keys, phone numbers, etc.

3. **Generate API keys:**
   - API Keys → Create New API Key
   - Enter app name
   - Copy and save the key

### For Full Details

See [ADMIN_GUIDE.md](./ADMIN_GUIDE.md) for:
- Detailed UI walkthrough
- All provider configuration examples
- Security best practices
- Troubleshooting guide
- API endpoint reference

