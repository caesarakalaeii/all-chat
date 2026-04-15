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
	client    RedisClient
	logger    *zap.Logger
	ttl       time.Duration
	globalTTL time.Duration
	keyNS     string
}

// NewEmoteCache creates a new emote cache
func NewEmoteCache(client RedisClient, logger *zap.Logger, ttl time.Duration) *EmoteCache {
	return &EmoteCache{
		client:    client,
		logger:    logger,
		ttl:       ttl,
		globalTTL: 30 * 24 * time.Hour, // Global Twitch emotes cached for 30 days
		keyNS:     cacheNamespace,
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

	// Use longer TTL for Twitch global emotes
	ttl := c.ttl
	if provider == "twitch" && channel == "global" {
		ttl = c.globalTTL
	}

	c.logger.Debug("Setting emotes in cache",
		zap.String("key", key),
		zap.Int("count", len(emotes)),
		zap.Duration("ttl", ttl))

	if err := c.client.Set(ctx, key, data, ttl).Err(); err != nil {
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
