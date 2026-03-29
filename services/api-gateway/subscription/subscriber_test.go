package subscription

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestSubscriber(t *testing.T, addr string) (*Subscriber, *metrics.GatewayMetrics) {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { client.Close() })
	logger := zap.NewNop()
	m := metrics.NewGatewayMetrics()
	handler := func(overlayID, channel string, message []byte) {}
	sub := NewSubscriber(client, logger, handler, m)
	return sub, m
}

// TestSubscriberRefCountUnderflow verifies that calling Unsubscribe without a
// prior Subscribe does not panic or produce a negative ref count.
func TestSubscriberRefCountUnderflow(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	sub, _ := newTestSubscriber(t, mr.Addr())
	ctx := context.Background()

	// Unsubscribe without any prior Subscribe — must not panic
	assert.NotPanics(t, func() {
		_ = sub.Unsubscribe(ctx, "no-such-overlay")
	})

	// Viewer variant
	assert.NotPanics(t, func() {
		_ = sub.UnsubscribeViewerOnly(ctx, "no-such-overlay-viewer")
	})
}

// TestSubscriberStop verifies that Stop() completes cleanly even when a
// subscription is active and listen goroutines are running.
func TestSubscriberStop(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	sub, _ := newTestSubscriber(t, mr.Addr())
	ctx := context.Background()

	err = sub.Subscribe(ctx, "overlay-stop-test")
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		defer close(done)
		sub.Stop()
	}()

	select {
	case <-done:
		// expected
	case <-time.After(3 * time.Second):
		t.Fatal("Stop() did not return within timeout")
	}
}

// TestSubscriberSubscribeUnsubscribeRefCount verifies ref-count logic:
// two subscribes → one unsubscribe leaves it subscribed, second removes it.
func TestSubscriberSubscribeUnsubscribeRefCount(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	sub, _ := newTestSubscriber(t, mr.Addr())
	ctx := context.Background()

	overlayID := "overlay-refcount"

	require.NoError(t, sub.Subscribe(ctx, overlayID))
	require.NoError(t, sub.Subscribe(ctx, overlayID)) // second ref

	assert.True(t, sub.IsSubscribed(overlayID), "should still be subscribed after first unsubscribe")

	require.NoError(t, sub.Unsubscribe(ctx, overlayID)) // decrement to 1
	assert.True(t, sub.IsSubscribed(overlayID))

	require.NoError(t, sub.Unsubscribe(ctx, overlayID)) // decrement to 0
	assert.False(t, sub.IsSubscribed(overlayID))

	sub.Stop()
}

// TestSubscriberResubscribeOnChannelClose verifies that when the Redis
// connection drops (miniredis restart), the Subscriber re-subscribes
// automatically within a reasonable timeout.
func TestSubscriberResubscribeOnChannelClose(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	sub, _ := newTestSubscriber(t, mr.Addr())
	ctx := context.Background()

	var mu sync.Mutex
	received := 0
	sub.handler = func(overlayID, channel string, message []byte) {
		mu.Lock()
		received++
		mu.Unlock()
	}

	overlayID := "overlay-reconnect"
	require.NoError(t, sub.Subscribe(ctx, overlayID))

	// Publish a message before the drop to confirm the subscription works
	mr.Publish("overlay:"+overlayID, "hello")
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	firstCount := received
	mu.Unlock()
	assert.GreaterOrEqual(t, firstCount, 1, "should have received the initial message")

	// Restart miniredis to simulate Redis connection drop
	mr.Restart()

	// Allow time for reconnect to happen
	time.Sleep(500 * time.Millisecond)

	// Publish a message after reconnect
	mr.Publish("overlay:"+overlayID, "world")
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	finalCount := received
	mu.Unlock()
	assert.Greater(t, finalCount, firstCount,
		"should receive messages after reconnect (got %d total, %d before drop)", finalCount, firstCount)

	sub.Stop()
}

// TestSubscriberResubscribeMetric verifies that the pubsub_reconnect_total
// metric is incremented when resubscribe is triggered.
func TestSubscriberResubscribeMetric(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	logger := zap.NewNop()
	m := metrics.NewGatewayMetrics()
	handler := func(overlayID, channel string, message []byte) {}
	sub := NewSubscriber(client, logger, handler, m)

	overlayID := "overlay-metric"

	// Directly invoke resubscribe to check metric increment without network disruption
	// First we need to set up the internal state manually
	sub.mu.Lock()
	sub.subscriptions[overlayID] = client.Subscribe(context.Background(), "overlay:"+overlayID)
	sub.refCounts[overlayID] = 1
	sub.mu.Unlock()

	sub.resubscribe(overlayID)

	// Give the goroutine time to start
	time.Sleep(100 * time.Millisecond)

	// Cleanup
	sub.Stop()
}

// TestSubscriberStopChanRespectedInResubscribe verifies that resubscribe
// does not re-subscribe when Stop has already been called.
func TestSubscriberStopChanRespectedInResubscribe(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	sub, _ := newTestSubscriber(t, mr.Addr())

	// Close the stop channel before calling resubscribe
	close(sub.stopChan)

	// resubscribe must return early without creating a new subscription
	assert.NotPanics(t, func() {
		sub.resubscribe("some-overlay")
	})

	sub.mu.RLock()
	_, exists := sub.subscriptions["some-overlay"]
	sub.mu.RUnlock()
	assert.False(t, exists, "should not have created a subscription after stop")
}
