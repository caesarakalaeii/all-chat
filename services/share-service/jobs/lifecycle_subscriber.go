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

package jobs

import (
	"context"
	"encoding/json"
	"time"

	"github.com/caesar/all-chat/services/share-service/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// StreamEndEvent is the payload published to Redis "lifecycle:stream_end"
type StreamEndEvent struct {
	Platform      string    `json:"platform"`
	UserID        string    `json:"user_id"`        // all-chat user UUID (may be "" for YouTube/TikTok — resolved via google_id)
	BroadcasterID string    `json:"broadcaster_id"` // platform-specific ID
	Timestamp     time.Time `json:"timestamp"`
}

// LifecycleSubscriber subscribes to stream-end events and expires this_stream shares.
type LifecycleSubscriber struct {
	repo   *repository.ShareRepository
	redis  *redis.Client
	db     *pgxpool.Pool // for google_id → user_id lookup (YouTube)
	logger *zap.SugaredLogger
}

// NewLifecycleSubscriber creates a new lifecycle subscriber.
func NewLifecycleSubscriber(repo *repository.ShareRepository, rdb *redis.Client, db *pgxpool.Pool, logger *zap.SugaredLogger) *LifecycleSubscriber {
	return &LifecycleSubscriber{
		repo:   repo,
		redis:  rdb,
		db:     db,
		logger: logger,
	}
}

// Start subscribes to lifecycle:stream_end and processes events in a goroutine.
func (ls *LifecycleSubscriber) Start(ctx context.Context) {
	go ls.run(ctx)
}

func (ls *LifecycleSubscriber) run(ctx context.Context) {
	if ls.redis == nil {
		ls.logger.Warn("LifecycleSubscriber: redis is nil, skipping")
		return
	}

	pubsub := ls.redis.Subscribe(ctx, "lifecycle:stream_end")
	defer pubsub.Close()

	ls.logger.Info("LifecycleSubscriber started, listening on lifecycle:stream_end")

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			ls.logger.Info("LifecycleSubscriber stopping")
			return
		case msg, ok := <-ch:
			if !ok {
				ls.logger.Warn("LifecycleSubscriber channel closed")
				return
			}
			var event StreamEndEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				ls.logger.Warnw("Failed to unmarshal lifecycle event", "error", err)
				continue
			}
			// Debounce: wait 60s before expiring to avoid false positives
			// (Twitch can send stream.offline on brief restarts / category changes)
			go ls.debounceExpire(ctx, event)
		}
	}
}

// debounceExpire waits 60s then expires this_stream shares for the given user.
func (ls *LifecycleSubscriber) debounceExpire(ctx context.Context, event StreamEndEvent) {
	ls.logger.Infow("Stream end received, debouncing 60s",
		"user_id", event.UserID,
		"platform", event.Platform,
		"broadcaster_id", event.BroadcasterID)

	select {
	case <-ctx.Done():
		return
	case <-time.After(60 * time.Second):
	}

	userID := event.UserID
	if userID == "" && event.Platform == "youtube" && ls.db != nil {
		// Resolve google_id (YouTube channel_id) to all-chat user_id
		if err := ls.db.QueryRow(ctx,
			"SELECT id FROM users WHERE google_id = $1", event.BroadcasterID).Scan(&userID); err != nil {
			ls.logger.Warnw("No user found for YouTube channel",
				"channel_id", event.BroadcasterID)
			return
		}
	}

	if userID == "" {
		ls.logger.Warnw("Cannot expire shares: empty user_id and no lookup available",
			"platform", event.Platform,
			"broadcaster_id", event.BroadcasterID)
		return
	}

	ls.expireThisStreamShares(ctx, userID)
}

// expireThisStreamShares expires all this_stream accepted shares for the given user.
func (ls *LifecycleSubscriber) expireThisStreamShares(ctx context.Context, userID string) {
	if ls.repo == nil {
		return
	}

	shares, err := ls.repo.GetThisStreamShares(ctx, userID)
	if err != nil {
		ls.logger.Errorw("Failed to get this_stream shares",
			"user_id", userID,
			"error", err)
		return
	}

	for _, share := range shares {
		if err := ls.repo.ExpireAcceptedShare(ctx, share.ID); err != nil {
			ls.logger.Errorw("Failed to expire this_stream share",
				"share_id", share.ID,
				"error", err)
		} else {
			ls.logger.Infow("Expired this_stream share",
				"share_id", share.ID,
				"user_id", userID)
		}
	}
}
