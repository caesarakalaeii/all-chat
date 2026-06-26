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
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/message-processor/registry"
	"github.com/caesar/all-chat/services/youtube-listener/handlers"
	ytmetrics "github.com/caesar/all-chat/services/youtube-listener/metrics"
	"github.com/caesar/all-chat/services/youtube-listener/notifications"
	"github.com/caesar/all-chat/services/youtube-listener/oauth"
	"github.com/caesar/all-chat/services/youtube-listener/publisher"
	"github.com/caesar/all-chat/services/youtube-listener/quota"
	"github.com/caesar/all-chat/services/youtube-listener/status"
	"github.com/caesar/all-chat/services/youtube-listener/streams"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/encryption"
	"github.com/caesar/all-chat/shared/listener"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/caesar/all-chat/shared/tracing"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	// Enable gRPC debug logging if requested
	if os.Getenv("GRPC_GO_LOG_VERBOSITY_LEVEL") != "" {
		os.Setenv("GRPC_GO_LOG_SEVERITY_LEVEL", "info")
	}
	if os.Getenv("GRPC_TRACE") != "" {
		// GRPC_TRACE=all enables all gRPC tracing
		// Useful values: http, api, channel, connectivity
	}

	// Initialize logger
	logLevel := listener.Env("LOG_LEVEL", "info")
	log := logger.NewLogger("youtube-listener", logLevel)
	defer log.Sync()

	log.Info("Starting YouTube Listener",
		zap.String("version", listener.Env("APP_VERSION", "dev")),
		zap.String("grpc_log_level", os.Getenv("GRPC_GO_LOG_VERBOSITY_LEVEL")),
		zap.String("grpc_trace", os.Getenv("GRPC_TRACE")),
	)

	// Initialize tracing
	tracingEnabled := listener.Env("OTEL_ENABLED", "false") == "true"
	if tracingEnabled {
		tracingCfg := tracing.Config{
			ServiceName:    "youtube-listener",
			ServiceVersion: listener.Env("APP_VERSION", "dev"),
			Environment:    listener.Env("ENVIRONMENT", "development"),
			OTLPEndpoint:   listener.Env("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
			Enabled:        true,
		}
		shutdownTracer, err := tracing.InitTracer(tracingCfg, log)
		if err != nil {
			log.Error("Failed to initialize tracer", zap.Error(err))
		} else {
			defer shutdownTracer(context.Background())
			log.Info("Tracer initialized", zap.String("service", "youtube-listener"))
		}
	}

	ctx := context.Background()

	// Validate required environment variables
	youtubeClientID := os.Getenv("YOUTUBE_CLIENT_ID")
	youtubeClientSecret := os.Getenv("YOUTUBE_CLIENT_SECRET")
	frontendURL := strings.TrimSuffix(listener.Env("FRONTEND_URL", "http://localhost:3000"), "/")
	youtubeRedirectURL := defaultCallbackURL(frontendURL, "http://localhost:8080", "/api/v1/auth/youtube/callback")

	if youtubeClientID == "" || youtubeClientSecret == "" {
		log.Fatal("YOUTUBE_CLIENT_ID and YOUTUBE_CLIENT_SECRET are required")
	}

	// Initialize multi-key encryptor for YouTube OAuth tokens (D-04 unified chain).
	// NewMultiKeyEncryptorFromEnv reads TOKEN_ENCRYPTION_KEY_V1 (required for new writes)
	// and YOUTUBE_TOKEN_ENCRYPTION_KEY as a legacy fallback so that tokens encrypted
	// before Phase 14 still decrypt transparently. TOKEN_ENCRYPTION_KEY is also a
	// legacy fallback (unified chain, D-04).
	tokenEncryptor, err := encryption.NewMultiKeyEncryptorFromEnvWithLogger(log)
	if err != nil {
		log.Fatal("Failed to initialize token encryptor (TOKEN_ENCRYPTION_KEY_V1 must be set)", zap.Error(err))
	}
	log.Info("YouTube token cipher initialized — unified chain reads TOKEN_ENCRYPTION_KEY_V<n>; legacy fallback also reads YOUTUBE_TOKEN_ENCRYPTION_KEY",
		zap.Uint8("current_kid", tokenEncryptor.CurrentKid()))

	// Connect to PostgreSQL
	dbHost := listener.Env("DATABASE_HOST", "localhost")
	dbPort := listener.Env("DATABASE_PORT", "5432")
	dbUser := listener.Env("DATABASE_USER", "allchat")
	dbPassword := listener.Env("DATABASE_PASSWORD", "")
	if dbPassword == "" {
		log.Fatal("DATABASE_PASSWORD must be set")
	}
	dbName := listener.Env("DATABASE_NAME", "allchat")

	connString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	db, err := database.NewPostgresPool(connString)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	log.Info("Connected to PostgreSQL")

	// Connect to Redis
	redisHost := listener.Env("REDIS_HOST", "localhost")
	redisPort := listener.Env("REDIS_PORT", "6379")
	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", redisHost, redisPort),
		Password: listener.Env("REDIS_PASSWORD", ""),
	})
	defer redisClient.Close()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	log.Info("Connected to Redis")

	// Initialize Message ID Registry for deletion event matching
	msgIDRegistry := registry.NewRedisRegistry(redisClient, 1*time.Hour)
	log.Info("Initialized Message ID Registry", zap.Duration("ttl", 1*time.Hour))

	// Initialize metrics (available via /metrics endpoint)
	listenerMetrics := metrics.NewListenerMetrics("youtube", "youtube-listener")
	log.Info("Initialized Prometheus metrics")

	// Initialize components
	tokenStore := oauth.NewPostgresTokenStore(db, tokenEncryptor, log)
	oauthManager := oauth.NewManager(youtubeClientID, youtubeClientSecret, youtubeRedirectURL, tracingEnabled, tokenStore, log)

	streamPublisher := publisher.NewStreamPublisher(redisClient, log)

	// Initialize quota tracker (legacy - kept for backward compatibility)
	quotaLimitStr := listener.Env("QUOTA_LIMIT_DAILY", "1009000")
	quotaLimit, err := strconv.Atoi(quotaLimitStr)
	if err != nil {
		log.Warn("Invalid QUOTA_LIMIT_DAILY, using default 1009000", zap.Error(err))
		quotaLimit = 1009000
	}

	quotaTracker := quota.NewTracker(db, quotaLimit, log, listenerMetrics)
	if err := quotaTracker.Start(ctx); err != nil {
		log.Fatal("Failed to start quota tracker", zap.Error(err))
	}

	// Wire the quota notifier so threshold/state-transition events publish to the
	// shared "quota:alerts" Redis channel the discord-bot subscribes to. Without this
	// SetNotifier call the tracker's notifier stayed nil and quota alerts never fired.
	// Gated by QUOTA_NOTIFIER_ENABLED (default on); the tracker no-ops on a nil notifier.
	quotaNotifierEnabled := listener.Env("QUOTA_NOTIFIER_ENABLED", "true") == "true"
	quotaNotifier := notifications.NewQuotaNotifier(redisClient, log, quotaNotifierEnabled, "quota:alerts")
	quotaTracker.SetNotifier(quotaNotifier)

	// Initialize per-channel quota tracker (new)
	perChannelQuotaConfig := quota.Config{
		GlobalDailyQuota:  parseIntEnv("YOUTUBE_GLOBAL_DAILY_QUOTA", 1009000),
		HighTierQuota:     parseIntEnv("YOUTUBE_HIGH_TIER_QUOTA", 200),
		StandardTierQuota: parseIntEnv("YOUTUBE_STANDARD_TIER_QUOTA", 100),
		LowTierQuota:      parseIntEnv("YOUTUBE_LOW_TIER_QUOTA", 50),
	}
	perChannelQuotaTracker := quota.NewPerChannelTracker(db, redisClient, log, perChannelQuotaConfig)

	// Start daily quota reset scheduler
	go func() {
		// Calculate time until next midnight PST (YouTube's quota reset timezone)
		now := time.Now().In(quota.YouTubePST)
		nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, quota.YouTubePST)
		durationUntilMidnight := nextMidnight.Sub(now)

		log.Info("Daily quota reset scheduler started",
			zap.Duration("time_until_first_reset", durationUntilMidnight),
		)

		// Wait until midnight, then reset every 24 hours
		timer := time.NewTimer(durationUntilMidnight)
		defer timer.Stop()

		for {
			select {
			case <-timer.C:
				log.Info("Performing daily quota reset")
				if err := perChannelQuotaTracker.ResetDailyQuotas(context.Background()); err != nil {
					log.Error("Failed to reset daily quotas", zap.Error(err))
				}

				// Also demote inactive channels
				if err := perChannelQuotaTracker.DemoteInactiveChannels(context.Background()); err != nil {
					log.Error("Failed to demote inactive channels", zap.Error(err))
				}

				// Schedule next reset in 24 hours
				timer.Reset(24 * time.Hour)

			case <-ctx.Done():
				log.Info("Quota reset scheduler stopped")
				return
			}
		}
	}()

	// Create message handler that publishes to Redis Streams and tracks quota
	messageHandler := NewMessageHandler(streamPublisher, quotaTracker, msgIDRegistry, listenerMetrics, log)

	ll, err := listener.NewLeadershipListenerFromEnv("youtube", redisClient, log)
	if err != nil {
		log.Fatal("Failed to initialize LeadershipListener", zap.Error(err))
	}

	// Initialize YouTube-specific metrics
	ytMetrics := ytmetrics.NewYouTubeMetrics()

	// Initialize status publisher for platform connection status
	statusPublisher := status.NewPublisher(redisClient, log)

	// Initialize stream manager
	streamRepo := streams.NewRepository(db, log)
	dbConnWrapper := &dbConnWrapper{pool: db}
	streamManager := streams.NewManager(streamRepo, oauthManager, messageHandler, dbConnWrapper, ll.LeadershipCoordinator(), quotaTracker, perChannelQuotaTracker, redisClient, ytMetrics, statusPublisher, log)

	// Start stream manager
	if err := streamManager.Start(ctx); err != nil {
		log.Fatal("Failed to start stream manager", zap.Error(err))
	}

	// Demand-driven activation (Phase 5)
	// Leadership-only listeners don't call base.Start so the SDK demand loop
	// doesn't run automatically. Subscribe directly to source:demand here.
	go func() {
		const platformFilter = "youtube"
		pubsub := redisClient.Subscribe(ctx, "source:demand")
		defer pubsub.Close()

		log.Info("Subscribed to source:demand for demand-driven activation",
			zap.String("platform", platformFilter))

		ch := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				var update struct {
					Type    string `json:"type"`
					Sources []struct {
						SourceID  string `json:"source_id"`
						ChannelID string `json:"channel_id"`
						Platform  string `json:"platform"`
						OverlayID string `json:"overlay_id"`
					} `json:"sources"`
				}
				if err := json.Unmarshal([]byte(msg.Payload), &update); err != nil {
					log.Warn("Failed to parse demand update", zap.Error(err))
					continue
				}
				demanded := make(map[string]bool)
				for _, s := range update.Sources {
					if s.Platform == platformFilter {
						demanded[s.ChannelID] = true
					}
				}
				log.Info("Demand update received",
					zap.Int("total_sources", len(update.Sources)),
					zap.Int("platform_sources", len(demanded)))
				streamManager.UpdateDemandedChannels(demanded)
			}
		}
	}()

	// Set up HTTP server for health checks
	if logLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	if tracingEnabled {
		router.Use(tracing.GinMiddleware("youtube-listener"))
	}

	// Health check handlers
	healthHandler := handlers.NewHealthHandler(streamManager, streamPublisher, quotaTracker)
	router.GET("/health/live", healthHandler.LivenessProbe)
	router.GET("/health/ready", healthHandler.ReadinessProbe)
	router.GET("/status", healthHandler.Status)

	// Quota handlers
	quotaCoordinator := quota.NewCoordinator(quotaTracker, perChannelQuotaTracker, log)
	quotaHandler := handlers.NewQuotaHandler(quotaCoordinator, quotaTracker, perChannelQuotaTracker, log)

	// Wire up circuit breaker interfaces for visibility and admin control
	quotaHandler.SetCircuitBreakerGetter(streamManager)
	quotaHandler.SetCircuitBreakerResetter(streamManager)

	// Detection control handler (new endpoints for manual intervention)
	detectionHandler := handlers.NewDetectionHandler(streamManager, streamManager.GetQuotaBudget(), quotaTracker, log)

	router.GET("/quota/status", quotaHandler.GetQuotaStatus)
	router.GET("/quota/channels/:channel_id", quotaHandler.GetChannelQuota)
	router.GET("/quota/history", quotaHandler.GetQuotaHistory)
	router.GET("/quota/predictions", quotaHandler.GetQuotaPrediction)
	router.GET("/quota/circuit-breakers", quotaHandler.GetCircuitBreakers) // Circuit breaker visibility
	router.POST("/quota/record", quotaHandler.RecordQuota)                 // Legacy endpoint for external services

	// Cross-service quota coordination API
	v1 := router.Group("/api/v1")
	{
		v1.POST("/quota/check", quotaHandler.CheckQuota)     // Check if quota available
		v1.POST("/quota/reserve", quotaHandler.ReserveQuota) // Reserve before API call
		v1.POST("/quota/confirm", quotaHandler.ConfirmQuota) // Confirm or rollback after call
	}

	// Admin endpoints for manual intervention
	admin := router.Group("/admin")
	{
		admin.POST("/circuit-breakers/:channel_id/reset", quotaHandler.ResetCircuitBreaker)
		admin.POST("/circuit-breakers/reset-all", quotaHandler.ResetAllCircuitBreakers)

		// Detection control endpoints (new)
		admin.GET("/detection/channels/:channel_id", detectionHandler.GetChannelState)
		admin.GET("/detection/channels", detectionHandler.ListAllChannelStates)
		admin.POST("/detection/channels/:channel_id/reset-backoff", detectionHandler.ResetChannelBackoff)
		admin.POST("/detection/channels/:channel_id/force-check", detectionHandler.ForceChannelDetection)
		admin.POST("/detection/reset-all", detectionHandler.ResetAllBackoff)
		admin.GET("/detection/quota-budget", detectionHandler.GetQuotaBudgetStatus)
	}

	// Prometheus metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Get port
	port := listener.Env("PORT", "8086")

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
		)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start HTTP server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down service...")

	// Stop stream manager
	streamManager.Stop()

	// Stop quota tracker
	quotaTracker.Stop()

	// Shutdown HTTP server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP server forced to shutdown", zap.Error(err))
	}

	log.Info("Service exited")
}

// parseIntEnv parses an integer environment variable or returns default
func parseIntEnv(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func defaultCallbackURL(frontendURL, fallbackBase, path string) string {
	normalizedPath := path
	if !strings.HasPrefix(normalizedPath, "/") {
		normalizedPath = "/" + normalizedPath
	}

	trimmedFrontend := strings.TrimSuffix(frontendURL, "/")
	trimmedFallback := strings.TrimSuffix(fallbackBase, "/")
	lowerFrontend := strings.ToLower(trimmedFrontend)

	if trimmedFrontend == "" ||
		strings.Contains(lowerFrontend, "localhost") ||
		strings.Contains(lowerFrontend, "127.0.0.1") {
		return trimmedFallback + normalizedPath
	}

	if strings.HasPrefix(trimmedFrontend, "http://") || strings.HasPrefix(trimmedFrontend, "https://") {
		return trimmedFrontend + normalizedPath
	}

	return trimmedFallback + normalizedPath
}

// dbConnWrapper wraps pgxpool.Pool to implement DBConnInterface
type dbConnWrapper struct {
	pool *pgxpool.Pool
}

func (w *dbConnWrapper) GetPool() interface{} {
	return w.pool
}
