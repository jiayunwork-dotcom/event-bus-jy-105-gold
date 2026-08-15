package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/eventbus/server/internal/engine"
	"github.com/eventbus/server/internal/middleware"
	"github.com/eventbus/server/internal/model"
	"github.com/eventbus/server/internal/service"
	"github.com/labstack/echo/v4"
)

type SubscriptionHandler struct {
	subService *service.SubscriptionService
}

func NewSubscriptionHandler(ss *service.SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{subService: ss}
}

func (h *SubscriptionHandler) RegisterRoutes(g *echo.Group) {
	g.POST("", h.Create)
	g.GET("", h.List)
	g.GET("/:id", h.Get)
	g.PUT("/:id", h.Update)
	g.DELETE("/:id", h.Delete)
	g.PUT("/:id/rate-limit", h.UpdateRateLimit)
	g.GET("/:id/dag", h.GetDAG)
	g.PUT("/:id/dag", h.SaveDAG)
}

func (h *SubscriptionHandler) Create(c echo.Context) error {
	tenantID := middleware.GetTenantID(c)
	var req map[string]interface{}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	sub, err := h.subService.Create(tenantID, req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, sub)
}

func (h *SubscriptionHandler) List(c echo.Context) error {
	tenantID := middleware.GetTenantID(c)
	subs, err := h.subService.List(tenantID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, subs)
}

func (h *SubscriptionHandler) Get(c echo.Context) error {
	id := c.Param("id")
	sub, err := h.subService.Get(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "subscription not found"})
	}
	return c.JSON(http.StatusOK, sub)
}

func (h *SubscriptionHandler) Update(c echo.Context) error {
	id := c.Param("id")
	var req map[string]interface{}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	sub, err := h.subService.Update(id, req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, sub)
}

func (h *SubscriptionHandler) Delete(c echo.Context) error {
	id := c.Param("id")
	if err := h.subService.Delete(id); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *SubscriptionHandler) UpdateRateLimit(c echo.Context) error {
	id := c.Param("id")
	var req map[string]interface{}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	rateLimit := int(toFloat64(req["rate_limit"]))
	burst := int(toFloat64(req["burst"]))
	if err := h.subService.UpdateRateLimit(id, rateLimit, burst); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "updated"})
}

func (h *SubscriptionHandler) GetDAG(c echo.Context) error {
	id := c.Param("id")
	dag, err := h.subService.GetDAG(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "DAG not found"})
	}
	return c.JSON(http.StatusOK, dag)
}

func (h *SubscriptionHandler) SaveDAG(c echo.Context) error {
	id := c.Param("id")
	var req map[string]interface{}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if err := h.subService.SaveDAG(id, req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "saved"})
}

type PublishHandler struct {
	deliveryEngine *engine.DeliveryEngine
}

func NewPublishHandler(de *engine.DeliveryEngine) *PublishHandler {
	return &PublishHandler{deliveryEngine: de}
}

func (h *PublishHandler) RegisterRoutes(g *echo.Group) {
	g.POST("/events", h.Publish)
	g.POST("/events/batch", h.PublishBatch)
}

func (h *PublishHandler) Publish(c echo.Context) error {
	tenantID := middleware.GetTenantID(c)
	var req model.PublishEvent
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	resp, err := h.deliveryEngine.PublishEvent(tenantID, req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if !resp.Success {
		return c.JSON(http.StatusUnprocessableEntity, resp)
	}
	return c.JSON(http.StatusAccepted, resp)
}

func (h *PublishHandler) PublishBatch(c echo.Context) error {
	tenantID := middleware.GetTenantID(c)
	var req model.PublishRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if len(req.Events) > 500 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "batch size exceeds maximum of 500"})
	}
	resp, err := h.deliveryEngine.PublishBatch(tenantID, req.Events)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if !resp.Success {
		return c.JSON(http.StatusUnprocessableEntity, resp)
	}
	return c.JSON(http.StatusAccepted, resp)
}

type DeadLetterHandler struct {
	dlService *service.DeadLetterService
}

func NewDeadLetterHandler(dls *service.DeadLetterService) *DeadLetterHandler {
	return &DeadLetterHandler{dlService: dls}
}

func (h *DeadLetterHandler) RegisterRoutes(g *echo.Group) {
	g.GET("", h.List)
	g.GET("/:id", h.Get)
	g.POST("/:id/retry", h.Retry)
	g.POST("/batch-retry", h.BatchRetry)
	g.PUT("/:id/edit-retry", h.EditAndRetry)
}

func (h *DeadLetterHandler) List(c echo.Context) error {
	tenantID := middleware.GetTenantID(c)
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	offset, _ := strconv.Atoi(c.QueryParam("offset"))
	if limit == 0 {
		limit = 50
	}
	entries, err := h.dlService.List(tenantID, limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, entries)
}

