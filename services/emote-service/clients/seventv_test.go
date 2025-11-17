package clients

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
	successResponse := `{
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
				},
				{
					"id": "603cac391cd55c0014d989be",
					"name": "OMEGALUL",
					"data": {
						"host": {
							"url": "//cdn.7tv.app/emote/603cac391cd55c0014d989be",
							"files": [
								{"name": "1x.webp", "width": 28}
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
				"id": "60ae7316f7c927fad14e6ca2",
				"name": "xqcL",
				"data": {
					"host": {
						"url": "//cdn.7tv.app/emote/60ae7316f7c927fad14e6ca2",
						"files": [
							{"name": "1x.webp", "width": 28}
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
							{"name": "1x.webp", "width": 28}
						]
					}
				}
			}
		]
	}`

	tests := []struct {
		name           string
		channel        string
		pathAssertion  func(t *testing.T, path string)
		mockStatusCode int
		mockResponse   string
		mockTwitchID   string
		mockTwitchErr  error
		wantEmoteCount int
		wantErr        bool
		errContains    string
		twitchCalled   bool
	}{
		{
			name:    "successful fetch with twitch lookup",
			channel: "xqc",
			pathAssertion: func(t *testing.T, path string) {
				assert.Equal(t, "/v3/users/twitch/71092938", path)
			},
			mockStatusCode: http.StatusOK,
			mockResponse:   successResponse,
			mockTwitchID:   "71092938",
			wantEmoteCount: 2,
			twitchCalled:   true,
		},
		{
			name:    "channel already numeric",
			channel: "12345",
			pathAssertion: func(t *testing.T, path string) {
				assert.Equal(t, "/v3/users/twitch/12345", path)
			},
			mockStatusCode: http.StatusOK,
			mockResponse:   successResponse,
			wantEmoteCount: 2,
			twitchCalled:   false,
		},
		{
			name:    "global emotes",
			channel: "global",
			pathAssertion: func(t *testing.T, path string) {
				assert.Equal(t, "/v3/emote-sets/global", path)
			},
			mockStatusCode: http.StatusOK,
			mockResponse:   globalResponse,
			wantEmoteCount: 2,
			twitchCalled:   false,
		},
		{
			name:           "user not found",
			channel:        "missing",
			pathAssertion:  func(t *testing.T, path string) {},
			mockStatusCode: http.StatusNotFound,
			mockResponse:   `{"error": "User not found"}`,
			mockTwitchID:   "9999",
			wantErr:        true,
			errContains:    "status code 404",
			twitchCalled:   true,
		},
		{
			name:           "invalid JSON response",
			channel:        "xqc",
			pathAssertion:  func(t *testing.T, path string) {},
			mockStatusCode: http.StatusOK,
			mockResponse:   `{invalid json}`,
			mockTwitchID:   "71092938",
			wantErr:        true,
			errContains:    "failed to decode",
			twitchCalled:   true,
		},
		{
			name:          "twitch lookup error",
			channel:       "xqc",
			mockTwitchErr: errors.New("boom"),
			wantErr:       true,
			errContains:   "failed to resolve",
			twitchCalled:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.pathAssertion != nil {
					tt.pathAssertion(t, r.URL.Path)
				}
				w.WriteHeader(tt.mockStatusCode)
				if tt.mockResponse != "" {
					w.Write([]byte(tt.mockResponse))
				}
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
				for _, emote := range emotes {
					assert.NotEmpty(t, emote.Code)
					assert.NotEmpty(t, emote.URL)
					assert.Equal(t, "7tv", emote.Provider)
					assert.Equal(t, tt.channel, emote.Channel)
					assert.NoError(t, emote.Validate())
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
