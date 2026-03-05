package deletion

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener-innertube/innertube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mockPublisher implements Publisher interface for testing
type mockPublisher struct {
	mu        sync.Mutex
	calls     []*innertube.RawChatMessage
	errors    map[string]error // messageID -> error to return
	callTimes []time.Time      // Track when each publish was called
}

func newMockPublisher() *mockPublisher {
	return &mockPublisher{
		calls:     make([]*innertube.RawChatMessage, 0),
		errors:    make(map[string]error),
		callTimes: make([]time.Time, 0),
	}
}

func (m *mockPublisher) Publish(ctx context.Context, msg *innertube.RawChatMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls = append(m.calls, msg)
	m.callTimes = append(m.callTimes, time.Now())

	if err, exists := m.errors[msg.MessageID]; exists {
		return err
	}
	return nil
}

func (m *mockPublisher) getCalls() []*innertube.RawChatMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]*innertube.RawChatMessage{}, m.calls...)
}

func (m *mockPublisher) getCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func (m *mockPublisher) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = make([]*innertube.RawChatMessage, 0)
	m.callTimes = make([]time.Time, 0)
}

func (m *mockPublisher) setError(messageID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors[messageID] = err
}

// createTestMessage creates a test deletion message
func createTestMessage(messageID, channelID string) *innertube.RawChatMessage {
	return &innertube.RawChatMessage{
		MessageID: messageID,
		Platform:  "youtube",
		ChannelID: channelID,
		UserID:    "user123",
		Username:  "testuser",
		Text:      "deleted message",
		EventType: "message_deletion",
		Timestamp: time.Now(),
	}
}

func TestNewDeletionBuffer(t *testing.T) {
	publisher := newMockPublisher()
	logger := zap.NewNop()

	buffer := NewDeletionBuffer(publisher, logger)

	assert.NotNil(t, buffer)
	assert.Equal(t, 500*time.Millisecond, buffer.bufferDuration)
	assert.Equal(t, 1000, buffer.maxSize)
	assert.Equal(t, 100*time.Millisecond, buffer.flushInterval)
	assert.NotNil(t, buffer.channels)
	assert.NotNil(t, buffer.publisher)
	assert.NotNil(t, buffer.logger)

	buffer.Shutdown()
}

func TestAdd_CreatesChannelBufferLazily(t *testing.T) {
	publisher := newMockPublisher()
	logger := zap.NewNop()
	buffer := NewDeletionBuffer(publisher, logger)
	defer buffer.Shutdown()

	channelID := "channel1"
	msg := createTestMessage("msg1", channelID)

	err := buffer.Add(channelID, msg)
	require.NoError(t, err)

	// Verify channel buffer was created
	buffer.mu.RLock()
	cb, exists := buffer.channels[channelID]
	buffer.mu.RUnlock()

	assert.True(t, exists)
	assert.NotNil(t, cb)
	assert.NotNil(t, cb.ring)
	assert.NotNil(t, cb.ticker)
	assert.Equal(t, 1, cb.count)
}

func TestAdd_BuffersEventWithDelay(t *testing.T) {
	publisher := newMockPublisher()
	logger := zap.NewNop()
	buffer := NewDeletionBuffer(publisher, logger)
	defer buffer.Shutdown()

	channelID := "channel1"
	msg := createTestMessage("msg1", channelID)

	startTime := time.Now()
	err := buffer.Add(channelID, msg)
	require.NoError(t, err)

	// Verify not published immediately
	assert.Equal(t, 0, publisher.getCallCount())

	// Wait for buffer duration + flush interval + margin
	time.Sleep(650 * time.Millisecond)

	// Verify published after delay
	calls := publisher.getCalls()
	assert.Equal(t, 1, len(calls))
	assert.Equal(t, "msg1", calls[0].MessageID)

	// Verify delay was at least 500ms
	elapsed := time.Since(startTime)
	assert.GreaterOrEqual(t, elapsed, 500*time.Millisecond)
}

func TestAdd_FIFOOverflow(t *testing.T) {
	publisher := newMockPublisher()
	logger := zap.NewNop()
	buffer := NewDeletionBuffer(publisher, logger)
	defer buffer.Shutdown()

	// Use smaller buffer for test
	buffer.maxSize = 10
	channelID := "channel1"

	// Add 11 events to trigger overflow
	for i := 0; i < 11; i++ {
		msg := createTestMessage(string(rune('a'+i)), channelID)
		err := buffer.Add(channelID, msg)
		require.NoError(t, err)
	}

	// Wait for flush
	time.Sleep(650 * time.Millisecond)

	// Verify only 10 events published (oldest dropped)
	calls := publisher.getCalls()
	assert.Equal(t, 10, len(calls))

	// Verify first event (oldest) was dropped, newest retained
	messageIDs := make([]string, len(calls))
	for i, call := range calls {
		messageIDs[i] = call.MessageID
	}

	// Should have messages b-k (a was dropped as oldest)
	assert.NotContains(t, messageIDs, "a")
	assert.Contains(t, messageIDs, "b") // Second oldest should be present
	assert.Contains(t, messageIDs, "k") // Newest should be present
}

