-- All-Chat Migration 055 (DOWN): Remove per-user granted OAuth scopes
-- Migration: 055
--
-- Reverts 055_twitch_granted_scopes.sql. Dropping granted_scopes makes every
-- Twitch channel fall back to the IRC listener (the EventSub partition predicate
-- can no longer be satisfied), so run this only alongside reverting the listener
-- query changes that read the column.

BEGIN;

DROP INDEX IF EXISTS idx_users_granted_scopes;

ALTER TABLE users
    DROP COLUMN IF EXISTS granted_scopes;

COMMIT;
