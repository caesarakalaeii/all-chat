package jobs

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestLifecycleSubscriber_StreamEnd verifies that when a lifecycle:stream_end
// event is received, shares with expiry_option='this_stream' for that user
// are expired atomically (EXPIRY-03).
// Wave 0: RED stub — LifecycleSubscriber does not exist yet.
func TestLifecycleSubscriber_StreamEnd(t *testing.T) {
	// RED: LifecycleSubscriber type does not exist yet.
	// Compile error gates Wave 2 implementation.
	log, _ := zap.NewDevelopment()
	_ = log

	payload := StreamEndEvent{
		Platform:      "twitch",
		UserID:        "user-uuid-123",
		BroadcasterID: "twitch-broadcaster-456",
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// LifecycleSubscriber must be constructed and its handleStreamEnd method
	// must expire this_stream shares for the given user_id.
	// Full assertion added in Wave 2.
	_ = NewLifecycleSubscriber(nil, nil, log.Sugar())
}
