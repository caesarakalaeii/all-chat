-- Migration: 125_viewer_session_identity_relink
-- Description: Point every viewer session at the durable viewer its platform
-- account maps to (ADR-0034).
--
-- engagement-service creates a viewers row and its viewer_platform_identities
-- mapping for every chatter who earns points, before that chatter ever signs in.
-- When they later signed in, auth-service found the existing identity but never
-- wrote it onto the new session, so viewer_sessions.viewer_id stayed NULL (and a
-- platform moved to another viewer by the connect flow kept pointing at the old
-- one). The admin view then showed no premium control for them, and the linked-
-- streamer premium inheritance could not reach them. auth-service now links the
-- session on every identity resolution; this repairs the rows written before.
--
-- Re-runs (scripts/run-migrations.sh re-runs every migration on every pod start)
-- are no-ops: the IS DISTINCT FROM guard only matches rows that disagree.

UPDATE viewer_sessions vs
SET viewer_id = vpi.viewer_id
FROM viewer_platform_identities vpi
WHERE vpi.platform = vs.platform
  AND vpi.platform_user_id = vs.platform_user_id
  AND vs.viewer_id IS DISTINCT FROM vpi.viewer_id;
