-- Migration: 105_credit_roll_include_toggles
-- Description: Add the three new credit-roll leaderboard categories as include_*
--   toggles on credit_roll_configs: treasure chests (TikTok treasure_chest and
--   super_fan_box), shares and likes. Defaults to FALSE, unlike the existing
--   include_* columns: boards a credit roll never had must not appear on it
--   uninvited the moment this migration runs — showing them is the streamer's
--   opt-in.
--   The older include_* toggles were parsed and saved here but never ENFORCED by
--   the aggregation query; creditroll/handler.go now skips disabled categories,
--   and these three ship enforced from day one.
--
--   include_channel_points is the exception among the old toggles: it defaulted
--   to FALSE and the settings page never offered it, so every stored FALSE is a
--   default nobody chose, while the Top Channel Points board was always shown.
--   Enforcing it as stored would silently remove that board from every credit
--   roll, so existing rows are switched on once and the default becomes TRUE.
--
-- Idempotent: the migration runner re-applies every migration on each pod start.
-- The one-time UPDATE is guarded by the absence of include_treasure_chests, which
-- this same migration adds, so it runs exactly once and never overrides a
-- streamer who later turns the board off.

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'credit_roll_configs' AND column_name = 'include_treasure_chests'
    ) THEN
        UPDATE credit_roll_configs SET include_channel_points = TRUE;
    END IF;
END $$;

ALTER TABLE credit_roll_configs
    ALTER COLUMN include_channel_points SET DEFAULT TRUE;

ALTER TABLE credit_roll_configs
    ADD COLUMN IF NOT EXISTS include_treasure_chests BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS include_shares BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS include_likes BOOLEAN NOT NULL DEFAULT FALSE;
