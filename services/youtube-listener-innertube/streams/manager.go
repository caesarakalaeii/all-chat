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
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener-innertube/deletion"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/innertube"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/metrics"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/poller"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/publisher"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/status"
	"github.com/caesar/all-chat/shared/sourcemanager"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// errLeadershipHeld is returned by startPoller when another instance holds leadership
// for the stream. It is NOT a real error — the caller should fall through to discovery
// to check if the cached video ID is stale (i.e. the old stream ended and a new one started).
var errLeadershipHeld = errors.New("leadership held by another instance")

// Source represents a YouTube source configuration from source-manager
type Source struct {
	ChannelID    string
	OverlayID    string
	StreamSelect string // Stream selection strategy (e.g. "most_viewers")
	StreamMatch  string // Match term for title_match strategy
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
	ChannelID        string
	OverlayID        string
	StreamSelect     string // Stream selection strategy
	StreamMatch      string // Match term for title_match
	StartedAt        time.Time
	Attempts         int
	CancelFunc       context.CancelFunc
	ResetBackoffChan chan struct{} // Signal to reset backoff when other platforms go live
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
	statusPublisher *status.Publisher
	batchDetector   *deletion.BatchDetector  // Batch deletion detector for cleanup
	deletionBuffer  *deletion.DeletionBuffer // Deletion event buffer for cleanup

	mu               sync.RWMutex
	activeStreams    map[string]*Stream         // videoID → stream state
	pollers          map[string]*poller.Poller  // videoID → active poller
	discovering      map[string]*DiscoveryState // channelID → discovery state
	connectedOverlays map[string]time.Time      // overlay_id → connection_time
	channelConnectedOverlays map[string]map[string]struct{} // channel_id → overlay_ids

	// Demand-driven gating (Phase 5 gap closure)
	demandMu        sync.RWMutex
	demandedChannels map[string]bool // channelID -> has demand; nil = no filtering (backward compat)

	// Discovery give-up tracking: channels whose discovery loop stopped after
	// maxDiscoveryDuration of fruitless polling. Periodic sync skips these until a
	// refresh (demand re-assertion) clears the marker. Guarded by gaveUpMu.
	gaveUpMu        sync.Mutex
	gaveUpDiscovery map[string]bool

	// Per-channel debounce timers for demand-loss. Cancelled if demand is restored
	// before the timer fires, preventing brief WebSocket disconnects from killing
	// the poller and losing messages that the api-gateway replay buffer captured.
	demandStopMu     sync.Mutex
	demandStopTimers map[string]*time.Timer // channelID -> pending stop timer

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
		statusPublisher:          status.NewPublisher(redisClient, logger),
		batchDetector:            batchDetector,
		deletionBuffer:           deletionBuffer,
		activeStreams:            make(map[string]*Stream),
		pollers:                  make(map[string]*poller.Poller),
		discovering:              make(map[string]*DiscoveryState),
		connectedOverlays:        make(map[string]time.Time),
		channelConnectedOverlays: make(map[string]map[string]struct{}),
		gaveUpDiscovery:          make(map[string]bool),
		demandStopTimers:         make(map[string]*time.Timer),
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
		m.startAsyncDiscovery(source.ChannelID, overlayID, DiscoveryOpts{})
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

// DiscoveryOpts holds optional parameters for stream discovery.
type DiscoveryOpts struct {
	StreamSelect string // Stream selection strategy (e.g. "most_viewers")
	StreamMatch  string // Match term for title_match strategy
}

// startAsyncDiscovery starts background discovery for a channel
// Checks Redis cache first, falls back to HTML discovery
func (m *Manager) startAsyncDiscovery(channelID, overlayID string, opts DiscoveryOpts) {
	streamSelect := opts.StreamSelect
	streamMatch := opts.StreamMatch

	// Build the discovery state up front so we can reserve the m.discovering slot
	// synchronously, before any slow work. Previously the slot was only written
	// *after* a 0-5s jitter sleep and a Redis round-trip, leaving a multi-second
	// TOCTOU window in which concurrent callers (periodic sync, overlay-connect,
	// demand updates) each passed the guard and spawned a duplicate discovery
	// loop. Those loops leaked and hammered YouTube independently — observed as
	// 8+ concurrent loops scraping the same offline channel.
	discoveryCtx, cancel := context.WithCancel(context.Background())
	state := &DiscoveryState{
		ChannelID:        channelID,
		OverlayID:        overlayID,
		StreamSelect:     streamSelect,
		StreamMatch:      streamMatch,
		StartedAt:        time.Now(),
		Attempts:         0,
		CancelFunc:       cancel,
		ResetBackoffChan: make(chan struct{}, 1), // Buffered to prevent blocking
	}

	m.mu.Lock()

	// Check if already discovering
	if existing, exists := m.discovering[channelID]; exists {
		m.logger.Debug("Discovery already in progress",
			zap.String("channel_id", channelID),
			zap.String("existing_overlay_id", existing.OverlayID),
		)
		m.mu.Unlock()
		cancel()
		return
	}

	// Track channel→overlay connection
	if _, exists := m.channelConnectedOverlays[channelID]; !exists {
		m.channelConnectedOverlays[channelID] = make(map[string]struct{})
	}
	m.channelConnectedOverlays[channelID][overlayID] = struct{}{}

	// Reserve the discovery slot now, before the jitter/Redis work below, so a
	// concurrent caller observes it as in-progress and bails out.
	m.discovering[channelID] = state

	m.mu.Unlock()

	// Jitter to avoid thundering-herd on YouTube watch page when many channels start simultaneously
	// (e.g. pod restart with 20+ channels). Random 0-5s spread reduces 429 rate limiting.
	jitter := time.Duration(rand.Intn(5000)) * time.Millisecond
	time.Sleep(jitter)

	// Check Redis cache first
	ctx := context.Background()
	cachedVideoID, err := m.repository.GetChannelVideoMapping(ctx, channelID)
	if err == nil && cachedVideoID != "" {
		m.logger.Info("Using cached video ID, starting poller immediately",
			zap.String("channel_id", channelID),
			zap.String("video_id", cachedVideoID),
		)
		// Start poller with cached video ID.
		// If another instance already holds leadership (errLeadershipHeld) we still fall
		// through to async discovery: the other instance might be polling a stale/ended
		// stream whose video ID was never evicted from the cache (e.g. stream ended without
		// a liveChatEnded error). Discovery will find the new video ID and overwrite the cache.
		if err := m.startPoller(ctx, channelID, cachedVideoID, overlayID); err != nil {
			if !errors.Is(err, errLeadershipHeld) {
				m.logger.Error("Failed to start poller with cached video ID, falling back to discovery",
					zap.String("channel_id", channelID),
					zap.String("video_id", cachedVideoID),
					zap.Error(err),
				)
			}
			// Fall through to async discovery below (covers both real errors and leadership contention)
		} else {
			// Poller started from cache — release the discovery reservation we
			// took above and stop, no discovery loop needed.
			m.cleanupDiscoveryState(channelID)
			cancel()
			return
		}
	}

	// No cache hit, start async discovery
	m.logger.Info("No cached video ID, starting async discovery",
		zap.String("channel_id", channelID),
		zap.String("overlay_id", overlayID),
	)

	// Subscribe to cross-platform events for this overlay
	m.wg.Add(1)
	go m.subscribeToPlatformEvents(discoveryCtx, state)

	// Start discovery loop in background
	m.wg.Add(1)
	go m.discoveryLoop(discoveryCtx, state)
}

// maxDiscoveryDuration caps how long discovery will poll YouTube for a single
// channel before giving up. A channel that is simply offline (streamer not
// live) would otherwise be scraped from YouTube every ~60s forever; across many
// such channels that is an unacceptable, near-DDoS request volume against
// YouTube's InnerTube endpoints. After this much wall-clock time of fruitless
// polling the loop stops, surfaces an error on the platform indicator, and waits
// for a refresh — a demand re-assertion (overlay reconnect/page refresh), see
// clearGaveUpForDemandChanges — before polling again.
const maxDiscoveryDuration = 1 * time.Hour

// discoveryLoop attempts discovery with exponential backoff.
// Backoff sequence: 10s, 20s, 30s, 60s — aggressive detection capped at 1 minute so a
// channel going live is picked up within ~1m. The loop gives up after
// maxDiscoveryDuration of fruitless polling (see giveUpDiscovery) so we never hammer
// YouTube indefinitely for an offline channel. Discovery only runs for demanded
// channels (an overlay is connected), so the 1m cap stays well within YouTube's
// tolerance for the InnerTube channel-page scrape.
func (m *Manager) discoveryLoop(ctx context.Context, state *DiscoveryState) {
	defer m.wg.Done()

	// Exponential backoff capped at 1 minute. Keep polling indefinitely until a stream is
	// discovered or the source is deactivated.
	backoffSequence := []time.Duration{
		10 * time.Second,
		20 * time.Second,
		30 * time.Second,
		60 * time.Second, // Max backoff - continue at 1m intervals
	}

	for {
		// Check if context cancelled (source deactivated or service shutdown)
		select {
		case <-ctx.Done():
			m.logger.Info("Discovery cancelled",
				zap.String("channel_id", state.ChannelID),
				zap.Int("attempts", state.Attempts),
			)
			m.cleanupDiscoveryState(state.ChannelID)
			return
		default:
		}

		// Hard wall-clock cap: stop polling YouTube once we've spent
		// maxDiscoveryDuration looking for a stream that never appeared. Surface
		// an error and park until a refresh, rather than scraping forever.
		if time.Since(state.StartedAt) >= maxDiscoveryDuration {
			m.giveUpDiscovery(state)
			return
		}

		// Attempt discovery
		state.Attempts++
		if m.metrics != nil {
			m.metrics.DiscoveryAttempts.WithLabelValues(metrics.ServiceLabel, state.ChannelID).Inc()
		}
		m.logger.Info("Attempting discovery",
			zap.String("channel_id", state.ChannelID),
			zap.Int("attempt", state.Attempts),
			zap.String("strategy", state.StreamSelect),
		)

		// discoveryErr captures why this attempt failed so the backoff warning can
		// surface it. Without this, schema regressions (e.g. YouTube replacing
		// videoRenderer with lockupViewModel) look identical to "channel offline".
		var discoveryErr error

		// Multi-stream strategies: discover and poll every matching stream
		if innertube.IsMultiStreamStrategy(state.StreamSelect) {
			titleFilter := ""
			if state.StreamSelect == innertube.StrategyTitleMatchAll {
				titleFilter = state.StreamMatch
			}
			videoIDs, err := m.discovery.DiscoverAllLiveStreams(ctx, state.ChannelID, titleFilter)
			if err == nil {
				m.logger.Info("All-streams discovery successful",
					zap.String("channel_id", state.ChannelID),
					zap.Int("stream_count", len(videoIDs)),
					zap.Int("attempts", state.Attempts),
				)
				started := 0
				for _, vid := range videoIDs {
					if err := m.repository.SetChannelVideoMapping(ctx, state.ChannelID, vid); err != nil {
						m.logger.Warn("Failed to cache video ID", zap.String("video_id", vid), zap.Error(err))
					}
					if err := m.startPoller(ctx, state.ChannelID, vid, state.OverlayID); err != nil {
						if !errors.Is(err, errLeadershipHeld) {
							m.logger.Warn("Failed to start poller for stream", zap.String("video_id", vid), zap.Error(err))
						}
					} else {
						started++
					}
				}
				if started > 0 {
					m.cleanupDiscoveryState(state.ChannelID)
					return
				}
				// All pollers failed (leadership held etc.) — fall through to backoff
				discoveryErr = errors.New("all pollers failed to start (leadership contention)")
			} else {
				discoveryErr = err
			}
		} else {
			// Single-stream strategies
			videoID, err := m.discovery.DiscoverLiveStream(ctx, state.ChannelID, state.StreamSelect, state.StreamMatch)
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
					if errors.Is(err, errLeadershipHeld) {
						m.logger.Info("Another instance claimed leadership for newly-discovered stream",
							zap.String("channel_id", state.ChannelID),
							zap.String("video_id", videoID),
						)
						m.cleanupDiscoveryState(state.ChannelID)
						return
					}
					m.logger.Error("Failed to start poller after discovery, will retry",
						zap.String("channel_id", state.ChannelID),
						zap.String("video_id", videoID),
						zap.Error(err),
					)
					discoveryErr = err
					// Fall through to backoff and retry the whole discovery+poller process
				} else {
					m.cleanupDiscoveryState(state.ChannelID)
					return
				}
			} else {
				discoveryErr = err
			}
		}

		// Discovery failed or all pollers failed — apply backoff
		attemptIndex := state.Attempts - 1
		if attemptIndex >= len(backoffSequence) {
			attemptIndex = len(backoffSequence) - 1
		}
		backoffDuration := backoffSequence[attemptIndex]

		m.logger.Warn("Discovery failed, applying backoff",
			zap.String("channel_id", state.ChannelID),
			zap.Duration("backoff", backoffDuration),
			zap.Int("attempt", state.Attempts),
			zap.Error(discoveryErr),
		)

		// Notify overlay: channel is reconnecting (no live stream found yet)
		nextRetry := time.Now().Add(backoffDuration)
		m.statusPublisher.Publish(ctx, status.Message{
			Platform:     "youtube",
			ChannelID:    state.ChannelID,
			Status:       "reconnecting",
			NextRetryAt:  &nextRetry,
			ErrorMessage: "Searching for stream",
		})

		// Wait with backoff, reset signal, or context cancellation
		timer := time.NewTimer(backoffDuration)
		select {
		case <-ctx.Done():
			timer.Stop()
			m.cleanupDiscoveryState(state.ChannelID)
			return
		case <-state.ResetBackoffChan:
			// Cross-platform trigger: another platform went live, retry immediately.
			// Reset the attempt counter so a dormant channel returns to aggressive
			// discovery — a sibling platform going live is a strong signal this
			// streamer is about to be live too.
			timer.Stop()
			state.Attempts = 0
			m.logger.Info("Backoff reset by cross-platform event, retrying discovery immediately",
				zap.String("channel_id", state.ChannelID),
				zap.String("overlay_id", state.OverlayID),
			)
			// Continue to next attempt immediately
		case <-timer.C:
			// Backoff elapsed, continue to next attempt
		}
	}
}

