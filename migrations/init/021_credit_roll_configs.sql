-- Migration: 021_credit_roll_configs
-- Description: Create credit_roll_configs table for credit roll configuration per overlay

-- Create credit_roll_configs table
CREATE TABLE IF NOT EXISTS credit_roll_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    overlay_id UUID NOT NULL REFERENCES overlays(id) ON DELETE CASCADE,

    -- Feature enable
    enabled BOOLEAN DEFAULT TRUE,

    -- Event type filters (which events to include in credit roll)
    include_subs BOOLEAN DEFAULT TRUE,
    include_resubs BOOLEAN DEFAULT TRUE,
    include_gift_subs BOOLEAN DEFAULT TRUE,
    include_bits BOOLEAN DEFAULT TRUE,
    include_raids BOOLEAN DEFAULT TRUE,
    include_channel_points BOOLEAN DEFAULT FALSE,
    include_super_chats BOOLEAN DEFAULT TRUE,
    include_memberships BOOLEAN DEFAULT TRUE,
    include_follows BOOLEAN DEFAULT TRUE,

    -- Leaderboard settings
    leaderboard_top_n INTEGER DEFAULT 10,
    leaderboard_sort_by VARCHAR(20) DEFAULT 'value', -- 'value' or 'count'

    -- Display settings
    scroll_speed INTEGER DEFAULT 50,
    display_duration_seconds INTEGER DEFAULT 120,
    background_opacity DECIMAL(3,2) DEFAULT 0.8,
    theme VARCHAR(50) DEFAULT 'classic', -- 'classic', 'cinematic', 'modern'

    -- Clips settings
    clips_enabled BOOLEAN DEFAULT TRUE,
    clips_max_count INTEGER DEFAULT 5,
    clips_fallback_days INTEGER DEFAULT 7,

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(overlay_id)
);

CREATE INDEX idx_credit_roll_configs_overlay_id ON credit_roll_configs(overlay_id);

-- Function to auto-create credit roll config for new overlays
CREATE OR REPLACE FUNCTION create_credit_roll_config()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO credit_roll_configs (overlay_id)
    VALUES (NEW.id)
    ON CONFLICT (overlay_id) DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to auto-create credit roll config when overlay is created
CREATE TRIGGER trigger_create_credit_roll_config
    AFTER INSERT ON overlays
    FOR EACH ROW
    EXECUTE FUNCTION create_credit_roll_config();

-- Backfill credit roll configs for existing overlays
INSERT INTO credit_roll_configs (overlay_id)
SELECT id FROM overlays
ON CONFLICT (overlay_id) DO NOTHING;
