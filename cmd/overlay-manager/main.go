package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caesar/all-chat/internal/overlay-manager/adapters/api"
	"github.com/caesar/all-chat/internal/overlay-manager/adapters/repository"
	"github.com/caesar/all-chat/internal/overlay-manager/core/services"
	"github.com/caesar/all-chat/pkg/database"
	"github.com/caesar/all-chat/pkg/logger"
	"github.com/caesar/all-chat/pkg/middleware"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	serviceName := getEnv("SERVICE_NAME", "overlay-manager")
	environment := getEnv("ENVIRONMENT", "development")

	if err := logger.Initialize(serviceName, environment); err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	defer logger.Sync()

	logger.Info("Starting overlay manager service", zap.String("environment", environment))

	// Initialize database
	ctx := context.Background()
	dbConfig := database.Config{
		Host:     getEnv("DATABASE_HOST", "localhost"),
		Port:     getEnvAsInt("DATABASE_PORT", 5432),
		User:     getEnv("DATABASE_USER", "allchat"),
		Password: getEnv("DATABASE_PASSWORD", "allchat_dev_password"),
		Database: getEnv("DATABASE_NAME", "allchat"),
		SSLMode:  getEnv("DATABASE_SSLMODE", "disable"),
	}

	dbPool, err := database.NewPool(ctx, dbConfig)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer dbPool.Close()

	logger.Info("Connected to database")

	// Initialize services
	overlayRepo := repository.NewPostgresOverlayRepository(dbPool)
	overlayService := services.NewOverlayService(overlayRepo)

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
		// Check database connection
		if err := dbPool.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "error": "database unavailable"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	// Register routes
	jwtSecret := getEnv("JWT_SECRET", "your-secret-key")
	overlayHandler := api.NewOverlayHandler(overlayService, jwtSecret)
	overlayHandler.RegisterRoutes(router)

	// Start server
	port := getEnv("PORT", "8082")
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
