-- All-Chat Migration 056 DOWN: remove per-link Twitch OAuth credentials

BEGIN;

DROP TABLE IF EXISTS twitch_oauth_tokens;

-- Restore the 047 version of the cleanup function (without the twitch table).
CREATE OR REPLACE FUNCTION cleanup_expired_oauth_tokens() RETURNS void AS $$
BEGIN
    DELETE FROM youtube_oauth_tokens
    WHERE expiry IS NOT NULL AND expiry < NOW() - INTERVAL '7 days';

    DELETE FROM tiktok_oauth_tokens
    WHERE expiry IS NOT NULL AND expiry < NOW() - INTERVAL '7 days';

    DELETE FROM kick_oauth_tokens
    WHERE expiry IS NOT NULL AND expiry < NOW() - INTERVAL '7 days';

    DELETE FROM viewer_sessions
    WHERE token_expires_at < NOW() - INTERVAL '7 days';
END;
$$ LANGUAGE plpgsql;

COMMIT;
