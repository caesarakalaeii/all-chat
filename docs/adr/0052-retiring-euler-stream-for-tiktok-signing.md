# ADR-0052: Retiring Euler Stream for TikTok webcast signing

## Status

Proposed — steps 3–4 implemented behind flags; step 1 (the signature itself) not started.

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

- `RouteConfig.fetchSignedWebSocketFromProvider` — the signature itself.
- `RoomIdRouteConfig.skipFetchRoomIdFromEulerRoute` and the identically named
  field on `IsLiveRouteConfig` — the Euler leg of the two composites.
- `SignConfig.basePath` — where the Euler SDK points, and also what the
  connector's whitelist check compares against for authenticated sockets.

So we can repoint the connector at our own implementation without forking it,
and without vendoring a patched copy we would then have to maintain.

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

### Licence

`tiktok-live-connector` carries a modified AGPL whose §19 withdraws its
additional permission for anything powering a *"commercial, closed-source, or
hosted SaaS platform, including but not limited to a WebSocket relay service,
data-scraping API, or managed hosting solution offered to third parties"*, and
whose §21 excepts Euler Stream Inc. and TikFinity / STV GmbH **by name**.

All-Chat is AGPL-3.0 and this work is for our own ingest, which appears to be
fine. But the author has a direct commercial interest in the service being
replaced, so **§19–21 must be read properly before any of this is published as a
reusable sign service or offered to third parties.** That review is a
precondition of extracting the sign service beyond our own deployment, not of
signing for ourselves.

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
