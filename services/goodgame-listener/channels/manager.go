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
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/goodgame-listener/metrics"
	"github.com/caesar/all-chat/services/goodgame-listener/status"
	"github.com/caesar/all-chat/shared/listener"
	"github.com/caesar/all-chat/shared/sourcemanager"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Compile-time assertion: Manager must satisfy the SDK ChannelManager interface.
var _ listener.ChannelManager = (*Manager)(nil)

const (
	// Sync interval for checking active channels.
	syncInterval = 30 * time.Second

	// PostgreSQL notification channel for source changes.
	notificationChannel = "chat_source_changes"

	// Delay before retrying LISTEN connection.
	listenRetryDelay = 5 * time.Second
)

// WSClient is the subset of websocket.Client the manager drives.
// Numeric channel ids come straight from the resolver.
type WSClient interface {
	Subscribe(channelID int64) error
	Unsubscribe(channelID int64) error
	IsConnected() bool
}

// Manager manages GoodGame channel subscriptions.
type Manager struct {
	repo            *Repository
	resolver        *Resolver
	wsClient        WSClient
	logger          *zap.Logger
	dbConn          DBConnInterface
	leader          *sourcemanager.LeadershipCoordinator
	redisClient     *redis.Client
	podID           string
	statusPublisher *status.Publisher

	// Demand filtering (shared contract; see listener.ChannelManager).
	demandedSourceIDs       map[string]listener.DemandedSource
	filteredAssignmentCount int
	subsMu                  sync.RWMutex

	// Track active subscriptions: slug -> numeric chat id.
	subscriptions map[string]int64
	// Reverse index: numeric id -> slug (message routing by channel_id).
	slugIndex map[int64]string
	// Overlays consuming each subscribed slug (captured at subscribe time).
	overlayIDsBySlug map[string][]string
	// DB source ids per slug (demand reconciliation).
	sourceIDsBySlug map[string]string
	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// DBConnInterface allows dependency injection (kick-listener parity).
type DBConnInterface interface {
	GetPool() interface{}
}

// NewManager creates a new channel manager.
func NewManager(
	repo *Repository,
	resolver *Resolver,
	wsClient WSClient,
	dbConn DBConnInterface,
	leader *sourcemanager.LeadershipCoordinator,
	redisClient *redis.Client,
	podID string,
	logger *zap.Logger,
) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		repo:             repo,
		resolver:         resolver,
		wsClient:         wsClient,
		logger:           logger,
		dbConn:           dbConn,
		leader:           leader,
		redisClient:      redisClient,
		podID:            podID,
		subscriptions:    make(map[string]int64),
		slugIndex:        make(map[int64]string),
		overlayIDsBySlug: make(map[string][]string),
		sourceIDsBySlug:  make(map[string]string),
		ctx:              ctx,
		cancel:           cancel,
	}
}

// SetStatusPublisher injects the status publisher after construction.
func (m *Manager) SetStatusPublisher(pub *status.Publisher) {
	m.subsMu.Lock()
	defer m.subsMu.Unlock()
	m.statusPublisher = pub
}

// Start begins the channel management loop.
func (m *Manager) Start(_ context.Context) error {
	m.logger.Info("Starting GoodGame channel manager")

	if err := m.syncChannels(); err != nil {
		m.logger.Error("Initial channel sync failed", zap.Error(err))
	}

	m.wg.Add(1)
	go m.syncLoop()

	if m.dbConn != nil {
		m.wg.Add(1)
		go m.listenForChanges()
	} else {
		m.logger.Warn("Database connection not configured, skipping LISTEN/NOTIFY watcher")
	}

	m.logger.Info("GoodGame channel manager started", zap.Duration("sync_interval", syncInterval))
	return nil
}

// SyncNow triggers an immediate channel sync (used after WebSocket reconnect;
// safe to call concurrently — syncChannels serialises internally on subsMu).
func (m *Manager) SyncNow() error {
	return m.syncChannels()
}

// Stop stops the channel manager.
func (m *Manager) Stop() {
	m.cancel()
	m.wg.Wait()
}

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

