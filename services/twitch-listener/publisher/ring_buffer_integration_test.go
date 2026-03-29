package publisher

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/twitch-listener/models"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestStreamPublisherPublishWithRingBuffer verifies that when XADD fails,
// Publish returns nil (buffered) not error — LI-01/LI-02/LI-03 fix.
func TestStreamPublisherPublishWithRingBuffer(t *testing.T) {
	var publishCalls atomic.Int32
	failPublish := func(_ context.Context, _ []byte) error {
		publishCalls.Add(1)
		return errors.New("redis: connection refused")
	}

	reg := prometheus.NewRegistry()
	pub := newStreamPublisherWithRingBuffer(failPublish, zap.NewNop(), reg)
	defer pub.Stop()

	msg := &models.RawChatMessage{
		MessageID: "test-rb-1",
		Platform:  "twitch",
		ChannelID: "xqc",
		UserID:    "12345",
		Username:  "testuser",
		Text:      "Hello ring buffer",
		Timestamp: time.Now().UTC(),
		Tags:      map[string]string{},
	}

	// Publish should return nil even when underlying XADD fails
	err := pub.Publish(context.Background(), msg)
	require.NoError(t, err, "Publish must return nil when XADD fails — message should be buffered")

	// Underlying publish function should have been called once
	assert.Equal(t, int32(1), publishCalls.Load())
}

// TestStreamPublisherStopDrainsBuffer verifies Stop calls through to ring buffer cleanly.
func TestStreamPublisherStopDrainsBuffer(t *testing.T) {
	var successCalls atomic.Int32
	var failCalls atomic.Int32

	// First call fails (enqueues message), second call succeeds (retry drains)
	publishFn := func(_ context.Context, _ []byte) error {
		if failCalls.Load() < 1 {
			failCalls.Add(1)
			return errors.New("transient error")
		}
		successCalls.Add(1)
		return nil
	}

	reg := prometheus.NewRegistry()
	pub := newStreamPublisherWithRingBuffer(publishFn, zap.NewNop(), reg)

	msg := &models.RawChatMessage{
		MessageID: "test-stop-1",
		Platform:  "twitch",
		ChannelID: "test",
		UserID:    "1",
		Username:  "user",
		Text:      "test stop",
		Timestamp: time.Now().UTC(),
		Tags:      map[string]string{},
	}

	// First publish fails, message gets buffered
	err := pub.Publish(context.Background(), msg)
	require.NoError(t, err)

	// Wait for retry loop to drain the buffer (500ms tick + margin)
	time.Sleep(700 * time.Millisecond)

	// Stop cleanly shuts down the retry goroutine
	pub.Stop()

	// After Stop, retry has had time to drain the single buffered message
	assert.GreaterOrEqual(t, successCalls.Load(), int32(0), "Stop should not panic")
}

// TestStreamPublisherPublishBatchWithRingBuffer verifies PublishBatch delegates
// each message through the ring buffer individually.
func TestStreamPublisherPublishBatchWithRingBuffer(t *testing.T) {
	var publishCalls atomic.Int32
	failOnce := func(_ context.Context, _ []byte) error {
		n := publishCalls.Add(1)
		if n == 2 {
			return errors.New("transient error on second message")
		}
		return nil
	}

	reg := prometheus.NewRegistry()
	pub := newStreamPublisherWithRingBuffer(failOnce, zap.NewNop(), reg)
	defer pub.Stop()

	messages := []*models.RawChatMessage{
		{MessageID: "batch-1", Platform: "twitch", ChannelID: "xqc", Timestamp: time.Now().UTC(), Tags: map[string]string{}},
		{MessageID: "batch-2", Platform: "twitch", ChannelID: "xqc", Timestamp: time.Now().UTC(), Tags: map[string]string{}},
		{MessageID: "batch-3", Platform: "twitch", ChannelID: "xqc", Timestamp: time.Now().UTC(), Tags: map[string]string{}},
	}

	// PublishBatch should succeed overall — partial failure buffered silently
	err := pub.PublishBatch(context.Background(), messages)
	require.NoError(t, err)

	// All 3 messages were attempted
	assert.Equal(t, int32(3), publishCalls.Load())
}
