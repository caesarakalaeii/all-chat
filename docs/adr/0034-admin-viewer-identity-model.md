# ADR-0034: Admin Viewer Identity Model

**Date**: 2026-07-17
**Status**: Accepted
**Revised**: 2026-11-04 (rows are platform accounts, not sessions; premium keyed on the durable viewer)
**Deciders**: caesarakalaeii

## Context and Problem Statement

A "viewer" in All-Chat is stored in two places:

- `viewer_sessions` (`migrations/011_viewer_authentication.sql`) holds **one OAuth sign-in per
  `(platform, platform_user_id)`** (`UNIQUE(platform, platform_user_id)`), with the encrypted
  tokens, rate-limit counters and the ban flag. Logging out deletes the row. Migration 040
  (`040_viewer_sessions_user_link.sql`) added `viewer_sessions.user_id` to link a viewer who
  *also* has a streamer (`users`) account so the badge enricher can resolve their All-Chat status.
- `viewers` plus `viewer_platform_identities` (migration 035) is the **durable viewer**: one
  `viewers` row with one identity row per linked platform account. Premium
  (`viewers.is_premium`, ADR-0019), cosmetics and the points economy key on it, and it is what the
  message-processor badge enricher reads. `viewer_sessions.viewer_id` points a session at it.

A durable viewer does not need a session. engagement-service creates the `viewers` row and its
identity for every chatter who earns points (`GetOrCreateViewerByPlatform` in
`services/engagement-service/repository`), long before that chatter signs in, if they ever do.
A viewer who logs out keeps their durable viewer but loses the session.

The one place a viewer's actual behavior is recorded is `viewer_message_history`
(`viewer_session_id`, `streamer_user_id`, `overlay_id`, `channel_id`/`channel_name`, `message_text`,
`sent_at`), indexed on `viewer_session_id`. That table was effectively **write-only** — the only
thing that ever read it back was the DSGVO data export.

The original admin viewer view was, in the maintainer's words, "basically useless." It listed raw
sessions with no usage context (which streamers does this viewer actually chat in?), no linkage to a
streamer account, and it surfaced the rate-limit counters `message_count_1min` / `message_count_1hour`
as if they were engagement numbers — they are not, they are throttle bookkeeping that resets on a timer.

The first version of this decision kept the view on sessions. That left two holes, found on
2026-11-04 with a YouTube chatter (`UCL3eFQtbthgTGal-ktJV5Vg`) the admin could neither find nor grant
premium to:

- A durable viewer without a session never appeared, so it was unreachable from the web.
- When such a chatter later signed in, auth-service found the existing identity but never wrote it
  onto the new session (`viewer_sessions.viewer_id` stayed NULL). The admin view then showed no
  premium control for them, and premium granted through a session could land on a viewer the
  enricher never reads.

## Decision Drivers

- Operate on the unit each admin action acts on. Ban acts on a `viewer_sessions` row
  (`viewer_sessions.is_banned`, checked when the viewer sends a message); premium acts on the
  durable `viewers` row.
- Every viewer the platform knows about must be reachable, signed in or not.
- Give the view real context — which streamers/overlays a viewer participates in, and whether the
  viewer is also a streamer — without a schema-heavy identity project.
- Keep it cheap: reads must ride existing indexes or unique keys, not new tables.
- Stop presenting misleading numbers.

## Considered Options

1. **One row per platform account: sessions full-outer-joined with identities; ban and activity on
   the session, premium on the durable viewer; per-session activity aggregate over
   `viewer_message_history`; no cross-platform dedup.**
   - ✅ Lists session-less identities; each action targets the row that holds its state; the join
     rides the two `UNIQUE(platform, platform_user_id)` keys; no new tables.
   - ❌ A human with several platform accounts still shows as several rows (no dedup yet), and
     chatters who never signed in share the list with signed-in viewers.
2. **Keep the view on raw `viewer_sessions` (the first version of this ADR).**
   - ✅ No join.
   - ❌ Session-less viewers are invisible and cannot be granted premium; this is the defect above.
3. **Collapse rows per durable viewer (one row per human).**
   - ✅ One row per person.
   - ❌ Ban and activity are per session, so a row would have to fan out into several sessions;
     the cross-platform matching is only as good as the viewer's own linking. Rejected as out of
     proportion to an admin read view.
