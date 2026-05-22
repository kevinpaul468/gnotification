# Database Migrations Guide

We use **Goose** for database migrations (similar to Alembic in Python).

## Overview

- **ORM**: GORM (like SQLAlchemy for Go)
- **Migration Tool**: Goose (like Alembic for Python)
- **Migrations Location**: `migrations/` directory
- **Migration Format**: SQL with Goose comments

## How It Works

```
GORM Models (pkg/models/)
    ↓
Migration Generator (cmd/migrate/main.go)
    ↓
Migration Files (migrations/*.sql)
    ↓
Goose (runs migrations)
    ↓
PostgreSQL Database
```

## Available Commands

### Generate Migration from Models

```bash
# Generate a new migration file from GORM models
go run ./cmd/migrate/main.go -command=generate -name="add_new_table"
```

This automatically creates a migration file with:
- Timestamp-based filename
- UP and DOWN SQL
- Proper rollback code

### Create Blank Migration

```bash
# Create a blank migration template
go run ./cmd/migrate/main.go -command=create -name="custom_migration"
```

Then edit the file to add your custom SQL.

### Run Migrations

```bash
# Apply all pending migrations (UP)
go run ./cmd/migrate/main.go -command=up

# Rollback one migration (DOWN)
go run ./cmd/migrate/main.go -command=down

# Check migration status
go run ./cmd/migrate/main.go -command=status

# Rollback all, then apply all (useful for dev)
go run ./cmd/migrate/main.go -command=reset
```

### Seed Database

```bash
# Add example data for development
go run ./cmd/migrate/main.go -command=seed
```

### Auto-Migrate (Development Only)

```bash
# Use GORM's AutoMigrate (dev only, not recommended for production)
go run ./cmd/migrate/main.go -command=auto
```

## Migration File Format

Each migration file uses Goose format with UP and DOWN:

```sql
-- +goose Up
-- +goose StatementBegin

CREATE TABLE users (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_name ON users(name);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX idx_users_name;
DROP TABLE users;

-- +goose StatementEnd
```

## Current Schema

Located in `migrations/00001_init_schema.sql`

Tables created:
1. **notifications** - Main notification records
2. **failed_notifications** - Dead letter queue entries
3. **api_keys** - API authentication (future use)
4. **provider_configs** - Provider configurations

All tables auto-indexed for performance.

## Adding a New Migration

### Step 1: Add Model

Edit `pkg/models/notification.go`:

```go
type MyNewTable struct {
    ID       string
    Name     string
    CreatedAt time.Time
}

func (MyNewTable) TableName() string {
    return "my_new_table"
}
```

### Step 2: Generate Migration

```bash
go run ./cmd/migrate/main.go -command=generate -name="add_my_new_table"
```

This creates `migrations/TIMESTAMP_add_my_new_table.sql` with UP/DOWN SQL.

### Step 3: Run Migration

```bash
go run ./cmd/migrate/main.go -command=up
```

## GORM vs Alembic

| Feature | GORM | SQLAlchemy + Alembic |
|---------|------|---------------------|
| **ORM** | GORM structs | SQLAlchemy models |
| **Migrations** | SQL files with Goose | Python files with Alembic |
| **Schema Definition** | Go structs | Python classes |
| **Type Safety** | ✅ Compile-time | ⚠️ Runtime |
| **Performance** | ✅ Faster | ⚠️ Slower |
| **Learning Curve** | ✅ Simpler | ⚠️ Complex |

## Production Workflow

1. **Make schema change** in `pkg/models/`
2. **Generate migration**: `go run ./cmd/migrate -command=generate -name="..."`
3. **Review** the generated SQL
4. **Test** locally: `go run ./cmd/migrate -command=reset`
5. **Commit** migration file to git
6. **Deploy** to production, run: `go run ./cmd/migrate -command=up`

## Migration Tracking

Goose automatically tracks migrations in the `goose_db_version` table:

```sql
SELECT * FROM goose_db_version;
```

Shows:
- Migration ID
- Version (timestamp)
- Is Applied (true/false)
- Tstamp (when applied)

## Troubleshooting

### Migration won't apply

```bash
# Check which migrations are pending
go run ./cmd/migrate -command=status

# Check goose table
psql -U postgres notifications -c "SELECT * FROM goose_db_version;"
```

### Want to change a migration

**Never edit applied migrations!** Instead:

1. Create a new migration to fix the issue
2. Run the new migration

### Rollback all migrations

```bash
go run ./cmd/migrate -command=reset
```

This will:
1. Rollback all applied migrations (DOWN)
2. Re-apply all migrations (UP)

## Files Reference

| File | Purpose |
|------|---------|
| `cmd/migrate/main.go` | Migration CLI tool |
| `migrations/*.sql` | Migration files (Goose format) |
| `pkg/models/notification.go` | GORM models definition |
| `pkg/database/db.go` | Database connection & setup |

## Related Documentation

- [GORM Documentation](https://gorm.io/)
- [Goose Documentation](https://github.com/pressly/goose)
- [PostgreSQL Documentation](https://www.postgresql.org/docs/)
