package listener_test

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caesar/all-chat/shared/coordination"
	"github.com/caesar/all-chat/shared/listener"
	"github.com/caesar/all-chat/shared/listener/testutil"
	"github.com/caesar/all-chat/shared/listener/testutil/redisutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/zap"
)

// demandCapturingManager records UpdateDemandedSourceIDs calls for test assertions.
type demandCapturingManager struct {
	mu              sync.Mutex
	assignedIDs     map[string]bool
	demandCalls     []map[string]listener.DemandedSource
	demandCallCount atomic.Int64
}

func (m *demandCapturingManager) Start(_ context.Context) error { return nil }
func (m *demandCapturingManager) Stop()                         {}
func (m *demandCapturingManager) HandleMigrationEvent(_ *coordination.MigrationEvent) error {
	return nil
}
func (m *demandCapturingManager) UpdateAssignedSourceIDs(ids map[string]bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.assignedIDs = ids
}
func (m *demandCapturingManager) UpdateDemandedSourceIDs(demanded map[string]listener.DemandedSource) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Store a copy so callers can inspect it.
	cp := make(map[string]listener.DemandedSource, len(demanded))
	for k, v := range demanded {
		cp[k] = v
	}
	m.demandCalls = append(m.demandCalls, cp)
	m.demandCallCount.Add(1)
}
func (m *demandCapturingManager) GetFilteredAssignmentCount() int { return 0 }
func (m *demandCapturingManager) GetActiveChannels() []string     { return nil }
func (m *demandCapturingManager) GetActiveChannelCount() int      { return 0 }

// getLastDemandCall returns the last argument passed to UpdateDemandedSourceIDs.
func (m *demandCapturingManager) getLastDemandCall() (map[string]listener.DemandedSource, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.demandCalls) == 0 {
		return nil, false
	}
	return m.demandCalls[len(m.demandCalls)-1], true
}

// demandUpdateJSON is a convenience helper that returns a JSON-encoded demand update
// matching the source:demand Pub/Sub wire format.
func demandUpdateJSON(sources []map[string]string) string {
	type src struct {
		SourceID  string `json:"source_id"`
		ChannelID string `json:"channel_id"`
		Platform  string `json:"platform"`
		OverlayID string `json:"overlay_id"`
	}
	type update struct {
		Type      string `json:"type"`
		Sources   []src  `json:"sources"`
		Timestamp string `json:"timestamp"`
	}
	var srcs []src
	for _, s := range sources {
		srcs = append(srcs, src{
			SourceID:  s["source_id"],
			ChannelID: s["channel_id"],
			Platform:  s["platform"],
			OverlayID: s["overlay_id"],
		})
	}
	b, _ := json.Marshal(update{Type: "demand_update", Sources: srcs, Timestamp: time.Now().Format(time.RFC3339)})
	return string(b)
}

// TestDemandFiltering verifies that only assigned sources that appear in the demand update
// are passed to UpdateDemandedSourceIDs (intersection logic).
func TestDemandFiltering(t *testing.T) {
	mr, rc := redisutil.StartTestRedisWithClient(t)
	defer func() {
		rc.Close()
		mr.Close()
		time.Sleep(10 * time.Millisecond) // let miniredis goroutines drain
		goleak.VerifyNone(t)
	}()

	mock := &testutil.MockCoordinator{
		Assignments: []*coordination.Assignment{
			{SourceID: "A"},
			{SourceID: "B"},
			{SourceID: "C"},
		},
	}
	cfg := listener.ListenerConfig{
		HeartbeatInterval:         50 * time.Millisecond,
		AssignmentRefreshInterval: 50 * time.Millisecond,
		StartupJitterMax:          0,
		Platform:                  "kick",
	}

	base := listener.NewListenerBase(cfg, mock, rc, "test-pod", zap.NewNop())
	mgr := &demandCapturingManager{}

	ctx, cancel := context.WithCancel(context.Background())

	require.NoError(t, base.Start(ctx, mgr))

	// Give the demand subscriber goroutine time to establish its Redis subscription.
	time.Sleep(50 * time.Millisecond)

	// Publish demand update containing A (assigned) and D (not assigned).
	// Expected intersection: {A}.
	payload := demandUpdateJSON([]map[string]string{
		{"source_id": "A", "channel_id": "chan-a", "platform": "kick", "overlay_id": "ov-1"},
		{"source_id": "D", "channel_id": "chan-d", "platform": "kick", "overlay_id": "ov-2"},
	})

	require.NoError(t, rc.Publish(ctx, "source:demand", payload).Err())

	// Wait for the demand loop to process the message.
	require.Eventually(t, func() bool {
		return mgr.demandCallCount.Load() >= 1
	}, 3*time.Second, 20*time.Millisecond, "UpdateDemandedSourceIDs should have been called")

	got, ok := mgr.getLastDemandCall()
	require.True(t, ok)

	assert.Contains(t, got, "A", "A is assigned and demanded — must be in result")
	assert.NotContains(t, got, "D", "D is not assigned — must NOT be in result")
	assert.NotContains(t, got, "B", "B is assigned but not in demand update — must NOT be in result")

	cancel()
	base.Stop()
}

