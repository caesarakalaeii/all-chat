-- All-Chat Migration 052: Track per-overlay last-connected timestamp
-- Migration: 052
--
-- Adds overlays.last_connected_at, bumped by api-gateway whenever a frontend
-- WebSocket attaches to an overlay (and on each heartbeat tick while attached).
-- twitch-listener filters its desired-channel query on this timestamp so
-- overlays nobody has watched in N days fall out of the listener's view and
-- get IRC-PARTed automatically. Without this, the bot account stays connected
-- to every twitch channel of every overlay we've ever created and runs into
-- Twitch's per-account concurrent-channel cap (msg_concurrent_channel_limit_reached).
--
-- The column is NOT NULL with default NOW() so existing rows back-fill to the
-- deploy moment — every overlay gets a fresh N-day grace period from rollout
-- and nothing disconnects the instant this migration applies.
--
-- An index on the column lets the listener's WHERE-clause evaluate cheaply at
-- 30s sync cadence even with thousands of overlays.

BEGIN;

ALTER TABLE overlays
    ADD COLUMN last_connected_at TIMESTAMP NOT NULL DEFAULT NOW();

CREATE INDEX IF NOT EXISTS idx_overlays_last_connected_at
    ON overlays(last_connected_at);

COMMENT ON COLUMN overlays.last_connected_at IS
    'Timestamp of the most recent WebSocket attach for this overlay (bumped by api-gateway on AddConnection and during the connection heartbeat tick). twitch-listener filters its source-discovery query on this column to skip channels nobody is watching, avoiding Twitchs per-account concurrent-channel cap.';

COMMIT;
