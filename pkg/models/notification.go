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
	ID           string     `gorm:"primaryKey"`
	AppID        string     `gorm:"index,unique"`
	KeyHash      string     // bcrypt hash of actual key
	Name         string     // Human-readable name (e.g., "Production", "Staging")
	Status       string     `gorm:"index"` // active, revoked, expired
	CreatedAt    time.Time
	UpdatedAt    time.Time
	LastUsedAt   *time.Time
	ExpiresAt    *time.Time `gorm:"index"` // null = never expires
	RevokedAt    *time.Time // when it was revoked
	RevokeReason string     // why it was revoked
	RateLimit    int        // requests per minute (0 = unlimited)
	CreatedBy    string     // which admin created it
	RevokedBy    string     // which admin revoked it
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

// APIKeyRequest tracks API key requests from users
type APIKeyRequest struct {
	ID            string    `gorm:"primaryKey"`
	AppName       string    `gorm:"index"` // Name of the application
	AppID         string    `gorm:"index"` // Requested app ID
	Email         string    // Requester email
	CompanyName   string    // Company/organization name
	Purpose       string    // Purpose of the API key
	Status        string    `gorm:"index"` // pending, approved, rejected
	AdminComment  string    // Reason for approval/rejection
	CreatedAt     time.Time
	UpdatedAt     time.Time
	ApprovedAt    *time.Time
	ApprovedBy    string    // Admin user who approved
}

func (APIKeyRequest) TableName() string {
	return "api_key_requests"
}

// APIKeyRotation tracks when keys were rotated
type APIKeyRotation struct {
	ID           string    `gorm:"primaryKey"`
	OldKeyID     string    // Previous key ID
	NewKeyID     string    // New key ID
	AppID        string    `gorm:"index"`
	Reason       string    // why rotated (expired, compromised, etc)
	RotatedAt    time.Time
	RotatedBy    string    // admin who initiated rotation
}

func (APIKeyRotation) TableName() string {
	return "api_key_rotations"
}

// APIKeyUsage tracks API key usage metrics
type APIKeyUsage struct {
	ID           string    `gorm:"primaryKey"`
	KeyID        string    `gorm:"index"`
	RequestCount int64     // Number of requests
	LastUsedAt   time.Time
	FirstUsedAt  time.Time
	BytesIn      int64     // Total request bytes
	BytesOut     int64     // Total response bytes
}

func (APIKeyUsage) TableName() string {
	return "api_key_usage"
}
