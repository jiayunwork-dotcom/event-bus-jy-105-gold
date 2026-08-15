package repository

import (
	"database/sql"
	"fmt"

	"github.com/eventbus/server/internal/model"
	"github.com/google/uuid"
)

type SubscriptionRepo struct {
	db *sql.DB
}

func NewSubscriptionRepo(db *sql.DB) *SubscriptionRepo {
	return &SubscriptionRepo{db: db}
}

func (r *SubscriptionRepo) Create(s *model.Subscription) error {
	s.ID = uuid.New().String()
	row := r.db.QueryRow(
		`INSERT INTO subscriptions (id, tenant_id, name, event_type, filter_expression, delivery_mode,
		 idempotent_key_path, idempotent_window_seconds, max_retries, consumer_url,
		 consumer_rate_limit, consumer_burst, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		 RETURNING created_at, updated_at`,
		s.ID, s.TenantID, s.Name, s.EventType, nullString(s.FilterExpression), s.DeliveryMode,
		nullString(s.IdempotentKeyPath), s.IdempotentWindowSeconds, s.MaxRetries, s.ConsumerURL,
		s.ConsumerRateLimit, s.ConsumerBurst, s.Status,
	)
	return row.Scan(&s.CreatedAt, &s.UpdatedAt)
}

func (r *SubscriptionRepo) GetByID(id string) (*model.Subscription, error) {
	row := r.db.QueryRow(
		`SELECT id, tenant_id, name, event_type, filter_expression, delivery_mode,
		 idempotent_key_path, idempotent_window_seconds, max_retries, consumer_url,
		 consumer_rate_limit, consumer_burst, status, created_at, updated_at
		 FROM subscriptions WHERE id=$1`, id,
	)
	var s model.Subscription
	var filterExpr, idempotentKeyPath sql.NullString
	err := row.Scan(&s.ID, &s.TenantID, &s.Name, &s.EventType, &filterExpr, &s.DeliveryMode,
		&idempotentKeyPath, &s.IdempotentWindowSeconds, &s.MaxRetries, &s.ConsumerURL,
		&s.ConsumerRateLimit, &s.ConsumerBurst, &s.Status, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	s.FilterExpression = filterExpr.String
	s.IdempotentKeyPath = idempotentKeyPath.String
	return &s, nil
}

func (r *SubscriptionRepo) ListByTenant(tenantID string) ([]*model.Subscription, error) {
	rows, err := r.db.Query(
		`SELECT id, tenant_id, name, event_type, filter_expression, delivery_mode,
		 idempotent_key_path, idempotent_window_seconds, max_retries, consumer_url,
		 consumer_rate_limit, consumer_burst, status, created_at, updated_at
		 FROM subscriptions WHERE tenant_id=$1 ORDER BY created_at DESC`, tenantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []*model.Subscription
	for rows.Next() {
		var s model.Subscription
		var filterExpr, idempotentKeyPath sql.NullString
		if err := rows.Scan(&s.ID, &s.TenantID, &s.Name, &s.EventType, &filterExpr, &s.DeliveryMode,
			&idempotentKeyPath, &s.IdempotentWindowSeconds, &s.MaxRetries, &s.ConsumerURL,
			&s.ConsumerRateLimit, &s.ConsumerBurst, &s.Status, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		s.FilterExpression = filterExpr.String
		s.IdempotentKeyPath = idempotentKeyPath.String
		subs = append(subs, &s)
	}
	return subs, nil
}

func (r *SubscriptionRepo) ListByTenantAndEventType(tenantID, eventType string) ([]*model.Subscription, error) {
	rows, err := r.db.Query(
		`SELECT id, tenant_id, name, event_type, filter_expression, delivery_mode,
		 idempotent_key_path, idempotent_window_seconds, max_retries, consumer_url,
		 consumer_rate_limit, consumer_burst, status, created_at, updated_at
		 FROM subscriptions WHERE tenant_id=$1 AND event_type=$2 AND status='active'`, tenantID, eventType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []*model.Subscription
	for rows.Next() {
		var s model.Subscription
		var filterExpr, idempotentKeyPath sql.NullString
		if err := rows.Scan(&s.ID, &s.TenantID, &s.Name, &s.EventType, &filterExpr, &s.DeliveryMode,
			&idempotentKeyPath, &s.IdempotentWindowSeconds, &s.MaxRetries, &s.ConsumerURL,
			&s.ConsumerRateLimit, &s.ConsumerBurst, &s.Status, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		s.FilterExpression = filterExpr.String
		s.IdempotentKeyPath = idempotentKeyPath.String
		subs = append(subs, &s)
	}
	return subs, nil
}

func (r *SubscriptionRepo) Update(s *model.Subscription) error {
	_, err := r.db.Exec(
		`UPDATE subscriptions SET name=$2, event_type=$3, filter_expression=$4, delivery_mode=$5,
		 idempotent_key_path=$6, idempotent_window_seconds=$7, max_retries=$8, consumer_url=$9,
		 consumer_rate_limit=$10, consumer_burst=$11, status=$12, updated_at=NOW()
		 WHERE id=$1`,
		s.ID, s.Name, s.EventType, s.FilterExpression, s.DeliveryMode,
		s.IdempotentKeyPath, s.IdempotentWindowSeconds, s.MaxRetries, s.ConsumerURL,
		s.ConsumerRateLimit, s.ConsumerBurst, s.Status,
	)
	return err
}

func (r *SubscriptionRepo) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM subscriptions WHERE id=$1`, id)
	return err
}

func (r *SubscriptionRepo) UpdateRateLimit(id string, rateLimit, burst int) error {
	_, err := r.db.Exec(
		`UPDATE subscriptions SET consumer_rate_limit=$2, consumer_burst=$3, updated_at=NOW() WHERE id=$1`,
		id, rateLimit, burst,
	)
	return err
}

func (r *SubscriptionRepo) GetBacklogCount(subscriptionID string) (int, error) {
	var count int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM deliveries WHERE subscription_id=$1 AND status IN ('pending', 'retrying')`,
		subscriptionID,
	).Scan(&count)
	return count, err
}

func (r *SubscriptionRepo) GetBacklogCountsByTenant(tenantID string) (map[string]int, error) {
	rows, err := r.db.Query(
		`SELECT subscription_id, COUNT(*) FROM deliveries WHERE tenant_id=$1 AND status IN ('pending', 'retrying') GROUP BY subscription_id`,
		tenantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var subID string
		var count int
		if err := rows.Scan(&subID, &count); err != nil {
			return nil, err
		}
		result[subID] = count
	}
	return result, nil
}

func (r *SubscriptionRepo) GetOnlineStatus(subscriptionID string) (bool, error) {
	var count int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM deliveries WHERE subscription_id=$1 AND status='delivered' AND updated_at > NOW() - INTERVAL '5 minutes'`,
		subscriptionID,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *SubscriptionRepo) GetDeliveryLatencyStats(tenantID string) (map[string]interface{}, error) {
	row := r.db.QueryRow(
		`SELECT 
			AVG(EXTRACT(EPOCH FROM (updated_at - created_at))*1000) as avg_latency_ms,
			PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (updated_at - created_at))*1000) as p50_ms,
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (updated_at - created_at))*1000) as p95_ms,
			PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (updated_at - created_at))*1000) as p99_ms
		 FROM deliveries WHERE tenant_id=$1 AND status='delivered' AND updated_at > NOW() - INTERVAL '1 hour'`,
		tenantID,
	)
	var avgLatency, p50, p95, p99 sql.NullFloat64
	err := row.Scan(&avgLatency, &p50, &p95, &p99)
	if err != nil {
		return map[string]interface{}{
			"avg_latency_ms": 0, "p50_ms": 0, "p95_ms": 0, "p99_ms": 0,
		}, nil
	}
	return map[string]interface{}{
		"avg_latency_ms": avgLatency.Float64,
		"p50_ms":         p50.Float64,
		"p95_ms":         p95.Float64,
		"p99_ms":         p99.Float64,
	}, nil
}

func (r *SubscriptionRepo) GetPublishQPS(tenantID string) (int, error) {
	var count int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM events WHERE tenant_id=$1 AND created_at > NOW() - INTERVAL '1 second'`,
		tenantID,
	).Scan(&count)
	return count, err
}

