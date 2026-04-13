-- All-Chat Expired OAuth Token Cleanup
-- Migration: 047
-- Description: Automated cleanup of expired OAuth tokens across all platform
--              token tables. DSGVO data-minimisation: credentials past their
--              expiry serve no purpose and should be deleted.

-- Function to delete expired OAuth tokens
CREATE OR REPLACE FUNCTION cleanup_expired_oauth_tokens() RETURNS void AS $$
BEGIN
    -- YouTube tokens
    DELETE FROM youtube_oauth_tokens
    WHERE expiry IS NOT NULL AND expiry < NOW() - INTERVAL '7 days';

    -- TikTok tokens
    DELETE FROM tiktok_oauth_tokens
    WHERE expiry IS NOT NULL AND expiry < NOW() - INTERVAL '7 days';

    -- Kick tokens
    DELETE FROM kick_oauth_tokens
    WHERE expiry IS NOT NULL AND expiry < NOW() - INTERVAL '7 days';

    -- Viewer sessions with expired tokens (not refreshed for 7+ days)
    DELETE FROM viewer_sessions
    WHERE token_expires_at < NOW() - INTERVAL '7 days';
END;
$$ LANGUAGE plpgsql;

-- Schedule daily cleanup at 03:00 UTC (requires pg_cron)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_cron') THEN
        PERFORM cron.schedule(
            'cleanup_expired_tokens',
            '0 3 * * *',
            'SELECT cleanup_expired_oauth_tokens()'
        );
    END IF;
END $$;

COMMENT ON FUNCTION cleanup_expired_oauth_tokens() IS
    'DSGVO data minimisation: Removes OAuth tokens that expired 7+ days ago.';
