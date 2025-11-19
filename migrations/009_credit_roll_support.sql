-- Credit Roll Feature Support
-- Migration: 009
-- Description: Add tables for stream sessions, events, clips, and credit roll settings

-- ============================================================================
-- Stream Sessions: Track individual streaming sessions
-- ============================================================================
CREATE TABLE IF NOT EXISTS stream_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Session metadata
    title TEXT,
    description TEXT,
    started_at TIMESTAMP NOT NULL,
    ended_at TIMESTAMP,

    -- Platform information (JSONB for flexibility)
    -- Example: {"twitch": {"channel_id": "12345", "game": "Just Chatting"}, "youtube": {...}}
    platform_info JSONB NOT NULL DEFAULT '{}'::jsonb,

    -- Status: live, ended, archived
    status VARCHAR(50) NOT NULL DEFAULT 'live',

    -- Cached statistics (updated in real-time by Event Collector)
    stats JSONB DEFAULT '{
        "total_events": 0,
        "followers": 0,
        "subscribers": 0,
        "bits_total": 0,
        "super_chat_total": 0,
        "unique_chatters": 0,
        "peak_viewers": 0
    }'::jsonb,

    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_stream_sessions_user_id ON stream_sessions(user_id);
CREATE INDEX idx_stream_sessions_started_at ON stream_sessions(started_at DESC);
CREATE INDEX idx_stream_sessions_status ON stream_sessions(status);
CREATE INDEX idx_stream_sessions_user_status ON stream_sessions(user_id, status);

-- ============================================================================
-- Stream Events: All platform events (subs, follows, bits, etc.)
-- ============================================================================
CREATE TABLE IF NOT EXISTS stream_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stream_session_id UUID NOT NULL REFERENCES stream_sessions(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Platform & event type
    platform VARCHAR(50) NOT NULL, -- twitch, youtube, kick, tiktok
    event_type VARCHAR(50) NOT NULL, -- follow, sub, bits, raid, gift_sub, super_chat, etc.
    event_subtype VARCHAR(50), -- new_sub, resub, tier_1, tier_2, tier_3, etc.

    -- User who triggered the event
    platform_user_id VARCHAR(255),
    platform_username VARCHAR(255),
    display_name VARCHAR(255),
    avatar_url TEXT,

    -- Event-specific data (flexible JSONB structure)
    metadata JSONB DEFAULT '{}'::jsonb,
    -- Examples:
    -- Bits: {"amount": 500, "message": "Great stream!"}
    -- Sub: {"tier": "1000", "months": 12, "streak": 6}
    -- Gift: {"recipient_count": 5, "recipients": [...]}
    -- Raid: {"raid_viewer_count": 50, "from_broadcaster": "..."}
    -- Super Chat: {"amount": 1000, "currency": "USD", "message": "..."}

    -- Timestamps
    occurred_at TIMESTAMP NOT NULL, -- When event happened on platform
    created_at TIMESTAMP NOT NULL DEFAULT NOW(), -- When we recorded it

    -- Flags
    is_test BOOLEAN DEFAULT FALSE, -- For testing
    is_backfilled BOOLEAN DEFAULT FALSE -- If added retroactively
);

CREATE INDEX idx_stream_events_session ON stream_events(stream_session_id);
CREATE INDEX idx_stream_events_user ON stream_events(user_id);
CREATE INDEX idx_stream_events_type ON stream_events(event_type, platform);
CREATE INDEX idx_stream_events_occurred_at ON stream_events(occurred_at DESC);
CREATE INDEX idx_stream_events_platform_user ON stream_events(platform_user_id);

-- Composite index for common queries (events by session and type)
CREATE INDEX idx_stream_events_session_type ON stream_events(stream_session_id, event_type);

