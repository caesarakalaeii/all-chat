package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caesar/all-chat/internal/chat-listener/adapters/clients"
	"github.com/caesar/all-chat/internal/chat-listener/adapters/redis"
	"github.com/caesar/all-chat/internal/chat-listener/adapters/repository"
	"github.com/caesar/all-chat/internal/chat-listener/adapters/twitch"
	"github.com/caesar/all-chat/internal/chat-listener/core/services"
	"github.com/caesar/all-chat/pkg/database"
	"github.com/caesar/all-chat/pkg/logger"
	redisClient "github.com/caesar/all-chat/pkg/redis"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	serviceName := getEnv("SERVICE_NAME", "chat-listener")
	environment := getEnv("ENVIRONMENT", "development")

	if err := logger.Initialize(serviceName, environment); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting Chat Listener Service", zap.String("environment", environment))

	// Validate required environment variables
	twitchUsername := getEnv("TWITCH_BOT_USERNAME", "")
	twitchOAuth := getEnv("TWITCH_BOT_OAUTH", "")
	if twitchUsername == "" || twitchOAuth == "" {
		logger.Fatal("TWITCH_BOT_USERNAME and TWITCH_BOT_OAUTH are required")
	}

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

	logger.Info("Database connection established")

	// Initialize Redis
	redisConfig := redisClient.Config{
		Host:     getEnv("REDIS_HOST", "localhost"),
		Port:     getEnvAsInt("REDIS_PORT", 6379),
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       getEnvAsInt("REDIS_DB", 0),
	}

	rdb, err := redisClient.NewClient(redisConfig)
	if err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	defer rdb.Close()

	logger.Info("Redis connection established")

	// Initialize adapters
	ircClient := twitch.NewIRCClient(twitchUsername, twitchOAuth)
	channelRepo := repository.NewPostgresChannelRepository(dbPool)
	emoteServiceURL := getEnv("EMOTE_SERVICE_URL", "http://localhost:8083")
	emoteClient := clients.NewHTTPEmoteClient(emoteServiceURL)
	publisher := redis.NewPublisher(rdb)

	// Initialize service
	chatService := services.NewChatService(
		ircClient,
		channelRepo,
		emoteClient,
		publisher,
		logger.Log,
	)

	// Start service
	if err := chatService.Start(ctx); err != nil {
		logger.Fatal("Failed to start chat service", zap.Error(err))
	}

	// Start channel refresh ticker
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			logger.Info("Refreshing channels")
			if err := chatService.RefreshChannels(context.Background()); err != nil {
				logger.Error("Failed to refresh channels", zap.Error(err))
			}
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down chat listener")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	if err := chatService.Stop(); err != nil {
		logger.Error("Error during shutdown", zap.Error(err))
	}

	// Wait for shutdown to complete or timeout
	<-shutdownCtx.Done()
	logger.Info("Chat listener stopped")
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
