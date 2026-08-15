package repository

import (
	"database/sql"
	"encoding/json"

	"github.com/eventbus/server/internal/model"
	"github.com/google/uuid"
)

type EventRepo struct {
	db *sql.DB
}

func NewEventRepo(db *sql.DB) *EventRepo {
	return &EventRepo{db: db}
}

func (r *EventRepo) DB() *sql.DB {
	return r.db
}

func (r *EventRepo) Create(e *model.Event) error {
	e.ID = uuid.New().String()
	row := r.db.QueryRow(
		`INSERT INTO events (id, tenant_id, event_type, schema_version, payload, idempotent_key)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING created_at`,
		e.ID, e.TenantID, e.EventType, e.SchemaVersion, e.Payload, nullString(e.IdempotentKey),
	)
	return row.Scan(&e.CreatedAt)
}

func (r *EventRepo) CreateBatch(events []*model.Event) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT INTO events (id, tenant_id, event_type, schema_version, payload, idempotent_key)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range events {
		e.ID = uuid.New().String()
		_, err := stmt.Exec(e.ID, e.TenantID, e.EventType, e.SchemaVersion, e.Payload, nullString(e.IdempotentKey))
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *EventRepo) GetByID(id string) (*model.Event, error) {
	row := r.db.QueryRow(
		`SELECT id, tenant_id, event_type, schema_version, payload, idempotent_key, created_at
		 FROM events WHERE id=$1`, id,
	)
	var e model.Event
	var idempotentKey sql.NullString
	err := row.Scan(&e.ID, &e.TenantID, &e.EventType, &e.SchemaVersion, &e.Payload, &idempotentKey, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	e.IdempotentKey = idempotentKey.String
	return &e, nil
}

func (r *EventRepo) ListByTenant(tenantID string, limit, offset int) ([]*model.Event, error) {
	rows, err := r.db.Query(
		`SELECT id, tenant_id, event_type, schema_version, payload, idempotent_key, created_at
		 FROM events WHERE tenant_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		tenantID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*model.Event
	for rows.Next() {
		var e model.Event
		var idempotentKey sql.NullString
		if err := rows.Scan(&e.ID, &e.TenantID, &e.EventType, &e.SchemaVersion, &e.Payload, &idempotentKey, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.IdempotentKey = idempotentKey.String
		events = append(events, &e)
	}
	return events, nil
}

func (r *EventRepo) GetByTimeRange(tenantID string, startTime, endTime string, limit int) ([]*model.Event, error) {
	rows, err := r.db.Query(
		`SELECT id, tenant_id, event_type, schema_version, payload, idempotent_key, created_at
		 FROM events WHERE tenant_id=$1 AND created_at >= $2 AND created_at <= $3
		 ORDER BY created_at ASC LIMIT $4`,
		tenantID, startTime, endTime, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*model.Event
	for rows.Next() {
		var e model.Event
		var idempotentKey sql.NullString
		if err := rows.Scan(&e.ID, &e.TenantID, &e.EventType, &e.SchemaVersion, &e.Payload, &idempotentKey, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.IdempotentKey = idempotentKey.String
		events = append(events, &e)
	}
	return events, nil
}

func (r *EventRepo) GetByEventTypeAndTimeRange(tenantID string, eventType string, startTime, endTime string, limit int) ([]*model.Event, error) {
	rows, err := r.db.Query(
		`SELECT id, tenant_id, event_type, schema_version, payload, idempotent_key, created_at
		 FROM events WHERE tenant_id=$1 AND event_type=$2 AND created_at >= $3 AND created_at <= $4
		 ORDER BY created_at ASC LIMIT $5`,
		tenantID, eventType, startTime, endTime, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*model.Event
	for rows.Next() {
		var e model.Event
		var idempotentKey sql.NullString
		if err := rows.Scan(&e.ID, &e.TenantID, &e.EventType, &e.SchemaVersion, &e.Payload, &idempotentKey, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.IdempotentKey = idempotentKey.String
		events = append(events, &e)
	}
	return events, nil
}

func (r *EventRepo) GetByOffset(tenantID string, afterID string, limit int) ([]*model.Event, error) {
	if afterID == "" {
		rows, err := r.db.Query(
			`SELECT id, tenant_id, event_type, schema_version, payload, idempotent_key, created_at
			 FROM events WHERE tenant_id=$1 ORDER BY created_at ASC LIMIT $2`,
			tenantID, limit,
		)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return scanEvents(rows)
	}

	rows, err := r.db.Query(
		`SELECT id, tenant_id, event_type, schema_version, payload, idempotent_key, created_at
		 FROM events WHERE tenant_id=$1 AND created_at > (SELECT created_at FROM events WHERE id=$2)
		 ORDER BY created_at ASC LIMIT $3`,
		tenantID, afterID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

func scanEvents(rows *sql.Rows) ([]*model.Event, error) {
	var events []*model.Event
	for rows.Next() {
		var e model.Event
		var idempotentKey sql.NullString
		if err := rows.Scan(&e.ID, &e.TenantID, &e.EventType, &e.SchemaVersion, &e.Payload, &idempotentKey, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.IdempotentKey = idempotentKey.String
		events = append(events, &e)
	}
	return events, nil
}

func (r *EventRepo) CheckIdempotentKey(tenantID, key string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM events WHERE tenant_id=$1 AND idempotent_key=$2)`,
		tenantID, key,
	).Scan(&exists)
	return exists, err
}

func (r *EventRepo) UpdatePayload(id string, payload string) error {
	_, err := r.db.Exec(`UPDATE events SET payload=$2 WHERE id=$1`, id, payload)
	return err
}

func (r *EventRepo) CountByTenant(tenantID string) (int64, error) {
	var count int64
	err := r.db.QueryRow(`SELECT COUNT(*) FROM events WHERE tenant_id=$1`, tenantID).Scan(&count)
	return count, err
}

func (r *EventRepo) CleanupOldEvents(tenantID string, retentionDays int) (int64, error) {
	result, err := r.db.Exec(
		`DELETE FROM events WHERE tenant_id=$1 AND created_at < NOW() - ($2 || ' days')::interval
		 AND id NOT IN (SELECT event_id FROM dead_letter_queue)`,
		tenantID, retentionDays,
	)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return affected, nil
}

func (r *EventRepo) CleanupBySizeLimit(tenantID string, maxMB int) (int64, error) {
	var totalMB float64
	r.db.QueryRow(
		`SELECT COALESCE(SUM(pg_column_size(payload)::float) / 1048576, 0) FROM events WHERE tenant_id=$1`,
		tenantID,
	).Scan(&totalMB)

	if int(totalMB) <= maxMB {
		return 0, nil
	}

	result, err := r.db.Exec(
		`DELETE FROM events WHERE tenant_id=$1 AND id IN (
		 SELECT id FROM events WHERE tenant_id=$1 AND id NOT IN (SELECT event_id FROM dead_letter_queue)
		 ORDER BY created_at ASC LIMIT 1000
		)`, tenantID,
	)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return affected, nil
}

func (r *EventRepo) GetQPSHistory(tenantID string, minutes int) ([]map[string]interface{}, error) {
	rows, err := r.db.Query(
		`SELECT date_trunc('minute', created_at) as minute, COUNT(*) as count
		 FROM events WHERE tenant_id=$1 AND created_at > NOW() - ($2 || ' minutes')::interval
		 GROUP BY minute ORDER BY minute`, tenantID, minutes,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var minute string
		var count int64
		if err := rows.Scan(&minute, &count); err != nil {
			return nil, err
		}
		result = append(result, map[string]interface{}{
			"time":  minute,
			"count": count,
		})
	}
	return result, nil
}

func (r *EventRepo) GetHeatmapData(tenantID string, minutes int) ([]map[string]interface{}, error) {
	rows, err := r.db.Query(
		`SELECT event_type, date_trunc('minute', created_at) as minute, COUNT(*) as count
		 FROM events WHERE tenant_id=$1 AND created_at > NOW() - ($2 || ' minutes')::interval
		 GROUP BY event_type, minute ORDER BY event_type, minute`, tenantID, minutes,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var eventType string
		var minute string
		var count int64
		if err := rows.Scan(&eventType, &minute, &count); err != nil {
			return nil, err
		}
		result = append(result, map[string]interface{}{
			"event_type": eventType,
			"minute":     minute,
			"count":      count,
		})
	}
	return result, nil
}

func (r *EventRepo) GetDistinctEventTypes(tenantID string) ([]string, error) {
	rows, err := r.db.Query(
		`SELECT DISTINCT event_type FROM events WHERE tenant_id=$1 ORDER BY event_type`, tenantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var types []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		types = append(types, t)
	}
	return types, nil
}

func (r *EventRepo) MarshalPayload(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
