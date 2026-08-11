-- Rollback: 083_discord_identities
--
-- Drops the Discord account links. Users who had linked must re-run the identify consent after a
-- roll-forward; nothing else is affected, since no other table references this one and the
-- Discord bot-invite flow (discord_guilds) is independent of it.

BEGIN;

DROP TRIGGER IF EXISTS update_discord_identities_updated_at ON discord_identities;
DROP TABLE IF EXISTS discord_identities;

COMMIT;
