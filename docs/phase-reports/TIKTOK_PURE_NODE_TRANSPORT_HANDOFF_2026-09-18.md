# TikTok pure-Node transport — agent handoff (2026-09-18, end of PR 0)

Whoever picks this up: PR 0 (gate tests) is **done, committed, and pushed**.
PR 1 is next, and its design got **simpler than the original plan** because of
what PR 0 measured. Read this top to bottom before touching anything.

Companion docs, same directory:
- `TIKTOK_PURE_NODE_TRANSPORT_PLAN_2026-09-18.md` — the work order (four PRs,
  hard constraints, per-PR file lists). Constraints 1-6 there are all still
  binding.
- `TIKTOK_NO_BROWSER_TRANSPORT_SPIKE_2026-09-18.md` — the effort's log. Its
  last four sections are PR 0's record; superseded readings carry explicit
  `[Superseded: …]` markers — trust the markers, read top-down once.
- `TIKTOK_SIGNING_RESEARCH_2026-09-16.md` — the #897-tier background.

Branch: `docs/tiktok-pr0-gate-tests` (pushed). Commits `98aaef70` (PR 0
results) and `7c7d6547` (cross-room entry). Docs only — no code changed yet.

## The one-paragraph state of the world

TikTok will not accept any signature we compute outside a real browser page
(`#2: harvest-needed`). The only accepted signatures are the ones a live
page computes (X-Gnarly + X-Dynosaur). BUT one captured session — cookies plus
WS URL from any single classic room — serves **every** room's chat delivery
(cross-room entry, proven 16:55-16:59 UTC: enter_room accepted, target room's
chat delivered, target roomId inside every payload). So the end-state
architecture is: **one or two warm classic rooms on the signer mint sessions;
the listener's Node client carries everything else** (room lookup, enter,
heartbeat, ack, delivery). No per-room browser tabs.

## What is proven (all measured 2026-09-18, lab rig)

| Fact | Consequence |
|---|---|
| im/fetch accepts plain-Node replays of a page URL (49.8 KB, no cookies) | No TLS/browser impersonation needed on the fetch leg |
| sign-url (X-Bogus only) → empty-200 across 3 identities | The signer cannot mint signatures in-page alone; a real page must compute them |
| Signature binds room_id (`#1: per-room`) | Per-room URLs — but see cross-room entry: the WS session is NOT per-room |
| URL decay inside a minute; rotated msToken doesn't revive (`#3`) | Sign at connect; never cache a harvest longer than the connect it serves |
| UA must match verbatim — version drift AND platform swap rejected (`#5: ua-exact-match`) | Identity is pinned end-to-end; PR 1's identity module is load-bearing; do not "track upstream Chrome majors" |
| WS needs cookies (`#4`) | `fetchResultCookieHeader` stays in the contract; the session jar is the asset |
| WS URL works ≥16 min old, unsigned | The push surface is durable; X-Bogus on the WS URL is not required in the connector's flow |
| **Cross-room WS entry works** (16:55) | One classic session serves all rooms; live_new rooms are deliverable; capture pool shrinks to 1-2 warm rooms |
| username→roomId via `api-live/user/room/` works capture-free | Room lookup needs no browser |

## The gotcha that will bite you if you forget it

The classic/live_new page variant is **per-room, not global**. Some rooms
(zaganovakov, kyle.toynbee all day) serve the classic client — im/fetch
present, harvestable. Others (jinuabi, neringakazlauskaite all day) serve
live_new — no im/fetch, uncapturable, and their feed URL is a discovery
listing (373 KB JSON of room summaries), not a chat bootstrap. Do not
conclude "TikTok flipped everyone" from a couple of rooms — check the
variant per room (harvest once, look for `imfetch-present`). Because of
cross-room entry, live_new rooms are no longer a delivery problem, only a
"don't panic" note.

## Lab rig (all commands, no host node/npm — NixOS)

- Containers: `signer-lab` (image healthcheck shows "unhealthy" — it probes
  8092 while the service listens on 18092; truth: `curl 127.0.0.1:18092/health/live`
  → `{"status":"ok"}`), `lab-listener` (Node 22, repo at `/repo`, connector via
  `-e NODE_PATH=/repo/node_modules`), `lab-pg`, `lab-redis`. `curlcffi` is an
  image only, no container.
- Signer runs viewer-mode (`SIGNER_VIEWER_MODE=page`): `POST /v1/sign` with
  `{roomId, username}` routes to a real page capture (~3.6 s on a classic
  room, returns full fetchResult + cookies). With username absent it uses
  the signature path — which PR 0 disproved as a standalone bootstrap; don't
  build on it.
