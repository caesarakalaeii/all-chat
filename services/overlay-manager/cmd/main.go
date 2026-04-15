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
	"strconv"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/overlay-manager/clients"
	"github.com/caesar/all-chat/services/overlay-manager/creditroll"
	"github.com/caesar/all-chat/services/overlay-manager/handlers"
	"github.com/caesar/all-chat/services/overlay-manager/repository"
	"github.com/caesar/all-chat/services/overlay-manager/youtube"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/caesar/all-chat/shared/middleware"
	sharedRedis "github.com/caesar/all-chat/shared/redis"
	"github.com/caesar/all-chat/shared/tracing"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logLevel := getEnv("LOG_LEVEL", "info")
	log := logger.NewLogger("overlay-manager", logLevel)
	defer log.Sync()

	log.Info("Starting Overlay Manager Service",
		zap.String("version", getEnv("APP_VERSION", "0.1.0")),
	)

	// Initialize OpenTelemetry tracing
	tracingEnabled := getEnv("OTEL_ENABLED", "false") == "true"
	if tracingEnabled {
		tracingCfg := tracing.Config{
			ServiceName:    "overlay-manager",
			ServiceVersion: getEnv("APP_VERSION", "0.1.0"),
			Environment:    getEnv("ENVIRONMENT", "development"),
			OTLPEndpoint:   getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
			Enabled:        true,
		}

		shutdownTracer, err := tracing.InitTracer(tracingCfg, log)
		if err != nil {
			log.Error("Failed to initialize tracer (continuing without tracing)", zap.Error(err))
		} else {
			defer shutdownTracer(context.Background())
			log.Info("OpenTelemetry tracing enabled")
		}
	}

	// Load configuration from environment
	config := loadConfig()

	// Validate Twitch credentials
	if config.TwitchClientID == "" || config.TwitchClientSecret == "" {
		log.Fatal("TWITCH_CLIENT_ID and TWITCH_CLIENT_SECRET are required")
	}

	// Connect to PostgreSQL
	log.Info("Connecting to PostgreSQL",
		zap.String("host", config.DatabaseHost),
		zap.String("database", config.DatabaseName),
	)

	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		config.DatabaseUser,
		config.DatabasePassword,
		config.DatabaseHost,
		config.DatabasePort,
		config.DatabaseName,
	)

	dbPool, err := database.NewPostgresPoolWithTracing(connString, tracingEnabled)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer dbPool.Close()

	log.Info("Connected to PostgreSQL successfully")

	// Connect to Redis
	log.Info("Connecting to Redis",
		zap.String("host", config.RedisHost),
		zap.String("port", config.RedisPort),
	)

	redisAddr := fmt.Sprintf("%s:%s", config.RedisHost, config.RedisPort)
	redisClient, err := sharedRedis.NewClientWithTracing(redisAddr, "", tracingEnabled)
	if err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	defer redisClient.Close()

	log.Info("Connected to Redis successfully")

	// Initialize metrics (available via /metrics endpoint)
	bm := metrics.NewBusinessMetrics()
	log.Info("Initialized Prometheus metrics")

	// HTTP metrics counters for overlay-manager
	httpRequestsTotal := promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests",
	}, []string{"service", "method", "path", "status"})

	httpRequestDuration := promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"service", "method", "path"})

	// Initialize repositories with the connection string
	overlayRepo, err := repository.NewOverlayRepository(connString)
	if err != nil {
		log.Fatal("Failed to create overlay repository", zap.Error(err))
	}

	configRepo, err := repository.NewOverlayConfigRepository(connString)
	if err != nil {
		log.Fatal("Failed to create overlay config repository", zap.Error(err))
	}

	sourceRepo, err := repository.NewSourceRepository(connString)
	if err != nil {
		log.Fatal("Failed to create source repository", zap.Error(err))
	}

	eventSettingsRepo, err := repository.NewEventSettingsRepository(connString)
	if err != nil {
		log.Fatal("Failed to create event settings repository", zap.Error(err))
	}

	creditRollRepo := repository.NewCreditRollRepository(dbPool)
	maintenanceRepo := repository.NewMaintenanceRepository(dbPool)

	// Initialize Twitch clips client
	twitchClipsClient := clients.NewTwitchClipsClient(config.TwitchClientID, config.TwitchClientSecret, log)

	// Initialize handlers
	mpClient := clients.NewMessageProcessorClient(config.MessageProcessorURL, config.MessageProcessorAPIKey, tracingEnabled, log)
	overlayHandler := handlers.NewOverlayHandler(overlayRepo, sourceRepo, configRepo)
	configHandler := handlers.NewConfigHandler(configRepo, overlayRepo, sourceRepo)
	sourcesHandler := handlers.NewSourcesHandler(sourceRepo, overlayRepo, dbPool, log, redisClient, bm)
	mockHandler := handlers.NewMockMessageHandler(overlayRepo, sourceRepo, mpClient, log)
	healthHandler := handlers.NewHealthHandler(dbPool, redisClient)
	adminHandler := handlers.NewAdminHandler(overlayRepo, sourceRepo, log)
	eventSettingsHandler := handlers.NewEventSettingsHandler(eventSettingsRepo, overlayRepo)
	creditRollHandler := creditroll.NewHandler(creditRollRepo, overlayRepo, sourceRepo, redisClient, log, twitchClipsClient)
	maintenanceHandler := handlers.NewMaintenanceHandler(maintenanceRepo, log)

	// YouTube helper
	youtubeAPIKey := getEnv("YOUTUBE_API_KEY", "")

	// Initialize YouTube quota client (connects to youtube-listener for quota tracking)
	youtubeListenerURL := getEnv("YOUTUBE_LISTENER_URL", "http://youtube-listener:8086")
	youtubeQuotaClient := clients.NewYouTubeQuotaClient(youtubeListenerURL, tracingEnabled, log)

	// Initialize resolver with quota tracking
	youtubeResolver := youtube.NewResolver(youtubeAPIKey, youtubeQuotaClient, log)
	youtubeHandler := handlers.NewYouTubeHandler(youtubeResolver, log)

	// Setup Gin router
	if config.GinMode == "release" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(httpMetricsMiddleware(httpRequestsTotal, httpRequestDuration, "overlay-manager"))

	// Add tracing middleware if enabled
	if tracingEnabled {
		router.Use(tracing.GinMiddleware("overlay-manager"))
	}

	// CORS is handled by API Gateway, not by individual services
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/health/live", "/health/ready"},
	}))

	// Register health routes (no auth required)
	healthHandler.RegisterRoutes(router)

	// Prometheus metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Public config for OBS/browser sources
	router.GET("/public/:id/config", configHandler.HandleGetPublicConfig)
	router.GET("/public/:id/event-settings", eventSettingsHandler.HandleGetPublicEventSettings)
	router.GET("/public/:id/creditroll", creditRollHandler.HandleGetPublicConfig)
	router.GET("/public/:id/credit-roll", creditRollHandler.HandleGetCreditRoll)

	// Protected routes (require JWT)
	protected := router.Group("/")
	protected.Use(middleware.JWTAuth(config.JWTSecret))
	{
		// Overlay CRUD routes (no :id prefix)
		protected.POST("/", overlayHandler.HandleCreateOverlay)
		protected.GET("/", overlayHandler.HandleListOverlays)
		protected.GET("/:id", overlayHandler.HandleGetOverlay)
		protected.PUT("/:id", overlayHandler.HandleUpdateOverlay)
		protected.DELETE("/:id", overlayHandler.HandleDeleteOverlay)
		protected.POST("/:id/clone", overlayHandler.HandleCloneOverlay)

		// Source management routes (nested under /:id)
		protected.GET("/:id/sources", sourcesHandler.HandleListSources)
		protected.POST("/:id/sources", sourcesHandler.HandleAddSource)
		protected.DELETE("/:id/sources/:source_id", sourcesHandler.HandleDeleteSource)
		protected.PATCH("/:id/sources/:source_id", sourcesHandler.HandleUpdateSourceConfig)

		protected.GET("/:id/config", configHandler.HandleGetConfig)
		protected.PUT("/:id/config", configHandler.HandleUpdateConfig)
		protected.GET("/:id/event-settings", eventSettingsHandler.HandleGetEventSettings)
		protected.PUT("/:id/event-settings", eventSettingsHandler.HandleUpdateEventSettings)
		protected.GET("/:id/creditroll", creditRollHandler.HandleGetConfig)
		protected.POST("/:id/creditroll", creditRollHandler.HandleUpdateConfig)
		protected.POST("/:id/mock-messages", mockHandler.HandleSendMockMessage)

		// YouTube helper routes
		protected.POST("/youtube/resolve", youtubeHandler.ResolveChannel)

		// Maintenance upcoming (JWT-protected, non-admin users)
		protected.GET("/maintenance/upcoming", maintenanceHandler.HandleListUpcoming)

		// Internal API routes (called by other services like auth-service)
		internal := protected.Group("/internal/overlays")
		{
			internal.POST("/:id/sources/auto", sourcesHandler.HandleAddSourceAuto)
		}
	}

	// Admin routes (JWT + Admin role required)
	admin := router.Group("/admin")
	admin.Use(middleware.JWTAuth(config.JWTSecret))
	admin.Use(middleware.AdminOnly())
	{
		admin.GET("/overlays", adminHandler.ListOverlays)
		admin.GET("/overlays/:id/sources", adminHandler.GetOverlaySources)
		admin.GET("/sources", adminHandler.ListAllSources)
		admin.GET("/users/:id/overlays", adminHandler.GetUserOverlays)

		// Maintenance window management
		admin.POST("/maintenance", maintenanceHandler.HandleCreateMaintenance)
		admin.GET("/maintenance", maintenanceHandler.HandleListMaintenance)
		admin.DELETE("/maintenance/:id", maintenanceHandler.HandleDeleteMaintenance)
	}

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + config.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Info("Overlay Manager Service started",
			zap.String("port", config.Port),
			zap.String("mode", config.GinMode),
		)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// Graceful shutdown with 25-second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", zap.Error(err))
	}

	log.Info("Server exited")
}

