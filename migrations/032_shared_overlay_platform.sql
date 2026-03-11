-- Migration 032: Add shared_overlay as a platform type and store recipient_overlay_id on share_requests
-- Up migration

-- 1. Add shared_overlay to the supported_platforms table
--    is_enabled = TRUE: platform is active (matches twitch pattern)
--    requires_oauth = FALSE: access is via share relationship, not OAuth
INSERT INTO supported_platforms (platform, display_name, is_enabled, requires_oauth)
VALUES ('shared_overlay', 'Shared Overlay', TRUE, FALSE)
ON CONFLICT (platform) DO NOTHING;

-- 2. Add recipient_overlay_id to share_requests (nullable: existing rows have no value)
--    This stores which overlay the recipient shared back, enabling the sender's add-source flow
ALTER TABLE share_requests
    ADD COLUMN IF NOT EXISTS recipient_overlay_id UUID REFERENCES overlays(id) ON DELETE SET NULL;
