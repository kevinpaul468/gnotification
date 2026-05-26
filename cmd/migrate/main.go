package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/swecha/notifications/pkg/database"
	"github.com/swecha/notifications/pkg/models"
)

func main() {
	godotenv.Load()

	command := flag.String("command", "status", "migration command: create, up, down, status, reset, generate, seed, auto")
	name := flag.String("name", "", "migration name (for create/generate command)")
	flag.Parse()

	dbDSN := os.Getenv("DATABASE_URL")
	if dbDSN == "" {
		log.Fatal("DATABASE_URL env var not set")
	}

	db, err := sql.Open("pgx", dbDSN)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("failed to set dialect: %v", err)
	}

	switch *command {
	case "generate":
		if *name == "" {
			log.Fatal("migration name required for generate command")
		}
		if err := generateMigration(*name); err != nil {
			log.Fatalf("failed to generate migration: %v", err)
		}
		fmt.Printf("✓ Generated migration: %s\n", *name)

	case "create":
		if *name == "" {
			log.Fatal("migration name required for create command")
		}
		if err := createMigration(*name); err != nil {
			log.Fatalf("failed to create migration: %v", err)
		}
		fmt.Printf("✓ Created blank migration: %s\n", *name)

	case "up":
		if err := goose.Up(db, "migrations"); err != nil {
			log.Fatalf("failed to migrate up: %v", err)
		}
		fmt.Println("✓ Migration up completed")

	case "down":
		if err := goose.Down(db, "migrations"); err != nil {
			log.Fatalf("failed to migrate down: %v", err)
		}
		fmt.Println("✓ Migration down completed")

	case "status":
		if err := goose.Status(db, "migrations"); err != nil {
			log.Fatalf("failed to get migration status: %v", err)
		}

	case "reset":
		// Rollback all migrations
		for {
			current, err := goose.GetDBVersion(db)
			if err != nil {
				log.Fatalf("failed to get current version: %v", err)
			}
			if current == 0 {
				break
			}
			if err := goose.Down(db, "migrations"); err != nil {
				log.Fatalf("failed to migrate down: %v", err)
			}
		}
		// Then apply all migrations
		if err := goose.Up(db, "migrations"); err != nil {
			log.Fatalf("failed to migrate up: %v", err)
		}
		fmt.Println("✓ Database reset completed")

	case "seed":
		// Seed with example data
		gormDB, err := database.NewDB(dbDSN)
		if err != nil {
			log.Fatalf("failed to connect to database: %v", err)
		}
		seedDatabase(gormDB)
		fmt.Println("✓ Database seeded with example data")

	case "auto":
		// Auto-migrate using GORM (use this during development)
		gormDB, err := database.NewDB(dbDSN)
		if err != nil {
			log.Fatalf("failed to connect to database: %v", err)
		}
		if err := gormDB.AutoMigrate(
			&models.Notification{},
			&models.App{},
			&models.APIKey{},
			&models.ProviderConfig{},
		); err != nil {
			log.Fatalf("failed to auto-migrate: %v", err)
		}
		fmt.Println("✓ Auto-migration completed")

	default:
		log.Fatalf("unknown command: %s", *command)
	}
}

// generateMigration creates migration SQL from GORM models
func generateMigration(name string) error {
	timestamp := time.Now().Unix()
	filename := fmt.Sprintf("migrations/%d_%s.sql", timestamp, name)

	// Get the DDL for all tables
	upSQL := generateUpSQL()
	downSQL := generateDownSQL()

	content := fmt.Sprintf(`-- +goose Up
-- +goose StatementBegin

%s

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

%s

-- +goose StatementEnd
`, upSQL, downSQL)

	return os.WriteFile(filename, []byte(content), 0644)
}

// generateUpSQL creates the UP migration SQL
func generateUpSQL() string {
	return `
CREATE TABLE IF NOT EXISTS notifications (
    id VARCHAR(36) PRIMARY KEY,
    app_id VARCHAR(255) NOT NULL,
    provider VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    recipient TEXT NOT NULL,
    subject TEXT,
    content TEXT NOT NULL,
    delivery_mode VARCHAR(20),
    provider_ref VARCHAR(255),
    error_message TEXT,
    retry_count INTEGER DEFAULT 0,
    last_retry_at TIMESTAMP NULL,
    next_retry_at TIMESTAMP NULL,
    idempotency_key VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    delivered_at TIMESTAMP NULL
);

CREATE INDEX IF NOT EXISTS idx_notifications_app_id ON notifications(app_id);
CREATE INDEX IF NOT EXISTS idx_notifications_provider ON notifications(provider);
CREATE INDEX IF NOT EXISTS idx_notifications_status ON notifications(status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_notifications_idempotency_key ON notifications(idempotency_key);
CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notifications(created_at);

CREATE TABLE IF NOT EXISTS api_keys (
    id VARCHAR(36) PRIMARY KEY,
    app_id VARCHAR(255) UNIQUE NOT NULL,
    key_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMP NULL
);

CREATE INDEX IF NOT EXISTS idx_api_keys_app_id ON api_keys(app_id);

CREATE TABLE IF NOT EXISTS apps (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    description TEXT,
    allowed_providers TEXT NOT NULL DEFAULT '[]',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_apps_name ON apps(name);

CREATE TABLE IF NOT EXISTS provider_configs (
    id VARCHAR(36) PRIMARY KEY,
    provider VARCHAR(50) NOT NULL,
    app_id VARCHAR(36) NULL,
    config TEXT NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_provider_configs_provider ON provider_configs(provider);
CREATE INDEX IF NOT EXISTS idx_provider_configs_provider_app ON provider_configs(provider, app_id);
`
}

// generateDownSQL creates the DOWN migration SQL
func generateDownSQL() string {
	return `
DROP INDEX IF EXISTS idx_provider_configs_provider_app;
DROP INDEX IF EXISTS idx_provider_configs_provider;
DROP TABLE IF EXISTS provider_configs;

DROP INDEX IF EXISTS idx_apps_name;
DROP TABLE IF EXISTS apps;

DROP INDEX IF EXISTS idx_api_keys_app_id;
DROP TABLE IF EXISTS api_keys;

DROP INDEX IF EXISTS idx_notifications_created_at;
DROP INDEX IF EXISTS idx_notifications_idempotency_key;
DROP INDEX IF EXISTS idx_notifications_status;
DROP INDEX IF EXISTS idx_notifications_provider;
DROP INDEX IF EXISTS idx_notifications_app_id;
DROP TABLE IF EXISTS notifications;
`
}

// createMigration creates a blank migration template
func createMigration(name string) error {
	timestamp := time.Now().Unix()
	filename := fmt.Sprintf("migrations/%d_%s.sql", timestamp, name)

	content := fmt.Sprintf(`-- +goose Up
-- +goose StatementBegin
-- Add your SQL migration here
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Add your rollback SQL here
-- +goose StatementEnd
`)

	return os.WriteFile(filename, []byte(content), 0644)
}

// seedDatabase adds example data for development
func seedDatabase(db *database.DB) {
	configJSON := `{"host":"smtp.gmail.com","port":587,"username":"example@gmail.com","password":"app-password","from":"noreply@example.com"}`

	pc := &models.ProviderConfig{
		ID:       "cfg-smtp-example",
		Provider: "smtp",
		Config:   configJSON,
		IsActive: true,
	}

	if err := db.SaveProviderConfig(pc); err != nil {
		fmt.Printf("Warning: failed to seed provider config: %v\n", err)
	}

	fmt.Println("✓ Example SMTP provider config created (cfg-smtp-example)")
}
