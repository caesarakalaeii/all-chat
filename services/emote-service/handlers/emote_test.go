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

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/caesar/all-chat/services/emote-service/cache"
	"github.com/caesar/all-chat/services/emote-service/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// mockEmoteClient for testing
type mockEmoteClient struct {
	emotes   []models.Emote
	err      error
	provider string
}

func (m *mockEmoteClient) FetchEmotes(ctx context.Context, channel string) ([]models.Emote, error) {
	return m.emotes, m.err
}

func (m *mockEmoteClient) Provider() string {
	return m.provider
}

// mockEmoteCache for testing. The handler fans provider fetches out across goroutines,
// so this shared cache must be safe for concurrent Get/Set.
type mockEmoteCache struct {
	mu   sync.Mutex
	data map[string][]models.Emote
}

func newMockEmoteCache() *mockEmoteCache {
	return &mockEmoteCache{
		data: make(map[string][]models.Emote),
	}
}

func (m *mockEmoteCache) Get(ctx context.Context, provider, channel string) ([]models.Emote, error) {
	key := provider + ":" + channel
	m.mu.Lock()
	defer m.mu.Unlock()
	emotes, ok := m.data[key]
	if !ok {
		return nil, cache.ErrCacheMiss
	}
	return emotes, nil
}

func (m *mockEmoteCache) Set(ctx context.Context, provider, channel string, emotes []models.Emote) error {
	key := provider + ":" + channel
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = emotes
	return nil
}

func TestEmoteHandler_GetChannelEmotes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		channel        string
		setupClients   func() map[string]EmoteClient
		setupCache     func() EmoteCache
		wantStatusCode int
		wantEmoteCount int
		wantErr        bool
	}{
		{
			name:    "successful fetch from all providers",
			channel: "xqc",
			setupClients: func() map[string]EmoteClient {
				return map[string]EmoteClient{
					"7tv": &mockEmoteClient{
						emotes: []models.Emote{
							{Code: "OMEGALUL", URL: "https://7tv.app/1", Provider: "7tv", Channel: "xqc"},
						},
						provider: "7tv",
					},
					"bttv": &mockEmoteClient{
						emotes: []models.Emote{
							{Code: "xqcL", URL: "https://bttv.net/1", Provider: "bttv", Channel: "xqc"},
						},
						provider: "bttv",
					},
					"ffz": &mockEmoteClient{
						emotes: []models.Emote{
							{Code: "xqcT", URL: "https://ffz.com/1", Provider: "ffz", Channel: "xqc"},
						},
						provider: "ffz",
					},
				}
			},
			setupCache:     func() EmoteCache { return newMockEmoteCache() },
			wantStatusCode: http.StatusOK,
			wantEmoteCount: 3,
			wantErr:        false,
		},
		{
			name:    "channel name with spaces",
			channel: "Caesar LP",
			setupClients: func() map[string]EmoteClient {
				return map[string]EmoteClient{
					"7tv": &mockEmoteClient{
						emotes: []models.Emote{
							{Code: "PogChamp", URL: "https://7tv.app/pogchamp", Provider: "7tv", Channel: "Caesar LP"},
						},
						provider: "7tv",
					},
				}
			},
			setupCache:     func() EmoteCache { return newMockEmoteCache() },
			wantStatusCode: http.StatusOK,
			wantEmoteCount: 1,
			wantErr:        false,
		},
		{
			name:    "cache hit - no external API calls",
			channel: "shroud",
			setupClients: func() map[string]EmoteClient {
				return map[string]EmoteClient{
					"7tv": &mockEmoteClient{
						emotes:   []models.Emote{},
						err:      errors.New("should not be called"),
						provider: "7tv",
					},
				}
			},
			setupCache: func() EmoteCache {
				cache := newMockEmoteCache()
				cache.data["7tv:shroud"] = []models.Emote{
					{Code: "Cached", URL: "https://cached.com/1", Provider: "7tv", Channel: "shroud"},
				}
				cache.data["bttv:shroud"] = []models.Emote{}
				cache.data["ffz:shroud"] = []models.Emote{}
				return cache
			},
			wantStatusCode: http.StatusOK,
			wantEmoteCount: 1,
			wantErr:        false,
		},
		{
			name:    "missing channel parameter",
			channel: "",
			setupClients: func() map[string]EmoteClient {
				return map[string]EmoteClient{}
			},
			setupCache:     func() EmoteCache { return newMockEmoteCache() },
			wantStatusCode: http.StatusNotFound, // Gin returns 404 for missing path params
			wantErr:        true,
		},
		{
			name:    "one provider fails, others succeed",
			channel: "xqc",
			setupClients: func() map[string]EmoteClient {
				return map[string]EmoteClient{
					"7tv": &mockEmoteClient{
						emotes: []models.Emote{
							{Code: "OMEGALUL", URL: "https://7tv.app/1", Provider: "7tv", Channel: "xqc"},
						},
						provider: "7tv",
					},
					"bttv": &mockEmoteClient{
						emotes:   nil,
						err:      errors.New("API error"),
						provider: "bttv",
					},
					"ffz": &mockEmoteClient{
						emotes: []models.Emote{
							{Code: "xqcT", URL: "https://ffz.com/1", Provider: "ffz", Channel: "xqc"},
						},
						provider: "ffz",
					},
				}
			},
			setupCache:     func() EmoteCache { return newMockEmoteCache() },
			wantStatusCode: http.StatusOK,
			wantEmoteCount: 2, // Only 7TV and FFZ succeed
			wantErr:        false,
		},
		{
			name:    "invalid channel characters rejected",
			channel: "bad!channel",
			setupClients: func() map[string]EmoteClient {
				return map[string]EmoteClient{}
			},
			setupCache:     func() EmoteCache { return newMockEmoteCache() },
			wantStatusCode: http.StatusBadRequest,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			handler := NewEmoteHandler(tt.setupClients(), tt.setupCache(), logger, nil)

			router := gin.New()
			router.GET("/emotes/channel/:channel", handler.GetChannelEmotes)

			req, _ := http.NewRequest("GET", "/emotes/channel/"+tt.channel, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatusCode, w.Code)

			if !tt.wantErr {
				var resp models.EmoteResponse
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.Equal(t, tt.channel, resp.Channel)
				assert.Len(t, resp.Emotes, tt.wantEmoteCount)
			}
		})
	}
}

