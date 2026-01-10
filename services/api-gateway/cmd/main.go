package main

// Updated: 2025-12-19 20:57 - Viewer JWT checked first in middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/api-gateway/handlers"
	localmiddleware "github.com/caesar/all-chat/services/api-gateway/middleware"
	"github.com/caesar/all-chat/services/api-gateway/models"
	sharedmiddleware "github.com/caesar/all-chat/shared/middleware"
	"github.com/caesar/all-chat/services/api-gateway/subscription"
	wsconn "github.com/caesar/all-chat/services/api-gateway/websocket"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/caesar/all-chat/shared/ratelimit"
	"github.com/caesar/all-chat/shared/tracing"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logLevel := getEnvOrDefault("LOG_LEVEL", "info")
	log := logger.NewLogger("api-gateway", logLevel)
	defer log.Sync()

	log.Info("Starting API Gateway",
		zap.String("version", getEnvOrDefault("APP_VERSION", "dev")),
	)

	// Initialize OpenTelemetry tracing
	tracingEnabled := getEnvOrDefault("OTEL_ENABLED", "false") == "true"
	if tracingEnabled {
		tracingCfg := tracing.Config{
			ServiceName:    "api-gateway",
			ServiceVersion: getEnvOrDefault("APP_VERSION", "dev"),
			Environment:    getEnvOrDefault("ENVIRONMENT", "development"),
			OTLPEndpoint:   getEnvOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
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

	ctx := context.Background()

	// Connect to PostgreSQL (for WebSocket overlay verification)
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

	// Connect to Redis (for WebSocket Pub/Sub)
	redisHost := getEnvOrDefault("REDIS_HOST", "localhost")
	redisPort := getEnvOrDefault("REDIS_PORT", "6379")
	redisClient := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort),
	})
	defer redisClient.Close()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	log.Info("Connected to Redis")

	// Migrate legacy connection tracking (one-time cleanup)
	migrateLegacyConnectionTracking(ctx, redisClient, log)

	// Initialize rate limiter (configurable via env vars)
	rateLimitPerMin := getEnvAsIntOrDefault("RATE_LIMIT_PER_MINUTE", 300) // Default: 300 requests/min per IP/user
	rateLimiter := ratelimit.NewRateLimiter(ratelimit.Config{
		RequestsPerMinute: rateLimitPerMin,
		KeyPrefix:         "api_gateway",
		RedisClient:       redisClient,
		Logger:            log,
	})
	log.Info("Initialized rate limiter",
		zap.Int("requests_per_minute", rateLimitPerMin),
	)

	// Initialize metrics (available via /metrics endpoint)
	gatewayMetrics := metrics.NewGatewayMetrics()
	log.Info("Initialized Prometheus metrics")

	// Load service registry from environment
	registry, err := models.NewServiceRegistry()
	if err != nil {
		log.Fatal("Failed to initialize service registry", zap.Error(err))
	}

	log.Info("Service registry initialized",
		zap.Int("services", len(registry.Services)),
	)

	// Create WebSocket components
	wsManager := wsconn.NewManager(log, gatewayMetrics, redisClient)

	// Create WebSocket health checker for state reconciliation
	healthChecker := wsconn.NewHealthChecker(wsManager, redisClient, log, gatewayMetrics)
	healthChecker.Start()
	defer healthChecker.Stop()

	// Create Redis Pub/Sub subscriber with message handler
	messageHandler := func(overlayID string, message []byte) {
		// Wrap the unified message in a WebSocket message envelope
		wsMsg := models.WSMessage{
			Type:      models.WSMessageTypeChatMessage,
			Data:      json.RawMessage(message), // Use RawMessage to avoid re-parsing
			Timestamp: time.Now().UTC(),
		}

		// Convert to JSON
		wsJSON, err := wsMsg.ToJSON()
		if err != nil {
			log.Error("Failed to wrap message in WebSocket envelope",
				zap.String("overlay_id", overlayID),
				zap.Error(err),
			)
			return
		}

		// Broadcast wrapped message to all connections in this overlay
		count := wsManager.BroadcastToOverlay(overlayID, wsJSON)
		log.Debug("Broadcast message to overlay",
			zap.String("overlay_id", overlayID),
			zap.Int("connections", count),
		)
	}

	subscriber := subscription.NewSubscriber(redisClient, log, messageHandler)
	defer subscriber.Stop()

	// Create repository for overlay verification
	subRepo := subscription.NewRepository(db)

	// Get JWT secret from environment
	jwtSecret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}

	// Get Twitch API credentials for badge fetching
	twitchClientID := strings.TrimSpace(os.Getenv("TWITCH_CLIENT_ID"))
	twitchAccessToken := strings.TrimSpace(os.Getenv("TWITCH_ACCESS_TOKEN"))
	if twitchClientID == "" || twitchAccessToken == "" {
		log.Fatal("TWITCH_CLIENT_ID and TWITCH_ACCESS_TOKEN environment variables are required")
	}

	// Create handlers
	proxyHandler := handlers.NewProxyHandler(registry)
	healthHandler := handlers.NewHealthHandler(registry)
	badgeHandler := handlers.NewTwitchBadgeHandler(log, twitchClientID, twitchAccessToken)
	wsHandler := handlers.NewWebSocketHandler(wsManager, subscriber, subRepo, jwtSecret, log)

	// Create viewer WebSocket handler (same origin policy as owner handler)
	viewerWsHandler := handlers.NewViewerWebSocketHandler(
		wsManager,
		subscriber,
		subRepo,
		jwtSecret,
		log,
	)

	// Set Gin mode
	if logLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create router
	router := gin.New()

	// Apply global middleware
	router.Use(gin.Recovery()) // Panic recovery
	router.Use(localmiddleware.Logging(log))

	// Add tracing middleware if enabled
	if tracingEnabled {
		router.Use(tracing.GinMiddleware("api-gateway"))
	}

	// CORS middleware - but skip for WebSocket routes
	router.Use(func(c *gin.Context) {
		// Skip CORS for WebSocket routes (they handle origin checking in upgrader)
		if strings.HasPrefix(c.Request.URL.Path, "/ws/") {
			c.Next()
			return
		}
		localmiddleware.CORS()(c)
	}) // TODO: Update to CORSFromEnv() after shared module rebuild

	// Rate limiting middleware - skip for health, metrics, WebSocket, and static files
	router.Use(func(c *gin.Context) {
		path := c.Request.URL.Path
		// Skip rate limiting for:
		// - Health checks (monitoring)
		// - Metrics (monitoring)
		// - WebSocket connections (different connection model)
		// - Static files (legal pages)
		if path == "/health" || path == "/metrics" ||
			strings.HasPrefix(path, "/ws/") ||
			strings.HasPrefix(path, "/legal/") {
			c.Next()
			return
		}
		rateLimiter.Middleware()(c)
	})

	// Health check endpoint (no auth required)
	router.GET("/health", healthHandler.CheckHealth)

	// Prometheus metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Static legal pages (no auth required)
	router.StaticFile("/legal/terms", "./static/legal/terms.html")
	router.StaticFile("/legal/privacy", "./static/legal/privacy.html")

	// WebSocket endpoint for overlay owners/OBS (triggers YouTube polling)
	router.GET("/ws/overlay/:overlay_id", wsHandler.HandleOverlayConnection)

	// WebSocket endpoint for viewers (does NOT trigger polling, no overlay ID exposed)
	router.GET("/ws/chat/:streamer_username", viewerWsHandler.HandleViewerChatConnection)

	// API routes - Group by authentication requirements

	// Public routes (no auth required)
	publicAPI := router.Group("/api/v1")
	{
		// Auth service routes
		publicAPI.POST("/auth/login", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/login", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/callback", proxyHandler.ForwardRequest)
		publicAPI.POST("/auth/refresh", proxyHandler.ForwardRequest)

		// Platform-specific OAuth routes
		publicAPI.GET("/auth/twitch/login", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/twitch/callback", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/twitch/add-source/:overlay_id", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/youtube/login", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/youtube/callback", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/youtube/add-source/:overlay_id", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/kick/login", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/kick/callback", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/kick/add-source/:overlay_id", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/tiktok/login", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/tiktok/callback", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/tiktok/add-source/:overlay_id", proxyHandler.ForwardRequest)

		// Viewer OAuth routes (for sending messages)
		publicAPI.GET("/auth/viewer/twitch/login", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/viewer/twitch/callback", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/viewer/youtube/login", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/viewer/youtube/callback", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/viewer/kick/login", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/viewer/kick/callback", proxyHandler.ForwardRequest)

		// Streamer info (public)
		publicAPI.GET("/auth/streamers/:username", proxyHandler.ForwardRequest)

		// Emote service routes (public)
		publicAPI.GET("/emotes/*path", proxyHandler.ForwardRequest)

		// Overlay config for public overlays
		publicAPI.GET("/overlays/public/:id/config", proxyHandler.ForwardRequest)
	}

	// Twitch badge proxy endpoints (public, but not part of /api/v1 service registry)
	router.GET("/api/twitch/badges/global", badgeHandler.GetGlobalBadges)
	router.GET("/api/twitch/badges/channels/:room_id", badgeHandler.GetChannelBadges)

	// Protected routes (JWT auth required for streamers/admins and viewers)
	protectedAPI := router.Group("/api/v1")
	protectedAPI.Use(sharedmiddleware.JWTAuth(jwtSecret))
	{
		// Auth service - protected routes
		protectedAPI.GET("/auth/me", proxyHandler.ForwardRequest)
		protectedAPI.POST("/auth/logout", proxyHandler.ForwardRequest)
		protectedAPI.DELETE("/auth/me", proxyHandler.ForwardRequest)

		// Viewer protected routes
		protectedAPI.GET("/auth/viewer/me", proxyHandler.ForwardRequest)
		protectedAPI.POST("/auth/viewer/logout", proxyHandler.ForwardRequest)
		protectedAPI.POST("/auth/viewer/chat/send", proxyHandler.ForwardRequest)

		// Overlay manager routes (all protected)
		protectedAPI.GET("/overlays", proxyHandler.ForwardRequest)
		protectedAPI.POST("/overlays", proxyHandler.ForwardRequest)
		protectedAPI.GET("/overlays/:id", proxyHandler.ForwardRequest)
		protectedAPI.PUT("/overlays/:id", proxyHandler.ForwardRequest)
		protectedAPI.DELETE("/overlays/:id", proxyHandler.ForwardRequest)
		protectedAPI.GET("/overlays/:id/config", proxyHandler.ForwardRequest)
		protectedAPI.PUT("/overlays/:id/config", proxyHandler.ForwardRequest)
		protectedAPI.GET("/overlays/:id/sources", proxyHandler.ForwardRequest)
		protectedAPI.POST("/overlays/:id/sources", proxyHandler.ForwardRequest)
		protectedAPI.POST("/overlays/:id/mock-messages", proxyHandler.ForwardRequest)
		protectedAPI.PUT("/overlays/:id/sources/:source_id", proxyHandler.ForwardRequest)
		protectedAPI.DELETE("/overlays/:id/sources/:source_id", proxyHandler.ForwardRequest)

		// YouTube resolver routes (protected)
		protectedAPI.POST("/youtube/resolve", proxyHandler.ForwardRequest)
		protectedAPI.POST("/overlays/youtube/resolve", proxyHandler.ForwardRequest)

		// Internal API routes (protected - used by other services)
		protectedAPI.POST("/internal/overlays/:id/sources/auto", proxyHandler.ForwardRequest)

		// Admin routes (protected - TODO: add admin role check)
		protectedAPI.GET("/admin/users", proxyHandler.ForwardRequest)           // -> auth-service
		protectedAPI.GET("/admin/users/:id", proxyHandler.ForwardRequest)       // -> auth-service
		protectedAPI.POST("/admin/users/:id/impersonate", proxyHandler.ForwardRequest) // -> auth-service

		// Admin user ban management
		protectedAPI.POST("/admin/users/:id/ban", proxyHandler.ForwardRequest)   // -> auth-service
		protectedAPI.POST("/admin/users/:id/unban", proxyHandler.ForwardRequest) // -> auth-service
		protectedAPI.GET("/admin/users/banned", proxyHandler.ForwardRequest)     // -> auth-service

		// Admin stats
		protectedAPI.GET("/admin/stats", proxyHandler.ForwardRequest)           // -> auth-service

		// Admin viewer management routes
		protectedAPI.GET("/admin/viewers", proxyHandler.ForwardRequest)         // -> auth-service
		protectedAPI.POST("/admin/viewers/:session_id/ban", proxyHandler.ForwardRequest)   // -> auth-service
		protectedAPI.POST("/admin/viewers/:session_id/unban", proxyHandler.ForwardRequest) // -> auth-service
		protectedAPI.GET("/admin/overlays", proxyHandler.ForwardRequest)        // -> overlay-manager
		protectedAPI.GET("/admin/sources", proxyHandler.ForwardRequest)         // -> overlay-manager
	}

	// Get port from environment
	port := getEnvOrDefault("PORT", "8080")

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Info("API Gateway listening",
			zap.String("port", port),
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

	// Give outstanding requests 25 seconds to complete
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	// Shutdown WebSocket manager first (cleans up Redis state)
	if err := wsManager.Shutdown(shutdownCtx); err != nil {
		log.Error("WebSocket manager shutdown error", zap.Error(err))
	}

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("Server forced to shutdown", zap.Error(err))
	}

	log.Info("Server exited")
}

// getEnvOrDefault gets an environment variable or returns a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsIntOrDefault gets an environment variable as int or returns a default value
func getEnvAsIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// migrateLegacyConnectionTracking removes the old Redis SET used for connection tracking
// The system now uses individual TTL keys (overlay:connected:{id}) exclusively
func migrateLegacyConnectionTracking(ctx context.Context, redisClient *redis.Client, log *zap.Logger) {
	legacyKey := "overlay:connected"

	// Check if legacy SET exists
	exists, err := redisClient.Exists(ctx, legacyKey).Result()
	if err != nil {
		log.Error("Failed to check legacy connection SET", zap.Error(err))
		return
	}

	if exists == 0 {
		log.Debug("No legacy connection SET found, migration not needed")
		return
	}

	// Get member count for logging
	count, err := redisClient.SCard(ctx, legacyKey).Result()
	if err != nil {
		log.Warn("Failed to get legacy SET member count", zap.Error(err))
	}

	// Delete the SET
	if err := redisClient.Del(ctx, legacyKey).Err(); err != nil {
		log.Error("Failed to delete legacy connection SET", zap.Error(err))
		return
	}

	log.Info("Removed legacy connection SET (health checker now uses TTL keys)",
		zap.Int64("members_removed", count),
	)
}
