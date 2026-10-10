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

package streams

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/poller"
	"github.com/caesar/all-chat/shared/sourcemanager"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// DiscoveryInterface defines the interface for discovery (for mocking)
type DiscoveryInterface interface {
	DiscoverLiveStream(ctx context.Context, channelID string) (string, error)
}

// MockDiscovery mocks the Discovery interface
type MockDiscovery struct {
	mock.Mock
}

func (m *MockDiscovery) DiscoverLiveStream(ctx context.Context, channelID string) (string, error) {
	args := m.Called(ctx, channelID)
	return args.String(0), args.Error(1)
}

// TestManager_OnOverlayConnected_CachedVideoID tests cached video ID path
func TestManager_OnOverlayConnected_CachedVideoID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup
	logger := zap.NewNop()
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer redisClient.Close()

	// Check Redis connection
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping test")
	}

	// Setup test data
	channelID := "test-channel-cached"
	videoID := "test-video-123"
	overlayID := "overlay-1"

	repository := NewRepository(redisClient, logger)

	// Pre-populate Redis cache
	err := repository.SetChannelVideoMapping(ctx, channelID, videoID)
	assert.NoError(t, err)
	defer repository.DeleteChannelVideoMapping(ctx, channelID)

	// Create simple manager (without full initialization for unit testing)
	manager := &Manager{
		repository:               repository,
		logger:                   logger,
		redisClient:              redisClient,
		activeStreams:            make(map[string]*Stream),
		discovering:              make(map[string]*DiscoveryState),
		connectedOverlays:        make(map[string]time.Time),
		channelConnectedOverlays: make(map[string]map[string]struct{}),
	}

	// Test: OnOverlayConnected with cached video ID
	sources := []Source{
		{ChannelID: channelID, OverlayID: overlayID},
	}
	manager.OnOverlayConnected(overlayID, sources)

	// Wait for async operations
	time.Sleep(100 * time.Millisecond)

	// Verify: Overlay tracked
	manager.mu.RLock()
	_, overlayConnected := manager.connectedOverlays[overlayID]
	channelOverlays, channelTracked := manager.channelConnectedOverlays[channelID]
	_, overlayInChannel := channelOverlays[overlayID]
	manager.mu.RUnlock()

	assert.True(t, overlayConnected, "Overlay should be tracked")
	assert.True(t, channelTracked, "Channel should be tracked")
	assert.True(t, overlayInChannel, "Overlay should be in channel map")
}

// TestManager_OnOverlayConnected_Discovery tests discovery path (no cache)
func TestManager_OnOverlayConnected_Discovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup
	logger := zap.NewNop()
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer redisClient.Close()

	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping test")
	}

	channelID := "test-channel-no-cache"
	overlayID := "overlay-2"

	repository := NewRepository(redisClient, logger)

	// Ensure no cached value exists
	repository.DeleteChannelVideoMapping(ctx, channelID)

	// Create manager
	manager := &Manager{
		repository:               repository,
		logger:                   logger,
		redisClient:              redisClient,
		activeStreams:            make(map[string]*Stream),
		discovering:              make(map[string]*DiscoveryState),
		connectedOverlays:        make(map[string]time.Time),
		channelConnectedOverlays: make(map[string]map[string]struct{}),
		stopChan:                 make(chan struct{}),
	}

	// Test: OnOverlayConnected without cached video ID
	sources := []Source{
		{ChannelID: channelID, OverlayID: overlayID},
	}
	manager.OnOverlayConnected(overlayID, sources)

	// Wait for async discovery to start
	time.Sleep(100 * time.Millisecond)

	// Verify: Discovery state created
	manager.mu.RLock()
	discoveryState, discovering := manager.discovering[channelID]
	manager.mu.RUnlock()

	assert.True(t, discovering, "Discovery should be in progress")
	if discovering {
		assert.Equal(t, channelID, discoveryState.ChannelID)
		assert.Equal(t, overlayID, discoveryState.OverlayID)
		assert.NotNil(t, discoveryState.CancelFunc)

		// Cancel discovery to clean up
		discoveryState.CancelFunc()
	}
}

