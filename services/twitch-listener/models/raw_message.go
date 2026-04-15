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
	"encoding/json"
	"time"
)

// RawChatMessage represents a raw chat message from Twitch IRC
// This is published to Redis Streams for processing
//
// Shared Chat Tags (when message is from collaborative stream):
//   - source-room-id: Room ID where message originated
//   - source-id: User ID of original sender
//   - source-badges: User badges in source room (format: "badge1/version1,badge2/version2")
//   - source-badge-info: Additional metadata for source badges
type RawChatMessage struct {
	MessageID string            `json:"message_id"` // UUID
	Platform  string            `json:"platform"`   // "twitch"
	ChannelID string            `json:"channel_id"` // Twitch channel name (lowercase)
	UserID    string            `json:"user_id"`    // Twitch user ID
	Username  string            `json:"username"`   // Username (lowercase)
	Text      string            `json:"text"`       // Raw message text
	Timestamp time.Time         `json:"timestamp"`  // UTC timestamp
	Tags      map[string]string `json:"tags"`       // IRC tags (badges, color, emotes, etc.)

	// Event support (backwards compatible - omitted for regular chat messages)
	EventType string                 `json:"event_type,omitempty"` // "subscription", "raid", "bits", etc.
	EventData map[string]interface{} `json:"event_data,omitempty"` // Event-specific payload
}

// ToJSON converts the message to JSON bytes
func (m *RawChatMessage) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// FromJSON parses JSON bytes into a RawChatMessage
func FromJSON(data []byte) (*RawChatMessage, error) {
	var msg RawChatMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// ChannelSource represents an active Twitch channel to monitor
type ChannelSource struct {
	OverlayID string // UUID of the overlay
	ChannelID string // Twitch channel name (lowercase)
}
