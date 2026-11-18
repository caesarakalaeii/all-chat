# ADR-0089: Enforce data retention with a sweeper CronJob

**Date**: 2026-11-02
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation; backups keep deleted rows for up to about 60 days)
**Deciders**: caesarakalaeii

## Context

The privacy policy's retention periods (viewer messages 1 hour, YouTube quota audit logs
30 days, expired OAuth tokens 7 days) were never enforced: the cleanups were pg_cron
functions behind `IF EXISTS pg_extension` blocks (migrations 016, 046, 047, extended by
056 and 063), and pg_cron is not installed, so every table grew unbounded (the 016
YouTube-audit cleanup also cannot run: `RETURNING COUNT(*)` is an error; and the
device-link sweep, ADR-0049, was a repository method with no caller). The token rule was
also wrong: token-refresh-service pushes a rejected refresh's expiry 30 days ahead, and
anyone holding a working refresh token can renew the credential, so access-token expiry
says nothing about whether it is dead. Constraints: the policy periods hold, checked by a
test against the real schema; never delete a refreshable credential; no long locks on the
backlog run; connection headroom is tight (ADR-0039); runs are re-runnable and
overlap-safe.

## Considered options

- Install pg_cron and keep the SQL functions: needs the extension in the image and
  `shared_preload_libraries` (a restart), keeps untested plpgsql rules, and the token rule
  still needs rewriting. Rejected.
- A ticker inside an existing long-running service: every replica would run it, needing
  an advisory lock, and its schedule is tied to that service's rollouts. Rejected.
- A `retention-sweeper` binary in the auth-service image run by a Kubernetes CronJob
  every 15 minutes, the same shape as the key-rotator: no new image or Deployment, one
  short-lived connection per run. Chosen.

## Decision

`services/auth-service/cmd/retention-sweeper` holds the policy as (table, predicate) rules
and deletes in batches:


```sql
DELETE FROM <table> WHERE id IN (
  SELECT id FROM <table> WHERE <predicate> LIMIT $1 FOR UPDATE SKIP LOCKED)
```

Each batch (default 1000 rows) is a short transaction, repeated until short, with a pause
between batches; the locking subquery re-checks the predicate against the newest row
version, so a row a refresh moved out of the window is kept. A failing table is logged
and the rest still run (non-zero exit); per-table counts are logged; one connection
(`application_name=retention-sweeper`), not a pool. The CronJob (caesar-deployment `retention-sweeper-cronjob.yaml`) runs every 15 minutes
with `concurrencyPolicy: Forbid` and `backoffLimit: 0`, so the next scheduled run is the
retry and a cut-short run keeps committed batches.

Rules: `viewer_message_history` after 1 hour; `youtube_quota_audit_log` and `alert_
events` after 30 days; `moderation_actions` after 90 days; `device_link_requests` one
hour after expiry when no device token was minted. OAuth credentials: only provably dead
rows are swept, 7 days after death: `tiktok_oauth_tokens` (legacy, no code touches it),
`kick_oauth_tokens` with NULL `kick_user_id` (legacy listener rows), and
`instagram_oauth_tokens` (no refresh grant; NULL `expires_at` never matches). Every other
credential table stays: `youtube_`/`twitch_oauth_tokens` refresh at any age and anchor
channel ownership; `patreon_oauth_tokens` is refreshed by payment-service; Facebook Page
tokens have no expiry column; `mod_oauth_credentials` is kept so the UI asks to
reconnect; `chat_bot_accounts` sends with an app token; `viewer_sessions`/`users` are
account rows. Their rows go on account deletion, Patreon disconnect or bot removal.
Recording rejected `invalid_grant` refreshes against the exact rejected refresh token
would make revoked grants deletable, but changes token-refresh-service and every reactive
refresher, so it is out of scope.

**Migration 118** drops the three pg_cron-only functions (and unschedules their jobs if
pg_cron ever appears) and adds the `created_at` indexes the batched deletes need on
`alert_events` and `moderation_actions`, correcting forward because migrations replay on
every pod start.

## Consequences

- The privacy policy's periods hold every 15 minutes, verified by an integration test
  against the migrated schema in CI; no new service, image or pooled connections.
- The admin viewer Activity dialog (ADR-0034) covers only the last hour (its copy says
  so); nothing alerts when the CronJob stops (a failed run is a failed Job, three kept);
  most OAuth credentials are deleted only with the account, so the policy says that
  instead of a 7-day cleanup; a message lives 60 to 75 minutes; and deleted rows survive
  in the Postgres backups (30-day recovery window plus 30-day noncurrent versions on
  Hetzner Object Storage and Backblaze B2, ADR-0003) for up to about 60 days, which the
  privacy policy says.

## Implementation

`services/auth-service/cmd/retention-sweeper/main.go` (shipped as `/app/retention-sweeper`;
integration test in the `test-engagement-integration` CI job against a fully migrated
Postgres); `migrations/118_retention_sweeper.sql`; `SweepExpiredLinkRequests` removed from
`repository/device_token_repository.go`; caesar-deployment
`retention-sweeper-cronjob.yaml`. Related: ADR-0039 (connection budget), ADR-0034
(activity aggregate), ADR-0003 (backups).
