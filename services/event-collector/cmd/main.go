package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/event-collector/handlers"
	"github.com/caesar/all-chat/services/event-collector/normalizers"
	"github.com/caesar/all-chat/services/event-collector/repository"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/logger"
	sharedRedis "github.com/caesar/all-chat/shared/redis"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	log := logger.NewLogger()
	defer log.Sync()

	log.Info("Starting Event Collector service...")

	// Get environment variables
	port := getEnv("PORT", "8090")
	dbURL := getEnv("DATABASE_URL", "postgresql://allchat:allchat_dev_password@localhost:5432/allchat")
	redisHost := getEnv("REDIS_HOST", "localhost")
	redisPort := getEnv("REDIS_PORT", "6379")

	// Initialize database connection
	db, err := database.NewPool(context.Background(), dbURL)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	log.Info("Connected to database")

	// Initialize Redis connection
	redisClient := sharedRedis.NewClient(redisHost, redisPort)
	defer redisClient.Close()

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	log.Info("Connected to Redis")

	// Initialize repositories
	eventRepo := repository.NewEventRepository(db)
	sessionRepo := repository.NewSessionRepository(db)

	// Initialize normalizers
	twitchNormalizer := normalizers.NewTwitchNormalizer()

	// TODO: Initialize Twitch EventSub collector
	// TODO: Initialize YouTube event extractor
	// TODO: Initialize Kick event listener
	// TODO: Initialize TikTok webhook handler

	_ = eventRepo
	_ = sessionRepo
	_ = twitchNormalizer

	// Initialize Gin router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/health/live", "/health/ready"},
	}))

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler(db, redisClient)

	// Health check endpoints
	router.GET("/health/live", healthHandler.LivenessProbe)
	router.GET("/health/ready", healthHandler.ReadinessProbe)

	// TODO: Add event collector endpoints
	// router.POST("/webhooks/twitch", twitchHandler.HandleWebhook)
	// router.POST("/webhooks/tiktok", tiktokHandler.HandleWebhook)

	// API v1 endpoints
	v1 := router.Group("/api/v1")
	{
		// TODO: Add session endpoints
		// v1.GET("/sessions/:id/events", eventHandler.GetEvents)
		// v1.GET("/sessions/:id/events/stats", eventHandler.GetStats)
		_ = v1
	}

	// Start HTTP server
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		log.Info("Event Collector service started", zap.String("port", port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down Event Collector service...")

	// Graceful shutdown with 25-second timeout
	ctx, cancel = context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", zap.Error(err))
	}

	log.Info("Event Collector service stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
