package listener_test

import (
	"context"
	"testing"
	"time"

	"github.com/caesar/all-chat/shared/listener"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// mockChannelManager satisfies listener.ChannelManager for unit tests.
type mockChannelManager struct{}

func (m *mockChannelManager) Start(_ context.Context) error                                { return nil }
func (m *mockChannelManager) Stop()                                                        {}
func (m *mockChannelManager) UpdateAssignedSourceIDs(_ map[string]bool)                   {}
func (m *mockChannelManager) UpdateDemandedSourceIDs(_ map[string]listener.DemandedSource) {}
func (m *mockChannelManager) GetFilteredAssignmentCount() int                              { return 0 }
func (m *mockChannelManager) GetActiveChannels() []string                                  { return nil }
func (m *mockChannelManager) GetActiveChannelCount() int                                   { return 0 }

func TestLeadershipListener_StartStop(t *testing.T) {
	defer goleak.VerifyNone(t)

	ll, err := listener.NewLeadershipListener(listener.LeadershipConfig{
		DisableDemandFiltering: true,
	}, nil, nil)
	require.NoError(t, err)

	mgr := &mockChannelManager{}

	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, ll.Start(ctx, mgr))

	cancel()
	ll.Stop()
	// goleak.VerifyNone fires here — all goroutines must have exited
}

func TestLeadershipListener_StartStop_WithRedisNil(t *testing.T) {
	defer goleak.VerifyNone(t)

	// When redisClient is nil, demand loop is skipped even if filtering enabled.
	ll, err := listener.NewLeadershipListener(listener.LeadershipConfig{
		DisableDemandFiltering: false,
	}, nil, nil)
	require.NoError(t, err)

	mgr := &mockChannelManager{}

	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, ll.Start(ctx, mgr))

	cancel()
	ll.Stop()
}

func TestLeadershipListener_StartCallsMgrStart(t *testing.T) {
	defer goleak.VerifyNone(t)

	startCalled := false
	mgr := &trackingChannelManager{onStart: func() { startCalled = true }}

	ll, err := listener.NewLeadershipListener(listener.LeadershipConfig{
		DisableDemandFiltering: true,
	}, nil, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, ll.Start(ctx, mgr))

	cancel()
	ll.Stop()

	assert.True(t, startCalled, "mgr.Start should have been called by ll.Start")
}

func TestLeadershipListener_NoStartupDelay(t *testing.T) {
	defer goleak.VerifyNone(t)

	ll, err := listener.NewLeadershipListener(listener.LeadershipConfig{
		DisableDemandFiltering: true,
	}, nil, nil)
	require.NoError(t, err)

	mgr := &mockChannelManager{}

	ctx, cancel := context.WithCancel(context.Background())
	start := time.Now()
	require.NoError(t, ll.Start(ctx, mgr))
	elapsed := time.Since(start)

	// Start should return immediately (no jitter, no assignment query)
	assert.Less(t, elapsed, 50*time.Millisecond, "Start should return immediately with no blocking")

	cancel()
	ll.Stop()
}

// trackingChannelManager records Start/Stop calls for assertions.
type trackingChannelManager struct {
	mockChannelManager
	onStart func()
	onStop  func()
}

func (m *trackingChannelManager) Start(ctx context.Context) error {
	if m.onStart != nil {
		m.onStart()
	}
	return nil
}

func (m *trackingChannelManager) Stop() {
	if m.onStop != nil {
		m.onStop()
	}
}