-- ============================================================================
-- Clips: Platform clips and user-provided videos
-- ============================================================================
CREATE TABLE IF NOT EXISTS clips (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    stream_session_id UUID REFERENCES stream_sessions(id) ON DELETE SET NULL,

    -- Platform information
    platform VARCHAR(50) NOT NULL, -- twitch, kick, youtube, user_upload
    platform_clip_id VARCHAR(255), -- Platform's ID for the clip

    -- URLs
    clip_url TEXT NOT NULL, -- Direct link to clip
    embed_url TEXT, -- Embeddable player URL
    thumbnail_url TEXT,

    -- Metadata
    title TEXT,
    duration_seconds INTEGER, -- Clip duration
    view_count INTEGER DEFAULT 0,
    created_at_platform TIMESTAMP, -- When clip was created on platform

    -- User-provided clips
    is_user_provided BOOLEAN DEFAULT FALSE,
    user_notes TEXT,

    -- Ranking (computed by Clip Manager)
    rank_score FLOAT, -- Computed ranking score (views, recency, duration)

    -- Timestamps
    fetched_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_updated TIMESTAMP NOT NULL DEFAULT NOW(),

    UNIQUE(platform, platform_clip_id)
);

CREATE INDEX idx_clips_user ON clips(user_id);
CREATE INDEX idx_clips_session ON clips(stream_session_id);
CREATE INDEX idx_clips_platform ON clips(platform);
CREATE INDEX idx_clips_rank ON clips(rank_score DESC NULLS LAST);

-- ============================================================================
-- User Credit Roll Settings: One-time user configuration for automatic generation
-- ============================================================================
CREATE TABLE IF NOT EXISTS user_credit_roll_settings (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,

    -- Event sections configuration
    sections_config JSONB DEFAULT '{
        "subscribers": {"enabled": true, "title": "Thank You Subscribers", "sort": "alphabetical"},
        "bits": {"enabled": true, "title": "Top Bits Supporters", "sort": "amount_desc"},
        "followers": {"enabled": true, "title": "New Followers", "sort": "alphabetical"},
        "chatters": {"enabled": true, "title": "Amazing Chatters", "sort": "message_count"},
        "raids": {"enabled": true, "title": "Raiders", "sort": "viewer_count"}
    }'::jsonb,

    -- Clip selection settings
    clip_selection_mode VARCHAR(50) DEFAULT 'auto', -- auto, manual (manual requires user review)
    max_clips INTEGER DEFAULT 5,
    min_clips INTEGER DEFAULT 1,
    prefer_recent BOOLEAN DEFAULT TRUE,
    min_duration_seconds INTEGER DEFAULT 10,
    max_duration_seconds INTEGER DEFAULT 60,

    -- Fallback video
    fallback_video_url TEXT,
    fallback_video_start_time INTEGER,

    -- Default background (if no clips/fallback)
    default_background_type VARCHAR(50) DEFAULT 'gradient',
    default_background_config JSONB DEFAULT '{
        "colors": ["#6366f1", "#8b5cf6", "#d946ef"]
    }'::jsonb,

    -- Styling (applied to all credit rolls)
    styling_config JSONB DEFAULT '{
        "font_family": "Inter",
        "text_color": "#ffffff",
        "background_overlay": "rgba(0,0,0,0.4)",
        "scroll_speed": "medium"
    }'::jsonb,

    -- Music settings
    music_enabled BOOLEAN DEFAULT FALSE,
    music_url TEXT,
    music_volume FLOAT DEFAULT 0.7,

    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- ============================================================================
-- Triggers: Auto-update timestamps
-- ============================================================================

CREATE TRIGGER update_stream_sessions_updated_at BEFORE UPDATE ON stream_sessions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_user_credit_roll_settings_updated_at BEFORE UPDATE ON user_credit_roll_settings
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_clips_last_updated BEFORE UPDATE ON clips
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- Initial Data: Create default settings for existing users
-- ============================================================================

-- Create default credit roll settings for all existing users
INSERT INTO user_credit_roll_settings (user_id)
SELECT id FROM users
ON CONFLICT (user_id) DO NOTHING;
