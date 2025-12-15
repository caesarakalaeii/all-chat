package seventv

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/message-processor/cache"
	"go.uber.org/zap"
)

const (
	// 7TV API base URL
	apiBaseURL = "https://7tv.io/v3"
)

// UserResponse represents a 7TV user (channel) response
type UserResponse struct {
	ID       string `json:"id"`
	Platform string `json:"platform"`
	Username string `json:"username"`
	EmoteSet struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"emote_set"`
}

// Manager manages 7TV event subscriptions and cache invalidation
type Manager struct {
	client      *Client
	cache       cache.Store
	logger      *zap.Logger
	httpClient  *http.Client

	// Map of channel ID -> emote set ID
	channelEmoteSets map[string]string
	mu               sync.RWMutex
}

// NewManager creates a new 7TV event manager
func NewManager(cacheStore cache.Store, logger *zap.Logger) *Manager {
	m := &Manager{
		cache:            cacheStore,
		logger:           logger,
		httpClient:       &http.Client{Timeout: 10 * time.Second},
		channelEmoteSets: make(map[string]string),
	}

	// Create client with event handler
	m.client = NewClient(logger, m.handleEmoteSetUpdate)

	return m
}

// Start connects to the 7TV EventAPI
func (m *Manager) Start(ctx context.Context) error {
	if err := m.client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to start 7TV event client: %w", err)
	}

	m.logger.Info("7TV event manager started")
	return nil
}

// Stop closes the connection to the 7TV EventAPI
func (m *Manager) Stop() error {
	return m.client.Close()
}

// TrackChannel subscribes to emote set updates for a Twitch channel
func (m *Manager) TrackChannel(ctx context.Context, platform, channelID string) error {
	// Currently only support Twitch
	if platform != "twitch" {
		return nil
	}

	m.mu.RLock()
	emoteSetID, exists := m.channelEmoteSets[channelID]
	m.mu.RUnlock()

	if !exists {
		// Fetch emote set ID from 7TV API
		user, err := m.fetch7TVUser(ctx, platform, channelID)
		if err != nil {
			// If channel doesn't have 7TV, silently skip
			m.logger.Debug("Channel not found on 7TV or no emote set",
				zap.String("platform", platform),
				zap.String("channel_id", channelID),
				zap.Error(err),
			)
			return nil
		}

		emoteSetID = user.EmoteSet.ID
		if emoteSetID == "" {
			m.logger.Debug("Channel has no emote set on 7TV",
				zap.String("channel_id", channelID),
			)
			return nil
		}

		m.mu.Lock()
		m.channelEmoteSets[channelID] = emoteSetID
		m.mu.Unlock()

		m.logger.Info("Mapped channel to 7TV emote set",
			zap.String("channel_id", channelID),
			zap.String("emote_set_id", emoteSetID),
		)
	}

	// Subscribe to emote set updates
	if err := m.client.Subscribe(ctx, emoteSetID); err != nil {
		return fmt.Errorf("failed to subscribe to emote set: %w", err)
	}

	return nil
}

// UntrackChannel unsubscribes from emote set updates for a channel
func (m *Manager) UntrackChannel(ctx context.Context, channelID string) error {
	m.mu.RLock()
	emoteSetID, exists := m.channelEmoteSets[channelID]
	m.mu.RUnlock()

	if !exists {
		return nil
	}

	if err := m.client.Unsubscribe(ctx, emoteSetID); err != nil {
		return fmt.Errorf("failed to unsubscribe from emote set: %w", err)
	}

	m.mu.Lock()
	delete(m.channelEmoteSets, channelID)
	m.mu.Unlock()

	return nil
}

// fetch7TVUser fetches a user's 7TV information
func (m *Manager) fetch7TVUser(ctx context.Context, platform, channelID string) (*UserResponse, error) {
	url := fmt.Sprintf("%s/users/%s/%s", apiBaseURL, platform, channelID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch 7TV user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("user not found on 7TV")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("7TV API returned status %d", resp.StatusCode)
	}

	var user UserResponse
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &user, nil
}

// handleEmoteSetUpdate is called when an emote set update is received
func (m *Manager) handleEmoteSetUpdate(ctx context.Context, update *EmoteSetUpdate) error {
	m.logger.Info("Received emote set update",
		zap.String("emote_set_id", update.ID),
		zap.Int("pushed", len(update.Pushed)),
		zap.Int("pulled", len(update.Pulled)),
		zap.Int("updated", len(update.Updated)),
	)

	// Find all channels using this emote set
	m.mu.RLock()
	channelsToInvalidate := make([]string, 0)
	for channelID, emoteSetID := range m.channelEmoteSets {
		if emoteSetID == update.ID {
			channelsToInvalidate = append(channelsToInvalidate, channelID)
		}
	}
	m.mu.RUnlock()

	// Invalidate cache for all affected channels
	for _, channelID := range channelsToInvalidate {
		if err := m.cache.Delete(ctx, channelID); err != nil {
			m.logger.Error("Failed to invalidate emote cache",
				zap.String("channel_id", channelID),
				zap.Error(err),
			)
		} else {
			m.logger.Info("Invalidated emote cache for channel",
				zap.String("channel_id", channelID),
				zap.String("emote_set_id", update.ID),
			)
		}
	}

	return nil
}
