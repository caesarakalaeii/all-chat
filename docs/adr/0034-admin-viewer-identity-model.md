# ADR-0034: Admin Viewer Identity Model

**Date**: 2026-07-17
**Status**: Accepted
**Revised**: 2026-11-18 (rows are platform accounts, not sessions; premium keyed on the durable viewer)
**Deciders**: caesarakalaeii

## Context

A "viewer" lives in two places:

- `viewer_sessions` (migration 011) holds one OAuth sign-in per `(platform, platform_user_id)`, with encrypted tokens, rate-limit counters and the ban flag. Logging out deletes the row. Migration 040 added `viewer_sessions.user_id` to link a viewer who also has a streamer (`users`) account.
- `viewers` plus `viewer_platform_identities` (migration 035) is the durable viewer: one `viewers` row with one identity row per linked platform account. Premium (`viewers.is_premium`, ADR-0019), cosmetics and the points economy key on it, and the message-processor badge enricher reads it. `viewer_sessions.viewer_id` points a session at it.

A durable viewer does not need a session: engagement-service creates the row and identity for every chatter who earns points (`GetOrCreateViewerByPlatform`), long before that chatter signs in, if ever. A viewer who logs out keeps the durable viewer but loses the session.

The one place viewer behaviour is recorded is `viewer_message_history` (keyed on `viewer_session_id`), effectively write-only — the only reader was the GDPR data export. The original admin viewer view listed raw sessions with no usage context or streamer linkage, and surfaced the rate-limit counters `message_count_1min` / `message_count_1hour` as if they were engagement numbers (they are throttle bookkeeping).

The first version of this decision kept the view on sessions, which left two holes (found 2026-11-04 with a YouTube chatter the admin could neither find nor grant premium to): a durable viewer without a session never appeared, and when such a chatter later signed in, auth-service never wrote the existing identity onto the new session (`viewer_sessions.viewer_id` stayed NULL) — so premium granted through a session could land on a viewer the enricher never reads.

## Decision

**One admin row per platform account**: sessions full-outer-joined with identities on `(platform, platform_user_id)`; ban and activity act on the session, premium on the durable viewer. No cross-platform dedup and no new tables.

- `ListAll` (`services/auth-service/repository/viewer_repository.go`) returns `AdminViewer` rows: `session_id` null for an identity without a session, `viewer_id` null for a session whose identity was never resolved. Premium is read from the identity's viewer (the one the enricher reads), falling back to the session's. Search, platform, ban and premium filters apply to the joined row.
- **Ban and activity** use `/admin/viewers/:session_id/{ban,unban,activity}`; a row without a session shows "Not signed in".
- **Premium** uses `POST /admin/viewers/identities/:viewer_id/premium` (`SetViewerPremium(viewerID)`, 404 when no such viewer), writing the ADR-0019 override with the optional ADR-0027 expiry and recomputing `viewers.is_premium`.
- **Sessions follow their identity**: every identity resolution in auth-service (`GetOrCreateViewerByPlatform`, `LinkPlatformToViewer`) points the session at the identity's viewer, including when the identity already existed or moved to another viewer. Migration 125 repaired pre-existing rows.
- **Context**: the linked streamer `user_id` is surfaced with a link into the streamer's admin record (ADR-0036), and a read-only per-session activity aggregate over `viewer_message_history` (existing index) shows which streamer/overlay the viewer chats in.
- The rate-limit counters are no longer presented.
- Human-level dedup across platform accounts stays deferred; collapsing a human's accounts into one row is out of proportion to an admin read view.

Rejected options: keeping the view on raw sessions (session-less viewers invisible, the defect above); collapsing rows per durable viewer (ban/activity are per session, so a row would fan out again); keeping the rate-limit counters.

## Consequences

- Every viewer identity is reachable (and granted) whether or not it has ever signed in; admin premium lands on the view the enricher reads.
- One human with several platform accounts still appears as several rows. The list now includes every chatter engagement-service created, so the newest-first default is dominated by never-signed-in chatters; filters and search narrow it.
- The activity aggregate puts message metadata (channel, streamer, timestamps) in one more admin place: existing PII under existing admin access.

## Implementation

- Data model: no new table; reuses migrations 011, 035, 040 and 125.
- auth-service: `ListAll` / `AdminViewer`, `SetViewerPremium(viewerID)`, `GetViewerActivity`, `linkSession` in the identity repository; routes in `cmd/main.go`.
- api-gateway: admin viewer routes in `services/api-gateway/cmd/viewer_routes.go` (`registerAdminViewerRoutes`; before this revision the activity route was never registered, so the Activity dialog 404'd).
- Frontend: `frontend/src/app/admin/viewers/page.tsx` and `frontend/src/app/admin/search/page.tsx`.

## Related

- [ADR-0036](./0036-admin-url-addressable-selection.md) — the master-detail pattern this view links into.
- [ADR-0019](./0019-split-streamer-viewer-premium.md) / [ADR-0027](./0027-time-limited-admin-premium-overrides.md) — the premium override and its expiry.
- [ADR-0035](./0035-admin-global-entity-search.md) — search resolves viewers into this view.