func TestEmoteHandler_GetProviderEmotes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		provider       string
		channel        string
		setupClient    func() EmoteClient
		setupCache     func() EmoteCache
		wantStatusCode int
		wantEmoteCount int
	}{
		{
			name:     "7tv emotes",
			provider: "7tv",
			channel:  "xqc",
			setupClient: func() EmoteClient {
				return &mockEmoteClient{
					emotes: []models.Emote{
						{Code: "OMEGALUL", URL: "https://7tv.app/1", Provider: "7tv", Channel: "xqc"},
						{Code: "LULW", URL: "https://7tv.app/2", Provider: "7tv", Channel: "xqc"},
					},
					provider: "7tv",
				}
			},
			setupCache:     func() EmoteCache { return newMockEmoteCache() },
			wantStatusCode: http.StatusOK,
			wantEmoteCount: 2,
		},
		{
			name:     "bttv emotes",
			provider: "bttv",
			channel:  "shroud",
			setupClient: func() EmoteClient {
				return &mockEmoteClient{
					emotes: []models.Emote{
						{Code: "shroudW", URL: "https://bttv.net/1", Provider: "bttv", Channel: "shroud"},
					},
					provider: "bttv",
				}
			},
			setupCache:     func() EmoteCache { return newMockEmoteCache() },
			wantStatusCode: http.StatusOK,
			wantEmoteCount: 1,
		},
		{
			name:     "ffz emotes",
			provider: "ffz",
			channel:  "lirik",
			setupClient: func() EmoteClient {
				return &mockEmoteClient{
					emotes:   []models.Emote{},
					provider: "ffz",
				}
			},
			setupCache:     func() EmoteCache { return newMockEmoteCache() },
			wantStatusCode: http.StatusOK,
			wantEmoteCount: 0,
		},
		{
			name:     "cache hit",
			provider: "7tv",
			channel:  "xqc",
			setupClient: func() EmoteClient {
				return &mockEmoteClient{
					emotes:   nil,
					err:      errors.New("should not be called"),
					provider: "7tv",
				}
			},
			setupCache: func() EmoteCache {
				cache := newMockEmoteCache()
				cache.data["7tv:xqc"] = []models.Emote{
					{Code: "Cached", URL: "https://cached.com/1", Provider: "7tv", Channel: "xqc"},
				}
				return cache
			},
			wantStatusCode: http.StatusOK,
			wantEmoteCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			clients := map[string]EmoteClient{
				tt.provider: tt.setupClient(),
			}
			handler := NewEmoteHandler(clients, tt.setupCache(), logger, nil)

			router := gin.New()
			router.GET("/emotes/:provider/:channel", handler.GetProviderEmotes)

			req, _ := http.NewRequest("GET", "/emotes/"+tt.provider+"/"+tt.channel, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatusCode, w.Code)

			var resp models.EmoteResponse
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, tt.channel, resp.Channel)
			assert.Len(t, resp.Emotes, tt.wantEmoteCount)
		})
	}
}

