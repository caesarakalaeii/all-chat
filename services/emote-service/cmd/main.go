package main

import (
	"context"
	"crypto/subtle"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/emote-service/cache"
	"github.com/caesar/all-chat/services/emote-service/clients"
	"github.com/caesar/all-chat/services/emote-service/handlers"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/caesar/all-chat/shared/metrics"
	sharedRedis "github.com/caesar/all-chat/shared/redis"
	"github.com/caesar/all-chat/shared/tracing"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	// Get configuration from environment
	port := getEnv("PORT", "8083")
	logLevel := getEnv("LOG_LEVEL", "info")
	redisHost := getEnv("REDIS_HOST", "localhost")
	redisPort := getEnv("REDIS_PORT", "6379")
	rateLimitRequests := getEnvInt("RATE_LIMIT_REQUESTS", 60)
	rateLimitWindow := time.Duration(getEnvInt("RATE_LIMIT_WINDOW_SECONDS", 60)) * time.Second
	apiKey := os.Getenv("EMOTE_SERVICE_API_KEY")
	cacheTTL := 1 * time.Hour // Emotes don't change frequently

	// Initialize logger
	log := logger.NewLogger("emote-service", logLevel)
	defer log.Sync()

	log.Info("Starting Emote Service")

	// Initialize tracing
	tracingEnabled := getEnv("OTEL_ENABLED", "false") == "true"
	if tracingEnabled {
		tracingCfg := tracing.Config{
			ServiceName:    "emote-service",
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
			log.Info("Tracer initialized", zap.String("service", "emote-service"))
		}
	}

	// Initialize Redis client
	log.Info("Connecting to Redis")
	redisAddr := sharedRedis.BuildDSN(redisHost, redisPort)
	redisClient, err := sharedRedis.NewClient(redisAddr, "")
	if err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	defer redisClient.Close()

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal("Failed to ping Redis", zap.Error(err))
	}
	log.Info("Connected to Redis")

	// Initialize metrics (available via /metrics endpoint)
	_ = metrics.NewProcessorMetrics()
	log.Info("Initialized Prometheus metrics")

	// Initialize emote cache
	emoteCache := cache.NewEmoteCache(redisClient, log, cacheTTL)

	twitchClientID := getEnv("TWITCH_CLIENT_ID", "")
	twitchClientSecret := getEnv("TWITCH_CLIENT_SECRET", "")
	if twitchClientID == "" || twitchClientSecret == "" {
		log.Fatal("TWITCH_CLIENT_ID and TWITCH_CLIENT_SECRET must be set", zap.String("component", "emote-service"))
	}

	twitchClient := clients.NewTwitchClient(twitchClientID, twitchClientSecret, log)

	// Initialize emote clients
	twitchEmoteClient := clients.NewTwitchEmoteClient(twitchClient, log)
	emoteClients := map[string]handlers.EmoteClient{
		"twitch": twitchEmoteClient,
		"7tv":    clients.NewSevenTVClient(log, twitchClient),
		"bttv":   clients.NewBTTVClient(log),
		"ffz":    clients.NewFFZClient(log),
	}

	// Preload Twitch global emotes on startup
	log.Info("Preloading Twitch global emotes")
	preloadCtx, preloadCancel := context.WithTimeout(context.Background(), 10*time.Second)
	globalEmotes, err := twitchEmoteClient.FetchEmotes(preloadCtx, "global")
	preloadCancel()
	if err != nil {
		log.Warn("Failed to preload Twitch global emotes, will retry on first request", zap.Error(err))
	} else {
		// Cache global emotes
		if err := emoteCache.Set(context.Background(), "twitch", "global", globalEmotes); err != nil {
			log.Warn("Failed to cache Twitch global emotes", zap.Error(err))
		} else {
			log.Info("Preloaded and cached Twitch global emotes", zap.Int("count", len(globalEmotes)))
		}
	}

	// Initialize cheermote client
	cheermoteClient := clients.NewTwitchCheermoteClient(twitchClient, log)

	// Initialize handlers
	emoteHandler := handlers.NewEmoteHandler(emoteClients, emoteCache, log)
	cheermoteHandler := handlers.NewCheermoteHandler(cheermoteClient, redisClient, log)
	healthHandler := handlers.NewHealthHandler(redisClient, log)

	// Set up Gin router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(ginLogger(log))
	if tracingEnabled {
		router.Use(tracing.GinMiddleware("emote-service"))
	}
	router.Use(rateLimitMiddleware(newRedisRateLimiter(redisClient, rateLimitRequests, rateLimitWindow), log))
	if apiKey != "" {
		router.Use(apiKeyMiddleware(apiKey, log))
	}

	// Register routes
	emoteHandler.RegisterRoutes(router)
	healthHandler.RegisterRoutes(router)

	// Cheermote routes
	router.GET("/emotes/cheermotes/:channel_id", cheermoteHandler.GetCheermotes)

	// Prometheus metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Create HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Info("Starting HTTP server", zap.String("port", port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// Graceful shutdown with 25-second timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("Server forced to shutdown", zap.Error(err))
	}

	log.Info("Server stopped")
}

