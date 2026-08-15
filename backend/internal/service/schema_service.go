package service

import (
	"encoding/json"
	"fmt"

	"github.com/eventbus/server/internal/model"
	"github.com/eventbus/server/internal/repository"
)

type SchemaService struct {
	schemaRepo *repository.SchemaRepo
}

func NewSchemaService(schemaRepo *repository.SchemaRepo) *SchemaService {
	return &SchemaService{schemaRepo: schemaRepo}
}

func (s *SchemaService) Register(tenantID string, req map[string]interface{}) (*model.EventSchema, error) {
	eventType, _ := req["event_type"].(string)
	schemaDef, _ := req["schema_def"].(string)

	if eventType == "" || schemaDef == "" {
		return nil, fmt.Errorf("event_type and schema_def are required")
	}

	var def map[string]interface{}
	if err := json.Unmarshal([]byte(schemaDef), &def); err != nil {
		return nil, fmt.Errorf("invalid JSON Schema: %w", err)
	}

	version, err := s.schemaRepo.NextVersion(tenantID, eventType)
	if err != nil {
		return nil, err
	}

	isCompatible := true
	compatibilityNote := ""

	if version > 1 {
		prev, err := s.schemaRepo.GetPreviousVersion(tenantID, eventType, version)
		if err == nil && prev != nil {
			isCompatible, compatibilityNote = s.schemaRepo.CheckForwardCompatibility(prev.SchemaDef, schemaDef)
		}
	}

	schema := &model.EventSchema{
		TenantID:          tenantID,
		EventType:         eventType,
		Version:           version,
		SchemaDef:         schemaDef,
		IsCompatible:      isCompatible,
		CompatibilityNote: compatibilityNote,
	}

	if err := s.schemaRepo.Create(schema); err != nil {
		return nil, err
	}
	return schema, nil
}

func (s *SchemaService) Get(id string) (*model.EventSchema, error) {
	return s.schemaRepo.GetByID(id)
}

func (s *SchemaService) GetByVersion(tenantID, eventType string, version int) (*model.EventSchema, error) {
	return s.schemaRepo.GetByVersion(tenantID, eventType, version)
}

func (s *SchemaService) GetLatest(tenantID, eventType string) (*model.EventSchema, error) {
	return s.schemaRepo.GetLatestVersion(tenantID, eventType)
}

func (s *SchemaService) ListVersions(tenantID, eventType string) ([]*model.EventSchema, error) {
	return s.schemaRepo.ListVersions(tenantID, eventType)
}

func (s *SchemaService) ListByTenant(tenantID string) ([]*model.EventSchema, error) {
	return s.schemaRepo.ListByTenant(tenantID)
}

func (s *SchemaService) CheckCompatibility(tenantID, eventType, newSchema string) (bool, string, error) {
	latest, err := s.schemaRepo.GetLatestVersion(tenantID, eventType)
	if err != nil {
		return true, "no previous version", nil
	}
	isCompatible, note := s.schemaRepo.CheckForwardCompatibility(latest.SchemaDef, newSchema)
	return isCompatible, note, nil
}

func (s *SchemaService) Diff(tenantID, eventType string, v1, v2 int) (map[string]interface{}, error) {
	s1, err := s.schemaRepo.GetByVersion(tenantID, eventType, v1)
	if err != nil {
		return nil, fmt.Errorf("version %d not found: %w", v1, err)
	}
	s2, err := s.schemaRepo.GetByVersion(tenantID, eventType, v2)
	if err != nil {
		return nil, fmt.Errorf("version %d not found: %w", v2, err)
	}

	return map[string]interface{}{
		"version_1": map[string]interface{}{
			"version":    s1.Version,
			"schema_def": s1.SchemaDef,
		},
		"version_2": map[string]interface{}{
			"version":    s2.Version,
			"schema_def": s2.SchemaDef,
		},
	}, nil
}