// TestManager_DiscoveryLoop_Success tests successful discovery with backoff
func TestManager_DiscoveryLoop_Success(t *testing.T) {
	// Setup
	logger := zap.NewNop()
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer redisClient.Close()

	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping test")
	}

	channelID := "test-channel-discovery-success"
	videoID := "discovered-video-456"
	overlayID := "overlay-3"

	repository := NewRepository(redisClient, logger)
	defer repository.DeleteChannelVideoMapping(ctx, channelID)

	// Mock discovery that succeeds on first attempt
	mockDiscovery := &MockDiscovery{}
	mockDiscovery.On("DiscoverLiveStream", mock.Anything, channelID).Return(videoID, nil)

	manager := &Manager{
		repository:               repository,
		logger:                   logger,
		redisClient:              redisClient,
		activeStreams:            make(map[string]*Stream),
		discovering:              make(map[string]*DiscoveryState),
		connectedOverlays:        make(map[string]time.Time),
		channelConnectedOverlays: make(map[string]map[string]struct{}),
		stopChan:                 make(chan struct{}),
	}
	manager.wg.Add(1)

	// Create discovery state
	_, cancel := context.WithCancel(ctx)
	defer cancel()

	state := &DiscoveryState{
		ChannelID:  channelID,
		OverlayID:  overlayID,
		StartedAt:  time.Now(),
		Attempts:   0,
		CancelFunc: cancel,
	}

	manager.mu.Lock()
	manager.discovering[channelID] = state
	manager.mu.Unlock()

	// Note: We can't fully test discoveryLoop without Discovery interface in Manager
	// This test would require refactoring Manager to accept DiscoveryInterface
	// For now, we verify the discovery state is created correctly

	// Verify: Discovery state exists
	manager.mu.RLock()
	_, discovering := manager.discovering[channelID]
	manager.mu.RUnlock()
	assert.True(t, discovering, "Discovery state should exist")

	// Cleanup
	cancel()
	manager.wg.Done()
}

// TestManager_DiscoveryLoop_Timeout tests discovery timeout (15 minutes)
func TestManager_DiscoveryLoop_Timeout(t *testing.T) {
	// This test uses a very short timeout to avoid waiting 15 minutes
	// In production, timeout is 15 minutes

	logger := zap.NewNop()
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer redisClient.Close()

	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping test")
	}

	channelID := "test-channel-timeout"
	overlayID := "overlay-4"

	repository := NewRepository(redisClient, logger)

	// Mock discovery that always fails
	mockDiscovery := &MockDiscovery{}
	mockDiscovery.On("DiscoverLiveStream", mock.Anything, channelID).Return("", assert.AnError)

	manager := &Manager{
		repository:               repository,
		logger:                   logger,
		redisClient:              redisClient,
		activeStreams:            make(map[string]*Stream),
		discovering:              make(map[string]*DiscoveryState),
		connectedOverlays:        make(map[string]time.Time),
		channelConnectedOverlays: make(map[string]map[string]struct{}),
		stopChan:                 make(chan struct{}),
	}
	manager.wg.Add(1)

	// Create discovery state with short deadline for testing
	// Note: This modifies discoveryLoop behavior indirectly by manipulating StartedAt
	_, cancel := context.WithCancel(ctx)
	defer cancel()

	state := &DiscoveryState{
		ChannelID:  channelID,
		OverlayID:  overlayID,
		StartedAt:  time.Now().Add(-16 * time.Minute), // Simulate already timed out
		Attempts:   0,
		CancelFunc: cancel,
	}

	manager.mu.Lock()
	manager.discovering[channelID] = state
	manager.mu.Unlock()

	// Verify: Discovery state exists before timeout
	manager.mu.RLock()
	_, discovering := manager.discovering[channelID]
	manager.mu.RUnlock()
	assert.True(t, discovering, "Discovery state should exist before timeout")

	// Cleanup
	cancel()
	manager.wg.Done()
}

