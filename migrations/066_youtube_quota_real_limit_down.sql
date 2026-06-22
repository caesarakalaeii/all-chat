-- Revert migration 066: restore the legacy 10,000 default and no-row fallback.
ALTER TABLE youtube_quota_usage ALTER COLUMN units_limit SET DEFAULT 10000;

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

    IF NOT FOUND THEN
        RETURN QUERY SELECT 0, 0, 10000, 10000, 0.0::NUMERIC;
    END IF;
END;
$$ LANGUAGE plpgsql;
