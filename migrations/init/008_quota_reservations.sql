-- Migration: Add YouTube quota reservation system
-- Purpose: Atomic reserve-confirm-rollback pattern to eliminate quota tracking drift
-- Date: 2026-01-09

-- Add units_reserved column to track in-flight API calls
ALTER TABLE youtube_quota_usage
ADD COLUMN IF NOT EXISTS units_reserved INT NOT NULL DEFAULT 0;

-- Create index for date queries
CREATE INDEX IF NOT EXISTS idx_youtube_quota_usage_date ON youtube_quota_usage(date);

-- ============================================================================
-- FUNCTION: reserve_youtube_quota
-- Purpose: Atomically reserves quota BEFORE making YouTube API call
-- Returns: TRUE if reservation successful, FALSE if insufficient quota
-- ============================================================================
CREATE OR REPLACE FUNCTION reserve_youtube_quota(
    p_date DATE,
    p_units INT,
    p_limit INT
) RETURNS BOOLEAN AS $$
DECLARE
    v_used INT;
    v_reserved INT;
BEGIN
    -- Lock row for this date (prevents race conditions)
    SELECT units_used, units_reserved
    INTO v_used, v_reserved
    FROM youtube_quota_usage
    WHERE date = p_date
    FOR UPDATE;

    -- Create row if it doesn't exist (first request of the day)
    IF NOT FOUND THEN
        INSERT INTO youtube_quota_usage (date, units_used, units_reserved, units_limit)
        VALUES (p_date, 0, p_units, p_limit);
        RETURN TRUE;
    END IF;

    -- Check if we have enough quota (used + reserved + requested <= limit)
    IF v_used + v_reserved + p_units > p_limit THEN
        RETURN FALSE;  -- Insufficient quota
    END IF;

    -- Reserve the quota
    UPDATE youtube_quota_usage
    SET units_reserved = units_reserved + p_units,
        updated_at = NOW()
    WHERE date = p_date;

    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- FUNCTION: confirm_youtube_quota
-- Purpose: Confirms reservation after successful YouTube API call
-- Moves units from reserved -> used
-- ============================================================================
CREATE OR REPLACE FUNCTION confirm_youtube_quota(
    p_date DATE,
    p_units INT
) RETURNS VOID AS $$
BEGIN
    UPDATE youtube_quota_usage
    SET units_used = units_used + p_units,
        units_reserved = GREATEST(0, units_reserved - p_units),
        updated_at = NOW()
    WHERE date = p_date;

    -- Log if row doesn't exist (should never happen)
    IF NOT FOUND THEN
        RAISE WARNING 'confirm_youtube_quota: No row found for date %', p_date;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- FUNCTION: rollback_youtube_quota
-- Purpose: Rollbacks reservation after failed YouTube API call
-- Releases reserved units back to available pool
-- ============================================================================
CREATE OR REPLACE FUNCTION rollback_youtube_quota(
    p_date DATE,
    p_units INT
) RETURNS VOID AS $$
BEGIN
    UPDATE youtube_quota_usage
    SET units_reserved = GREATEST(0, units_reserved - p_units),
        updated_at = NOW()
    WHERE date = p_date;

    -- Log if row doesn't exist (should never happen)
    IF NOT FOUND THEN
        RAISE WARNING 'rollback_youtube_quota: No row found for date %', p_date;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- FUNCTION: cleanup_stale_quota_reservations
-- Purpose: Cleans up stale reservations (>5 minutes old)
-- Called periodically to recover from crashed processes
-- Returns: Number of units recovered
-- ============================================================================
CREATE OR REPLACE FUNCTION cleanup_stale_quota_reservations()
RETURNS INT AS $$
DECLARE
    cleanup_count INT := 0;
    v_reserved INT;
BEGIN
    -- Only clean up reservations for past dates (today's are still valid)
    SELECT COALESCE(SUM(units_reserved), 0)
    INTO v_reserved
    FROM youtube_quota_usage
    WHERE units_reserved > 0
      AND updated_at < NOW() - INTERVAL '5 minutes'
      AND date < CURRENT_DATE;

    IF v_reserved > 0 THEN
        UPDATE youtube_quota_usage
        SET units_reserved = 0,
            updated_at = NOW()
        WHERE units_reserved > 0
          AND updated_at < NOW() - INTERVAL '5 minutes'
          AND date < CURRENT_DATE;

        cleanup_count := v_reserved;

        RAISE NOTICE 'Cleaned up % stale reserved quota units', cleanup_count;
    END IF;

    RETURN cleanup_count;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- FUNCTION: get_youtube_quota_with_reserved
-- Purpose: Returns quota status including reserved units
-- Used for monitoring and diagnostics
-- ============================================================================
CREATE OR REPLACE FUNCTION get_youtube_quota_with_reserved(p_date DATE)
RETURNS TABLE (
    used INT,
    reserved INT,
    available INT,
    limit_val INT,
    percentage NUMERIC
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        units_used,
        units_reserved,
        (units_limit - units_used - units_reserved) AS available,
        units_limit,
        ROUND((units_used + units_reserved)::NUMERIC / units_limit::NUMERIC * 100, 2) AS percentage
    FROM youtube_quota_usage
    WHERE date = p_date;

    -- Return zeros if no row exists
    IF NOT FOUND THEN
        RETURN QUERY SELECT 0, 0, 10000, 10000, 0.0::NUMERIC;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- Add comment explaining the reservation system
COMMENT ON COLUMN youtube_quota_usage.units_reserved IS 'Quota units reserved for in-flight API calls. Prevents over-consumption via atomic reserve-confirm-rollback pattern.';
COMMENT ON FUNCTION reserve_youtube_quota IS 'Atomically reserves quota before YouTube API call. Returns false if insufficient quota available.';
COMMENT ON FUNCTION confirm_youtube_quota IS 'Confirms reservation after successful API call. Moves units from reserved to used.';
COMMENT ON FUNCTION rollback_youtube_quota IS 'Rollbacks reservation after failed API call. Releases reserved units.';
COMMENT ON FUNCTION cleanup_stale_quota_reservations IS 'Cleans up stale reservations (>5 min old) from crashed processes. Returns count of units recovered.';
