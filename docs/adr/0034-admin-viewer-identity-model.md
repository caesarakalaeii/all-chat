# ADR-0034: Admin Viewer Identity Model

**Date**: 2026-07-17
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

A "viewer" in All-Chat is not a durable person. The `viewer_sessions` table
(`migrations/011_viewer_authentication.sql`) stores **one OAuth session per
`(platform, platform_user_id)`** (`UNIQUE(platform, platform_user_id)`). A single human who
connects Twitch and YouTube, or reconnects on a new platform account, produces several
independent `viewer_sessions` rows. Premium and moderation state key on a linked `viewer_id`,
and migration 040 (`040_viewer_sessions_user_link.sql`) added `viewer_sessions.user_id` to link a
viewer who *also* has a streamer (`users`) account so the badge enricher can resolve their
All-Chat status.

The one place a viewer's actual behavior is recorded is `viewer_message_history`
(`viewer_session_id`, `streamer_user_id`, `overlay_id`, `channel_id`/`channel_name`, `message_text`,
`sent_at`), indexed on `viewer_session_id`. That table was effectively **write-only** — the only
thing that ever read it back was the DSGVO data export.

The result: the admin viewer view was, in the maintainer's words, "basically useless." It listed raw
sessions with no usage context (which streamers does this viewer actually chat in?), no linkage to a
streamer account, and it surfaced the rate-limit counters `message_count_1min` / `message_count_1hour`
as if they were engagement numbers — they are not, they are throttle bookkeeping that resets on a timer.

## Decision Drivers

- Operate on the unit that admin actions actually act on. Ban and premium act on a `viewer_sessions`
  row (a `(platform, platform_user_id)` identity), so the admin view must too.
- Give the view real context — which streamers/overlays a viewer participates in, and whether the
  viewer is also a streamer — without a schema-heavy identity project.
- Keep it cheap: reads must ride existing indexes, not new scans.
- Stop presenting misleading numbers.
- Do not paint ourselves into a corner: a future durable cross-session identity must be able to layer
  on top without reworking this.

## Considered Options

1. **Operate on raw `viewer_sessions`, surface the linked streamer `user_id`, and add a read-only
   per-session activity aggregate over `viewer_message_history`; explicitly defer durable identity.**
   - ✅ Matches the ban/premium unit; adds streamer/overlay context and real usage counts using the
     existing `idx_viewer_message_history_viewer_session_id`; no new PII; no new tables; a durable
     identity can be built later on top.
   - ❌ A human with several sessions still shows as several rows (no dedup yet).
2. **Introduce a durable `viewer_identity` table now and dedup sessions into people.**
   - ✅ One row per human; cross-session history in one place.
   - ❌ Large schema + backfill + a matching heuristic (which sessions are the same person?) that is
     itself error-prone, all to fix an *admin read view*. Ban/premium still key on sessions, so the
     write path would have to be reconciled too. Rejected as premature — out of proportion to the
     problem.
3. **Leave the view on `viewer_sessions` but keep showing `message_count_1min`/`1hour`.**
   - ✅ No work.
   - ❌ Those are rate-limit counters, not engagement; presenting them as activity is the misleading
     behavior we are trying to remove. Rejected.

## Decision Outcome

**Chosen**: Option 1.

- **The admin viewer view operates on raw `viewer_sessions` rows** — the same `(platform,
  platform_user_id)` unit that ban and premium act on, so what an admin sees is what an admin acts on.
- **Surface the linked streamer `user_id` when present.** If `viewer_sessions.user_id` is set
  (migration 040), the view shows that this viewer is also a streamer and links to the streamer's admin
  record (via the URL-addressable pattern of ADR-0036).
- **Add a READ-ONLY per-session activity aggregate over `viewer_message_history`.** Grouped by
  `viewer_session_id` (using the existing index, so it is cheap), joined to `overlays` /
  `users.username` via `streamer_user_id`, to answer *"whose chats does this viewer participate in,
  and how much."* This makes `viewer_message_history` an admin-queryable **read surface** for the first
  time. It stays strictly read-only from the admin view.
- **Stop presenting `message_count_1min` / `message_count_1hour` as engagement.** They are rate-limit
  bookkeeping; the real per-streamer usage now comes from the `viewer_message_history` aggregate.
- **Do NOT introduce a durable cross-session viewer-identity table.** Session dedup — collapsing a
  human's several sessions into one person — is explicitly deferred.

## Consequences

### Positive
- The viewer view gains streamer/overlay context and real, per-streamer usage counts at low cost
  (indexed group-by, no new scan).
- `viewer_message_history` becomes a useful admin read surface instead of a write-only DSGVO artifact,
  with no new PII beyond what admins already have access to.
- Misleading rate-limit counters are no longer shown as engagement.
- A future durable-identity model can layer on top of this view without reworking it.

### Negative
- No session dedup yet: one human with several platform sessions still appears as several viewer rows.
  This is a deliberate deferral, not an oversight.
- The activity aggregate reads message metadata (channel, streamer, timestamps) into the admin view;
  this is existing PII under existing admin access, but it is now surfaced in one more place and should
  be treated accordingly.

## Implementation

- **Data model**: no schema change. Reuses `viewer_sessions` (migration 011, `UNIQUE(platform,
  platform_user_id)`), `viewer_sessions.user_id` (migration 040), and `viewer_message_history`
  (migration 011, `idx_viewer_message_history_viewer_session_id`).
- **Backend**: admin viewer list/detail endpoints add a read-only per-`viewer_session_id` aggregate
  over `viewer_message_history` joined to `users.username`, plus the linked-streamer `user_id`.
- **Frontend**: `frontend/src/app/admin/viewers/page.tsx` renders the master-detail viewer view
  (ADR-0036), showing linked-streamer context and the activity aggregate, and drops the
  `message_count_1min`/`1hour` presentation.

## Related Decisions

- [ADR-0036](./0036-admin-url-addressable-selection.md) — the URL-addressable master-detail pattern
  this view links into (viewer → streamer account, viewer → overlays).
- [ADR-0019](./0019-split-streamer-viewer-premium.md) — viewer premium keys on the linked `viewer_id`;
  this view acts on the same session unit.
- [ADR-0035](./0035-admin-global-entity-search.md) — global search resolves viewers by
  username/`platform_user_id` into this view.
