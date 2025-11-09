package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caesar/all-chat/internal/emote-service/adapters/api"
	"github.com/caesar/all-chat/internal/emote-service/adapters/clients"
	"github.com/caesar/all-chat/internal/emote-service/core/services"
	"github.com/caesar/all-chat/pkg/logger"
	"github.com/caesar/all-chat/pkg/middleware"
	pkgredis "github.com/caesar/all-chat/pkg/redis"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	serviceName := getEnv("SERVICE_NAME", "emote-service")
	environment := getEnv("ENVIRONMENT", "development")

	if err := logger.Initialize(serviceName, environment); err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	defer logger.Sync()

	logger.Info("Starting emote service", zap.String("environment", environment))

	// Initialize Redis
	ctx := context.Background()
	redisConfig := pkgredis.Config{
		Host:     getEnv("REDIS_HOST", "localhost"),
		Port:     getEnvAsInt("REDIS_PORT", 6379),
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       getEnvAsInt("REDIS_DB", 0),
	}

	redisClient, err := pkgredis.NewClient(redisConfig)
	if err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	defer redisClient.Close()

	logger.Info("Connected to Redis")

	// Initialize services
	emoteClient := clients.NewEmoteClient()
	emoteService := services.NewEmoteService(emoteClient, redisClient)

	// Pre-warm cache
	logger.Info("Pre-warming emote cache...")
	if err := emoteService.RefreshCache(ctx); err != nil {
		logger.Warn("Failed to pre-warm cache", zap.Error(err))
	}

	// Set Gin mode
	if environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize Gin router
	router := gin.Default()

	// CORS middleware
	router.Use(middleware.CORSMiddleware([]string{"*"}))

	// Health check endpoints
	router.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "alive"})
	})

	router.GET("/health/ready", func(c *gin.Context) {
		// Check Redis connection
		if err := redisClient.Ping(ctx).Err(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "error": "redis unavailable"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	// Register routes
	emoteHandler := api.NewEmoteHandler(emoteService)
	emoteHandler.RegisterRoutes(router)

	// Start server
	port := getEnv("PORT", "8083")
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Graceful shutdown
	go func() {
		logger.Info("Starting HTTP server", zap.String("port", port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}
