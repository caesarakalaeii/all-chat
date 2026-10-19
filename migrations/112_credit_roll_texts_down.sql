-- Migration: 112_credit_roll_texts (down)
-- Description: Drop the credit roll's custom texts and text sizes.

ALTER TABLE credit_roll_configs
    DROP COLUMN IF EXISTS texts,
    DROP COLUMN IF EXISTS font_scales;
