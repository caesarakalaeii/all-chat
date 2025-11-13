package models

import (
	"encoding/json"
	"time"
)

// RawChatMessage represents the raw message from Redis Streams (from Twitch Listener)
type RawChatMessage struct {
	MessageID string            `json:"message_id"`
	Platform  string            `json:"platform"`
	ChannelID string            `json:"channel_id"`
	UserID    string            `json:"user_id"`
	Username  string            `json:"username"`
	Text      string            `json:"text"`
	Timestamp time.Time         `json:"timestamp"`
	Tags      map[string]string `json:"tags"`
}

// UnifiedChatMessage represents the normalized, enriched message published to Pub/Sub
type UnifiedChatMessage struct {
	ID          string                 `json:"id"`
	OverlayID   string                 `json:"overlay_id"`
	Platform    string                 `json:"platform"`
	ChannelID   string                 `json:"channel_id"`
	ChannelName string                 `json:"channel_name"`
	User        UserInfo               `json:"user"`
	Message     MessageInfo            `json:"message"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// UserInfo contains information about the message author
type UserInfo struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"display_name"`
	AvatarURL   string   `json:"avatar_url,omitempty"`
	Badges      []string `json:"badges"`
	Color       string   `json:"color,omitempty"`
}

// MessageInfo contains the message content and emotes
type MessageInfo struct {
	Text   string  `json:"text"`
	Emotes []Emote `json:"emotes"`
}

// Emote represents an emote in the message
type Emote struct {
	Code      string  `json:"code"`
	Provider  string  `json:"provider"`
	URL       string  `json:"url"`
	Positions [][]int `json:"positions"` // [[start, end], [start, end]]
}

// ToJSON converts UnifiedChatMessage to JSON bytes
func (m *UnifiedChatMessage) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// FromJSON parses JSON bytes into UnifiedChatMessage
func FromJSON(data []byte) (*UnifiedChatMessage, error) {
	var msg UnifiedChatMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// ParseRawMessage parses JSON bytes into RawChatMessage
func ParseRawMessage(data []byte) (*RawChatMessage, error) {
	var msg RawChatMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// OverlayTarget represents an overlay that should receive this message
type OverlayTarget struct {
	OverlayID string
	UserID    string
}
