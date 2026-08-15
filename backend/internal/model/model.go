package model

import "time"

type Tenant struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Status           string                 `json:"status"`
	MaxPublishQPS    int                    `json:"max_publish_qps"`
	MaxSubscriptions int                    `json:"max_subscriptions"`
	MaxStorageMB     int                    `json:"max_storage_mb"`
	RetentionPolicy  RetentionPolicy        `json:"retention_policy"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

type RetentionPolicy struct {
	Type       string `json:"type"`
	ValueDays  int    `json:"value_days,omitempty"`
	ValueMB    int    `json:"value_mb,omitempty"`
}

type EventSchema struct {
	ID                string    `json:"id"`
	TenantID          string    `json:"tenant_id"`
	EventType         string    `json:"event_type"`
	Version           int       `json:"version"`
	SchemaDef         string    `json:"schema_def"`
	IsCompatible      bool      `json:"is_compatible"`
	CompatibilityNote string   `json:"compatibility_note,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

type Subscription struct {
	ID                      string    `json:"id"`
	TenantID                string    `json:"tenant_id"`
	Name                    string    `json:"name"`
	EventType               string    `json:"event_type"`
	FilterExpression        string    `json:"filter_expression,omitempty"`
	DeliveryMode            string    `json:"delivery_mode"`
	IdempotentKeyPath       string    `json:"idempotent_key_path,omitempty"`
	IdempotentWindowSeconds int       `json:"idempotent_window_seconds"`
	MaxRetries              int       `json:"max_retries"`
	ConsumerURL             string    `json:"consumer_url"`
	ConsumerRateLimit       int       `json:"consumer_rate_limit"`
	ConsumerBurst           int       `json:"consumer_burst"`
	Status                  string    `json:"status"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

type OrchestrationDAG struct {
	ID             string    `json:"id"`
	SubscriptionID string    `json:"subscription_id"`
	Nodes          []DAGNode `json:"nodes"`
	Edges          []DAGEdge `json:"edges"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type DAGNode struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Name     string                 `json:"name"`
	Config   map[string]interface{} `json:"config,omitempty"`
	Position map[string]float64     `json:"position,omitempty"`
}

type DAGEdge struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Condition string `json:"condition,omitempty"`
}

type Event struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	EventType     string    `json:"event_type"`
	SchemaVersion int       `json:"schema_version"`
	Payload       string    `json:"payload"`
	IdempotentKey string    `json:"idempotent_key,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type Delivery struct {
	ID             string     `json:"id"`
	EventID        string     `json:"event_id"`
	SubscriptionID string     `json:"subscription_id"`
	TenantID       string     `json:"tenant_id"`
	Status         string     `json:"status"`
	RetryCount     int        `json:"retry_count"`
	NextRetryAt    *time.Time `json:"next_retry_at,omitempty"`
	IdempotentKey  string     `json:"idempotent_key,omitempty"`
	LastError      string     `json:"last_error,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type DeadLetterEntry struct {
	ID              string     `json:"id"`
	EventID         string     `json:"event_id"`
	SubscriptionID  string     `json:"subscription_id"`
	TenantID        string     `json:"tenant_id"`
	OriginalPayload string     `json:"original_payload"`
	FailureReason   string     `json:"failure_reason"`
	RetryCount      int        `json:"retry_count"`
	LastRetryAt     *time.Time `json:"last_retry_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type DeliveryTrace struct {
	ID             string    `json:"id"`
	EventID        string    `json:"event_id"`
	SubscriptionID string    `json:"subscription_id"`
	TenantID       string    `json:"tenant_id"`
	NodeID         string    `json:"node_id"`
	NodeType       string    `json:"node_type"`
	NodeName       string    `json:"node_name"`
	Status         string    `json:"status"`
	InputPayload   string    `json:"input_payload,omitempty"`
	OutputPayload  string    `json:"output_payload,omitempty"`
	ErrorMessage   string    `json:"error_message,omitempty"`
	DurationMs     int       `json:"duration_ms"`
	CreatedAt      time.Time `json:"created_at"`
}

type BackpressureAlert struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	SubscriptionID string     `json:"subscription_id"`
	AlertType      string     `json:"alert_type"`
	Message        string     `json:"message"`
	IsResolved     bool       `json:"is_resolved"`
	CreatedAt      time.Time  `json:"created_at"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
}

type PublishRequest struct {
	Events []PublishEvent `json:"events" validate:"required,max=500"`
}

type PublishEvent struct {
	EventType     string                 `json:"event_type" validate:"required"`
	Payload       map[string]interface{} `json:"payload" validate:"required"`
	IdempotentKey string                 `json:"idempotent_key,omitempty"`
}

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type PublishResponse struct {
	Success bool              `json:"success"`
	EventIDs []string         `json:"event_ids,omitempty"`
	Errors   []ValidationError `json:"errors,omitempty"`
}

type ReplayRequest struct {
	SubscriptionID string `json:"subscription_id" validate:"required"`
	StartTime      *string `json:"start_time,omitempty"`
	EndTime        *string `json:"end_time,omitempty"`
	Offset         int64  `json:"offset,omitempty"`
	Rate           string `json:"rate" validate:"required,oneof=1x 5x 10x"`
}

type MigrationStatus string

