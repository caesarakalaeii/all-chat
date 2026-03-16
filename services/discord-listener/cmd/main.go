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

	"github.com/caesar/all-chat/services/discord-listener/gateway"
	"github.com/caesar/all-chat/services/discord-listener/handlers"
	"github.com/caesar/all-chat/services/discord-listener/publisher"
	"github.com/caesar/all-chat/services/discord-listener/relay"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// redisSessionStore wraps *redis.Client to satisfy gateway.SessionStore interface.
type redisSessionStore struct{ client *redis.Client }

func (r *redisSessionStore) Set(ctx context.Context, key, value string) error {
	return r.client.Set(ctx, key, value, 0).Err()
}
func (r *redisSessionStore) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

// channelRegistryValue is the JSON structure stored at discord:channels:{channel_id}.
type channelRegistryValue struct {
	OverlayID string `json:"overlay_id"`
	SourceID  string `json:"source_id"`
}

// redisChannelRegistry implements gateway.ChannelRegistry backed by Redis GET calls.
// Per CONTEXT.md the pure-Redis-GET approach is acceptable at v1.5 scale.
type redisChannelRegistry struct{ client *redis.Client }

func (r *redisChannelRegistry) GetOverlayForChannel(ctx context.Context, channelID string) (string, bool, error) {
	key := "discord:channels:" + channelID
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("channel registry get failed: %w", err)
	}
	var v channelRegistryValue
	if err := json.Unmarshal([]byte(val), &v); err != nil {
		return "", false, fmt.Errorf("channel registry unmarshal failed: %w", err)
	}
	return v.OverlayID, true, nil
}

func (r *redisChannelRegistry) Subscribe(_ context.Context, _ chan<- string) error {
	// Not used in the pure-Redis-GET approach — each lookup is a direct GET.
	return nil
}

// redisGuildCache implements gateway.GuildCache backed by Redis.
// Channel names are stored at discord:guild:channels:{channelID} (distinct from
// discord:channels:{channelID} used by the channel registry to avoid key collision).
// Role names are stored at discord:guild:roles:{roleID}.
type redisGuildCache struct{ client *redis.Client }

func (r *redisGuildCache) SetChannelName(ctx context.Context, channelID, name string) error {
	return r.client.Set(ctx, "discord:guild:channels:"+channelID, name, 0).Err()
}

func (r *redisGuildCache) GetChannelName(ctx context.Context, channelID string) (string, bool, error) {
	val, err := r.client.Get(ctx, "discord:guild:channels:"+channelID).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return val, true, nil
}

func (r *redisGuildCache) DeleteChannelName(ctx context.Context, channelID string) error {
	return r.client.Del(ctx, "discord:guild:channels:"+channelID).Err()
}

func (r *redisGuildCache) SetRoleName(ctx context.Context, roleID, name string) error {
	return r.client.Set(ctx, "discord:guild:roles:"+roleID, name, 0).Err()
}

func (r *redisGuildCache) GetRoleName(ctx context.Context, roleID string) (string, bool, error) {
	val, err := r.client.Get(ctx, "discord:guild:roles:"+roleID).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return val, true, nil
}

func (r *redisGuildCache) DeleteRoleName(ctx context.Context, roleID string) error {
	return r.client.Del(ctx, "discord:guild:roles:"+roleID).Err()
}

// publisherAdapter adapts *publisher.StreamPublisher to gateway.MessagePublisher.
// The gateway package uses interface{} to avoid a circular import.
type publisherAdapter struct{ pub *publisher.StreamPublisher }

func (a *publisherAdapter) Publish(ctx context.Context, msg interface{}) error {
	// Re-marshal through JSON to build a proper publisher.RawMessage from the
	// map[string]interface{} that handleMessageCreate constructs.
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal raw message map: %w", err)
	}
	var rawMsg publisher.RawMessage
	if err := json.Unmarshal(data, &rawMsg); err != nil {
		return fmt.Errorf("failed to unmarshal raw message: %w", err)
	}
	return a.pub.Publish(ctx, &rawMsg)
}

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync() //nolint:errcheck

	botToken := os.Getenv("DISCORD_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("DISCORD_BOT_TOKEN must be set")
	}

	redisHost := getEnv("REDIS_HOST", "localhost")
	redisPort := getEnv("REDIS_PORT", "6379")
	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort),
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dbPool, err := pgxpool.New(ctx, buildDatabaseDSN())
	if err != nil {
		log.Fatal("failed to connect to database for relay", zap.Error(err))
	}
	defer dbPool.Close()

	relayRepo := relay.NewRepository(dbPool)
	relayPoster := relay.NewHTTPPoster(botToken, &http.Client{Timeout: 10 * time.Second}, log)
	relayMgr := relay.NewManager(relayRepo, relayPoster, rdb, dbPool, log)

	gatewayURL := getEnv("DISCORD_GATEWAY_URL", "wss://gateway.discord.gg/?v=10&encoding=json")
	store := &redisSessionStore{client: rdb}
	registry := &redisChannelRegistry{client: rdb}
	guildCache := &redisGuildCache{client: rdb}
	streamPub := publisher.NewStreamPublisher(rdb, log)
	pubAdapter := &publisherAdapter{pub: streamPub}

	gwClient := gateway.NewGatewayClient(botToken, gatewayURL, store, log, registry, pubAdapter, guildCache)

	// Start Gateway connection in background with automatic reconnect.
	// TODO(Phase 31): gate on shard ownership via source-manager leader election
	//   Redis lock key: discord:gateway:shard:0:holder
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			log.Info("Starting Gateway connection")
			if err := gwClient.Connect(ctx); err != nil && ctx.Err() == nil {
				log.Warn("Gateway disconnected, reconnecting in 5s", zap.Error(err))
				select {
				case <-time.After(5 * time.Second):
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	go func() {
		if err := relayMgr.Start(ctx); err != nil && ctx.Err() == nil {
			log.Error("relay manager start failed", zap.Error(err))
		}
	}()

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	healthHandler := handlers.NewHealthHandler(rdb)
	router.GET("/health/live", healthHandler.CheckLive)
	router.GET("/health/ready", healthHandler.CheckReady)

	port := getEnv("PORT", "8086")
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("discord-listener HTTP server starting", zap.String("port", port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("HTTP server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	log.Info("Shutting down discord-listener")
	gwClient.Close()
	relayMgr.Stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func buildDatabaseDSN() string {
	host := getEnv("DATABASE_HOST", "localhost")
	port := getEnv("DATABASE_PORT", "5432")
	name := getEnv("DATABASE_NAME", "allchat")
	user := getEnv("DATABASE_USER", "allchat")
	password := getEnv("DATABASE_PASSWORD", "allchat_dev_password")
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s", user, password, host, port, name)
}
