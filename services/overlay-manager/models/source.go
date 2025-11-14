package models

import (
	"errors"
	"time"
)

// ChatSource represents a chat source for an overlay
type ChatSource struct {
	ID           string                 `json:"id"`
	OverlayID    string                 `json:"overlay_id"`
	Platform     string                 `json:"platform"`
	ChannelID    string                 `json:"channel_id"`
	ChannelName  string                 `json:"channel_name"`
	AuthRequired bool                   `json:"auth_required"`
	Config       map[string]interface{} `json:"config"`
	IsActive     bool                   `json:"is_active"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// Validate validates the chat source fields
func (s *ChatSource) Validate() error {
	if s.OverlayID == "" {
		return errors.New("overlay_id is required")
	}

	if s.Platform == "" {
		return errors.New("platform is required")
	}

	validPlatforms := map[string]bool{
		"twitch":  true,
		"youtube": true,
		"kick":    true,
		"tiktok":  true,
	}

	if !validPlatforms[s.Platform] {
		return errors.New("platform must be one of: twitch, youtube, kick, tiktok")
	}

	if s.ChannelID == "" {
		return errors.New("channel_id is required")
	}

	if len(s.ChannelID) > 100 {
		return errors.New("channel_id must be 100 characters or less")
	}

	if len(s.ChannelName) > 100 {
		return errors.New("channel_name must be 100 characters or less")
	}

	return nil
}
