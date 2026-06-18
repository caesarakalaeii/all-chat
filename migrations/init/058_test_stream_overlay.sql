-- All-Chat Migration 058: Dedicated public test-stream overlay
-- Migration: 058
--
-- Provides a fixed, always-active overlay that the message-processor's public
-- test-stream generator publishes fake chat/events to. External tools can
-- connect to /ws/overlay/<this id> (OBS mode, no token) and observe a
-- deterministic-shaped stream of messages, poll-vote numbers (1/2/3/4) and
-- platform events without any real streaming platform being involved.
--
-- The IDs are fixed/recognizable so deployments and docs can reference them:
--   user    00000000-0000-4000-8000-000000000a12  (synthetic, never logs in)
--   overlay 00000000-0000-4000-8000-000000000a11
--
-- No overlay_chat_sources are seeded on purpose: the generator publishes
-- straight to the overlay Pub/Sub channel, so we must NOT make real listeners
-- (twitch/youtube/...) try to join a fake channel.
--
-- IDEMPOTENCY: fixed UUIDs + ON CONFLICT (id) DO NOTHING, safe to re-run on
-- every pod start (see scripts/run-migrations.sh).

BEGIN;

-- Synthetic owner. access_token/refresh_token are NOT NULL but never used:
-- this user never authenticates and the overlay is driven by the generator.
INSERT INTO users (id, twitch_id, username, display_name, access_token, refresh_token, token_expires_at)
VALUES (
    '00000000-0000-4000-8000-000000000a12',
    'test-stream-bot',
    'allchat_test_stream',
    'All-Chat Test Stream',
    'unused',
    'unused',
    NOW() + INTERVAL '100 years'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO overlays (id, user_id, name, description, is_active)
VALUES (
    '00000000-0000-4000-8000-000000000a11',
    '00000000-0000-4000-8000-000000000a12',
    'Public Test Stream',
    'Fixed overlay for the message-processor public test-stream generator. Connect a WebSocket client to observe fake chat, poll votes (1/2/3/4) and events.',
    TRUE
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO overlay_configs (overlay_id, enable_7tv, enable_bttv, enable_ffz)
VALUES (
    '00000000-0000-4000-8000-000000000a11',
    TRUE,
    TRUE,
    TRUE
)
ON CONFLICT (overlay_id) DO NOTHING;

COMMIT;
