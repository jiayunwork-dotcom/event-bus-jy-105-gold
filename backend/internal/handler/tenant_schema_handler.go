package handler

import (
	"fmt"
	"net/http"

	"github.com/eventbus/server/internal/middleware"
	"github.com/eventbus/server/internal/service"
	"github.com/labstack/echo/v4"
)

type TenantHandler struct {
	tenantService *service.TenantService
}

func NewTenantHandler(ts *service.TenantService) *TenantHandler {
	return &TenantHandler{tenantService: ts}
}

func (h *TenantHandler) RegisterRoutes(g *echo.Group) {
	g.POST("", h.Create)
	g.GET("", h.List)
	g.GET("/:id", h.Get)
	g.PUT("/:id", h.Update)
	g.PUT("/:id/disable", h.Disable)
}

func (h *TenantHandler) Create(c echo.Context) error {
	var req map[string]interface{}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	tenant, err := h.tenantService.Create(req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, tenant)
}

func (h *TenantHandler) List(c echo.Context) error {
	tenants, err := h.tenantService.List()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, tenants)
}

func (h *TenantHandler) Get(c echo.Context) error {
	id := c.Param("id")
	tenant, err := h.tenantService.Get(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "tenant not found"})
	}
	return c.JSON(http.StatusOK, tenant)
}

func (h *TenantHandler) Update(c echo.Context) error {
	id := c.Param("id")
	var req map[string]interface{}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	tenant, err := h.tenantService.Update(id, req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, tenant)
}

func (h *TenantHandler) Disable(c echo.Context) error {
	id := c.Param("id")
	if err := h.tenantService.Disable(id); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "disabled"})
}

type SchemaHandler struct {
	schemaService *service.SchemaService
}

func NewSchemaHandler(ss *service.SchemaService) *SchemaHandler {
	return &SchemaHandler{schemaService: ss}
}

func (h *SchemaHandler) RegisterRoutes(g *echo.Group) {
	g.POST("", h.Register)
	g.GET("", h.ListByTenant)
	g.GET("/:event_type/versions", h.ListVersions)
	g.GET("/:event_type/versions/:version", h.GetByVersion)
	g.GET("/:event_type/latest", h.GetLatest)
	g.POST("/:event_type/check-compatibility", h.CheckCompatibility)
	g.GET("/:event_type/diff", h.Diff)
}

func (h *SchemaHandler) Register(c echo.Context) error {
	tenantID := middleware.GetTenantID(c)
	var req map[string]interface{}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	schema, err := h.schemaService.Register(tenantID, req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, schema)
}

func (h *SchemaHandler) ListByTenant(c echo.Context) error {
	tenantID := middleware.GetTenantID(c)
	schemas, err := h.schemaService.ListByTenant(tenantID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, schemas)
}

func (h *SchemaHandler) ListVersions(c echo.Context) error {
	tenantID := middleware.GetTenantID(c)
	eventType := c.Param("event_type")
	schemas, err := h.schemaService.ListVersions(tenantID, eventType)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, schemas)
}

func (h *SchemaHandler) GetByVersion(c echo.Context) error {
	tenantID := middleware.GetTenantID(c)
	eventType := c.Param("event_type")
	version := c.Param("version")
	var ver int
	fmt.Sscanf(version, "%d", &ver)
	schema, err := h.schemaService.GetByVersion(tenantID, eventType, ver)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "schema version not found"})
	}
	return c.JSON(http.StatusOK, schema)
}

func (h *SchemaHandler) GetLatest(c echo.Context) error {
	tenantID := middleware.GetTenantID(c)
	eventType := c.Param("event_type")
	schema, err := h.schemaService.GetLatest(tenantID, eventType)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "no schema found"})
	}
	return c.JSON(http.StatusOK, schema)
}

func (h *SchemaHandler) CheckCompatibility(c echo.Context) error {
	tenantID := middleware.GetTenantID(c)
	eventType := c.Param("event_type")
	var req map[string]interface{}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	schemaDef, _ := req["schema_def"].(string)
	isCompatible, note, err := h.schemaService.CheckCompatibility(tenantID, eventType, schemaDef)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"is_compatible": isCompatible,
		"note":          note,
	})
}

func (h *SchemaHandler) Diff(c echo.Context) error {
	tenantID := middleware.GetTenantID(c)
	eventType := c.Param("event_type")
	var v1, v2 int
	fmt.Sscanf(c.QueryParam("v1"), "%d", &v1)
	fmt.Sscanf(c.QueryParam("v2"), "%d", &v2)
	result, err := h.schemaService.Diff(tenantID, eventType, v1, v2)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, result)
}
