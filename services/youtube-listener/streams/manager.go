package streams

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener/api"
	"github.com/caesar/all-chat/services/youtube-listener/models"
	"github.com/caesar/all-chat/services/youtube-listener/oauth"
	"github.com/caesar/all-chat/services/youtube-listener/quota"
	"github.com/caesar/all-chat/shared/sourcemanager"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// OverlayConnectionEvent represents an overlay connection event from API Gateway
type OverlayConnectionEvent struct {
	Type      string    `json:"type"`       // "connected" or "disconnected"
	OverlayID string    `json:"overlay_id"`
	Timestamp time.Time `json:"timestamp"`
}

// Manager manages active YouTube streams and coordinates polling
type Manager struct {
	repository     *Repository
	oauthManager   *oauth.Manager
	messageHandler MessageHandler
	logger         *zap.Logger
	leader         *sourcemanager.LeadershipCoordinator
	quotaTracker   *quota.Tracker

	mu            sync.RWMutex
	activeStreams map[string]*models.YouTubeStream // streamID -> stream
	pollers       map[string]*Poller               // streamID -> poller

	// Overlay connection tracking
	connMu            sync.RWMutex
	connectedOverlays map[string]time.Time // overlay_id -> connection_time
	redisClient       *redis.Client

	// Livestream detection backoff
	detectionMu         sync.RWMutex
	channelLastCheck    map[string]time.Time // channelID -> last check time
	channelBackoff      map[string]time.Duration // channelID -> current backoff duration
	baseDetectionInterval time.Duration
	maxDetectionInterval  time.Duration

	syncInterval time.Duration
	stopChan     chan struct{}
	wg           sync.WaitGroup
	dbConn       DBConnInterface // For PostgreSQL LISTEN

	// Global sync leadership (prevents multiple replicas from doing expensive discovery)
	// Safe to share the same LeadershipCoordinator because stream IDs are globally unique
	// ("global-sync" will never conflict with actual video IDs which are alphanumeric)
	syncLeader         *sourcemanager.LeadershipCoordinator
	syncLeaderStreamID string // Constant stream ID for global sync leadership
}

// DBConnInterface allows getting a raw pgxpool.Pool for LISTEN
type DBConnInterface interface {
	GetPool() interface{}
}

// NewManager creates a new stream manager
func NewManager(
	repository *Repository,
	oauthManager *oauth.Manager,
	messageHandler MessageHandler,
	dbConn DBConnInterface,
	leader *sourcemanager.LeadershipCoordinator,
	quotaTracker *quota.Tracker,
	redisClient *redis.Client,
	logger *zap.Logger,
) *Manager {
	return &Manager{
		repository:            repository,
		oauthManager:          oauthManager,
		messageHandler:        messageHandler,
		dbConn:                dbConn,
		logger:                logger,
		leader:                leader,
		quotaTracker:          quotaTracker,
		redisClient:           redisClient,
		activeStreams:         make(map[string]*models.YouTubeStream),
		pollers:               make(map[string]*Poller),
		connectedOverlays:     make(map[string]time.Time),
		channelLastCheck:      make(map[string]time.Time),
		channelBackoff:        make(map[string]time.Duration),
		baseDetectionInterval: 30 * time.Second,  // Start checking every 30s
		maxDetectionInterval:  10 * time.Minute,  // Max 10 minutes between checks
		syncInterval:          30 * time.Second,
		stopChan:              make(chan struct{}),
		syncLeader:            leader, // Use same coordinator for global sync leadership
		syncLeaderStreamID:    "global-sync", // Constant stream ID for global sync leadership
	}
}

