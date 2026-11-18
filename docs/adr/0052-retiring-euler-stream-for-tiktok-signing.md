# ADR-0052: Retiring Euler Stream for TikTok webcast signing

**Date**: 2026-08-14
**Status**: Accepted
**Revised**: 2026-11-18
**Deciders**: caesarakalaeii

## Implementation (current state)

The self-hosted sign service `services/tiktok-signer` computes X-Bogus with
TikTok's own web SDK in a headless browser (vendored from the MIT-licensed
carcabot/tiktok-signature; see its `vendor/PROVENANCE.md`) and X-Gnarly with a
vendored encoder; it signs *and executes* `/webcast/im/fetch/` exactly as
Euler's `/webcast/fetch` did, returning the decoded protobuf plus the cookies
TikTok set (`SignResult`). The listener wires it behind the existing flags:
`SelfSigner` (`src/sign/self.ts`) when `TIKTOK_SIGNER_URL` is set, device
presets pinned to the signer's browser identity.

Retired in production: `TIKTOK_SIGNER_MODE=pure-node`,
`TIKTOK_SELF_SIGN_FALLBACK=false` — no TikTok connection has an Euler leg. The
code default is still `euler`; the walk `shadow` (measure) → `self` (cutover,
fallback on) → fallback off (retire) in the listener README applies to any
deployment that sets nothing. Lever 1 (`TIKTOK_DISABLE_EULER_FALLBACKS`,
default on) drops Euler from the room-id/is-live composites, where it was only
ever the last leg. Gift enrichment (`TIKTOK_EXTENDED_GIFT_INFO`) defaults on
only under `self`, because the direct `gift/list/` route must itself be signed.

Two incident follow-ups, absorbed into the current design:

- **Concurrency ceiling** (2026-09-14): one listener pod overran Euler's
  free-tier concurrent WebSocket proxy cap (above it, the proxy accepts the
  handshake then silently withholds push). Mitigations now in place: ADR-0007
  rebalancing in the service's coordinator, and a per-pod ceiling
  (`TIKTOK_MAX_STREAMS_PER_POD`, default 20).
- **TikTok session gating** (from 2026-09-09): TikTok gates webcast data
  endpoints on browser-grade sessions — an anonymous non-browser fetcher gets
  an empty 200 or 403 regardless of signature quality (measured side by side:
  undici fetch fails, a real-display Chromium succeeds — TikTok judges the
  session, not the signature). The signer's **page-viewer mode**
  (`src/signing/viewer.ts`, `SIGNER_VIEWER_MODE=page`) runs a non-headless
  browser tab per room on the streamer's live page, capturing the SDK-signed
  im/fetch the player itself receives and returning the same
  `{fetchResult, fetchResultCookieHeader}` contract; it needs a real rendering
  stack (Xvfb measured insufficient; removed renderer-spoof args had caused a
  full `viewer_capture_failed` failure). The listener's push WebSocket
  meanwhile mirrors Chrome's TLS ClientHello (`tls-impersonate`,
  `src/ws/chrome-tls.ts`). Verified live by `src/sign/euler-free.live.test.ts`
  (opt-in `TIKTOK_LIVE_TESTS=1`): Euler demonstrably is not on that path.

## Context

`tiktok-live-connector` cannot open a TikTok LIVE WebSocket without a signed
URL, and by default obtains it from **Euler Stream**, a third-party sign
service that sat on the critical path of every connection. Costs measured on
2026-08-14: a hard ceiling on concurrent rooms (12 attempts exhausted the
free-tier sign limit, failing uncleanly); paywalled gift enrichment
(`fetchAvailableGifts()` requires a Business plan); a third party in a
credential path (authenticated sessions forward the TikTok session cookie to
the sign server).

Only the signature required solving something hard — the connector already
ships direct-to-TikTok routes for room id, room info, is-live and gifts, and
Euler is not just a signer but an executor of `/im/fetch/`. The signing
primitives (X-Bogus, X-Gnarly, msToken) exist nowhere in the library bundle:
the algorithms live on Euler's server. There are **two** signing seams,
`fetchSignedWebSocketFromProvider` (the handshake) and
`fetchWebcastSignatureFromProvider` (generic HTTP URL signing, which is what
gift enrichment actually flows through); a cutover overriding only the first
leaves Euler on the gift path. No fork is needed: the connector's
`RouteConfig` / `RoomIdRouteConfig` / `IsLiveRouteConfig` / `SignConfig` are
mutable module-level objects covering every call site.

The trade, deliberately taken: when TikTok changes the algorithm it becomes
our on-call problem and TikTok ingest is down until fixed. `shadow` mode
exists to measure our success rate against Euler's, concurrently on the same
rooms, before anything depends on the answer.

## Consequences

- The signature seam is a single interface (`WebcastSigner`), swappable for a
  remote sign service by changing `TIKTOK_SIGNER_URL`.
- `tiktok_sign_attempts_total` is labelled by signer, outcome, reason and
  `load_bearing`; failures are recorded even when the fallback rescues them,
  so a broken self-signer is visible before the net is removed.
- Shadow mode doubles initial im/fetch traffic per connect; Euler consumption
  unchanged. Three interacting flags are deliberate: collapsing them would
  couple the safe change to the risky one. Until the signer exists, `shadow`
  and `self` degrade to Euler with a warning, and the install report reports
  the *effective* state.
- Licence: the modified-AGPL §19 SaaS restriction discussed in the issue does
  not apply to the version depended on (2.4.0, plain MIT, checked in the
  lockfile). The relicence is real and lands in 2.4.3 against the caret range
  `^2.4.0` — pin to `2.4.0` rather than leave a caret, and re-read the shipped
  `LICENSE` on any bump.

## Alternatives considered

- **Fork the connector.** Unnecessary: the mutable config objects cover every
  call site, and a fork carries upstream's churn.
- **Pay for a Business plan.** The pricing is what prompted this; leaves the
  ceiling and the cookie path partly in place.
- **Hard cutover, no shadow step.** Converts a cost problem into an
  availability problem with no measured success rate.
- **Only lever 1 and stop.** Viable and built so it remains an option; it does
  not lift the ceiling or unblock gifts.
