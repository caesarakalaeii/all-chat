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

package clients

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

const ffzGlobalResponse = `{
	"default_sets": [3],
	"sets": {
		"3": {
			"id": 3,
			"title": "Global Emotes",
			"emoticons": [
				{"id": 240003, "name": "BeanieHipster", "urls": {"1": "https://cdn.frankerfacez.com/emote/240003/1"}},
				{"id": 240004, "name": "CoolCat", "urls": {"1": "https://cdn.frankerfacez.com/emote/240004/1"}}
			]
		}
	}
}`

func TestFFZClient_FetchEmotes(t *testing.T) {
	tests := []struct {
		name              string
		channel           string
		channelStatusCode int
		channelResponse   string
		globalStatusCode  int
		globalResponse    string
		wantEmoteCount    int
		wantErr           bool
		wantErrIs         error
		errContains       string
		wantURL           string
		wantChannelEmotes []string
		wantGlobalEmotes  []string
	}{
		{
			name:              "room emotes merge with global set",
			channel:           "xqc",
			channelStatusCode: http.StatusOK,
			channelResponse: `{
				"room": {
					"id": "123456",
					"display_name": "xQc"
				},
				"sets": {
					"123456": {
						"emoticons": [
							{"id": 1234, "name": "xqcL", "urls": {"1": "https://cdn.frankerfacez.com/emote/1234/1"}},
							{"id": 5678, "name": "xqcT", "urls": {"1": "https://cdn.frankerfacez.com/emote/5678/1"}}
						]
					}
				}
			}`,
			globalStatusCode:  http.StatusOK,
			globalResponse:    ffzGlobalResponse,
			wantEmoteCount:    4, // 2 room + 2 global
			wantChannelEmotes: []string{"xqcL", "xqcT"},
			wantGlobalEmotes:  []string{"BeanieHipster", "CoolCat"},
		},
		{
			name:              "multiple room sets merge with globals",
			channel:           "shroud",
			channelStatusCode: http.StatusOK,
			channelResponse: `{
				"room": {
					"id": "111",
					"display_name": "shroud"
				},
				"sets": {
					"111": {
						"emoticons": [
							{"id": 1111, "name": "shroudW", "urls": {"1": "https://cdn.frankerfacez.com/emote/1111/1"}}
						]
					},
					"222": {
						"emoticons": [
							{"id": 2222, "name": "shroudGG", "urls": {"1": "https://cdn.frankerfacez.com/emote/2222/1"}}
						]
					}
				}
			}`,
			globalStatusCode:  http.StatusOK,
			globalResponse:    ffzGlobalResponse,
			wantEmoteCount:    4, // 2 room + 2 global
			wantChannelEmotes: []string{"shroudW", "shroudGG"},
			wantGlobalEmotes:  []string{"BeanieHipster", "CoolCat"},
		},
		{
			name:              "channel with no FFZ room returns global set",
			channel:           "nonexistent",
			channelStatusCode: http.StatusNotFound,
			channelResponse:   `{"error": "room not found"}`,
			globalStatusCode:  http.StatusOK,
			globalResponse:    ffzGlobalResponse,
			wantEmoteCount:    2,
			wantGlobalEmotes: []string{"BeanieHipster", "CoolCat"},
		},
		{
			name:              "room with no emotes and global fetch failure propagates error",
			channel:           "emptyaccount",
			channelStatusCode: http.StatusOK,
			channelResponse: `{
				"room": {
					"id": "777",
					"display_name": "emptyaccount"
				},
				"sets": {}
			}`,
			globalStatusCode: http.StatusInternalServerError,
			wantErr:          true,
			errContains:      "failed to fetch global emotes",
		},
		{
			name:             "global channel returns only globals",
			channel:          "global",
			globalStatusCode: http.StatusOK,
			globalResponse:   ffzGlobalResponse,
			wantEmoteCount:   2,
			wantGlobalEmotes: []string{"BeanieHipster", "CoolCat"},
		},
		{
			name:              "room emotes take precedence on code collision",
			channel:           "xqc",
			channelStatusCode: http.StatusOK,
			channelResponse: `{
				"room": {
					"id": "123456",
					"display_name": "xQc"
				},
				"sets": {
					"123456": {
						"emoticons": [
							{"id": 1234, "name": "BeanieHipster", "urls": {"1": "https://cdn.frankerfacez.com/emote/1234/1"}}
						]
					}
				}
			}`,
			globalStatusCode: http.StatusOK,
			globalResponse: `{
				"default_sets": [3],
				"sets": {
					"3": {
						"emoticons": [
							{"id": 240003, "name": "BeanieHipster", "urls": {"1": "https://cdn.frankerfacez.com/emote/240003/1"}}
						]
					}
				}
			}`,
			wantEmoteCount:    1,
			wantChannelEmotes: []string{"BeanieHipster"},
			// Room emote id 1234 must win over global id 240003 — the URL is
			// what distinguishes them, both emotes share the code.
			wantURL: "https://cdn.frankerfacez.com/emote/1234/1",
		},
		{
			name:              "channel miss with global fetch failure propagates error",
			channel:           "nonexistent",
			channelStatusCode: http.StatusNotFound,
			channelResponse:   `{"error": "room not found"}`,
			globalStatusCode:  http.StatusInternalServerError,
			wantErr:           true,
			errContains:       "failed to fetch global emotes",
		},
		{
			name:              "channel miss with global 429 surfaces rate limit",
			channel:           "nonexistent",
			channelStatusCode: http.StatusNotFound,
			channelResponse:   `{"error": "room not found"}`,
			globalStatusCode:  http.StatusTooManyRequests,
			wantErr:           true,
			// The handler opens its cooldown via errors.Is, not the message.
			wantErrIs: ErrRateLimited,
		},
		{
			name:              "room emotes with global 429 returns room only",
			channel:           "xqc",
			channelStatusCode: http.StatusOK,
			channelResponse: `{
				"room": {
					"id": "123456",
					"display_name": "xQc"
				},
				"sets": {
					"123456": {
						"emoticons": [
							{"id": 1234, "name": "xqcL", "urls": {"1": "https://cdn.frankerfacez.com/emote/1234/1"}}
						]
					}
				}
			}`,
			globalStatusCode:  http.StatusTooManyRequests,
			wantEmoteCount:   1,
			wantChannelEmotes: []string{"xqcL"},
		},
		{
			name:              "global fetch failure with room emotes returns room only",
			channel:           "xqc",
			channelStatusCode: http.StatusOK,
			channelResponse: `{
				"room": {
					"id": "123456",
					"display_name": "xQc"
				},
				"sets": {
					"123456": {
						"emoticons": [
							{"id": 1234, "name": "xqcL", "urls": {"1": "https://cdn.frankerfacez.com/emote/1234/1"}}
						]
					}
				}
			}`,
			globalStatusCode:  http.StatusInternalServerError,
			wantEmoteCount:    1,
			wantChannelEmotes: []string{"xqcL"},
		},
		{
			name:              "both fetches fail",
			channel:           "xqc",
			channelStatusCode: http.StatusInternalServerError,
			channelResponse:   `{"error": "internal server error"}`,
			globalStatusCode:  http.StatusInternalServerError,
			wantErr:           true,
			errContains:       "failed to fetch emotes",
		},
		{
			name:              "invalid JSON room response",
			channel:           "xqc",
			channelStatusCode: http.StatusOK,
			channelResponse:   `{invalid json}`,
			globalStatusCode:  http.StatusOK,
			globalResponse:    ffzGlobalResponse,
			wantErr:           true,
			errContains:       "failed to decode",
		},
		{
			name:        "empty channel rejected",
			channel:     " ",
			wantErr:     true,
			errContains: "channel cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/v1/set/global"):
					w.WriteHeader(tt.globalStatusCode)
					w.Write([]byte(tt.globalResponse))
				case strings.Contains(r.URL.Path, "/v1/room/"):
					if tt.channelStatusCode == 0 {
						w.WriteHeader(http.StatusNotFound)
						return
					}
					w.WriteHeader(tt.channelStatusCode)
					w.Write([]byte(tt.channelResponse))
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			// Create client with mock server URL
			logger := zaptest.NewLogger(t)
			client := NewFFZClient(logger)
			client.baseURL = server.URL

			// Execute
			emotes, err := client.FetchEmotes(context.Background(), tt.channel)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrIs != nil {
					assert.ErrorIs(t, err, tt.wantErrIs)
				}
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			assert.Len(t, emotes, tt.wantEmoteCount)

			codes := make(map[string]bool, len(emotes))
			for _, emote := range emotes {
				assert.NotEmpty(t, emote.Code)
				assert.NotEmpty(t, emote.URL)
				assert.Equal(t, "ffz", emote.Provider)
				assert.Equal(t, tt.channel, emote.Channel)
				assert.NoError(t, emote.Validate())
				codes[emote.Code] = true
			}
			for _, code := range tt.wantChannelEmotes {
				assert.True(t, codes[code], "expected channel emote %q", code)
			}
			for _, code := range tt.wantGlobalEmotes {
				assert.True(t, codes[code], "expected global emote %q", code)
			}
			if tt.wantURL != "" {
				assert.Equal(t, tt.wantURL, emotes[0].URL)
			}
		})
	}
}

func TestFFZClient_Provider(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := NewFFZClient(logger)
	assert.Equal(t, "ffz", client.Provider())
}
