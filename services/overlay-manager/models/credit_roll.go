package models

import "time"

// CreditRollConfig represents the credit roll configuration for an overlay
type CreditRollConfig struct {
	ID        string `json:"id" db:"id"`
	OverlayID string `json:"overlay_id" db:"overlay_id"`

	// Feature enable
	Enabled bool `json:"enabled" db:"enabled"`

	// Event type filters
	IncludeSubs          bool `json:"include_subs" db:"include_subs"`
	IncludeResubs        bool `json:"include_resubs" db:"include_resubs"`
	IncludeGiftSubs      bool `json:"include_gift_subs" db:"include_gift_subs"`
	IncludeBits          bool `json:"include_bits" db:"include_bits"`
	IncludeRaids         bool `json:"include_raids" db:"include_raids"`
	IncludeChannelPoints bool `json:"include_channel_points" db:"include_channel_points"`
	IncludeSuperChats    bool `json:"include_super_chats" db:"include_super_chats"`
	IncludeMemberships   bool `json:"include_memberships" db:"include_memberships"`
	IncludeFollows       bool `json:"include_follows" db:"include_follows"`

	// Leaderboard settings
	LeaderboardTopN    int    `json:"leaderboard_top_n" db:"leaderboard_top_n"`
	LeaderboardSortBy  string `json:"leaderboard_sort_by" db:"leaderboard_sort_by"` // "value" or "count"

	// Display settings
	ScrollSpeed              int     `json:"scroll_speed" db:"scroll_speed"`
	DisplayDurationSeconds   int     `json:"display_duration_seconds" db:"display_duration_seconds"`
	BackgroundOpacity        float64 `json:"background_opacity" db:"background_opacity"`
	Theme                    string  `json:"theme" db:"theme"` // "classic", "cinematic", "modern"

	// Clips settings
	ClipsEnabled      bool `json:"clips_enabled" db:"clips_enabled"`
	ClipsMaxCount     int  `json:"clips_max_count" db:"clips_max_count"`
	ClipsFallbackDays int  `json:"clips_fallback_days" db:"clips_fallback_days"`

	// Custom CSS
	CustomCSS string `json:"custom_css" db:"custom_css"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// StreamSession represents a streaming session
type StreamSession struct {
	ID        string `json:"id" db:"id"`
	OverlayID string `json:"overlay_id" db:"overlay_id"`

	StartedAt time.Time  `json:"started_at" db:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty" db:"ended_at"`
	State     string     `json:"state" db:"state"` // ACTIVE, ENDING, COMPLETED

	TotalEvents        int                    `json:"total_events" db:"total_events"`
	EventCounts        map[string]interface{} `json:"event_counts" db:"event_counts"`
	TotalMonetaryValue float64                `json:"total_monetary_value" db:"total_monetary_value"`

	CreditRollDisplayedCount int        `json:"credit_roll_displayed_count" db:"credit_roll_displayed_count"`
	LastCreditRollAt         *time.Time `json:"last_credit_roll_at,omitempty" db:"last_credit_roll_at"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// LeaderboardEntry represents a single entry in a leaderboard
type LeaderboardEntry struct {
	Rank        int                    `json:"rank"`
	UserID      string                 `json:"user_id"`
	DisplayName string                 `json:"display_name"`
	AvatarURL   string                 `json:"avatar_url"`
	Platform    string                 `json:"platform"`
	Count       int                    `json:"count,omitempty"`
	TotalValue  float64                `json:"total_value,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Leaderboards contains all leaderboard categories
type Leaderboards struct {
	Subs       []LeaderboardEntry `json:"subs,omitempty"`
	Bits       []LeaderboardEntry `json:"bits,omitempty"`
	Raids      []LeaderboardEntry `json:"raids,omitempty"`
	SuperChats []LeaderboardEntry `json:"super_chats,omitempty"`
	Follows    []LeaderboardEntry `json:"follows,omitempty"`
	Gifts      []LeaderboardEntry `json:"gifts,omitempty"`
	Points     []LeaderboardEntry `json:"points,omitempty"`
}

// Clip represents a Twitch clip
type Clip struct {
	ID           string    `json:"id"`
	URL          string    `json:"url"`
	EmbedURL     string    `json:"embed_url"`
	Title        string    `json:"title"`
	ViewCount    int       `json:"view_count"`
	CreatedAt    time.Time `json:"created_at"`
	ThumbnailURL string    `json:"thumbnail_url"`
	Duration     float64   `json:"duration"`
}

// CreditRollResponse is the response for GET /credit-roll
type CreditRollResponse struct {
	OverlayID              string       `json:"overlay_id"`
	SessionID              string       `json:"session_id"`
	SessionStartedAt       time.Time    `json:"session_started_at"`
	SessionDurationSeconds int          `json:"session_duration_seconds"`
	Leaderboards           Leaderboards `json:"leaderboards"`
	Clips                  []Clip       `json:"clips"`
	ClipsIsFallback        bool         `json:"clips_is_fallback"`
}

// SessionInfo represents active session info stored in Redis
type SessionInfo struct {
	SessionID    string    `json:"session_id"`
	StartedAt    time.Time `json:"started_at"`
	State        string    `json:"state"`
	EventCount   int       `json:"event_count"`
	LastEventAt  time.Time `json:"last_event_at,omitempty"`
}
