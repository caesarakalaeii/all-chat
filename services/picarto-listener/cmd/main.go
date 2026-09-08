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
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/picarto-listener/channels"
	"github.com/caesar/all-chat/services/picarto-listener/handlers"
	"github.com/caesar/all-chat/services/picarto-listener/metrics"
	"github.com/caesar/all-chat/services/picarto-listener/publisher"
	"github.com/caesar/all-chat/services/picarto-listener/status"
	"github.com/caesar/all-chat/services/picarto-listener/token"
	"github.com/caesar/all-chat/services/picarto-listener/websocket"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/listener"
	"github.com/caesar/all-chat/shared/logger"
	sharedmetrics "github.com/caesar/all-chat/shared/metrics"
	sharedredis "github.com/caesar/all-chat/shared/redis"
	"github.com/caesar/all-chat/shared/tracing"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	logLevel := listener.Env("LOG_LEVEL", "info")
	log := logger.NewLogger("picarto-listener", logLevel)
	defer log.Sync()

	log.Info("Starting Picarto Listener",
		zap.String("version", listener.Env("APP_VERSION", "dev")),
	)

	tracingEnabled := listener.Env("OTEL_ENABLED", "false") == "true"
	if tracingEnabled {
		tracingCfg := tracing.Config{
			ServiceName:    "picarto-listener",
			ServiceVersion: listener.Env("APP_VERSION", "dev"),
			Environment:    listener.Env("ENVIRONMENT", "development"),
			OTLPEndpoint:   listener.Env("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
			Enabled:        true,
		}
		shutdownTracer, err := tracing.InitTracer(tracingCfg, log)
		if err != nil {
			log.Error("Failed to initialize tracer", zap.Error(err))
		} else {
			defer shutdownTracer(context.Background())
			log.Info("Tracer initialized", zap.String("service", "picarto-listener"))
		}
	}

	ctx := context.Background()

	dbHost := listener.Env("DATABASE_HOST", "localhost")
	dbPort := listener.Env("DATABASE_PORT", "5432")
	dbUser := listener.Env("DATABASE_USER", "allchat")
	dbPassword := listener.Env("DATABASE_PASSWORD", "")
	if dbPassword == "" {
		log.Fatal("DATABASE_PASSWORD must be set")
	}
	dbName := listener.Env("DATABASE_NAME", "allchat")

	connString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	db, err := database.NewPostgresPool(connString)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	log.Info("Connected to PostgreSQL")

	redisHost := listener.Env("REDIS_HOST", "localhost")
	redisPort := listener.Env("REDIS_PORT", "6379")
	redisAddr := sharedredis.BuildDSN(redisHost, redisPort)

	startupCtx, stopStartup := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	redisClient, err := sharedredis.NewClientWithRetry(startupCtx, redisAddr, listener.Env("REDIS_PASSWORD", ""), tracingEnabled,
		sharedredis.DefaultRetryOptions(),
		func(attempt int, err error, backoff time.Duration) {
			log.Warn("Redis not reachable, retrying with backoff",
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff),
				zap.Error(err),
			)
		})
	stopStartup()
	if err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	defer redisClient.Close()

	log.Info("Connected to Redis")

	shardMetrics := sharedmetrics.NewShardMetrics()
	log.Info("Initialized Prometheus shard metrics")

	podName := listener.Env("HOSTNAME", "picarto-listener-0")

	ll, err := listener.NewLeadershipListenerFromEnv("picarto", redisClient, log)
	if err != nil {
		log.Fatal("Failed to initialize leadership listener", zap.Error(err))
	}

	streamPublisher := publisher.NewStreamPublisher(redisClient, log)
	channelRepo := channels.NewRepository(db, log)
	dbWrapper := &dbConnWrapper{pool: db}

	statusPublisher := status.NewPublisher(redisClient, log)
	log.Info("Initialized platform status publisher")

	var channelMgr *channels.Manager

	messageHandler := func(channelName string, message *websocket.ChatMessage) {
		if channelMgr == nil {
			log.Warn("Channel manager not initialized yet")
			return
		}
		handleChatMessage(channelName, message, streamPublisher, channelMgr, log)
	}

	tokenFetcher := token.NewFetcher().Fetch
	wsClient := websocket.NewClient(log, tokenFetcher, messageHandler)

	channelMgr = channels.NewManager(channelRepo, wsClient, streamPublisher, dbWrapper, ll.LeadershipCoordinator(), redisClient, podName, log)
	channelMgr.SetStatusPublisher(statusPublisher)

	if logLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	if tracingEnabled {
		router.Use(tracing.GinMiddleware("picarto-listener"))
	}

	healthHandler := handlers.NewHealthHandler(streamPublisher, channelMgr)
	router.GET("/health/live", healthHandler.LivenessProbe)
	router.GET("/health/ready", healthHandler.ReadinessProbe)
	router.GET("/status", healthHandler.Status)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	port := listener.Env("PORT", "8097")

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("HTTP server listening", zap.String("port", port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start HTTP server", zap.Error(err))
		}
	}()

	if err := ll.Start(ctx, channelMgr); err != nil {
		log.Fatal("Failed to start listener", zap.Error(err))
	}

	filteredCount := channelMgr.GetFilteredAssignmentCount()
	shardMetrics.PodChannelCount.WithLabelValues(podName).Set(float64(filteredCount))
	log.Info("Recorded channel count metric",
		zap.String("pod_id", podName),
		zap.Int("channel_count", filteredCount),
	)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down service...")

	streamPublisher.Stop()
	wsClient.DisconnectAll()

	listener.ShutdownCoordinator(ll, channelMgr,
		func() {},
		srv,
		log,
	)

	log.Info("Service exited")
}

