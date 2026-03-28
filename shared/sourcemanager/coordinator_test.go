package sourcemanager

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// fakeLeadershipClient records calls and simulates leadership operations.
type fakeLeadershipClient struct {
	mu        sync.Mutex
	peerCount int
	claimed   map[string]string // streamID -> callerID
	released  []string          // streamIDs released
}

func newFakeClient(peerCount int) *fakeLeadershipClient {
	return &fakeLeadershipClient{
		peerCount: peerCount,
		claimed:   make(map[string]string),
	}
}

func (f *fakeLeadershipClient) ClaimLeadership(_ context.Context, _, streamID, callerID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.claimed[streamID]; exists {
		return false, nil
	}
	f.claimed[streamID] = callerID
	return true, nil
}

func (f *fakeLeadershipClient) RenewLeadership(_ context.Context, _, streamID, callerID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	owner, exists := f.claimed[streamID]
	if !exists {
		return false, nil
	}
	return owner == callerID, nil
}

func (f *fakeLeadershipClient) ReleaseLeadership(_ context.Context, _, streamID, callerID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.claimed[streamID] == callerID {
		delete(f.claimed, streamID)
	}
	f.released = append(f.released, streamID)
	return nil
}

func (f *fakeLeadershipClient) RegisterPeer(_ context.Context, _, _ string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.peerCount, nil
}

func (f *fakeLeadershipClient) setPeerCount(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.peerCount = n
}

func TestRebalance_ShedsExcess(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := newFakeClient(2)
	coord := NewLeadershipCoordinator("twitch", client, 5*time.Second, logger)

	ctx := context.Background()

	// Claim 10 streams (simulating pod that held all channels)
	for i := 0; i < 10; i++ {
		ok, err := coord.EnsureLeadership(ctx, streamName(i), nil)
		require.NoError(t, err)
		require.True(t, ok)
	}
	assert.Equal(t, 10, coord.LeaseCount())

	// Rebalance with 2 peers and 10 total streams → max 5 per pod
	released, err := coord.Rebalance(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, released, 5)
	assert.Equal(t, 5, coord.LeaseCount())
}

func TestRebalance_NoExcess(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := newFakeClient(2)
	coord := NewLeadershipCoordinator("twitch", client, 5*time.Second, logger)

	ctx := context.Background()

	// Claim 5 streams
	for i := 0; i < 5; i++ {
		ok, err := coord.EnsureLeadership(ctx, streamName(i), nil)
		require.NoError(t, err)
		require.True(t, ok)
	}

	// Rebalance with 2 peers and 10 total → max 5, already at 5
	released, err := coord.Rebalance(ctx, 10)
	require.NoError(t, err)
	assert.Nil(t, released)
	assert.Equal(t, 5, coord.LeaseCount())
}

func TestRebalance_ThreePods(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := newFakeClient(3)
	coord := NewLeadershipCoordinator("twitch", client, 5*time.Second, logger)

	ctx := context.Background()

	// Claim 144 streams
	for i := 0; i < 144; i++ {
		ok, err := coord.EnsureLeadership(ctx, streamName(i), nil)
		require.NoError(t, err)
		require.True(t, ok)
	}

	// Rebalance with 3 peers and 144 total → max 48 per pod
	released, err := coord.Rebalance(ctx, 144)
	require.NoError(t, err)
	assert.Len(t, released, 96)
	assert.Equal(t, 48, coord.LeaseCount())
}

func TestRebalance_SinglePod(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := newFakeClient(1)
	coord := NewLeadershipCoordinator("twitch", client, 5*time.Second, logger)

	ctx := context.Background()

	// Claim 10 streams
	for i := 0; i < 10; i++ {
		ok, err := coord.EnsureLeadership(ctx, streamName(i), nil)
		require.NoError(t, err)
		require.True(t, ok)
	}

	// Rebalance with 1 peer → max 10, no shedding
	released, err := coord.Rebalance(ctx, 10)
	require.NoError(t, err)
	assert.Nil(t, released)
	assert.Equal(t, 10, coord.LeaseCount())
}

func TestRebalance_ScaleDown(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := newFakeClient(3)
	coord := NewLeadershipCoordinator("twitch", client, 5*time.Second, logger)

	ctx := context.Background()

	// Start with 48 streams (was 3 pods, 144 total)
	for i := 0; i < 48; i++ {
		ok, err := coord.EnsureLeadership(ctx, streamName(i), nil)
		require.NoError(t, err)
		require.True(t, ok)
	}

	// Scale down to 2 pods → max 72. We have 48, well under limit.
	client.setPeerCount(2)
	released, err := coord.Rebalance(ctx, 144)
	require.NoError(t, err)
	assert.Nil(t, released)
	assert.Equal(t, 48, coord.LeaseCount())
}

func TestRebalance_ReleasesAlphabetically(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := newFakeClient(2)
	coord := NewLeadershipCoordinator("twitch", client, 5*time.Second, logger)

	ctx := context.Background()

	// Claim streams in random order
	streams := []string{"charlie", "alpha", "echo", "bravo", "delta", "foxtrot"}
	for _, s := range streams {
		ok, err := coord.EnsureLeadership(ctx, s, nil)
		require.NoError(t, err)
		require.True(t, ok)
	}

	// Rebalance: 2 peers, 6 total → max 3, release 3
	released, err := coord.Rebalance(ctx, 6)
	require.NoError(t, err)
	assert.Len(t, released, 3)

	// Should keep the first 3 alphabetically: alpha, bravo, charlie
	// Released should be: delta, echo, foxtrot
	sort.Strings(released)
	assert.Equal(t, []string{"delta", "echo", "foxtrot"}, released)

	remaining := coord.HeldStreamIDs()
	sort.Strings(remaining)
	assert.Equal(t, []string{"alpha", "bravo", "charlie"}, remaining)
}

func TestRebalance_NilCoordinator(t *testing.T) {
	var coord *LeadershipCoordinator
	released, err := coord.Rebalance(context.Background(), 10)
	assert.NoError(t, err)
	assert.Nil(t, released)
}

func TestRebalance_CeilDivision(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := newFakeClient(3)
	coord := NewLeadershipCoordinator("twitch", client, 5*time.Second, logger)

	ctx := context.Background()

	// Claim 10 streams
	for i := 0; i < 10; i++ {
		ok, err := coord.EnsureLeadership(ctx, streamName(i), nil)
		require.NoError(t, err)
		require.True(t, ok)
	}

	// 3 peers, 10 total → ceil(10/3) = 4 max per pod
	released, err := coord.Rebalance(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, released, 6)
	assert.Equal(t, 4, coord.LeaseCount())
}

func streamName(i int) string {
	return fmt.Sprintf("stream-%03d", i)
}
