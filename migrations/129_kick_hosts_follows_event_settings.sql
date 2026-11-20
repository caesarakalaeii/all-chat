-- Kick hosts (Kick's raids) and follows get their own per-overlay toggles,
-- next to enable_kick_subs and enable_kick_gifts (migration 017).
ALTER TABLE overlay_event_settings
    ADD COLUMN IF NOT EXISTS enable_kick_hosts BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE overlay_event_settings
    ADD COLUMN IF NOT EXISTS enable_kick_follows BOOLEAN NOT NULL DEFAULT TRUE;
