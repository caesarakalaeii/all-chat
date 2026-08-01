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

// EventSettings represents the event display configuration for an overlay
// Includes platform events and system events like token warnings
type EventSettings struct {
	ID        string    `json:"id" db:"id"`
	OverlayID string    `json:"overlay_id" db:"overlay_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`

	// Twitch Events
	EnableTwitchSubs          bool `json:"enable_twitch_subs" db:"enable_twitch_subs"`
	EnableTwitchResubs        bool `json:"enable_twitch_resubs" db:"enable_twitch_resubs"`
	EnableTwitchGiftSubs      bool `json:"enable_twitch_gift_subs" db:"enable_twitch_gift_subs"`
	EnableTwitchBits          bool `json:"enable_twitch_bits" db:"enable_twitch_bits"`
	EnableTwitchRaids         bool `json:"enable_twitch_raids" db:"enable_twitch_raids"`
	EnableTwitchChannelPoints bool `json:"enable_twitch_channel_points" db:"enable_twitch_channel_points"`
	EnableTwitchFollows       bool `json:"enable_twitch_follows" db:"enable_twitch_follows"`
	// Watch streaks arrive on channel.chat.notification and carry the viewer's own chat message
	// (ADR-0046); they fire once per returning viewer per stream, hence a dedicated toggle.
	EnableTwitchWatchStreaks bool `json:"enable_twitch_watch_streaks" db:"enable_twitch_watch_streaks"`

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
