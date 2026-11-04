# ADR-0088: Account and viewer data erasure

**Date**: 2026-11-02
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

GDPR Art. 17 gives a person the right to have their data erased. All-Chat had one
deletion path, `DELETE /api/v1/auth/me`, which ran `DELETE FROM users` and relied on
foreign keys. Code reading found it incomplete in four ways:

1. **It failed.** `share_requests.sender_user_id`, `recipient_user_id` and
   `sender_overlay_id` were `ON DELETE RESTRICT` (migration 030), and share rows are never
   deleted, only moved to a terminal status. Anyone who ever sent or received a share
   request got a 500, and so did deleting an overlay that was ever shared.
   `users.banned_by` and `banned_platform_ids.banned_by` (migration 015) had no delete
   rule, so an admin who ever banned someone could not delete their account either.
2. **Uploaded media stayed public.** Alert media lives in the public-read MinIO bucket
   `allchat-media` (served at `media.allch.at`). The account's `media_objects` rows
   cascaded, but nothing removed the objects, so every file stayed downloadable at its
   URL.
3. **The streamer's own viewer identity stayed.** `viewer_sessions.user_id` is only
   `SET NULL` (migration 040), and the viewer rows of the same platform account
   (`viewers`, platform identities, cosmetics, points, votes) were untouched.
4. **Viewers could not delete anything.** Viewer logout deletes one `viewer_sessions` row.
   There was no erasure at all for a viewer, including for someone who never signed in
   but voted, bet or earned points by chat command: engagement-service creates `viewers`
   and `viewer_platform_identities` rows for them on the fly.

## Decision Drivers

- Deletion must succeed for every account, and must be one transaction: half an erasure
  is worse than none, because the person is told it is done.
- What is erased must be stated precisely enough to quote in the privacy policy.
- Other people's data must stay consistent: open prediction pools, other viewers'
  ledgers, bans an admin issued, a recipient's view of a shared overlay.
- The media bucket is public; a file without an owner must stop being served even when
  the deletion that orphaned it happened before this change, or failed halfway.
- Keep the foreign keys as the primary mechanism; the codebase already relies on them.

## Considered Options

### Foreign keys that block deletion

1. **Change the delete rules in an idempotent migration (chosen).** Share requests
   `CASCADE` with either party or the shared overlay; `banned_by` becomes `SET NULL`.
2. Delete the share requests and null the ban columns in the handler first. Rejected: it
   fixes one caller (account deletion) and leaves overlay deletion and admin deletes
   broken, and the next RESTRICT key would 500 again.

### Media files

1. **Orphan sweep in media-service (chosen).** Every interval, list the bucket and remove
   each object whose key has no `media_objects` row.
   - ✅ Self-healing: covers account deletion, a failed MinIO removal on `DELETE /media`,
     and the files of accounts deleted before this change, whose rows are already gone.
   - ✅ No new coupling: auth-service needs no MinIO access and no call to media-service.
   - ❌ Removal lags deletion by up to one interval, and the sweep lists the whole bucket.
2. Deletion hook: auth-service calls media-service (or MinIO) before deleting the user.
   Rejected: a cross-service call inside account deletion fails the whole erasure when
   MinIO is down, needs service-to-service auth that is not wired (`shared/signing`), and
   never cleans up what is already orphaned.
3. Outbox: an `ON DELETE` trigger on `media_objects` records the key, media-service drains
   it. Rejected: precise and fast, but it cannot see the orphans that exist today, and it
   adds a table and a trigger for what the sweep already covers.

### The deleting streamer's viewer data

1. **Erase it with the account (chosen).** "Delete my account" means all of the person's
   data; the viewer identity of a platform account the user has proven it controls is
   theirs.
2. Keep it as a separate account. Rejected: the person would have to know about and find
   a second deletion path, and the viewer row stays linked to their platform accounts.

## Decision Outcome

### Migration 119

