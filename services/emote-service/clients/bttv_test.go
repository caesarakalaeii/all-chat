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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestBTTVClient_FetchEmotes(t *testing.T) {
	tests := []struct {
		name           string
		channel        string
		mockStatusCode int
		mockResponse   string
		wantEmoteCount int
		wantErr        bool
		errContains    string
	}{
		{
			name:           "successful fetch with emotes",
			channel:        "xqc",
			mockStatusCode: http.StatusOK,
			mockResponse: `{
				"id": "5e4b3e186b9f0f6c6d3b9e3a",
				"channelEmotes": [
					{
						"id": "54fa8f1401e468494b85b537",
						"code": "xqcL",
						"imageType": "png"
					},
					{
						"id": "5e4b3e186b9f0f6c6d3b9e3a",
						"code": "xqcT",
						"imageType": "png"
					}
				],
				"sharedEmotes": [
					{
						"id": "54fa925e01e468494b85b54b",
						"code": "KKona",
						"imageType": "png"
					}
				]
			}`,
			wantEmoteCount: 3,
			wantErr:        false,
		},
		{
			name:           "only channel emotes",
			channel:        "shroud",
			mockStatusCode: http.StatusOK,
			mockResponse: `{
				"id": "5e4b3e186b9f0f6c6d3b9e3a",
				"channelEmotes": [
					{
						"id": "54fa8f1401e468494b85b537",
						"code": "shroudW",
						"imageType": "png"
					}
				],
				"sharedEmotes": []
			}`,
			wantEmoteCount: 1,
			wantErr:        false,
		},
		{
			name:           "empty emote list",
			channel:        "newstreamer",
			mockStatusCode: http.StatusOK,
			mockResponse: `{
				"id": "5e4b3e186b9f0f6c6d3b9e3a",
				"channelEmotes": [],
				"sharedEmotes": []
			}`,
			wantEmoteCount: 0,
			wantErr:        false,
		},
		{
			name:           "user not found",
			channel:        "nonexistent",
			mockStatusCode: http.StatusNotFound,
			mockResponse:   `{"message": "user not found"}`,
			wantEmoteCount: 0,
			wantErr:        true,
			errContains:    "failed to fetch emotes",
		},
		{
			name:           "server error",
			channel:        "xqc",
			mockStatusCode: http.StatusInternalServerError,
			mockResponse:   `{"error": "internal server error"}`,
			wantEmoteCount: 0,
			wantErr:        true,
			errContains:    "failed to fetch emotes",
		},
		{
			name:           "invalid JSON response",
			channel:        "xqc",
			mockStatusCode: http.StatusOK,
			mockResponse:   `{invalid json}`,
			wantEmoteCount: 0,
			wantErr:        true,
			errContains:    "failed to decode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Contains(t, r.URL.Path, "/3/cached/users/twitch/")
				w.WriteHeader(tt.mockStatusCode)
				w.Write([]byte(tt.mockResponse))
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
			} else {
				require.NoError(t, err)
				assert.Len(t, emotes, tt.wantEmoteCount)

				// Verify emote structure if we got emotes
				if tt.wantEmoteCount > 0 {
					for _, emote := range emotes {
						assert.NotEmpty(t, emote.Code)
						assert.NotEmpty(t, emote.URL)
						assert.Equal(t, "bttv", emote.Provider)
						assert.Equal(t, tt.channel, emote.Channel)
						assert.NoError(t, emote.Validate())
					}
				}
			}
		})
	}
}

func TestBTTVClient_Provider(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := NewBTTVClient(logger)
	assert.Equal(t, "bttv", client.Provider())
}
