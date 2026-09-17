# TikTok Transport Plan — no-chrome primary + viewer-tab canary

Status: agreed 2026-09-16 after the lab validation described in
docs/phase-reports/TIKTOK_SIGNING_RESEARCH_2026-09-16.md. This is the
working plan; the PR description carries the shipped account.

**Progress: Phases 1, 2 and 3 are IMPLEMENTED** (phase 3 on
2026-09-17, same branch). Remaining: Phase 4 (ship). Fresh sessions start
at "Phase 4" below.

**Two operator notes (2026-09-17):**
- **Flap retry tuning is env-tunable** (2026-09-17): `TIKTOK_FLAP_MAX_FAST_RETRIES`
  and `TIKTOK_FLAP_RETRY_DELAY_MS` (defaults 3 / 1000ms, the lab-measured
  constants in `connection-decisions.ts`). Prod measured rooms clearing on
  attempt 4-8, so prod sets retries to 6; retry waits progress 1s, 1s, then
  3s — flaps are session-scoring artifacts that ease with spacing. Judge by
  `tiktok_ws_flap_retries_total` vs `tiktok_ws_flap_exhausted_total`: if
  retries succeed but exhaustion still fires, the budget is fine.
- **Promotion is the breakage canary.** `TikTokFallbackPromoted` (added to
  `allchat-warning-alerts.yaml`, fires on ANY promotion) alerts when a
  premium room switches tiers — the upgraded users are the first to feel
  whatever TikTok changed, before non-premium rooms go dark. Treat every
  promotion as a signal to check the primary tier, not as the fallback
  "working as intended".
## Decision (measured, not assumed)

**Primary transport: the listener's existing Node WebSocket.** No resident
Chrome per stream. The signer's viewer tab is a ~6s capture bootstrap only.

Lab proof (2026-09-16, diamondslay, chatty room):

- Existing prod code path (connector WS + SelfSigner + Chrome-TLS lane
  pinning) connected and held a sustained stream.
- "Unexpected server response: 200" is a flap, not a wall: 2 failed
  attempts (4-8s backoff), then connected.
- 1756 messages into chat:raw over 10 minutes (~3/s), full decode chain
  (PushFrame → messages → RawChatMessage) working.
- Clean disconnect on room end; poller backed off correctly.

Why not Euler-style risk: Euler broke on TikTok churn AND free-plan rate
limits. The in-house Node WS has no vendor, no quota. Its only exposure is
TikTok session scoring on the WS handshake — which the canary watches.

Why not viewer-tab-per-stream as primary: ~150MB renderer per active room
in the signer pod; scales badly. It stays as canary + premium fallback.

Superseded by this decision: the h2 long-poll migration (premise was
wrong — im/fetch is a one-shot bootstrap, chat rides WS PushFrames; see
the CORRECTED section in the phase report).

## Phase 1 — harden the primary ✅ DONE (2026-09-16)

All four items shipped, lab-verified on dan2dxo. Files and shapes:

1. **Connect-flap tolerance** — DONE.
   - `services/tiktok-listener/src/reliability/connection-decisions.ts`:
     `WS_FLAP_SIGNATURE = 'Unexpected server response: 200'`,
     `isWsFlapError()`, `WS_FLAP_MAX_FAST_RETRIES = 3`,
     `WS_FLAP_RETRY_DELAY_MS = 1000`.
   - `services/tiktok-listener/src/index.ts` connectToStream: retry loop
     around `connection.connect()`; flap → immediate retry (up to 3×1s),
     then normal error backoff. The connector resets to DISCONNECTED on a
     failed connect, so the same connection object is re-dialled.
   - Metrics: `tiktok_ws_flap_total`, `tiktok_ws_flap_retries_total`,
     `tiktok_ws_flap_exhausted_total` (all by username).
   - Lab proof: flap classified, retried, connected on the fast retry;
     counters exported.
2. **Circuit breaker on captures** — DONE, with a design correction.
   - **Per-room, not per-lane**: the lab verdict is that capture failures
     are room-correlated (gated rooms fail every lane, healthy rooms
     capture first-attempt on the same lanes), so a lane-keyed breaker
     would bench healthy lanes on gated rooms' evidence.
   - `services/tiktok-signer/src/signing/capture-breaker.ts`: threshold 3
     consecutive failures per room → refusal window; cooldown escalates
     5min ×2 per consecutive trip, capped 60min; success resets. Injected
     via `ServerOptions.captureBreaker`; fast-fail returns
     502 `viewer_capture_refused` in ~0.4s instead of burning
     maxLaneAttempts × ~60s.
   - Metrics: `signer_capture_breaker_trips_total`,
     `signer_capture_breaker_refusals_total` (by username).
   - Lab proof: tripped live on a failing room; healthy captures continued
     on the same pool.
