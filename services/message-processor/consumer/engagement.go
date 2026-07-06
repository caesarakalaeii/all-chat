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
	"strconv"
	"strings"

	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// engagementCommandsMaxLen caps the engagement:commands stream so a stalled
// engagement-service can't grow it unbounded (approximate trim, cheap).
const engagementCommandsMaxLen = 10000

// forwardEngagementCommand is the hot-path hook for issue #523. For a chat message
// that *looks* like a vote/wager it hands the message off to the background forwarder
// (runEngagementForwarder) so the Redis EXISTS/XADD round-trips never block the
// consume loop (L-Perf1). Ordinary chat pays only the in-process looksLikeCommand
// check. Entirely best-effort: if the forwarder buffer is full the candidate is
// dropped and counted — a missed forward is a missed vote, never corruption.
func (c *StreamConsumer) forwardEngagementCommand(raw *models.RawChatMessage) {
	if raw.EventType != "" && raw.EventType != "chat_message" {
		return // only real chat messages carry vote/wager commands
	}
	if !looksLikeCommand(raw.Text) {
		return // fast in-process reject — no Redis call, no channel op, for ordinary chat
	}
	select {
	case c.engagementCh <- raw:
	default:
		c.metrics.RecordEngagementForward("dropped")
	}
}

// runEngagementForwarder drains queued command-shaped chat messages off the hot path
// and performs the live-round EXISTS check plus, on a hit, the XADD to the durable
// engagement:commands stream. Runs for the consumer's lifetime.
func (c *StreamConsumer) runEngagementForwarder(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case raw := <-c.engagementCh:
			c.forwardEngagementNow(ctx, raw)
		}
	}
}

// forwardEngagementNow does the actual EXISTS + XADD for one candidate. All heavy
// work (grammar parse, viewer resolution, DB writes) still happens off in
// engagement-service; this only routes the job to the durable stream.
func (c *StreamConsumer) forwardEngagementNow(ctx context.Context, raw *models.RawChatMessage) {
	key := models.EngagementActiveKey(raw.Platform, raw.ChannelID)
	exists, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		c.metrics.RecordEngagementForward("error")
		c.logger.Debug("engagement active check failed", zap.Error(err))
		return
	}
	if exists == 0 {
		c.metrics.RecordEngagementForward("miss")
		return // no live poll/prediction on this channel → nothing to forward
	}

	msgID := raw.Tags["id"] // native (Twitch) id is a stable UUID for replay dedup
	if msgID == "" {
		msgID = raw.MessageID
	}
	job := models.CommandJob{
		MessageID: msgID,
		Platform:  raw.Platform,
		ChannelID: raw.ChannelID,
		UserID:    raw.UserID,
		Username:  raw.Username,
		Text:      raw.Text,
		Timestamp: raw.Timestamp,
	}
	data, err := json.Marshal(job)
	if err != nil {
		c.metrics.RecordEngagementForward("error")
		return
	}
	if err := c.client.XAdd(ctx, &redis.XAddArgs{
		Stream: models.StreamEngagementCommands,
		MaxLen: engagementCommandsMaxLen,
		Approx: true,
		Values: map[string]interface{}{models.FieldEngagementData: string(data)},
	}).Err(); err != nil {
		c.metrics.RecordEngagementForward("error")
		c.logger.Debug("forward engagement command failed", zap.Error(err))
		return
	}
	c.metrics.RecordEngagementForward("hit")
}

// looksLikeCommand is a cheap pre-filter: true when the trimmed text starts with
// '!' (explicit command) or is a single short integer (the bare-number vote
// shortcut). This keeps the Redis EXISTS check off the path for ordinary chat.
func looksLikeCommand(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return false
	}
	if t[0] == '!' {
		return true
	}
	// Bare 1–2 digit number → possible poll-vote shortcut.
	if len(t) <= 2 {
		if _, err := strconv.Atoi(t); err == nil {
			return true
		}
	}
	return false
}
