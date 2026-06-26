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
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/message-processor/registry"
	"github.com/caesar/all-chat/services/twitch-eventsub-listener/channels"
	"github.com/caesar/all-chat/services/twitch-eventsub-listener/claimexport"
	"github.com/caesar/all-chat/services/twitch-eventsub-listener/eventsub"
	"github.com/caesar/all-chat/services/twitch-eventsub-listener/publisher"
	"github.com/caesar/all-chat/services/twitch-eventsub-listener/scopeexport"
	"github.com/caesar/all-chat/services/twitch-eventsub-listener/status"
	"github.com/caesar/all-chat/services/twitch-eventsub-listener/webhooks"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/encryption"
	"github.com/caesar/all-chat/shared/listener"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/caesar/all-chat/shared/tracing"
	"github.com/caesar/all-chat/shared/twitchchat"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// ChannelSyncInterval is how often to sync active channels from database
	ChannelSyncInterval = 30 * time.Second
)

func main() {
	// Initialize logger
	logLevel := listener.Env("LOG_LEVEL", "info")
	log := logger.NewLogger("twitch-eventsub-listener", logLevel)
	defer log.Sync()

	log.Info("Starting Twitch EventSub Listener Service",
		zap.String("version", listener.Env("APP_VERSION", "dev")),
	)

	// Initialize tracing
	tracingEnabled := listener.Env("OTEL_ENABLED", "false") == "true"
	if tracingEnabled {
		tracingCfg := tracing.Config{
			ServiceName:    "twitch-eventsub-listener",
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
			log.Info("Tracer initialized", zap.String("service", "twitch-eventsub-listener"))
		}
	}

	ctx := context.Background()

	// Get configuration from environment
	twitchClientID := strings.TrimSpace(os.Getenv("TWITCH_CLIENT_ID"))
	twitchClientSecret := strings.TrimSpace(os.Getenv("TWITCH_CLIENT_SECRET"))
	webhookSecret := strings.TrimSpace(os.Getenv("EVENTSUB_WEBHOOK_SECRET"))
	callbackURL := strings.TrimSpace(os.Getenv("EVENTSUB_CALLBACK_URL"))

	if twitchClientID == "" || twitchClientSecret == "" {
		log.Fatal("TWITCH_CLIENT_ID and TWITCH_CLIENT_SECRET are required")
	}
	if webhookSecret == "" {
		log.Fatal("EVENTSUB_WEBHOOK_SECRET is required for webhook signature verification")
	}
	if callbackURL == "" {
		log.Fatal("EVENTSUB_CALLBACK_URL is required (e.g., https://allch.at/webhooks/eventsub)")
	}

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

	// Connect to Redis
	redisHost := listener.Env("REDIS_HOST", "localhost")
	redisPort := listener.Env("REDIS_PORT", "6379")
	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", redisHost, redisPort),
		Password: listener.Env("REDIS_PASSWORD", ""),
	})
	defer redisClient.Close()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	log.Info("Connected to Redis")

	// Initialize multi-key encryptor for decrypting user OAuth tokens (must match auth-service).
	// Reads TOKEN_ENCRYPTION_KEY_V1 (required) and TOKEN_ENCRYPTION_KEY (legacy fallback).
	// NOTE: Deployment manifest currently mounts ENCRYPTION_KEY; Plan 14-07 renames it to
	// TOKEN_ENCRYPTION_KEY. This service will fail to start until Plan 14-07 ships.
	tokenCipher, err := encryption.NewMultiKeyEncryptorFromEnvWithLogger(log)
	if err != nil {
		log.Fatal("Failed to initialize token cipher (TOKEN_ENCRYPTION_KEY_V1 must be set)", zap.Error(err))
	}
	log.Info("Token cipher initialized", zap.Uint8("current_kid", tokenCipher.CurrentKid()))

	// Initialize LeadershipListener — per D-12/D-13 EventSub gets demand gating
	ll, err := listener.NewLeadershipListenerFromEnv("twitch-eventsub", redisClient, log)
	if err != nil {
		log.Fatal("Failed to initialize leadership listener", zap.Error(err))
	}
	// We coordinate leadership as "twitch-eventsub" but read "twitch" sources, so match
	// demand updates against the source platform "twitch" (chat subscriptions are gated on
	// live-overlay demand; see channels.Manager.reconcileChatLocked).
	ll.SetDemandPlatform("twitch")

	// Initialize metrics (available via /metrics endpoint)
	listenerMetrics := metrics.NewListenerMetrics("twitch-eventsub", "twitch-eventsub-listener")
	log.Info("Initialized Prometheus metrics")

	// Initialize components
	streamPublisher := publisher.NewStreamPublisher(redisClient, log)
	statusPublisher := status.NewPublisher(redisClient, log)

	// Chat-ownership claim store (ADR-0015): the webhook handler writes a per-channel claim on
	// delivered chat so the IRC listener excludes only channels EventSub is actually serving. TTL
	// bounds the IRC fallback delay in a total-outage case; overridable via EVENTSUB_CHAT_CLAIM_TTL.
	claimTTL := twitchchat.DefaultClaimTTL
	if v := listener.Env("EVENTSUB_CHAT_CLAIM_TTL", ""); v != "" {
		if parsed, perr := time.ParseDuration(v); perr == nil && parsed > 0 {
			claimTTL = parsed
		} else {
			log.Warn("Invalid EVENTSUB_CHAT_CLAIM_TTL, using default",
				zap.String("value", v), zap.Duration("default", claimTTL))
		}
	}
	chatClaims := twitchchat.NewClaimStoreWithTTL(redisClient, claimTTL)
	log.Info("Chat-ownership claim store initialized", zap.Duration("claim_ttl", claimTTL))

	// Mirror the live chat-ownership claim set into a per-login gauge so the IRC→EventSub
	// migration dashboard can subtract already-migrated channels from "still on IRC" panels
	// (ADR-0015). Runs on every replica; duplicate series dedupe in PromQL.
	go claimexport.ExportOwnedChannels(ctx, chatClaims, "twitch-eventsub-listener", log)

	// Report the IRC→EventSub migration backlog (active Twitch channels lacking chat scope) as a
	// gauge derived from granted OAuth scope. Unlike the dashboard's message-activity panels — which
	// are skewed by demand-gated IRC fallback traffic and never drain to zero — this is the
	// authoritative "still needs to migrate" signal and the correct gate for the IRC sunset
	// (ADR-0026). Runs on every replica; duplicate series dedupe under `max by (migration_state)`.
	go scopeexport.Export(ctx, db, "twitch-eventsub-listener", log)

	// Message-ID registry (native Twitch id → internal UUID), shared format with twitch-listener
	// and read by message-processor to resolve single-message deletions. Same 1h TTL as IRC so the
	// two writers behave identically. The webhook handler populates it on each delivered chat
	// message; chat-scoped channels are EventSub-only (ADR-0015), so EventSub is their sole writer.
	msgRegistry := registry.NewRedisRegistry(redisClient, 1*time.Hour)

	subscriptionMgr := eventsub.NewSubscriptionManager(twitchClientID, twitchClientSecret, webhookSecret, callbackURL, log)
	channelManager := channels.NewManager(db, log, subscriptionMgr, tokenCipher, ChannelSyncInterval)
	channelManager.SetStatusPublisher(statusPublisher)
	channelManager.SetClaimStore(chatClaims)

	// isLeader tracks whether this pod currently holds EventSub leadership.
	// Protected by mu; read by subscription callback and HTTP handlers.
	//
	// Defined BEFORE ll.Start because that call synchronously starts the channel manager's sync
	// loop (and its initial sync), which refreshes chat-ownership claims. Wiring the leader gate
	// first ensures a standby never writes claims (a nil gate would be treated as "always leader").
	var isLeaderMu sync.RWMutex
	isLeader := false
	isLeaderFn := func() bool {
		isLeaderMu.RLock()
		defer isLeaderMu.RUnlock()
		return isLeader
	}
	channelManager.SetLeaderFunc(isLeaderFn)

	// Start LeadershipListener — runs demand subscriber; channel manager started after leadership
	if err := ll.Start(ctx, channelManager); err != nil {
		log.Fatal("Failed to start leadership listener", zap.Error(err))
	}

	// Create webhook handler
	webhookHandler := webhooks.NewHandler(webhookSecret, redisClient, db, streamPublisher, listenerMetrics, statusPublisher, chatClaims, msgRegistry, log)

	// Set up channel manager callback. Actions: "subscribe" creates the event subscriptions
	// for every active channel; "subscribe_chat"/"unsubscribe_chat" manage the
	// channel.chat.message subscription, which the manager gates on chat scope AND live-overlay
	// demand; "unsubscribe" deletes all subscriptions for a removed channel.
	channelManager.SetSubscriptionCallback(func(broadcasterID string, accessToken string, action string) error {
		if !isLeaderFn() {
			// Only leader creates/deletes subscriptions
			return nil
		}

		if action == "subscribe" {
			// Subscribe to all supported EventSub event types
			var successCount, failCount, scopeErrorCount int

			// Channel points - uses app access token
			if _, err := subscriptionMgr.SubscribeChannelPoints(ctx, broadcasterID); err != nil {
				if strings.Contains(err.Error(), "subscription already exists") {
					log.Info("Channel points subscription already exists", zap.String("broadcaster_id", broadcasterID))
					successCount++
				} else {
					log.Warn("Failed to subscribe to channel points", zap.String("broadcaster_id", broadcasterID), zap.Error(err))
					failCount++
				}
			} else {
				log.Info("Subscribed to channel points", zap.String("broadcaster_id", broadcasterID))
				successCount++
			}

			// Subscriptions - uses app access token
			if _, err := subscriptionMgr.SubscribeToSubscriptions(ctx, broadcasterID); err != nil {
				if strings.Contains(err.Error(), "subscription already exists") {
					successCount++
				} else if isScopeError(err) {
					log.Info("Subscription event requires re-authentication with channel:read:subscriptions scope",
						zap.String("broadcaster_id", broadcasterID))
					scopeErrorCount++
				} else {
					log.Warn("Failed to subscribe to subscriptions", zap.String("broadcaster_id", broadcasterID), zap.Error(err))
					failCount++
				}
			} else {
				successCount++
			}

			// Gifts - uses app access token
			if _, err := subscriptionMgr.SubscribeToGifts(ctx, broadcasterID); err != nil {
				if strings.Contains(err.Error(), "subscription already exists") {
					successCount++
				} else if isScopeError(err) {
					log.Info("Gift event requires re-authentication with channel:read:subscriptions scope",
						zap.String("broadcaster_id", broadcasterID))
					scopeErrorCount++
				} else {
					log.Warn("Failed to subscribe to gifts", zap.String("broadcaster_id", broadcasterID), zap.Error(err))
					failCount++
				}
			} else {
				successCount++
			}

			// Resubscriptions - uses app access token
			if _, err := subscriptionMgr.SubscribeToResubscriptions(ctx, broadcasterID); err != nil {
				if strings.Contains(err.Error(), "subscription already exists") {
					successCount++
				} else if isScopeError(err) {
					log.Info("Resub event requires re-authentication with channel:read:subscriptions scope",
						zap.String("broadcaster_id", broadcasterID))
					scopeErrorCount++
				} else {
					log.Warn("Failed to subscribe to resubs", zap.String("broadcaster_id", broadcasterID), zap.Error(err))
					failCount++
				}
			} else {
				successCount++
			}

			// Raids - uses app access token with special condition
			if _, err := subscriptionMgr.SubscribeToRaids(ctx, broadcasterID); err != nil {
				if strings.Contains(err.Error(), "subscription already exists") {
					successCount++
				} else {
					log.Warn("Failed to subscribe to raids", zap.String("broadcaster_id", broadcasterID), zap.Error(err))
					failCount++
				}
			} else {
				successCount++
			}

			// Cheers - uses app access token
			if _, err := subscriptionMgr.SubscribeToCheers(ctx, broadcasterID); err != nil {
				if strings.Contains(err.Error(), "subscription already exists") {
					successCount++
				} else if isScopeError(err) {
					log.Info("Cheer event requires re-authentication with bits:read scope",
						zap.String("broadcaster_id", broadcasterID))
					scopeErrorCount++
				} else {
					log.Warn("Failed to subscribe to cheers", zap.String("broadcaster_id", broadcasterID), zap.Error(err))
					failCount++
				}
			} else {
				successCount++
			}

			// Follows - uses app access token with special condition
			if _, err := subscriptionMgr.SubscribeToFollows(ctx, broadcasterID); err != nil {
				if strings.Contains(err.Error(), "subscription already exists") {
					successCount++
				} else if isScopeError(err) {
					log.Info("Follow event requires re-authentication with moderator:read:followers scope",
						zap.String("broadcaster_id", broadcasterID))
					scopeErrorCount++
				} else {
					log.Warn("Failed to subscribe to follows", zap.String("broadcaster_id", broadcasterID), zap.Error(err))
					failCount++
				}
			} else {
				successCount++
			}

			// Stream offline — uses app access token (no user scope needed)
			if _, err := subscriptionMgr.SubscribeToStreamOffline(ctx, broadcasterID); err != nil {
				if strings.Contains(err.Error(), "subscription already exists") {
					successCount++
				} else {
					log.Warn("Failed to subscribe to stream.offline",
						zap.String("broadcaster_id", broadcasterID),
						zap.Error(err))
					failCount++
				}
			} else {
				successCount++
			}

			// Chat (channel.chat.message) is NOT created here — it is managed separately via
			// the subscribe_chat/unsubscribe_chat actions, gated on chat scope AND live-overlay
			// demand. Keeping it out of "subscribe" ensures the event subscriptions above are
			// created for every active channel regardless of chat scope or demand.

			log.Info("EventSub subscription sync complete",
				zap.String("broadcaster_id", broadcasterID),
				zap.Int("success", successCount),
				zap.Int("failed", failCount),
				zap.Int("scope_errors", scopeErrorCount),
			)

			// Don't return error if at least one subscription succeeded
			// Scope errors don't count as failures (user just needs to re-auth)
			if successCount > 0 {
				return nil
			}
			return fmt.Errorf("all subscriptions failed for broadcaster %s", broadcasterID)

		} else if action == "subscribe_chat" {
			// Create the chat subscription. A scope error is non-fatal: return nil so the
			// manager marks chat active and doesn't retry-spam; that channel just won't get
			// EventSub chat (it stays on IRC unless/until the owner grants the missing scope).
			if _, err := subscriptionMgr.SubscribeToChatMessages(ctx, broadcasterID); err != nil {
				if strings.Contains(err.Error(), "subscription already exists") {
					// Already subscribed — fall through to ensure deletion subs also exist.
				} else if isScopeError(err) {
					log.Info("Chat message subscription requires user:read:chat + user:bot scopes",
						zap.String("broadcaster_id", broadcasterID))
					// No chat sub → the deletion subs (same scope) would fail too. The channel
					// stays on IRC, which still handles its deletions.
					return nil
				} else {
					return err
				}
			} else {
				log.Info("Subscribed to chat messages", zap.String("broadcaster_id", broadcasterID))
			}

			// Chat-moderation subscriptions: deletions of a single message, of a user's messages
			// (timeout/ban), and full chat clears. They share user:read:chat and the chat
			// condition, so they live with the chat subscription. Best-effort — a failure here
			// doesn't undo the chat subscription (the channel still reads chat, just may miss a
			// deletion type until the next sync re-attempts).
			for _, sub := range []struct {
				name string
				fn   func(context.Context, string) (string, error)
			}{
				{"channel.chat.message_delete", subscriptionMgr.SubscribeToChatMessageDelete},
				{"channel.chat.clear_user_messages", subscriptionMgr.SubscribeToChatClearUserMessages},
				{"channel.chat.clear", subscriptionMgr.SubscribeToChatClear},
			} {
				if _, err := sub.fn(ctx, broadcasterID); err != nil {
					if strings.Contains(err.Error(), "subscription already exists") || isScopeError(err) {
						continue
					}
					log.Warn("Failed to subscribe to chat moderation event",
						zap.String("broadcaster_id", broadcasterID),
						zap.String("type", sub.name),
						zap.Error(err))
				}
			}
			return nil

		} else if action == "unsubscribe_chat" {
			// Tear down the chat subscription and its moderation subscriptions together — they
			// share the same lifecycle (chat scope + live-overlay demand). Returns the first error
			// but attempts every deletion.
			var firstErr error
			for _, subType := range []string{
				"channel.chat.message",
				"channel.chat.message_delete",
				"channel.chat.clear_user_messages",
				"channel.chat.clear",
			} {
				if err := subscriptionMgr.UnsubscribeType(ctx, broadcasterID, subType); err != nil && firstErr == nil {
					firstErr = err
				}
			}
			return firstErr

		} else if action == "unsubscribe" {
			return subscriptionMgr.Unsubscribe(ctx, broadcasterID)
		}

		return nil
	})

	setLeader := func(v bool) {
		isLeaderMu.Lock()
		isLeader = v
		isLeaderMu.Unlock()
	}

	// Leadership acquisition goroutine. The channel manager itself is already running on every
	// pod (LeadershipListener.Start started it); leadership only gates whether this pod's
	// subscription callback does real work. We therefore RETRY acquisition forever and
	// re-acquire after loss — a one-shot attempt (the previous behaviour) left no pod as leader
	// after a rolling deploy, because the new pods raced the outgoing leader's still-held lease,
	// got a "claim skipped", and never tried again. See ADR-0007: EnsureLeadership is meant to
	// be called repeatedly, not once.
	go func() {
		lc := ll.LeadershipCoordinator()
		if lc == nil {
			log.Info("Leadership coordination disabled — acting as standalone leader")
			channelManager.ResetTracking()
			setLeader(true)
			channelManager.TriggerSync()
			return
		}

		const reacquireInterval = 10 * time.Second
		lostCh := make(chan struct{}, 1)
		onLost := func() {
			log.Warn("Lost EventSub leadership — will re-acquire")
			setLeader(false)
			select {
			case lostCh <- struct{}{}:
			default:
			}
		}

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			acquired, err := lc.EnsureLeadership(ctx, "shard:0", onLost)
			if err != nil {
				log.Error("Leadership acquisition failed — retrying", zap.Error(err))
			} else if acquired {
				log.Info("Acquired EventSub leadership")
				// Rebuild from a clean slate: as a standby this pod may have tracked channels
				// with a no-op callback, so clear that state before (re)creating subscriptions.
				channelManager.ResetTracking()
				setLeader(true)
				channelManager.TriggerSync()

				// Hold leadership until it is lost or the service shuts down.
				select {
				case <-ctx.Done():
					return
				case <-lostCh:
					continue // re-acquire immediately
				}
			}

			// Not acquired (another pod holds it) or transient error — wait and retry so this
			// standby takes over promptly once the lease frees.
			select {
			case <-ctx.Done():
				return
			case <-time.After(reacquireInterval):
			}
		}
	}()

	// Start HTTP server for health checks and webhook endpoint
	startHTTPServer(log, listener.Env("PORT", "8090"), isLeaderFn, webhookHandler, db, redisClient, tracingEnabled)

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down...")

	// Stop ring buffer publisher — drains retry goroutine before closing Redis.
	streamPublisher.Stop()

	channelManager.Stop()
	ll.Stop()

	log.Info("Shutdown complete")
}

