package service

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"

	"github.com/eventbus/server/internal/engine"
	"github.com/eventbus/server/internal/model"
	"github.com/eventbus/server/internal/repository"
)

type MigrationService struct {
	migrationRepo     *repository.MigrationRepo
	eventRepo         *repository.EventRepo
	schemaRepo        *repository.SchemaRepo
	migrationEngine   *engine.MigrationEngine
	migrationExecutor *engine.MigrationExecutor
	subscriptionRepo  *repository.SubscriptionRepo
	orchestrationRepo *repository.OrchestrationRepo
}

func NewMigrationService(
	migrationRepo *repository.MigrationRepo,
	eventRepo *repository.EventRepo,
	schemaRepo *repository.SchemaRepo,
	migrationEngine *engine.MigrationEngine,
	migrationExecutor *engine.MigrationExecutor,
	subscriptionRepo *repository.SubscriptionRepo,
	orchestrationRepo *repository.OrchestrationRepo,
) *MigrationService {
	return &MigrationService{
		migrationRepo:     migrationRepo,
		eventRepo:         eventRepo,
		schemaRepo:        schemaRepo,
		migrationEngine:   migrationEngine,
		migrationExecutor: migrationExecutor,
		subscriptionRepo:  subscriptionRepo,
		orchestrationRepo: orchestrationRepo,
	}
}

func (s *MigrationService) ValidateRules(rules []model.MigrationRule) []model.MigrationRuleValidationError {
	return s.migrationEngine.ValidateRules(rules)
}

func (s *MigrationService) Preview(tenantID string, req *model.MigrationPreviewRequest) ([]*model.MigrationPreviewResult, error) {
	if len(req.MigrationRules) == 0 {
		return nil, fmt.Errorf("至少需要配置一条迁移规则")
	}

	validationErrors := s.migrationEngine.ValidateRules(req.MigrationRules)
	if len(validationErrors) > 0 {
		return nil, fmt.Errorf("invalid migration rules: %+v", validationErrors)
	}

	if _, err := s.schemaRepo.GetByVersion(tenantID, req.EventType, req.SourceVersion); err != nil {
		return nil, fmt.Errorf("source version %d not found: %w", req.SourceVersion, err)
	}

	if _, err := s.schemaRepo.GetByVersion(tenantID, req.EventType, req.TargetVersion); err != nil {
		return nil, fmt.Errorf("target version %d not found: %w", req.TargetVersion, err)
	}

	events, err := s.getRandomEvents(tenantID, req.EventType, req.SourceVersion, 10)
	if err != nil {
		return nil, fmt.Errorf("get events failed: %w", err)
	}

	var results []*model.MigrationPreviewResult
	for _, event := range events {
		result := &model.MigrationPreviewResult{
			EventID: event.ID,
		}

		var originalPayload map[string]interface{}
		if err := json.Unmarshal([]byte(event.Payload), &originalPayload); err != nil {
			result.Success = false
			result.ErrorMessage = fmt.Sprintf("parse payload failed: %v", err)
			results = append(results, result)
			continue
		}

		result.OriginalPayload = originalPayload

		converted, err := s.migrationEngine.ApplyRules(originalPayload, req.MigrationRules)
		if err != nil {
			result.Success = false
			result.ErrorMessage = err.Error()
			results = append(results, result)
			continue
		}

		result.ConvertedPayload = converted
		result.Success = true
		results = append(results, result)
	}

	return results, nil
}

func (s *MigrationService) StartMigration(tenantID string, req *model.MigrationStartRequest) (*model.SchemaMigration, error) {
	if len(req.MigrationRules) == 0 {
		return nil, fmt.Errorf("至少需要配置一条迁移规则")
	}

	validationErrors := s.migrationEngine.ValidateRules(req.MigrationRules)
	if len(validationErrors) > 0 {
		return nil, fmt.Errorf("invalid migration rules: %+v", validationErrors)
	}

	running, err := s.migrationRepo.CheckRunningMigration(tenantID, req.EventType)
	if err != nil {
		return nil, fmt.Errorf("check running migration failed: %w", err)
	}
	if running {
		return nil, fmt.Errorf("a migration is already running for this event type")
	}

	if _, err := s.schemaRepo.GetByVersion(tenantID, req.EventType, req.SourceVersion); err != nil {
		return nil, fmt.Errorf("source version %d not found: %w", req.SourceVersion, err)
	}

	if _, err := s.schemaRepo.GetByVersion(tenantID, req.EventType, req.TargetVersion); err != nil {
		return nil, fmt.Errorf("target version %d not found: %w", req.TargetVersion, err)
	}

	if req.SourceVersion >= req.TargetVersion {
		return nil, fmt.Errorf("target version must be greater than source version")
	}

	rulesJSON, err := json.Marshal(req.MigrationRules)
	if err != nil {
		return nil, fmt.Errorf("marshal rules failed: %w", err)
	}

	migration := &model.SchemaMigration{
		TenantID:       tenantID,
		EventType:      req.EventType,
		SourceVersion:  req.SourceVersion,
		TargetVersion:  req.TargetVersion,
		MigrationRules: string(rulesJSON),
	}

	if err := s.migrationRepo.Create(migration); err != nil {
		return nil, fmt.Errorf("create migration failed: %w", err)
	}

	if err := s.migrationExecutor.StartMigration(migration.ID); err != nil {
		return nil, fmt.Errorf("start migration failed: %w", err)
	}

	return migration, nil
}

