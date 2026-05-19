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

package channels

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/twitch-listener/models"
	"github.com/caesar/all-chat/shared/sourcemanager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// MockJoinParter implements JoinParterInterface for testing
type MockJoinParter struct {
	mu          sync.Mutex
	joined      []string
	departed    []string
	joinCalls   int
	departCalls int
	// bannedChannels simulates the IRC-layer ban list: Join() returns false
	// for any channel in this set, mirroring the production short-circuit
	// path. Tests that exercise the activeChans bookkeeping gate (Join
	// returning false must NOT mark a channel as active) populate this map.
	bannedChannels map[string]bool
}

func NewMockJoinParter() *MockJoinParter {
	return &MockJoinParter{
		joined:         make([]string, 0),
		departed:       make([]string, 0),
		bannedChannels: make(map[string]bool),
	}
}

// SetBanned marks channel as banned in the mock IRC layer. Subsequent Join
// calls for this channel return false (the wire JOIN is short-circuited),
// matching the production isJoinBanned path.
func (m *MockJoinParter) SetBanned(channel string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bannedChannels[channel] = true
}

func (m *MockJoinParter) Join(channel string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.joinCalls++
	if m.bannedChannels[channel] {
		// Banned channels do not record a "joined" entry — production Join
		// short-circuits before client.Join, so the wire never sees the JOIN.
		return false
	}
	m.joined = append(m.joined, channel)
	return true
}

func (m *MockJoinParter) Depart(channel string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.departed = append(m.departed, channel)
	m.departCalls++
}

func (m *MockJoinParter) GetJoined() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.joined))
	copy(result, m.joined)
	return result
}

func (m *MockJoinParter) GetDeparted() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.departed))
	copy(result, m.departed)
	return result
}

func (m *MockJoinParter) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.joined = make([]string, 0)
	m.departed = make([]string, 0)
	m.joinCalls = 0
	m.departCalls = 0
}

func (m *MockJoinParter) GetJoinCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.joinCalls
}

func (m *MockJoinParter) GetDepartCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.departCalls
}

// MockRepository implements RepositoryInterface for testing
type MockRepository struct {
	channels []string
	err      error
}

func (m *MockRepository) GetActiveChannels(ctx context.Context) ([]models.ChannelSource, error) {
	// Not used in these tests, but required by interface
	return nil, nil
}

func (m *MockRepository) GetUniqueChannels(ctx context.Context) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.channels, nil
}

func (m *MockRepository) SetSourceActive(ctx context.Context, channelName string, isActive bool) error {
	// Not used in these tests, but required by interface
	return nil
}

func (m *MockRepository) GetSourceIDsForChannels(ctx context.Context, channels []string) map[string]string {
	// Return a simple map for testing - channel name maps to itself as source ID
	result := make(map[string]string)
	for _, ch := range channels {
		result[ch] = ch + "-source-id"
	}
	return result
}

func (m *MockRepository) GetOverlayIDsForChannel(ctx context.Context, channelName string) ([]string, error) {
	// Return a test overlay ID for cross-platform event testing
	return []string{"test-overlay-" + channelName}, nil
}

func TestManager_SyncChannels_InitialJoin(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	repo := &MockRepository{
		channels: []string{"xqc", "summit1g", "shroud"},
	}

	mockJP := NewMockJoinParter()
	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)

	err := manager.SyncChannels(ctx)
	require.NoError(t, err)

	// Verify all channels were joined
	joined := mockJP.GetJoined()
	assert.Len(t, joined, 3)
	assert.Contains(t, joined, "xqc")
	assert.Contains(t, joined, "summit1g")
	assert.Contains(t, joined, "shroud")

	// Verify manager tracks them as active
	assert.Equal(t, 3, manager.GetActiveChannelCount())
	assert.True(t, manager.IsChannelActive("xqc"))
	assert.True(t, manager.IsChannelActive("summit1g"))
	assert.True(t, manager.IsChannelActive("shroud"))
}