func (m *Manager) listenForChanges() {
	defer m.wg.Done()

	for {
		select {
		case <-m.ctx.Done():
			return
		default:
		}

		poolInterface := m.dbConn.GetPool()
		if poolInterface == nil {
			m.logger.Error("Failed to get database pool for LISTEN")
			if !m.sleepWithContext(listenRetryDelay) {
				return
			}
			continue
		}

		if err := m.listenAndWait(poolInterface); err != nil {
			m.logger.Warn("PostgreSQL LISTEN error, will retry",
				zap.Error(err),
				zap.Duration("retry_in", listenRetryDelay),
			)
			if !m.sleepWithContext(listenRetryDelay) {
				return
			}
		}
	}
}

func (m *Manager) listenAndWait(poolInterface interface{}) error {
	pool, ok := poolInterface.(*pgxpool.Pool)
	if !ok {
		return fmt.Errorf("invalid pool type for LISTEN")
	}

	ctx := m.ctx

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, fmt.Sprintf("LISTEN %s", notificationChannel)); err != nil {
		return fmt.Errorf("failed to LISTEN on %s: %w", notificationChannel, err)
	}

	m.logger.Info("PostgreSQL LISTEN active", zap.String("channel", notificationChannel))

	const debounceWindow = 2 * time.Second
	notifyCh := make(chan struct{}, 1)

	go func() {
		var timer *time.Timer
		for {
			select {
			case <-m.ctx.Done():
				if timer != nil {
					timer.Stop()
				}
				return
			case <-notifyCh:
				if timer != nil {
					timer.Stop()
				}
				timer = time.NewTimer(debounceWindow)
			case <-func() <-chan time.Time {
				if timer != nil {
					return timer.C
				}
				return nil
			}():
				timer = nil
				if err := m.syncChannels(); err != nil {
					m.logger.Error("Failed to sync after notification", zap.Error(err))
				}
			}
		}
	}()

	for {
		select {
		case <-m.ctx.Done():
			return nil
		default:
		}

		notification, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return fmt.Errorf("notification wait failed: %w", err)
		}

		if !isGoodgameNotification(notification.Payload) {
			continue
		}

		m.logger.Info("Source change notification received", zap.String("payload", notification.Payload))
		select {
		case notifyCh <- struct{}{}:
		default:
		}
	}
}

type sourceChangePayload struct {
	Platform string `json:"platform"`
}

// isGoodgameNotification fails open on unparseable payloads (kick-listener parity).
func isGoodgameNotification(payload string) bool {
	var p sourceChangePayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return true
	}
	return p.Platform == "" || p.Platform == "goodgame"
}

