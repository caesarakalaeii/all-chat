package repository

import (
	"context"
	"fmt"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CreditRollRepository handles database operations for credit roll configs
type CreditRollRepository struct {
	db *pgxpool.Pool
}

// NewCreditRollRepository creates a new credit roll repository
func NewCreditRollRepository(db *pgxpool.Pool) *CreditRollRepository {
	return &CreditRollRepository{db: db}
}

// GetByOverlayID retrieves credit roll config by overlay ID
func (r *CreditRollRepository) GetByOverlayID(ctx context.Context, overlayID string) (*models.CreditRollConfig, error) {
	query := `
		SELECT id, overlay_id, enabled,
		       include_subs, include_resubs, include_gift_subs, include_bits,
		       include_raids, include_channel_points, include_super_chats,
		       include_memberships, include_follows,
		       leaderboard_top_n, leaderboard_sort_by,
		       scroll_speed, display_duration_seconds, background_opacity, theme,
		       clips_enabled, clips_max_count, clips_fallback_days,
		       custom_css,
		       created_at, updated_at
		FROM credit_roll_configs
		WHERE overlay_id = $1
	`

	config := &models.CreditRollConfig{}
	err := r.db.QueryRow(ctx, query, overlayID).Scan(
		&config.ID, &config.OverlayID, &config.Enabled,
		&config.IncludeSubs, &config.IncludeResubs, &config.IncludeGiftSubs, &config.IncludeBits,
		&config.IncludeRaids, &config.IncludeChannelPoints, &config.IncludeSuperChats,
		&config.IncludeMemberships, &config.IncludeFollows,
		&config.LeaderboardTopN, &config.LeaderboardSortBy,
		&config.ScrollSpeed, &config.DisplayDurationSeconds, &config.BackgroundOpacity, &config.Theme,
		&config.ClipsEnabled, &config.ClipsMaxCount, &config.ClipsFallbackDays,
		&config.CustomCSS,
		&config.CreatedAt, &config.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get credit roll config: %w", err)
	}

	return config, nil
}

// Update updates a credit roll config
func (r *CreditRollRepository) Update(ctx context.Context, config *models.CreditRollConfig) error {
	query := `
		UPDATE credit_roll_configs
		SET enabled = $1,
		    include_subs = $2, include_resubs = $3, include_gift_subs = $4, include_bits = $5,
		    include_raids = $6, include_channel_points = $7, include_super_chats = $8,
		    include_memberships = $9, include_follows = $10,
		    leaderboard_top_n = $11, leaderboard_sort_by = $12,
		    scroll_speed = $13, display_duration_seconds = $14, background_opacity = $15, theme = $16,
		    clips_enabled = $17, clips_max_count = $18, clips_fallback_days = $19,
		    custom_css = $20,
		    updated_at = NOW()
		WHERE id = $21
	`

	_, err := r.db.Exec(ctx, query,
		config.Enabled,
		config.IncludeSubs, config.IncludeResubs, config.IncludeGiftSubs, config.IncludeBits,
		config.IncludeRaids, config.IncludeChannelPoints, config.IncludeSuperChats,
		config.IncludeMemberships, config.IncludeFollows,
		config.LeaderboardTopN, config.LeaderboardSortBy,
		config.ScrollSpeed, config.DisplayDurationSeconds, config.BackgroundOpacity, config.Theme,
		config.ClipsEnabled, config.ClipsMaxCount, config.ClipsFallbackDays,
		config.CustomCSS,
		config.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update credit roll config: %w", err)
	}

	return nil
}
