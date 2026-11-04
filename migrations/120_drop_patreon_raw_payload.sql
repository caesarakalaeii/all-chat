-- All-Chat Migration 120: drop the stored Patreon payloads
-- Migration: 120
--
-- premium_subscriptions.raw (migration 063) held the full body of every Patreon
-- members:* webhook: the patron's email, full name, profile URL, social
-- connections, the creator note and, when present, their postal address. No
-- code ever read it back; entitlement is decided from the typed columns
-- (provider_user_id, status, tier_id, cents, current_period_end), which this
-- leaves in place. Dropping the column deletes every stored payload, and
-- payment-service no longer writes one.
--
-- IDEMPOTENCY: every service runs the full migration set on each pod restart
-- (the runner does not track applied migrations), so this must be safe to
-- re-execute, hence DROP COLUMN IF EXISTS.

BEGIN;

ALTER TABLE premium_subscriptions DROP COLUMN IF EXISTS raw;

COMMIT;