func (m *Manager) sleepWithContext(d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-m.ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// syncChannels reconciles WebSocket channel memberships with the database.
// Slugs resolve to numeric chat ids through the cached resolver.
func (m *Manager) syncChannels() error {
	m.logger.Debug("Syncing GoodGame channels from database")

	channels, err := m.repo.GetActiveChannels(m.ctx)
	if err != nil {
		return fmt.Errorf("failed to get active channels: %w", err)
	}

	desired := make(map[string]int64, len(channels))
	overlaysBySlug := make(map[string]map[string]struct{})

	m.subsMu.RLock()
	demanded := m.demandedSourceIDs
	m.subsMu.RUnlock()

	for _, ch := range channels {
		if demanded != nil {
			if _, ok := demanded[ch.SourceID]; !ok {
				continue
			}
		}
		if _, seen := desired[ch.ChannelSlug]; !seen {
			overlaysBySlug[ch.ChannelSlug] = make(map[string]struct{})
		}
		overlaysBySlug[ch.ChannelSlug][ch.OverlayID] = struct{}{}
	}

	for slug := range overlaysBySlug {
		id, err := m.resolver.Resolve(m.ctx, slug)
		if err != nil {
			var nf *NotFoundError
			if errors.As(err, &nf) {
				m.logger.Warn("GoodGame channel not found, marking source inactive",
					zap.String("channel", slug), zap.Error(err))
				if setErr := m.repo.SetSourceActive(m.ctx, slug, false); setErr != nil {
					m.logger.Error("Failed to mark source inactive", zap.String("channel", slug), zap.Error(setErr))
				}
			} else {
				// Transient API failure: skip this cycle, keep subscription state.
				m.logger.Warn("Failed to resolve GoodGame channel id",
					zap.String("channel", slug), zap.Error(err))
			}
			continue
		}
		desired[slug] = id
	}

	m.subsMu.Lock()
	defer m.subsMu.Unlock()
	m.filteredAssignmentCount = len(desired)

	// Unjoin channels no longer desired.
	for slug, id := range m.subscriptions {
		if _, exists := desired[slug]; !exists {
			m.logger.Info("Unjoining channel", zap.String("channel", slug), zap.Int64("channel_id", id))
			if err := m.wsClient.Unsubscribe(id); err != nil {
				m.logger.Error("Failed to unsubscribe", zap.String("channel", slug), zap.Error(err))
			}
			delete(m.subscriptions, slug)
			delete(m.slugIndex, id)
			m.releaseLeadership(slug)
			metrics.ObserveSubscription("unsubscribe")

			if m.statusPublisher != nil {
				m.statusPublisher.Publish(m.ctx, status.Message{
					Platform:  "goodgame",
					ChannelID: slug,
					Status:    "offline",
				})
			}
		}
	}

	// Join new channels.
	for slug, id := range desired {
		if _, exists := m.subscriptions[slug]; exists {
			continue
		}

		if m.leader != nil {
			ok, err := m.leader.EnsureLeadership(m.ctx, slug, func(channel string) func() {
				lossCtx := context.Background()
				return func() {
					m.handleLeadershipLoss(lossCtx, channel)
				}
			}(slug))
			if err != nil {
				m.logger.Error("Failed to claim leadership", zap.String("channel", slug), zap.Error(err))
				continue
			}
			if !ok {
				m.logger.Debug("Skipping subscription; leadership owned elsewhere", zap.String("channel", slug))
				continue
			}
		}

		if err := m.wsClient.Subscribe(id); err != nil {
			m.logger.Error("Failed to subscribe",
				zap.String("channel", slug), zap.Int64("channel_id", id), zap.Error(err))
			m.releaseLeadership(slug)
			continue
		}

		m.subscriptions[slug] = id
		m.slugIndex[id] = slug
		for overlayID := range overlaysBySlug[slug] {
			m.overlayIDsBySlug[slug] = append(m.overlayIDsBySlug[slug], overlayID)
		}
		for _, ch := range channels {
			if ch.ChannelSlug == slug {
				m.sourceIDsBySlug[slug] = ch.SourceID
				break
			}
		}
		metrics.ObserveSubscription("subscribe")

		if err := m.repo.SetSourceActive(m.ctx, slug, true); err != nil {
			m.logger.Error("Failed to update source status after subscribe",
				zap.String("channel", slug), zap.Error(err))
		}

		if m.statusPublisher != nil {
			m.statusPublisher.Publish(m.ctx, status.Message{
				Platform:  "goodgame",
				ChannelID: slug,
				Status:    "connected",
			})
		}
	}

	metrics.SetActiveSubscriptions(len(m.subscriptions))
	m.logger.Info("Channel sync completed", zap.Int("active_subscriptions", len(m.subscriptions)))
	return nil
}

func (m *Manager) releaseLeadership(channelSlug string) {
	if m.leader == nil {
		return
	}
	m.leader.Release(channelSlug)
}

func (m *Manager) handleLeadershipLoss(_ context.Context, channelSlug string) {
	if m.leader == nil {
		return
	}

	m.subsMu.Lock()
	defer m.subsMu.Unlock()

	id, exists := m.subscriptions[channelSlug]
	if !exists {
		return
	}

	if err := m.wsClient.Unsubscribe(id); err != nil {
		m.logger.Error("Failed to unsubscribe after losing leadership",
			zap.String("channel", channelSlug), zap.Error(err))
	}

	delete(m.subscriptions, channelSlug)
	delete(m.slugIndex, id)
	m.logger.Warn("Dropped subscription after leadership loss (sources remain active in DB)",
		zap.String("channel", channelSlug))
	metrics.ObserveSubscription("unsubscribe")
}

// GetOverlayTargetsForChatroom returns all overlays consuming a chatroom id.
// chatroomID is parsed from the numeric channel id string that ride the
// GoodGame message frames.
func (m *Manager) GetOverlayTargetsForChannel(channelID int64) ([]OverlayTarget, bool) {
	m.subsMu.RLock()
	defer m.subsMu.RUnlock()

	slug, exists := m.slugIndex[channelID]
	if !exists {
		return nil, false
	}

	// Build the target list from the demand-filtered view of the DB. The
	// manager keeps one subscription per slug covering every overlay that
	// consumes it (syncChannels groups overlays by slug), so the overlay set
	// is reconstructed from repo state at call time via the sync snapshot.
	return m.snapshotOverlayTargets(slug), true
}

// snapshotOverlayTargets lists overlays recorded for a slug. Overlays are
// captured at subscribe time in overlayIDsBySlug; see syncChannels updates.
func (m *Manager) snapshotOverlayTargets(slug string) []OverlayTarget {
	// overlayIDsBySlug is written by syncChannels under subsMu.
	overlays := m.overlayIDsBySlug[slug]
	targets := make([]OverlayTarget, 0, len(overlays))
	for _, overlayID := range overlays {
		targets = append(targets, OverlayTarget{
			OverlayID:   overlayID,
			ChannelSlug: slug,
		})
	}
	return targets
}

// OverlayTarget represents an overlay consuming a channel.
type OverlayTarget struct {
	OverlayID   string
	ChannelSlug string
}

// SignalFirstMessage is a no-op slot retained for the ChannelManager
// interface (kick-listener uses it for migration confirmation).
func (m *Manager) SignalFirstMessage(int) {}

// GetAssignmentCount returns 0 — assignment-based coordination is not used;
// demand filtering drives this listener.
func (m *Manager) GetAssignmentCount() int { return 0 }

// GetFilteredAssignmentCount returns the number of resolved, demanded channels.
func (m *Manager) GetFilteredAssignmentCount() int {
	m.subsMu.RLock()
	defer m.subsMu.RUnlock()
	return m.filteredAssignmentCount
}

// GetActiveChannels returns the slugs of all currently subscribed channels.
func (m *Manager) GetActiveChannels() []string {
	m.subsMu.RLock()
	defer m.subsMu.RUnlock()
	slugs := make([]string, 0, len(m.subscriptions))
	for slug := range m.subscriptions {
		slugs = append(slugs, slug)
	}
	return slugs
}

// GetActiveChannelCount returns the number of currently subscribed channels.
func (m *Manager) GetActiveChannelCount() int {
	m.subsMu.RLock()
	defer m.subsMu.RUnlock()
	return len(m.subscriptions)
}

// UpdateAssignedSourceIDs is a no-op slot retained for interface stability.
func (m *Manager) UpdateAssignedSourceIDs(map[string]bool) {}

// UpdateDemandedSourceIDs stores the demanded set and triggers reconciliation.
func (m *Manager) UpdateDemandedSourceIDs(demanded map[string]listener.DemandedSource) {
	m.subsMu.Lock()
	m.demandedSourceIDs = demanded
	m.subsMu.Unlock()
	m.reconcileDemand()
}

// reconcileDemand unsubscribes channels whose source_id lost demand.
func (m *Manager) reconcileDemand() {
	m.subsMu.Lock()
	defer m.subsMu.Unlock()

	demanded := m.demandedSourceIDs
	if demanded == nil {
		return
	}

	for slug, id := range m.subscriptions {
		if _, ok := demanded[m.sourceIDBySlug(slug)]; ok {
			continue
		}
		m.logger.Info("Demand lost, unsubscribing channel", zap.String("channel", slug))
		if err := m.wsClient.Unsubscribe(id); err != nil {
			m.logger.Error("Failed to unsubscribe on demand loss",
				zap.String("channel", slug), zap.Error(err))
		}
		delete(m.subscriptions, slug)
		delete(m.slugIndex, id)
		m.releaseLeadership(slug)

		if m.statusPublisher != nil {
			m.statusPublisher.Publish(m.ctx, status.Message{
				Platform:  "goodgame",
				ChannelID: slug,
				Status:    "offline",
			})
		}
	}
	m.filteredAssignmentCount = len(m.subscriptions)
}

// sourceIDBySlug looks up the DB source id recorded for a subscription.
// Recorded at subscribe time in sourceIDsBySlug (populated by syncChannels).
func (m *Manager) sourceIDBySlug(slug string) string {
	return m.sourceIDsBySlug[slug]
}

// GetSubscriptionCount returns the number of active subscriptions.
func (m *Manager) GetSubscriptionCount() int {
	m.subsMu.RLock()
	defer m.subsMu.RUnlock()
	return len(m.subscriptions)
}

// IsConnected reports the WebSocket state.
func (m *Manager) IsConnected() bool {
	return m.wsClient != nil && m.wsClient.IsConnected()
}
