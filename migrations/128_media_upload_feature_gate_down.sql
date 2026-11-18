-- All-Chat Migration 128 (down): media upload feature gate
--
-- Reverses 128_media_upload_feature_gate.sql. Safe to run if the row is absent.
--
-- With the row gone the gate cache fails closed and reports the key as
-- premium-required, so uploads stay premium-only until the row is re-seeded.
-- Files already uploaded are untouched.

BEGIN;

DELETE FROM feature_gates WHERE feature_key = 'media_upload';

COMMIT;
