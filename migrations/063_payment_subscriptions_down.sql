-- All-Chat Migration 063 DOWN: remove Patreon premium entitlements

BEGIN;

DROP TABLE IF EXISTS premium_subscriptions;
DROP TABLE IF EXISTS patreon_oauth_tokens;

ALTER TABLE users DROP COLUMN IF EXISTS premium_admin_override;

-- Restore the 056 version of the cleanup function (without the patreon table).
CREATE OR REPLACE FUNCTION cleanup_expired_oauth_tokens() RETURNS void AS $$
BEGIN
    DELETE FROM youtube_oauth_tokens
    WHERE expiry IS NOT NULL AND expiry < NOW() - INTERVAL '7 days';

    DELETE FROM tiktok_oauth_tokens
    WHERE expiry IS NOT NULL AND expiry < NOW() - INTERVAL '7 days';

    DELETE FROM kick_oauth_tokens
    WHERE expiry IS NOT NULL AND expiry < NOW() - INTERVAL '7 days';

    DELETE FROM twitch_oauth_tokens
    WHERE token_expires_at < NOW() - INTERVAL '7 days';

    DELETE FROM viewer_sessions
    WHERE token_expires_at < NOW() - INTERVAL '7 days';
END;
$$ LANGUAGE plpgsql;

COMMIT;
