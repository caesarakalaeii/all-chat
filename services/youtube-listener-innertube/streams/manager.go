package streams

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener-innertube/deletion"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/innertube"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/metrics"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/poller"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/publisher"
	"github.com/caesar/all-chat/shared/sourcemanager"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Source represents a YouTube source configuration from source-manager
type Source struct {
	ChannelID string
	OverlayID string
}

// Stream represents an active YouTube stream being polled
type Stream struct {
	VideoID      string
	ChannelID    string
	OverlayID    string
	Continuation string
	StartedAt    time.Time
}

// DiscoveryState tracks ongoing discovery attempts for a channel
type DiscoveryState struct {
	ChannelID  string
	OverlayID  string
	StartedAt  time.Time
	Attempts   int
	CancelFunc context.CancelFunc
}

// Manager manages active YouTube streams and coordinates with source-manager
// Matches official youtube-listener's architecture:
//   - LeadershipCoordinator for stream ownership
//   - Async discovery with exponential backoff
//   - Redis-cached channel→video mappings
//   - Overlay connection lifecycle tracking
type Manager struct {
	leader        *sourcemanager.LeadershipCoordinator
	smClient      *sourcemanager.Client // Source manager client for querying active sources
	repository    *Repository
	discovery      *innertube.Discovery
	publisher      *publisher.StreamPublisher
	client         *innertube.Client
	redisClient    *redis.Client
	logger         *zap.Logger
	metrics        *metrics.InnerTubeMetrics
	batchDetector  *deletion.BatchDetector  // Batch deletion detector for cleanup
	deletionBuffer *deletion.DeletionBuffer // Deletion event buffer for cleanup

	mu               sync.RWMutex
	activeStreams    map[string]*Stream         // videoID → stream state
	pollers          map[string]*poller.Poller  // videoID → active poller
	discovering      map[string]*DiscoveryState // channelID → discovery state
	connectedOverlays map[string]time.Time      // overlay_id → connection_time
	channelConnectedOverlays map[string]map[string]struct{} // channel_id → overlay_ids

	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewManager creates a new stream manager
func NewManager(
	leader *sourcemanager.LeadershipCoordinator,
	smClient *sourcemanager.Client,
	repository *Repository,
	discovery *innertube.Discovery,
	publisher *publisher.StreamPublisher,
	client *innertube.Client,
	redisClient *redis.Client,
	logger *zap.Logger,
	m *metrics.InnerTubeMetrics,
	batchDetector *deletion.BatchDetector,
	deletionBuffer *deletion.DeletionBuffer,
) *Manager {
	return &Manager{
		leader:                   leader,
		smClient:                 smClient,
		repository:               repository,
		discovery:                discovery,
		publisher:                publisher,
		client:                   client,
		redisClient:              redisClient,
		logger:                   logger,
		metrics:                  m,
		batchDetector:            batchDetector,
		deletionBuffer:           deletionBuffer,
		activeStreams:            make(map[string]*Stream),
		pollers:                  make(map[string]*poller.Poller),
		discovering:              make(map[string]*DiscoveryState),
		connectedOverlays:        make(map[string]time.Time),
		channelConnectedOverlays: make(map[string]map[string]struct{}),
		stopChan:                 make(chan struct{}),
	}
}

// Start begins managing streams (non-blocking)
// Matches official youtube-listener pattern: periodic sync + PostgreSQL LISTEN
func (m *Manager) Start(ctx context.Context) error {
	m.logger.Info("Starting stream manager")

	// Start periodic sync (every 30s, matches official listener)
	m.wg.Add(1)
	go m.periodicSync(ctx)

	// TODO: Add PostgreSQL LISTEN for instant source changes
	// For now, rely on periodic sync only

	return nil
}

// OnOverlayConnected handles overlay connection events from API Gateway
// Triggers async discovery for all YouTube sources in the overlay
func (m *Manager) OnOverlayConnected(overlayID string, sources []Source) {
	m.logger.Info("Overlay connected",
		zap.String("overlay_id", overlayID),
		zap.Int("youtube_source_count", len(sources)),
	)

	// Track overlay connection
	m.mu.Lock()
	m.connectedOverlays[overlayID] = time.Now()
	m.mu.Unlock()

	// Start async discovery for each YouTube source
	for _, source := range sources {
		m.startAsyncDiscovery(source.ChannelID, overlayID)
	}
}

// OnOverlayDisconnected handles overlay disconnection events
// Stops pollers after debounce delay if no other overlays connected
func (m *Manager) OnOverlayDisconnected(overlayID string) {
	m.logger.Info("Overlay disconnected",
		zap.String("overlay_id", overlayID),
	)

	// Remove overlay from tracking
	m.mu.Lock()
	delete(m.connectedOverlays, overlayID)

	// Check which channels are affected
	channelsToCheck := make([]string, 0)
	for channelID, overlays := range m.channelConnectedOverlays {
		if _, exists := overlays[overlayID]; exists {
			delete(overlays, overlayID)
			if len(overlays) == 0 {
				channelsToCheck = append(channelsToCheck, channelID)
				delete(m.channelConnectedOverlays, channelID)
			}
		}
	}
	m.mu.Unlock()

	// Stop pollers for channels with no connected overlays (debounced to handle reconnects)
	for _, channelID := range channelsToCheck {
		go m.stopPollerAfterDebounce(channelID, 5*time.Second)
	}
}

// startAsyncDiscovery starts background discovery for a channel
// Checks Redis cache first, falls back to HTML discovery
func (m *Manager) startAsyncDiscovery(channelID, overlayID string) {
	m.mu.Lock()

	// Check if already discovering
	if state, exists := m.discovering[channelID]; exists {
		m.logger.Debug("Discovery already in progress",
			zap.String("channel_id", channelID),
			zap.String("existing_overlay_id", state.OverlayID),
		)
		m.mu.Unlock()
		return
	}

	// Track channel→overlay connection
	if _, exists := m.channelConnectedOverlays[channelID]; !exists {
		m.channelConnectedOverlays[channelID] = make(map[string]struct{})
	}
	m.channelConnectedOverlays[channelID][overlayID] = struct{}{}

	m.mu.Unlock()

	// Check Redis cache first
	ctx := context.Background()
	cachedVideoID, err := m.repository.GetChannelVideoMapping(ctx, channelID)
	if err == nil && cachedVideoID != "" {
		m.logger.Info("Using cached video ID, starting poller immediately",
			zap.String("channel_id", channelID),
			zap.String("video_id", cachedVideoID),
		)
		// Start poller with cached video ID
		if err := m.startPoller(ctx, channelID, cachedVideoID, overlayID); err != nil {
			m.logger.Error("Failed to start poller with cached video ID",
				zap.String("channel_id", channelID),
				zap.String("video_id", cachedVideoID),
				zap.Error(err),
			)
		}
		return
	}

	// No cache hit, start async discovery
	m.logger.Info("No cached video ID, starting async discovery",
		zap.String("channel_id", channelID),
		zap.String("overlay_id", overlayID),
	)

	// Create discovery state with cancellable context
	discoveryCtx, cancel := context.WithCancel(context.Background())
	state := &DiscoveryState{
		ChannelID:  channelID,
		OverlayID:  overlayID,
		StartedAt:  time.Now(),
		Attempts:   0,
		CancelFunc: cancel,
	}

	m.mu.Lock()
	m.discovering[channelID] = state
	m.mu.Unlock()

	// Start discovery loop in background
	m.wg.Add(1)
	go m.discoveryLoop(discoveryCtx, state)
}

// discoveryLoop attempts discovery with exponential backoff
// Backoff sequence: 30s, 1m, 2m, 5m, 10m (matches official listener pattern)
// 15-minute timeout per user decision
func (m *Manager) discoveryLoop(ctx context.Context, state *DiscoveryState) {
	defer m.wg.Done()

	backoffSequence := []time.Duration{
		30 * time.Second,
		1 * time.Minute,
		2 * time.Minute,
		5 * time.Minute,
		10 * time.Minute,
	}
	timeout := 15 * time.Minute

	deadline := time.Now().Add(timeout)

	for {
		// Check if context cancelled or deadline exceeded
		select {
		case <-ctx.Done():
			m.logger.Info("Discovery cancelled",
				zap.String("channel_id", state.ChannelID),
			)
			m.cleanupDiscoveryState(state.ChannelID)
			return
		default:
		}

		if time.Now().After(deadline) {
			m.logger.Error("Discovery timeout exceeded",
				zap.String("channel_id", state.ChannelID),
				zap.Duration("timeout", timeout),
				zap.Int("attempts", state.Attempts),
			)
			m.cleanupDiscoveryState(state.ChannelID)
			return
		}

		// Attempt discovery
		state.Attempts++
		m.logger.Info("Attempting discovery",
			zap.String("channel_id", state.ChannelID),
			zap.Int("attempt", state.Attempts),
		)

		videoID, err := m.discovery.DiscoverLiveStream(ctx, state.ChannelID)
		if err == nil {
			m.logger.Info("Discovery successful",
				zap.String("channel_id", state.ChannelID),
				zap.String("video_id", videoID),
				zap.Int("attempts", state.Attempts),
				zap.Duration("total_time", time.Since(state.StartedAt)),
			)

			// Persist to Redis cache
			if err := m.repository.SetChannelVideoMapping(ctx, state.ChannelID, videoID); err != nil {
				m.logger.Warn("Failed to cache video ID",
					zap.String("channel_id", state.ChannelID),
					zap.String("video_id", videoID),
					zap.Error(err),
				)
			}

			// Start poller
			if err := m.startPoller(ctx, state.ChannelID, videoID, state.OverlayID); err != nil {
				m.logger.Error("Failed to start poller after discovery",
					zap.String("channel_id", state.ChannelID),
					zap.String("video_id", videoID),
					zap.Error(err),
				)
			}

			m.cleanupDiscoveryState(state.ChannelID)
			return
		}

		// Discovery failed, apply backoff
		attemptIndex := state.Attempts - 1
		if attemptIndex >= len(backoffSequence) {
			attemptIndex = len(backoffSequence) - 1
		}
		backoffDuration := backoffSequence[attemptIndex]

		m.logger.Warn("Discovery failed, applying backoff",
			zap.String("channel_id", state.ChannelID),
			zap.Error(err),
			zap.Duration("backoff", backoffDuration),
			zap.Int("attempt", state.Attempts),
		)

		// Wait with backoff or context cancellation
		timer := time.NewTimer(backoffDuration)
		select {
		case <-ctx.Done():
			timer.Stop()
			m.cleanupDiscoveryState(state.ChannelID)
			return
		case <-timer.C:
			// Continue to next attempt
		}
	}
}

// startPoller starts a poller for a discovered stream
// Claims leadership via LeadershipCoordinator before starting
func (m *Manager) startPoller(ctx context.Context, channelID, videoID, overlayID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if poller already exists
	if _, exists := m.pollers[videoID]; exists {
		m.logger.Debug("Poller already running",
			zap.String("video_id", videoID),
		)
		return nil
	}

	// Claim leadership for this stream
	if m.leader != nil {
		isLeader, err := m.leader.EnsureLeadership(ctx, videoID, func(streamID string) func() {
			// Leadership loss callback
			return func() {
				m.handleLeadershipLoss(context.Background(), streamID)
			}
		}(videoID))
		if err != nil {
			return fmt.Errorf("failed to claim leadership: %w", err)
		}
		if !isLeader {
			m.logger.Debug("Leadership held by another instance",
				zap.String("video_id", videoID),
			)
			return nil
		}
	}

	m.logger.Info("Starting poller for stream",
		zap.String("channel_id", channelID),
		zap.String("video_id", videoID),
		zap.String("overlay_id", overlayID),
	)

	// TODO: Get initial continuation token
	// For MVP, we'll need to implement continuation extraction from video page
	// For now, use placeholder that will trigger offline detection quickly
	initialContinuation := ""

	// Create and start poller
	p := poller.NewPoller(
		m.client,
		initialContinuation,
		channelID,
		m.logger,
		&poller.PollerOptions{
			Interval: 2 * time.Second,
			VideoID:  videoID,
			Metrics:  m.metrics,
		},
	)

	// Set message callback to publish to Redis Streams
	p.SetMessageCallback(func(messages []*innertube.RawChatMessage) {
		for _, msg := range messages {
			if err := m.publisher.Publish(ctx, msg); err != nil {
				m.logger.Error("Failed to publish message",
					zap.String("message_id", msg.MessageID),
					zap.Error(err),
				)
			}
		}
	})

	if err := p.Start(ctx); err != nil {
		if m.leader != nil {
			m.leader.Release(videoID)
		}
		return fmt.Errorf("failed to start poller: %w", err)
	}

	// Track active stream and poller
	stream := &Stream{
		VideoID:      videoID,
		ChannelID:    channelID,
		OverlayID:    overlayID,
		Continuation: initialContinuation,
		StartedAt:    time.Now(),
	}
	m.activeStreams[videoID] = stream
	m.pollers[videoID] = p

	return nil
}

// stopPollerAfterDebounce stops a poller after a debounce delay
// Allows for overlay reconnection before stopping (e.g., page refresh)
func (m *Manager) stopPollerAfterDebounce(channelID string, delay time.Duration) {
	time.Sleep(delay)

	// Check if any overlays still connected for this channel
	m.mu.RLock()
	overlays, exists := m.channelConnectedOverlays[channelID]
	hasConnections := exists && len(overlays) > 0
	m.mu.RUnlock()

	if hasConnections {
		m.logger.Debug("Overlay reconnected during debounce, keeping poller",
			zap.String("channel_id", channelID),
		)
		return
	}

	// Find and stop poller for this channel
	m.mu.Lock()
	defer m.mu.Unlock()

	for videoID, stream := range m.activeStreams {
		if stream.ChannelID == channelID {
			if p, exists := m.pollers[videoID]; exists {
				m.logger.Info("Stopping poller (no connected overlays)",
					zap.String("channel_id", channelID),
					zap.String("video_id", videoID),
				)
				p.Stop()
				delete(m.pollers, videoID)
				delete(m.activeStreams, videoID)

				// Release leadership
				if m.leader != nil {
					m.leader.Release(videoID)
				}

				// Cleanup batch detector state for this channel
				if m.batchDetector != nil {
					if err := m.batchDetector.Cleanup(channelID); err != nil {
						m.logger.Warn("Failed to cleanup batch detector",
							zap.String("channel_id", channelID),
							zap.Error(err),
						)
					}
				}

				// Cleanup deletion buffer for this channel (flush remaining events)
				if m.deletionBuffer != nil {
					m.deletionBuffer.Cleanup(channelID)
				}

				// Clear Redis cache to force rediscovery
				ctx := context.Background()
				if err := m.repository.DeleteChannelVideoMapping(ctx, channelID); err != nil {
					m.logger.Warn("Failed to clear channel video mapping",
						zap.String("channel_id", channelID),
						zap.Error(err),
					)
				}
			}
			break
		}
	}
}

// handleLeadershipLoss handles loss of leadership for a stream
// Called by LeadershipCoordinator when leadership is lost
func (m *Manager) handleLeadershipLoss(ctx context.Context, videoID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Warn("Lost leadership for stream",
		zap.String("video_id", videoID),
	)

	// Get channel ID before deleting stream
	var channelID string
	if stream, exists := m.activeStreams[videoID]; exists {
		channelID = stream.ChannelID
	}

	if p, exists := m.pollers[videoID]; exists {
		p.Stop()
		delete(m.pollers, videoID)
	}
	delete(m.activeStreams, videoID)

	// Cleanup batch detector state for this channel
	if channelID != "" && m.batchDetector != nil {
		if err := m.batchDetector.Cleanup(channelID); err != nil {
			m.logger.Warn("Failed to cleanup batch detector",
				zap.String("channel_id", channelID),
				zap.Error(err),
			)
		}
	}

	// Cleanup deletion buffer for this channel
	if channelID != "" && m.deletionBuffer != nil {
		m.deletionBuffer.Cleanup(channelID)
	}
}

// cleanupDiscoveryState removes discovery state after completion or cancellation
func (m *Manager) cleanupDiscoveryState(channelID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.discovering, channelID)
}

