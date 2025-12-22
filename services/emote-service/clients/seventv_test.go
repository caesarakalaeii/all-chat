package clients

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/emote-service/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type mockTwitchLookup struct {
	id        string
	err       error
	called    bool
	lastLogin string
}

func (m *mockTwitchLookup) GetUserID(ctx context.Context, login string) (string, error) {
	m.called = true
	m.lastLogin = login
	if m.err != nil {
		return "", m.err
	}
	return m.id, nil
}

func TestSevenTVClient_FetchEmotes(t *testing.T) {
	channelResponse := `{
		"emote_set": {
			"id": "set-123",
			"name": "Cool Emotes",
			"emotes": [
				{
					"id": "60ae7316f7c927fad14e6ca2",
					"name": "xqcL",
					"data": {
						"host": {
							"url": "//cdn.7tv.app/emote/60ae7316f7c927fad14e6ca2",
							"files": [
								{"name": "1x.webp", "width": 28},
								{"name": "2x.webp", "width": 56}
							]
						}
					}
				}
			]
		}
	}`

	globalResponse := `{
		"id": "global-set",
		"name": "Global Emotes",
		"emotes": [
			{
				"id": "603cac391cd55c0014d989be",
				"name": "Stare",
				"data": {
					"host": {
						"url": "//cdn.7tv.app/emote/603cac391cd55c0014d989be",
						"files": [
							{"name": "1x.webp", "width": 28}
						]
					}
				}
			},
			{
				"id": "603cac391cd55c0014d989bf",
				"name": "OMEGALUL",
				"data": {
					"host": {
						"url": "//cdn.7tv.app/emote/603cac391cd55c0014d989bf",
						"files": [
							{"name": "1x.webp", "width": 28}
						]
					}
				}
			}
		]
	}`

	tests := []struct {
		name                 string
		channel              string
		mockTwitchID         string
		mockTwitchErr        error
		channelStatusCode    int
		channelResponse      string
		globalStatusCode     int
		globalResponse       string
		wantEmoteCount       int
		wantErr              bool
		errContains          string
		twitchCalled         bool
		expectChannelEmotes  bool
		expectGlobalEmotes   bool
	}{
		{
			name:                "successful fetch with channel and global emotes",
			channel:             "xqc",
			mockTwitchID:        "71092938",
			channelStatusCode:   http.StatusOK,
			channelResponse:     channelResponse,
			globalStatusCode:    http.StatusOK,
			globalResponse:      globalResponse,
			wantEmoteCount:      3, // 1 channel + 2 global
			twitchCalled:        true,
			expectChannelEmotes: true,
			expectGlobalEmotes:  true,
		},
		{
			name:                "channel already numeric",
			channel:             "12345",
			channelStatusCode:   http.StatusOK,
			channelResponse:     channelResponse,
			globalStatusCode:    http.StatusOK,
			globalResponse:      globalResponse,
			wantEmoteCount:      3, // 1 channel + 2 global
			twitchCalled:        false,
			expectChannelEmotes: true,
			expectGlobalEmotes:  true,
		},
		{
			name:               "global emotes only",
			channel:            "global",
			globalStatusCode:   http.StatusOK,
			globalResponse:     globalResponse,
			wantEmoteCount:     2, // Only global emotes
			twitchCalled:       false,
			expectGlobalEmotes: true,
		},
		{
			name:              "channel not found fails",
			channel:           "missing",
			mockTwitchID:      "9999",
			channelStatusCode: http.StatusNotFound,
			wantErr:           true,
			errContains:       "status code 404",
			twitchCalled:      true,
		},
		{
			name:            "invalid JSON response",
			channel:         "xqc",
			mockTwitchID:    "71092938",
			channelStatusCode: http.StatusOK,
			channelResponse: `{invalid json}`,
			wantErr:         true,
			errContains:     "failed to decode",
			twitchCalled:    true,
		},
		{
			name:          "twitch lookup error",
			channel:       "xqc",
			mockTwitchErr: errors.New("boom"),
			wantErr:       true,
			errContains:   "failed to resolve",
			twitchCalled:  true,
		},
		{
			name:                "channel emotes with global fetch failure",
			channel:             "xqc",
			mockTwitchID:        "71092938",
			channelStatusCode:   http.StatusOK,
			channelResponse:     channelResponse,
			globalStatusCode:    http.StatusInternalServerError,
			wantEmoteCount:      1, // Only channel emotes (global fetch failed gracefully)
			twitchCalled:        true,
			expectChannelEmotes: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestCount++
				
				// Handle global emote requests
				if strings.HasSuffix(r.URL.Path, "/v3/emote-sets/global") {
					w.WriteHeader(tt.globalStatusCode)
					if tt.globalResponse != "" {
						w.Write([]byte(tt.globalResponse))
					}
					return
				}

				// Handle channel emote requests
				if strings.Contains(r.URL.Path, "/v3/users/twitch/") {
					w.WriteHeader(tt.channelStatusCode)
					if tt.channelResponse != "" {
						w.Write([]byte(tt.channelResponse))
					}
					return
				}

				// Unknown path
				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()

			logger := zaptest.NewLogger(t)
			mockTwitch := &mockTwitchLookup{
				id:  tt.mockTwitchID,
				err: tt.mockTwitchErr,
			}

			client := NewSevenTVClient(logger, mockTwitch)
			client.baseURL = server.URL

			emotes, err := client.FetchEmotes(context.Background(), tt.channel)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Len(t, emotes, tt.wantEmoteCount)
				
				// Verify emote properties
				emoteMap := make(map[string]models.Emote)
				for _, emote := range emotes {
					assert.NotEmpty(t, emote.Code)
					assert.NotEmpty(t, emote.URL)
					assert.Equal(t, "7tv", emote.Provider)
					assert.Equal(t, tt.channel, emote.Channel)
					assert.NoError(t, emote.Validate())
					emoteMap[emote.Code] = emote
				}

				// Verify specific emotes are present if expected
				if tt.expectChannelEmotes {
					_, hasChannelEmote := emoteMap["xqcL"]
					assert.True(t, hasChannelEmote, "Expected channel emote 'xqcL' to be present")
				}
				if tt.expectGlobalEmotes {
					_, hasGlobalEmote := emoteMap["Stare"]
					assert.True(t, hasGlobalEmote, "Expected global emote 'Stare' to be present")
				}
			}

			assert.Equal(t, tt.twitchCalled, mockTwitch.called)
		})
	}
}

func TestSevenTVClient_Provider(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := NewSevenTVClient(logger, &mockTwitchLookup{id: "1"})
	assert.Equal(t, "7tv", client.Provider())
}

func TestSevenTVClient_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := zaptest.NewLogger(t)
	client := NewSevenTVClient(logger, &mockTwitchLookup{id: "1"})
	client.baseURL = server.URL
	client.httpClient.Timeout = 50 * time.Millisecond

	ctx := context.Background()
	_, err := client.FetchEmotes(ctx, "xqc")

	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "deadline") || strings.Contains(err.Error(), "Timeout"))
}
