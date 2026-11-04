-- Migration: 114_chat_command_images
-- Description: Chat command images (ADR-0080). A command may show an image on
--   the streamer's alerts overlays (image_url is a template engagement-service
--   renders), so an image alone is now enough for a command to do something.
--   Re-runnable: the migration runner applies every file on each pod start.
--   The chat_commands_does_something check that allows an image-only command is
--   defined in 123, the one place that predicate lives: re-adding a narrower
--   copy here failed every pod start once a sound-only command existed.

BEGIN;

ALTER TABLE chat_commands ADD COLUMN IF NOT EXISTS image_url TEXT NOT NULL DEFAULT ''
    CHECK (length(image_url) <= 1000);

COMMIT;
