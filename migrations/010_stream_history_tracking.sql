-- Migration 010: Stream history tracking for all platforms
-- Purpose: Track streaming patterns to optimize live detection across YouTube, TikTok, Kick, and Twitch
-- This enables smart polling based on historical activity

BEGIN;

-- Create stream_history table (platform-agnostic)
CREATE TABLE IF NOT EXISTS stream_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Platform and channel identification
    platform VARCHAR(50) NOT NULL,
    channel_id VARCHAR(100) NOT NULL,
    channel_name VARCHAR(100) NOT NULL,

    -- Live detection tracking
    last_seen_live TIMESTAMP,              -- When we last detected stream was live
    last_seen_offline TIMESTAMP,           -- When we last confirmed stream was offline
    last_check_time TIMESTAMP,             -- Last time we checked (regardless of result)
    consecutive_offline_checks INTEGER DEFAULT 0,

    -- Stream frequency tracking (for smart polling predictions)
    total_streams_detected INTEGER DEFAULT 0,
    avg_stream_duration_minutes INTEGER,   -- Historical average duration
    typical_stream_days INTEGER[],         -- Array of weekday numbers [0=Sun, 1=Mon, ..., 6=Sat]
    typical_stream_hours INTEGER[],        -- Array of hour numbers [0-23] in UTC

    -- Additional metadata
    notes TEXT,                            -- Admin notes or special handling instructions

    -- Timestamps
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    -- Unique constraint: one record per platform-channel combination
    UNIQUE(platform, channel_id)
);

-- Indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_stream_history_platform_channel ON stream_history(platform, channel_id);
CREATE INDEX IF NOT EXISTS idx_stream_history_last_seen_live ON stream_history(last_seen_live)
    WHERE last_seen_live IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_stream_history_last_check ON stream_history(last_check_time);
CREATE INDEX IF NOT EXISTS idx_stream_history_platform ON stream_history(platform);
CREATE INDEX IF NOT EXISTS idx_stream_history_consecutive_offline ON stream_history(consecutive_offline_checks)
    WHERE consecutive_offline_checks > 0;

-- Trigger for updated_at timestamp
DROP TRIGGER IF EXISTS update_stream_history_updated_at ON stream_history;
CREATE TRIGGER update_stream_history_updated_at
    BEFORE UPDATE ON stream_history
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Function to update stream history after live detection
-- This is called by listener services when they detect live status changes
CREATE OR REPLACE FUNCTION update_stream_history_on_detection(
    p_platform VARCHAR(50),
    p_channel_id VARCHAR(100),
    p_channel_name VARCHAR(100),
    p_is_live BOOLEAN
) RETURNS VOID AS $$
DECLARE
    v_last_live TIMESTAMP;
    v_stream_duration_minutes INTEGER;
BEGIN
    -- Insert or update stream history
    INSERT INTO stream_history (
        platform,
        channel_id,
        channel_name,
        last_seen_live,
        last_seen_offline,
        last_check_time,
        consecutive_offline_checks,
        total_streams_detected
    ) VALUES (
        p_platform,
        p_channel_id,
        p_channel_name,
        CASE WHEN p_is_live THEN NOW() ELSE NULL END,
        CASE WHEN NOT p_is_live THEN NOW() ELSE NULL END,
        NOW(),
        CASE WHEN NOT p_is_live THEN 1 ELSE 0 END,
        CASE WHEN p_is_live THEN 1 ELSE 0 END
    )
    ON CONFLICT (platform, channel_id)
    DO UPDATE SET
        -- Update live timestamp if currently live
        last_seen_live = CASE
            WHEN p_is_live THEN NOW()
            ELSE stream_history.last_seen_live
        END,

        -- Update offline timestamp if currently offline
        last_seen_offline = CASE
            WHEN NOT p_is_live THEN NOW()
            ELSE stream_history.last_seen_offline
        END,

        -- Always update last check time
        last_check_time = NOW(),

        -- Reset or increment offline counter
        consecutive_offline_checks = CASE
            WHEN p_is_live THEN 0
            ELSE stream_history.consecutive_offline_checks + 1
        END,

        -- Increment stream count if this is a new stream session
        -- (live now, and either never live before OR >30 minutes since last seen live)
        total_streams_detected = CASE
            WHEN p_is_live AND stream_history.last_seen_live IS NULL THEN
                stream_history.total_streams_detected + 1
            WHEN p_is_live AND (NOW() - stream_history.last_seen_live) > INTERVAL '30 minutes' THEN
                stream_history.total_streams_detected + 1
            ELSE
                stream_history.total_streams_detected
        END,

        -- Update channel name in case it changed
        channel_name = p_channel_name,

        -- Trigger will handle updated_at
        updated_at = NOW()
    RETURNING last_seen_live INTO v_last_live;

    -- Calculate average stream duration if stream just ended
    IF NOT p_is_live AND v_last_live IS NOT NULL THEN
        v_stream_duration_minutes := EXTRACT(EPOCH FROM (NOW() - v_last_live)) / 60;

        -- Update average duration (simple moving average)
        UPDATE stream_history
        SET avg_stream_duration_minutes = CASE
            WHEN avg_stream_duration_minutes IS NULL THEN v_stream_duration_minutes
            ELSE (avg_stream_duration_minutes + v_stream_duration_minutes) / 2
        END
        WHERE platform = p_platform AND channel_id = p_channel_id;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- Function to analyze streaming patterns and update typical times
