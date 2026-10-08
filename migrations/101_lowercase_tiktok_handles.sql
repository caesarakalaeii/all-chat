-- TikTok's live-status and profile lookups are case-sensitive (uniqueId "MrBeast"
-- returns user_not_found, "mrbeast" resolves) and TikTok handles are always
-- lowercase. Sources saved with the casing the streamer typed were therefore never
-- detected as live. overlay-manager lowercases new TikTok handles; this fixes the
-- rows written before that.
--
-- Collision-safe and idempotent, like 020: the runner re-applies every migration on
-- each pod start, and the unique (overlay_id, platform, channel_id) constraint would
-- fail where an overlay already has the lowercase handle. Those rows are skipped and
-- left as a harmless dead duplicate.
UPDATE overlay_chat_sources ocs
SET channel_id = LOWER(ocs.channel_id),
    updated_at = NOW()
WHERE ocs.platform = 'tiktok'
  AND ocs.channel_id <> LOWER(ocs.channel_id)
  AND NOT EXISTS (
      SELECT 1 FROM overlay_chat_sources dup
      WHERE dup.overlay_id = ocs.overlay_id
        AND dup.platform = 'tiktok'
        AND dup.channel_id = LOWER(ocs.channel_id)
        AND dup.id <> ocs.id
  );
