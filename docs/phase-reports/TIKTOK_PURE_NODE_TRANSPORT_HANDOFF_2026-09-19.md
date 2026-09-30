# TikTok pure-Node transport — agent handoff (2026-09-19, end of PR 2)

Whoever picks this up: PR 0, PR 1 and PR 2 are **done, committed, and
pushed**. PR 3 (canary detection + premium fallback wiring) is next.
Read this top to bottom before touching anything.

Branch: `docs/tiktok-pr0-gate-tests` (pushed, tracks origin). PR 2's
merge into main is the operator's call — PR 3 builds on the branch
either way.

## Companion docs, same directory

- `TIKTOK_PURE_NODE_TRANSPORT_PLAN_2026-09-18.md` — the work order. Its
  PR 3 section is the starting contract; apply the deltas below.
- `TIKTOK_NO_BROWSER_TRANSPORT_SPIKE_2026-09-18.md` — the effort's log.
  Its last three sections are PR 2's record (Task 1 shape falsification,
  the webshare retirement, the soak pass). Trust the supersede markers,
  read top-down once.
- `TIKTOK_PURE_NODE_TRANSPORT_HANDOFF_2026-09-18.md` — the PR 0-era
  handoff. Superseded on process details by this file; its lab-rig
  section is still accurate.

## The one-paragraph state of the world

Pure-node delivery is **proven end to end**: one warm classic room's
leased WS session (cookies + recorded wsUrl) serves every room's chat via
cross-room `im_enter_room`. The listener's `TIKTOK_SIGNER_MODE=pure-node`
connected two live rooms for 31 minutes with zero lease refreshes, zero
sign failures, zero 403s, and Euler fully off the connect path. What is
NOT yet wired: the premium fallback tier (relay) is honestly OFF in
pure-node mode — the relay needs a target-room warm viewer tab that the
lease never creates — and the canary mirror (divergence detection)
cannot attach for the same reason. PR 3 wires both.

## WEBSHARE IS DROPPED (operator decision, do not forget)

Spike report, "Egress policy" section: the datacenter IPs have better
reputation than the webshare residential proxies. No
`SIGNER_WEBSHARE_TOKEN`, no proxy lanes, no lane-pin criteria. Lab and
prod run DIRECT egress. Do not re-introduce webshare wiring in any PR
without an explicit operator instruction. Any plan or test that
presupposes proxy lanes (the PR 2 plan's Task 5 `pinned` clause did) is
void on that clause.

## What PR 2 shipped (commit a81af4fd)

Listener side, all in `services/tiktok-listener/src/sign/`:

- `pure-node.ts` — `PureNodeSigner`: GET /v1/session lease (10-min
  cache, single-flight), 8/hour lease budget (successes only),
  12/hour per-pod WS-connect budget (the `ws-pin` pre-sign excluded),
  bounded classification (401/404 → `signature`, 503/ECONNREFUSED →
  `network`). Synthesizes the fetchResult: **routeParams forwards the
  lease wsUrl's recorded identity params** minus the four keys the
  connector appends itself (`compress`, `room_id`, `internal_ext`,
  `cursor`), pushServer = origin+path, cursor `'0'`. The bare-origin
  shape is FALSIFIED — measured handshake rejection (spike report Task 1).
- `sign-clients.ts` — `pickSignClients` (mode-gated construction;
  euler/shadow unchanged, pure-node constructs no SelfSigner) and
  `fallbackPromotionAvailable` (pure-node → FALSE honestly). NOTE:
  the predicate is tested but NOT yet consumed — index.ts:1654 still
  has the inline `!signerUrl || !this.selfSigner` guard. PR 3 should
  route the guard through the predicate (drift path left open
  deliberately, both council lenses flagged it).
- `config.ts` — `SignerMode` gains `'pure-node'`;
  `eulerStillReachableForSignature` pure-node → false;
  `enableExtendedGiftInfo` stays self-only.
- `installer.ts` — `signers.pureNode` case (MeasuredSigner, no Euler
  fallback), the `:244` effective-reachability check extended;
  `asRouteHandler` seeds EVERY cookie pair into the jar (the connector
  absorbs only the first pair) with a per-pair try/catch.
- `index.ts` — pickSignClients wiring, pre-sign re-keyed to
  `pureNodeSigner ?? selfSigner`.

Signer side (PR 1, commit 2845b5df): identity module, CDP wsUrl
capture, `GET /v1/session` four-phase lease endpoint with
`signer_session_leases_total{outcome}`, canary profile isolation,
dual-listing startup refusal.

Tests: 483 listener tests green (121 in src/sign/), 63 signer tests
green. One pre-existing 1ms-tick flake in
`connection-decisions.test.ts:166` (fails on a clean tree
occasionally) — worth a separate one-line fix, NOT in PR 3's diff.

## PR 3 per the work order + measured deltas

Plan section "PR 3 — canary detection + premium fallback wiring":
`src/canary/canary-consumer.ts` (primary feed switch),
`src/fallback/fallback-consumer.ts` (trigger set), Grafana rules via a
separate caesar-deployment PR.

Deltas from what PR 2 measured/decided:

1. **The fallback executor already exists and is the right one**: the
   relay (`GET /v1/stream/:username`) attaches to a warm viewer tab.
   PR 3's promotion path is: on a pure-node flap-exhausted premium
   room, ask the SIGNER to warm a tab for the target room
   (`POST /v1/sign` with `{roomId, username}` does exactly this — the
   pre-sign comment at index.ts:1674-1677 documents it) and then
   promote via the existing FallbackConsumer. The signer runs the
   capture; the listener never signs per-room.
2. **`fallbackPromotionAvailable` wants wiring**: replace index.ts's
   inline guard with the predicate (its test matrix already covers
   every mode), then extend the predicate for PR 3's new availability
   condition (pure-node + warmable tab → true).
3. **Canary mirror in pure-node**: the 409 retry loop observed in the
   soak is the canary consumer failing to attach (no warm tab). PR 3's
   primary-side feed needs the same warm-a-tab step, or the canary
   rooms list stays empty under pure-node and the divergence
   classifier keeps running on whatever tier the canary room is
   actually delivered by. Decide this in the plan council — it is a
   real fork (warm-per-canary costs signer captures; empty canary
   costs the divergence signal during pure-node rollout).
4. **Demotion triggers** per the plan: room-end, or N minutes of
   healthy primary on the promoted room. The capture breaker on the
   signer side paces any warm-tab churn — respect the single-digit
   captures/hour budget when designing N.

## Process rules that carried three PRs (keep them)

Plan → council (feasibility/decomposition/criteria lenses for plans;
correctness/design/tests/fit lenses for artifacts) → build → artifact
council → commit. Blocking verdicts honored; re-convene after fixes;
verify claimed artifacts. The councils caught real defects every
round: PR 2's plan survived 5 rounds, and a mid-plan live measurement
FALSIFIED the plan's own WS-shape hypothesis (run the measurement, read
it honestly, re-convene on the amendment — that loop worked exactly as
designed). Budget discipline is binding forever: single-digit TikTok
requests per surface per hour, ≥15 min between surfaces, one 403 = stop
that surface for the hour. Conventional commits, no emoji, no AI
attribution, smallest honest diff, docs in the same PR. `git push` and
merges are the operator's keypress.

