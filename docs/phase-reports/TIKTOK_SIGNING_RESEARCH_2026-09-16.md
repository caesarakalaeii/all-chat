# TikTok Signing Research — 2026-09-16

Research handoff: everything measured during the 2026-09-16 signing-lab day,
so a fresh session can start implementing without re-deriving the findings.
Status: fixes shipped (all-chat #895, #896, caesar-deployment #104); the
items under "Open work" are unimplemented.

## Timeline of the day

1. Fleet-wide `viewer_capture_failed` diagnosed via in-cluster signing lab
   (throwaway pods in ns `allchat`, context `default`, k3s home cluster).
   Root cause: the ANGLE-on-Vulkan/lavapipe viewer args (commit b87ecd26)
   wedge the page entirely. Fixed by all-chat #895 (args removed,
   mesa-vulkan-drivers dropped from the image).
2. Push-WebSocket TLS hardening shipped in the same PR: the listener's WS
   egress now mirrors Chrome's ClientHello via tls-impersonate (Node 26
   runtime). Verified against tls.peet.ws through a CONNECT passthrough:
   JA4 cipher/extension segments match Chrome exactly, one deliberate ALPN
   deviation (h1 vs h2, required by the ws upgrade).
3. All ten webshare static-residential exit IPs burned during the day
   (thousands of failed captures). Confirmed from two origins: cluster pod
   and a locally-run signer both fail through every lane, while direct
   egress captures from every origin tried.
4. Signer direct egress deployed temporarily (caesar-deployment #104, the
   `SIGNER_WEBSHARE_TOKEN` block commented out — restore after rotating IPs).
   Production captures confirmed working: `burritostreamgr` 61s attempt 2,
   `bigjaygaming01` 52s attempt 1.
5. Signer crash found and fixed (all-chat #896): stealth-plugin CDP close
   race — unhandled TargetCloseError/ProtocolError during lane profile
   rotation killed the service twice in 12 minutes. Entry-point guard now
   swallows exactly those two rejection names; everything else stays fatal.
6. Decrypted packet capture of a real Chromium session on a live room
   (SSLKEYLOGFILE + tcpdump, tshark analysis) — see "Protocol findings".

## Protocol findings (decrypted capture, Chromium 152, live room)

### The signing surface changed

Real-browser request choreography before im/fetch (times relative to page
load; live room `burritostreamgr`):

```
t=3.0s   POST /webcast/room/enter/     (X-Dynosaur param)
t=3.5s   POST /v1/user/webid           (web identity registration)
t=5.4s   POST /v1/user/webid           (retry)
t=5.7s   GET  /webcast/im/fetch/       (X-Dynosaur, ws_direct=1)
t=~30s   POST /webcast/epiphron/feature/upload/  (recurring telemetry)
t=~5.2s  GET  /webcast/room/check_alive/         (recurring)
```

Key deltas vs what our signer replicates:

- **`X-Dynosaur`** is now on `room/enter` and `im/fetch` query strings.
  The signer vendors X-Bogus/X-Gnarly encoders (see `vendor/PROVENANCE.md`)
  but has no Dynosaur implementation. A community reference exists:
  Evil0ctal/Douyin_TikTok_Download_API implements X-Dynosaur in Python.
  [INFERENCE] The capture-only sessions may be judged partly on missing
  Dynosaur, which would make fresh IPs burn faster than the originals did.
- **New im/fetch params**: `ws_direct=1`, `sup_ws_ds_opt=1`, `did_rule=3`.
- **`/v1/user/webid`** POST fires immediately before im/fetch. Our signer
  rides the viewer page's own identity; a capture-only session may need to
  replicate this registration call.
- **`/webcast/epiphron/feature/upload/`** POSTs every ~30s — the fingerprint
  telemetry beacon. Real sessions beacon; capture-only sessions do not.
  [INFERENCE] Session-quality scoring may expect the beacon.

### The web client no longer uses WebSocket for chat

In a 100s capture of a live room with chat flowing:

- Exactly **one** im/fetch request. Its HTTP/2 response stream is held open
  and receives DATA in bursts (observed at t=6s, 38s, 89s). Chat arrives as
  server push over that h2 stream.
- **Zero WebSocket handshakes** across the whole capture (all connections
  port 443; no WS upgrade anywhere).
- `ws_direct=1` on im/fetch is the marker for this mode.

The listener's entire WS leg — tiktok-live-connector's push WS, lane-pinned
WS egress, the Chrome-TLS agent on that leg — serves a protocol the web
client has moved off. It still works today; it is the likeliest next thing
TikTok breaks. The alternative transport is what the signer already
captures: the h2 long-poll im/fetch stream itself.

### WebGL is NOT required by the gate

- Your LibreWolf blocks WebGL entirely and sees live chat fine.
- Lab A/B: a successful 33KB capture had `NO-CONTEXT` for WebGL.
- The gate cares about a coherent real session (cookies, identity, IP),
  not the renderer. `--disable-gpu`/SwiftShader fails for other reasons
  (and on Chromium 152 without Vulkan ICDs the GPU process hangs the page
  entirely — that was the Vulkan-args regression, not a WebGL gate).

## What is broken and what is not

| Component | State |
|---|---|
| Signer viewer capture (plain args, real display) | Working, verified direct egress |
| Signer X-Bogus/X-Gnarly signing session | Working (unchanged) |
| Webshare static-residential pool (10 IPs) | Burned, needs rotation |
| Listener push WS + Chrome-TLS egress | Working (legacy protocol, still served) |
| X-Dynosaur signing | Not implemented |
| Lane burn protection (circuit breaker) | Not implemented |

## Open work, in the order it should land

### 1. Lane circuit-breaker + direct fallback (before rotating IPs)

The failure loop is what burned 10 IPs in one day: the listener retries
every live room with backoff, each retry is a full page load + failed
capture on a lane. With ~20 live channels cycling, that is hundreds of
lane hits per hour. Before new IPs go in:

- Global lane circuit-breaker in `ViewerPool` (services/tiktok-signer/
  src/signing/viewer.ts): after N consecutive failures across all lanes,
  fail fast (502) for a cooldown window instead of hammering. The listener
  already backs off on sign failures, so a fast 502 stops the burn.
- "All lanes exhausted → direct fallback" so chat stays up when the pool
  dies again.
- Watch `epiphron/feature/upload` presence in the viewer tabs: if TikTok
  scores sessions on the telemetry beacon, keeping the viewer tab alive
  after capture (already done — tabs stay open) is what preserves the
  lane, and the breaker must not close tabs that a room depends on.

### 2. Restore webshare pool (operator dashboard action)

Rotate the static-residential list in the webshare dashboard (4 rotations
left this month). The hourly token refresh propagates automatically.
Then revert caesar-deployment #104: uncomment the `SIGNER_WEBSHARE_TOKEN`
block in apps/workloads/all-chat/tiktok-signer-deployment.yaml.

### 3. X-Dynosaur investigation

Diff the im/fetch request the signer's viewer produces (it rides a real
page, so the SDK generates Dynosaur for it) against what the connector
sends on the WS leg. The WS URL itself may now need a Dynosaur signature.
Reference implementations: Evil0ctal/Douyin_TikTok_Download_API (Python,
pure reimplementation, broken-by-SDK-update risk), armxe/tiktok-api
(pipeline breakdown).

