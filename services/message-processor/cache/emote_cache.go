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
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ErrCacheMiss indicates no cached emote entry was found for a channel.
var ErrCacheMiss = errors.New("emote cache miss")

const cacheNamespace = "mp:emotes:v2:"

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
	Delete(ctx context.Context, channel string) error
	GetWithUser(ctx context.Context, channel, userID string) ([]CachedEmote, error)
	SetWithUser(ctx context.Context, channel, userID string, emotes []CachedEmote) error
	DeletePattern(ctx context.Context, pattern string) error
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
		prefix: cacheNamespace,
	}
}

func (c *EmoteCache) key(channel string) string {
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel == "" {
		channel = "global"
	}
	prefix := c.prefix
	if prefix == "" {
		prefix = cacheNamespace
	}
	return prefix + channel
}

func (c *EmoteCache) keyWithUser(channel, userID string) string {
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel == "" {
		channel = "global"
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return c.key(channel)
	}
	prefix := c.prefix
	if prefix == "" {
		prefix = cacheNamespace
	}
	return fmt.Sprintf("%s%s:%s", prefix, channel, userID)
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

func (c *EmoteCache) Delete(ctx context.Context, channel string) error {
	if err := c.client.Del(ctx, c.key(channel)).Err(); err != nil {
		return fmt.Errorf("failed to delete emotes: %w", err)
	}
	return nil
}

func (c *EmoteCache) GetWithUser(ctx context.Context, channel, userID string) ([]CachedEmote, error) {
	raw, err := c.client.Get(ctx, c.keyWithUser(channel, userID)).Bytes()
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

func (c *EmoteCache) SetWithUser(ctx context.Context, channel, userID string, emotes []CachedEmote) error {
	data, err := json.Marshal(emotes)
	if err != nil {
		return fmt.Errorf("failed to marshal emotes: %w", err)
	}
	// Use shorter TTL for user-specific caches (1 hour instead of 6)
	ttl := 1 * time.Hour
	if err := c.client.Set(ctx, c.keyWithUser(channel, userID), data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to store emotes: %w", err)
	}
	return nil
}

func (c *EmoteCache) DeletePattern(ctx context.Context, pattern string) error {
	// Use SCAN to find all matching keys
	iter := c.client.Scan(ctx, 0, pattern, 100).Iterator()
	keysToDelete := make([]string, 0)

	for iter.Next(ctx) {
		keysToDelete = append(keysToDelete, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("failed to scan keys: %w", err)
	}

	// Delete all found keys
	if len(keysToDelete) > 0 {
		if err := c.client.Del(ctx, keysToDelete...).Err(); err != nil {
			return fmt.Errorf("failed to delete keys: %w", err)
		}
		c.logger.Debug("Deleted cache keys by pattern",
			zap.String("pattern", pattern),
			zap.Int("count", len(keysToDelete)))
	}

	return nil
}

var _ Store = (*EmoteCache)(nil)