// giveUpDiscovery stops discovery for a channel that has been polled for
// maxDiscoveryDuration without finding a live stream. It surfaces an error on
// the platform indicator and removes the channel from the in-progress set so a
// refresh (overlay reconnect → demand re-assertion) can restart discovery. The
// gave-up marker prevents the periodic source sync from immediately restarting
// the loop — the channel stays parked until that refresh arrives.
func (m *Manager) giveUpDiscovery(state *DiscoveryState) {
	m.logger.Warn("Discovery gave up after max duration, awaiting refresh to avoid hammering YouTube",
		zap.String("channel_id", state.ChannelID),
		zap.Int("attempts", state.Attempts),
		zap.Duration("max_duration", maxDiscoveryDuration),
	)

	if m.metrics != nil {
		m.metrics.DiscoveryGaveUp.WithLabelValues(metrics.ServiceLabel, state.ChannelID).Inc()
	}

	ctx := context.Background()

	// Surface an error on the platform indicator.
	m.statusPublisher.Publish(ctx, status.Message{
		Platform:     "youtube",
		ChannelID:    state.ChannelID,
		Status:       "error",
		ErrorMessage: "No live stream found after 1h — refresh your overlay to retry",
	})

	// Mark gave-up first, then drop the in-progress reservation, so there is no
	// window in which the channel looks idle to a concurrent periodic sync.
	m.markDiscoveryGaveUp(state.ChannelID)
	m.cleanupDiscoveryState(state.ChannelID)

	// Stop the cross-platform subscription goroutine for this discovery.
	if state.CancelFunc != nil {
		state.CancelFunc()
	}
}

