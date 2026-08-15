package engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/eventbus/server/internal/model"
	"github.com/eventbus/server/internal/repository"
)

type MigrationExecutor struct {
	eventRepo     *repository.EventRepo
	migrationRepo *repository.MigrationRepo
	engine        *MigrationEngine
	runningTasks  map[string]context.CancelFunc
	mu            sync.Mutex
}

func NewMigrationExecutor(eventRepo *repository.EventRepo, migrationRepo *repository.MigrationRepo, engine *MigrationEngine) *MigrationExecutor {
	return &MigrationExecutor{
		eventRepo:     eventRepo,
		migrationRepo: migrationRepo,
		engine:        engine,
		runningTasks:  make(map[string]context.CancelFunc),
	}
}

func (e *MigrationExecutor) StartMigration(migrationID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.runningTasks[migrationID]; exists {
		return fmt.Errorf("migration %s is already running", migrationID)
	}

	ctx, cancel := context.WithCancel(context.Background())
	e.runningTasks[migrationID] = cancel

	go e.executeMigration(ctx, migrationID)
	return nil
}

func (e *MigrationExecutor) CancelMigration(migrationID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	cancel, exists := e.runningTasks[migrationID]
	if !exists {
		return fmt.Errorf("migration %s is not running", migrationID)
	}

	cancel()
	delete(e.runningTasks, migrationID)

	return e.migrationRepo.UpdateStatus(migrationID, model.MigrationStatusCancelled)
}

func (e *MigrationExecutor) StartRollback(migrationID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.runningTasks[migrationID]; exists {
		return fmt.Errorf("migration %s has a running task", migrationID)
	}

	migration, err := e.migrationRepo.GetByID(migrationID)
	if err != nil {
		return fmt.Errorf("migration not found: %w", err)
	}

	if migration.Status != model.MigrationStatusCompleted {
		return fmt.Errorf("can only rollback completed migrations, current status: %s", migration.Status)
	}

	if migration.RollbackDeadline != nil && time.Now().After(*migration.RollbackDeadline) {
		return fmt.Errorf("rollback deadline has passed")
	}

	ctx, cancel := context.WithCancel(context.Background())
	e.runningTasks[migrationID] = cancel

	go e.executeRollback(ctx, migrationID)
	return nil
}

