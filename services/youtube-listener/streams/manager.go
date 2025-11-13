package streams

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener/api"
	"github.com/caesar/all-chat/services/youtube-listener/models"
	"github.com/caesar/all-chat/services/youtube-listener/oauth"
	"go.uber.org/zap"
)

// Manager manages active YouTube streams and coordinates polling
type Manager struct {
	repository     *Repository
	oauthManager   *oauth.Manager
	messageHandler MessageHandler
	logger         *zap.Logger

	mu            sync.RWMutex
	activeStreams map[string]*models.YouTubeStream // streamID -> stream
	pollers       map[string]*Poller               // streamID -> poller

	syncInterval time.Duration
	stopChan     chan struct{}
	wg           sync.WaitGroup
}

// NewManager creates a new stream manager
func NewManager(
	repository *Repository,
	oauthManager *oauth.Manager,
	messageHandler MessageHandler,
	logger *zap.Logger,
) *Manager {
	return &Manager{
		repository:     repository,
		oauthManager:   oauthManager,
		messageHandler: messageHandler,
		logger:         logger,
		activeStreams:  make(map[string]*models.YouTubeStream),
		pollers:        make(map[string]*Poller),
		syncInterval:   30 * time.Second,
		stopChan:       make(chan struct{}),
	}
}

// Start begins managing streams
func (m *Manager) Start(ctx context.Context) error {
	m.logger.Info("Starting stream manager")

	// Initial sync
	if err := m.syncStreams(ctx); err != nil {
		m.logger.Error("Failed initial stream sync", zap.Error(err))
		return fmt.Errorf("failed initial sync: %w", err)
	}

	// Start periodic sync
	m.wg.Add(1)
	go m.periodicSync(ctx)

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
	}
	m.pollers = make(map[string]*Poller)
	m.mu.Unlock()

	// Wait for goroutines
	m.wg.Wait()

	m.logger.Info("Stream manager stopped")
}

// periodicSync periodically syncs streams from database
func (m *Manager) periodicSync(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(m.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := m.syncStreams(ctx); err != nil {
				m.logger.Error("Failed to sync streams", zap.Error(err))
			}
		case <-m.stopChan:
			return
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

	// Group sources by channel ID
	channelSources := make(map[string][]*models.StreamSource)
	for _, source := range sources {
		channelSources[source.ChannelID] = append(channelSources[source.ChannelID], source)
	}

	m.logger.Info("Found active YouTube channels",
		zap.Int("channel_count", len(channelSources)),
		zap.Int("source_count", len(sources)),
	)

	// For each channel, check for live streams
	for channelID, channelSourceList := range channelSources {
		if err := m.syncChannel(ctx, channelID, channelSourceList); err != nil {
			m.logger.Error("Failed to sync channel",
				zap.String("channel_id", channelID),
				zap.Error(err),
			)
			continue
		}
	}

	// Stop pollers for streams that are no longer active
	m.cleanupInactivePollers(channelSources)

	return nil
}

// syncChannel checks for live streams on a channel and starts pollers
func (m *Manager) syncChannel(ctx context.Context, channelID string, sources []*models.StreamSource) error {
	// Get user ID for OAuth
	userID, err := m.repository.GetUserIDForChannel(ctx, channelID)
	if err != nil {
		return fmt.Errorf("failed to get user ID: %w", err)
	}

	// Create YouTube service with OAuth
	service, err := m.oauthManager.CreateYouTubeService(ctx, userID, channelID)
	if err != nil {
		return fmt.Errorf("failed to create YouTube service: %w", err)
	}

	// Create API client
	apiClient := api.NewClient(service, m.logger)

	// Get live streams for channel
	liveStreams, err := apiClient.GetLiveStreams(ctx, channelID)
	if err != nil {
		return fmt.Errorf("failed to get live streams: %w", err)
	}

	if len(liveStreams) == 0 {
		m.logger.Debug("No live streams found for channel",
			zap.String("channel_id", channelID),
		)
		return nil
	}

	// Start pollers for each live stream
	for _, stream := range liveStreams {
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
		return nil
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
		return fmt.Errorf("failed to start poller: %w", err)
	}

	m.activeStreams[stream.StreamID] = stream
	m.pollers[stream.StreamID] = poller

	return nil
}

// cleanupInactivePollers stops pollers for channels that are no longer active
func (m *Manager) cleanupInactivePollers(activeChannels map[string][]*models.StreamSource) {
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
		}
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