// markDiscoveryGaveUp records that discovery for a channel has stopped and is
// awaiting a refresh.
func (m *Manager) markDiscoveryGaveUp(channelID string) {
	m.gaveUpMu.Lock()
	m.gaveUpDiscovery[channelID] = true
	m.gaveUpMu.Unlock()
}

// hasDiscoveryGivenUp reports whether discovery for a channel is parked awaiting
// a refresh.
func (m *Manager) hasDiscoveryGivenUp(channelID string) bool {
	m.gaveUpMu.Lock()
	defer m.gaveUpMu.Unlock()
	return m.gaveUpDiscovery[channelID]
}

// clearGaveUpForDemandChanges treats any change in a channel's demand status
// (newly demanded, or demand lost) as the refresh that re-enables discovery: it
// drops the gave-up marker so the next sync can poll YouTube again. A channel
// that stays continuously demanded keeps its marker — that is the "wait for a
// refresh" behaviour for a streamer who is simply offline for a long time.
func (m *Manager) clearGaveUpForDemandChanges(prev, demanded map[string]bool) {
	m.gaveUpMu.Lock()
	defer m.gaveUpMu.Unlock()
	for ch := range m.gaveUpDiscovery {
		// Indexing a nil map yields false, so this is safe when either side is nil.
		if prev[ch] != demanded[ch] {
			delete(m.gaveUpDiscovery, ch)
			m.logger.Info("Demand change detected, clearing discovery give-up marker",
				zap.String("channel_id", ch))
		}
	}
}

