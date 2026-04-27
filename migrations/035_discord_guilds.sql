-- Phase 27: Discord Listener — guild tracking table
-- guild_id is VARCHAR(30), NOT BIGINT: Discord Snowflake IDs exceed JS safe-integer range (2^53)
CREATE TABLE IF NOT EXISTS discord_guilds (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    guild_id      VARCHAR(30) NOT NULL,
    guild_name    VARCHAR(255) NOT NULL,
    guild_icon    VARCHAR(255),
    connected_at  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, guild_id)
);

CREATE INDEX IF NOT EXISTS idx_discord_guilds_user_id ON discord_guilds(user_id);
CREATE INDEX IF NOT EXISTS idx_discord_guilds_guild_id ON discord_guilds(guild_id);

-- Grant application user access. CloudNativePG (production) runs migrations
-- as the postgres superuser and the app connects as a separate allchat_user
-- role. Local dev / docker-compose / test containers may not have that role,
-- so guard the GRANTs to keep this migration runnable everywhere.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'allchat_user') THEN
        GRANT ALL ON discord_guilds TO allchat_user;
        GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO allchat_user;
    END IF;
END $$;
