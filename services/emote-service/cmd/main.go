package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/emote-service/cache"
	"github.com/caesar/all-chat/services/emote-service/clients"
	"github.com/caesar/all-chat/services/emote-service/handlers"
	"github.com/caesar/all-chat/shared/logger"
	sharedRedis "github.com/caesar/all-chat/shared/redis"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	// Get configuration from environment
	port := getEnv("PORT", "8083")
	logLevel := getEnv("LOG_LEVEL", "info")
	redisHost := getEnv("REDIS_HOST", "localhost")
	redisPort := getEnv("REDIS_PORT", "6379")
	cacheTTL := 1 * time.Hour // Emotes don't change frequently

	// Initialize logger
	log := logger.NewLogger("emote-service", logLevel)
	defer log.Sync()

	log.Info("Starting Emote Service")

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

	// Initialize emote cache
	emoteCache := cache.NewEmoteCache(redisClient, log, cacheTTL)

	// Initialize emote clients
	emoteClients := map[string]handlers.EmoteClient{
		"7tv":  clients.NewSevenTVClient(log),
		"bttv": clients.NewBTTVClient(log),
		"ffz":  clients.NewFFZClient(log),
	}

	// Initialize handlers
	emoteHandler := handlers.NewEmoteHandler(emoteClients, emoteCache, log)
	healthHandler := handlers.NewHealthHandler(redisClient, log)

	// Set up Gin router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(ginLogger(log))

	// Register routes
	emoteHandler.RegisterRoutes(router)
	healthHandler.RegisterRoutes(router)

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
