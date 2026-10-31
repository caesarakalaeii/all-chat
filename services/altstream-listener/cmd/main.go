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
	"strings"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/altstream-listener/altstream"
	"github.com/caesar/all-chat/services/altstream-listener/channels"
	"github.com/caesar/all-chat/services/altstream-listener/handlers"
	"github.com/caesar/all-chat/services/altstream-listener/metrics"
	"github.com/caesar/all-chat/services/altstream-listener/publisher"
	"github.com/caesar/all-chat/services/altstream-listener/status"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/listener"
	"github.com/caesar/all-chat/shared/logger"
	sharedmetrics "github.com/caesar/all-chat/shared/metrics"
	sharedredis "github.com/caesar/all-chat/shared/redis"
	"github.com/caesar/all-chat/shared/tracing"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

const platform = "altstream"

func main() {
	logLevel := listener.Env("LOG_LEVEL", "info")
	log := logger.NewLogger("altstream-listener", logLevel)
	defer log.Sync()

	log.Info("Starting Altstream Listener",
		zap.String("version", listener.Env("APP_VERSION", "dev")),
	)

	tracingEnabled := listener.Env("OTEL_ENABLED", "false") == "true"
	if tracingEnabled {
		tracingCfg := tracing.Config{
			ServiceName:    "altstream-listener",
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
			log.Info("Tracer initialized", zap.String("service", "altstream-listener"))
		}
	}

	ctx := context.Background()

	dbPassword := listener.Env("DATABASE_PASSWORD", "")
	if dbPassword == "" {
		log.Fatal("DATABASE_PASSWORD must be set")
	}
	connString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		listener.Env("DATABASE_USER", "allchat"), dbPassword,
		listener.Env("DATABASE_HOST", "localhost"), listener.Env("DATABASE_PORT", "5432"),
		listener.Env("DATABASE_NAME", "allchat"))

	db, err := database.NewPostgresPool(connString)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()
	log.Info("Connected to PostgreSQL")

	redisAddr := sharedredis.BuildDSN(listener.Env("REDIS_HOST", "localhost"), listener.Env("REDIS_PORT", "6379"))
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
	podName := listener.Env("HOSTNAME", "altstream-listener-0")

	ll, err := listener.NewLeadershipListenerFromEnv(platform, redisClient, log)
	if err != nil {
		log.Fatal("Failed to initialize leadership listener", zap.Error(err))
	}

	streamPublisher := publisher.NewStreamPublisher(redisClient, log)

	var channelMgr *channels.Manager
	baseURL := listener.Env("ALTSTREAM_BASE_URL", altstream.DefaultBaseURL)
	client := altstream.NewClient(baseURL, func(channel string, ev *altstream.Event) {
		if channelMgr == nil {
			return
		}
		handleEvent(channel, ev, channelMgr.OverlayIDsForChannel(channel), streamPublisher, log)
	}, log)

	channelMgr = channels.NewManager(channels.NewRepository(db, log), client, ll.LeadershipCoordinator(),
		status.NewPublisher(redisClient, log), log)

	if logLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	if tracingEnabled {
		router.Use(tracing.GinMiddleware("altstream-listener"))
	}

	healthHandler := handlers.NewHealthHandler(streamPublisher, channelMgr)
	router.GET("/health/live", healthHandler.LivenessProbe)
	router.GET("/health/ready", healthHandler.ReadinessProbe)
	router.GET("/status", healthHandler.Status)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	port := listener.Env("PORT", "8101")
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// The health server starts before ll.Start, whose initial sync claims
	// leadership per channel and may take a while.
	go func() {
		log.Info("HTTP server listening", zap.String("port", port), zap.String("altstream", baseURL))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start HTTP server", zap.Error(err))
		}
	}()

	if err := ll.Start(ctx, channelMgr); err != nil {
		log.Fatal("Failed to start listener", zap.Error(err))
	}
	shardMetrics.PodChannelCount.WithLabelValues(podName).Set(float64(channelMgr.GetActiveChannelCount()))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down service...")
	listener.ShutdownCoordinator(ll, channelMgr, client.DisconnectAll, srv, log)
	streamPublisher.Stop()
	log.Info("Service exited")
}

// handleEvent publishes one Altstream event once per consuming overlay.
func handleEvent(channel string, ev *altstream.Event, overlayIDs []string, pub *publisher.StreamPublisher, log *zap.Logger) {
	start := time.Now()
	msg, ok := toRawMessage(channel, ev)
	if !ok {
		return
	}
	if len(overlayIDs) == 0 {
		metrics.IncDropped("unknown_channel")
		return
	}

	for _, overlayID := range overlayIDs {
		out := *msg
		out.OverlayID = overlayID
		publishCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := pub.Publish(publishCtx, &out)
		cancel()
		if err != nil {
			log.Error("Failed to publish Altstream event",
				zap.Error(err), zap.String("overlay_id", overlayID), zap.String("channel", channel))
			metrics.IncDropped("publish_error")
			metrics.IncMessage("failed", "publish_error")
			continue
		}
		metrics.IncMessage("published", ev.Type)
		metrics.ObservePublishLatency(time.Since(start))
	}
}

// toRawMessage maps an event to its chat:raw payload (without overlay_id).
// message and highlight become chat messages; delete and clear become
// message_deletion events; every other type is out of scope and dropped.
func toRawMessage(channel string, ev *altstream.Event) (*publisher.RawMessage, bool) {
	msg := &publisher.RawMessage{
		MessageID:   ev.ID,
		Platform:    platform,
		ChannelID:   channel,
		ChannelName: channel,
		Timestamp:   time.Now().UTC(),
	}
	if ev.SentAt > 0 {
		msg.Timestamp = time.UnixMilli(ev.SentAt).UTC()
	}

	switch ev.Type {
	case altstream.TypeMessage, altstream.TypeHighlight:
		if ev.ChatText() == "" && ev.StickerURL == "" {
			metrics.IncDropped("empty")
			return nil, false
		}
		// authorUsername is the stable identity (there is no numeric user id);
		// REST-shaped events lack it, so fall back to the display name.
		username := ev.AuthorUsername
		if username == "" {
			username = ev.Author
		}
		msg.UserID = strings.ToLower(username)
		msg.Username = username
		msg.Text = ev.ChatText()
		msg.Tags = map[string]string{"message_type": ev.Type}
		msg.RawMessage = withEmotes(ev.Raw, ev.Emotes)

	case altstream.TypeDelete:
		if ev.TargetID == "" {
			metrics.IncDropped("delete_without_target")
			return nil, false
		}
		// discord-listener's deletion shape. The processor maps targetId to
		// the rendered message through its msgid registry.
		msg.EventType = "message_deletion"
		msg.EventData = map[string]any{"deletion_type": "single", "target_msg_id": ev.TargetID}

	case altstream.TypeClear:
		msg.EventType = "message_deletion"
		msg.EventData = map[string]any{"deletion_type": "clear"}

	default:
		metrics.IncIgnored(ev.Type)
		return nil, false
	}
	return msg, true
}

// withEmotes returns the event JSON with the listener-added "emotes" key: the
// :name: tokens of the text this pod could resolve.
func withEmotes(raw json.RawMessage, emotes []altstream.Emote) json.RawMessage {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		obj = map[string]json.RawMessage{}
	}
	if emotes == nil {
		emotes = []altstream.Emote{}
	}
	obj["emotes"], _ = json.Marshal(emotes)
	out, _ := json.Marshal(obj)
	return out
}