func (s *MigrationService) GetProgress(tenantID, migrationID string) (*model.MigrationProgress, error) {
	migration, err := s.migrationRepo.GetByID(migrationID)
	if err != nil {
		return nil, fmt.Errorf("migration not found: %w", err)
	}

	if migration.TenantID != tenantID {
		return nil, fmt.Errorf("migration not found")
	}

	return s.migrationExecutor.GetProgress(migrationID)
}

func (s *MigrationService) CancelMigration(tenantID, migrationID string) error {
	migration, err := s.migrationRepo.GetByID(migrationID)
	if err != nil {
		return fmt.Errorf("migration not found: %w", err)
	}

	if migration.TenantID != tenantID {
		return fmt.Errorf("migration not found")
	}

	return s.migrationExecutor.CancelMigration(migrationID)
}

func (s *MigrationService) RollbackMigration(tenantID, migrationID string) error {
	migration, err := s.migrationRepo.GetByID(migrationID)
	if err != nil {
		return fmt.Errorf("migration not found: %w", err)
	}

	if migration.TenantID != tenantID {
		return fmt.Errorf("migration not found")
	}

	return s.migrationExecutor.StartRollback(migrationID)
}

func (s *MigrationService) ListMigrations(tenantID, eventType string) ([]*model.SchemaMigration, error) {
	return s.migrationRepo.ListByEventType(tenantID, eventType)
}

func (s *MigrationService) GetMigration(tenantID, migrationID string) (*model.SchemaMigration, error) {
	migration, err := s.migrationRepo.GetByID(migrationID)
	if err != nil {
		return nil, fmt.Errorf("migration not found: %w", err)
	}

	if migration.TenantID != tenantID {
		return nil, fmt.Errorf("migration not found")
	}

	return migration, nil
}

