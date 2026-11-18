-- Migration: 124_credit_roll_restart_delay (down)
-- Description: Drop the credit roll's restart delay.

ALTER TABLE credit_roll_configs
    DROP COLUMN IF EXISTS restart_delay_seconds;
