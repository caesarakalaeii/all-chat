# tiktok-signer

Self-hosted TikTok webcast signing service. Replaces **Euler Stream** on the
TikTok signing path — the third-party sign server that capped our concurrent
rooms, paywalled gift enrichment, and sat in the credential path for
authenticated connections (ADR-0052).

## What it does

`tiktok-live-connector` cannot open a TikTok LIVE WebSocket without a signed
`/webcast/im/fetch/` exchange. Where Euler performed that exchange on their
servers, this service does it on ours:

**Signature mode** (`SIGNER_VIEWER_MODE=signature`, the original path):

- a headless Chromium loads a real TikTok page with TikTok's own web SDK
  (`vendor/webmssdk_5.1.3.js`) injected locally,
- the SDK computes `X-Bogus` for the URL we need signed (TikTok's own code, so
  signature churn is mostly survived without changes here),
- the service then executes `/webcast/im/fetch/` itself and returns the
  protobuf body, `Set-Cookie` header and room ID — the exact exchange Euler's
  `/webcast/fetch` returned, which is what the connector expects.

**Page-viewer mode** (`SIGNER_VIEWER_MODE=page`, the post-2026-09-09 path):

- Since 2026-09-09 TikTok gates webcast data on a *browser-grade session*, not
  just a signature (zerodytrash/TikTok-Live-Connector#329): measured the same
  day, the signature mode's fetches receive 200 with an empty body, headless
  or Xvfb browsers receive 403, and the same Chromium on a real display
  receives 200 with the full protobuf. The session, not the signature, is
  what TikTok now judges.
- So in page mode the service *is* a viewer: a non-headless Chromium tab per
  room on the streamer's live page, and the service captures the SDK-signed
  `/webcast/im/fetch/` response the player itself receives, returning the same
  `{fetchResult, fetchResultCookieHeader}` contract the connector expects.
  Tabs stay open between captures (the page keeps the session warm on
  TikTok's side) and are evicted after 5 minutes idle.
- Requires a **real rendering stack** — Xvfb was measured insufficient.

> **X-Gnarly is deliberately not attached** (measured 2026-09-15):
> `/webcast/im/fetch/` accepts msToken + X-Bogus and returns 200, while the
> same request carrying the vendored X-Gnarly encoder's output returns 403 —
> TikTok validates that parameter server-side and the encoder has drifted
> from what the live web client computes. The encoder stays vendored behind
> the `attachXGnarly` session option for the day TikTok *requires* the
> parameter; `scripts/verify-gnarly-necessity.mjs` is the harness to
> re-measure with. Until then X-Bogus — computed by TikTok's own SDK — is
> what survives signature churn.

Both vendored files are MIT, from
[carcabot/tiktok-signature](https://github.com/carcabot/tiktok-signature); see
`vendor/PROVENANCE.md`.

## API

| Endpoint | Purpose |
|---|---|
| `POST /v1/sign` | The webcast WebSocket seam. Body `{ roomId, username?, cursor?, cookieHeader? }` — the optional `username` routes the request to a real viewer tab when viewer mode is on. Returns `{ fetchResult (base64 protobuf), fetchResultCookieHeader, fetchResultRoomId? }`. Called by `tiktok-listener`'s `SelfSigner`, and by its pure-node mode as a tab warm (PR 3 of the pure-Node transport): the listener POSTs `{ roomId: 'unused', username }` before attaching the relay for a canary mirror or a premium-fallback promotion — the viewer-path capture registers the target room's tab, the response body is discarded. |
| `POST /v1/sign-url` | The generic HTTP URL seam (gift list). Body `{ url, method? }`, `webcast.tiktok.com` URLs only. Returns `{ response: { signedUrl, userAgent } }`. |
| `GET /v1/identity` | The stable browser identity (User-Agent, platform, screen). The listener pins its connector device presets to this so the fetch and the WebSocket handshake describe the same browser. Under viewer mode this is the viewer identity (Linux Chrome/144) — the browser that captured the session. |
| `GET /v1/session` | Shared WS session lease (pure-Node transport, PR 1 of the 2026-09-18 plan): the warm classic room's webcast push WS URL + freshly re-read cookie jar, `{ wsUrl, cookieHeader, roomId, userAgent, proxyHost, capturedAt }`. The listener consumes this in `pure-node` signer mode (`TIKTOK_SIGNER_MODE=pure-node`, `src/sign/pure-node.ts`): it synthesizes the connector's initial fetch result — forwarding the wsUrl's recorded identity params (handshake-load-bearing), cursor `0`, the cookie jar — and enters the target room cross-room. No capture cadence — a capture runs only on request (TikTok request-budget discipline). Flow: warm lease with a non-empty `wsUrl` is served; otherwise the first tab-less, breaker-eligible room is cold-captured; otherwise unservable warm tabs (dead page or empty `wsUrl`) are closed — never ones with an active relay subscriber — and the freed rooms cold-captured, ending on the first served lease or a retryable 503 `no_warm_session`. Errors the consumer's classification keys on: 401 without the bearer, 404 `session_endpoint_disabled` when `SIGNER_WARM_ROOMS` is empty, 503 `viewer mode not enabled` without page mode, 503 `no_warm_session` when every phase is exhausted. Metric: `signer_session_leases_total{outcome}`. |
| `GET /v1/stream/:username` | Relay SSE stream of the room's viewer-tab WS frames — opaque base64 `PushFrame`s the listener decodes with its own connector schemas; heartbeat comment every 15s, `state` events (`capture`/`open`/`ws_closed`/`recapture`) alongside `frame` events. Rooms must be in `SIGNER_RELAY_CANARY_ROOMS`, or any warm-tab room when `SIGNER_RELAY_FALLBACK=on` (ADR-0058); 404 otherwise. Requires the room to have a warm tab (run a `/v1/sign` capture first). |
| `GET /health/live` | Liveness. |

All endpoints speak JSON. Auth: `Authorization: Bearer $SIGNER_AUTH_TOKEN`
when the token is set; none when it is empty (cluster-internal deployment
behind the default-deny NetworkPolicy).

## Environment

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8092` | HTTP port. |
| `SIGNER_AUTH_TOKEN` | empty | Bearer token clients must present. |
| `SIGNER_VIEWER_MODE` | `signature` | `signature` (X-Bogus fetch path) or `page` (real viewer tabs; see "What it does"). |
| `SIGNER_DISPLAY` | empty | X display the viewer browser renders on (e.g. `:0`). Page-viewer mode requires a real display stack. |
| `SIGNER_USER_DATA_DIR` | `/tmp/tiktok-signer-profile` | Writable browser profile dir (emptyDir in k8s). |
| `PUPPETEER_EXECUTABLE_PATH` | puppeteer default | Chromium path (set to `/usr/bin/chromium` in the image). |
| `SIGNER_WEBSHARE_TOKEN` | empty | Webshare API token: the proxy list is fetched from the API at startup and refreshed hourly, so dashboard-side rotations propagate without touching the cluster. **Preferred** over the static list. |
| `SIGNER_PROXY_HOSTS` | empty | Static comma-separated residential proxy list (`host:port,...`) for the viewer lanes, used when no webshare token is set. One browser per proxy, rooms pinned to their lane, failing lanes benched for a cooldown. **Required for page mode in production**: TikTok gates the chat bootstrap on IP reputation; the datacenter IP never receives im/fetch. |
| `SIGNER_PROXY_HOST` | empty | Singular proxy for the signature session (X-Bogus fetch path). |
| `SIGNER_RELAY_CANARY_ROOMS` | empty | Comma-separated room list whose viewer tabs can be relayed via `GET /v1/stream/:username` (the canary of the 2026-09-16 transport plan). Empty disables the endpoint entirely. Canary rooms capture on their own browser profiles (see `SIGNER_CANARY_PROFILE`): a flag TikTok earned on a primary jar cannot blind the canary. |
| `SIGNER_RELAY_FALLBACK` | `off` | `on` opens `GET /v1/stream/:username` to any room with a warm tab, not just the canary set — the listener's premium-fallback tier (ADR-0058) promotes rooms onto the relay after their primary WS exhausts flap retries. The canary set stays the read-only mirror cohort. |
| `SIGNER_WARM_ROOMS` | empty | Comma-separated classic rooms (lowercased) whose captured WS sessions `GET /v1/session` leases; empty disables the endpoint. **Must list classic, verified-live rooms only**: only classic rooms emit the harvestable im/fetch (the page variant is per-room — check for im/fetch on a harvest before trusting a room). Their tabs are pinned against idle eviction, and the capture breaker paces a dead or live_new room's retries. A room listed both here and in `SIGNER_RELAY_CANARY_ROOMS` is a misconfiguration whose canary jar would be leased as the primary session — the service refuses to start. |
| `SIGNER_CANARY_PROFILE` | `$SIGNER_USER_DATA_DIR-viewer-canary` | Profile directory base for canary lanes; the primary lanes use `$SIGNER_USER_DATA_DIR-viewer`. Each proxy hosts one primary and one canary browser, launched lazily, so isolation costs a second browser only while canary rooms are actually captured. |

### Identity invariant

One module (`src/signing/identity.ts`) defines the two identities the
service presents: `SIGNING_IDENTITY` (Safari/macOS, the X-Bogus signing
session) and `VIEWER_IDENTITY` (Linux Chrome/144, the page captures and WS
sessions). TikTok silently empty-200s any request whose User-Agent
disagrees with the URL's self-describing params — measured 2026-09-18, both
a version drift and a platform swap are rejected — so `imFetchParams`
derives `browser_platform`/`browser_version`/`os`/`screen_*` from the
identity it is handed (never hardcoded), the signing session pins its page
UA, viewport and `navigator.platform` to it, and `/v1/identity` reports it
for the listener to pin its connector presets to.

`capturedAt` on a session lease stamps the capture of the served `wsUrl`
(a kept wsUrl keeps its original stamp), so a consumer's freshness budget
keys on the URL's true age. The session lease is the credential source for
the pure-Node delivery tier the listener builds next (its mode value and
tier table land with that change).


## Scripts

| Script | Purpose |
|---|---|
| `scripts/smoke-sign-direct.mjs` | Drive the signing session directly and print a signed URL. First-run sanity check on a new host. |
| `scripts/smoke-roomid.mjs <username>` | Resolve a room ID through the connector's direct (Euler-free) routes. |
| `scripts/find-live-room.mjs` | Scan known accounts for a room that is LIVE right now, for end-to-end smokes. |
| `scripts/check-live-status.mjs` | Scrape a profile page for `liveRoomStatus`. |
| `scripts/verify-gnarly-necessity.mjs` | The 403-vs-200 bisect: re-run before ever enabling `attachXGnarly`. |

## Operation

- **Single replica.** One browser session, one sequential sign queue. The
  browser refreshes itself after 500 signatures or 30 minutes, whichever first.
- **Watch the listener's metrics**, not this service's logs:
  `tiktok_sign_attempts_total{signer="self",...}` and
  `tiktok_sign_duration_seconds` already exist (ADR-0052) and are labelled by
  outcome, reason and load-bearing.
- **Signatures stop working?** Pull the latest `javascript/webmssdk_*.js` and
  `xgnarly.mjs` from upstream carcabot/tiktok-signature (MIT), update
  `vendor/PROVENANCE.md` with the commit, redeploy. That is the break-fix
  cycle ADR-0052 warned about — it is ours now.
- **Rate limited by TikTok (429 on /v1/sign)?** Add a residential proxy.

## Rollout (ADR-0052 lever 2)

1. Deploy this service. Listener keeps `TIKTOK_SIGNER_MODE=euler` (default).
2. Set `TIKTOK_SIGNER_MODE=shadow`: Euler still signs live connections; ours
   runs in parallel and its success rate is measured. Watch
   `tiktok_sign_attempts_total{signer="self"}`.
3. On a healthy rate, set `TIKTOK_SIGNER_MODE=self` (Euler stays as fallback
   via `TIKTOK_SELF_SIGN_FALLBACK=true`, the default).
4. Finally set `TIKTOK_SELF_SIGN_FALLBACK=false`. That is what retires Euler.

## Capture breaker and canary (2026-09-16 transport plan)

- **Per-room capture breaker**: 3 consecutive `/v1/sign` viewer-capture
  failures for a room trip a breaker that fast-fails further captures with
  502 `viewer_capture_refused` (measured 2026-09-16: capture failures are
  room-correlated — gated rooms fail on every lane while healthy rooms
  capture first-attempt on the same lanes, so the breaker is per room, not
  per lane). Cooldown escalates 5 min → 10 → 20 → ... capped at 60 min; a
  success resets it. Metrics: `signer_capture_breaker_trips_total`,
* **Canary relay**: with `SIGNER_RELAY_CANARY_ROOMS` set, the listener
  (`TIKTOK_CANARY_ROOMS` on its side) mirrors a canary room's primary WS
  against `GET /v1/stream/:username` and fires
  `tiktok_canary_divergences_total{kind=...}` on divergence
  (`stalled` / `method_set` / `frame_rate` / `decode_failure_rate`).
  The canary is never load-bearing: divergence only logs and counts.
* **Premium fallback tier (ADR-0058)**: with `SIGNER_RELAY_FALLBACK=on`,
  the same relay endpoint serves any warm-tab room, and the listener
  (`TIKTOK_PREMIUM_FALLBACK=on` on its side) promotes premium rooms onto
  it after flap exhaustion — there the relay frames are the *delivered*
  stream, not a mirror. `signer_capture_breaker_*` gates the captures that
  warm the tabs; a capture-refused room cannot be promoted until the
  breaker's cooldown passes.

`TIKTOK_EXTENDED_GIFT_INFO` defaults on under `self` — the gift list request is
signed through `/v1/sign-url`, so it stops hitting the Euler paywall.
