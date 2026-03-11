-- Migration: 023_credit_roll_custom_css
-- Description: Add custom_css field to credit_roll_configs table for CSS customization

ALTER TABLE credit_roll_configs
    ADD COLUMN IF NOT EXISTS custom_css TEXT DEFAULT '';

-- Ensure non-null constraint
UPDATE credit_roll_configs
SET custom_css = ''
WHERE custom_css IS NULL;

ALTER TABLE credit_roll_configs
    ALTER COLUMN custom_css SET NOT NULL;
