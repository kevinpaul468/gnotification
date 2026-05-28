package database

import (
	"os"
	"testing"

	"github.com/swecha/notifications/pkg/models"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	db, err := NewDB(dsn)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.Migrate(); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	return db
}

func seedProviderConfig(t *testing.T, db *DB, pc *models.ProviderConfig) {
	t.Helper()
	if err := db.SaveProviderConfig(pc); err != nil {
		t.Fatalf("failed to seed provider config: %v", err)
	}
}

func TestGetMergedProviderConfig_GlobalOnly(t *testing.T) {
	db := testDB(t)

	seedProviderConfig(t, db, &models.ProviderConfig{
		ID:       "global-smtp",
		Provider: "smtp",
		Config:   `{"host":"smtp.example.com","port":587}`,
		IsActive: true,
	})

	got, err := db.GetMergedProviderConfig("smtp", "")
	if err != nil {
		t.Fatalf("GetMergedProviderConfig() error = %v", err)
	}
	if got["host"] != "smtp.example.com" {
		t.Errorf("expected host smtp.example.com, got %v", got["host"])
	}
	if got["port"] != float64(587) {
		t.Errorf("expected port 587, got %v", got["port"])
	}
}

func TestGetMergedProviderConfig_PerAppOverridesGlobal(t *testing.T) {
	db := testDB(t)

	seedProviderConfig(t, db, &models.ProviderConfig{
		ID:       "global-smtp",
		Provider: "smtp",
		Config:   `{"host":"smtp.example.com","port":587,"from":"default@example.com"}`,
		IsActive: true,
	})
	appID := "app-test"
	seedProviderConfig(t, db, &models.ProviderConfig{
		ID:       "app-smtp",
		Provider: "smtp",
		AppID:    &appID,
		Config:   `{"from":"app@example.com"}`,
		IsActive: true,
	})

	got, err := db.GetMergedProviderConfig("smtp", appID)
	if err != nil {
		t.Fatalf("GetMergedProviderConfig() error = %v", err)
	}
	if got["from"] != "app@example.com" {
		t.Errorf("expected from app@example.com (overridden), got %v", got["from"])
	}
	if got["host"] != "smtp.example.com" {
		t.Errorf("expected host smtp.example.com (inherited), got %v", got["host"])
	}
	if got["port"] != float64(587) {
		t.Errorf("expected port 587 (inherited), got %v", got["port"])
	}
}

func TestGetNotificationsByApp_Empty(t *testing.T) {
	db := testDB(t)

	got, err := db.GetNotificationsByApp("nonexistent-app", 10, 0)
	if err != nil {
		t.Fatalf("GetNotificationsByApp() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty list, got %d items", len(got))
	}
}

func TestGetProviderConfig_AppSpecific(t *testing.T) {
	db := testDB(t)

	seedProviderConfig(t, db, &models.ProviderConfig{
		ID:       "global-smtp",
		Provider: "smtp",
		Config:   `{"host":"smtp.example.com"}`,
		IsActive: true,
	})
	appID := "app-specific"
	seedProviderConfig(t, db, &models.ProviderConfig{
		ID:       "app-smtp",
		Provider: "smtp",
		AppID:    &appID,
		Config:   `{"host":"app.smtp.com"}`,
		IsActive: true,
	})

	got, err := db.GetProviderConfig("smtp", appID)
	if err != nil {
		t.Fatalf("GetProviderConfig() error = %v", err)
	}
	if got.AppID == nil || *got.AppID != appID {
		t.Errorf("expected app-specific config, got app_id=%v", got.AppID)
	}
}

func TestGetProviderConfig_FallbackToGlobal(t *testing.T) {
	db := testDB(t)

	seedProviderConfig(t, db, &models.ProviderConfig{
		ID:       "global-smtp",
		Provider: "smtp",
		Config:   `{"host":"smtp.example.com"}`,
		IsActive: true,
	})

	got, err := db.GetProviderConfig("smtp", "nonexistent-app")
	if err != nil {
		t.Fatalf("GetProviderConfig() error = %v", err)
	}
	if got.AppID != nil {
		t.Errorf("expected global config (nil app_id), got app_id=%v", *got.AppID)
	}
}