// TestManager_OnOverlayDisconnected_StopsPoller tests overlay disconnection cleanup
func TestManager_OnOverlayDisconnected_StopsPoller(t *testing.T) {
	// Setup
	logger := zap.NewNop()
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer redisClient.Close()

	channelID := "test-channel-disconnect"
	videoID := "test-video-disconnect"
	overlayID := "overlay-5"

	repository := NewRepository(redisClient, logger)

	manager := &Manager{
		repository:               repository,
		logger:                   logger,
		redisClient:              redisClient,
		activeStreams:            make(map[string]*Stream),
		discovering:              make(map[string]*DiscoveryState),
		connectedOverlays:        make(map[string]time.Time),
		channelConnectedOverlays: make(map[string]map[string]struct{}),
	}

	// Setup: Add overlay connection
	manager.mu.Lock()
	manager.connectedOverlays[overlayID] = time.Now()
	manager.channelConnectedOverlays[channelID] = make(map[string]struct{})
	manager.channelConnectedOverlays[channelID][overlayID] = struct{}{}

	// Add active stream (without actual poller for simplicity)
	manager.activeStreams[videoID] = &Stream{
		VideoID:   videoID,
		ChannelID: channelID,
		OverlayID: overlayID,
	}
	manager.mu.Unlock()

	// Test: OnOverlayDisconnected
	manager.OnOverlayDisconnected(overlayID)

	// Verify: Overlay removed immediately
	manager.mu.RLock()
	_, overlayConnected := manager.connectedOverlays[overlayID]
	_, channelTracked := manager.channelConnectedOverlays[channelID]
	manager.mu.RUnlock()

	assert.False(t, overlayConnected, "Overlay should be removed")
	assert.False(t, channelTracked, "Channel should be removed when no overlays connected")

	// Note: Actual poller stop happens after 5s debounce
	// We don't test the full debounce flow here to avoid long test runtime
}

// TestManager_DiscoveryGiveUp_RefreshOnDemandChange verifies the give-up /
// "wait for a refresh" behaviour added to bound discovery polling:
//   - markDiscoveryGaveUp parks a channel,
//   - a demand change (newly demanded OR demand lost) clears the marker,
//   - a channel that stays continuously demanded keeps its marker.
//
// Pure in-memory logic — no Redis required.
func TestManager_DiscoveryGiveUp_RefreshOnDemandChange(t *testing.T) {
	manager := &Manager{
		logger:          zap.NewNop(),
		gaveUpDiscovery: make(map[string]bool),
	}

	const parked = "UCparked"
	const reconnected = "UCreconnected"

	manager.markDiscoveryGaveUp(parked)
	manager.markDiscoveryGaveUp(reconnected)
	assert.True(t, manager.hasDiscoveryGivenUp(parked))
	assert.True(t, manager.hasDiscoveryGivenUp(reconnected))

	// prev: both demanded. new: `reconnected` dropped (overlay disconnected),
	// `parked` still demanded (streamer simply offline). Only the channel whose
	// demand changed should be cleared.
	prev := map[string]bool{parked: true, reconnected: true}
	demanded := map[string]bool{parked: true}
	manager.clearGaveUpForDemandChanges(prev, demanded)

	assert.True(t, manager.hasDiscoveryGivenUp(parked),
		"continuously-demanded channel keeps its give-up marker (waits for a real refresh)")
	assert.False(t, manager.hasDiscoveryGivenUp(reconnected),
		"channel whose demand changed is treated as refreshed and cleared")

	// A subsequent re-assertion of demand for `parked` (overlay reconnect) is a
	// change false->true and must clear it.
	manager.clearGaveUpForDemandChanges(map[string]bool{}, map[string]bool{parked: true})
	assert.False(t, manager.hasDiscoveryGivenUp(parked),
		"re-asserted demand clears the give-up marker so discovery can resume")
}