// Config holds application configuration
type Config struct {
	Port                   string
	GinMode                string
	DatabaseHost           string
	DatabasePort           string
	DatabaseUser           string
	DatabasePassword       string
	DatabaseName           string
	RedisHost              string
	RedisPort              string
	JWTSecret              string
	MessageProcessorURL    string
	MessageProcessorAPIKey string
	TwitchClientID         string
	TwitchClientSecret     string
}

// loadConfig loads configuration from environment variables
func loadConfig() *Config {
	return &Config{
		Port:                   getEnv("PORT", "8082"),
		GinMode:                getEnv("GIN_MODE", "debug"),
		DatabaseHost:           getEnv("DATABASE_HOST", "localhost"),
		DatabasePort:           getEnv("DATABASE_PORT", "5432"),
		DatabaseUser:           getEnv("DATABASE_USER", "allchat"),
		DatabasePassword:       getEnv("DATABASE_PASSWORD", "allchat_dev_password"),
		DatabaseName:           getEnv("DATABASE_NAME", "allchat"),
		RedisHost:              getEnv("REDIS_HOST", "localhost"),
		RedisPort:              getEnv("REDIS_PORT", "6379"),
		JWTSecret:              getEnv("JWT_SECRET", "default-secret-change-in-production"),
		MessageProcessorURL:    getEnv("MESSAGE_PROCESSOR_URL", "http://message-processor:8087"),
		MessageProcessorAPIKey: getEnv("MESSAGE_PROCESSOR_API_KEY", ""),
		TwitchClientID:         getEnv("TWITCH_CLIENT_ID", ""),
		TwitchClientSecret:     getEnv("TWITCH_CLIENT_SECRET", ""),
	}
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// httpMetricsMiddleware records HTTP request count and duration metrics
func httpMetricsMiddleware(requests *prometheus.CounterVec, duration *prometheus.HistogramVec, service string) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		status := strconv.Itoa(c.Writer.Status())
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		requests.WithLabelValues(service, c.Request.Method, path, status).Inc()
		duration.WithLabelValues(service, c.Request.Method, path).Observe(time.Since(start).Seconds())
	}
}
