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
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/altstream-listener/metrics"
	"github.com/caesar/all-chat/services/altstream-listener/status"
	"github.com/caesar/all-chat/shared/listener"
	"github.com/caesar/all-chat/shared/sourcemanager"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Compile-time assertion: Manager must satisfy the SDK ChannelManager interface.
var _ listener.ChannelManager = (*Manager)(nil)

// syncInterval is how often the manager reconciles streams with the database.
const syncInterval = 30 * time.Second

// validUsername is an Altstream username after lowercasing (observed charset
// [a-z0-9_]). Anything else in channel_name is skipped, never dialled.
var validUsername = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)

// streamClient is the subset of altstream.Client the manager drives.
type streamClient interface {
	Connect(channel string) error
	Disconnect(channel string)
	IsConnected(channel string) bool
}

type queryExecutor interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Repository handles database operations for channels
type Repository struct {
	db     queryExecutor
	logger *zap.Logger
}

// NewRepository creates a new channel repository backed by a pgx pool.
func NewRepository(pool *pgxpool.Pool, logger *zap.Logger) *Repository {
	return &Repository{db: pool, logger: logger}
}

// ActiveChannel is one overlay source reading an Altstream channel.
type ActiveChannel struct {
	SourceID    string // UUID from overlay_chat_sources.id
	OverlayID   string
	ChannelName string // lowercased Altstream username (the stream key)
	IsActive    bool
}

