-- Migration: 114_chat_command_images
-- Description: Chat command images (ADR-0080). A command may show an image on
--   the streamer's alerts overlays (image_url is a template engagement-service
--   renders), so an image alone is now enough for a command to do something.
--   Re-runnable: the migration runner applies every file on each pod start.

BEGIN;

ALTER TABLE chat_commands ADD COLUMN IF NOT EXISTS image_url TEXT NOT NULL DEFAULT ''
    CHECK (length(image_url) <= 1000);

ALTER TABLE chat_commands DROP CONSTRAINT IF EXISTS chat_commands_does_something;
ALTER TABLE chat_commands ADD CONSTRAINT chat_commands_does_something
    CHECK (response <> '' OR create_clip OR image_url <> '');

COMMIT;
