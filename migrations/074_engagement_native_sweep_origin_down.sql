-- 074_engagement_native_sweep_origin_down.sql
-- Reverses 074: drops the sweep-origin marker column on mirrored Twitch predictions.
ALTER TABLE predictions DROP COLUMN IF EXISTS sweep_canceled;
