# ADR-0060: Facebook via Graph API polling with a dual-scope page credential and a moderation write path

**Date**: 2026-09-08
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

All-Chat aggregates live chat from Twitch, YouTube, Kick, TikTok and Discord.
Facebook Live is the last large platform where a streamer's audience chats on
the video itself: comments on the live broadcast, moderated (or not) from the
Page's own tools. Adding it means two things at once — ingesting those
comments onto overlays, and letting the streamer moderate them (delete a
comment, ban a commenter) from All-Chat's dashboard, the way they already can
for Twitch and Kick (ADR-0017).

Three facts about Facebook's platform shape the whole design:

- **There is no public realtime comment API.** Meta's realtime surface for
  Pages is webhooks (`live_videos` is a subscribable field), and webhooks need
  a public HTTPS endpoint plus a verify token per deployment. All-Chat's
  cloud-hosted model already polls YouTube live chat for exactly this reason,
  and the comment edge is explicitly documented as best polled:
  the `live-video/comments` reference says *"The best practice for querying
  comments on a Live video is to continually poll for comments in the
  reversechronological ordering mode."*
- **The permission model is two scopes, not one.** Reading live-video
  comments needs `pages_read_engagement`; the moderation write path (comment
  delete, comment hide, Page block) needs `pages_manage_engagement`. Meta's
  Permissions Reference defines the latter as allowing the app to *"create,
  edit and delete comments posted on the Page"*. Both sit behind App Review
  with Advanced Access, and review is slow (~20 days is common). Building the
  write path but asking for the scope later would mean a second consent round
  per streamer and a second review cycle — so both scopes are requested at
  first consent.
- **The credential that works is a Page token, and it does not expire.** The
  token chain is: authorization code → short-lived user token (~1–2 h) →
  long-lived user token (~60 days) → Page token via `/me/accounts`. Meta's
  Long-Lived Access Tokens doc states: *"Long-lived Page access token do not
  have an expiration date and only expire or are invalidated under certain
  conditions"* (password change, permission revocation, app deauthorization).
  A non-expiring credential removes the whole refresh subsystem for this
  platform.

## Decision

**Ingest: HTTP polling of the live-video comments edge, no webhooks, no quota
machinery.** `facebook-listener` resolves the Page's currently-live video
(`GET /{page-id}/live_videos`, filtered client-side to `status == "LIVE_NOW"`)
and polls `GET /{live-video-id}/comments` with `order=reverse_chronological`
and a `since` cursor pinned to the newest seen `created_time`
(`FACEBOOK_POLLING_INTERVAL_MS`, default 10 s). The cursor is idempotent: a
restarted or duplicated poll replays nothing, because every comment carries a
creation timestamp. Unknown/attachment-only comments are logged and dropped.

**No quota subsystem.** YouTube needed ADR-0006's reserve-confirm-rollback
ledger because Google sells Data API units. Meta's Pages rate limit is
`4800 * Number of Engaged Users` calls per rolling 24 h when using a Page
token (or `200 * DAU` per hour on the user-token Platform limit) — a hobby
deployment polls a handful of Pages every 10 s and cannot approach either.
The listener reads the `X-Business-Use-Case-Usage` header and warns at 90%
instead of metering.

**Credential: the Page token, stored encrypted, refresh-free.** The OAuth
callback exchanges the code all the way to the Page token at consent time and
stores the result in `facebook_oauth_tokens` (migration 094): page id, page
name, AES-GCM-encrypted token, granted scopes. There is no refresh token and
no expiry column, and token-refresh-service explicitly skips the platform:
the credential's whole lifecycle is "replaced on re-consent, invalidated by
revocation" — the revocation surfaces as a Graph 401/403 at use time, which
every consumer already maps to a reconnect prompt.

