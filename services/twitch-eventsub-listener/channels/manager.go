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
	SourceIDs       []string // overlay_chat_sources.id values for this channel (used to match demand)
	HasChatScope    bool     // Owner granted user:read:chat → eligible for the channel.chat.message subscription
	ChatActive      bool     // A channel.chat.message subscription currently exists (managed by reconcileChat)
}

// SubscriptionCallback is invoked by the manager to create/delete subscriptions. action is one of:
//   - "subscribe":       create the event subscriptions (points, subs, raids, …) for the channel
//   - "subscribe_chat":   create the channel.chat.message subscription
//   - "unsubscribe_chat": delete only the channel.chat.message subscription
//   - "unsubscribe":      delete ALL subscriptions for the channel
//
// Event subscriptions are created for every active channel; the chat subscription is gated on
// chat scope AND live-overlay demand (see reconcileChatLocked), so it uses the *_chat actions.
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
	cipher   *encryption.MultiKeyEncryptor

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
func NewManager(db *pgxpool.Pool, logger *zap.Logger, resolver UserIDResolver, cipher *encryption.MultiKeyEncryptor, syncInterval time.Duration) *Manager {
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

// UpdateDemandedSourceIDs stores the demanded source set (sources whose overlay has a live
// WebSocket) and immediately reconciles chat subscriptions: chat is read only while an
// overlay using the channel is connected. Event subscriptions are NOT demand-gated — they
// exist for every active channel regardless. Called by the SDK demand loop (already filtered
// to the twitch platform via DemandPlatform).
func (m *Manager) UpdateDemandedSourceIDs(demanded map[string]listener.DemandedSource) {
	m.assignmentMu.Lock()
	m.demandedSourceIDs = demanded
	m.assignmentMu.Unlock()

	m.logger.Debug("Demand update received",
		zap.Int("demanded_sources", len(demanded)),
	)

	// Reconcile chat subscriptions now so a connecting overlay gets chat promptly and a
	// disconnecting one stops it promptly (rather than waiting for the next sync tick).
	m.mu.Lock()
	m.reconcileChatLocked(demanded)
	m.mu.Unlock()
}

// snapshotDemand returns the current demanded-source map (replaced wholesale by
// UpdateDemandedSourceIDs, so the returned reference is safe to read without the lock).
func (m *Manager) snapshotDemand() map[string]listener.DemandedSource {
	m.assignmentMu.RLock()
	defer m.assignmentMu.RUnlock()
	return m.demandedSourceIDs
}

// isChatDemanded reports whether any of the channel's sources currently has demand.
// A nil demanded map means "no demand information yet" (e.g. before the first snapshot or
// when the demand pipeline is unavailable) and is treated as demanded, so chat degrades to
// always-on rather than silently off.
func isChatDemanded(sourceIDs []string, demanded map[string]listener.DemandedSource) bool {
	if demanded == nil {
		return true
	}
	for _, sid := range sourceIDs {
		if _, ok := demanded[sid]; ok {
			return true
		}
	}
	return false
}

// reconcileChatLocked creates or deletes channel.chat.message subscriptions so that a chat
// subscription exists exactly for channels that (a) have the chat scope and (b) have live
// overlay demand. Must be called with m.mu held. Only the leader's callback performs real
// work (the callback is a no-op on non-leaders), so this is safe to run anywhere.
func (m *Manager) reconcileChatLocked(demanded map[string]listener.DemandedSource) {
	if m.callback == nil {
		return
	}
	for broadcasterID, ch := range m.channels {
		want := ch.HasChatScope && isChatDemanded(ch.SourceIDs, demanded)
		switch {
		case want && !ch.ChatActive:
			if err := m.callback(broadcasterID, ch.AccessToken, "subscribe_chat"); err != nil {
				m.logger.Warn("Failed to subscribe chat", zap.String("broadcaster_id", broadcasterID), zap.Error(err))
				continue
			}
			ch.ChatActive = true
		case !want && ch.ChatActive:
			if err := m.callback(broadcasterID, "", "unsubscribe_chat"); err != nil {
				m.logger.Warn("Failed to unsubscribe chat", zap.String("broadcaster_id", broadcasterID), zap.Error(err))
				continue
			}
			ch.ChatActive = false
		}
	}
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
	// Query active Twitch sources and join with users table to get OAuth tokens.
	// This selects ALL active Twitch sources with a connected owner (LEFT JOIN; rows without
	// a token are skipped below) so the existing event subscriptions (points, subs, raids, …)
	// are created for every connected channel — unchanged by the chat feature.
	//
	// has_chat_scope marks whether the owner additionally granted user:read:chat. It gates
	// ONLY the channel.chat.message subscription (see the leader callback in cmd/main.go).
	// It is the EventSub side of the IRC↔EventSub partition: twitch-listener (IRC) excludes
	// exactly the channels where this predicate is true, via a byte-identical NOT EXISTS in
	// services/twitch-listener/channels/repository.go — so a chat-scoped channel is read by
	// EventSub and a non-scoped one by IRC, never both, never neither.
	query := `
		SELECT DISTINCT
			ocs.id,
			ocs.channel_id,
			ocs.overlay_id,
			u.access_token,
			u.token_expires_at,
			COALESCE('user:read:chat' = ANY(u.granted_scopes), false) AS has_chat_scope
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
		var hasChatScope bool

		if err := rows.Scan(&sourceID, &channelID, &overlayID, &accessToken, &tokenExpiresAt, &hasChatScope); err != nil {
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
				SourceIDs:       []string{},
				HasChatScope:    hasChatScope,
			}
		}

		activeChannels[broadcasterID].OverlayIDs = append(activeChannels[broadcasterID].OverlayIDs, overlayID)
		activeChannels[broadcasterID].SourceIDs = append(activeChannels[broadcasterID].SourceIDs, sourceID)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating rows: %w", err)
	}

	m.logger.Debug("Synced active channels from database",
		zap.Int("active_channels", len(activeChannels)),
	)

	// Snapshot demand before taking m.mu so chat reconciliation uses a consistent view
	// without nesting locks.
	demanded := m.snapshotDemand()

	// Compare with current channels and create/remove subscriptions
	m.mu.Lock()
	defer m.mu.Unlock()

	// New channels: create the event subscriptions. Existing channels: refresh mutable
	// fields (token, overlays, source IDs, chat scope) so re-consents and source changes are
	// picked up — preserving ChatActive, since the chat subscription lifecycle is owned by
	// reconcileChatLocked (gated on chat scope AND live-overlay demand) below.
	for broadcasterID, fresh := range activeChannels {
		existing, exists := m.channels[broadcasterID]
		if !exists {
			m.logger.Info("New channel detected",
				zap.String("broadcaster_id", broadcasterID),
				zap.Int("overlay_count", len(fresh.OverlayIDs)),
				zap.Bool("has_chat_scope", fresh.HasChatScope),
			)
			if m.callback != nil {
				if err := m.callback(broadcasterID, fresh.AccessToken, "subscribe"); err != nil {
					m.logger.Error("Failed to subscribe to channel",
						zap.String("broadcaster_id", broadcasterID),
						zap.Error(err),
					)
					continue
				}
			}
			m.channels[broadcasterID] = fresh
		} else {
			existing.AccessToken = fresh.AccessToken
			existing.OverlayIDs = fresh.OverlayIDs
			existing.SourceIDs = fresh.SourceIDs
			existing.HasChatScope = fresh.HasChatScope
		}
	}

	// Find removed channels (delete ALL their subscriptions)
	for broadcasterID := range m.channels {
		if _, exists := activeChannels[broadcasterID]; !exists {
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

	// Create/delete chat subscriptions per chat-scope AND live-overlay demand.
	m.reconcileChatLocked(demanded)

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
