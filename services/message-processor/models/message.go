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
	Attachments []Attachment      `json:"attachments,omitempty"`
	RawMessage  json.RawMessage   `json:"raw_message,omitempty"`

	// Event support (backwards compatible - omitted for chat messages)
	EventType   string                 `json:"event_type,omitempty"`   // "subscription", "raid", "gift", "like_aggregate", etc.
	EventData   map[string]interface{} `json:"event_data,omitempty"`   // Platform-specific event payload
}

// UnifiedChatMessage represents the normalized, enriched message published to Pub/Sub
type UnifiedChatMessage struct {
	ID          string                 `json:"id"`
	OverlayID   string                 `json:"overlay_id"`
	Platform    string                 `json:"platform"`
	// Platforms is set only for a streamer "send to all" message whose per-platform
	// echoes were collapsed into one: it lists every platform the message was sent to,
	// so consumers can render a combined multi-platform pill. Omitted (nil) for ordinary
	// single-platform messages, where the singular Platform above is authoritative.
	Platforms   []string               `json:"platforms,omitempty"`
	ChannelID   string                 `json:"channel_id"`
	ChannelName string                 `json:"channel_name"`
	User        UserInfo               `json:"user"`
	Message     MessageInfo            `json:"message"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata"`

	// Event support (nil for chat messages)
	Event       *EventInfo             `json:"event,omitempty"`
}

// UserInfo contains information about the message author
type UserInfo struct {
	ID            string  `json:"id"`
	Username      string  `json:"username"`
	DisplayName   string  `json:"display_name"`
	AvatarURL     string  `json:"avatar_url,omitempty"`
	Badges        []Badge `json:"badges"`
	// Color is the AUTHORITATIVE username colour: the viewer's manually chosen
	// All-Chat colour, else the platform-native colour (Twitch/Kick/Discord).
	// Empty when the chatter has neither — the overlay then falls back to the
	// streamer's "Username color" setting and finally to AutoColor (ADR-0047).
	Color string `json:"color,omitempty"`
	// AutoColor is the deterministic palette fallback, always populated (except
	// when a gradient is set) so the overlay never has to invent one. It is a
	// SEPARATE field from Color precisely so the streamer's per-overlay setting
	// can rank between the platform colour and this fallback (ADR-0047).
	AutoColor      string  `json:"auto_color,omitempty"`
	NameGradient   string  `json:"name_gradient,omitempty"`   // Phase 29: raw JSONB string e.g. {"type":"linear","colors":["#ff0000","#0000ff"],"angle":90}
	SourceBadges   []Badge `json:"source_badges,omitempty"`   // Badges from source channel (shared chat)
	SourceUserID   string  `json:"source_user_id,omitempty"`  // User ID in source channel (shared chat)
	AvatarFrameURL string  `json:"avatar_frame_url,omitempty"` // Phase 30: URL of selected avatar frame
	AvatarFlairURL string  `json:"avatar_flair_url,omitempty"` // Phase 30: URL of selected avatar flair
	Pronouns       string  `json:"pronouns,omitempty"`          // Phase 9: display text e.g. "she/her"
	TwitchUsername string  `json:"-"`                           // Phase 9: INTERNAL for cross-platform pronoun lookup; never serialized
}

// Badge represents a user badge (subscriber, moderator, etc.)
type Badge struct {
	Name    string `json:"name"`     // e.g., "subscriber", "moderator"
	Version string `json:"version"`  // e.g., "12", "1"
	IconURL string `json:"icon_url"` // URL to badge image
}

// MessageInfo contains the message content, emotes, and media attachments
type MessageInfo struct {
	Text        string       `json:"text"`
	Emotes      []Emote      `json:"emotes"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// Emote represents an emote in the message
type Emote struct {
	Code      string  `json:"code"`
	Provider  string  `json:"provider"`
	URL       string  `json:"url"`
	Positions [][]int `json:"positions"` // [[start, end], [start, end]]
}

// Attachment is a renderable image/GIF/video shared in a chat message (Discord
// uploads and Tenor/Giphy link previews, and Twitch chat GIFs — see ADR-0037).
// Type is "image" or "video"; GIFs are images that animate natively. ThumbURL is
// an optional poster frame for videos. Spoiler marks media the sender flagged as a
// spoiler so the overlay can blur it. Filename doubles as the render alt text (for
// Twitch GIFs it holds the GIF's alt caption).
type Attachment struct {
	Type        string `json:"type"`
	URL         string `json:"url"`
	ContentType string `json:"content_type,omitempty"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	ThumbURL    string `json:"thumb_url,omitempty"`
	Spoiler     bool   `json:"spoiler,omitempty"`
	Filename    string `json:"filename,omitempty"`
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
