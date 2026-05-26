package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/swecha/notifications/pkg/database"
)

type AuthMiddleware struct {
	db *database.DB
}

func NewAuthMiddleware(db *database.DB) *AuthMiddleware {
	return &AuthMiddleware{db: db}
}

func (m *AuthMiddleware) Authenticate(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		authHeader := c.Request().Header.Get("Authorization")
		if authHeader == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Missing authorization header"})
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid authorization header format"})
		}

		keyHash := sha256.Sum256([]byte(parts[1]))
		hashStr := hex.EncodeToString(keyHash[:])

		apiKey, err := m.db.GetAPIKey(hashStr)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid API key"})
		}

		app, err := m.db.GetApp(apiKey.AppID)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "App not found"})
		}

		c.Set("app_id", apiKey.AppID)
		c.Set("allowed_providers", app.AllowedProviders)

		return next(c)
	}
}
