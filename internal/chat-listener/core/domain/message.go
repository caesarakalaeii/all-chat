package domain

import "time"

// ChatMessage represents a complete chat message with enriched data
type ChatMessage struct {
	OverlayID string    `json:"overlay_id"`
	Channel   string    `json:"channel"`
	User      User      `json:"user"`
	Message   Message   `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// User represents the user who sent the message
type User struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Color       string   `json:"color"`
	Badges      []string `json:"badges"`
}

// Message represents the message content with emotes
type Message struct {
	Text   string  `json:"text"`
	Emotes []Emote `json:"emotes"`
}

// Emote represents an emote in the message
type Emote struct {
	Code     string `json:"code"`
	URL      string `json:"url"`
	Provider string `json:"provider"` // "twitch", "7tv", "bttv", "ffz"
	Animated bool   `json:"animated"`
}

// ActiveChannel represents a Twitch channel that should be monitored
type ActiveChannel struct {
	OverlayID      string
	Channel        string
	Enable7TV      bool
	EnableBTTV     bool
	EnableFFZ      bool
	BlockedUsers   []string
	BlockedWords   []string
}
