# ADR-0062: Instagram Live comments via HTTP polling of `live_comments` behind a Page-linked IG credential

**Date**: 2026-09-11
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

All-Chat aggregates live chat from Twitch, YouTube, Kick, TikTok, Discord,
Owncast, GoodGame, Picarto, Rumble and (Phase 4, ADR-0060) Facebook. Instagram
Live is the last surface in the platform expansion plan (Phase 6): the sister
surface to Facebook — same Meta app, same polling architecture — but a separate
listener, not a config flip on facebook-listener.

Facts about Instagram's platform that shape the design:

- **The ingest surface is the IG Graph API, not the Page API.** Comments on an
  Instagram live broadcast are read via `GET /{ig-user-id}/live_comments` on
  the streamer's Instagram professional account (business or creator), which
  must be linked to a Facebook Page. The IG Comment node reference states the
  constraint outright: *"Comments on live video IG Media can only be read
  while the IG Media upon which the comment was created is being broadcast."*
- **There is no public realtime comment API for the polling shape All-Chat
  uses.** Meta's realtime surface for Instagram is webhooks (`live_comments`
  is a subscribable Instagram webhook field), and webhooks need a public HTTPS
  endpoint plus a per-deployment verify token. All-Chat already polls Facebook
  live-video comments for exactly this reason (ADR-0060), and the Facebook
  platform's own reference recommends continual polling.
- **The credential is Page-adjacent.** Per Meta's Comment Moderation guide,
  the "Instagram API with Facebook Login" column for comment endpoints on
  graph.facebook.com requires `instagram_basic` + `instagram_manage_comments`
  (+ `pages_read_engagement`), and the token that works is the Facebook Page
  access token for the Page linked to the IG account. As with Facebook, a Page
  token obtained from a long-lived user token does not expire — so the default
  stored credential is non-expiring. The expiry path exists anyway because the
  re-auth flow can store a 60-day long-lived user token instead; the schema
  keeps `expires_at` to represent that honestly.
- **Rate limits are BUC-based, like Facebook's.** A hobby deployment polling a
  handful of IG users every 10 s cannot approach Meta's rate limits, so no
  quota ledger is warranted.

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
runs on port 8100 (8095-8099 are taken); its pollers key on the IG user id,
which is the source's `channel_id` in `overlay_chat_sources`. The token lives
in its own `instagram_oauth_tokens` table (migration 096), keyed on
`(user_id, ig_user_id)` — not in `facebook_oauth_tokens`, because the IG
identity the source keys on is the IG user id, and the two platforms' re-consent
and expiry semantics differ. The same Meta app carries both products; the
Instagram permissions fold into the next App Review cycle (a separate
submission would be a second ~20-day review).

**Token chain chosen by the auth-service slice, within this contract:** prefer
the non-expiring Page-derived token (the ADR-0060 chain: code → short-lived
user token → long-lived user token → Page token via `/me/accounts`, storing the
Page token for the Page whose `instagram_business_account` matches) with
`expires_at = NULL`; fall back to storing the 60-day long-lived user token with
`expires_at` set only when the API shape requires it. Any additional dependency
scope beyond the contract's `instagram_basic` + `instagram_manage_comments` is
limited to what the account-resolution call requires (in practice
`pages_show_list`, because the IG user id is only obtainable by enumerating
Pages) and is documented there rather than requested speculatively.

**Read-only ingest, deliberately narrower than Facebook.** No moderation write
path (out of plan scope), no emotes (out of scope for all new platforms), no
quota subsystem (BUC-based limits like Facebook's, ADR-0060's analysis
transfers). A poller 401/403 marks the source inactive and emits a
`platform:status` event so the dashboard prompts the streamer to re-auth; an
expired row (per `expires_at`) is treated as a missing credential, not a Graph
error — the listener fails the poll with an UnresolvedTokenError-shaped error
and skips.

**Single-pod listener, no leader election.** Same argument as ADR-0060: the
poll is cheap and the comment_id cursor tolerates replays, so the deployment
pins `replicas: 1` and skips HPA and leadership plumbing.

**Fail-closed source adds.** overlay-manager refuses an Instagram source unless
`instagram_oauth_tokens` has a row for that exact (user, ig_user_id) pair —
same ownership-evidence argument as ADR-0060's Page anchor.

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

- Live comments (the ingest edge, `comment_id` pagination):
  https://developers.facebook.com/docs/instagram-platform/instagram-graph-api/live-comments/
- IG User live media (broadcast discovery, "only media being broadcast"):
  https://developers.facebook.com/docs/instagram-platform/instagram-graph-api/reference/ig-user/live_media/
- IG Comment node (fields; live-read limitation):
  https://developers.facebook.com/docs/instagram-platform/instagram-graph-api/reference/ig-comment/
- IG media comments ("cannot be filtered by timestamp"):
  https://developers.facebook.com/docs/instagram-platform/instagram-graph-api/reference/ig-media/comments
- Comment Moderation guide (permissions matrix, Page token requirement):
  https://developers.facebook.com/docs/instagram-platform/comment-moderation
- Permissions reference (`instagram_basic`, `instagram_manage_comments`):
  https://developers.facebook.com/docs/permissions
- Webhooks reference (the realtime alternative, deliberately not used):
  https://developers.facebook.com/docs/graph-api/webhooks/reference/instagram/
- ADR-0060 (Facebook polling — the template this decision mirrors), ADR-0008
  (feature gate seeded by migration 096), ADR-0032 (source liveness/status)
