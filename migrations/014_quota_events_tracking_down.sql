-- Migration Rollback: Quota Events Tracking
-- Purpose: Remove quota events tracking table and indexes
-- Created: 2026-01-08

-- Drop indexes first
DROP INDEX IF EXISTS idx_youtube_quota_events_state_time;
DROP INDEX IF EXISTS idx_youtube_quota_events_severity;
DROP INDEX IF EXISTS idx_youtube_quota_events_created_at;
DROP INDEX IF EXISTS idx_youtube_quota_events_state;
DROP INDEX IF EXISTS idx_youtube_quota_events_type;

-- Drop the quota events table
DROP TABLE IF EXISTS youtube_quota_events;
