// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/message-processor/registry"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/deletion"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/handlers"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/innertube"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/metrics"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/publisher"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/streams"
	"github.com/caesar/all-chat/shared/listener"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// publisherAdapter adapts StreamPublisher to deletion.Publisher interface
type publisherAdapter struct {
	publisher *publisher.StreamPublisher
}

func (a *publisherAdapter) Publish(ctx context.Context, msg deletion.RawMessage) error {
	// Type assert to concrete RawChatMessage
	rawMsg, ok := msg.(*innertube.RawChatMessage)
	if !ok {
		return fmt.Errorf("unexpected message type: %T", msg)
	}
	return a.publisher.Publish(ctx, rawMsg)
}

// metricsAdapter adapts InnerTubeMetrics to deletion.MetricsRecorder interface
type metricsAdapter struct {
	metrics *metrics.InnerTubeMetrics
}

func (a *metricsAdapter) RecordOverflow(channelID string) {
	if a.metrics != nil {
		a.metrics.DeletionBufferOverflows.WithLabelValues(metrics.ServiceLabel, channelID).Inc()
	}
}

func main() {
	// 1. Environment variables
	logLevel := listener.Env("LOG_LEVEL", "info")
	redisHost := listener.Env("REDIS_HOST", "localhost")
	redisPort := listener.Env("REDIS_PORT", "6379")
	httpPort := listener.Env("HTTP_PORT", "8080")

	// 2. Logger initialization
	logger := newLogger("youtube-listener-innertube", logLevel)
	defer logger.Sync()

	logger.Info("Starting YouTube Listener InnerTube",
		zap.String("version", listener.Env("APP_VERSION", "dev")),
		zap.String("log_level", logLevel),
	)

	ctx := context.Background()

	// 3. Initialize Prometheus metrics
	innertubeMetrics := metrics.NewInnerTubeMetrics()
	logger.Info("Initialized Prometheus metrics")

	// 3.5. Initialize batch deletion detector
	batchThresholdStr := listener.Env("BATCH_DELETION_THRESHOLD", "5")
	batchThreshold, err := strconv.Atoi(batchThresholdStr)
	if err != nil {
		logger.Warn("Invalid BATCH_DELETION_THRESHOLD, using default",
			zap.String("value", batchThresholdStr),
			zap.Error(err),
		)
		batchThreshold = 5
	}
	batchDetector := deletion.NewBatchDetector(batchThreshold, logger)
	innertube.SetBatchDetector(batchDetector)
	logger.Info("Initialized batch deletion detector",
		zap.Int("threshold", batchThreshold),
	)

	// 4. Redis client
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

	// 5. Initialize components
	// Fetch current InnerTube client version + API key from YouTube at startup.
	// Falls back to compiled-in defaults if the fetch fails.
	innertubeConfig := innertube.FetchClientConfig(ctx, logger)

	innertubeClient := innertube.NewClient(innertube.ClientOptions{
		APIKey:        innertubeConfig.APIKey,
		ClientVersion: innertubeConfig.ClientVersion,
		Timeout:       10 * time.Second,
		Logger:        logger,
		Metrics:       innertubeMetrics,
	})

	// Discovery
	httpClient := &http.Client{Timeout: 10 * time.Second}
	discovery := innertube.NewDiscovery(httpClient, logger, innertubeConfig)

	// Repository for Redis persistence
	repository := streams.NewRepository(redisClient, logger)

	// Publisher
	streamPublisher := publisher.NewStreamPublisher(redisClient, logger, innertubeMetrics, nil)

	// Message-ID registry: maps InnerTube renderer IDs (Tags["youtube_message_id"])
	// to the internal UUIDs we publish. The message-processor consults this
	// registry when a deletion event arrives — without it, every single-message
	// deletion is silently dropped (#284). 1-hour TTL matches twitch-listener.
	msgRegistry := registry.NewRedisRegistry(redisClient, 1*time.Hour)
	streamPublisher.SetMessageIDRegistry(msgRegistry)
	logger.Info("Initialized message ID registry", zap.Duration("ttl", 1*time.Hour))

	// Create publisher adapter for deletion buffer (adapts RawMessage to RawChatMessage)
	bufferPublisher := &publisherAdapter{publisher: streamPublisher}

	// Initialize deletion buffer with adapted publisher
	deletionBuffer := deletion.NewDeletionBuffer(bufferPublisher, logger)

	// Set metrics recorder
	metricsRec := &metricsAdapter{metrics: innertubeMetrics}
	deletionBuffer.SetMetrics(metricsRec)

	streamPublisher.SetDeletionBuffer(deletionBuffer)
	logger.Info("Initialized deletion event buffer",
		zap.Duration("delay", 500*time.Millisecond),
		zap.Int("max_size", 1000))

	// Leadership coordination via SDK
	ll, err := listener.NewLeadershipListenerFromEnv("youtube", redisClient, logger)
	if err != nil {
		logger.Fatal("Failed to initialize leadership listener", zap.Error(err))
	}

	// 6. Initialize and start stream manager
	streamManager := streams.NewManager(
		ll.LeadershipCoordinator(),
		ll.SMClient(),
		repository,
		discovery,
		streamPublisher,
		innertubeClient,
		redisClient,
		logger,
		innertubeMetrics,
		batchDetector,
		deletionBuffer,
	)

	if err := streamManager.Start(ctx); err != nil {
		logger.Fatal("Failed to start stream manager", zap.Error(err))
	}

	// Demand-driven activation (Phase 5)
	// Leadership-only listeners don't call base.Start so the SDK demand loop
	// doesn't run automatically. Subscribe directly to source:demand here.
	go func() {
		const platformFilter = "youtube"
		pubsub := redisClient.Subscribe(ctx, "source:demand")
		defer pubsub.Close()

		logger.Info("Subscribed to source:demand for demand-driven activation",
			zap.String("platform", platformFilter))

		ch := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				var update struct {
					Type    string `json:"type"`
					Sources []struct {
						SourceID  string `json:"source_id"`
						ChannelID string `json:"channel_id"`
						Platform  string `json:"platform"`
						OverlayID string `json:"overlay_id"`
					} `json:"sources"`
				}
				if err := json.Unmarshal([]byte(msg.Payload), &update); err != nil {
					logger.Warn("Failed to parse demand update", zap.Error(err))
					continue
				}
				demanded := make(map[string]bool)
				for _, s := range update.Sources {
					if s.Platform == platformFilter {
						demanded[s.ChannelID] = true
					}
				}
				logger.Info("Demand update received",
					zap.Int("total_sources", len(update.Sources)),
					zap.Int("platform_sources", len(demanded)))
				streamManager.UpdateDemandedChannels(demanded)
			}
		}
	}()

	// 7. HTTP server with metrics and health checks
	if logLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	// Register Prometheus metrics endpoint (must be before health checks for canary monitoring)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	logger.Info("Registered Prometheus /metrics endpoint")

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
			zap.String("port", httpPort),
			zap.String("metrics_endpoint", "/metrics"))

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server failed", zap.Error(err))
		}
	}()

	// 9. Wait for interrupt signal for graceful shutdown
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

	// Stop ring buffer publisher — drains retry goroutine before closing Redis.
	streamPublisher.Stop()
	logger.Info("Stream publisher ring buffer stopped")

	// Shutdown deletion buffer (flush all remaining events)
	deletionBuffer.Shutdown()
	logger.Info("Deletion buffer shutdown complete")

	// Shutdown HTTP server
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server forced shutdown", zap.Error(err))
	}

	logger.Info("Service stopped gracefully")
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
			zap.String("version", listener.Env("APP_VERSION", "dev")),
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
