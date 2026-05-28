# Architecture

## System Overview

The notification service is a centralized, multi-tenant platform for sending emails, SMS, and push notifications. It decouples your applications from direct provider integrations by providing a single REST API that handles routing, delivery guarantees, retries, and audit logging.

```
┌─────────────┐  POST /notifications/send
│  Client App │─────────────────────────────┐
│  (API Key)  │                             │
└─────────────┘                             ▼
                                   ┌─────────────────┐
                                   │  Server (Echo)   │
                                   │  cmd/server/     │
                                   └────────┬────────┘
                                            │
                              ┌─────────────┼─────────────┐
                              │             │             │
                              ▼             ▼             ▼
                       ┌──────────┐  ┌──────────┐  ┌──────────┐
                       │PostgreSQL│  │RabbitMQ  │  │  Health  │
                       │ (Audit,  │  │ (Queue)  │  │  Check   │
                       │  Config) │  └────┬─────┘  │          │
                       └──────────┘       │       └──────────┘
                                          │
                                          ▼
                                   ┌─────────────────┐
                                   │  Worker Pool     │
                                   │  cmd/worker/     │
                                   └────────┬────────┘
                                            │
                              ┌─────────────┼─────────────┐
                              │             │             │
                              ▼             ▼             ▼
                       ┌──────────┐  ┌──────────┐  ┌──────────┐
                       │  SMTP    │  │   SMS    │  │   Push   │
                       │ Provider │  │ Provider │  │ Provider │
                       └──────────┘  └──────────┘  └──────────┘
```

Key architectural properties:
- **Async processing**: API returns 202 immediately, worker sends in background
- **At-least-once delivery**: Retries with exponential backoff + Dead Letter Queue
- **Multi-tenant**: Apps are isolated with their own API keys and provider overrides
- **Plugin-based providers**: Add new delivery methods via Go interface

---

## Components

### 1. Server (`cmd/server/`)

HTTP REST API built with Echo framework. Responsible for:

- **Authentication**: Validates `Authorization: Bearer <key>` via middleware, resolves `app_id` and permissions
- **Validation**: Checks request fields, idempotency keys, and provider permissions
- **Persistence**: Stores notification record in PostgreSQL
- **Queueing**: Publishes message to RabbitMQ for async processing
- **App Management**: CRUD endpoints for apps and API keys
- **Admin Dashboard**: Web UI and API for operational management