func (h *DeadLetterHandler) Get(c echo.Context) error {
	id := c.Param("id")
	entry, err := h.dlService.Get(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "entry not found"})
	}
	return c.JSON(http.StatusOK, entry)
}

func (h *DeadLetterHandler) Retry(c echo.Context) error {
	id := c.Param("id")
	if err := h.dlService.Retry(id); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "retried"})
}

func (h *DeadLetterHandler) BatchRetry(c echo.Context) error {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if err := h.dlService.BatchRetry(req.IDs); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "batch_retried"})
}

func (h *DeadLetterHandler) EditAndRetry(c echo.Context) error {
	id := c.Param("id")
	var req struct {
		Payload string `json:"payload"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if err := h.dlService.EditAndRetry(id, req.Payload); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "edited_and_retried"})
}

type ReplayHandler struct {
	replayService *service.ReplayService
}

func NewReplayHandler(rs *service.ReplayService) *ReplayHandler {
	return &ReplayHandler{replayService: rs}
}

func (h *ReplayHandler) RegisterRoutes(g *echo.Group) {
	g.POST("/start", h.StartReplay)
}

func (h *ReplayHandler) StartReplay(c echo.Context) error {
	tenantID := middleware.GetTenantID(c)
	var req model.ReplayRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	task, err := h.replayService.StartReplay(req, tenantID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusAccepted, task)
}

type MonitorHandler struct {
	monitorService *service.MonitorService
	backpressure   *engine.BackpressureController
	tenantService  *service.TenantService
}

func NewMonitorHandler(ms *service.MonitorService, bp *engine.BackpressureController, ts *service.TenantService) *MonitorHandler {
	return &MonitorHandler{monitorService: ms, backpressure: bp, tenantService: ts}
}

func (h *MonitorHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/dashboard", h.GetDashboard)
	g.GET("/heatmap", h.GetHeatmap)
	g.GET("/events", h.GetEventsByFilter)
	g.GET("/traces/:event_id", h.GetEventTrace)
	g.GET("/traces/:event_id/:subscription_id", h.GetEventTraceBySubscription)
	g.GET("/alerts", h.GetAlerts)
	g.GET("/backpressure/:subscription_id", h.GetBackpressureStats)
}

func (h *MonitorHandler) GetDashboard(c echo.Context) error {
	tenantID := middleware.GetTenantID(c)
	dashboard, err := h.monitorService.GetDashboard(tenantID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, dashboard)
}

func (h *MonitorHandler) GetEventTrace(c echo.Context) error {
	eventID := c.Param("event_id")
	traces, err := h.monitorService.GetEventTrace(eventID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, traces)
}

func (h *MonitorHandler) GetEventTraceBySubscription(c echo.Context) error {
	eventID := c.Param("event_id")
	subscriptionID := c.Param("subscription_id")
	traces, err := h.monitorService.GetEventTraceBySubscription(eventID, subscriptionID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, traces)
}

func (h *MonitorHandler) GetEventsByFilter(c echo.Context) error {
	tenantID := middleware.GetTenantID(c)
	eventType := c.QueryParam("event_type")
	startTime := c.QueryParam("start_time")
	endTime := c.QueryParam("end_time")
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 {
		limit = 100
	}

	if eventType == "" || startTime == "" || endTime == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "event_type, start_time and end_time are required"})
	}

	events, err := h.monitorService.GetEventsByTypeAndTimeRange(tenantID, eventType, startTime, endTime, limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, events)
}

func (h *MonitorHandler) GetHeatmap(c echo.Context) error {
	tenantID := middleware.GetTenantID(c)
	minutes, _ := strconv.Atoi(c.QueryParam("minutes"))
	if minutes <= 0 {
		minutes = 60
	}

	data, err := h.monitorService.GetHeatmapData(tenantID, minutes)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	maxPublishQPS := 0
	tenant, err := h.tenantService.Get(tenantID)
	if err == nil && tenant != nil {
		maxPublishQPS = tenant.MaxPublishQPS
	}
	data["max_publish_qps"] = maxPublishQPS

	return c.JSON(http.StatusOK, data)
}

func (h *MonitorHandler) GetAlerts(c echo.Context) error {
	tenantID := middleware.GetTenantID(c)
	resolved := c.QueryParam("resolved") == "true"
	alerts, err := h.monitorService.GetAlerts(tenantID, resolved)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, alerts)
}

func (h *MonitorHandler) GetBackpressureStats(c echo.Context) error {
	subID := c.Param("subscription_id")
	stats := h.backpressure.GetBucketStats(subID)
	return c.JSON(http.StatusOK, stats)
}

func toFloat64(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
}

var _ = fmt.Sprintf
