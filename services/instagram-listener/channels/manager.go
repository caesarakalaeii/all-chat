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

// Package channels owns the per-IG-user poll loop: it resolves the IG user's
// currently-broadcast live media, polls its live comments with a comment_id
// cursor, and publishes each new comment to chat:raw. See ADR-0062 (Instagram
// live comments via HTTP polling).
package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/instagram-listener/client"
	"github.com/caesar/all-chat/services/instagram-listener/models"
	"github.com/caesar/all-chat/services/instagram-listener/publisher"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Manager reconciles the configured Instagram sources into per-IG-user pollers.
type Manager struct {
	repo           *Repository
	stateStore     *StateStore
	graph          *client.Client
	publisher      *publisher.StreamPublisher
	tokenStoreImpl TokenStore
	logger         *zap.Logger
	pollEvery      time.Duration // re-sync sources interval
	pollGap        time.Duration // live-comments poll interval per IG user
	minInterval    time.Duration // hard floor between two comment requests
	mu             sync.Mutex
	pollers        map[string]*IGPoller // ig_user_id -> poller
	cancel         context.CancelFunc
}

// NewManager wires the manager. tokenStore resolves the encrypted token for
// (user, ig user); pollEvery is the source re-sync cadence, pollGap the
// live-comments poll interval per IG user.
func NewManager(repo *Repository, stateStore *StateStore, graph *client.Client, pub *publisher.StreamPublisher, tokens TokenStore, logger *zap.Logger, pollEvery, pollGap time.Duration) *Manager {
	if pollEvery <= 0 {
		pollEvery = 30 * time.Second
	}
	if pollGap <= 0 {
		pollGap = 10 * time.Second
	}
	return &Manager{
		repo:           repo,
		stateStore:     stateStore,
		graph:          graph,
		publisher:      pub,
		tokenStoreImpl: tokens,
		logger:         logger,
		pollEvery:      pollEvery,
		pollGap:        pollGap,
		minInterval:    time.Second,
		pollers:        map[string]*IGPoller{},
	}
}

// Start runs the reconcile loop until ctx is cancelled.
func (m *Manager) Start(ctx context.Context) {
	ctx, m.cancel = context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(m.pollEvery)
		defer ticker.Stop()
		m.reconcile(ctx) // first run immediately
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.reconcile(ctx)
			}
		}
	}()
}

// Stop cancels the reconcile loop and every poller.
func (m *Manager) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.pollers {
		p.stop()
	}
	m.pollers = map[string]*IGPoller{}
}

// ActivePollers reports how many IG-user pollers are currently running.
func (m *Manager) ActivePollers() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.pollers)
}

// reconcile diff-starts/stops pollers to match the current source set.
func (m *Manager) reconcile(ctx context.Context) {
	sources, err := m.repo.GetActiveSources(ctx)
	if err != nil {
		m.logger.Error("Failed to fetch active instagram sources", zap.Error(err))
		return
	}

	want := map[string]*SourceRecord{}
	for _, s := range sources {
		if s.IGUserID == "" {
			continue
		}
		want[s.IGUserID] = s
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for igUserID, poller := range m.pollers {
		if _, ok := want[igUserID]; !ok {
			m.logger.Info("Stopping instagram poller (source removed or overlay inactive)", zap.String("ig_user_id", igUserID))
			poller.stop()
			delete(m.pollers, igUserID)
		}
	}
	for igUserID, src := range want {
		if _, ok := m.pollers[igUserID]; ok {
			continue
		}
		m.logger.Info("Starting instagram poller", zap.String("ig_user_id", igUserID), zap.String("ig_username", src.IGUsername), zap.String("overlay_id", src.OverlayID))
		poller := newIGPoller(m, src)
		m.pollers[igUserID] = poller
		go poller.run(ctx)
	}
}

// StateStore persists per-source poll state in Redis across restarts.
type StateStore struct {
	rdb *redis.Client
}

// NewStateStore builds the Redis-backed state store.
func NewStateStore(rdb *redis.Client) *StateStore { return &StateStore{rdb: rdb} }

func stateKey(sourceID string) string { return "instagram:source:state:" + sourceID }

// Get returns the stored state or nil.
func (s *StateStore) Get(ctx context.Context, sourceID string) (*PollerState, error) {
	raw, err := s.rdb.Get(ctx, stateKey(sourceID)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var st PollerState
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		return nil, err
	}
	return &st, nil
}

// Set stores the state with a 24h TTL (a stale live-media id and a comment-id
// cursor pointing at a broadcast that ended hours ago are both useless; the
// next resolve cycle would discard them anyway).
func (s *StateStore) Set(ctx context.Context, sourceID string, st *PollerState) error {
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, stateKey(sourceID), raw, 24*time.Hour).Err()
}

