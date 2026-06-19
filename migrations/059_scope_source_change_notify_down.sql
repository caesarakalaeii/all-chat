-- Down migration for 059_scope_source_change_notify.sql
-- Restores the original unconditional trigger from migration 004.

DROP TRIGGER IF EXISTS chat_source_change_ins_del_trigger ON overlay_chat_sources;
DROP TRIGGER IF EXISTS chat_source_change_update_trigger ON overlay_chat_sources;

DROP TRIGGER IF EXISTS chat_source_change_trigger ON overlay_chat_sources;
CREATE TRIGGER chat_source_change_trigger
AFTER INSERT OR UPDATE OR DELETE ON overlay_chat_sources
FOR EACH ROW
EXECUTE FUNCTION notify_chat_source_change();