func TestAdd_PerChannelIsolation(t *testing.T) {
	publisher := newMockPublisher()
	logger := zap.NewNop()
	buffer := NewDeletionBuffer(publisher, logger)
	defer buffer.Shutdown()

	// Add events to different channels
	msg1 := createTestMessage("msg1", "channel1")
	msg2 := createTestMessage("msg2", "channel2")

	err := buffer.Add("channel1", msg1)
	require.NoError(t, err)

	err = buffer.Add("channel2", msg2)
	require.NoError(t, err)

	// Verify separate buffers created
	buffer.mu.RLock()
	assert.Len(t, buffer.channels, 2)
	assert.NotNil(t, buffer.channels["channel1"])
	assert.NotNil(t, buffer.channels["channel2"])
	buffer.mu.RUnlock()

	// Wait for flush
	time.Sleep(650 * time.Millisecond)

	// Verify both published independently
	calls := publisher.getCalls()
	assert.Equal(t, 2, len(calls))
}

func TestCleanup_FlushesRemainingEvents(t *testing.T) {
	publisher := newMockPublisher()
	logger := zap.NewNop()
	buffer := NewDeletionBuffer(publisher, logger)
	defer buffer.Shutdown()

	channelID := "channel1"

	// Add events
	for i := 0; i < 5; i++ {
		msg := createTestMessage(string(rune('a'+i)), channelID)
		err := buffer.Add(channelID, msg)
		require.NoError(t, err)
	}

	// Cleanup immediately (before 500ms delay)
	buffer.Cleanup(channelID)

	// Verify all events were flushed
	calls := publisher.getCalls()
	assert.Equal(t, 5, len(calls))

	// Verify buffer removed
	buffer.mu.RLock()
	_, exists := buffer.channels[channelID]
	buffer.mu.RUnlock()
	assert.False(t, exists)
}

func TestCleanup_StopsFlusher(t *testing.T) {
	publisher := newMockPublisher()
	logger := zap.NewNop()
	buffer := NewDeletionBuffer(publisher, logger)
	defer buffer.Shutdown()

	channelID := "channel1"
	msg := createTestMessage("msg1", channelID)

	err := buffer.Add(channelID, msg)
	require.NoError(t, err)

	// Cleanup
	buffer.Cleanup(channelID)

	// Wait to ensure no more flushes happen
	publisher.reset()
	time.Sleep(200 * time.Millisecond)

	// Verify no additional flushes occurred
	assert.Equal(t, 0, publisher.getCallCount())
}

func TestPublisherError_ContinuesFlushing(t *testing.T) {
	publisher := newMockPublisher()
	logger := zap.NewNop()
	buffer := NewDeletionBuffer(publisher, logger)
	defer buffer.Shutdown()

	channelID := "channel1"

	// Add 3 events, make middle one fail
	msg1 := createTestMessage("msg1", channelID)
	msg2 := createTestMessage("msg2", channelID)
	msg3 := createTestMessage("msg3", channelID)

	publisher.setError("msg2", assert.AnError)

	err := buffer.Add(channelID, msg1)
	require.NoError(t, err)
	err = buffer.Add(channelID, msg2)
	require.NoError(t, err)
	err = buffer.Add(channelID, msg3)
	require.NoError(t, err)

	// Wait for flush
	time.Sleep(650 * time.Millisecond)

	// Verify all 3 were attempted (error doesn't stop flushing)
	calls := publisher.getCalls()
	assert.Equal(t, 3, len(calls))
}

func TestConcurrentAdd_ThreadSafe(t *testing.T) {
	publisher := newMockPublisher()
	logger := zap.NewNop()
	buffer := NewDeletionBuffer(publisher, logger)
	defer buffer.Shutdown()

	channelID := "channel1"
	concurrency := 10
	eventsPerGoroutine := 10

	var wg sync.WaitGroup
	wg.Add(concurrency)

	// Concurrent adds
	for i := 0; i < concurrency; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				msgID := string(rune('a'+goroutineID)) + string(rune('0'+j))
				msg := createTestMessage(msgID, channelID)
				err := buffer.Add(channelID, msg)
				require.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()

	// Wait for flush
	time.Sleep(650 * time.Millisecond)

	// Verify no panics and all published (or dropped if buffer overflow)
	calls := publisher.getCalls()
	assert.LessOrEqual(t, len(calls), concurrency*eventsPerGoroutine)
	assert.Greater(t, len(calls), 0)
}