// subscribeToPlatformEvents subscribes to cross-platform events for an overlay
// and signals backoff reset when other platforms go live
func (m *Manager) subscribeToPlatformEvents(ctx context.Context, state *DiscoveryState) {
	defer m.wg.Done()

	eventChannel := fmt.Sprintf("platform:event:%s", state.OverlayID)
	pubsub := m.redisClient.Subscribe(ctx, eventChannel)
	defer pubsub.Close()

	m.logger.Info("Subscribed to cross-platform events",
		zap.String("channel_id", state.ChannelID),
		zap.String("overlay_id", state.OverlayID),
		zap.String("event_channel", eventChannel),
	)

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-pubsub.Channel():
			// Another platform went live on this overlay - signal backoff reset
			m.logger.Info("Cross-platform event received, signaling backoff reset",
				zap.String("channel_id", state.ChannelID),
				zap.String("overlay_id", state.OverlayID),
				zap.String("event", msg.Payload),
			)

			// Non-blocking send to avoid deadlock
			select {
			case state.ResetBackoffChan <- struct{}{}:
			default:
				// Channel full or discovery not waiting - ignore
			}
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
			return errLeadershipHeld
		}
	}

	m.logger.Info("Starting poller for stream",
		zap.String("channel_id", channelID),
		zap.String("video_id", videoID),
		zap.String("overlay_id", overlayID),
	)

	// Get initial continuation token from watch page
	initialContinuation, visitorData, err := m.discovery.GetInitialContinuation(ctx, videoID, channelID)
	if err != nil {
		m.logger.Error("Failed to get initial continuation token",
			zap.String("video_id", videoID),
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		if m.leader != nil {
			m.leader.Release(videoID)
		}
		// If the stream has ended (isReplay or no liveChatRenderer), clear the Redis
		// cache so discovery will find the new live stream instead of retrying the ended one.
		// For transient errors (429, network), keep the cache so we retry the same video.
		errStr := err.Error()
		if strings.Contains(errStr, "stream may have ended") || strings.Contains(errStr, "isReplay") {
			if delErr := m.repository.DeleteChannelVideoMapping(ctx, channelID); delErr != nil {
				m.logger.Warn("Failed to clear stale video mapping", zap.Error(delErr))
			}
		}
		return fmt.Errorf("get initial continuation: %w", err)
	}

	// Create and start poller
	p := poller.NewPoller(
		m.client,
		initialContinuation,
		channelID,
		m.logger,
		&poller.PollerOptions{
			Interval:            2 * time.Second,
			VideoID:             videoID,
			VisitorData:         visitorData,
			Metrics:             m.metrics,
			Refresher:           m.discovery,
			ZeroActionThreshold: 150, // ~5 minutes at 2s floor interval
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

	// Notify overlay: channel is now connected
	m.statusPublisher.Publish(context.Background(), status.Message{
		Platform:  "youtube",
		ChannelID: channelID,
		Status:    "connected",
	})

	// Watch for self-termination (e.g. stream went offline detected inside poller).
	// When the poller exits on its own, clean up maps and release leadership so
	// syncSources can trigger rediscovery.
	go func() {
		<-p.IsDone()

		m.mu.Lock()
		// Only clean up if this specific poller is still the registered one
		// (it might have already been replaced by stopPollerAfterDebounce).
		if current, exists := m.pollers[videoID]; exists && current == p {
			delete(m.pollers, videoID)
			delete(m.activeStreams, videoID)
			m.logger.Info("Poller self-terminated, cleaned up state",
				zap.String("channel_id", channelID),
				zap.String("video_id", videoID),
				zap.String("final_state", string(p.GetState())),
			)
		}
		m.mu.Unlock()

		if m.leader != nil {
			m.leader.Release(videoID)
		}
	}()

	// Mark source active in the DB so the cleanup job doesn't deactivate it.
	// Fire-and-forget: log on failure but don't block poller startup.
	if m.smClient != nil {
		go func() {
			if err := m.smClient.ActivateSource(context.Background(), "youtube", channelID); err != nil {
				m.logger.Warn("Failed to activate source in DB",
					zap.String("channel_id", channelID),
					zap.Error(err),
				)
			}
		}()
	}

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

				// Notify overlay: channel is now offline
				m.statusPublisher.Publish(ctx, status.Message{
					Platform:  "youtube",
					ChannelID: channelID,
					Status:    "offline",
				})

				// Mark source inactive so admin panel reflects actual state immediately
				// rather than waiting up to 24 h for the cleanup job.
				if m.smClient != nil {
					go func(chID string) {
						if err := m.smClient.DeactivateSource(context.Background(), "youtube", chID); err != nil {
							m.logger.Warn("Failed to deactivate source in DB",
								zap.String("channel_id", chID),
								zap.Error(err),
							)
						}
					}(channelID)
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

	// Notify overlay: channel is offline (leadership lost, other instance takes over)
	if channelID != "" {
		m.statusPublisher.Publish(ctx, status.Message{
			Platform:  "youtube",
			ChannelID: channelID,
			Status:    "offline",
		})

		// Mark source inactive. The pod that wins the new leadership election will
		// call ActivateSource when it starts its poller; this pod must deactivate
		// to avoid both appearing active simultaneously.
		if m.smClient != nil {
			go func(chID string) {
				if err := m.smClient.DeactivateSource(context.Background(), "youtube", chID); err != nil {
					m.logger.Warn("Failed to deactivate source after leadership loss",
						zap.String("channel_id", chID),
						zap.Error(err),
					)
				}
			}(channelID)
		}
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

	// Rebalance leadership leases before acquiring new ones.
	// Released video IDs will have their pollers stopped below.
	if m.leader != nil {
		if released, err := m.leader.Rebalance(ctx, len(sources)); err != nil {
			m.logger.Warn("Leadership rebalance failed", zap.Error(err))
		} else if len(released) > 0 {
			m.logger.Info("Rebalanced: released excess streams",
				zap.Int("released", len(released)),
				zap.Int("total_sources", len(sources)),
			)
			// Stop pollers for released video IDs so another pod can take over
			m.mu.Lock()
			for _, videoID := range released {
				if p, exists := m.pollers[videoID]; exists {
					m.logger.Info("Stopping poller (rebalanced away)",
						zap.String("video_id", videoID),
					)
					p.Stop()
					delete(m.pollers, videoID)
					delete(m.activeStreams, videoID)
				}
			}
			m.mu.Unlock()
		}
	}

	m.logger.Debug("Synced YouTube sources from source-manager",
		zap.Int("source_count", len(sources)),
	)

	// channelSourceInfo carries per-channel overlay IDs and stream selection config.
	// When multiple overlays share a channel, the first source's strategy is used.
	type channelSourceInfo struct {
		OverlayIDs   []string
		StreamSelect string
		StreamMatch  string
	}

	// Group sources by channel to handle multiple overlays for same channel
	channelOverlays := make(map[string]*channelSourceInfo)
	for _, source := range sources {
		if source.IsActive {
			info, exists := channelOverlays[source.ChannelID]
			if !exists {
				info = &channelSourceInfo{
					StreamSelect: source.StreamSelect,
					StreamMatch:  source.StreamMatch,
				}
				channelOverlays[source.ChannelID] = info
			}
			info.OverlayIDs = append(info.OverlayIDs, source.OverlayID)
		}
	}

	// Filter by demand: only process channels with active overlay demand
	m.demandMu.RLock()
	demanded := m.demandedChannels
	m.demandMu.RUnlock()

	if demanded != nil {
		for channelID := range channelOverlays {
			if _, ok := demanded[channelID]; !ok {
				delete(channelOverlays, channelID)
			}
		}
		m.logger.Debug("Filtered sources by demand",
			zap.Int("demanded_channels", len(demanded)),
			zap.Int("active_channels", len(channelOverlays)))
	}

	// For each channel, ensure we have a poller or discovery in progress
	for channelID, info := range channelOverlays {
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

		if isPolling {
			// Heartbeat: keep ocs.is_active = true in the DB while we are actively polling.
			// This ensures (a) new overlay sources added while the poller is already running
			// get activated, and (b) the cleanup job doesn't mark sources stale after 24 h.
			go func(chID string, ids []string) {
				if err := m.smClient.ActivateSource(context.Background(), "youtube", chID); err != nil {
					m.logger.Warn("Failed to heartbeat-activate source",
						zap.String("channel_id", chID),
						zap.Error(err),
					)
				}
			}(channelID, info.OverlayIDs)
			continue
		}

		// Skip channels whose discovery gave up after maxDiscoveryDuration. They
		// stay parked until a refresh (demand re-assertion) clears the marker.
		if m.hasDiscoveryGivenUp(channelID) {
			continue
		}

		if !isDiscovering {
			// Before starting discovery, check if another pod already holds leadership
			// for a known video ID. If so, skip — we don't need to discover or publish
			// spurious "reconnecting" statuses when another replica is actively polling.
			anotherPodPolling := false
			if cachedVideoID, err := m.repository.GetChannelVideoMapping(ctx, channelID); err == nil && cachedVideoID != "" {
				leaderKey := "leader:youtube:" + cachedVideoID
				if val, err := m.redisClient.Get(ctx, leaderKey).Result(); err == nil && val != "" {
					anotherPodPolling = true
					m.logger.Debug("Another pod holds leadership for channel, skipping discovery",
						zap.String("channel_id", channelID),
						zap.String("video_id", cachedVideoID),
					)
				}
			}

			if !anotherPodPolling {
				m.logger.Info("Starting async discovery for new YouTube source",
					zap.String("channel_id", channelID),
					zap.Strings("overlay_ids", info.OverlayIDs),
					zap.String("stream_select", info.StreamSelect),
				)
				m.startAsyncDiscovery(channelID, info.OverlayIDs[0], DiscoveryOpts{
					StreamSelect: info.StreamSelect,
					StreamMatch:  info.StreamMatch,
				})
			}
		}
	}
}

// demandStopDebounce is how long we wait after demand-loss before actually
// stopping a poller. Must exceed any realistic transient WebSocket disconnect
// (page refresh, network blip, browser tab backgrounded). Set to 5 minutes:
//   - WebSocket disconnect grace period is 60s, so a real reconnect re-establishes
//     demand within ~60s
//   - Browser tab backgrounding can suspend the tab for minutes on mobile
//   - The api-gateway replay buffer covers this same window so messages produced
//     by the still-running poller are replayed to the client on reconnect
const demandStopDebounce = 5 * time.Minute

// UpdateDemandedChannels receives the set of channel IDs that currently have overlay demand.
// nil means no demand filtering (backward compat). Empty map means zero demand.
// Channels that lose demand have their poller stopped after a 5-minute debounce;
// channels that gain demand trigger an immediate syncSources to start discovery.
func (m *Manager) UpdateDemandedChannels(demanded map[string]bool) {
	m.demandMu.Lock()
	prev := m.demandedChannels
	m.demandedChannels = demanded
	m.demandMu.Unlock()

	m.logger.Info("Demanded channels updated",
		zap.Int("demanded_count", len(demanded)))

	// A change in demand (overlay reconnect/refresh) is the "refresh" that
	// re-enables discovery for channels parked after maxDiscoveryDuration.
	m.clearGaveUpForDemandChanges(prev, demanded)

	// Cancel any pending stop timers for channels whose demand has been restored.
	// This is the key reconnect path: if a brief disconnect scheduled a stop and
	// the overlay reconnects within the debounce window, the timer is cancelled
	// and the poller keeps running uninterrupted.
	m.cancelStopTimersForRestoredDemand(demanded)

	// Detect newly-demanded channels (present in new set, absent in old set).
	// These need discovery immediately — don't wait for the next 30s periodic sync.
	hasNewChannels := false
	if demanded != nil {
		for ch := range demanded {
			if prev == nil || !prev[ch] {
				hasNewChannels = true
				break
			}
		}
	}
	if hasNewChannels {
		ctx := context.Background()
		go m.syncSources(ctx)
	}

	// Schedule deferred stops for channels that lost demand, cancel in-progress discovery.
	m.reconcileDemand()
}

// cancelStopTimersForRestoredDemand stops any pending stop timer for channels
// that are in the new demanded set. Called when a demand update arrives.
func (m *Manager) cancelStopTimersForRestoredDemand(demanded map[string]bool) {
	if demanded == nil {
		return
	}
	m.demandStopMu.Lock()
	defer m.demandStopMu.Unlock()
	for channelID := range demanded {
		if timer, exists := m.demandStopTimers[channelID]; exists {
			if timer.Stop() {
				m.logger.Info("Demand restored during debounce, cancelling pending poller stop",
					zap.String("channel_id", channelID))
			}
			delete(m.demandStopTimers, channelID)
		}
	}
}

// reconcileDemand schedules deferred poller stops for channels that lost demand,
// and cancels in-progress discovery for the same set. The 5-minute debounce lets
// brief WebSocket disconnects recover before the poller is killed.
func (m *Manager) reconcileDemand() {
	m.demandMu.RLock()
	demanded := m.demandedChannels
	m.demandMu.RUnlock()

	if demanded == nil {
		return // nil = no filtering (backward compat)
	}

	// Collect channels that lost demand.
	m.mu.RLock()
	var lostPolledChannels []string
	for _, stream := range m.activeStreams {
		if _, ok := demanded[stream.ChannelID]; !ok {
			lostPolledChannels = append(lostPolledChannels, stream.ChannelID)
		}
	}
	var lostDiscoveries []string
	for channelID := range m.discovering {
		if _, ok := demanded[channelID]; !ok {
			lostDiscoveries = append(lostDiscoveries, channelID)
		}
	}
	m.mu.RUnlock()

	// Schedule deferred stop for each polled channel that lost demand.
	// If the channel already has a pending timer, leave it — the first scheduled
	// stop time stands so a flapping overlay doesn't extend the polling window
	// indefinitely.
	for _, channelID := range lostPolledChannels {
		m.scheduleDeferredStop(channelID)
	}

	// Cancel discovery immediately — it hasn't published yet so there's no
	// message continuity to preserve, and we save YouTube watch-page calls.
	if len(lostDiscoveries) > 0 {
		m.mu.Lock()
		for _, channelID := range lostDiscoveries {
			if state, exists := m.discovering[channelID]; exists {
				m.demandMu.RLock()
				_, stillDemanded := m.demandedChannels[channelID]
				m.demandMu.RUnlock()
				if !stillDemanded {
					state.CancelFunc()
					delete(m.discovering, channelID)
					m.logger.Info("Demand lost, cancelling discovery",
						zap.String("channel_id", channelID))
				}
			}
		}
		m.mu.Unlock()
	}
}

// scheduleDeferredStop registers a 5-minute timer that will stop the poller for
// channelID unless demand is restored before the timer fires. Idempotent — a
// second call while a timer is already pending does nothing.
func (m *Manager) scheduleDeferredStop(channelID string) {
	m.demandStopMu.Lock()
	if _, exists := m.demandStopTimers[channelID]; exists {
		// Already scheduled — keep the original deadline.
		m.demandStopMu.Unlock()
		return
	}
	timer := time.AfterFunc(demandStopDebounce, func() {
		m.executeDeferredStop(channelID)
	})
	m.demandStopTimers[channelID] = timer
	m.demandStopMu.Unlock()

	m.logger.Info("Demand lost, scheduling deferred poller stop",
		zap.String("channel_id", channelID),
		zap.Duration("debounce", demandStopDebounce))
}

// executeDeferredStop fires when the 5-minute debounce elapses. It re-checks
// demand under lock, and only stops the poller if demand is still absent.
func (m *Manager) executeDeferredStop(channelID string) {
	m.demandStopMu.Lock()
	delete(m.demandStopTimers, channelID)
	m.demandStopMu.Unlock()

	m.demandMu.RLock()
	demanded := m.demandedChannels
	m.demandMu.RUnlock()

	if demanded == nil {
		return
	}
	if _, ok := demanded[channelID]; ok {
		m.logger.Info("Demand restored before debounce fired, keeping poller",
			zap.String("channel_id", channelID))
		return
	}

	// Demand still absent — stop the poller.
	m.mu.Lock()
	defer m.mu.Unlock()

	for videoID, stream := range m.activeStreams {
		if stream.ChannelID != channelID {
			continue
		}
		if p, exists := m.pollers[videoID]; exists {
			p.Stop()
			delete(m.pollers, videoID)
			m.logger.Info("Demand lost, stopping poller after debounce",
				zap.String("channel_id", channelID),
				zap.String("video_id", videoID))
		}
		delete(m.activeStreams, videoID)

		if m.leader != nil {
			m.leader.Release(videoID)
		}
		if m.batchDetector != nil {
			if err := m.batchDetector.Cleanup(channelID); err != nil {
				m.logger.Warn("Failed to cleanup batch detector",
					zap.String("channel_id", channelID),
					zap.Error(err))
			}
		}
		if m.deletionBuffer != nil {
			m.deletionBuffer.Cleanup(channelID)
		}

		ctx := context.Background()
		if err := m.repository.DeleteChannelVideoMapping(ctx, channelID); err != nil {
			m.logger.Warn("Failed to clear channel video mapping",
				zap.String("channel_id", channelID),
				zap.Error(err))
		}
		m.statusPublisher.Publish(ctx, status.Message{
			Platform:  "youtube",
			ChannelID: channelID,
			Status:    "offline",
		})

		if m.smClient != nil {
			chID := channelID
			go func() {
				if err := m.smClient.DeactivateSource(context.Background(), "youtube", chID); err != nil {
					m.logger.Warn("Failed to deactivate source after demand loss",
						zap.String("channel_id", chID),
						zap.Error(err))
				}
			}()
		}
		break
	}
}

// Shutdown gracefully shuts down the manager
// Stops all pollers, cancels discovery goroutines, releases leadership
func (m *Manager) Shutdown(ctx context.Context) error {
	m.logger.Info("Shutting down stream manager")

	// Signal stop
	close(m.stopChan)

	// Cancel pending deferred-stop timers so they don't fire after shutdown
	m.demandStopMu.Lock()
	for channelID, timer := range m.demandStopTimers {
		timer.Stop()
		delete(m.demandStopTimers, channelID)
	}
	m.demandStopMu.Unlock()

	// Cancel all discovery goroutines
	m.mu.Lock()
	for _, state := range m.discovering {
		state.CancelFunc()
	}
	m.mu.Unlock()

	// Stop all pollers and collect channel IDs to deactivate
	var channelsToDeactivate []string
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
	for _, stream := range m.activeStreams {
		channelsToDeactivate = append(channelsToDeactivate, stream.ChannelID)
	}
	m.pollers = make(map[string]*poller.Poller)
	m.activeStreams = make(map[string]*Stream)
	m.mu.Unlock()

	// Mark all active sources inactive so admin panel reflects actual state after restart.
	if m.smClient != nil {
		for _, channelID := range channelsToDeactivate {
			if err := m.smClient.DeactivateSource(ctx, "youtube", channelID); err != nil {
				m.logger.Warn("Failed to deactivate source during shutdown",
					zap.String("channel_id", channelID),
					zap.Error(err),
				)
			}
		}
	}

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
