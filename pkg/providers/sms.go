package providers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// MockSMSProvider is a placeholder for SMS providers (Twilio, AWS SNS, etc)
type MockSMSProvider struct {
	APIURL string
	APIKey string
}

// NewMockSMSProvider factory function
func NewMockSMSProvider(config map[string]interface{}) (Provider, error) {
	p := &MockSMSProvider{}

	if err := p.parseConfig(config); err != nil {
		return nil, err
	}

	return p, nil
}

func (p *MockSMSProvider) parseConfig(config map[string]interface{}) error {
	apiURL, ok := config["api_url"].(string)
	if !ok || apiURL == "" {
		return fmt.Errorf("%w: missing 'api_url'", ErrInvalidConfig)
	}
	p.APIURL = apiURL

	apiKey, ok := config["api_key"].(string)
	if !ok || apiKey == "" {
		return fmt.Errorf("%w: missing 'api_key'", ErrInvalidConfig)
	}
	p.APIKey = apiKey

	return nil
}

func (p *MockSMSProvider) Name() string {
	return "sms"
}

func (p *MockSMSProvider) Initialize(config map[string]interface{}) error {
	return p.parseConfig(config)
}

func (p *MockSMSProvider) Send(ctx context.Context, req *NotificationRequest) (*NotificationResponse, error) {
	// In production: call Twilio/AWS SNS/etc API
	// This is a mock that logs the request
	fmt.Printf("[MockSMS] Sending to %s: %s\n", req.Recipient, req.Content)

	client := &http.Client{}
	form := url.Values{}
	form.Add("to", req.Recipient)
	form.Add("message", req.Content)

	httpReq, _ := http.NewRequestWithContext(ctx, "POST", p.APIURL, nil)
	httpReq.Header.Add("Authorization", "Bearer "+p.APIKey)

	resp, err := client.Do(httpReq)
	if err != nil {
		return &NotificationResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return &NotificationResponse{
			Success:     true,
			ProviderRef: "sms-" + req.ID,
		}, nil
	}

	return &NotificationResponse{
		Success:      false,
		ErrorMessage: fmt.Sprintf("SMS API returned status %d", resp.StatusCode),
	}, nil
}

func (p *MockSMSProvider) Health() error {
	// Check API connectivity
	resp, err := http.Head(p.APIURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("SMS API unhealthy: %d", resp.StatusCode)
	}

	return nil
}

func init() {
	Register("sms", NewMockSMSProvider)
}
