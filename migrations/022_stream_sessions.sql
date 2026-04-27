-- Migration: 022_stream_sessions
-- Description: Create stream_sessions table for historical session tracking

-- Create stream_sessions table
CREATE TABLE IF NOT EXISTS stream_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    overlay_id UUID NOT NULL REFERENCES overlays(id) ON DELETE CASCADE,

    -- Session lifecycle
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    state VARCHAR(20) NOT NULL DEFAULT 'ACTIVE', -- ACTIVE, ENDING, COMPLETED

    -- Aggregated statistics (populated on session end)
    total_events INTEGER DEFAULT 0,
    event_counts JSONB DEFAULT '{}'::jsonb,
    -- Example: {"subs": 5, "bits": 1200, "raids": 2, "super_chats": 3}

    total_monetary_value DECIMAL(10,2) DEFAULT 0, -- Total $ value of all donations

    -- Credit roll metadata
    credit_roll_displayed_count INTEGER DEFAULT 0, -- How many times displayed
    last_credit_roll_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_stream_sessions_overlay_id ON stream_sessions(overlay_id);
CREATE INDEX IF NOT EXISTS idx_stream_sessions_started_at ON stream_sessions(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_stream_sessions_state ON stream_sessions(state);
