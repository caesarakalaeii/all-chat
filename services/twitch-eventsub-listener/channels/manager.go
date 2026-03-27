package channels

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/caesar/all-chat/shared/encryption"
	"github.com/caesar/all-chat/shared/listener"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Channel represents an active Twitch channel
type Channel struct {
	BroadcasterID   string
	BroadcasterName string
	AccessToken     string   // User OAuth token for EventSub subscriptions
	OverlayIDs      []string // Overlays using this channel
}

// SubscriptionCallback is called when channels are added/removed
// Receives broadcaster ID, access token, and action (subscribe/unsubscribe)
type SubscriptionCallback func(broadcasterID string, accessToken string, action string) error

// UserIDResolver resolves Twitch usernames to user IDs
type UserIDResolver interface {
	GetUserIDByLogin(ctx context.Context, login string) (string, error)
}

// Manager tracks active Twitch channels and manages subscriptions
type Manager struct {
	db       *pgxpool.Pool
	logger   *zap.Logger
	resolver UserIDResolver
	callback SubscriptionCallback
	cipher   *encryption.AESEncryptor

	mu       sync.RWMutex
	channels map[string]*Channel // broadcaster_id -> Channel

	// Coordinator integration for sharding
	assignedSourceIDs map[string]bool                    // source_id -> bool (assigned to this pod)
	demandedSourceIDs map[string]listener.DemandedSource // nil = no demand filtering
	podName           string
	assignmentMu      sync.RWMutex

	stopChan     chan struct{}
	wg           sync.WaitGroup
	syncInterval time.Duration
}

// compile-time assertion: Manager must satisfy listener.ChannelManager
var _ listener.ChannelManager = (*Manager)(nil)

// NewManager creates a new channel manager
func NewManager(db *pgxpool.Pool, logger *zap.Logger, resolver UserIDResolver, cipher *encryption.AESEncryptor, syncInterval time.Duration) *Manager {
	return &Manager{
		db:           db,
		cipher:       cipher,
		logger:       logger,
		resolver:     resolver,
		channels:     make(map[string]*Channel),
		stopChan:     make(chan struct{}),
		syncInterval: syncInterval,
	}
}

// SetSubscriptionCallback sets the callback for channel changes
func (m *Manager) SetSubscriptionCallback(callback SubscriptionCallback) {
	m.callback = callback
}

// SetAssignedSourceIDs sets the assigned source IDs for filtering (coordinator integration)
func (m *Manager) SetAssignedSourceIDs(assignedSourceIDs map[string]bool, podName string) {
	m.assignmentMu.Lock()
	defer m.assignmentMu.Unlock()

	m.assignedSourceIDs = assignedSourceIDs
	m.podName = podName

	m.logger.Info("Set assigned source IDs",
		zap.Int("count", len(assignedSourceIDs)),
		zap.String("pod_name", podName),
	)
}

// UpdateAssignedSourceIDs updates the assigned source IDs (for dynamic assignment refresh).
func (m *Manager) UpdateAssignedSourceIDs(assignedSourceIDs map[string]bool) {
	m.assignmentMu.Lock()
	defer m.assignmentMu.Unlock()

	m.assignedSourceIDs = assignedSourceIDs

	m.logger.Info("Updated assigned source IDs",
		zap.Int("count", len(assignedSourceIDs)),
		zap.String("pod_name", m.podName),
	)
}

// UpdateDemandedSourceIDs stores the demanded source set and triggers reconciliation.
// For EventSub, "demand" means subscriptions should exist only for demanded sources.
// Sources that lose demand are unsubscribed on the next SyncChannels cycle; the SDK
// is responsible for filtering before calling this method.
//
// Note: EventSub subscriptions are leader-managed — only the leader creates/deletes
// subscriptions. This method stores the demand state; actual reconciliation happens
// in SyncChannels to avoid double-locking.
func (m *Manager) UpdateDemandedSourceIDs(demanded map[string]listener.DemandedSource) {
	m.assignmentMu.Lock()
	m.demandedSourceIDs = demanded
	m.assignmentMu.Unlock()

	m.logger.Debug("Demand update received",
		zap.Int("demanded_sources", len(demanded)),
	)
}

// Start begins periodic channel syncing
func (m *Manager) Start(ctx context.Context) error {
	m.logger.Info("Starting channel manager",
		zap.Duration("sync_interval", m.syncInterval),
	)

	// Initial sync
	if err := m.SyncChannels(ctx); err != nil {
		m.logger.Error("Initial channel sync failed", zap.Error(err))
	}

	// Periodic sync
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()

		ticker := time.NewTicker(m.syncInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-m.stopChan:
				return
			case <-ticker.C:
				if err := m.SyncChannels(ctx); err != nil {
					m.logger.Error("Channel sync failed", zap.Error(err))
				}
			}
		}
	}()
	return nil
}

// Stop stops the channel manager
func (m *Manager) Stop() {
	close(m.stopChan)
	m.wg.Wait()
	m.logger.Info("Channel manager stopped")
}

