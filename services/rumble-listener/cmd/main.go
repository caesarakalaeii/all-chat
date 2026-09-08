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

	"github.com/caesar/all-chat/services/rumble-listener/channels"
	"github.com/caesar/all-chat/services/rumble-listener/handlers"
	"github.com/caesar/all-chat/services/rumble-listener/metrics"
	"github.com/caesar/all-chat/services/rumble-listener/publisher"
	"github.com/caesar/all-chat/services/rumble-listener/status"
	"github.com/caesar/all-chat/services/rumble-listener/websocket"
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
	// Initialize logger
	logLevel := listener.Env("LOG_LEVEL", "info")
	log := logger.NewLogger("rumble-listener", logLevel)
	defer log.Sync()

	log.Info("Starting Kick Listener",
		zap.String("version", listener.Env("APP_VERSION", "dev")),
	)

	// Initialize tracing
	tracingEnabled := listener.Env("OTEL_ENABLED", "false") == "true"
	if tracingEnabled {
		tracingCfg := tracing.Config{
			ServiceName:    "rumble-listener",
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
			log.Info("Tracer initialized", zap.String("service", "rumble-listener"))
		}
	}

	ctx := context.Background()

	// Connect to PostgreSQL
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

	// Connect to Redis, retrying with backoff so a transient Redis outage
	// (e.g. the pod being rescheduled onto another node) does not crash-loop
	// this service. The retry is cancelled on shutdown signals so SIGTERM still
	// terminates the process promptly while it is waiting for Redis.
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

	// Initialize shard metrics (available via /metrics endpoint)
	shardMetrics := sharedmetrics.NewShardMetrics()
	log.Info("Initialized Prometheus shard metrics")

	// SDK setup
	podName := listener.Env("HOSTNAME", "rumble-listener-0")

	ll, err := listener.NewLeadershipListenerFromEnv("rumble", redisClient, log)
	if err != nil {
		log.Fatal("Failed to initialize leadership listener", zap.Error(err))
	}

	// Configure the Rumble chat SSE client. Anonymous reads work for
	// arbitrary channels (ADR-0061 spike); the optional session cookie only
	// preserves follower/subscriber-scoped detail.
	wsConfig := websocket.Config{
		SessionCookie: os.Getenv("RUMBLE_SESSION_COOKIE"),
	}
	if wsConfig.SessionCookie != "" {
		log.Info("Rumble session cookie configured")
	}

	// Initialize components
	streamPublisher := publisher.NewStreamPublisher(redisClient, log)
	channelRepo := channels.NewRepository(db, log)
	dbWrapper := &dbConnWrapper{pool: db}

	// Initialize status publisher for platform status indicators
	statusPublisher := status.NewPublisher(redisClient, log)
	log.Info("Initialized platform status publisher")

	// Create channel manager first (without WebSocket client yet)
	var channelMgr *channels.Manager

	// Create message handler that uses channelMgr (will be set before use)
	messageHandler := func(chatID string, message *websocket.SSEMessage) {
		if channelMgr == nil {
			log.Warn("Channel manager not initialized yet")
			return
		}
		handleChatMessage(chatID, message, streamPublisher, channelMgr, log)
	}

	// Create WebSocket client with message handler
	wsClient := websocket.NewClient(wsConfig, messageHandler, log)

	// Set deletion handler
	wsClient.SetDeletionHandler(func(chatID string, messageID string) {
		handleDeletionEvent(chatID, messageID, streamPublisher, channelMgr, log)
	})

	// Now initialize channel manager with the WebSocket client
	// Pass nil for assignedSourceIDs — SDK populates via UpdateAssignedSourceIDs inside ll.Start
	channelMgr = channels.NewManager(channelRepo, wsClient, streamPublisher, dbWrapper, ll.LeadershipCoordinator(), nil, redisClient, podName, log)

	// Inject status publisher into channel manager
	channelMgr.SetStatusPublisher(statusPublisher)

	// Set up HTTP server for health checks — must start BEFORE ll.Start() because
	// ll.Start() → mgr.Start() → SyncChannels may block while subscribing to channels.
	// Starting the server first ensures the liveness probe never times out during init.
	if logLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	if tracingEnabled {
		router.Use(tracing.GinMiddleware("rumble-listener"))
	}

	// Health check handlers
	healthHandler := handlers.NewHealthHandler(wsClient, streamPublisher, channelMgr)
	router.GET("/health/live", healthHandler.LivenessProbe)
	router.GET("/health/ready", healthHandler.ReadinessProbe)
	router.GET("/status", healthHandler.Status)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Get port
	port := listener.Env("PORT", "8089")

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start HTTP server in goroutine before the blocking ll.Start() call
	go func() {
		log.Info("HTTP server listening",
			zap.String("port", port),
		)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start HTTP server", zap.Error(err))
		}
	}()

	// Connect to Kick Pusher WebSocket
	if err := wsClient.Connect(); err != nil {
		log.Fatal("Failed to connect to Kick WebSocket", zap.Error(err))
	}

	// Wait a bit for WebSocket connection to establish
	time.Sleep(2 * time.Second)

	if err := ll.Start(ctx, channelMgr); err != nil {
		log.Fatal("Failed to start listener", zap.Error(err))
	}

	// Record per-pod channel count metric (after filtering by coordinator assignments)
	filteredCount := channelMgr.GetFilteredAssignmentCount()
	shardMetrics.PodChannelCount.WithLabelValues(podName).Set(float64(filteredCount))
	log.Info("Recorded channel count metric",
		zap.String("pod_id", podName),
		zap.Int("channel_count", filteredCount),
	)

	// Handle reconnections
	go handleReconnections(wsClient, channelMgr, log)

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down service...")

	// Stop ring buffer publisher — drains retry goroutine before disconnecting WebSocket.
	streamPublisher.Stop()

	listener.ShutdownCoordinator(ll, channelMgr,
		func() { _ = wsClient.Disconnect() },
		srv,
		log,
	)

	log.Info("Service exited")
}

