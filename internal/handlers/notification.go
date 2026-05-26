package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/swecha/notifications/pkg/database"
	"github.com/swecha/notifications/pkg/models"
	"github.com/swecha/notifications/pkg/queue"
)

type NotificationHandler struct {
	db    *database.DB
	queue *queue.Queue
}

// NewNotificationHandler creates a new handler
func NewNotificationHandler(db *database.DB, q *queue.Queue) *NotificationHandler {
	return &NotificationHandler{
		db:    db,
		queue: q,
	}
}

// SendNotificationRequest is the API request body
type SendNotificationRequest struct {
	Provider        string                 `json:"provider" validate:"required"`
	Recipient       string                 `json:"recipient" validate:"required"`
	Subject         string                 `json:"subject"`
	Content         string                 `json:"content" validate:"required"`
	DeliveryMode    string                 `json:"delivery_mode" validate:"required,oneof=at_least_once at_most_once"`
	IdempotencyKey  string                 `json:"idempotency_key"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// SendNotification handles POST /notifications/send
func (h *NotificationHandler) SendNotification(c echo.Context) error {
	var req SendNotificationRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Extract app ID from auth middleware
	appID, ok := c.Get("app_id").(string)
	if !ok || appID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
	}

	// Validate provider permission
	allowedRaw := c.Get("allowed_providers")
	if allowedStr, ok := allowedRaw.(string); ok && allowedStr != "" {
		var allowed []string
		if err := json.Unmarshal([]byte(allowedStr), &allowed); err == nil && len(allowed) > 0 {
			hasPermission := false
			for _, p := range allowed {
				if strings.EqualFold(p, req.Provider) {
					hasPermission = true
					break
				}
			}
			if !hasPermission {
				return c.JSON(http.StatusForbidden, map[string]string{
					"error": "App does not have permission for provider: " + req.Provider,
				})
			}
		}
	}

	// Check for duplicate using idempotency key
	if req.IdempotencyKey != "" {
		existing := &models.Notification{}
		if err := h.db.First(existing, "idempotency_key = ?", req.IdempotencyKey).Error; err == nil {
			return c.JSON(http.StatusConflict, map[string]interface{}{
				"id":     existing.ID,
				"status": existing.Status,
				"message": "Notification already sent with this idempotency key",
			})
		}
	}

	// Create notification record
	notifID := uuid.New().String()
	notification := &models.Notification{
		ID:             notifID,
		AppID:          appID,
		Provider:       req.Provider,
		Status:         models.StatusPending,
		Recipient:      req.Recipient,
		Subject:        req.Subject,
		Content:        req.Content,
		DeliveryMode:   req.DeliveryMode,
		IdempotencyKey: req.IdempotencyKey,
	}

	// Save to database
	if err := h.db.CreateNotification(notification); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create notification"})
	}

	// Queue the message
	msg := &queue.Message{
		ID:           notifID,
		AppID:        appID,
		Provider:     req.Provider,
		Recipient:    req.Recipient,
		Subject:      req.Subject,
		Content:      req.Content,
		Metadata:     req.Metadata,
		Retries:      0,
		DeliveryMode: req.DeliveryMode,
	}

	if err := h.queue.Publish(c.Request().Context(), msg); err != nil {
		// Mark as failed if we can't queue it
		h.db.UpdateNotificationError(notifID, "Failed to queue message: "+err.Error(), 0)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to queue notification"})
	}

	return c.JSON(http.StatusAccepted, map[string]interface{}{
		"id":     notifID,
		"status": models.StatusPending,
	})
}

// GetNotificationStatus handles GET /notifications/:id
func (h *NotificationHandler) GetNotificationStatus(c echo.Context) error {
	id := c.Param("id")

	notification, err := h.db.GetNotification(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Notification not found"})
	}

	return c.JSON(http.StatusOK, notification)
}

// HealthHandler handles GET /health
func (h *NotificationHandler) HealthHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// hashAPIKey creates a hash of the API key for storage
func hashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}
