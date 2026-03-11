-- Migration 032 rollback: Remove shared_overlay platform and recipient_overlay_id column

-- 1. Remove recipient_overlay_id column
ALTER TABLE share_requests
    DROP COLUMN IF EXISTS recipient_overlay_id;

-- 2. Remove shared_overlay from supported_platforms
--    Note: will fail if any overlay_chat_sources rows reference this platform (ON DELETE RESTRICT behavior)
DELETE FROM supported_platforms WHERE platform = 'shared_overlay';
