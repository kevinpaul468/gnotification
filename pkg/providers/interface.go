package providers

import "context"

// NotificationRequest represents a generic notification request
type NotificationRequest struct {
	ID        string                 `json:"id"`
	Recipient string                 `json:"recipient"` // email, phone, user_id, etc
	Subject   string                 `json:"subject,omitempty"`
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// NotificationResponse represents the result of sending a notification
type NotificationResponse struct {
	Success      bool
	ProviderRef  string // reference from provider (message ID, tracking ID, etc)
	ErrorMessage string
	Timestamp    int64
}

// Provider is the base interface all notification providers must implement
// This is how plugin architecture works in Go - interfaces!
type Provider interface {
	// Name returns provider identifier
	Name() string

	// Send attempts to send a notification
	Send(ctx context.Context, req *NotificationRequest) (*NotificationResponse, error)

	// Initialize sets up provider with config (SMTP creds, API keys, etc)
	// config format is provider-specific
	Initialize(config map[string]interface{}) error

	// Validate checks if provider is healthy
	Health() error
}

// ProviderFactory creates provider instances
type ProviderFactory func(config map[string]interface{}) (Provider, error)

// ProviderRegistry holds all registered providers
var providerRegistry = make(map[string]ProviderFactory)

// Register makes a provider available for use
func Register(name string, factory ProviderFactory) {
	if providerRegistry[name] != nil {
		panic("provider " + name + " already registered")
	}
	providerRegistry[name] = factory
}

// Create instantiates a provider by name
func Create(name string, config map[string]interface{}) (Provider, error) {
	factory, exists := providerRegistry[name]
	if !exists {
		return nil, ErrProviderNotFound
	}
	return factory(config)
}

// GetRegistered returns all registered provider names
func GetRegistered() []string {
	var names []string
	for name := range providerRegistry {
		names = append(names, name)
	}
	return names
}
