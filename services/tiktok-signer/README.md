# tiktok-signer

Self-hosted TikTok webcast signing service. Replaces **Euler Stream** on the
TikTok signing path — the third-party sign server that capped our concurrent
rooms, paywalled gift enrichment, and sat in the credential path for
authenticated connections (ADR-0052).

## What it does

`tiktok-live-connector` cannot open a TikTok LIVE WebSocket without a signed
`/webcast/im/fetch/` exchange. Where Euler performed that exchange on their
servers, this service does it on ours:

- a headless Chromium loads a real TikTok page with TikTok's own web SDK
  (`vendor/webmssdk_5.1.3.js`) injected locally,
- the SDK computes `X-Bogus` for the URL we need signed (TikTok's own code, so
  signature churn is mostly survived without changes here),
- the service then executes `/webcast/im/fetch/` itself and returns the
  protobuf body, `Set-Cookie` header and room ID — the exact exchange Euler's
  `/webcast/fetch` returned, which is what the connector expects.

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
| `POST /v1/sign` | The webcast WebSocket seam. Body `{ roomId, cursor?, cookieHeader? }`. Returns `{ fetchResult (base64 protobuf), fetchResultCookieHeader, fetchResultRoomId? }`. Called by `tiktok-listener`'s `SelfSigner`. |
| `POST /v1/sign-url` | The generic HTTP URL seam (gift list). Body `{ url, method? }`, `webcast.tiktok.com` URLs only. Returns `{ response: { signedUrl, userAgent } }`. |
| `GET /v1/identity` | The stable browser identity (User-Agent, platform, screen). The listener pins its connector device presets to this so the fetch and the WebSocket handshake describe the same browser. |
| `GET /health/live` | Liveness. |

All endpoints speak JSON. Auth: `Authorization: Bearer $SIGNER_AUTH_TOKEN`
when the token is set; none when it is empty (cluster-internal deployment
behind the default-deny NetworkPolicy).

## Environment

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8092` | HTTP port. |
| `SIGNER_AUTH_TOKEN` | empty | Bearer token clients must present. |
| `SIGNER_USER_DATA_DIR` | `/tmp/tiktok-signer-profile` | Writable browser profile dir (emptyDir in k8s). |
| `PUPPETEER_EXECUTABLE_PATH` | puppeteer default | Chromium path (set to `/usr/bin/chromium` in the image). |
| `SIGNER_PROXY_HOST` | empty | Residential proxy `host:port`. **Recommended in production**: TikTok blocks datacenter IPs; expect empty responses / 429s without one. |
| `SIGNER_PROXY_USER` / `SIGNER_PROXY_PASS` | empty | Proxy credentials. |

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

`TIKTOK_EXTENDED_GIFT_INFO` defaults on under `self` — the gift list request is
signed through `/v1/sign-url`, so it stops hitting the Euler paywall.
