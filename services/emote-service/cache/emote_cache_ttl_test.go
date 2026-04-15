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
	"testing"
	"time"

	"github.com/caesar/all-chat/services/emote-service/models"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// mockRedisClientWithTTL is a mock that tracks TTL values
type mockRedisClientWithTTL struct {
	data map[string]string
	ttls map[string]time.Duration
}

func newMockRedisClientWithTTL() *mockRedisClientWithTTL {
	return &mockRedisClientWithTTL{
		data: make(map[string]string),
		ttls: make(map[string]time.Duration),
	}
}

func (m *mockRedisClientWithTTL) Get(ctx context.Context, key string) *redis.StringCmd {
	cmd := redis.NewStringCmd(ctx)
	val, ok := m.data[key]
	if !ok {
		cmd.SetErr(redis.Nil)
		return cmd
	}
	cmd.SetVal(val)
	return cmd
}

func (m *mockRedisClientWithTTL) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(ctx)
	// Convert value to string (it may be []byte or string)
	switch v := value.(type) {
	case string:
		m.data[key] = v
	case []byte:
		m.data[key] = string(v)
	default:
		m.data[key] = ""
	}
	m.ttls[key] = expiration
	cmd.SetVal("OK")
	return cmd
}

func TestEmoteCache_Set_GlobalEmotesTTL(t *testing.T) {
	tests := []struct {
		name        string
		provider    string
		channel     string
		expectedTTL time.Duration
	}{
		{
			name:        "Twitch global emotes get 30-day TTL",
			provider:    "twitch",
			channel:     "global",
			expectedTTL: 30 * 24 * time.Hour,
		},
		{
			name:        "Twitch channel emotes get 1-hour TTL",
			provider:    "twitch",
			channel:     "xqc",
			expectedTTL: 1 * time.Hour,
		},
		{
			name:        "7TV emotes get 1-hour TTL",
			provider:    "7tv",
			channel:     "xqc",
			expectedTTL: 1 * time.Hour,
		},
		{
			name:        "BTTV emotes get 1-hour TTL",
			provider:    "bttv",
			channel:     "shroud",
			expectedTTL: 1 * time.Hour,
		},
		{
			name:        "FFZ emotes get 1-hour TTL",
			provider:    "ffz",
			channel:     "lirik",
			expectedTTL: 1 * time.Hour,
		},
		{
			name:        "Global channel for non-Twitch provider gets 1-hour TTL",
			provider:    "7tv",
			channel:     "global",
			expectedTTL: 1 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			client := newMockRedisClientWithTTL()
			cache := NewEmoteCache(client, logger, 1*time.Hour)

			emotes := []models.Emote{
				{Code: "TestEmote", URL: "https://example.com/emote.png", Provider: tt.provider, Channel: tt.channel},
			}

			err := cache.Set(context.Background(), tt.provider, tt.channel, emotes)
			require.NoError(t, err)

			key := cache.key(tt.provider, tt.channel)
			actualTTL, ok := client.ttls[key]
			require.True(t, ok, "TTL should be set for key %s", key)
			assert.Equal(t, tt.expectedTTL, actualTTL, "TTL mismatch for %s:%s", tt.provider, tt.channel)
		})
	}
}

func TestEmoteCache_GlobalTTL_Default(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := newMockRedisClientWithTTL()
	cache := NewEmoteCache(client, logger, 2*time.Hour)

	// Verify that globalTTL is set to 30 days by default
	assert.Equal(t, 30*24*time.Hour, cache.globalTTL, "Global TTL should default to 30 days")
	assert.Equal(t, 2*time.Hour, cache.ttl, "Regular TTL should match constructor argument")
}
