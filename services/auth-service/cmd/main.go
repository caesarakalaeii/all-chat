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

	"github.com/caesar/all-chat/services/auth-service/handlers"
	"github.com/caesar/all-chat/services/auth-service/oauth"
	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/caesar/all-chat/shared/middleware"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logLevel := getEnvOrDefault("LOG_LEVEL", "info")
	log := logger.NewLogger("auth-service", logLevel)
	defer log.Sync()

	log.Info("Starting Auth Service",
		zap.String("version", getEnvOrDefault("APP_VERSION", "dev")),
	)

	ctx := context.Background()

	// Get environment variables
	twitchClientID := os.Getenv("TWITCH_CLIENT_ID")
	twitchClientSecret := os.Getenv("TWITCH_CLIENT_SECRET")
	twitchRedirectURL := getEnvOrDefault("TWITCH_REDIRECT_URL", "http://localhost:8080/api/v1/auth/callback")
	jwtSecret := os.Getenv("JWT_SECRET")
	jwtExpiryHours := getEnvAsIntOrDefault("JWT_EXPIRY_HOURS", 24)

	if twitchClientID == "" || twitchClientSecret == "" {
		log.Fatal("TWITCH_CLIENT_ID and TWITCH_CLIENT_SECRET must be set")
	}

	if jwtSecret == "" {
		log.Warn("JWT_SECRET not set, using default (NOT FOR PRODUCTION)")
		jwtSecret = "default-secret-change-in-production"
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

	// Initialize components
	oauthClient := oauth.NewTwitchOAuth(twitchClientID, twitchClientSecret, twitchRedirectURL)
	userRepo := repository.NewUserRepository(db)

	// Create handlers
	authHandler := handlers.NewAuthHandler(oauthClient, userRepo, redisClient, jwtSecret, jwtExpiryHours, log)
	healthHandler := handlers.NewHealthHandler(db, redisClient)

	// Set Gin mode
	if logLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create router
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.Logging(log))
	// CORS is handled by API Gateway, not by individual services

	// Health check endpoints
	router.GET("/health/live", healthHandler.CheckLive)
	router.GET("/health/ready", healthHandler.CheckReady)

	// Auth routes (no /auth prefix - API Gateway strips /api/v1/auth and forwards rest)
	router.GET("/login", authHandler.HandleLogin)
	router.POST("/login", authHandler.HandleLogin)
	router.GET("/callback", authHandler.HandleCallback)
	router.POST("/refresh", authHandler.HandleRefresh)

	// Protected routes (require JWT)
	protected := router.Group("/")
	protected.Use(middleware.JWTAuth(jwtSecret))
	{
		protected.GET("/me", authHandler.HandleGetMe)
		protected.POST("/logout", authHandler.HandleLogout)
	}

	// Get port from environment
	port := getEnvOrDefault("PORT", "8081")

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Info("Auth Service listening",
			zap.String("port", port),
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

	// Give outstanding requests 25 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", zap.Error(err))
	}

	log.Info("Server exited")
}

// getEnvOrDefault gets an environment variable or returns a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsIntOrDefault gets an environment variable as int or returns default
func getEnvAsIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
