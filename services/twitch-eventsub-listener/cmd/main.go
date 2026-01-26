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
	"github.com/caesar/all-chat/services/twitch-eventsub-listener/models"
	"github.com/caesar/all-chat/services/twitch-eventsub-listener/publisher"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
	isLeader          bool
	eventSubSessionID string
}

func main() {
	// Initialize logger
	logLevel := getEnv("LOG_LEVEL", "info")
	log := logger.NewLogger("twitch-eventsub-listener", logLevel)
	defer log.Sync()

	log.Info("Starting Twitch EventSub Listener Service",
		zap.String("version", getEnv("APP_VERSION", "dev")),
	)

	ctx := context.Background()
	instanceID := uuid.New().String()

	// Get configuration from environment
	twitchClientID := strings.TrimSpace(os.Getenv("TWITCH_CLIENT_ID"))
	twitchClientSecret := strings.TrimSpace(os.Getenv("TWITCH_CLIENT_SECRET"))
	if twitchClientID == "" || twitchClientSecret == "" {
		log.Fatal("TWITCH_CLIENT_ID and TWITCH_CLIENT_SECRET are required")
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

	// Initialize components
	streamPublisher := publisher.NewStreamPublisher(redisClient, log)
	subscriptionMgr := eventsub.NewSubscriptionManager(twitchClientID, twitchClientSecret, log)
	channelManager := channels.NewManager(db, log, subscriptionMgr)

	// Create EventSub client
	eventSubClient := eventsub.NewClient(log)

	// Leader election state (use struct with mutex for thread-safe access from HTTP handlers)
	state := &leaderState{}

	// Set up notification handler (processes channel point redemptions)
	eventSubClient.SetOnNotification(func(event eventsub.Event, subscription *eventsub.Subscription) {
		// Only process if we're the leader
		state.RLock()
		isLdr := state.isLeader
		state.RUnlock()

		if !isLdr {
			log.Debug("Received notification but not leader, ignoring")
			return
		}

		handleEventSubNotification(ctx, event, subscription, streamPublisher, log)
	})

	// Set up welcome handler (session established, ready to create subscriptions)
	eventSubClient.SetOnWelcome(func(sessionID string) {
		state.Lock()
		state.eventSubSessionID = sessionID
		state.Unlock()
		subscriptionMgr.SetSessionID(sessionID)

		log.Info("EventSub session established",
			zap.String("session_id", sessionID),
		)

		// Subscribe to all active channels (check leadership status)
		state.RLock()
		isLdr := state.isLeader
		state.RUnlock()

		if isLdr {
			activeChannels := channelManager.GetActiveChannels()
			for broadcasterID, channel := range activeChannels {
				if _, err := subscriptionMgr.SubscribeChannelPoints(ctx, broadcasterID, channel.AccessToken); err != nil {
					log.Error("Failed to create subscription",
						zap.String("broadcaster_id", broadcasterID),
						zap.Error(err),
					)
				}
			}
		}
	})

	// Set up reconnect handler
	eventSubClient.SetOnReconnect(func(reconnectURL string) {
		log.Warn("EventSub reconnection requested",
			zap.String("reconnect_url", reconnectURL),
		)

		// Disconnect from current connection
		eventSubClient.Disconnect()

		// Reconnect to new URL
		time.Sleep(1 * time.Second) // Brief delay before reconnect
		if err := eventSubClient.ConnectTo(ctx, reconnectURL); err != nil {
			log.Error("Failed to reconnect", zap.Error(err))
		}
	})

	// Set up channel manager callback (creates/deletes subscriptions)
	channelManager.SetSubscriptionCallback(func(broadcasterID string, accessToken string, action string) error {
		state.RLock()
		isLdr := state.isLeader
		sessionID := state.eventSubSessionID
		state.RUnlock()

		if !isLdr || sessionID == "" {
			// Only leader creates subscriptions, and only after session is established
			return nil
		}

		if action == "subscribe" {
			_, err := subscriptionMgr.SubscribeChannelPoints(ctx, broadcasterID, accessToken)
			return err
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
					// Became leader
					log.Info("Acquired leadership", zap.String("instance_id", instanceID))

					// Connect to EventSub WebSocket
					if err := eventSubClient.Connect(ctx); err != nil {
						log.Error("Failed to connect to EventSub", zap.Error(err))
						state.Lock()
						state.isLeader = false
						state.Unlock()
						continue
					}

					// Start channel manager
					channelManager.Start(ctx, ChannelSyncInterval)

				} else if wasLeader && !acquired {
					// Lost leadership
					log.Warn("Lost leadership", zap.String("instance_id", instanceID))

					// Disconnect from EventSub
					eventSubClient.Disconnect()

					// Stop channel manager
					channelManager.Stop()
				}
			}
		}
	}()

	// Start HTTP server for health checks
	startHTTPServer(log, getEnv("PORT", "8090"), state, eventSubClient)

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
		eventSubClient.Disconnect()
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

// handleEventSubNotification processes EventSub notifications
func handleEventSubNotification(
	ctx context.Context,
	event eventsub.Event,
	subscription *eventsub.Subscription,
	pub *publisher.StreamPublisher,
	log *zap.Logger,
) {
	switch subscription.Type {
	case "channel.channel_points_custom_reward_redemption.add":
		handleChannelPointsRedemption(ctx, event, pub, log)

	case "channel.subscribe":
		// Optional: handle subscribe events (already have via IRC)
		log.Debug("Received subscribe event (already handled via IRC)")

	default:
		log.Debug("Unhandled subscription type",
			zap.String("type", subscription.Type),
		)
	}
}

// handleChannelPointsRedemption processes channel points redemption events
func handleChannelPointsRedemption(
	ctx context.Context,
	event eventsub.Event,
	pub *publisher.StreamPublisher,
	log *zap.Logger,
) {
	redemption, err := eventsub.ParseChannelPointsRedemption(event)
	if err != nil {
		log.Error("Failed to parse channel points redemption", zap.Error(err))
		return
	}

	// Create raw message
	rawMsg := &models.RawChatMessage{
		MessageID: uuid.New().String(),
		Platform:  "twitch",
		ChannelID: strings.ToLower(redemption.BroadcasterUserLogin),
		UserID:    redemption.UserID,
		Username:  strings.ToLower(redemption.UserLogin),
		Text:      fmt.Sprintf("Redeemed %s", redemption.Reward.Title),
		Timestamp: redemption.RedeemedAt.UTC(),
		Tags: map[string]string{
			"user-id":      redemption.UserID,
			"login":        redemption.UserLogin,
			"display-name": redemption.UserName,
		},
		EventType: "channel_points",
		EventData: map[string]interface{}{
			"reward_id":    redemption.Reward.ID,
			"reward_title": redemption.Reward.Title,
			"reward_cost":  redemption.Reward.Cost,
			"reward_prompt": redemption.Reward.Prompt,
			"user_input":   redemption.UserInput,
			"status":       redemption.Status,
			"redeemed_at":  redemption.RedeemedAt.Format(time.RFC3339),
		},
	}

	// Publish to Redis Stream
	if err := pub.Publish(ctx, rawMsg); err != nil {
		log.Error("Failed to publish channel points redemption",
			zap.String("channel", rawMsg.ChannelID),
			zap.String("reward", redemption.Reward.Title),
			zap.Error(err),
		)
		return
	}

	log.Info("Published channel points redemption",
		zap.String("channel", rawMsg.ChannelID),
		zap.String("username", rawMsg.Username),
		zap.String("reward", redemption.Reward.Title),
		zap.Int("cost", redemption.Reward.Cost),
	)
}

// startHTTPServer starts the HTTP server for health checks
func startHTTPServer(log *zap.Logger, port string, state *leaderState, client *eventsub.Client) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// Liveness probe
	router.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Readiness probe
	router.GET("/health/ready", func(c *gin.Context) {
		state.RLock()
		isLdr := state.isLeader
		state.RUnlock()

		// Pod is ready if service is running
		// Leader status and WebSocket connection don't affect readiness
		c.JSON(http.StatusOK, gin.H{
			"status":    "ready",
			"is_leader": isLdr,
			"connected": client.IsConnected(),
		})
	})

	// Status endpoint
	router.GET("/status", func(c *gin.Context) {
		state.RLock()
		isLdr := state.isLeader
		state.RUnlock()

		c.JSON(http.StatusOK, gin.H{
			"is_leader":  isLdr,
			"connected":  client.IsConnected(),
			"session_id": client.GetSessionID(),
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