func (r *SubscriptionRepo) CountByTenant(tenantID string) (int, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM subscriptions WHERE tenant_id=$1`, tenantID).Scan(&count)
	return count, err
}

func (r *SubscriptionRepo) ListActiveByEventType(tenantID, eventType string) ([]*model.Subscription, error) {
	return r.ListByTenantAndEventType(tenantID, eventType)
}

func (r *SubscriptionRepo) GetAllActiveForTenant(tenantID string) ([]*model.Subscription, error) {
	rows, err := r.db.Query(
		`SELECT id, tenant_id, name, event_type, filter_expression, delivery_mode,
		 idempotent_key_path, idempotent_window_seconds, max_retries, consumer_url,
		 consumer_rate_limit, consumer_burst, status, created_at, updated_at
		 FROM subscriptions WHERE tenant_id=$1 AND status='active'`, tenantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []*model.Subscription
	for rows.Next() {
		var s model.Subscription
		var filterExpr, idempotentKeyPath sql.NullString
		if err := rows.Scan(&s.ID, &s.TenantID, &s.Name, &s.EventType, &filterExpr, &s.DeliveryMode,
			&idempotentKeyPath, &s.IdempotentWindowSeconds, &s.MaxRetries, &s.ConsumerURL,
			&s.ConsumerRateLimit, &s.ConsumerBurst, &s.Status, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		s.FilterExpression = filterExpr.String
		s.IdempotentKeyPath = idempotentKeyPath.String
		subs = append(subs, &s)
	}
	return subs, nil
}

func (r *SubscriptionRepo) UpdateBackpressure(subID string, rateLimit, burst int) error {
	return r.UpdateRateLimit(subID, rateLimit, burst)
}

func (r *SubscriptionRepo) GetDeliveryStats(tenantID string, minutes int) (map[string]int64, error) {
	row := r.db.QueryRow(
		fmt.Sprintf(`SELECT 
			COUNT(*) FILTER (WHERE status = 'delivered') as delivered,
			COUNT(*) FILTER (WHERE status = 'pending') as pending,
			COUNT(*) FILTER (WHERE status = 'retrying') as retrying,
			COUNT(*) FILTER (WHERE status = 'failed') as failed
		 FROM deliveries WHERE tenant_id=$1 AND created_at > NOW() - INTERVAL '%d minutes'`, minutes),
		tenantID,
	)
	var delivered, pending, retrying, failed int64
	err := row.Scan(&delivered, &pending, &retrying, &failed)
	if err != nil {
		return nil, err
	}
	return map[string]int64{
		"delivered": delivered,
		"pending":   pending,
		"retrying":  retrying,
		"failed":    failed,
	}, nil
}
