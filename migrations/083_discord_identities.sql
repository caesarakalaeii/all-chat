-- Migration: 083_discord_identities
-- Description: Links an All-Chat account to a Discord USER account (ADR-0048, Discord leg).
--
-- Why a new table rather than a column on users, and why it does not already exist: the only
-- Discord flow All-Chat has ever run is a `scope=bot` guild invite, whose callback returns a
-- guild_id and no user identity at all. So `discord_guilds` records which SERVERS a streamer
-- connected while All-Chat has never known WHO any user is on Discord.
--
-- The Discord leg of delegated moderation cannot work without that. Discord has no per-user
-- moderation API, so the shared bot performs every write and All-Chat must itself verify the
-- acting human's guild permissions -- which requires their Discord user id. The overlay owner's
-- id is needed too, to prove they control the guild the moderator would act in.
--
-- Idempotent throughout: the migration runner re-applies every migration on each pod start, so
-- a non-idempotent statement would crash-loop fresh pods.

BEGIN;

CREATE TABLE IF NOT EXISTS discord_identities (
    -- One Discord account per All-Chat user, hence the PK rather than a surrogate id.
    user_id          UUID         PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    -- Discord snowflake. VARCHAR, not BIGINT: snowflakes are 64-bit unsigned and every Discord
    -- API surface exchanges them as strings, matching discord_guilds.guild_id.
    discord_user_id  VARCHAR(30)  NOT NULL,
    -- Display only, for "you are linked as <name>" copy. Discord usernames are mutable, so this
    -- is refreshed on every re-link and must never be used to identify anyone.
    discord_username VARCHAR(255),
    linked_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- A Discord account may back at most ONE All-Chat account, and this is a security control, not
-- hygiene: without it a second All-Chat user could claim someone else's Discord identity and
-- inherit their guild permissions on every delegated action. A re-link attempt by a different
-- All-Chat user must fail loudly here rather than silently take the identity over.
CREATE UNIQUE INDEX IF NOT EXISTS uq_discord_identities_discord_user_id
    ON discord_identities (discord_user_id);

COMMENT ON TABLE discord_identities IS
    'Links an All-Chat user to their Discord user account (ADR-0048). Required because Discord '
    'has no per-user moderation API: the shared bot performs every write, so All-Chat must '
    'verify the acting human''s own guild permissions, which needs their Discord snowflake. '
    'Holds no OAuth token -- the identify grant is used once, at link time, and discarded.';

COMMENT ON COLUMN discord_identities.discord_user_id IS
    'Discord snowflake, globally unique across All-Chat accounts so one Discord identity can '
    'never be claimed by two users.';

COMMENT ON COLUMN discord_identities.discord_username IS
    'Display only. Discord usernames are mutable; never identify a user by this.';

-- Reuse the shared updated_at trigger function if this database has it (it is created by the
-- early migrations); skip silently otherwise so a partially-migrated database still applies.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_proc WHERE proname = 'update_updated_at_column') THEN
        DROP TRIGGER IF EXISTS update_discord_identities_updated_at ON discord_identities;
        CREATE TRIGGER update_discord_identities_updated_at
            BEFORE UPDATE ON discord_identities
            FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
    END IF;
END $$;

COMMIT;
