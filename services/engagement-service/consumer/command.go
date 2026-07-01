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

// Package consumer holds the engagement service's Redis consumers: a durable
// command stream (chat votes/wagers) and a best-effort earning Pub/Sub.
package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/caesar/all-chat/services/engagement-service/publisher"
	"github.com/caesar/all-chat/services/engagement-service/repository"
	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const consumerGroup = "engagement"

// CommandConsumer drains engagement:commands, resolves the durable viewer, and
// applies votes/wagers to the overlay's live poll/prediction. Confirmations are
// intentionally silent on the platform (send is opt-in); feedback reaches the
// viewer via the broadcast tally and the web page's pull endpoint.
type CommandConsumer struct {
	rdb          *redis.Client
	repo         *repository.Repository
	pub          *publisher.Publisher
	log          *zap.Logger
	consumerName string
}

// NewCommandConsumer creates a CommandConsumer. consumerName should be unique per
// pod (e.g. hostname) so pending-entry ownership is per-instance.
func NewCommandConsumer(rdb *redis.Client, repo *repository.Repository, pub *publisher.Publisher, consumerName string, log *zap.Logger) *CommandConsumer {
	return &CommandConsumer{rdb: rdb, repo: repo, pub: pub, log: log, consumerName: consumerName}
}

// Run blocks consuming the command stream until ctx is cancelled.
func (c *CommandConsumer) Run(ctx context.Context) {
	// Create the group at the stream tail (MKSTREAM); ignore BUSYGROUP.
	if err := c.rdb.XGroupCreateMkStream(ctx, mpmodels.StreamEngagementCommands, consumerGroup, "$").Err(); err != nil &&
		!strings.Contains(err.Error(), "BUSYGROUP") {
		c.log.Warn("create engagement command group", zap.Error(err))
	}
	c.log.Info("engagement command consumer started", zap.String("consumer", c.consumerName))

	for {
		if ctx.Err() != nil {
			return
		}
		res, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    consumerGroup,
			Consumer: c.consumerName,
			Streams:  []string{mpmodels.StreamEngagementCommands, ">"},
			Count:    64,
			Block:    5000, // ms
		}).Result()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, redis.Nil) {
				continue
			}
			c.log.Warn("xreadgroup engagement commands", zap.Error(err))
			continue
		}
		for _, stream := range res {
			for _, msg := range stream.Messages {
				c.handle(ctx, msg)
				if err := c.rdb.XAck(ctx, mpmodels.StreamEngagementCommands, consumerGroup, msg.ID).Err(); err != nil {
					c.log.Warn("xack engagement command", zap.String("id", msg.ID), zap.Error(err))
				}
			}
		}
	}
}

func (c *CommandConsumer) handle(ctx context.Context, msg redis.XMessage) {
	raw, ok := msg.Values[mpmodels.FieldEngagementData].(string)
	if !ok {
		return
	}
	var job mpmodels.CommandJob
	if err := json.Unmarshal([]byte(raw), &job); err != nil {
		c.log.Warn("unmarshal command job", zap.Error(err))
		return
	}

	kind, idx, amount, ok := parseCommand(job.Text)
	if !ok {
		return
	}

	overlays, err := c.repo.OverlaysForChannel(ctx, job.Platform, job.ChannelID)
	if err != nil || len(overlays) == 0 {
		return
	}
	viewerID, err := c.repo.GetOrCreateViewerByPlatform(ctx, job.Platform, job.UserID)
	if err != nil {
		c.log.Warn("resolve viewer for command", zap.Error(err))
		return
	}
	srcMsgID := parseUUIDOrNil(job.MessageID)

	for _, overlayID := range overlays {
		switch kind {
		case cmdVote:
			poll, err := c.repo.GetActivePoll(ctx, overlayID)
			if err != nil {
				continue // no active poll on this overlay
			}
			accepted, err := c.repo.RecordVote(ctx, poll.ID, viewerID, idx, job.Platform, srcMsgID)
			if err != nil {
				c.log.Warn("record chat vote", zap.Error(err))
				continue
			}
			if accepted {
				if updated, err := c.repo.GetPoll(ctx, poll.ID); err == nil {
					c.pub.PublishPoll(ctx, updated)
				}
			}
		case cmdWager:
			pred, err := c.repo.GetActivePrediction(ctx, overlayID)
			if err != nil {
				continue
			}
			res, err := c.repo.Wager(ctx, pred.ID, viewerID, overlayID, idx, amount, job.Platform, srcMsgID)
			if err != nil {
				c.log.Warn("record chat wager", zap.Error(err))
				continue
			}
			if res.Accepted {
				if updated, err := c.repo.GetPrediction(ctx, pred.ID); err == nil {
					c.pub.PublishPrediction(ctx, updated)
				}
			}
		}
	}
}

type cmdKind int

const (
	cmdNone cmdKind = iota
	cmdVote
	cmdWager
)

// parseCommand extracts a vote or wager from a chat message. Grammar:
//   - "!vote N" / "!v N"                → vote for option N
//   - "!predict N amount" / "!bet N …"  → wager `amount` on outcome N
//   - a bare single integer "N"         → vote for option N (low-friction shortcut)
//
// The bare-number shortcut only fires when the whole trimmed message is one small
// integer, so ordinary chat that merely contains a number is not a vote. The
// consumer additionally requires an active poll before treating it as one.
func parseCommand(text string) (kind cmdKind, idx int, amount int64, ok bool) {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return cmdNone, 0, 0, false
	}

	if strings.HasPrefix(fields[0], "!") {
		switch strings.ToLower(strings.TrimPrefix(fields[0], "!")) {
		case "vote", "v":
			if len(fields) >= 2 {
				if n, err := strconv.Atoi(fields[1]); err == nil && n >= 1 {
					return cmdVote, n, 0, true
				}
			}
		case "predict", "bet", "p":
			if len(fields) >= 3 {
				n, err1 := strconv.Atoi(fields[1])
				amt, err2 := strconv.ParseInt(fields[2], 10, 64)
				if err1 == nil && err2 == nil && n >= 1 && amt > 0 {
					return cmdWager, n, amt, true
				}
			}
		}
		return cmdNone, 0, 0, false
	}

	// Bare single integer → poll vote shortcut.
	if len(fields) == 1 {
		if n, err := strconv.Atoi(fields[0]); err == nil && n >= 1 && n <= 99 {
			return cmdVote, n, 0, true
		}
	}
	return cmdNone, 0, 0, false
}

// parseUUIDOrNil returns a *uuid.UUID if s is a valid UUID (e.g. a Twitch message
// id), else nil so the source_message_id replay-dedup guard is simply skipped on
// platforms whose message ids aren't UUIDs (the per-viewer PK still dedupes).
func parseUUIDOrNil(s string) *uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return nil
	}
	return &id
}
