-- Migration 007 Rollback: Remove YouTube per-channel quota tracking
-- This rollback safely removes the quota tracking table and functions

BEGIN;

-- Drop functions
DROP FUNCTION IF EXISTS demote_inactive_youtube_channels();
DROP FUNCTION IF EXISTS promote_youtube_channel_tier(VARCHAR);
DROP FUNCTION IF EXISTS reset_youtube_daily_quotas();
DROP FUNCTION IF EXISTS record_youtube_quota_usage(VARCHAR, INT);
DROP FUNCTION IF EXISTS update_youtube_channel_quota_updated_at();

-- Drop indexes (will be dropped automatically with table, but explicit for clarity)
DROP INDEX IF EXISTS idx_youtube_quota_cached_video;
DROP INDEX IF EXISTS idx_youtube_quota_reset;
DROP INDEX IF EXISTS idx_youtube_quota_last_live;
DROP INDEX IF EXISTS idx_youtube_quota_priority;
DROP INDEX IF EXISTS idx_youtube_quota_user_id;

-- Drop trigger
DROP TRIGGER IF EXISTS youtube_channel_quota_updated_at_trigger ON youtube_channel_quota;

-- Drop table
DROP TABLE IF EXISTS youtube_channel_quota;

COMMIT;
