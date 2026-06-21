// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/source-manager/cleanup"
	"github.com/caesar/all-chat/services/source-manager/demand"
	"github.com/caesar/all-chat/services/source-manager/election"
	"github.com/caesar/all-chat/services/source-manager/handlers"
	"github.com/caesar/all-chat/services/source-manager/registry"
	sharedAuth "github.com/caesar/all-chat/shared/auth"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/caesar/all-chat/shared/middleware"
	sharedredis "github.com/caesar/all-chat/shared/redis"
	"github.com/caesar/all-chat/shared/tracing"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logLevel := getEnvOrDefault("LOG_LEVEL", "info")
	log := logger.NewLogger("source-manager", logLevel)
	defer log.Sync()

	// Rebuild trigger
	log.Info("Starting Source Manager",
		zap.String("version", getEnvOrDefault("APP_VERSION", "dev")),
	)

	// Initialize tracing
	tracingEnabled := getEnvOrDefault("OTEL_ENABLED", "false") == "true"
	if tracingEnabled {
		tracingCfg := tracing.Config{
			ServiceName:    "source-manager",
			ServiceVersion: getEnvOrDefault("APP_VERSION", "dev"),
			Environment:    getEnvOrDefault("ENVIRONMENT", "development"),
			OTLPEndpoint:   getEnvOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
			Enabled:        true,
		}
		shutdownTracer, err := tracing.InitTracer(tracingCfg, log)
		if err != nil {
			log.Error("Failed to initialize tracer", zap.Error(err))
		} else {
			defer shutdownTracer(context.Background())
			log.Info("Tracer initialized", zap.String("service", "source-manager"))
		}
	}

	ctx := context.Background()

	// Connect to PostgreSQL
	dbHost := getEnvOrDefault("DATABASE_HOST", "localhost")
	dbPort := getEnvOrDefault("DATABASE_PORT", "5432")
	dbUser := getEnvOrDefault("DATABASE_USER", "allchat")
	dbPassword := getEnvOrDefault("DATABASE_PASSWORD", "allchat_dev_password")
	dbName := getEnvOrDefault("DATABASE_NAME", "allchat")

	connString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	db, err := database.NewPostgresPool(connString)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	log.Info("Connected to PostgreSQL")

	// Connect to Redis, retrying with backoff so a transient Redis outage
	// (e.g. the pod being rescheduled onto another node) does not crash-loop
	// this service. The retry is cancelled on shutdown signals so SIGTERM still
	// terminates the process promptly while it is waiting for Redis.
	redisHost := getEnvOrDefault("REDIS_HOST", "localhost")
	redisPort := getEnvOrDefault("REDIS_PORT", "6379")
	redisAddr := sharedredis.BuildDSN(redisHost, redisPort)

	startupCtx, stopStartup := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	redisClient, err := sharedredis.NewClientWithRetry(startupCtx, redisAddr, getEnvOrDefault("REDIS_PASSWORD", ""), tracingEnabled,
		sharedredis.DefaultRetryOptions(),
		func(attempt int, err error, backoff time.Duration) {
			log.Warn("Redis not reachable, retrying with backoff",
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff),
				zap.Error(err),
			)
		})
	stopStartup()
	if err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	defer redisClient.Close()

	log.Info("Connected to Redis")

	log.Info("Initialized Prometheus metrics")

	// Initialize components
	repo := registry.NewRepository(db, log)
	sourceRegistry := registry.NewRegistry(repo, 30*time.Second, log)
	leaderManager := election.NewManager(redisClient, log)
	cleanupJob := cleanup.NewJob(db, log)

	// Initialize demand subscriber (Phase 5)
	demandSubscriber := demand.NewOverlayDemandSubscriber(redisClient, repo, log)
	demandSubscriber.SetDB(db)
	log.Info("Initialized demand subscriber")

	// Start registry
	if err := sourceRegistry.Start(ctx); err != nil {
		log.Fatal("Failed to start source registry", zap.Error(err))
	}

	// Start cleanup job
	if err := cleanupJob.Start(ctx); err != nil {
		log.Fatal("Failed to start cleanup job", zap.Error(err))
	}

	// Set up HTTP server
	if logLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	if tracingEnabled {
		router.Use(tracing.GinMiddleware("source-manager"))
	}

	// Health check handlers
	healthHandler := handlers.NewHealthHandler(sourceRegistry, leaderManager)
	router.GET("/health/live", healthHandler.LivenessProbe)
	router.GET("/health/ready", healthHandler.ReadinessProbe)
	router.GET("/status", healthHandler.Status)

	// Prometheus metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	serviceKeyChain, err := sharedAuth.NewKeyChainFromEnv("SERVICE_JWT_SECRET")
	if err != nil {
		log.Fatal("service JWT key chain init failed (SERVICE_JWT_SECRET_V1 must be set)", zap.Error(err))
	}
	log.Info("service JWT key chain initialized", zap.String("latest_kid", serviceKeyChain.LatestKid()))

	protected := router.Group("/")
	protected.Use(middleware.ServiceJWTAuth(serviceKeyChain))

	// Source handlers
	sourceHandler := handlers.NewSourceHandler(sourceRegistry, leaderManager)
	protected.GET("/sources", sourceHandler.GetSources)
	protected.POST("/sources/activate", sourceHandler.ActivateSource)
	protected.POST("/sources/deactivate", sourceHandler.DeactivateSource)
	protected.POST("/leadership/claim", sourceHandler.ClaimLeadership)
	protected.POST("/leadership/renew", sourceHandler.RenewLeadership)
	protected.POST("/leadership/release", sourceHandler.ReleaseLeadership)
	protected.GET("/leadership", sourceHandler.GetLeadershipStatus)
	protected.POST("/leadership/peers/register", sourceHandler.RegisterPeer)

	// Demand handlers (Phase 5)
	demandHandler := demand.NewDemandHandler(demandSubscriber)
	protected.GET("/demand", demandHandler.GetDemand)

	// Get port
	port := getEnvOrDefault("PORT", "8083")

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start HTTP server in goroutine
	go func() {
		log.Info("HTTP server listening",
			zap.String("port", port),
			zap.String("instance_id", leaderManager.GetInstanceID()),
		)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start HTTP server", zap.Error(err))
		}
	}()

	// Start demand subscriber (Phase 5)
	go func() {
		log.Info("Starting demand subscriber")
		if err := demandSubscriber.Start(ctx); err != nil {
			log.Error("Demand subscriber failed", zap.Error(err))
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down service...")

	// Stop registry and cleanup job
	sourceRegistry.Stop()
	cleanupJob.Stop()

	// Shutdown HTTP server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP server forced to shutdown", zap.Error(err))
	}

	log.Info("Service exited")
}

// getEnvOrDefault gets an environment variable or returns a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
