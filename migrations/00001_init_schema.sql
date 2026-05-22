-- +goose Up
-- +goose StatementBegin

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

CREATE TABLE IF NOT EXISTS provider_configs (
    id VARCHAR(36) PRIMARY KEY,
    provider VARCHAR(50) NOT NULL,
    config TEXT NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_provider_configs_provider ON provider_configs(provider);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_provider_configs_provider;
DROP TABLE IF EXISTS provider_configs;

DROP INDEX IF EXISTS idx_api_keys_app_id;
DROP TABLE IF EXISTS api_keys;

DROP INDEX IF EXISTS idx_notifications_created_at;
DROP INDEX IF EXISTS idx_notifications_idempotency_key;
DROP INDEX IF EXISTS idx_notifications_status;
DROP INDEX IF EXISTS idx_notifications_provider;
DROP INDEX IF EXISTS idx_notifications_app_id;
DROP TABLE IF EXISTS notifications;

-- +goose StatementEnd
