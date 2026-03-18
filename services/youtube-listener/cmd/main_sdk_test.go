package main

import (
	"context"
	"testing"
	"time"

	"github.com/caesar/all-chat/shared/coordination"
	"github.com/caesar/all-chat/shared/listener"
	"github.com/caesar/all-chat/shared/listener/testutil"
	"go.uber.org/goleak"
)

// mockChannelManagerForTest is a no-op stub that satisfies listener.ChannelManager.
// It is defined inline here to avoid importing streams.Manager which requires real DB/Redis.
type mockChannelManagerForTest struct{}

func (m *mockChannelManagerForTest) Start(_ context.Context) error { return nil }
func (m *mockChannelManagerForTest) Stop()                         {}
func (m *mockChannelManagerForTest) HandleMigrationEvent(_ *coordination.MigrationEvent) error {
	return nil
}
func (m *mockChannelManagerForTest) UpdateAssignedSourceIDs(_ map[string]bool) {}
func (m *mockChannelManagerForTest) GetFilteredAssignmentCount() int            { return 0 }
func (m *mockChannelManagerForTest) GetActiveChannels() []string                { return nil }
func (m *mockChannelManagerForTest) GetActiveChannelCount() int                 { return 0 }

func TestListenerBase_StartStop_NoGoroutineLeak(t *testing.T) {
	defer goleak.VerifyNone(t)

	mock := &testutil.MockCoordinator{}
	cfg := listener.ListenerConfig{
		HeartbeatInterval:         20 * time.Millisecond,
		AssignmentRefreshInterval: 20 * time.Millisecond,
		StartupJitterMax:          0,
	}
	base := listener.NewListenerBase(cfg, mock, nil, "test-pod", nil)
	mgr := &mockChannelManagerForTest{}

	ctx, cancel := context.WithCancel(context.Background())
	if err := base.Start(ctx, mgr); err != nil {
		t.Fatal(err)
	}
	cancel()
	base.Stop()
}
