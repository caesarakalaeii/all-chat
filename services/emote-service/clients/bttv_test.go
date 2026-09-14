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

const bttvGlobalResponse = `[
	{"id": "54fa8f1401e468494b85b537", "code": ":tf:", "imageType": "png"},
	{"id": "557029e2d90919094b1a1c22", "code": "AngelThump", "imageType": "png"}
]`

func TestBTTVClient_FetchEmotes(t *testing.T) {
	tests := []struct {
		name              string
		channel           string
		channelStatusCode int
		channelResponse   string
		globalStatusCode  int
		globalResponse    string
		wantEmoteCount    int
		wantErr           bool
		errContains       string
		wantChannelEmotes []string
		wantGlobalEmotes  []string
		wantURL           string
	}{
		{
			name:              "channel emotes merge with global set",
			channel:           "xqc",
			channelStatusCode: http.StatusOK,
			channelResponse: `{
				"id": "5e4b3e186b9f0f6c6d3b9e3a",
				"channelEmotes": [
					{"id": "54fa8f1401e468494b85b537", "code": "xqcL", "imageType": "png"},
					{"id": "5e4b3e186b9f0f6c6d3b9e3a", "code": "xqcT", "imageType": "png"}
				],
				"sharedEmotes": [
					{"id": "54fa925e01e468494b85b54b", "code": "KKona", "imageType": "png"}
				]
			}`,
			globalStatusCode:  http.StatusOK,
			globalResponse:    bttvGlobalResponse,
			wantEmoteCount:    5, // 3 channel + 2 global
			wantChannelEmotes: []string{"xqcL", "xqcT", "KKona"},
			wantGlobalEmotes:  []string{":tf:", "AngelThump"},
		},
		{
			name:              "channel emotes take precedence on code collision",
			channel:           "xqc",
			channelStatusCode: http.StatusOK,
			channelResponse: `{
				"id": "5e4b3e186b9f0f6c6d3b9e3a",
				"channelEmotes": [
					{"id": "54fa8f1401e468494b85b537", "code": ":tf:", "imageType": "png"}
				],
				"sharedEmotes": []
			}`,
			globalStatusCode: http.StatusOK,
			globalResponse: `[
				{"id": "557029e2d90919094b1a1c22", "code": ":tf:", "imageType": "png"}
			]`,
			wantEmoteCount:    1,
			wantChannelEmotes: []string{":tf:"},
			// Channel emote id 54fa8f14… must win over global id 557029e2…:
			// proves the merge kept the channel emote, not just "a :tf: exists".
			wantURL: "https://cdn.betterttv.net/emote/54fa8f1401e468494b85b537/1x",
		},
		{
			name:              "channel miss with global fetch failure propagates error",
			channel:           "nonexistent",
			channelStatusCode: http.StatusNotFound,
			channelResponse:   `{"message": "user not found"}`,
			globalStatusCode:  http.StatusInternalServerError,
			wantErr:           true,
			errContains:       "failed to fetch global emotes",
		},
		{
			name:              "channel with no BTTV account returns global set",
			channel:           "nonexistent",
			channelStatusCode: http.StatusNotFound,
			channelResponse:   `{"message": "user not found"}`,
			globalStatusCode:  http.StatusOK,
			globalResponse:    bttvGlobalResponse,
			wantEmoteCount:    2,
			wantGlobalEmotes: []string{":tf:", "AngelThump"},
		},
		{
			name:              "channel with no BTTV emotes and global fetch failure propagates error",
			channel:           "emptyaccount",
			channelStatusCode: http.StatusOK,
			channelResponse:   `{"id": "5e4b3e186b9f0f6c6d3b9e3a", "channelEmotes": [], "sharedEmotes": []}`,
			globalStatusCode:  http.StatusInternalServerError,
			wantErr:           true,
			errContains:       "failed to fetch global emotes",
		},
		{
			name:             "global channel returns only globals",
			channel:          "global",
			globalStatusCode: http.StatusOK,
			globalResponse:   bttvGlobalResponse,
			wantEmoteCount:   2,
			wantGlobalEmotes: []string{":tf:", "AngelThump"},
		},
		{
			name:              "global fetch failure with channel emotes returns channel only",
			channel:           "xqc",
			channelStatusCode: http.StatusOK,
			channelResponse: `{
				"id": "5e4b3e186b9f0f6c6d3b9e3a",
				"channelEmotes": [
					{"id": "54fa8f1401e468494b85b537", "code": "xqcL", "imageType": "png"}
				],
				"sharedEmotes": []
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
			name:              "invalid JSON channel response",
			channel:           "xqc",
			channelStatusCode: http.StatusOK,
			channelResponse:   `{invalid json}`,
			globalStatusCode:  http.StatusOK,
			globalResponse:    bttvGlobalResponse,
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
				case strings.HasSuffix(r.URL.Path, "/3/cached/emotes/global"):
					w.WriteHeader(tt.globalStatusCode)
					w.Write([]byte(tt.globalResponse))
				case strings.Contains(r.URL.Path, "/3/cached/users/twitch/"):
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
			client := NewBTTVClient(logger)
			client.baseURL = server.URL

			// Execute
			emotes, err := client.FetchEmotes(context.Background(), tt.channel)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}
			require.NoError(t, err)
			assert.Len(t, emotes, tt.wantEmoteCount)
			codes := make(map[string]bool, len(emotes))
			for _, emote := range emotes {
				assert.NotEmpty(t, emote.Code)
				assert.NotEmpty(t, emote.URL)
				assert.Equal(t, "bttv", emote.Provider)
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

func TestBTTVClient_Provider(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := NewBTTVClient(logger)
	assert.Equal(t, "bttv", client.Provider())
}
