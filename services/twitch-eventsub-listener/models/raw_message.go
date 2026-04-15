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

// RawChatMessage represents a raw event message from Twitch EventSub
// This matches the format used by twitch-listener for consistency
type RawChatMessage struct {
	MessageID string            `json:"message_id"` // UUID
	Platform  string            `json:"platform"`   // "twitch"
	ChannelID string            `json:"channel_id"` // Twitch channel name (lowercase)
	UserID    string            `json:"user_id"`    // Twitch user ID
	Username  string            `json:"username"`   // Username (lowercase)
	Text      string            `json:"text"`       // Event description text
	Timestamp time.Time         `json:"timestamp"`  // UTC timestamp
	Tags      map[string]string `json:"tags"`       // Event metadata

	// Event fields
	EventType string                 `json:"event_type"` // "channel_points", "subscription", etc.
	EventData map[string]interface{} `json:"event_data"` // Event-specific payload
}

// ToJSON converts the message to JSON bytes
func (m *RawChatMessage) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}