// Display-name strings (e.g. "شوشو") sometimes leak into
// overlay_chat_sources.channel_name. Twitch silently drops JOINs for these,
// which used to age out in pendingJoins and force a full IRC reconnect every
// ~4 minutes. SyncChannels must filter them out before issuing any JOIN.
func TestManager_SyncChannels_FiltersInvalidLogins(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	repo := &MockRepository{
		channels: []string{"caedrel", "شوشو", "一代鹹魚", "summit1g", "hello-world"},
	}

	mockJP := NewMockJoinParter()
	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)

	require.NoError(t, manager.SyncChannels(ctx))

	joined := mockJP.GetJoined()
	assert.ElementsMatch(t, []string{"caedrel", "summit1g"}, joined,
		"only syntactically valid Twitch logins should reach the IRC layer")
	assert.Equal(t, 2, manager.GetActiveChannelCount())
}

func TestManager_SyncChannels_PartRemovedChannels(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	repo := &MockRepository{
		channels: []string{"xqc", "summit1g"},
	}

	mockJP := NewMockJoinParter()
	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)

	// Initial sync
	err := manager.SyncChannels(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, manager.GetActiveChannelCount())

	// Update repo to remove one channel
	repo.channels = []string{"xqc"}

	// Sync again
	err = manager.SyncChannels(ctx)
	require.NoError(t, err)

	// Verify summit1g was parted.
	// Note: departed also includes depart-before-join calls (stale state prevention),
	// so we check that the explicit PART for "summit1g" is present rather than counting.
	departed := mockJP.GetDeparted()
	assert.Contains(t, departed, "summit1g")

	// Verify manager only tracks xqc now
	assert.Equal(t, 1, manager.GetActiveChannelCount())
	assert.True(t, manager.IsChannelActive("xqc"))
	assert.False(t, manager.IsChannelActive("summit1g"))
}

func TestManager_SyncChannels_JoinNewChannels(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	repo := &MockRepository{
		channels: []string{"xqc"},
	}

	mockJP := NewMockJoinParter()
	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)

	// Initial sync
	err := manager.SyncChannels(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, manager.GetActiveChannelCount())

	// Add new channels
	repo.channels = []string{"xqc", "summit1g", "shroud"}

	// Sync again
	err = manager.SyncChannels(ctx)
	require.NoError(t, err)

	// Verify new channels were joined
	joined := mockJP.GetJoined()
	assert.GreaterOrEqual(t, len(joined), 3) // At least 3 (initial + new)

	// Verify manager tracks all 3 channels
	assert.Equal(t, 3, manager.GetActiveChannelCount())
	assert.True(t, manager.IsChannelActive("xqc"))
	assert.True(t, manager.IsChannelActive("summit1g"))
	assert.True(t, manager.IsChannelActive("shroud"))
}

// Regression for the 2026-05-19 caesarlp outage: when the IRC layer short-
// circuits a JOIN (transient ban applied by joinAckWatchdog while the
// underlying connection was zombie), the channel manager MUST NOT record the
// channel in activeChans. Recording a phantom join means subsequent sync
// cycles see the channel as "already joined" and never retry it, leaving the
// streamer's chat completely offline until the source is manually toggled —
// even after the ban window expires.
//
// Expectation: SyncChannels calls Join() once; Join() returns false; activeChans
// stays empty; the next sync cycle (after Join() starts returning true again)
// finally records the channel as active.
func TestManager_SyncChannels_DoesNotMarkActiveWhenJoinShortCircuited(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	repo := &MockRepository{
		channels: []string{"caesarlp"},
	}

	mockJP := NewMockJoinParter()
	mockJP.SetBanned("caesarlp") // IRC layer will refuse this JOIN

	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)

	// First sync: Join() returns false; channel must not be recorded as active.
	require.NoError(t, manager.SyncChannels(ctx))
	assert.Equal(t, 0, manager.GetActiveChannelCount(),
		"a banned channel must not appear in activeChans — recording it freezes the channel out of all future syncs")
	assert.False(t, manager.IsChannelActive("caesarlp"))
	assert.Equal(t, 1, mockJP.GetJoinCallCount(),
		"the manager should still attempt the JOIN — the gating is on the *result*, not on skipping the call")

	// Simulate ban expiry: clear the mock's ban list. The next sync must retry
	// the JOIN (because activeChans is still empty) and finally mark it active.
	mockJP.mu.Lock()
	mockJP.bannedChannels = make(map[string]bool)
	mockJP.mu.Unlock()

	require.NoError(t, manager.SyncChannels(ctx))
	assert.Equal(t, 1, manager.GetActiveChannelCount(),
		"after the IRC ban clears, the channel must finally be recorded as active")
	assert.True(t, manager.IsChannelActive("caesarlp"))
}