`119_account_erasure.sql` sets `share_requests.sender_user_id`, `recipient_user_id` and
`sender_overlay_id` to `ON DELETE CASCADE`, and `users.banned_by` and
`banned_platform_ids.banned_by` to `ON DELETE SET NULL` (`banned_platform_ids.banned_by`
becomes nullable). Constraints are matched by column, not by name, and replaced only
when the rule is wrong, so the re-run on every pod start takes no locks. An
`AFTER DELETE` trigger on `overlays` removes the `shared_overlay` chat sources other
users created for that overlay (their `channel_id` is the overlay id and has no foreign
key), so a recipient is not left with a source pointing at nothing.

An inventory of every foreign key in the migrated schema found no other `RESTRICT` or
`NO ACTION` rule on a key referencing `users`, `overlays` or `viewers`.

### Streamer account deletion (`DELETE /api/v1/auth/me`)

One transaction in `UserRepository.Delete` (auth-service `repository/erasure.go`):

- **Erased through foreign keys** (as before, now without failures): the `users` row and
  every row keyed to the account or its overlays: overlays and their configs, sources,
  event, TTS, credit roll, alert, goal and list settings, stream sessions, polls,
  predictions and points economies (with every viewer's votes, wagers, balances and
  ledger rows in them), the messages viewers sent to this streamer through All-Chat
  (`viewer_message_history`), OAuth tokens of every platform, Discord guilds and identity,
  Patreon token and subscription, API and device tokens, chat commands, bots and timers,
  moderator grants, the ambassador showcase, media registry rows, share requests in both
  directions, and impersonation audit rows naming the user.
- **Erased explicitly** (no foreign key to cascade through): the user's own viewer
  identity, which is every viewer mapped to a platform account the user has proven it
  controls: `users.twitch_id`, `kick_id`, `google_id`, the Twitch, Kick and YouTube
  accounts connected through OAuth (`twitch_oauth_tokens.twitch_user_id`,
  `kick_oauth_tokens.kick_user_id`, `youtube_oauth_tokens.channel_id`) and the viewer
  sessions the viewer callback linked to the account (`viewer_sessions.user_id`). Each
  such viewer is erased as in viewer deletion below. Also the overlays' alert history
  (`alert_events`, kept on overlay deletion by ADR-0068, unreadable once the account is
  gone) and YouTube quota bookkeeping (`youtube_quota_audit_log` rows of the overlays,
  `youtube_channel_quota` rows of the user).
- **Set to NULL, kept**: `banned_by` on bans the user issued as an admin (the bans stay in
  force), and the user as requester, submitter or reviewer on localization records (migration 097).
- **Kept**: the moderation audit log (`moderation_actions`), which ADR-0017 keeps through
  account deletion for abuse forensics. Per-channel liveness state (`stream_history`) is
  keyed by public platform channel ids, shared by every overlay that sources the channel,
  and not linked to an account.

### Viewer self-deletion (`DELETE /api/v1/auth/viewer/me`)

New endpoint on auth-service's viewer routes (viewer JWT, the same auth as the other
viewer endpoints) and on the gateway's protected group. `ViewerIdentityRepository.EraseViewer`
resolves the viewers to erase from the token: the `viewer_id` claim (empty on tokens
issued before viewer identities existed) and the viewer the token's
`(platform, platform_user_id)` maps to. In one transaction it deletes every viewer session
of those viewers or of any platform identity linked to them, which cascades to
`viewer_message_history`, and the `viewers` rows, which cascade to
`viewer_platform_identities`, `viewer_cosmetics`, `viewer_points`, `points_transactions`,
`poll_votes`, `prediction_entries` and the viewer-owned `patreon_oauth_tokens` and
`premium_subscriptions` rows. After the commit the bearer token is blacklisted and the
message-processor's `viewer:identity:{platform}:{id}` cache entries are dropped, so the
viewer's cosmetics stop showing in chat at once.

