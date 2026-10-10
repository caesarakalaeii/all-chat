-- All-Chat Migration 102 (down): remove the YouTube official-API mode feature gate
--
-- Reverses 102_youtube_official_api_gate.sql. Safe to run if the row is absent.
--
-- With the row gone the gate cache reports the key as premium (unknown keys fail
-- closed), so only premium owners stay eligible. Stored official_api opt-ins are left
-- in place; they are inert without the Data API listener.

BEGIN;

DELETE FROM feature_gates WHERE feature_key = 'youtube_official_api';

COMMIT;