// SyncChannels queries the database for active Twitch channels and creates/removes subscriptions
func (m *Manager) SyncChannels(ctx context.Context) error {
	// Query active Twitch sources and join with users table to get OAuth tokens
	// We need the broadcaster's OAuth token for EventSub subscriptions
	query := `
		SELECT DISTINCT
			ocs.id,
			ocs.channel_id,
			ocs.overlay_id,
			u.access_token,
			u.token_expires_at
		FROM overlay_chat_sources ocs
		JOIN overlays o ON ocs.overlay_id = o.id
		LEFT JOIN users u ON LOWER(u.username) = LOWER(ocs.channel_id) AND u.auth_provider = 'twitch'
		WHERE ocs.platform = 'twitch'
		  AND ocs.is_active = true
		  AND o.is_active = true
		ORDER BY ocs.channel_id
	`

	rows, err := m.db.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query active channels: %w", err)
	}
	defer rows.Close()

	// Build map of active channels
	activeChannels := make(map[string]*Channel)

	for rows.Next() {
		var sourceID, channelID, overlayID string
		var accessToken *string
		var tokenExpiresAt *time.Time

		if err := rows.Scan(&sourceID, &channelID, &overlayID, &accessToken, &tokenExpiresAt); err != nil {
			m.logger.Warn("Failed to scan row", zap.Error(err))
			continue
		}

		// Note: EventSub uses leader election - only the leader creates subscriptions for ALL Twitch sources
		// We don't filter by coordinator assignments because:
		// 1. Webhooks are stateless - all pods can receive events
		// 2. Only the leader manages subscriptions (checked in callback)
		// 3. This ensures consistent subscription coverage across all active channels
		// The coordinator integration is used for pod health monitoring and migration notifications only

		// channelID from database is the Twitch username (login)
		// We need to resolve it to broadcaster_user_id via Twitch API
		broadcasterID, err := m.resolver.GetUserIDByLogin(ctx, channelID)
		if err != nil {
			m.logger.Warn("Failed to resolve username to user ID",
				zap.String("username", channelID),
				zap.Error(err),
			)
			continue
		}

		// Skip channels without OAuth tokens - we can't subscribe to EventSub without them
		if accessToken == nil || *accessToken == "" {
			m.logger.Debug("Skipping channel without OAuth token",
				zap.String("broadcaster_id", broadcasterID),
				zap.String("username", channelID),
			)
			continue
		}

		// Check if token is expired
		if tokenExpiresAt != nil && time.Now().After(*tokenExpiresAt) {
			m.logger.Warn("Skipping channel with expired OAuth token",
				zap.String("broadcaster_id", broadcasterID),
				zap.String("username", channelID),
				zap.Time("expired_at", *tokenExpiresAt),
			)
			continue
		}

		// Decrypt the access token (tokens are encrypted at rest)
		decryptedToken, err := m.decryptToken(*accessToken)
		if err != nil {
			m.logger.Error("Failed to decrypt access token",
				zap.String("broadcaster_id", broadcasterID),
				zap.String("username", channelID),
				zap.Error(err),
			)
			continue
		}

		if _, exists := activeChannels[broadcasterID]; !exists {
			activeChannels[broadcasterID] = &Channel{
				BroadcasterID:   broadcasterID,
				BroadcasterName: channelID,
				AccessToken:     decryptedToken, // Store decrypted user OAuth token
				OverlayIDs:      []string{},
			}
		}

		activeChannels[broadcasterID].OverlayIDs = append(activeChannels[broadcasterID].OverlayIDs, overlayID)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating rows: %w", err)
	}

	m.logger.Debug("Synced active channels from database",
		zap.Int("active_channels", len(activeChannels)),
	)

	// Compare with current channels and create/remove subscriptions
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find new channels (need to subscribe)
	for broadcasterID, channel := range activeChannels {
		if _, exists := m.channels[broadcasterID]; !exists {
			// New channel - create subscription
			m.logger.Info("New channel detected",
				zap.String("broadcaster_id", broadcasterID),
				zap.Int("overlay_count", len(channel.OverlayIDs)),
			)

			if m.callback != nil {
				if err := m.callback(broadcasterID, channel.AccessToken, "subscribe"); err != nil {
					m.logger.Error("Failed to subscribe to channel",
						zap.String("broadcaster_id", broadcasterID),
						zap.Error(err),
					)
					continue
				}
			}

			m.channels[broadcasterID] = channel
		}
	}

	// Find removed channels (need to unsubscribe)
	for broadcasterID := range m.channels {
		if _, exists := activeChannels[broadcasterID]; !exists {
			// Channel removed - delete subscription
			m.logger.Info("Channel removed",
				zap.String("broadcaster_id", broadcasterID),
			)

			if m.callback != nil {
				if err := m.callback(broadcasterID, "", "unsubscribe"); err != nil {
					m.logger.Error("Failed to unsubscribe from channel",
						zap.String("broadcaster_id", broadcasterID),
						zap.Error(err),
					)
					continue
				}
			}

			delete(m.channels, broadcasterID)
		}
	}

	return nil
}

// GetActiveChannelMap returns the active channels as a map (broadcaster_id -> Channel).
// Prefer GetActiveChannels for SDK-compatible access.
func (m *Manager) GetActiveChannelMap() map[string]*Channel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return copy to avoid race conditions
	channels := make(map[string]*Channel)
	for k, v := range m.channels {
		channels[k] = v
	}
	return channels
}

// GetActiveChannels returns broadcaster IDs of all active channels (satisfies listener.ChannelManager).
func (m *Manager) GetActiveChannels() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.channels))
	for id := range m.channels {
		ids = append(ids, id)
	}
	return ids
}

// GetActiveChannelCount returns the number of active channels (satisfies listener.ChannelManager).
func (m *Manager) GetActiveChannelCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.channels)
}

// GetFilteredAssignmentCount returns the number of assigned source IDs (satisfies listener.ChannelManager).
func (m *Manager) GetFilteredAssignmentCount() int {
	m.assignmentMu.RLock()
	defer m.assignmentMu.RUnlock()
	return len(m.assignedSourceIDs)
}

// decryptToken decrypts an encrypted access token
func (m *Manager) decryptToken(encryptedToken string) (string, error) {
	if m.cipher == nil || encryptedToken == "" {
		return encryptedToken, nil
	}
	return m.cipher.DecryptString(encryptedToken)
}

