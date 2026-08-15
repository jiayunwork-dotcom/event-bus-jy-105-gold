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

CREATE INDEX idx_migrations_tenant_type ON schema_migrations(tenant_id, event_type);
CREATE INDEX idx_migrations_tenant_status ON schema_migrations(tenant_id, status);
CREATE INDEX idx_migrations_running ON schema_migrations(tenant_id, event_type, status) WHERE status IN ('running', 'rollback_running');

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

CREATE INDEX idx_backups_migration ON migration_event_backups(migration_id);
CREATE INDEX idx_backups_migration_status ON migration_event_backups(migration_id, status);
CREATE INDEX idx_backups_event ON migration_event_backups(event_id);
