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

package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EventSettingsRepository handles persistence for overlay_event_settings
type EventSettingsRepository struct {
	pool *pgxpool.Pool
}

// NewEventSettingsRepository creates a repository backed by PostgreSQL
func NewEventSettingsRepository(connString string) (*EventSettingsRepository, error) {
	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &EventSettingsRepository{pool: pool}, nil
}

// NewEventSettingsRepositoryFromPool creates a repository from an existing pool
func NewEventSettingsRepositoryFromPool(pool *pgxpool.Pool) *EventSettingsRepository {
	return &EventSettingsRepository{pool: pool}
}

// GetByOverlayID returns event settings for the provided overlay
func (r *EventSettingsRepository) GetByOverlayID(ctx context.Context, overlayID string) (*models.EventSettings, error) {
	query := `
		SELECT id, overlay_id,
		       enable_twitch_subs, enable_twitch_resubs, enable_twitch_gift_subs,
		       enable_twitch_bits, enable_twitch_raids, enable_twitch_channel_points,
		       enable_twitch_follows,
		       enable_youtube_super_chat, enable_youtube_super_sticker, enable_youtube_members,
		       enable_youtube_member_milestones, enable_youtube_member_gifts,
		       enable_kick_subs, enable_kick_gifts,
		       enable_tiktok_likes, enable_tiktok_gifts, enable_tiktok_follows, enable_tiktok_shares,
		       enable_token_warnings,
		       tiktok_like_aggregation_window_seconds, event_display_duration_multiplier,
		       created_at, updated_at
		FROM overlay_event_settings
		WHERE overlay_id = $1
	`

	row := r.pool.QueryRow(ctx, query, overlayID)
	return scanEventSettings(row)
}

// Update persists the provided event settings
func (r *EventSettingsRepository) Update(ctx context.Context, settings *models.EventSettings) error {
	query := `
		UPDATE overlay_event_settings
		SET enable_twitch_subs = $1,
		    enable_twitch_resubs = $2,
		    enable_twitch_gift_subs = $3,
		    enable_twitch_bits = $4,
		    enable_twitch_raids = $5,
		    enable_twitch_channel_points = $6,
		    enable_twitch_follows = $7,
		    enable_youtube_super_chat = $8,
		    enable_youtube_super_sticker = $9,
		    enable_youtube_members = $10,
		    enable_youtube_member_milestones = $11,
		    enable_youtube_member_gifts = $12,
		    enable_kick_subs = $13,
		    enable_kick_gifts = $14,
		    enable_tiktok_likes = $15,
		    enable_tiktok_gifts = $16,
		    enable_tiktok_follows = $17,
		    enable_tiktok_shares = $18,
		    enable_token_warnings = $19,
		    tiktok_like_aggregation_window_seconds = $20,
		    event_display_duration_multiplier = $21,
		    updated_at = NOW()
		WHERE overlay_id = $22
		RETURNING id, overlay_id,
		          enable_twitch_subs, enable_twitch_resubs, enable_twitch_gift_subs,
		          enable_twitch_bits, enable_twitch_raids, enable_twitch_channel_points,
		          enable_twitch_follows,
		          enable_youtube_super_chat, enable_youtube_super_sticker, enable_youtube_members,
		          enable_youtube_member_milestones, enable_youtube_member_gifts,
		          enable_kick_subs, enable_kick_gifts,
		          enable_tiktok_likes, enable_tiktok_gifts, enable_tiktok_follows, enable_tiktok_shares,
		          enable_token_warnings,
		          tiktok_like_aggregation_window_seconds, event_display_duration_multiplier,
		          created_at, updated_at
	`

	row := r.pool.QueryRow(ctx, query,
		settings.EnableTwitchSubs,
		settings.EnableTwitchResubs,
		settings.EnableTwitchGiftSubs,
		settings.EnableTwitchBits,
		settings.EnableTwitchRaids,
		settings.EnableTwitchChannelPoints,
		settings.EnableTwitchFollows,
		settings.EnableYouTubeSuperChat,
		settings.EnableYouTubeSuperSticker,
		settings.EnableYouTubeMembers,
		settings.EnableYouTubeMemberMilestones,
		settings.EnableYouTubeMemberGifts,
		settings.EnableKickSubs,
		settings.EnableKickGifts,
		settings.EnableTikTokLikes,
		settings.EnableTikTokGifts,
		settings.EnableTikTokFollows,
		settings.EnableTikTokShares,
		settings.EnableTokenWarnings,
		settings.TikTokLikeAggregationWindowSeconds,
		settings.EventDisplayDurationMultiplier,
		settings.OverlayID,
	)

	updated, err := scanEventSettings(row)
	if err != nil {
		return err
	}

	*settings = *updated
	return nil
}

// scanEventSettings scans a database row into EventSettings
func scanEventSettings(row pgx.Row) (*models.EventSettings, error) {
	settings := &models.EventSettings{}

	err := row.Scan(
		&settings.ID,
		&settings.OverlayID,
		&settings.EnableTwitchSubs,
		&settings.EnableTwitchResubs,
		&settings.EnableTwitchGiftSubs,
		&settings.EnableTwitchBits,
		&settings.EnableTwitchRaids,
		&settings.EnableTwitchChannelPoints,
		&settings.EnableTwitchFollows,
		&settings.EnableYouTubeSuperChat,
		&settings.EnableYouTubeSuperSticker,
		&settings.EnableYouTubeMembers,
		&settings.EnableYouTubeMemberMilestones,
		&settings.EnableYouTubeMemberGifts,
		&settings.EnableKickSubs,
		&settings.EnableKickGifts,
		&settings.EnableTikTokLikes,
		&settings.EnableTikTokGifts,
		&settings.EnableTikTokFollows,
		&settings.EnableTikTokShares,
		&settings.EnableTokenWarnings,
		&settings.TikTokLikeAggregationWindowSeconds,
		&settings.EventDisplayDurationMultiplier,
		&settings.CreatedAt,
		&settings.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("event settings not found")
		}
		return nil, fmt.Errorf("failed to scan event settings: %w", err)
	}

	return settings, nil
}