// A banned channel must remain in toJoin across many syncs as long as the IRC
// ban persists — i.e. Join() is retried each cycle so the channel comes back
// the moment the ban expires. Without this property, the only way to recover
// from a transient ban is to manually toggle the source in the database.
func TestManager_SyncChannels_RetriesBannedChannelEachCycle(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	repo := &MockRepository{
		channels: []string{"caesarlp"},
	}

	mockJP := NewMockJoinParter()
	mockJP.SetBanned("caesarlp")

	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)

	const cycles = 5
	for i := 0; i < cycles; i++ {
		require.NoError(t, manager.SyncChannels(ctx))
	}

	assert.Equal(t, cycles, mockJP.GetJoinCallCount(),
		"Join() must be retried every sync cycle while the ban is in effect; "+
			"otherwise a transient ban becomes permanent in the manager's bookkeeping")
	assert.Equal(t, 0, manager.GetActiveChannelCount())
}

func TestManager_SyncChannels_NoChanges(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	repo := &MockRepository{
		channels: []string{"xqc", "summit1g"},
	}

	mockJP := NewMockJoinParter()
	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)

	// Initial sync
	err := manager.SyncChannels(ctx)
	require.NoError(t, err)

	initialJoinCount := mockJP.GetJoinCallCount()
	initialDepartCount := mockJP.GetDepartCallCount()

	// Sync again with no changes
	err = manager.SyncChannels(ctx)
	require.NoError(t, err)

	// Verify no new joins or parts
	assert.Equal(t, initialJoinCount, mockJP.GetJoinCallCount())
	assert.Equal(t, initialDepartCount, mockJP.GetDepartCallCount())
}

func TestManager_SyncChannels_EmptyChannels(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	repo := &MockRepository{
		channels: []string{},
	}

	mockJP := NewMockJoinParter()
	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)

	err := manager.SyncChannels(ctx)
	require.NoError(t, err)

	// No channels should be joined
	assert.Equal(t, 0, manager.GetActiveChannelCount())
	assert.Equal(t, 0, mockJP.GetJoinCallCount())
}

func TestManager_RateLimiting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping rate limiting test in short mode")
	}

	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	// Create a lot of channels to test rate limiting. Names must satisfy
	// IsValidTwitchLogin (^[a-z0-9_]{3,25}$) — otherwise SyncChannels filters
	// them out before the rate limiter ever runs.
	channels := make([]string, 50)
	for i := 0; i < 50; i++ {
		channels[i] = fmt.Sprintf("ch_%02d", i)
	}

	repo := &MockRepository{
		channels: channels,
	}

	mockJP := NewMockJoinParter()
	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)

	start := time.Now()
	err := manager.SyncChannels(ctx)
	require.NoError(t, err)
	duration := time.Since(start)

	// With burst allowance of 20 and 1 event per 500ms afterwards, 50 joins take >=15s
	expectedMinDuration := 15 * time.Second
	assert.GreaterOrEqual(t, duration, expectedMinDuration,
		"Rate limiting should enforce minimum duration")

	// All channels should be joined
	assert.Equal(t, 50, manager.GetActiveChannelCount())
}

