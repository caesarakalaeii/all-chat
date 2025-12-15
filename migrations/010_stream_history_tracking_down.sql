-- Migration 010 Rollback: Remove stream history tracking
-- This rollback safely removes the stream history table and functions

BEGIN;

-- Drop functions
DROP FUNCTION IF EXISTS get_likely_live_channels(VARCHAR);
DROP FUNCTION IF EXISTS analyze_streaming_patterns();
DROP FUNCTION IF EXISTS update_stream_history_on_detection(VARCHAR, VARCHAR, VARCHAR, BOOLEAN);

-- Drop indexes (will be dropped automatically with table, but explicit for clarity)
DROP INDEX IF EXISTS idx_stream_history_consecutive_offline;
DROP INDEX IF EXISTS idx_stream_history_platform;
DROP INDEX IF EXISTS idx_stream_history_last_check;
DROP INDEX IF EXISTS idx_stream_history_last_seen_live;
DROP INDEX IF EXISTS idx_stream_history_platform_channel;

-- Drop trigger
DROP TRIGGER IF EXISTS update_stream_history_updated_at ON stream_history;

-- Drop table
DROP TABLE IF EXISTS stream_history;

COMMIT;