**Moderation: ADR-0017's write-path shape over the Page token.**
`moderation-service` gains a Facebook dispatcher mapped onto three Graph
verbs: `DELETE /{comment-id}` (delete), `POST/DELETE /{page-id}/blocked` with
the `user` parameter (ban/unban — the reference documents `user` as *"List of
User or Page IDs to block"*, superseding the deprecated `uid`). The single
permission `pages_manage_engagement` authorizes all of them, so the scope
matrix is one-to-one where Twitch needed nine. Hide/unhide
(`POST /{comment-id}` with `is_hidden`) is deliberately **not** mapped to a
dashboard action: hiding is not deletion, and offering it under the delete
button would misstate what happened on the Page. Timeout is absent for the
same reason — Facebook has no time-bounded mute. Delegated moderation
(ADR-0048) is refused: Facebook offers no moderator-consent surface in this
phase, and the refusal (not a fallback to the owner's token) is the
invariant.

**Single-pod listener, no leader election.** source-manager's leadership
machinery exists because YouTube's pollers are quota-expensive and must not
duplicate (ADR-0007). The Facebook poll is cheap and cursor-idempotent, so
the deployment pins `replicas: 1` and skips the HPA and the leadership
plumbing entirely; two replicas would only double the call count.

**Fail-closed source adds.** overlay-manager refuses a Facebook source unless
`facebook_oauth_tokens` has a row for that exact (user, page) pair — the row
exists only because Facebook issued the token for that Page, which is the
same ownership evidence argument as the YouTube per-channel anchor
(ADR-0048's pure-gate form).

## Risks and Operator Actions

The code ships dev-mode-ready. **App Review is the deployment bottleneck**,
and it is the operator's work:

1. **Create the Meta app** (type Business), add the Facebook Login product,
   and register `<FRONTEND_URL>/api/v1/auth/facebook/callback` as a valid
   OAuth redirect URI.
2. **Start Business Verification early.** Advanced Access on both
   `pages_read_engagement` and `pages_manage_engagement` requires it, and the
   combined path (verification + review) commonly takes on the order of 20
   days. Dev-mode usage — which works against Pages owned by app admins and
   testers — covers the entire integration and testing window.
3. **Submit App Review for both permissions**, with screencasts showing: the
   Facebook Login consent on All-Chat, live comments appearing on an overlay,
   and a comment deleted/blocked from the moderation panel.
4. In dev mode, the streamer must hold a role (admin/developer/tester) on the
   app — that is the intended test path until review lands.

Rate-limit exposure after launch is bounded: page-token calls draw from
`4800 * engaged users / 24 h`, and the listener's 10 s cadence per Page uses
~8,600 calls/day for a permanently-live Page — inside the limit for a Page
with even a couple of engaged users, and back-pressure (a 90%-usage warning
plus Graph's throttle error codes 4/17/32/613 mapped to a logged backoff)
covers the rest.

## Consequences

- Adding Facebook costs one listener, one normalizer, one OAuth provider, one
  migration and one moderation client — the same surface as any other
  platform — **minus** the quota ledger and **minus** token refresh.
- A streamer who declines `pages_manage_engagement` on the granular consent
  screen still gets working ingest (read scope granted); their moderation
  controls report `missing_scope` and their fix is re-consent, which re-runs
  the whole two-scope request.
- A page token invalidated by a password change surfaces as a Graph 401 on
  the next poll; the listener logs and skips, and the streamer reconnects
  from the dashboard. There is no background job that could have pre-warned,
  because nothing expires.
- Crossposting from Instagram to a Facebook Page is a separate API surface
  and is not covered here; an IG-origin comment on a crossposted video does
  not flow through this listener.

## References

- Live-video comments edge (order/since semantics): developers.facebook.com/docs/graph-api/reference/live-video/comments/
- Page blocked edge (ban/unban, `user` param): developers.facebook.com/docs/graph-api/reference/page/blocked/
- Comment node (delete/hide): developers.facebook.com/docs/graph-api/reference/comment/
- Token chain and page-token non-expiry: developers.facebook.com/docs/facebook-login/guides/access-tokens/get-long-lived
- Permissions reference (`pages_read_engagement`, `pages_manage_engagement`): developers.facebook.com/docs/permissions
- Rate limiting (`4800 * engaged users`, BUC headers): developers.facebook.com/docs/graph-api/overview/rate-limiting
- ADR-0006 (YouTube quota — deliberately not replicated), ADR-0017 (moderation write path), ADR-0048 (delegated moderation, refusal semantics), ADR-0008 (feature gate seeded by migration 094)
