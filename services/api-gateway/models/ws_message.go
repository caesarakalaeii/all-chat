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

// WSMessageType represents the type of WebSocket message
type WSMessageType string

const (
	// WSMessageTypeChatMessage is a chat message or event from a platform
	WSMessageTypeChatMessage WSMessageType = "chat_message"

	// WSMessageTypeMessageUpdate is an update to an existing message (TikTok like aggregates)
	WSMessageTypeMessageUpdate WSMessageType = "message_update"

	// WSMessageTypePing is a ping from server to client
	WSMessageTypePing WSMessageType = "ping"

	// WSMessageTypePong is a pong response from client to server
	WSMessageTypePong WSMessageType = "pong"

	// WSMessageTypeError is an error message
	WSMessageTypeError WSMessageType = "error"

	// WSMessageTypeConnected is sent when connection is established
	WSMessageTypeConnected WSMessageType = "connected"

	// WSMessageTypePlatformStatus is sent when platform connection status changes
	WSMessageTypePlatformStatus WSMessageType = "platform_status"

	// WSMessageTypePollUpdate carries an aggregate poll snapshot (issue #523).
	// Broadcast to the overlay; the payload's state field conveys active vs ended.
	WSMessageTypePollUpdate WSMessageType = "poll_update"

	// WSMessageTypePredictionUpdate carries an aggregate prediction snapshot
	// (issue #523). Broadcast to the overlay; state conveys active/locked/resolved.
	WSMessageTypePredictionUpdate WSMessageType = "prediction_update"
)

// WSMessage is the wrapper for all WebSocket messages
type WSMessage struct {
	Type      WSMessageType `json:"type"`
	Data      interface{}   `json:"data,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}

// ChatMessageData represents a chat message in the WebSocket
type ChatMessageData struct {
	ID          string                 `json:"id"`
	OverlayID   string                 `json:"overlay_id"`
	Platform    string                 `json:"platform"`
	ChannelID   string                 `json:"channel_id"`
	ChannelName string                 `json:"channel_name"`
	User        UserInfo               `json:"user"`
	Message     MessageContent         `json:"message"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// UserInfo contains user information
type UserInfo struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	DisplayName string  `json:"display_name"`
	AvatarURL   string  `json:"avatar_url,omitempty"`
	BadgeURLs   []string `json:"badge_urls,omitempty"` // Legacy field for compatibility
	Badges      []Badge `json:"badges"`
	Color       string  `json:"color,omitempty"`
}

// Badge represents a user badge
type Badge struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	IconURL string `json:"icon_url"`
}

// MessageContent contains the message text and emotes
type MessageContent struct {
	Text   string  `json:"text"`
	Emotes []Emote `json:"emotes"`
}

// Emote represents an emote in the message
type Emote struct {
	Code      string  `json:"code"`
	Provider  string  `json:"provider"`
	URL       string  `json:"url"`
	Positions [][]int `json:"positions"`
}

// ErrorData represents an error message
type ErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ConnectedData is sent when a client successfully connects
type ConnectedData struct {
	OverlayID string `json:"overlay_id"`
	Message   string `json:"message"`
}

// ViewerConnectedData is sent when a viewer successfully connects (no overlay_id exposed)
type ViewerConnectedData struct {
	Message string `json:"message"`
}

// PlatformStatusData represents connection status for a platform
type PlatformStatusData struct {
	Platform     string     `json:"platform"`                  // "youtube", "twitch", "kick", "tiktok", "discord"
	ChannelID    string     `json:"channel_id"`                // Platform-specific channel identifier
	ChannelName  string     `json:"channel_name,omitempty"`    // Human-readable channel name
	Status       string     `json:"status"`                    // "connected", "reconnecting", "offline", "quota_exceeded"
	NextRetryAt  *time.Time `json:"next_retry_at,omitempty"`   // Timestamp when next reconnection happens (nil if connected)
	ErrorMessage string     `json:"error_message,omitempty"`   // Human-readable error
}

// NewChatMessage creates a new chat message WebSocket message
func NewChatMessage(data ChatMessageData) *WSMessage {
	return &WSMessage{
		Type:      WSMessageTypeChatMessage,
		Data:      data,
		Timestamp: time.Now().UTC(),
	}
}

// NewPing creates a new ping WebSocket message
func NewPing() *WSMessage {
	return &WSMessage{
		Type:      WSMessageTypePing,
		Timestamp: time.Now().UTC(),
	}
}

// NewPong creates a new pong WebSocket message
func NewPong() *WSMessage {
	return &WSMessage{
		Type:      WSMessageTypePong,
		Timestamp: time.Now().UTC(),
	}
}

// NewError creates a new error WebSocket message
func NewError(code, message string) *WSMessage {
	return &WSMessage{
		Type: WSMessageTypeError,
		Data: ErrorData{
			Code:    code,
			Message: message,
		},
		Timestamp: time.Now().UTC(),
	}
}

// NewConnected creates a new connected WebSocket message
func NewConnected(overlayID string) *WSMessage {
	return &WSMessage{
		Type: WSMessageTypeConnected,
		Data: ConnectedData{
			OverlayID: overlayID,
			Message:   "Connected to overlay stream",
		},
		Timestamp: time.Now().UTC(),
	}
}

// NewViewerConnected creates a new connected message for viewers (no overlay_id)
func NewViewerConnected() *WSMessage {
	return &WSMessage{
		Type: WSMessageTypeConnected,
		Data: ViewerConnectedData{
			Message: "Connected to chat stream",
		},
		Timestamp: time.Now().UTC(),
	}
}

// NewPlatformStatus creates a new platform status WebSocket message
func NewPlatformStatus(data PlatformStatusData) *WSMessage {
	return &WSMessage{
		Type:      WSMessageTypePlatformStatus,
		Data:      data,
		Timestamp: time.Now().UTC(),
	}
}

// ToJSON converts a WSMessage to JSON bytes
func (m *WSMessage) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// ParseWSMessage parses JSON bytes into a WSMessage
func ParseWSMessage(data []byte) (*WSMessage, error) {
	var msg WSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}
