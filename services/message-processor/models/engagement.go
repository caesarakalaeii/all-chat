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

package models

import (
	"fmt"
	"time"
)

// Engagement transport contract (issue #523). Defined in the message-processor
// module because it is the PRODUCER of these signals; engagement-service imports
// this module, so both sides share one definition and can't drift.
const (
	// StreamEngagementCommands is the Redis Stream the hot path XADDs candidate
	// vote/wager chat commands to. engagement-service consumes it via a durable
	// consumer group (votes must survive a restart), unlike the best-effort
	// earning Pub/Sub below.
	StreamEngagementCommands = "engagement:commands"

	// ChannelEngagementEvents is the Pub/Sub channel the message-processor
	// republishes event-bearing unified messages (subs/bits/donations/gifts) to,
	// for the points earning engine. Payload is a raw UnifiedChatMessage JSON.
	// Best-effort (Pub/Sub): a missed event just means an unpaid earn, never a
	// corrupted balance — unlike the durable command stream above.
	ChannelEngagementEvents = "engagement:events"

	// ChannelEngagementChat carries throttled chat-activity signals (one per
	// active minute per chatter per channel) for chat-participation points.
	// Payload is a ChatActivity JSON.
	ChannelEngagementChat = "engagement:chat"

	// FieldEngagementData is the Stream field name carrying the JSON CommandJob.
	FieldEngagementData = "data"
)

// ChatActivity is a throttled "this viewer chatted this minute" signal. The hot
// path publishes at most one per (platform, channel, user, minute) via a Redis
// SET NX EX 60 gate, so volume is bounded to distinct active chatters per minute.
type ChatActivity struct {
	Platform  string `json:"platform"`
	ChannelID string `json:"channel_id"`
	UserID    string `json:"user_id"`
	Bucket    int64  `json:"bucket"` // unix-minute bucket, for per-minute dedup
}

// EngagementActiveKey is the Redis key the hot path checks (EXISTS) to decide
// whether a chat message on (platform, channelID) could be an engagement command.
// engagement-service maintains it as a refcounted SET while a poll/prediction is
// live on that channel.
func EngagementActiveKey(platform, channelID string) string {
	return fmt.Sprintf("engagement:active:%s:%s", platform, channelID)
}

// CommandJob is a candidate engagement chat command forwarded by the hot path.
// engagement-service resolves the durable viewer id from (Platform, UserID) and
// parses the grammar (!vote N / !predict N amount / bare number) off the hot path.
type CommandJob struct {
	MessageID string    `json:"message_id"`
	Platform  string    `json:"platform"`
	ChannelID string    `json:"channel_id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
}
