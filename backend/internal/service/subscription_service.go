package service

import (
	"encoding/json"
	"fmt"

	"github.com/eventbus/server/internal/engine"
	"github.com/eventbus/server/internal/model"
	"github.com/eventbus/server/internal/repository"
)

type SubscriptionService struct {
	subscriptionRepo  *repository.SubscriptionRepo
	orchestrationRepo *repository.OrchestrationRepo
	tenantRepo        *repository.TenantRepo
	orchestrator      *engine.Orchestrator
	backpressure      *engine.BackpressureController
}

func NewSubscriptionService(
	subscriptionRepo *repository.SubscriptionRepo,
	orchestrationRepo *repository.OrchestrationRepo,
	tenantRepo *repository.TenantRepo,
	orchestrator *engine.Orchestrator,
	backpressure *engine.BackpressureController,
) *SubscriptionService {
	return &SubscriptionService{
		subscriptionRepo:  subscriptionRepo,
		orchestrationRepo: orchestrationRepo,
		tenantRepo:        tenantRepo,
		orchestrator:      orchestrator,
		backpressure:      backpressure,
	}
}

func (s *SubscriptionService) Create(tenantID string, req map[string]interface{}) (*model.Subscription, error) {
	ok, reason, err := s.tenantRepo.ValidateQuota(tenantID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf(reason)
	}

	sub := &model.Subscription{
		TenantID:                tenantID,
		Name:                    getString(req, "name"),
		EventType:               getString(req, "event_type"),
		FilterExpression:        getString(req, "filter_expression"),
		DeliveryMode:            getWithDefault(req, "delivery_mode", "at_least_once"),
		IdempotentKeyPath:       getString(req, "idempotent_key_path"),
		IdempotentWindowSeconds: getInt(req, "idempotent_window_seconds", 86400),
		MaxRetries:              getInt(req, "max_retries", 5),
		ConsumerURL:             getString(req, "consumer_url"),
		ConsumerRateLimit:       getInt(req, "consumer_rate_limit", 100),
		ConsumerBurst:           getInt(req, "consumer_burst", 200),
		Status:                  "active",
	}

	if sub.Name == "" || sub.EventType == "" || sub.ConsumerURL == "" {
		return nil, fmt.Errorf("name, event_type, and consumer_url are required")
	}

	if sub.DeliveryMode != "at_least_once" && sub.DeliveryMode != "exactly_once" {
		return nil, fmt.Errorf("delivery_mode must be 'at_least_once' or 'exactly_once'")
	}

	if err := s.subscriptionRepo.Create(sub); err != nil {
		return nil, err
	}

	s.backpressure.InitBucket(sub.ID, float64(sub.ConsumerRateLimit), float64(sub.ConsumerBurst))

	if dagRaw, ok := req["dag"]; ok {
		if err := s.saveDAG(sub.ID, dagRaw); err != nil {
			return nil, fmt.Errorf("failed to save DAG: %w", err)
		}
	}

	return sub, nil
}

func (s *SubscriptionService) Get(id string) (*model.Subscription, error) {
	return s.subscriptionRepo.GetByID(id)
}

func (s *SubscriptionService) List(tenantID string) ([]*model.Subscription, error) {
	return s.subscriptionRepo.ListByTenant(tenantID)
}

func (s *SubscriptionService) Update(id string, req map[string]interface{}) (*model.Subscription, error) {
	sub, err := s.subscriptionRepo.GetByID(id)
	if err != nil {
		return nil, err
	}

	if name, ok := req["name"]; ok {
		sub.Name = fmt.Sprintf("%v", name)
	}
	if eventType, ok := req["event_type"]; ok {
		sub.EventType = fmt.Sprintf("%v", eventType)
	}
	if filterExpr, ok := req["filter_expression"]; ok {
		sub.FilterExpression = fmt.Sprintf("%v", filterExpr)
	}
	if deliveryMode, ok := req["delivery_mode"]; ok {
		sub.DeliveryMode = fmt.Sprintf("%v", deliveryMode)
	}
	if consumerURL, ok := req["consumer_url"]; ok {
		sub.ConsumerURL = fmt.Sprintf("%v", consumerURL)
	}
	if v, ok := req["max_retries"]; ok {
		sub.MaxRetries = int(toFloat64(v))
	}
	if v, ok := req["consumer_rate_limit"]; ok {
		sub.ConsumerRateLimit = int(toFloat64(v))
	}
	if v, ok := req["consumer_burst"]; ok {
		sub.ConsumerBurst = int(toFloat64(v))
	}
	if status, ok := req["status"]; ok {
		sub.Status = fmt.Sprintf("%v", status)
	}

	if err := s.subscriptionRepo.Update(sub); err != nil {
		return nil, err
	}

	if _, ok := req["consumer_rate_limit"]; ok {
		s.backpressure.UpdateRate(sub.ID, float64(sub.ConsumerRateLimit), float64(sub.ConsumerBurst))
	}

	return sub, nil
}

func (s *SubscriptionService) Delete(id string) error {
	return s.subscriptionRepo.Delete(id)
}

func (s *SubscriptionService) UpdateRateLimit(id string, rateLimit, burst int) error {
	if err := s.subscriptionRepo.UpdateRateLimit(id, rateLimit, burst); err != nil {
		return err
	}
	s.backpressure.UpdateRate(id, float64(rateLimit), float64(burst))
	return nil
}

func (s *SubscriptionService) SaveDAG(subscriptionID string, dagData map[string]interface{}) error {
	sub, err := s.subscriptionRepo.GetByID(subscriptionID)
	if err != nil {
		return err
	}
	return s.saveDAG(sub.ID, dagData)
}

func (s *SubscriptionService) saveDAG(subscriptionID string, dagRaw interface{}) error {
	dagMap, ok := dagRaw.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid DAG format")
	}

	var nodes []model.DAGNode
	var edges []model.DAGEdge

	if nodesRaw, ok := dagMap["nodes"]; ok {
		b, _ := json.Marshal(nodesRaw)
		json.Unmarshal(b, &nodes)
	}
	if edgesRaw, ok := dagMap["edges"]; ok {
		b, _ := json.Marshal(edgesRaw)
		json.Unmarshal(b, &edges)
	}

	if err := s.orchestrator.ValidateDAG(nodes, edges); err != nil {
		return err
	}

	existing, _ := s.orchestrationRepo.GetBySubscriptionID(subscriptionID)
	if existing != nil {
		existing.Nodes = nodes
		existing.Edges = edges
		return s.orchestrationRepo.Update(existing)
	}

	dag := &model.OrchestrationDAG{
		SubscriptionID: subscriptionID,
		Nodes:          nodes,
		Edges:          edges,
	}
	return s.orchestrationRepo.Create(dag)
}

func (s *SubscriptionService) GetDAG(subscriptionID string) (*model.OrchestrationDAG, error) {
	return s.orchestrationRepo.GetBySubscriptionID(subscriptionID)
}

func getWithDefault(m map[string]interface{}, key, defaultVal string) string {
	if v, ok := m[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return defaultVal
}
