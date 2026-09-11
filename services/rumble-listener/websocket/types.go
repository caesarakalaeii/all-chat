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

package websocket

import "encoding/json"

// SSEStreamEvent is one parsed `data:` payload from the Rumble chat SSE stream.
// Type is either "init" (history snapshot + chat config, sent once on connect)
// or "messages" (live batch). Unknown event types are logged and dropped by the
// parser, never propagated.
type SSEStreamEvent struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// SSEInitData is the payload of the "init" event.
type SSEInitData struct {
	Chat     SSEChatInfo   `json:"chat"`
	Messages []SSEMessage  `json:"messages,omitempty"`
	Users    []SSEUser     `json:"users,omitempty"`
	Channels []SSEChannel  `json:"channels,omitempty"`
	Config   SSEChatConfig `json:"config,omitempty"`
}

// SSEChatInfo identifies the chat room the stream belongs to.
type SSEChatInfo struct {
	ID string `json:"id"`
}

// SSEChatConfig carries the chat badge catalog from the init event. Badge
// slugs seen on users ("verified", "admin", "recurring_subscription") map to
// display labels and icon sets.
type SSEChatConfig struct {
	Badges map[string]SSEBadgeDef `json:"badges,omitempty"`
}

// SSEBadgeDef describes one badge from the chat config catalog.
type SSEBadgeDef struct {
	Label SSEBadgeLabel     `json:"label,omitempty"`
	Icons map[string]string `json:"icons,omitempty"`
}

// SSEBadgeLabel carries the localized display text of a badge.
type SSEBadgeLabel struct {
	EN string `json:"en,omitempty"`
}

// SSEMessagesData is the payload of both the "init" (history) and "messages"
// (live) events: a batch of messages plus the users and channels they reference.
type SSEMessagesData struct {
	Messages []SSEMessage `json:"messages,omitempty"`
	Users    []SSEUser    `json:"users,omitempty"`
	Channels []SSEChannel `json:"channels,omitempty"`
}

// SSEMessage is one chat message. Text is pre-joined by the sender's client;
// Blocks carries the same content as structured text blocks and is used as a
// fallback when Text is empty. Rant (donation) messages carry a non-nil Rant
// with price and display duration.
type SSEMessage struct {
	ID        string          `json:"id"`
	Time      string          `json:"time,omitempty"` // RFC3339 with offset
	UserID    string          `json:"user_id,omitempty"`
	ChannelID string          `json:"channel_id,omitempty"`
	Text      string          `json:"text,omitempty"`
	Blocks    json.RawMessage `json:"blocks,omitempty"`
	Type      string          `json:"type,omitempty"` // "regular", "crypto_donation", ...
	Rant      *SSERant        `json:"rant,omitempty"`
	IsDeleted bool            `json:"is_deleted,omitempty"`
}

// SSERant describes a donation ("rant") attached to a message.
type SSERant struct {
	PriceCents int `json:"price_cents,omitempty"`
	Duration   int `json:"duration"` // seconds the rant stays pinned
}

// SSEUser is a chatter profile as delivered by the users array. Color is a
// rumble-assigned per-user colour; badges are opaque slugs resolved via the
// config catalog. Image uses the literal JSON key "image.1" as observed on
// the wire.
type SSEUser struct {
	ID         string   `json:"id"`
	Username   string   `json:"username"`
	Link       string   `json:"link,omitempty"`
	Image      string   `json:"image.1,omitempty"`
	IsFollower bool     `json:"is_follower,omitempty"`
	Color      string   `json:"color,omitempty"`
	Badges     []string `json:"badges,omitempty"`
}

// SSEChannel is a channel identity a message may be sent as, referenced from
// a message by channel_id.
type SSEChannel struct {
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	Username string `json:"username,omitempty"`
}
