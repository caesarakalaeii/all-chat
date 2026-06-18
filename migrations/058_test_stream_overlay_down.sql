-- All-Chat Migration 058 DOWN: remove the public test-stream overlay
--
-- Removing the user cascades to the overlay, its config and any sources
-- (ON DELETE CASCADE), so a single delete is sufficient.

BEGIN;

DELETE FROM users WHERE id = '00000000-0000-4000-8000-000000000a12';

COMMIT;
