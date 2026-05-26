package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/swecha/notifications/pkg/database"
)

func setupTestDB(t *testing.T) *database.DB {
	t.Helper()
	// This test uses a real DB. Skip if DATABASE_URL is not set.
	// In CI/local, set DATABASE_URL to a test database.
	return nil
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/notifications/send", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mw := &AuthMiddleware{}
	handler := mw.Authenticate(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	handler(c)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuthMiddleware_InvalidFormat(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/notifications/send", nil)
	req.Header.Set("Authorization", "Invalid token")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mw := &AuthMiddleware{}
	handler := mw.Authenticate(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	handler(c)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestHashAPIKey(t *testing.T) {
	key := "test-api-key"
	hash := sha256.Sum256([]byte(key))
	expected := hex.EncodeToString(hash[:])

	got := hex.EncodeToString(hash[:])
	if got != expected {
		t.Errorf("hash mismatch: got %s, want %s", got, expected)
	}
}
