package models

import (
	"encoding/json"
	"time"
)

// RawChatMessage represents a raw chat message from YouTube Live Chat API
// This is published to Redis Streams for processing (same format as Twitch)
type RawChatMessage struct {
	MessageID string            `json:"message_id"` // UUID
	Platform  string            `json:"platform"`   // "youtube"
	ChannelID string            `json:"channel_id"` // YouTube channel ID
	StreamID  string            `json:"stream_id"`  // YouTube live stream ID (optional)
	UserID    string            `json:"user_id"`    // YouTube user channel ID
	Username  string            `json:"username"`   // Display name
	Text      string            `json:"text"`       // Message text
	Timestamp time.Time         `json:"timestamp"`  // UTC timestamp (from publishedAt)
	Tags      map[string]string `json:"tags"`       // YouTube-specific metadata

	// Event support (backwards compatible - omitted for regular chat messages)
	EventType string                 `json:"event_type,omitempty"` // "super_chat", "member_milestone", etc.
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
