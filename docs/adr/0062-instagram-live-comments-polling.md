# ADR-0062: Instagram Live comments via HTTP polling of `live_comments` behind a Page-linked IG credential

**Date**: 2026-09-11
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation)
**Deciders**: caesarakalaeii

## Context

Instagram Live is the sister surface to Facebook (ADR-0060, same Meta app,
same polling architecture) but a separate listener. Platform facts that shape
the design:

- The ingest surface is the IG Graph API, not the Page API: comments on an
  Instagram live broadcast are read via `live_comments` on the streamer's
  Instagram professional account (business or creator), which must be linked
  to a Facebook Page; and per the IG Comment node reference, comments can only
  be read while the broadcast runs.
- There is no public realtime comment API for the polling shape All-Chat
  uses; Meta's realtime surface is webhooks, which need a public endpoint and
  a per-deployment verify token.
- The credential that works is the Facebook Page token for the Page linked to
  the IG account (like Facebook, a Page token from a long-lived user token
  does not expire; the schema keeps `expires_at` for the 60-day user-token
  fallback path).
- Rate limits are BUC-based like Facebook's; no quota ledger.
## Decision

**Ingest: HTTP polling of `GET /{ig-user-id}/live_comments` in a new
`instagram-listener` service, no webhooks, no quota machinery.** The listener
resolves the IG user's currently-broadcast live media (`GET
/{ig-user-id}/live_media`, which per its reference returns *"only live video
IG Media being broadcast at the time of the request"* — an empty set IS the
not-live signal, a normal state, not an error) and polls
`/{ig-user-id}/live_comments` with `fields=id,text,timestamp,username,user` and
pagination via the **`comment_id` query parameter, which returns the comments
made AFTER that comment id** — an id cursor, not a `since` datetime (IG
comments cannot be filtered by timestamp; the IG media comments reference
states *"Comments cannot be filtered by timestamp"*). The cursor is persisted
per source in Redis under `instagram:source:state:` so a poller restart does
not re-publish the whole live backlog more than once.

**Separate service and token table, shared Meta app.** `instagram-listener`
runs on port 8100; pollers key on the IG user id (the source's `channel_id`).
The token lives in its own `instagram_oauth_tokens` table (migration 096),
keyed on `(user_id, ig_user_id)` — the IG identity is the user id and the two
platforms' re-consent and expiry semantics differ. The Instagram permissions
fold into the next App Review cycle (a separate submission would be a second
~20-day review).

**Token chain**: prefer the non-expiring Page-derived token (ADR-0060's
chain; store the Page token of the Page whose `instagram_business_account`
matches) with `expires_at = NULL`, falling back to the 60-day long-lived user
token only when the API shape requires it. Beyond the contract's
`instagram_basic` + `instagram_manage_comments`, additional scope is limited
to what account resolution requires (in practice `pages_show_list`, since the
IG user id is only obtainable by enumerating Pages).

**Read-only**, deliberately narrower than Facebook: no moderation write path,
no emotes, no quota subsystem. A poller 401/403 marks the source inactive and
emits a `platform:status` event prompting re-auth; an expired row
(`expires_at`) is treated as a missing credential, not a Graph error.

**Single pod, no leader election** (same argument as ADR-0060). **Fail-closed
source adds**: overlay-manager refuses an Instagram source unless
`instagram_oauth_tokens` has a row for that exact (user, ig_user_id) pair.

## Risks and Operator Actions

- The streamer must hold an **Instagram professional (business/creator)
  account linked to a Facebook Page** — a personal IG account cannot be
  connected at all.
- **App Review**: `instagram_manage_comments` needs Advanced Access before
  production traffic; dev-mode works against the app admins' own accounts.
  Fold into the same review cycle as Facebook's permissions.
- The `comment_id` cursor is id-based: it advances only when the poll returns
  data. That is correct (no comments = nothing missed), but it means a restart
  after a poller outage longer than the broadcast's lifetime replays nothing
  rather than failing — acceptable for a chat overlay.
- Live comments are only readable while the broadcast runs (IG Comment node
  limitation); the listener treats broadcast end as a normal state transition
  and clears the cursor.

## Consequences

- Adding Instagram costs one listener, one migration and one OAuth provider on
  an existing Meta app — minus quota, minus moderation, minus token refresh.
- A streamer whose IG token expires (the 60-day path) sees their source go
  inactive with a platform:status event; the fix is re-auth from the dashboard.
- There is no background refresh: like Facebook, the credential's lifecycle is
  "replaced on re-consent, invalidated by revocation".
- Out of scope, explicitly: moderation write path for Instagram, emotes, the
  quota subsystem.

## References

- [Live comments edge](https://developers.facebook.com/docs/instagram-platform/instagram-graph-api/live-comments/) (`comment_id` pagination), [IG User live media](https://developers.facebook.com/docs/instagram-platform/instagram-graph-api/reference/ig-user/live_media/), [IG Comment node](https://developers.facebook.com/docs/instagram-platform/instagram-graph-api/reference/ig-comment/) (live-read limitation), [Comment Moderation guide](https://developers.facebook.com/docs/instagram-platform/comment-moderation) (permissions matrix, Page token)
- ADR-0060 (the template), ADR-0008 (the `platform_instagram` gate), ADR-0032 (source liveness/status)
