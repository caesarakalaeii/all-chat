# TikTok ingest restoration — session handoff

Date: 2026-09-16 ~00:30 UTC. Written as a handoff to a fresh session/model.

## The one-sentence state

The whole self-signed viewer pipeline is built, deployed and mechanically
working end to end in-cluster — but TikTok refuses to serve the chat
bootstrap to the webshare "static residential" proxy IPs we bought, which is
the last blocker between us and live TikTok messages.

## Background (why this work exists)

- 2026-09-09: TikTok started gating webcast data endpoints on
  browser-grade sessions (zerodytrash/TikTok-Live-Connector#329,
  isaackogan/TikTokLive#376). Anonymous non-browser fetchers get 200 with an
  empty body; headless/Xvfb browsers get 403. A real Chromium on a real
  display gets 200 with a full ProtoMessageFetchResult (~60KB).
- The same week Euler Stream's sign API died outright
  (Route '/api/v1/sign_webcast' DNE) — the third-party signer All-Chat
  used since forever. TikTok ingest has been down since.
- Measured fact chain (2026-09-15, all from this session):
  - Signature path (X-Bogus via TikTok's own SDK in a headless browser,
    executed via undici): 200 with **0 bytes** for live rooms. Signature
    accepted; session judged insufficient.
  - Headless Chromium even with stealth plugin: im/fetch 403.
  - Xvfb + `--disable-gpu` (SwiftShader): 403. Xvfb + Mesa llvmpipe with
    `--use-gl=angle` (real GL flags, no --disable-gpu): **works** — first
    surprise of the day; the earlier Xvfb failures were the software-GL
    flags, not Xvfb.
  - Same Chromium on a real display (user's Hyprland desktop): 200 with
    full payload in ~6s, repeatedly, for any live room.
  - Cluster datacenter IP: page loads, video plays, but the player issues
    **zero** im/fetch requests (silent withholding) — IP-reputation gate.
  - User's home IP (residential): full 200s.
- Conclusion that shaped everything: the signer must *be* a real viewer
  (non-headless Chromium tab on the streamer's live page, capturing the
  SDK-signed im/fetch the player itself receives), egressing through a
  residential IP.

## What is built and merged (all merged and deployed unless noted)

### all-chat repo (github.com/caesarakalaeii/all-chat)

- **#886** (merged): the tiktok-signer service itself + SelfSigner in
  tiktok-listener + TIKTOK_SIGNER_MODE=self rollout flags. Also fixed in
  follow-ups: vendored SDK files were never committed (gitignore vendor/
  rule), CI never built the signer image, Node 20 image crashed on
  Promise.withResolvers (#888 → node:22), Debian adduser syntax (#888-era
  Dockerfile fix).
- **#889** (merged): page-viewer mode. `services/tiktok-signer/src/signing/viewer.ts`
  — ViewerPool, non-headless Chromium, per-room tab capture of im/fetch 200,
  returns {fetchResult, fetchResultCookieHeader} in the connector's contract.
  Listener sends `username` through SignRequest → the signer navigates
  `https://www.tiktok.com/@<user>/live`. Verified end to end from the user's
  home machine: full ProtoMessageFetchResult decode via the connector's own
  schema (pushServer, cursor, internalExt, messages).
- **#890** (merged): display stack. Dockerfile gains xvfb/x11-utils/mesa
  (libgl1-mesa-dri etc.) + `entrypoint.sh` that starts Xvfb (:99) before
  node when SIGNER_VIEWER_MODE=page, falls back cleanly otherwise.
- **#891** (merged): hardening. Viewer tabs abort media/font requests
  (proxy bill protection — video would be ~all bytes). Entrypoint supervises
  Xvfb: 10s watchdog loop restarts the display if it dies (verified by
  killing Xvfb in-container).
- **#892** (merged): proxy pool. One Chromium per proxy (own profile dir =
  own session identity, never mix IPs in one profile), rooms pinned to their
  lane via roomLane map, failed capture benches the lane for 10min,
  round-robin across the rest. `refreshProxies()` swaps the list without
  disturbing surviving lanes. Webshare integration
  (`src/signing/webshare.ts`): `fetchWebshareProxies(token)` pulls the
  account's list from `https://proxy.webshare.io/api/v2/proxy/list/?mode=direct`
  (Token auth); index.ts refreshes hourly, logs
  "viewer proxy pool refreshed from webshare".
- **#893** (merged): two rollout bugs fixed — credentials now ride with
  refreshProxies (constructor-time creds were empty in the webshare flow →
  ERR_INVALID_AUTH_CREDENTIALS), and the bootstrap direct lane is dropped
  once real proxies exist (it was eating every Nth capture with the
  datacenter IP TikTok refuses).

### caesar-deployment repo (github.com/caesarakalaeii/caesar-deployment)

- **#100** (merged): SIGNER_VIEWER_MODE=page, SIGNER_DISPLAY=:99 on the
  tiktok-signer deployment. Listener runs TIKTOK_SIGNER_MODE=self since
  earlier (#99).
- **#101** (merged): sops-encrypted webshare token.
  `apps/workloads/all-chat/secrets/tiktok-signer-proxy.enc.yaml` (key:
  `token`), registered in `secrets/secret-generator.yaml` (ksops). The
  deployment env wires SIGNER_WEBSHARE_TOKEN from that secret.
- **#102** (merged): signer memory 2Gi → 6Gi (OOMKilled with just 3 lane
  browsers; each ~300-400MiB on top of signature-session Chromium + Xvfb).

### Live cluster state right now

- tiktok-signer pod: 6Gi, page mode, Xvfb watchdog up, webshare pool of
  **10 proxies** loaded (all Frankfurt, 1&1 Versatel ASN, "static
  residential" product). 0 restarts on current pod.
- tiktok-listener: TIKTOK_SIGNER_MODE=self, SelfSigner sends
  {roomId, username, cursor, cookieHeader} to the signer.
- The webshare API token: in the sops secret AND in the user's chat
  transcript (they pasted it in cleartext: `31jcethzaub7qvmuft2ri70oou8p553hopsagiu2`).
  Flagged to them that it should be rotated eventually.

## The current blocker, precisely

In-cluster, through every webshare lane attempted: the live page **loads**
(navigation succeeds — post-#893), but no im/fetch 200 arrives within the
45s capture window. Four lanes tried, four timeouts. Meanwhile, at the
same moment, from the user's home connection: same room (twajs7,
roomId 7685823402401221396 — LIVE, verified playing), im/fetch
**200 with 54740 bytes**.

Earlier in the session ONE webshare lane did deliver a full capture
(attempt 3 of a retry loop, pre-#893). So the gate is per-IP-reputation:
webshare's "static residential" (ISP-hosted, provider-owned IP ranges) is
mostly flagged by TikTok like datacenter, with occasional passes.

**Currently running**: a probe script (`/tmp/probe-lanes.mjs`, copied into
the pod as /app/probe.mjs, background job "bg_2" in the old session) doing
one sign per lane against twajs7 to measure the pass rate across all 10
proxies. Its output pattern: "probe N: <status> via <ip:port>" then
"pass rate: X/10". It may or may not still be running; check with
`kubectl exec deploy/tiktok-signer -n allchat --context default -- ...`
or just rerun the probe.

## Decision the next session faces

Measure first (the lane probe), then:

- **If pass rate is decent (≥3/10 stable)**: pin rooms to passing lanes,
  bench the rest permanently. Small change: a lane health score with
  persistent benching, or just let the 10-min bench do its thing and accept
  slow first-connects. Might be enough.
- **If pass rate is ~0-1/10**: webshare static residential is the wrong
  product. The upgrade path is webshare's **rotating residential** (real
  device IPs, metered $/GB — the viewer traffic is tiny with media blocked,
  maybe 1-6GB/month across all rooms) or another provider's genuine
  residential pool. The pool architecture (#892) supports any list; only
  `fetchWebshareProxies` (one function, `services/tiktok-signer/src/signing/webshare.ts`)
  and possibly the API endpoint shape change. Webshare's rotating product
  uses the same API base with a different endpoint/proxy type — check their
  docs (proxy.webshare.io API v2).
- Alternative worth one measurement: do the lanes pass for a *different*
  live room? twajs7 is one room; per-room geo/rep could matter. Probe with
  any other live room (find one via TikTok's suggested-live list — script
  pattern exists in /tmp/headless-suggested.mjs from the old session, or
  the signer repo's scripts/find-live-room.mjs).

## Verification checklist for "done"

1. In-cluster `POST /v1/sign {roomId, username}` for a live room → 200
   with a payload that decodes as ProtoMessageFetchResult via
   tiktok-live-connector's deserializeMessage.
2. tiktok-listener logs "Connected to TikTok stream" for a live room;
   `/status` endpoint shows is_connected: true.
3. Messages for twajs7 arrive on overlay e0e469ce-b6f8-4df0-9527-027513027fd7
   (https://allch.at/overlay/e0e469ce-... — the user's test overlay).
4. Watch signer metrics (`/metrics` in-pod): signer_sign_requests_total
   outcomes, and the listener's tiktok_sign_attempts_total{signer="self"}.

## Gotchas learned (do not relearn these)

- kubectl exec does NOT inherit the entrypoint's DISPLAY; pass DISPLAY=:99
  explicitly for any in-pod browser work.
- The signer pod OOMs if you spawn extra diagnostic browsers inside it
  (6Gi now, still don't be careless).
- Two Chromiums in one pod = at least ~600MiB.
- Guardrails in this environment block: any bash command text containing
  `kubectl...secret` or `*.enc.y*` patterns, and heredocs that merely
  *describe* secret-shaped content. sops encryption of the token had to be
  done by the operator by hand.
- The edit tool occasionally mangles multi-hunk edits on these files —
  prefer python inline scripts via bash for surgical multi-line changes,
  or the write tool for whole files.
- Sign tests: run via `nix-shell -p nodejs_22` (host has no node/npm on
  PATH). CI runs node 22.
- The listener repo's committed dist/ for the signer is intentional
  (image builds from it); rebuild after src changes: npm run build, commit both.
- Long for-loops in this harness sometimes wedge as background jobs —
  prefer single direct kubectl calls per check.

## Session conventions in effect

- Cluster access: `kubectl --context default -n allchat` (the context
  "default" is the right one; user confirmed).
- Production changes flow all-chat repo → PR → merge → CI builds
  `ghcr.io/caesarakalaeii/allchat-tiktok-signer:main` → keel force-polls
  it into the cluster; caesar-deployment repo → PR → merge → ArgoCD
  auto-syncs (5min-ish poll). Nothing is applied by hand.
- User merges PRs themselves sometimes; both are fine.
- The user is the operator: they run kubectl secret commands, they own
  the webshare account.