const (
	MigrationStatusDraft         MigrationStatus = "draft"
	MigrationStatusRunning       MigrationStatus = "running"
	MigrationStatusCompleted     MigrationStatus = "completed"
	MigrationStatusFailed        MigrationStatus = "failed"
	MigrationStatusCancelled     MigrationStatus = "cancelled"
	MigrationStatusRollbackRunning MigrationStatus = "rollback_running"
	MigrationStatusRollbacked    MigrationStatus = "rollbacked"
	MigrationStatusRollbackFailed MigrationStatus = "rollback_failed"
)

type MigrationRuleType string

const (
	RuleTypeRename    MigrationRuleType = "rename"
	RuleTypeDelete    MigrationRuleType = "delete"
	RuleTypeAdd       MigrationRuleType = "add"
	RuleTypeConvert   MigrationRuleType = "convert"
	RuleTypeMapPath   MigrationRuleType = "map_path"
)

type MigrationRule struct {
	Type          MigrationRuleType `json:"type"`
	SourcePath    string            `json:"source_path,omitempty"`
	TargetPath    string            `json:"target_path,omitempty"`
	DefaultValue  interface{}       `json:"default_value,omitempty"`
	SourceType    string            `json:"source_type,omitempty"`
	TargetType    string            `json:"target_type,omitempty"`
}

type SchemaMigration struct {
	ID             string            `json:"id"`
	TenantID       string            `json:"tenant_id"`
	EventType      string            `json:"event_type"`
	SourceVersion  int               `json:"source_version"`
	TargetVersion  int               `json:"target_version"`
	Status         MigrationStatus   `json:"status"`
	MigrationRules string            `json:"migration_rules"`
	TotalEvents    int               `json:"total_events"`
	ProcessedEvents int              `json:"processed_events"`
	FailedEvents   int               `json:"failed_events"`
	ErrorMessage   string            `json:"error_message,omitempty"`
	StartedAt      *time.Time        `json:"started_at,omitempty"`
	CompletedAt    *time.Time        `json:"completed_at,omitempty"`
	RollbackDeadline *time.Time      `json:"rollback_deadline,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

type MigrationEventBackup struct {
	ID                   string    `json:"id"`
	MigrationID          string    `json:"migration_id"`
	EventID              string    `json:"event_id"`
	OriginalPayload      string    `json:"original_payload"`
	OriginalSchemaVersion int      `json:"original_schema_version"`
	NewPayload           string    `json:"new_payload,omitempty"`
	NewSchemaVersion     int       `json:"new_schema_version,omitempty"`
	Status               string    `json:"status"`
	ErrorMessage         string    `json:"error_message,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type MigrationPreviewRequest struct {
	EventType      string                 `json:"event_type" validate:"required"`
	SourceVersion  int                    `json:"source_version" validate:"required,min=1"`
	TargetVersion  int                    `json:"target_version" validate:"required,min=1"`
	MigrationRules []MigrationRule        `json:"migration_rules" validate:"required"`
}

type MigrationPreviewResult struct {
	EventID          string                 `json:"event_id"`
	OriginalPayload  map[string]interface{} `json:"original_payload"`
	ConvertedPayload map[string]interface{} `json:"converted_payload"`
	Success          bool                   `json:"success"`
	ErrorMessage     string                 `json:"error_message,omitempty"`
}

type MigrationStartRequest struct {
	EventType      string          `json:"event_type" validate:"required"`
	SourceVersion  int             `json:"source_version" validate:"required,min=1"`
	TargetVersion  int             `json:"target_version" validate:"required,min=1"`
	MigrationRules []MigrationRule `json:"migration_rules" validate:"required"`
}

type MigrationProgress struct {
	MigrationID       string  `json:"migration_id"`
	Status            string  `json:"status"`
	TotalEvents       int     `json:"total_events"`
	ProcessedEvents   int     `json:"processed_events"`
	FailedEvents      int     `json:"failed_events"`
	ProgressPercent   float64 `json:"progress_percent"`
	EstimatedRemainingSeconds float64 `json:"estimated_remaining_seconds,omitempty"`
	ErrorMessage      string  `json:"error_message,omitempty"`
}

type MigrationCancelRequest struct {
	MigrationID string `json:"migration_id" validate:"required"`
}

type MigrationRollbackRequest struct {
	MigrationID string `json:"migration_id" validate:"required"`
}

type MigrationRuleValidationError struct {
	RuleIndex int    `json:"rule_index"`
	Field     string `json:"field"`
	Message   string `json:"message"`
}

type AffectedSubscription struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	EventType        string `json:"event_type"`
	FilterExpression string `json:"filter_expression"`
	MatchedFields    []string `json:"matched_fields"`
}

type AffectedDAGNode struct {
	SubscriptionID   string                 `json:"subscription_id"`
	SubscriptionName string                 `json:"subscription_name"`
	DAGID            string                 `json:"dag_id"`
	NodeID           string                 `json:"node_id"`
	NodeName         string                 `json:"node_name"`
	NodeType         string                 `json:"node_type"`
	Config           map[string]interface{} `json:"config"`
	MatchedFields    []string               `json:"matched_fields"`
}

type MigrationImpactReport struct {
	MigrationID         string                `json:"migration_id"`
	AffectedFields      []string              `json:"affected_fields"`
	AffectedSubscriptions []AffectedSubscription `json:"affected_subscriptions"`
	AffectedDAGNodes    []AffectedDAGNode     `json:"affected_dag_nodes"`
	Suggestions         []string              `json:"suggestions"`
	HasImpact           bool                  `json:"has_impact"`
}
