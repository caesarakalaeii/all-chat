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

	"github.com/caesar/all-chat/services/kick-listener/channels"
	"github.com/caesar/all-chat/services/kick-listener/handlers"
	"github.com/caesar/all-chat/services/kick-listener/publisher"
	"github.com/caesar/all-chat/services/kick-listener/websocket"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
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

	// Initialize components
	streamPublisher := publisher.NewStreamPublisher(redisClient, log)
	channelRepo := channels.NewRepository(db, log)

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
	wsClient := websocket.NewClient(messageHandler, log)

	// Now initialize channel manager with the WebSocket client
	channelMgr = channels.NewManager(channelRepo, wsClient, streamPublisher, log)

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

	// Health check handlers
	healthHandler := handlers.NewHealthHandler(wsClient, streamPublisher, channelMgr)
	router.GET("/health/live", healthHandler.LivenessProbe)
	router.GET("/health/ready", healthHandler.ReadinessProbe)
	router.GET("/status", healthHandler.Status)

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
	// Extract chatroom ID from channel name (format: "chatrooms.123456")
	var chatroomID int
	fmt.Sscanf(channel, "chatrooms.%d", &chatroomID)

	// Get overlay ID for this chatroom
	overlayID, channelSlug, found := channelMgr.GetOverlayIDForChatroom(chatroomID)
	if !found {
		log.Warn("Received message for unknown chatroom",
			zap.String("channel", channel),
			zap.Int("chatroom_id", chatroomID),
		)
		return
	}

	// Marshal raw message
	rawMsg, err := json.Marshal(message)
	if err != nil {
		log.Error("Failed to marshal raw message", zap.Error(err))
		return
	}

	// Create raw message for publishing
	msg := publisher.RawMessage{
		Platform:    "kick",
		OverlayID:   overlayID,
		ChannelID:   channelSlug,
		ChannelName: channelSlug,
		RawMessage:  rawMsg,
		Timestamp:   time.Now(),
	}

	// Publish to Redis Stream
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pub.Publish(ctx, &msg); err != nil {
		log.Error("Failed to publish message",
			zap.Error(err),
			zap.String("overlay_id", overlayID),
			zap.String("channel", channelSlug),
		)
	}

	log.Debug("Published Kick message",
		zap.String("overlay_id", overlayID),
		zap.String("channel", channelSlug),
		zap.String("sender", message.Sender.Username),
	)
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

// dbConnWrapper wraps pgxpool.Pool to implement DBConnInterface
type dbConnWrapper struct {
	pool *pgxpool.Pool
}

func (w *dbConnWrapper) GetPool() interface{} {
	return w.pool
}