// periodicSync periodically syncs active sources (fallback for PostgreSQL LISTEN)
func (m *Manager) periodicSync(ctx context.Context) {
	defer m.wg.Done()

	// Perform initial sync immediately
	m.syncSources(ctx)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.syncSources(ctx)
		case <-m.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// syncSources queries source-manager for active YouTube sources and starts discovery
func (m *Manager) syncSources(ctx context.Context) {
	if m.smClient == nil {
		m.logger.Debug("No source-manager client configured, skipping sync")
		return
	}

	sources, err := m.smClient.GetSources(ctx, "youtube")
	if err != nil {
		m.logger.Error("Failed to query source-manager for YouTube sources",
			zap.Error(err),
		)
		return
	}

	m.logger.Debug("Synced YouTube sources from source-manager",
		zap.Int("source_count", len(sources)),
	)

	// Group sources by channel to handle multiple overlays for same channel
	channelOverlays := make(map[string][]string)
	for _, source := range sources {
		if source.IsActive {
			channelOverlays[source.ChannelID] = append(channelOverlays[source.ChannelID], source.OverlayID)
		}
	}

	// For each channel, ensure we have a poller or discovery in progress
	for channelID, overlayIDs := range channelOverlays {
		m.mu.RLock()
		// Check if we're already discovering or polling this channel
		_, isDiscovering := m.discovering[channelID]
		isPolling := false
		for _, stream := range m.activeStreams {
			if stream.ChannelID == channelID {
				isPolling = true
				break
			}
		}
		m.mu.RUnlock()

		if !isDiscovering && !isPolling {
			// Start async discovery for this channel
			m.logger.Info("Starting async discovery for new YouTube source",
				zap.String("channel_id", channelID),
				zap.Strings("overlay_ids", overlayIDs),
			)
			// Use first overlay ID for discovery (all overlays will get messages once polling starts)
			m.startAsyncDiscovery(channelID, overlayIDs[0])
		}
	}
}

// Shutdown gracefully shuts down the manager
// Stops all pollers, cancels discovery goroutines, releases leadership
func (m *Manager) Shutdown(ctx context.Context) error {
	m.logger.Info("Shutting down stream manager")

	// Signal stop
	close(m.stopChan)

	// Cancel all discovery goroutines
	m.mu.Lock()
	for _, state := range m.discovering {
		state.CancelFunc()
	}
	m.mu.Unlock()

	// Stop all pollers
	m.mu.Lock()
	for videoID, p := range m.pollers {
		m.logger.Info("Stopping poller",
			zap.String("video_id", videoID),
		)
		p.Stop()

		// Release leadership
		if m.leader != nil {
			m.leader.Release(videoID)
		}
	}
	m.pollers = make(map[string]*poller.Poller)
	m.activeStreams = make(map[string]*Stream)
	m.mu.Unlock()

	// Wait for goroutines with timeout
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		m.logger.Info("Stream manager shutdown complete")
		return nil
	case <-time.After(25 * time.Second):
		m.logger.Warn("Stream manager shutdown timeout exceeded")
		return fmt.Errorf("shutdown timeout")
	}
}
