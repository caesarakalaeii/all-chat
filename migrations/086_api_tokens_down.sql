-- Rollback: 086_api_tokens
--
-- Drops the personal access token store. Every issued token is destroyed, so desktop clients
-- (Stream Deck / StreamController plugins) must be re-issued a token after a roll-forward. The
-- plaintext secrets were never stored, so nothing here is recoverable by design.
--
-- Nothing else is touched: PATs are an additional authentication path, so the cookie/JWT session
-- flow keeps working unchanged with this table absent (the resolver simply finds no rows and
-- every `allchat_pat_` bearer is rejected).

BEGIN;

DROP INDEX IF EXISTS idx_api_tokens_live;
DROP INDEX IF EXISTS idx_api_tokens_user;
DROP TABLE IF EXISTS api_tokens;

COMMIT;
