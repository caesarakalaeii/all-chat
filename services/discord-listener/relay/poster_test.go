package relay

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatRelayContent(t *testing.T) {
	result := formatRelayContent("twitch", "alice", "hello")
	assert.Equal(t, "🟣 alice: hello", result)
}

func TestFormatRelayContent_UnknownPlatform(t *testing.T) {
	result := formatRelayContent("kick2", "alice", "hello")
	assert.Equal(t, "💬 alice: hello", result)
}

func TestHTTPPoster_UsesCorrectChannelID(t *testing.T) {
	channelID := "987654321"
	var capturedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	poster := &httpPoster{
		token:   "testtoken",
		client:  server.Client(),
		baseURL: server.URL,
	}

	err := poster.Post(context.Background(), channelID, "🟣 alice: hello")
	require.NoError(t, err)
	assert.True(t, strings.Contains(capturedPath, channelID),
		"expected path to contain channel ID %q, got %q", channelID, capturedPath)
}

func TestHTTPPoster_429RetriesOnce(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Retry-After", "0.01")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	poster := &httpPoster{
		token:   "testtoken",
		client:  server.Client(),
		baseURL: server.URL,
	}

	err := poster.Post(context.Background(), "123456789", "content")
	require.NoError(t, err)
	assert.Equal(t, 2, callCount, "expected exactly 2 HTTP calls (original + one retry)")
}

func TestHTTPPoster_403Drops(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	poster := &httpPoster{
		token:   "testtoken",
		client:  server.Client(),
		baseURL: server.URL,
	}

	err := poster.Post(context.Background(), "123456789", "content")
	assert.NoError(t, err, "403 should return nil error (silent drop)")
}
