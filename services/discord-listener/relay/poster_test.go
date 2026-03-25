package relay

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatWebhookUsername(t *testing.T) {
	result := formatWebhookUsername("alice", "twitch")
	assert.Equal(t, "alice [Twitch]", result)
}

func TestFormatWebhookUsername_EmptyDisplayName(t *testing.T) {
	// When display_name is empty, caller passes username instead.
	// The function itself just formats whatever it gets.
	result := formatWebhookUsername("bob", "twitch")
	assert.Equal(t, "bob [Twitch]", result)
}

func TestFormatWebhookUsername_PlatformTitleCased(t *testing.T) {
	tests := []struct {
		platform string
		expected string
	}{
		{"twitch", "alice [Twitch]"},
		{"youtube", "alice [Youtube]"},
		{"kick", "alice [Kick]"},
		{"tiktok", "alice [Tiktok]"},
	}
	for _, tt := range tests {
		t.Run(tt.platform, func(t *testing.T) {
			result := formatWebhookUsername("alice", tt.platform)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWebhookPoster_PostsToCorrectURL(t *testing.T) {
	var capturedBody map[string]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		// Webhook requests must NOT have an Authorization header
		assert.Empty(t, r.Header.Get("Authorization"))

		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&capturedBody)
		require.NoError(t, err)

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	poster := NewWebhookPoster(server.Client(), nil)
	// Override baseURL for test
	poster.(*webhookPoster).baseURL = server.URL

	payload := RelayPayload{
		Content:   "hello world",
		Username:  "alice [Twitch]",
		AvatarURL: "https://example.com/avatar.png",
	}

	err := poster.Post(context.Background(), server.URL+"/api/webhooks/123/token", payload)
	require.NoError(t, err)

	assert.Equal(t, "hello world", capturedBody["content"])
	assert.Equal(t, "alice [Twitch]", capturedBody["username"])
	assert.Equal(t, "https://example.com/avatar.png", capturedBody["avatar_url"])
}

func TestWebhookPoster_OmitsEmptyAvatarURL(t *testing.T) {
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&capturedBody)
		require.NoError(t, err)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	poster := NewWebhookPoster(server.Client(), nil)
	poster.(*webhookPoster).baseURL = server.URL

	payload := RelayPayload{
		Content:  "hello",
		Username: "alice [Twitch]",
		// AvatarURL intentionally empty
	}

	err := poster.Post(context.Background(), server.URL+"/api/webhooks/123/token", payload)
	require.NoError(t, err)

	_, hasAvatar := capturedBody["avatar_url"]
	assert.False(t, hasAvatar, "avatar_url should be omitted when empty")
}

func TestWebhookPoster_204Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	poster := NewWebhookPoster(server.Client(), nil)
	poster.(*webhookPoster).baseURL = server.URL

	err := poster.Post(context.Background(), server.URL+"/webhook", RelayPayload{Content: "test", Username: "u"})
	assert.NoError(t, err)
}

func TestWebhookPoster_200Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	poster := NewWebhookPoster(server.Client(), nil)
	poster.(*webhookPoster).baseURL = server.URL

	err := poster.Post(context.Background(), server.URL+"/webhook", RelayPayload{Content: "test", Username: "u"})
	assert.NoError(t, err)
}

func TestWebhookPoster_429RetriesOnce(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Retry-After", "0.01")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	poster := NewWebhookPoster(server.Client(), nil)
	poster.(*webhookPoster).baseURL = server.URL

	err := poster.Post(context.Background(), server.URL+"/webhook", RelayPayload{Content: "test", Username: "u"})
	require.NoError(t, err)
	assert.Equal(t, 2, callCount, "expected exactly 2 HTTP calls (original + one retry)")
}

func TestWebhookPoster_403SilentDrop(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	poster := NewWebhookPoster(server.Client(), nil)
	poster.(*webhookPoster).baseURL = server.URL

	err := poster.Post(context.Background(), server.URL+"/webhook", RelayPayload{Content: "test", Username: "u"})
	assert.NoError(t, err, "403 should return nil error (silent drop)")
}

func TestWebhookPoster_404SilentDrop(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	poster := NewWebhookPoster(server.Client(), nil)
	poster.(*webhookPoster).baseURL = server.URL

	err := poster.Post(context.Background(), server.URL+"/webhook", RelayPayload{Content: "test", Username: "u"})
	assert.NoError(t, err, "404 should return nil error (silent drop)")
}
