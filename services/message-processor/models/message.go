package models

import (
	"encoding/json"
	"time"
)

// RawChatMessage represents the raw message from Redis Streams (from Twitch Listener)
type RawChatMessage struct {
	MessageID   string            `json:"message_id"`
	Platform    string            `json:"platform"`
	OverlayID   string            `json:"overlay_id,omitempty"`
	ChannelID   string            `json:"channel_id"`
	ChannelName string            `json:"channel_name,omitempty"`
	UserID      string            `json:"user_id"`
	Username    string            `json:"username"`
	Text        string            `json:"text"`
	Timestamp   time.Time         `json:"timestamp"`
	Tags        map[string]string `json:"tags"`
	RawMessage  json.RawMessage   `json:"raw_message,omitempty"`

	// Event support (backwards compatible - omitted for chat messages)
	EventType   string                 `json:"event_type,omitempty"`   // "subscription", "raid", "gift", "like_aggregate", etc.
	EventData   map[string]interface{} `json:"event_data,omitempty"`   // Platform-specific event payload
}

// UnifiedChatMessage represents the normalized, enriched message published to Pub/Sub
type UnifiedChatMessage struct {
	ID              string                 `json:"id"`
	OverlayID       string                 `json:"overlay_id"`
	Platform        string                 `json:"platform"`
	ChannelID       string                 `json:"channel_id"`
	ChannelName     string                 `json:"channel_name"`
	User            UserInfo               `json:"user"`
	Message         MessageInfo            `json:"message"`
	Timestamp       time.Time              `json:"timestamp"`
	Metadata        map[string]interface{} `json:"metadata"`
	ClientMessageID string                 `json:"client_message_id,omitempty"` // Client-generated ID for optimistic UI deduplication

	// Event support (nil for chat messages)
	Event           *EventInfo             `json:"event,omitempty"`
}

// UserInfo contains information about the message author
type UserInfo struct {
	ID            string  `json:"id"`
	Username      string  `json:"username"`
	DisplayName   string  `json:"display_name"`
	AvatarURL     string  `json:"avatar_url,omitempty"`
	Badges        []Badge `json:"badges"`
	Color         string  `json:"color,omitempty"`
	NameGradient   string  `json:"name_gradient,omitempty"`   // Phase 29: raw JSONB string e.g. {"type":"linear","colors":["#ff0000","#0000ff"],"angle":90}
	SourceBadges   []Badge `json:"source_badges,omitempty"`   // Badges from source channel (shared chat)
	SourceUserID   string  `json:"source_user_id,omitempty"`  // User ID in source channel (shared chat)
	AvatarFrameURL string  `json:"avatar_frame_url,omitempty"` // Phase 30: URL of selected avatar frame
	AvatarFlairURL string  `json:"avatar_flair_url,omitempty"` // Phase 30: URL of selected avatar flair
}

// Badge represents a user badge (subscriber, moderator, etc.)
type Badge struct {
	Name    string `json:"name"`     // e.g., "subscriber", "moderator"
	Version string `json:"version"`  // e.g., "12", "1"
	IconURL string `json:"icon_url"` // URL to badge image
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

// EventInfo contains information about platform events (subscriptions, donations, etc.)
type EventInfo struct {
	Type           string                 `json:"type"`                       // "subscription", "bits", "follow", "raid", etc.
	Tier           string                 `json:"tier"`                       // "high", "medium", "low"
	Value          *EventValue            `json:"value,omitempty"`            // Monetary/count value
	Duration       int                    `json:"duration"`                   // Display duration in seconds
	AggregationID  string                 `json:"aggregation_id,omitempty"`   // For TikTok like aggregation
	IsUpdate       bool                   `json:"is_update"`                  // True if updating existing message
	Metadata       map[string]interface{} `json:"metadata,omitempty"`         // Event-specific metadata
}

// EventValue represents the value/amount associated with an event
type EventValue struct {
	Amount      float64 `json:"amount"`        // Numeric value (100 bits, 5 gifts, 50 likes)
	Currency    string  `json:"currency"`      // "bits", "USD", "likes", "gifts", "viewers"
	DisplayText string  `json:"display_text"`  // Human-readable string ("100 bits", "$4.99", "50 likes")
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
