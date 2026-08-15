package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"time"

	"github.com/eventbus/server/internal/filter"
	"github.com/eventbus/server/internal/model"
	"github.com/eventbus/server/internal/repository"
	"github.com/redis/go-redis/v9"
)

type DeliveryEngine struct {
	eventRepo         *repository.EventRepo
	deliveryRepo      *repository.DeliveryRepo
	subscriptionRepo  *repository.SubscriptionRepo
	orchestrationRepo *repository.OrchestrationRepo
	traceRepo         *repository.TraceRepo
	deadLetterRepo    *repository.DeadLetterRepo
	schemaRepo        *repository.SchemaRepo
	filterEngine      *filter.FilterEngine
	orchestrator      *Orchestrator
	backpressure      *BackpressureController
	redis             *redis.Client
}

func NewDeliveryEngine(
	eventRepo *repository.EventRepo,
	deliveryRepo *repository.DeliveryRepo,
	subscriptionRepo *repository.SubscriptionRepo,
	orchestrationRepo *repository.OrchestrationRepo,
	traceRepo *repository.TraceRepo,
	deadLetterRepo *repository.DeadLetterRepo,
	schemaRepo *repository.SchemaRepo,
	filterEngine *filter.FilterEngine,
	orchestrator *Orchestrator,
	backpressure *BackpressureController,
	rdb *redis.Client,
) *DeliveryEngine {
	return &DeliveryEngine{
		eventRepo:         eventRepo,
		deliveryRepo:      deliveryRepo,
		subscriptionRepo:  subscriptionRepo,
		orchestrationRepo: orchestrationRepo,
		traceRepo:         traceRepo,
		deadLetterRepo:    deadLetterRepo,
		schemaRepo:        schemaRepo,
		filterEngine:      filterEngine,
		orchestrator:      orchestrator,
		backpressure:      backpressure,
		redis:             rdb,
	}
}

func (e *DeliveryEngine) PublishEvent(tenantID string, req model.PublishEvent) (*model.PublishResponse, error) {
	schema, err := e.getLatestSchema(tenantID, req.EventType)
	if err != nil {
		return nil, fmt.Errorf("no schema found for event type %s: %w", req.EventType, err)
	}

	var payload map[string]interface{}
	payloadBytes, _ := json.Marshal(req.Payload)
	json.Unmarshal(payloadBytes, &payload)

	validationErrors := validateAgainstSchema(payload, schema.SchemaDef)
	if len(validationErrors) > 0 {
		return &model.PublishResponse{
			Success: false,
			Errors:  validationErrors,
		}, nil
	}

	if req.IdempotentKey != "" {
		exists, err := e.eventRepo.CheckIdempotentKey(tenantID, req.IdempotentKey)
		if err != nil {
			return nil, fmt.Errorf("idempotency check failed: %w", err)
		}
		if exists {
			return &model.PublishResponse{
				Success:  true,
				EventIDs: []string{"duplicate_skipped"},
			}, nil
		}
	}

	event := &model.Event{
		TenantID:      tenantID,
		EventType:     req.EventType,
		SchemaVersion: schema.Version,
		Payload:       string(payloadBytes),
		IdempotentKey: req.IdempotentKey,
	}

	if err := e.eventRepo.Create(event); err != nil {
		return nil, fmt.Errorf("failed to store event: %w", err)
	}

	go e.routeEvent(event)

	return &model.PublishResponse{
		Success:  true,
		EventIDs: []string{event.ID},
	}, nil
}

