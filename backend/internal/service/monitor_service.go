package service

import (
	"encoding/json"
	"fmt"

	"github.com/eventbus/server/internal/engine"
	"github.com/eventbus/server/internal/model"
	"github.com/eventbus/server/internal/repository"
)

type DeadLetterService struct {
	deadLetterRepo *repository.DeadLetterRepo
	eventRepo      *repository.EventRepo
}

func NewDeadLetterService(deadLetterRepo *repository.DeadLetterRepo, eventRepo *repository.EventRepo) *DeadLetterService {
	return &DeadLetterService{deadLetterRepo: deadLetterRepo, eventRepo: eventRepo}
}

func (s *DeadLetterService) List(tenantID string, limit, offset int) ([]*model.DeadLetterEntry, error) {
	return s.deadLetterRepo.ListByTenant(tenantID, limit, offset)
}

func (s *DeadLetterService) Get(id string) (*model.DeadLetterEntry, error) {
	return s.deadLetterRepo.GetByID(id)
}

func (s *DeadLetterService) Retry(id string) error {
	return s.deadLetterRepo.Retry(id)
}

func (s *DeadLetterService) BatchRetry(ids []string) error {
	return s.deadLetterRepo.BatchRetry(ids)
}

func (s *DeadLetterService) EditAndRetry(id string, newPayload string) error {
	_, err := s.deadLetterRepo.GetByID(id)
	if err != nil {
		return err
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(newPayload), &payload); err != nil {
		return fmt.Errorf("invalid JSON payload: %w", err)
	}

	if err := s.deadLetterRepo.UpdatePayload(id, newPayload); err != nil {
		return err
	}

	return s.deadLetterRepo.Retry(id)
}

func (s *DeadLetterService) Count(tenantID string) (int, error) {
	return s.deadLetterRepo.CountByTenant(tenantID)
}

type ReplayService struct {
	replayEngine *engine.ReplayEngine
}

func NewReplayService(replayEngine *engine.ReplayEngine) *ReplayService {
	return &ReplayService{replayEngine: replayEngine}
}

func (s *ReplayService) StartReplay(req model.ReplayRequest, tenantID string) (*engine.ReplayTask, error) {
	return s.replayEngine.StartReplay(req, tenantID)
}

type MonitorService struct {
	subscriptionRepo *repository.SubscriptionRepo
	eventRepo        *repository.EventRepo
	deadLetterRepo   *repository.DeadLetterRepo
	traceRepo        *repository.TraceRepo
	alertRepo        *repository.AlertRepo
}

func NewMonitorService(
	subscriptionRepo *repository.SubscriptionRepo,
	eventRepo *repository.EventRepo,
	deadLetterRepo *repository.DeadLetterRepo,
	traceRepo *repository.TraceRepo,
	alertRepo *repository.AlertRepo,
) *MonitorService {
	return &MonitorService{
		subscriptionRepo: subscriptionRepo,
		eventRepo:        eventRepo,
		deadLetterRepo:   deadLetterRepo,
		traceRepo:        traceRepo,
		alertRepo:        alertRepo,
	}
}

func (s *MonitorService) GetDashboard(tenantID string) (map[string]interface{}, error) {
	backlogCounts, _ := s.subscriptionRepo.GetBacklogCountsByTenant(tenantID)
	dlqCount, _ := s.deadLetterRepo.CountByTenant(tenantID)
	latencyStats, _ := s.subscriptionRepo.GetDeliveryLatencyStats(tenantID)
	qpsHistory, _ := s.eventRepo.GetQPSHistory(tenantID, 60)
	deliveryStats, _ := s.subscriptionRepo.GetDeliveryStats(tenantID, 60)

	subs, _ := s.subscriptionRepo.GetAllActiveForTenant(tenantID)
	consumerStatuses := make(map[string]bool)
	for _, sub := range subs {
		online, _ := s.subscriptionRepo.GetOnlineStatus(sub.ID)
		consumerStatuses[sub.ID] = online
	}

	return map[string]interface{}{
		"publish_qps_history": qpsHistory,
		"delivery_latency":    latencyStats,
		"backlog_counts":      backlogCounts,
		"dlq_depth":           dlqCount,
		"consumer_statuses":   consumerStatuses,
		"delivery_stats":      deliveryStats,
	}, nil
}

func (s *MonitorService) GetEventTrace(eventID string) ([]*model.DeliveryTrace, error) {
	return s.traceRepo.GetByEventID(eventID)
}

func (s *MonitorService) GetEventTraceBySubscription(eventID, subscriptionID string) ([]*model.DeliveryTrace, error) {
	return s.traceRepo.GetByEventAndSubscription(eventID, subscriptionID)
}

func (s *MonitorService) GetEventsByTypeAndTimeRange(tenantID string, eventType string, startTime, endTime string, limit int) ([]*model.Event, error) {
	return s.eventRepo.GetByEventTypeAndTimeRange(tenantID, eventType, startTime, endTime, limit)
}

func (s *MonitorService) GetHeatmapData(tenantID string, minutes int) (map[string]interface{}, error) {
	heatmapData, err := s.eventRepo.GetHeatmapData(tenantID, minutes)
	if err != nil {
		return nil, err
	}
	eventTypes, err := s.eventRepo.GetDistinctEventTypes(tenantID)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"heatmap_data": heatmapData,
		"event_types":  eventTypes,
	}, nil
}

func (s *MonitorService) GetAlerts(tenantID string, resolved bool) ([]*model.BackpressureAlert, error) {
	return s.alertRepo.ListByTenant(tenantID, resolved)
}
