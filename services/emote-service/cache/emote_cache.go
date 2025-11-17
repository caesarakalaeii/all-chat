package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/emote-service/models"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var (
	// ErrCacheMiss is returned when a key is not found in cache
	ErrCacheMiss = errors.New("cache miss")
)

const cacheNamespace = "emotes:v2"

// RedisClient interface for Redis operations (allows mocking)
type RedisClient interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
}

// EmoteCache handles caching of emotes in Redis
type EmoteCache struct {
	client RedisClient
	logger *zap.Logger
	ttl    time.Duration
	keyNS  string
}

// NewEmoteCache creates a new emote cache
func NewEmoteCache(client RedisClient, logger *zap.Logger, ttl time.Duration) *EmoteCache {
	return &EmoteCache{
		client: client,
		logger: logger,
		ttl:    ttl,
		keyNS:  cacheNamespace,
	}
}

// Get retrieves emotes from cache
func (c *EmoteCache) Get(ctx context.Context, provider, channel string) ([]models.Emote, error) {
	key := c.key(provider, channel)

	c.logger.Debug("Getting emotes from cache",
		zap.String("key", key))

	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrCacheMiss
		}
		return nil, fmt.Errorf("failed to get from cache: %w", err)
	}

	var emotes []models.Emote
	if err := json.Unmarshal([]byte(val), &emotes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached emotes: %w", err)
	}

	c.logger.Debug("Cache hit",
		zap.String("key", key),
		zap.Int("count", len(emotes)))

	return emotes, nil
}

// Set stores emotes in cache
func (c *EmoteCache) Set(ctx context.Context, provider, channel string, emotes []models.Emote) error {
	key := c.key(provider, channel)

	data, err := json.Marshal(emotes)
	if err != nil {
		return fmt.Errorf("failed to marshal emotes: %w", err)
	}

	c.logger.Debug("Setting emotes in cache",
		zap.String("key", key),
		zap.Int("count", len(emotes)),
		zap.Duration("ttl", c.ttl))

	if err := c.client.Set(ctx, key, data, c.ttl).Err(); err != nil {
		return fmt.Errorf("failed to set cache: %w", err)
	}

	return nil
}

// key generates a cache key for a provider and channel
func (c *EmoteCache) key(provider, channel string) string {
	prefix := c.keyNS
	if prefix == "" {
		prefix = cacheNamespace
	}
	return fmt.Sprintf("%s:%s:%s", prefix, provider, channel)
}
