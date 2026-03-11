-- Migration: Quota Events Tracking
-- Purpose: Track YouTube API quota events for analysis, alerting, and historical reporting
-- Created: 2026-01-08

-- Create quota events table for audit trail
CREATE TABLE IF NOT EXISTS youtube_quota_events (
    id SERIAL PRIMARY KEY,
    event_type VARCHAR(50) NOT NULL,
    quota_state VARCHAR(20) NOT NULL,
    usage_percentage DECIMAL(5, 2) NOT NULL,
    units_used INT NOT NULL,
    units_limit INT NOT NULL,
    units_remaining INT NOT NULL,
    previous_state VARCHAR(20),
    affected_channels TEXT[], -- Array of channel IDs affected by this event
    message TEXT NOT NULL,
    severity VARCHAR(20) NOT NULL, -- info, warning, error, critical
    metadata JSONB, -- Additional event-specific data
    created_at TIMESTAMP DEFAULT NOW() NOT NULL
);

-- Create index on event_type for filtering by type
CREATE INDEX IF NOT EXISTS idx_youtube_quota_events_type
ON youtube_quota_events(event_type);

-- Create index on quota_state for filtering by state
CREATE INDEX IF NOT EXISTS idx_youtube_quota_events_state
ON youtube_quota_events(quota_state);

-- Create index on created_at for time-range queries
CREATE INDEX IF NOT EXISTS idx_youtube_quota_events_created_at
ON youtube_quota_events(created_at DESC);

-- Create index on severity for filtering critical events
CREATE INDEX IF NOT EXISTS idx_youtube_quota_events_severity
ON youtube_quota_events(severity) WHERE severity IN ('error', 'critical');

-- Create composite index for common queries (state + time)
CREATE INDEX IF NOT EXISTS idx_youtube_quota_events_state_time
ON youtube_quota_events(quota_state, created_at DESC);

-- Add comment to table
COMMENT ON TABLE youtube_quota_events IS 'Tracks YouTube API quota events including state transitions, threshold crossings, and depletion events for monitoring and analysis';

-- Add column comments
COMMENT ON COLUMN youtube_quota_events.event_type IS 'Type of quota event: state_changed, threshold_crossed, quota_exhausted, quota_depleted, quota_recovered, channel_quota_exceeded';
COMMENT ON COLUMN youtube_quota_events.quota_state IS 'Global quota state at time of event: HEALTHY, DEGRADED, CRITICAL, EXHAUSTED, DEPLETED';
COMMENT ON COLUMN youtube_quota_events.usage_percentage IS 'Percentage of daily quota used at time of event';
COMMENT ON COLUMN youtube_quota_events.affected_channels IS 'Array of YouTube channel IDs affected by this quota event';
COMMENT ON COLUMN youtube_quota_events.severity IS 'Event severity for alerting: info, warning, error, critical';
COMMENT ON COLUMN youtube_quota_events.metadata IS 'Additional event-specific data in JSON format';