func TestManager_GetActiveChannels(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	repo := &MockRepository{
		channels: []string{"xqc", "summit1g", "shroud"},
	}

	mockJP := NewMockJoinParter()
	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)

	err := manager.SyncChannels(ctx)
	require.NoError(t, err)

	activeChannels := manager.GetActiveChannels()
	assert.Len(t, activeChannels, 3)
	assert.Contains(t, activeChannels, "xqc")
	assert.Contains(t, activeChannels, "summit1g")
	assert.Contains(t, activeChannels, "shroud")
}

func TestManager_StartStop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logger := zaptest.NewLogger(t)

	repo := &MockRepository{
		channels: []string{"xqc"},
	}

	mockJP := NewMockJoinParter()
	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)

	// Start manager
	err := manager.Start(ctx)
	require.NoError(t, err)

	// Let it run for a bit
	time.Sleep(100 * time.Millisecond)

	// Stop manager
	manager.Stop()

	// Verify at least initial sync happened
	assert.Greater(t, mockJP.GetJoinCallCount(), 0)
}

func TestManager_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	repo := &MockRepository{
		channels: []string{"xqc"},
	}

	mockJP := NewMockJoinParter()
	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)

	err := manager.SyncChannels(ctx)
	require.NoError(t, err)

	// Concurrent reads should not panic
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = manager.GetActiveChannels()
			_ = manager.GetActiveChannelCount()
			_ = manager.IsChannelActive("xqc")
		}()
	}

	wg.Wait()
}

// mockLeadershipClient is a LeadershipClient that grants leadership to the first claimer
// and denies it to all subsequent claimers, simulating two-pod channel splitting.
type mockLeadershipClient struct {
	mu      sync.Mutex
	claimed map[string]string // streamID -> callerID that owns it
}

func newMockLeadershipClient() *mockLeadershipClient {
	return &mockLeadershipClient{claimed: make(map[string]string)}
}

func (m *mockLeadershipClient) ClaimLeadership(_ context.Context, _, streamID, callerID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.claimed[streamID]; exists {
		return false, nil // already claimed by another pod
	}
	m.claimed[streamID] = callerID
	return true, nil
}

func (m *mockLeadershipClient) RenewLeadership(_ context.Context, _, streamID, callerID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	owner, exists := m.claimed[streamID]
	if !exists {
		return false, nil
	}
	return owner == callerID, nil
}

func (m *mockLeadershipClient) ReleaseLeadership(_ context.Context, _, streamID, callerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.claimed[streamID] == callerID {
		delete(m.claimed, streamID)
	}
	return nil
}

func (m *mockLeadershipClient) RegisterPeer(_ context.Context, _, _ string) (int, error) {
	return 2, nil // simulate 2 peers
}

// TestManager_JoinChannelsMultipleConnections_RespectsLeadership is a regression test for
// the bug where joinChannelsMultipleConnections bypassed EnsureLeadership, causing both
// twitch-listener pods to join ALL channels instead of splitting them.
//
// This test simulates two concurrent managers (pod A and pod B) joining the same 100+
// channels. With correct leadership, each channel must be joined by exactly one pod.
func TestManager_JoinChannelsMultipleConnections_RespectsLeadership(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	// Build 110 channels to force the multi-connection path (>= 100)
	allChannels := make([]string, 110)
	for i := range allChannels {
		allChannels[i] = fmt.Sprintf("channel%d", i)
	}

	// Shared leadership backend — simulates the Redis-backed source-manager
	sharedClient := newMockLeadershipClient()

	newPod := func() (*Manager, *MockJoinParter) {
		repo := &MockRepository{channels: allChannels}
		jp := NewMockJoinParter()
		coord := sourcemanager.NewLeadershipCoordinator("twitch", sharedClient, 5*time.Second, logger)
		mgr := NewManager(repo, jp, nil, coord, nil, nil, "", logger, nil)
		return mgr, jp
	}

	mgrA, jpA := newPod()
	mgrB, jpB := newPod()

	// Both pods sync concurrently, mimicking a real deployment rollout
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); require.NoError(t, mgrA.SyncChannels(ctx)) }()
	go func() { defer wg.Done(); require.NoError(t, mgrB.SyncChannels(ctx)) }()
	wg.Wait()

	joinedA := jpA.GetJoined()
	joinedB := jpB.GetJoined()

	// Each channel should be joined by exactly one pod
	allJoined := make(map[string]int, len(allChannels))
	for _, ch := range joinedA {
		allJoined[ch]++
	}
	for _, ch := range joinedB {
		allJoined[ch]++
	}

	for _, ch := range allChannels {
		assert.Equal(t, 1, allJoined[ch],
			"channel %q should be joined by exactly one pod (was joined %d times)", ch, allJoined[ch])
	}

	// Both pods together must cover all channels
	assert.Equal(t, len(allChannels), len(joinedA)+len(joinedB),
		"total joined channels across both pods should equal total channels")

	// Neither pod alone should have all channels (verify splitting happened)
	assert.Less(t, len(joinedA), len(allChannels),
		"pod A should not have joined all channels")
	assert.Less(t, len(joinedB), len(allChannels),
		"pod B should not have joined all channels")
}

