-- Migration: Add public overlay flag for viewer access control
-- Created: 2025-12-20
-- Purpose: Allow streamers to designate which overlay is publicly accessible to viewers
--          without exposing the overlay ID. This prevents viewers from triggering polling
--          and maintains overlay ID secrecy.

-- Add flag to control which overlay is exposed to viewers
ALTER TABLE overlays ADD COLUMN IF NOT EXISTS is_public_for_viewers BOOLEAN NOT NULL DEFAULT false;

-- Add index for efficient viewer queries
-- Partial index only on public, active overlays for performance
CREATE INDEX IF NOT EXISTS idx_overlays_user_public ON overlays(user_id, is_public_for_viewers)
WHERE is_public_for_viewers = true AND is_active = true;

-- Add comment for documentation
COMMENT ON COLUMN overlays.is_public_for_viewers IS
'When true, this overlay is visible to viewers via /ws/chat/{username}. Viewers can connect to the WebSocket without knowing the overlay ID. Only one overlay per user should be public to avoid confusion.';
