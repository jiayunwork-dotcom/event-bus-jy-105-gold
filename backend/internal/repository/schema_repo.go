package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/eventbus/server/internal/model"
	"github.com/google/uuid"
)

type SchemaRepo struct {
	db *sql.DB
}

func NewSchemaRepo(db *sql.DB) *SchemaRepo {
	return &SchemaRepo{db: db}
}

func (r *SchemaRepo) Create(s *model.EventSchema) error {
	s.ID = uuid.New().String()
	row := r.db.QueryRow(
		`INSERT INTO event_schemas (id, tenant_id, event_type, version, schema_def, is_compatible, compatibility_note)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING created_at`,
		s.ID, s.TenantID, s.EventType, s.Version, s.SchemaDef, s.IsCompatible, s.CompatibilityNote,
	)
	return row.Scan(&s.CreatedAt)
}

func (r *SchemaRepo) GetByID(id string) (*model.EventSchema, error) {
	row := r.db.QueryRow(
		`SELECT id, tenant_id, event_type, version, schema_def, is_compatible, compatibility_note, created_at
		 FROM event_schemas WHERE id=$1`, id,
	)
	var s model.EventSchema
	err := row.Scan(&s.ID, &s.TenantID, &s.EventType, &s.Version, &s.SchemaDef, &s.IsCompatible, &s.CompatibilityNote, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SchemaRepo) GetLatestVersion(tenantID, eventType string) (*model.EventSchema, error) {
	row := r.db.QueryRow(
		`SELECT id, tenant_id, event_type, version, schema_def, is_compatible, compatibility_note, created_at
		 FROM event_schemas WHERE tenant_id=$1 AND event_type=$2 AND is_compatible=true
		 ORDER BY version DESC LIMIT 1`, tenantID, eventType,
	)
	var s model.EventSchema
	err := row.Scan(&s.ID, &s.TenantID, &s.EventType, &s.Version, &s.SchemaDef, &s.IsCompatible, &s.CompatibilityNote, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SchemaRepo) GetByVersion(tenantID, eventType string, version int) (*model.EventSchema, error) {
	row := r.db.QueryRow(
		`SELECT id, tenant_id, event_type, version, schema_def, is_compatible, compatibility_note, created_at
		 FROM event_schemas WHERE tenant_id=$1 AND event_type=$2 AND version=$3`, tenantID, eventType, version,
	)
	var s model.EventSchema
	err := row.Scan(&s.ID, &s.TenantID, &s.EventType, &s.Version, &s.SchemaDef, &s.IsCompatible, &s.CompatibilityNote, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SchemaRepo) ListVersions(tenantID, eventType string) ([]*model.EventSchema, error) {
	rows, err := r.db.Query(
		`SELECT id, tenant_id, event_type, version, schema_def, is_compatible, compatibility_note, created_at
		 FROM event_schemas WHERE tenant_id=$1 AND event_type=$2 ORDER BY version DESC`, tenantID, eventType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schemas []*model.EventSchema
	for rows.Next() {
		var s model.EventSchema
		if err := rows.Scan(&s.ID, &s.TenantID, &s.EventType, &s.Version, &s.SchemaDef, &s.IsCompatible, &s.CompatibilityNote, &s.CreatedAt); err != nil {
			return nil, err
		}
		schemas = append(schemas, &s)
	}
	return schemas, nil
}

func (r *SchemaRepo) NextVersion(tenantID, eventType string) (int, error) {
	var maxVer sql.NullInt64
	err := r.db.QueryRow(
		`SELECT MAX(version) FROM event_schemas WHERE tenant_id=$1 AND event_type=$2`, tenantID, eventType,
	).Scan(&maxVer)
	if err != nil {
		return 0, err
	}
	if !maxVer.Valid {
		return 1, nil
	}
	return int(maxVer.Int64) + 1, nil
}

func (r *SchemaRepo) ListByTenant(tenantID string) ([]*model.EventSchema, error) {
	rows, err := r.db.Query(
		`SELECT DISTINCT ON (event_type) id, tenant_id, event_type, version, schema_def, is_compatible, compatibility_note, created_at
		 FROM event_schemas WHERE tenant_id=$1 ORDER BY event_type, version DESC`, tenantID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schemas []*model.EventSchema
	for rows.Next() {
		var s model.EventSchema
		if err := rows.Scan(&s.ID, &s.TenantID, &s.EventType, &s.Version, &s.SchemaDef, &s.IsCompatible, &s.CompatibilityNote, &s.CreatedAt); err != nil {
			return nil, err
		}
		schemas = append(schemas, &s)
	}
	return schemas, nil
}

func (r *SchemaRepo) GetPreviousVersion(tenantID, eventType string, currentVersion int) (*model.EventSchema, error) {
	row := r.db.QueryRow(
		`SELECT id, tenant_id, event_type, version, schema_def, is_compatible, compatibility_note, created_at
		 FROM event_schemas WHERE tenant_id=$1 AND event_type=$2 AND version < $3
		 ORDER BY version DESC LIMIT 1`, tenantID, eventType, currentVersion,
	)
	var s model.EventSchema
	err := row.Scan(&s.ID, &s.TenantID, &s.EventType, &s.Version, &s.SchemaDef, &s.IsCompatible, &s.CompatibilityNote, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SchemaRepo) CheckForwardCompatibility(oldSchema, newSchema string) (bool, string) {
	var oldDef, newDef map[string]interface{}
	if err := json.Unmarshal([]byte(oldSchema), &oldDef); err != nil {
		return false, fmt.Sprintf("invalid old schema: %v", err)
	}
	if err := json.Unmarshal([]byte(newSchema), &newDef); err != nil {
		return false, fmt.Sprintf("invalid new schema: %v", err)
	}

	oldRequired, _ := oldDef["required"].([]interface{})
	newRequired, _ := newDef["required"].([]interface{})

	oldReqSet := make(map[string]bool)
	for _, r := range oldRequired {
		oldReqSet[fmt.Sprintf("%v", r)] = true
	}
	for _, r := range newRequired {
		name := fmt.Sprintf("%v", r)
		if !oldReqSet[name] {
			return false, fmt.Sprintf("new required field '%s' breaks forward compatibility", name)
		}
	}

	oldProps, _ := oldDef["properties"].(map[string]interface{})
	newProps, _ := newDef["properties"].(map[string]interface{})

	for name := range oldProps {
		if _, exists := newProps[name]; !exists {
			return false, fmt.Sprintf("removed field '%s' breaks forward compatibility", name)
		}
	}

	return true, ""
}