### 4. Decision: migrate chat transport to the h2 long-poll

If the WS leg breaks (or before), the capture stream IS the transport:
the signer's viewer tab holds the im/fetch h2 stream open and chat arrives
in bursts. Architecture: viewer tab per room stays open (already the
case), signer streams decoded ProtoMessageFetchResult frames to the
listener over its HTTP API (or Redis Streams directly). This removes:
the WS handshake entirely, WS lane pinning, and the separate signed-URL
round trip per connection. Cost: signer becomes per-room stateful and
holds N open pages; CPU per tab ~50-150MiB (measured, deployment repo
comment).

## Measurement notes for whoever continues

- Rooms go live and end constantly; three test rooms ended mid-experiment
  on 2026-09-16. Always confirm `room/enter` status_code 0 / check_alive
  shows the room live before concluding a capture approach failed.
- `room/enter` intermittently 403s on first attempt and succeeds on retry —
  one 403 is noise, not a verdict.
- Capture-class im/fetch bodies in-cluster measured 2.6-33KB; the viewer
  treats <1000 bytes as a keepalive, not a capture (viewer.ts, buf.length
  < 1000).
- The 60s per-attempt capture timeout is marginal: real-browser im/fetch
  fires at ~5.7s after page load on a warm profile but cold lanes with
  full page bootstrap took 33-52s in-cluster, and failures cluster at the
  60-65s boundary. The lab evidence says raise, don't lower.
- Deployment repo: git@github.com:caesarakalaeii/caesar-deployment.git,
  path apps/workloads/all-chat. Argo app `all-chat`, selfHeal true —
  manual kubectl changes revert within a minute; go through Git.
- Lab pod pattern that works: throwaway Pod in ns allchat, signer image,
  SIGNER_WEBSHARE_TOKEN from secret `tiktok-signer-proxy` (key `token`),
  HOME and SIGNER_USER_DATA_DIR under /tmp (readOnlyRootFilesystem),
  requests 512Mi/500m limits 2Gi/2. Secrets never enter the transcript.
- Decrypted capture rig: docker debian:bookworm-slim + tcpdump + tshark +
  chromium + xvfb, SSLKEYLOGFILE mounted out, `tshark -o
  tls.keylog_file:... -Y http2`. Captures are in git-ignored /tmp only.
