package models

import "time"

// EventSettings represents the event display configuration for an overlay
type EventSettings struct {
	ID        string    `json:"id" db:"id"`
	OverlayID string    `json:"overlay_id" db:"overlay_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`

	// Twitch Events
	EnableTwitchSubs         bool `json:"enable_twitch_subs" db:"enable_twitch_subs"`
	EnableTwitchResubs       bool `json:"enable_twitch_resubs" db:"enable_twitch_resubs"`
	EnableTwitchGiftSubs     bool `json:"enable_twitch_gift_subs" db:"enable_twitch_gift_subs"`
	EnableTwitchBits         bool `json:"enable_twitch_bits" db:"enable_twitch_bits"`
	EnableTwitchRaids        bool `json:"enable_twitch_raids" db:"enable_twitch_raids"`
	EnableTwitchChannelPoints bool `json:"enable_twitch_channel_points" db:"enable_twitch_channel_points"`

	// YouTube Events
	EnableYouTubeSuperChat        bool `json:"enable_youtube_super_chat" db:"enable_youtube_super_chat"`
	EnableYouTubeSuperSticker     bool `json:"enable_youtube_super_sticker" db:"enable_youtube_super_sticker"`
	EnableYouTubeMembers          bool `json:"enable_youtube_members" db:"enable_youtube_members"`
	EnableYouTubeMemberMilestones bool `json:"enable_youtube_member_milestones" db:"enable_youtube_member_milestones"`
	EnableYouTubeMemberGifts      bool `json:"enable_youtube_member_gifts" db:"enable_youtube_member_gifts"`

	// Kick Events
	EnableKickSubs  bool `json:"enable_kick_subs" db:"enable_kick_subs"`
	EnableKickGifts bool `json:"enable_kick_gifts" db:"enable_kick_gifts"`

	// TikTok Events
	EnableTikTokLikes   bool `json:"enable_tiktok_likes" db:"enable_tiktok_likes"`
	EnableTikTokGifts   bool `json:"enable_tiktok_gifts" db:"enable_tiktok_gifts"`
	EnableTikTokFollows bool `json:"enable_tiktok_follows" db:"enable_tiktok_follows"`
	EnableTikTokShares  bool `json:"enable_tiktok_shares" db:"enable_tiktok_shares"`

	// System Events
	EnableTokenWarnings bool `json:"enable_token_warnings" db:"enable_token_warnings"`

	// Aggregation Settings
	TikTokLikeAggregationWindowSeconds int     `json:"tiktok_like_aggregation_window_seconds" db:"tiktok_like_aggregation_window_seconds"`
	EventDisplayDurationMultiplier     float64 `json:"event_display_duration_multiplier" db:"event_display_duration_multiplier"`
}
