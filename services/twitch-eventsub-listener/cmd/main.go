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

	"github.com/caesar/all-chat/services/twitch-eventsub-listener/channels"
	"github.com/caesar/all-chat/services/twitch-eventsub-listener/eventsub"
	"github.com/caesar/all-chat/services/twitch-eventsub-listener/publisher"
	"github.com/caesar/all-chat/services/twitch-eventsub-listener/webhooks"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/encryption"
	"github.com/caesar/all-chat/shared/listener"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/caesar/all-chat/shared/tracing"
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

	// Connect to Redis
	redisHost := listener.Env("REDIS_HOST", "localhost")
	redisPort := listener.Env("REDIS_PORT", "6379")
	redisClient := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort),
	})
	defer redisClient.Close()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	log.Info("Connected to Redis")

	// Load encryption key for token decryption (must match auth-service encryption)
	encryptionKey := os.Getenv("ENCRYPTION_KEY")
	if encryptionKey == "" {
		log.Fatal("ENCRYPTION_KEY is required for decrypting user OAuth tokens")
	}

	parsedKey, err := encryption.ParseKey(encryptionKey)
	if err != nil {
		log.Fatal("Failed to parse ENCRYPTION_KEY", zap.Error(err))
	}

	tokenCipher, err := encryption.NewAESEncryptor(parsedKey)
	if err != nil {
		log.Fatal("Failed to initialize token cipher", zap.Error(err))
	}

	// Initialize LeadershipListener — per D-12/D-13 EventSub gets demand gating
	ll, err := listener.NewLeadershipListenerFromEnv("twitch-eventsub", redisClient, log)
	if err != nil {
		log.Fatal("Failed to initialize leadership listener", zap.Error(err))
	}

	// Initialize metrics (available via /metrics endpoint)
	listenerMetrics := metrics.NewListenerMetrics("twitch-eventsub", "twitch-eventsub-listener")
	log.Info("Initialized Prometheus metrics")

	// Initialize components
	streamPublisher := publisher.NewStreamPublisher(redisClient, log)
	subscriptionMgr := eventsub.NewSubscriptionManager(twitchClientID, twitchClientSecret, webhookSecret, callbackURL, log)
	channelManager := channels.NewManager(db, log, subscriptionMgr, tokenCipher, ChannelSyncInterval)

	// Start LeadershipListener — runs demand subscriber; channel manager started after leadership
	if err := ll.Start(ctx, channelManager); err != nil {
		log.Fatal("Failed to start leadership listener", zap.Error(err))
	}

	// Create webhook handler
	webhookHandler := webhooks.NewHandler(webhookSecret, redisClient, db, streamPublisher, listenerMetrics, log)

	// isLeader tracks whether this pod currently holds EventSub leadership.
	// Protected by mu; read by subscription callback and HTTP handlers.
	var isLeaderMu sync.RWMutex
	isLeader := false
	isLeaderFn := func() bool {
		isLeaderMu.RLock()
		defer isLeaderMu.RUnlock()
		return isLeader
	}

	// Set up channel manager callback (creates/deletes subscriptions for all event types)
	channelManager.SetSubscriptionCallback(func(broadcasterID string, accessToken string, action string) error {
		if !isLeaderFn() {
			// Only leader creates/deletes subscriptions
			return nil
		}

		if action == "subscribe" {
			// Subscribe to all supported EventSub event types
			var successCount, failCount, scopeErrorCount int

			// Helper function to check if error is due to missing OAuth scopes
			isScopeError := func(err error) bool {
				return strings.Contains(err.Error(), "missing proper authorization") ||
					strings.Contains(err.Error(), "403")
			}

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

		} else if action == "unsubscribe" {
			return subscriptionMgr.Unsubscribe(ctx, broadcasterID)
		}

		return nil
	})

	// Leadership acquisition goroutine — replaces old Redis SETNX loop
	go func() {
		lc := ll.LeadershipCoordinator()
		if lc == nil {
			log.Info("Leadership coordination disabled — acting as standalone leader")
			isLeaderMu.Lock()
			isLeader = true
			isLeaderMu.Unlock()
			if err := channelManager.Start(ctx); err != nil {
				log.Error("Channel manager start failed", zap.Error(err))
			}
			return
		}

		acquired, err := lc.EnsureLeadership(ctx, "shard:0", func() {
			log.Warn("Lost EventSub leadership — stopping subscription management")
			isLeaderMu.Lock()
			isLeader = false
			isLeaderMu.Unlock()
			channelManager.Stop()
		})
		if err != nil {
			log.Error("Leadership acquisition failed", zap.Error(err))
			return
		}
		if acquired {
			log.Info("Acquired EventSub leadership")
			isLeaderMu.Lock()
			isLeader = true
			isLeaderMu.Unlock()
			if err := channelManager.Start(ctx); err != nil {
				log.Error("Channel manager start failed", zap.Error(err))
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

	channelManager.Stop()
	ll.Stop()

	log.Info("Shutdown complete")
}

// startHTTPServer starts the HTTP server for health checks and webhook endpoint.
// isLeaderFn is a nil-safe function returning the current leadership state.
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
