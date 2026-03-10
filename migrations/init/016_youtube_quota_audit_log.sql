-- Migration: Add comprehensive YouTube API audit logging
-- Purpose: Track every YouTube API call for debugging and drift analysis
-- Date: 2026-01-11

-- Create audit log table for all YouTube API operations
CREATE TABLE IF NOT EXISTS youtube_quota_audit_log (
    id BIGSERIAL PRIMARY KEY,
    date DATE NOT NULL,
    timestamp TIMESTAMP NOT NULL DEFAULT NOW(),

    -- API call details
    operation_type VARCHAR(20) NOT NULL,  -- 'reserve', 'confirm', 'rollback', 'record'
    service_name VARCHAR(50) NOT NULL,    -- 'youtube-listener', 'overlay-manager', 'auth-service'
    endpoint VARCHAR(50) NOT NULL,        -- 'Search.List', 'Videos.List', 'LiveChatMessages.List'

    -- Quota changes
    units_delta INT NOT NULL,             -- +100, -100 (negative for rollbacks)
    units_before INT NOT NULL,
    units_after INT NOT NULL,

    -- Success tracking
    api_success BOOLEAN,                  -- true/false/null (null = pre-call/reservation)
    error_type VARCHAR(20),               -- '4xx', '5xx', 'network', 'timeout', 'emergency_shutoff'
    error_message TEXT,
    http_status_code INT,

    -- Context
    channel_id VARCHAR(255),
    video_id VARCHAR(255),
    overlay_id UUID,
    reservation_id VARCHAR(255),          -- Links reserve/confirm/rollback operations

    -- Performance
    latency_ms INT,

    -- Metadata
    client_ip VARCHAR(50),
    user_agent VARCHAR(255),
    metadata JSONB
);

-- Indexes for efficient querying
CREATE INDEX idx_youtube_audit_timestamp ON youtube_quota_audit_log(timestamp DESC);
CREATE INDEX idx_youtube_audit_date ON youtube_quota_audit_log(date DESC);
CREATE INDEX idx_youtube_audit_endpoint ON youtube_quota_audit_log(endpoint, timestamp DESC);
CREATE INDEX idx_youtube_audit_service ON youtube_quota_audit_log(service_name, timestamp DESC);
CREATE INDEX idx_youtube_audit_reservation ON youtube_quota_audit_log(reservation_id) WHERE reservation_id IS NOT NULL;
CREATE INDEX idx_youtube_audit_channel ON youtube_quota_audit_log(channel_id, timestamp DESC) WHERE channel_id IS NOT NULL;
CREATE INDEX idx_youtube_audit_overlay ON youtube_quota_audit_log(overlay_id, timestamp DESC) WHERE overlay_id IS NOT NULL;

-- Create daily reconciliation table to track drift
CREATE TABLE IF NOT EXISTS youtube_quota_reconciliation (
    id SERIAL PRIMARY KEY,
    date DATE NOT NULL UNIQUE,
    reconciled_at TIMESTAMP NOT NULL DEFAULT NOW(),

    -- Database vs YouTube Console (manual entry)
    db_units_used INT NOT NULL,
    api_console_units INT,                -- Manual entry from YouTube API console

    -- Audit log verification
    audit_log_total INT,                  -- Sum of all increments in audit log

    -- Drift analysis
    drift_db_vs_console INT,              -- db_units_used - api_console_units
    drift_db_vs_audit INT,                -- db_units_used - audit_log_total
    drift_percentage DECIMAL(5,2),

    -- Status
    status VARCHAR(20) NOT NULL,          -- 'ok', 'warning', 'critical'
    notes TEXT,

    -- Detailed breakdown
    by_service JSONB,                     -- {"youtube-listener": 8000, "overlay-manager": 1500, "auth-service": 500}
    by_endpoint JSONB                     -- {"Search.List": 5000, "LiveChatMessages.List": 4500, ...}
);

CREATE INDEX idx_youtube_reconciliation_date ON youtube_quota_reconciliation(date DESC);

