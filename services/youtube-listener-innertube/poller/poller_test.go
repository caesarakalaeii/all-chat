package poller

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/caesar/all-chat/services/youtube-listener-innertube/innertube"
)

// MockClient implements a mock InnerTube client for testing
type MockClient struct {
	responses      []*innertube.LiveChatResponse
	errors         []error
	callCount      int
	continuations  []string
	pollIntervals  []time.Duration
}

func (m *MockClient) GetLiveChatReplay(ctx context.Context, continuation string, visitorData string) (*innertube.LiveChatResponse, error) {
	if m.callCount >= len(m.responses) {
		// Return last error repeatedly if out of responses
		if len(m.errors) > 0 {
			return nil, m.errors[len(m.errors)-1]
		}
		return &innertube.LiveChatResponse{}, nil
	}

	idx := m.callCount
	m.callCount++

	if idx < len(m.errors) && m.errors[idx] != nil {
		return nil, m.errors[idx]
	}

	if idx < len(m.responses) {
		return m.responses[idx], nil
	}

	return &innertube.LiveChatResponse{}, nil
}

func (m *MockClient) ExtractContinuation(resp *innertube.LiveChatResponse) string {
	if m.callCount-1 < len(m.continuations) {
		return m.continuations[m.callCount-1]
	}
	return ""
}

func (m *MockClient) GetPollInterval(resp *innertube.LiveChatResponse) time.Duration {
	if m.callCount-1 < len(m.pollIntervals) {
		return m.pollIntervals[m.callCount-1]
	}
	return 0 // let the poller use its own interval
}

func TestNewPoller(t *testing.T) {
	logger := zap.NewNop()
	client := &MockClient{}

	// Test with default options
	p := NewPoller(client, "initial-token", "channel-id", logger, nil)

	if p == nil {
		t.Fatal("NewPoller returned nil")
	}

	if p.continuation != "initial-token" {
		t.Errorf("continuation = %v, want initial-token", p.continuation)
	}

	if p.channelID != "channel-id" {
		t.Errorf("channelID = %v, want channel-id", p.channelID)
	}

	if p.interval != 2*time.Second {
		t.Errorf("interval = %v, want 2s (default)", p.interval)
	}

	if p.logLevel != "info" {
		t.Errorf("logLevel = %v, want info (default)", p.logLevel)
	}

	if p.state.GetState() != StateActive {
		t.Errorf("initial state = %v, want active", p.state.GetState())
	}

	// Test with custom options
	opts := &PollerOptions{
		Interval: 5 * time.Second,
		LogLevel: "debug",
	}
	p2 := NewPoller(client, "token2", "channel2", logger, opts)

	if p2.interval != 5*time.Second {
		t.Errorf("custom interval = %v, want 5s", p2.interval)
	}

	if p2.logLevel != "debug" {
		t.Errorf("custom logLevel = %v, want debug", p2.logLevel)
	}
}

func TestPoller_SuccessfulPolling(t *testing.T) {
	logger := zap.NewNop()

	// Mock client that returns 3 successful responses with valid continuations
	client := &MockClient{
		responses: []*innertube.LiveChatResponse{
			{
				ContinuationContents: innertube.ContinuationContents{
					LiveChatContinuation: innertube.LiveChatContinuation{
						Continuations: []innertube.Continuation{{TimedContinuationData: &innertube.TimedContinuationData{Continuation: "token-1"}}},
					},
				},
			},
			{
				ContinuationContents: innertube.ContinuationContents{
					LiveChatContinuation: innertube.LiveChatContinuation{
						Continuations: []innertube.Continuation{{TimedContinuationData: &innertube.TimedContinuationData{Continuation: "token-2"}}},
					},
				},
			},
			{
				ContinuationContents: innertube.ContinuationContents{
					LiveChatContinuation: innertube.LiveChatContinuation{
						Continuations: []innertube.Continuation{{TimedContinuationData: &innertube.TimedContinuationData{Continuation: "token-3"}}},
					},
				},
			},
		},
		errors: []error{nil, nil, nil},
		continuations: []string{
			"token-1",
			"token-2",
			"token-3",
		},
	}
	opts := &PollerOptions{
		Interval: 100 * time.Millisecond, // Fast interval for testing
		LogLevel: "info",
	}
	p := NewPoller(client, "initial-token", "test-channel", logger, opts)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := p.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for polling to occur
	time.Sleep(400 * time.Millisecond)

	// Stop poller
	p.Stop()

	// Verify state
	if p.state.GetState() != StateActive {
		t.Errorf("state = %v, want active", p.state.GetState())
	}

	// Verify continuation token was updated
	if p.continuation != "token-3" && p.continuation != "token-2" {
		// Could be token-2 or token-3 depending on timing
		t.Errorf("continuation = %v, want token-2 or token-3", p.continuation)
	}

	// Verify at least one poll occurred
	if client.callCount < 1 {
		t.Errorf("callCount = %d, want >= 1", client.callCount)
	}
}

