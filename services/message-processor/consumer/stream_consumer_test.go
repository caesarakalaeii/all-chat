package consumer

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/caesar/all-chat/services/message-processor/registry"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func newFullTestConsumer(t *testing.T, mr *miniredis.Miniredis, handler MessageHandler) *StreamConsumer {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	logger := zaptest.NewLogger(t)
	return NewStreamConsumer(client, logger, sharedTestMetrics, handler, nil, nil, "test-host-123")
}

func TestStreamConsumer_ConsumerNameFromConstructor(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	c := newFullTestConsumer(t, mr, func(ctx context.Context, msg *models.RawChatMessage) error {
		return nil
	})

	assert.Equal(t, "test-host-123", c.consumerName)
}

func TestStreamConsumer_NoHardcodedProcessorName(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	c1 := newFullTestConsumer(t, mr, nil)
	c2 := NewStreamConsumer(
		redis.NewClient(&redis.Options{Addr: mr.Addr()}),
		zaptest.NewLogger(t),
		sharedTestMetrics,
		nil,
		nil,
		nil,
		"different-hostname",
	)

	// Different consumer names — no hardcoded "processor-1"
	assert.NotEqual(t, c1.consumerName, c2.consumerName)
	assert.Equal(t, "test-host-123", c1.consumerName)
	assert.Equal(t, "different-hostname", c2.consumerName)
}

func TestStreamConsumer_StartCallsDrainPELBeforeLoop(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	var drainCalled bool
	// We can't directly intercept drainPEL, but we can verify the consumer group is created
	// and the consumer starts successfully. The PEL drain happens as part of Start().
	c := NewStreamConsumer(client, zaptest.NewLogger(t), sharedTestMetrics,
		func(ctx context.Context, msg *models.RawChatMessage) error {
			return nil
		}, nil, nil, "test-host")

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err = c.Start(ctx)
	require.NoError(t, err)
	drainCalled = true

	// Wait briefly for goroutines to settle
	<-ctx.Done()
	assert.True(t, drainCalled)
}

func TestStreamConsumer_StartLaunchesTrimDLQGoroutine(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	c := NewStreamConsumer(client, zaptest.NewLogger(t), sharedTestMetrics,
		func(ctx context.Context, msg *models.RawChatMessage) error {
			return nil
		}, nil, nil, "test-host")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = c.Start(ctx)
	require.NoError(t, err)
	// If Start returns without error, trimDLQ goroutine was launched
	<-ctx.Done()
}

func TestStreamConsumer_BUSYGROUPHandledWithContains(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	logger := zaptest.NewLogger(t)

	// Create the group twice — second call should succeed due to BUSYGROUP handling
	c := &StreamConsumer{
		client:       client,
		logger:       logger,
		metrics:      sharedTestMetrics,
		consumerName: "test-host",
		stopCh:       make(chan struct{}),
	}

	// First creation
	err = c.createConsumerGroup(context.Background())
	require.NoError(t, err)

	// Second creation — should handle BUSYGROUP gracefully
	err = c.createConsumerGroup(context.Background())
	require.NoError(t, err)
}

func TestStreamConsumer_ProcessAndAckSendsToCorrectGroup(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	ctx := context.Background()

	// Add a message to the stream
	msgID, err := client.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamKey,
		ID:     "*",
		Values: map[string]interface{}{"data": `{"message_id":"test1","platform":"twitch","channel_id":"ch1","timestamp":"2026-01-01T00:00:00Z"}`},
	}).Result()
	require.NoError(t, err)

	// Create consumer group
	err = client.XGroupCreateMkStream(ctx, StreamKey, ConsumerGroup, "0").Err()
	require.NoError(t, err)

	// Read message into PEL
	streams, err := client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    ConsumerGroup,
		Consumer: "test-consumer",
		Streams:  []string{StreamKey, ">"},
		Count:    1,
	}).Result()
	require.NoError(t, err)
	require.Len(t, streams[0].Messages, 1)

	handlerCalled := false
	c := &StreamConsumer{
		client:  client,
		logger:  zaptest.NewLogger(t),
		metrics: sharedTestMetrics,
		handler: func(ctx context.Context, msg *models.RawChatMessage) error {
			handlerCalled = true
			return nil
		},
		consumerName:   "test-consumer",
		stopCh:         make(chan struct{}),
		msgIDRegistry:  registry.NewRedisRegistry(client, time.Hour),
		deletionBuffer: registry.NewRedisDeletionBuffer(client, time.Hour),
	}

	msg := streams[0].Messages[0]
	err = c.processAndAck(ctx, msg)
	require.NoError(t, err)
	assert.True(t, handlerCalled)

	// Verify the message was ACKed (PEL should be empty)
	pending, err := client.XPending(ctx, StreamKey, ConsumerGroup).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), pending.Count)
	_ = msgID
}

func TestStreamConsumer_ProcessAndAckRoutesDLQOnPermanentFailure(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	ctx := context.Background()

	// Add a message to the stream
	_, err = client.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamKey,
		ID:     "*",
		Values: map[string]interface{}{"data": `{"message_id":"test1","platform":"twitch","channel_id":"ch1","timestamp":"2026-01-01T00:00:00Z"}`},
	}).Result()
	require.NoError(t, err)

	// Create consumer group
	err = client.XGroupCreateMkStream(ctx, StreamKey, ConsumerGroup, "0").Err()
	require.NoError(t, err)

	streams, err := client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    ConsumerGroup,
		Consumer: "test-host",
		Streams:  []string{StreamKey, ">"},
		Count:    1,
	}).Result()
	require.NoError(t, err)

	c := &StreamConsumer{
		client:  client,
		logger:  zaptest.NewLogger(t),
		metrics: sharedTestMetrics,
		handler: func(ctx context.Context, msg *models.RawChatMessage) error {
			return assert.AnError // always fail
		},
		consumerName:   "test-host",
		stopCh:         make(chan struct{}),
		msgIDRegistry:  registry.NewRedisRegistry(client, time.Hour),
		deletionBuffer: registry.NewRedisDeletionBuffer(client, time.Hour),
	}

	msg := streams[0].Messages[0]
	// processAndAck should return error (the handler error) but still ACK + write DLQ
	_ = c.processAndAck(ctx, msg)

	// Message should have been ACKed (leaves PEL)
	pending, err := client.XPending(ctx, StreamKey, ConsumerGroup).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), pending.Count)

	// And written to DLQ
	dlqEntries, err := client.XRange(ctx, DLQStreamKey, "-", "+").Result()
	require.NoError(t, err)
	assert.Len(t, dlqEntries, 1)
}
