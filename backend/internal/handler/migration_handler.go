package handler

import (
	"net/http"

	"github.com/eventbus/server/internal/middleware"
	"github.com/eventbus/server/internal/model"
	"github.com/eventbus/server/internal/service"
	"github.com/labstack/echo/v4"
)

type MigrationHandler struct {
	migrationService *service.MigrationService
}

func NewMigrationHandler(ms *service.MigrationService) *MigrationHandler {
	return &MigrationHandler{migrationService: ms}
}

func (h *MigrationHandler) RegisterRoutes(g *echo.Group) {
	g.POST("/validate-rules", h.ValidateRules)
	g.POST("/preview", h.Preview)
	g.POST("/start", h.Start)
	g.GET("/:id/progress", h.GetProgress)
	g.POST("/:id/cancel", h.Cancel)
	g.POST("/:id/rollback", h.Rollback)
	g.GET("/:id/impact", h.AnalyzeImpact)
	g.GET("", h.List)
	g.GET("/:id", h.Get)
}

func (h *MigrationHandler) ValidateRules(c echo.Context) error {
	var req struct {
		MigrationRules []model.MigrationRule `json:"migration_rules" validate:"required"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if len(req.MigrationRules) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"valid": false,
			"errors": []model.MigrationRuleValidationError{
				{
					RuleIndex: 0,
					Field:     "migration_rules",
					Message:   "至少需要配置一条迁移规则",
				},
			},
		})
	}

	errors := h.migrationService.ValidateRules(req.MigrationRules)
	if len(errors) > 0 {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"valid":  false,
			"errors": errors,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"valid": true,
	})
}

func (h *MigrationHandler) Preview(c echo.Context) error {
	tenantID := middleware.GetTenantID(c)
	var req model.MigrationPreviewRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	results, err := h.migrationService.Preview(tenantID, &req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, results)
}

func (h *MigrationHandler) Start(c echo.Context) error {
	tenantID := middleware.GetTenantID(c)
	var req model.MigrationStartRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if len(req.MigrationRules) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "至少需要配置一条迁移规则"})
	}

	migration, err := h.migrationService.StartMigration(tenantID, &req)
	if err != nil {
		if err.Error() == "a migration is already running for this event type" {
			return c.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, migration)
}

func (h *MigrationHandler) GetProgress(c echo.Context) error {
	tenantID := middleware.GetTenantID(c)
	migrationID := c.Param("id")

	progress, err := h.migrationService.GetProgress(tenantID, migrationID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, progress)
}

func (h *MigrationHandler) Cancel(c echo.Context) error {
	tenantID := middleware.GetTenantID(c)
	migrationID := c.Param("id")

	if err := h.migrationService.CancelMigration(tenantID, migrationID); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "cancelled"})
}

func (h *MigrationHandler) Rollback(c echo.Context) error {
	tenantID := middleware.GetTenantID(c)
	migrationID := c.Param("id")

	if err := h.migrationService.RollbackMigration(tenantID, migrationID); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "rollback_started"})
}

func (h *MigrationHandler) List(c echo.Context) error {
	tenantID := middleware.GetTenantID(c)
	eventType := c.QueryParam("event_type")
	if eventType == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "event_type query parameter is required"})
	}

	migrations, err := h.migrationService.ListMigrations(tenantID, eventType)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, migrations)
}

func (h *MigrationHandler) Get(c echo.Context) error {
	tenantID := middleware.GetTenantID(c)
	migrationID := c.Param("id")

	migration, err := h.migrationService.GetMigration(tenantID, migrationID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, migration)
}

func (h *MigrationHandler) AnalyzeImpact(c echo.Context) error {
	tenantID := middleware.GetTenantID(c)
	migrationID := c.Param("id")

	report, err := h.migrationService.AnalyzeImpact(tenantID, migrationID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, report)
}
