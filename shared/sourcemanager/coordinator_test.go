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

package sourcemanager

import (
	"context"
	"fmt"
	"sort"
	"strings"
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
	coord.stabilizationPeriod = 0 // disable stabilization window in tests

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
	coord.stabilizationPeriod = 0

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
	coord.stabilizationPeriod = 0

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
	coord.stabilizationPeriod = 0

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
	coord.stabilizationPeriod = 0

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
	coord.stabilizationPeriod = 0

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

// TestRebalance_StabilizationWindow verifies that Rebalance withholds lease
// release until the peer count has been stable for the configured period,
// preventing the release→re-acquire oscillation that occurs when pods scale.
func TestRebalance_StabilizationWindow(t *testing.T) {
	logger := zaptest.NewLogger(t)
	client := newFakeClient(1) // start with 1 peer
	coord := NewLeadershipCoordinator("twitch", client, 5*time.Second, logger)
	coord.stabilizationPeriod = 100 * time.Millisecond

	ctx := context.Background()

	// Claim 10 streams on the single pod.
	for i := 0; i < 10; i++ {
		ok, err := coord.EnsureLeadership(ctx, streamName(i), nil)
		require.NoError(t, err)
		require.True(t, ok)
	}

	// First call: peer count changes 0→1, stabilization window starts.
	released, err := coord.Rebalance(ctx, 10)
	require.NoError(t, err)
	assert.Nil(t, released, "should not release before stabilization window expires")
	assert.Equal(t, 10, coord.LeaseCount())

	// Second call within window: still no release.
	released, err = coord.Rebalance(ctx, 10)
	require.NoError(t, err)
	assert.Nil(t, released)
	assert.Equal(t, 10, coord.LeaseCount())

	// Now simulate a second pod appearing; window resets.
	client.setPeerCount(2)
	released, err = coord.Rebalance(ctx, 10)
	require.NoError(t, err)
	assert.Nil(t, released, "should not release immediately when peer count changes")

	// Wait for the stabilization window to expire.
	time.Sleep(150 * time.Millisecond)

	// Call again after window: should release excess leases (max 5 with 2 peers).
	released, err = coord.Rebalance(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, released, 5)
	assert.Equal(t, 5, coord.LeaseCount())
}

func streamName(i int) string {
	return fmt.Sprintf("stream-%03d", i)
}

// TestRebalance_SidecarLeasesMustNotShedProductionStreams pins the reason a
// background lease (a canary poller, say) belongs in its own coordinator.
//
// Rebalance derives maxPerPod from the totalStreams the caller passes, but
// compares it against the coordinator's *total* lease count. A caller that
// counts only production sources — as the stream manager does, passing
// len(sources) — therefore under-states the budget by exactly the number of
// background leases held, and Rebalance sheds real user streams to close a gap
// that does not exist.
//
// Sorting makes it worse rather than random: releases go in alphabetical order
// and "canary:" sorts ahead of most YouTube video IDs, so the background leases
// are the ones kept and the user's streams are the ones dropped.
//
// The control is the same pod with the same production load and no canary
// leases, so the only variable is where the canary leases live.
func TestRebalance_SidecarLeasesMustNotShedProductionStreams(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := context.Background()

	// Four production streams, one peer: this pod is entitled to keep them all.
	production := []string{"dQw4w9WgXcQ", "jNQXAC9IVRw", "kAbc123defg", "mXyz987hijk"}
	canaries := []string{"canary:aaa111", "canary:bbb222"}

	claimAll := func(c *LeadershipCoordinator, ids []string) {
		t.Helper()
		for _, id := range ids {
			ok, err := c.EnsureLeadership(ctx, id, nil)
			require.NoError(t, err)
			require.True(t, ok)
		}
	}

	// Control: no canary leases at all. Nothing should be shed.
	control := NewLeadershipCoordinator("youtube", newFakeClient(1), 5*time.Second, logger)
	control.stabilizationPeriod = 0
	claimAll(control, production)
	released, err := control.Rebalance(ctx, len(production))
	require.NoError(t, err)
	require.Empty(t, released,
		"control: a pod within its budget must not shed anything")

	// The bug: canary leases in the production coordinator. Same production
	// load, same peer count, same totalStreams — yet real streams get shed.
	shared := NewLeadershipCoordinator("youtube", newFakeClient(1), 5*time.Second, logger)
	shared.stabilizationPeriod = 0
	claimAll(shared, production)
	claimAll(shared, canaries)

	released, err = shared.Rebalance(ctx, len(production))
	require.NoError(t, err)

	shedProduction := []string{}
	for _, id := range released {
		if !strings.HasPrefix(id, "canary:") {
			shedProduction = append(shedProduction, id)
		}
	}
	require.NotEmpty(t, shedProduction,
		"expected the shared-coordinator arrangement to shed production streams; "+
			"if this no longer holds, Rebalance's accounting changed and this test needs rewriting")

	// The fix: the sidecar coordinates under its own platform, so its leases are
	// invisible here and the production budget is computed against production
	// leases alone — restoring the control's behaviour.
	isolated := NewLeadershipCoordinator("youtube", newFakeClient(1), 5*time.Second, logger)
	isolated.stabilizationPeriod = 0
	sidecar := NewLeadershipCoordinator("youtube-canary", newFakeClient(1), 5*time.Second, logger)
	sidecar.stabilizationPeriod = 0
	claimAll(isolated, production)
	claimAll(sidecar, canaries)

	released, err = isolated.Rebalance(ctx, len(production))
	require.NoError(t, err)
	assert.Empty(t, released, "isolated coordinator must not shed production streams")
	assert.Equal(t, len(production), isolated.LeaseCount())
	assert.Equal(t, len(canaries), sidecar.LeaseCount(),
		"canary leases must survive independently")
}
