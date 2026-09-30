-- Migration: 098_youtube_subscribers_event_setting
-- Description: Add enable_youtube_subscribers column to overlay_event_settings.
--   Free (non-membership) YouTube subscribes were never surfaced before: they only
--   become visible through subscriptions.list?myRecentSubscribers polling, which the
--   listener runs while a channel's stream is live, so there is no payload to
--   backfill — a sub that arrives while the overlay is dark simply never existed.
--   They fire once per new subscriber, so streamers need a toggle to keep them off
--   busy overlays: a free one, like the existing Twitch subs and follows.
--   Defaults to TRUE: the events never reached an overlay before, so enabling them
--   is the fix, not a surprise.
--
-- Idempotent (ADD COLUMN IF NOT EXISTS): the migration runner re-applies every
-- migration on each pod start, so a non-idempotent statement would crash-loop
-- fresh pods.

ALTER TABLE overlay_event_settings
    ADD COLUMN IF NOT EXISTS enable_youtube_subscribers BOOLEAN NOT NULL DEFAULT TRUE;