// Start begins managing streams and PostgreSQL LISTEN
func (m *Manager) Start(ctx context.Context) error {
	m.logger.Info("Starting stream manager")

	// Load existing overlay connections from Redis
	if err := m.loadExistingConnections(ctx); err != nil {
		m.logger.Error("Failed to load existing overlay connections", zap.Error(err))
		// Don't fail startup, just log the error
	}

	// Skip initial sync to avoid quota usage on pod restarts
	// The periodic sync and PostgreSQL LISTEN will handle updates
	m.logger.Info("Skipping initial sync to preserve quota, periodic sync will handle updates")

	// Start periodic sync (fallback)
	m.wg.Add(1)
	go m.periodicSync(ctx)

	// Start PostgreSQL LISTEN for instant notifications
	m.wg.Add(1)
	go m.listenForChanges(ctx)

	// Start Redis subscription for overlay connection events
	m.wg.Add(1)
	go m.listenForOverlayConnections(ctx)

	return nil
}

// Stop stops managing streams
func (m *Manager) Stop() {
	m.logger.Info("Stopping stream manager")

	// Signal stop
	close(m.stopChan)

	// Stop all pollers
	m.mu.Lock()
	for streamID, poller := range m.pollers {
		m.logger.Info("Stopping poller", zap.String("stream_id", streamID))
		poller.Stop()
		m.releaseLeadership(streamID)
	}
	m.pollers = make(map[string]*Poller)
	m.mu.Unlock()

	// Wait for goroutines
	m.wg.Wait()

	if m.leader != nil {
		m.leader.Stop()
	}

	m.logger.Info("Stream manager stopped")
}

// periodicSync periodically syncs streams from database
// Only the global sync leader performs this work to avoid quota waste
func (m *Manager) periodicSync(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(m.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Try to acquire global sync leadership
			// Note: No callback is needed for global sync leadership because:
			// 1. Losing leadership just means another replica will take over syncing
			// 2. There's no local state to clean up (unlike per-stream pollers)
			// 3. The next periodic sync will re-acquire leadership if available
			if m.syncLeader != nil {
				isLeader, err := m.syncLeader.EnsureLeadership(ctx, m.syncLeaderStreamID, nil)
				if err != nil {
					m.logger.Error("Failed to check global sync leadership", zap.Error(err))
					continue
				}
				if !isLeader {
					m.logger.Debug("Not global sync leader, skipping periodic sync")
					continue
				}
				m.logger.Debug("Global sync leader, performing periodic sync")
			}

			if err := m.syncStreams(ctx); err != nil {
				m.logger.Error("Failed to sync streams", zap.Error(err))
			}
		case <-m.stopChan:
			// Release global sync leadership on shutdown
			if m.syncLeader != nil {
				m.syncLeader.Release(m.syncLeaderStreamID)
				// Note: Ignoring error on shutdown - lock will expire naturally (10s TTL)
				// and failure to release is not critical during graceful shutdown
			}
			return
		}
	}
}

// listenForChanges listens for PostgreSQL NOTIFY events for instant source updates
func (m *Manager) listenForChanges(ctx context.Context) {
	defer m.wg.Done()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ctx.Done():
			return
		default:
			// Get connection from pool for LISTEN
			pool := m.dbConn.GetPool()
			if pool == nil {
				m.logger.Error("Failed to get database pool for LISTEN")
				time.Sleep(5 * time.Second)
				continue
			}

			// Acquire connection and LISTEN
			if err := m.listenAndWait(ctx, pool); err != nil {
				m.logger.Warn("PostgreSQL LISTEN error, will retry",
					zap.Error(err),
					zap.Duration("retry_in", 5*time.Second),
				)
				time.Sleep(5 * time.Second)
			}
		}
	}
}

