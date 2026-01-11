-- Rollback: Remove YouTube API audit logging

DROP FUNCTION IF EXISTS reconcile_youtube_quota_usage(DATE);
DROP FUNCTION IF EXISTS cleanup_old_youtube_audit_logs();

DROP TABLE IF EXISTS youtube_quota_reconciliation;
DROP TABLE IF EXISTS youtube_quota_audit_log;