-- This can be run periodically (e.g., weekly) to update pattern predictions
CREATE OR REPLACE FUNCTION analyze_streaming_patterns()
RETURNS INT AS $$
DECLARE
    pattern_count INT := 0;
BEGIN
    -- This is a placeholder for future pattern analysis
    -- In a full implementation, this would:
    -- 1. Query stream event logs (not implemented yet)
    -- 2. Analyze when streams typically occur
    -- 3. Update typical_stream_days and typical_stream_hours arrays

    -- For now, just return 0
    RETURN pattern_count;
END;
$$ LANGUAGE plpgsql;

-- Function to get channels that are likely to be live based on patterns
-- Returns channels that typically stream at the current day/hour
CREATE OR REPLACE FUNCTION get_likely_live_channels(
    p_platform VARCHAR(50)
)
RETURNS TABLE (
    channel_id VARCHAR(100),
    channel_name VARCHAR(100),
    last_seen_live TIMESTAMP,
    total_streams_detected INTEGER
) AS $$
DECLARE
    v_current_day INTEGER;
    v_current_hour INTEGER;
BEGIN
    -- Get current day of week (0 = Sunday) and hour (UTC)
    v_current_day := EXTRACT(DOW FROM NOW());
    v_current_hour := EXTRACT(HOUR FROM NOW());

    -- Return channels that:
    -- 1. Match the platform
    -- 2. Have been seen live before
    -- 3. Typically stream on this day/hour (if pattern data exists)
    -- 4. Or haven't been seen live recently (need checking)
    RETURN QUERY
    SELECT
        sh.channel_id,
        sh.channel_name,
        sh.last_seen_live,
        sh.total_streams_detected
    FROM stream_history sh
    WHERE sh.platform = p_platform
      AND (
          -- Has pattern data matching current time
          (v_current_day = ANY(sh.typical_stream_days) AND
           v_current_hour = ANY(sh.typical_stream_hours))
          OR
          -- Or was live recently (within 7 days)
          sh.last_seen_live > NOW() - INTERVAL '7 days'
          OR
          -- Or never seen live (needs first check)
          sh.last_seen_live IS NULL
      )
    ORDER BY
        -- Prioritize by likelihood
        CASE
            WHEN sh.last_seen_live > NOW() - INTERVAL '1 day' THEN 1
            WHEN sh.last_seen_live > NOW() - INTERVAL '7 days' THEN 2
            ELSE 3
        END,
        sh.total_streams_detected DESC;
END;
$$ LANGUAGE plpgsql;

-- Populate initial stream history from existing active sources
INSERT INTO stream_history (platform, channel_id, channel_name)
SELECT DISTINCT
    ocs.platform,
    ocs.channel_id,
    ocs.channel_name
FROM overlay_chat_sources ocs
JOIN overlays o ON ocs.overlay_id = o.id
WHERE o.is_active = true
  AND ocs.platform IN ('youtube', 'tiktok', 'kick', 'twitch')
ON CONFLICT (platform, channel_id) DO NOTHING;

COMMIT;