// listenAndWait establishes LISTEN connection and waits for notifications
func (m *Manager) listenAndWait(ctx context.Context, poolInterface interface{}) error {
	pool, ok := poolInterface.(*pgxpool.Pool)
	if !ok {
		return fmt.Errorf("invalid pool type")
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire connection: %w", err)
	}
	defer conn.Release()

	// Start listening for notifications
	_, err = conn.Exec(ctx, "LISTEN chat_source_changes")
	if err != nil {
		return fmt.Errorf("failed to LISTEN: %w", err)
	}

	m.logger.Info("PostgreSQL LISTEN active",
		zap.String("channel", "chat_source_changes"),
	)

	// Wait for notifications
	for {
		select {
		case <-m.stopChan:
			return nil
		case <-ctx.Done():
			return nil
		default:
			// Wait for notification with timeout
			notification, err := conn.Conn().WaitForNotification(ctx)
			if err != nil {
				return fmt.Errorf("notification wait failed: %w", err)
			}

			m.logger.Info("Source change notification received",
				zap.String("payload", notification.Payload),
			)

			// Try to acquire global sync leadership before syncing
			// This prevents multiple replicas from racing to sync on the same notification
			if m.syncLeader != nil {
				isLeader, err := m.syncLeader.EnsureLeadership(ctx, m.syncLeaderStreamID, nil)
				if err != nil {
					m.logger.Error("Failed to check global sync leadership after notification", zap.Error(err))
					continue
				}
				if !isLeader {
					m.logger.Debug("Not global sync leader, skipping sync after notification")
					continue
				}
				m.logger.Debug("Global sync leader, performing sync after notification")
			}

			// Trigger immediate sync
			if err := m.syncStreams(ctx); err != nil {
				m.logger.Error("Failed to sync after notification", zap.Error(err))
			}
		}
	}
}

// syncStreams fetches active sources and starts/stops pollers as needed
func (m *Manager) syncStreams(ctx context.Context) error {
	m.logger.Debug("Syncing streams from database")

	// Get active sources from database
	sources, err := m.repository.GetActiveSources(ctx)
	if err != nil {
		return fmt.Errorf("failed to get active sources: %w", err)
	}

	// Filter sources to only those with connected overlays
	m.connMu.RLock()
	connectedSources := make([]*models.StreamSource, 0)
	for _, source := range sources {
		if _, connected := m.connectedOverlays[source.OverlayID]; connected {
			connectedSources = append(connectedSources, source)
		}
	}
	m.connMu.RUnlock()

	m.logger.Info("Filtered YouTube sources by overlay connections",
		zap.Int("total_sources", len(sources)),
		zap.Int("connected_sources", len(connectedSources)),
		zap.Int("connected_overlays", len(m.connectedOverlays)),
	)

	// Group connected sources by channel ID
	channelSources := make(map[string][]*models.StreamSource)
	for _, source := range connectedSources {
		channelSources[source.ChannelID] = append(channelSources[source.ChannelID], source)
	}

	m.logger.Info("Found active YouTube channels with connected overlays",
		zap.Int("channel_count", len(channelSources)),
		zap.Int("source_count", len(connectedSources)),
	)

	// For each channel, check for live streams (with exponential backoff)
	for channelID, channelSourceList := range channelSources {
		// Check if we should skip this channel due to backoff
		if m.shouldSkipDetection(channelID) {
			continue
		}

		if err := m.syncChannel(ctx, channelID, channelSourceList); err != nil {
			m.logger.Error("Failed to sync channel",
				zap.String("channel_id", channelID),
				zap.Error(err),
			)
			// Increase backoff on error
			m.increaseDetectionBackoff(channelID)
			continue
		}

		// Successfully checked - update backoff
		m.updateDetectionBackoff(channelID)
	}

	// Stop pollers for streams that are no longer active
	m.cleanupInactivePollers(ctx, channelSources)

	return nil
}

