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

// Package channels owns the per-Page poll loop: it resolves the Page's current
// live video, polls its comments with a `since` cursor, and publishes each new
// comment to chat:raw. See ADR-0060 (Graph API via HTTP polling).
package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/facebook-listener/client"
	"github.com/caesar/all-chat/services/facebook-listener/models"
	"github.com/caesar/all-chat/services/facebook-listener/publisher"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Manager reconciles the configured Facebook sources into per-Page pollers.
type Manager struct {
	repo           *Repository
	stateStore     *StateStore
	graph          *client.Client
	publisher      *publisher.StreamPublisher
	tokenStoreImpl TokenStore
	logger         *zap.Logger
	pollEvery      time.Duration // re-sync sources interval
	pollGap        time.Duration // comments poll interval per page
	minInterval    time.Duration // hard floor between two comment requests
	mu             sync.Mutex
	pollers        map[string]*PagePoller // page_id -> poller
	cancel         context.CancelFunc
}

// NewManager wires the manager. tokenStore resolves the encrypted Page token
// for (user, page); pollEvery is the source re-sync cadence, pollGap the
// comments poll interval per Page.
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
		pollers:        map[string]*PagePoller{},
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
	m.pollers = map[string]*PagePoller{}
}

// ActivePollers reports how many Page pollers are currently running.
func (m *Manager) ActivePollers() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.pollers)
}

// reconcile diff-starts/stops pollers to match the current source set.
func (m *Manager) reconcile(ctx context.Context) {
	sources, err := m.repo.GetActiveSources(ctx)
	if err != nil {
		m.logger.Error("Failed to fetch active facebook sources", zap.Error(err))
		return
	}

	want := map[string]*SourceRecord{}
	for _, s := range sources {
		if s.PageID == "" {
			continue
		}
		want[s.PageID] = s
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for pageID, poller := range m.pollers {
		if _, ok := want[pageID]; !ok {
			m.logger.Info("Stopping facebook poller (source removed or overlay inactive)", zap.String("page_id", pageID))
			poller.stop()
			delete(m.pollers, pageID)
		}
	}
	for pageID, src := range want {
		if _, ok := m.pollers[pageID]; ok {
			continue
		}
		m.logger.Info("Starting facebook poller", zap.String("page_id", pageID), zap.String("page_name", src.PageName), zap.String("overlay_id", src.OverlayID))
		poller := newPagePoller(m, src)
		m.pollers[pageID] = poller
		go poller.run(ctx)
	}
}

// StateStore persists per-source poll state in Redis across restarts.
type StateStore struct {
	rdb *redis.Client
}

// NewStateStore builds the Redis-backed state store.
func NewStateStore(rdb *redis.Client) *StateStore { return &StateStore{rdb: rdb} }

func stateKey(sourceID string) string { return "facebook:source:state:" + sourceID }

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

// Set stores the state with a 24h TTL (a stale live video id after a day is
// useless; the next resolve cycle would discard it anyway).
func (s *StateStore) Set(ctx context.Context, sourceID string, st *PollerState) error {
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, stateKey(sourceID), raw, 24*time.Hour).Err()
}

// Clear drops the state (source removed or video ended).
func (s *StateStore) Clear(ctx context.Context, sourceID string) error {
	return s.rdb.Del(ctx, stateKey(sourceID)).Err()
}

// PagePoller polls one Page: it resolves the live video, then polls comments.
type PagePoller struct {
	mgr      *Manager
	src      *SourceRecord
	liveID   string
	since    string
	stopOnce sync.Once
	stopChan chan struct{}
}

func newPagePoller(m *Manager, src *SourceRecord) *PagePoller {
	return &PagePoller{mgr: m, src: src, stopChan: make(chan struct{})}
}

func (p *PagePoller) stop() {
	p.stopOnce.Do(func() { close(p.stopChan) })
}

// run is the poll loop for this Page.
func (p *PagePoller) run(ctx context.Context) {
	// Restore any previous state so a restart does not replay old comments.
	if st, err := p.mgr.stateStore.Get(ctx, p.src.SourceID); err == nil && st != nil {
		p.liveID = st.LiveVideoID
		p.since = st.LastSince
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
			p.mgr.logger.Warn("facebook poll failed",
				zap.String("page_id", p.src.PageID),
				zap.String("live_video", p.liveID),
				zap.Error(err))
		}
		_ = p.mgr.repo.ActivateSource(ctx, p.src.PageID)

		select {
		case <-ctx.Done():
			return
		case <-p.stopChan:
			return
		case <-time.After(p.mgr.pollGap):
		}
	}
}

