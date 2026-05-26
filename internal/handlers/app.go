package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/swecha/notifications/pkg/database"
	"github.com/swecha/notifications/pkg/models"
)

type AppHandler struct {
	db *database.DB
}

func NewAppHandler(db *database.DB) *AppHandler {
	return &AppHandler{db: db}
}

type CreateAppRequest struct {
	Name        string   `json:"name" validate:"required"`
	Description string   `json:"description"`
	Providers   []string `json:"providers"`
}

type CreateAppResponse struct {
	App    models.App `json:"app"`
	APIKey string     `json:"api_key"`
}

func (h *AppHandler) CreateApp(c echo.Context) error {
	var req CreateAppRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Name is required"})
	}

	if req.Providers == nil {
		req.Providers = []string{}
	}

	providersJSON, err := json.Marshal(req.Providers)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to marshal providers"})
	}

	app := &models.App{
		ID:               uuid.New().String(),
		Name:             req.Name,
		Description:      req.Description,
		AllowedProviders: string(providersJSON),
	}

	if err := h.db.CreateApp(app); err != nil {
		return c.JSON(http.StatusConflict, map[string]string{"error": "App name already exists"})
	}

	rawKey := generateAPIKey()
	keyHash := sha256.Sum256([]byte(rawKey))
	apiKey := &models.APIKey{
		ID:      uuid.New().String(),
		AppID:   app.ID,
		KeyHash: hex.EncodeToString(keyHash[:]),
	}

	if err := h.db.CreateAPIKey(apiKey); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create API key"})
	}

	return c.JSON(http.StatusCreated, CreateAppResponse{
		App:    *app,
		APIKey: rawKey,
	})
}

func (h *AppHandler) ListApps(c echo.Context) error {
	apps, err := h.db.ListApps()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to list apps"})
	}
	return c.JSON(http.StatusOK, apps)
}

func (h *AppHandler) GetApp(c echo.Context) error {
	id := c.Param("id")
	app, err := h.db.GetApp(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "App not found"})
	}
	return c.JSON(http.StatusOK, app)
}

func (h *AppHandler) CreateAPIKey(c echo.Context) error {
	appID := c.Param("id")

	if _, err := h.db.GetApp(appID); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "App not found"})
	}

	rawKey := generateAPIKey()
	keyHash := sha256.Sum256([]byte(rawKey))
	apiKey := &models.APIKey{
		ID:      uuid.New().String(),
		AppID:   appID,
		KeyHash: hex.EncodeToString(keyHash[:]),
	}

	if err := h.db.CreateAPIKey(apiKey); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create API key"})
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"id":      apiKey.ID,
		"app_id":  apiKey.AppID,
		"api_key": rawKey,
	})
}

func (h *AppHandler) DeleteAPIKey(c echo.Context) error {
	id := c.Param("id")
	if err := h.db.DeleteAPIKey(id); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "API key not found"})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *AppHandler) GetUsage(c echo.Context) error {
	appID := c.Param("id")

	limitStr := c.QueryParam("limit")
	offsetStr := c.QueryParam("offset")

	limit := 50
	offset := 0

	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	if offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
			offset = v
		}
	}

	notifications, err := h.db.GetNotificationsByApp(appID, limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch usage"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"notifications": notifications,
		"limit":         limit,
		"offset":        offset,
	})
}
