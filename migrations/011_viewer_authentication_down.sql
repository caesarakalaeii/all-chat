-- Rollback viewer authentication support
-- Migration: 011 (DOWN)
-- Description: Remove viewer authentication tables

-- Drop tables in reverse order (respect foreign keys)
DROP TABLE IF EXISTS viewer_message_history CASCADE;
DROP TABLE IF EXISTS viewer_sessions CASCADE;
