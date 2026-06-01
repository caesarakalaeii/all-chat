-- All-Chat Migration 055: Persist granted OAuth scopes per user
-- Migration: 055
--
-- EventSub channel.chat.message reading requires the streamer to have granted
-- the chat scopes (user:read:chat, user:bot, channel:bot). We record the scope
-- set Twitch returns at consent so the two Twitch chat listeners can partition
-- channels between them without ever double-reading or dropping one:
--   - twitch-eventsub-listener reads a channel only when its owner granted
--     'user:read:chat' (and the token is still valid).
--   - twitch-listener (IRC) reads every other Twitch channel (the exact
--     complement, via NOT EXISTS on the same predicate).
--
-- Twitch preserves scopes server-side across token refresh, so this column is
-- written only at OAuth consent (auth-service) and never by token-refresh.
--
-- The column is NOT NULL with default '{}' so existing rows back-fill to the
-- empty set: pre-existing streamers (whose consent predates this feature) are
-- treated as "no chat scopes" and stay on IRC. Nobody is forced to re-auth and
-- nothing disconnects the instant this migration applies.
--
-- A GIN index lets the listeners' membership predicate
-- ('user:read:chat' = ANY(granted_scopes)) evaluate cheaply at 30s sync cadence.
--
-- IDEMPOTENCY: every service runs the full migration set on each pod restart
-- (the runner does not track applied migrations), so this script must be safe
-- to re-execute — hence ADD COLUMN IF NOT EXISTS / CREATE INDEX IF NOT EXISTS.

BEGIN;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS granted_scopes TEXT[] NOT NULL DEFAULT '{}';

CREATE INDEX IF NOT EXISTS idx_users_granted_scopes
    ON users USING GIN (granted_scopes);

COMMENT ON COLUMN users.granted_scopes IS
    'OAuth scopes granted at the most recent consent for this users auth_provider account. Written by auth-service on user create/link/login; never modified by token-refresh (the provider preserves scopes across refresh). Used to partition Twitch channels between IRC (twitch-listener) and EventSub (twitch-eventsub-listener) chat reading.';

COMMIT;
