package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/message-processor/cache"
	"github.com/caesar/all-chat/services/message-processor/consumer"
	"github.com/caesar/all-chat/services/message-processor/dedup"
	"github.com/caesar/all-chat/services/message-processor/enricher"
	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/caesar/all-chat/services/message-processor/normalizer"
	"github.com/caesar/all-chat/services/message-processor/publisher"
	"github.com/caesar/all-chat/services/message-processor/router"
	"github.com/caesar/all-chat/services/message-processor/seventv"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/caesar/all-chat/shared/tracing"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logLevel := getEnvOrDefault("LOG_LEVEL", "info")
	log := logger.NewLogger("message-processor", logLevel)
	defer log.Sync()

	log.Info("Starting Message Processor",
		zap.String("version", getEnvOrDefault("APP_VERSION", "dev")),
	)

	// Initialize OpenTelemetry tracing
	tracingEnabled := getEnvOrDefault("OTEL_ENABLED", "false") == "true"
	if tracingEnabled {
		tracingCfg := tracing.Config{
			ServiceName:    "message-processor",
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

	// Parse message age cutoff (default 60 seconds)
	messageAgeCutoffSeconds := 60
	if cutoffStr := getEnvOrDefault("MESSAGE_AGE_CUTOFF_SECONDS", "60"); cutoffStr != "" {
		if parsed, err := time.ParseDuration(cutoffStr + "s"); err == nil {
			messageAgeCutoffSeconds = int(parsed.Seconds())
		} else {
			log.Warn("Invalid MESSAGE_AGE_CUTOFF_SECONDS, using default",
				zap.String("value", cutoffStr),
				zap.Int("default", 60),
			)
		}
	}
	messageAgeCutoff := time.Duration(messageAgeCutoffSeconds) * time.Second
	log.Info("Message age cutoff configured",
		zap.Duration("cutoff", messageAgeCutoff),
	)

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

	// Connect to Redis
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

	// Initialize metrics (available via /metrics endpoint)
	processorMetrics := metrics.NewProcessorMetrics()
	log.Info("Initialized Prometheus metrics")

	// Initialize components
	twitchNormalizer := normalizer.NewTwitchNormalizer()
	youtubeNormalizer := normalizer.NewYouTubeNormalizer()
	tiktokNormalizer := normalizer.NewTikTokNormalizer()
	kickNormalizer := normalizer.NewKickNormalizer()

	// Map of platform-specific normalizers
	normalizers := map[string]normalizer.Normalizer{
		"twitch":  twitchNormalizer,
		"youtube": youtubeNormalizer,
		"tiktok":  tiktokNormalizer,
		"kick":    kickNormalizer,
	}

	emoteServiceURL := getEnvOrDefault("EMOTE_SERVICE_URL", "http://localhost:8083")
	emoteClient := enricher.NewHTTPEmoteClient(emoteServiceURL, log)
	emoteCacheStore := cache.NewEmoteCache(redisClient, log, 0)
	emoteEnricher := enricher.NewEnricher(emoteClient, emoteCacheStore, log)

	// Initialize 7TV event manager for real-time emote updates
	seventvManager := seventv.NewManager(emoteCacheStore, log)
	if err := seventvManager.Start(ctx); err != nil {
		log.Warn("Failed to start 7TV event manager, continuing without real-time updates",
			zap.Error(err),
		)
	} else {
		log.Info("7TV event manager started successfully")
	}
	defer seventvManager.Stop()

	// Avatar enricher for Twitch users
	twitchClientID := getEnvOrDefault("TWITCH_CLIENT_ID", "")
	twitchClientSecret := getEnvOrDefault("TWITCH_CLIENT_SECRET", "")
	avatarEnricher := enricher.NewAvatarEnricher(redisClient, twitchClientID, twitchClientSecret, log)
	badgeEnricher := enricher.NewBadgeEnricher(redisClient, twitchClientID, twitchClientSecret, log)

	overlayRepo := router.NewRepository(db)
	overlayRouter := router.NewRouter(overlayRepo, log)

	pubsubPublisher := publisher.NewPubSubPublisher(redisClient, log)

	// Create deduplicator to prevent duplicate message publishing
	deduplicator := dedup.NewDeduplicator(redisClient, log)

	// Define message handler
	messageHandler := func(ctx context.Context, rawMsg *models.RawChatMessage) error {
		// Filter out old messages based on timestamp
		messageAge := time.Since(rawMsg.Timestamp)
		if messageAge > messageAgeCutoff {
			log.Debug("Ignoring old message",
				zap.String("message_id", rawMsg.MessageID),
				zap.String("platform", rawMsg.Platform),
				zap.String("channel_id", rawMsg.ChannelID),
				zap.Duration("message_age", messageAge),
				zap.Duration("cutoff", messageAgeCutoff),
				zap.Time("timestamp", rawMsg.Timestamp),
			)
			processorMetrics.RecordMessageProcessed("message-processor", rawMsg.Platform, "filtered_old", "success")
			return nil
		}

		// Check for duplicate messages (prevents double-publishing to overlays)
		isDup, dedupErr := deduplicator.IsDuplicate(ctx, rawMsg.Platform, rawMsg.ChannelID, rawMsg.UserID, rawMsg.Text, rawMsg.Timestamp)
		if dedupErr != nil {
			log.Warn("Deduplication check failed, processing message anyway",
				zap.Error(dedupErr),
				zap.String("message_id", rawMsg.MessageID),
			)
			// Continue processing on error (fail open)
		} else if isDup {
			log.Debug("Duplicate message detected, skipping",
				zap.String("platform", rawMsg.Platform),
				zap.String("channel", rawMsg.ChannelID),
				zap.String("user", rawMsg.UserID),
				zap.String("message_id", rawMsg.MessageID),
			)
			processorMetrics.RecordMessageProcessed("message-processor", rawMsg.Platform, "deduplicated", "skipped")
			return nil
		}

		// Find target overlays for this message
		var overlays []models.OverlayTarget
		if rawMsg.OverlayID != "" {
			overlays = []models.OverlayTarget{
				{OverlayID: rawMsg.OverlayID},
			}
		} else {
			routed, err := overlayRouter.Route(ctx, rawMsg.Platform, rawMsg.ChannelID)
			if err != nil {
				return fmt.Errorf("failed to route message: %w", err)
			}
			overlays = routed
		}

		if len(overlays) == 0 {
			log.Debug("No overlays found for message, skipping",
				zap.String("platform", rawMsg.Platform),
				zap.String("channel", rawMsg.ChannelID),
			)
			return nil
		}

		// Get the appropriate normalizer for this platform
		platformNormalizer, ok := normalizers[rawMsg.Platform]
		if !ok {
			log.Warn("Unknown platform, skipping message",
				zap.String("platform", rawMsg.Platform),
				zap.String("message_id", rawMsg.MessageID),
			)
			return nil
		}

		// Track channel for 7TV real-time emote updates (fire-and-forget)
		go func() {
			if err := seventvManager.TrackChannel(context.Background(), rawMsg.Platform, rawMsg.ChannelID); err != nil {
				log.Debug("Failed to track channel for 7TV updates",
					zap.String("platform", rawMsg.Platform),
					zap.String("channel_id", rawMsg.ChannelID),
					zap.Error(err),
				)
			}
		}()

		// Process message for each overlay
		for _, overlay := range overlays {
			// Normalize message using platform-specific normalizer
			startNormalize := time.Now()
			unified, err := platformNormalizer.Normalize(rawMsg, overlay.OverlayID)
			if err != nil {
				log.Warn("Failed to normalize message",
					zap.String("message_id", rawMsg.MessageID),
					zap.String("platform", rawMsg.Platform),
					zap.Error(err),
				)
				processorMetrics.RecordMessageProcessed("message-processor", rawMsg.Platform, "normalized", "failed")
				continue
			}
			processorMetrics.RecordMessageProcessed("message-processor", rawMsg.Platform, "normalized", "success")
			processorMetrics.StageDuration.WithLabelValues("message-processor", rawMsg.Platform, "normalization").Observe(time.Since(startNormalize).Seconds())

			// Enrich with avatars (Twitch only, cached in Redis)
			startAvatar := time.Now()
			if err := avatarEnricher.Enrich(ctx, unified); err != nil {
				log.Warn("Failed to enrich avatar",
					zap.String("message_id", rawMsg.MessageID),
					zap.Error(err),
				)
				// Continue even if enrichment fails
			}
			processorMetrics.StageDuration.WithLabelValues("message-processor", rawMsg.Platform, "avatar_enrichment").Observe(time.Since(startAvatar).Seconds())

			// Enrich with badge icons (Twitch only, cached in Redis)
			startBadge := time.Now()
			if err := badgeEnricher.Enrich(ctx, unified); err != nil {
				log.Warn("Failed to enrich badges",
					zap.String("message_id", rawMsg.MessageID),
					zap.Error(err),
				)
				// Continue even if enrichment fails
			}
			processorMetrics.StageDuration.WithLabelValues("message-processor", rawMsg.Platform, "badge_enrichment").Observe(time.Since(startBadge).Seconds())

			// Enrich with emotes
			startEmote := time.Now()
			if err := emoteEnricher.Enrich(ctx, unified); err != nil {
				log.Warn("Failed to enrich message",
					zap.String("message_id", rawMsg.MessageID),
					zap.Error(err),
				)
				processorMetrics.RecordMessageProcessed("message-processor", rawMsg.Platform, "enriched", "failed")
				// Continue even if enrichment fails
			} else {
				processorMetrics.RecordMessageProcessed("message-processor", rawMsg.Platform, "enriched", "success")
			}
			processorMetrics.StageDuration.WithLabelValues("message-processor", rawMsg.Platform, "emote_enrichment").Observe(time.Since(startEmote).Seconds())

			// Publish to overlay channel
			startPublish := time.Now()
			if err := pubsubPublisher.Publish(ctx, overlay.OverlayID, unified); err != nil {
				log.Error("Failed to publish to overlay",
					zap.String("overlay_id", overlay.OverlayID),
					zap.String("message_id", rawMsg.MessageID),
					zap.Error(err),
				)
				processorMetrics.RecordMessagePublished("message-processor", overlay.OverlayID, rawMsg.Platform, "failed")
				continue
			}
			processorMetrics.RecordMessagePublished("message-processor", overlay.OverlayID, rawMsg.Platform, "success")
			processorMetrics.FanoutDuration.WithLabelValues("message-processor").Observe(time.Since(startPublish).Seconds())
		}

		return nil
	}

	// Create and start stream consumer
	streamConsumer := consumer.NewStreamConsumer(redisClient, log, processorMetrics, messageHandler)
	if err := streamConsumer.Start(ctx); err != nil {
		log.Fatal("Failed to start stream consumer", zap.Error(err))
	}

	log.Info("Message processor started")

	// Set up HTTP server for health checks
	if logLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	// Add tracing middleware if enabled
	if tracingEnabled {
		router.Use(tracing.GinMiddleware("message-processor"))
	}

	// Health check endpoints
	router.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "alive"})
	})

	router.GET("/health/ready", func(c *gin.Context) {
		// Check Redis connection
		if err := redisClient.Ping(ctx).Err(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not_ready",
				"error":  "redis connection failed",
			})
			return
		}

		// Check database connection
		if err := db.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not_ready",
				"error":  "database connection failed",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	router.GET("/status", func(c *gin.Context) {
		pendingCount, _ := streamConsumer.GetPendingCount(ctx)

		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"consumer": gin.H{
				"pending_messages": pendingCount,
			},
		})
	})

	// Prometheus metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	mockAPIKey := getEnvOrDefault("MOCK_MESSAGE_API_KEY", "")
	router.POST("/internal/mock-messages", func(c *gin.Context) {
		if mockAPIKey == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "mock messaging disabled"})
			return
		}

		if token := c.GetHeader("X-Internal-Token"); token == "" || token != mockAPIKey {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req mockMessageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		normalizeMockRequest(&req)
		msg := buildMockMessage(&req)

		if err := emoteEnricher.Enrich(ctx, msg); err != nil {
			log.Warn("Failed to enrich mock message", zap.Error(err))
		}

		if err := pubsubPublisher.Publish(ctx, req.OverlayID, msg); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to publish mock message"})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{
			"status":     "queued",
			"message_id": msg.ID,
		})
	})

	// Get port
	port := getEnvOrDefault("PORT", "8087")

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

	// Stop 7TV event manager
	if err := seventvManager.Stop(); err != nil {
		log.Error("Failed to stop 7TV event manager", zap.Error(err))
	}

	// Stop stream consumer
	streamConsumer.Stop()

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