// handleChatMessage processes a chat message and publishes to Redis
func handleChatMessage(
	chatID string,
	message *websocket.SSEMessage,
	pub *publisher.StreamPublisher,
	channelMgr *channels.Manager,
	log *zap.Logger,
) {
	start := time.Now()

	chatroomID, err := strconv.Atoi(chatID)
	if err != nil || chatroomID <= 0 {
		log.Warn("Unable to determine chatroom ID from message",
			zap.String("chat_id", chatID),
		)
		metrics.IncDropped("missing_chatroom_id")
		metrics.IncMessage("dropped", "missing_chatroom_id")
		return
	}

	// Signal first message for migration protocol
	channelMgr.SignalFirstMessage(chatroomID)

	// Get overlay targets for this chatroom
	targets, found := channelMgr.GetOverlayTargetsForChatroom(chatroomID)
	if !found || len(targets) == 0 {
		log.Warn("Received message for unknown chatroom",
			zap.String("chat_id", chatID),
		)
		metrics.IncDropped("unknown_chatroom")
		metrics.IncMessage("dropped", "unknown_chatroom")
		return
	}

	// Marshal raw message
	rawMsg, err := json.Marshal(message)
	if err != nil {
		log.Error("Failed to marshal raw message", zap.Error(err))
		metrics.IncDropped("marshal_error")
		metrics.IncMessage("dropped", "marshal_error")
		return
	}

	messageID := message.ID
	if messageID == "" {
		messageID = fmt.Sprintf("rumble-%s-%d", chatID, time.Now().UnixNano())
	}

	// Sender name and colour live on the users array of the SSE batch, not on
	// the message; the raw_message carries them, so the normalizer resolves
	// from there. Tags carry everything numeric for the normalizer fallbacks.
	tags := buildRumbleTags(message, chatID, targets[0].ChannelSlug)
	text := message.Text

	for _, target := range targets {
		publishCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		msg := publisher.RawMessage{
			MessageID:   messageID,
			Platform:    "rumble",
			OverlayID:   target.OverlayID,
			ChannelID:   target.ChannelSlug,
			ChannelName: target.ChannelSlug,
			UserID:      message.UserID,
			Text:        text,
			Tags:        tags,
			RawMessage:  rawMsg,
			Timestamp:   start,
		}

		if err := pub.Publish(publishCtx, &msg); err != nil {
			log.Error("Failed to publish message",
				zap.Error(err),
				zap.String("overlay_id", target.OverlayID),
				zap.String("channel", target.ChannelSlug),
			)
			metrics.IncDropped("publish_error")
			metrics.IncMessage("failed", "publish_error")
		} else {
			log.Debug("Published Rumble message",
				zap.String("overlay_id", target.OverlayID),
				zap.String("channel", target.ChannelSlug),
				zap.Duration("publish_latency", time.Since(start)),
			)
			metrics.IncMessage("published", "success")
			metrics.ObservePublishLatency(time.Since(start))
		}
		cancel()
	}
}

