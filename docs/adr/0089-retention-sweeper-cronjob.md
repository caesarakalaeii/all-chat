# ADR-0089: Enforce data retention with a sweeper CronJob

**Date**: 2026-11-02
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation; backups keep deleted rows for up to about 60 days)
**Deciders**: caesarakalaeii

## Context and Problem Statement

The privacy policy promises retention periods: messages viewers send through
All-Chat are deleted after one hour, YouTube quota audit logs after 30 days,
expired OAuth tokens are cleaned up after 7 days. None of these were enforced.

The cleanups existed only as SQL functions scheduled through pg_cron, inside
`IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_cron')` blocks:

- `cleanup_old_message_history()` (migration 046): `viewer_message_history`
  older than 1 hour, every 15 minutes.
- `cleanup_expired_oauth_tokens()` (047, extended by 056 and 063): token rows
  whose access-token expiry is 7+ days old, daily.
- `cleanup_old_youtube_audit_logs()` (016): `youtube_quota_audit_log` older
  than 30 days. Never scheduled, and it cannot run: `RETURNING COUNT(*)` is an
  error.

pg_cron is not installed (the `allchat-cluster` postInitSQL creates only
`uuid-ossp` and `pg_trgm`), and no Go code or CronJob calls the functions. So
every table grew without bound. The same was true of the device-link sweep
(`SweepExpiredLinkRequests`, ADR-0049), a repository method with no caller.

