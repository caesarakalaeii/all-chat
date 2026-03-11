package eventsub

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestSubscribeToStreamOffline verifies that SubscribeToStreamOffline can
// be called on the SubscriptionManager (EXPIRY-02).
// Wave 0: RED stub — method does not exist yet.
func TestSubscribeToStreamOffline(t *testing.T) {
	// RED: SubscribeToStreamOffline method does not exist yet.
	log, _ := zap.NewDevelopment()
	sm := NewSubscriptionManager("client-id", "client-secret", "webhook-secret", "https://example.com/callback", log)
	require.NotNil(t, sm)

	// This will fail to compile until Wave 2 adds the method.
	_, err := sm.SubscribeToStreamOffline(context.Background(), "broadcaster-123")
	// We expect an error (no real Twitch API in test), but the call must exist.
	_ = err
}