// TestDemandBeforeAssignments verifies that demand updates received before
// initial assignments are loaded do NOT call UpdateDemandedSourceIDs.
func TestDemandBeforeAssignments(t *testing.T) {
	mr, rc := redisutil.StartTestRedisWithClient(t)
	defer func() {
		rc.Close()
		mr.Close()
		time.Sleep(10 * time.Millisecond)
		goleak.VerifyNone(t)
	}()

	// Coordinator that delays QueryAssignments so the demand update
	// arrives before assignments are set.
	blockingMock := &redisutil.DelayedMockCoordinator{Delay: 300 * time.Millisecond}

	cfg := listener.ListenerConfig{
		HeartbeatInterval:         50 * time.Millisecond,
		AssignmentRefreshInterval: 500 * time.Millisecond,
		StartupJitterMax:          0,
		Platform:                  "kick",
	}

	base := listener.NewListenerBase(cfg, blockingMock, rc, "test-pod", zap.NewNop())
	mgr := &demandCapturingManager{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start base in a goroutine because it will block on QueryAssignments.
	startDone := make(chan error, 1)
	go func() {
		startDone <- base.Start(ctx, mgr)
	}()

	// Wait briefly to ensure the demand loop goroutine is spawned but assignments
	// have NOT been loaded yet (blockingMock delays 300ms).
	time.Sleep(50 * time.Millisecond)

	// Publish demand update before assignments complete.
	payload := demandUpdateJSON([]map[string]string{
		{"source_id": "X", "channel_id": "chan-x", "platform": "kick", "overlay_id": "ov-1"},
	})
	require.NoError(t, rc.Publish(ctx, "source:demand", payload).Err())

	// Give a short time for the demand loop to process the early message.
	time.Sleep(100 * time.Millisecond)

	// UpdateDemandedSourceIDs must NOT have been called yet.
	assert.Equal(t, int64(0), mgr.demandCallCount.Load(),
		"UpdateDemandedSourceIDs must not be called before initial assignments are loaded")

	// Wait for base.Start to complete (after blockingMock's delay).
	require.NoError(t, <-startDone)

	cancel()
	base.Stop()
}

// TestDemandWithDisableFiltering verifies that when DisableDemandFiltering=true,
// the demand loop exits early without subscribing to Redis.
// The manager's UpdateDemandedSourceIDs must not be called via the demand loop.
func TestDemandWithDisableFiltering(t *testing.T) {
	defer goleak.VerifyNone(t)

	mock := &testutil.MockCoordinator{}
	cfg := listener.ListenerConfig{
		HeartbeatInterval:         50 * time.Millisecond,
		AssignmentRefreshInterval: 50 * time.Millisecond,
		StartupJitterMax:          0,
		DisableDemandFiltering:    true,
	}

	// nil Redis client — demand loop must handle this without panic when disabled.
	base := listener.NewListenerBase(cfg, mock, nil, "test-pod", zap.NewNop())
	mgr := &demandCapturingManager{}

	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, base.Start(ctx, mgr))

	// Give a brief window — UpdateDemandedSourceIDs must NOT be called.
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int64(0), mgr.demandCallCount.Load(),
		"DisableDemandFiltering=true: UpdateDemandedSourceIDs must NOT be called by demand loop")

	cancel()
	base.Stop()
}

