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
	"fmt"
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

	// A missed lock/end would strand a mirrored round live on the overlay with no
	// later event to self-heal from (ADR-0030), so reclaim orphaned entries (H3).
	drainPEL(ctx, c.rdb, mpmodels.StreamEngagementTwitchNative, nativeConsumerGroup, c.consumerName, c.log, c.safeHandle)
	go periodicDrain(ctx, c.rdb, mpmodels.StreamEngagementTwitchNative, nativeConsumerGroup, c.consumerName, c.log, c.safeHandle)

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
				// Ack only on success/permanent; a transient upsert error leaves the
				// entry pending so the mirrored lifecycle event is retried (H2/H3).
				if err := c.safeHandle(ctx, msg); err != nil {
					c.log.Warn("native engagement deferred for retry", zap.String("id", msg.ID), zap.Error(err))
					continue
				}
				if err := c.rdb.XAck(ctx, mpmodels.StreamEngagementTwitchNative, nativeConsumerGroup, msg.ID).Err(); err != nil {
					c.log.Warn("xack native engagement", zap.String("id", msg.ID), zap.Error(err))
				}
			}
		}
	}
}

// safeHandle runs handle under a recover (see CommandConsumer.safeHandle). A
// recovered panic is permanent (returns nil → ack) so it can't poison the PEL.
func (c *NativeConsumer) safeHandle(ctx context.Context, msg redis.XMessage) (err error) {
	defer func() {
		if r := recover(); r != nil {
			c.log.Error("panic handling native engagement", zap.Any("panic", r), zap.String("id", msg.ID))
			err = nil
		}
	}()
	// Per-message deadline (see CommandConsumer.safeHandle) — bounds a stuck upsert so
	// it can't wedge the native consumer or the PEL drain.
	hctx, cancel := context.WithTimeout(ctx, handleTimeout)
	defer cancel()
	return c.handle(hctx, msg)
}

// handle mirrors one native lifecycle event. Returns nil on success or a
// permanently unprocessable event (bad JSON, missing ids), and a non-nil error only
// on a transient DB failure so the caller leaves it pending for retry (H2/H3).
func (c *NativeConsumer) handle(ctx context.Context, msg redis.XMessage) error {
	raw, ok := msg.Values[mpmodels.FieldEngagementData].(string)
	if !ok {
		return nil
	}
	var ev mpmodels.NativeEngagementEvent
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		c.log.Warn("unmarshal native engagement event", zap.Error(err))
		return nil // permanent
	}
	if ev.ExternalID == "" || ev.ChannelID == "" {
		return nil // malformed — nothing to mirror
	}

	overlays, err := c.repo.OverlaysForChannel(ctx, ev.Platform, ev.ChannelID)
	if err != nil {
		return fmt.Errorf("overlays for native channel: %w", err) // transient → retry
	}
	var retryErr error
	for _, overlayID := range overlays {
		switch ev.Kind {
		case mpmodels.NativeKindPoll:
			if err := c.mirrorPoll(ctx, overlayID, ev); err != nil {
				retryErr = errors.Join(retryErr, err)
			}
		case mpmodels.NativeKindPrediction:
			if err := c.mirrorPrediction(ctx, overlayID, ev); err != nil {
				retryErr = errors.Join(retryErr, err)
			}
		}
	}
	return retryErr
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

func (c *NativeConsumer) mirrorPoll(ctx context.Context, overlayID uuid.UUID, ev mpmodels.NativeEngagementEvent) error {
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
		return fmt.Errorf("mirror native poll %s: %w", ev.ExternalID, err) // transient → retry
	}
	if poll == nil {
		return nil // stale/out-of-order event was ignored — nothing new to broadcast
	}
	c.broadcastDisplayPoll(ctx, overlayID, poll)
	return nil
}

// broadcastDisplayPoll publishes the overlay's DISPLAY poll rather than the specific
// native round just upserted. The pub/sub channel is last-writer-wins, and ADR-0030
// requires a live All-Chat round to keep the wire (it holds real wagered points and
// must stay resolvable) — so broadcasting the native snapshot directly could clobber
// it on any real-time consumer. GetActiveDisplayPoll applies that precedence; when
// nothing is active (e.g. the native round just closed) we fall back to the upserted
// row so the terminal frame still propagates (M-C2).
func (c *NativeConsumer) broadcastDisplayPoll(ctx context.Context, overlayID uuid.UUID, upserted *models.Poll) {
	disp, err := c.repo.GetActiveDisplayPoll(ctx, overlayID)
	switch {
	case err == nil:
		c.pub.PublishPoll(ctx, disp)
	case errors.Is(err, repository.ErrNotFound):
		c.pub.PublishPoll(ctx, upserted)
	default:
		c.log.Warn("resolve display poll for native broadcast", zap.Error(err))
		c.pub.PublishPoll(ctx, upserted)
	}
}

func (c *NativeConsumer) mirrorPrediction(ctx context.Context, overlayID uuid.UUID, ev mpmodels.NativeEngagementEvent) error {
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
		return fmt.Errorf("mirror native prediction %s: %w", ev.ExternalID, err) // transient → retry
	}
	if pred == nil {
		return nil // stale/out-of-order event was ignored — nothing new to broadcast
	}
	c.broadcastDisplayPrediction(ctx, overlayID, pred)
	return nil
}

// broadcastDisplayPrediction publishes the overlay's DISPLAY prediction, keeping a
// live All-Chat round on the wire ahead of a mirrored Twitch one (see
// broadcastDisplayPoll / ADR-0030). Falls back to the upserted row when none is
// active so a RESOLVED/CANCELED frame still propagates (M-C2).
func (c *NativeConsumer) broadcastDisplayPrediction(ctx context.Context, overlayID uuid.UUID, upserted *models.Prediction) {
	disp, err := c.repo.GetActiveDisplayPrediction(ctx, overlayID)
	switch {
	case err == nil:
		c.pub.PublishPrediction(ctx, disp)
	case errors.Is(err, repository.ErrNotFound):
		c.pub.PublishPrediction(ctx, upserted)
	default:
		c.log.Warn("resolve display prediction for native broadcast", zap.Error(err))
		c.pub.PublishPrediction(ctx, upserted)
	}
}