Routes:

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | /health | No | Liveness check |
| POST | /apps | No | Register a new app (bootstrap) |
| GET | /apps | Yes | List all registered apps |
| GET | /apps/:id | Yes | Get app details |
| POST | /apps/:id/api-keys | Yes | Generate new API key for an app |
| DELETE | /api-keys/:id | Yes | Revoke an API key |
| GET | /apps/:id/usage | Yes | Paginated notification history for an app |
| POST | /notifications/send | Yes | Send a notification |
| GET | /notifications/:id | Yes | Get notification status |
| GET | /admin | No | Admin dashboard HTML UI |
| POST/GET/DELETE | /admin/* | No | Admin API endpoints |

### 2. Worker (`cmd/worker/`)

Background process that consumes messages from RabbitMQ and sends notifications. Runs independently and scales horizontally.

- **Queue Consumer**: Pulls messages from `notifications` queue with prefetch 10
- **Provider Resolution**: Resolves per-app provider instances with merged config (see Provider Resolution below)
- **Delivery**: Calls `provider.Send()` with a 30-second timeout
- **Retry Logic**: Exponential backoff (5s, 10s, 20s, 40s, 80s) up to 5 attempts, then DLQ
- **Queue Reconciler**: Background goroutine that re-publishes stale pending notifications (older than 5 minutes) to recover from RabbitMQ failures

### 3. RabbitMQ (`pkg/queue/`)

Three queues for reliable delivery:

| Queue | Purpose | TTL |
|-------|---------|-----|
| `notifications` | Main work queue (persistent) | 24 hours |
| `notifications-retry` | Delayed retry with TTL, re-publishes to main queue on expiry | 5 minutes |
| `notifications-dlq` | Dead letter queue for final failures | None |

Messages are JSON-serialized `queue.Message` with payload: `{id, app_id, provider, recipient, subject, content, metadata, retries, delivery_mode}`.

### 4. PostgreSQL (`pkg/database/`)

Tables:

| Table | Purpose |
|-------|---------|
| `apps` | Registered client applications with allowed provider permissions |
| `api_keys` | SHA256-hashed API keys linked to apps (one key per app) |
| `provider_configs` | Global and per-app provider configurations (JSON) |
| `notifications` | Full audit trail of all notifications with status and retry info |

---

## Multi-Tenant Model

### Apps and API Keys

Each client application is registered as an `App` with:
- A unique name
- A list of allowed providers (JSON array)

Each App has exactly one API key (enforced by unique `app_id` on `api_keys`). The raw key is a random 32-byte hex string prefixed with `sk_`. Only the SHA256 hash is stored in the database.

```
┌──────────────────────────────────────────────────┐
│  App "acme-billing"                              │
│  AllowedProviders: ["smtp", "sms"]               │
│  API Key: sk_4f8a... (SHA256 hash stored)        │
└──────────────────────────────────────────────────┘
```

Auth flow:
1. Client sends `Authorization: Bearer sk_4f8a...`
2. Server hashes the key with SHA256
3. Looks up hash in `api_keys` table → resolves `app_id`
4. Loads the App to get `allowed_providers`
5. Sets `app_id` and `allowed_providers` in Echo context

### Provider Config Resolution

Provider configs support two scopes:

| Scope | `app_id` | Use case |
|-------|----------|----------|
| Global | `NULL` | Shared infrastructure (SMTP relay host/port, credentials) |
| Per-app | `app-xxx` | App-specific overrides (from address, sender ID) |

Resolution algorithm for a given `(provider, appID)`:

1. Look for active config with matching `provider` and `app_id`
2. If not found, fall back to active global config (`app_id IS NULL`)
3. If still not found, return error

The merge resolution model combines both:

```
Global config:  {"host":"smtp.sendgrid.net","port":587,"from":"default@company.com"}
App override:   {"from":"billing@acme.com"}
Merged result:  {"host":"smtp.sendgrid.net","port":587,"from":"billing@acme.com"}
```

Per-app configs can also stand alone (no global fallback needed) for providers like Firebase that are entirely per-project.

### Permission Validation

When `POST /notifications/send` is called, the server validates that the requested provider is in the app's `allowed_providers` list. Comparison is case-insensitive.

- If allowed list is empty (`[]`), all providers are denied
- If allowed list contains `["smtp"]`, only SMTP is allowed
- The 403 response includes the provider name that was denied

---

## Provider Plugin Architecture

### Interface

```go
type Provider interface {
    Name() string
    Send(ctx context.Context, req *NotificationRequest) (*NotificationResponse, error)
    Initialize(config map[string]interface{}) error
    Health() error
}
```

### Registry Pattern

Providers register themselves via `init()` functions:

```go
func init() { Register("smtp", NewSMTPProvider) }
```

The registry is a `map[string]ProviderFactory`. When Go imports `pkg/providers`, all `init()` functions run, populating the registry at startup.

### Worker Provider Resolution

The worker maintains two caches:

1. **Global providers**: Initialized at startup from `provider_configs WHERE app_id IS NULL AND is_active = true`
2. **Per-app providers**: Resolved lazily when processing the first notification for each `(appID, provider)` pair

Per-app resolution:
```
Notification arrives with {AppID: "acme", Provider: "smtp"}
    ↓
getOrCreateProvider("acme", "smtp")
    ↓
Check cache: worker.appProviders["acme"]["smtp"]
    ↓ (miss)
db.GetMergedProviderConfig("smtp", "acme") → merged config
    ↓
providers.Create("smtp", mergedConfig) → SMTPProvider instance
    ↓
Cache: worker.appProviders["acme"]["smtp"] = instance
    ↓
Send notification
```

If per-app resolution fails, the worker falls back to the global provider and logs a warning.

---

## Delivery Guarantees

### At-Least-Once (default)

1. Server stores notification as `pending` in DB
2. Publishes to RabbitMQ (persistent delivery mode)
3. Worker receives, sends, updates status to `sent`
4. Worker acks the message (manual ack mode)

On failure:
- Worker does NOT ack (message remains in queue) for transient errors
- For provider failures: publishes to `notifications-retry` with TTL, acks original
- Retry queue re-publishes to main queue after TTL expires
- After 5 retries: moves to DLQ, updates status to `failed`

### At-Most-Once

- Worker sends once, does not retry on failure
- If failed, immediately moves to DLQ

### Queue Reconciler

A background goroutine in the worker runs every 5 minutes and queries:
```sql
SELECT * FROM notifications
WHERE status = 'pending' AND updated_at < NOW() - INTERVAL '5 minutes'
```

Stale pending notifications are re-published to RabbitMQ and their `updated_at` is bumped. This acts as a safety net for messages lost during RabbitMQ crashes or other failures.

---

## Database Schema

### apps
```sql
CREATE TABLE apps (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    description TEXT,
    allowed_providers TEXT NOT NULL DEFAULT '[]',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### api_keys
```sql
CREATE TABLE api_keys (
    id VARCHAR(36) PRIMARY KEY,
    app_id VARCHAR(255) UNIQUE NOT NULL,
    key_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMP NULL
);
```

### provider_configs
```sql
CREATE TABLE provider_configs (
    id VARCHAR(36) PRIMARY KEY,
    provider VARCHAR(50) NOT NULL,
    app_id VARCHAR(36) NULL,        -- NULL = global, non-NULL = per-app
    config TEXT NOT NULL,            -- JSON-encoded provider config
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### notifications
```sql
CREATE TABLE notifications (
    id VARCHAR(36) PRIMARY KEY,
    app_id VARCHAR(255) NOT NULL,
    provider VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    recipient TEXT NOT NULL,
    subject TEXT,
    content TEXT NOT NULL,
    delivery_mode VARCHAR(20),
    provider_ref VARCHAR(255),
    error_message TEXT,
    retry_count INTEGER DEFAULT 0,
    last_retry_at TIMESTAMP NULL,
    next_retry_at TIMESTAMP NULL,
    idempotency_key VARCHAR(255) UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    delivered_at TIMESTAMP NULL
);
```

---

## Tests

| Package | Coverage | Type |
|---------|----------|------|
| `pkg/database` | Merge logic, config parsing | Unit tests |
| `pkg/database` | Provider config resolution, notifications query | Integration (requires DATABASE_URL) |
| `pkg/providers` | Registry, create, duplicate detection | Unit tests |
| `internal/handlers` | Permission validation, request validation | Unit tests |
| `internal/middleware` | Auth header parsing, missing/invalid keys | Unit tests |

Test files are co-located with source (`*_test.go`). Integration tests skip gracefully when `DATABASE_URL` is not set.

---

## File Structure

```
.
├── cmd/
│   ├── server/main.go         → REST API server entry point
│   ├── worker/main.go         → Queue consumer worker entry point
│   └── migrate/main.go        → Database migration tool
├── internal/
│   ├── handlers/
│   │   ├── notification.go    → Send/status endpoints, permission validation
│   │   ├── app.go             → App management endpoints
│   │   ├── admin.go           → Admin dashboard + API
│   │   └── admin_html.go      → Admin HTML UI
│   ├── middleware/
│   │   └── auth.go            → Bearer token authentication middleware
├── pkg/
│   ├── database/
│   │   ├── db.go              → PostgreSQL client and queries
│   │   ├── db_test.go         → Integration tests
│   │   └── merge_test.go      → Merge logic unit tests
│   ├── models/
│   │   └── notification.go    → All database models
│   ├── providers/
│   │   ├── interface.go       → Provider interface + registry
│   │   ├── errors.go          → Provider error types
│   │   ├── smtp.go            → SMTP email provider
│   │   ├── sms.go             → SMS provider (mock)
│   │   └── providers_test.go  → Registry unit tests
│   └── queue/
│       ├── queue.go           → RabbitMQ client
│       └── queue_test.go      → Queue unit tests
├── migrations/
│   └── 00001_init_schema.sql  → Initial database schema
├── docker-compose.yml         → Local development environment
├── Dockerfile.server          → Server container build
├── Dockerfile.worker          → Worker container build
├── ARCHITECTURE.md            → This document
├── ARCHITECTURE_DECISIONS.md  → Technology choice rationale
├── PLUGIN_ARCHITECTURE.md     → How to add new providers
├── README.md                  → Main documentation
├── PROJECT_SUMMARY.md         → Project overview
├── GETTING_STARTED.md         → Step-by-step tutorial
├── QUICK_REFERENCE.md         → Common commands reference
└── 00_START_HERE.md           → Onboarding guide
```
