package listener

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// newTestRingBuffer creates a RingBufferPublisher with an isolated Prometheus registry
// to prevent duplicate metric registration panics across tests.
func newTestRingBuffer(t *testing.T, capacity int, publishFn PublishFunc) *RingBufferPublisher {
	t.Helper()
	reg := prometheus.NewRegistry()
	return NewRingBufferPublisherWithRegisterer(capacity, publishFn, zap.NewNop(), "test-service", reg)
}

// TestRingBufferPublishSuccess verifies that when publishFn succeeds, messages are
// not buffered (buffer remains empty).
func TestRingBufferPublishSuccess(t *testing.T) {
	called := 0
	publishFn := func(ctx context.Context, payload []byte) error {
		called++
		return nil
	}

	rb := newTestRingBuffer(t, 100, publishFn)
	defer rb.Stop()

	err := rb.Publish(context.Background(), []byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, 1, called)
	assert.Equal(t, 0, rb.getSize(), "buffer should be empty when publish succeeds")
}

// TestRingBufferPublishFailureBuffers verifies that when publishFn fails, the message
// is enqueued in the ring buffer and Publish returns nil (caller is unaware of the failure).
func TestRingBufferPublishFailureBuffers(t *testing.T) {
	publishFn := func(ctx context.Context, payload []byte) error {
		return errors.New("redis unavailable")
	}

	rb := newTestRingBuffer(t, 100, publishFn)
	defer rb.Stop()

	err := rb.Publish(context.Background(), []byte("hello"))
	require.NoError(t, err, "Publish should return nil even when publishFn fails")
	assert.Equal(t, 1, rb.getSize(), "message should be buffered")
}

// TestRingBufferRetryDrains verifies that when publishFn starts succeeding, the retry
// goroutine drains the buffer within one tick interval.
func TestRingBufferRetryDrains(t *testing.T) {
	var failNext atomic.Bool
	failNext.Store(true)

	publishFn := func(ctx context.Context, payload []byte) error {
		if failNext.Load() {
			return errors.New("redis unavailable")
		}
		return nil
	}

	rb := newTestRingBuffer(t, 100, publishFn)
	defer rb.Stop()

	// Enqueue a message by having publishFn fail
	err := rb.Publish(context.Background(), []byte("hello"))
	require.NoError(t, err)
	require.Equal(t, 1, rb.getSize(), "message should be buffered before retry")

	// Allow publishFn to succeed
	failNext.Store(false)

	// Wait for retry tick (500ms interval + buffer)
	time.Sleep(600 * time.Millisecond)

	assert.Equal(t, 0, rb.getSize(), "buffer should be empty after successful retry")
}

// TestRingBufferCapacityOverflow verifies that when the buffer is full, the oldest
// message is dropped and drops_total is incremented.
func TestRingBufferCapacityOverflow(t *testing.T) {
	publishFn := func(ctx context.Context, payload []byte) error {
		return errors.New("redis unavailable")
	}

	capacity := 1000
	rb := newTestRingBuffer(t, capacity, publishFn)
	defer rb.Stop()

	// Enqueue capacity+1 messages
	for i := 0; i < capacity+1; i++ {
		err := rb.Publish(context.Background(), []byte("msg"))
		require.NoError(t, err)
	}

	assert.Equal(t, capacity, rb.getSize(), "buffer size should be capped at capacity")
	assert.Equal(t, int64(1), rb.getDropsTotal(), "exactly 1 message should have been dropped")
}

// TestRingBufferStopCleansUp verifies that Stop() causes the retry goroutine to exit
// and wg.Wait() returns promptly.
func TestRingBufferStopCleansUp(t *testing.T) {
	publishFn := func(ctx context.Context, payload []byte) error {
		return errors.New("redis unavailable")
	}

	rb := newTestRingBuffer(t, 100, publishFn)

	// Enqueue some items
	for i := 0; i < 5; i++ {
		_ = rb.Publish(context.Background(), []byte("msg"))
	}

	done := make(chan struct{})
	go func() {
		rb.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Stop returned promptly
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not return within 2 seconds")
	}
}

