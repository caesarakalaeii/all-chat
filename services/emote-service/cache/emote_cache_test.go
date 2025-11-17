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

// mockRedisClient for testing
type mockRedisClient struct {
	data map[string]string
}

func newMockRedisClient() *mockRedisClient {
	return &mockRedisClient{
		data: make(map[string]string),
	}
}

func (m *mockRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	val, ok := m.data[key]
	cmd := redis.NewStringCmd(ctx)
	if !ok {
		cmd.SetErr(redis.Nil)
	} else {
		cmd.SetVal(val)
	}
	return cmd
}

func (m *mockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	// Handle both string and []byte
	switch v := value.(type) {
	case string:
		m.data[key] = v
	case []byte:
		m.data[key] = string(v)
	default:
		m.data[key] = string(value.([]byte))
	}
	cmd := redis.NewStatusCmd(ctx)
	cmd.SetVal("OK")
	return cmd
}

func TestEmoteCache_Get(t *testing.T) {
	tests := []struct {
		name       string
		provider   string
		channel    string
		setupCache func(*mockRedisClient)
		wantEmotes int
		wantErr    bool
		isNotFound bool
	}{
		{
			name:     "cache hit",
			provider: "7tv",
			channel:  "xqc",
			setupCache: func(m *mockRedisClient) {
				m.data["emotes:v2:7tv:xqc"] = `[{"code":"OMEGALUL","url":"https://cdn.7tv.app/emote/123/1x.webp","provider":"7tv","channel":"xqc"}]`
			},
			wantEmotes: 1,
			wantErr:    false,
			isNotFound: false,
		},
		{
			name:     "cache miss",
			provider: "bttv",
			channel:  "shroud",
			setupCache: func(m *mockRedisClient) {
				// No data in cache
			},
			wantEmotes: 0,
			wantErr:    true,
			isNotFound: true,
		},
		{
			name:     "empty emote list cached",
			provider: "ffz",
			channel:  "newstreamer",
			setupCache: func(m *mockRedisClient) {
				m.data["emotes:v2:ffz:newstreamer"] = `[]`
			},
			wantEmotes: 0,
			wantErr:    false,
			isNotFound: false,
		},
		{
			name:     "invalid JSON in cache",
			provider: "7tv",
			channel:  "xqc",
			setupCache: func(m *mockRedisClient) {
				m.data["emotes:v2:7tv:xqc"] = `{invalid json}`
			},
			wantEmotes: 0,
			wantErr:    true,
			isNotFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRedis := newMockRedisClient()
			tt.setupCache(mockRedis)

			logger := zaptest.NewLogger(t)
			cache := &EmoteCache{
				client: mockRedis,
				logger: logger,
				ttl:    time.Hour,
			}

			emotes, err := cache.Get(context.Background(), tt.provider, tt.channel)

			if tt.wantErr {
				require.Error(t, err)
				if tt.isNotFound {
					assert.ErrorIs(t, err, ErrCacheMiss)
				}
			} else {
				require.NoError(t, err)
				assert.Len(t, emotes, tt.wantEmotes)
			}
		})
	}
}

func TestEmoteCache_Set(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		channel  string
		emotes   []models.Emote
		wantErr  bool
	}{
		{
			name:     "set emotes successfully",
			provider: "7tv",
			channel:  "xqc",
			emotes: []models.Emote{
				{
					Code:     "OMEGALUL",
					URL:      "https://cdn.7tv.app/emote/123/1x.webp",
					Provider: "7tv",
					Channel:  "xqc",
				},
			},
			wantErr: false,
		},
		{
			name:     "set empty emote list",
			provider: "bttv",
			channel:  "shroud",
			emotes:   []models.Emote{},
			wantErr:  false,
		},
		{
			name:     "set multiple emotes",
			provider: "ffz",
			channel:  "xqc",
			emotes: []models.Emote{
				{
					Code:     "xqcL",
					URL:      "https://cdn.ffz.com/1",
					Provider: "ffz",
					Channel:  "xqc",
				},
				{
					Code:     "xqcT",
					URL:      "https://cdn.ffz.com/2",
					Provider: "ffz",
					Channel:  "xqc",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRedis := newMockRedisClient()
			logger := zaptest.NewLogger(t)
			cache := &EmoteCache{
				client: mockRedis,
				logger: logger,
				ttl:    time.Hour,
			}

			err := cache.Set(context.Background(), tt.provider, tt.channel, tt.emotes)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				// Verify data was stored
				key := cache.key(tt.provider, tt.channel)
				_, exists := mockRedis.data[key]
				assert.True(t, exists, "data should be stored in cache")
			}
		})
	}
}

func TestEmoteCache_Key(t *testing.T) {
	cache := &EmoteCache{}

	tests := []struct {
		provider string
		channel  string
		want     string
	}{
		{"7tv", "xqc", "emotes:v2:7tv:xqc"},
		{"bttv", "shroud", "emotes:v2:bttv:shroud"},
		{"ffz", "lirik", "emotes:v2:ffz:lirik"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := cache.key(tt.provider, tt.channel)
			assert.Equal(t, tt.want, got)
		})
	}
}
