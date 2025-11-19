package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/clip-manager/fetchers"
	"github.com/caesar/all-chat/services/clip-manager/ranker"
	"github.com/caesar/all-chat/services/clip-manager/repository"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logLevel := getEnv("LOG_LEVEL", "info")
	log := logger.NewLogger("clip-manager", logLevel)
	defer log.Sync()

	log.Info("Starting Clip Manager service...")

	ctx := context.Background()

	// Get environment variables
	port := getEnv("PORT", "8091")
	dbHost := getEnv("DATABASE_HOST", "localhost")
	dbPort := getEnv("DATABASE_PORT", "5432")
	dbUser := getEnv("DATABASE_USER", "allchat")
	dbPassword := getEnv("DATABASE_PASSWORD", "allchat_dev_password")
	dbName := getEnv("DATABASE_NAME", "allchat")

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

	// Get Twitch credentials
	twitchClientID := getEnv("TWITCH_CLIENT_ID", "")
	twitchAccessToken := getEnv("TWITCH_ACCESS_TOKEN", "")

	// Initialize repositories
	clipRepo := repository.NewClipRepository(db)

	// Initialize fetchers
	twitchFetcher := fetchers.NewTwitchFetcher(twitchClientID, twitchAccessToken, log)

	// Initialize ranker
	clipRanker := ranker.NewClipRanker(log)

	_ = clipRepo
	_ = twitchFetcher
	_ = clipRanker

	// Initialize Gin router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/health/live", "/health/ready"},
	}))

	// Health check endpoints
	router.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "clip-manager"})
	})

	router.GET("/health/ready", func(c *gin.Context) {
		if err := db.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "error": "database unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "clip-manager"})
	})

	// TODO: API endpoints
	// v1 := router.Group("/api/v1")
	// {
	// 	v1.POST("/clips/fetch/:session_id", handler.FetchClips)
	// 	v1.GET("/clips", handler.GetClips)
	// 	v1.POST("/clips/select", handler.SelectClips)
	// }

	// Start HTTP server
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	go func() {
		log.Info("Clip Manager service started", zap.String("port", port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down Clip Manager service...")

	// Graceful shutdown with 25-second timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("Server forced to shutdown", zap.Error(err))
	}

	log.Info("Clip Manager service stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