// TestRingBufferRetryUsesBackgroundContext verifies that the retry goroutine passes
// context.Background() to publishFn, not the original publish context.
func TestRingBufferRetryUsesBackgroundContext(t *testing.T) {
	var capturedCtx context.Context
	var retryCallCount atomic.Int32

	publishFn := func(ctx context.Context, payload []byte) error {
		count := retryCallCount.Add(1)
		if count == 1 {
			// First call (from Publish) — fail to trigger buffering
			return errors.New("redis unavailable")
		}
		// Subsequent calls (from retryLoop)
		capturedCtx = ctx
		return nil
	}

	rb := newTestRingBuffer(t, 100, publishFn)
	defer rb.Stop()

	// Use a cancellable context to publish
	ctx, cancel := context.WithCancel(context.Background())
	err := rb.Publish(ctx, []byte("hello"))
	require.NoError(t, err)

	// Cancel the original context
	cancel()

	// Wait for the retry goroutine to attempt at least once more
	time.Sleep(600 * time.Millisecond)

	require.NotNil(t, capturedCtx, "retry goroutine should have called publishFn")
	assert.Equal(t, context.Background(), capturedCtx, "retry should use context.Background(), not the original cancelled context")
}

// TestRingBuffer_OverflowLog verifies that enqueue at capacity emits an Error-level
// log with sentinel message "ring_buffer_overflow_drop" and required zap fields.
func TestRingBuffer_OverflowLog(t *testing.T) {
	publishFn := func(ctx context.Context, payload []byte) error {
		return errors.New("redis unavailable")
	}

	// Build an observed zap logger so we can inspect emitted log entries.
	core, logs := observer.New(zapcore.ErrorLevel)
	logger := zap.New(core)

	reg := prometheus.NewRegistry()
	rb := NewRingBufferPublisherWithRegisterer(1, publishFn, logger, "test-service", reg)
	defer rb.Stop()

	// Fill the buffer (capacity=1, first message enqueues OK).
	_ = rb.Publish(context.Background(), []byte("first"))
	// Second publish triggers overflow drop.
	_ = rb.Publish(context.Background(), []byte("second"))

	entries := logs.All()
	require.Len(t, entries, 1, "expected exactly one Error log on overflow")

	entry := entries[0]
	assert.Equal(t, zapcore.ErrorLevel, entry.Level, "overflow log must be Error level")
	assert.Equal(t, "ring_buffer_overflow_drop", entry.Message, "overflow log must use sentinel message")

	fieldMap := make(map[string]interface{})
	for _, f := range entry.Context {
		fieldMap[f.Key] = f.Integer
		if f.Type == zapcore.StringType {
			fieldMap[f.Key] = f.String
		}
	}
	assert.Contains(t, fieldMap, "service", "overflow log must include 'service' field")
	assert.Contains(t, fieldMap, "capacity", "overflow log must include 'capacity' field")
	assert.Contains(t, fieldMap, "current_depth", "overflow log must include 'current_depth' field")
}

// TestRingBufferMultipleMessages verifies correct FIFO ordering within the ring buffer.
func TestRingBufferMultipleMessages(t *testing.T) {
	received := make([]string, 0)
	var callCount atomic.Int32

	publishFn := func(ctx context.Context, payload []byte) error {
		count := callCount.Add(1)
		if count <= 3 {
			// First 3 calls (initial Publish attempts) — fail to buffer
			return errors.New("redis unavailable")
		}
		// Retry calls succeed — capture the order
		received = append(received, string(payload))
		return nil
	}

	rb := newTestRingBuffer(t, 100, publishFn)
	defer rb.Stop()

	_ = rb.Publish(context.Background(), []byte("first"))
	_ = rb.Publish(context.Background(), []byte("second"))
	_ = rb.Publish(context.Background(), []byte("third"))

	require.Equal(t, 3, rb.getSize())

	// Wait for retry loop to drain
	time.Sleep(1200 * time.Millisecond)

	assert.Equal(t, 0, rb.getSize(), "buffer should be empty after retries")
	assert.Equal(t, []string{"first", "second", "third"}, received, "messages should be retried in FIFO order")
}
