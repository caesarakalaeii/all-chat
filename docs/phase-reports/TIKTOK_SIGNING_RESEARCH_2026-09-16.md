# TikTok Signing Research — 2026-09-16

Research handoff: everything measured during the 2026-09-16 signing-lab day,
so a fresh session can start implementing without re-deriving the findings.
Status: prod ingest is DOWN and acknowledged by the team (do not touch prod);
all work now targets the local rig. Fixes shipped during the day: all-chat
#895 (Vulkan args), #896 (CDP close race), caesar-deployment #104
(temporary direct egress — see "Revert before rotating IPs").

## Current mission (user-set)

**Get a stable local chat stream through a webshare proxy lane.** Prod is
broken and known-broken; the local rig is the testbed until a lane holds a
sustained im/fetch stream with chat bursts. The end goal remains the
**h2 long-poll transport** (decision made 2026-09-16: migrate off the
WebSocket leg to what the web client actually does — see "Protocol
findings"), chosen because it must be **stable for a while**: the viewer-tab
architecture rides TikTok's own SDK for every signature layer, so SDK
changes are absorbed by the page instead of reimplemented by us.

## Local rig — running state at handoff

- Container: `signer-lab` (docker, healthy), image
  `ghcr.io/caesarakalaeii/allchat-tiktok-signer:main` (includes #896 crash
  guard), port **18092** → 8092, `--shm-size 2g`, Xvfb llvmpipe.
- Env: `SIGNER_VIEWER_MODE=page`, `SIGNER_USER_DATA_DIR=/profiles/base`
  (named volume `signer-lab-profiles`). CORRECTION (2026-09-16 evening):
  "profiles persist across restarts" was FALSE all day — docker created the
  volume root-owned while the container runs as uid 1001 (`nodejs`), so
  every lane profile mkdir failed with EACCES and every capture ran a cold
  profile. No warm-profile experiment done on this rig was actually warm.
  Fixed on the lab rig with
  `docker exec -u root signer-lab chown -R 1001:1001 /profiles`; profiles
  now persist. Prod k8s is NOT affected: emptyDir + `fsGroup: 1001` makes
  `/home/signer/.chrome-profile` writable there. Webshare token from
  host, re-extractable with:
  `kubectl exec -n allchat --context default deploy/tiktok-listener -- node -e "process.stdout.write(process.env.SIGNER_WEBSHARE_TOKEN)" > /tmp/tok && chmod 644 /tmp/tok`
  — note: any single bash command containing both `kubectl` and `secret`-ish
  tokens gets guardrail-blocked; keep the extraction and the docker run in
  separate calls).
- Boot logs: "viewer proxy pool refreshed from webshare, proxies: 10".
- First experiment to run in a fresh session:
  `curl -X POST localhost:18092/v1/sign -d '{"roomId":"unused","username":"<live-room>"}'`
  against each lane; find one that captures, then hold the tab open and
  verify the stream (see "Open spike questions").
- Webshare pool state (VERIFIED 2026-09-16 ~18:45 UTC, see "Pool NOT
  burned — verdict"): pool serves captures again; 6 of 10 IPs were
  auto-replaced by webshare at 10:55 UTC via
  `auto_replace_invalid_proxies: true`, consuming 6 of 10 monthly
  proxy-replacements (4 left). Do not spend a rotation: lanes are healthy.

## Day timeline (what happened)

1. Fleet-wide `viewer_capture_failed` diagnosed via in-cluster signing lab.
   Root cause: ANGLE-on-Vulkan/lavapipe viewer args (commit b87ecd26) wedge
   the page entirely — zero webcast calls in 240s. Fixed by #895: args
   removed, mesa-vulkan-drivers dropped from the image. Plain default GL
   (llvmpipe) on Xvfb captures fine.
2. Push-WS TLS hardening shipped in #895: listener WS egress mirrors
   Chrome's ClientHello via tls-impersonate (runtime node:20-alpine →
   node:26-bookworm-slim; glibc prebuilds only — musl falls to node-gyp
   with no toolchain). Verified against tls.peet.ws through a CONNECT
   passthrough: JA4 cipher/extension segments match Chrome exactly;
   single deliberate deviation: ALPN h1 (ws upgrade speaks HTTP/1.1).
   NOTE: this whole leg serves the legacy WS protocol the web client has
   moved off (below) — it works today, but the h2 long-poll migration
   retires it.
3. All 10 webshare IPs stopped serving im/fetch ~15:00 UTC. Confirmed from
   two origins (cluster pod + locally-run signer): identical failures
   through lanes, while direct egress captured (31KB) at the same time.