// syncChannel checks for live streams on a channel and starts pollers
// Uses a two-tier approach: lightweight status check (1 unit) for cached videos,
// full search (100 units) only when needed
func (m *Manager) syncChannel(ctx context.Context, channelID string, sources []*models.StreamSource) error {
	// Get user ID for OAuth
	userID, err := m.repository.GetUserIDForChannel(ctx, channelID)
	if err != nil {
		// Mark source as inactive - can't get OAuth
		if setErr := m.repository.SetSourceActive(ctx, channelID, false); setErr != nil {
			m.logger.Error("Failed to mark source inactive after OAuth error",
				zap.String("channel_id", channelID),
				zap.Error(setErr),
			)
		}
		return fmt.Errorf("failed to get user ID: %w", err)
	}

	// Create YouTube service with OAuth
	service, err := m.oauthManager.CreateYouTubeService(ctx, userID, channelID)
	if err != nil {
		// Mark source as inactive - OAuth failed
		if setErr := m.repository.SetSourceActive(ctx, channelID, false); setErr != nil {
			m.logger.Error("Failed to mark source inactive after OAuth creation error",
				zap.String("channel_id", channelID),
				zap.Error(setErr),
			)
		}
		return fmt.Errorf("failed to create YouTube service: %w", err)
	}

	// Create API client
	apiClient := api.NewClient(service, m.quotaTracker, m.logger)

	// Try lightweight status check first if we have a cached video ID
	cachedVideoID, err := m.repository.GetCachedVideoID(ctx, channelID)
	if err == nil && cachedVideoID != "" {
		m.logger.Debug("Attempting lightweight status check using cached video ID",
			zap.String("channel_id", channelID),
			zap.String("cached_video_id", cachedVideoID),
		)

		// Check quota for status check (only 1 unit!)
		if !m.quotaTracker.CanMakeRequest(1) {
			m.logger.Warn("Insufficient quota even for lightweight status check",
				zap.String("channel_id", channelID),
				zap.Int("remaining_quota", m.quotaTracker.GetRemainingQuota()),
			)
			return fmt.Errorf("insufficient quota: %d units remaining", m.quotaTracker.GetRemainingQuota())
		}

		// Perform lightweight status check
		isLive, statusErr := apiClient.CheckStreamStatus(ctx, cachedVideoID)
		if statusErr == nil {
			if isLive {
				m.logger.Info("Cached video is live, using lightweight check (saved 99 quota units)",
					zap.String("channel_id", channelID),
					zap.String("video_id", cachedVideoID),
				)

				// Get full video details to start polling
				stream, detailsErr := apiClient.GetVideoDetails(ctx, cachedVideoID)
				if detailsErr == nil && stream.IsLive && stream.LiveChatID != "" {
					// Set the overlay ID from sources
					if len(sources) > 0 {
						stream.OverlayID = sources[0].OverlayID
					}
					stream.StreamID = cachedVideoID

					if err := m.startPoller(ctx, stream, apiClient); err != nil {
						m.logger.Error("Failed to start poller for cached video",
							zap.String("stream_id", cachedVideoID),
							zap.Error(err),
						)
					}
					return nil
				}
			}

			// Cached video is not live, clear the cache and fall through to full search
			m.logger.Debug("Cached video is not live, clearing cache",
				zap.String("channel_id", channelID),
				zap.String("video_id", cachedVideoID),
			)
			if clearErr := m.repository.ClearCachedVideoID(ctx, channelID); clearErr != nil {
				m.logger.Warn("Failed to clear cached video ID",
					zap.String("channel_id", channelID),
					zap.Error(clearErr),
				)
			}
		} else {
			m.logger.Debug("Status check failed for cached video, falling back to full search",
				zap.String("channel_id", channelID),
				zap.Error(statusErr),
			)
		}
	}

	// Fallback to full search (expensive: 100 units)
	// Check quota before making expensive API call
	if !m.quotaTracker.CanMakeRequest(100) {
		m.logger.Warn("Insufficient quota for full stream search, skipping",
			zap.String("channel_id", channelID),
			zap.Int("remaining_quota", m.quotaTracker.GetRemainingQuota()),
		)
		return fmt.Errorf("insufficient quota: %d units remaining, need 100", m.quotaTracker.GetRemainingQuota())
	}

	m.logger.Debug("Performing full live stream search",
		zap.String("channel_id", channelID),
	)

	// Get live streams for channel
	liveStreams, err := apiClient.GetLiveStreams(ctx, channelID)
	if err != nil {
		// Mark source as inactive - API call failed
		if setErr := m.repository.SetSourceActive(ctx, channelID, false); setErr != nil {
			m.logger.Error("Failed to mark source inactive after API error",
				zap.String("channel_id", channelID),
				zap.Error(setErr),
			)
		}
		return fmt.Errorf("failed to get live streams: %w", err)
	}

	if len(liveStreams) == 0 {
		m.logger.Debug("No live streams found for channel (will retry with backoff)",
			zap.String("channel_id", channelID),
		)
		// Don't deactivate sources when no stream is found
		// The channel might go live later, and we already have exponential backoff
		// Sources should only be deactivated on hard errors (OAuth, API failures)
		// or when explicitly removed by users
		return nil
	}

	// Cache the first live stream's video ID for future lightweight checks
	if len(liveStreams) > 0 {
		videoID := liveStreams[0].StreamID
		videoTitle := liveStreams[0].Title
		if videoTitle == "" {
			videoTitle = liveStreams[0].ChannelName // Fallback if title is empty
		}
		if cacheErr := m.repository.UpdateCachedVideoID(ctx, channelID, videoID, videoTitle); cacheErr != nil {
			m.logger.Warn("Failed to cache video ID",
				zap.String("channel_id", channelID),
				zap.String("video_id", videoID),
				zap.Error(cacheErr),
			)
		} else {
			m.logger.Info("Cached video ID for future lightweight checks",
				zap.String("channel_id", channelID),
				zap.String("video_id", videoID),
			)
		}
	}

	// Start pollers for each live stream
	// Note: A channel can have multiple overlay sources, but we only poll once per stream
	// We'll use the first overlay's ID for tracking purposes
	for _, stream := range liveStreams {
		// Set the overlay ID from sources (use first one if multiple)
		if len(sources) > 0 {
			stream.OverlayID = sources[0].OverlayID
		}

		if err := m.startPoller(ctx, stream, apiClient); err != nil {
			m.logger.Error("Failed to start poller",
				zap.String("stream_id", stream.StreamID),
				zap.Error(err),
			)
			continue
		}
	}

	return nil
}