func TestEmoteHandler_GetChannelEmotes_WithTwitchGlobalForNonTwitchPlatform(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name            string
		channel         string
		platform        string
		twitchChannel   string
		setupClients    func() map[string]EmoteClient
		setupCache      func() EmoteCache
		wantStatusCode  int
		wantEmoteCount  int
		hasTwitchGlobal bool
	}{
		{
			name:          "YouTube channel with linked Twitch includes Twitch global + provider emotes",
			channel:       "somechannel",
			platform:      "youtube",
			twitchChannel: "linkedtwitch",
			setupClients: func() map[string]EmoteClient {
				return map[string]EmoteClient{
					"twitch": &mockEmoteClient{
						emotes: []models.Emote{
							{Code: "Kappa", URL: "https://static-cdn.jtvnw.net/emoticons/v2/25/default/dark/2.0", Provider: "twitch", Channel: "global"},
							{Code: "PogChamp", URL: "https://static-cdn.jtvnw.net/emoticons/v2/305954156/default/dark/2.0", Provider: "twitch", Channel: "global"},
						},
						provider: "twitch",
					},
					"7tv": &mockEmoteClient{
						emotes: []models.Emote{
							{Code: "OMEGALUL", URL: "https://7tv.app/1", Provider: "7tv", Channel: "somechannel"},
						},
						provider: "7tv",
					},
				}
			},
			setupCache:      func() EmoteCache { return newMockEmoteCache() },
			wantStatusCode:  http.StatusOK,
			wantEmoteCount:  5, // 2 Twitch global + 2 regular Twitch (from mock) + 1 7TV = 5 total
			hasTwitchGlobal: true,
		},
		{
			name:          "Kick channel with linked Twitch includes Twitch global + provider emotes",
			channel:       "kickstreamer",
			platform:      "kick",
			twitchChannel: "linkedtwitch",
			setupClients: func() map[string]EmoteClient {
				return map[string]EmoteClient{
					"twitch": &mockEmoteClient{
						emotes: []models.Emote{
							{Code: "Kappa", URL: "https://static-cdn.jtvnw.net/emoticons/v2/25/default/dark/2.0", Provider: "twitch", Channel: "global"},
						},
						provider: "twitch",
					},
					"bttv": &mockEmoteClient{
						emotes: []models.Emote{
							{Code: "xqcL", URL: "https://bttv.net/1", Provider: "bttv", Channel: "kickstreamer"},
						},
						provider: "bttv",
					},
				}
			},
			setupCache:      func() EmoteCache { return newMockEmoteCache() },
			wantStatusCode:  http.StatusOK,
			wantEmoteCount:  3, // 1 Twitch global + 1 regular Twitch (via linked channel) + 1 BTTV = 3 total
			hasTwitchGlobal: true,
		},
		{
			// ADR-0033 follow-up: with no linked twitch_channel, BTTV/FFZ/Twitch cannot
			// resolve a non-Twitch (YouTube) channel id, so those Twitch-keyed providers
			// are skipped entirely (no guaranteed-404 upstream calls). Only the Twitch
			// GLOBAL emotes are added for the platform.
			name:     "Non-Twitch channel without linked Twitch skips Twitch-keyed providers",
			channel:  "someYtChannel",
			platform: "youtube",
			setupClients: func() map[string]EmoteClient {
				return map[string]EmoteClient{
					"twitch": &mockEmoteClient{
						emotes: []models.Emote{
							{Code: "Kappa", URL: "https://static-cdn.jtvnw.net/emoticons/v2/25/default/dark/2.0", Provider: "twitch", Channel: "global"},
							{Code: "PogChamp", URL: "https://static-cdn.jtvnw.net/emoticons/v2/305954156/default/dark/2.0", Provider: "twitch", Channel: "global"},
						},
						provider: "twitch",
					},
					"bttv": &mockEmoteClient{
						emotes:   []models.Emote{{Code: "xqcL", URL: "https://bttv.net/1", Provider: "bttv", Channel: "someYtChannel"}},
						provider: "bttv",
					},
					"ffz": &mockEmoteClient{
						emotes:   []models.Emote{{Code: "ZULUL", URL: "https://ffz.net/1", Provider: "ffz", Channel: "someYtChannel"}},
						provider: "ffz",
					},
				}
			},
			setupCache:      func() EmoteCache { return newMockEmoteCache() },
			wantStatusCode:  http.StatusOK,
			wantEmoteCount:  2, // only the 2 Twitch GLOBAL emotes; bttv/ffz/twitch-channel skipped
			hasTwitchGlobal: true,
		},
		{
			name:     "Twitch channel does NOT duplicate global emotes",
			channel:  "twitchstreamer",
			platform: "twitch",
			setupClients: func() map[string]EmoteClient {
				return map[string]EmoteClient{
					"twitch": &mockEmoteClient{
						emotes: []models.Emote{
							{Code: "Kappa", URL: "https://static-cdn.jtvnw.net/emoticons/v2/25/default/dark/2.0", Provider: "twitch", Channel: "global"},
						},
						provider: "twitch",
					},
					"7tv": &mockEmoteClient{
						emotes: []models.Emote{
							{Code: "OMEGALUL", URL: "https://7tv.app/1", Provider: "7tv", Channel: "twitchstreamer"},
						},
						provider: "7tv",
					},
				}
			},
			setupCache:      func() EmoteCache { return newMockEmoteCache() },
			wantStatusCode:  http.StatusOK,
			wantEmoteCount:  2, // 1 Twitch + 1 7TV (no duplicate fetch of global)
			hasTwitchGlobal: false,
		},
		{
			name:     "No platform specified - no Twitch global added",
			channel:  "somechannel",
			platform: "",
			setupClients: func() map[string]EmoteClient {
				return map[string]EmoteClient{
					"twitch": &mockEmoteClient{
						emotes: []models.Emote{
							{Code: "Kappa", URL: "https://static-cdn.jtvnw.net/emoticons/v2/25/default/dark/2.0", Provider: "twitch", Channel: "somechannel"},
						},
						provider: "twitch",
					},
				}
			},
			setupCache:      func() EmoteCache { return newMockEmoteCache() },
			wantStatusCode:  http.StatusOK,
			wantEmoteCount:  1, // Just regular Twitch emotes
			hasTwitchGlobal: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			handler := NewEmoteHandler(tt.setupClients(), tt.setupCache(), logger, nil)

			router := gin.New()
			router.GET("/emotes/channel/:channel", handler.GetChannelEmotes)

			url := "/emotes/channel/" + tt.channel
			if tt.platform != "" {
				url += "?platform=" + tt.platform
			}
			if tt.twitchChannel != "" {
				sep := "?"
				if tt.platform != "" {
					sep = "&"
				}
				url += sep + "twitch_channel=" + tt.twitchChannel
			}
			req, _ := http.NewRequest("GET", url, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatusCode, w.Code)

			var resp models.EmoteResponse
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, tt.channel, resp.Channel)
			assert.Len(t, resp.Emotes, tt.wantEmoteCount)

			if tt.hasTwitchGlobal {
				// Verify at least one Twitch emote with channel="global" is present
				hasTwitchGlobal := false
				for _, emote := range resp.Emotes {
					if emote.Provider == "twitch" && emote.Channel == "global" {
						hasTwitchGlobal = true
						break
					}
				}
				assert.True(t, hasTwitchGlobal, "Expected Twitch global emotes to be present")
			}
		})
	}
}
