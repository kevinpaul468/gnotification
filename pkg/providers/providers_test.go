package providers

import (
	"context"
	"testing"
)

type testProvider struct {
	name string
}

func (p *testProvider) Name() string {
	return p.name
}

func (p *testProvider) Send(ctx context.Context, req *NotificationRequest) (*NotificationResponse, error) {
	return &NotificationResponse{Success: true, ProviderRef: "test-" + req.ID}, nil
}

func (p *testProvider) Initialize(config map[string]interface{}) error {
	return nil
}

func (p *testProvider) Health() error {
	return nil
}

func TestRegisterAndCreate(t *testing.T) {
	name := "test_provider"
	Register(name, func(config map[string]interface{}) (Provider, error) {
		return &testProvider{name: name}, nil
	})

	p, err := Create(name, nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if p.Name() != name {
		t.Errorf("Name() = %s, want %s", p.Name(), name)
	}
}

func TestCreate_NotFound(t *testing.T) {
	_, err := Create("nonexistent", nil)
	if err == nil {
		t.Fatal("Create() expected error for nonexistent provider")
	}
}

func TestGetRegistered(t *testing.T) {
	names := GetRegistered()
	found := false
	for _, n := range names {
		if n == "smtp" {
			found = true
			break
		}
	}
	if !found {
		t.Error("GetRegistered() should include 'smtp'")
	}
}

func TestRegister_DuplicatePanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Register() should panic on duplicate")
		}
	}()

	Register("duplicate_test", func(config map[string]interface{}) (Provider, error) {
		return &testProvider{name: "dup"}, nil
	})
	Register("duplicate_test", func(config map[string]interface{}) (Provider, error) {
		return &testProvider{name: "dup"}, nil
	})
}

func TestNotificationRequest(t *testing.T) {
	req := &NotificationRequest{
		ID:        "test-id",
		Recipient: "user@example.com",
		Subject:   "Hello",
		Content:   "World",
	}
	if req.ID != "test-id" {
		t.Errorf("ID mismatch")
	}
	if req.Recipient != "user@example.com" {
		t.Errorf("Recipient mismatch")
	}
}

func TestNotificationResponse(t *testing.T) {
	resp := &NotificationResponse{
		Success:     true,
		ProviderRef: "ref-123",
		ErrorMessage: "",
	}
	if !resp.Success {
		t.Error("expected success")
	}
	if resp.ProviderRef != "ref-123" {
		t.Errorf("ProviderRef mismatch")
	}
}
