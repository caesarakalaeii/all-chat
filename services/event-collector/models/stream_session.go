package models

import (
	"time"

	"github.com/google/uuid"
)

// StreamSession represents a single streaming session
type StreamSession struct {
	ID           uuid.UUID              `json:"id"`
	UserID       uuid.UUID              `json:"user_id"`
	Title        *string                `json:"title,omitempty"`
	Description  *string                `json:"description,omitempty"`
	StartedAt    time.Time              `json:"started_at"`
	EndedAt      *time.Time             `json:"ended_at,omitempty"`
	PlatformInfo map[string]interface{} `json:"platform_info"` // Platform-specific data
	Status       string                 `json:"status"` // live, ended, archived
	Stats        SessionStats           `json:"stats"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// SessionStats represents aggregated statistics for a session
type SessionStats struct{
	TotalEvents      int `json:"total_events"`
	Followers        int `json:"followers"`
	Subscribers      int `json:"subscribers"`
	BitsTotal        int `json:"bits_total"`
	SuperChatTotal   int `json:"super_chat_total"`
	UniqueChatters   int `json:"unique_chatters"`
	PeakViewers      int `json:"peak_viewers"`
}

// Session status constants
const (
	SessionStatusLive     = "live"
	SessionStatusEnded    = "ended"
	SessionStatusArchived = "archived"
)
