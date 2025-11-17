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
	"github.com/caesar/all-chat/services/message-processor/enricher"
	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/caesar/all-chat/services/message-processor/normalizer"
	"github.com/caesar/all-chat/services/message-processor/publisher"
	"github.com/caesar/all-chat/services/message-processor/router"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/gin-gonic/gin"
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

	// Avatar enricher for Twitch users
	twitchClientID := getEnvOrDefault("TWITCH_CLIENT_ID", "")
	twitchClientSecret := getEnvOrDefault("TWITCH_CLIENT_SECRET", "")
	avatarEnricher := enricher.NewAvatarEnricher(redisClient, twitchClientID, twitchClientSecret, log)
	badgeEnricher := enricher.NewBadgeEnricher(redisClient, twitchClientID, twitchClientSecret, log)

	overlayRepo := router.NewRepository(db)
	overlayRouter := router.NewRouter(overlayRepo, log)

	pubsubPublisher := publisher.NewPubSubPublisher(redisClient, log)

	// Define message handler
	messageHandler := func(ctx context.Context, rawMsg *models.RawChatMessage) error {
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

		// Process message for each overlay
		for _, overlay := range overlays {
			// Normalize message using platform-specific normalizer
			unified, err := platformNormalizer.Normalize(rawMsg, overlay.OverlayID)
			if err != nil {
				log.Warn("Failed to normalize message",
					zap.String("message_id", rawMsg.MessageID),
					zap.String("platform", rawMsg.Platform),
					zap.Error(err),
				)
				continue
			}

			// Enrich with avatars (Twitch only, cached in Redis)
			if err := avatarEnricher.Enrich(ctx, unified); err != nil {
				log.Warn("Failed to enrich avatar",
					zap.String("message_id", rawMsg.MessageID),
					zap.Error(err),
				)
				// Continue even if enrichment fails
			}

			// Enrich with badge icons (Twitch only, cached in Redis)
			if err := badgeEnricher.Enrich(ctx, unified); err != nil {
				log.Warn("Failed to enrich badges",
					zap.String("message_id", rawMsg.MessageID),
					zap.Error(err),
				)
				// Continue even if enrichment fails
			}

			// Enrich with emotes
			if err := emoteEnricher.Enrich(ctx, unified); err != nil {
				log.Warn("Failed to enrich message",
					zap.String("message_id", rawMsg.MessageID),
					zap.Error(err),
				)
				// Continue even if enrichment fails
			}

			// Publish to overlay channel
			if err := pubsubPublisher.Publish(ctx, overlay.OverlayID, unified); err != nil {
				log.Error("Failed to publish to overlay",
					zap.String("overlay_id", overlay.OverlayID),
					zap.String("message_id", rawMsg.MessageID),
					zap.Error(err),
				)
				continue
			}
		}

		return nil
	}

	// Create and start stream consumer
	streamConsumer := consumer.NewStreamConsumer(redisClient, log, messageHandler)
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
