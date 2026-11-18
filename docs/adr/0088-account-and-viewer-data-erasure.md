# ADR-0088: Account and viewer data erasure

**Date**: 2026-11-02
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context

GDPR Art. 17 erasure. The single path `DELETE /api/v1/auth/me` ran `DELETE FROM users`
relying on foreign keys and failed four ways: 500 for anyone who ever sent or received a
share request (`share_requests` was `ON DELETE RESTRICT`, migration 030) or, as an
admin, banned someone (`banned_by` had no rule); uploaded alert media stayed public at
`media.allch.at` after its registry rows cascaded; the deleting streamer's own viewer
identity (`viewer_sessions.user_id` only `SET NULL`) and its viewer rows stayed; and
viewers, including chatters who never signed in (engagement-service creates `viewers`
rows on the fly), had no erasure at all. Constraints: one successful transaction per
account; a precise, quotable erasure list; other people's data stays consistent; an
orphaned public file stops being served; foreign keys stay the primary mechanism.

## Considered options

- Fix the blocking foreign keys in an idempotent migration (chosen) rather than nulling
  rows in the handler first (fixes one caller, leaves overlay and admin deletes broken,
  and the next RESTRICT key 500s again).
- Media: an orphan sweep in media-service (chosen) instead of a deletion hook (a
  cross-service call inside account deletion fails the erasure when MinIO is down and
  never cleans existing orphans) or an `ON DELETE` outbox trigger (precise but blind to
  already-orphaned files, and the sweep already covers it).
- The deleting streamer's viewer data is erased with the account: "delete my account"
  means all of the person's data; keeping it as a separate viewer account leaves a row
  linked to their platform accounts behind a second deletion path they must find.

## Decision

**Migration 119** sets `share_requests.sender_user_id`, `recipient_user_id` and
`sender_overlay_id` to `ON DELETE CASCADE` and `users.banned_by` /
`banned_platform_ids.banned_by` to `ON DELETE SET NULL` (the latter becomes nullable);
constraints are matched by column, replaced only when wrong, so the re-run on pod start
takes no locks. An `AFTER DELETE` trigger on `overlays` removes other users'
`shared_overlay` chat sources of that overlay (their `channel_id` is the overlay id with
no foreign key). A schema inventory found no other `RESTRICT`/`NO ACTION` key referencing
`users`, `overlays` or `viewers`.

**Streamer (`DELETE /api/v1/auth/me`)** is one transaction in `UserRepository.Delete`:
everything keyed to the account or its overlays cascades (overlays, sources, engagement
settings, sessions, polls, predictions, points economies, viewer message history, OAuth
and Patreon tokens, API/device tokens, chat commands/bots/timers, moderator grants,
ambassador showcase, media registry, share requests, impersonation audit rows). Erased
explicitly: the user's own viewer identity, every viewer mapped to a platform account the
user controls (the `twitch_id`/`kick_id`/`google_id` columns, platform ids in the OAuth
token tables, `viewer_sessions.user_id`), plus `alert_events` and YouTube quota
bookkeeping. Set to NULL: `banned_by` on the admin's issued bans (they stay in force) and
localization requester/submitter/reviewer slots. Kept: `moderation_actions` (ADR-0017,
abuse forensics) and `stream_history` (keyed by public channel ids shared across
overlays).

**Viewer (`DELETE /api/v1/auth/viewer/me`)** on auth-service viewer routes and the
gateway protected group. `ViewerIdentityRepository.EraseViewer` resolves the viewers from
the token (`viewer_id` claim plus the viewer its `(platform, platform_user_id)` maps to)
and in one transaction deletes their sessions (cascades `viewer_message_history`) and the
`viewers` rows (cascade identities, cosmetics, points, `poll_votes`,
`prediction_entries`, viewer Patreon rows). After commit the token is blacklisted and
message-processor's `viewer:identity:*` cache entries dropped, so cosmetics stop
rendering immediately. Chatters who never signed in are covered because
engagement-service writes the same platform keys viewer login uses. Others stay
consistent: tallies and pools recompute from remaining rows, a payout is a row in the
recipient's ledger, a wager leaves the pool with its balance. `/settings/viewer` offers
it behind a confirmation dialog.

**Media.** media-service sweeps the bucket at startup and every
`MEDIA_ORPHAN_SWEEP_INTERVAL` (default 15 min) per replica (`orphans/`), removing each
object of the `{user_id}/{uuid}/{filename}` key shape with no `media_objects` row. A
failed lookup stops the sweep; a failed removal is retried next time. No race: the row is
written before the presigned URL is handed out.

## Consequences

- Every account and every ever-shared overlay can be deleted; viewers, including
  chatters who never signed in, can erase themselves.
- Files leave the bucket within one sweep interval (plus fronting cache), including
  pre-existing orphans; a presigned URL issued just before deletion stays valid
  `MEDIA_PRESIGN_EXPIRY` (5 min) and its upload is swept next.
- Each replica lists the whole bucket per interval (an advisory lock is the upgrade if
  listing gets expensive). Chatting again after viewer erasure creates a new viewer row,
  new processing rather than a failed erasure; a session ban is lifted with the session,
  as logout already did; and a sender deleting their account removes the share
  relationship and the recipient's source silently.

## Implementation

- `migrations/119_account_erasure.sql`; auth-service `repository/erasure.go`
  (`UserRepository.Delete`, `ViewerIdentityRepository.EraseViewer`),
  `handlers.ViewerAuthHandler.HandleDeleteViewer`, `dropViewerIdentityCache`, route
  `DELETE /viewer/me`; api-gateway `cmd/viewer_routes.go`; media-service `orphans/`;
  frontend `settings/viewer` page and `settings.viewer.delete*` strings.
- Tests: `repository/erasure_integration_test.go` and `orphans/sweep_integration_test.go`
  against a fully migrated Postgres in CI.