// handleChatMessage processes an incoming Picarto chat message and publishes
// one RawMessage per consuming overlay.
func handleChatMessage(
	channelName string,
	message *websocket.ChatMessage,
	pub *publisher.StreamPublisher,
	channelMgr *channels.Manager,
	log *zap.Logger,
) {
	start := time.Now()

	targets, found := channelMgr.GetOverlayTargetsForChannel(channelName)
	if !found || len(targets) == 0 {
		log.Debug("Received message for unknown channel",
			zap.String("channel", channelName),
		)
		metrics.IncDropped("unknown_channel")
		metrics.IncMessage("dropped", "unknown_channel")
		return
	}

	rawMsg, err := json.Marshal(message)
	if err != nil {
		log.Error("Failed to marshal raw message", zap.Error(err))
		metrics.IncDropped("marshal_error")
		metrics.IncMessage("dropped", "marshal_error")
		return
	}

	messageID := message.MessageID
	if messageID == "" {
		messageID = fmt.Sprintf("picarto-%s-%d", channelName, time.Now().UnixNano())
	}

	timestamp := start
	if message.Timestamp > 0 {
		timestamp = time.UnixMilli(message.Timestamp)
	}

	for _, target := range targets {
		publishCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		msg := publisher.RawMessage{
			MessageID:   messageID,
			Platform:    "picarto",
			OverlayID:   target.OverlayID,
			ChannelID:   target.ChannelName,
			ChannelName: target.ChannelName,
			UserID:      message.UserID,
			Username:    message.Username,
			Text:        message.Text,
			Tags: map[string]string{
				"display_name": message.DisplayName,
				"avatar_url":   message.AvatarURL,
			},
			RawMessage: rawMsg,
			Timestamp:  timestamp,
		}

		if err := pub.Publish(publishCtx, &msg); err != nil {
			log.Error("Failed to publish message",
				zap.Error(err),
				zap.String("overlay_id", target.OverlayID),
				zap.String("channel", target.ChannelName),
			)
			metrics.IncDropped("publish_error")
			metrics.IncMessage("failed", "publish_error")
		} else {
			log.Debug("Published Picarto message",
				zap.String("overlay_id", target.OverlayID),
				zap.String("channel", target.ChannelName),
				zap.String("sender", message.Username),
				zap.Duration("publish_latency", time.Since(start)),
			)
			metrics.IncMessage("published", "success")
			metrics.ObservePublishLatency(time.Since(start))
		}
		cancel()
	}
}

// dbConnWrapper adapts *pgxpool.Pool to channels.DBConnInterface.
type dbConnWrapper struct {
	pool *pgxpool.Pool
}

func (w *dbConnWrapper) GetPool() interface{} {
	return w.pool
}