func TestFlushExpired_OnlyFlushesOldEvents(t *testing.T) {
	publisher := newMockPublisher()
	logger := zap.NewNop()
	buffer := NewDeletionBuffer(publisher, logger)
	defer buffer.Shutdown()

	channelID := "channel1"

	// Add first event
	msg1 := createTestMessage("msg1", channelID)
	err := buffer.Add(channelID, msg1)
	require.NoError(t, err)

	// Wait 300ms (not expired yet)
	time.Sleep(300 * time.Millisecond)

	// Add second event
	msg2 := createTestMessage("msg2", channelID)
	err = buffer.Add(channelID, msg2)
	require.NoError(t, err)

	// Wait another 300ms (total 600ms from msg1, 300ms from msg2)
	time.Sleep(300 * time.Millisecond)

	// Only msg1 should be flushed
	calls := publisher.getCalls()
	assert.Equal(t, 1, len(calls))
	assert.Equal(t, "msg1", calls[0].MessageID)

	publisher.reset()

	// Wait another 300ms for msg2 to expire
	time.Sleep(300 * time.Millisecond)

	// Now msg2 should be flushed
	calls = publisher.getCalls()
	assert.Equal(t, 1, len(calls))
	assert.Equal(t, "msg2", calls[0].MessageID)
}

func TestEmptyBufferFlush_NoOp(t *testing.T) {
	publisher := newMockPublisher()
	logger := zap.NewNop()
	buffer := NewDeletionBuffer(publisher, logger)
	defer buffer.Shutdown()

	channelID := "channel1"

	// Create empty buffer by adding then cleaning up
	msg := createTestMessage("msg1", channelID)
	err := buffer.Add(channelID, msg)
	require.NoError(t, err)

	// Wait for flush
	time.Sleep(650 * time.Millisecond)

	// Verify flushed
	assert.Equal(t, 1, publisher.getCallCount())

	publisher.reset()

	// Wait for another flush cycle (should be no-op)
	time.Sleep(200 * time.Millisecond)

	// Verify no additional publishes
	assert.Equal(t, 0, publisher.getCallCount())
}

func TestSingleEventBuffer(t *testing.T) {
	publisher := newMockPublisher()
	logger := zap.NewNop()
	buffer := NewDeletionBuffer(publisher, logger)
	defer buffer.Shutdown()

	buffer.maxSize = 1 // Single event buffer
	channelID := "channel1"

	msg := createTestMessage("msg1", channelID)
	err := buffer.Add(channelID, msg)
	require.NoError(t, err)

	// Wait for flush
	time.Sleep(650 * time.Millisecond)

	calls := publisher.getCalls()
	assert.Equal(t, 1, len(calls))
	assert.Equal(t, "msg1", calls[0].MessageID)
}

func TestExactlyMaxSizeBuffer(t *testing.T) {
	publisher := newMockPublisher()
	logger := zap.NewNop()
	buffer := NewDeletionBuffer(publisher, logger)
	defer buffer.Shutdown()

	buffer.maxSize = 5
	channelID := "channel1"

	// Add exactly maxSize events
	for i := 0; i < 5; i++ {
		msg := createTestMessage(string(rune('a'+i)), channelID)
		err := buffer.Add(channelID, msg)
		require.NoError(t, err)
	}

	// Wait for flush
	time.Sleep(650 * time.Millisecond)

	// All should be published
	calls := publisher.getCalls()
	assert.Equal(t, 5, len(calls))
}

func TestShutdown_FlushesAllChannels(t *testing.T) {
	publisher := newMockPublisher()
	logger := zap.NewNop()
	buffer := NewDeletionBuffer(publisher, logger)

	// Add events to multiple channels
	for i := 0; i < 3; i++ {
		channelID := string(rune('A' + i))
		msg := createTestMessage("msg"+channelID, channelID)
		err := buffer.Add(channelID, msg)
		require.NoError(t, err)
	}

	// Shutdown immediately (before delay)
	buffer.Shutdown()

	// Verify all events flushed
	calls := publisher.getCalls()
	assert.Equal(t, 3, len(calls))

	// Verify all channels cleaned up
	buffer.mu.RLock()
	assert.Equal(t, 0, len(buffer.channels))
	buffer.mu.RUnlock()
}

func TestMultipleChannels_IndependentBuffers(t *testing.T) {
	publisher := newMockPublisher()
	logger := zap.NewNop()
	buffer := NewDeletionBuffer(publisher, logger)
	defer buffer.Shutdown()

	// Add to channel1
	msg1 := createTestMessage("msg1", "channel1")
	err := buffer.Add("channel1", msg1)
	require.NoError(t, err)

	// Wait 200ms
	time.Sleep(200 * time.Millisecond)

	// Add to channel2
	msg2 := createTestMessage("msg2", "channel2")
	err = buffer.Add("channel2", msg2)
	require.NoError(t, err)

	// Wait 400ms (total 600ms for channel1, 400ms for channel2)
	time.Sleep(400 * time.Millisecond)

	// Only channel1 should have flushed
	calls := publisher.getCalls()
	assert.Equal(t, 1, len(calls))
	assert.Equal(t, "msg1", calls[0].MessageID)

	publisher.reset()

	// Wait another 200ms for channel2
	time.Sleep(200 * time.Millisecond)

	// Now channel2 should flush
	calls = publisher.getCalls()
	assert.Equal(t, 1, len(calls))
	assert.Equal(t, "msg2", calls[0].MessageID)
}