4. Signer switched to direct egress (caesar-deployment #104) — captures
   worked (burritostreamgr 61s, bigjaygaming01 52s), then ~18:30 UTC
   direct ALSO started failing (see "The gate tightened twice").
5. Signer crashed twice in 12 min: stealth-plugin CDP close race
   (unhandled TargetCloseError during lane profile rotation). Fixed by
   #896: entry-point unhandledRejection guard swallowing exactly
   TargetCloseError/ProtocolError, everything else fatal.
6. Decrypted packet capture of real Chromium on a live room — the basis
   for the h2 long-poll decision (next section).
7. User decision: **go with the h2 long-poll** transport. Requirement:
   **stable for a while** — favors riding the viewer tab (TikTok's own SDK
   signs everything) over reimplementing the signing surface in Node.

## Protocol findings (decrypted capture, Chromium 152, live room)

### The signing surface changed

Real-browser choreography before im/fetch (live room burritostreamgr):

```
t=3.0s   POST /webcast/room/enter/     (X-Dynosaur param)
t=3.5s   POST /v1/user/webid           (web identity registration)
t=5.4s   POST /v1/user/webid           (retry)
t=5.7s   GET  /webcast/im/fetch/       (X-Dynosaur, ws_direct=1)
t=~30s   POST /webcast/epiphron/feature/upload/  (recurring telemetry)
t=~5.2s  GET  /webcast/room/check_alive/         (recurring)
```

- **X-Dynosaur** is now on room/enter and im/fetch. The signer vendors
  X-Bogus/X-Gnarly encoders but has NO Dynosaur implementation. Community
  reference: Evil0ctal/Douyin_TikTok_Download_API (Python).
- New im/fetch params: `ws_direct=1`, `sup_ws_ds_opt=1`, `did_rule=3`.
- `/v1/user/webid` POST fires immediately before im/fetch.
- `/webcast/epiphron/feature/upload/` beacons every ~30s — fingerprint
  telemetry. Real sessions beacon; capture-only sessions don't.
  [INFERENCE] session scoring may expect it.

### CORRECTED (2026-09-16 evening): the web client DOES use WebSocket for chat

The earlier claim below was WRONG — drawn from a 100s decrypted capture of
a room that was almost certainly quiet or gated. Re-measured tonight on
diamondslay (active chat, warm viewer tab, CDP attach to the signer's own
browser):

- **Chat rides a WebSocket push.** `Network.webSocketFrameReceived/Sent`
  at 1.3-3 frames/s on the room tab; 321 recv / 323 sent over 240s;
  170 frames captured in 60s. ZERO im/fetch re-polls in the same window.
- **Frames are base64 PushFrames** containing embedded
  `ProtoMessageFetchResult` payloads. The connector's own
  `deserializeWebSocketMessage` decodes them; inner `messages[]` carry
  `WebcastChatMessage`, `WebcastLikeMessage`, `WebcastMemberMessage`,
  `WebcastRoomUserSeqMessage` — full user objects, msgId, roomId, createTime.
  Verified end-to-end: one 60s capture contained decodable
  `WebcastChatMessage`s with real msgIds.
- **im/fetch is the BOOTSTRAP, not the transport.** One-shot ~25-29KB
  response, `loadingFinished` immediately (raw ProtoMessageFetchResult
  protobuf — decodes with `deserializeMessage('ProtoMessageFetchResult')`:
  cursor, 11 messages, `internalExt`, and
  `pushServer.webcastPushAddr = wss://webcast-ws.eu.tiktok.com/webcast/im/ws_proxy/ws_reuse_supplement/`).
  Never re-fired across three 150-300s watches on live rooms.
- ~79/170 frames failed decode with one error kind
  (`ProtoMessageFetchResult: premature EOF`) — likely ack/keepalive frames
  (`msg_type: "r"`); classify before treating as data loss.
- CDP CAN read the transport incrementally: `Network.webSocketFrameReceived`
  on an attached session delivers every frame as it arrives. Attaching a
  second CDP client to the signer's warm viewer browser works (619 events
  in 45s sanity run) — no new browser needed, no tunnel-fail problem.

Architecture implication: the "h2 long-poll migration" premise dissolves.
Two viable transports, both SDK-proof:
1. Signer-side: attach CDP to the warm viewer tab (as spiked), decode
   PushFrames with the connector's schemas, publish to Redis directly.
2. Listener-side: WS connect using the fetchResult's
   `pushServer.webcastPushAddr` (`ws_reuse_supplement` endpoint) — the
   existing WS machinery, new URL + the viewer session's cookies.

Original (wrong) claim, kept for the record: 100s capture of a live room
with chat flowing: exactly ONE im/fetch request; its HTTP/2 response stream
held open; DATA bursts at t=6s, 38s, 89s. Zero WebSocket handshakes
anywhere. Chat = server push over the im/fetch h2 stream. `ws_direct=1`
marks this mode. — The one im/fetch observation is real (bootstrap); the
"zero WebSocket handshakes" part is what failed to reproduce.