// startPoller starts a poller for a stream (if not already running)
func (m *Manager) startPoller(ctx context.Context, stream *models.YouTubeStream, apiClient *api.Client) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if poller already exists
	if _, exists := m.pollers[stream.StreamID]; exists {
		m.logger.Debug("Poller already running for stream",
			zap.String("stream_id", stream.StreamID),
		)
		// Update database status even when poller already exists
		// This ensures database reflects actual polling state
		if err := m.repository.SetSourceActive(ctx, stream.ChannelID, true); err != nil {
			m.logger.Error("Failed to update source status for existing poller",
				zap.String("channel_id", stream.ChannelID),
				zap.Error(err),
			)
		}
		return nil
	}

	if m.leader != nil {
		ok, err := m.leader.EnsureLeadership(ctx, stream.StreamID, func(streamID string) func() {
			// Capture context for leadership loss callback
			lossCtx := context.Background()
			return func() {
				m.handleLeadershipLoss(lossCtx, streamID)
			}
		}(stream.StreamID))
		if err != nil {
			return fmt.Errorf("failed to claim leadership: %w", err)
		}
		if !ok {
			m.logger.Debug("Leadership held by another instance, skipping poller",
				zap.String("stream_id", stream.StreamID),
			)
			return nil
		}
	}

	m.logger.Info("Starting poller for stream",
		zap.String("stream_id", stream.StreamID),
		zap.String("channel_id", stream.ChannelID),
		zap.String("channel_name", stream.ChannelName),
	)

	// Create and start poller
	poller := NewPoller(stream, apiClient, m.logger)
	poller.SetMessageHandler(m.messageHandler)
	if err := poller.Start(ctx); err != nil {
		if m.leader != nil {
			m.leader.Release(stream.StreamID)
		}
		return fmt.Errorf("failed to start poller: %w", err)
	}

	m.activeStreams[stream.StreamID] = stream
	m.pollers[stream.StreamID] = poller

	// Update database status to active
	if err := m.repository.SetSourceActive(ctx, stream.ChannelID, true); err != nil {
		m.logger.Error("Failed to update source status after starting poller",
			zap.String("channel_id", stream.ChannelID),
			zap.Error(err),
		)
	}

	return nil
}