func (e *MigrationExecutor) executeMigration(ctx context.Context, migrationID string) {
	defer e.cleanupTask(migrationID)

	migration, err := e.migrationRepo.GetByID(migrationID)
	if err != nil {
		log.Printf("Migration %s failed: get migration failed: %v", migrationID, err)
		e.migrationRepo.FailMigration(migrationID, fmt.Sprintf("get migration failed: %v", err), 0, 0)
		return
	}

	var rules []model.MigrationRule
	if err := json.Unmarshal([]byte(migration.MigrationRules), &rules); err != nil {
		log.Printf("Migration %s failed: parse rules failed: %v", migrationID, err)
		e.migrationRepo.FailMigration(migrationID, fmt.Sprintf("parse rules failed: %v", err), 0, 0)
		return
	}

	events, err := e.getEventsForMigration(migration.TenantID, migration.EventType, migration.SourceVersion, migration.CreatedAt)
	if err != nil {
		log.Printf("Migration %s failed: get events failed: %v", migrationID, err)
		e.migrationRepo.FailMigration(migrationID, fmt.Sprintf("get events failed: %v", err), 0, 0)
		return
	}

	migration.TotalEvents = len(events)
	e.migrationRepo.StartMigration(migrationID)

	for i, event := range events {
		select {
		case <-ctx.Done():
			log.Printf("Migration %s cancelled", migrationID)
			e.migrationRepo.UpdateStatus(migrationID, model.MigrationStatusCancelled)
			return
		default:
		}

		backup := &model.MigrationEventBackup{
			MigrationID:          migrationID,
			EventID:              event.ID,
			OriginalPayload:      event.Payload,
			OriginalSchemaVersion: event.SchemaVersion,
			Status:               "pending",
		}
		if err := e.migrationRepo.CreateBackup(backup); err != nil {
			log.Printf("Migration %s: create backup for event %s failed: %v", migrationID, event.ID, err)
			continue
		}

		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
			log.Printf("Migration %s: parse event %s payload failed: %v", migrationID, event.ID, err)
			e.migrationRepo.UpdateBackupFailed(backup.ID, fmt.Sprintf("parse payload failed: %v", err))
			migration.FailedEvents++
			e.migrationRepo.UpdateProgress(migrationID, migration.ProcessedEvents, migration.FailedEvents)
			continue
		}

		converted, err := e.engine.ApplyRules(payload, rules)
		if err != nil {
			log.Printf("Migration %s: apply rules to event %s failed: %v", migrationID, event.ID, err)
			e.migrationRepo.UpdateBackupFailed(backup.ID, err.Error())
			migration.FailedEvents++
			e.migrationRepo.UpdateProgress(migrationID, migration.ProcessedEvents, migration.FailedEvents)
			continue
		}

		convertedJSON, _ := json.Marshal(converted)

		tx, err := e.migrationRepo.DB().Begin()
		if err != nil {
			log.Printf("Migration %s: begin tx failed: %v", migrationID, err)
			e.migrationRepo.UpdateBackupFailed(backup.ID, err.Error())
			migration.FailedEvents++
			e.migrationRepo.UpdateProgress(migrationID, migration.ProcessedEvents, migration.FailedEvents)
			continue
		}

		if _, err := tx.Exec(`UPDATE events SET payload=$1, schema_version=$2 WHERE id=$3`,
			string(convertedJSON), migration.TargetVersion, event.ID); err != nil {
			tx.Rollback()
			log.Printf("Migration %s: update event %s failed: %v", migrationID, event.ID, err)
			e.migrationRepo.UpdateBackupFailed(backup.ID, err.Error())
			migration.FailedEvents++
			e.migrationRepo.UpdateProgress(migrationID, migration.ProcessedEvents, migration.FailedEvents)
			continue
		}

		if _, err := tx.Exec(`UPDATE migration_event_backups SET new_payload=$1, new_schema_version=$2, status='success', updated_at=NOW() WHERE id=$3`,
			string(convertedJSON), migration.TargetVersion, backup.ID); err != nil {
			tx.Rollback()
			log.Printf("Migration %s: update backup %s failed: %v", migrationID, backup.ID, err)
			migration.FailedEvents++
			e.migrationRepo.UpdateProgress(migrationID, migration.ProcessedEvents, migration.FailedEvents)
			continue
		}

		if err := tx.Commit(); err != nil {
			log.Printf("Migration %s: commit tx failed: %v", migrationID, err)
			e.migrationRepo.UpdateBackupFailed(backup.ID, err.Error())
			migration.FailedEvents++
			e.migrationRepo.UpdateProgress(migrationID, migration.ProcessedEvents, migration.FailedEvents)
			continue
		}

		migration.ProcessedEvents++

		if (i+1)%10 == 0 {
			e.migrationRepo.UpdateProgress(migrationID, migration.ProcessedEvents, migration.FailedEvents)
		}

		time.Sleep(1 * time.Millisecond)
	}

	e.migrationRepo.CompleteMigration(migrationID, migration.ProcessedEvents, migration.FailedEvents)
	log.Printf("Migration %s completed: processed=%d, failed=%d", migrationID, migration.ProcessedEvents, migration.FailedEvents)
}

