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
	"strings"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/twitch-eventsub-listener/status"
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

	// statusPublisher emits platform:status "offline" when a channel's chat subscription is
	// torn down (demand lost or channel removed). The matching "connected" is published by the
	// webhook handler when Twitch verifies the chat subscription (it is actually enabled by
	// then). nil disables status publishing.
	statusPublisher *status.Publisher

	mu       sync.RWMutex
	channels map[string]*Channel // broadcaster_id -> Channel

	// Coordinator integration for sharding
	assignedSourceIDs map[string]bool                    // source_id -> bool (assigned to this pod)
	demandedSourceIDs map[string]listener.DemandedSource // nil = no demand filtering
	podName           string
	assignmentMu      sync.RWMutex

	// Resolve cache: login → broadcaster_id. A Twitch user ID is immutable, so positive
	// results are cached for the process lifetime; unresolvable logins (deleted/invalid
	// accounts) are negatively cached for resolveNegativeTTL. This avoids hitting the Twitch
	// API for every source on every sync.
	resolveMu    sync.Mutex
	resolveCache map[string]string
	resolveNeg   map[string]time.Time

	// syncSignal triggers an immediate (coalesced) SyncChannels when demand changes, so a
	// newly-connected overlay's channel is subscribed without waiting for the periodic tick.
	syncSignal chan struct{}

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
		resolveCache: make(map[string]string),
		resolveNeg:   make(map[string]time.Time),
		syncSignal:   make(chan struct{}, 1),
		stopChan:     make(chan struct{}),
		syncInterval: syncInterval,
	}
}

// resolveNegativeTTL bounds how long an unresolvable login (deleted/invalid account) stays
// negatively cached before being retried.
const resolveNegativeTTL = time.Hour

// resolveBroadcasterID resolves a Twitch login to its (immutable) broadcaster user ID,
// caching results so the listener does not call the Twitch API for every channel on every
// sync. Failures are negatively cached for resolveNegativeTTL so deleted accounts are not
// re-resolved each cycle.
func (m *Manager) resolveBroadcasterID(ctx context.Context, login string) (string, error) {
	m.resolveMu.Lock()
	if id, ok := m.resolveCache[login]; ok {
		m.resolveMu.Unlock()
		return id, nil
	}
	if failedAt, ok := m.resolveNeg[login]; ok && time.Since(failedAt) < resolveNegativeTTL {
		m.resolveMu.Unlock()
		return "", fmt.Errorf("login %q unresolved (negative-cached)", login)
	}
	m.resolveMu.Unlock()

	id, err := m.resolver.GetUserIDByLogin(ctx, login)

	m.resolveMu.Lock()
	if err != nil {
		m.resolveNeg[login] = time.Now()
	} else {
		m.resolveCache[login] = id
		delete(m.resolveNeg, login)
	}
	m.resolveMu.Unlock()
	return id, err
}

// SetSubscriptionCallback sets the callback for channel changes
func (m *Manager) SetSubscriptionCallback(callback SubscriptionCallback) {
	m.callback = callback
}

// SetStatusPublisher injects the platform-status publisher used to emit "offline" on chat
// teardown. Safe to leave unset (publishing becomes a no-op).
func (m *Manager) SetStatusPublisher(pub *status.Publisher) {
	m.statusPublisher = pub
}

// ResetTracking drops the in-memory channel map so the next SyncChannels treats every active
// channel as new and (re)creates its subscriptions. Called when this pod ACQUIRES leadership:
// the manager runs on every pod (started by LeadershipListener.Start), so a standby may have
// populated the map while its subscription callback was a no-op; clearing it ensures a freshly
// promoted leader actually creates the subscriptions instead of seeing stale "already tracked"
// entries and skipping them. Real Twitch subscriptions are not deleted here — re-creation is
// idempotent (Twitch returns "already exists").
func (m *Manager) ResetTracking() {
	m.mu.Lock()
	m.channels = make(map[string]*Channel)
	m.mu.Unlock()
}

// TriggerSync requests an immediate (coalesced) SyncChannels via the periodic sync goroutine,
// without blocking. Used right after acquiring leadership so subscriptions are rebuilt promptly
// instead of waiting for the next periodic tick.
func (m *Manager) TriggerSync() {
	select {
	case m.syncSignal <- struct{}{}:
	default:
	}
}