// TestManager_IsInitialSyncComplete verifies that the flag is false before
// SyncChannels runs and true afterwards, regardless of how many channels the
// pod actually joins (including zero, as happens when a peer holds all locks).
func TestManager_IsInitialSyncComplete(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	repo := &MockRepository{channels: []string{"xqc", "summit1g"}}
	mockJP := NewMockJoinParter()
	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)

	// Before any sync the flag must be false.
	assert.False(t, manager.IsInitialSyncComplete(), "should be false before first sync")

	require.NoError(t, manager.SyncChannels(ctx))

	// After a successful sync the flag must be true even if the pod joined channels.
	assert.True(t, manager.IsInitialSyncComplete(), "should be true after first sync")
}

// TestManager_IsInitialSyncComplete_ZeroChannels verifies that the flag is set
// to true even when the sync results in 0 active channels.  This is the
// production scenario where the first pod holds all Redis leadership locks and
// the second pod wins none — the second pod must still become ready.
func TestManager_IsInitialSyncComplete_ZeroChannels(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	// Empty channel list → 0 channels joined.
	repo := &MockRepository{channels: []string{}}
	mockJP := NewMockJoinParter()
	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)

	require.NoError(t, manager.SyncChannels(ctx))

	assert.Equal(t, 0, manager.GetActiveChannelCount())
	assert.True(t, manager.IsInitialSyncComplete(),
		"should be true even when 0 channels are active (peer holds all locks)")
}

// TestManager_IsLeadershipEnabled verifies that the flag reflects whether a
// LeadershipCoordinator was injected.
func TestManager_IsLeadershipEnabled(t *testing.T) {
	logger := zaptest.NewLogger(t)
	repo := &MockRepository{}
	mockJP := NewMockJoinParter()

	// Without a leader the flag must be false.
	noLeader := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)
	assert.False(t, noLeader.IsLeadershipEnabled())

	// With a real LeadershipCoordinator the flag must be true.
	lc := sourcemanager.NewLeadershipCoordinator("twitch", nil, 0, logger)
	withLeader := NewManager(repo, mockJP, nil, lc, nil, nil, "", logger, nil)
	assert.True(t, withLeader.IsLeadershipEnabled())
}

