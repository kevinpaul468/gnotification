package database

import (
	"encoding/json"
	"fmt"
	"time"
	"github.com/swecha/notifications/pkg/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type DB struct {
	*gorm.DB
}

// NewDB creates a new database connection
func NewDB(dsn string) (*DB, error) {
	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return &DB{gormDB}, nil
}

// Migrate runs database migrations
func (db *DB) Migrate() error {
	return db.AutoMigrate(
		&models.Notification{},
		&models.App{},
		&models.APIKey{},
		&models.ProviderConfig{},
	)
}

// GetNotification retrieves a notification by ID
func (db *DB) GetNotification(id string) (*models.Notification, error) {
	var notification models.Notification
	if err := db.First(&notification, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &notification, nil
}

// CreateNotification saves a new notification
func (db *DB) CreateNotification(n *models.Notification) error {
	return db.Create(n).Error
}

// UpdateNotificationStatus updates notification status
func (db *DB) UpdateNotificationStatus(id, status, providerRef string) error {
	return db.Model(&models.Notification{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":       status,
			"provider_ref": providerRef,
		}).Error
}

// UpdateNotificationError marks a notification as failed
func (db *DB) UpdateNotificationError(id, errorMsg string, retryCount int) error {
	return db.Model(&models.Notification{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":        models.StatusFailed,
			"error_message": errorMsg,
			"retry_count":   retryCount,
			"next_retry_at": nil,
		}).Error
}

// GetPendingNotifications retrieves notifications stuck in pending status
// that haven't been updated since the given cutoff time.
// These are candidates for queue reconciliation (re-publishing to RMQ).
func (db *DB) GetPendingNotifications(before time.Time) ([]models.Notification, error) {
	var notifications []models.Notification
	err := db.Where("status = ? AND updated_at < ?", models.StatusPending, before).
		Order("updated_at ASC").
		Find(&notifications).Error
	return notifications, err
}

// GetMergedProviderConfig returns the effective config for a provider for a given app.
// If appID is empty, returns the global config.
// If appID is provided, loads both global and app-specific configs and deep-merges
// them (app-specific fields override global).
func (db *DB) GetMergedProviderConfig(provider, appID string) (map[string]interface{}, error) {
	var globalConfig models.ProviderConfig
	err := db.Where("provider = ? AND app_id IS NULL AND is_active = ?", provider, true).
		First(&globalConfig).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	hasGlobal := err == nil

	if appID == "" {
		if hasGlobal {
			return parseConfigJSON(globalConfig.Config)
		}
		return nil, fmt.Errorf("no global config found for provider: %s", provider)
	}

	var appConfig models.ProviderConfig
	err = db.Where("provider = ? AND app_id = ? AND is_active = ?", provider, appID, true).
		First(&appConfig).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	hasApp := err == nil

	if !hasGlobal && !hasApp {
		return nil, fmt.Errorf("no config found for provider: %s (app: %s)", provider, appID)
	}

	if !hasGlobal {
		return parseConfigJSON(appConfig.Config)
	}

	globalMap, err := parseConfigJSON(globalConfig.Config)
	if err != nil {
		return nil, err
	}

	if !hasApp {
		return globalMap, nil
	}

	appMap, err := parseConfigJSON(appConfig.Config)
	if err != nil {
		return nil, err
	}

	return deepMergeMaps(globalMap, appMap), nil
}

// GetNotificationsByApp retrieves paginated notifications for an app.
func (db *DB) GetNotificationsByApp(appID string, limit, offset int) ([]models.Notification, error) {
	var notifications []models.Notification
	err := db.Where("app_id = ?", appID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&notifications).Error
	return notifications, err
}

// parseConfigJSON parses a JSON string into a map.
func parseConfigJSON(s string) (map[string]interface{}, error) {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, err
	}
	return m, nil
}

// deepMergeMaps merges src into dst. Nested maps are merged recursively.
// src values override dst values for the same key.
func deepMergeMaps(dst, src map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(dst))
	for k, v := range dst {
		result[k] = v
	}
	for k, v := range src {
		if dstMap, ok := result[k].(map[string]interface{}); ok {
			if srcMap, ok := v.(map[string]interface{}); ok {
				result[k] = deepMergeMaps(dstMap, srcMap)
				continue
			}
		}
		result[k] = v
	}
	return result
}