// cleanupInactivePollers stops pollers for channels that are no longer active
func (m *Manager) cleanupInactivePollers(ctx context.Context, activeChannels map[string][]*models.StreamSource) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for streamID, poller := range m.pollers {
		stream := m.activeStreams[streamID]
		if stream == nil {
			continue
		}

		// Check if channel is still active
		if _, active := activeChannels[stream.ChannelID]; !active {
			m.logger.Info("Stopping poller for inactive channel",
				zap.String("stream_id", streamID),
				zap.String("channel_id", stream.ChannelID),
			)
			poller.Stop()
			delete(m.pollers, streamID)
			delete(m.activeStreams, streamID)
			m.releaseLeadership(streamID)

			// Reset detection backoff to allow quick re-detection when channel goes live again
			m.resetDetectionBackoff(stream.ChannelID)

			// Don't deactivate database sources when cleanup runs
			// Sources should remain active in DB even if temporarily not polling
			// This allows quick resumption when overlays reconnect
			m.logger.Debug("Stopped poller for channel (sources remain active in DB)",
				zap.String("channel_id", stream.ChannelID),
			)
		}
	}
}

func (m *Manager) releaseLeadership(streamID string) {
	if m.leader == nil {
		return
	}
	m.leader.Release(streamID)
}

func (m *Manager) handleLeadershipLoss(ctx context.Context, streamID string) {
	m.mu.Lock()
	stream := m.activeStreams[streamID]
	poller, exists := m.pollers[streamID]
	if exists {
		poller.Stop()
		delete(m.pollers, streamID)
	}
	delete(m.activeStreams, streamID)

	// Update database status to inactive if we have stream info
	if stream != nil {
		if err := m.repository.SetSourceActive(ctx, stream.ChannelID, false); err != nil {
			m.logger.Error("Failed to update source status after leadership loss",
				zap.String("channel_id", stream.ChannelID),
				zap.Error(err),
			)
		}
	}
	m.mu.Unlock()

	if exists {
		m.logger.Warn("Stopped poller after losing leadership",
			zap.String("stream_id", streamID),
		)
	}
}

// GetActiveStreams returns a list of currently active streams
func (m *Manager) GetActiveStreams() []*models.YouTubeStream {
	m.mu.RLock()
	defer m.mu.RUnlock()

	streams := make([]*models.YouTubeStream, 0, len(m.activeStreams))
	for _, stream := range m.activeStreams {
		streams = append(streams, stream)
	}

	return streams
}

// loadExistingConnections loads currently connected overlays from Redis on startup
func (m *Manager) loadExistingConnections(ctx context.Context) error {
	// Query Redis SET for connected overlays
	overlayIDs, err := m.redisClient.SMembers(ctx, "overlay:connected").Result()
	if err != nil {
		return fmt.Errorf("failed to query connected overlays: %w", err)
	}

	if len(overlayIDs) == 0 {
		m.logger.Info("No existing overlay connections found")
		return nil
	}

	// Add to connectedOverlays map
	m.connMu.Lock()
	now := time.Now()
	for _, overlayID := range overlayIDs {
		m.connectedOverlays[overlayID] = now
	}
	m.connMu.Unlock()

	m.logger.Info("Loaded existing overlay connections",
		zap.Int("count", len(overlayIDs)),
		zap.Strings("overlay_ids", overlayIDs),
	)

	return nil
}

