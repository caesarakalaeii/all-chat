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

func TestFFZClient_FetchEmotes(t *testing.T) {
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
				"room": {
					"id": "123456",
					"display_name": "xQc"
				},
				"sets": {
					"123456": {
						"emoticons": [
							{
								"id": 1234,
								"name": "xqcL",
								"urls": {
									"1": "https://cdn.frankerfacez.com/emote/1234/1"
								}
							},
							{
								"id": 5678,
								"name": "xqcT",
								"urls": {
									"1": "https://cdn.frankerfacez.com/emote/5678/1"
								}
							}
						]
					}
				}
			}`,
			wantEmoteCount: 2,
			wantErr:        false,
		},
		{
			name:           "multiple sets",
			channel:        "shroud",
			mockStatusCode: http.StatusOK,
			mockResponse: `{
				"room": {
					"id": "111",
					"display_name": "shroud"
				},
				"sets": {
					"111": {
						"emoticons": [
							{
								"id": 1111,
								"name": "shroudW",
								"urls": {
									"1": "https://cdn.frankerfacez.com/emote/1111/1"
								}
							}
						]
					},
					"222": {
						"emoticons": [
							{
								"id": 2222,
								"name": "shroudGG",
								"urls": {
									"1": "https://cdn.frankerfacez.com/emote/2222/1"
								}
							}
						]
					}
				}
			}`,
			wantEmoteCount: 2,
			wantErr:        false,
		},
		{
			name:           "empty emote list",
			channel:        "newstreamer",
			mockStatusCode: http.StatusOK,
			mockResponse: `{
				"room": {
					"id": "999",
					"display_name": "newstreamer"
				},
				"sets": {}
			}`,
			wantEmoteCount: 0,
			wantErr:        false,
		},
		{
			name:           "room not found",
			channel:        "nonexistent",
			mockStatusCode: http.StatusNotFound,
			mockResponse:   `{"error": "room not found"}`,
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
				assert.Contains(t, r.URL.Path, "/v1/room/")
				w.WriteHeader(tt.mockStatusCode)
				w.Write([]byte(tt.mockResponse))
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
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				assert.Len(t, emotes, tt.wantEmoteCount)

				// Verify emote structure if we got emotes
				if tt.wantEmoteCount > 0 {
					for _, emote := range emotes {
						assert.NotEmpty(t, emote.Code)
						assert.NotEmpty(t, emote.URL)
						assert.Equal(t, "ffz", emote.Provider)
						assert.Equal(t, tt.channel, emote.Channel)
						assert.NoError(t, emote.Validate())
					}
				}
			}
		})
	}
}

func TestFFZClient_Provider(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := NewFFZClient(logger)
	assert.Equal(t, "ffz", client.Provider())
}