3. **Lane-pinning visibility** — DONE.
   - `tiktok_ws_lane_pins_total{outcome=pinned|unpinned_no_lane|skipped|failed}`
     + pre-sign latency in the pin log line. `unpinned_no_lane` and
     `failed` are warn-logged — both are the exact shape that produced the
     2026-09-16 flaps.
4. **Regression tests + lab smoke** — DONE.
   - `connection-decisions.test.ts`: 6 new flap-classifier tests.
   - `capture-breaker.test.ts`: 7 tests (threshold, refusal window, reset,
     escalation+cap, room independence, interleaved success, forget).
   - Suites green: listener 402 passed, signer 17 passed; both `tsc` clean.

**Bonus fix found during verification (prod-relevant, already committed):**
webshare issues **per-proxy** credentials — one username/password per
proxy, not one pair for the list. The pool had been authenticating every
lane with proxy[0]'s pair; after webshare's auto-replace rotated IPs, all
non-first lanes failed Chromium proxy auth
(`ERR_INVALID_AUTH_CREDENTIALS` / `ERR_TUNNEL_CONNECTION_FAILED`) while
undici through the same proxy+creds returned 200.
- `webshare.ts` now returns `credentials: Record<host, {username, password}>`
  (+ shared pair only when every entry matches).
- `viewer.ts` `page.authenticate` uses the lane's own pair, shared pair as
  fallback; `refreshProxies` takes a third `perLaneCredentials` param.

## Phase 2 — canary machinery ✅ DONE (2026-09-16)

1. **Signer relay mode** — DONE, built differently than planned.
   - `page.on('websocket')` **does not exist** in the installed
     puppeteer v24 build (no CDP websocket interception API); instead the
     relay attaches a **CDP session** to the page
     (`page.createCDPSession()` + `Network.webSocketFrameReceived`) — the
     same protocol the lab spike used, in-process.
   - `services/tiktok-signer/src/signing/relay.ts`: `RelayHub`, one tap per
     room; webcast-scoped socket filter, binary frames only (opcode 2);
     per-frame `pinTab` refresh pins the tab against `tabIdleMs` eviction;
     zero-subscriber detach; `state` events capture/open/ws_closed/
     recapture/tap_error.
   - **New constraint discovered in lab**: the tap attaches to a page whose
     live-page WS already exists, so its requestId is unknown and every
     frame is filtered (relay silent → `stalled` divergence). Fix shipped:
     `page.reload()` right after `Network.enable` — the player reboots and
     opens a fresh WS under the live tap. Without it the canary is blind.
   - Route: `GET /v1/stream/:username` in `api.ts`, SSE with 15s heartbeat
     comments; canary set = `SIGNER_RELAY_CANARY_ROOMS` (404 outside it);
     409 when the room has no warm tab (a `/v1/sign` capture must run
     first — the listener's pre-sign guarantees this on every connect).
2. **Listener canary consumer** — DONE.
   - `services/tiktok-listener/src/canary/canary-consumer.ts`: `CanaryConsumer`
     per canary room. SSE reader, decodes frames with the connector's own
     `deserializeWebSocketMessage` (same decoder both sides → mismatch is
     wire, not decoder). 60s comparison windows; thresholds: `method_set`
     (canary methods absent on primary, >20 canary frames), `frame_rate`
     (canary < 50% of primary over >50 primary frames), `stalled` (relay
     silent while primary received >10), `decode_failure_rate` (delta
     > 20 points). Capped-backoff reconnect (1s→60s); 404 from signer
     stops the consumer.
   - Wired in `index.ts`: started on `connected`, stopped on
     `disconnected`/teardown, `decodedData` feeds `notePrimaryFrame`.
     Env: `TIKTOK_CANARY_ROOMS` (comma list). Never load-bearing.
   - Metric: `tiktok_canary_divergences_total{kind}` + structured
     `canary divergence` logs.
   - Tests: `canary-consumer.test.ts`, 11 tests green.
3. **Alert rules** — DONE. `deployments/k8s/monitoring/alerts/
   allchat-warning-alerts.yaml`: `TikTokCanaryDivergence`
   (>3 in 10m), `TikTokWsFlapExhausted` (>5 in 15m), with runbook-style
   remediation steps.
4. **Canary rooms in prod: NOT yet chosen.** Both env vars are empty by
   default. Choosing 1-2 premium rooms and setting both envs is part of
   Phase 4 rollout.
   - Lab proof: dan2dxo mirrored end-to-end — SSE frames decoded,
     divergence fired correctly on a genuinely stalled relay (pre-reload-
     fix), reconnect loop survived a signer restart, counters exported.

Original spike constraints, all honored (the tap-before-reload one now by
construction — attach, then reload):

- Attach the tap before the page (re)loads or the socket's early frames
  are missed.
- `tabIdleMs` (5 min) eviction kills the tab's WS — subscribed canary tabs
  are pinned (refresh `lastUsed` per frame).
- A reload during a live relay recreates the WS; the relay tolerates the
  gap and re-emits from the new socket.
- Frames arrive as base64 PushFrames; ~50% are ack/keepalive that decode
  to no messages — counted as neither success nor failure on the canary
  side; confirm the ratio stays stable once prod canary runs.

## Phase 3 — premium fallback ✅ DONE (2026-09-17)

All items shipped; details in ADR-0058 and the service READMEs.

1. **Promotion on flap exhaustion** — DONE. In connectToStream's
   flap-exhausted branch, `tryPromoteFallback(username, overlayId,
   connection)` runs before the error is thrown into backoff:
   `TIKTOK_PREMIUM_FALLBACK=on` + `users.is_premium` via overlay ownership
   (TTL-cached, fail-closed) + signer reachable → delivery switches to a
   `FallbackConsumer` (`src/canary/fallback-consumer.ts`).
2. **Delivery** — DONE, via the connector itself: relay frames are decoded
   with `deserializeWebSocketMessage` and replayed into the room's
   `TikTokLiveConnection.processProtoMessageFetchResult`, so every
   chat/gift/social/envelope handler, dedup and the heartbeat monitor run
   exactly as on the primary path. The connection stays DISCONNECTED —
   only its event listeners are used.
3. **Demotion** — DONE, with a design correction: "demote after X hours of
   primary-tier health" is unevaluable while the fallback delivers (the
   primary is not connected, its health is unobservable). A stint ends on
   relay `ws_closed`, `tap_error`, signer 404, or
   `TIKTOK_FALLBACK_MAX_DURATION_MS` (default 6h); the room then returns
   to the poller for a fresh primary attempt with a fresh flap budget.
