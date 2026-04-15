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
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRedis(t *testing.T) (*RedisRegistry, *miniredis.Miniredis) {
	t.Helper()

	// Create in-memory Redis server
	mr, err := miniredis.Run()
	require.NoError(t, err)

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// Create registry with 1-hour TTL
	registry := NewRedisRegistry(client, 1*time.Hour)

	return registry, mr
}

func TestAddAndLookup(t *testing.T) {
	registry, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Add a message ID
	err := registry.Add(ctx, "twitch", "shroud", "msg-123", "uuid-456")
	require.NoError(t, err)

	// Lookup should return the UUID
	uuid, err := registry.Lookup(ctx, "twitch", "shroud", "msg-123")
	require.NoError(t, err)
	assert.Equal(t, "uuid-456", uuid)
}

func TestLookupNonExistent(t *testing.T) {
	registry, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Lookup non-existent message should return ErrMessageNotFound
	_, err := registry.Lookup(ctx, "twitch", "shroud", "non-existent")
	assert.ErrorIs(t, err, ErrMessageNotFound)
}

func TestTTLSetCorrectly(t *testing.T) {
	registry, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Add a message ID
	err := registry.Add(ctx, "twitch", "shroud", "msg-123", "uuid-456")
	require.NoError(t, err)

	// Check TTL is set correctly (3600 seconds = 1 hour)
	key := "msgid:registry:twitch:shroud"
	ttl := mr.TTL(key)

	// TTL should be approximately 1 hour (allow 1 second tolerance for test execution)
	assert.InDelta(t, 3600, ttl.Seconds(), 1.0)
}

func TestMultipleChannelsDontConflict(t *testing.T) {
	registry, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Add same platform message ID to different channels
	err := registry.Add(ctx, "twitch", "shroud", "msg-123", "uuid-aaa")
	require.NoError(t, err)

	err = registry.Add(ctx, "twitch", "ninja", "msg-123", "uuid-bbb")
	require.NoError(t, err)

	// Lookups should return different UUIDs
	uuid1, err := registry.Lookup(ctx, "twitch", "shroud", "msg-123")
	require.NoError(t, err)
	assert.Equal(t, "uuid-aaa", uuid1)

	uuid2, err := registry.Lookup(ctx, "twitch", "ninja", "msg-123")
	require.NoError(t, err)
	assert.Equal(t, "uuid-bbb", uuid2)
}

func TestSamePlatformMessageIDInDifferentChannels(t *testing.T) {
	registry, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Add same platform message ID to different channels with different UUIDs
	err := registry.Add(ctx, "twitch", "channel1", "msg-999", "uuid-111")
	require.NoError(t, err)

	err = registry.Add(ctx, "twitch", "channel2", "msg-999", "uuid-222")
	require.NoError(t, err)

	// Verify both are stored separately
	uuid1, err := registry.Lookup(ctx, "twitch", "channel1", "msg-999")
	require.NoError(t, err)
	assert.Equal(t, "uuid-111", uuid1)

	uuid2, err := registry.Lookup(ctx, "twitch", "channel2", "msg-999")
	require.NoError(t, err)
	assert.Equal(t, "uuid-222", uuid2)
}

func TestPipelineAtomicity(t *testing.T) {
	registry, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Add message ID
	err := registry.Add(ctx, "twitch", "shroud", "msg-123", "uuid-456")
	require.NoError(t, err)

	// Verify both HSET and EXPIRE succeeded
	key := "msgid:registry:twitch:shroud"

	// Check that key exists
	exists := mr.Exists(key)
	assert.True(t, exists)

	// Check that TTL is set
	ttl := mr.TTL(key)
	assert.Greater(t, ttl.Seconds(), float64(0))
}

