package models

import (
	"time"

	"github.com/google/uuid"
)

// Clip represents a platform clip or user-provided video
type Clip struct {
	ID                uuid.UUID  `json:"id"`
	UserID            uuid.UUID  `json:"user_id"`
	StreamSessionID   *uuid.UUID `json:"stream_session_id,omitempty"`
	Platform          string     `json:"platform"` // twitch, kick, youtube, user_upload
	PlatformClipID    *string    `json:"platform_clip_id,omitempty"`
	ClipURL           string     `json:"clip_url"`
	EmbedURL          *string    `json:"embed_url,omitempty"`
	ThumbnailURL      *string    `json:"thumbnail_url,omitempty"`
	Title             *string    `json:"title,omitempty"`
	DurationSeconds   *int       `json:"duration_seconds,omitempty"`
	ViewCount         int        `json:"view_count"`
	CreatedAtPlatform *time.Time `json:"created_at_platform,omitempty"`
	IsUserProvided    bool       `json:"is_user_provided"`
	UserNotes         *string    `json:"user_notes,omitempty"`
	RankScore         *float64   `json:"rank_score,omitempty"`
	FetchedAt         time.Time  `json:"fetched_at"`
	LastUpdated       time.Time  `json:"last_updated"`
}

// Platform constants
const (
	PlatformTwitch     = "twitch"
	PlatformKick       = "kick"
	PlatformYouTube    = "youtube"
	PlatformUserUpload = "user_upload"
)

// UserCreditRollSettings represents user preferences for credit rolls
type UserCreditRollSettings struct {
	UserID                  uuid.UUID              `json:"user_id"`
	SectionsConfig          map[string]interface{} `json:"sections_config"`
	ClipSelectionMode       string                 `json:"clip_selection_mode"` // auto, manual
	MaxClips                int                    `json:"max_clips"`
	MinClips                int                    `json:"min_clips"`
	PreferRecent            bool                   `json:"prefer_recent"`
	MinDurationSeconds      int                    `json:"min_duration_seconds"`
	MaxDurationSeconds      int                    `json:"max_duration_seconds"`
	FallbackVideoURL        *string                `json:"fallback_video_url,omitempty"`
	FallbackVideoStartTime  *int                   `json:"fallback_video_start_time,omitempty"`
	DefaultBackgroundType   string                 `json:"default_background_type"`
	DefaultBackgroundConfig map[string]interface{} `json:"default_background_config"`
	StylingConfig           map[string]interface{} `json:"styling_config"`
	MusicEnabled            bool                   `json:"music_enabled"`
	MusicURL                *string                `json:"music_url,omitempty"`
	MusicVolume             float64                `json:"music_volume"`
	CreatedAt               time.Time              `json:"created_at"`
	UpdatedAt               time.Time              `json:"updated_at"`
}

// Default values
const (
	DefaultClipSelectionMode  = "auto"
	DefaultMaxClips           = 5
	DefaultMinClips           = 1
	DefaultMinDurationSeconds = 10
	DefaultMaxDurationSeconds = 60
	DefaultBackgroundType     = "gradient"
	DefaultMusicVolume        = 0.7
)
