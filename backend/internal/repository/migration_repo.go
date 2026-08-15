package repository

import (
	"database/sql"
	"time"

	"github.com/eventbus/server/internal/model"
	"github.com/google/uuid"
)

type MigrationRepo struct {
	db *sql.DB
}

func NewMigrationRepo(db *sql.DB) *MigrationRepo {
	return &MigrationRepo{db: db}
}

func (r *MigrationRepo) DB() *sql.DB {
	return r.db
}

func (r *MigrationRepo) Create(m *model.SchemaMigration) error {
	m.ID = uuid.New().String()
	m.Status = model.MigrationStatusDraft
	row := r.db.QueryRow(
		`INSERT INTO schema_migrations (id, tenant_id, event_type, source_version, target_version, status, migration_rules, total_events, processed_events, failed_events)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING created_at, updated_at`,
		m.ID, m.TenantID, m.EventType, m.SourceVersion, m.TargetVersion, m.Status, m.MigrationRules,
		m.TotalEvents, m.ProcessedEvents, m.FailedEvents,
	)
	return row.Scan(&m.CreatedAt, &m.UpdatedAt)
}

func (r *MigrationRepo) GetByID(id string) (*model.SchemaMigration, error) {
	row := r.db.QueryRow(
		`SELECT id, tenant_id, event_type, source_version, target_version, status, migration_rules, 
		 total_events, processed_events, failed_events, error_message, started_at, completed_at, rollback_deadline, created_at, updated_at
		 FROM schema_migrations WHERE id=$1`, id,
	)
	var m model.SchemaMigration
	var startedAt, completedAt, rollbackDeadline sql.NullTime
	err := row.Scan(&m.ID, &m.TenantID, &m.EventType, &m.SourceVersion, &m.TargetVersion, &m.Status,
		&m.MigrationRules, &m.TotalEvents, &m.ProcessedEvents, &m.FailedEvents, &m.ErrorMessage,
		&startedAt, &completedAt, &rollbackDeadline, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if startedAt.Valid {
		m.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		m.CompletedAt = &completedAt.Time
	}
	if rollbackDeadline.Valid {
		m.RollbackDeadline = &rollbackDeadline.Time
	}
	return &m, nil
}

func (r *MigrationRepo) ListByEventType(tenantID, eventType string) ([]*model.SchemaMigration, error) {
	rows, err := r.db.Query(
		`SELECT id, tenant_id, event_type, source_version, target_version, status, migration_rules,
		 total_events, processed_events, failed_events, error_message, started_at, completed_at, rollback_deadline, created_at, updated_at
		 FROM schema_migrations WHERE tenant_id=$1 AND event_type=$2 ORDER BY created_at DESC`,
		tenantID, eventType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var migrations []*model.SchemaMigration
	for rows.Next() {
		var m model.SchemaMigration
		var startedAt, completedAt, rollbackDeadline sql.NullTime
		err := rows.Scan(&m.ID, &m.TenantID, &m.EventType, &m.SourceVersion, &m.TargetVersion, &m.Status,
			&m.MigrationRules, &m.TotalEvents, &m.ProcessedEvents, &m.FailedEvents, &m.ErrorMessage,
			&startedAt, &completedAt, &rollbackDeadline, &m.CreatedAt, &m.UpdatedAt)
		if err != nil {
			return nil, err
		}
		if startedAt.Valid {
			m.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			m.CompletedAt = &completedAt.Time
		}
		if rollbackDeadline.Valid {
			m.RollbackDeadline = &rollbackDeadline.Time
		}
		migrations = append(migrations, &m)
	}
	return migrations, nil
}

func (r *MigrationRepo) CheckRunningMigration(tenantID, eventType string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM schema_migrations 
		 WHERE tenant_id=$1 AND event_type=$2 AND status IN ($3, $4))`,
		tenantID, eventType, model.MigrationStatusRunning, model.MigrationStatusRollbackRunning,
	).Scan(&exists)
	return exists, err
}

func (r *MigrationRepo) UpdateStatus(id string, status model.MigrationStatus) error {
	now := time.Now()
	_, err := r.db.Exec(
		`UPDATE schema_migrations SET status=$2, updated_at=$3 WHERE id=$1`,
		id, status, now,
	)
	return err
}

func (r *MigrationRepo) StartMigration(id string) error {
	now := time.Now()
	_, err := r.db.Exec(
		`UPDATE schema_migrations SET status=$2, started_at=$3, updated_at=$3 WHERE id=$1`,
		id, model.MigrationStatusRunning, now,
	)
	return err
}

func (r *MigrationRepo) CompleteMigration(id string, processed, failed int) error {
	now := time.Now()
	deadline := now.Add(24 * time.Hour)
	_, err := r.db.Exec(
		`UPDATE schema_migrations SET status=$2, processed_events=$3, failed_events=$4, 
		 completed_at=$5, rollback_deadline=$6, updated_at=$5 WHERE id=$1`,
		id, model.MigrationStatusCompleted, processed, failed, now, deadline,
	)
	return err
}

func (r *MigrationRepo) FailMigration(id string, errorMsg string, processed, failed int) error {
	now := time.Now()
	_, err := r.db.Exec(
		`UPDATE schema_migrations SET status=$2, error_message=$3, processed_events=$4, failed_events=$5, 
		 completed_at=$6, updated_at=$6 WHERE id=$1`,
		id, model.MigrationStatusFailed, errorMsg, processed, failed, now,
	)
	return err
}

func (r *MigrationRepo) UpdateProgress(id string, processed, failed int) error {
	_, err := r.db.Exec(
		`UPDATE schema_migrations SET processed_events=$2, failed_events=$3, updated_at=NOW() WHERE id=$1`,
		id, processed, failed,
	)
	return err
}

func (r *MigrationRepo) StartRollback(id string) error {
	now := time.Now()
	_, err := r.db.Exec(
		`UPDATE schema_migrations SET status=$2, started_at=$3, updated_at=$3 WHERE id=$1`,
		id, model.MigrationStatusRollbackRunning, now,
	)
	return err
}

func (r *MigrationRepo) CompleteRollback(id string, processed, failed int) error {
	now := time.Now()
	_, err := r.db.Exec(
		`UPDATE schema_migrations SET status=$2, processed_events=$3, failed_events=$4, 
		 completed_at=$5, updated_at=$5 WHERE id=$1`,
		id, model.MigrationStatusRollbacked, processed, failed, now,
	)
	return err
}

func (r *MigrationRepo) FailRollback(id string, errorMsg string, processed, failed int) error {
	now := time.Now()
	_, err := r.db.Exec(
		`UPDATE schema_migrations SET status=$2, error_message=$3, processed_events=$4, failed_events=$5, 
		 completed_at=$6, updated_at=$6 WHERE id=$1`,
		id, model.MigrationStatusRollbackFailed, errorMsg, processed, failed, now,
	)
	return err
}

func (r *MigrationRepo) CreateBackup(b *model.MigrationEventBackup) error {
	b.ID = uuid.New().String()
	now := time.Now()
	b.CreatedAt = now
	b.UpdatedAt = now
	_, err := r.db.Exec(
		`INSERT INTO migration_event_backups (id, migration_id, event_id, original_payload, original_schema_version, status)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		b.ID, b.MigrationID, b.EventID, b.OriginalPayload, b.OriginalSchemaVersion, b.Status,
	)
	return err
}

