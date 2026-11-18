-- All-Chat Migration 120 (down): restore the premium_subscriptions.raw column
--
-- Reverses the schema change of 120_drop_patreon_raw_payload.sql so a
-- payment-service image from before it (which still writes raw) can run again.
-- The dropped payloads are gone for good; the column comes back empty.

BEGIN;

ALTER TABLE premium_subscriptions ADD COLUMN IF NOT EXISTS raw JSONB;

COMMIT;
