package deletion

import (
	"context"
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
