-- Migration: 113_chat_commands
-- Description: Streamer-defined chat commands and keyword triggers (ADR-0079).
--   A row belongs to a user and fires on chat from the chat sources of that
--   user's overlays: a 'command' when the message's first word equals trigger
--   ("!hello"), a 'keyword' when trigger appears anywhere in the message. It can
--   reply with a rendered template (sent as the streamer) and/or create a Twitch
--   clip. engagement-service owns the table.
--
-- Also seeds the chat_commands feature gate (ADR-0008), OPEN (is_premium=FALSE):
--   only replies that spend YouTube Data API quota are premium, and that is a
--   per-send check on the owner, not this gate. The gate exists so the admin UI
--   can make the feature an upsell without a deploy.
--
-- Idempotent throughout: the migration runner re-applies every migration on
--   each pod start (CREATE ... IF NOT EXISTS, ON CONFLICT DO NOTHING).

BEGIN;

CREATE TABLE IF NOT EXISTS chat_commands (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind             VARCHAR(10)  NOT NULL CHECK (kind IN ('command', 'keyword')),
    trigger          VARCHAR(100) NOT NULL CHECK (btrim(trigger) <> ''),
    response         TEXT         NOT NULL DEFAULT '' CHECK (length(response) <= 2000),
    create_clip      BOOLEAN      NOT NULL DEFAULT FALSE,
    min_role         VARCHAR(12)  NOT NULL DEFAULT 'everyone'
                     CHECK (min_role IN ('everyone', 'subscriber', 'vip', 'moderator', 'broadcaster')),
    cooldown_seconds INT          NOT NULL DEFAULT 5 CHECK (cooldown_seconds BETWEEN 0 AND 86400),
    enabled          BOOLEAN      NOT NULL DEFAULT TRUE,
    use_count        BIGINT       NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT chat_commands_does_something CHECK (response <> '' OR create_clip)
);

CREATE UNIQUE INDEX IF NOT EXISTS chat_commands_user_trigger
    ON chat_commands (user_id, kind, LOWER(trigger));

COMMENT ON TABLE chat_commands IS
    'Streamer-defined chat commands and keyword triggers (ADR-0079), owned by engagement-service.';

INSERT INTO feature_gates (feature_key, is_premium, early_access, description)
VALUES (
    'chat_commands',
    FALSE,
    FALSE,
    'Create and edit chat commands and keyword triggers (replies, Twitch clips). Replies in YouTube chat always need premium because they spend YouTube API quota'
)
ON CONFLICT (feature_key) DO NOTHING;

COMMIT;
