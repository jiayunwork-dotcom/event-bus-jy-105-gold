package repository

import (
	"database/sql"
	"encoding/json"

	"github.com/eventbus/server/internal/model"
	"github.com/google/uuid"
)

type DeliveryRepo struct {
	db *sql.DB
}

func NewDeliveryRepo(db *sql.DB) *DeliveryRepo {
	return &DeliveryRepo{db: db}
}

func (r *DeliveryRepo) Create(d *model.Delivery) error {
	d.ID = uuid.New().String()
	row := r.db.QueryRow(
		`INSERT INTO deliveries (id, event_id, subscription_id, tenant_id, status, retry_count, next_retry_at, idempotent_key, last_error)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING created_at, updated_at`,
		d.ID, d.EventID, d.SubscriptionID, d.TenantID, d.Status, d.RetryCount, d.NextRetryAt, nullString(d.IdempotentKey), nullString(d.LastError),
	)
	return row.Scan(&d.CreatedAt, &d.UpdatedAt)
}

func (r *DeliveryRepo) CreateBatch(deliveries []*model.Delivery) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT INTO deliveries (id, event_id, subscription_id, tenant_id, status, retry_count, next_retry_at, idempotent_key)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, d := range deliveries {
		d.ID = uuid.New().String()
		_, err := stmt.Exec(d.ID, d.EventID, d.SubscriptionID, d.TenantID, d.Status, d.RetryCount, d.NextRetryAt, nullString(d.IdempotentKey))
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *DeliveryRepo) UpdateStatus(id string, status string, lastError string) error {
	_, err := r.db.Exec(
		`UPDATE deliveries SET status=$2, last_error=$3, updated_at=NOW() WHERE id=$1`,
		id, status, nullString(lastError),
	)
	return err
}

func (r *DeliveryRepo) IncrementRetry(id string, nextRetryAt interface{}, lastError string) error {
	_, err := r.db.Exec(
		`UPDATE deliveries SET retry_count=retry_count+1, status='retrying', next_retry_at=$2, last_error=$3, updated_at=NOW() WHERE id=$1`,
		id, nextRetryAt, nullString(lastError),
	)
	return err
}