// buildRumbleTags packs the SSE message metadata into tags. The normalizer
// reads them back; every field is optional and defensively populated.
func buildRumbleTags(message *websocket.SSEMessage, chatID, channelSlug string) map[string]string {
	tags := map[string]string{
		"chatroom_id":  chatID,
		"channel_slug": channelSlug,
	}

	if message.Type != "" {
		tags["message_type"] = message.Type
	}
	if message.Rant != nil && message.Rant.PriceCents > 0 {
		tags["rant_price_cents"] = strconv.Itoa(message.Rant.PriceCents)
		tags["rant_duration"] = strconv.Itoa(message.Rant.Duration)
		tags["event_type"] = "rant"
	}
	if message.UserID != "" {
		tags["sender_user_id"] = message.UserID
	}
	return tags
}

func pickKickUsername(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// handleDeletionEvent processes a Kick deletion event and publishes to Redis Streams
func handleDeletionEvent(
	chatID string,
	messageID string,
	pub *publisher.StreamPublisher,
	channelMgr *channels.Manager,
	log *zap.Logger,
) {
	chatroomID, err := strconv.Atoi(chatID)
	if err != nil {
		log.Error("Failed to parse chatroom ID",
			zap.String("chat_id", chatID),
			zap.Error(err))
		metrics.IncDropped("invalid_chatroom_id")
		return
	}

	targets, found := channelMgr.GetOverlayTargetsForChatroom(chatroomID)
	if !found || len(targets) == 0 {
		log.Debug("Received deletion event for unknown chatroom",
			zap.String("chat_id", chatID),
		)
		metrics.IncDropped("unknown_chatroom")
		return
	}

	for _, target := range targets {
		publishCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		tags := map[string]string{
			"event_type":    "message_deletion",
			"deletion_type": "single",
			"target_msg_id": messageID,
			"chatroom_id":   chatID,
		}

		rawMsg := publisher.RawMessage{
			Platform:    "rumble",
			OverlayID:   target.OverlayID,
			ChannelID:   target.ChannelSlug,
			ChannelName: target.ChannelSlug,
			Tags:        tags,
			Timestamp:   time.Now().UTC(),
		}

		if err := pub.Publish(publishCtx, &rawMsg); err != nil {
			log.Error("Failed to publish deletion event to Redis",
				zap.Error(err),
				zap.String("overlay_id", target.OverlayID),
				zap.String("message_id", messageID),
			)
			metrics.IncDropped("publish_error")
		} else {
			log.Debug("Published Rumble deletion event to Redis Streams",
				zap.String("overlay_id", target.OverlayID),
				zap.String("message_id", messageID),
			)
			metrics.IncMessage("published", "deletion")
		}
		cancel()
	}
}

// handleReconnections handles WebSocket reconnection logic
func handleReconnections(
	wsClient *websocket.Client,
	channelMgr *channels.Manager,
	log *zap.Logger,
) {
	for range wsClient.ReconnectChan() {
		log.Warn("chat stream disconnected, attempting to reconnect...")

		// Exponential backoff for reconnection
		backoff := time.Second
		maxBackoff := 60 * time.Second

		for {
			log.Info("Reconnecting to Rumble chat stream", zap.Duration("backoff", backoff))
			time.Sleep(backoff)

			if err := wsClient.Connect(); err != nil {
				log.Error("Reconnection failed", zap.Error(err))

				// Increase backoff
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			}

			log.Info("Reconnected to Rumble chat stream successfully")

			// Wait for connection to stabilize
			time.Sleep(2 * time.Second)

			// Re-subscribe to all channels
			log.Info("Re-syncing channels after reconnection")
			// The channel manager's sync loop will handle re-subscriptions
			break
		}
	}
}

// dbConnWrapper wraps pgxpool.Pool to implement DBConnInterface
type dbConnWrapper struct {
	pool *pgxpool.Pool
}

func (w *dbConnWrapper) GetPool() interface{} {
	return w.pool
}
