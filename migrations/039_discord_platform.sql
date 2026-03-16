-- Migration 039: Add Discord to supported_platforms
-- Required for the overlay_chat_sources FK constraint (platform -> supported_platforms.platform)
INSERT INTO supported_platforms (platform, display_name, is_enabled, requires_oauth, config_schema)
VALUES ('discord', 'Discord', true, false, '{"api_type": "gateway", "requires_bot": true}')
ON CONFLICT (platform) DO NOTHING;