4. **ADR-0058** — DONE (`docs/adr/0058-tiktok-transport-tiers-by-entitlement.md`).
5. **Signer-side room list** — DONE per the plan's own note: the fallback
   does not ride `SIGNER_RELAY_CANARY_ROOMS`; a separate gate
   `SIGNER_RELAY_FALLBACK=on` opens `GET /v1/stream/:username` to any
   warm-tab room. Both env flags default off.
6. **Metrics** — `tiktok_fallback_promotions_total{username,outcome}`
   (promoted | not_premium | relay_unavailable) and
   `tiktok_fallback_deliveries_total{username,outcome}` (delivered |
   no_messages | decode_failure | replay_error).
7. **Bug fixed en route (phase 2)**: the canary consumer called the async
   `deserializeWebSocketMessage` without awaiting it, so every relay frame
   counted as an ack — `method_set` and `decode_failure_rate` divergences
   could never fire. Fixed; regression test feeds a real wire-format
   PushFrame. Both consumers now share `RelayStream`
   (`src/canary/relay-stream.ts`) for the SSE/backoff machinery.
8. **Lab verification (2026-09-17)**: signer gate open (non-canary room
   answers 409-after-capture, not 404); a live `FallbackConsumer` harness
   against the real relay delivered 57 frames / 10 chat events / 0 decode
   failures on bigjaygaming01; the premium check answered true/false
   correctly against lab-pg. The promotion trigger itself (a genuine flap
   wall) cannot be summoned on demand in the lab — every stage of its
   path is individually proven and the decision logic is unit-tested.
   First prod flap-exhaustion after Phase 4 turns the flag on is the real
   test; watch `tiktok_fallback_promotions_total`.

## Phase 4 — ship ⬜ NEXT (fresh sessions start here)

All implementation work is done and committed on
`fix/tiktok-viewer-vulkan-regression`; what remains is rollout:

- Config + `caesar-deployment` PR: revert #104 direct-egress to lane
  egress (lanes healthy again per the phase report), flap classification +
  breaker + canary env; both fallback flags stay off initially.
- Pick 1-2 canary rooms (premium users' rooms — double duty as the
  fallback tier) and set `SIGNER_RELAY_CANARY_ROOMS` (signer) +
  `TIKTOK_CANARY_ROOMS` (listener) to match.
- Docs: service READMEs + ADR-0058 shipped in the phase-3 commit; nothing
  further owed.
- Rollout order: phase 1 → lab soak → prod behind breaker → canary on →
  fallback flags on after a week of canary data.