// TestManager_SyncChannels_DoesNotBlockHealthProbe verifies that the readiness
// probe methods (GetActiveChannelCount, IsInitialSyncComplete) are never blocked
// while SyncChannels is executing.  Before the fix, SyncChannels held m.mu for
// the entire rate-limited JOIN loop, causing the probe to time out and Kubernetes
// to cycle the pod.
func TestManager_SyncChannels_DoesNotBlockHealthProbe(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	// Use a blocking JoinParter to simulate the slow IRC join rate limiter.
	blockCh := make(chan struct{})
	slowJP := newSlowJoinParter(blockCh)

	repo := &MockRepository{channels: []string{"xqc", "summit1g", "shroud"}}
	manager := NewManager(repo, slowJP, nil, nil, nil, nil, "", logger, nil)

	syncDone := make(chan struct{})
	go func() {
		defer close(syncDone)
		_ = manager.SyncChannels(ctx)
	}()

	// Wait until the slow JoinParter is blocked mid-join.
	select {
	case <-slowJP.blocking:
	case <-time.After(2 * time.Second):
		t.Fatal("SyncChannels did not start joining within 2s")
	}

	// The readiness probe must respond immediately (< 100ms) while SyncChannels
	// is blocked in the slow join, because m.mu must NOT be held during joins.
	probeDone := make(chan struct{})
	go func() {
		defer close(probeDone)
		_ = manager.GetActiveChannelCount()
		_ = manager.IsInitialSyncComplete()
	}()

	select {
	case <-probeDone:
		// Good: probe returned without waiting for SyncChannels to finish.
	case <-time.After(100 * time.Millisecond):
		t.Error("health probe methods blocked during SyncChannels — mutex held across slow join")
	}

	// Unblock the join so the goroutine can finish cleanly.
	close(blockCh)
	<-syncDone
}

// slowJoinParter blocks on Join() until blockCh is closed, simulating the slow
// IRC rate-limited join loop.
type slowJoinParter struct {
	blockCh  chan struct{}
	blocking chan struct{}
	once     sync.Once
}

func (s *slowJoinParter) Join(_ string) bool {
	s.once.Do(func() {
		// Signal that we have reached the blocking point.
		close(s.blocking)
	})
	<-s.blockCh // block until test unblocks us
	return true
}

func (s *slowJoinParter) Depart(_ string) {}

func newSlowJoinParter(blockCh chan struct{}) *slowJoinParter {
	return &slowJoinParter{
		blockCh:  blockCh,
		blocking: make(chan struct{}),
	}
}

// TestManager_AtomicProbe_InitialSyncComplete verifies that IsInitialSyncComplete
// reads from an atomic field — it returns the correct value without acquiring the mutex.
// The test holds the write lock from a goroutine while checking the probe method
// to confirm there is no deadlock.
func TestManager_AtomicProbe_InitialSyncComplete(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	repo := &MockRepository{channels: []string{"xqc"}}
	mockJP := NewMockJoinParter()
	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)

	require.NoError(t, manager.SyncChannels(ctx))

	// Hold the write lock from another goroutine.
	released := make(chan struct{})
	manager.mu.Lock()
	go func() {
		defer manager.mu.Unlock()
		<-released
	}()

	// IsInitialSyncComplete must return immediately despite the write lock.
	done := make(chan bool, 1)
	go func() {
		done <- manager.IsInitialSyncComplete()
	}()

	select {
	case result := <-done:
		assert.True(t, result)
	case <-time.After(500 * time.Millisecond):
		t.Error("IsInitialSyncComplete blocked — it must use atomic.Bool, not mutex")
	}

	close(released) // let the write-lock goroutine exit
}

// TestManager_AtomicProbe_ActiveChannelCount verifies GetActiveChannelCount reads
// from an atomic field without blocking on the mutex.
func TestManager_AtomicProbe_ActiveChannelCount(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	repo := &MockRepository{channels: []string{"xqc", "summit1g", "shroud"}}
	mockJP := NewMockJoinParter()
	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)

	require.NoError(t, manager.SyncChannels(ctx))
	assert.Equal(t, 3, manager.GetActiveChannelCount())

	// Hold the write lock from another goroutine.
	released := make(chan struct{})
	manager.mu.Lock()
	go func() {
		defer manager.mu.Unlock()
		<-released
	}()

	done := make(chan int, 1)
	go func() {
		done <- manager.GetActiveChannelCount()
	}()

	select {
	case count := <-done:
		assert.Equal(t, 3, count)
	case <-time.After(500 * time.Millisecond):
		t.Error("GetActiveChannelCount blocked — it must use atomic.Int64, not mutex")
	}

	close(released)
}

