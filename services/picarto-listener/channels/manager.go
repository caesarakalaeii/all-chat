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

	"github.com/caesar/all-chat/services/picarto-listener/metrics"
	"github.com/caesar/all-chat/services/picarto-listener/publisher"
	"github.com/caesar/all-chat/services/picarto-listener/status"
	"github.com/caesar/all-chat/shared/listener"
	"github.com/caesar/all-chat/shared/sourcemanager"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Compile-time assertion: Manager must satisfy the SDK ChannelManager interface.
var _ listener.ChannelManager = (*Manager)(nil)

const (
	// Sync interval for checking active channels
	syncInterval = 30 * time.Second

	// PostgreSQL notification channel for source changes
	notificationChannel = "chat_source_changes"

	// Delay before retrying LISTEN connection
	listenRetryDelay = 5 * time.Second
)

// DBConnInterface allows for dependency injection
type DBConnInterface interface {
	GetPool() interface{}
}

// wsClient is the subset of websocket.Client the manager drives.
type wsClient interface {
	Connect(channelName string) error
	Disconnect(channelName string)
	IsConnected(channelName string) bool
}

// Repository handles database operations for channels
type Repository struct {
	db     queryExecutor
	logger *zap.Logger
}

type queryExecutor interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// NewRepository creates a new channel repository backed by a pgx pool.
func NewRepository(pool *pgxpool.Pool, logger *zap.Logger) *Repository {
	return &Repository{db: pool, logger: logger}
}

// ActiveChannel represents an active Picarto channel to monitor.
type ActiveChannel struct {
	SourceID    string // UUID from overlay_chat_sources.id
	OverlayID   string
	ChannelName string // Picarto channel name (the subscription key)
	IsActive    bool
}

