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
	"github.com/caesar/all-chat/shared/logger"
	"github.com/caesar/all-chat/shared/tracing"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// LeaderElectionKey is the Redis key for global leader election
	LeaderElectionKey = "leader:twitch-eventsub"

	// LeaderLockTTL is the TTL for the leader lock
	LeaderLockTTL = 10 * time.Second

	// LeaderRenewalInterval is how often to renew leadership
	LeaderRenewalInterval = 5 * time.Second

	// ChannelSyncInterval is how often to sync active channels from database
	ChannelSyncInterval = 30 * time.Second
)

// leaderState holds the shared leadership state with thread-safe access
type leaderState struct {
	sync.RWMutex
	isLeader bool
}

func main() {
	// Initialize logger
	logLevel := getEnv("LOG_LEVEL", "info")
	log := logger.NewLogger("twitch-eventsub-listener", logLevel)
	defer log.Sync()

	log.Info("Starting Twitch EventSub Listener Service",
		zap.String("version", getEnv("APP_VERSION", "dev")),
	)

	// Initialize tracing
	tracingEnabled := getEnv("OTEL_ENABLED", "false") == "true"
	if tracingEnabled {
		tracingCfg := tracing.Config{
			ServiceName:    "twitch-eventsub-listener",
			ServiceVersion: getEnv("APP_VERSION", "dev"),
			Environment:    getEnv("ENVIRONMENT", "development"),
			OTLPEndpoint:   getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
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
	instanceID := uuid.New().String()

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
	dbHost := getEnv("DATABASE_HOST", "localhost")
	dbPort := getEnv("DATABASE_PORT", "5432")
	dbUser := getEnv("DATABASE_USER", "allchat")
	dbPassword := getEnv("DATABASE_PASSWORD", "allchat_dev_password")
	dbName := getEnv("DATABASE_NAME", "allchat")

	connString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	db, err := database.NewPostgresPool(connString)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	log.Info("Connected to PostgreSQL")

	// Connect to Redis
	redisHost := getEnv("REDIS_HOST", "localhost")
	redisPort := getEnv("REDIS_PORT", "6379")
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

	// Initialize components
	streamPublisher := publisher.NewStreamPublisher(redisClient, log)
	subscriptionMgr := eventsub.NewSubscriptionManager(twitchClientID, twitchClientSecret, webhookSecret, callbackURL, log)
	channelManager := channels.NewManager(db, log, subscriptionMgr, tokenCipher)

	// Create webhook handler
	webhookHandler := webhooks.NewHandler(webhookSecret, redisClient, streamPublisher, log)

	// Leader election state (use struct with mutex for thread-safe access from HTTP handlers)
	state := &leaderState{}

	// Set up channel manager callback (creates/deletes subscriptions for all event types)
	channelManager.SetSubscriptionCallback(func(broadcasterID string, accessToken string, action string) error {
		state.RLock()
		isLdr := state.isLeader
		state.RUnlock()

		if !isLdr {
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
			if _, err := subscriptionMgr.SubscribeChannelPoints(ctx, broadcasterID, accessToken); err != nil {
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

			// Subscriptions - uses user access token
			if _, err := subscriptionMgr.SubscribeToSubscriptions(ctx, broadcasterID, accessToken); err != nil {
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

			// Gifts - uses user access token
			if _, err := subscriptionMgr.SubscribeToGifts(ctx, broadcasterID, accessToken); err != nil {
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

			// Resubscriptions - uses user access token
			if _, err := subscriptionMgr.SubscribeToResubscriptions(ctx, broadcasterID, accessToken); err != nil {
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
			if _, err := subscriptionMgr.SubscribeToRaids(ctx, broadcasterID, accessToken); err != nil {
				if strings.Contains(err.Error(), "subscription already exists") {
					successCount++
				} else {
					log.Warn("Failed to subscribe to raids", zap.String("broadcaster_id", broadcasterID), zap.Error(err))
					failCount++
				}
			} else {
				successCount++
			}

			// Cheers - uses user access token
			if _, err := subscriptionMgr.SubscribeToCheers(ctx, broadcasterID, accessToken); err != nil {
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

			// Follows - uses user access token with special condition
			if _, err := subscriptionMgr.SubscribeToFollows(ctx, broadcasterID, accessToken); err != nil {
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

	// Leader election loop
	leaderCtx, leaderCancel := context.WithCancel(ctx)
	defer leaderCancel()

	go func() {
		ticker := time.NewTicker(LeaderRenewalInterval)
		defer ticker.Stop()

		// Try to acquire leadership immediately on startup
		acquired, err := tryAcquireLeadership(ctx, redisClient, instanceID)
		if err != nil {
			log.Error("Initial leader election failed", zap.Error(err))
		} else {
			state.Lock()
			state.isLeader = acquired
			state.Unlock()

			if acquired {
				log.Info("Acquired leadership", zap.String("instance_id", instanceID))
				// Start channel manager (creates/deletes EventSub subscriptions)
				channelManager.Start(ctx, ChannelSyncInterval)
			}
		}

		for {
			select {
			case <-leaderCtx.Done():
				return
			case <-ticker.C:
				// Try to acquire/renew leadership
				state.RLock()
				wasLeader := state.isLeader
				state.RUnlock()

				acquired, err := tryAcquireLeadership(ctx, redisClient, instanceID)
				if err != nil {
					log.Error("Leader election failed", zap.Error(err))
					continue
				}

				state.Lock()
				state.isLeader = acquired
				state.Unlock()

				if !wasLeader && acquired {
					// Became leader - start managing subscriptions
					log.Info("Acquired leadership", zap.String("instance_id", instanceID))

					// Start channel manager (creates/deletes EventSub subscriptions)
					channelManager.Start(ctx, ChannelSyncInterval)

				} else if wasLeader && !acquired {
					// Lost leadership - stop managing subscriptions
					log.Warn("Lost leadership", zap.String("instance_id", instanceID))

					// Stop channel manager (stops creating/deleting subscriptions)
					// Note: Webhook HTTP server continues running on all instances
					channelManager.Stop()
				}
			}
		}
	}()

	// Start HTTP server for health checks and webhook endpoint
	startHTTPServer(log, getEnv("PORT", "8090"), state, webhookHandler, db, redisClient, tracingEnabled)

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down...")

	// Release leadership
	state.RLock()
	isLdr := state.isLeader
	state.RUnlock()

	if isLdr {
		releaseLeadership(context.Background(), redisClient, instanceID)
		channelManager.Stop()
	}

	log.Info("Shutdown complete")
}

// tryAcquireLeadership attempts to acquire or renew leadership
func tryAcquireLeadership(ctx context.Context, client *redis.Client, instanceID string) (bool, error) {
	// Check current leader
	currentLeader, err := client.Get(ctx, LeaderElectionKey).Result()
	if err == redis.Nil {
		// No leader, try to acquire
		success, err := client.SetNX(ctx, LeaderElectionKey, instanceID, LeaderLockTTL).Result()
		return success, err
	}
	if err != nil {
		return false, err
	}

	if currentLeader == instanceID {
		// We are leader, renew lock
		if err := client.Expire(ctx, LeaderElectionKey, LeaderLockTTL).Err(); err != nil {
			return false, err
		}
		return true, nil
	}

	// Someone else is leader
	return false, nil
}

// releaseLeadership releases leadership if held
func releaseLeadership(ctx context.Context, client *redis.Client, instanceID string) {
	currentLeader, err := client.Get(ctx, LeaderElectionKey).Result()
	if err == nil && currentLeader == instanceID {
		client.Del(ctx, LeaderElectionKey)
	}
}

// startHTTPServer starts the HTTP server for health checks and webhook endpoint
func startHTTPServer(log *zap.Logger, port string, state *leaderState, webhookHandler *webhooks.Handler, db *pgxpool.Pool, redis *redis.Client, tracingEnabled bool) {
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

		state.RLock()
		isLdr := state.isLeader
		state.RUnlock()

		c.JSON(http.StatusOK, gin.H{
			"status":    "ready",
			"is_leader": isLdr,
		})
	})

	// Status endpoint
	router.GET("/status", func(c *gin.Context) {
		state.RLock()
		isLdr := state.isLeader
		state.RUnlock()

		c.JSON(http.StatusOK, gin.H{
			"is_leader": isLdr,
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

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return defaultValue
}
