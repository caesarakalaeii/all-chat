-- Migration: 004_source_change_notifications.sql
-- Description: Add PostgreSQL LISTEN/NOTIFY for instant source change detection
-- This eliminates the 30-second polling delay in listener services

-- Create notification function that fires when chat sources change
CREATE OR REPLACE FUNCTION notify_chat_source_change()
RETURNS TRIGGER AS $$
DECLARE
  notification_payload JSON;
BEGIN
  -- Build JSON payload with change details
  notification_payload := json_build_object(
    'action', TG_OP,
    'overlay_id', COALESCE(NEW.overlay_id, OLD.overlay_id),
    'platform', COALESCE(NEW.platform, OLD.platform),
    'channel_id', COALESCE(NEW.channel_id, OLD.channel_id),
    'channel_name', COALESCE(NEW.channel_name, OLD.channel_name),
    'is_active', COALESCE(NEW.is_active, OLD.is_active),
    'timestamp', NOW()
  );

  -- Send notification to 'chat_source_changes' channel
  PERFORM pg_notify('chat_source_changes', notification_payload::text);

  -- Log the notification for debugging
  RAISE NOTICE 'Chat source change notification sent: %', notification_payload;

  RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

-- Create trigger on overlay_chat_sources table
-- Fires AFTER any INSERT, UPDATE, or DELETE
DROP TRIGGER IF EXISTS chat_source_change_trigger ON overlay_chat_sources;
CREATE TRIGGER chat_source_change_trigger
AFTER INSERT OR UPDATE OR DELETE ON overlay_chat_sources
FOR EACH ROW
EXECUTE FUNCTION notify_chat_source_change();

-- Add comment for documentation
COMMENT ON FUNCTION notify_chat_source_change() IS
'Sends PostgreSQL notification when chat sources are added, updated, or removed. Listener services subscribe to chat_source_changes channel for instant updates.';

COMMENT ON TRIGGER chat_source_change_trigger ON overlay_chat_sources IS
'Triggers instant notification to listener services when chat sources change, eliminating polling delay.';