// GetActiveChannels returns every Altstream source of an active overlay.
// source.is_active is a status flag the listener sets, not an eligibility
// filter: demand decides which channels to stream.
func (r *Repository) GetActiveChannels(ctx context.Context) ([]*ActiveChannel, error) {
	query := `
		SELECT
			ocs.id as source_id,
			ocs.overlay_id,
			COALESCE(ocs.channel_handle, ocs.channel_name) as channel_name,
			ocs.is_active
		FROM overlay_chat_sources ocs
		JOIN overlays o ON ocs.overlay_id = o.id
		WHERE ocs.platform = 'altstream'
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
		ch.ChannelName = strings.ToLower(strings.TrimSpace(ch.ChannelName))
		if !validUsername.MatchString(ch.ChannelName) {
			r.logger.Warn("Skipping source with invalid Altstream username",
				zap.String("source_id", ch.SourceID), zap.String("channel", ch.ChannelName))
			continue
		}
		channels = append(channels, &ch)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating channel rows: %w", err)
	}
	return channels, nil
}

// SetSourceActive updates the is_active flag for Altstream sources of a channel.
func (r *Repository) SetSourceActive(ctx context.Context, channelName string, isActive bool) error {
	query := `
		UPDATE overlay_chat_sources
		SET is_active = $1, updated_at = NOW()
		WHERE platform = 'altstream'
		  AND LOWER(COALESCE(channel_handle, channel_name)) = $2
		  AND is_active != $1
	`
	if _, err := r.db.Exec(ctx, query, isActive, channelName); err != nil {
		return fmt.Errorf("failed to update source status: %w", err)
	}
	return nil
}

// Manager keeps the set of Altstream streams in sync with the database and
// the SDK demand set, claiming per-channel leadership so HPA replicas split
// channels instead of double-ingesting them.
type Manager struct {
	repo            *Repository
	client          streamClient
	leader          *sourcemanager.LeadershipCoordinator // nil-safe
	statusPublisher *status.Publisher
	logger          *zap.Logger

	demandedSourceIDs       map[string]listener.DemandedSource
	filteredAssignmentCount int

	subscriptions map[string]map[string]struct{} // lowercased username -> overlay ids
	subsMu        sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewManager creates a new channel manager. leader and statusPublisher may be nil.
func NewManager(
	repo *Repository,
	client streamClient,
	leader *sourcemanager.LeadershipCoordinator,
	statusPublisher *status.Publisher,
	logger *zap.Logger,
) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		repo:            repo,
		client:          client,
		leader:          leader,
		statusPublisher: statusPublisher,
		logger:          logger,
		subscriptions:   make(map[string]map[string]struct{}),
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Start runs the initial sync and the periodic reconciliation loop.
func (m *Manager) Start(_ context.Context) error {
	if err := m.syncChannels(); err != nil {
		m.logger.Error("Initial channel sync failed", zap.Error(err))
	}
	m.wg.Add(1)
	go m.syncLoop()
	m.logger.Info("Altstream channel manager started", zap.Duration("sync_interval", syncInterval))
	return nil
}

// Stop stops the reconciliation loop and releases leadership.
func (m *Manager) Stop() {
	m.cancel()
	m.wg.Wait()
	m.leader.Stop()
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

// OverlayIDsForChannel returns the overlays consuming a channel.
func (m *Manager) OverlayIDsForChannel(channel string) []string {
	m.subsMu.RLock()
	defer m.subsMu.RUnlock()
	overlays := m.subscriptions[strings.ToLower(channel)]
	ids := make([]string, 0, len(overlays))
	for id := range overlays {
		ids = append(ids, id)
	}
	return ids
}

// syncChannels reconciles streams with the database source list.
func (m *Manager) syncChannels() error {
	channels, err := m.repo.GetActiveChannels(m.ctx)
	if err != nil {
		return fmt.Errorf("failed to get active channels: %w", err)
	}

	m.subsMu.Lock()
	defer m.subsMu.Unlock()

	// Fold sources by channel: one stream per channel, possibly consumed by
	// many overlays. Without a demand set yet, every source counts.
	desired := make(map[string]map[string]struct{}, len(channels))
	for _, ch := range channels {
		if m.demandedSourceIDs != nil {
			if _, ok := m.demandedSourceIDs[ch.SourceID]; !ok {
				continue
			}
		}
		if desired[ch.ChannelName] == nil {
			desired[ch.ChannelName] = map[string]struct{}{}
		}
		desired[ch.ChannelName][ch.OverlayID] = struct{}{}
	}
	m.filteredAssignmentCount = len(desired)

	for name := range m.subscriptions {
		if _, ok := desired[name]; !ok {
			m.dropLocked(name, "Stopping stream: no source demands it")
			m.leader.Release(name)
			m.publishStatus(name, "offline")
		}
	}

	for name, overlays := range desired {
		if _, ok := m.subscriptions[name]; ok {
			m.subscriptions[name] = overlays
			continue
		}

		ok, err := m.leader.EnsureLeadership(m.ctx, name, func() { m.handleLeadershipLoss(name) })
		if err != nil {
			m.logger.Error("Failed to claim leadership", zap.String("channel", name), zap.Error(err))
			continue
		}
		if !ok {
			m.logger.Debug("Skipping channel; leadership owned elsewhere", zap.String("channel", name))
			continue
		}

		if err := m.client.Connect(name); err != nil {
			m.logger.Error("Failed to start stream", zap.String("channel", name), zap.Error(err))
			m.leader.Release(name)
			continue
		}
		m.subscriptions[name] = overlays
		metrics.ObserveSubscription("subscribe")

		if err := m.repo.SetSourceActive(m.ctx, name, true); err != nil {
			m.logger.Error("Failed to update source status after connect", zap.String("channel", name), zap.Error(err))
		}
		m.publishStatus(name, "connected")
	}

	metrics.SetActiveSubscriptions(len(m.subscriptions))
	m.logger.Info("Channel sync completed", zap.Int("active_subscriptions", len(m.subscriptions)))
	return nil
}

// dropLocked stops a channel's stream. Caller holds subsMu.
func (m *Manager) dropLocked(name, why string) {
	m.logger.Info(why, zap.String("channel", name))
	m.client.Disconnect(name)
	delete(m.subscriptions, name)
	metrics.ObserveSubscription("unsubscribe")
}

// handleLeadershipLoss stops a channel another pod now leads. The source
// stays active and no offline status goes out: the new leader streams it.
func (m *Manager) handleLeadershipLoss(name string) {
	m.subsMu.Lock()
	defer m.subsMu.Unlock()
	if _, ok := m.subscriptions[name]; ok {
		m.dropLocked(name, "Stopping stream after leadership loss")
	}
}

func (m *Manager) publishStatus(name, state string) {
	if m.statusPublisher != nil {
		m.statusPublisher.Publish(m.ctx, status.Message{Platform: "altstream", ChannelID: name, Status: state})
	}
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

// GetActiveChannels returns the streamed channel names.
func (m *Manager) GetActiveChannels() []string {
	m.subsMu.RLock()
	defer m.subsMu.RUnlock()
	names := make([]string, 0, len(m.subscriptions))
	for name := range m.subscriptions {
		names = append(names, name)
	}
	return names
}

// GetActiveChannelCount returns the number of streamed channels.
func (m *Manager) GetActiveChannelCount() int {
	m.subsMu.RLock()
	defer m.subsMu.RUnlock()
	return len(m.subscriptions)
}

// ConnectedCount returns how many streamed channels have their SSE response open.
func (m *Manager) ConnectedCount() int {
	m.subsMu.RLock()
	defer m.subsMu.RUnlock()
	n := 0
	for name := range m.subscriptions {
		if m.client.IsConnected(name) {
			n++
		}
	}
	return n
}