The token rule was also wrong. `token-refresh-service` keeps credentials alive
by refreshing them, and when a refresh is rejected it pushes the expiry 30 days
into the future instead of leaving it in the past. An access token whose expiry
lies in the past says nothing about whether the credential is dead: any holder
of a valid refresh token (token-refresh-service inside its 48-hour window, or a
consumer's reactive refresh) can still renew it. Deleting by access-token expiry
would have deleted working credentials.

## Decision Drivers

- The periods in the privacy policy must actually hold, and be checked by a
  test against the real schema.
- Never delete a credential that any code path could still refresh or use.
- The first run removes months of backlog: no long locks, no table scans per
  batch.
- The database has little connection headroom (ADR-0039).
- Safe to re-run and to run twice at once.

## Considered Options

1. **Install pg_cron and keep the SQL functions.** Needs the extension in the
   cluster's Postgres image and in `shared_preload_libraries` (a restart), keeps the rules
   in plpgsql that no test exercises, and still needs the token rule rewritten.
   Rejected.
2. **A ticker inside an existing long-running service** (auth-service or
   token-refresh-service). Every replica would run it, so it needs leader
   election or an advisory lock, and its schedule is tied to that service's
   rollouts. Rejected.
3. **A `retention-sweeper` binary in the auth-service image, run by a
   Kubernetes CronJob every 15 minutes.** Same shape as the key-rotator
   (`cmd/key-rotator` + `key-rotator-cronjob.yaml`):
   no new image, no new Deployment, one short-lived connection per run.
   **Chosen.**

## Decision Outcome

`services/auth-service/cmd/retention-sweeper` holds the whole policy as a list
of (table, predicate) rules and deletes matching rows in batches:

```sql
DELETE FROM <table> WHERE id IN (
  SELECT id FROM <table> WHERE <predicate> LIMIT $1 FOR UPDATE SKIP LOCKED)
```

Each batch (default 1000 rows) is its own short transaction, repeated until a
batch comes back short, with a pause between batches. `SKIP LOCKED` lets an
overlapping run or a concurrent writer proceed without waiting, and the locking
subquery re-checks the predicate against the newest row version, so a row that
a refresh moved out of the window in the meantime is kept. One table failing is
logged and the rest still run; the exit code is then non-zero. Each table's
deleted row count is logged. The sweeper opens a single connection
(`application_name=retention-sweeper`), not a pool.

The CronJob (caesar-deployment `retention-sweeper-cronjob.yaml`) runs every 15
minutes with `concurrencyPolicy: Forbid` and an `activeDeadlineSeconds` below
the interval, so a slow run never makes the next one skip. A run cut short
keeps every batch it committed. Its `backoffLimit` is 0, so a failed run is
not retried inside its own slot; the next scheduled run is the retry.

### Retention rules

`alert_events` and `moderation_actions` had no retention period at all; this
decision sets 30 and 90 days.

| Table | Deleted when |
|---|---|
| `viewer_message_history` | `created_at` more than 1 hour ago |
| `youtube_quota_audit_log` | `timestamp` more than 30 days ago |
| `alert_events` | `created_at` more than 30 days ago (event log; list overlays rank from Redis and the data export caps what it returns) |
| `moderation_actions` | `created_at` more than 90 days ago (audit log; no product flow reads it back, the data export caps what it returns) |
| `device_link_requests` | expired more than 1 hour ago and never minted a device token (a request that did is that token's audit trail) |

### OAuth credentials: "dead" per table

A credential is deleted only when nothing can renew it, 7 days after it stopped
working. "Expired" is not "dead": an access token past its expiry is renewed by
anyone holding a working refresh token, and token-refresh-service records a
rejected refresh by pushing the expiry 30 days ahead (for `invalid_client`, our
own configuration error, as well as for revoked grants). So expiry alone is a
rule only where nothing can refresh:

| Table | Rule | Why |
|---|---|---|
| `tiktok_oauth_tokens` | `expiry` more than 7 days ago | Legacy: no code reads, writes or refreshes the table (TikTok chat needs no OAuth), so the access token's expiry is the moment it died. |
| `kick_oauth_tokens` | `kick_user_id IS NULL` and `expiry` more than 7 days ago | Legacy listener rows. Moderation and chat send only resolve and refresh rows with a numeric `kick_user_id`; the kick-listener only reads unexpired rows and never refreshes; token-refresh-service does not cover Kick. Rows with `kick_user_id` are refreshed on use at any age and are kept. |
| `instagram_oauth_tokens` | `expires_at` more than 7 days ago | No refresh grant exists, so an expired token is dead. `expires_at` is NULL for the non-expiring Page-derived tokens the connect flow stores today, so no row matches yet. |

No rule proves death in the other credential tables, so the sweeper leaves
them alone. Their rows go when the account is deleted (every one cascades from
`users`); short of that, only Patreon (disconnect in Settings) and chat bot
accounts (remove the bot) have a delete path:

- `youtube_oauth_tokens`, `twitch_oauth_tokens`: refreshed by token-refresh-service
  (48-hour window) and reactively by moderation, chat send, goals and the
  YouTube listeners at any age; a revoked grant keeps getting its expiry pushed
  forward. The rows are also the channel-ownership anchor (official-API opt-in,
  delegated moderation, bot-reply gating, EventSub credential choice).
- `patreon_oauth_tokens`: refreshed by payment-service at any age, and the row is
  the webhook subject mapping premium depends on.
- `facebook_oauth_tokens`: Page tokens have no expiry column; death is invisible
  to the database.
- `mod_oauth_credentials`: refreshed by token-refresh-service and moderation; the
  row is kept on purpose so the UI asks to reconnect.
- `chat_bot_accounts`: Twitch bots send with the app token, so an expired bot
  token still works; the row selects the sender.
- `viewer_sessions` and `users`: account rows (login session, ban state, premium
  link, message history cascade), not just credentials. The old 047 function
  deleted `viewer_sessions` by expiry, which would also have lifted bans.

Recording a rejected refresh precisely (rejected `invalid_grant`, not our own
`invalid_client`, compared against the exact refresh token that was rejected)
would make revoked YouTube/Twitch/moderator grants deletable. That changes
token-refresh-service and every reactive refresher, so it is not part of this
decision.

### Migration 118

Drops the three pg_cron-only functions (and unschedules their jobs if pg_cron
is ever installed) and adds the `created_at` indexes the batched deletes need on
`alert_events` and `moderation_actions`. It corrects forward instead of editing
016/046/047/056/063: migrations are replayed on every pod start, so those
files recreate the functions and 118 drops them again on each run.

## Consequences

### Positive

- The privacy policy's periods hold, every 15 minutes, and an integration test
  runs the binary against the migrated schema in CI.
- No credential that can still be refreshed is deleted. The swept legacy Kick
  rows still mark a channel as owned for chat-command replies, but every reply
  path needs `kick_user_id`, so those replies failed anyway; now they are not
  attempted.
- No new service, image or pooled connections.

### Negative

- The admin viewer Activity dialog (ADR-0034) now only ever covers the last
  hour, because that is all that is kept. Its copy says so.
- Nothing alerts when the CronJob stops running; a failed run shows only as a
  failed Job (three are kept).
- Most OAuth credentials are deleted only with the account, not after expiry;
  the privacy policy has to say that instead of promising a 7-day cleanup for
  all tokens.
- The 1-hour message promise holds to within one interval: a message is deleted
  between 60 and about 75 minutes after it was sent.
- The sweeper deletes rows from the live database only. The Postgres backups
  (ADR-0003 → Backups) keep WAL and base backups for a 30-day recovery window
  on Hetzner Object Storage, mirrored nightly to Backblaze B2, and both
  buckets keep what Barman expires for another 30 days as noncurrent versions, so a deleted row survives in the
  backups for up to about 60 days. The privacy policy says so in its retention
  section (Backups row) and names the providers in Section 5.7.

## Implementation

- `services/auth-service/cmd/retention-sweeper/main.go`, built and shipped by
  `services/auth-service/Dockerfile` as `/app/retention-sweeper`.
- `services/auth-service/cmd/retention-sweeper/main_test.go`
  (`//go:build integration`), run by the `test-engagement-integration` CI job
  against a Postgres with every migration applied.
- `migrations/118_retention_sweeper.sql` (+ `_down`).
- `SweepExpiredLinkRequests` removed from `repository/device_token_repository.go`;
  its rule moved into the sweeper.
- caesar-deployment: `apps/workloads/all-chat/retention-sweeper-cronjob.yaml`.

## Related Decisions

- [ADR-0039](./0039-database-connection-budget.md): connection budget.
- [ADR-0016](./0016-linked-twitch-credentials.md) and migration 056,
  [ADR-0018](./0018-premium-entitlements-via-patreon.md): extended the 047
  function, which never ran.
- [ADR-0034](./0034-admin-viewer-identity-model.md): the activity aggregate
  over `viewer_message_history`.
