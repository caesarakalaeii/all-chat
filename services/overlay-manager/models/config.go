package models

import "time"

// OverlayConfig represents the persisted configuration for an overlay display
type OverlayConfig struct {
	ID              string         `json:"id"`
	OverlayID       string         `json:"overlay_id"`
	DisplaySettings map[string]any `json:"display_settings"`
	FilterSettings  map[string]any `json:"filter_settings"`
	Enable7TV       bool           `json:"enable_7tv"`
	EnableBTTV      bool           `json:"enable_bttv"`
	EnableFFZ       bool           `json:"enable_ffz"`
	CustomCSS       string         `json:"custom_css"`
	VisualSettings  map[string]any `json:"visual_settings"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
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
