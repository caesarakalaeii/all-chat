-- Migration: 123_chat_command_alerts
-- Description: Chat command alerts (ADR-0080). Besides its image, a command may
--   carry its own alert sound (an https URL) and alert text (a template
--   engagement-service renders), so each command is its own alert and a sound
--   or a text alone is enough for a command to do something.
--   Re-runnable: the migration runner applies every file on each pod start.
--   This is the only place chat_commands_does_something is redefined. A later
--   migration that widens it must move the definition there and delete it here:
--   the runner replays every file, so an older, narrower copy is re-checked
--   against current rows on each pod start and fails the init container.

BEGIN;

ALTER TABLE chat_commands ADD COLUMN IF NOT EXISTS sound_url TEXT NOT NULL DEFAULT ''
    CHECK (length(sound_url) <= 1000);
ALTER TABLE chat_commands ADD COLUMN IF NOT EXISTS alert_text TEXT NOT NULL DEFAULT ''
    CHECK (length(alert_text) <= 500);

ALTER TABLE chat_commands DROP CONSTRAINT IF EXISTS chat_commands_does_something;
ALTER TABLE chat_commands ADD CONSTRAINT chat_commands_does_something
    CHECK (response <> '' OR create_clip OR image_url <> '' OR sound_url <> '' OR alert_text <> '');

COMMIT;