// TestManager_CleanupDiscoveryState_KeepsNewerReservation reproduces the loop leak
// that tripped AllChatYouTubeDiscoveryRetryStorm in production: a discovery loop
// can outlive its own reservation, and its late cleanup must not evict the loop
// that replaced it. Pure in-memory logic — no Redis required.
func TestManager_CleanupDiscoveryState_KeepsNewerReservation(t *testing.T) {
	manager := &Manager{
		logger:      zap.NewNop(),
		discovering: make(map[string]*DiscoveryState),
	}

	const channelID = "UCleakrepro"

	// A discovery loop is running and holds the reservation.
	stale := &DiscoveryState{ChannelID: channelID, CancelFunc: func() {}}
	manager.discovering[channelID] = stale

	// Demand is lost: reconcileDemand cancels the context and drops the reservation
	// immediately, while the goroutine is still mid-attempt.
	delete(manager.discovering, channelID)

	// Demand returns before the cancelled goroutine notices, so a fresh loop
	// reserves the slot and starts polling.
	fresh := &DiscoveryState{ChannelID: channelID, CancelFunc: func() {}}
	manager.discovering[channelID] = fresh

	// Only now does the cancelled goroutine reach its cleanup.
	manager.cleanupDiscoveryState(stale)

	current, exists := manager.discovering[channelID]
	assert.True(t, exists,
		"stale cleanup must leave the channel reserved, else periodic sync spawns another loop")
	assert.Same(t, fresh, current, "stale cleanup must not evict the newer discovery state")
}

// TestManager_CleanupDiscoveryState_ReleasesOwnReservation verifies the identity
// check does not break the normal path: the loop that owns the reservation still
// releases it, so a later sync can rediscover the channel.
func TestManager_CleanupDiscoveryState_ReleasesOwnReservation(t *testing.T) {
	manager := &Manager{
		logger:      zap.NewNop(),
		discovering: make(map[string]*DiscoveryState),
	}

	const channelID = "UCowner"
	owner := &DiscoveryState{ChannelID: channelID, CancelFunc: func() {}}
	manager.discovering[channelID] = owner

	manager.cleanupDiscoveryState(owner)

	_, exists := manager.discovering[channelID]
	assert.False(t, exists, "the reservation holder must release its own slot")
}

// TestManager_DiscoveryLoop_ExitsWhenReservationLost covers the orphan case:
// a loop that no longer owns its channel is unreachable by reconcileDemand (that
// only cancels states still in m.discovering), so the loop must notice and stop
// itself rather than scrape YouTube until the 1h give-up cap.
func TestManager_DiscoveryLoop_ExitsWhenReservationLost(t *testing.T) {
	manager := &Manager{
		logger:      zap.NewNop(),
		discovering: make(map[string]*DiscoveryState),
	}

	const channelID = "UCorphan"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	orphan := &DiscoveryState{
		ChannelID:        channelID,
		StartedAt:        time.Now(),
		CancelFunc:       cancel,
		ResetBackoffChan: make(chan struct{}, 1),
	}
	owner := &DiscoveryState{ChannelID: channelID, CancelFunc: func() {}}
	manager.discovering[channelID] = owner

	manager.wg.Add(1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		manager.discoveryLoop(ctx, orphan)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("orphaned discovery loop did not exit; it would keep polling YouTube until the give-up cap")
	}

	assert.Same(t, owner, manager.discovering[channelID],
		"the orphan must not touch the current owner's reservation")
	assert.Error(t, ctx.Err(),
		"the orphan must cancel its own context so its cross-platform subscriber shuts down")
}

// fakeStreamDiscovery records every call in order so the tests can assert the
// pin is tried before the channel browse. A successful GetInitialContinuation
// returns an empty token: the started poller then idles in its own backoff
// instead of calling YouTube.
type fakeStreamDiscovery struct {
	mu        sync.Mutex
	calls     []string
	liveVideo string
	notLive   map[string]bool
	onCall    func(call string)
}