// Clear drops the state (source removed or broadcast ended).
func (s *StateStore) Clear(ctx context.Context, sourceID string) error {
	return s.rdb.Del(ctx, stateKey(sourceID)).Err()
}

// IGPoller polls one IG user: it resolves the live broadcast, then polls its
// live comments behind a comment_id cursor.
type IGPoller struct {
	mgr      *Manager
	src      *SourceRecord
	liveID   string
	cursor   string // last-seen comment id
	stopOnce sync.Once
	stopChan chan struct{}
}

func newIGPoller(m *Manager, src *SourceRecord) *IGPoller {
	return &IGPoller{mgr: m, src: src, stopChan: make(chan struct{})}
}

func (p *IGPoller) stop() {
	p.stopOnce.Do(func() { close(p.stopChan) })
}

// run is the poll loop for this IG user.
func (p *IGPoller) run(ctx context.Context) {
	// Restore any previous state so a restart does not replay the whole live
	// backlog more than once: the comment_id cursor survives in Redis.
	if st, err := p.mgr.stateStore.Get(ctx, p.src.SourceID); err == nil && st != nil {
		p.liveID = st.LiveMediaID
		p.cursor = st.LastCommentID
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopChan:
			return
		default:
		}

		if err := p.pollOnce(ctx); err != nil {
			p.mgr.logger.Warn("instagram poll failed",
				zap.String("ig_user_id", p.src.IGUserID),
				zap.String("live_media", p.liveID),
				zap.Error(err))
		}
		_ = p.mgr.repo.ActivateSource(ctx, p.src.IGUserID)

		select {
		case <-ctx.Done():
			return
		case <-p.stopChan:
			return
		case <-time.After(p.mgr.pollGap):
		}
	}
}

// pollOnce resolves the live broadcast (or reuses the cached one) and polls
// the new comments after the last-seen comment id.
func (p *IGPoller) pollOnce(ctx context.Context) error {
	token, err := p.mgr.tokenStore().GetToken(ctx, p.src.UserID, p.src.IGUserID)
	if err != nil {
		return &UnresolvedTokenError{IGUserID: p.src.IGUserID}
	}

	if err := p.resolveLiveMedia(ctx, token); err != nil {
		return err
	}
	if p.liveID == "" {
		// Not live. live_media returns only media being broadcast at request
		// time, so an empty set IS the offline signal — a normal state, not an
		// error: no retry-hammering beyond the regular pollGap tick.
		return nil
	}

	data, after, usage, err := p.mgr.graph.LiveCommentsWithUsage(ctx, p.src.IGUserID, token, client.LiveCommentsParams{
		CommentID: p.cursor,
		Limit:     "50",
	})
	if err != nil {
		if errors.Is(err, client.ErrUnauthorized) || errors.Is(err, client.ErrForbidden) {
			// Token expired or revoked: deactivate the source and tell the
			// frontend via a platform:status event so the streamer re-auths.
			_ = p.mgr.repo.DeactivateSource(ctx, p.src.IGUserID)
			_ = p.mgr.stateStore.Clear(ctx, p.src.SourceID)
			p.publishStatus(ctx, "offline", err.Error())
		}
		return fmt.Errorf("poll live comments: %w", err)
	}

	// The API returns comments newest-first; publish chronologically.
	messages := make([]*models.RawChatMessage, 0, len(data))
	for i := len(data) - 1; i >= 0; i-- {
		cm := data[i]
		if p.cursor != "" && cm.ID == p.cursor {
			continue
		}
		if cm.Text == "" {
			continue // attachments-only or deleted comments: nothing to show
		}
		ts, _ := time.Parse(time.RFC3339, cm.Timestamp)
		if t, err := time.Parse(time.RFC3339Nano, cm.Timestamp); err == nil {
			ts = t
		}
		userID := cm.From.ID
		username := cm.From.Username
		if username == "" && cm.User != nil && *cm.User != "" {
			username = *cm.User
		}
		msg := &models.RawChatMessage{
			MessageID:   cm.ID,
			Platform:    "instagram",
			OverlayID:   p.src.OverlayID,
			ChannelID:   p.src.IGUserID,
			StreamID:    p.liveID,
			ChannelName: p.src.IGUsername,
			UserID:      userID,
			Username:    username,
			Text:        cm.Text,
			Timestamp:   ts,
			Tags: map[string]string{
				"live_media_id": p.liveID,
			},
		}
		if cm.ParentID != "" {
			msg.Tags["parent_comment_id"] = cm.ParentID
		}
		messages = append(messages, msg)
	}

	if len(messages) > 0 {
		if err := p.mgr.publisher.PublishBatch(ctx, messages); err != nil {
			p.mgr.logger.Error("failed publishing instagram comments", zap.Int("count", len(messages)), zap.Error(err))
		}
	}
	if usage.CallCount >= 90 {
		p.mgr.logger.Warn("instagram rate limit nearly exhausted",
			zap.Int64("call_count_pct", usage.CallCount),
			zap.String("ig_user_id", p.src.IGUserID))
	}
	_ = after // the comment_id cursor supersedes paging cursors here

	// Advance the cursor to the newest comment id in the batch. The data
	// arrives newest-first, so data[0] is the newest.
	newest := p.cursor
	if len(data) > 0 {
		newest = data[0].ID
	}
	if newest != p.cursor {
		p.cursor = newest
		_ = p.mgr.stateStore.Set(ctx, p.src.SourceID, &PollerState{
			LiveMediaID:   p.liveID,
			LastCommentID: p.cursor,
			UpdatedAt:     time.Now(),
		})
	}
	return nil
}

