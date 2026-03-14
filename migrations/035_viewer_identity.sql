-- 035_viewer_identity.sql
-- Description: Add viewer identity tables for cross-platform cosmetics (Phase 28)

CREATE TABLE IF NOT EXISTS viewers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMP DEFAULT NOW()
);

ALTER TABLE viewer_sessions
    ADD COLUMN IF NOT EXISTS viewer_id UUID REFERENCES viewers(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_viewer_sessions_viewer_id ON viewer_sessions(viewer_id);

CREATE TABLE IF NOT EXISTS viewer_platform_identities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    viewer_id UUID NOT NULL REFERENCES viewers(id) ON DELETE CASCADE,
    platform VARCHAR(50) NOT NULL,
    platform_user_id VARCHAR(100) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(platform, platform_user_id)
);

CREATE INDEX IF NOT EXISTS idx_viewer_platform_identities_viewer_id ON viewer_platform_identities(viewer_id);
CREATE INDEX IF NOT EXISTS idx_viewer_platform_identities_lookup ON viewer_platform_identities(platform, platform_user_id);

CREATE TABLE IF NOT EXISTS viewer_cosmetics (
    viewer_id UUID PRIMARY KEY REFERENCES viewers(id) ON DELETE CASCADE,
    name_color VARCHAR(7),
    updated_at TIMESTAMP DEFAULT NOW()
);
