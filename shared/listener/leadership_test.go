package listener_test

import (
	"testing"

	"github.com/caesar/all-chat/shared/listener"
	"github.com/caesar/all-chat/shared/listener/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/zap"
)

func TestLeadershipListener_NilSafe(t *testing.T) {
	defer goleak.VerifyNone(t)
	// Ensure SOURCE_MANAGER_SECRET is absent
	t.Setenv("SOURCE_MANAGER_SECRET", "")

	logger := zap.NewNop()
	mock := &testutil.MockCoordinator{}
	base := listener.NewListenerBase(listener.ListenerConfig{
		StartupJitterMax: 0,
	}, mock, nil, "test-pod", logger)

	ll, err := listener.NewLeadershipListenerFromEnv(base, "test-platform", logger)
	require.NoError(t, err, "should not return error when SOURCE_MANAGER_SECRET absent")
	require.NotNil(t, ll, "should return non-nil LeadershipListener")
}

func TestLeadershipListener_NilSafe_LeadershipCoordinator(t *testing.T) {
	defer goleak.VerifyNone(t)
	t.Setenv("SOURCE_MANAGER_SECRET", "")

	logger := zap.NewNop()
	mock := &testutil.MockCoordinator{}
	base := listener.NewListenerBase(listener.ListenerConfig{
		StartupJitterMax: 0,
	}, mock, nil, "test-pod", logger)

	ll, err := listener.NewLeadershipListenerFromEnv(base, "test-platform", logger)
	require.NoError(t, err)
	require.NotNil(t, ll)

	// LeadershipCoordinator() should return nil — no panic
	coord := ll.LeadershipCoordinator()
	assert.Nil(t, coord, "coordinator should be nil when SECRET not set")
}