func (e *MigrationExecutor) executeRollback(ctx context.Context, migrationID string) {
	defer e.cleanupTask(migrationID)

	migration, err := e.migrationRepo.GetByID(migrationID)
	if err != nil {
		log.Printf("Rollback %s failed: get migration failed: %v", migrationID, err)
		e.migrationRepo.FailRollback(migrationID, fmt.Sprintf("get migration failed: %v", err), 0, 0)
		return
	}

	if err := e.migrationRepo.StartRollback(migrationID); err != nil {
		log.Printf("Rollback %s failed: start rollback failed: %v", migrationID, err)
		e.migrationRepo.FailRollback(migrationID, fmt.Sprintf("start rollback failed: %v", err), 0, 0)
		return
	}

	processed := 0
	failed := 0
	batchSize := 100

	for {
		select {
		case <-ctx.Done():
			log.Printf("Rollback %s cancelled", migrationID)
			e.migrationRepo.UpdateStatus(migrationID, model.MigrationStatusRollbackFailed)
			return
		default:
		}

		backups, err := e.migrationRepo.GetSuccessBackups(migrationID, batchSize)
		if err != nil {
			log.Printf("Rollback %s failed: get backups failed: %v", migrationID, err)
			e.migrationRepo.FailRollback(migrationID, fmt.Sprintf("get backups failed: %v", err), processed, failed)
			return
		}

		if len(backups) == 0 {
			break
		}

		for _, backup := range backups {
			tx, err := e.migrationRepo.DB().Begin()
			if err != nil {
				log.Printf("Rollback %s: begin tx failed: %v", migrationID, err)
				failed++
				continue
			}

			if _, err := tx.Exec(`UPDATE events SET payload=$1, schema_version=$2 WHERE id=$3`,
				backup.OriginalPayload, backup.OriginalSchemaVersion, backup.EventID); err != nil {
				tx.Rollback()
				log.Printf("Rollback %s: update event %s failed: %v", migrationID, backup.EventID, err)
				failed++
				continue
			}

			if _, err := tx.Exec(`UPDATE migration_event_backups SET status='rollbacked', updated_at=NOW() WHERE id=$1`,
				backup.ID); err != nil {
				tx.Rollback()
				log.Printf("Rollback %s: update backup %s failed: %v", migrationID, backup.ID, err)
				failed++
				continue
			}

			if err := tx.Commit(); err != nil {
				log.Printf("Rollback %s: commit tx failed: %v", migrationID, err)
				failed++
				continue
			}

			processed++
		}

		migration.ProcessedEvents = processed
		migration.FailedEvents = failed
		e.migrationRepo.UpdateProgress(migrationID, processed, failed)
	}

	e.migrationRepo.CompleteRollback(migrationID, processed, failed)
	log.Printf("Rollback %s completed: processed=%d, failed=%d", migrationID, processed, failed)
}

func (e *MigrationExecutor) getEventsForMigration(tenantID, eventType string, sourceVersion int, createdBefore time.Time) ([]*model.Event, error) {
	rows, err := e.eventRepo.DB().Query(
		`SELECT id, tenant_id, event_type, schema_version, payload, idempotent_key, created_at
		 FROM events WHERE tenant_id=$1 AND event_type=$2 AND schema_version=$3 AND created_at <= $4
		 ORDER BY created_at ASC`,
		tenantID, eventType, sourceVersion, createdBefore,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*model.Event
	for rows.Next() {
		var e model.Event
		var idempotentKey sql.NullString
		err := rows.Scan(&e.ID, &e.TenantID, &e.EventType, &e.SchemaVersion, &e.Payload, &idempotentKey, &e.CreatedAt)
		if err != nil {
			return nil, err
		}
		e.IdempotentKey = idempotentKey.String
		events = append(events, &e)
	}
	return events, nil
}

func (e *MigrationExecutor) cleanupTask(migrationID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.runningTasks, migrationID)
}

func (e *MigrationExecutor) GetProgress(migrationID string) (*model.MigrationProgress, error) {
	migration, err := e.migrationRepo.GetByID(migrationID)
	if err != nil {
		return nil, err
	}

	progress := &model.MigrationProgress{
		MigrationID:     migration.ID,
		Status:          string(migration.Status),
		TotalEvents:     migration.TotalEvents,
		ProcessedEvents: migration.ProcessedEvents,
		FailedEvents:    migration.FailedEvents,
		ErrorMessage:    migration.ErrorMessage,
	}

	if migration.TotalEvents > 0 {
		progress.ProgressPercent = float64(migration.ProcessedEvents) / float64(migration.TotalEvents) * 100
	}

	if migration.StartedAt != nil && migration.Status == model.MigrationStatusRunning && progress.ProgressPercent > 0 {
		elapsed := time.Since(*migration.StartedAt).Seconds()
		remaining := (elapsed / progress.ProgressPercent) * (100 - progress.ProgressPercent)
		progress.EstimatedRemainingSeconds = remaining
	}

	return progress, nil
}

func (e *MigrationExecutor) IsRunning(migrationID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	_, exists := e.runningTasks[migrationID]
	return exists
}
