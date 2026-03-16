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
	"github.com/caesar/all-chat/services/api-gateway/replay"
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
	wsManager := wsconn.NewManager(log, gatewayMetrics, redisClient, db)

	// Initialize deletion replay buffer
	replayBuffer := replay.NewRedisDeletionReplayBuffer(redisClient, 60*time.Second)
	log.Info("Initialized deletion replay buffer", zap.Duration("ttl", 60*time.Second))

	// Create WebSocket health checker for state reconciliation
	healthChecker := wsconn.NewHealthChecker(wsManager, redisClient, log, gatewayMetrics)
	healthChecker.Start()
	defer healthChecker.Stop()

	// Create Redis Pub/Sub subscriber with message handler
	messageHandler := func(overlayID string, channel string, message []byte) {
		// Determine message type based on channel
		// Main channel: overlay:{id} -> regular messages/events
		// Update channel: overlay:{id}:updates -> TikTok like aggregate updates
		msgType := models.WSMessageTypeChatMessage
		if len(channel) > 8 && channel[len(channel)-8:] == ":updates" {
			msgType = models.WSMessageTypeMessageUpdate
		}

		// Check if this is a deletion event for replay buffer
		// Parse the message to detect deletion events
		var unifiedMsg struct {
			Platform string `json:"platform"`
			Event    *struct {
				Type     string                 `json:"type"`
				Metadata map[string]interface{} `json:"metadata"`
			} `json:"event"`
		}
		if err := json.Unmarshal(message, &unifiedMsg); err == nil && unifiedMsg.Event != nil && unifiedMsg.Event.Type == "message_deletion" {
			// Add deletion event to replay buffer (best-effort, don't fail broadcast)
			deletionEvent := &replay.DeletionEvent{
				Platform:  unifiedMsg.Platform,
				Timestamp: time.Now().UTC(),
			}

			// Extract deletion type and target from metadata
			if delType, ok := unifiedMsg.Event.Metadata["deletion_type"].(string); ok {
				deletionEvent.DeletionType = delType
			}
			if targetUUID, ok := unifiedMsg.Event.Metadata["target_uuid"].(string); ok {
				deletionEvent.TargetUUID = targetUUID
			}
			if targetUserID, ok := unifiedMsg.Event.Metadata["target_user_id"].(string); ok {
				deletionEvent.TargetUserID = targetUserID
			}

			if err := replayBuffer.Add(context.Background(), overlayID, deletionEvent); err != nil {
				log.Error("Failed to add deletion to replay buffer",
					zap.String("overlay_id", overlayID),
					zap.Error(err),
				)
				// Continue - Pub/Sub broadcast is more critical than replay buffer
			} else {
				log.Debug("Added deletion to replay buffer",
					zap.String("overlay_id", overlayID),
					zap.String("type", deletionEvent.DeletionType),
				)
			}
		}

		// Wrap the unified message in a WebSocket message envelope
		wsMsg := models.WSMessage{
			Type:      msgType,
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

	// Create status subscriber for platform connection status
	statusSubscriber := subscription.NewStatusSubscriber(redisClient, wsManager, log)
	if err := statusSubscriber.Start(ctx); err != nil {
		log.Fatal("Failed to start status subscriber", zap.Error(err))
	}
	defer statusSubscriber.Stop()

	// Create repository for overlay verification
	subRepo := subscription.NewRepository(db)

	// Get JWT secret from environment
	jwtSecret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}

	// Get Twitch API credentials for badge fetching
	// For automatic token refresh, use TWITCH_CLIENT_SECRET (recommended)
	// Otherwise, provide TWITCH_ACCESS_TOKEN manually
	twitchClientID := strings.TrimSpace(os.Getenv("TWITCH_CLIENT_ID"))
	twitchClientSecret := strings.TrimSpace(os.Getenv("TWITCH_CLIENT_SECRET"))
	if twitchClientID == "" {
		log.Fatal("TWITCH_CLIENT_ID environment variable is required")
	}
	if twitchClientSecret == "" {
		log.Warn("TWITCH_CLIENT_SECRET not set - badge API will not auto-refresh tokens and may fail")
	}

	// Create handlers
	proxyHandler := handlers.NewProxyHandler(registry)
	healthHandler := handlers.NewHealthHandler(registry)
	badgeHandler := handlers.NewTwitchBadgeHandler(log, twitchClientID, twitchClientSecret)
	statsHandler := handlers.NewStatsHandler(redisClient)
	wsHandler := handlers.NewWebSocketHandler(wsManager, subscriber, subRepo, jwtSecret, replayBuffer, log)

	// Create viewer WebSocket handler (same origin policy as owner handler)
	viewerWsHandler := handlers.NewViewerWebSocketHandler(
		wsManager,
		subscriber,
		subRepo,
		jwtSecret,
		replayBuffer,
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
		// Platform message stats (last 24h)
		publicAPI.GET("/stats", statsHandler.GetPlatformStats)

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
		publicAPI.GET("/auth/discord/callback", proxyHandler.ForwardRequest)

		// Viewer OAuth routes (for sending messages)
		publicAPI.GET("/auth/viewer/twitch/login", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/viewer/twitch/callback", proxyHandler.ForwardRequest)
		publicAPI.POST("/auth/viewer/twitch/exchange", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/viewer/youtube/login", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/viewer/youtube/callback", proxyHandler.ForwardRequest)
		publicAPI.POST("/auth/viewer/youtube/exchange", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/viewer/kick/login", proxyHandler.ForwardRequest)
		publicAPI.GET("/auth/viewer/kick/callback", proxyHandler.ForwardRequest)
		publicAPI.POST("/auth/viewer/kick/exchange", proxyHandler.ForwardRequest)

		// Streamer info (public)
		publicAPI.GET("/auth/streamers/:username", proxyHandler.ForwardRequest)

		// Emote service routes (public)
		publicAPI.GET("/emotes/*path", proxyHandler.ForwardRequest)

		// Overlay config for public overlays
		publicAPI.GET("/overlays/public/:id/config", proxyHandler.ForwardRequest)
		publicAPI.GET("/overlays/public/:id/creditroll", proxyHandler.ForwardRequest)
		publicAPI.GET("/overlays/public/:id/credit-roll", proxyHandler.ForwardRequest)

		// Viewer cosmetics catalog (public — no auth required)
		publicAPI.GET("/auth/viewer/catalog/frames", proxyHandler.ForwardRequest) // -> auth-service
		publicAPI.GET("/auth/viewer/catalog/flairs", proxyHandler.ForwardRequest) // -> auth-service
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

		// Discord guild management
		protectedAPI.GET("/auth/discord/connect", proxyHandler.ForwardRequest)
		protectedAPI.GET("/auth/guilds", proxyHandler.ForwardRequest)
		protectedAPI.GET("/auth/guilds/:guild_id/channels", proxyHandler.ForwardRequest)
		protectedAPI.DELETE("/auth/guilds/:guild_id", proxyHandler.ForwardRequest)

		// Viewer protected routes
		protectedAPI.GET("/auth/viewer/me", proxyHandler.ForwardRequest)
		protectedAPI.POST("/auth/viewer/logout", proxyHandler.ForwardRequest)
		protectedAPI.POST("/auth/viewer/chat/send", proxyHandler.ForwardRequest)
		protectedAPI.PATCH("/auth/viewer/cosmetics", proxyHandler.ForwardRequest)

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
		protectedAPI.PATCH("/overlays/:id/sources/:source_id", proxyHandler.ForwardRequest)
		protectedAPI.DELETE("/overlays/:id/sources/:source_id", proxyHandler.ForwardRequest)
		protectedAPI.GET("/overlays/:id/event-settings", proxyHandler.ForwardRequest)
		protectedAPI.PUT("/overlays/:id/event-settings", proxyHandler.ForwardRequest)
		protectedAPI.GET("/overlays/:id/creditroll", proxyHandler.ForwardRequest)
		protectedAPI.POST("/overlays/:id/creditroll", proxyHandler.ForwardRequest)
		protectedAPI.GET("/overlays/:id/credit-roll", proxyHandler.ForwardRequest)

		// YouTube resolver routes (protected)
		protectedAPI.POST("/youtube/resolve", proxyHandler.ForwardRequest)
		protectedAPI.POST("/overlays/youtube/resolve", proxyHandler.ForwardRequest)

		// Internal API routes (protected - used by other services)
		protectedAPI.POST("/internal/overlays/:id/sources/auto", proxyHandler.ForwardRequest)

		// Share service routes (all protected - require JWT auth)
		protectedAPI.GET("/users/search", proxyHandler.ForwardRequest)                    // -> share-service
		protectedAPI.GET("/shares/incoming", proxyHandler.ForwardRequest)                 // -> share-service
		protectedAPI.GET("/shares/accepted", proxyHandler.ForwardRequest)                 // -> share-service (Phase 16)
		protectedAPI.GET("/shares/unseen-acceptances", proxyHandler.ForwardRequest)       // -> share-service
		protectedAPI.POST("/shares", proxyHandler.ForwardRequest)                         // -> share-service
		protectedAPI.POST("/shares/:id/accept", proxyHandler.ForwardRequest)              // -> share-service
		protectedAPI.POST("/shares/:id/reject", proxyHandler.ForwardRequest)              // -> share-service
		protectedAPI.POST("/shares/:id/revoke", proxyHandler.ForwardRequest)              // -> share-service
		protectedAPI.POST("/shares/:id/mark-seen", proxyHandler.ForwardRequest)           // -> share-service
		protectedAPI.POST("/admin/premium/users/:id", proxyHandler.ForwardRequest)         // -> share-service

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
		protectedAPI.GET("/admin/overlays/:id/sources", proxyHandler.ForwardRequest) // -> overlay-manager
		protectedAPI.GET("/admin/sources", proxyHandler.ForwardRequest)         // -> overlay-manager

		// Admin cosmetics catalog management (protected — JWT auth; admin role enforced at auth-service)
		protectedAPI.GET("/admin/cosmetics/frames", proxyHandler.ForwardRequest)          // -> auth-service
		protectedAPI.POST("/admin/cosmetics/frames", proxyHandler.ForwardRequest)         // -> auth-service
		protectedAPI.DELETE("/admin/cosmetics/frames/:id", proxyHandler.ForwardRequest)   // -> auth-service
		protectedAPI.GET("/admin/cosmetics/flairs", proxyHandler.ForwardRequest)          // -> auth-service
		protectedAPI.POST("/admin/cosmetics/flairs", proxyHandler.ForwardRequest)         // -> auth-service
		protectedAPI.DELETE("/admin/cosmetics/flairs/:id", proxyHandler.ForwardRequest)   // -> auth-service
	}

	// Internal routes (service-to-service, no auth for MVP - rely on network isolation)
	// TODO: Add service-to-service auth for production
	internal := router.Group("/internal")
	{
		internal.POST("/ws/notify", wsHandler.NotifyUser)
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
