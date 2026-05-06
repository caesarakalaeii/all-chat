-- Add channel_handle field to overlay_chat_sources
ALTER TABLE overlay_chat_sources
ADD COLUMN IF NOT EXISTS channel_handle VARCHAR(255);

-- Create index for efficient handle lookups
CREATE INDEX IF NOT EXISTS idx_overlay_chat_sources_channel_handle
ON overlay_chat_sources(LOWER(channel_handle));

-- Backfill existing data:
-- For Twitch/Kick: handle = channel_id (username)
-- For YouTube: handle will be NULL initially (to be populated when users re-add sources)
--
-- IDEMPOTENCY: every service runs the full migration set on each pod restart, so
-- this UPDATE must be a no-op once backfilled. The IS DISTINCT FROM guard skips
-- rows whose handle already matches; without it, the BEFORE-UPDATE trigger
-- (update_overlay_chat_sources_updated_at) bumps updated_at on every row each
-- pod restart, defeating the source-cleanup job's 24h staleness threshold.
UPDATE overlay_chat_sources
SET channel_handle = channel_id
WHERE platform IN ('twitch', 'kick', 'tiktok')
  AND channel_handle IS DISTINCT FROM channel_id;

-- For YouTube sources, we can't backfill without calling YouTube API
-- These will remain NULL until users re-authenticate or we run a separate backfill script
