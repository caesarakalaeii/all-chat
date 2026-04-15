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

// ActiveSource represents a source that should be monitored
type ActiveSource struct {
	ID           string    `json:"id"`            // overlay_chat_source.id (UUID)
	OverlayID    string    `json:"overlay_id"`    // overlay ID
	Platform     string    `json:"platform"`      // "twitch", "youtube", "kick", "tiktok"
	ChannelID    string    `json:"channel_id"`    // Platform-specific channel ID
	StreamID     string    `json:"stream_id"`     // YouTube live stream ID (empty for Twitch)
	StreamSelect string    `json:"stream_select"` // Stream selection strategy (e.g. "most_viewers", "title_match")
	StreamMatch  string    `json:"stream_match"`  // Match term for title_match strategy
	IsActive     bool      `json:"is_active"`     // Should be monitored
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// LeadershipStatus represents leadership for a stream
type LeadershipStatus struct {
	StreamID   string    `json:"stream_id"`   // YouTube stream ID
	Platform   string    `json:"platform"`    // "youtube"
	LeaderID   string    `json:"leader_id"`   // Listener instance ID
	AcquiredAt time.Time `json:"acquired_at"` // When leadership was acquired
	ExpiresAt  time.Time `json:"expires_at"`  // When lock expires
	IsLeader   bool      `json:"is_leader"`   // Is current instance the leader
}
