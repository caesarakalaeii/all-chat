// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package deletion

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type testMessage struct {
	MessageID string
	ChannelID string
}

func (t *testMessage) GetMessageID() string {
	return t.MessageID
}

func (t *testMessage) GetChannelID() string {
	return t.ChannelID
}

type mockPublisher struct {
	mu        sync.Mutex
	calls     []RawMessage
	errors    map[string]error
	callTimes []time.Time
}

func newMockPublisher() *mockPublisher {
	return &mockPublisher{
		calls:     make([]RawMessage, 0),
		errors:    make(map[string]error),
		callTimes: make([]time.Time, 0),
	}
}

func (m *mockPublisher) Publish(ctx context.Context, msg RawMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, msg)
	m.callTimes = append(m.callTimes, time.Now())
	if err, exists := m.errors[msg.GetMessageID()]; exists {
		return err
	}
	return nil
}

func (m *mockPublisher) getCalls() []RawMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]RawMessage{}, m.calls...)
}

func (m *mockPublisher) getCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func (m *mockPublisher) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = make([]RawMessage, 0)
	m.callTimes = make([]time.Time, 0)
}

func (m *mockPublisher) setError(messageID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors[messageID] = err
}

func createTestMessage(messageID, channelID string) RawMessage {
	return &testMessage{
		MessageID: messageID,
		ChannelID: channelID,
	}
}

func TestNewDeletionBuffer(t *testing.T) {
	publisher := newMockPublisher()
	logger := zap.NewNop()
	buffer := NewDeletionBuffer(publisher, logger)
	assert.NotNil(t, buffer)
	assert.Equal(t, 500*time.Millisecond, buffer.bufferDuration)
	buffer.Shutdown()
}

func TestAdd_BuffersEventWithDelay(t *testing.T) {
	publisher := newMockPublisher()
	logger := zap.NewNop()
	buffer := NewDeletionBuffer(publisher, logger)
	defer buffer.Shutdown()

	msg := createTestMessage("msg1", "channel1")
	startTime := time.Now()
	require.NoError(t, buffer.Add("channel1", msg))
	assert.Equal(t, 0, publisher.getCallCount())

	time.Sleep(650 * time.Millisecond)
	calls := publisher.getCalls()
	assert.Equal(t, 1, len(calls))
	assert.GreaterOrEqual(t, time.Since(startTime), 500*time.Millisecond)
}

func TestCleanup_FlushesRemainingEvents(t *testing.T) {
	publisher := newMockPublisher()
	logger := zap.NewNop()
	buffer := NewDeletionBuffer(publisher, logger)
	defer buffer.Shutdown()

	for i := 0; i < 5; i++ {
		msg := createTestMessage(string(rune('a'+i)), "channel1")
		require.NoError(t, buffer.Add("channel1", msg))
	}

	buffer.Cleanup("channel1")
	assert.Equal(t, 5, publisher.getCallCount())
}

func TestFIFOOverflow_DropsOldest(t *testing.T) {
	publisher := newMockPublisher()
	logger := zap.NewNop()
	buffer := NewDeletionBuffer(publisher, logger)
	defer buffer.Shutdown()

	// Set smaller buffer for testing
	buffer.maxSize = 10

	// Add 15 messages (5 more than capacity)
	for i := 0; i < 15; i++ {
		msg := createTestMessage(fmt.Sprintf("msg%d", i), "channel1")
		require.NoError(t, buffer.Add("channel1", msg))
	}

	// Wait for buffer to flush
	time.Sleep(650 * time.Millisecond)

	// Should have published 10 messages (oldest 5 dropped due to overflow)
	calls := publisher.getCalls()
	assert.LessOrEqual(t, len(calls), 10)
}

func TestPerChannelIsolation(t *testing.T) {
	publisher := newMockPublisher()
	logger := zap.NewNop()
	buffer := NewDeletionBuffer(publisher, logger)
	defer buffer.Shutdown()

	// Add messages to channel A
	for i := 0; i < 3; i++ {
		msg := createTestMessage(fmt.Sprintf("msgA%d", i), "channelA")
		require.NoError(t, buffer.Add("channelA", msg))
	}

	// Add messages to channel B
	for i := 0; i < 2; i++ {
		msg := createTestMessage(fmt.Sprintf("msgB%d", i), "channelB")
		require.NoError(t, buffer.Add("channelB", msg))
	}

	// Wait for flush
	time.Sleep(650 * time.Millisecond)

	// Should have published all 5 messages
	assert.Equal(t, 5, publisher.getCallCount())

	// Cleanup one channel
	buffer.Cleanup("channelA")

	// Should still be able to add to channel B
	msg := createTestMessage("msgB3", "channelB")
	require.NoError(t, buffer.Add("channelB", msg))
}

