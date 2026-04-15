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

import "time"

// YouTubeStream represents a YouTube live stream being monitored
type YouTubeStream struct {
	StreamID        string    `json:"stream_id"`        // YouTube live stream ID (e.g., "abc123xyz")
	VideoID         string    `json:"video_id"`         // YouTube video ID (same as StreamID, kept for clarity)
	ChannelID       string    `json:"channel_id"`       // YouTube channel ID (e.g., "UCxxxxxx")
	ChannelName     string    `json:"channel_name"`     // Display name
	Title           string    `json:"title"`            // Stream title
	ThumbnailURL    string    `json:"thumbnail_url"`    // Thumbnail image URL
	LiveChatID      string    `json:"live_chat_id"`     // Live chat ID for API calls
	OverlayID       string    `json:"overlay_id"`       // Overlay that requested this stream
	IsLive          bool      `json:"is_live"`          // Stream is currently live
	PollingInterval int       `json:"polling_interval"` // Milliseconds between polls (from API)
	NextPageToken   string    `json:"next_page_token"`  // Token for next page of messages
	LastPolledAt    time.Time `json:"last_polled_at"`   // Last time we polled
	PublishedAt     time.Time `json:"published_at"`     // When the stream was published
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// StreamSource represents an active YouTube channel to monitor
type StreamSource struct {
	OverlayID string // UUID of the overlay
	ChannelID string // YouTube channel ID
}
