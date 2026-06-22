-- Migration 066: report the real YouTube daily-quota limit (1,009,000), not the legacy 10,000.
--
-- The actual granted YouTube Data API quota is 1,009,000 units/day (see CLAUDE.md and
-- shared/quota.DefaultDailyQuota = 1009000). Two places still carried the old 10,000 default:
--   1. youtube_quota_usage.units_limit column DEFAULT (migration 003), and
--   2. get_youtube_quota_with_reserved()'s "no row yet today" fallback (migration 008),
--      which the youtube-quota-monitor returns verbatim on /quota/status — so on any day
--      before the first quota-spending call (no row exists yet) it reported "0 / 10000",
--      which the discord-bot then posted.
--
-- Existing rows already store 1,009,000 (the reserve path passes the configured limit), so
-- no data backfill is needed. This migration must sort AFTER 008 because the runner re-runs
-- every migration on each pod start; 008 CREATE OR REPLACEs the function with the 10,000
-- fallback, and this file then overwrites it with the correct value. Idempotent.

ALTER TABLE youtube_quota_usage ALTER COLUMN units_limit SET DEFAULT 1009000;

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

    -- No row yet today: report zero usage against the real daily quota.
    IF NOT FOUND THEN
        RETURN QUERY SELECT 0, 0, 1009000, 1009000, 0.0::NUMERIC;
    END IF;
END;
$$ LANGUAGE plpgsql;
