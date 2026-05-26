package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestPermissionValidation_Denied(t *testing.T) {
	allowed, _ := json.Marshal([]string{"smtp", "push"})
	body := `{"provider":"sms","recipient":"user@example.com","content":"test","delivery_mode":"at_least_once"}`
	req := httptest.NewRequest(http.MethodPost, "/notifications/send", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.Set("app_id", "test-app")
	c.Set("allowed_providers", string(allowed))

	handler := &NotificationHandler{}
	err := handler.SendNotification(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestPermissionValidation_NoAppID(t *testing.T) {
	allowed, _ := json.Marshal([]string{"smtp"})
	body := `{"provider":"smtp","recipient":"user@example.com","content":"test","delivery_mode":"at_least_once"}`
	req := httptest.NewRequest(http.MethodPost, "/notifications/send", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.Set("allowed_providers", string(allowed))

	handler := &NotificationHandler{}
	err := handler.SendNotification(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestPermissionValidation_EmptyAllowed(t *testing.T) {
	body := `{"provider":"smtp","recipient":"user@example.com","content":"test","delivery_mode":"at_least_once"}`
	req := httptest.NewRequest(http.MethodPost, "/notifications/send", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.Set("app_id", "test-app")
	c.Set("allowed_providers", `[]`)

	handler := &NotificationHandler{}
	err := handler.SendNotification(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for empty allowed list, got %d", rec.Code)
	}
}

func TestSendNotification_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/notifications/send", strings.NewReader("{invalid}"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.Set("app_id", "test-app")

	handler := &NotificationHandler{}
	err := handler.SendNotification(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", rec.Code)
	}
}

func TestHashAPIKey(t *testing.T) {
	hash1 := hashAPIKey("test-key")
	hash2 := hashAPIKey("test-key")

	if hash1 != hash2 {
		t.Error("hashAPIKey should be deterministic")
	}
	if len(hash1) != 64 {
		t.Errorf("expected 64 hex chars, got %d", len(hash1))
	}
}
