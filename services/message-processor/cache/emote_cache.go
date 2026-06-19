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

// Namespace is the Redis key prefix for cached emote entries. It is exported so
// callers that build key patterns (e.g. for invalidation) stay in sync with the
// cache implementation. The version suffix is bumped whenever the stored value
// format changes; v3 introduced the freshness envelope (stale-while-revalidate).
const Namespace = "mp:emotes:v3:"

// userTTL is the freshness window for user-specific cache entries. User emote
// sets change more often than channel sets, so they go stale sooner.
const userTTL = 1 * time.Hour

// defaultStaleGrace is how long an entry remains servable as "stale" past its
// freshness window. During this window reads return the stale value immediately
// and trigger a background refresh instead of blocking on the emote service.
const defaultStaleGrace = 12 * time.Hour

// CachedEmote stores the minimum metadata needed to reconstruct emote markup.
type CachedEmote struct {
	Code     string `json:"code"`
	URL      string `json:"url"`
	Provider string `json:"provider"`
}

// Entry is the result of a cache lookup: the cached emotes plus whether the
// entry is past its freshness window and should be refreshed in the background.
type Entry struct {
	Emotes []CachedEmote
	Stale  bool
}

// envelope is the on-disk representation of a cache entry. The soft expiry lets
// reads distinguish a fresh entry from a stale-but-servable one even though the
// Redis key itself lives longer (soft TTL + stale grace).
type envelope struct {
	Emotes      []CachedEmote `json:"emotes"`
	SoftExpires int64         `json:"soft_expires_unix"`
}

// Store defines the emote cache behaviour expected by consumers.
type Store interface {
	GetEntry(ctx context.Context, channel string) (Entry, error)
	Set(ctx context.Context, channel string, emotes []CachedEmote) error
	Delete(ctx context.Context, channel string) error
	GetEntryWithUser(ctx context.Context, channel, userID string) (Entry, error)
	SetWithUser(ctx context.Context, channel, userID string, emotes []CachedEmote) error
	DeletePattern(ctx context.Context, pattern string) error
}

type EmoteCache struct {
	client     *redis.Client
	logger     *zap.Logger
	ttl        time.Duration
	staleGrace time.Duration
	prefix     string
}

func NewEmoteCache(client *redis.Client, logger *zap.Logger, ttl time.Duration) *EmoteCache {
	if ttl <= 0 {
		ttl = 6 * time.Hour
	}
	return &EmoteCache{
		client:     client,
		logger:     logger.With(zap.String("component", "emote-cache")),
		ttl:        ttl,
		staleGrace: defaultStaleGrace,
		prefix:     Namespace,
	}
}

func (c *EmoteCache) key(channel string) string {
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel == "" {
		channel = "global"
	}
	prefix := c.prefix
	if prefix == "" {
		prefix = Namespace
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
		prefix = Namespace
	}
	return fmt.Sprintf("%s%s:%s", prefix, channel, userID)
}

// getEntry reads and decodes an entry for an already-built key, flagging it
// stale when it has passed its soft expiry.
func (c *EmoteCache) getEntry(ctx context.Context, key string) (Entry, error) {
	raw, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return Entry{}, ErrCacheMiss
		}
		return Entry{}, err
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return Entry{}, fmt.Errorf("failed to decode cached emotes: %w", err)
	}
	stale := env.SoftExpires > 0 && time.Now().Unix() > env.SoftExpires
	return Entry{Emotes: env.Emotes, Stale: stale}, nil
}

// setEntry marshals emotes into an envelope with the given freshness window and
// stores it under key with a hard TTL of softTTL + staleGrace, so the value
// survives long enough to be served stale while a background refresh runs.
func (c *EmoteCache) setEntry(ctx context.Context, key string, emotes []CachedEmote, softTTL time.Duration) error {
	env := envelope{
		Emotes:      emotes,
		SoftExpires: time.Now().Add(softTTL).Unix(),
	}
	data, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("failed to marshal emotes: %w", err)
	}
	if err := c.client.Set(ctx, key, data, softTTL+c.staleGrace).Err(); err != nil {
		return fmt.Errorf("failed to store emotes: %w", err)
	}
	return nil
}

func (c *EmoteCache) GetEntry(ctx context.Context, channel string) (Entry, error) {
	return c.getEntry(ctx, c.key(channel))
}

func (c *EmoteCache) Set(ctx context.Context, channel string, emotes []CachedEmote) error {
	return c.setEntry(ctx, c.key(channel), emotes, c.ttl)
}

func (c *EmoteCache) Delete(ctx context.Context, channel string) error {
	if err := c.client.Del(ctx, c.key(channel)).Err(); err != nil {
		return fmt.Errorf("failed to delete emotes: %w", err)
	}
	return nil
}

func (c *EmoteCache) GetEntryWithUser(ctx context.Context, channel, userID string) (Entry, error) {
	return c.getEntry(ctx, c.keyWithUser(channel, userID))
}

func (c *EmoteCache) SetWithUser(ctx context.Context, channel, userID string, emotes []CachedEmote) error {
	// User-specific entries use a shorter freshness window than channel entries.
	return c.setEntry(ctx, c.keyWithUser(channel, userID), emotes, userTTL)
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