// TestDemandEmptySources verifies that a DemandUpdate with an empty sources array
// calls UpdateDemandedSourceIDs with an empty map, causing all listeners to disconnect.
func TestDemandEmptySources(t *testing.T) {
	mr, rc := redisutil.StartTestRedisWithClient(t)
	defer func() {
		rc.Close()
		mr.Close()
		time.Sleep(10 * time.Millisecond)
		goleak.VerifyNone(t)
	}()

	mock := &testutil.MockCoordinator{
		Assignments: []*coordination.Assignment{
			{SourceID: "A"},
		},
	}
	cfg := listener.ListenerConfig{
		HeartbeatInterval:         50 * time.Millisecond,
		AssignmentRefreshInterval: 50 * time.Millisecond,
		StartupJitterMax:          0,
	}

	base := listener.NewListenerBase(cfg, mock, rc, "test-pod", zap.NewNop())
	mgr := &demandCapturingManager{}

	ctx, cancel := context.WithCancel(context.Background())

	require.NoError(t, base.Start(ctx, mgr))

	// Give the demand subscriber goroutine time to establish its Redis subscription.
	time.Sleep(50 * time.Millisecond)

	// Publish demand update with empty sources.
	payload := demandUpdateJSON(nil)
	require.NoError(t, rc.Publish(ctx, "source:demand", payload).Err())

	require.Eventually(t, func() bool {
		return mgr.demandCallCount.Load() >= 1
	}, 3*time.Second, 20*time.Millisecond)

	got, ok := mgr.getLastDemandCall()
	require.True(t, ok)
	assert.Empty(t, got, "empty sources must result in empty demanded map")

	cancel()
	base.Stop()
}

// TestDemandSubscriberReconnect verifies that after a simulated disconnect (context cancel +
// restart), the demand state is restored from the next DemandUpdate.
func TestDemandSubscriberReconnect(t *testing.T) {
	mr, rc := redisutil.StartTestRedisWithClient(t)
	defer func() {
		rc.Close()
		mr.Close()
		time.Sleep(10 * time.Millisecond)
		goleak.VerifyNone(t)
	}()

	mock := &testutil.MockCoordinator{
		Assignments: []*coordination.Assignment{
			{SourceID: "A"},
		},
	}
	cfg := listener.ListenerConfig{
		HeartbeatInterval:         50 * time.Millisecond,
		AssignmentRefreshInterval: 50 * time.Millisecond,
		StartupJitterMax:          0,
	}

	// First run.
	base1 := listener.NewListenerBase(cfg, mock, rc, "test-pod", zap.NewNop())
	mgr := &demandCapturingManager{}

	ctx1, cancel1 := context.WithCancel(context.Background())

	require.NoError(t, base1.Start(ctx1, mgr))

	// Give the demand subscriber goroutine time to establish its Redis subscription.
	time.Sleep(50 * time.Millisecond)

	// Publish and confirm receipt.
	payload := demandUpdateJSON([]map[string]string{
		{"source_id": "A", "channel_id": "chan-a", "platform": "", "overlay_id": "ov-1"},
	})
	require.NoError(t, rc.Publish(ctx1, "source:demand", payload).Err())

	require.Eventually(t, func() bool {
		return mgr.demandCallCount.Load() >= 1
	}, 3*time.Second, 20*time.Millisecond)

	// Simulate disconnect: stop the first base.
	cancel1()
	base1.Stop()

	prevCount := mgr.demandCallCount.Load()

	// Second run — fresh base, same manager.
	base2 := listener.NewListenerBase(cfg, mock, rc, "test-pod", zap.NewNop())
	ctx2, cancel2 := context.WithCancel(context.Background())

	require.NoError(t, base2.Start(ctx2, mgr))

	// Give the demand subscriber goroutine time to establish its Redis subscription.
	time.Sleep(50 * time.Millisecond)

	// Publish again; demand loop should process it and call UpdateDemandedSourceIDs again.
	require.NoError(t, rc.Publish(ctx2, "source:demand", payload).Err())

	require.Eventually(t, func() bool {
		return mgr.demandCallCount.Load() > prevCount
	}, 3*time.Second, 20*time.Millisecond, "demand state should be restored after reconnect")

	cancel2()
	base2.Stop()
}