func (s *MigrationService) getRandomEvents(tenantID, eventType string, sourceVersion int, count int) ([]*model.Event, error) {
	rows, err := s.eventRepo.DB().Query(
		`SELECT id, tenant_id, event_type, schema_version, payload, idempotent_key, created_at
		 FROM events WHERE tenant_id=$1 AND event_type=$2 AND schema_version=$3
		 ORDER BY RANDOM() LIMIT $4`,
		tenantID, eventType, sourceVersion, count,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*model.Event
	for rows.Next() {
		var e model.Event
		var idempotentKey *string
		err := rows.Scan(&e.ID, &e.TenantID, &e.EventType, &e.SchemaVersion, &e.Payload, &idempotentKey, &e.CreatedAt)
		if err != nil {
			return nil, err
		}
		if idempotentKey != nil {
			e.IdempotentKey = *idempotentKey
		}
		events = append(events, &e)
	}

	return events, nil
}

func (s *MigrationService) AnalyzeImpact(tenantID, migrationID string) (*model.MigrationImpactReport, error) {
	migration, err := s.migrationRepo.GetByID(migrationID)
	if err != nil {
		return nil, fmt.Errorf("migration not found: %w", err)
	}
	if migration.TenantID != tenantID {
		return nil, fmt.Errorf("migration not found")
	}

	var rules []model.MigrationRule
	if err := json.Unmarshal([]byte(migration.MigrationRules), &rules); err != nil {
		return nil, fmt.Errorf("invalid migration rules: %w", err)
	}

	affectedFields := s.extractAffectedFields(rules)
	if len(affectedFields) == 0 {
		return &model.MigrationImpactReport{
			MigrationID:           migrationID,
			AffectedFields:        []string{},
			AffectedSubscriptions: []model.AffectedSubscription{},
			AffectedDAGNodes:      []model.AffectedDAGNode{},
			Suggestions:           []string{},
			HasImpact:             false,
		}, nil
	}

	affectedSubs := s.findAffectedSubscriptions(tenantID, migration.EventType, affectedFields)
	affectedDAGNodes := s.findAffectedDAGNodes(tenantID, migration.EventType, affectedFields)

	var suggestions []string
	for _, sub := range affectedSubs {
		suggestions = append(suggestions, fmt.Sprintf(
			"订阅 [%s] 的过滤表达式引用了被修改的字段 %v，建议更新过滤表达式",
			sub.Name, sub.MatchedFields,
		))
	}
	for _, node := range affectedDAGNodes {
		suggestions = append(suggestions, fmt.Sprintf(
			"编排DAG节点 [%s] (订阅: %s) 的配置引用了被修改的字段 %v，建议更新编排配置",
			node.NodeName, node.SubscriptionName, node.MatchedFields,
		))
	}

	hasImpact := len(affectedSubs) > 0 || len(affectedDAGNodes) > 0

	return &model.MigrationImpactReport{
		MigrationID:           migrationID,
		AffectedFields:        affectedFields,
		AffectedSubscriptions: affectedSubs,
		AffectedDAGNodes:      affectedDAGNodes,
		Suggestions:           suggestions,
		HasImpact:             hasImpact,
	}, nil
}

func (s *MigrationService) extractAffectedFields(rules []model.MigrationRule) []string {
	fieldSet := make(map[string]bool)
	for _, rule := range rules {
		switch rule.Type {
		case model.RuleTypeRename, model.RuleTypeDelete, model.RuleTypeConvert:
			if rule.SourcePath != "" {
				fieldSet[rule.SourcePath] = true
			}
		case model.RuleTypeMapPath:
			if rule.SourcePath != "" {
				fieldSet[rule.SourcePath] = true
			}
		}
	}
	fields := make([]string, 0, len(fieldSet))
	for f := range fieldSet {
		fields = append(fields, f)
	}
	return fields
}

func (s *MigrationService) findAffectedSubscriptions(tenantID, eventType string, affectedFields []string) []model.AffectedSubscription {
	subs, err := s.subscriptionRepo.ListByTenantAndEventType(tenantID, eventType)
	if err != nil {
		return nil
	}

	var result []model.AffectedSubscription
	for _, sub := range subs {
		if sub.FilterExpression == "" {
			continue
		}
		matched := matchFieldsInText(sub.FilterExpression, affectedFields)
		if len(matched) > 0 {
			result = append(result, model.AffectedSubscription{
				ID:               sub.ID,
				Name:             sub.Name,
				EventType:        sub.EventType,
				FilterExpression: sub.FilterExpression,
				MatchedFields:    matched,
			})
		}
	}
	return result
}

func (s *MigrationService) findAffectedDAGNodes(tenantID, eventType string, affectedFields []string) []model.AffectedDAGNode {
	dags, err := s.orchestrationRepo.ListByTenant(tenantID)
	if err != nil {
		return nil
	}

	subMap := make(map[string]*model.Subscription)
	allSubs, _ := s.subscriptionRepo.ListByTenant(tenantID)
	for _, sub := range allSubs {
		subMap[sub.ID] = sub
	}

	var result []model.AffectedDAGNode
	for _, dag := range dags {
		sub, ok := subMap[dag.SubscriptionID]
		if !ok || sub.EventType != eventType {
			continue
		}
		for _, node := range dag.Nodes {
			nodeText := serializeNodeConfig(node.Config)
			matched := matchFieldsInText(nodeText, affectedFields)
			if len(matched) > 0 {
				subName := ""
				if sub != nil {
					subName = sub.Name
				}
				result = append(result, model.AffectedDAGNode{
					SubscriptionID:   dag.SubscriptionID,
					SubscriptionName: subName,
					DAGID:            dag.ID,
					NodeID:           node.ID,
					NodeName:         node.Name,
					NodeType:         node.Type,
					Config:           node.Config,
					MatchedFields:    matched,
				})
			}
		}
	}
	return result
}

func matchFieldsInText(text string, fields []string) []string {
	var matched []string
	for _, field := range fields {
		if strings.Contains(text, field) {
			matched = append(matched, field)
		}
	}
	return matched
}

func serializeNodeConfig(config map[string]interface{}) string {
	if config == nil {
		return ""
	}
	b, _ := json.Marshal(config)
	return string(b)
}

func (s *MigrationService) generateSampleEvents(sourceVersion int, count int) ([]*model.Event, error) {
	var events []*model.Event

	samplePayloads := []map[string]interface{}{
		{
			"order_id":   "ORD-" + fmt.Sprintf("%06d", rand.Intn(1000000)),
			"amount":     float64(rand.Intn(10000)) / 100,
			"status":     "created",
			"user_id":    "USR-" + fmt.Sprintf("%04d", rand.Intn(10000)),
			"created_at": "2024-01-15T10:30:00Z",
		},
		{
			"order_id":   "ORD-" + fmt.Sprintf("%06d", rand.Intn(1000000)),
			"amount":     float64(rand.Intn(10000)) / 100,
			"status":     "paid",
			"user_id":    "USR-" + fmt.Sprintf("%04d", rand.Intn(10000)),
			"created_at": "2024-01-15T11:00:00Z",
		},
	}

	for i := 0; i < count && i < len(samplePayloads); i++ {
		payloadJSON, _ := json.Marshal(samplePayloads[i])
		event := &model.Event{
			ID:            "sample-" + fmt.Sprintf("%d", i+1),
			SchemaVersion: sourceVersion,
			Payload:       string(payloadJSON),
		}
		events = append(events, event)
	}

	return events, nil
}
