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
	"strings"
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
// live on that channel. Both inputs are lowercased so the writer (which keys with
// the DB-stored channel casing) and the reader (which keys with the
// listener-lowercased casing) always agree on the same key.
func EngagementActiveKey(platform, channelID string) string {
	return fmt.Sprintf("engagement:active:%s:%s", strings.ToLower(platform), strings.ToLower(channelID))
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

// StreamEngagementTwitchNative is the durable Redis Stream carrying normalized
// Twitch-native poll/prediction lifecycle events (channel.poll.* /
// channel.prediction.*), produced by twitch-eventsub-listener and consumed by
// engagement-service, which mirrors them per overlay — state + aggregate tallies
// only; native engagements never touch All-Chat points. Durable like the command
// stream because a missed lock/end event would strand a mirrored round in a live
// state on the overlay (there is no later event to self-heal from).
const StreamEngagementTwitchNative = "engagement:twitch-native"

// NativeEngagementEvent kinds and lifecycle phases.
const (
	NativeKindPoll       = "poll"
	NativeKindPrediction = "prediction"

	NativeEventBegin    = "begin"
	NativeEventProgress = "progress"
	NativeEventLock     = "lock"
	NativeEventEnd      = "end"
)

// NativeOutcome is one Twitch poll choice or prediction outcome with its
// aggregate tally (Twitch owns the individual votes/wagers, so only totals
// cross this boundary).
type NativeOutcome struct {
	ExternalID string `json:"external_id"` // Twitch choice/outcome id
	Idx        int    `json:"idx"`         // 1-based, stable Twitch array order
	Label      string `json:"label"`
	Color      string `json:"color,omitempty"` // predictions: pink/blue
	Votes      int64  `json:"votes"`           // polls: total votes
	Points     int64  `json:"points"`          // predictions: channel points wagered
	Users      int64  `json:"users"`           // predictions: entrants
}

// NativeEngagementEvent is one normalized lifecycle event of a Twitch-native
// poll or prediction. ChannelID is the LOWERCASE broadcaster login — the same
// identifier overlay_chat_sources.channel_id stores for twitch sources, so the
// consumer can fan out to overlays without a Helix id→login lookup.
type NativeEngagementEvent struct {
	Kind              string          `json:"kind"`  // poll | prediction
	Event             string          `json:"event"` // begin | progress | lock | end
	Platform          string          `json:"platform"`
	ChannelID         string          `json:"channel_id"`
	ExternalID        string          `json:"external_id"` // Twitch poll/prediction id
	Title             string          `json:"title"`
	Outcomes          []NativeOutcome `json:"outcomes"`
	Status            string          `json:"status,omitempty"` // end only: completed|archived|terminated / resolved|canceled
	WinningExternalID string          `json:"winning_external_id,omitempty"`
	EndsAt            *time.Time      `json:"ends_at,omitempty"`  // polls
	LocksAt           *time.Time      `json:"locks_at,omitempty"` // predictions
	Timestamp         time.Time       `json:"timestamp"`
}
