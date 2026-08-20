-- Rollback: 088_device_tokens
--
-- Drops the paired-device credential store and the pending-link table. Every issued device
-- token is destroyed, so a paired Stream Deck / StreamController plugin must be re-linked
-- after a roll-forward. The plaintext secrets were never stored, so nothing here is
-- recoverable by design — the same property that makes the tables safe also makes this
-- rollback lossy, deliberately.
--
-- Nothing else is touched:
--
--   * Personal access tokens (api_tokens, migration 086) are a SEPARATE credential on the same
--     resolver seam and keep working unchanged. ADR-0049's loopback flow never reached a
--     headless box; a PAT is still how that machine authenticates.
--   * The cookie/JWT session flow is unaffected: with these tables absent the device resolver
--     simply finds no rows and every `allchat_dev_` bearer is rejected.
--
-- Order matters: device_link_requests carries an FK to device_tokens, so it goes first.

BEGIN;

DROP INDEX IF EXISTS idx_device_link_requests_user;
DROP INDEX IF EXISTS idx_device_link_requests_expiry;
DROP INDEX IF EXISTS idx_device_link_requests_user_code;
DROP TABLE IF EXISTS device_link_requests;

DROP INDEX IF EXISTS idx_device_tokens_live;
DROP INDEX IF EXISTS idx_device_tokens_user;
DROP TABLE IF EXISTS device_tokens;

COMMIT;
