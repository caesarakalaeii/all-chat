# ADR-0060: Facebook via Graph API polling with a dual-scope page credential and a moderation write path

**Date**: 2026-09-08
**Revised**: 2026-11-03 (brought in line with the current implementation)
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

Three platform facts shape the design:

- **No public realtime comment API.** Meta's realtime surface for Pages is
  webhooks, which need a public HTTPS endpoint and a per-deployment verify
  token; the comments edge is documented as best polled
  (reversechronological, continually).
- **Two scopes, not one.** Reading live-video comments needs
  `pages_read_engagement`; the moderation write path (delete, hide, Page
  block) needs `pages_manage_engagement`. Both sit behind App Review with
  Advanced Access (~20 days is common), so a later write-scope request would
  mean a second consent round per streamer and a second review cycle: both are
  requested at first consent.
- **The working credential is a Page token, and it does not expire.**
  Authorization code, then short-lived user token, then long-lived user token,
  then Page token via the accounts edge; a Page token has no expiration date
  and is invalidated only by password change, permission revocation or app
  deauthorization. This removes the whole refresh subsystem for the platform.

## Decision

**Ingest: HTTP polling of the live-video comments edge; no webhooks, no quota
machinery.** `facebook-listener` resolves the Page's currently-live video
(Graph `live_videos` filtered to status LIVE_NOW) and polls the comments edge
of that video reverse-chronologically with a `since` cursor pinned to the
newest seen `created_time` (`FACEBOOK_POLLING_INTERVAL_MS`, default 10 s). The
cursor is idempotent: a restarted poll replays nothing. Unknown or
attachment-only comments are logged and dropped. No quota ledger: Meta's
page-token limit (`4800 * engaged users` per 24 h) is far beyond a hobby
deployment's reach; the listener reads the `X-Business-Use-Case-Usage` header
and warns at 90% instead of metering. Single-pod deployment (`replicas: 1`, no
HPA, no leadership plumbing): the poll is cheap and cursor-idempotent, so
duplication buys nothing (ADR-0007 leadership exists for quota-expensive
YouTube pollers).

**Credential: a Page token, stored encrypted, refresh-free.** The OAuth
callback exchanges the code all the way to the Page token at consent time
(code, then short-lived user token, then long-lived user token, then Page
token via the accounts edge) and stores it in `facebook_oauth_tokens`
(migration 094): page id, page name, AES-GCM-encrypted token, granted scopes.
A Page token does not expire (it is invalidated only by password change,
permission revocation or app deauthorization), so there is no refresh token,
no expiry column, and token-refresh-service skips the platform: the lifecycle
is replaced on re-consent, invalidated by revocation, surfacing as a Graph
401/403 that consumers map to a reconnect prompt.

**Moderation: ADR-0017's write-path shape over the Page token.** The
dispatcher maps to the comment delete edge and the Page blocked edge
(ban/unban). The single permission `pages_manage_engagement` authorizes all of
them: one-to-one where Twitch needed nine. Hide/unhide is deliberately not
mapped (hiding is not deletion); there is no timeout (Facebook has no
time-bounded mute); delegated moderation is refused outright, never fallen
back to the owner's token (ADR-0048's invariant). Both scopes
(`pages_read_engagement` to read, `pages_manage_engagement` to moderate) are
requested at first consent, since App Review with Advanced Access is slow
(~20 days) and a later request would mean a second consent round per streamer
and a second review cycle.

**Fail-closed source adds**: overlay-manager refuses a Facebook source unless
`facebook_oauth_tokens` has a row for that exact (user, page) pair: same
ownership-evidence argument as the YouTube per-channel anchor (ADR-0048).

**Operator actions** (dev-mode ready; App Review is the deployment
bottleneck): create the Meta app (Business type, Facebook Login, callback URI
registered); start Business Verification early; submit App Review for both
permissions with screencasts of consent, live comments on an overlay, and a
moderation action. In dev mode the streamer must hold a role on the app.

## Consequences

- Adding Facebook costs one listener, one normalizer, one OAuth provider, one
  migration and one moderation client — the same surface as any other platform
  — **minus** the quota ledger and **minus** token refresh. The frontend still
  hides the platform outside beta.allch.at (`BETA_ONLY_PLATFORMS` in
  `frontend/src/lib/platform-availability.ts`); release means removing it there
  and flipping `platform_facebook` in `/admin/features` once Meta App Review
  has passed.
- A streamer declining `pages_manage_engagement` still gets working ingest;
  moderation controls report `missing_scope` and the fix is re-consent. A page
  token invalidated by a password change surfaces as Graph 401 on the next
  poll: no background job could pre-warn, because nothing expires.
- Crossposting from Instagram to a Facebook Page is a separate API surface; an
  IG-origin comment on a crossposted video does not flow through this
  listener.

## References

- [Live-video comments edge](https://developers.facebook.com/docs/graph-api/reference/live-video/comments/) (order/since semantics), [Page blocked edge](https://developers.facebook.com/docs/graph-api/reference/page/blocked/) (ban/unban), [Long-lived tokens](https://developers.facebook.com/docs/facebook-login/guides/access-tokens/get-long-lived) (page-token non-expiry), [Rate limiting](https://developers.facebook.com/docs/graph-api/overview/rate-limiting/)
- ADR-0006 (quota, deliberately not replicated), ADR-0017 (moderation write path), ADR-0048 (refusal semantics), ADR-0008 (the `platform_facebook` gate)
