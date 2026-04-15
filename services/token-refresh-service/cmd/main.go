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

	authOAuth "github.com/caesar/all-chat/services/auth-service/oauth"
	"github.com/caesar/all-chat/services/token-refresh-service/refresher"
	"github.com/caesar/all-chat/services/token-refresh-service/repository"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/encryption"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/caesar/all-chat/shared/tracing"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logLevel := getEnv("LOG_LEVEL", "info")
	log := logger.NewLogger("token-refresh-service", logLevel)
	defer log.Sync()

	log.Info("Starting Token Refresh Service",
		zap.String("version", getEnv("APP_VERSION", "dev")),
	)

	// Initialize tracing
	tracingEnabled := getEnv("OTEL_ENABLED", "false") == "true"
	if tracingEnabled {
		tracingCfg := tracing.Config{
			ServiceName:    "token-refresh-service",
			ServiceVersion: getEnv("APP_VERSION", "dev"),
			Environment:    getEnv("ENVIRONMENT", "development"),
			OTLPEndpoint:   getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
			Enabled:        true,
		}
		shutdownTracer, err := tracing.InitTracer(tracingCfg, log)
		if err != nil {
			log.Error("Failed to initialize tracer", zap.Error(err))
		} else {
			defer shutdownTracer(context.Background())
			log.Info("Tracer initialized", zap.String("service", "token-refresh-service"))
		}
	}

	ctx := context.Background()

	// Load OAuth credentials for all platforms
	twitchClientID := os.Getenv("TWITCH_CLIENT_ID")
	twitchClientSecret := os.Getenv("TWITCH_CLIENT_SECRET")
	youtubeClientID := os.Getenv("YOUTUBE_CLIENT_ID")
	youtubeClientSecret := os.Getenv("YOUTUBE_CLIENT_SECRET")
	kickClientID := os.Getenv("KICK_CLIENT_ID")
	kickClientSecret := os.Getenv("KICK_CLIENT_SECRET")

	if twitchClientID == "" || twitchClientSecret == "" {
		log.Fatal("TWITCH_CLIENT_ID and TWITCH_CLIENT_SECRET are required")
	}
	if youtubeClientID == "" || youtubeClientSecret == "" {
		log.Fatal("YOUTUBE_CLIENT_ID and YOUTUBE_CLIENT_SECRET are required")
	}
	if kickClientID == "" || kickClientSecret == "" {
		log.Fatal("KICK_CLIENT_ID and KICK_CLIENT_SECRET are required")
	}

	// Load encryption key for token decryption
	encryptionKey := os.Getenv("ENCRYPTION_KEY")
	if encryptionKey == "" {
		log.Fatal("ENCRYPTION_KEY is required")
	}

	// Initialize AES encryptor for token encryption/decryption (must match auth-service)
	parsedKey, err := encryption.ParseKey(encryptionKey)
	if err != nil {
		log.Fatal("Failed to parse ENCRYPTION_KEY", zap.Error(err))
	}

	encryptor, err := encryption.NewAESEncryptor(parsedKey)
	if err != nil {
		log.Fatal("Failed to initialize encryptor", zap.Error(err))
	}

	// Connect to PostgreSQL
	dbHost := getEnv("DATABASE_HOST", "localhost")
	dbPort := getEnv("DATABASE_PORT", "5432")
	dbUser := getEnv("DATABASE_USER", "allchat")
	dbPassword := getEnv("DATABASE_PASSWORD", "allchat_dev_password")
	dbName := getEnv("DATABASE_NAME", "allchat")

	connString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	db, err := database.NewPostgresPool(connString)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	log.Info("Connected to PostgreSQL")

	// Connect to Redis
	redisHost := getEnv("REDIS_HOST", "localhost")
	redisPort := getEnv("REDIS_PORT", "6379")
	redisClient := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort),
	})
	defer redisClient.Close()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	log.Info("Connected to Redis")

	// Initialize OAuth providers
	// Note: We're importing from auth-service for consistency
	// In production, consider moving OAuth to shared package
	frontendURL := getEnv("FRONTEND_URL", "http://localhost:3000")
	twitchRedirectURL := fmt.Sprintf("%s/api/v1/auth/twitch/callback", frontendURL)
	youtubeRedirectURL := fmt.Sprintf("%s/api/v1/auth/youtube/callback", frontendURL)
	kickRedirectURL := fmt.Sprintf("%s/api/v1/auth/kick/callback", frontendURL)

	twitchOAuth := authOAuth.NewTwitchOAuth(twitchClientID, twitchClientSecret, twitchRedirectURL)
	youtubeOAuth := authOAuth.NewYouTubeOAuth(youtubeClientID, youtubeClientSecret, youtubeRedirectURL)
	kickOAuth := authOAuth.NewKickOAuth(kickClientID, kickClientSecret, kickRedirectURL)

	providers := map[authOAuth.Platform]authOAuth.OAuthProvider{
		authOAuth.PlatformTwitch:  twitchOAuth,
		authOAuth.PlatformYouTube: youtubeOAuth,
		authOAuth.PlatformKick:    kickOAuth,
	}

	// Initialize repository and refresh manager
	tokenRepo := repository.NewTokenRepository(db, encryptor, log)

	refreshInterval := getDuration("TOKEN_REFRESH_INTERVAL", 5*time.Minute)
	expiryBuffer := getDuration("TOKEN_REFRESH_BUFFER", 10*time.Minute)
	batchSize := getInt("TOKEN_REFRESH_BATCH_SIZE", 100)
	retryAttempts := getInt("TOKEN_REFRESH_RETRY_ATTEMPTS", 3)

	refreshManager := refresher.NewManager(
		tokenRepo,
		providers,
		redisClient,
		log,
		refreshInterval,
		expiryBuffer,
		batchSize,
		retryAttempts,
	)

	// Start refresh manager in background
	refreshCtx, refreshCancel := context.WithCancel(ctx)
	defer refreshCancel()

	go func() {
		if err := refreshManager.Start(refreshCtx); err != nil {
			log.Error("Refresh manager stopped with error", zap.Error(err))
		}
	}()

	log.Info("Token refresh manager started",
		zap.Duration("interval", refreshInterval),
		zap.Duration("expiry_buffer", expiryBuffer),
		zap.Int("batch_size", batchSize),
	)

	// HTTP server for health checks
	port := getEnv("PORT", "8090")

	// HTTP metrics for token-refresh-service
	httpRequestsTotal := promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests",
	}, []string{"service", "method", "path", "status"})

	httpRequestDuration := promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"service", "method", "path"})

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(httpMetricsMiddleware(httpRequestsTotal, httpRequestDuration, "token-refresh-service"))
	if tracingEnabled {
		router.Use(tracing.GinMiddleware("token-refresh-service"))
	}

	// Health endpoints
	router.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.GET("/health/ready", func(c *gin.Context) {
		// Check database connection
		if err := db.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "error": "database unavailable"})
			return
		}

		// Check Redis connection
		if err := redisClient.Ping(ctx).Err(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "error": "redis unavailable"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Status endpoint (basic info about refresh activity)
	router.GET("/status", func(c *gin.Context) {
		stats := refreshManager.GetStats()
		c.JSON(http.StatusOK, stats)
	})

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Start HTTP server in background
	go func() {
		log.Info("HTTP server starting", zap.String("port", port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("HTTP server failed", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down gracefully...")

	// Stop refresh manager
	refreshCancel()

	// Shutdown HTTP server with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP server shutdown error", zap.Error(err))
	}

	log.Info("Service stopped")
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}

func getInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var i int
		if _, err := fmt.Sscan(value, &i); err == nil && i > 0 {
			return i
		}
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