// listenForOverlayConnections subscribes to Redis overlay connection events
func (m *Manager) listenForOverlayConnections(ctx context.Context) {
	defer m.wg.Done()

	m.logger.Info("Starting overlay connection listener")

	pubsub := m.redisClient.Subscribe(ctx, "overlay:connections")
	defer pubsub.Close()

	ch := pubsub.Channel()

	for {
		select {
		case msg := <-ch:
			var event OverlayConnectionEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				m.logger.Error("Failed to unmarshal overlay connection event",
					zap.Error(err),
					zap.String("payload", msg.Payload),
				)
				continue
			}

			m.logger.Info("Received overlay connection event",
				zap.String("type", event.Type),
				zap.String("overlay_id", event.OverlayID),
			)

			switch event.Type {
			case "connected":
				m.handleOverlayConnected(ctx, event.OverlayID)
			case "disconnected":
				m.handleOverlayDisconnected(ctx, event.OverlayID)
			default:
				m.logger.Warn("Unknown overlay connection event type",
					zap.String("type", event.Type),
				)
			}

		case <-m.stopChan:
			m.logger.Info("Stopping overlay connection listener")
			return
		case <-ctx.Done():
			m.logger.Info("Context cancelled, stopping overlay connection listener")
			return
		}
	}
}

// handleOverlayConnected handles an overlay connection event
func (m *Manager) handleOverlayConnected(ctx context.Context, overlayID string) {
	m.connMu.Lock()
	m.connectedOverlays[overlayID] = time.Now()
	m.connMu.Unlock()

	m.logger.Info("Overlay connected, starting polling",
		zap.String("overlay_id", overlayID),
	)

	// Try to acquire global sync leadership before syncing
	// This prevents multiple replicas from racing to start polling
	if m.syncLeader != nil {
		isLeader, err := m.syncLeader.EnsureLeadership(ctx, m.syncLeaderStreamID, nil)
		if err != nil {
			m.logger.Error("Failed to check global sync leadership after overlay connection", zap.Error(err))
			return
		}
		if !isLeader {
			m.logger.Debug("Not global sync leader, skipping sync after overlay connection")
			return
		}
		m.logger.Debug("Global sync leader, performing sync after overlay connection")
	}

	// Trigger immediate sync to start polling for this overlay's sources
	if err := m.syncStreams(ctx); err != nil {
		m.logger.Error("Failed to sync streams after overlay connection",
			zap.String("overlay_id", overlayID),
			zap.Error(err),
		)
	}
}

// handleOverlayDisconnected handles an overlay disconnection event
func (m *Manager) handleOverlayDisconnected(ctx context.Context, overlayID string) {
	m.connMu.Lock()
	delete(m.connectedOverlays, overlayID)
	m.connMu.Unlock()

	m.logger.Info("Overlay disconnected, stopping polling",
		zap.String("overlay_id", overlayID),
	)

	// Stop pollers for sources belonging to this overlay
	m.mu.Lock()
	for streamID, stream := range m.activeStreams {
		if stream.OverlayID == overlayID {
			if poller, exists := m.pollers[streamID]; exists {
				m.logger.Info("Stopping poller for disconnected overlay",
					zap.String("stream_id", streamID),
					zap.String("overlay_id", overlayID),
				)
				poller.Stop()
				delete(m.pollers, streamID)
				delete(m.activeStreams, streamID)
				m.releaseLeadership(streamID)

				// Mark source as inactive for this specific overlay only
				// Don't deactivate sources for other overlays using the same channel
				if err := m.repository.SetSourceActiveByOverlay(ctx, overlayID, stream.ChannelID, false); err != nil {
					m.logger.Error("Failed to mark source inactive after overlay disconnect",
						zap.String("overlay_id", overlayID),
						zap.String("channel_id", stream.ChannelID),
						zap.Error(err),
					)
				}
			}
		}
	}
	m.mu.Unlock()
}

// IsOverlayConnected checks if an overlay has active WebSocket connections
func (m *Manager) IsOverlayConnected(overlayID string) bool {
	m.connMu.RLock()
	defer m.connMu.RUnlock()

	_, connected := m.connectedOverlays[overlayID]
	return connected
}

