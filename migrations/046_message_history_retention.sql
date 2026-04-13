-- All-Chat Message History Retention Policy
-- Migration: 046
-- Description: Add automated cleanup of viewer_message_history rows older than
--              1 hour, per DSGVO storage-limitation principle (Art. 5(1)(e)).
--              Uses the same pg_cron pattern as the YouTube quota audit log cleanup.

-- Function to purge old message history
CREATE OR REPLACE FUNCTION cleanup_old_message_history() RETURNS void AS $$
BEGIN
    DELETE FROM viewer_message_history
    WHERE created_at < NOW() - INTERVAL '1 hour';
END;
$$ LANGUAGE plpgsql;

-- Schedule the cleanup to run every 15 minutes (requires pg_cron extension).
-- If pg_cron is not available the function can be called from a Go cron job instead.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_cron') THEN
        PERFORM cron.schedule(
            'cleanup_message_history',
            '*/15 * * * *',
            'SELECT cleanup_old_message_history()'
        );
    END IF;
END $$;

-- Add an index on created_at to speed up the retention delete
CREATE INDEX IF NOT EXISTS idx_viewer_message_history_created_at
    ON viewer_message_history(created_at);

COMMENT ON FUNCTION cleanup_old_message_history() IS
    'DSGVO Art.5(1)(e): Deletes viewer message history older than 1 hour.';