func (r *MigrationRepo) UpdateBackupSuccess(backupID, newPayload string, newSchemaVersion int) error {
	_, err := r.db.Exec(
		`UPDATE migration_event_backups SET new_payload=$2, new_schema_version=$3, status='success', updated_at=NOW() 
		 WHERE id=$1`,
		backupID, newPayload, newSchemaVersion,
	)
	return err
}

func (r *MigrationRepo) UpdateBackupFailed(backupID, errorMsg string) error {
	_, err := r.db.Exec(
		`UPDATE migration_event_backups SET status='failed', error_message=$2, updated_at=NOW() WHERE id=$1`,
		backupID, errorMsg,
	)
	return err
}

func (r *MigrationRepo) GetPendingBackups(migrationID string, limit int) ([]*model.MigrationEventBackup, error) {
	rows, err := r.db.Query(
		`SELECT id, migration_id, event_id, original_payload, original_schema_version, status, created_at, updated_at
		 FROM migration_event_backups WHERE migration_id=$1 AND status='pending' ORDER BY created_at ASC LIMIT $2`,
		migrationID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var backups []*model.MigrationEventBackup
	for rows.Next() {
		var b model.MigrationEventBackup
		var newPayload sql.NullString
		var newSchemaVersion sql.NullInt64
		var errorMessage sql.NullString
		err := rows.Scan(&b.ID, &b.MigrationID, &b.EventID, &b.OriginalPayload, &b.OriginalSchemaVersion,
			&b.Status, &b.CreatedAt, &b.UpdatedAt)
		if err != nil {
			return nil, err
		}
		if newPayload.Valid {
			b.NewPayload = newPayload.String
		}
		if newSchemaVersion.Valid {
			b.NewSchemaVersion = int(newSchemaVersion.Int64)
		}
		if errorMessage.Valid {
			b.ErrorMessage = errorMessage.String
		}
		backups = append(backups, &b)
	}
	return backups, nil
}

func (r *MigrationRepo) GetSuccessBackups(migrationID string, limit int) ([]*model.MigrationEventBackup, error) {
	rows, err := r.db.Query(
		`SELECT id, migration_id, event_id, original_payload, original_schema_version, new_payload, new_schema_version, status, created_at, updated_at
		 FROM migration_event_backups WHERE migration_id=$1 AND status='success' ORDER BY created_at ASC LIMIT $2`,
		migrationID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var backups []*model.MigrationEventBackup
	for rows.Next() {
		var b model.MigrationEventBackup
		var newPayload sql.NullString
		var newSchemaVersion sql.NullInt64
		var errorMessage sql.NullString
		err := rows.Scan(&b.ID, &b.MigrationID, &b.EventID, &b.OriginalPayload, &b.OriginalSchemaVersion,
			&newPayload, &newSchemaVersion, &b.Status, &b.CreatedAt, &b.UpdatedAt)
		if err != nil {
			return nil, err
		}
		if newPayload.Valid {
			b.NewPayload = newPayload.String
		}
		if newSchemaVersion.Valid {
			b.NewSchemaVersion = int(newSchemaVersion.Int64)
		}
		if errorMessage.Valid {
			b.ErrorMessage = errorMessage.String
		}
		backups = append(backups, &b)
	}
	return backups, nil
}

func (r *MigrationRepo) UpdateBackupRollback(backupID string) error {
	_, err := r.db.Exec(
		`UPDATE migration_event_backups SET status='rollbacked', updated_at=NOW() WHERE id=$1`,
		backupID,
	)
	return err
}
