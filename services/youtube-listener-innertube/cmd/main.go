package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener-innertube/handlers"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/innertube"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/publisher"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/streams"
	"github.com/caesar/all-chat/shared/sourcemanager"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	// 1. Environment variables
	logLevel := getEnv("LOG_LEVEL", "info")
	redisHost := getEnv("REDIS_HOST", "localhost")
	redisPort := getEnv("REDIS_PORT", "6379")
	httpPort := getEnv("HTTP_PORT", "8080")

	// 2. Logger initialization
	logger := newLogger("youtube-listener-innertube", logLevel)
	defer logger.Sync()

	logger.Info("Starting YouTube Listener InnerTube",
		zap.String("version", getEnv("APP_VERSION", "dev")),
		zap.String("log_level", logLevel),
	)

	ctx := context.Background()

	// 3. Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort),
	})
	defer redisClient.Close()

	// Test Redis connection
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	logger.Info("Connected to Redis",
		zap.String("addr", fmt.Sprintf("%s:%s", redisHost, redisPort)))

	// 4. Initialize components
	// InnerTube client (hardcoded API key for MVP)
	innertubeClient := innertube.NewClient(innertube.ClientOptions{
		APIKey:  innertube.DefaultAPIKey,
		Timeout: 10 * time.Second,
		Logger:  logger,
	})

	// Discovery
	httpClient := &http.Client{Timeout: 10 * time.Second}
	discovery := innertube.NewDiscovery(httpClient, logger)

	// Repository for Redis persistence
	repository := streams.NewRepository(redisClient, logger)

	// Publisher
	streamPublisher := publisher.NewStreamPublisher(redisClient, logger)

	// Leadership coordinator for stream ownership
	sourceManagerURL := getEnv("SOURCE_MANAGER_URL", "http://source-manager:8088")
	sourceManagerSecret := getEnv("SOURCE_MANAGER_SECRET", "dev-service-secret")
	var leaderCoord *sourcemanager.LeadershipCoordinator
	if sourceManagerSecret == "" {
		logger.Warn("SOURCE_MANAGER_SECRET not set; InnerTube listener will not coordinate leadership")
	} else {
		tokenSource := sourcemanager.NewSigningTokenSource("innertube", sourceManagerSecret, 15*time.Minute)
		smClient, err := sourcemanager.NewClient(sourceManagerURL, tokenSource)
		if err != nil {
			logger.Fatal("Failed to initialize Source Manager client", zap.Error(err))
		}
		leaderCoord = sourcemanager.NewLeadershipCoordinator("innertube", smClient, 5*time.Second, logger)
	}

	// 5. Initialize and start stream manager
	streamManager := streams.NewManager(
		leaderCoord,
		repository,
		discovery,
		streamPublisher,
		innertubeClient,
		redisClient,
		logger,
	)

	if err := streamManager.Start(ctx); err != nil {
		logger.Fatal("Failed to start stream manager", zap.Error(err))
	}

	// 6. HTTP server with health checks
	if logLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	healthHandler := handlers.NewHealthHandler(streamPublisher, innertubeClient, logger)
	router.GET("/health/live", healthHandler.LivenessProbe)
	router.GET("/health/ready", healthHandler.ReadinessProbe)
	router.GET("/status", healthHandler.Status)

	srv := &http.Server{
		Addr:         ":" + httpPort,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 7. Start HTTP server in goroutine
	go func() {
		logger.Info("HTTP server listening",
			zap.String("port", httpPort))

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server failed", zap.Error(err))
		}
	}()

	// 8. Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	logger.Info("Shutting down service...")

	// Create shutdown context with 25s timeout (Kubernetes sends SIGKILL at 30s)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer shutdownCancel()

	// Stop stream manager first
	if err := streamManager.Shutdown(shutdownCtx); err != nil {
		logger.Error("Stream manager shutdown error", zap.Error(err))
	}

	// Shutdown HTTP server
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server forced shutdown", zap.Error(err))
	}

	logger.Info("Service stopped gracefully")
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// newLogger creates a new Zap logger with the given service name and log level
func newLogger(serviceName, level string) *zap.Logger {
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(parseLevel(level))
	config.EncoderConfig.TimeKey = "ts"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	logger, err := config.Build(
		zap.Fields(
			zap.String("service", serviceName),
			zap.String("version", getEnv("APP_VERSION", "dev")),
		),
	)
	if err != nil {
		panic(err)
	}

	return logger
}

func parseLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}
