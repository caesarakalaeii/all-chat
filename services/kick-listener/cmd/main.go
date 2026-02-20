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
	"github.com/caesar/all-chat/services/kick-listener/websocket"
	"github.com/caesar/all-chat/shared/coordination"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/caesar/all-chat/shared/sourcemanager"
	"github.com/caesar/all-chat/shared/tracing"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logLevel := getEnvOrDefault("LOG_LEVEL", "info")
	log := logger.NewLogger("kick-listener", logLevel)
	defer log.Sync()

	log.Info("Starting Kick Listener",
		zap.String("version", getEnvOrDefault("APP_VERSION", "dev")),
	)

	// Initialize tracing
	tracingEnabled := getEnvOrDefault("OTEL_ENABLED", "false") == "true"
	if tracingEnabled {
		tracingCfg := tracing.Config{
			ServiceName:    "kick-listener",
			ServiceVersion: getEnvOrDefault("APP_VERSION", "dev"),
			Environment:    getEnvOrDefault("ENVIRONMENT", "development"),
			OTLPEndpoint:   getEnvOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
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

	// Initialize coordinator client and query assignments (KICK-01)
	coordinatorURL := getEnvOrDefault("COORDINATOR_URL", "http://source-manager:8088")
	serviceJWT := getEnvOrDefault("SERVICE_JWT_SECRET", "dev-service-secret")
	podName := os.Getenv("HOSTNAME") // Kubernetes sets HOSTNAME to pod name
	if podName == "" {
		podName = "kick-listener-local" // Fallback for local development
	}

	coordClient := coordination.NewCoordinatorClient(coordinatorURL, serviceJWT, log)

	// Query assignments from coordinator (blocks until response)
	assignments, err := coordClient.QueryAssignments(ctx, podName)
	if err != nil {
		log.Fatal("Failed to query coordinator assignments", zap.Error(err))
	}
	log.Info("Received assignments from coordinator",
		zap.Int("count", len(assignments)),
		zap.String("pod_id", podName),
	)

	// Extract assigned source IDs into map for filtering
	assignedSourceIDs := make(map[string]bool)
	for _, a := range assignments {
		assignedSourceIDs[a.SourceID] = true
	}

	// Configure Pusher connection
	pusherAppKey := getEnvOrDefault("KICK_PUSHER_APP_KEY", "32cbd69e4b950bf97679")
	if pusherAppKey == "" {
		log.Fatal("KICK_PUSHER_APP_KEY must be set", zap.Error(fmt.Errorf("missing app key")))
	}

	primaryCluster := getEnvOrDefault("KICK_PUSHER_CLUSTER", "us2")
	fallbackClusterEnv := os.Getenv("KICK_PUSHER_CLUSTER_FALLBACKS")
	clusterList := buildClusterList(primaryCluster, parseClusterList(fallbackClusterEnv))

	wsConfig := websocket.Config{
		AppKey:   pusherAppKey,
		Clusters: clusterList,
	}

	sourceManagerURL := getEnvOrDefault("SOURCE_MANAGER_URL", "http://source-controller:8088")
	sourceManagerSecret := getEnvOrDefault("SOURCE_MANAGER_SECRET", "dev-service-secret")
	var leaderCoord *sourcemanager.LeadershipCoordinator
	if sourceManagerSecret == "" {
		log.Warn("SOURCE_MANAGER_SECRET not set; Kick Listener will not coordinate leadership")
	} else {
		tokenSource := sourcemanager.NewSigningTokenSource("kick-listener", sourceManagerSecret, 15*time.Minute)
		smClient, err := sourcemanager.NewClient(sourceManagerURL, tokenSource)
		if err != nil {
			log.Fatal("Failed to initialize Source Manager client", zap.Error(err))
		}
		leaderCoord = sourcemanager.NewLeadershipCoordinator("kick", smClient, 5*time.Second, log)
	}

	// Initialize components
	streamPublisher := publisher.NewStreamPublisher(redisClient, log)
	channelRepo := channels.NewRepository(db, log)
	dbWrapper := &dbConnWrapper{pool: db}

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

	// Now initialize channel manager with the WebSocket client and assigned source IDs
	channelMgr = channels.NewManager(channelRepo, wsClient, streamPublisher, dbWrapper, leaderCoord, assignedSourceIDs, log)

	// Connect to Kick Pusher WebSocket
	if err := wsClient.Connect(); err != nil {
		log.Fatal("Failed to connect to Kick WebSocket", zap.Error(err))
	}

	// Wait a bit for WebSocket connection to establish
	time.Sleep(2 * time.Second)

	// Start channel manager (will sync and subscribe to channels)
	if err := channelMgr.Start(); err != nil {
		log.Fatal("Failed to start channel manager", zap.Error(err))
	}

	// Start migration subscriber (KICK-03, KICK-04)
	migrationSub := coordination.NewMigrationSubscriber(
		redisClient,
		channelMgr.HandleMigrationEvent,
		log,
	)
	go func() {
		if err := migrationSub.Subscribe(ctx); err != nil {
			log.Error("Migration subscriber error", zap.Error(err))
		}
	}()

	// Start heartbeat publisher (KICK-02)
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := coordClient.PublishHeartbeat(ctx, podName); err != nil {
					log.Warn("Failed to publish heartbeat", zap.Error(err))
				}
			}
		}
	}()

	// Start assignment refresh (re-query every 60 seconds to pick up dynamic changes)
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				newAssignments, err := coordClient.QueryAssignments(ctx, podName)
				if err != nil {
					log.Warn("Failed to refresh assignments", zap.Error(err))
					continue
				}

				// Update assignedSourceIDs map
				newAssignedIDs := make(map[string]bool)
				for _, a := range newAssignments {
					newAssignedIDs[a.SourceID] = true
				}

				channelMgr.UpdateAssignedSourceIDs(newAssignedIDs)

				log.Info("Refreshed assignments from coordinator",
					zap.Int("count", len(newAssignments)),
					zap.String("pod_id", podName),
				)
			}
		}
	}()

	log.Info("Started assignment refresh", zap.Duration("interval", 60*time.Second))

	// Handle reconnections
	go handleReconnections(wsClient, channelMgr, log)

	// Set up HTTP server for health checks
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
	port := getEnvOrDefault("PORT", "8089")

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start HTTP server in goroutine
	go func() {
		log.Info("HTTP server listening",
			zap.String("port", port),
		)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start HTTP server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down service...")

	// Stop channel manager
	channelMgr.Stop()

	// Disconnect from WebSocket
	if err := wsClient.Disconnect(); err != nil {
		log.Error("Error disconnecting from WebSocket", zap.Error(err))
	}

	// Shutdown HTTP server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP server forced to shutdown", zap.Error(err))
	}

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

// getEnvOrDefault gets an environment variable or returns a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
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
