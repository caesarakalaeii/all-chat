# ADR-0052: Retiring Euler Stream for TikTok webcast signing

## Status

Proposed — steps 3–4 implemented behind flags; step 1 (the signature itself) not
started, and scoped at two seams rather than one (see "There are two Euler
signing seams").

Neither acceptance criterion of the issue — the connection-rate ceiling and gift
enrichment — is met yet; both depend on that unstarted work.

## Context

`tiktok-live-connector` cannot open a TikTok LIVE WebSocket without a signed URL,
and by default it obtains that signature from **Euler Stream**, a third-party
sign service. Euler therefore sits on the critical path of *every* TikTok
connection All-Chat makes.

Three costs were measured on 2026-08-14, while investigating coin chests (PR #695):

1. **A hard ceiling on concurrent rooms.** Twelve simultaneous connection
   attempts exhausted the free-tier sign limit. It does not even fail cleanly:
   the connector throws `Cannot read properties of undefined (reading
   'retry-after')` while trying to read the 429 response. This is a direct limit
   on how many TikTok streamers we can serve.
2. **Paywalled gift enrichment.** `fetchAvailableGifts()` returns *"This endpoint
   requires a Business plan."* That is why `enableExtendedGiftInfo` has been off,
   and why TikTok gifts are not enriched the way they are on other platforms.
3. **A third party in a credential path.** When authenticated sessions are used,
   the connector forwards the TikTok session cookie *to the sign server*.

Euler's pricing is not viable for us, so the free tier is shaping our behaviour
in ways we did not choose.

### What actually has to be built

Scoping this revealed that it is much smaller than "replace the library".
`tiktok-live-connector` already ships direct-to-TikTok routes for nearly
everything it asks Euler for:

| Need | Euler route | Direct alternative in the library |
|---|---|---|
| WebSocket signature | `fetchSignedWebSocketFromEulerRoute` | **none — this is the real work** |
| Room ID | `fetchRoomIdFromEulerRoute` | `fetchRoomIdRoute`, `fetchRoomIdComposite` |
| Room info | `fetchRoomInfoFromEulerRoute` | `fetchRoomInfoRoute`, `fetchRoomInfoApiLiveRoute`, `fetchRoomInfoFromHtmlRoute` |
| Is-live | — | `fetchIsLiveRoute`, `fetchIsLiveComposite` |
| Gifts | `fetchRoomGiftsFromEulerRoute` | `fetchRoomGiftsRoute` |

Only the signature genuinely requires solving something hard.

### No fork is required

The library exposes module-level **mutable** configuration objects that between
them cover every Euler call site. Its own documentation invites the substitution:

> Global route registry. Call sites should read handlers from here rather than
> importing the route functions directly, so downstream consumers can swap
> implementations.

Concretely:

- `RouteConfig.fetchSignedWebSocketFromProvider` — the WebSocket signature.
- `RouteConfig.fetchWebcastSignatureFromProvider` — a **second, distinct**
  signing seam, described below.
- `RoomIdRouteConfig.skipFetchRoomIdFromEulerRoute` and the identically named
  field on `IsLiveRouteConfig` — the Euler leg of the two composites.
- `SignConfig.basePath` — where the Euler SDK points, and also what the
  connector's whitelist check compares against for authenticated sockets.

So we can repoint the connector at our own implementation without forking it,
and without vendoring a patched copy we would then have to maintain.

### There are two Euler signing seams, not one

The issue's table implies a single signing dependency. Reading the connector's
bundled source shows **two**, and a cutover that overrides only the first leaves
Euler on the critical path for gifts:

| Seam | Used for | Reached via |
|---|---|---|
| `fetchSignedWebSocketFromProvider` | the webcast WebSocket handshake | connect |
| `fetchWebcastSignatureFromProvider` | signing an arbitrary **HTTP** URL | `WebcastHttpClient.request({ signRequest: true })` |

The second matters because of gift enrichment. `fetchAvailableGifts()` resolves
to `RouteConfig.fetchRoomGifts`, which already defaults to the *direct* route
`fetchRoomGiftsRoute` — it calls TikTok's `webcast/gift/list/` itself and never
touches Euler's gift endpoint. But it passes `signRequest: true`, so the request
is signed through `fetchWebcastSignatureFromProvider`, i.e. through Euler.

So the Business-plan error is *not* because the library asks Euler for the gift
list. It is because Euler must sign the URL of our own direct request. That is
why `TIKTOK_EXTENDED_GIFT_INFO` is gated on signing for ourselves rather than
being independently switchable, and why implementing only the WebSocket
signature would not unblock the gift-enrichment acceptance criterion.

### Which routes actually still reach Euler

Of the routes the connector genuinely invokes, only the two signature seams are
Euler-bound. `RouteConfig.fetchRoomInfo` and `RouteConfig.fetchRoomGifts`
**already default to the direct-to-TikTok routes**, so the room-info row of the
issue's table needs no work: its "alternative" is the existing default. The
`fetchRoomIdFromProvider` / `fetchRoomInfoFromProvider` Euler routes are only
reachable as the composites' last leg, which lever 1 disables.

## Decision

Separate the work into **two independent levers with very different risk**, and
gate each behind its own flag rather than shipping one combined switch.

### Lever 1 — drop Euler from the composites (`TIKTOK_DISABLE_EULER_FALLBACKS`, default **on**)

Room ID and is-live already try the HTML scrape and TikTok's API endpoint first
and consult Euler only when both have failed. Turning off that last leg reduces
free-tier consumption immediately and cannot lose a capability, because Euler was
never the primary source. This is worth doing on its own merits even if the
signing work is never finished.

### Lever 2 — sign it ourselves (`TIKTOK_SIGNER_MODE`, default **`euler`**)

Three modes, intended to be walked in order:

- **`euler`** — unchanged behaviour, and the baseline. Our code is not on the
  connect path at all.
- **`shadow`** — Euler signs the connection that is actually used; our signer
  runs concurrently against the same room and its outcome is recorded and
  discarded. Enabling this cannot change connection behaviour.
- **`self`** — we sign. With `TIKTOK_SELF_SIGN_FALLBACK` on (the default), Euler
  still catches our failures; turning it off is what finally retires the
  dependency.

Gift enrichment (`TIKTOK_EXTENDED_GIFT_INFO`) defaults on **only** under `self`,
because the direct `gift/list/` route must itself be signed — enabling it any
earlier just reinstates the Business-plan error on every connect.

### The trade-off, stated deliberately

Today, when TikTok changes the signing algorithm, it is Euler's problem and we
get the fix for free. Afterwards it is our on-call problem, and **TikTok ingest
is fully down until we fix it**. Signing also implies fingerprinting and device
presets that TikTok actively churns, so this is an ongoing maintenance
commitment, not a one-off build.

That trade is only defensible with evidence, which is what `shadow` mode is for:
it exists so we can measure our own signature success rate against Euler's, on
the same rooms, at the same time, before anything depends on it. `ShadowSigner`
starts the candidate before awaiting the primary specifically so the comparison
is concurrent — TikTok's behaviour varies by room and by minute, and a candidate
run seconds later is not a comparison.

## Consequences

### Positive

- The composite change removes free-tier calls now, with no signing work.
- The signature seam is a single interface (`WebcastSigner`), so the eventual
  implementation has one place to land and can be swapped for a remote sign
  service by changing `TIKTOK_SIGNER_URL` alone.
- `tiktok_sign_attempts_total` is labelled by signer, outcome, reason and
  `load_bearing`, so the shadow experiment and real availability are the same
  metric read two ways. Failures are also recorded when the fallback rescues
  them, so a totally broken self-signer is visible before we remove the net.
- The rate-limit classifier special-cases the connector's `retry-after`
  `TypeError`, so the exact failure that prompted this work is legible rather
  than landing in `unknown`.

### Negative

- Shadow mode doubles initial `/im/fetch/` traffic per connect (not WebSocket
  connections). Euler consumption is unchanged, since the candidate does not
  call them.
- Three interacting flags is more configuration surface than one switch. This is
  deliberate: collapsing them would couple the safe change to the risky one.
- Until the signer exists, `shadow` and `self` degrade to Euler with a warning.
  The install report deliberately reports the **effective** state, not the
  requested one, so a dashboard cannot show Euler retired while it is not.
- The signing work is **two** implementations, not one: the WebSocket handshake
  and the generic HTTP URL signer behind
  `fetchWebcastSignatureFromProvider`. `WebcastSigner` covers only the first.
  Fully retiring Euler — and unblocking gift enrichment specifically — needs the
  second as well, and it is not yet designed.

### Licence

The issue describes `tiktok-live-connector` as carrying a modified AGPL whose
§19 withdraws its additional permission for anything powering a *"commercial,
closed-source, or hosted SaaS platform, including but not limited to a WebSocket
relay service, data-scraping API, or managed hosting solution offered to third
parties"*, with §21 excepting Euler Stream Inc. and TikFinity / STV GmbH by
name.

**That is not the licence of the version we actually depend on.** Checked
2026-08-17 against the installed tree: `tiktok-live-connector@2.4.0` ships a
plain **MIT** licence (`node_modules/tiktok-live-connector/LICENSE`, 21 lines,
"MIT License / Copyright (c) 2026 zerodytrash"), and `package-lock.json`
records `"license": "MIT"` for the pinned resolution. There is no §19, no §21
and no SaaS carve-out in the text we are bound by.

So the licence risk as stated in the issue does not apply to the code we ship
today, and it is **not** a precondition of the signing work. Two caveats keep it
from being a non-issue entirely:

- **The relicence is real, just not ours yet.** It lands in **2.4.3** (already
  recorded in this service's README, from PR #695). We depend on the range
  `^2.4.0`, so a routine lockfile refresh would pull 2.4.3+ and the modified
  AGPL with it, silently. If the §19 SaaS restriction matters to us, the
  dependency should be **pinned to `2.4.0`** rather than left on a caret range,
  and the shipped `LICENSE` re-read on any deliberate bump.
- The author has a direct commercial interest in the service being replaced.
  That is a reason to expect upstream churn around the signing seams — and a
  reason a future version may restrict precisely this use — not a constraint on
  2.4.0.

MIT also *removes* the obstacle to extracting a reusable sign service, which the
AGPL reading would have blocked. Any such extraction should still re-verify the
licence of the version in the lockfile at that time.

## Alternatives considered

**Fork or vendor the connector.** Rejected: unnecessary. The three mutable
globals cover every call site, and a fork would mean carrying upstream's
churn — on a library whose whole value is tracking a moving target.

**Pay for a Business plan.** The pricing is what prompted this issue. It also
leaves costs 1 and 3 only partly addressed: a higher ceiling is still a ceiling,
and the third party stays in the credential path.

**Hard cutover to self-signing.** Rejected: it converts a cost problem into an
availability problem in one step, with no measured success rate to justify it.
Hence shadow mode and a configurable fallback.

**Only do the composite work (lever 1) and stop.** Still viable, and lever 1 is
deliberately built so this remains an option. It does not lift the connection
ceiling or unblock gift enrichment, both of which are driven by the signature.
