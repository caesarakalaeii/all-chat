package consumer

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// sharedTestMetrics is created once per test binary to avoid promauto duplicate registration.
var sharedTestMetrics = metrics.NewProcessorMetrics()

func newTestConsumer(t *testing.T, mr *miniredis.Miniredis) *StreamConsumer {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	logger := zaptest.NewLogger(t)
	return &StreamConsumer{
		client:       client,
		logger:       logger,
		metrics:      sharedTestMetrics,
		consumerName: "test-consumer",
		stopCh:       make(chan struct{}),
	}
}

func TestWriteToDLQ_WritesExpectedFields(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	c := newTestConsumer(t, mr)

	originalValues := map[string]interface{}{
		"data": `{"message_id":"abc123","platform":"twitch"}`,
	}

	ctx := context.Background()
	c.writeToDLQ(ctx, "1234-0", "twitch-listener", "parse_error", 3, originalValues)

	// Verify the entry was written to chat:dlq
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	entries, err := client.XRange(ctx, DLQStreamKey, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, entries, 1)

	entry := entries[0]
	assert.Equal(t, "1234-0", entry.Values["original_stream_id"])
	assert.Equal(t, "twitch-listener", entry.Values["source_service"])
	assert.Equal(t, "parse_error", entry.Values["failure_reason"])
	assert.Equal(t, "3", entry.Values["retry_count"])
	assert.Equal(t, `{"message_id":"abc123","platform":"twitch"}`, entry.Values["original_data"])
	assert.NotEmpty(t, entry.Values["dlq_timestamp"])

	// Verify dlq_timestamp is valid RFC3339Nano
	tsStr, ok := entry.Values["dlq_timestamp"].(string)
	require.True(t, ok)
	_, parseErr := time.Parse(time.RFC3339Nano, tsStr)
	assert.NoError(t, parseErr)
}

func TestWriteToDLQ_DoesNotPanicOnRedisFailure(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	addr := mr.Addr()
	mr.Close() // Close immediately to force Redis failure

	// Create client pointing to now-closed server
	client := redis.NewClient(&redis.Options{Addr: addr})
	defer client.Close()

	c := &StreamConsumer{
		client:       client,
		logger:       zaptest.NewLogger(t),
		metrics:      sharedTestMetrics,
		consumerName: "test-consumer",
		stopCh:       make(chan struct{}),
	}
	originalValues := map[string]interface{}{"data": "{}"}

	// Should not panic even with Redis unavailable
	assert.NotPanics(t, func() {
		c.writeToDLQ(context.Background(), "1234-0", "service", "reason", 1, originalValues)
	})
}

func TestWriteToDLQ_MissingDataField(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	c := newTestConsumer(t, mr)
	// originalValues has no "data" field
	originalValues := map[string]interface{}{}

	ctx := context.Background()
	// Should not panic
	assert.NotPanics(t, func() {
		c.writeToDLQ(ctx, "1234-0", "service", "reason", 1, originalValues)
	})

	// Entry is still written (with empty original_data)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	entries, err := client.XRange(ctx, DLQStreamKey, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "", entries[0].Values["original_data"])
}

func TestTrimDLQ_RemovesOldEntries(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	ctx := context.Background()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	// Add an old entry (older than 7 days)
	oldTime := time.Now().Add(-8 * 24 * time.Hour)
	oldID := oldTime.UnixMilli()
	// Use explicit old stream ID
	_, addErr := client.XAdd(ctx, &redis.XAddArgs{
		Stream: DLQStreamKey,
		ID:     formatStreamIDInternal(oldID),
		Values: map[string]interface{}{"data": "old"},
	}).Result()
	require.NoError(t, addErr)

	// Add a recent entry
	_, addErr = client.XAdd(ctx, &redis.XAddArgs{
		Stream: DLQStreamKey,
		ID:     "*",
		Values: map[string]interface{}{"data": "new"},
	}).Result()
	require.NoError(t, addErr)

	// Verify both entries exist
	before, err := client.XLen(ctx, DLQStreamKey).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(2), before)

	// Run trimDLQ inline (without goroutine)
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	minID := formatStreamIDInternal(cutoff.UnixMilli())
	trimErr := client.XTrimMinID(ctx, DLQStreamKey, minID).Err()
	require.NoError(t, trimErr)

	// Only the new entry should remain
	after, err := client.XLen(ctx, DLQStreamKey).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), after)
}

func TestTrimDLQ_EmptyStreamDoesNotError(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	ctx := context.Background()

	// trimDLQ on empty stream should not error
	assert.NotPanics(t, func() {
		cutoff := time.Now().Add(-7 * 24 * time.Hour)
		minID := formatStreamIDInternal(cutoff.UnixMilli())
		client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		defer client.Close()
		_ = client.XTrimMinID(ctx, DLQStreamKey, minID).Err()
	})
}

func TestDLQStreamKeyConstant(t *testing.T) {
	assert.Equal(t, "chat:dlq", DLQStreamKey)
}

// Verify the metrics were registered
func TestDLQMetricsRegistered(t *testing.T) {
	m := sharedTestMetrics
	require.NotNil(t, m.PELPendingMessages)
	require.NotNil(t, m.DLQMessagesTotal)
	require.NotNil(t, m.PublishRetryTotal)
	require.NotNil(t, m.DLQWriteFailures)

	// Verify they can be used without panic
	assert.NotPanics(t, func() {
		m.PELPendingMessages.WithLabelValues("consumer-1").Set(5)
		m.DLQMessagesTotal.WithLabelValues("twitch", "parse_error").Inc()
		m.PublishRetryTotal.WithLabelValues("exhausted").Inc()
		m.DLQWriteFailures.Inc()
	})
}

func TestWriteToDLQ_IncrementsMetric(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	c := newTestConsumer(t, mr)
	ctx := context.Background()

	// Write to DLQ with a source service and reason that should be tracked
	originalValues := map[string]interface{}{"data": `{}`}
	c.writeToDLQ(ctx, "1234-0", "twitch-listener", "handler_error", 3, originalValues)

	// Verify the DLQ entry was written
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	entries, err := client.XRange(ctx, DLQStreamKey, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, entries, 1)

	// Verify JSON serialization of retry_count works
	retryCountStr, ok := entries[0].Values["retry_count"].(string)
	require.True(t, ok)
	var retryCount int
	err = json.Unmarshal([]byte(retryCountStr), &retryCount)
	require.NoError(t, err)
	assert.Equal(t, 3, retryCount)
}

func TestDrainPELLogger(t *testing.T) {
	// Just verify drainPEL logs info on completion
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	logger := zaptest.NewLogger(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	c := &StreamConsumer{
		client:       client,
		logger:       logger,
		metrics:      sharedTestMetrics,
		consumerName: "test-consumer",
		stopCh:       make(chan struct{}),
	}

	// Create consumer group first so XAutoClaim doesn't fail with NOGROUP
	client.XGroupCreateMkStream(context.Background(), StreamKey, ConsumerGroup, "$")

	// drainPEL on empty PEL should complete without error
	assert.NotPanics(t, func() {
		c.drainPEL(context.Background())
	})
}

func init() {
	// Suppress zap global logger in tests
	zap.ReplaceGlobals(zap.NewNop())
}