type mockMessageRequest struct {
	OverlayID   string                 `json:"overlay_id" binding:"required"`
	Platform    string                 `json:"platform"`
	ChannelID   string                 `json:"channel_id"`
	ChannelName string                 `json:"channel_name"`
	UserID      string                 `json:"user_id"`
	Username    string                 `json:"username"`
	DisplayName string                 `json:"display_name"`
	AvatarURL   string                 `json:"avatar_url"`
	Color       string                 `json:"color"`
	Badges      []models.Badge         `json:"badges"`
	Text        string                 `json:"text" binding:"required"`
	Metadata    map[string]interface{} `json:"metadata"`
}

func normalizeMockRequest(req *mockMessageRequest) {
	req.Platform = strings.ToLower(strings.TrimSpace(req.Platform))
	if req.Platform == "" {
		req.Platform = "twitch"
	}

	req.ChannelID = strings.TrimSpace(req.ChannelID)
	req.ChannelName = strings.TrimSpace(req.ChannelName)
	if req.ChannelID == "" {
		req.ChannelID = req.ChannelName
	}
	if req.ChannelID == "" {
		req.ChannelID = "mock-channel"
	}
	if req.ChannelName == "" {
		req.ChannelName = req.ChannelID
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		req.Username = "mockuser"
	}
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if req.DisplayName == "" {
		req.DisplayName = req.Username
	}
	req.UserID = strings.TrimSpace(req.UserID)
	if req.UserID == "" {
		req.UserID = "mock-user"
	}

	if req.Metadata == nil {
		req.Metadata = map[string]interface{}{}
	}
}

func buildMockMessage(req *mockMessageRequest) *models.UnifiedChatMessage {
	metadata := map[string]interface{}{
		"mock": true,
	}
	for k, v := range req.Metadata {
		metadata[k] = v
	}

	return &models.UnifiedChatMessage{
		ID:        fmt.Sprintf("mock-%d", time.Now().UnixNano()),
		OverlayID: req.OverlayID,
		Platform:  req.Platform,
		ChannelID: req.ChannelID,
		ChannelName: func() string {
			if req.ChannelName != "" {
				return req.ChannelName
			}
			return req.ChannelID
		}(),
		User: models.UserInfo{
			ID:          req.UserID,
			Username:    req.Username,
			DisplayName: req.DisplayName,
			AvatarURL:   req.AvatarURL,
			Badges:      req.Badges,
			Color:       req.Color,
		},
		Message: models.MessageInfo{
			Text: req.Text,
		},
		Timestamp: time.Now().UTC(),
		Metadata:  metadata,
	}
}
