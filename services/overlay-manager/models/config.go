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

// OverlayConfig represents the persisted configuration for an overlay display
type OverlayConfig struct {
	ID                 string         `json:"id"`
	OverlayID          string         `json:"overlay_id"`
	DisplaySettings    map[string]any `json:"display_settings"`
	FilterSettings     map[string]any `json:"filter_settings"`
	Enable7TV          bool           `json:"enable_7tv"`
	EnableBTTV         bool           `json:"enable_bttv"`
	EnableFFZ          bool           `json:"enable_ffz"`
	CustomCSS          string         `json:"custom_css"`
	VisualSettings     map[string]any `json:"visual_settings"`
	SevenTVEmoteSetID  string         `json:"seventv_emote_set_id"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

// EnsureMaps initializes map fields when nil
func (c *OverlayConfig) EnsureMaps() {
	if c.DisplaySettings == nil {
		c.DisplaySettings = map[string]any{}
	}
	if c.FilterSettings == nil {
		c.FilterSettings = map[string]any{}
	}
	if c.VisualSettings == nil {
		c.VisualSettings = map[string]any{}
	}
}
