package repository

import (
	"database/sql"
	"fmt"
	"log"
)

func RunMigrations(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations_history (
			version INTEGER PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("create migrations history table: %w", err)
	}

	migrations := []struct {
		version int
		name    string
		sql     string
	}{
		{
			version: 1,
			name:    "001_init.sql",
			sql: `-- 001_init.sql 已在首次启动时执行，此处仅记录`,
		},
		{
			version: 2,
			name:    "002_add_migration_tables.sql",
			sql: `
CREATE TABLE IF NOT EXISTS schema_migrations (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(36) NOT NULL REFERENCES tenants(id),
    event_type VARCHAR(255) NOT NULL,
    source_version INTEGER NOT NULL,
    target_version INTEGER NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    migration_rules JSONB NOT NULL,
    total_events INTEGER NOT NULL DEFAULT 0,
    processed_events INTEGER NOT NULL DEFAULT 0,
    failed_events INTEGER NOT NULL DEFAULT 0,
    error_message TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    rollback_deadline TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_migrations_tenant_type ON schema_migrations(tenant_id, event_type);
CREATE INDEX IF NOT EXISTS idx_migrations_tenant_status ON schema_migrations(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_migrations_running ON schema_migrations(tenant_id, event_type, status) WHERE status IN ('running', 'rollback_running');

CREATE TABLE IF NOT EXISTS migration_event_backups (
    id VARCHAR(36) PRIMARY KEY,
    migration_id VARCHAR(36) NOT NULL REFERENCES schema_migrations(id) ON DELETE CASCADE,
    event_id VARCHAR(36) NOT NULL,
    original_payload JSONB NOT NULL,
    original_schema_version INTEGER NOT NULL,
    new_payload JSONB,
    new_schema_version INTEGER,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_backups_migration ON migration_event_backups(migration_id);
CREATE INDEX IF NOT EXISTS idx_backups_migration_status ON migration_event_backups(migration_id, status);
CREATE INDEX IF NOT EXISTS idx_backups_event ON migration_event_backups(event_id);
			`,
		},
	}

	for _, m := range migrations {
		var exists bool
		err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM schema_migrations_history WHERE version = $1)", m.version).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check migration %d: %w", m.version, err)
		}

		if exists {
			log.Printf("Migration %d (%s) already applied, skipping", m.version, m.name)
			continue
		}

		log.Printf("Applying migration %d: %s", m.version, m.name)
		if _, err := db.Exec(m.sql); err != nil {
			return fmt.Errorf("apply migration %d: %w", m.version, err)
		}

		_, err = db.Exec(
			"INSERT INTO schema_migrations_history (version, name) VALUES ($1, $2)",
			m.version, m.name,
		)
		if err != nil {
			return fmt.Errorf("record migration %d: %w", m.version, err)
		}

		log.Printf("Migration %d applied successfully", m.version)
	}

	return nil
}