// shouldSkipDetection checks if we should skip livestream detection for a channel due to backoff
func (m *Manager) shouldSkipDetection(channelID string) bool {
	m.detectionMu.RLock()
	defer m.detectionMu.RUnlock()

	lastCheck, exists := m.channelLastCheck[channelID]
	if !exists {
		return false // First check, don't skip
	}

	backoff := m.channelBackoff[channelID]
	if backoff == 0 {
		backoff = m.baseDetectionInterval
	}

	timeSinceLastCheck := time.Since(lastCheck)
	shouldSkip := timeSinceLastCheck < backoff

	if shouldSkip {
		m.logger.Debug("Skipping livestream detection due to backoff",
			zap.String("channel_id", channelID),
			zap.Duration("backoff", backoff),
			zap.Duration("time_since_last_check", timeSinceLastCheck),
		)
	}

	return shouldSkip
}

// updateDetectionBackoff updates backoff after successful livestream detection
func (m *Manager) updateDetectionBackoff(channelID string) {
	m.detectionMu.Lock()
	defer m.detectionMu.Unlock()

	m.channelLastCheck[channelID] = time.Now()

	// Check if we found a stream (have active poller)
	m.mu.RLock()
	hasActivePoller := false
	for streamID := range m.pollers {
		// Check if this poller is for this channel
		if stream, ok := m.activeStreams[streamID]; ok && stream.ChannelID == channelID {
			hasActivePoller = true
			break
		}
	}
	m.mu.RUnlock()

	if hasActivePoller {
		// Stream found - set to max backoff since we're now polling chat messages
		// No need to keep checking for stream existence while actively polling
		m.channelBackoff[channelID] = m.maxDetectionInterval

		m.logger.Info("Set livestream detection to max backoff (stream active, polling chat)",
			zap.String("channel_id", channelID),
			zap.Duration("backoff", m.maxDetectionInterval),
		)
	} else {
		// No stream found - INCREASE backoff exponentially to conserve quota
		// This prevents burning quota checking for streams that aren't live
		currentBackoff := m.channelBackoff[channelID]
		if currentBackoff == 0 {
			currentBackoff = m.baseDetectionInterval
		}
		// Double the backoff (exponential), up to max
		newBackoff := currentBackoff * 2
		if newBackoff > m.maxDetectionInterval {
			newBackoff = m.maxDetectionInterval
		}
		m.channelBackoff[channelID] = newBackoff

		m.logger.Info("Increased livestream detection backoff (no stream found)",
			zap.String("channel_id", channelID),
			zap.Duration("previous_backoff", currentBackoff),
			zap.Duration("new_backoff", newBackoff),
		)
	}
}

// increaseDetectionBackoff increases backoff after detection error
func (m *Manager) increaseDetectionBackoff(channelID string) {
	m.detectionMu.Lock()
	defer m.detectionMu.Unlock()

	m.channelLastCheck[channelID] = time.Now()

	currentBackoff := m.channelBackoff[channelID]
	if currentBackoff == 0 {
		currentBackoff = m.baseDetectionInterval
	}

	// Double the backoff on error
	newBackoff := currentBackoff * 2
	if newBackoff > m.maxDetectionInterval {
		newBackoff = m.maxDetectionInterval
	}
	m.channelBackoff[channelID] = newBackoff

	m.logger.Warn("Increased livestream detection backoff due to error",
		zap.String("channel_id", channelID),
		zap.Duration("new_backoff", newBackoff),
	)
}

// resetDetectionBackoff resets backoff to base interval when a poller stops
// This allows quick re-detection if the channel goes live again shortly after
func (m *Manager) resetDetectionBackoff(channelID string) {
	m.detectionMu.Lock()
	defer m.detectionMu.Unlock()

	m.channelBackoff[channelID] = m.baseDetectionInterval
	m.channelLastCheck[channelID] = time.Now()

	m.logger.Info("Reset livestream detection backoff (stream ended)",
		zap.String("channel_id", channelID),
		zap.Duration("backoff", m.baseDetectionInterval),
	)
}