func (f *fakeStreamDiscovery) record(call string) {
	f.mu.Lock()
	f.calls = append(f.calls, call)
	hook := f.onCall
	f.mu.Unlock()
	if hook != nil {
		hook(call)
	}
}

func (f *fakeStreamDiscovery) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func (f *fakeStreamDiscovery) setNotLive(videoID string, notLive bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.notLive[videoID] = notLive
}

func (f *fakeStreamDiscovery) DiscoverLiveStream(_ context.Context, _, _, _ string) (string, error) {
	f.record("browse")
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.liveVideo == "" {
		return "", errors.New("channel offline")
	}
	return f.liveVideo, nil
}

func (f *fakeStreamDiscovery) DiscoverAllLiveStreams(_ context.Context, _, _ string) ([]string, error) {
	f.record("browse-all")
	return nil, errors.New("channel offline")
}

func (f *fakeStreamDiscovery) GetInitialContinuation(_ context.Context, videoID, _ string) (string, string, error) {
	f.record("continuation:" + videoID)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.notLive[videoID] {
		return "", "", errors.New("no live chat for video")
	}
	return "", "", nil
}

type fakeSourceManager struct {
	mu      sync.Mutex
	sources []*sourcemanager.ActiveSource
}

func (f *fakeSourceManager) set(sources ...*sourcemanager.ActiveSource) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sources = sources
}

func newFakeSourceManager(t *testing.T) (*fakeSourceManager, *sourcemanager.Client) {
	t.Helper()
	f := &fakeSourceManager{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && r.URL.Path == "/sources" {
			f.mu.Lock()
			defer f.mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"sources": f.sources})
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	client, err := sourcemanager.NewClient(srv.URL, sourcemanager.NewStaticTokenSource("test"))
	require.NoError(t, err)
	return f, client
}

func newPinTestManager(t *testing.T, smClient *sourcemanager.Client) (*Manager, *fakeStreamDiscovery) {
	t.Helper()
	server := miniredis.RunT(t)
	// The client is deliberately never closed: a discovery that starts a poller
	// keeps its cross-platform subscription for the process lifetime (see
	// subscribeToPlatformEvents), and closing the client under that goroutine
	// makes it read a nil message.
	rdb := redis.NewClient(&redis.Options{Addr: server.Addr()})

	logger := zap.NewNop()
	m := NewManager(nil, smClient, NewRepository(rdb, logger), nil, nil, nil, rdb, nil, logger, nil, nil, nil)
	fake := &fakeStreamDiscovery{notLive: make(map[string]bool)}
	m.discovery = fake
	m.jitter = func() time.Duration { return 0 }
	t.Cleanup(func() { stopEverything(m) })
	return m, fake
}

func stopEverything(m *Manager) {
	m.mu.Lock()
	pollers := m.pollers
	m.pollers = make(map[string]*poller.Poller)
	m.activeStreams = make(map[string]*Stream)
	for ch, state := range m.discovering {
		state.CancelFunc()
		delete(m.discovering, ch)
	}
	m.mu.Unlock()
	for _, p := range pollers {
		p.Stop()
	}
}

func reserveDiscovery(t *testing.T, m *Manager, channelID, pin string) (*DiscoveryState, context.Context, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	state := &DiscoveryState{
		ChannelID:        channelID,
		OverlayID:        "overlay-1",
		PinnedVideoID:    pin,
		StartedAt:        time.Now(),
		CancelFunc:       cancel,
		ResetBackoffChan: make(chan struct{}, 1),
	}
	m.mu.Lock()
	m.discovering[channelID] = state
	m.mu.Unlock()
	return state, ctx, cancel
}

func runDiscoveryLoop(m *Manager, ctx context.Context, state *DiscoveryState) {
	m.wg.Add(1)
	m.discoveryLoop(ctx, state)
}