- `GET /v1/identity` returns the VIEWER identity in this lab (Linux Chrome/144)
  — NOT the signing session identity (Safari/macOS, `session.ts:55` IDENTITY).
  Experiments needing the signing identity must read `session.ts`, never the
  endpoint.
- Stale Chromium `Singleton*` locks in `/profiles/base*` wedge the session
  after container recreation — clear while stopped.
- HTTP, not HTTPS, to the signer on 18092 (plain `http.request`; an https
  attempt fails with a TLS record-header error).

## Instruments (on containers; re-verify before trusting)

- `signer-lab:/app/spike-req2.cjs` — capture-style harvest (classic rooms
  only). `spike-live2.cjs` — liveness check (player root
  `#tiktok-live-main-container-id` + playing video). `feed-browser.cjs` —
  live-room discovery from the feed page (warm browser, www surface).
- `lab-listener:/tmp/node-fetch.cjs` (verbatim im/fetch replay + decode),
  `node-ws2.cjs` (full WS protocol: enter/heartbeat/ack; no-cookie by default,
  `--cookie` flag), `cross-room-ws.cjs` / `xrw5.cjs` (cross-room entry with
  per-message roomId printing), `roomids.cjs` (username→roomId, www).
  Host-side `/tmp` in omp's bash does NOT see container files: write
  instruments with the `write` tool, `docker cp` them in; for in-container
  edits, patch with a small .cjs patcher file (inline `node -e` with
  escaped newlines corrupts).
- Gate: `/tmp/pr0-gate.sh` (fail-fast, stub-verified; run from repo root:
  `bash /tmp/pr0-gate.sh docs/phase-reports/TIKTOK_NO_BROWSER_TRANSPORT_SPIKE_2026-09-18.md`
  → `PR0-GATE-PASS`, exit 0).

## Request budget discipline (constraint 1 — binding, forever)

~15 im/fetch in ~3 min flags the session (flag outlives an hour); ~15 WS
connects/hour throttles WS. Spent 2026-09-18: ~10 fetch replays (worst burst
7 in 49 s — control stayed data-bearing, zero 403s), ~5 WS connects, all
fine. Rules that held: single-digit per surface per hour, ≥15 min between
surfaces, one 403 = stop that surface for the hour (never re-run to "check"),
fresh control before believing any negative, live-room verification before
any capture (rooms churn in minutes — sarameels died mid-session, jinuabi by
afternoon). Recordings contain live cookies: never commit, never quote.

## What PR 1 should build (the measured design)

The plan's PR 1 section predates the cross-room finding — read it, then apply
these deltas:

1. **Identity module** (unchanged from plan): one object (UA + platform +
   os + screen) consumed by session.ts IDENTITY, api.ts imFetchParams
   (delete the hardcoded MacIntel/mac contradiction), GET /v1/identity.
   ua-exact-match makes this load-bearing.
2. **Signer endpoint**: expose the *shared session*, not per-room signatures:
   the warm classic room's WS URL + cookie jar, freshness-stamped. The
   listener re-enters it per target room. Keep `POST /v1/sign` (page capture)
   as the bootstrap for backlog cursor/internalExt when the target room is
   classic; live_new targets enter with `cursor: ""` (room state pushes on
   enter — morning spike finding).
3. **Drop from plan**: `performSignUrl` allowlist extension (WS URL is
   unsigned in prod, PR 0 #6a), any per-room harvest cadence machinery
   (session is shared; harvest only when the warm session dies).
4. **Canary isolation + tests**: per plan (dedicated `SIGNER_CANARY_PROFILE`,
   identity golden test, endpoint contracts).

PR 2 (listener `PureNodeSigner` → now more of a session-client), PR 3
(canary detection + premium fallback), PR 4 (rollout): per plan, with the
same deltas — no per-room signing in the listener, cross-room entry at
connect, cookies forwarded from the shared session.

## Process rules that carried this session (keep them)

Plan → council (feasibility/decomposition/criteria lenses, blocking verdicts
honored, stub-test the gate before trusting it) → measure under budget →
record in the spike report with supersede markers when corrected → gate
green → review council on the artifact → commit. The council caught real
defects every round (decorative gate, stale prose contradicting measured
results, a false budget claim) — do not skip it to save time. Prod is
GitOps; from coding sessions never touch prod; `git push` is the operator's.
