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

	"github.com/caesar/all-chat/services/goodgame-listener/channels"
	"github.com/caesar/all-chat/services/goodgame-listener/handlers"
	"github.com/caesar/all-chat/services/goodgame-listener/metrics"
	"github.com/caesar/all-chat/services/goodgame-listener/publisher"
	"github.com/caesar/all-chat/services/goodgame-listener/status"
	"github.com/caesar/all-chat/services/goodgame-listener/websocket"
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
	log := logger.NewLogger("goodgame-listener", logLevel)
	defer log.Sync()

	log.Info("Starting GoodGame Listener",
		zap.String("version", listener.Env("APP_VERSION", "dev")),
	)

	tracingEnabled := listener.Env("OTEL_ENABLED", "false") == "true"
	if tracingEnabled {
		tracingCfg := tracing.Config{
			ServiceName:    "goodgame-listener",
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
			log.Info("Tracer initialized")
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

	podName := listener.Env("HOSTNAME", "goodgame-listener-0")

	ll, err := listener.NewLeadershipListenerFromEnv("goodgame", redisClient, log)
	if err != nil {
		log.Fatal("Failed to initialize leadership listener", zap.Error(err))
	}

	// The GoodGame chat server accepts guest joins for public channels: no
	// auth, no tokens, no OAuth (join frame is just {type:"join",channel_id}).
	streamPublisher := publisher.NewStreamPublisher(redisClient, log)
	channelRepo := channels.NewRepository(db, log)
	resolver := channels.NewResolver(log)
	dbWrapper := &dbConnWrapper{pool: db}

	statusPublisher := status.NewPublisher(redisClient, log)
	log.Info("Initialized platform status publisher")

	var channelMgr *channels.Manager

	messageHandler := func(channelID string, message *websocket.ChatMessageData) {
		if channelMgr == nil {
			log.Warn("Channel manager not initialized yet")
			return
		}
		handleChatMessage(channelID, message, streamPublisher, channelMgr, log)
	}

	wsClient := websocket.NewClient(messageHandler, log)

	wsClient.SetDeletionHandler(func(channelID string, event *websocket.RemoveMessageData) {
		handleDeletionEvent(channelID, event, streamPublisher, channelMgr, log)
	})

	channelMgr = channels.NewManager(channelRepo, resolver, wsClient, dbWrapper, ll.LeadershipCoordinator(), redisClient, podName, log)
	channelMgr.SetStatusPublisher(statusPublisher)

	if logLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	if tracingEnabled {
		router.Use(tracing.GinMiddleware("goodgame-listener"))
	}

	healthHandler := handlers.NewHealthHandler(wsClient, streamPublisher, channelMgr)
	router.GET("/health/live", healthHandler.LivenessProbe)
	router.GET("/health/ready", healthHandler.ReadinessProbe)
	router.GET("/status", healthHandler.Status)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	port := listener.Env("PORT", "8096")

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

	if err := wsClient.Connect(); err != nil {
		log.Fatal("Failed to connect to GoodGame WebSocket", zap.Error(err))
	}
	time.Sleep(2 * time.Second)

	if err := ll.Start(ctx, channelMgr); err != nil {
		log.Fatal("Failed to start listener", zap.Error(err))
	}

	filteredCount := channelMgr.GetFilteredAssignmentCount()
	shardMetrics.PodChannelCount.WithLabelValues(podName).Set(float64(filteredCount))
	log.Info("Recorded channel count metric",
		zap.String("pod_id", podName),
		zap.Int("channel_count", filteredCount),
	)

	go handleReconnections(wsClient, channelMgr, log)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down service...")
	streamPublisher.Stop()

	listener.ShutdownCoordinator(ll, channelMgr,
		func() { _ = wsClient.Disconnect() },
		srv,
		log,
	)

	log.Info("Service exited")
}

// handleChatMessage processes a GoodGame chat message and publishes to Redis.
func handleChatMessage(
	channelID string,
	message *websocket.ChatMessageData,
	pub *publisher.StreamPublisher,
	channelMgr *channels.Manager,
	log *zap.Logger,
) {
	start := time.Now()

	chatID, err := strconv.ParseInt(channelID, 10, 64)
	if err != nil {
		log.Warn("Unable to parse channel id from message",
			zap.String("channel_id", channelID),
		)
		metrics.IncDropped("invalid_channel_id")
		metrics.IncMessage("dropped", "invalid_channel_id")
		return
	}

	targets, found := channelMgr.GetOverlayTargetsForChannel(chatID)
	if !found || len(targets) == 0 {
		log.Warn("Received message for unknown channel",
			zap.String("channel_id", channelID),
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

	messageID := message.MessageID.String()
	if messageID == "" || messageID == "0" {
		messageID = fmt.Sprintf("goodgame-%s-%d", channelID, time.Now().UnixNano())
	}

	// GoodGame message timestamps are unix seconds (verified live).

	timestamp := start
	if ts, tsErr := message.Timestamp.Int64(); tsErr == nil && ts > 0 {
		timestamp = time.Unix(ts, 0).UTC()
	}

	tags := buildGoodgameTags(message, channelID)

	for _, target := range targets {
		publishCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		msg := publisher.RawMessage{
			MessageID:   messageID,
			Platform:    "goodgame",
			OverlayID:   target.OverlayID,
			ChannelID:   target.ChannelSlug,
			ChannelName: target.ChannelSlug,
			UserID:      message.UserID.String(),
			Username:    message.UserName,
			Text:        message.Text,
			Tags:        tags,
			RawMessage:  rawMsg,
			Timestamp:   timestamp,
		}

		if err := pub.Publish(publishCtx, &msg); err != nil {
			log.Error("Failed to publish message",
				zap.Error(err),
				zap.String("overlay_id", target.OverlayID),
				zap.String("channel_id", channelID),
			)
			metrics.IncDropped("publish_error")
			metrics.IncMessage("failed", "publish_error")
		} else {
			log.Debug("Published GoodGame message",
				zap.String("overlay_id", target.OverlayID),
				zap.String("channel_id", channelID),
				zap.String("sender", message.UserName),
				zap.Duration("publish_latency", time.Since(start)),
			)
			metrics.IncMessage("published", "success")
			metrics.ObservePublishLatency(time.Since(start))
		}
		cancel()
	}
}

func buildGoodgameTags(message *websocket.ChatMessageData, channelID string) map[string]string {
	tags := map[string]string{
		"channel_id": channelID,
	}

	if rights := message.UserRights.String(); rights != "" && rights != "0" {
		tags["user_rights"] = rights
	}
	if premium := message.Premium.String(); premium != "" && premium != "0" {
		tags["premium"] = premium
	}
	if message.Color != "" && message.Color != "none" && message.Color != "simple" {
		tags["color"] = message.Color
	}
	if message.Icon != "" && message.Icon != "none" {
		tags["icon"] = message.Icon
	}
	if mobile := message.Mobile.String(); mobile != "" && mobile != "0" {
		tags["mobile"] = mobile
	}
	if message.Role != "" {
		tags["role"] = message.Role
	}
	return tags
}

// handleDeletionEvent publishes a GoodGame message deletion to Redis Streams.
func handleDeletionEvent(
	channelID string,
	event *websocket.RemoveMessageData,
	pub *publisher.StreamPublisher,
	channelMgr *channels.Manager,
	log *zap.Logger,
) {
	chatID, err := strconv.ParseInt(channelID, 10, 64)
	if err != nil {
		log.Error("Failed to parse channel id from deletion", zap.String("channel_id", channelID), zap.Error(err))
		metrics.IncDropped("invalid_channel_id")
		return
	}

	targets, found := channelMgr.GetOverlayTargetsForChannel(chatID)
	if !found || len(targets) == 0 {
		log.Debug("Received deletion for unknown channel", zap.String("channel_id", channelID))
		metrics.IncDropped("unknown_channel")
		return
	}

	deletedMsgID := event.MessageID.String()

	for _, target := range targets {
		publishCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		tags := map[string]string{
			"event_type":    "message_deletion",
			"deletion_type": "single",
			"target_msg_id": deletedMsgID,
			"chatroom_id":   channelID,
		}

		rawMsg := publisher.RawMessage{
			Platform:    "goodgame",
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
				zap.String("message_id", deletedMsgID),
			)
			metrics.IncDropped("publish_error")
		} else {
			log.Debug("Published GoodGame deletion event to Redis Streams",
				zap.String("overlay_id", target.OverlayID),
				zap.String("message_id", deletedMsgID),
			)
			metrics.IncMessage("published", "deletion")
		}
		cancel()
	}
}

// handleReconnections handles WebSocket reconnection with exponential backoff.
func handleReconnections(
	wsClient *websocket.Client,
	channelMgr *channels.Manager,
	log *zap.Logger,
) {
	for range wsClient.ReconnectChan() {
		log.Warn("WebSocket disconnected, attempting to reconnect...")

		backoff := time.Second
		maxBackoff := 60 * time.Second

		for {
			log.Info("Reconnecting to GoodGame WebSocket", zap.Duration("backoff", backoff))
			time.Sleep(backoff)

			if err := wsClient.Connect(); err != nil {
				log.Error("Reconnection failed", zap.Error(err))
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			}

			log.Info("Reconnected to GoodGame WebSocket successfully")
			time.Sleep(2 * time.Second)
			// The channel manager's 30s sync loop re-joins the recorded
			// channels; the socket also replays resubscribeAll on welcome.
			log.Info("Re-syncing channels after reconnection")
			if err := channelMgr.SyncNow(); err != nil {
				log.Error("Channel re-sync after reconnect failed", zap.Error(err))
			}
			break
		}
	}
}

// dbConnWrapper wraps pgxpool.Pool to implement DBConnInterface.
type dbConnWrapper struct {
	pool *pgxpool.Pool
}

func (w *dbConnWrapper) GetPool() interface{} {
	return w.pool
}
