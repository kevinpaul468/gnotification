package models

import "time"

const (
	StatusPending   = "pending"
	StatusSent      = "sent"
	StatusFailed    = "failed"
	StatusDelivered = "delivered"
	StatusBounced   = "bounced"
)

const (
	AtLeastOnce = "at_least_once"
	AtMostOnce  = "at_most_once"
)

// Notification tracks a notification record
type Notification struct {
	ID             string     `gorm:"primaryKey"`
	AppID          string     `gorm:"index"` // which app sent this
	Provider       string     `gorm:"index"` // email, sms, push, etc
	Status         string     `gorm:"index"` // pending, sent, failed, delivered
	Recipient      string     // email, phone number, user ID
	Subject        string
	Content        string
	DeliveryMode   string     // at_least_once, at_most_once
	ProviderRef    string     // reference from provider (message ID)
	ErrorMessage   string
	RetryCount     int
	LastRetryAt    *time.Time
	NextRetryAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeliveredAt    *time.Time
	IdempotencyKey string    `gorm:"index,unique"` // prevent duplicate sends
}

// TableName specifies custom table name
func (Notification) TableName() string {
	return "notifications"
}

// APIKey tracks API keys for app authentication
type APIKey struct {
	ID        string    `gorm:"primaryKey"`
	AppID     string    `gorm:"index,unique"`
	KeyHash   string    // bcrypt hash of actual key
	CreatedAt time.Time
	UpdatedAt time.Time
	LastUsedAt *time.Time
}

func (APIKey) TableName() string {
	return "api_keys"
}

// ProviderConfig stores provider configurations
type ProviderConfig struct {
	ID        string    `gorm:"primaryKey"`
	Provider  string    `gorm:"index"` // smtp, sms, push
	Config    string    // JSON-encoded config
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (ProviderConfig) TableName() string {
	return "provider_configs"
}