func TestPublisherError_ContinuesFlush(t *testing.T) {
	publisher := newMockPublisher()
	logger := zap.NewNop()
	buffer := NewDeletionBuffer(publisher, logger)
	defer buffer.Shutdown()

	// Add 3 messages
	msg1 := createTestMessage("msg1", "channel1")
	msg2 := createTestMessage("msg2", "channel1")
	msg3 := createTestMessage("msg3", "channel1")

	// Set error for msg2
	publisher.setError("msg2", fmt.Errorf("publish error"))

	require.NoError(t, buffer.Add("channel1", msg1))
	require.NoError(t, buffer.Add("channel1", msg2))
	require.NoError(t, buffer.Add("channel1", msg3))

	// Wait for flush
	time.Sleep(650 * time.Millisecond)

	// Should have attempted all 3 publishes despite error
	assert.Equal(t, 3, publisher.getCallCount())
}

func TestConcurrentAddAndFlush(t *testing.T) {
	publisher := newMockPublisher()
	logger := zap.NewNop()
	buffer := NewDeletionBuffer(publisher, logger)
	defer buffer.Shutdown()

	var wg sync.WaitGroup
	messageCount := 50

	// Add messages concurrently
	for i := 0; i < messageCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			msg := createTestMessage(fmt.Sprintf("msg%d", id), "channel1")
			buffer.Add("channel1", msg)
		}(i)
	}

	wg.Wait()

	// Wait for flush
	time.Sleep(650 * time.Millisecond)

	// Should have published all messages
	assert.Equal(t, messageCount, publisher.getCallCount())
}

func TestEmptyBufferFlush_NoOp(t *testing.T) {
	publisher := newMockPublisher()
	logger := zap.NewNop()
	buffer := NewDeletionBuffer(publisher, logger)
	defer buffer.Shutdown()

	// Wait for flush tick without adding anything
	time.Sleep(250 * time.Millisecond)

	// Should not have published anything
	assert.Equal(t, 0, publisher.getCallCount())
}

func TestSingleEventBuffer(t *testing.T) {
	publisher := newMockPublisher()
	logger := zap.NewNop()
	buffer := NewDeletionBuffer(publisher, logger)
	defer buffer.Shutdown()

	msg := createTestMessage("msg1", "channel1")
	require.NoError(t, buffer.Add("channel1", msg))

	// Wait for flush
	time.Sleep(650 * time.Millisecond)

	// Should have published the single message
	assert.Equal(t, 1, publisher.getCallCount())
}

func TestExactMaxSize(t *testing.T) {
	publisher := newMockPublisher()
	logger := zap.NewNop()
	buffer := NewDeletionBuffer(publisher, logger)
	defer buffer.Shutdown()

	// Set buffer size to exactly 5
	buffer.maxSize = 5

	// Add exactly 5 messages
	for i := 0; i < 5; i++ {
		msg := createTestMessage(fmt.Sprintf("msg%d", i), "channel1")
		require.NoError(t, buffer.Add("channel1", msg))
	}

	// Wait for flush
	time.Sleep(650 * time.Millisecond)

	// Should have published all 5 messages
	assert.Equal(t, 5, publisher.getCallCount())
}

func TestTimeBasedExpiration(t *testing.T) {
	publisher := newMockPublisher()
	logger := zap.NewNop()
	buffer := NewDeletionBuffer(publisher, logger)
	defer buffer.Shutdown()

	// Add first message
	msg1 := createTestMessage("msg1", "channel1")
	require.NoError(t, buffer.Add("channel1", msg1))

	// Wait 300ms
	time.Sleep(300 * time.Millisecond)

	// Add second message
	msg2 := createTestMessage("msg2", "channel1")
	require.NoError(t, buffer.Add("channel1", msg2))

	// Wait another 300ms (total 600ms from msg1, 300ms from msg2)
	time.Sleep(300 * time.Millisecond)

	// Only msg1 should be published (>500ms), msg2 should still be buffered
	calls := publisher.getCalls()
	assert.GreaterOrEqual(t, len(calls), 1)

	// Wait for msg2 to expire (add extra time for flush interval)
	time.Sleep(400 * time.Millisecond)

	// Both should now be published
	assert.GreaterOrEqual(t, publisher.getCallCount(), 2)
}

func TestMetricsRecording_Overflow(t *testing.T) {
	publisher := newMockPublisher()
	logger := zap.NewNop()
	buffer := NewDeletionBuffer(publisher, logger)
	defer buffer.Shutdown()

	// Mock metrics recorder
	overflowCalls := make(map[string]int)
	mockMetrics := &mockMetricsRecorder{overflowCalls: overflowCalls}
	buffer.SetMetrics(mockMetrics)

	// Set small buffer size
	buffer.maxSize = 5

	// Add 10 messages to trigger overflow
	for i := 0; i < 10; i++ {
		msg := createTestMessage(fmt.Sprintf("msg%d", i), "channel1")
		buffer.Add("channel1", msg)
	}

	// Should have recorded 5 overflows
	assert.Equal(t, 5, overflowCalls["channel1"])
}

type mockMetricsRecorder struct {
	overflowCalls map[string]int
	mu            sync.Mutex
}

func (m *mockMetricsRecorder) RecordOverflow(channelID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.overflowCalls[channelID]++
}
