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
	"errors"
	"time"
)

// ChatSource represents a chat source configuration for an overlay
type ChatSource struct {
	ID            string                 `json:"id"`
	OverlayID     string                 `json:"overlay_id"`
	Platform      string                 `json:"platform"`
	ChannelID     string                 `json:"channel_id"`
	ChannelName   string                 `json:"channel_name"`
	ChannelHandle *string                `json:"channel_handle,omitempty"`
	AuthRequired  bool                   `json:"auth_required"`
	Config        map[string]interface{} `json:"config"`
	IsActive      bool                   `json:"is_active"`
	ShareStatus   *string                `json:"share_status,omitempty"` // Only set for platform='shared_overlay'
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

// Valid platforms
var validPlatforms = map[string]bool{
	"twitch":         true,
	"youtube":        true,
	"kick":           true,
	"tiktok":         true,
	"shared_overlay": true, // Phase 16: shared overlay sources
	"discord":        true, // Phase 27: Discord Listener
}

// Validate validates the chat source fields
func (c *ChatSource) Validate() error {
	if c.OverlayID == "" {
		return errors.New("overlay_id is required")
	}

	if c.Platform == "" {
		return errors.New("platform is required")
	}

	if !c.IsValidPlatform() {
		return errors.New("invalid platform")
	}

	if c.ChannelID == "" {
		return errors.New("channel_id is required")
	}

	if c.ChannelName == "" {
		return errors.New("channel_name is required")
	}

	return nil
}

// IsValidPlatform checks if the platform is supported
func (c *ChatSource) IsValidPlatform() bool {
	return validPlatforms[c.Platform]
}