// resolveLiveMedia pins the IG user's currently-broadcast live media; the
// cached id is kept until live_media no longer lists it.
func (p *IGPoller) resolveLiveMedia(ctx context.Context, token string) error {
	media, err := p.mgr.graph.LiveMedia(ctx, p.src.IGUserID, token)
	if err != nil {
		if errors.Is(err, client.ErrUnauthorized) || errors.Is(err, client.ErrForbidden) {
			_ = p.mgr.repo.DeactivateSource(ctx, p.src.IGUserID)
			_ = p.mgr.stateStore.Clear(ctx, p.src.SourceID)
			p.publishStatus(ctx, "offline", err.Error())
		}
		return fmt.Errorf("resolve live media: %w", err)
	}
	for _, m := range media {
		if m.IsLiveBroadcast() {
			if m.ID != p.liveID {
				p.mgr.logger.Info("instagram live media changed",
					zap.String("ig_user_id", p.src.IGUserID),
					zap.String("old", p.liveID),
					zap.String("new", m.ID))
				p.liveID = m.ID
				p.cursor = "" // full first fetch of the new broadcast
				p.publishStatus(ctx, "connected", "")
			}
			return nil
		}
	}
	// Not live.
	if p.liveID != "" {
		p.mgr.logger.Info("instagram live media ended", zap.String("ig_user_id", p.src.IGUserID), zap.String("live_media", p.liveID))
		p.liveID = ""
		p.cursor = ""
		_ = p.mgr.stateStore.Clear(ctx, p.src.SourceID)
		p.publishStatus(ctx, "offline", "broadcast ended")
	}
	return nil
}

// publishStatus emits a platform:status event, mirroring the other listeners'
// status publisher (ADR-0032 consumers).
func (p *IGPoller) publishStatus(ctx context.Context, status, errMsg string) {
	msg := models.StatusMessage{
		Platform:     "instagram",
		ChannelID:    p.src.IGUserID,
		ChannelName:  p.src.IGUsername,
		Status:       status,
		ErrorMessage: errMsg,
	}
	if err := p.mgr.publisher.PublishStatus(ctx, msg); err != nil {
		p.mgr.logger.Warn("failed to publish platform status",
			zap.String("ig_user_id", p.src.IGUserID), zap.Error(err))
	}
}

func (m *Manager) tokenStore() TokenStore { return m.tokenStoreImpl }
