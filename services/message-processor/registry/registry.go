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

package registry

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrMessageNotFound is returned when a message ID lookup fails
var ErrMessageNotFound = errors.New("message not found in registry")

// MessageIDRegistry defines the interface for mapping platform message IDs to internal UUIDs
type MessageIDRegistry interface {
	// Add stores a mapping from platform message ID to internal UUID
	Add(ctx context.Context, platform, channelID, platformMsgID, internalUUID string) error

	// Lookup retrieves the internal UUID for a given platform message ID
	Lookup(ctx context.Context, platform, channelID, platformMsgID string) (string, error)

	// Remove deletes a message ID mapping (used for testing/cleanup)
	Remove(ctx context.Context, platform, channelID, platformMsgID string) error
}

// RedisRegistry implements MessageIDRegistry using Redis hashes
type RedisRegistry struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisRegistry creates a new Redis-backed message ID registry
func NewRedisRegistry(client *redis.Client, ttl time.Duration) *RedisRegistry {
	return &RedisRegistry{
		client: client,
		ttl:    ttl,
	}
}

// Add stores a platform message ID to internal UUID mapping
// Uses Redis HSET with EXPIRE to refresh TTL on each add
func (r *RedisRegistry) Add(ctx context.Context, platform, channelID, platformMsgID, internalUUID string) error {
	if platform == "" || channelID == "" || platformMsgID == "" || internalUUID == "" {
		return errors.New("platform, channelID, platformMsgID, and internalUUID cannot be empty")
	}

	key := buildRegistryKey(platform, channelID)
	// Store value as "{uuid}|{timestamp}" for debugging
	value := fmt.Sprintf("%s|%d", internalUUID, time.Now().Unix())

	// Use pipeline for atomicity: HSET + EXPIRE in single transaction
	pipe := r.client.Pipeline()
	pipe.HSet(ctx, key, platformMsgID, value)
	pipe.Expire(ctx, key, r.ttl)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to add message ID to registry: %w", err)
	}

	return nil
}

// Lookup retrieves the internal UUID for a platform message ID
// Returns ErrMessageNotFound if the message ID doesn't exist
func (r *RedisRegistry) Lookup(ctx context.Context, platform, channelID, platformMsgID string) (string, error) {
	if platform == "" || channelID == "" || platformMsgID == "" {
		return "", errors.New("platform, channelID, and platformMsgID cannot be empty")
	}

	key := buildRegistryKey(platform, channelID)

	value, err := r.client.HGet(ctx, key, platformMsgID).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", ErrMessageNotFound
		}
		return "", fmt.Errorf("failed to lookup message ID: %w", err)
	}

	// Extract UUID from stored value by splitting on pipe character
	parts := strings.Split(value, "|")
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid stored value format: %s", value)
	}

	return parts[0], nil
}

// Remove deletes a message ID mapping from the registry
// Used for testing and cleanup operations
func (r *RedisRegistry) Remove(ctx context.Context, platform, channelID, platformMsgID string) error {
	if platform == "" || channelID == "" || platformMsgID == "" {
		return errors.New("platform, channelID, and platformMsgID cannot be empty")
	}

	key := buildRegistryKey(platform, channelID)

	err := r.client.HDel(ctx, key, platformMsgID).Err()
	if err != nil {
		return fmt.Errorf("failed to remove message ID: %w", err)
	}

	return nil
}

// buildRegistryKey constructs the Redis hash key for a platform/channel combination
// Format: msgid:registry:{platform}:{channelID}
func buildRegistryKey(platform, channelID string) string {
	return fmt.Sprintf("msgid:registry:%s:%s", platform, channelID)
}
