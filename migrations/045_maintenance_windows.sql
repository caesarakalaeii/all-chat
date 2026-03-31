-- Migration 045: Maintenance Windows
--
-- Creates the maintenance_windows table for scheduled downtime notifications.
-- Admins schedule maintenance windows with a title, description, and time range.
-- Users see upcoming/active maintenance banners on the dashboard.
--
-- Completed windows (ends_at < NOW()) are excluded from user-facing queries.

CREATE TABLE IF NOT EXISTS maintenance_windows (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title       VARCHAR(200) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    starts_at   TIMESTAMP WITH TIME ZONE NOT NULL,
    ends_at     TIMESTAMP WITH TIME ZONE NOT NULL,
    created_by  VARCHAR(100) NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_maintenance_windows_ends_at ON maintenance_windows (ends_at);

COMMENT ON TABLE maintenance_windows IS 'Scheduled maintenance/downtime windows. Admins create these to proactively notify users of planned outages.';
COMMENT ON COLUMN maintenance_windows.title IS 'Short title shown in the user-facing banner (e.g. "Database maintenance")';
COMMENT ON COLUMN maintenance_windows.description IS 'Optional longer description of the maintenance work';
COMMENT ON COLUMN maintenance_windows.starts_at IS 'When maintenance begins (with timezone)';
COMMENT ON COLUMN maintenance_windows.ends_at IS 'When maintenance ends; windows with ends_at < NOW() are excluded from user queries';
COMMENT ON COLUMN maintenance_windows.created_by IS 'User ID of the admin who created the window';
