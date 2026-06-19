-- Migration: 059_scope_source_change_notify.sql
-- Description: Scope the chat_source_changes NOTIFY trigger so heartbeat-only
--              writes stop masquerading as source-config changes.
--
-- BACKGROUND
-- Migration 004 created chat_source_change_trigger as
--   AFTER INSERT OR UPDATE OR DELETE ... FOR EACH ROW
-- with no column filter, so EVERY update fired pg_notify('chat_source_changes').
--
-- Listeners heartbeat their active sources by calling source-manager's
-- ActivateSource (services/source-manager/registry/repository.go), which runs
--   UPDATE overlay_chat_sources SET is_active = true, updated_at = NOW() ...
-- once per poll cycle (~30s) per actively-polled channel. is_active is already
-- true, so the only real change is updated_at — yet the unconditional trigger
-- still fired a notification. source-manager's demand subscriber
-- (services/source-manager/demand/subscriber.go) then re-fetched sources and
-- re-published demand to every listener, ~once every 30s per overlay. Observed
-- in production as a steady stream of "Source change detected, demand refreshed
-- action=UPDATE" log lines and listeners churning their channels ("source
-- flapping").
--
-- The cleanup job (services/source-manager/cleanup) keys staleness off
-- updated_at, so the heartbeat MUST keep refreshing updated_at — we cannot stop
-- the write. Instead we stop the NOTIFY for updated_at-only changes by scoping
-- the UPDATE trigger to listener-relevant columns. INSERT and DELETE always
-- notify (they are always meaningful).
--
-- IDEMPOTENT: CREATE OR REPLACE FUNCTION + DROP TRIGGER IF EXISTS + CREATE
-- TRIGGER. Safe to re-run on every pod start (see scripts/run-migrations.sh).

-- Function body is unchanged from migration 004; restated here so this file is
-- self-contained and re-runnable in isolation.
CREATE OR REPLACE FUNCTION notify_chat_source_change()
RETURNS TRIGGER AS $$
DECLARE
  notification_payload JSON;
BEGIN
  notification_payload := json_build_object(
    'action', TG_OP,
    'overlay_id', COALESCE(NEW.overlay_id, OLD.overlay_id),
    'platform', COALESCE(NEW.platform, OLD.platform),
    'channel_id', COALESCE(NEW.channel_id, OLD.channel_id),
    'channel_name', COALESCE(NEW.channel_name, OLD.channel_name),
    'is_active', COALESCE(NEW.is_active, OLD.is_active),
    'timestamp', NOW()
  );

  PERFORM pg_notify('chat_source_changes', notification_payload::text);

  RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

-- Replace the single unconditional trigger (migration 004) with:
--   1. an INSERT/DELETE trigger that always notifies, and
--   2. a column-scoped UPDATE trigger that ignores updated_at-only heartbeats.
DROP TRIGGER IF EXISTS chat_source_change_trigger ON overlay_chat_sources;
DROP TRIGGER IF EXISTS chat_source_change_ins_del_trigger ON overlay_chat_sources;
DROP TRIGGER IF EXISTS chat_source_change_update_trigger ON overlay_chat_sources;

CREATE TRIGGER chat_source_change_ins_del_trigger
AFTER INSERT OR DELETE ON overlay_chat_sources
FOR EACH ROW
EXECUTE FUNCTION notify_chat_source_change();

CREATE TRIGGER chat_source_change_update_trigger
AFTER UPDATE ON overlay_chat_sources
FOR EACH ROW
WHEN (
     OLD.overlay_id     IS DISTINCT FROM NEW.overlay_id
  OR OLD.platform       IS DISTINCT FROM NEW.platform
  OR OLD.channel_id     IS DISTINCT FROM NEW.channel_id
  OR OLD.channel_name   IS DISTINCT FROM NEW.channel_name
  OR OLD.channel_handle IS DISTINCT FROM NEW.channel_handle
  OR OLD.is_active      IS DISTINCT FROM NEW.is_active
  OR OLD.auth_required  IS DISTINCT FROM NEW.auth_required
  OR OLD.config         IS DISTINCT FROM NEW.config
)
EXECUTE FUNCTION notify_chat_source_change();

COMMENT ON FUNCTION notify_chat_source_change() IS
'Sends a chat_source_changes notification when chat sources change. Attached via chat_source_change_ins_del_trigger (INSERT/DELETE) and chat_source_change_update_trigger (UPDATE, scoped to listener-relevant columns).';

COMMENT ON TRIGGER chat_source_change_update_trigger ON overlay_chat_sources IS
'Notifies chat_source_changes only when listener-relevant columns change. updated_at-only heartbeat writes (ActivateSource, every ~30s per polled channel) are intentionally excluded to prevent demand-refresh storms / source flapping.';
