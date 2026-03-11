-- Migration 026: Backfill missing YouTube channel quota records
-- Fixes data inconsistency where channels lack quota records
--
-- Context: Some YouTube channels were added when their overlay was inactive
-- or after migration 007 ran, causing them to miss the initial quota record
-- creation. This migration ensures ALL YouTube channels have quota records.

BEGIN;

-- Insert missing quota records for all YouTube sources
-- Uses DISTINCT ON to handle multiple overlays using the same channel
INSERT INTO youtube_channel_quota (channel_id, user_id, priority_tier)
SELECT DISTINCT ON (ocs.channel_id)
    ocs.channel_id,
    o.user_id,
    'standard' AS priority_tier
FROM overlay_chat_sources ocs
JOIN overlays o ON ocs.overlay_id = o.id
WHERE ocs.platform = 'youtube'
  AND NOT EXISTS (
      SELECT 1 FROM youtube_channel_quota ycq
      WHERE ycq.channel_id = ocs.channel_id
  )
ORDER BY ocs.channel_id, o.created_at ASC  -- Use oldest user_id if multiple overlays
ON CONFLICT (channel_id) DO NOTHING;

-- Log the results for visibility
DO $$
DECLARE
    inserted_count INT;
BEGIN
    GET DIAGNOSTICS inserted_count = ROW_COUNT;
    RAISE NOTICE 'Backfilled % missing YouTube channel quota records', inserted_count;
END $$;

COMMIT;
