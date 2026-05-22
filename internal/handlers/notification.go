package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"

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

	// Extract API key and app ID from header (auth middleware should do this)
	appIDRaw := c.Get("app_id")
	appID, ok := appIDRaw.(string)
	if !ok || appID == "" {
		// Default to "unknown" if no app_id is set (for development/testing)
		appID = "unknown"
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
