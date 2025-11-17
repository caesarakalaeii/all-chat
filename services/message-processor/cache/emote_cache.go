package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ErrCacheMiss indicates no cached emote entry was found for a channel.
var ErrCacheMiss = errors.New("emote cache miss")

// CachedEmote stores the minimum metadata needed to reconstruct emote markup.
type CachedEmote struct {
	Code     string `json:"code"`
	URL      string `json:"url"`
	Provider string `json:"provider"`
}

// Store defines the emote cache behaviour expected by consumers.
type Store interface {
	Get(ctx context.Context, channel string) ([]CachedEmote, error)
	Set(ctx context.Context, channel string, emotes []CachedEmote) error
}

type EmoteCache struct {
	client *redis.Client
	logger *zap.Logger
	ttl    time.Duration
	prefix string
}

func NewEmoteCache(client *redis.Client, logger *zap.Logger, ttl time.Duration) *EmoteCache {
	if ttl <= 0 {
		ttl = 6 * time.Hour
	}
	return &EmoteCache{
		client: client,
		logger: logger.With(zap.String("component", "emote-cache")),
		ttl:    ttl,
		prefix: "mp:emotes:",
	}
}

func (c *EmoteCache) key(channel string) string {
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel == "" {
		channel = "global"
	}
	return c.prefix + channel
}

func (c *EmoteCache) Get(ctx context.Context, channel string) ([]CachedEmote, error) {
	raw, err := c.client.Get(ctx, c.key(channel)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrCacheMiss
		}
		return nil, err
	}
	var emotes []CachedEmote
	if err := json.Unmarshal(raw, &emotes); err != nil {
		return nil, fmt.Errorf("failed to decode cached emotes: %w", err)
	}
	return emotes, nil
}

func (c *EmoteCache) Set(ctx context.Context, channel string, emotes []CachedEmote) error {
	data, err := json.Marshal(emotes)
	if err != nil {
		return fmt.Errorf("failed to marshal emotes: %w", err)
	}
	if err := c.client.Set(ctx, c.key(channel), data, c.ttl).Err(); err != nil {
		return fmt.Errorf("failed to store emotes: %w", err)
	}
	return nil
}

var _ Store = (*EmoteCache)(nil)
