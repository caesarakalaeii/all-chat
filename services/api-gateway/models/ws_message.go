package models

import (
	"encoding/json"
	"time"
)

// WSMessageType represents the type of WebSocket message
type WSMessageType string

const (
	// WSMessageTypeChatMessage is a chat message from a platform
	WSMessageTypeChatMessage WSMessageType = "chat_message"

	// WSMessageTypePing is a ping from server to client
	WSMessageTypePing WSMessageType = "ping"

	// WSMessageTypePong is a pong response from client to server
	WSMessageTypePong WSMessageType = "pong"

	// WSMessageTypeError is an error message
	WSMessageTypeError WSMessageType = "error"

	// WSMessageTypeConnected is sent when connection is established
	WSMessageTypeConnected WSMessageType = "connected"
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
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"display_name"`
	AvatarURL   string   `json:"avatar_url,omitempty"`
	Badges      []string `json:"badges"`
	Color       string   `json:"color,omitempty"`
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
