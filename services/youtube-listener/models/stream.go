package models

import "time"

// YouTubeStream represents a YouTube live stream being monitored
type YouTubeStream struct {
	StreamID        string    `json:"stream_id"`        // YouTube live stream ID (e.g., "abc123xyz")
	ChannelID       string    `json:"channel_id"`       // YouTube channel ID (e.g., "UCxxxxxx")
	ChannelName     string    `json:"channel_name"`     // Display name
	LiveChatID      string    `json:"live_chat_id"`     // Live chat ID for API calls
	IsLive          bool      `json:"is_live"`          // Stream is currently live
	PollingInterval int       `json:"polling_interval"` // Milliseconds between polls (from API)
	NextPageToken   string    `json:"next_page_token"`  // Token for next page of messages
	LastPolledAt    time.Time `json:"last_polled_at"`   // Last time we polled
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// StreamSource represents an active YouTube channel to monitor
type StreamSource struct {
	OverlayID string // UUID of the overlay
	ChannelID string // YouTube channel ID
}