func TestPoller_TransientError(t *testing.T) {
	logger := zap.NewNop()

	// Mock client with transient error, then success
	transientErr := &innertube.HTTPStatusError{StatusCode: 503, Body: "Service Unavailable"}
	client := &MockClient{
		responses: []*innertube.LiveChatResponse{
			nil,
			{
				ContinuationContents: innertube.ContinuationContents{
					LiveChatContinuation: innertube.LiveChatContinuation{
						Continuations: []innertube.Continuation{{TimedContinuationData: &innertube.TimedContinuationData{Continuation: "token-after-error"}}},
					},
				},
			},
		},
		errors: []error{
			transientErr,
			nil,
		},
		continuations: []string{
			"",
			"token-after-error",
		},
	}
	opts := &PollerOptions{
		Interval: 100 * time.Millisecond,
		LogLevel: "info",
	}
	p := NewPoller(client, "initial-token", "test-channel", logger, opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := p.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for backoff (first transient error should trigger ~2s backoff)
	time.Sleep(3 * time.Second)

	// Stop poller
	p.Stop()

	// Should be active after recovering from transient error
	if p.state.GetState() != StateActive {
		t.Errorf("state = %v, want active (recovered from transient error)", p.state.GetState())
	}

	// Should have retried after backoff
	if client.callCount < 2 {
		t.Errorf("callCount = %d, want >= 2 (initial + retry)", client.callCount)
	}
}

func TestPoller_FatalError(t *testing.T) {
	logger := zap.NewNop()

	// Mock client with fatal error (401 Unauthorized)
	fatalErr := &innertube.HTTPStatusError{StatusCode: 401, Body: "Unauthorized"}
	client := &MockClient{
		responses: []*innertube.LiveChatResponse{nil},
		errors:    []error{fatalErr},
	}
	opts := &PollerOptions{
		Interval: 100 * time.Millisecond,
		LogLevel: "info",
	}
	p := NewPoller(client, "initial-token", "test-channel", logger, opts)

	ctx := context.Background()

	err := p.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for fatal error to be processed
	time.Sleep(300 * time.Millisecond)

	// Should be in failed state
	if p.state.GetState() != StateFailed {
		t.Errorf("state = %v, want failed (fatal error)", p.state.GetState())
	}

	// Should have stopped polling (only 1 call)
	if client.callCount > 1 {
		t.Errorf("callCount = %d, want 1 (stopped after fatal error)", client.callCount)
	}

	// Error should be recorded
	if p.state.GetError() == nil {
		t.Error("state error = nil, want fatal error recorded")
	}

	// Stop should complete quickly
	p.Stop()
}

func TestPoller_StreamEnded(t *testing.T) {
	logger := zap.NewNop()

	// Mock client with no continuation token (stream ended)
	client := &MockClient{
		responses: []*innertube.LiveChatResponse{
			{},
		},
		errors: []error{nil},
		continuations: []string{
			"", // Empty continuation = stream ended
		},
	}
	opts := &PollerOptions{
		Interval: 100 * time.Millisecond,
		LogLevel: "info",
	}
	p := NewPoller(client, "initial-token", "test-channel", logger, opts)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := p.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for stream-ended detection
	time.Sleep(300 * time.Millisecond)

	// Should be in offline state
	if p.state.GetState() != StateOffline {
		t.Errorf("state = %v, want offline (stream ended)", p.state.GetState())
	}

	// No error should be recorded (normal stream end)
	if p.state.GetError() != nil {
		t.Errorf("state error = %v, want nil (normal stream end)", p.state.GetError())
	}

	p.Stop()
}

func TestPoller_GracefulShutdown(t *testing.T) {
	logger := zap.NewNop()

	// Mock client with infinite responses
	client := &MockClient{
		responses: make([]*innertube.LiveChatResponse, 100),
		errors:    make([]error, 100),
		continuations: func() []string {
			tokens := make([]string, 100)
			for i := range tokens {
				tokens[i] = "token"
			}
			return tokens
		}(),
	}
	opts := &PollerOptions{
		Interval: 50 * time.Millisecond, // Fast polling
		LogLevel: "info",
	}
	p := NewPoller(client, "initial-token", "test-channel", logger, opts)

	ctx := context.Background()

	err := p.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Let it poll a few times
	time.Sleep(200 * time.Millisecond)

	// Stop should complete within 1 second
	stopStart := time.Now()
	p.Stop()
	stopDuration := time.Since(stopStart)

	if stopDuration > 1*time.Second {
		t.Errorf("Stop took %v, want < 1s (graceful shutdown)", stopDuration)
	}

	// State should remain active (no error)
	if p.state.GetState() != StateActive {
		t.Errorf("state = %v, want active (graceful shutdown)", p.state.GetState())
	}
}

func TestPoller_ContextCancellation(t *testing.T) {
	logger := zap.NewNop()

	client := &MockClient{
		responses: make([]*innertube.LiveChatResponse, 100),
		errors:    make([]error, 100),
		continuations: func() []string {
			tokens := make([]string, 100)
			for i := range tokens {
				tokens[i] = "token"
			}
			return tokens
		}(),
	}
	opts := &PollerOptions{
		Interval: 50 * time.Millisecond,
		LogLevel: "info",
	}
	p := NewPoller(client, "initial-token", "test-channel", logger, opts)

	ctx, cancel := context.WithCancel(context.Background())

	err := p.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Let it poll a few times
	time.Sleep(200 * time.Millisecond)

	// Cancel context (should stop poller)
	cancel()

	// Wait for shutdown
	time.Sleep(200 * time.Millisecond)

	// Stop should return immediately (already stopped)
	stopStart := time.Now()
	p.Stop()
	stopDuration := time.Since(stopStart)

	if stopDuration > 100*time.Millisecond {
		t.Errorf("Stop took %v, want < 100ms (already stopped)", stopDuration)
	}
}

func TestState_ThreadSafety(t *testing.T) {
	s := NewState()

	// Concurrent reads and writes
	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			s.SetState(StateActive)
			s.SetState(StateFailed)
			s.SetState(StateOffline)
			s.SetError(errors.New("test error"))
			s.UpdatePollTime()
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			_ = s.GetState()
			_ = s.GetError()
			_ = s.GetLastPollTime()
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// No panic = thread-safe
}