// ginLogger creates a Gin middleware for logging
const apiKeyHeader = "X-API-Key"

func ginLogger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)

		log.Info("HTTP request",
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", latency),
			zap.String("ip", c.ClientIP()),
		)
	}
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			return parsed
		}
	}
	return defaultValue
}

func apiKeyMiddleware(expectedKey string, log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		provided := c.GetHeader(apiKeyHeader)
		if provided == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing API key"})
			return
		}

		if subtle.ConstantTimeCompare([]byte(provided), []byte(expectedKey)) != 1 {
			log.Warn("Invalid API key provided", zap.String("ip", c.ClientIP()))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid API key"})
			return
		}

		c.Next()
	}
}

func rateLimitMiddleware(rl *rateLimiter, log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rl == nil || rl.limit <= 0 {
			c.Next()
			return
		}

		key := rateLimitKey(c)
		allowed, retryAfter, err := rl.allow(c.Request.Context(), key)
		if err != nil {
			log.Error("Rate limiter error, allowing request", zap.Error(err))
			c.Next()
			return
		}
		if !allowed {
			retrySeconds := int(math.Ceil(retryAfter.Seconds()))
			if retrySeconds < 1 {
				retrySeconds = 1
			}
			c.Header("Retry-After", strconv.Itoa(retrySeconds))
			log.Warn("Rate limit exceeded", zap.String("key", key), zap.Int("retry_after_seconds", retrySeconds))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded", "retry_after": retrySeconds})
			return
		}

		c.Next()
	}
}

func rateLimitKey(c *gin.Context) string {
	if apiKey := c.GetHeader(apiKeyHeader); apiKey != "" {
		return "api:" + apiKey
	}
	return "ip:" + c.ClientIP()
}

type rateLimiter struct {
	limit  int
	window time.Duration
	client sharedRedis.Client
}

func newRedisRateLimiter(client sharedRedis.Client, limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		limit:  limit,
		window: window,
		client: client,
	}
}

func (rl *rateLimiter) allow(ctx context.Context, key string) (bool, time.Duration, error) {
	if rl == nil || rl.limit <= 0 || rl.window <= 0 || rl.client == nil {
		return true, 0, nil
	}

	redisKey := "rate:" + key
	count, err := rl.client.Incr(ctx, redisKey).Result()
	if err != nil {
		return true, 0, err
	}

	if count == 1 {
		if err := rl.client.Expire(ctx, redisKey, rl.window).Err(); err != nil {
			return true, 0, err
		}
		return true, 0, nil
	}

	if count <= int64(rl.limit) {
		return true, 0, nil
	}

	ttl, err := rl.client.TTL(ctx, redisKey).Result()
	if err != nil || ttl < 0 {
		ttl = rl.window
	}

	return false, ttl, nil
}
