package collectors

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/event-collector/collectors/twitch"
	"github.com/caesar/all-chat/services/event-collector/normalizers"
	"github.com/caesar/all-chat/services/event-collector/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// CollectorManager manages EventSub collectors for multiple users
type CollectorManager struct {
	twitchClients    map[uuid.UUID]*twitch.EventSubClient
	twitchClientsMux sync.RWMutex
	eventRepo        *repository.EventRepository
	sessionRepo      *repository.SessionRepository
	normalizer       *normalizers.TwitchNormalizer
	clientID         string
	clientSecret     string
	logger           *zap.Logger
}

// UserBroadcasterMapping maps our user ID to platform broadcaster IDs
type UserBroadcasterMapping struct {
	UserID          uuid.UUID
	TwitchID        string
	TwitchAccessToken string
}

// NewCollectorManager creates a new collector manager
func NewCollectorManager(
	eventRepo *repository.EventRepository,
	sessionRepo *repository.SessionRepository,
	normalizer *normalizers.TwitchNormalizer,
	clientID string,
	clientSecret string,
	logger *zap.Logger,
) *CollectorManager {
	return &CollectorManager{
		twitchClients: make(map[uuid.UUID]*twitch.EventSubClient),
		eventRepo:     eventRepo,
		sessionRepo:   sessionRepo,
		normalizer:    normalizer,
		clientID:      clientID,
		clientSecret:  clientSecret,
		logger:        logger,
	}
}

// StartTwitchCollector starts a Twitch EventSub collector for a user
func (cm *CollectorManager) StartTwitchCollector(ctx context.Context, mapping *UserBroadcasterMapping) error {
	cm.twitchClientsMux.Lock()
	defer cm.twitchClientsMux.Unlock()

	// Check if collector already running
	if _, exists := cm.twitchClients[mapping.UserID]; exists {
		return fmt.Errorf("twitch collector already running for user %s", mapping.UserID)
	}

	// Create EventSub client
	client := twitch.NewEventSubClient(
		cm.logger,
		cm.eventRepo,
		cm.sessionRepo,
		cm.normalizer,
		cm.clientID,
		mapping.TwitchAccessToken,
	)

	// Store broadcaster ID in client context (for event processing)
	client.BroadcasterID = mapping.TwitchID
	client.UserID = mapping.UserID

	// Connect to EventSub WebSocket
	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to EventSub: %w", err)
	}

	// Start listening in background
	go func() {
		if err := client.Listen(ctx); err != nil {
			cm.logger.Error("EventSub client error",
				zap.String("user_id", mapping.UserID.String()),
				zap.Error(err),
			)
		}

		// Remove from map when disconnected
		cm.twitchClientsMux.Lock()
		delete(cm.twitchClients, mapping.UserID)
		cm.twitchClientsMux.Unlock()
	}()

	// Subscribe to events after getting session ID
	// This will be done after receiving welcome message
	go func() {
		// Wait for session ID (with timeout)
		timeout := time.After(30 * time.Second)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-timeout:
				cm.logger.Error("Timeout waiting for EventSub session ID")
				return
			case <-ticker.C:
				if client.GetSessionID() != "" {
					goto subscribe
				}
			}
		}

subscribe:

		// Create subscription manager
		subManager := twitch.NewSubscriptionManager(cm.clientID, mapping.TwitchAccessToken, cm.logger)

		// Subscribe to all event types
		if err := subManager.SubscribeToEvents(ctx, mapping.TwitchID, client.GetSessionID()); err != nil {
			cm.logger.Error("Failed to subscribe to events",
				zap.String("user_id", mapping.UserID.String()),
				zap.Error(err),
			)
			return
		}

		cm.logger.Info("Subscribed to Twitch events",
			zap.String("user_id", mapping.UserID.String()),
			zap.String("broadcaster_id", mapping.TwitchID),
		)
	}()

	// Store client
	cm.twitchClients[mapping.UserID] = client

	cm.logger.Info("Started Twitch EventSub collector",
		zap.String("user_id", mapping.UserID.String()),
		zap.String("broadcaster_id", mapping.TwitchID),
	)

	return nil
}

// StopTwitchCollector stops a Twitch EventSub collector for a user
func (cm *CollectorManager) StopTwitchCollector(userID uuid.UUID) error {
	cm.twitchClientsMux.Lock()
	defer cm.twitchClientsMux.Unlock()

	client, exists := cm.twitchClients[userID]
	if !exists {
		return fmt.Errorf("no twitch collector running for user %s", userID)
	}

	// Close client
	if err := client.Close(); err != nil {
		return fmt.Errorf("failed to close EventSub client: %w", err)
	}

	// Remove from map
	delete(cm.twitchClients, userID)

	cm.logger.Info("Stopped Twitch EventSub collector",
		zap.String("user_id", userID.String()),
	)

	return nil
}

// GetTwitchCollector returns the Twitch collector for a user
func (cm *CollectorManager) GetTwitchCollector(userID uuid.UUID) (*twitch.EventSubClient, bool) {
	cm.twitchClientsMux.RLock()
	defer cm.twitchClientsMux.RUnlock()

	client, exists := cm.twitchClients[userID]
	return client, exists
}

// ListActiveCollectors returns all active collector user IDs
func (cm *CollectorManager) ListActiveCollectors() []uuid.UUID {
	cm.twitchClientsMux.RLock()
	defer cm.twitchClientsMux.RUnlock()

	userIDs := make([]uuid.UUID, 0, len(cm.twitchClients))
	for userID := range cm.twitchClients {
		userIDs = append(userIDs, userID)
	}

	return userIDs
}

// Shutdown gracefully stops all collectors
func (cm *CollectorManager) Shutdown() {
	cm.twitchClientsMux.Lock()
	defer cm.twitchClientsMux.Unlock()

	cm.logger.Info("Shutting down all collectors", zap.Int("count", len(cm.twitchClients)))

	for userID, client := range cm.twitchClients {
		if err := client.Close(); err != nil {
			cm.logger.Error("Failed to close client",
				zap.String("user_id", userID.String()),
				zap.Error(err),
			)
		}
	}

	cm.twitchClients = make(map[uuid.UUID]*twitch.EventSubClient)
}
