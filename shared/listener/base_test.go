package listener_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caesar/all-chat/shared/coordination"
	"github.com/caesar/all-chat/shared/listener"
	"github.com/caesar/all-chat/shared/listener/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/zap"
)

// mockChannelManager satisfies listener.ChannelManager for unit tests.
type mockChannelManager struct{}

func (m *mockChannelManager) Start(_ context.Context) error                                     { return nil }
func (m *mockChannelManager) Stop()                                                              {}
func (m *mockChannelManager) HandleMigrationEvent(_ *coordination.MigrationEvent) error         { return nil }
func (m *mockChannelManager) UpdateAssignedSourceIDs(_ map[string]bool)                         {}
func (m *mockChannelManager) UpdateDemandedSourceIDs(_ map[string]listener.DemandedSource)      {}
func (m *mockChannelManager) GetFilteredAssignmentCount() int                                    { return 0 }
func (m *mockChannelManager) GetActiveChannels() []string                                        { return nil }
func (m *mockChannelManager) GetActiveChannelCount() int                                         { return 0 }

func testConfig() listener.ListenerConfig {
	return listener.ListenerConfig{
		HeartbeatInterval:         20 * time.Millisecond,
		AssignmentRefreshInterval: 20 * time.Millisecond,
		StartupJitterMax:          0, // no jitter in tests
	}
}

func TestListenerBase_StartStop(t *testing.T) {
	defer goleak.VerifyNone(t)
	logger := zap.NewNop()
	mock := &testutil.MockCoordinator{}
	base := listener.NewListenerBase(testConfig(), mock, nil, "test-pod", logger)
	mgr := &mockChannelManager{}

	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, base.Start(ctx, mgr))

	cancel()
	base.Stop()
	// goleak.VerifyNone fires here — all goroutines must have exited
}

func TestListenerBase_HeartbeatFires(t *testing.T) {
	defer goleak.VerifyNone(t)
	logger := zap.NewNop()
	mock := &testutil.MockCoordinator{}
	base := listener.NewListenerBase(testConfig(), mock, nil, "test-pod", logger)
	mgr := &mockChannelManager{}

	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, base.Start(ctx, mgr))

	// Wait for at least one heartbeat tick (interval is 20ms, wait 200ms)
	time.Sleep(200 * time.Millisecond)
	assert.Greater(t, mock.HeartbeatCallCount(), int64(0), "heartbeat should have fired at least once")

	cancel()
	base.Stop()
}

func TestListenerBase_AssignmentRefreshFires(t *testing.T) {
	defer goleak.VerifyNone(t)
	logger := zap.NewNop()
	mock := &testutil.MockCoordinator{}
	base := listener.NewListenerBase(testConfig(), mock, nil, "test-pod", logger)
	mgr := &mockChannelManager{}

	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, base.Start(ctx, mgr))

	// Initial assignment query + at least one refresh tick
	time.Sleep(200 * time.Millisecond)
	assert.GreaterOrEqual(t, mock.AssignmentCallCount(), int64(2),
		"should have initial query + at least one refresh")

	cancel()
	base.Stop()
}

func TestListenerBase_NoJitter(t *testing.T) {
	defer goleak.VerifyNone(t)
	logger := zap.NewNop()
	mock := &testutil.MockCoordinator{}
	cfg := testConfig()
	cfg.StartupJitterMax = 0 // explicit zero — no jitter
	base := listener.NewListenerBase(cfg, mock, nil, "test-pod", logger)
	mgr := &mockChannelManager{}

	ctx, cancel := context.WithCancel(context.Background())
	start := time.Now()
	require.NoError(t, base.Start(ctx, mgr))
	elapsed := time.Since(start)

	// Start should return in well under 50ms with zero jitter
	assert.Less(t, elapsed, 50*time.Millisecond, "Start should not sleep when jitter is 0")

	cancel()
	base.Stop()
}

func TestListenerBase_OnFatalError(t *testing.T) {
	defer goleak.VerifyNone(t)
	logger := zap.NewNop()
	mock := &testutil.MockCoordinator{ShouldFailHeartbeat: true}

	var callbackSource atomic.Value
	cfg := testConfig()
	cfg.HeartbeatInterval = 10 * time.Millisecond
	cfg.OnFatalError = func(source string, err error) {
		callbackSource.Store(source)
	}

	base := listener.NewListenerBase(cfg, mock, nil, "test-pod", logger)
	mgr := &mockChannelManager{}

	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, base.Start(ctx, mgr))

	// Wait for heartbeat failure to trigger callback
	time.Sleep(200 * time.Millisecond)
	stored := callbackSource.Load()
	assert.NotNil(t, stored, "OnFatalError should have been called")
	if stored != nil {
		assert.Equal(t, "heartbeat", stored.(string))
	}

	cancel()
	base.Stop()
}
