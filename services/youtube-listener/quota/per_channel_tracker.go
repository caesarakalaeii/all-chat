package quota

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// ChannelQuota represents quota allocation and usage for a single channel
type ChannelQuota struct {
	ChannelID                    string
	UserID                       string
	DailyQuotaUsed               int
	DailyQuotaLimit              int
	QuotaResetAt                 time.Time
	PriorityTier                 string
	LastSeenLiveAt               *time.Time
	TotalStreamsDetected         int
	ConsecutiveOfflineChecks     int
	ConsecutiveStatusCheckFailures int
	LastStatusCheckAt            *time.Time
	LastFullDetectionAt          *time.Time
	CachedVideoID                *string
	CachedVideoTitle             *string
	CreatedAt                    time.Time
	UpdatedAt                    time.Time
}

// PerChannelTracker manages quota allocation and usage per YouTube channel
type PerChannelTracker struct {
	db           *pgxpool.Pool
	redis        *redis.Client
	logger       *zap.Logger
	globalLimit  int

	// Tier quota limits
	highTierQuota     int
	standardTierQuota int
	lowTierQuota      int
}

// Config holds configuration for the per-channel tracker
type Config struct {
	GlobalDailyQuota  int
	HighTierQuota     int
	StandardTierQuota int
	LowTierQuota      int
}

// DefaultConfig returns default quota configuration
func DefaultConfig() Config {
	return Config{
		GlobalDailyQuota:  10000,
		HighTierQuota:     200,
		StandardTierQuota: 100,
		LowTierQuota:      50,
	}
}

// NewPerChannelTracker creates a new per-channel quota tracker
func NewPerChannelTracker(
	db *pgxpool.Pool,
	redis *redis.Client,
	logger *zap.Logger,
	config Config,
) *PerChannelTracker {
	return &PerChannelTracker{
		db:                db,
		redis:             redis,
		logger:            logger,
		globalLimit:       config.GlobalDailyQuota,
		highTierQuota:     config.HighTierQuota,
		standardTierQuota: config.StandardTierQuota,
		lowTierQuota:      config.LowTierQuota,
	}
}

