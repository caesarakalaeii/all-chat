package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/event-collector/collectors"
	"github.com/caesar/all-chat/services/event-collector/handlers"
	"github.com/caesar/all-chat/services/event-collector/normalizers"
	"github.com/caesar/all-chat/services/event-collector/repository"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logLevel := getEnv("LOG_LEVEL", "info")
	log := logger.NewLogger("event-collector", logLevel)
	defer log.Sync()

	log.Info("Starting Event Collector service...")

	ctx := context.Background()

	// Get environment variables
	port := getEnv("PORT", "8090")
	dbHost := getEnv("DATABASE_HOST", "localhost")
	dbPort := getEnv("DATABASE_PORT", "5432")
	dbUser := getEnv("DATABASE_USER", "allchat")
	dbPassword := getEnv("DATABASE_PASSWORD", "allchat_dev_password")
	dbName := getEnv("DATABASE_NAME", "allchat")
	redisHost := getEnv("REDIS_HOST", "localhost")
	redisPort := getEnv("REDIS_PORT", "6379")

	// Build connection string
	connString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	// Initialize database connection
	db, err := database.NewPostgresPool(connString)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	log.Info("Connected to database")

	// Initialize Redis connection
	redisClient := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort),
	})
	defer redisClient.Close()

	// Test Redis connection
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	log.Info("Connected to Redis")

	// Initialize repositories
	eventRepo := repository.NewEventRepository(db)
	sessionRepo := repository.NewSessionRepository(db)

	// Initialize normalizers
	twitchNormalizer := normalizers.NewTwitchNormalizer()

	// Get Twitch credentials from environment
	twitchClientID := getEnv("TWITCH_CLIENT_ID", "")
	twitchClientSecret := getEnv("TWITCH_CLIENT_SECRET", "")

	// Initialize collector manager
	collectorManager := collectors.NewCollectorManager(
		eventRepo,
		sessionRepo,
		twitchNormalizer,
		twitchClientID,
		twitchClientSecret,
		log,
	)

	// TODO: Initialize YouTube event extractor
	// TODO: Initialize Kick event listener
	// TODO: Initialize TikTok webhook handler

	// Initialize Gin router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/health/live", "/health/ready"},
	}))

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler(db, redisClient)
	eventsHandler := handlers.NewEventsHandler(eventRepo, sessionRepo, log)
	collectorHandler := handlers.NewCollectorHandler(collectorManager, log)

	// Health check endpoints
	router.GET("/health/live", healthHandler.LivenessProbe)
	router.GET("/health/ready", healthHandler.ReadinessProbe)

	// TODO: Add webhook endpoints for EventSub (alternative to WebSocket)
	// webhooks := router.Group("/webhooks")
	// {
	// 	webhooks.POST("/twitch", twitchHandler.HandleWebhook)
	// 	webhooks.POST("/tiktok", tiktokHandler.HandleWebhook)
	// }

	// API v1 endpoints
	v1 := router.Group("/api/v1")
	{
		// Session endpoints
		v1.GET("/sessions/:id/events", eventsHandler.GetEventsBySession)
		v1.GET("/sessions/:id/stats", eventsHandler.GetSessionStats)
		v1.GET("/users/:id/sessions", eventsHandler.GetUserSessions)
		v1.GET("/users/:id/sessions/active", eventsHandler.GetActiveSession)

		// Collector management endpoints
		collectors := v1.Group("/collectors")
		{
			collectors.POST("/twitch/start", collectorHandler.StartTwitchCollector)
			collectors.POST("/twitch/stop", collectorHandler.StopTwitchCollector)
			collectors.GET("/twitch/:user_id", collectorHandler.GetCollectorStatus)
			collectors.GET("/active", collectorHandler.ListActiveCollectors)
		}
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

	// Shutdown all collectors first
	collectorManager.Shutdown()

	// Graceful shutdown with 25-second timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
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