func (e *DeliveryEngine) PublishBatch(tenantID string, reqs []model.PublishEvent) (*model.PublishResponse, error) {
	if len(reqs) > 500 {
		return nil, fmt.Errorf("batch size exceeds maximum of 500")
	}

	var allEvents []*model.Event
	var allValidationErrors []model.ValidationError

	for i, req := range reqs {
		schema, err := e.getLatestSchema(tenantID, req.EventType)
		if err != nil {
			return nil, fmt.Errorf("event[%d]: no schema for type %s", i, req.EventType)
		}

		var payload map[string]interface{}
		payloadBytes, _ := json.Marshal(req.Payload)
		json.Unmarshal(payloadBytes, &payload)

		validationErrors := validateAgainstSchema(payload, schema.SchemaDef)
		if len(validationErrors) > 0 {
			for _, ve := range validationErrors {
				allValidationErrors = append(allValidationErrors, model.ValidationError{
					Field:   fmt.Sprintf("events[%d].%s", i, ve.Field),
					Message: ve.Message,
				})
			}
		}

		allEvents = append(allEvents, &model.Event{
			TenantID:      tenantID,
			EventType:     req.EventType,
			SchemaVersion: schema.Version,
			Payload:       string(payloadBytes),
			IdempotentKey: req.IdempotentKey,
		})
	}

	if len(allValidationErrors) > 0 {
		return &model.PublishResponse{
			Success: false,
			Errors:  allValidationErrors,
		}, nil
	}

	if err := e.eventRepo.CreateBatch(allEvents); err != nil {
		return nil, fmt.Errorf("batch insert failed: %w", err)
	}

	var eventIDs []string
	for _, ev := range allEvents {
		eventIDs = append(eventIDs, ev.ID)
		go e.routeEvent(ev)
	}

	return &model.PublishResponse{
		Success:  true,
		EventIDs: eventIDs,
	}, nil
}

func (e *DeliveryEngine) routeEvent(event *model.Event) {
	subs, err := e.subscriptionRepo.ListActiveByEventType(event.TenantID, event.EventType)
	if err != nil || len(subs) == 0 {
		return
	}

	var payload map[string]interface{}
	json.Unmarshal([]byte(event.Payload), &payload)

	for _, sub := range subs {
		if sub.FilterExpression != "" {
			matched, err := e.filterEngine.MatchWithCache(sub.ID, sub.FilterExpression, payload)
			if err != nil || !matched {
				continue
			}
		}

		delivery := &model.Delivery{
			EventID:        event.ID,
			SubscriptionID: sub.ID,
			TenantID:       event.TenantID,
			Status:         "pending",
			RetryCount:     0,
		}

		if sub.DeliveryMode == "exactly_once" && sub.IdempotentKeyPath != "" {
			key := extractIdempotentKey(payload, sub.IdempotentKeyPath)
			if key != "" {
				isDuplicate, err := e.checkIdempotency(event.TenantID, sub.ID, key, sub.IdempotentWindowSeconds)
				if err == nil && isDuplicate {
					continue
				}
				delivery.IdempotentKey = key
			}
		}

		if err := e.deliveryRepo.Create(delivery); err != nil {
			log.Printf("failed to create delivery: %v", err)
			continue
		}

		go e.deliver(delivery, sub)
	}
}

func (e *DeliveryEngine) deliver(delivery *model.Delivery, sub *model.Subscription) {
	if !e.backpressure.Allow(sub.ID) {
		e.backpressure.Wait(sub.ID)
	}

	dag, err := e.orchestrationRepo.GetBySubscriptionID(sub.ID)
	if err == nil && dag != nil && len(dag.Nodes) > 0 {
		var payload map[string]interface{}
		json.Unmarshal([]byte(e.getEventPayload(delivery.EventID)), &payload)

		ctx := &ExecutionContext{
			EventID:        delivery.EventID,
			SubscriptionID: sub.ID,
			TenantID:       delivery.TenantID,
			Payload:        payload,
			Nodes:          dag.Nodes,
			Edges:          dag.Edges,
		}

		if err := e.orchestrator.Execute(ctx); err != nil {
			e.handleDeliveryFailure(delivery, sub, err.Error())
			return
		}

		for _, result := range ctx.Results {
			if result.Status == "failed" {
				e.handleDeliveryFailure(delivery, sub, result.ErrorMessage)
				return
			}
		}

		e.deliveryRepo.UpdateStatus(delivery.ID, "delivered", "")
		return
	}

	err = e.sendToConsumer(sub.ConsumerURL, delivery)
	if err != nil {
		e.handleDeliveryFailure(delivery, sub, err.Error())
		return
	}

	e.deliveryRepo.UpdateStatus(delivery.ID, "delivered", "")
}

