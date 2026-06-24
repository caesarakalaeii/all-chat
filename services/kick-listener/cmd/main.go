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
	"strings"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/kick-listener/channels"
	"github.com/caesar/all-chat/services/kick-listener/handlers"
	"github.com/caesar/all-chat/services/kick-listener/metrics"
	"github.com/caesar/all-chat/services/kick-listener/publisher"
	"github.com/caesar/all-chat/services/kick-listener/status"
	"github.com/caesar/all-chat/services/kick-listener/websocket"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/encryption"
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
	log := logger.NewLogger("kick-listener", logLevel)
	defer log.Sync()

	log.Info("Starting Kick Listener",
		zap.String("version", listener.Env("APP_VERSION", "dev")),
	)

	// Initialize tracing
	tracingEnabled := listener.Env("OTEL_ENABLED", "false") == "true"
	if tracingEnabled {
		tracingCfg := tracing.Config{
			ServiceName:    "kick-listener",
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
			log.Info("Tracer initialized", zap.String("service", "kick-listener"))
		}
	}

	ctx := context.Background()

	// Connect to PostgreSQL
	dbHost := listener.Env("DATABASE_HOST", "localhost")
	dbPort := listener.Env("DATABASE_PORT", "5432")
	dbUser := listener.Env("DATABASE_USER", "allchat")
	dbPassword := listener.Env("DATABASE_PASSWORD", "allchat_dev_password")
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
	podName := listener.Env("HOSTNAME", "kick-listener-0")

	ll, err := listener.NewLeadershipListenerFromEnv("kick", redisClient, log)
	if err != nil {
		log.Fatal("Failed to initialize leadership listener", zap.Error(err))
	}

	// Configure Pusher connection
	pusherAppKey := listener.Env("KICK_PUSHER_APP_KEY", "32cbd69e4b950bf97679")
	if pusherAppKey == "" {
		log.Fatal("KICK_PUSHER_APP_KEY must be set", zap.Error(fmt.Errorf("missing app key")))
	}

	primaryCluster := listener.Env("KICK_PUSHER_CLUSTER", "us2")
	fallbackClusterEnv := os.Getenv("KICK_PUSHER_CLUSTER_FALLBACKS")
	clusterList := buildClusterList(primaryCluster, parseClusterList(fallbackClusterEnv))

	wsConfig := websocket.Config{
		AppKey:   pusherAppKey,
		Clusters: clusterList,
	}

	// Initialize token cipher for kick_oauth_tokens decryption (D-16).
	// Optional: if TOKEN_ENCRYPTION_KEY_V1 is not set, cipher is nil and only
	// plaintext rows (encryption_version=0) are supported.
	var tokenCipher *encryption.MultiKeyEncryptor
	if cipher, cipherErr := encryption.NewMultiKeyEncryptorFromEnv(); cipherErr == nil {
		tokenCipher = cipher
		log.Info("kick token cipher initialized", zap.Uint8("current_kid", tokenCipher.CurrentKid()))
	} else {
		log.Warn("kick token cipher not configured — encrypted kick_oauth_tokens will fail",
			zap.Error(cipherErr),
		)
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
	messageHandler := func(channel string, message *websocket.KickChatMessage) {
		if channelMgr == nil {
			log.Warn("Channel manager not initialized yet")
			return
		}
		handleChatMessage(channel, message, streamPublisher, channelMgr, log)
	}

	// Create WebSocket client with message handler
	wsClient := websocket.NewClient(wsConfig, messageHandler, log)

	// Set deletion handler
	wsClient.SetDeletionHandler(func(channel string, event *websocket.KickMessageDeletedEvent) {
		handleDeletionEvent(channel, event, streamPublisher, channelMgr, log)
	})

	// Now initialize channel manager with the WebSocket client
	// Pass nil for assignedSourceIDs — SDK populates via UpdateAssignedSourceIDs inside ll.Start
	channelMgr = channels.NewManager(channelRepo, wsClient, streamPublisher, dbWrapper, ll.LeadershipCoordinator(), nil, redisClient, podName, tokenCipher, log)

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
		router.Use(tracing.GinMiddleware("kick-listener"))
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
	channel string,
	message *websocket.KickChatMessage,
	pub *publisher.StreamPublisher,
	channelMgr *channels.Manager,
	log *zap.Logger,
) {
	start := time.Now()

	// Prefer chatroom ID embedded in message payload; fall back to channel name.
	chatroomID := message.ChatroomID
	if chatroomID == 0 {
		if _, err := fmt.Sscanf(channel, "chatrooms.%d.v2", &chatroomID); err != nil {
			fmt.Sscanf(channel, "chatrooms.%d", &chatroomID)
		}
	}

	if chatroomID == 0 {
		log.Warn("Unable to determine chatroom ID from message",
			zap.String("channel", channel),
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
			zap.String("channel", channel),
			zap.Int("chatroom_id", chatroomID),
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
		messageID = fmt.Sprintf("kick-%s-%d", channel, time.Now().UnixNano())
	}

	userID := ""
	if message.Sender.ID > 0 {
		userID = strconv.Itoa(message.Sender.ID)
	}

	username := pickKickUsername(message.Sender.Username, message.Sender.Slug)
	text := message.Content
	tags := buildKickTags(message, chatroomID)

	for _, target := range targets {
		publishCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		msg := publisher.RawMessage{
			MessageID:   messageID,
			Platform:    "kick",
			OverlayID:   target.OverlayID,
			ChannelID:   target.ChannelSlug,
			ChannelName: target.ChannelSlug,
			UserID:      userID,
			Username:    username,
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
			log.Debug("Published Kick message",
				zap.String("overlay_id", target.OverlayID),
				zap.String("channel", target.ChannelSlug),
				zap.String("sender", message.Sender.Username),
				zap.Duration("publish_latency", time.Since(start)),
			)
			metrics.IncMessage("published", "success")
			metrics.ObservePublishLatency(time.Since(start))
		}
		cancel()
	}
}

func buildKickTags(message *websocket.KickChatMessage, chatroomID int) map[string]string {
	tags := map[string]string{
		"chatroom_id": strconv.Itoa(chatroomID),
	}

	if message.Type != "" {
		tags["message_type"] = message.Type
	}

	if slug := message.Sender.Slug; slug != "" {
		tags["sender_slug"] = slug
	}

	if uname := message.Sender.Username; uname != "" {
		tags["sender_username"] = uname
	}

	if color := message.Sender.Identity.Color; color != "" {
		tags["color"] = color
	}

	if badgeList := formatKickBadges(message.Sender.Identity.Badges); badgeList != "" {
		tags["badges"] = badgeList
	}

	return tags
}

func formatKickBadges(badges []websocket.KickBadge) string {
	if len(badges) == 0 {
		return ""
	}

	names := make([]string, 0, len(badges))
	for _, badge := range badges {
		if badge.Type != "" {
			names = append(names, badge.Type)
		}
	}

	return strings.Join(names, ",")
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
	channel string,
	event *websocket.KickMessageDeletedEvent,
	pub *publisher.StreamPublisher,
	channelMgr *channels.Manager,
	log *zap.Logger,
) {
	// Extract chatroom ID from channel (format: "chatrooms.{id}.v2")
	chatroomIDStr := strings.TrimPrefix(channel, "chatrooms.")
	chatroomIDStr = strings.TrimSuffix(chatroomIDStr, ".v2")
	chatroomID, err := strconv.Atoi(chatroomIDStr)
	if err != nil {
		log.Error("Failed to parse chatroom ID from channel",
			zap.String("channel", channel),
			zap.Error(err))
		metrics.IncDropped("invalid_chatroom_id")
		return
	}

	// Get overlay targets for this chatroom
	targets, found := channelMgr.GetOverlayTargetsForChatroom(chatroomID)
	if !found || len(targets) == 0 {
		log.Debug("Received deletion event for unknown chatroom",
			zap.String("channel", channel),
			zap.Int("chatroom_id", chatroomID),
		)
		metrics.IncDropped("unknown_chatroom")
		return
	}

	// Publish deletion event for each overlay target
	for _, target := range targets {
		publishCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		// Build tags with deletion event metadata following Phase 1 schema
		tags := make(map[string]string)
		tags["event_type"] = "message_deletion"
		tags["deletion_type"] = "single" // Kick only supports single message deletion
		tags["target_msg_id"] = event.DeletedMessage.ID
		tags["deleted_by"] = strconv.Itoa(event.DeletedMessage.DeletedBy)
		tags["chatroom_id"] = event.DeletedMessage.ChatroomID

		// Create raw deletion event
		rawMsg := publisher.RawMessage{
			Platform:    "kick",
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
				zap.String("channel", target.ChannelSlug),
				zap.String("message_id", event.DeletedMessage.ID),
			)
			metrics.IncDropped("publish_error")
		} else {
			log.Debug("Published Kick deletion event to Redis Streams",
				zap.String("overlay_id", target.OverlayID),
				zap.String("message_id", event.DeletedMessage.ID),
				zap.Int("deleted_by", event.DeletedMessage.DeletedBy),
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
		log.Warn("WebSocket disconnected, attempting to reconnect...")

		// Exponential backoff for reconnection
		backoff := time.Second
		maxBackoff := 60 * time.Second

		for {
			log.Info("Reconnecting to Kick WebSocket", zap.Duration("backoff", backoff))
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

			log.Info("Reconnected to Kick WebSocket successfully")

			// Wait for connection to stabilize
			time.Sleep(2 * time.Second)

			// Re-subscribe to all channels
			log.Info("Re-syncing channels after reconnection")
			// The channel manager's sync loop will handle re-subscriptions
			break
		}
	}
}

func parseClusterList(raw string) []string {
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	clusters := make([]string, 0, len(parts))
	for _, part := range parts {
		cluster := strings.TrimSpace(part)
		if cluster != "" {
			clusters = append(clusters, cluster)
		}
	}
	return clusters
}

func buildClusterList(primary string, fallbacks []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, 1+len(fallbacks))

	add := func(cluster string) {
		cluster = strings.TrimSpace(cluster)
		if cluster == "" {
			return
		}
		if _, exists := seen[cluster]; exists {
			return
		}
		seen[cluster] = struct{}{}
		result = append(result, cluster)
	}

	add(primary)
	for _, cluster := range fallbacks {
		add(cluster)
	}

	return result
}

// dbConnWrapper wraps pgxpool.Pool to implement DBConnInterface
type dbConnWrapper struct {
	pool *pgxpool.Pool
}

func (w *dbConnWrapper) GetPool() interface{} {
	return w.pool
}
