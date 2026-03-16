package publisher_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/discord-listener/publisher"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// xAddCapture is a minimal redis.Cmdable stub that captures XAdd arguments.
type xAddCapture struct {
	redis.Cmdable
	capturedStream string
	capturedValues map[string]interface{}
}

func (x *xAddCapture) XAdd(_ context.Context, args *redis.XAddArgs) *redis.StringCmd {
	x.capturedStream = args.Stream
	x.capturedValues = make(map[string]interface{})
	for k, v := range args.Values.(map[string]interface{}) {
		x.capturedValues[k] = v
	}
	return redis.NewStringCmd(context.Background())
}

// TestPublish_HappyPath verifies that Publish calls XAdd on "chat:raw" with a "data" key only.
func TestPublish_HappyPath(t *testing.T) {
	cap := &xAddCapture{}
	log := zap.NewNop()

	pub := publisher.NewStreamPublisherFromCmdable(cap, log)

	msg := &publisher.RawMessage{
		MessageID:   "msg-1",
		Platform:    "discord",
		OverlayID:   "overlay-1",
		ChannelID:   "channel-1",
		ChannelName: "channel-1",
		UserID:      "user-1",
		Username:    "test-user",
		Text:        "hello",
		Timestamp:   time.Now(),
	}

	err := pub.Publish(context.Background(), msg)
	require.NoError(t, err)

	assert.Equal(t, "chat:raw", cap.capturedStream, "expected stream key 'chat:raw'")
	require.Contains(t, cap.capturedValues, "data", "expected 'data' field in XAdd values")

	// Verify only "data" field is present (not any other fields)
	assert.Len(t, cap.capturedValues, 1, "expected exactly one field in XAdd values")

	// Verify the data field contains valid JSON with the message content
	dataStr, ok := cap.capturedValues["data"].(string)
	require.True(t, ok, "expected data field to be a string")

	var decoded publisher.RawMessage
	err = json.Unmarshal([]byte(dataStr), &decoded)
	require.NoError(t, err)
	assert.Equal(t, "discord", decoded.Platform)
	assert.Equal(t, "overlay-1", decoded.OverlayID)
}
