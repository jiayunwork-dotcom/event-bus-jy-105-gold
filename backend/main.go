package main

import (
	"fmt"
	"log"
	"time"

	"github.com/eventbus/server/config"
	"github.com/eventbus/server/internal/engine"
	"github.com/eventbus/server/internal/filter"
	"github.com/eventbus/server/internal/handler"
	"github.com/eventbus/server/internal/middleware"
	"github.com/eventbus/server/internal/model"
	"github.com/eventbus/server/internal/repository"
	"github.com/eventbus/server/internal/service"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg := config.Load()

	db, err := repository.NewDB(cfg.DSN())
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := repository.RunMigrations(db); err != nil {
		log.Fatalf("failed to run database migrations: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr(),
	})

	tenantRepo := repository.NewTenantRepo(db)
	schemaRepo := repository.NewSchemaRepo(db)
	eventRepo := repository.NewEventRepo(db)
	deliveryRepo := repository.NewDeliveryRepo(db)
	subscriptionRepo := repository.NewSubscriptionRepo(db)
	orchestrationRepo := repository.NewOrchestrationRepo(db)
	traceRepo := repository.NewTraceRepo(db)
	deadLetterRepo := repository.NewDeadLetterRepo(db)
	alertRepo := repository.NewAlertRepo(db)
	migrationRepo := repository.NewMigrationRepo(db)

	filterEngine := filter.NewFilterEngine()
	orchestrator := engine.NewOrchestrator(traceRepo)
	backpressure := engine.NewBackpressureController(rdb)
	backpressure.LoadFromRedis()

	deliveryEngine := engine.NewDeliveryEngine(
		eventRepo, deliveryRepo, subscriptionRepo,
		orchestrationRepo, traceRepo, deadLetterRepo,
		schemaRepo, filterEngine, orchestrator, backpressure, rdb,
	)

	replayEngine := engine.NewReplayEngine(
		eventRepo, deliveryRepo, subscriptionRepo, backpressure,
	)

	migrationEngine := engine.NewMigrationEngine()
	migrationExecutor := engine.NewMigrationExecutor(eventRepo, migrationRepo, migrationEngine)

	tenantService := service.NewTenantService(tenantRepo)
	schemaService := service.NewSchemaService(schemaRepo)
	subscriptionService := service.NewSubscriptionService(
		subscriptionRepo, orchestrationRepo, tenantRepo,
		orchestrator, backpressure,
	)
	deadLetterService := service.NewDeadLetterService(deadLetterRepo, eventRepo)
	replayService := service.NewReplayService(replayEngine)
	monitorService := service.NewMonitorService(
		subscriptionRepo, eventRepo, deadLetterRepo, traceRepo, alertRepo,
	)

	migrationService := service.NewMigrationService(
		migrationRepo, eventRepo, schemaRepo, migrationEngine, migrationExecutor,
		subscriptionRepo, orchestrationRepo,
	)

	tenantHandler := handler.NewTenantHandler(tenantService)
	schemaHandler := handler.NewSchemaHandler(schemaService)
	subscriptionHandler := handler.NewSubscriptionHandler(subscriptionService)
	publishHandler := handler.NewPublishHandler(deliveryEngine)
	deadLetterHandler := handler.NewDeadLetterHandler(deadLetterService)
	replayHandler := handler.NewReplayHandler(replayService)
	monitorHandler := handler.NewMonitorHandler(monitorService, backpressure, tenantService)
	migrationHandler := handler.NewMigrationHandler(migrationService)

	e := echo.New()

	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("Access-Control-Allow-Origin", "*")
			c.Response().Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Response().Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Admin-Key")
			if c.Request().Method == "OPTIONS" {
				return c.NoContent(204)
			}
			return next(c)
		}
	})

	adminApi := e.Group("/api", middleware.AdminAuth())
	tenantHandler.RegisterRoutes(adminApi.Group("/tenants"))

	api := e.Group("/api", middleware.TenantIsolation())

	schemaHandler.RegisterRoutes(api.Group("/schemas"))
	subscriptionHandler.RegisterRoutes(api.Group("/subscriptions"))
	publishHandler.RegisterRoutes(api.Group("/publish"))
	deadLetterHandler.RegisterRoutes(api.Group("/dead-letter"))
	replayHandler.RegisterRoutes(api.Group("/replay"))
	monitorHandler.RegisterRoutes(api.Group("/monitor"))
	migrationHandler.RegisterRoutes(api.Group("/migrations"))

	go startRetryWorker(deliveryEngine)
	go startBacklogMonitor(backpressure, subscriptionRepo, alertRepo)

	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	log.Printf("Event Bus Server starting on %s", addr)
	e.Logger.Fatal(e.Start(addr))
}

func startRetryWorker(de *engine.DeliveryEngine) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		de.ProcessRetries()
	}
}

func startBacklogMonitor(bp *engine.BackpressureController, subRepo *repository.SubscriptionRepo, alertRepo *repository.AlertRepo) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		for alert := range bp.AlertChannel() {
			alertRepo.Create(&model.BackpressureAlert{
				AlertType:      alert.AlertType,
				TenantID:       alert.TenantID,
				SubscriptionID: alert.SubscriptionID,
				Message:        alert.Message,
			})
		}
	}
}
