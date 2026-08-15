package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

const TenantCtxKey = "tenant_id"

func TenantIsolation() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				authHeader = c.QueryParam("tenant_id")
				if authHeader == "" {
					return c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing tenant identification"})
				}
			} else {
				authHeader = strings.TrimPrefix(authHeader, "Bearer ")
			}

			c.Set(TenantCtxKey, authHeader)
			return next(c)
		}
	}
}

func GetTenantID(c echo.Context) string {
	tenantID, _ := c.Get(TenantCtxKey).(string)
	return tenantID
}

func AdminAuth() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			adminKey := c.Request().Header.Get("X-Admin-Key")
			defaultKey := "eventbus-admin-key-2024"
			if adminKey == "" || adminKey != defaultKey {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "admin access required"})
			}
			return next(c)
		}
	}
}
