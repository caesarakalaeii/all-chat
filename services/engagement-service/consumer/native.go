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
	"errors"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/engagement-service/models"
	"github.com/caesar/all-chat/services/engagement-service/publisher"
	"github.com/caesar/all-chat/services/engagement-service/repository"
	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const nativeConsumerGroup = "engagement-native"

// NativeConsumer mirrors Twitch-native poll/prediction lifecycle events (issue
// #523, task H) from the durable engagement:twitch-native stream. For each event
// it upserts a source='twitch_native' row per overlay sourcing the channel —
// state + aggregate tallies only; mirrored rounds run on Twitch channel points
// and never touch All-Chat viewer points. It does NOT flag channels active (the
// hot path forwards chat commands, and native votes/wagers happen on Twitch), so
// mirrored rounds never pull All-Chat votes.
type NativeConsumer struct {
	rdb          *redis.Client
	repo         *repository.Repository
	pub          *publisher.Publisher
	log          *zap.Logger
	consumerName string
}

// NewNativeConsumer creates a NativeConsumer. consumerName should be unique per
// pod so pending-entry ownership is per-instance.
func NewNativeConsumer(rdb *redis.Client, repo *repository.Repository, pub *publisher.Publisher, consumerName string, log *zap.Logger) *NativeConsumer {
	return &NativeConsumer{rdb: rdb, repo: repo, pub: pub, log: log, consumerName: consumerName}
}

// Run blocks consuming the native-mirror stream until ctx is cancelled.
func (c *NativeConsumer) Run(ctx context.Context) {
	if err := c.rdb.XGroupCreateMkStream(ctx, mpmodels.StreamEngagementTwitchNative, nativeConsumerGroup, "$").Err(); err != nil &&
		!strings.Contains(err.Error(), "BUSYGROUP") {
		c.log.Warn("create native engagement group", zap.Error(err))
	}
	c.log.Info("engagement native consumer started", zap.String("consumer", c.consumerName))

	for {
		if ctx.Err() != nil {
			return
		}
		res, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    nativeConsumerGroup,
			Consumer: c.consumerName,
			Streams:  []string{mpmodels.StreamEngagementTwitchNative, ">"},
			Count:    32,
			Block:    5000, // ms
		}).Result()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, redis.Nil) {
				continue
			}
			c.log.Warn("xreadgroup native engagement", zap.Error(err))
			continue
		}
		for _, stream := range res {
			for _, msg := range stream.Messages {
				c.handle(ctx, msg)
				if err := c.rdb.XAck(ctx, mpmodels.StreamEngagementTwitchNative, nativeConsumerGroup, msg.ID).Err(); err != nil {
					c.log.Warn("xack native engagement", zap.String("id", msg.ID), zap.Error(err))
				}
			}
		}
	}
}

func (c *NativeConsumer) handle(ctx context.Context, msg redis.XMessage) {
	raw, ok := msg.Values[mpmodels.FieldEngagementData].(string)
	if !ok {
		return
	}
	var ev mpmodels.NativeEngagementEvent
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		c.log.Warn("unmarshal native engagement event", zap.Error(err))
		return
	}
	if ev.ExternalID == "" || ev.ChannelID == "" {
		return
	}

	overlays, err := c.repo.OverlaysForChannel(ctx, ev.Platform, ev.ChannelID)
	if err != nil {
		c.log.Warn("overlays for native channel", zap.Error(err))
		return
	}
	for _, overlayID := range overlays {
		switch ev.Kind {
		case mpmodels.NativeKindPoll:
			c.mirrorPoll(ctx, overlayID, ev)
		case mpmodels.NativeKindPrediction:
			c.mirrorPrediction(ctx, overlayID, ev)
		}
	}
}

// nativePollState maps a Twitch poll lifecycle phase to our two-state poll
// machine: begin/progress are ACTIVE, end is CLOSED.
func nativePollState(phase string) string {
	if phase == mpmodels.NativeEventEnd {
		return models.PollClosed
	}
	return models.PollActive
}

// nativePredictionState maps a Twitch prediction lifecycle phase (and, for end,
// its status) to our state machine. lock → LOCKED; end → RESOLVED unless the
// status marks a cancellation; begin/progress → ACTIVE. An unknown end status is
// treated as RESOLVED rather than leaving the round stuck live on the overlay.
func nativePredictionState(phase, status string) string {
	switch phase {
	case mpmodels.NativeEventLock:
		return models.PredLocked
	case mpmodels.NativeEventEnd:
		if strings.EqualFold(status, "canceled") || strings.EqualFold(status, "cancelled") {
			return models.PredCanceled
		}
		return models.PredResolved
	default:
		return models.PredActive
	}
}

func (c *NativeConsumer) mirrorPoll(ctx context.Context, overlayID uuid.UUID, ev mpmodels.NativeEngagementEvent) {
	state := nativePollState(ev.Event)
	var closedAt *time.Time
	if state == models.PollClosed {
		t := ev.Timestamp
		closedAt = &t
	}
	outcomes := make([]repository.NativeOutcomeInput, 0, len(ev.Outcomes))
	for _, o := range ev.Outcomes {
		outcomes = append(outcomes, repository.NativeOutcomeInput{
			ExternalID: o.ExternalID, Idx: o.Idx, Label: o.Label, Votes: o.Votes,
		})
	}
	poll, err := c.repo.UpsertNativePoll(ctx, overlayID, ev.ExternalID, ev.Title, state, outcomes, ev.EndsAt, closedAt)
	if err != nil {
		c.log.Warn("mirror native poll", zap.String("external_id", ev.ExternalID), zap.Error(err))
		return
	}
	if poll == nil {
		return // stale/out-of-order event was ignored — nothing new to broadcast
	}
	c.pub.PublishPoll(ctx, poll)
}

func (c *NativeConsumer) mirrorPrediction(ctx context.Context, overlayID uuid.UUID, ev mpmodels.NativeEngagementEvent) {
	state := nativePredictionState(ev.Event, ev.Status)
	var lockedAt, resolvedAt *time.Time
	switch state {
	case models.PredLocked:
		t := ev.Timestamp
		lockedAt = &t
	case models.PredResolved, models.PredCanceled:
		t := ev.Timestamp
		resolvedAt = &t
	}
	outcomes := make([]repository.NativeOutcomeInput, 0, len(ev.Outcomes))
	for _, o := range ev.Outcomes {
		outcomes = append(outcomes, repository.NativeOutcomeInput{
			ExternalID: o.ExternalID, Idx: o.Idx, Label: o.Label, Color: o.Color,
			Points: o.Points, Entrants: o.Users,
		})
	}
	pred, err := c.repo.UpsertNativePrediction(ctx, overlayID, ev.ExternalID, ev.Title, state,
		ev.WinningExternalID, outcomes, ev.LocksAt, lockedAt, resolvedAt)
	if err != nil {
		c.log.Warn("mirror native prediction", zap.String("external_id", ev.ExternalID), zap.Error(err))
		return
	}
	if pred == nil {
		return // stale/out-of-order event was ignored — nothing new to broadcast
	}
	c.pub.PublishPrediction(ctx, pred)
}
