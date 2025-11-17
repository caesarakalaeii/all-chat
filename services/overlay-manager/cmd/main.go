package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/overlay-manager/clients"
	"github.com/caesar/all-chat/services/overlay-manager/handlers"
	"github.com/caesar/all-chat/services/overlay-manager/repository"
	"github.com/caesar/all-chat/services/overlay-manager/youtube"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/caesar/all-chat/shared/middleware"
	sharedRedis "github.com/caesar/all-chat/shared/redis"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logLevel := getEnv("LOG_LEVEL", "info")
	log := logger.NewLogger("overlay-manager", logLevel)
	defer log.Sync()

	log.Info("Starting Overlay Manager Service",
		zap.String("version", getEnv("APP_VERSION", "0.1.0")),
	)

	// Load configuration from environment
	config := loadConfig()

	// Connect to PostgreSQL
	log.Info("Connecting to PostgreSQL",
		zap.String("host", config.DatabaseHost),
		zap.String("database", config.DatabaseName),
	)

	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		config.DatabaseUser,
		config.DatabasePassword,
		config.DatabaseHost,
		config.DatabasePort,
		config.DatabaseName,
	)

	dbPool, err := database.NewPostgresPool(connString)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer dbPool.Close()

	log.Info("Connected to PostgreSQL successfully")

	// Connect to Redis
	log.Info("Connecting to Redis",
		zap.String("host", config.RedisHost),
		zap.String("port", config.RedisPort),
	)

	redisAddr := fmt.Sprintf("%s:%s", config.RedisHost, config.RedisPort)
	redisClient := sharedRedis.NewRedisClient(redisAddr)
	defer redisClient.Close()

	// Test Redis connection
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	log.Info("Connected to Redis successfully")

	// Initialize repositories with the connection string
	overlayRepo, err := repository.NewOverlayRepository(connString)
	if err != nil {
		log.Fatal("Failed to create overlay repository", zap.Error(err))
	}

	configRepo, err := repository.NewOverlayConfigRepository(connString)
	if err != nil {
		log.Fatal("Failed to create overlay config repository", zap.Error(err))
	}

	sourceRepo, err := repository.NewSourceRepository(connString)
	if err != nil {
		log.Fatal("Failed to create source repository", zap.Error(err))
	}

	// Initialize handlers
	mpClient := clients.NewMessageProcessorClient(config.MessageProcessorURL, config.MessageProcessorAPIKey, log)
	overlayHandler := handlers.NewOverlayHandler(overlayRepo)
	configHandler := handlers.NewConfigHandler(configRepo, overlayRepo)
	sourcesHandler := handlers.NewSourcesHandler(sourceRepo, overlayRepo)
	mockHandler := handlers.NewMockMessageHandler(overlayRepo, sourceRepo, mpClient)
	healthHandler := handlers.NewHealthHandler(dbPool, redisClient)

	// YouTube helper
	youtubeAPIKey := getEnv("YOUTUBE_API_KEY", "")
	youtubeResolver := youtube.NewResolver(youtubeAPIKey)
	youtubeHandler := handlers.NewYouTubeHandler(youtubeResolver, log)

	// Setup Gin router
	if config.GinMode == "release" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	// CORS is handled by API Gateway, not by individual services
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/health/live", "/health/ready"},
	}))

	// Register health routes (no auth required)
	healthHandler.RegisterRoutes(router)

	// Public config for OBS/browser sources
	router.GET("/public/:id/config", configHandler.HandleGetPublicConfig)

	// Protected routes (require JWT)
	protected := router.Group("/")
	protected.Use(middleware.JWTAuth(config.JWTSecret))
	{
		// Overlay CRUD routes (no :id prefix)
		protected.POST("/", overlayHandler.HandleCreateOverlay)
		protected.GET("/", overlayHandler.HandleListOverlays)
		protected.GET("/:id", overlayHandler.HandleGetOverlay)
		protected.PUT("/:id", overlayHandler.HandleUpdateOverlay)
		protected.DELETE("/:id", overlayHandler.HandleDeleteOverlay)

		// Source management routes (nested under /:id)
		protected.GET("/:id/sources", sourcesHandler.HandleListSources)
		protected.POST("/:id/sources", sourcesHandler.HandleAddSource)
		protected.DELETE("/:id/sources/:source_id", sourcesHandler.HandleDeleteSource)

		protected.GET("/:id/config", configHandler.HandleGetConfig)
		protected.PUT("/:id/config", configHandler.HandleUpdateConfig)
		protected.POST("/:id/mock-messages", mockHandler.HandleSendMockMessage)

		// YouTube helper routes
		protected.POST("/youtube/resolve", youtubeHandler.ResolveChannel)

		// Internal API routes (called by other services like auth-service)
		internal := protected.Group("/internal/overlays")
		{
			internal.POST("/:id/sources/auto", sourcesHandler.HandleAddSourceAuto)
		}
	}

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + config.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Info("Overlay Manager Service started",
			zap.String("port", config.Port),
			zap.String("mode", config.GinMode),
		)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// Graceful shutdown with 25-second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", zap.Error(err))
	}

	log.Info("Server exited")
}

// Config holds application configuration
type Config struct {
	Port                   string
	GinMode                string
	DatabaseHost           string
	DatabasePort           string
	DatabaseUser           string
	DatabasePassword       string
	DatabaseName           string
	RedisHost              string
	RedisPort              string
	JWTSecret              string
	MessageProcessorURL    string
	MessageProcessorAPIKey string
}

// loadConfig loads configuration from environment variables
func loadConfig() *Config {
	return &Config{
		Port:                   getEnv("PORT", "8082"),
		GinMode:                getEnv("GIN_MODE", "debug"),
		DatabaseHost:           getEnv("DATABASE_HOST", "localhost"),
		DatabasePort:           getEnv("DATABASE_PORT", "5432"),
		DatabaseUser:           getEnv("DATABASE_USER", "allchat"),
		DatabasePassword:       getEnv("DATABASE_PASSWORD", "allchat_dev_password"),
		DatabaseName:           getEnv("DATABASE_NAME", "allchat"),
		RedisHost:              getEnv("REDIS_HOST", "localhost"),
		RedisPort:              getEnv("REDIS_PORT", "6379"),
		JWTSecret:              getEnv("JWT_SECRET", "default-secret-change-in-production"),
		MessageProcessorURL:    getEnv("MESSAGE_PROCESSOR_URL", "http://message-processor:8087"),
		MessageProcessorAPIKey: getEnv("MESSAGE_PROCESSOR_API_KEY", ""),
	}
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