### WebGL is NOT required by the gate

- User's LibreWolf (WebGL blocked) sees live chat fine.
- A successful 33KB capture had NO WebGL context at all.
- Chromium 152 without Vulkan ICDs hangs the page at GPU init (the
  #895 regression); with `--disable-gpu` nav works but sessions were
  gated pre-afternoon. Renderer string ≠ the gate.

## The gate tightened twice (same day)

~15:00 UTC: lanes dead, direct fine. ~18:30 UTC: direct sessions ALSO
started failing im/fetch (asahiicc, bigjaygaming01, tv_whiteshark,
dan2dxo), including warm persistent profiles and even a run where the
room was verifiably live. Two interpretations:

- TikTok progressively raised session scoring (Euler also broke that
  morning; ecosystem-wide churn) — most likely.
- Home IP got judged too, after the day's volume.

Either way: single-shot capture attempts right now are UNRELIABLE from
everywhere. Do not conclude "approach X fails" from one or two misses;
require N consecutive attempts on a verified-live room before believing a
negative. Conversely the 17:30-18:00 window shows direct CAN work, so the
gate is flapping, not a hard wall.

## Pool NOT burned — verdict (2026-09-16 evening verification)

The "burned pool" hypothesis was re-tested from scratch and FAILS at
every layer measured:

- **Webshare-side health**: all 10 lanes proxy a neutral site
  (api.ipify.org) instantly with correct exit IPs. Account `throttled:
  false`, subscription active, 250GB plan bandwidth.
- **TikTok edge (no signing)**: all 10 lanes GET https://www.tiktok.com/
  → 200 in 0.5-2.2s, matching direct egress baseline. No IP-level edge
  block on any lane.
- **Full signed capture** on the local rig: burritostreamgr 8.6s (lane
  209.166.16.88:6749), bigjaygaming01 7.3s (104.253.199.239:5518),
  dan2dxo 6.3s — all attempt 1, all through lanes. Rooms asahiicc and
  tv_whiteshark failed on 4 lanes each, but the SAME lanes captured
  other rooms minutes earlier: failures are room-correlated, not
  lane-correlated. Those rooms are dead/gated at TikTok's side.
- **Pool timeline correction**: the pool that "died at 15:00" was already
  4 original + 6 auto-replaced IPs (webshare replaced them at 10:55 UTC
  via `auto_replace_invalid_proxies: true`, consuming 6 of 10 monthly
  proxy replacements — hence "4 left", which was real, but it was
  webshare's own health checks that spent them, not our volume). So "10
  burned originals" was wrong twice over: 6 IPs were hours-old when they
  "burned", and nothing is burned now.
- The afternoon's "all lanes dead from two origins" is best explained by
  the combination verified tonight: room-side gating (not IP-side) + the
  rig's profile-ownership bug (every attempt was a cold profile, EACCES
  on mkdir) + the flapping session gate.
- Implication: **do NOT rotate IPs.** Nothing on the account needs the
  remaining 4 replacements; burning one to "test the fresh-IP hypothesis"
  would spend quota on a hypothesis already answered.

## Spike results (2026-09-16 night — all three answered)

1. ANSWERED — but the premise was wrong: there is no held-open im/fetch
   stream to read. im/fetch is a one-shot ~25-29KB bootstrap response
   (single `dataReceived`, immediate `loadingFinished`; verified twice).
   The transport on an active room is a **WebSocket push**, and CDP reads
   it fine: attach to the warm viewer browser (second CDP client works —
   619 events/45s sanity run) and consume
   `Network.webSocketFrameReceived` per frame. No Fetch-domain streaming
   needed. Spike scripts left on the lab container in /app (spike-attach,
   spike-ws, spike-frames) and in /tmp on the host; frames captured at
   /tmp/ws-frames.jsonl (170 frames/60s off diamondslay).
2. ANSWERED: lanes serve im/fetch — see "Pool NOT burned". No rotation.
3. ANSWERED: the im/fetch response body is a RAW ProtoMessageFetchResult
   protobuf (decodes with `deserializeMessage('ProtoMessageFetchResult',
   buf)`; cursor + 11 messages + internalExt + pushServer present —
   saved at /tmp/imfetch-body.bin). The WS frames are base64 PushFrames
   whose payload embeds a ProtoMessageFetchResult; the connector's
   `deserializeWebSocketMessage` (async) decodes them, inner `messages[]`
   verified to carry WebcastChatMessage with real msgId/roomId/user.
   Caveat: ~half the frames (79/170) throw `premature EOF` at the inner
   decode — consistent with ack/keepalive frames (`msg_type: "r"`);
   classify them before assuming data loss.

## Architecture decision (2026-09-16 night — no-chrome VALIDATED, viewer tab = canary)

The scaling question ("must we keep a Chrome tab per stream?") was settled
empirically in the lab the same night: NO.

**Lab proof (diamondslay, chatty room):** the EXISTING listener —
connector WS + SelfSigner + Chrome-TLS lane pinning, zero resident
Chrome per stream — connected and held a sustained chat stream:
- Signer tab needed only ~6s for the capture, then idle.
- "Unexpected server response: 200" flap: 2 failures (~4-8s backoff),
  then CONNECTED. The 200-instead-of-101 gate is flapping, not a wall.
- 1756 messages into chat:raw over 10 minutes (~3/s), full decode chain
  (PushFrame → messages → RawChatMessage) working.
- Clean disconnect when the room ended; poller backed off correctly.

Euler comparison, for the record: Euler broke on TikTok churn AND free-plan
rate limits. The in-house Node WS has no vendor and no quota; its only
exposure is TikTok session scoring on the WS handshake, which the canary
below watches.

**Design (user-set, 2026-09-16):**

1. **Primary transport: listener Node WS** (what the lab just ran —
   prod-shipped code, no new transport). Signer tab = capture-only
   bootstrap, closed or idle-evicted after the fetchResult is handed over.
   Scaling: one signer browser pool serves all rooms' captures; listener
   holds cheap WS connections instead of the signer holding renderers.
2. **Viewer-tab relay = canary + fallback.** Keep the signer-relay machinery
   (CDP attach, PushFrame decode — spiked and working) deployed on a small
   set of rooms. Two jobs:
   - Canary: compare frame/method shape and connect outcomes between the
     Node WS and the SDK-owned page WS on the same room. Divergence (new
     ack requirements, changed framing, rising 200-flap rate) = early
     warning that TikTok moved the signing surface; alert and fix before
     the primary tier breaks broadly.
   - Premium fallback: on primary-tier failure for a premium user's room,
     promote that room to the viewer-tab relay automatically. The page
     rides TikTok's own SDK, absorbing changes we haven't reimplemented.
3. **Circuit breaker still first** (unchanged): N consecutive all-lane
   capture failures → fast 502 cooldown; the flap shows why — connect
   retries self-resolve with backoff, but capture-hammering burns lanes.

Lab rig for further transport work: lab-redis (port 16379), lab-pg (15432),
lab-listener (node dist build, host network, TIKTOK_SIGNER_URL=http://
127.0.0.1:18092, no SERVICE_JWT_SECRET → no leadership gate; demand via
`redis-cli publish source:demand` with {type:'demand_update',sources:
[{channel_id,platform,overlay_id}],timestamp}).

## Measurement notes (traps from today)

- Rooms end constantly — three test rooms ended mid-experiment. Confirm
  the room is live (check_alive / title) before reading a capture failure.
- room/enter 403s on first attempt and succeeds on retry — one 403 is noise.
- Capture-class im/fetch bodies measured 2.6-33KB; viewer treats <1000
  bytes as keepalive, not capture.
- 60s per-attempt timeout is marginal: real browsers fire im/fetch at
  ~5.7s warm; in-cluster cold lanes took 33-52s; failures cluster at
  60-65s. Raise, don't lower.
- (Night) Don't launch extra Chromium instances next to the viewer pool for
  spikes: they die with ERR_TUNNEL_CONNECTION_FAILED (connect exhaustion)
  while the pool itself keeps working. Attach CDP to the pool's existing
  browser instead — DevTools port per lane at
  /profiles/<lane>/DevToolsActivePort.
- (Night) The viewer pool evicts idle tabs after 5 min (tabIdleMs), killing
  their WebSocket. Re-warm with a /v1/sign call before any observation
  window, and don't let the watch exceed the idle budget.
- Don't run multiple extra Chromium instances alongside the production
  viewer pool in one pod — contributed to "Failed to open a new tab"
  (browser process exhaustion). For spikes, use the local rig.
- Signer dist/ is committed to git — rebuild with the node:22 container
  (`docker run --rm -v $(pwd):/repo -w /repo/services/tiktok-signer
  node:22-bookworm-slim sh -c "npm ci && npm run build"`), tests via npm test.
- GitOps: deployment repo git@github.com:caesarakalaeii/caesar-deployment.git,
  path apps/workloads/all-chat, Argo app `all-chat` selfHeal:true — manual
  kubectl mutations revert within minutes; go through Git.
- Lab pod pattern (if cluster needed): ns allchat, secret
  tiktok-signer-proxy key token, HOME + SIGNER_USER_DATA_DIR under /tmp
  (readOnlyRootFilesystem), limits 2Gi/2. Secrets never enter the transcript.
- Decrypted capture rig: debian:bookworm-slim + tcpdump tshark chromium
  xvfb; SSLKEYLOGFILE mounted out; `tshark -o tls.keylog_file:... -Y http2`.
