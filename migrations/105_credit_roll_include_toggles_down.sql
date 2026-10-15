-- Migration: 105_credit_roll_include_toggles (down)
-- Description: Drop the three opt-in leaderboard toggles added by the up script.

ALTER TABLE credit_roll_configs
    DROP COLUMN IF EXISTS include_treasure_chests,
    DROP COLUMN IF EXISTS include_shares,
    DROP COLUMN IF EXISTS include_likes;

ALTER TABLE credit_roll_configs
    ALTER COLUMN include_channel_points SET DEFAULT FALSE;
