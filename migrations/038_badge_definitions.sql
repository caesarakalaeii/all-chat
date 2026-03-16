-- Migration 038: badge_definitions catalog table
-- Phase 31: All-Chat platform badges — catalog of displayable badge types

CREATE TABLE IF NOT EXISTS badge_definitions (
    name        VARCHAR(50)  PRIMARY KEY,
    icon_url_1x TEXT         NOT NULL DEFAULT '',
    icon_url_2x TEXT         NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Seed built-in badge types (idempotent)
INSERT INTO badge_definitions (name, icon_url_1x, icon_url_2x)
VALUES
    ('allchat', '', ''),
    ('premium', '', '')
ON CONFLICT (name) DO NOTHING;