// startHTTPServer starts the HTTP server for health checks and webhook endpoint.
// isLeaderFn is a nil-safe function returning the current leadership state.
// isScopeError reports whether a subscription error is due to the broadcaster not having
// granted the required OAuth scopes (Twitch returns 403 / "missing proper authorization").
func isScopeError(err error) bool {
	return strings.Contains(err.Error(), "missing proper authorization") ||
		strings.Contains(err.Error(), "403")
}

func startHTTPServer(log *zap.Logger, port string, isLeaderFn func() bool, webhookHandler *webhooks.Handler, db *pgxpool.Pool, redis *redis.Client, tracingEnabled bool) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	if tracingEnabled {
		router.Use(tracing.GinMiddleware("twitch-eventsub-listener"))
	}

	// EventSub webhook endpoint
	router.POST("/webhooks/eventsub", webhookHandler.HandleEventSubWebhook)

	// Liveness probe
	router.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Readiness probe
	router.GET("/health/ready", func(c *gin.Context) {
		ctx := c.Request.Context()

		// Check database connection
		if err := db.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "error": "database unavailable"})
			return
		}

		// Check Redis connection
		if err := redis.Ping(ctx).Err(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "error": "redis unavailable"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":    "ready",
			"is_leader": isLeaderFn(),
		})
	})

	// Status endpoint
	router.GET("/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"is_leader": isLeaderFn(),
			"transport": "webhook",
		})
	})

	// Prometheus metrics
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	go func() {
		if err := router.Run(":" + port); err != nil {
			log.Fatal("Failed to start HTTP server", zap.Error(err))
		}
	}()

	log.Info("HTTP server started", zap.String("port", port))
}
