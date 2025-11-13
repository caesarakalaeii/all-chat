package models

import (
	"encoding/json"
	"time"
)

// RawChatMessage represents a raw chat message from Twitch IRC
// This is published to Redis Streams for processing
type RawChatMessage struct {
	MessageID string            `json:"message_id"` // UUID
	Platform  string            `json:"platform"`   // "twitch"
	ChannelID string            `json:"channel_id"` // Twitch channel name (lowercase)
	UserID    string            `json:"user_id"`    // Twitch user ID
	Username  string            `json:"username"`   // Username (lowercase)
	Text      string            `json:"text"`       // Raw message text
	Timestamp time.Time         `json:"timestamp"`  // UTC timestamp
	Tags      map[string]string `json:"tags"`       // IRC tags (badges, color, emotes, etc.)
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
