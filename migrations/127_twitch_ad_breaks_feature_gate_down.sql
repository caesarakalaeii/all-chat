-- All-Chat Migration 127 (down): Twitch ad break notices feature gate
--
-- Reverses 127_twitch_ad_breaks_feature_gate.sql. Safe to run if the row is absent.
--
-- With the row gone the gate cache fails closed and reports the key as
-- premium-required, so the ad break opt-in becomes premium-only until the row is
-- re-seeded. Grants already made are untouched.

BEGIN;

DELETE FROM feature_gates WHERE feature_key = 'twitch_ad_breaks';

COMMIT;