-- Function to auto-cleanup old audit logs (keep 30 days for forensics)
CREATE OR REPLACE FUNCTION cleanup_old_youtube_audit_logs() RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    DELETE FROM youtube_quota_audit_log
    WHERE timestamp < NOW() - INTERVAL '30 days'
    RETURNING COUNT(*) INTO deleted_count;

    IF deleted_count > 0 THEN
        RAISE NOTICE 'Cleaned up % old YouTube audit log entries', deleted_count;
    END IF;

    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- Function to generate daily reconciliation report
CREATE OR REPLACE FUNCTION reconcile_youtube_quota_usage(p_date DATE) RETURNS JSONB AS $$
DECLARE
    v_db_usage INT;
    v_audit_total INT;
    v_by_service JSONB;
    v_by_endpoint JSONB;
    v_drift INT;
    v_status VARCHAR(20);
    v_result JSONB;
BEGIN
    -- Get usage from main quota table
    SELECT units_used INTO v_db_usage
    FROM youtube_quota_usage
    WHERE date = p_date;

    IF NOT FOUND THEN
        v_db_usage := 0;
    END IF;

    -- Calculate total from audit log (sum of all confirms, minus rollbacks)
    SELECT COALESCE(SUM(units_delta), 0) INTO v_audit_total
    FROM youtube_quota_audit_log
    WHERE date = p_date
      AND operation_type IN ('confirm', 'record');  -- Only count actual charged operations

    -- Break down by service
    SELECT jsonb_object_agg(service_name, total) INTO v_by_service
    FROM (
        SELECT service_name, SUM(units_delta) as total
        FROM youtube_quota_audit_log
        WHERE date = p_date AND operation_type IN ('confirm', 'record')
        GROUP BY service_name
    ) svc;

    -- Break down by endpoint
    SELECT jsonb_object_agg(endpoint, total) INTO v_by_endpoint
    FROM (
        SELECT endpoint, SUM(units_delta) as total
        FROM youtube_quota_audit_log
        WHERE date = p_date AND operation_type IN ('confirm', 'record')
        GROUP BY endpoint
    ) ep;

    -- Calculate drift
    v_drift := v_db_usage - v_audit_total;

    -- Determine status
    IF ABS(v_drift) <= 5 THEN
        v_status := 'ok';
    ELSIF ABS(v_drift) <= 50 THEN
        v_status := 'warning';
    ELSE
        v_status := 'critical';
    END IF;

    -- Build result JSON
    v_result := jsonb_build_object(
        'date', p_date,
        'db_usage', v_db_usage,
        'audit_total', v_audit_total,
        'drift', v_drift,
        'status', v_status,
        'by_service', COALESCE(v_by_service, '{}'::jsonb),
        'by_endpoint', COALESCE(v_by_endpoint, '{}'::jsonb)
    );

    -- Store reconciliation result
    INSERT INTO youtube_quota_reconciliation (
        date, db_units_used, audit_log_total,
        drift_db_vs_audit, status, by_service, by_endpoint
    ) VALUES (
        p_date, v_db_usage, v_audit_total,
        v_drift, v_status, v_by_service, v_by_endpoint
    ) ON CONFLICT (date) DO UPDATE SET
        reconciled_at = NOW(),
        db_units_used = EXCLUDED.db_units_used,
        audit_log_total = EXCLUDED.audit_log_total,
        drift_db_vs_audit = EXCLUDED.drift_db_vs_audit,
        status = EXCLUDED.status,
        by_service = EXCLUDED.by_service,
        by_endpoint = EXCLUDED.by_endpoint;

    RETURN v_result;
END;
$$ LANGUAGE plpgsql;

-- Add comment explaining the audit log purpose
COMMENT ON TABLE youtube_quota_audit_log IS 'Comprehensive audit trail of all YouTube API operations for drift analysis and debugging. Retention: 30 days.';
COMMENT ON TABLE youtube_quota_reconciliation IS 'Daily reconciliation reports comparing database usage, audit logs, and YouTube API console.';
