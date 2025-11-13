package clients

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestSevenTVClient_FetchEmotes(t *testing.T) {
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
				"user": {
					"username": "xqc",
					"connections": [
						{
							"platform": "TWITCH",
							"id": "71092938"
						}
					]
				},
				"emote_set": {
					"emotes": [
						{
							"id": "60ae7316f7c927fad14e6ca2",
							"name": "xqcL",
							"data": {
								"host": {
									"url": "//cdn.7tv.app/emote/60ae7316f7c927fad14e6ca2",
									"files": [
										{"name": "1x.webp"}
									]
								}
							}
						},
						{
							"id": "603cac391cd55c0014d989be",
							"name": "OMEGALUL",
							"data": {
								"host": {
									"url": "//cdn.7tv.app/emote/603cac391cd55c0014d989be",
									"files": [
										{"name": "1x.webp"}
									]
								}
							}
						}
					]
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
				"user": {
					"username": "newstreamer"
				},
				"emote_set": {
					"emotes": []
				}
			}`,
			wantEmoteCount: 0,
			wantErr:        false,
		},
		{
			name:           "user not found",
			channel:        "nonexistent",
			mockStatusCode: http.StatusNotFound,
			mockResponse:   `{"error": "User not found"}`,
			wantEmoteCount: 0,
			wantErr:        true,
			errContains:    "failed to fetch emotes",
		},
		{
			name:           "server error",
			channel:        "xqc",
			mockStatusCode: http.StatusInternalServerError,
			mockResponse:   `{"error": "Internal server error"}`,
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
				assert.Contains(t, r.URL.Path, "/v3/users/twitch/")
				w.WriteHeader(tt.mockStatusCode)
				w.Write([]byte(tt.mockResponse))
			}))
			defer server.Close()

			// Create client with mock server URL
			logger := zaptest.NewLogger(t)
			client := NewSevenTVClient(logger)
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
						assert.Equal(t, "7tv", emote.Provider)
						assert.Equal(t, tt.channel, emote.Channel)
						assert.NoError(t, emote.Validate())
					}
				}
			}
		})
	}
}

func TestSevenTVClient_Provider(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := NewSevenTVClient(logger)
	assert.Equal(t, "7tv", client.Provider())
}

func TestSevenTVClient_Timeout(t *testing.T) {
	// Create server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Delay longer than client timeout
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := zaptest.NewLogger(t)
	client := NewSevenTVClient(logger)
	client.baseURL = server.URL
	client.httpClient.Timeout = 50 * time.Millisecond // Short timeout for test

	ctx := context.Background()
	_, err := client.FetchEmotes(ctx, "xqc")

	require.Error(t, err)
	// Error can be "context deadline exceeded" or "Client.Timeout exceeded"
	assert.True(t, strings.Contains(err.Error(), "deadline") || strings.Contains(err.Error(), "Timeout"))
}
