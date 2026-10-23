-- Migration: 115_chat_bots
-- Description: Chat-bot sender accounts (ADR-0081). chat_bot_accounts holds the
--   OAuth credential of a bot account that sends chat on a streamer's behalf:
--   owner_user_id NULL = the global All-Chat Bot (one per platform, managed by
--   admins via /admin/chat-bots), non-NULL = a streamer's custom bot connected
--   from /commands (gated by the custom_chat_bot feature gate, premium +
--   early access). chat_bot_settings records the per-user sender choice
--   ('bot' default, row absent = default); engagement-service owns writes,
--   auth-service only reads it on bot-send.
--
-- Also seeds the custom_chat_bot feature gate (ADR-0008): connecting a streamer's
--   OWN bot account is premium + early access — a second Twitch/Kick rate-limit
--   identity spends the same shared send budget, and the feature ships to beta
--   testers first. Flip either flag with the feature-gate admin endpoint.
--
-- Idempotent throughout: the migration runner re-applies every migration on
--   each pod start (CREATE ... IF NOT EXISTS, ON CONFLICT DO NOTHING).

BEGIN;

CREATE TABLE IF NOT EXISTS chat_bot_accounts (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id     UUID        NULL REFERENCES users(id) ON DELETE CASCADE,
    platform          VARCHAR(10) NOT NULL CHECK (platform IN ('twitch', 'kick', 'youtube')),
    platform_user_id  TEXT        NOT NULL,
    login             TEXT        NOT NULL,
    display_name      TEXT        NOT NULL,
    access_token_enc  TEXT        NOT NULL DEFAULT '',
    refresh_token_enc TEXT        NOT NULL DEFAULT '',
    token_expires_at  TIMESTAMPTZ NULL,
    scopes            TEXT[]      NOT NULL DEFAULT '{}',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One bot per (owner-less OR owner, platform). The COALESCE collapses NULL
-- (All-Chat Bot) onto a fixed sentinel so a single UNIQUE index covers both.
CREATE UNIQUE INDEX IF NOT EXISTS chat_bot_accounts_owner_platform
    ON chat_bot_accounts (COALESCE(owner_user_id, '00000000-0000-0000-0000-000000000000'::uuid), platform);

COMMENT ON TABLE chat_bot_accounts IS
    'Full-Chat bot sender credentials (ADR-0081): owner_user_id NULL = All-Chat Bot, else a streamer custom bot.';

CREATE TABLE IF NOT EXISTS chat_bot_settings (
    user_id    UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    sender     VARCHAR(8)  NOT NULL DEFAULT 'bot' CHECK (sender IN ('bot', 'self')),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE chat_bot_settings IS
    'Per-user chat-command sender choice (ADR-0081); no row = All-Chat Bot. engagement-service writes, auth-service reads.';

INSERT INTO feature_gates (feature_key, is_premium, early_access, description)
VALUES (
    'custom_chat_bot',
    TRUE,
    TRUE,
    'Connect your own Twitch, Kick or YouTube account as a chat-bot sender for commands and timers, with your bot''s name in chat'
)
ON CONFLICT (feature_key) DO NOTHING;

COMMIT;