A chatter who never signed in is covered: engagement-service writes the same
`(platform, platform_user_id)` keys viewer login uses (Twitch user id, YouTube channel id,
Kick numeric user id), so signing in as a viewer with the same platform account resolves
to those rows, and deleting erases them.

Other people stay consistent because nothing about them is stored on the erased rows:
poll tallies and prediction pools are computed from the remaining votes and wagers, a
payout or refund already made is a row in the recipient's own ledger, and a wager in an
open prediction leaves the pool together with the balance it came from.

The settings page `/settings/viewer` has a "Delete my viewer data" control behind a
confirmation dialog.

### Media files

media-service sweeps the bucket at startup and then every `MEDIA_ORPHAN_SWEEP_INTERVAL`
(default 15 minutes) on every replica (`orphans/`): it lists the bucket and removes each
object that has no `media_objects` row. Only keys of the service's own
`{user_id}/{uuid}/{filename}` shape are considered. A failed registry lookup stops the
sweep with nothing removed; a failed removal is retried by the next sweep. A live upload
cannot be removed by race, because the row is written before the upload URL is handed
out: any object the listing sees had its row committed before the lookup.

## Consequences

### Positive

- Every account can be deleted, and an overlay that was ever shared can be deleted.
- Viewers, including chatters who never signed in, can erase their data themselves.
- A deleted account's files stop being served within one sweep interval, and files
  orphaned before this change are removed by the first sweep after the deploy.

### Negative

- Files stay downloadable for up to `MEDIA_ORPHAN_SWEEP_INTERVAL` after the deletion
  (plus any cache in front of `media.allch.at`), and a presigned upload URL issued just
  before the deletion stays valid for `MEDIA_PRESIGN_EXPIRY` (5 minutes); a file uploaded
  through it is removed by the next sweep.
- Each replica lists the whole bucket every interval. Fine at today's size; an advisory
  lock (one replica sweeps) is the upgrade if listing gets expensive.
- Someone who keeps chatting after deleting their viewer data in a channel with All-Chat
  polls or points gets a new viewer row on their next vote, bet or earned point: that is
  new processing, not a failure to erase.
- Viewer bans live on `viewer_sessions` rows (`is_banned`), so erasing the session lifts
  the ban. Viewer logout already deletes that row and has the same effect; a ban that
  outlives the session needs its own record (not part of this change).
- A sender deleting their account removes the share relationship and the recipient's
  source silently; the recipient is not notified.

## Implementation

- `migrations/119_account_erasure.sql` (+ `_down.sql`).
- auth-service: `repository/erasure.go` (`UserRepository.Delete`,
  `ViewerIdentityRepository.EraseViewer`), `handlers.ViewerAuthHandler.HandleDeleteViewer`,
  `dropViewerIdentityCache` (also used by the cosmetics handler and account deletion),
  route `DELETE /viewer/me`.
- api-gateway: `cmd/viewer_routes.go` (`DELETE /api/v1/auth/viewer/me`).
- media-service: `orphans/` (sweep), `MinioStore.Keys`, `MediaRepository.ExistingKeys`,
  `MEDIA_ORPHAN_SWEEP_INTERVAL`.
- frontend: `src/app/settings/viewer/page.tsx` (`DeleteViewerDataCard`),
  `viewerApi.deleteViewerData`, `settings.viewer.delete*` strings (en, de).
- Tests: `repository/erasure_integration_test.go` (auth-service) and
  `orphans/sweep_integration_test.go` (media-service) against Postgres with every
  migration applied, run by CI's integration job; handler, route, sweep and page unit
  tests.

## Related Decisions

- [ADR-0017](./0017-chat-moderation-write-path.md): the moderation audit log that
  survives account deletion.
- [ADR-0019](./0019-split-streamer-viewer-premium.md): viewer-owned Patreon rows.
- [ADR-0068](./0068-event-alerts-platform.md): `alert_events` survives overlay
  deletion; account deletion erases it.