// TestManager_AtomicProbe_FilteredAssignmentCount verifies GetFilteredAssignmentCount
// reads from an atomic field without blocking on the mutex.
func TestManager_AtomicProbe_FilteredAssignmentCount(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	repo := &MockRepository{channels: []string{"xqc", "summit1g"}}
	mockJP := NewMockJoinParter()
	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)

	require.NoError(t, manager.SyncChannels(ctx))
	// filteredAssignmentCount equals len(desiredChannels) when assignedSourceIDs is nil
	assert.Equal(t, 2, manager.GetFilteredAssignmentCount())

	// Hold the write lock from another goroutine.
	released := make(chan struct{})
	manager.mu.Lock()
	go func() {
		defer manager.mu.Unlock()
		<-released
	}()

	done := make(chan int, 1)
	go func() {
		done <- manager.GetFilteredAssignmentCount()
	}()

	select {
	case count := <-done:
		assert.Equal(t, 2, count)
	case <-time.After(500 * time.Millisecond):
		t.Error("GetFilteredAssignmentCount blocked — it must use atomic.Int64, not mutex")
	}

	close(released)
}

// TestManager_LeadershipAudit_EvictsOrphanedChannels verifies that channels
// in activeChans without a corresponding coordinator lease are evicted and
// then re-joined on the next sync cycle.
func TestManager_LeadershipAudit_EvictsOrphanedChannels(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	sharedClient := newMockLeadershipClient()
	repo := &MockRepository{channels: []string{"xqc", "summit1g", "shroud"}}
	mockJP := NewMockJoinParter()
	coord := sourcemanager.NewLeadershipCoordinator("twitch", sharedClient, 5*time.Second, logger)
	manager := NewManager(repo, mockJP, nil, coord, nil, nil, "", logger, nil)

	// Initial sync: all three channels joined and leadership acquired.
	require.NoError(t, manager.SyncChannels(ctx))
	assert.Equal(t, 3, manager.GetActiveChannelCount())
	assert.ElementsMatch(t, []string{"xqc", "summit1g", "shroud"}, mockJP.GetJoined())

	// Simulate leadership loss for "summit1g" by releasing it from the coordinator
	// AND clearing the mock client's record (mimics the Redis key expiring after
	// the old pod died, without the lostCallback firing cleanly).
	coord.Release("summit1g")
	// Wait briefly for the async release goroutine, then clear the claim so the
	// channel is claimable again (simulates Redis TTL expiry).
	time.Sleep(10 * time.Millisecond)
	sharedClient.mu.Lock()
	delete(sharedClient.claimed, "summit1g")
	sharedClient.mu.Unlock()

	// Next sync: the leadership audit should detect summit1g has no lease,
	// evict it from activeChans, and re-join it.
	mockJP.Reset()
	require.NoError(t, manager.SyncChannels(ctx))

	// summit1g should have been departed (eviction) and then re-joined.
	departed := mockJP.GetDeparted()
	joined := mockJP.GetJoined()
	assert.Contains(t, departed, "summit1g", "summit1g should be departed during eviction")
	assert.Contains(t, joined, "summit1g", "summit1g should be re-joined after eviction")
	assert.Equal(t, 3, manager.GetActiveChannelCount(), "all 3 channels should be active again")
}

func TestIsTwitchNotification(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    bool
	}{
		{"twitch platform", `{"platform":"twitch","channel_id":"w10u"}`, true},
		{"kick platform", `{"platform":"kick","channel_id":"foo"}`, false},
		{"tiktok platform", `{"platform":"tiktok","channel_id":"bar"}`, false},
		{"youtube platform", `{"platform":"youtube","channel_id":"baz"}`, false},
		{"empty platform", `{"platform":"","channel_id":"x"}`, true},
		{"no platform field", `{"channel_id":"x"}`, true},
		{"invalid json", `not json`, true},
		{"empty payload", ``, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTwitchNotification(tt.payload)
			assert.Equal(t, tt.want, got)
		})
	}
}
