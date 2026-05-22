# Notification Service - Plugin Architecture Guide

## Overview

The notification service uses a **plugin architecture** that differs from Python wheels. Instead of dynamic loading, we use **interface-based provider registration**.

## How It Works (Go vs Python)

### Python Approach (Wheel)
```python
# Install wheel
pip install notifications-smtp-plugin-1.0.whl

# Auto-discovery via module imports
from plugins import smtp_provider
```

### Go Approach (Interface-Based)
```go
// 1. Define interface (pkg/providers/interface.go)
type Provider interface {
    Name() string
    Send(ctx context.Context, req *NotificationRequest) (*NotificationResponse, error)
    Initialize(config map[string]interface{}) error
    Health() error
}

// 2. Implement interface (pkg/providers/smtp.go)
type SMTPProvider struct { ... }
func (p *SMTPProvider) Send(...) { ... }

// 3. Auto-register (init() function runs on import)
func init() {
    Register("smtp", NewSMTPProvider)
}

// 4. Use via registry
provider, err := providers.Create("smtp", config)
```

## Key Differences

| Aspect | Python Wheels | Go Interfaces |
|--------|---------------|---------------|
| **Compilation** | Runtime (interpreted) | Build-time (compiled) |
| **Plugin Loading** | Dynamic (pip install) | Compile plugins into binary |
| **Memory** | Higher (runtime interpreter) | Lower (compiled) |
| **Speed** | Slower startup | Instant startup |
| **Type Safety** | Runtime errors | Compile-time safety |

## Creating a New Provider Plugin

### Step 1: Create Provider Implementation

```go
// pkg/providers/telegram.go
package providers

import (
    "context"
    "fmt"
    "net/http"
)

type TelegramProvider struct {
    BotToken string
}

func NewTelegramProvider(config map[string]interface{}) (Provider, error) {
    p := &TelegramProvider{}
    if err := p.parseConfig(config); err != nil {
        return nil, err
    }
    return p, nil
}

func (p *TelegramProvider) parseConfig(config map[string]interface{}) error {
    token, ok := config["bot_token"].(string)
    if !ok || token == "" {
        return fmt.Errorf("%w: missing 'bot_token'", ErrInvalidConfig)
    }
    p.BotToken = token
    return nil
}

func (p *TelegramProvider) Name() string {
    return "telegram"
}

func (p *TelegramProvider) Initialize(config map[string]interface{}) error {
    return p.parseConfig(config)
}

func (p *TelegramProvider) Send(ctx context.Context, req *NotificationRequest) (*NotificationResponse, error) {
    // Implementation
    url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", p.BotToken)
    // ... call Telegram API
    return &NotificationResponse{Success: true}, nil
}

func (p *TelegramProvider) Health() error {
    // Test bot token validity
    return nil
}

// IMPORTANT: Register on init
func init() {
    Register("telegram", NewTelegramProvider)
}
```

### Step 2: Rebuild and Deploy

```bash
go build -o notification-server ./cmd/server
go build -o notification-worker ./cmd/worker
```

The new provider is now available!

## Adding SMS Provider Plugins

Here's how to add multiple SMS providers:

### Twilio SMS Provider

```go
// pkg/providers/twilio.go
package providers

type TwilioProvider struct {
    AccountSID string
    AuthToken  string
    FromNumber string
}

func NewTwilioProvider(config map[string]interface{}) (Provider, error) { ... }

func (p *TwilioProvider) Name() string { return "twilio" }

func (p *TwilioProvider) Send(ctx context.Context, req *NotificationRequest) (*NotificationResponse, error) {
    // Call Twilio API
}

func init() {
    Register("twilio", NewTwilioProvider)
}
```

### AWS SNS SMS Provider

```go
// pkg/providers/aws_sns.go
package providers

type AWSSNSProvider struct {
    AccessKey    string
    SecretKey    string
    Region       string
}

func NewAWSSNSProvider(config map[string]interface{}) (Provider, error) { ... }

func (p *AWSSNSProvider) Name() string { return "aws_sns" }

func (p *AWSSNSProvider) Send(ctx context.Context, req *NotificationRequest) (*NotificationResponse, error) {
    // Call AWS SNS API
}

func init() {
    Register("aws_sns", NewAWSSNSProvider)
}
```

## Provider Configuration Storage

Providers are configured in the database:

```sql
INSERT INTO provider_configs (id, provider, config, is_active)
VALUES (
    'cfg-1',
    'smtp',
    '{"host":"smtp.gmail.com","port":587,"username":"your-email@gmail.com","password":"app-password","from":"noreply@example.com"}',
    true
);

INSERT INTO provider_configs (id, provider, config, is_active)
VALUES (
    'cfg-2',
    'twilio',
    '{"account_sid":"AC...","auth_token":"...","from_number":"+1234567890"}',
    true
);
```

## How Worker Discovers Providers

```
1. Worker starts → reads all active configs from DB
2. For each config, calls providers.Create(providerName, configMap)
3. Registry lookup finds factory function
4. Factory creates provider instance with config
5. Worker now has initialized provider ready to send
```

```
DB: {provider: "smtp", config: {...}}
    ↓
Worker: providers.Create("smtp", {...})
    ↓
Registry: providerRegistry["smtp"](config)
    ↓
Result: SMTPProvider instance ready to send
```

## Advantages of This Approach

✅ **Compile-time safety** - Type errors caught during build, not runtime
✅ **Single binary deployment** - No dependency hell, no version conflicts  
✅ **Self-contained** - All providers compiled into one binary
✅ **Easy to extend** - Just add new file, implement interface, rebuild
✅ **No dynamic loading complexity** - No reflection magic needed
✅ **Better performance** - Compiled code runs faster

## Adding Push Notification Plugin Later

```go
// pkg/providers/firebase.go
package providers

type FirebaseProvider struct {
    ProjectID string
    PrivateKey string
}

func (p *FirebaseProvider) Send(ctx context.Context, req *NotificationRequest) (*NotificationResponse, error) {
    // Send to FCM (Firebase Cloud Messaging)
}

func init() {
    Register("firebase", NewFirebaseProvider)
}
```

Then in config:
```sql
INSERT INTO provider_configs (id, provider, config, is_active)
VALUES ('cfg-3', 'firebase', '{"project_id":"...","private_key":"..."}', true);
```

## Project Structure Summary

```
notifications/
├── pkg/
│   ├── providers/
│   │   ├── interface.go      ← Core Provider interface
│   │   ├── errors.go
│   │   ├── smtp.go           ← Email provider plugin
│   │   ├── sms.go            ← SMS provider plugin
│   │   ├── telegram.go       ← Can add more plugins here
│   │   └── firebase.go       ← Future push notifications
│   ├── models/
│   ├── queue/
│   └── database/
├── cmd/
│   ├── server/main.go        ← API server
│   └── worker/main.go        ← Message worker
```

**That's it!** Go's interface system + init() functions = plugin architecture without the complexity.