func (e *DeliveryEngine) handleDeliveryFailure(delivery *model.Delivery, sub *model.Subscription, errMsg string) {
	if delivery.RetryCount >= sub.MaxRetries {
		e.deliveryRepo.MoveToDeadLetter(delivery, errMsg)
		return
	}

	nextRetry := time.Now().Add(time.Duration(math.Pow(2, float64(delivery.RetryCount))) * time.Second)
	e.deliveryRepo.IncrementRetry(delivery.ID, nextRetry, errMsg)
}

func (e *DeliveryEngine) ProcessRetries() {
	deliveries, err := e.deliveryRepo.GetPendingRetries(100)
	if err != nil || len(deliveries) == 0 {
		return
	}

	for _, d := range deliveries {
		sub, err := e.subscriptionRepo.GetByID(d.SubscriptionID)
		if err != nil {
			continue
		}
		go e.deliver(d, sub)
	}
}

func (e *DeliveryEngine) sendToConsumer(url string, delivery *model.Delivery) error {
	event, err := e.eventRepo.GetByID(delivery.EventID)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	body := map[string]interface{}{
		"event_id":   event.ID,
		"event_type": event.EventType,
		"payload":    json.RawMessage(event.Payload),
		"timestamp":  event.CreatedAt,
	}
	bodyBytes, _ := json.Marshal(body)

	resp, err := client.Post(url, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("consumer returned status %d", resp.StatusCode)
	}
	return nil
}

func (e *DeliveryEngine) getLatestSchema(tenantID, eventType string) (*model.EventSchema, error) {
	return e.schemaRepo.GetLatestVersion(tenantID, eventType)
}

func (e *DeliveryEngine) getEventPayload(eventID string) string {
	event, err := e.eventRepo.GetByID(eventID)
	if err != nil {
		return "{}"
	}
	return event.Payload
}

func (e *DeliveryEngine) checkIdempotency(tenantID, subscriptionID, key string, windowSeconds int) (bool, error) {
	ctx := context.Background()
	redisKey := fmt.Sprintf("idempotent:%s:%s:%s", tenantID, subscriptionID, key)
	exists, err := e.redis.Exists(ctx, redisKey).Result()
	if err != nil {
		return false, err
	}
	if exists > 0 {
		return true, nil
	}
	e.redis.Set(ctx, redisKey, "1", time.Duration(windowSeconds)*time.Second)
	return false, nil
}

func extractIdempotentKey(payload map[string]interface{}, keyPath string) string {
	val, ok := getNestedField(payload, keyPath)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%v", val)
}

func validateAgainstSchema(payload map[string]interface{}, schemaDef string) []model.ValidationError {
	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(schemaDef), &schema); err != nil {
		return []model.ValidationError{{Field: "schema", Message: "invalid schema definition"}}
	}

	var errors []model.ValidationError

	required, _ := schema["required"].([]interface{})
	properties, _ := schema["properties"].(map[string]interface{})

	for _, req := range required {
		reqStr, _ := req.(string)
		if _, ok := payload[reqStr]; !ok {
			errors = append(errors, model.ValidationError{
				Field:   reqStr,
				Message: fmt.Sprintf("required field '%s' is missing", reqStr),
			})
		}
	}

	for field, value := range payload {
		if prop, ok := properties[field]; ok {
			propDef, _ := prop.(map[string]interface{})
			if expectedType, ok := propDef["type"].(string); ok {
				if !checkType(value, expectedType) {
					errors = append(errors, model.ValidationError{
						Field:   field,
						Message: fmt.Sprintf("field '%s' expected type '%s'", field, expectedType),
					})
				}
			}
		}
	}

	return errors
}

func checkType(value interface{}, expectedType string) bool {
	switch expectedType {
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		switch value.(type) {
		case float64, float32, int, int64, int32:
			return true
		default:
			return false
		}
	case "integer":
		switch value.(type) {
		case int, int64, int32:
			return true
		case float64:
			return float64(int(value.(float64))) == value.(float64)
		default:
			return false
		}
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "object":
		_, ok := value.(map[string]interface{})
		return ok
	case "array":
		_, ok := value.([]interface{})
		return ok
	default:
		return true
	}
}
