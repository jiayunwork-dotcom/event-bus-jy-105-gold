package engine

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/eventbus/server/internal/model"
	"github.com/eventbus/server/internal/repository"
)

type ReplayEngine struct {
	eventRepo       *repository.EventRepo
	deliveryRepo    *repository.DeliveryRepo
	subscriptionRepo *repository.SubscriptionRepo
	backpressure    *BackpressureController
}

func NewReplayEngine(
	eventRepo *repository.EventRepo,
	deliveryRepo *repository.DeliveryRepo,
	subscriptionRepo *repository.SubscriptionRepo,
	backpressure *BackpressureController,
) *ReplayEngine {
	return &ReplayEngine{
		eventRepo:       eventRepo,
		deliveryRepo:    deliveryRepo,
		subscriptionRepo: subscriptionRepo,
		backpressure:    backpressure,
	}
}

type ReplayTask struct {
	ID             string
	TenantID       string
	SubscriptionID string
	StartTime      string
	EndTime        string
	Offset         int64
	Rate           string
	Status         string
	ProcessedCount int
	TotalCount     int
	CreatedAt      time.Time
}

func (e *ReplayEngine) StartReplay(req model.ReplayRequest, tenantID string) (*ReplayTask, error) {
	sub, err := e.subscriptionRepo.GetByID(req.SubscriptionID)
	if err != nil {
		return nil, fmt.Errorf("subscription not found: %w", err)
	}
	if sub.TenantID != tenantID {
		return nil, fmt.Errorf("subscription does not belong to tenant")
	}

	var events []*model.Event
	if req.StartTime != nil && req.EndTime != nil {
		events, err = e.eventRepo.GetByTimeRange(tenantID, *req.StartTime, *req.EndTime, 10000)
	} else if req.Offset > 0 {
		events, err = e.eventRepo.GetByOffset(tenantID, "", 10000)
	} else {
		return nil, fmt.Errorf("must specify either time range or offset")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}

	task := &ReplayTask{
		ID:             fmt.Sprintf("replay-%d", time.Now().UnixNano()),
		TenantID:       tenantID,
		SubscriptionID: req.SubscriptionID,
		StartTime:      stringOrNil(req.StartTime),
		EndTime:        stringOrNil(req.EndTime),
		Rate:           req.Rate,
		Status:         "running",
		TotalCount:     len(events),
		CreatedAt:      time.Now(),
	}

	go e.executeReplay(task, events, sub)

	return task, nil
}

func (e *ReplayEngine) executeReplay(task *ReplayTask, events []*model.Event, sub *model.Subscription) {
	delay := e.replayDelay(task.Rate)

	for i, event := range events {
		if i > 0 {
			time.Sleep(delay)
		}

		if !e.backpressure.Allow(sub.ID) {
			e.backpressure.Wait(sub.ID)
		}

		delivery := &model.Delivery{
			EventID:        event.ID,
			SubscriptionID: sub.ID,
			TenantID:       task.TenantID,
			Status:         "pending",
			RetryCount:     0,
		}

		if err := e.deliveryRepo.Create(delivery); err != nil {
			log.Printf("replay: failed to create delivery for event %s: %v", event.ID, err)
			continue
		}

		task.ProcessedCount++
	}

	task.Status = "completed"
	log.Printf("replay task %s completed: %d/%d events processed", task.ID, task.ProcessedCount, task.TotalCount)
}

func (e *ReplayEngine) replayDelay(rate string) time.Duration {
	switch rate {
	case "5x":
		return 200 * time.Millisecond
	case "10x":
		return 100 * time.Millisecond
	default:
		return time.Second
	}
}

func stringOrNil(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func marshalIndent(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
