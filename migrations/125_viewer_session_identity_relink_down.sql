-- Rollback: 125_viewer_session_identity_relink
-- Nothing to undo: the up migration only corrected viewer_sessions.viewer_id to
-- the identity mapping auth-service now writes on every sign-in anyway.
SELECT 1;