// publishChatOffline emits a platform:status "offline" for the channel's chat indicator,
// keyed by the lowercased login (channel_id in overlay_chat_sources) so the api-gateway
// status subscriber routes it to the right overlays — matching the IRC listener's convention.
//
// It publishes on a background goroutine with its own timeout: callers hold m.mu (it is invoked
// from reconcileChatLocked / SyncChannels), and the publisher retries with backoff on Redis
// failure, so a synchronous call would hold the manager lock across that retry window.
func (m *Manager) publishChatOffline(login string) {
	if m.statusPublisher == nil || login == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		m.statusPublisher.Publish(ctx, status.Message{
			Platform:  "twitch",
			ChannelID: strings.ToLower(login),
			Status:    "offline",
		})
	}()
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
// WebSocket). EventSub work is demand-gated: SyncChannels only resolves/subscribes channels
// that are demanded, so a channel with no connected overlay carries no subscriptions at all.
// Called by the SDK demand loop (already filtered to the twitch platform via DemandPlatform).
func (m *Manager) UpdateDemandedSourceIDs(demanded map[string]listener.DemandedSource) {
	m.assignmentMu.Lock()
	m.demandedSourceIDs = demanded
	m.assignmentMu.Unlock()

	m.logger.Debug("Demand update received",
		zap.Int("demanded_sources", len(demanded)),
	)

	// Immediately unsubscribe chat for already-tracked channels that just lost demand (stops
	// webhook traffic promptly without waiting for the sync to remove the channel).
	m.mu.Lock()
	m.reconcileChatLocked(demanded)
	m.mu.Unlock()

	// Trigger a sync (coalesced) to pick up channels that just gained demand and drop ones
	// that lost it. Non-blocking: a pending signal already covers this update.
	select {
	case m.syncSignal <- struct{}{}:
	default:
	}
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
			// Chat reading stopped for this channel — clear its overlay indicator.
			m.publishChatOffline(ch.BroadcasterName)
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
			case <-m.syncSignal:
				if err := m.SyncChannels(ctx); err != nil {
					m.logger.Error("Channel sync failed (demand-triggered)", zap.Error(err))
				}
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

// QueryActiveTwitchChannelCredentials selects every active Twitch source with the best
// available OAuth credential for its channel. has_chat_scope marks whether the owner
// granted user:read:chat — it gates the channel.chat.message subscription and is the
// EventSub side of the IRC↔EventSub partition (see ADR-0015; the IRC exclusion is
// claim-based in twitch-listener).
//
// Credentials come from two places (ADR-0016): the channel owner's Twitch-login users
// row, or linked Twitch credentials in twitch_oauth_tokens (YouTube/Kick-login accounts
// that completed the Twitch add-source consent). The LATERAL picks ONE credential per
// channel, preferring whichever is chat-scoped AND unexpired, with the users row
// breaking ties. Exported for test assertions.
const QueryActiveTwitchChannelCredentials = `
	SELECT
		ocs.id,
		ocs.channel_id,
		ocs.overlay_id,
		cred.access_token,
		cred.token_expires_at,
		COALESCE(cred.has_chat_scope, false) AS has_chat_scope
	FROM overlay_chat_sources ocs
	JOIN overlays o ON ocs.overlay_id = o.id
	LEFT JOIN LATERAL (
		SELECT c.access_token, c.token_expires_at, c.has_chat_scope
		FROM (
			SELECT u.access_token, u.token_expires_at,
			       COALESCE('user:read:chat' = ANY(u.granted_scopes), false) AS has_chat_scope,
			       1 AS pri
			FROM users u
			WHERE LOWER(u.username) = LOWER(ocs.channel_id)
			  AND u.auth_provider = 'twitch'
			UNION ALL
			SELECT t.access_token, t.token_expires_at,
			       'user:read:chat' = ANY(t.granted_scopes) AS has_chat_scope,
			       2 AS pri
			FROM twitch_oauth_tokens t
			WHERE LOWER(t.twitch_login) = LOWER(ocs.channel_id)
		) c
		ORDER BY (c.has_chat_scope AND c.token_expires_at > NOW()) DESC, c.pri ASC
		LIMIT 1
	) cred ON TRUE
	WHERE ocs.platform = 'twitch'
	  AND ocs.is_active = true
	  AND o.is_active = true
	ORDER BY ocs.channel_id
`

// SyncChannels queries the database for active Twitch channels and creates/removes subscriptions
func (m *Manager) SyncChannels(ctx context.Context) error {
	// Demand snapshot: when non-nil we process only sources whose overlay is currently
	// connected (see the skip in the scan loop), so an idle channel costs no resolve and no
	// subscriptions. nil means "no demand info yet" and fails open (process everything).
	demanded := m.snapshotDemand()

	rows, err := m.db.Query(ctx, QueryActiveTwitchChannelCredentials)
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

		// Demand gate: skip sources whose overlay isn't currently connected. Done BEFORE the
		// (cached) resolve so idle channels cost nothing — no Twitch API call, no subscription.
		// nil demand fails open (process everything). Only the leader actually creates subs
		// (the callback is a no-op on non-leaders).
		if demanded != nil {
			if _, ok := demanded[sourceID]; !ok {
				continue
			}
		}

		// channelID from the database is the Twitch login; resolve it (cached) to the
		// broadcaster_user_id needed by the EventSub subscription condition.
		broadcasterID, err := m.resolveBroadcasterID(ctx, channelID)
		if err != nil {
			m.logger.Debug("Skipping channel: failed to resolve username to user ID",
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

		// Check if token is expired (debug-level: with demand gating this only concerns
		// channels someone is actively watching, but it can still be noisy on rollover).
		if tokenExpiresAt != nil && time.Now().After(*tokenExpiresAt) {
			m.logger.Debug("Skipping channel with expired OAuth token",
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

	// Compare with current channels and create/remove subscriptions. Uses the demand snapshot
	// taken at the top of this sync.
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
				// Create the chat subscription FIRST when the channel already has demand: it is
				// the latency-sensitive, overlay-visible path, so we don't make it wait behind
				// the (slower, sequential) always-on event-subscription setup. The chat callback
				// is idempotent, so a later reconcileChatLocked pass is a no-op.
				if fresh.HasChatScope && isChatDemanded(fresh.SourceIDs, demanded) {
					if err := m.callback(broadcasterID, fresh.AccessToken, "subscribe_chat"); err != nil {
						m.logger.Warn("Failed to subscribe chat for new channel",
							zap.String("broadcaster_id", broadcasterID),
							zap.Error(err),
						)
					} else {
						fresh.ChatActive = true
					}
				}

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
	for broadcasterID, ch := range m.channels {
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

			// Removing the channel deletes its chat subscription too — clear the indicator.
			if ch.ChatActive {
				m.publishChatOffline(ch.BroadcasterName)
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
