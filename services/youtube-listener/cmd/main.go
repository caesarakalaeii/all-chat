package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener/handlers"
	"github.com/caesar/all-chat/services/youtube-listener/oauth"
	"github.com/caesar/all-chat/services/youtube-listener/publisher"
	"github.com/caesar/all-chat/services/youtube-listener/quota"
	"github.com/caesar/all-chat/services/youtube-listener/streams"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/encryption"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/caesar/all-chat/shared/sourcemanager"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logLevel := getEnvOrDefault("LOG_LEVEL", "info")
	log := logger.NewLogger("youtube-listener", logLevel)
	defer log.Sync()

	log.Info("Starting YouTube Listener",
		zap.String("version", getEnvOrDefault("APP_VERSION", "dev")),
	)

	ctx := context.Background()

	// Validate required environment variables
	youtubeClientID := os.Getenv("YOUTUBE_CLIENT_ID")
	youtubeClientSecret := os.Getenv("YOUTUBE_CLIENT_SECRET")
	frontendURL := strings.TrimSuffix(getEnvOrDefault("FRONTEND_URL", "http://localhost:3000"), "/")
	youtubeRedirectURL := defaultCallbackURL(frontendURL, "http://localhost:8080", "/api/v1/auth/youtube/callback")

	if youtubeClientID == "" || youtubeClientSecret == "" {
		log.Fatal("YOUTUBE_CLIENT_ID and YOUTUBE_CLIENT_SECRET are required")
	}

	// Load encryption key for OAuth tokens
	youtubeEncryptionKey := os.Getenv("YOUTUBE_TOKEN_ENCRYPTION_KEY")
	parsedKey, err := encryption.ParseKey(youtubeEncryptionKey)
	if err != nil {
		log.Fatal("Invalid YOUTUBE_TOKEN_ENCRYPTION_KEY", zap.Error(err))
	}

	tokenEncryptor, err := encryption.NewAESEncryptor(parsedKey)
	if err != nil {
		log.Fatal("Failed to initialize token encryptor", zap.Error(err))
	}

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
	listenerMetrics := metrics.NewListenerMetrics("youtube", "youtube-listener")
	log.Info("Initialized Prometheus metrics")

	// Initialize components
	tokenStore := oauth.NewPostgresTokenStore(db, tokenEncryptor, log)
	oauthManager := oauth.NewManager(youtubeClientID, youtubeClientSecret, youtubeRedirectURL, tokenStore, log)

	streamPublisher := publisher.NewStreamPublisher(redisClient, log)

	// Initialize quota tracker (legacy - kept for backward compatibility)
	quotaLimitStr := getEnvOrDefault("QUOTA_LIMIT_DAILY", "10000")
	quotaLimit, err := strconv.Atoi(quotaLimitStr)
	if err != nil {
		log.Warn("Invalid QUOTA_LIMIT_DAILY, using default 10000", zap.Error(err))
		quotaLimit = 10000
	}

	quotaTracker := quota.NewTracker(db, quotaLimit, log, listenerMetrics)
	if err := quotaTracker.Start(ctx); err != nil {
		log.Fatal("Failed to start quota tracker", zap.Error(err))
	}

	// Initialize per-channel quota tracker (new)
	perChannelQuotaConfig := quota.Config{
		GlobalDailyQuota:  parseIntEnv("YOUTUBE_GLOBAL_DAILY_QUOTA", 10000),
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
	messageHandler := NewMessageHandler(streamPublisher, quotaTracker, log)

	sourceManagerURL := getEnvOrDefault("SOURCE_MANAGER_URL", "http://source-manager:8088")
	sourceManagerSecret := getEnvOrDefault("SOURCE_MANAGER_SECRET", "dev-service-secret")
	var leaderCoord *sourcemanager.LeadershipCoordinator
	if sourceManagerSecret == "" {
		log.Warn("SOURCE_MANAGER_SECRET not set; YouTube Listener will not coordinate leadership")
	} else {
		tokenSource := sourcemanager.NewSigningTokenSource("youtube-listener", sourceManagerSecret, 15*time.Minute)
		smClient, err := sourcemanager.NewClient(sourceManagerURL, tokenSource)
		if err != nil {
			log.Fatal("Failed to initialize Source Manager client", zap.Error(err))
		}
		leaderCoord = sourcemanager.NewLeadershipCoordinator("youtube", smClient, 5*time.Second, log)
	}

	// Initialize stream manager
	streamRepo := streams.NewRepository(db, log)
	dbConnWrapper := &dbConnWrapper{pool: db}
	streamManager := streams.NewManager(streamRepo, oauthManager, messageHandler, dbConnWrapper, leaderCoord, quotaTracker, perChannelQuotaTracker, redisClient, log)

	// Start stream manager
	if err := streamManager.Start(ctx); err != nil {
		log.Fatal("Failed to start stream manager", zap.Error(err))
	}

	// Set up HTTP server for health checks
	if logLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	// Health check handlers
	healthHandler := handlers.NewHealthHandler(streamManager, streamPublisher, quotaTracker)
	router.GET("/health/live", healthHandler.LivenessProbe)
	router.GET("/health/ready", healthHandler.ReadinessProbe)
	router.GET("/status", healthHandler.Status)

	// Quota handlers
	quotaCoordinator := quota.NewCoordinator(quotaTracker, perChannelQuotaTracker, log)
	quotaHandler := handlers.NewQuotaHandler(quotaCoordinator, quotaTracker, perChannelQuotaTracker, log)
	router.GET("/quota/status", quotaHandler.GetQuotaStatus)
	router.GET("/quota/channels/:channel_id", quotaHandler.GetChannelQuota)
	router.GET("/quota/history", quotaHandler.GetQuotaHistory)
	router.GET("/quota/predictions", quotaHandler.GetQuotaPrediction)
	router.POST("/quota/record", quotaHandler.RecordQuota)  // For external services (overlay-manager)

	// Prometheus metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Get port
	port := getEnvOrDefault("PORT", "8086")

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

// getEnvOrDefault gets an environment variable or returns a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
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
