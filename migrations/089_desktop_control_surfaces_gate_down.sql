-- All-Chat Migration 089 (down): remove the desktop control surfaces feature gate
--
-- Reverses 089_desktop_control_surfaces_gate.sql. Safe to run if the row is absent.
--
-- With the row gone, RequirePremium("desktop_control_surfaces") sees an unregistered key.
-- Device tokens already issued keep working: this row gates the APPROVE step only, never the
-- resolution of an existing token.

BEGIN;

DELETE FROM feature_gates WHERE feature_key = 'desktop_control_surfaces';

COMMIT;
