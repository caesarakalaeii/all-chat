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
	"github.com/caesar/all-chat/services/youtube-listener-innertube/poller"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/publisher"
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
	initialContinuation := getEnv("INITIAL_CONTINUATION", "")
	channelID := getEnv("CHANNEL_ID", "")

	// Validate required environment variables
	if initialContinuation == "" {
		fmt.Fprintf(os.Stderr, "ERROR: INITIAL_CONTINUATION is required\n")
		os.Exit(1)
	}
	if channelID == "" {
		fmt.Fprintf(os.Stderr, "ERROR: CHANNEL_ID is required\n")
		os.Exit(1)
	}

	// 2. Logger initialization
	logger := newLogger("youtube-listener-innertube", logLevel)
	defer logger.Sync()

	logger.Info("Starting YouTube Listener InnerTube PoC",
		zap.String("version", getEnv("APP_VERSION", "poc")),
		zap.String("log_level", logLevel),
		zap.String("channel_id", channelID),
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

	// 4. InnerTube client (hardcoded API key for PoC)
	innertubeClient := innertube.NewClient(innertube.ClientOptions{
		APIKey:  innertube.DefaultAPIKey,
		Timeout: 10 * time.Second,
		Logger:  logger,
	})

	// 5. Publisher
	streamPublisher := publisher.NewStreamPublisher(redisClient, logger)

	// 6. Poller with message callback
	pollerInstance := poller.NewPoller(
		innertubeClient,
		initialContinuation,
		channelID,
		logger,
		&poller.PollerOptions{
			Interval: 2 * time.Second,
			LogLevel: logLevel,
		},
	)

	// 7. HTTP server with health checks
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

	// 8. Start HTTP server in goroutine
	go func() {
		logger.Info("HTTP server listening",
			zap.String("port", httpPort))

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server failed", zap.Error(err))
		}
	}()

	// 9. Start poller with message handler callback
	pollerCtx, pollerCancel := context.WithCancel(ctx)
	defer pollerCancel()

	// Set message callback to publish to Redis Streams
	pollerInstance.SetMessageCallback(func(messages []*innertube.RawChatMessage) {
		for _, msg := range messages {
			if err := streamPublisher.Publish(pollerCtx, msg); err != nil {
				logger.Error("Failed to publish message",
					zap.String("message_id", msg.MessageID),
					zap.Error(err))
				// Continue processing other messages (don't crash on Redis error)
			}
		}
	})

	if err := pollerInstance.Start(pollerCtx); err != nil {
		logger.Fatal("Failed to start poller", zap.Error(err))
	}

	// 10. Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	logger.Info("Shutting down service...")

	// Stop poller first
	pollerInstance.Stop()
	logger.Info("Poller stopped")

	// Shutdown HTTP server with 25s timeout (Kubernetes sends SIGKILL at 30s)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer shutdownCancel()

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
			zap.String("version", getEnv("APP_VERSION", "poc")),
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
