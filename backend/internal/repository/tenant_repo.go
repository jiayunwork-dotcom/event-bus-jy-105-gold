package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/eventbus/server/internal/model"
	"github.com/google/uuid"
)

type TenantRepo struct {
	db *sql.DB
}

func NewTenantRepo(db *sql.DB) *TenantRepo {
	return &TenantRepo{db: db}
}

func (r *TenantRepo) Create(t *model.Tenant) error {
	t.ID = uuid.New().String()
	rp, _ := json.Marshal(t.RetentionPolicy)
	row := r.db.QueryRow(
		`INSERT INTO tenants (id, name, status, max_publish_qps, max_subscriptions, max_storage_mb, retention_policy)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING created_at, updated_at`,
		t.ID, t.Name, t.Status, t.MaxPublishQPS, t.MaxSubscriptions, t.MaxStorageMB, rp,
	)
	return row.Scan(&t.CreatedAt, &t.UpdatedAt)
}

func (r *TenantRepo) GetByID(id string) (*model.Tenant, error) {
	row := r.db.QueryRow(
		`SELECT id, name, status, max_publish_qps, max_subscriptions, max_storage_mb, retention_policy, created_at, updated_at
		 FROM tenants WHERE id=$1`, id,
	)
	var t model.Tenant
	var rp []byte
	err := row.Scan(&t.ID, &t.Name, &t.Status, &t.MaxPublishQPS, &t.MaxSubscriptions, &t.MaxStorageMB, &rp, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(rp, &t.RetentionPolicy)
	return &t, nil
}

func (r *TenantRepo) List() ([]*model.Tenant, error) {
	rows, err := r.db.Query(
		`SELECT id, name, status, max_publish_qps, max_subscriptions, max_storage_mb, retention_policy, created_at, updated_at
		 FROM tenants ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []*model.Tenant
	for rows.Next() {
		var t model.Tenant
		var rp []byte
		if err := rows.Scan(&t.ID, &t.Name, &t.Status, &t.MaxPublishQPS, &t.MaxSubscriptions, &t.MaxStorageMB, &rp, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal(rp, &t.RetentionPolicy)
		tenants = append(tenants, &t)
	}
	return tenants, nil
}

func (r *TenantRepo) Update(t *model.Tenant) error {
	rp, _ := json.Marshal(t.RetentionPolicy)
	_, err := r.db.Exec(
		`UPDATE tenants SET name=$2, status=$3, max_publish_qps=$4, max_subscriptions=$5, max_storage_mb=$6, retention_policy=$7, updated_at=NOW()
		 WHERE id=$1`,
		t.ID, t.Name, t.Status, t.MaxPublishQPS, t.MaxSubscriptions, t.MaxStorageMB, rp,
	)
	return err
}

func (r *TenantRepo) GetSubscriptionCount(tenantID string) (int, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM subscriptions WHERE tenant_id=$1`, tenantID).Scan(&count)
	return count, err
}

func (r *TenantRepo) GetStorageUsageMB(tenantID string) (float64, error) {
	var size float64
	err := r.db.QueryRow(
		`SELECT COALESCE(pg_column_size(payload)::float / 1048576, 0) FROM events WHERE tenant_id=$1 LIMIT 1`,
		tenantID,
	).Scan(&size)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	var totalMB float64
	r.db.QueryRow(
		`SELECT COALESCE(SUM(pg_column_size(payload)::float) / 1048576, 0) FROM events WHERE tenant_id=$1`,
		tenantID,
	).Scan(&totalMB)
	return totalMB, nil
}

func (r *TenantRepo) ValidateQuota(tenantID string) (bool, string, error) {
	t, err := r.GetByID(tenantID)
	if err != nil {
		return false, "tenant not found", err
	}
	if t.Status != "active" {
		return false, "tenant is not active", nil
	}
	subCount, err := r.GetSubscriptionCount(tenantID)
	if err != nil {
		return false, "failed to check subscription count", err
	}
	if subCount >= t.MaxSubscriptions {
		return false, fmt.Sprintf("subscription limit reached (%d/%d)", subCount, t.MaxSubscriptions), nil
	}
	return true, "", nil
}
