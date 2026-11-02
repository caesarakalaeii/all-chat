-- Migration: 118_retention_sweeper
-- Description: Retention is enforced by the retention-sweeper CronJob (ADR-0087), so the
--   cleanup functions that were only ever scheduled through pg_cron are dropped.
--
-- pg_cron was never installed on the cluster (allchat-cluster creates only uuid-ossp and
-- pg_trgm), so the `IF EXISTS pg_cron` blocks in 046 and 047 never scheduled anything and
-- nothing else called these functions: the retention periods they implement were never
-- applied. cleanup_old_youtube_audit_logs (016) could not even run (RETURNING COUNT(*) is an
-- error). The sweeper now owns every one of these periods.
--
-- Corrected forward rather than by editing 016/046/047/056/063: the runner replays every
-- migration on every pod start, so those files recreate the functions on each run and this
-- one drops them again. Idempotent throughout.

BEGIN;

-- Defensive: if pg_cron is ever installed, 046/047 schedule their jobs on the next run.
-- Unschedule them so they never call a function that no longer exists.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_cron') THEN
        PERFORM cron.unschedule(jobid)
           FROM cron.job
          WHERE jobname IN ('cleanup_message_history', 'cleanup_expired_tokens');
    END IF;
END $$;

DROP FUNCTION IF EXISTS cleanup_old_message_history();
DROP FUNCTION IF EXISTS cleanup_expired_oauth_tokens();
DROP FUNCTION IF EXISTS cleanup_old_youtube_audit_logs();

-- The sweeper deletes by age in batches. Without an index on the age column each batch
-- would scan the whole table. viewer_message_history (046), youtube_quota_audit_log (016)
-- and device_link_requests (088) already have one.
CREATE INDEX IF NOT EXISTS idx_alert_events_created_at
    ON alert_events (created_at);
CREATE INDEX IF NOT EXISTS idx_moderation_actions_created_at
    ON moderation_actions (created_at);

COMMIT;
