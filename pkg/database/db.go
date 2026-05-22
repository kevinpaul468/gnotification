package database

import (
	"fmt"
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

// SaveProviderConfig stores provider configuration
func (db *DB) SaveProviderConfig(pc *models.ProviderConfig) error {
	return db.Save(pc).Error
}

// GetProviderConfig retrieves provider configuration
func (db *DB) GetProviderConfig(provider string) (*models.ProviderConfig, error) {
	var pc models.ProviderConfig
	if err := db.First(&pc, "provider = ? AND is_active = ?", provider, true).Error; err != nil {
		return nil, err
	}
	return &pc, nil
}

// GetActiveProviderConfigs retrieves all active provider configurations
func (db *DB) GetActiveProviderConfigs() ([]models.ProviderConfig, error) {
	var configs []models.ProviderConfig
	err := db.Where("is_active = ?", true).Find(&configs).Error
	return configs, err
}

// GetAPIKey retrieves and validates API key
func (db *DB) GetAPIKey(keyHash string) (*models.APIKey, error) {
	var key models.APIKey
	if err := db.First(&key, "key_hash = ?", keyHash).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
