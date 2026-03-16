-- 040_viewer_sessions_user_link.sql
-- Phase 31 fix: Add user_id FK to viewer_sessions so ViewerBadgeEnricher can resolve
-- All-Chat admin/premium status for viewers who also have streamer accounts.
-- Also adds enable_twitch_follows to overlay_event_settings for Twitch EventSub follow events.

ALTER TABLE viewer_sessions
    ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_viewer_sessions_user_id ON viewer_sessions(user_id);

ALTER TABLE overlay_event_settings
    ADD COLUMN IF NOT EXISTS enable_twitch_follows BOOLEAN NOT NULL DEFAULT TRUE;