// pollOnce resolves the live video (or reuses the cached one) and polls the
// new comments since the last-seen created_time.
func (p *PagePoller) pollOnce(ctx context.Context) error {
	token, err := p.mgr.tokenStore().GetPageToken(ctx, p.src.UserID, p.src.PageID)
	if err != nil {
		return &UnresolvedTokenError{PageID: p.src.PageID}
	}

	if err := p.resolveLiveVideo(ctx, token); err != nil {
		return err
	}
	if p.liveID == "" {
		return nil // Page not live; nothing to poll
	}

	data, after, usage, err := p.mgr.graph.CommentsWithUsage(ctx, p.liveID, token, client.CommentsParams{
		Order: "reverse_chronological",
		Since: p.since,
		Limit: "100",
	})
	if err != nil {
		return fmt.Errorf("poll comments: %w", err)
	}

	newest := p.since
	messages := make([]*models.RawChatMessage, 0, len(data))
	for i := len(data) - 1; i >= 0; i-- { // chronological publish
		c := data[i]
		if p.since != "" && c.CreatedTime <= p.since {
			continue
		}
		if c.Message == "" {
			continue // attachments-only or deleted comments: nothing to show
		}
		ts, _ := time.Parse(time.RFC3339, c.CreatedTime)
		if t, err := time.Parse(time.RFC3339Nano, c.CreatedTime); err == nil {
			ts = t
		}
		streamID := p.liveID
		parentID := c.Parent.ID
		msg := &models.RawChatMessage{
			MessageID:   c.ID,
			Platform:    "facebook",
			OverlayID:   p.src.OverlayID,
			ChannelID:   p.src.PageID,
			StreamID:    streamID,
			ChannelName: p.src.PageName,
			UserID:      c.From.ID,
			Username:    c.From.Name,
			Text:        c.Message,
			Timestamp:   ts,
			Tags: map[string]string{
				"live_video_id": streamID,
			},
		}
		if parentID != "" {
			msg.Tags["parent_comment_id"] = parentID
		}
		messages = append(messages, msg)
		if c.CreatedTime > newest {
			newest = c.CreatedTime
		}
	}

	if len(messages) > 0 {
		if err := p.mgr.publisher.PublishBatch(ctx, messages); err != nil {
			p.mgr.logger.Error("failed publishing facebook comments", zap.Int("count", len(messages)), zap.Error(err))
		}
	}
	if usage.CallCount >= 90 {
		p.mgr.logger.Warn("facebook page rate limit nearly exhausted",
			zap.Int64("call_count_pct", usage.CallCount),
			zap.String("page_id", p.src.PageID))
	}
	_ = after // created_time cursor supersedes paging cursors here

	if newest != p.since {
		p.since = newest
		_ = p.mgr.stateStore.Set(ctx, p.src.SourceID, &PollerState{
			LiveVideoID: p.liveID,
			LastSince:   p.since,
			UpdatedAt:   time.Now(),
		})
	}
	return nil
}

// resolveLiveVideo pins the Page's currently-live video; the cached id is kept
// until Graph no longer lists it as LIVE_NOW.
func (p *PagePoller) resolveLiveVideo(ctx context.Context, token string) error {
	videos, err := p.mgr.graph.LiveVideos(ctx, p.src.PageID, token)
	if err != nil {
		return fmt.Errorf("resolve live video: %w", err)
	}
	for _, v := range videos {
		if v.Status == client.LiveStatusLiveNow {
			if v.ID != p.liveID {
				p.mgr.logger.Info("facebook live video changed",
					zap.String("page_id", p.src.PageID),
					zap.String("old", p.liveID),
					zap.String("new", v.ID))
				p.liveID = v.ID
				if p.src.StreamSince != "" {
					p.since = p.src.StreamSince
				} else {
					p.since = "" // full first fetch of the new broadcast
				}
			}
			return nil
		}
	}
	// Not live.
	if p.liveID != "" {
		p.mgr.logger.Info("facebook live video ended", zap.String("page_id", p.src.PageID), zap.String("live_video", p.liveID))
		p.liveID = ""
		_ = p.mgr.stateStore.Clear(ctx, p.src.SourceID)
	}
	return nil
}

func (m *Manager) tokenStore() TokenStore { return m.tokenStoreImpl }