- The `TikTokFallbackPromoted` alert rule
  (`deployments/k8s/monitoring/alerts/allchat-warning-alerts.yaml`, added
  in the phase-3 work) must land with the monitoring config in the same
  rollout. It fires on ANY promotion because promoted users are the
  breakage canary for the whole transport (see the operator note at the
  top of this file).
- Decide before shipping whether to PR from
  `fix/tiktok-viewer-vulkan-regression` or cherry onto a fresh
  `feat/tiktok-transport` branch.

## Not built (deliberately)

- h2 long-poll anything (premise dead).
- Decoding in the signer (relay passes opaque frames; decode stays with
  the connector schemas in the listener).
- `ws_reuse_supplement` Node-WS experiment (superseded — the connector's
  existing WS connect held 10 minutes in the lab).

## Lab rig (kept running; recreated 2026-09-16 evening)

- `signer-lab`: `--network host` with `PORT=18092`. Env:
  `SIGNER_RELAY_CANARY_ROOMS=dan2dxo`, `SIGNER_RELAY_FALLBACK=on`.
  Recreated 2026-09-17 with the phase-3 dist (`/app/dist-old` accumulates
  from swaps — clear as root when it grows). Profiles live at
  `.claude/signer-lab-profiles` (host) → `/profiles`; they must be owned
  by uid 1001 (container runs as `nodejs`), or lane-profile creation
  fails with EACCES. When recreating the container, swap the WHOLE dist
  (`docker cp dist/. signer-lab:/app/dist/`) — copying only index.js
  leaves the old dist's missing module files behind and the entrypoint
  dies with ERR_MODULE_NOT_FOUND.

- `lab-redis` (port 16379), `lab-pg` (15432), `lab-listener` (node dist
  build, host network, `TIKTOK_SIGNER_URL=http://127.0.0.1:18092`, env
  `TIKTOK_CANARY_ROOMS=dan2dxo` and now `TIKTOK_PREMIUM_FALLBACK=on`, no
  `SERVICE_JWT_SECRET` → no leadership gate). lab-listener mounts the
  service dir at `/repo` and runs `node dist/index.js`, so
  `cd /repo && ./node_modules/.bin/tsc` deploys changes live. lab-pg now
  carries the base schema (migration 001) + `users.is_premium` and a
  premium test user (`premiumtest`) whose overlay demands
  bigjaygaming01 — the fallback-eligibility fixture.
- Demand: `docker exec lab-redis redis-cli publish source:demand
  '{"type":"demand_update","sources":[{"channel_id":"<user>",
  "platform":"tiktok","overlay_id":"lab-test"}],"timestamp":"<ts>"}'`
- Admin: `POST /api/reset-backoff {"username":"<user>"}` on :8089 to skip
  backoff after deliberate failures.
- **No local node/npm on the host (NixOS)** — typecheck/test through the
  containers: listener via `docker exec lab-listener`, signer via
  `docker run --rm -v .../tiktok-signer:/repo:ro node:22-slim` (or rebuild
  and `docker cp` dist into signer-lab as root: the container's `nodejs`
  user cannot remove root-owned files).
- **Lab quirks learned the hard way (2026-09-16 evening):**
  - Killing/recreating signer-lab leaves stale Chromium `Singleton*` locks
    in the `/profiles/base*` dirs (pointing at dead container hostnames):
    the signature session wedges ("Timed out... waiting for the WS
    endpoint URL"). Clear with `rm -f /profiles/base*/Singleton*` while no
    chromium runs, then restart.
  - Crashed lane profiles can poison Chromium's proxy tunnel state even
    with correct credentials (`ERR_TUNNEL_CONNECTION_FAILED` on every
    lane while undici through the same proxy+creds returns 200): wipe
    `/profiles/base-viewer-*` and restart. Pool rotation only fires at 3
    consecutive failures per lane, which takes a while.
  - The entrypoint's Xvfb watchdog flaps on some boots ("display did not
    come up within 15s") — a plain `docker restart` usually settles it.
  - lab-pg lacks some prod schema (stream-history/source-active update
    errors in lab-listener logs) — pre-existing, unrelated to transport.
- Test rooms (2026-09-16): diamondslay / burritostreamgr / bigjaygaming01 /
  dan2dxo were live at various points; dan2dxo was the canary proof room.
  asahiicc / tv_whiteshark gated/dead at TikTok's side — do not use as
  negatives. carloxyy_shresthaa failed captures with tunnel errors across
  lanes while dan2dxo captured fine simultaneously — room-correlated,
  exactly the breaker's target shape.

## Estimates

Phase 1: 1 day ✅. Phase 2: 1-2 days ✅. Phase 3: 2 days ✅.
Phase 4: 1 day ⬜.
