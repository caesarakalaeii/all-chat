-- Migration 007: YouTube per-channel quota tracking
-- Purpose: Implement quota management per YouTube channel to prevent exhaustion
-- and enable priority-based detection scheduling

BEGIN;

-- Create youtube_channel_quota table for per-channel quota tracking
CREATE TABLE IF NOT EXISTS youtube_channel_quota (
    channel_id VARCHAR(255) PRIMARY KEY,
    user_id UUID NOT NULL,

    -- Quota tracking
    daily_quota_used INT NOT NULL DEFAULT 0,
    daily_quota_limit INT NOT NULL DEFAULT 100,
    quota_reset_at TIMESTAMP NOT NULL DEFAULT (NOW() + INTERVAL '1 day'),

    -- Priority tier (affects check frequency and quota allocation)
    priority_tier VARCHAR(20) NOT NULL DEFAULT 'standard'
        CHECK (priority_tier IN ('high', 'standard', 'low')),

    -- Stream detection history
    last_seen_live_at TIMESTAMP,
    total_streams_detected INT NOT NULL DEFAULT 0,

    -- Status check tracking
    consecutive_offline_checks INT NOT NULL DEFAULT 0,
    consecutive_status_check_failures INT NOT NULL DEFAULT 0,
    last_status_check_at TIMESTAMP,
    last_full_detection_at TIMESTAMP,

    -- Cached data for lightweight checks
    cached_video_id VARCHAR(255),
    cached_video_title VARCHAR(500),

    -- Metadata
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_youtube_quota_user_id ON youtube_channel_quota(user_id);
CREATE INDEX IF NOT EXISTS idx_youtube_quota_priority ON youtube_channel_quota(priority_tier);
CREATE INDEX IF NOT EXISTS idx_youtube_quota_last_live ON youtube_channel_quota(last_seen_live_at);
CREATE INDEX IF NOT EXISTS idx_youtube_quota_reset ON youtube_channel_quota(quota_reset_at);
CREATE INDEX IF NOT EXISTS idx_youtube_quota_cached_video ON youtube_channel_quota(cached_video_id)
    WHERE cached_video_id IS NOT NULL;

-- Trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_youtube_channel_quota_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS youtube_channel_quota_updated_at_trigger ON youtube_channel_quota;
CREATE TRIGGER youtube_channel_quota_updated_at_trigger
    BEFORE UPDATE ON youtube_channel_quota
    FOR EACH ROW
    EXECUTE FUNCTION update_youtube_channel_quota_updated_at();

-- Function to record quota usage
CREATE OR REPLACE FUNCTION record_youtube_quota_usage(
    p_channel_id VARCHAR(255),
    p_units INT
) RETURNS VOID AS $$
BEGIN
    UPDATE youtube_channel_quota
    SET daily_quota_used = daily_quota_used + p_units,
        last_status_check_at = NOW()
    WHERE channel_id = p_channel_id;

    -- If channel doesn't exist in quota table, insert with usage
    IF NOT FOUND THEN
        INSERT INTO youtube_channel_quota (channel_id, user_id, daily_quota_used, last_status_check_at)
        SELECT DISTINCT channel_id,
                        (SELECT user_id FROM overlays WHERE id = ocs.overlay_id LIMIT 1),
                        p_units,
                        NOW()
        FROM overlay_chat_sources ocs
        WHERE ocs.channel_id = p_channel_id AND ocs.platform = 'youtube'
        LIMIT 1;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- Function to reset daily quotas (called at midnight UTC)
CREATE OR REPLACE FUNCTION reset_youtube_daily_quotas()
RETURNS INT AS $$
DECLARE
    reset_count INT;
BEGIN
    UPDATE youtube_channel_quota
    SET daily_quota_used = 0,
        quota_reset_at = NOW() + INTERVAL '1 day',
        -- Reset high-tier channels' offline counters on daily reset
        consecutive_offline_checks = CASE
            WHEN priority_tier = 'high' THEN 0
            ELSE consecutive_offline_checks
        END
    WHERE quota_reset_at <= NOW();

    GET DIAGNOSTICS reset_count = ROW_COUNT;
    RETURN reset_count;
END;
$$ LANGUAGE plpgsql;

-- Function to promote channel tier (called when stream goes live)
CREATE OR REPLACE FUNCTION promote_youtube_channel_tier(
    p_channel_id VARCHAR(255)
) RETURNS VOID AS $$
BEGIN
    UPDATE youtube_channel_quota
    SET priority_tier = 'high',
        last_seen_live_at = NOW(),
        total_streams_detected = total_streams_detected + 1,
        consecutive_offline_checks = 0,
        consecutive_status_check_failures = 0
    WHERE channel_id = p_channel_id;
END;
$$ LANGUAGE plpgsql;

-- Function to demote channel tiers based on inactivity
CREATE OR REPLACE FUNCTION demote_inactive_youtube_channels()
RETURNS INT AS $$
DECLARE
    demoted_count INT := 0;
    additional_count INT;
BEGIN
    -- Demote high → standard after 24 hours of no activity
    UPDATE youtube_channel_quota
    SET priority_tier = 'standard'
    WHERE priority_tier = 'high'
      AND last_seen_live_at IS NOT NULL
      AND NOW() - last_seen_live_at > INTERVAL '24 hours';

    GET DIAGNOSTICS demoted_count = ROW_COUNT;

    -- Demote standard → low after 7 days of no activity
    UPDATE youtube_channel_quota
    SET priority_tier = 'low'
    WHERE priority_tier = 'standard'
      AND (last_seen_live_at IS NULL OR NOW() - last_seen_live_at > INTERVAL '7 days');

    GET DIAGNOSTICS additional_count = ROW_COUNT;
    demoted_count := demoted_count + additional_count;

    RETURN demoted_count;
END;
$$ LANGUAGE plpgsql;

-- Populate initial quota data from existing active YouTube sources
-- This ensures existing channels have quota entries
INSERT INTO youtube_channel_quota (channel_id, user_id, priority_tier)
SELECT DISTINCT
    ocs.channel_id,
    o.user_id,
    'standard' AS priority_tier
FROM overlay_chat_sources ocs
JOIN overlays o ON ocs.overlay_id = o.id
WHERE ocs.platform = 'youtube'
  AND o.is_active = true
ON CONFLICT (channel_id) DO NOTHING;

COMMIT;
