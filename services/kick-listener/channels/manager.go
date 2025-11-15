package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/kick-listener/publisher"
	"github.com/caesar/all-chat/services/kick-listener/websocket"
	"go.uber.org/zap"
)

const (
	// Kick API endpoint for channel info
	kickAPIChannelURL = "https://kick.com/api/v2/channels/%s"

	// Sync interval for checking active channels
	syncInterval = 30 * time.Second
)

// DBConnInterface allows for dependency injection
type DBConnInterface interface {
	GetPool() interface{}
}

// WebSocketClient interface for testing
type WebSocketClient interface {
	Subscribe(chatroomID int) error
	Unsubscribe(chatroomID int) error
	IsConnected() bool
}

// Manager manages Kick channel subscriptions
type Manager struct {
	repo         *Repository
	wsClient     WebSocketClient
	publisher    *publisher.StreamPublisher
	logger       *zap.Logger
	httpClient   *http.Client

	// Track active subscriptions
	subscriptions map[string]*ActiveChannel // key: channel_slug
	subsMu        sync.RWMutex

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewManager creates a new channel manager
func NewManager(
	repo *Repository,
	wsClient WebSocketClient,
	publisher *publisher.StreamPublisher,
	logger *zap.Logger,
) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		repo:          repo,
		wsClient:      wsClient,
		publisher:     publisher,
		logger:        logger,
		httpClient:    &http.Client{Timeout: 10 * time.Second},
		subscriptions: make(map[string]*ActiveChannel),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start begins the channel management loop
func (m *Manager) Start() error {
	m.logger.Info("Starting Kick channel manager")

	// Initial sync
	if err := m.syncChannels(); err != nil {
		m.logger.Error("Initial channel sync failed", zap.Error(err))
		// Don't fail startup, will retry
	}

	// Start periodic sync
	m.wg.Add(1)
	go m.syncLoop()

	return nil
}

// Stop stops the channel manager
func (m *Manager) Stop() {
	m.logger.Info("Stopping Kick channel manager")
	m.cancel()
	m.wg.Wait()
	m.logger.Info("Kick channel manager stopped")
}

// syncLoop periodically syncs channels from database
func (m *Manager) syncLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			if err := m.syncChannels(); err != nil {
				m.logger.Error("Failed to sync channels", zap.Error(err))
			}
		}
	}
}

// syncChannels synchronizes channel subscriptions with database
func (m *Manager) syncChannels() error {
	m.logger.Debug("Syncing Kick channels from database")

	// Get active channels from database
	channels, err := m.repo.GetActiveChannels(m.ctx)
	if err != nil {
		return fmt.Errorf("failed to get active channels: %w", err)
	}

	// Build map of desired channels
	desiredChannels := make(map[string]*ActiveChannel)
	for _, ch := range channels {
		desiredChannels[ch.ChannelSlug] = ch
	}

	m.subsMu.Lock()
	defer m.subsMu.Unlock()

	// Unsubscribe from channels no longer active
	for slug, ch := range m.subscriptions {
		if _, exists := desiredChannels[slug]; !exists {
			m.logger.Info("Unsubscribing from channel", zap.String("channel", slug))
			if err := m.wsClient.Unsubscribe(ch.ChatroomID); err != nil {
				m.logger.Error("Failed to unsubscribe", zap.String("channel", slug), zap.Error(err))
			}
			delete(m.subscriptions, slug)
		}
	}

	// Subscribe to new channels
	for slug, ch := range desiredChannels {
		if _, exists := m.subscriptions[slug]; !exists {
			// Need to fetch chatroom ID if not set
			if ch.ChatroomID == 0 {
				chatroomID, err := m.fetchChatroomID(ch.ChannelSlug)
				if err != nil {
					m.logger.Error("Failed to fetch chatroom ID",
						zap.String("channel", slug),
						zap.Error(err),
					)
					continue
				}
				ch.ChatroomID = chatroomID

				// Update in database
				if err := m.repo.UpdateChatroomID(m.ctx, ch.OverlayID, ch.ChannelSlug, chatroomID); err != nil {
					m.logger.Warn("Failed to update chatroom ID in database", zap.Error(err))
				}
			}

			m.logger.Info("Subscribing to channel",
				zap.String("channel", slug),
				zap.Int("chatroom_id", ch.ChatroomID),
			)

			if err := m.wsClient.Subscribe(ch.ChatroomID); err != nil {
				m.logger.Error("Failed to subscribe",
					zap.String("channel", slug),
					zap.Error(err),
				)
				continue
			}

			m.subscriptions[slug] = ch
		}
	}

	m.logger.Info("Channel sync completed",
		zap.Int("active_subscriptions", len(m.subscriptions)),
	)

	return nil
}

// fetchChatroomID fetches the chatroom ID for a Kick channel from the API
func (m *Manager) fetchChatroomID(channelSlug string) (int, error) {
	url := fmt.Sprintf(kickAPIChannelURL, channelSlug)

	req, err := http.NewRequestWithContext(m.ctx, "GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "AllChat/1.0")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch channel info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var channelInfo websocket.KickChannelInfo
	if err := json.NewDecoder(resp.Body).Decode(&channelInfo); err != nil {
		return 0, fmt.Errorf("failed to decode channel info: %w", err)
	}

	if channelInfo.Chatroom.ID == 0 {
		return 0, fmt.Errorf("chatroom ID is 0 for channel %s", channelSlug)
	}

	m.logger.Info("Fetched chatroom ID",
		zap.String("channel_slug", channelSlug),
		zap.Int("chatroom_id", channelInfo.Chatroom.ID),
	)

	return channelInfo.Chatroom.ID, nil
}

// GetSubscriptions returns current subscriptions (for status endpoint)
func (m *Manager) GetSubscriptions() map[string]*ActiveChannel {
	m.subsMu.RLock()
	defer m.subsMu.RUnlock()

	subs := make(map[string]*ActiveChannel)
	for k, v := range m.subscriptions {
		subs[k] = v
	}
	return subs
}

// GetOverlayIDForChatroom returns the overlay ID for a chatroom
func (m *Manager) GetOverlayIDForChatroom(chatroomID int) (string, string, bool) {
	m.subsMu.RLock()
	defer m.subsMu.RUnlock()

	for _, ch := range m.subscriptions {
		if ch.ChatroomID == chatroomID {
			return ch.OverlayID, ch.ChannelSlug, true
		}
	}

	return "", "", false
}
