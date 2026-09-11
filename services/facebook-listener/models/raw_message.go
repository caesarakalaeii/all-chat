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

// RawChatMessage represents a raw chat message from the Facebook Graph API.
// This is published to Redis Streams for processing (same format as other listeners).
type RawChatMessage struct {
	MessageID   string            `json:"message_id"` // Graph comment id
	Platform    string            `json:"platform"`   // "facebook"
	OverlayID   string            `json:"overlay_id,omitempty"`
	ChannelID   string            `json:"channel_id"` // Facebook Page ID
	StreamID    string            `json:"stream_id"`  // Live video ID
	ChannelName string            `json:"channel_name,omitempty"`
	UserID      string            `json:"user_id"`  // Commenter's Facebook user ID (PSID-style app-scoped id)
	Username    string            `json:"username"` // Commenter display name
	Text        string            `json:"text"`     // Comment message text
	Timestamp   time.Time         `json:"timestamp"`
	Tags        map[string]string `json:"tags"`

	// Event support (backwards compatible - omitted for regular chat messages)
	EventType string                 `json:"event_type,omitempty"`
	EventData map[string]interface{} `json:"event_data,omitempty"`
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
