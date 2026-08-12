-- Migration: 084_tiktok_treasure_chest_event_setting
-- Description: Add enable_tiktok_treasure_chests column to overlay_event_settings.
--   Coin chests (TikTok's treasure boxes) arrive on the ENVELOPE message, which the
--   listener never subscribed to, so these events were previously never emitted at
--   all. They fire once per viewer drop, so streamers need a toggle to keep them off
--   busy overlays: a free one, like the existing TikTok likes/gifts/follows/shares.
--   Defaults to TRUE: the events never reached an overlay before, so enabling them is
--   the fix, not a surprise.
--
-- Idempotent (ADD COLUMN IF NOT EXISTS): the migration runner re-applies every
-- migration on each pod start, so a non-idempotent statement would crash-loop
-- fresh pods.

ALTER TABLE overlay_event_settings
    ADD COLUMN IF NOT EXISTS enable_tiktok_treasure_chests BOOLEAN NOT NULL DEFAULT TRUE;
