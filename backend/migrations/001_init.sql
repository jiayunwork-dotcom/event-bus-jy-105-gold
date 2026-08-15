CREATE TABLE IF NOT EXISTS tenants (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    max_publish_qps INTEGER NOT NULL DEFAULT 1000,
    max_subscriptions INTEGER NOT NULL DEFAULT 100,
    max_storage_mb INTEGER NOT NULL DEFAULT 10240,
    retention_policy JSONB NOT NULL DEFAULT '{"type":"time","value_days":30}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS event_schemas (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(36) NOT NULL REFERENCES tenants(id),
    event_type VARCHAR(255) NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    schema_def JSONB NOT NULL,
    is_compatible BOOLEAN NOT NULL DEFAULT true,
    compatibility_note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, event_type, version)
);

CREATE INDEX idx_schemas_tenant_type ON event_schemas(tenant_id, event_type);
CREATE INDEX idx_schemas_tenant_type_version ON event_schemas(tenant_id, event_type, version DESC);

CREATE TABLE IF NOT EXISTS subscriptions (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(36) NOT NULL REFERENCES tenants(id),
    name VARCHAR(255) NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    filter_expression TEXT,
    delivery_mode VARCHAR(20) NOT NULL DEFAULT 'at_least_once',
    idempotent_key_path TEXT,
    idempotent_window_seconds INTEGER NOT NULL DEFAULT 86400,
    max_retries INTEGER NOT NULL DEFAULT 5,
    consumer_url TEXT NOT NULL,
    consumer_rate_limit INTEGER NOT NULL DEFAULT 100,
    consumer_burst INTEGER NOT NULL DEFAULT 200,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_subscriptions_tenant ON subscriptions(tenant_id);
CREATE INDEX idx_subscriptions_tenant_type ON subscriptions(tenant_id, event_type);

CREATE TABLE IF NOT EXISTS orchestration_dags (
    id VARCHAR(36) PRIMARY KEY,
    subscription_id VARCHAR(36) NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    nodes JSONB NOT NULL,
    edges JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS events (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(36) NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    schema_version INTEGER NOT NULL,
    payload JSONB NOT NULL,
    idempotent_key VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_events_tenant_type ON events(tenant_id, event_type);
CREATE INDEX idx_events_tenant_created ON events(tenant_id, created_at DESC);
CREATE INDEX idx_events_idempotent ON events(tenant_id, idempotent_key) WHERE idempotent_key IS NOT NULL;

CREATE TABLE IF NOT EXISTS deliveries (
    id VARCHAR(36) PRIMARY KEY,
    event_id VARCHAR(36) NOT NULL REFERENCES events(id),
    subscription_id VARCHAR(36) NOT NULL REFERENCES subscriptions(id),
    tenant_id VARCHAR(36) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    retry_count INTEGER NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMPTZ,
    idempotent_key VARCHAR(255),
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_deliveries_subscription_status ON deliveries(subscription_id, status);
CREATE INDEX idx_deliveries_tenant_status ON deliveries(tenant_id, status);
CREATE INDEX idx_deliveries_next_retry ON deliveries(next_retry_at) WHERE status = 'retrying';

CREATE TABLE IF NOT EXISTS dead_letter_queue (
    id VARCHAR(36) PRIMARY KEY,
    event_id VARCHAR(36) NOT NULL,
    subscription_id VARCHAR(36) NOT NULL REFERENCES subscriptions(id),
    tenant_id VARCHAR(36) NOT NULL,
    original_payload JSONB NOT NULL,
    failure_reason TEXT NOT NULL,
    retry_count INTEGER NOT NULL DEFAULT 0,
    last_retry_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_dlq_tenant ON dead_letter_queue(tenant_id);
CREATE INDEX idx_dlq_subscription ON dead_letter_queue(subscription_id);

CREATE TABLE IF NOT EXISTS delivery_traces (
    id VARCHAR(36) PRIMARY KEY,
    event_id VARCHAR(36) NOT NULL,
    subscription_id VARCHAR(36) NOT NULL,
    tenant_id VARCHAR(36) NOT NULL,
    node_id VARCHAR(255) NOT NULL,
    node_type VARCHAR(50) NOT NULL,
    node_name VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL,
    input_payload JSONB,
    output_payload JSONB,
    error_message TEXT,
    duration_ms INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_traces_event ON delivery_traces(event_id);
CREATE INDEX idx_traces_event_subscription ON delivery_traces(event_id, subscription_id);

CREATE TABLE IF NOT EXISTS backpressure_alerts (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(36) NOT NULL,
    subscription_id VARCHAR(36) NOT NULL REFERENCES subscriptions(id),
    alert_type VARCHAR(50) NOT NULL,
    message TEXT NOT NULL,
    is_resolved BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

CREATE INDEX idx_alerts_tenant ON backpressure_alerts(tenant_id);
CREATE INDEX idx_alerts_unresolved ON backpressure_alerts(is_resolved) WHERE is_resolved = false;
