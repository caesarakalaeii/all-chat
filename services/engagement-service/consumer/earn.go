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

package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/caesar/all-chat/services/engagement-service/engine"
	"github.com/caesar/all-chat/services/engagement-service/repository"
	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// EarnConsumer awards points from the best-effort earning feeds: platform events
// (subs/bits/donations/gifts) on engagement:events and throttled chat-activity on
// engagement:chat. Every award is idempotent via the ledger dedup_key, so the
// Pub/Sub fan-out to multiple engagement replicas credits each award exactly once.
type EarnConsumer struct {
	rdb  *redis.Client
	repo *repository.Repository
	log  *zap.Logger
}

// NewEarnConsumer creates an EarnConsumer.
func NewEarnConsumer(rdb *redis.Client, repo *repository.Repository, log *zap.Logger) *EarnConsumer {
	return &EarnConsumer{rdb: rdb, repo: repo, log: log}
}

// Run subscribes to the earning channels until ctx is cancelled.
func (e *EarnConsumer) Run(ctx context.Context) {
	sub := e.rdb.Subscribe(ctx, mpmodels.ChannelEngagementEvents, mpmodels.ChannelEngagementChat)
	defer sub.Close()
	e.log.Info("engagement earn consumer started")
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			switch msg.Channel {
			case mpmodels.ChannelEngagementEvents:
				e.safely("earn event", func() { e.handleEvent(ctx, []byte(msg.Payload)) })
			case mpmodels.ChannelEngagementChat:
				e.safely("earn chat", func() { e.handleChat(ctx, []byte(msg.Payload)) })
			}
		}
	}
}

// safely runs fn under a recover so one poison payload can't kill the Run goroutine
// and silently stop earning for the pod's lifetime (L-C3). Pub/Sub has no ack, so a
// recovered panic just logs — the payload is already consumed and won't redeliver.
func (e *EarnConsumer) safely(what string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			e.log.Error("panic in engagement earn handler", zap.String("handler", what), zap.Any("panic", r))
		}
	}()
	fn()
}

// handleEvent awards points for a monetary/loyalty event. The unified message is
// already scoped to one overlay (message-processor publishes one per overlay), so
// the award lands in that overlay's economy.
func (e *EarnConsumer) handleEvent(ctx context.Context, payload []byte) {
	msg, err := mpmodels.FromJSON(payload)
	if err != nil || msg.Event == nil || msg.OverlayID == "" || msg.User.ID == "" {
		return
	}
	overlayID, err := uuid.Parse(msg.OverlayID)
	if err != nil {
		return
	}
	cfg, err := e.repo.GetEarnConfig(ctx, overlayID)
	if err != nil {
		return
	}
	award, ok := engine.PointsForEvent(cfg, msg.Event)
	if !ok {
		return
	}
	viewerID, err := e.repo.GetOrCreateViewerByPlatform(ctx, msg.Platform, msg.User.ID)
	if err != nil {
		e.log.Warn("resolve viewer for earn event", zap.Error(err))
		return
	}
	// dedup per (overlay, message, reason): stable across replicas receiving the
	// same published bytes, and distinct per overlay economy.
	dedup := fmt.Sprintf("earn:%s:%s:%s", overlayID, msg.ID, award.Reason)
	if _, err := e.repo.AwardPoints(ctx, viewerID, overlayID, award.Delta, award.Reason, "event", nil, dedup); err != nil {
		e.log.Warn("award event points", zap.Error(err))
	}
}

// handleChat awards chat-participation points, resolving the overlays the channel
// feeds and crediting each economy once per minute-bucket per viewer.
func (e *EarnConsumer) handleChat(ctx context.Context, payload []byte) {
	var act mpmodels.ChatActivity
	if err := json.Unmarshal(payload, &act); err != nil || act.UserID == "" {
		return
	}
	overlays, err := e.repo.OverlaysForChannel(ctx, act.Platform, act.ChannelID)
	if err != nil || len(overlays) == 0 {
		return
	}
	viewerID, err := e.repo.GetOrCreateViewerByPlatform(ctx, act.Platform, act.UserID)
	if err != nil {
		return
	}
	for _, overlayID := range overlays {
		cfg, err := e.repo.GetEarnConfig(ctx, overlayID)
		if err != nil || !cfg.Enabled || cfg.ChatPerMinute <= 0 {
			continue
		}
		dedup := fmt.Sprintf("chat:%s:%s:%d", overlayID, viewerID, act.Bucket)
		if _, err := e.repo.AwardPoints(ctx, viewerID, overlayID, cfg.ChatPerMinute, "earn_chat", "chat", nil, dedup); err != nil {
			e.log.Warn("award chat points", zap.Error(err))
		}
	}
}
