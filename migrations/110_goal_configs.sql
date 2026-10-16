-- Migration: 110_goal_configs
-- Description: Goal bar configuration of a 'goal' overlay (issue #955,
--   ADR-0072): one metric per overlay. followers (Twitch) and subscribers
--   (YouTube) are API-polled counts; bits (Twitch) and super_chat (YouTube)
--   are cumulative counters the alert-processor keeps in Redis on top of
--   starting_value. Allowed metric/platform pairs are validated in Go
--   (shared/alerts.ValidGoalMetric), not by a CHECK constraint.
--
-- Also seeds the goal_overlay feature gate (ADR-0008): creating a goal
--   overlay is premium; existing goal overlays keep working.
--
-- Idempotent throughout: the migration runner re-applies every migration on
--   each pod start (CREATE ... IF NOT EXISTS, ON CONFLICT DO NOTHING).

BEGIN;

CREATE TABLE IF NOT EXISTS goal_configs (
    overlay_id     UUID         PRIMARY KEY REFERENCES overlays(id) ON DELETE CASCADE,
    metric         VARCHAR(20)  NOT NULL,
    platform       VARCHAR(20)  NOT NULL,
    target         BIGINT       NOT NULL,
    starting_value BIGINT       NOT NULL DEFAULT 0,
    title          TEXT         NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now()
);

COMMENT ON TABLE goal_configs IS
    'Goal bar of a goal overlay (ADR-0072). Current values live in Redis overlay:{id}:goal:{metric}.';

INSERT INTO feature_gates (feature_key, is_premium, early_access, description)
VALUES (
    'goal_overlay',
    TRUE,
    FALSE,
    'Create goal overlays (follower, subscriber, bits and Super Chat goal bars). Existing goal overlays keep working'
)
ON CONFLICT (feature_key) DO NOTHING;

COMMIT;
