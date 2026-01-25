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