// GetActiveChannels retrieves all Picarto channels eligible for connection.
func (r *Repository) GetActiveChannels(ctx context.Context) ([]*ActiveChannel, error) {
	query := `
		SELECT
			ocs.id as source_id,
			ocs.overlay_id,
			COALESCE(ocs.channel_handle, ocs.channel_name) as channel_name,
			ocs.is_active
		FROM overlay_chat_sources ocs
		JOIN overlays o ON ocs.overlay_id = o.id
		WHERE ocs.platform = 'picarto'
		  AND o.is_active = true
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active channels: %w", err)
	}
	defer rows.Close()

	var channels []*ActiveChannel
	for rows.Next() {
		var ch ActiveChannel
		if err := rows.Scan(&ch.SourceID, &ch.OverlayID, &ch.ChannelName, &ch.IsActive); err != nil {
			r.logger.Error("Failed to scan channel row", zap.Error(err))
			continue
		}
		channels = append(channels, &ch)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating channel rows: %w", err)
	}

	r.logger.Info("Retrieved active Picarto channels", zap.Int("count", len(channels)))
	return channels, nil
}

// SetSourceActive updates the is_active flag for Picarto sources with the given channel name.
func (r *Repository) SetSourceActive(ctx context.Context, channelName string, isActive bool) error {
	query := `
		UPDATE overlay_chat_sources
		SET is_active = $1, updated_at = NOW()
		WHERE platform = 'picarto'
		  AND (channel_handle = $2 OR channel_name = $2)
		  AND is_active != $1
	`
	result, err := r.db.Exec(ctx, query, isActive, channelName)
	if err != nil {
		return fmt.Errorf("failed to update source status: %w", err)
	}
	if result.RowsAffected() > 0 {
		r.logger.Debug("Updated source status",
			zap.String("channel", channelName),
			zap.Bool("is_active", isActive),
		)
	}
	return nil
}

// trackedChannel is one subscribed channel and the overlays consuming it.
type trackedChannel struct {
	ChannelName string
	SourceID    string
	OverlayIDs  map[string]struct{}
}

// OverlayTarget represents an overlay consuming a channel.
type OverlayTarget struct {
	OverlayID   string
	ChannelName string
}

// Manager keeps the set of live Picarto connections in sync with the database.
type Manager struct {
	repo      *Repository
	wsClient  wsClient
	publisher *publisher.StreamPublisher
	logger    *zap.Logger
	dbConn    DBConnInterface
	leader    *sourcemanager.LeadershipCoordinator

	redisClient *redis.Client
	podID       string

	statusPublisher *status.Publisher

	demandedSourceIDs       map[string]listener.DemandedSource
	filteredAssignmentCount int

	subscriptions map[string]*trackedChannel // key: channel name (lowercase)
	subsMu        sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewManager creates a new channel manager.
func NewManager(
	repo *Repository,
	wsClient wsClient,
	pub *publisher.StreamPublisher,
	dbConn DBConnInterface,
	leader *sourcemanager.LeadershipCoordinator,
	redisClient *redis.Client,
	podID string,
	logger *zap.Logger,
) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		repo:          repo,
		wsClient:      wsClient,
		publisher:     pub,
		logger:        logger,
		dbConn:        dbConn,
		leader:        leader,
		redisClient:   redisClient,
		podID:         podID,
		subscriptions: make(map[string]*trackedChannel),
		ctx:           ctx,
		cancel:        cancel,
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
	m.logger.Info("Starting Picarto channel manager")

	if err := m.syncChannels(); err != nil {
		m.logger.Error("Initial channel sync failed", zap.Error(err))
	}

	m.wg.Add(1)
	go m.syncLoop()

	// Picarto connects one lightweight WebSocket per channel; the 30 s sync
	// loop reconciles source changes, so no LISTEN/NOTIFY watcher is needed.

	m.logger.Info("Picarto channel manager started", zap.Duration("sync_interval", syncInterval))
	return nil
}

// Stop stops the channel manager and all connections.
func (m *Manager) Stop() {
	m.logger.Info("Stopping Picarto channel manager")
	m.cancel()
	m.wg.Wait()
	if m.leader != nil {
		m.leader.Stop()
	}
	m.logger.Info("Picarto channel manager stopped")
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

// GetOverlayTargetsForChannel returns the overlays consuming a channel.
func (m *Manager) GetOverlayTargetsForChannel(channelName string) ([]OverlayTarget, bool) {
	m.subsMu.RLock()
	defer m.subsMu.RUnlock()

	ch, exists := m.subscriptions[lower(channelName)]
	if !exists {
		return nil, false
	}
	targets := make([]OverlayTarget, 0, len(ch.OverlayIDs))
	for overlayID := range ch.OverlayIDs {
		targets = append(targets, OverlayTarget{OverlayID: overlayID, ChannelName: ch.ChannelName})
	}
	return targets, true
}

func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 32
		}
	}
	return string(b)
}

// syncChannels reconciles live connections with the database source list.
func (m *Manager) syncChannels() error {
	channels, err := m.repo.GetActiveChannels(m.ctx)
	if err != nil {
		return fmt.Errorf("failed to get active channels: %w", err)
	}

	m.subsMu.RLock()
	demanded := m.demandedSourceIDs
	m.subsMu.RUnlock()

	if demanded != nil {
		filtered := make([]*ActiveChannel, 0, len(demanded))
		for _, ch := range channels {
			if _, ok := demanded[ch.SourceID]; ok {
				filtered = append(filtered, ch)
			}
		}
		m.logger.Info("Filtered channels by demand",
			zap.Int("total_channels", len(channels)),
			zap.Int("demanded_channels", len(filtered)),
		)
		channels = filtered
	}

	// Fold sources by channel: one connection per Picarto channel, possibly
	// consumed by many overlays.
	desired := make(map[string]*trackedChannel, len(channels))
	for _, ch := range channels {
		key := lower(ch.ChannelName)
		tc, exists := desired[key]
		if !exists {
			tc = &trackedChannel{
				ChannelName: ch.ChannelName,
				SourceID:    ch.SourceID,
				OverlayIDs:  make(map[string]struct{}),
			}
			desired[key] = tc
		}
		tc.OverlayIDs[ch.OverlayID] = struct{}{}
	}

	m.subsMu.Lock()
	defer m.subsMu.Unlock()
	m.filteredAssignmentCount = len(desired)

	// Drop connections for channels no longer active or demanded.
	for key, ch := range m.subscriptions {
		if _, exists := desired[key]; exists {
			continue
		}
		m.logger.Info("Disconnecting channel", zap.String("channel", ch.ChannelName))
		m.wsClient.Disconnect(ch.ChannelName)
		delete(m.subscriptions, key)
		metrics.ObserveSubscription("unsubscribe")

		if m.statusPublisher != nil {
			m.statusPublisher.Publish(m.ctx, status.Message{
				Platform:  "picarto",
				ChannelID: ch.ChannelName,
				Status:    "offline",
			})
		}
	}

	// Connect new channels.
	for key, want := range desired {
		if _, exists := m.subscriptions[key]; exists {
			// Keep overlay set fresh; reconnect if the socket dropped.
			m.subscriptions[key].OverlayIDs = want.OverlayIDs
			if !m.wsClient.IsConnected(want.ChannelName) {
				if err := m.wsClient.Connect(want.ChannelName); err != nil {
					m.logger.Warn("Failed to reconnect channel",
						zap.String("channel", want.ChannelName),
						zap.Error(err),
					)
				}
			}
			continue
		}

		if err := m.wsClient.Connect(want.ChannelName); err != nil {
			m.logger.Error("Failed to connect channel",
				zap.String("channel", want.ChannelName),
				zap.Error(err),
			)
			continue
		}

		m.subscriptions[key] = want
		metrics.ObserveSubscription("subscribe")

		if err := m.repo.SetSourceActive(m.ctx, want.ChannelName, true); err != nil {
			m.logger.Error("Failed to update source status after connect",
				zap.String("channel", want.ChannelName),
				zap.Error(err),
			)
		}

		if m.statusPublisher != nil {
			m.statusPublisher.Publish(m.ctx, status.Message{
				Platform:  "picarto",
				ChannelID: want.ChannelName,
				Status:    "connected",
			})
		}
	}

	metrics.SetActiveSubscriptions(len(m.subscriptions))
	m.logger.Info("Channel sync completed", zap.Int("active_subscriptions", len(m.subscriptions)))
	return nil
}

// IsConnected reports whether the manager has at least one live connection.
func (m *Manager) IsConnected() bool {
	m.subsMu.RLock()
	name := ""
	for _, ch := range m.subscriptions {
		name = ch.ChannelName
		break
	}
	m.subsMu.RUnlock()
	return m.wsClient != nil && name != "" && m.wsClient.IsConnected(name)
}

// UpdateDemandedSourceIDs updates the demanded source set and reconciles.
func (m *Manager) UpdateDemandedSourceIDs(demanded map[string]listener.DemandedSource) {
	m.subsMu.Lock()
	m.demandedSourceIDs = demanded
	m.subsMu.Unlock()

	if err := m.syncChannels(); err != nil {
		m.logger.Error("Failed to sync after demand update", zap.Error(err))
	}
}

// UpdateAssignedSourceIDs is a no-op retained for interface stability.
func (m *Manager) UpdateAssignedSourceIDs(map[string]bool) {}

// GetFilteredAssignmentCount returns the demanded channel count.
func (m *Manager) GetFilteredAssignmentCount() int {
	m.subsMu.RLock()
	defer m.subsMu.RUnlock()
	return m.filteredAssignmentCount
}

// GetActiveChannels returns connected channel names.
func (m *Manager) GetActiveChannels() []string {
	m.subsMu.RLock()
	defer m.subsMu.RUnlock()
	names := make([]string, 0, len(m.subscriptions))
	for _, ch := range m.subscriptions {
		names = append(names, ch.ChannelName)
	}
	return names
}

// GetActiveChannelCount returns the number of connected channels.
func (m *Manager) GetActiveChannelCount() int {
	m.subsMu.RLock()
	defer m.subsMu.RUnlock()
	return len(m.subscriptions)
}

// GetSubscriptionCount returns the number of active subscriptions.
func (m *Manager) GetSubscriptionCount() int {
	m.subsMu.RLock()
	defer m.subsMu.RUnlock()
	return len(m.subscriptions)
}
