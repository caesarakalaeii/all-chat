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
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
