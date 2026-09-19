# TikTok pure-Node transport — build handoff

Self-contained work order for an implementation agent. Design rationale,
measurement history and protocol details live in
[TIKTOK_NO_BROWSER_TRANSPORT_SPIKE_2026-09-18.md](./TIKTOK_NO_BROWSER_TRANSPORT_SPIKE_2026-09-18.md)
(the spike report — read its "The im/fetch replay, exactly" and "The WS leg,
exactly" sections before touching protocol code) and
[TIKTOK_SIGNING_RESEARCH_2026-09-16.md](./TIKTOK_SIGNING_RESEARCH_2026-09-16.md)
(the #897 tier design this builds on).

Mission: make every room's chat ride pure-Node connections (im/fetch +
WS held by the listener, no per-room viewer capture), with a kept Chromium
canary that detects pure-Node breakage and pulls premium rooms back to
the Chromium-based relay (viewer-tab fallback).

## Hard constraints (read first — agents have tripped on all of these)

1. **Gate discipline.** The im/fetch and WS surfaces flag sessions by
   burst rate: ~15 fetches in minutes 403'd the whole session (including
   the browser's own fetches); ~15 WS connects in an hour throttled the WS
   surface. Budget: single-digit requests per surface per hour during any
   testing. One 403 is noise; require N consecutive on a verified-live
   room before believing a negative. Live-room discovery: `tiktok.com/live`
   feed links; verify with player root `#tiktok-live-main-container-id` +
   playing `<video>` (the capture flow's own signal — see
   `LIVE_PAGE_SELECTOR` in `services/tiktok-signer/src/signing/viewer.ts`).
2. **Toolchain.** NixOS host: no node/npm. Typecheck/tests via the
   `lab-listener` container (repo mounted at `/repo`, tsc in
   `node_modules/.bin`), signer builds via its Dockerfile (multi-stage,
   builds from src — `dist` in the image is rebuilt, never hand-copied).
   Lab rig state and quirks: `docker ps` for `signer-lab` / `lab-listener`
   / `lab-redis` / `lab-pg`; stale Chromium `Singleton*` locks in
   `/profiles/base*` wedge the session after container recreation (clear
   while stopped); the "unhealthy" badge on signer-lab is the image
   healthcheck probing 8092 while the service listens on 18092 — check
   `curl 127.0.0.1:18092/health/live` for truth.
3. **Identity consistency is the load-bearing fix.** TikTok silently
   empty-200s any request whose User-Agent disagrees with the URL's
   self-describing params (`browser_platform`, `browser_version`, `os`).
   This is what the 2026-09-18 morning spike misdiagnosed as TLS. The UA
   must equal `GET /v1/identity`'s, and the params must describe the
   same platform — everywhere, forever.
4. **Recordings and spike artifacts on /tmp contain live session
   cookies.** Never commit them; never paste cookie values anywhere.
5. Prod is GitOps (caesar-deployment, Argo selfHeal) — manual kubectl
   mutations revert; cluster changes go through deployment PRs. From
   coding sessions, never touch prod; local lab only. `git push` is
   prompt-gated (operator's keypress).
6. Conventional commits, no emoji, no AI attribution, smallest honest
   diff. Docs updated in the same PR as the code (see "Same-PR
   obligations" per phase).

## Ready-made assets (on host /tmp)

- `spike-req2.cjs` — capture-style tab recorder (verbatim im/fetch URL,
  headers, cookies, WS URLs). Its request interception mirrors
  `newLanePage` exactly — do NOT "improve" the tracker regex: an
  aggressive one (e.g. `log` matching `login`) aborts the webapp bundles
  and produces a zombie page.
- `node-fetch.cjs` — pure-Node bootstrap fetch + connector decode.
- `node-ws2.cjs`, `ws-retry.cjs` — pure-Node WS delivery with the full
  enter/heartbeat/ack protocol (reference implementation of what the
  connector already does internally).
- `spike-e2e.sh <room>` — one-shot end-to-end (harvest → pure-Node fetch →
  pure-Node WS chat).
- `replay*.py`, `rotate.py`, `decay.py` — fetch-replay instruments
  (python + curl_cffi; run in the `curlcffi` image).
Run spike scripts inside the containers (`docker exec signer-lab node
/app/...`, `docker exec -e NODE_PATH=/repo/node_modules lab-listener
node /tmp/...`).

## Build workflow — four PRs, each gated on the previous

### PR 0 — gate-test batch (no code; resolves the design forks)

Six single requests on a healthy gate + code recon. Each answer picks a
branch in PR 1/PR 2. Run from the lab, one request at a time, ≥15 min
between surfaces. Append results to the spike report.

| # | Experiment | Decides |
|---|---|---|
| 1 | room_id-swap on a fresh recorded URL (keep signatures, change room_id) | If data: one harvest serves all rooms. If empty: sign per room. |
| 2 | `POST /v1/sign-url` (X-Bogus + fresh msToken from signing page) + consistent UA → fetch | If data: bootstrap needs no Gnarly/Dynosaur, no canary harvest — signing session alone feeds tier 0. |
| 3 | Rotate `x-ms-token` from response headers into a fresh signed URL | If data: harvests long-lived. If not: every URL ~5-min fresh, sign per connect. |
| 4 | WS connect without cookies (spike client, drop `--cookie`) | If data: `fetchResultCookieHeader` becomes vestigial. If not: signer must expose the warm jar. |
| 5 | UA variants: Linux Chrome/152, Windows Chrome/144 | How brittle the gate is; whether identity can track upstream Chrome majors. |
| 6 | Code recon (no requests) | (a) how the connector builds/signs its WS URL in self mode (`sign/installer.ts`, connector dist `lib-YL2P_UWg.js`); (b) `fetchRoomIdFromApiRoute` usability for username→roomId without capture; (c) prod `TIKTOK_SIGNER_MODE` + relay flag state in caesar-deployment; (d) overlap with the `feat/retire-euler-tiktok-signing` branch (pure-node requires self mode; land euler-retire first or together). |

Acceptance: spike report has the six answers; every fork below is resolved
to a named branch.

### PR 1 — signer service

Files: `services/tiktok-signer/src/signing/identity.ts` (new),
`src/api.ts`, `src/signing/session.ts`, `src/signing/viewer.ts`,
`src/api.test.ts`, signer README, Dockerfile untouched.

1. **Identity module**: one object (UA + platform + os + screen) consumed
   by `session.ts` IDENTITY, `api.ts` `imFetchParams` (delete the
   hardcoded `MacIntel`/`mac` — it contradicts the Linux/Chrome-144
   identity and is the standing instance of constraint 3), and
   `GET /v1/identity`. Exported shape unchanged.
2. **Signing endpoint per PR 0 #2**: if sign-url suffices → `POST
   /v1/sign-fetch` (builds the identity-consistent im/fetch param set
   server-side, signs in-page, returns `{ signedUrl, userAgent,
   cookieHeader?, signedAt }`; no navigation, ~100 ms). If not → `GET
   /v1/harvest` off the canary tab (CDP `requestWillBeSent` of its own
   im/fetch: URL template + age), harvest cadence per PR 0 #3.
3. **WS-URL signing seam** per PR 0 #6: extend `performSignUrl`'s
   allowlist to the push host (`webcast-ws.*.tiktok.com`) if the
   connector doesn't already cover it.
4. **Canary isolation**: canary tabs on a dedicated profile dir
   (`SIGNER_CANARY_PROFILE`, default sibling of `SIGNER_USER_DATA_DIR`)
   so primary-tier flags (shared-jar propagation, spike-proven) cannot
   blind the canary.
5. Tests: param-set ≡ UA consistency (golden test), endpoint contracts,
   allowlist. Run: `docker exec lab-listener sh -c "cd /repo && npm test
   --workspace"` — no: signer tests run in its own image build; use
   `docker run --rm -v $PWD/services/tiktok-signer:/app -w /app
   node:22-bookworm-slim sh -c "npm ci && npm test"`.

Acceptance: identity golden test green; sign-fetch (or harvest) returns a
URL that `node-fetch.cjs` turns into ≥25 KB protobuf on a live room; README
documents the new endpoint(s) and the identity invariant.

### PR 2 — listener `PureNodeSigner`

Files: `services/tiktok-listener/src/sign/pure-node.ts` (new),
`src/sign/config.ts`, `src/sign/installer.ts`, `src/sign/config.test.ts`,
`src/sign/self.ts` (reference), listener README.

1. Implement `WebcastSigner` (`sign/signer.ts`): username → roomId
   (connector's direct-to-TikTok route; PR 0 #6b) → signer sign endpoint
   (PR 1) → undici GET, headers = identity UA + `Referer:
   https://www.tiktok.com/` only → `deserializeMessage(
   'ProtoMessageFetchResult')` → `SignResult`. Empty body / 403 →
   re-sign once → classify `tiktok_rejected` (existing reason set) on
   repeat. Freshness budget from signedAt per PR 0 #3.
2. Config: `TIKTOK_SIGNER_MODE=pure-node` (new value in config.ts,
   installer wiring mirroring self). Extend `shadow` to compare
   pure-node vs capture outcomes on live rooms (the soak instrument).
3. Reconnect discipline: jittered backoff exists; add per-room connect
   budget so a flap storm cannot burst-flag the session (constraint 1 is
   a production property too).
4. Tests: SignResult contract (relay fixtures in `canary/relay-fixtures.ts`
   encode proto shapes), stale/empty/403 classification, budget
   behavior, config table. Typecheck+tests: `docker exec lab-listener
   sh -c "cd /repo && npx tsc --noEmit && npm test"`.

Acceptance: shadow mode on 2 lab rooms decodes identical message counts
vs the capture path over 30 min; no session flags (check the lab
signer's capture breaker metrics stay quiet).

### PR 3 — canary detection + premium fallback wiring

Files: `src/canary/canary-consumer.ts` (primary feed switch),
`src/fallback/fallback-consumer.ts` (trigger set), `src/fallback/premium.ts`
(unchanged — PremiumChecker stays fail-closed), README, alert docs.

1. Primary-side feed into the existing divergence classifier (frame rate,
   method-shape classes — no protocol change; both tiers decode to the
   same connector schemas).
2. Promotion trigger set: { primary breaker tripped, sustained divergence,
   canary-alive-primary-dead } → `PremiumChecker.isPremiumRoom` → promote
   via existing relay (viewer tab opened by the pool's captureRoom
   keep-open path — no new signer machinery). Non-premium: backoff on
   primary only.
3. Demotion: room-end, or N minutes of healthy primary on the promoted
   room.
4. Grafana: two rules alongside #105's transport alerts (divergence
   sustained; empty-fetch rate on pure-node) — the cluster changes go
   through a caesar-deployment PR, keep it separate from the app PR.

Acceptance: fault-injection test in lab (block pure-node egress for a
premium-flagged test room) promotes that room to relay within one breaker
window and demotes after recovery; alert fires in lab Grafana.

### PR 4 — rollout (days, mostly waiting; operator-driven)

1. Lab soak, 48 h, 2–3 chatty rooms: zero missed-chat divergence vs
   capture path, zero flags. `spike-e2e.sh` is the smoke test.
2. Prod canary rooms (existing `SIGNER_RELAY_CANARY_ROOMS` slots, empty
   since #105): 1–3 rooms, primary stays capture, pure-node shadow runs
   alongside.
3. Cohort flip via caesar PRs: canary overlays → premium cohort →
   everyone. Each step revertible.
4. Steady state: viewer pool scaled to canary + fallback duty only; the
   capture code path stays (it is the fallback executor).

## Same-PR obligations (every phase)

- `services/tiktok-listener/README.md` / `services/tiktok-signer/README.md`:
  mode value, endpoints, tier table, identity invariant.
- Spike report: append PR 0 answers and per-phase results (it is this
  effort's log).
- No new root docs; no new markdown outside docs/phase-reports/.
- Memory: after each phase, `retain` the durable outcome (verdicts, gotchas);
  invalidate superseded ones.
