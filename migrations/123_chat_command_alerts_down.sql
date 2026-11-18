-- Migration: 123_chat_command_alerts (down)
-- Reverses 123_chat_command_alerts.sql. Commands whose only action is a sound or
-- an alert text cannot exist without the columns, so they are deleted. Safe to
-- run when the objects are absent.

BEGIN;

DELETE FROM chat_commands WHERE response = '' AND NOT create_clip AND image_url = '';
ALTER TABLE chat_commands DROP CONSTRAINT IF EXISTS chat_commands_does_something;
ALTER TABLE chat_commands ADD CONSTRAINT chat_commands_does_something
    CHECK (response <> '' OR create_clip OR image_url <> '');
ALTER TABLE chat_commands DROP COLUMN IF EXISTS alert_text;
ALTER TABLE chat_commands DROP COLUMN IF EXISTS sound_url;

COMMIT;
