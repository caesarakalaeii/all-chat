package enricher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"go.uber.org/zap"
)

func TestHTTPEmoteClient_GetEmotesForChannel_EscapesChannelID(t *testing.T) {
	maliciousChannel := "../evil channel"
	var requestedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"channel":"test","emotes":[]}`))
	}))
	defer server.Close()

	client := NewHTTPEmoteClient(server.URL, zap.NewNop())

	if _, err := client.GetEmotesForChannel(context.Background(), maliciousChannel); err != nil {
		t.Fatalf("GetEmotesForChannel returned error: %v", err)
	}

	expectedPath := "/emotes/channel/" + url.PathEscape(maliciousChannel)
	if requestedPath != expectedPath {
		t.Fatalf("expected path %q, got %q", expectedPath, requestedPath)
	}
}