func TestRefreshTTLOnAdd(t *testing.T) {
	registry, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Add a message ID
	err := registry.Add(ctx, "twitch", "shroud", "msg-123", "uuid-456")
	require.NoError(t, err)

	// Fast forward time by 30 minutes
	key := "msgid:registry:twitch:shroud"
	mr.FastForward(30 * time.Minute)

	// TTL should be around 30 minutes
	ttl := mr.TTL(key)
	assert.InDelta(t, 1800, ttl.Seconds(), 1.0)

	// Add another message to same channel (refreshes TTL)
	err = registry.Add(ctx, "twitch", "shroud", "msg-789", "uuid-abc")
	require.NoError(t, err)

	// TTL should be refreshed back to ~1 hour
	ttl = mr.TTL(key)
	assert.InDelta(t, 3600, ttl.Seconds(), 1.0)
}

func TestRemove(t *testing.T) {
	registry, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Add a message ID
	err := registry.Add(ctx, "twitch", "shroud", "msg-123", "uuid-456")
	require.NoError(t, err)

	// Remove it
	err = registry.Remove(ctx, "twitch", "shroud", "msg-123")
	require.NoError(t, err)

	// Lookup should now return ErrMessageNotFound
	_, err = registry.Lookup(ctx, "twitch", "shroud", "msg-123")
	assert.ErrorIs(t, err, ErrMessageNotFound)
}

func TestAddEmptyParameters(t *testing.T) {
	registry, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	tests := []struct {
		name         string
		platform     string
		channelID    string
		platformMsgID string
		internalUUID string
	}{
		{"empty platform", "", "channel", "msg-123", "uuid-456"},
		{"empty channelID", "twitch", "", "msg-123", "uuid-456"},
		{"empty platformMsgID", "twitch", "channel", "", "uuid-456"},
		{"empty internalUUID", "twitch", "channel", "msg-123", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.Add(ctx, tt.platform, tt.channelID, tt.platformMsgID, tt.internalUUID)
			assert.Error(t, err)
		})
	}
}

func TestLookupEmptyParameters(t *testing.T) {
	registry, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	tests := []struct {
		name         string
		platform     string
		channelID    string
		platformMsgID string
	}{
		{"empty platform", "", "channel", "msg-123"},
		{"empty channelID", "twitch", "", "msg-123"},
		{"empty platformMsgID", "twitch", "channel", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := registry.Lookup(ctx, tt.platform, tt.channelID, tt.platformMsgID)
			assert.Error(t, err)
		})
	}
}

func TestRemoveEmptyParameters(t *testing.T) {
	registry, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	tests := []struct {
		name         string
		platform     string
		channelID    string
		platformMsgID string
	}{
		{"empty platform", "", "channel", "msg-123"},
		{"empty channelID", "twitch", "", "msg-123"},
		{"empty platformMsgID", "twitch", "channel", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.Remove(ctx, tt.platform, tt.channelID, tt.platformMsgID)
			assert.Error(t, err)
		})
	}
}

func TestBuildRegistryKey(t *testing.T) {
	tests := []struct {
		platform  string
		channelID string
		expected  string
	}{
		{"twitch", "shroud", "msgid:registry:twitch:shroud"},
		{"youtube", "UCxyz", "msgid:registry:youtube:UCxyz"},
		{"kick", "xqc", "msgid:registry:kick:xqc"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			key := buildRegistryKey(tt.platform, tt.channelID)
			assert.Equal(t, tt.expected, key)
		})
	}
}

func TestMultiplePlatformsDontConflict(t *testing.T) {
	registry, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// Add same channel name across different platforms
	err := registry.Add(ctx, "twitch", "shroud", "msg-123", "uuid-twitch")
	require.NoError(t, err)

	err = registry.Add(ctx, "youtube", "shroud", "msg-123", "uuid-youtube")
	require.NoError(t, err)

	// Lookups should return different UUIDs per platform
	uuidTwitch, err := registry.Lookup(ctx, "twitch", "shroud", "msg-123")
	require.NoError(t, err)
	assert.Equal(t, "uuid-twitch", uuidTwitch)

	uuidYoutube, err := registry.Lookup(ctx, "youtube", "shroud", "msg-123")
	require.NoError(t, err)
	assert.Equal(t, "uuid-youtube", uuidYoutube)
}