func (r *DeliveryRepo) GetPendingRetries(limit int) ([]*model.Delivery, error) {
	rows, err := r.db.Query(
		`SELECT id, event_id, subscription_id, tenant_id, status, retry_count, next_retry_at, idempotent_key, last_error, created_at, updated_at
		 FROM deliveries WHERE status='retrying' AND next_retry_at <= NOW() ORDER BY next_retry_at ASC LIMIT $1`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDeliveries(rows)
}

func (r *DeliveryRepo) GetByEventID(eventID string) ([]*model.Delivery, error) {
	rows, err := r.db.Query(
		`SELECT id, event_id, subscription_id, tenant_id, status, retry_count, next_retry_at, idempotent_key, last_error, created_at, updated_at
		 FROM deliveries WHERE event_id=$1 ORDER BY created_at`, eventID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDeliveries(rows)
}

func (r *DeliveryRepo) MoveToDeadLetter(d *model.Delivery, failureReason string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var event model.Event
	err = tx.QueryRow(
		`SELECT payload FROM events WHERE id=$1`, d.EventID,
	).Scan(&event.Payload)
	if err != nil {
		return err
	}

	dlqID := uuid.New().String()
	_, err = tx.Exec(
		`INSERT INTO dead_letter_queue (id, event_id, subscription_id, tenant_id, original_payload, failure_reason, retry_count)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		dlqID, d.EventID, d.SubscriptionID, d.TenantID, event.Payload, failureReason, d.RetryCount,
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec(
		`UPDATE deliveries SET status='failed', last_error=$2, updated_at=NOW() WHERE id=$1`,
		d.ID, failureReason,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func scanDeliveries(rows *sql.Rows) ([]*model.Delivery, error) {
	var deliveries []*model.Delivery
	for rows.Next() {
		var d model.Delivery
		var nextRetryAt sql.NullTime
		var idempotentKey, lastError sql.NullString
		err := rows.Scan(&d.ID, &d.EventID, &d.SubscriptionID, &d.TenantID, &d.Status, &d.RetryCount,
			&nextRetryAt, &idempotentKey, &lastError, &d.CreatedAt, &d.UpdatedAt)
		if err != nil {
			return nil, err
		}
		if nextRetryAt.Valid {
			d.NextRetryAt = &nextRetryAt.Time
		}
		d.IdempotentKey = idempotentKey.String
		d.LastError = lastError.String
		deliveries = append(deliveries, &d)
	}
	return deliveries, nil
}

type DeadLetterRepo struct {
	db *sql.DB
}

func NewDeadLetterRepo(db *sql.DB) *DeadLetterRepo {
	return &DeadLetterRepo{db: db}
}

func (r *DeadLetterRepo) ListByTenant(tenantID string, limit, offset int) ([]*model.DeadLetterEntry, error) {
	rows, err := r.db.Query(
		`SELECT id, event_id, subscription_id, tenant_id, original_payload, failure_reason, retry_count, last_retry_at, created_at
		 FROM dead_letter_queue WHERE tenant_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		tenantID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*model.DeadLetterEntry
	for rows.Next() {
		var e model.DeadLetterEntry
		var lastRetryAt sql.NullTime
		err := rows.Scan(&e.ID, &e.EventID, &e.SubscriptionID, &e.TenantID, &e.OriginalPayload, &e.FailureReason, &e.RetryCount, &lastRetryAt, &e.CreatedAt)
		if err != nil {
			return nil, err
		}
		if lastRetryAt.Valid {
			e.LastRetryAt = &lastRetryAt.Time
		}
		entries = append(entries, &e)
	}
	return entries, nil
}

func (r *DeadLetterRepo) GetByID(id string) (*model.DeadLetterEntry, error) {
	row := r.db.QueryRow(
		`SELECT id, event_id, subscription_id, tenant_id, original_payload, failure_reason, retry_count, last_retry_at, created_at
		 FROM dead_letter_queue WHERE id=$1`, id,
	)
	var e model.DeadLetterEntry
	var lastRetryAt sql.NullTime
	err := row.Scan(&e.ID, &e.EventID, &e.SubscriptionID, &e.TenantID, &e.OriginalPayload, &e.FailureReason, &e.RetryCount, &lastRetryAt, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	if lastRetryAt.Valid {
		e.LastRetryAt = &lastRetryAt.Time
	}
	return &e, nil
}

func (r *DeadLetterRepo) Retry(id string) error {
	entry, err := r.GetByID(id)
	if err != nil {
		return err
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	deliveryID := uuid.New().String()
	_, err = tx.Exec(
		`INSERT INTO deliveries (id, event_id, subscription_id, tenant_id, status, retry_count)
		 VALUES ($1, $2, $3, $4, 'pending', 0)`,
		deliveryID, entry.EventID, entry.SubscriptionID, entry.TenantID,
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec(
		`UPDATE dead_letter_queue SET retry_count=retry_count+1, last_retry_at=NOW() WHERE id=$1`,
		id,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *DeadLetterRepo) BatchRetry(ids []string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, id := range ids {
		entry, err := r.GetByID(id)
		if err != nil {
			return err
		}
		deliveryID := uuid.New().String()
		_, err = tx.Exec(
			`INSERT INTO deliveries (id, event_id, subscription_id, tenant_id, status, retry_count)
			 VALUES ($1, $2, $3, $4, 'pending', 0)`,
			deliveryID, entry.EventID, entry.SubscriptionID, entry.TenantID,
		)
		if err != nil {
			return err
		}
		_, err = tx.Exec(
			`UPDATE dead_letter_queue SET retry_count=retry_count+1, last_retry_at=NOW() WHERE id=$1`,
			id,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *DeadLetterRepo) UpdatePayload(id string, payload string) error {
	_, err := r.db.Exec(
		`UPDATE dead_letter_queue SET original_payload=$2 WHERE id=$1`, id, payload,
	)
	return err
}

func (r *DeadLetterRepo) CountByTenant(tenantID string) (int, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM dead_letter_queue WHERE tenant_id=$1`, tenantID).Scan(&count)
	return count, err
}

type OrchestrationRepo struct {
	db *sql.DB
}

func NewOrchestrationRepo(db *sql.DB) *OrchestrationRepo {
	return &OrchestrationRepo{db: db}
}

func (r *OrchestrationRepo) Create(dag *model.OrchestrationDAG) error {
	dag.ID = uuid.New().String()
	nodes, _ := json.Marshal(dag.Nodes)
	edges, _ := json.Marshal(dag.Edges)
	row := r.db.QueryRow(
		`INSERT INTO orchestration_dags (id, subscription_id, nodes, edges)
		 VALUES ($1, $2, $3, $4)
		 RETURNING created_at, updated_at`,
		dag.ID, dag.SubscriptionID, nodes, edges,
	)
	return row.Scan(&dag.CreatedAt, &dag.UpdatedAt)
}

func (r *OrchestrationRepo) GetBySubscriptionID(subscriptionID string) (*model.OrchestrationDAG, error) {
	row := r.db.QueryRow(
		`SELECT id, subscription_id, nodes, edges, created_at, updated_at
		 FROM orchestration_dags WHERE subscription_id=$1`, subscriptionID,
	)
	var dag model.OrchestrationDAG
	var nodes, edges []byte
	err := row.Scan(&dag.ID, &dag.SubscriptionID, &nodes, &edges, &dag.CreatedAt, &dag.UpdatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(nodes, &dag.Nodes)
	json.Unmarshal(edges, &dag.Edges)
	return &dag, nil
}

func (r *OrchestrationRepo) Update(dag *model.OrchestrationDAG) error {
	nodes, _ := json.Marshal(dag.Nodes)
	edges, _ := json.Marshal(dag.Edges)
	_, err := r.db.Exec(
		`UPDATE orchestration_dags SET nodes=$2, edges=$3, updated_at=NOW() WHERE id=$1`,
		dag.ID, nodes, edges,
	)
	return err
}

func (r *OrchestrationRepo) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM orchestration_dags WHERE id=$1`, id)
	return err
}

func (r *OrchestrationRepo) ListByTenant(tenantID string) ([]*model.OrchestrationDAG, error) {
	rows, err := r.db.Query(
		`SELECT od.id, od.subscription_id, od.nodes, od.edges, od.created_at, od.updated_at
		 FROM orchestration_dags od
		 JOIN subscriptions s ON od.subscription_id = s.id
		 WHERE s.tenant_id=$1`, tenantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dags []*model.OrchestrationDAG
	for rows.Next() {
		var dag model.OrchestrationDAG
		var nodes, edges []byte
		if err := rows.Scan(&dag.ID, &dag.SubscriptionID, &nodes, &edges, &dag.CreatedAt, &dag.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal(nodes, &dag.Nodes)
		json.Unmarshal(edges, &dag.Edges)
		dags = append(dags, &dag)
	}
	return dags, nil
}

type TraceRepo struct {
	db *sql.DB
}

func NewTraceRepo(db *sql.DB) *TraceRepo {
	return &TraceRepo{db: db}
}

func (r *TraceRepo) Create(t *model.DeliveryTrace) error {
	t.ID = uuid.New().String()
	row := r.db.QueryRow(
		`INSERT INTO delivery_traces (id, event_id, subscription_id, tenant_id, node_id, node_type, node_name, status, input_payload, output_payload, error_message, duration_ms)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 RETURNING created_at`,
		t.ID, t.EventID, t.SubscriptionID, t.TenantID, t.NodeID, t.NodeType, t.NodeName, t.Status,
		nullString(t.InputPayload), nullString(t.OutputPayload), nullString(t.ErrorMessage), t.DurationMs,
	)
	return row.Scan(&t.CreatedAt)
}

func (r *TraceRepo) GetByEventID(eventID string) ([]*model.DeliveryTrace, error) {
	rows, err := r.db.Query(
		`SELECT id, event_id, subscription_id, tenant_id, node_id, node_type, node_name, status, input_payload, output_payload, error_message, duration_ms, created_at
		 FROM delivery_traces WHERE event_id=$1 ORDER BY created_at`, eventID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var traces []*model.DeliveryTrace
	for rows.Next() {
		var t model.DeliveryTrace
		var inputPayload, outputPayload, errorMessage sql.NullString
		err := rows.Scan(&t.ID, &t.EventID, &t.SubscriptionID, &t.TenantID, &t.NodeID, &t.NodeType, &t.NodeName,
			&t.Status, &inputPayload, &outputPayload, &errorMessage, &t.DurationMs, &t.CreatedAt)
		if err != nil {
			return nil, err
		}
		t.InputPayload = inputPayload.String
		t.OutputPayload = outputPayload.String
		t.ErrorMessage = errorMessage.String
		traces = append(traces, &t)
	}
	return traces, nil
}

func (r *TraceRepo) GetByEventAndSubscription(eventID, subscriptionID string) ([]*model.DeliveryTrace, error) {
	rows, err := r.db.Query(
		`SELECT id, event_id, subscription_id, tenant_id, node_id, node_type, node_name, status, input_payload, output_payload, error_message, duration_ms, created_at
		 FROM delivery_traces WHERE event_id=$1 AND subscription_id=$2 ORDER BY created_at`,
		eventID, subscriptionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var traces []*model.DeliveryTrace
	for rows.Next() {
		var t model.DeliveryTrace
		var inputPayload, outputPayload, errorMessage sql.NullString
		err := rows.Scan(&t.ID, &t.EventID, &t.SubscriptionID, &t.TenantID, &t.NodeID, &t.NodeType, &t.NodeName,
			&t.Status, &inputPayload, &outputPayload, &errorMessage, &t.DurationMs, &t.CreatedAt)
		if err != nil {
			return nil, err
		}
		t.InputPayload = inputPayload.String
		t.OutputPayload = outputPayload.String
		t.ErrorMessage = errorMessage.String
		traces = append(traces, &t)
	}
	return traces, nil
}

type AlertRepo struct {
	db *sql.DB
}

func NewAlertRepo(db *sql.DB) *AlertRepo {
	return &AlertRepo{db: db}
}

func (r *AlertRepo) Create(a *model.BackpressureAlert) error {
	a.ID = uuid.New().String()
	row := r.db.QueryRow(
		`INSERT INTO backpressure_alerts (id, tenant_id, subscription_id, alert_type, message)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING created_at`,
		a.ID, a.TenantID, a.SubscriptionID, a.AlertType, a.Message,
	)
	return row.Scan(&a.CreatedAt)
}

func (r *AlertRepo) ListByTenant(tenantID string, resolved bool) ([]*model.BackpressureAlert, error) {
	query := `SELECT id, tenant_id, subscription_id, alert_type, message, is_resolved, created_at, resolved_at
		 FROM backpressure_alerts WHERE tenant_id=$1`
	if !resolved {
		query += ` AND is_resolved=false`
	}
	query += ` ORDER BY created_at DESC`

	rows, err := r.db.Query(query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []*model.BackpressureAlert
	for rows.Next() {
		var a model.BackpressureAlert
		var resolvedAt sql.NullTime
		err := rows.Scan(&a.ID, &a.TenantID, &a.SubscriptionID, &a.AlertType, &a.Message, &a.IsResolved, &a.CreatedAt, &resolvedAt)
		if err != nil {
			return nil, err
		}
		if resolvedAt.Valid {
			a.ResolvedAt = &resolvedAt.Time
		}
		alerts = append(alerts, &a)
	}
	return alerts, nil
}

func (r *AlertRepo) Resolve(id string) error {
	_, err := r.db.Exec(
		`UPDATE backpressure_alerts SET is_resolved=true, resolved_at=NOW() WHERE id=$1`, id,
	)
	return err
}