4. **Keep showing `message_count_1min`/`1hour`.**
   - ❌ Rate-limit counters, not engagement. Rejected.

## Decision Outcome

**Chosen**: Option 1.

- **A row is a platform account.** `ListAll` (`services/auth-service/repository/viewer_repository.go`)
  full-outer-joins `viewer_sessions` and `viewer_platform_identities` on `(platform,
  platform_user_id)` and returns `AdminViewer` rows: `session_id` is null for an identity without a
  session, `viewer_id` is null for a session no identity was resolved for. Premium is read from the
  identity's viewer (the one the enricher reads), falling back to the session's. Search, platform,
  ban and premium filters apply to the joined row.
- **Ban and activity act on the session** (`/admin/viewers/:session_id/{ban,unban,activity}`).
  A row without a session shows "Not signed in" in their place: there is nothing to ban and no sent
  message to show.
- **Premium acts on the durable viewer** (`POST /admin/viewers/identities/:viewer_id/premium`,
  `SetViewerPremium(viewerID)`; 404 when no such viewer). It writes the ADR-0019 admin override and
  recomputes `viewers.is_premium`, with the optional ADR-0027 expiry.
- **Sessions follow their identity.** Every identity resolution in auth-service
  (`GetOrCreateViewerByPlatform`, `LinkPlatformToViewer` in
  `services/auth-service/repository/viewer_identity_repository.go`) points the session at the
  identity's viewer, including when the identity already existed or was moved to another viewer.
  Migration 125 repaired the rows written before.
- **Surface the linked streamer `user_id` when present**, linking to the streamer's admin record
  (ADR-0036).
- **A READ-ONLY per-session activity aggregate over `viewer_message_history`**, grouped by
  `viewer_session_id` (existing index) and joined to `users.username` via `streamer_user_id`,
  carrying `overlay_id` for deep links (ADR-0036).
- **Do not present `message_count_1min` / `message_count_1hour`.** The admin row no longer carries
  them.
- **No cross-platform dedup** and no new identity table: the existing `viewers` row already is the
  durable identity; collapsing a human's platform accounts into one row stays deferred.

## Consequences

### Positive
- Every viewer identity is reachable from the admin view and global search (ADR-0035), and premium
  can be granted to it whether or not it has ever signed in.
- Premium granted by an admin lands on the viewer the badge enricher reads.
- The viewer view has streamer/overlay context and real per-streamer usage counts at low cost.
- Misleading rate-limit counters are no longer shown as engagement.

### Negative
- One human with several platform accounts still appears as several rows.
- The list now includes every chatter engagement-service created, so the default (newest first) view
  is dominated by chatters who never signed in; the filters and search narrow it.
- The activity aggregate surfaces message metadata (channel, streamer, timestamps) in one more admin
  place; this is existing PII under existing admin access.

## Implementation

- **Data model**: no new table. Reuses `viewer_sessions` (migrations 011, 035 `viewer_id`, 040
  `user_id`), `viewers` / `viewer_platform_identities` (035), and `viewer_message_history` (011).
  Migration 125 re-points `viewer_sessions.viewer_id` at the identity's viewer (idempotent).
- **Backend** (auth-service): `ListAll` / `AdminViewer`, `SetViewerPremium(viewerID)`,
  `GetViewerActivity`, `linkSession` in the identity repository; routes in `cmd/main.go`.
  The api-gateway registers the admin viewer routes in `registerAdminViewerRoutes`
  (`services/api-gateway/cmd/viewer_routes.go`); before this revision the activity route was never
  registered there, so the Activity dialog 404'd.
- **Frontend**: `frontend/src/app/admin/viewers/page.tsx` renders the master-detail view (ADR-0036)
  keyed by `platform:platform_user_id`; `frontend/src/app/admin/search/page.tsx` lists the same rows.

## Related Decisions

- [ADR-0036](./0036-admin-url-addressable-selection.md) — the URL-addressable master-detail pattern
  this view links into (viewer → streamer account, viewer → overlays).
- [ADR-0019](./0019-split-streamer-viewer-premium.md) — viewer premium lives on the `viewers` row;
  this view grants it there directly.
- [ADR-0027](./0027-time-limited-admin-premium-overrides.md) — the optional grant expiry.
- [ADR-0035](./0035-admin-global-entity-search.md) — global search resolves viewers by
  username/`platform_user_id` into this view.