// SaveProviderConfig stores provider configuration
func (db *DB) SaveProviderConfig(pc *models.ProviderConfig) error {
	return db.Save(pc).Error
}

// GetProviderConfig retrieves the global provider configuration
func (db *DB) GetProviderConfig(provider string) (*models.ProviderConfig, error) {
	var pc models.ProviderConfig
	if err := db.First(&pc, "provider = ? AND app_id IS NULL AND is_active = ?", provider, true).Error; err != nil {
		return nil, err
	}
	return &pc, nil
}

// GetActiveProviderConfigs retrieves all active global provider configurations
func (db *DB) GetActiveProviderConfigs() ([]models.ProviderConfig, error) {
	var configs []models.ProviderConfig
	err := db.Where("app_id IS NULL AND is_active = ?", true).Find(&configs).Error
	return configs, err
}

// GetAppProviderConfigs retrieves all active provider configs for a specific app
func (db *DB) GetAppProviderConfigs(appID string) ([]models.ProviderConfig, error) {
	var configs []models.ProviderConfig
	err := db.Where("app_id = ? AND is_active = ?", appID, true).Find(&configs).Error
	return configs, err
}

// CreateApp creates a new app
func (db *DB) CreateApp(app *models.App) error {
	return db.Create(app).Error
}

// GetApp retrieves an app by ID
func (db *DB) GetApp(id string) (*models.App, error) {
	var app models.App
	if err := db.First(&app, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &app, nil
}

// ListApps returns all registered apps
func (db *DB) ListApps() ([]models.App, error) {
	var apps []models.App
	if err := db.Order("created_at DESC").Find(&apps).Error; err != nil {
		return nil, err
	}
	return apps, nil
}

// CreateAPIKey creates a new API key record
func (db *DB) CreateAPIKey(key *models.APIKey) error {
	return db.Create(key).Error
}

// DeleteAPIKey deletes an API key by ID
func (db *DB) DeleteAPIKey(id string) error {
	return db.Delete(&models.APIKey{}, "id = ?", id).Error
}

// GetAPIKey retrieves and validates API key
func (db *DB) GetAPIKey(keyHash string) (*models.APIKey, error) {
	var key models.APIKey
	if err := db.First(&key, "key_hash = ?", keyHash).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

// SaveAPIKey saves an API key (for admin)
func (db *DB) SaveAPIKey(key *models.APIKey) error {
	return db.Save(key).Error
}

// GetAllProviderConfigs retrieves all provider configs (active and inactive)
func (db *DB) GetAllProviderConfigs() ([]models.ProviderConfig, error) {
	var configs []models.ProviderConfig
	err := db.Find(&configs).Error
	return configs, err
}

// SaveAPIKeyRequest saves an API key request
func (db *DB) SaveAPIKeyRequest(req *models.APIKeyRequest) error {
	return db.Save(req).Error
}

// GetAPIKeyRequests retrieves API key requests with filters
func (db *DB) GetAPIKeyRequests(status string) ([]models.APIKeyRequest, error) {
	var requests []models.APIKeyRequest
	query := db.DB
	if status != "" {
		query = query.Where("status = ?", status)
	}
	err := query.Order("created_at DESC").Find(&requests).Error
	return requests, err
}

// GetAPIKeyRequest retrieves a single API key request
func (db *DB) GetAPIKeyRequest(id string) (*models.APIKeyRequest, error) {
	var req models.APIKeyRequest
	if err := db.First(&req, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &req, nil
}

// UpdateAPIKeyRequest updates API key request status
func (db *DB) UpdateAPIKeyRequest(id, status, adminComment, approvedBy string) error {
	updates := map[string]interface{}{
		"status":         status,
		"admin_comment":  adminComment,
		"updated_at":     time.Now(),
	}
	if status == "approved" {
		updates["approved_at"] = time.Now()
		updates["approved_by"] = approvedBy
	}
	return db.Model(&models.APIKeyRequest{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// Close closes the database connection
func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