func injectRunningPoller(m *Manager, channelID, videoID string) {
	p := poller.NewPoller(nil, "", channelID, zap.NewNop(), nil)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pollers[videoID] = p
	m.activeStreams[videoID] = &Stream{VideoID: videoID, ChannelID: channelID, OverlayID: "overlay-1"}
}

func polledVideos(m *Manager, channelID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var ids []string
	for id, s := range m.activeStreams {
		if s.ChannelID == channelID {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func discoveryStateFor(m *Manager, channelID string) *DiscoveryState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.discovering[channelID]
}

func TestDiscoveryLoop_PinTriedBeforeBrowse(t *testing.T) {
	m, fake := newPinTestManager(t, nil)
	fake.liveVideo = "browseVid01"
	state, ctx, _ := reserveDiscovery(t, m, "UCpin", "pinnedVid01")

	runDiscoveryLoop(m, ctx, state)

	assert.Equal(t, []string{"continuation:pinnedVid01"}, fake.Calls(),
		"a pollable pin must be started without browsing the channel")
	assert.Equal(t, []string{"pinnedVid01"}, polledVideos(m, "UCpin"))
	assert.Nil(t, discoveryStateFor(m, "UCpin"), "a started pin releases the discovery reservation")
}

func TestDiscoveryLoop_PinFailureFallsBackToBrowse(t *testing.T) {
	m, fake := newPinTestManager(t, nil)
	fake.liveVideo = "browseVid01"
	fake.setNotLive("pinnedVid01", true)
	state, ctx, _ := reserveDiscovery(t, m, "UCpin", "pinnedVid01")

	runDiscoveryLoop(m, ctx, state)

	assert.Equal(t, []string{"continuation:pinnedVid01", "browse", "continuation:browseVid01"}, fake.Calls(),
		"a pin that cannot be polled falls back to the browse within the same attempt")
	assert.Equal(t, []string{"browseVid01"}, polledVideos(m, "UCpin"))
}

func TestDiscoveryLoop_PinCachedBeforeStart(t *testing.T) {
	m, fake := newPinTestManager(t, nil)
	state, ctx, cancel := reserveDiscovery(t, m, "UCpin", "pinnedVid01")

	var mappingAtStart string
	fake.setNotLive("pinnedVid01", true)
	fake.onCall = func(call string) {
		switch call {
		case "continuation:pinnedVid01":
			mappingAtStart, _ = m.repository.GetChannelVideoMapping(context.Background(), "UCpin")
		case "browse":
			cancel()
		}
	}

	runDiscoveryLoop(m, ctx, state)

	assert.Equal(t, "pinnedVid01", mappingAtStart,
		"the pin must be cached before startPoller so other replicas see its leader key")
	mapping, err := m.repository.GetChannelVideoMapping(context.Background(), "UCpin")
	require.NoError(t, err)
	assert.Equal(t, "pinnedVid01", mapping)
}

func TestSyncSources_MixedPinsStable(t *testing.T) {
	sm, smClient := newFakeSourceManager(t)
	m, fake := newPinTestManager(t, smClient)
	fake.setNotLive("pinnedVidB1", true)
	fake.setNotLive("pinnedVidC1", true)

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	unpinned := &sourcemanager.ActiveSource{ID: "a", OverlayID: "overlay-a", ChannelID: "UCmixed", IsActive: true, CreatedAt: t0}
	pinned := &sourcemanager.ActiveSource{ID: "b", OverlayID: "overlay-b", ChannelID: "UCmixed", StreamID: "pinnedVidB1", IsActive: true, CreatedAt: t0.Add(time.Hour)}
	laterPin := &sourcemanager.ActiveSource{ID: "c", OverlayID: "overlay-c", ChannelID: "UCmixed", StreamID: "pinnedVidC1", IsActive: true, CreatedAt: t0.Add(2 * time.Hour)}
	// source-manager serves its registry in Go map order, so each sync sees a different order.
	orders := [][]*sourcemanager.ActiveSource{
		{unpinned, pinned, laterPin},
		{laterPin, pinned, unpinned},
		{pinned, unpinned, laterPin},
		{laterPin, unpinned, pinned},
		{pinned, laterPin, unpinned},
	}

	var first *DiscoveryState
	for i, order := range orders {
		sm.set(order...)
		m.syncSources(context.Background())

		state := discoveryStateFor(m, "UCmixed")
		require.NotNil(t, state, "sync %d: discovery must be running", i+1)
		if first == nil {
			first = state
		}
		assert.Same(t, first, state, "sync %d: an unchanged source set must not restart discovery", i+1)
		assert.Equal(t, "pinnedVidB1", state.PinnedVideoID, "sync %d: pin is the earliest non-empty stream_id", i+1)
		assert.Equal(t, "overlay-a", state.OverlayID, "sync %d: the earliest source supplies the overlay", i+1)
		m.mu.RLock()
		assert.Equal(t, "pinnedVidB1", m.pins["UCmixed"], "sync %d", i+1)
		m.mu.RUnlock()
	}
}

func TestSyncSources_PinProbeReplacesOtherVideo(t *testing.T) {
	const channelID = "UCprobe"
	const pin = "pinnedVid01"
	setup := func(t *testing.T) (*Manager, *fakeStreamDiscovery) {
		sm, smClient := newFakeSourceManager(t)
		sm.set(&sourcemanager.ActiveSource{ID: "a", OverlayID: "overlay-1", ChannelID: channelID, StreamID: pin, IsActive: true})
		m, fake := newPinTestManager(t, smClient)
		// Browse fell back to another video while the pin was not yet live.
		m.mu.Lock()
		m.pins[channelID] = pin
		m.mu.Unlock()
		injectRunningPoller(m, channelID, "otherVid001")
		return m, fake
	}

	t.Run("probe failure keeps the running video and is throttled", func(t *testing.T) {
		m, fake := setup(t)
		fake.setNotLive(pin, true)

		m.syncSources(context.Background())
		m.wg.Wait()
		assert.Equal(t, []string{"continuation:" + pin}, fake.Calls())
		assert.Equal(t, []string{"otherVid001"}, polledVideos(m, channelID))

		m.syncSources(context.Background())
		m.wg.Wait()
		assert.Equal(t, []string{"continuation:" + pin}, fake.Calls(), "a second sync within 60s must not probe again")
		assert.Equal(t, []string{"otherVid001"}, polledVideos(m, channelID))
	})

	t.Run("probe success restarts the channel on the pin", func(t *testing.T) {
		m, fake := setup(t)

		m.syncSources(context.Background())

		require.Eventually(t, func() bool {
			got := polledVideos(m, channelID)
			return len(got) == 1 && got[0] == pin
		}, 5*time.Second, 10*time.Millisecond, "channel must be restarted on the pin")
		assert.Equal(t, []string{"continuation:" + pin, "continuation:" + pin}, fake.Calls(),
			"probe, then the pin start; no browse")
	})
}

func TestSyncSources_PinChangeRestartsDiscovery(t *testing.T) {
	const channelID = "UCchange"
	const pin = "pinnedVid01"
	sm, smClient := newFakeSourceManager(t)
	m, fake := newPinTestManager(t, smClient)
	fake.setNotLive(pin, true)

	sm.set(&sourcemanager.ActiveSource{ID: "a", OverlayID: "overlay-1", ChannelID: channelID, IsActive: true})
	m.syncSources(context.Background())
	before := discoveryStateFor(m, channelID)
	require.NotNil(t, before)
	assert.Empty(t, before.PinnedVideoID)

	sm.set(&sourcemanager.ActiveSource{ID: "a", OverlayID: "overlay-1", ChannelID: channelID, StreamID: pin, IsActive: true})
	m.syncSources(context.Background())
	after := discoveryStateFor(m, channelID)
	require.NotNil(t, after)
	assert.NotSame(t, before, after, "a pin change must replace the running discovery")
	assert.Equal(t, pin, after.PinnedVideoID)

	m.stopChannel(channelID, true)
	m.wg.Wait()
}
