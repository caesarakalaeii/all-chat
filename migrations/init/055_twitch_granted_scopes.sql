-- All-Chat Migration 055 (local initdb mirror): per-user granted OAuth scopes
-- Migration: 055
--
-- Mirror of migrations/055_twitch_granted_scopes.sql for the docker-compose.frontend.yml
-- local stack, whose Postgres builds its schema from this directory's initdb scripts
-- (it does not run the migration runner). overlay-manager's source-list query
-- (ListByOverlayID) references users.granted_scopes to compute chat_via_eventsub, so the
-- column must exist for the local overlay page to load sources.
--
-- Note: initdb scripts only run on a fresh data volume. Existing local volumes must be
-- recreated (docker compose -f docker-compose.frontend.yml down -v) to pick this up.
--
-- IDEMPOTENT: safe to re-run (ADD COLUMN/CREATE INDEX IF NOT EXISTS).

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS granted_scopes TEXT[] NOT NULL DEFAULT '{}';

CREATE INDEX IF NOT EXISTS idx_users_granted_scopes
    ON users USING GIN (granted_scopes);