// CanUseQuota checks if a channel can use the specified quota units
func (t *PerChannelTracker) CanUseQuota(ctx context.Context, channelID string, units int) (bool, error) {
	quota, err := t.GetChannelQuota(ctx, channelID)
	if err != nil {
		t.logger.Error("Failed to get channel quota",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		return false, err
	}

	// Check if quota reset is needed
	if time.Now().After(quota.QuotaResetAt) {
		if err := t.ResetChannelQuota(ctx, channelID); err != nil {
			t.logger.Error("Failed to reset channel quota",
				zap.String("channel_id", channelID),
				zap.Error(err),
			)
		}
		quota.DailyQuotaUsed = 0
	}

	// Check channel-specific limit
	remaining := quota.DailyQuotaLimit - quota.DailyQuotaUsed
	if remaining < units {
		t.logger.Warn("Channel quota insufficient",
			zap.String("channel_id", channelID),
			zap.String("tier", quota.PriorityTier),
			zap.Int("remaining", remaining),
			zap.Int("required", units),
		)
		return false, nil
	}

	// Check global limit
	globalUsed, err := t.GetGlobalUsage(ctx)
	if err != nil {
		t.logger.Error("Failed to get global usage", zap.Error(err))
		// Continue anyway - don't block on global check
	} else if globalUsed+units > t.globalLimit {
		t.logger.Warn("Global quota near limit",
			zap.Int("global_used", globalUsed),
			zap.Int("global_limit", t.globalLimit),
		)
		// Apply degradation: only allow high-tier channels
		if quota.PriorityTier != "high" {
			return false, nil
		}
	}

	return true, nil
}

// RecordUsage records quota usage for a channel
func (t *PerChannelTracker) RecordUsage(ctx context.Context, channelID string, units int) error {
	_, err := t.db.Exec(ctx,
		`SELECT record_youtube_quota_usage($1, $2)`,
		channelID, units,
	)
	if err != nil {
		t.logger.Error("Failed to record quota usage",
			zap.String("channel_id", channelID),
			zap.Int("units", units),
			zap.Error(err),
		)
		return fmt.Errorf("failed to record quota usage: %w", err)
	}

	t.logger.Debug("Recorded quota usage",
		zap.String("channel_id", channelID),
		zap.Int("units", units),
	)

	return nil
}

// GetChannelQuota retrieves quota information for a specific channel
func (t *PerChannelTracker) GetChannelQuota(ctx context.Context, channelID string) (*ChannelQuota, error) {
	var quota ChannelQuota
	err := t.db.QueryRow(ctx, `
		SELECT
			channel_id,
			user_id,
			daily_quota_used,
			daily_quota_limit,
			quota_reset_at,
			priority_tier,
			last_seen_live_at,
			total_streams_detected,
			consecutive_offline_checks,
			consecutive_status_check_failures,
			last_status_check_at,
			last_full_detection_at,
			cached_video_id,
			cached_video_title,
			created_at,
			updated_at
		FROM youtube_channel_quota
		WHERE channel_id = $1
	`, channelID).Scan(
		&quota.ChannelID,
		&quota.UserID,
		&quota.DailyQuotaUsed,
		&quota.DailyQuotaLimit,
		&quota.QuotaResetAt,
		&quota.PriorityTier,
		&quota.LastSeenLiveAt,
		&quota.TotalStreamsDetected,
		&quota.ConsecutiveOfflineChecks,
		&quota.ConsecutiveStatusCheckFailures,
		&quota.LastStatusCheckAt,
		&quota.LastFullDetectionAt,
		&quota.CachedVideoID,
		&quota.CachedVideoTitle,
		&quota.CreatedAt,
		&quota.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel quota: %w", err)
	}

	return &quota, nil
}

// UpdateChannelQuota updates quota information for a channel
func (t *PerChannelTracker) UpdateChannelQuota(ctx context.Context, quota *ChannelQuota) error {
	_, err := t.db.Exec(ctx, `
		UPDATE youtube_channel_quota
		SET
			daily_quota_used = $2,
			priority_tier = $3,
			last_seen_live_at = $4,
			total_streams_detected = $5,
			consecutive_offline_checks = $6,
			consecutive_status_check_failures = $7,
			cached_video_id = $8,
			cached_video_title = $9
		WHERE channel_id = $1
	`,
		quota.ChannelID,
		quota.DailyQuotaUsed,
		quota.PriorityTier,
		quota.LastSeenLiveAt,
		quota.TotalStreamsDetected,
		quota.ConsecutiveOfflineChecks,
		quota.ConsecutiveStatusCheckFailures,
		quota.CachedVideoID,
		quota.CachedVideoTitle,
	)
	if err != nil {
		return fmt.Errorf("failed to update channel quota: %w", err)
	}

	return nil
}

// ResetChannelQuota resets daily quota for a channel
func (t *PerChannelTracker) ResetChannelQuota(ctx context.Context, channelID string) error {
	_, err := t.db.Exec(ctx, `
		UPDATE youtube_channel_quota
		SET
			daily_quota_used = 0,
			quota_reset_at = NOW() + INTERVAL '1 day'
		WHERE channel_id = $1
	`, channelID)

	if err != nil {
		return fmt.Errorf("failed to reset channel quota: %w", err)
	}

	t.logger.Info("Reset channel quota",
		zap.String("channel_id", channelID),
	)

	return nil
}

// ResetDailyQuotas resets all daily quotas (called at midnight UTC)
func (t *PerChannelTracker) ResetDailyQuotas(ctx context.Context) error {
	var resetCount int
	err := t.db.QueryRow(ctx, `SELECT reset_youtube_daily_quotas()`).Scan(&resetCount)
	if err != nil {
		return fmt.Errorf("failed to reset daily quotas: %w", err)
	}

	t.logger.Info("Reset daily quotas",
		zap.Int("count", resetCount),
	)

	return nil
}

// PromoteChannelTier promotes a channel to high tier (called when stream goes live)
func (t *PerChannelTracker) PromoteChannelTier(ctx context.Context, channelID string) error {
	_, err := t.db.Exec(ctx, `SELECT promote_youtube_channel_tier($1)`, channelID)
	if err != nil {
		return fmt.Errorf("failed to promote channel tier: %w", err)
	}

	t.logger.Info("Promoted channel to high tier",
		zap.String("channel_id", channelID),
	)

	return nil
}

// DemoteInactiveChannels demotes channels based on inactivity
func (t *PerChannelTracker) DemoteInactiveChannels(ctx context.Context) error {
	var demotedCount int
	err := t.db.QueryRow(ctx, `SELECT demote_inactive_youtube_channels()`).Scan(&demotedCount)
	if err != nil {
		return fmt.Errorf("failed to demote inactive channels: %w", err)
	}

	if demotedCount > 0 {
		t.logger.Info("Demoted inactive channels",
			zap.Int("count", demotedCount),
		)
	}

	return nil
}

// GetGlobalUsage returns total quota used across all channels today
func (t *PerChannelTracker) GetGlobalUsage(ctx context.Context) (int, error) {
	var totalUsed int
	err := t.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(daily_quota_used), 0)
		FROM youtube_channel_quota
		WHERE quota_reset_at > NOW()
	`).Scan(&totalUsed)

	if err != nil {
		return 0, fmt.Errorf("failed to get global usage: %w", err)
	}

	return totalUsed, nil
}

// GetChannelsByTier returns all channels in a specific tier
func (t *PerChannelTracker) GetChannelsByTier(ctx context.Context, tier string) ([]string, error) {
	rows, err := t.db.Query(ctx, `
		SELECT channel_id
		FROM youtube_channel_quota
		WHERE priority_tier = $1
		ORDER BY last_seen_live_at DESC NULLS LAST
	`, tier)
	if err != nil {
		return nil, fmt.Errorf("failed to get channels by tier: %w", err)
	}
	defer rows.Close()

	var channels []string
	for rows.Next() {
		var channelID string
		if err := rows.Scan(&channelID); err != nil {
			return nil, fmt.Errorf("failed to scan channel ID: %w", err)
		}
		channels = append(channels, channelID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating channels: %w", err)
	}

	return channels, nil
}

// RecordStatusCheckFailure increments the status check failure counter
func (t *PerChannelTracker) RecordStatusCheckFailure(ctx context.Context, channelID string) error {
	_, err := t.db.Exec(ctx, `
		UPDATE youtube_channel_quota
		SET consecutive_status_check_failures = consecutive_status_check_failures + 1
		WHERE channel_id = $1
	`, channelID)

	if err != nil {
		return fmt.Errorf("failed to record status check failure: %w", err)
	}

	return nil
}

// ResetStatusCheckFailures resets the status check failure counter
func (t *PerChannelTracker) ResetStatusCheckFailures(ctx context.Context, channelID string) error {
	_, err := t.db.Exec(ctx, `
		UPDATE youtube_channel_quota
		SET consecutive_status_check_failures = 0
		WHERE channel_id = $1
	`, channelID)

	if err != nil {
		return fmt.Errorf("failed to reset status check failures: %w", err)
	}

	return nil
}

// UpdateCachedVideoID updates the cached video ID for a channel
func (t *PerChannelTracker) UpdateCachedVideoID(ctx context.Context, channelID, videoID, videoTitle string) error {
	_, err := t.db.Exec(ctx, `
		UPDATE youtube_channel_quota
		SET
			cached_video_id = $2,
			cached_video_title = $3,
			consecutive_offline_checks = 0
		WHERE channel_id = $1
	`, channelID, videoID, videoTitle)

	if err != nil {
		return fmt.Errorf("failed to update cached video ID: %w", err)
	}

	return nil
}

// ClearCachedVideoID clears the cached video ID for a channel
func (t *PerChannelTracker) ClearCachedVideoID(ctx context.Context, channelID string) error {
	_, err := t.db.Exec(ctx, `
		UPDATE youtube_channel_quota
		SET
			cached_video_id = NULL,
			cached_video_title = NULL
		WHERE channel_id = $1
	`, channelID)

	if err != nil {
		return fmt.Errorf("failed to clear cached video ID: %w", err)
	}

	return nil
}

// RecordOfflineCheck increments the consecutive offline check counter
func (t *PerChannelTracker) RecordOfflineCheck(ctx context.Context, channelID string) error {
	_, err := t.db.Exec(ctx, `
		UPDATE youtube_channel_quota
		SET consecutive_offline_checks = consecutive_offline_checks + 1
		WHERE channel_id = $1
	`, channelID)

	if err != nil {
		return fmt.Errorf("failed to record offline check: %w", err)
	}

	// Clear cached video ID after 3 consecutive offline checks
	quota, err := t.GetChannelQuota(ctx, channelID)
	if err == nil && quota.ConsecutiveOfflineChecks >= 3 && quota.CachedVideoID != nil {
		if err := t.ClearCachedVideoID(ctx, channelID); err != nil {
			t.logger.Warn("Failed to clear cached video ID",
				zap.String("channel_id", channelID),
				zap.Error(err),
			)
		}
	}

	return nil
}

// GetTierQuotaLimit returns the quota limit for a specific tier
func (t *PerChannelTracker) GetTierQuotaLimit(tier string) int {
	switch tier {
	case "high":
		return t.highTierQuota
	case "low":
		return t.lowTierQuota
	default:
		return t.standardTierQuota
	}
}

// EnsureChannelExists ensures a channel has a quota record
func (t *PerChannelTracker) EnsureChannelExists(ctx context.Context, channelID, userID string) error {
	_, err := t.db.Exec(ctx, `
		INSERT INTO youtube_channel_quota (channel_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (channel_id) DO NOTHING
	`, channelID, userID)

	if err != nil {
		return fmt.Errorf("failed to ensure channel exists: %w", err)
	}

	return nil
}
