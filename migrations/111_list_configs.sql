-- Migration: 111_list_configs
-- Description: Leaderboard configuration of a 'list' overlay (issue #955,
--   ADR-0072): which board it shows (bits | gifts | super_chat), how many
--   ranks, and a title. The rankings themselves are the alert-processor's
--   Redis sorted sets overlay:{id}:leaderboard:{category} (ADR-0068).
--
-- Also seeds the list_overlay feature gate (ADR-0008): creating a list
--   overlay is premium; existing list overlays keep working.
--
-- Idempotent throughout: the migration runner re-applies every migration on
--   each pod start (CREATE ... IF NOT EXISTS, ON CONFLICT DO NOTHING).

BEGIN;

CREATE TABLE IF NOT EXISTS list_configs (
    overlay_id  UUID         PRIMARY KEY REFERENCES overlays(id) ON DELETE CASCADE,
    category    VARCHAR(20)  NOT NULL,
    "limit"     INT          NOT NULL DEFAULT 10,
    title       TEXT         NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

COMMENT ON TABLE list_configs IS
    'Leaderboard shown by a list overlay (ADR-0072). Rankings live in Redis overlay:{id}:leaderboard:{category}.';

INSERT INTO feature_gates (feature_key, is_premium, early_access, description)
VALUES (
    'list_overlay',
    TRUE,
    FALSE,
    'Create list overlays (top supporter leaderboards for bits, gifts and Super Chats). Existing list overlays keep working'
)
ON CONFLICT (feature_key) DO NOTHING;

COMMIT;