## Lab rig state (as left, 2026-09-19)

- `signer-lab`: PR 1 image (`allchat-tiktok-signer:pr1`, built from a
  scoped context — see below), viewer-mode,
  `SIGNER_WARM_ROOMS=zaganovakov`, `SIGNER_AUTH_TOKEN=labtoken_pr2_task1`,
  host network, port 18092 (the image's unhealthy badge probes 8092 —
  always ignore it; `curl 127.0.0.1:18092/health/live` is the truth).
- `lab-listener`: node:22 image running the PR 2 dist in **pure-node
  mode** (`TIKTOK_SIGNER_MODE=pure-node`,
  `TIKTOK_SIGNER_URL=http://127.0.0.1:18092`), host network, repo
  mounted at `/repo`. STILL CONNECTED to the two soak rooms and
  delivering. Restart it to change modes.
- Lab DB has two extra sources (`zaganovakov`, `kyle.toynbee` on the
  seed overlay); demand snapshots are published by hand:
  `docker exec lab-redis redis-cli -p 6379 PUBLISH source:demand
  '{"type":"demand_update","timestamp":"<now>","sources":[{"source_id":"s1","channel_id":"<user>","platform":"tiktok","overlay_id":"22222222-2222-2222-2222-222222222222"}]}'`
  (there is no source-manager in the lab; the listener only connects
  what arrives on that channel).
- Image-build gotcha: `docker build` from the repo root FAILS — the
  root-owned `.claude/signer-lab-profiles` breaks the context tarball
  even though `.dockerignore` excludes `.claude/` (buildx ignores
  excludes on unreadable dirs; legacy builder also broken). Build from
  a scoped context: copy `services/tiktok-signer/` + `LICENSE` to
  /tmp, build there. The old lab instruments (spike-live2.cjs etc.)
  were in the PREVIOUS signer-lab image and are gone; write new ones
  with the `write` tool + `docker cp` (host /tmp does not see
  container /tmp).
- Liveness check without a browser: `GET
  https://www.tiktok.com/api-live/user/room/?uniqueId=<user>&sourceType=54&aid=1988`
  with a Chrome UA + Referer; `data.user.status === 2` was live,
  `4` offline, at last use. The listener's own `fetchIsLive()` stays
  the authority before connecting.

## Verified rooms (2026-09-18, churn fast — re-verify)

zaganovakov (classic, `SIGNER_WARM_ROOMS`), dan2dxo (canary set) and
kyle.toynbee were live at soak time; jinuabi /
neringakazlauskaite / sarameels served live_new or offline. Rooms churn
in minutes: re-verify liveness before any capture or soak.

## Next steps, in order

1. Read the three companion docs (plan's PR 3 section, spike report's
   last three sections, this file's deltas).
2. Draft the PR 3 plan (the PR 2 plan at `local://tiktok-pr2-plan.md`
   in the prior session's artifact store is the format/altitude model;
   re-create it — the goal texts must be self-contained per task).
3. Plan council (feasibility/decomposition/criteria). Rule the canary
   fork (delta 3) explicitly.
4. Build, artifact council, commit. Push is the operator's.
