-- Migration: 114_chat_command_images (down)
-- Reverses 114_chat_command_images.sql. Image-only commands cannot exist without
-- the column, so they are deleted. Safe to run when the objects are absent.

BEGIN;

DELETE FROM chat_commands WHERE response = '' AND NOT create_clip;
ALTER TABLE chat_commands DROP CONSTRAINT IF EXISTS chat_commands_does_something;
ALTER TABLE chat_commands ADD CONSTRAINT chat_commands_does_something
    CHECK (response <> '' OR create_clip);
ALTER TABLE chat_commands DROP COLUMN IF EXISTS image_url;

COMMIT;
