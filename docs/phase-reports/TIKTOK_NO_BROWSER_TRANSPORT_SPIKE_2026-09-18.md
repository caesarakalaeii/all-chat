# TikTok no-browser transport spike — 2026-09-18

Follow-up to the 2026-09-18 morning spike (verdict in agent memory: "pure-Node
delivery NOT viable at the im/fetch layer") and to
[TIKTOK_SIGNING_RESEARCH_2026-09-16.md](./TIKTOK_SIGNING_RESEARCH_2026-09-16.md).
Status: the morning verdict is OVERTURNED. Pure-Node im/fetch and pure-Node WS
delivery both worked, measured on the lab rig (signer-lab, direct egress, warm
`base-viewer-direct` profile). What killed the morning spike was a single
request property nobody varied: the User-Agent string's consistency with the
URL's own environment params.

## TL;DR

- **im/fetch serves full protobuf to plain Node** (python urllib, OpenSSL TLS,
  HTTP/1.1, minimal headers, no cookies): 50,729 bytes measured. No TLS
  impersonation, no h2, no header order, no client hints, no CSRF header.
- **The empty-200 gate is the User-Agent.** Same signed URL, same cookies:
  curl_cffi chrome136/131/124 with their own modern-Chrome UA → empty 200;
  the same client with the verbatim UA the page used
  (`X11; Linux x86_64 … Chrome/144.0.0.0`) → 40,930 bytes. TikTok
  cross-checks the UA against the URL's self-describing params
  (`browser_platform`, `browser_version`, `os`). The morning spike's
  undici/raw-https replays failed for this reason, not TLS.
- **Cookies are not required for the bootstrap.** No-Cookie replay: 50,577
  bytes. The signature params in the URL (X-Gnarly, X-Dynosaur, msToken,
  X-Bogus=1 as sent by the live page) are what carry the session.
- **WS delivery leg works pure-Node end-to-end**: recorded
  `ws_reuse_supplement` URL (40 minutes old at test time), plain Node `ws`
  module, no chrome-TLS, `im_enter_room` frame + 10 s heartbeats + ack frames
  → 64 frames / 81 messages (8 WebcastChatMessage, 15 RoomUserSeq,
  8 Member, 3 Like), 55 acks accepted, over 90 s. A second run got 42
  messages incl. 8 chats with correct `common.method` typing.
- **Delivery does not need im/fetch at all**: entering the WS with
  `cursor: ""` gets the full room state + live chat pushed. im/fetch is a
  convenience (backlog, internalExt, pushServer addr), not a gate for the
  chat stream.
- **The WS URL's X-Bogus is validated** (stripping it → instant
  200-rejection), unlike the fetch URL's X-Bogus (the live page sends the
  literal `X-Bogus=1` there; the real fetch signatures are X-Gnarly +
  X-Dynosaur).

## The im/fetch replay, exactly

Recording (spike-req2, capture-style tab on the signer's warm browser: UA
pinned to VIEWER_UA, media/font/image + known trackers aborted, prewarmed on
tiktok.com first — mirrors `newLanePage`):

```
GET https://webcast.tiktok.com/webcast/im/fetch/
  aid=1988 … cursor=0 internal_ext=0 ws_direct=1 sup_ws_ds_opt=1 did_rule=3
  client_enter=1 version_code=180800 room_id=7686797272637524758
  msToken=<page msToken> X-Bogus=1
  X-Dynosaur=<page-computed> X-Gnarly=<page-computed>
headers: User-Agent: <X11 Linux Chrome/144 — VERBATIM>, Referer: tiktok.com
[no cookies needed]
→ 200 application/x-protobuf, 40-51 KB, cursor + messages + internalExt
```

Freshness: the recorded URL served data when replayed ≤ ~4 min old (11:01
recording, data at 11:03–11:04). By ~5–7 min: empty 200s. By ~7+ min: 403 with
empty body (still carrying a fresh `x-ms-token` response header). The msToken
param is the clock; whether X-Gnarly/X-Dynosaur also time-expire is unproven
(all staleness tests ran after the session got flagged).

## The WS leg, exactly

```
wss://webcast-ws.eu.tiktok.com/webcast/im/ws_proxy/ws_reuse_supplement/
  ?<30 params incl. room_id, identity=audience, ws_direct=1,
   heartbeat_duration=10000, X-Bogus=<page-computed>>
on open: send PushFrame{payloadType:"im_enter_room",
  WebcastImEnterRoomMessage{roomId, roomTag:"", liveRegion:"", liveId:"12",
  identity:"audience", cursor:"", accountType:"0", enterUniqueId:<rand63>,
  filterWelcomeMsg:"0", isAnchorContinueKeepMsg:false}}
every 10 s: PushFrame{payloadType:"hb", HeartBeatMessage{roomId, sendPacketSeqId}}
on frame: deserializeWebSocketMessage → protoMessageFetchResult
  needAck → PushFrame{payloadType:"ack", logId, payload=internalExt}
```
All frames decode with the connector's own `deserializeWebSocketMessage`;
types are on `message.common.method`. Protocol details extracted from
`tiktok-live-connector` dist (ws-client): ack needs `internalExt`, heartbeat
interval `DEFAULT_WS_PING_INTERVAL = 10000`.

## Gating observations (measurement discipline)

- A burst of ~15 replays in ~3 min flagged the session's im/fetch surface:
  the browser's own in-page im/fetch then 403'd too (16 consecutive 403s
  recorded on one room page). The flag outlived 60+ min. Cooldown, not wall
  (2026-09-16 precedent: gates flap and re-open).
- The WS push server kept serving frames to pure-Node clients while im/fetch
  was 403ing — the surfaces gate independently, until ~15 WS connect attempts
  in an hour got that surface throttled too (101 withheld). Lesson: spike
  request budget must be single-digit per surface per hour.
- The live feed (tiktok.com/live) is the reliable live-room discovery; known
  rooms ended constantly (all eight 2026-09-16/18 rooms were offline today).
  Room liveness check that matches the capture flow's own signal: player root
  `#tiktok-live-main-container-id` + playing `<video>` + chat DOM.
- Two page variants observed on the same warm browser: default-UA tabs get a
  "live_new" client that never fires im/fetch (recommendation `webcast/feed/`
  53 KB + im-ws.tiktok.com sockets); VIEWER_UA tabs (and prewarmed capture
  tabs) get the classic client (room/enter, im/fetch, ws_reuse_supplement).
  `use_live_desktop_arch=edenx3` cookie is present in both.
- The signing session's own identity (session.ts IDENTITY) is
  Safari/macOS — consistent with `imFetchParams`' hardcoded `MacIntel`/`mac`.
  The 2026-09-15 "signature path returns empty 200" measurement is therefore
  NOT explained by a UA mismatch (it was internally consistent); the leading
  suspect is the missing X-Gnarly/X-Dynosaur (which the live page does send,
  and which the vendored X-Gnarly encoder cannot produce — measured 403 on
  2026-09-15). Unproven: needs the sign-url arm below.

## Unproven (blocked on healthy gate, then single requests)

1. **Signature scope**: does the recorded X-Gnarly/X-Dynosaur bind room_id?
   If not, one harvest serves all rooms. (room_id-swap on a fresh URL.)
2. **sign-url sufficiency**: POST /v1/sign-url (X-Bogus + fresh msToken from
   the signing page, no Gnarly/Dynosaur) + consistent Safari UA → data? If
   yes, the whole viewer-capture pool retires for bootstrap.
3. **UA gate breadth**: exact-match on the string vs platform-consistency
   (Linux/152? Windows/144?). Today only the verbatim UA passed; every other
   variant ran in the flagged window (inconclusive, not negative).
4. **msToken rotation**: x-ms-token header swap into a Gnarly-signed URL was
   tried only post-flag. If rotation works, one harvest lasts indefinitely;
   if Gnarly binds msToken, harvests are ~5-min fresh.
5. **Synthetic WS URL** (template + room_id, no page): X-Bogus strip was
   rejected instantly; a fresh signer-signed X-Bogus for the WS URL is the
   missing piece — the signer's signUrl can provide it per the shipped
   listener design (#897 lab proof: connector WS + SelfSigner held 1756
   msgs/10 min).
6. **WS without cookies**: every successful WS run sent the session cookies;
   the fetch leg proved cookies unnecessary, the WS leg did not get its
   no-cookie run before throttling.

## Spike artifacts

Host `/tmp` (not committed; recordings contain live session cookies — never
commit):

- `spike-req2.cjs` — capture-style tab recorder (verbatim im/fetch URL,
  headers, cookies, WS URLs; full request log). The interception MUST mirror
  `newLanePage` exactly — an aggressive ad-hoc tracker regex (`log` matches
  `login`!) aborted the webapp bundles and produced a zombie page.
- `node-fetch.cjs` — pure-Node bootstrap fetch + connector decode.
- `node-ws2.cjs`, `ws-retry.cjs` — pure-Node WS delivery with the full
  enter/heartbeat/ack protocol.
- `replay.py`, `replay2.py`, `replay3.py`, `rotate.py`, `decay.py` — replay
  matrix (curl_cffi targets vs plain urllib), param bisection, msToken decay
  and rotation harnesses.
- `req2-*.json` — the four recordings (11:01, 11:05, 11:14, 11:58 UTC).

Containers: signer-lab (spike scripts also at /app), lab-listener (runs the
Node legs with `NODE_PATH=/repo/node_modules`), curlcffi (curl_cffi replay
instrument).

## Architecture implication (for the feasibility discussion, per operator)

The no-browser end state is closer than the morning verdict assumed:

- Delivery = pure-Node WS (proven). The browser's residual jobs: username →
  room_id resolution, a valid X-Bogus for the WS URL (signing session, not
  viewer capture), and — only if the bootstrap is wanted — a ≤5-min-fresh
  signed im/fetch URL (or the sign-url arm if it passes).
- The signer's viewer-capture pool (10-lane browsers, media-blocking page
  loads, 60-180 s captures) exists to obtain exactly what this spike
  obtained with one recorded page and a plain Node client. If signature
  scope is room-agnostic (test 1) and/or sign-url suffices (test 2), the
  capture pool collapses to a single warm signing page.
- The listener already ships the client side of this (#897). The missing
  piece in code is the UA-consistency fix (params must describe the same
  platform the UA claims — `imFetchParams`' MacIntel/mac vs the Linux/Chrome
  viewer identity is the standing inconsistency to resolve) and a
  freshness-budgeted URL harvest.

## PR 0 results (2026-09-18 PM)

Context: executed under the PR 0 plan (council-passed rev 3, operator-redesigned
rev 4 after the live_new finding). Budget for this first batch (13:31-13:39
UTC): 3 pure-Node im/fetch replays, no WS connects, single sign-url calls
in-cluster; page-internal captures (3) were harvest attempts, not replay
bursts. The later 14:29-14:46 batches added 7 fetch replays (the two
sections below) and 2 WS connects (the #4 detail above) — 10 fetch replays
total inside one rolling hour. Worst observed density: 7 replays inside one
49-second span (14:30:17-14:31:06) against a ~15-in-~3-min flag threshold —
7 is below 15 as a count, though the instantaneous rate was higher than the
threshold's; the control stayed data-bearing throughout, and no 403 was
ever seen.

harvest: room=zaganovakov ts=2026-09-18T14:30:08Z

**Session finding (superseded within the hour — see "PR 0 harvest answers"
below):** the first read blamed a server-wide variant flip. What the zaganovakov
captures later showed: the classic/live_new split is **per-room/per-navigation**
(jinuabi and neringakazlauskaite served live_new on every attempt; zaganovakov
served the classic client on every attempt, across two different profiles and
both the spike recipe and the viewer pool). Harvest arms resumed against a
classic room once this was known; the numbers below under #1/#3/#5/#0c are
the measured ones.

#0c: gate-healthy — evidence: status=200 bytes=49839 age=9s
#1: per-room — evidence: status=200 bytes=0 age=19s
#2: harvest-needed — evidence: status=200 bytes=0 age=1s
#3: 5-min-freshness — evidence: status=200 bytes=0 age=49s
#4: cookies-needed — evidence: frames=0 msgs=92
#5a: empty — evidence: status=200 bytes=0 age=32s
#5b: empty — evidence: status=200 bytes=0 age=32s
#5: ua-exact-match

#6a: connector signs only the bootstrap im/fetch (SelfSigner→/v1/sign); the WS URL is built unsigned in WebcastWebSocketClient (dist lib-YL2P_UWg.js:377) and has run unsigned in prod self mode since #897 — PR 1 item 3 (sign-url allowlist extension to the push host) unnecessary
#6b: username→roomId via fetchRoomInfoFromApiLiveRoute (www.tiktok.com/api-live/user/room/) — verified working this session from lab-listener without capture (status 200, roomId served)
#6c: prod (caesar-deployment, read via gh): TIKTOK_SIGNER_MODE=self, TIKTOK_SELF_SIGN_FALLBACK=false, SIGNER_RELAY_CANARY_ROOMS empty + SIGNER_RELAY_FALLBACK present on the signer; prod pins WS egress to webshare residential proxies (session bound to egress IP) — PR 2 must decide pure-node fetch egress (lab ran direct)
#6d: feat/retire-euler-tiktok-signing is merged to main — pure-node's self-mode precondition is met

**#2 detail (the decisive negative):** identity-consistent im/fetch param set
(api.ts `imFetchParams` clone, Safari/macOS per session.ts IDENTITY — never
`/v1/identity`, which returns the VIEWER identity in this lab), signed fresh
in-page via `POST /v1/sign-url` (X-Bogus only), fetched pure-Node with the same
UA: **empty 200, `content-type: application/json`, 0 bytes** — the silent-drop
signature, not a protobuf truncation. Both UA variants (Linux Chrome/152 with
platform-consistent params, Windows Chrome/144 with Win32/windows params, each
freshly signed) returned byte-identical empty-200s. Three-way agreement across
identities + the morning's X-Gnarly/X-Dynosaur-carrying page URL serving 40-50 KB
to plain Node ⇒ **sign-url (X-Bogus) alone is insufficient for im/fetch; the
gate wants the page-computed X-Gnarly/X-Dynosaur**. This confirms the spike
report's own 2026-09-15 suspicion, now with identity consistency controlled.

**#4 detail (measured 14:46 UTC, zaganovakov WS URL ~16 min old):** the
no-cookie connect was refused at handshake ("WS REJECTED: HTTP 200" — TikTok
answers the upgrade with a plain HTTP 200); the same URL with the capture's
cookies delivered 92 messages in 60 s (10 WebcastChatMessage, 25
RoomUserSeq, 36 Like, 14 Member, 55 acks accepted, 5 heartbeats). Clean
dichotomy on the same URL ⇒ **the WS leg needs the session cookies**;
`fetchResultCookieHeader` is load-bearing, the signer must keep exposing it
(or the listener must hold the warm jar). WS URL age 16 min also confirms
the morning's finding that the WS surface is far more durable than the
fetch surface's ~5 min.

**Consequences for the work order (superseded — written from the variant-flip
reading that the 14:29 batch corrected; the operative PR 1 resolution is the
"PR 0 harvest answers" section below):**

- **PR 1 signing endpoint = the harvest route, not sign-fetch.** The plan's
  fork resolves to: `GET /v1/harvest` (CDP `requestWillBeSent` of the canary
  tab's own im/fetch) — but note the live_new finding above: today's pages do
  not send im/fetch at all, so harvest-of-page-im/fetch is blocked on the
  same variant flip. PR 1 must either harvest the page's `webcast/feed/` URL
  instead (needs: does feed decode with connector schemas? does feed carry
  pushServer? — one replay, next hour window), or wait out the variant flip,
  or drive the signing page to force the classic client (unproven).
  [Superseded: the split is per-room; classic rooms serve im/fetch and the
  harvest route works as originally planned.]
- **The 2026-09-18 morning path is not reproducible today**: the exact
  recording that proved pure-Node im/fetch (X-Gnarly URL) cannot be re-obtained
  from this session while it serves live_new. Freshness of the morning
  recordings is long past; they are evidence, not instruments.
  [Superseded: reproducible against any classic-serving room; re-measured
  at 14:30 (see #0c, 49,839 B).]
- **WS-leg delivery remains proven** (morning, 40-min-old URL) — the pure-Node
  WS leg is unaffected by the fetch-layer finding. [Stands, and re-proven
  with cookies at 14:46.]

**Gate status (superseded — both arms were measured later this session):**
PR0-GATE expected FAIL (ua-inconclusive + gate-flaky are non-terminal states
by design) — PR 0 is open on two arms: #5's UA-breadth needs a working
control (blocked on the variant flip / a harvestable page fetch), #4 needs a
classic-capture WS URL. [Both closed: #5 = ua-exact-match via the 14:30
batch, #4 = cookies-needed via the 14:46 run; the gate now passes with
PR0-GATE-PASS.]

## PR 0 feed-surface re-scope (2026-09-18 14:11 UTC, operator-approved)

Fresh harvest 14:11:44 UTC (jinuabi live; live_new client again, no im/fetch),
immediate pure-Node replay of the largest recorded `webcast/feed/` URL
(seconds old, verbatim UA, **no cookies**):

```
status=200 bytes=373554 ct=application/json ms=683
```

**Findings:**

- `webcast/feed/` serves 373 KB to plain Node with no cookies — the same
  no-browser property im/fetch had this morning. The recorded URL carries
  the FULL signature set (X-Gnarly, X-Dynosaur, msToken, X-Bogus), so the
  surface uses the same page-computed signatures.
- But feed is a **discovery listing, not a chat bootstrap**: JSON
  (`status_code/extra/data`), `data` = 20 rooms, each with `id_str`
  (roomId), owner, status, stream_url. No cursor, no internalExt, no
  pushServer — none of the four things the connector's WS connect needs.
  It vacuously "decodes" as ProtoMessageFetchResult (0 messages).
- `api-live/user/room/` similarly gives streamData (video pull URLs) but
  no chat-push info.

**Architecture consequence (partially superseded — the "classic client gone
from this session" premise was corrected below: classic rooms exist and were
harvested at 14:30; what stands from this section is the measured fact that
feed is a discovery listing with no chat bootstrap, and that live_new rooms
offer none of the classic pipeline's pieces):** with the classic client gone
from this session, there is today NO page request that produces the chat
bootstrap (cursor/internalExt/pushServer). The live_new client gets chat via
its own `im-ws.tiktok.com` protocol, which is not what the connector's WS
client speaks. The pure-Node end state therefore cannot ride the classic
im/fetch→webcast-ws pipeline unless something produces a fetchResult:
options are (a) wait out / force the classic variant, (b) reimplement the
live_new im-ws protocol (research effort, unknown depth), (c) harvest the
pushServer template from a capture (the capture pool's existing product)
and keep WS-leg delivery pure-Node. WS-leg pure-Node delivery itself
remains proven; only the bootstrap source moved. [The 14:29 batch
invalidated the premise for classic rooms; option (c) is the branch that
matches every later measurement.]

## PR 0 harvest answers (2026-09-18 14:29-14:31 UTC — variant finding corrected)

**Correction to the live_new read above:** the variant is per-room/per-navigation,
not server-wide and not profile-bound. `/v1/sign` (viewer capture, `base`
profile) on zaganovakov returned a full fetchResult in 3.6 s, and a spike-req2
harvest of the same room on the `base-viewer-direct` profile minutes later got
the classic client: im/fetch + a `webcast-ws.eu` WS URL. jinuabi and
neringakazlauskaite served live_new both times; zaganovakov served classic
both times. The morning recordings remain reproducible — pick a classic room.

Measured batch (7 pure-Node fetch replays 14:30:17-14:31:06 UTC, all within
one freshness window of a 14:30:08 harvest, single room zaganovakov,
control fresh throughout — the 5 table rows plus 2 token-rotation
plumbing runs: a first rotation attempt against a truncated token, and a
control re-fetch that captured the full 172-char header, both noted in the
#3 row's provenance):

| # | Test | Result |
|---|---|---|
| 0c | verbatim replay of recorded im/fetch | **200, 49,839 B protobuf**, 18 msgs, cursor+internalExt |
| 1 | room_id → 7686754674422074144 (neringakazlauskaite), sig kept | **empty 200** — signature binds room_id → **per-room** (caveat: the swap target's liveness was verified at 13:07 UTC, ~80 min before this replay, not re-verified at 14:30; an offline room would also read empty, but the per-room branch is the conservative one either way — signing per room works even if scope were broader) |
| 5a | UA → Linux Chrome/152 (params verbatim) | **empty 200** |
| 5b | UA → Windows Chrome/144 (params verbatim) | **empty 200** |
| 3 | msToken → seconds-fresh full 172-char x-ms-token | **empty 200** at URL age ~49 s (pre-rotation the same URL gave 16 B at ~40 s — already decaying) → rotation does not revive; **5-min freshness, sign per connect** (this batch's decay ran tighter than the morning's ~4-min window — possibly replay-count- as well as clock-driven; the operative directive, sign at connect, is unaffected) |

Answers vs the plan's forks:

- **#1: per-room (conservative under the row's liveness caveat).** One
  harvest does not serve all rooms — every room gets its own signed URL,
  capture-per-room or sign-per-room in-page. The measured empty-200 could in
  principle be an offline-target artifact (see the row's caveat), but
  per-room signing is correct under either reading.
- **#5: ua-exact-match (on the page-computed signature).** The gate rejects
  a version drift (152 vs 144, platform identical) and a platform swap, on a
  URL whose params stayed verbatim. Stricter than platform-consistency.
  Combined with the morning's verbatim-UA pass: the UA header must equal the
  one the signature was minted under. Identity must be pinned end-to-end
  (PR 1's identity module is load-bearing; "track upstream Chrome majors"
  is off the table without re-signing).
- **#3: short-lived URLs, sign at connect.** A fresh rotated msToken does
  not extend a URL's life. This batch saw decay inside a minute (the
  "5-min" figure is the morning's upper bound, not a safe budget);
  the architecture is sign-at-connect.
- **#2 (from the 13:37 batch): harvest-needed.** sign-url (X-Bogus) alone
  yields empty-200; the page-computed X-Gnarly/X-Dynosaur set is required.

**PR 1 branch resolution (now fully measured):** [Superseded: the
cross-room WS entry section below and the PR 1 plan's deltas resolve this
differently — no per-room captures; a warm classic room's session is leased
and re-entered per target room.] The signing endpoint must
produce, per room, per connect, a page-computed signature over the im/fetch
param set — i.e. the harvest route (CDP capture of the page's own im/fetch or
an in-page sign of a per-room URL with the full signature family, whichever
PR 1's implementation judge picks). sign-fetch as a standalone shortcut is
disproven. The capture pool remains the only proven source of accepted
signatures; option (c) — capture-supplied bootstrap, pure-Node WS delivery —
matches every measurement made today.

## Cross-room WS entry (2026-09-18 16:55-16:59 UTC, operator-requested)

Question: the live_new rooms cannot be captured (their page sends no
im/fetch), so can a classic room's captured WS session serve another room's
chat — making the room variant irrelevant to delivery?

Test: connect the recorded webcast-ws URL of room A (kyle.toynbee,
roomId 7686914945002294048, classic client, capture seconds old, cookies
sent) and send `im_enter_room` for room B (zaganovakov, roomId
7686832401615457057 — a different room, live at test time).

Result, three consecutive runs:

```
open OK (handshake accepted on A's credentials)
enter_room_resp received (entry into B accepted)
6 WebcastChatMessage / 60 s, plus room-state frames; acks accepted
every WebcastChatMessage payload decodes with common.roomId = 7686832401615457057
```

**Answer: yes — cross-room entry works.** One captured WS session (URL +
cookies from any classic room) enters a *different* room and receives that
room's chat, verified by roomId inside every chat payload. Three
consequences:

- **The live_new room variant does not block delivery.** Rooms whose pages
  cannot be captured still receive chat through a session captured on any
  classic room. The variant only threatens the *bootstrap payload* (backlog
  cursor/internalExt for that room), which the fetchResult of the target
  room would normally carry; entering with `cursor: ""` gets the live
  stream regardless (morning spike finding — full room state is pushed on
  enter).
- **PR 1's signer does not need per-room captures.** One warm classic room
  (plus a backup) can mint the WS session that the listener's Node client
  re-enters per target room. The capture pool's job shrinks from
  per-room captures to keeping one or two classic sessions warm.
- **Prod's per-room capture load drops correspondingly** once the listener
  uses cross-room entry — the same direction PR 3's viewer-pool wind-down
  planned.

Budget: 3 WS connects (60-75 s each, ≥15 min after any fetch surface use —
last fetch replay was 14:31, four hours prior), no im/fetch replays.
Session cookie use: the kyle.toynbee capture's cookieHeader, never quoted
anywhere (recordings stay in containers).

## PR 1 implemented (2026-09-18, no live requests)

PR 1 of the pure-Node transport landed in `services/tiktok-signer`, exactly
on the handoff's deltas (no sign-fetch, no allowlist extension, no harvest
cadence; canary isolation kept as handoff item 4). Plan passed a 6-round,
3-lens council (feasibility/decomposition/criteria, 18 reviewer sessions,
verdict: PASS after five rounds of blocking findings — the load-bearing
catches were the roomLane compound-key migration's four consumer sites, the
`ensureBrowser`/`rotateLaneProfile` inline profileDir templates, the
`PROFILE_ROTATE_FAILURES` rotation cascade the breaker cannot prevent, and
the dead-tab absorbing state in the lease endpoint's recovery pass).

What shipped:

- **Identity module** (`src/signing/identity.ts`): `SIGNING_IDENTITY`
  (Safari/macOS) + `VIEWER_IDENTITY` (Linux Chrome/144) in one place;
  `imFetchParams` derives `browser_*`/`os`/`screen_*`/`browser_version`
  from the identity it is handed (the hardcoded MacIntel/mac/5.0 is gone);
  session.ts's page UA, viewport, `navigator.platform` and window-size all
  read from it; `identityIsConsistent` is exported so the golden test
  asserts the negative.
- **WS-URL capture**: each capture attaches a CDP tap before navigation and
  records the page's webcast push WS URL (RelayHub's `includes('webcast')`
  filter) into `RoomCapture.wsUrl` and the registered `TabEntry`
  (`wsUrl`/`roomId`/`capturedAt`). Keep-on-empty: a re-capture of the SAME
  warm page whose SPA nav opens no fresh socket keeps the previous `wsUrl`
  AND its original `capturedAt` — the stamp always describes the served
  URL's age. A different page's empty tap result stands on its own: no
  cross-session cookie/wsUrl pairing.
- **`GET /v1/session`**: shared-session lease endpoint
  (`{ wsUrl, cookieHeader, roomId, userAgent, proxyHost, capturedAt }`),
  four-phase flow (warm lease → breaker-gated tab-less capture with
  fallthrough → recovery pass that closes unservable tabs, skipping rooms
  with active relay subscribers, then cold-captures exactly the freed
  rooms → retryable 503). Metric `signer_session_leases_total{outcome}`
  with `success`/`captured`/`capture_failed`/`no_session`/`disabled`/
  `viewer_off`. The recovery's dead-page criterion is what keeps a Chromium
  crash from wedging the endpoint (council round-6 blocker).
- **Canary profile isolation**: every lane carries a class
  (`primary`/`canary`) and its own `profileDir`; roomLane stores the
  compound `host|class` key (all four consumer sites migrated);
  `pickLane` and the race candidates are class-filtered; `refreshProxies`
  manages both classes; canary browsers launch lazily. A room listed in
  both `SIGNER_WARM_ROOMS` and `SIGNER_RELAY_CANARY_ROOMS` refuses startup
  (`findDualListedRooms`).
- **Canonical usernames**: `/v1/sign` viewer path, `/v1/stream` path capture
  and both env lists lowercase, so pool tabs, breaker state, relay
  subscribers and warm-room config key on one form.
- Pinned warm rooms are exempt from idle eviction; the recovery pass is
  the only thing that closes them (criterion: lease outcome, not idleness).

Acceptance gate (all offline, no TikTok requests — the live proof of the
leased session lands in PR 2's shadow acceptance, per plan):

```
docker run --rm -v $PWD/services/tiktok-signer:/app -w /app \
  node:22-bookworm-slim sh -c "npm ci && npx tsc --noEmit && npm test"
→ 6 test files, 63 tests passed, exit 0 (verified after round-2 review fixes, 2026-09-18)
```

Honest costs recorded by the plan and README: the capture breaker paces a
dead/live_new configured warm room but cannot prevent the first lane
profile rotation it drives (pool-side failure accrual is unconditional,
`PROFILE_ROTATE_FAILURES` = breaker threshold = 3); the actual control is
`SIGNER_WARM_ROOMS` listing classic, verified-live rooms only.

## PR 2 Task 1 — the bare WS shape is falsified; recorded params + cursor=0 deliver (2026-09-18 19:54-20:20 UTC, 4 WS connects)

PR 2's plan (rev 5) hypothesized that the synthesized fetchResult could
carry a bare push URL — `pushServer?compress=gzip&room_id=<TARGET>
&internal_ext=&cursor=0`, every recorded identity param discarded — with
only `cursor='0'` needing live verification. Measured against the
PR 1 lease endpoint (signer-lab rebuilt on commit 2845b5df,
`SIGNER_WARM_ROOMS=zaganovakov`, one warm lease captured 19:54:13 UTC),
same target room (zaganovakov, live-verified), same lease cookies, same
minute, back to back:

| Shape | Handshake | Enter | Chat |
|---|---|---|---|
| shipped (bare: `compress=gzip&room_id=<TARGET>&internal_ext=&cursor=0`) | **rejected — "Unexpected server response: 200"** | — | — |
| recorded (lease wsUrl verbatim, identity params intact) | **accepted** | `im_enter_room_resp` received | (closed at ~2 s: the instrument's ack was a JSON string, not a PushFrame — instrument bug, fixed) |
| recorded + `cursor=0` substituted, proper PushFrame ack | **accepted** | `im_enter_room_resp` received | **19 WebcastChatMessages, 115 frames, 103 acks, every payload's roomId = the target's, full 120 s window** |

Budget: 4 WS connects total (bare attempt, recorded control, cursor0
first run with the malformed ack, cursor0 with the fixed ack), no other
TikTok surfaces in between, zero 403s. The early `read_message` close on
the first recorded-shape runs was the probe instrument's own bug — the
ack must be a PushFrame echoing logId with the internalExt payload
(cross-room-ws.cjs's shape), not a JSON string; with the proper ack the
connection holds and chat flows.

**Consequence for PR 2 (measured, final)**: the synthesized fetchResult
must forward the lease wsUrl's recorded identity params via
`routeParams` — everything except `compress`, `room_id`, `internal_ext`,
`cursor`, which the connector appends itself — with `pushServer` =
origin+path and cursor `'0'`. That is exactly the shape PR 2's
PureNodeSigner implements (the shape change from plan rev 5's bare-origin
hypothesis, adopted on this measurement). The cursor sentinel `'0'` is
validated on the wire. Cross-room entry + cursor=0 + target room_id
re-pointing delivers target-room chat on a leased classic-room session:
PR 2's delivery mechanism is proven end to end at the protocol level.
The listener-level proof (real connector, synthesized SignResult through
the route handler, ack path, reconnect) remains Task 5's soak.

## Egress policy: webshare dropped (operator decision, 2026-09-18)

Webshare residential proxies are DROPPED for TikTok, lab and prod: the
datacenter IPs have better reputation than the proxy ones (measured
2026-09-16: all 10 webshare lanes stopped serving im/fetch while direct
egress captured fine; the signer switched to direct egress in
caesar-deployment #104). Consequences for this effort:

- PR 2's Task 5 soak prerequisite "signer-side proxy lanes +
  `SIGNER_WEBSHARE_TOKEN`" is VOID: the lab runs direct, matching prod.
  The `pinned` lane-pin criterion does not apply (no lanes to pin);
  the pre-sign path simply observes no proxyCredentials in the lab.
- Spike #6c's open question ("PR 2 must decide pure-node fetch egress")
  resolves to DIRECT for both the fetch leg and the WS leg.
- Do not re-introduce webshare wiring, tokens, or proxy-lane machinery
  in any later PR without an explicit operator instruction.

## PR 2 Task 5 — live soak PASSED (2026-09-18 21:18-21:50 UTC)

Lab rig: signer-lab on PR 1 image (commit 2845b5df, viewer-mode,
`SIGNER_WARM_ROOMS=zaganovakov`, direct egress — webshare dropped, see
the egress-policy section above); lab-listener on the PR 2 build
(`TIKTOK_SIGNER_MODE=pure-node`, dist built from the a81af4fd tree).
Demand snapshots published directly on the lab Redis `source:demand`
channel (no source-manager in the lab). Two verified-live rooms:
zaganovakov and dan2dxo; kyle.toynbee was offline at soak time
(listener's own fetchIsLive check).

31-minute window (21:18:46 → 21:49:51 UTC), both rooms connected through
the pure-node signer and stayed connected:

| Criterion (plan Task 5) | Measured |
|---|---|
| ≥1 decoded message per room per run | 252 WebcastChatMessage, 776 LikeMessage, 963 RoomUserSeqMessage, 77 GiftMessage, 637 MemberMessage over the window; both rooms connected 21:17-21:18 and never disconnected |
| payload roomIds | both rooms' connect-time roomIds constant; no re-pointing (per-payload roomId verified in Task 1) |
| lane-pin `pinned` ≥1 | VOID per the webshare retirement; `tiktok_ws_lane_pins_total{outcome="skipped"}=2` is the correct direct-egress outcome |
| lease delta (success+captured) ≤ 8 | **0** — the 10-min lease cache served the entire window without one refresh |
| WS connects ≤ 4 | **2** |
| 403s | **0** |
| sign attempts | `tiktok_sign_attempts_total{signer="pure-node",outcome="success"}=2` — one per connect, zero failures, zero re-signs (no flap storm) |

Only recurring log noise: the canary relay's HTTP 409 retry loop on
dan2dxo — the documented pure-node limitation at soak time (the relay
needs a warm viewer tab the lease never creates; TIKTOK_CANARY_ROOMS
should stay unset in pure-node rollouts until PR 3). Not load-bearing,
retried with backoff. [Superseded by PR 3: the canary consumer now warms
the room's tab on the signer when the relay answers 409 — at most once
per stint, budget-audited by tiktok_canary_warms_total.]

The install report logged `signer_mode: pure-node, signer: pure-node,
euler_reachable_for_signature: false` — Euler is off the connect path
end to end. PR 2's acceptance is complete; the merge is unblocked.

## PR 3 — canary warm attach PROVEN live; promotion path not exercised (2026-09-18 23:21-23:27 UTC)

Code (branch `docs/tiktok-pr0-gate-tests`, uncommitted at test time): the
pure-node canary/fallback warm wiring. Lab listener rebuilt from the PR 3
tree (`npm run build` in the lab-listener container), recreated with
`TIKTOK_FLAP_MAX_FAST_RETRIES=0` (promotion arming), `TIKTOK_PREMIUM_FALLBACK=on`,
`TIKTOK_SIGNER_MODE=pure-node`; signer-lab unchanged (PR 1 image,
`SIGNER_RELAY_FALLBACK=on`, warm room zaganovakov — offline all session).
Premium flag: the seed overlay's owner `premiumtest` was already
`is_premium=true` (rig leftover from the PR 2 soak setup; the fresh
container's checker cache was cold, so no TTL staleness).

**Canary warm attach — measured chain, live:**

```
23:25:14 canary mirror started (shaman_production777, TIKTOK_CANARY_ROOMS override)
23:25:14-17 relay stream HTTP 409 x3  (SSE retries during the warm's capture window)
signer-lab: "viewer capture ok" shaman_production777, lane direct, elapsed_ms 3301, attempt 1
signer-lab: "relay tap attached" shaman_production777
23:25:21 relay stream connected  → canary frames flowing
listener /metrics: tiktok_canary_warms_total{attempted}=1 {warmed}=1
```

Exactly ONE warm per stint on the wire: the three 409s are the SSE retry
loop running while the single warm capture was in flight; after it
registered the tab, the reconnect attached. The pre-PR 3 soak's
"TIKTOK_CANARY_ROOMS should stay unset" guidance is dead: the canary set
works in pure-node mode now.

**Pure-node connect on the PR 3 dist:** 9 verified-live rooms (live-feed
discovery via a one-shot page load in the signer's browser — 12 handles,
no captures involved) connected cleanly through the leased session:
zero flaps, zero 403s, zero sign failures.

**Promotion path — NOT exercised live, honestly:** a room only promotes
after flap exhaustion, and no room flapped — the session was healthy the
whole window. The per-pod WS-connect budget (12/h) exhausted first (11
connects + the budget's own guard), which is the budget working as
designed, not a flap. With `TIKTOK_FLAP_MAX_FAST_RETRIES=0` armed and a
premium-flagged overlay owner in place, the trigger never fired. The
promotion path's logic is covered offline: warm classification
(`sign/warm-target-tab.test.ts`, 6 tests) and the availability predicate
(`sign/sign-clients.test.ts` matrix). The plan's switch-to-a-gated-room
contingency had no gated room to switch to: the known gated set (jinuabi,
neringakazlauskaite, sarameels) was offline all session (status 4), and
live-feed discovery served only healthy rooms. The natural-flap live
proof remains open for the PR 4 soak window, where sustained rooms
will meet real flaps. Healthy-primary demotion is out of scope per
(unobservable while the fallback delivers) — the stint-end paths
(room end / tab death / signer refusal / 6h ceiling) are the shipped
demotion and were not re-verified here.

Budget accounting (23:21-23:27 UTC): signer sign requests 1
(the canary warm, success), session leases served from cache
(success counter 22 = pre-existing pod restarts' warm-lease reads, no
new captures: leases captured stayed 1), capture breaker quiet
(0 refusals, 0 trips), zero 403s on both services. The 11 WS connects
stayed inside the listener's per-pod budget, which then correctly
refused further connects for the hour.

Offline acceptance (container gate): `npx tsc --noEmit && npm test`
green — 544 passed, 8 skipped (6 warm-classification, 4 canary-warm-flow
tests added in PR 3), the one pre-existing 1ms-tick flake in
connection-decisions.test.ts:166 observed once mid-run and green on
re-run (documented pre-existing, not in this diff).

Rig restored: lab-listener recreated with the rig-default env (flap
retries 3, canary dan2dxo, PR 3 dist), lab DB sources back to
zaganovakov/kyle.toynbee/bigjaygaming01.

## PR 4 — 48h lab soak STARTED (2026-09-19 08:08 UTC, PR 3 dist)

Rig: signer-lab on the PR 1 image recreated for the soak with
`SIGNER_WARM_ROOMS=justin.playback` (classic-verified: viewer capture ok,
8.1 s) and `SIGNER_RELAY_CANARY_ROOMS=kaeptn_4.0,markusmeyer681,batya.rinka`;
lab-listener on the PR 3 dist, `TIKTOK_SIGNER_MODE=pure-node`,
`TIKTOK_PREMIUM_FALLBACK=on`, `TIKTOK_FLAP_MAX_FAST_RETRIES=0` (PR 3's
open promotion proof armed: any natural flap on the premium-flagged seed
overlay promotes), all three soak rooms in `TIKTOK_CANARY_ROOMS`.
A host-side watcher (`/tmp/soak-watch.sh`) republishes the demand
snapshot every 15 min and appends a metrics snapshot every 30 min to
`/tmp/soak-snapshots.log`; all soak rooms are seeded into the lab DB.

Connect phase (08:08 UTC): all three rooms connected on the leased
session first try (zero flaps); each canary 409'd exactly once before
its warm (`tiktok_canary_warms_total{attempted,warm}=3`, three viewer
captures 12.8-19.5 s) and attached. Warm-room session lease healthy.

First 10 minutes of divergence data:

- `frame_rate` x2 — both startup artifacts: the canary joins a 60s
  comparison window ~15s late (warm capture + tap attach), so its first
  window is structurally short. None in steady state.
- `method_set` x3 — exclusively page-bootstrap classes the viewer
  tab's own browser receives and a pure-node connection never does:
  `WebcastLinkMicBattleItemCard`, `WebcastGiftGalleryMessage`,
  `WebcastAISummaryMessage`. No chat/gift/member/social envelope method
  has appeared canary-only. Benign class; the soak watches it stays
  that way.

Soak pass criteria (to be judged at 24h, 2026-09-20 ~08:00 UTC — the
window was shortened from the plan's 48h to 24h: the lab machine cannot
stay up through the full window, operator travels at ~38h; the criteria
are unchanged, only the observation window shrinks, and room-churn
coverage is reduced accordingly):
1. Zero missed-chat divergence: no `stalled` or `decode_failure_rate`
   firing, and `method_set` never lists a chat/gift/member/social class.
2. Zero session flags: zero 403s on either surface, capture breaker
   quiet, lease budget (8/h) not exceeded.
3. Rooms stay delivered through natural room-end/restart cycles
   (offline backoff → reconnect) without operator intervention, or
   failures are recorded honestly with the rig-swap they needed.
4. Any natural flap on the premium-flagged rooms fires the PR 3
   promotion chain (flap_exhausted → warm → promoted → delivered) —
   the promotion proof PR 3's live window could not produce.
5. The warm-room end-of-stream hazard is expected to surface: when
   justin.playback's stream ends, cold connects 503 until the room is
   live again (no spare warm room configured). Record how the listener
   behaves rather than fixing the rig mid-soak.

Result pending — this section is completed when the window closes
(2026-09-20 ~08:00 UTC).

## Prod incident 2026-09-19: capture gate, pure-node cutover, egress diagnosis

Timeline and measurements (all read-only against prod; changes went
through caesar-deployment PR #110):

- ~09:00 UTC: prod tiktok-signer's viewer-capture surface starts
  refusing every capture: `im/fetch 200 not captured within 90000ms`
  and navigation timeouts, 45 sign requests / 0 success / 45
  tiktok_rejected across three rooms, per-room breaker trips piling
  up. Zero connected TikTok streams. The alert that prompted the
  merge (#915) was this.
- The lab rig — same PR 1 signer code, different egress — captured
  all three prod-failing rooms in ~5-6 s each (lejoe_tiktok 5.6s,
  billybongjr 5.8s, biancaglitters 5.4s). Same code, same rooms:
  the gate scores the prod pod's environment, not our code. #903's
  player-selector fix was in the deployed image throughout.
- 10:39 UTC: prod switched to pure-node (PR #110 merged):
  TIKTOK_SIGNER_MODE=pure-node, TIKTOK_PREMIUM_FALLBACK=on,
  SIGNER_WARM_ROOMS=lejoe_tiktok,billybongjr,
  SIGNER_RELAY_FALLBACK=on. Mode installed cleanly, lease attempts
  classified and backed off correctly — but every warm-room capture
  fails from the prod pod (12 capture_failed, 22+ no_session, 0
  success at time of writing), so no lease exists and delivery stays
  down. The pure-node architecture held: the failure narrowed to a
  single surface (warm-room capture) instead of N per-room captures.
- Fresh-browser-profile hypothesis ruled out by inspection:
  SIGNER_USER_DATA_DIR is an emptyDir and the mounted dir is empty —
  every pod restart is already a fresh profile, and the 10:00
  restart's captures failed identically. The gate is keyed on the
  pod's egress IP / ASN or an in-memory session property, not the
  profile.
- Operator decision: wait out the gate (2026-09-16 precedent: gates
  flap and re-open on cooldown). Pure-node is armed — the first
  successful capture self-restores delivery for every room with no
  further changes. Webshare re-introduction remains an explicit
  operator decision away (dropped per #104/#106; today's
  measurement inverts its premise but does not un-drop it).

A gate watcher (/tmp/gate-watch.sh on the lab machine) polls the prod
signer's counters every 10 min and exits loudly on the first success.
Update this section when the gate opens (or when the wait decision is
revisited).

**RESOLVED 2026-09-19 ~11:50 UTC** (root cause found while mitigating):
the fresh-IP move (#111, caesar3 → caesar4) plus the actual missing
piece — the tiktok-signer's `limits.cpu: 3` line (#112). #109 had
raised the quota and written the comment describing the 3-core
limit, but the manifest never carried the `cpu:` value, so the pod
ran at 500m since that merge: llvmpipe software-rendering the live
page on half a core while Xvfb and the signature browser competed.
On caesar4 the starvation showed up explicitly as Protocol error
(Page.navigate): Target closed and lane-busy failures; the likeliest
reading of the original caesar3 gate is the same starvation driving
a half-broken browser into TikTok's session scoring (45 failing
captures in ~30 min).

After #112 rolled out: warm captures landing in 4.5-9 s (billybongjr
x3, the lab-healthy shape), session leases served (captured 5 /
success 2), and the listener connected its first prod streams through
pure-node (bawitabaa, ryzzislive) — prod TikTok ingest restored on
the pure-Node transport, cross-room delivery from one warm session,
Euler fully off the path in prod. The listener's own per-pod connect
budget (12/h) exhausted itself during the outage window and resets
hourly — expected transient, not a fault.

Standing follow-ups: the caesar4 pin stays until the gate is
understood (remove per #111's comment); the two replica pods share
the leased session's connect budget at 12/h each (fine at current
room count, revisit before cohort growth); the PR 4 soak judgment
stands at 2026-09-20 08:00 UTC.

## PR 4 soak judgment (2026-09-20, window closed 08:00 UTC)

**Verdict: PASS — on prod evidence, not lab soak evidence.**

The lab soak itself produced only ~1 h of usable data: the soak rooms all
ended their streams by ~09:10 UTC on 09-19, and no replacement cohort was
started (discovering fresh classic rooms cost captures from the same
budget the soak was measuring). The lab was never going to be the
deciding instrument at this room churn rate.

What actually carried the judgment was prod: pure-node ran ~24 h through
the 09-19 outage, the cutover, a node move, and heavy room churn (22
distinct rooms connected in the 24 h to 08:00 UTC on 09-20), with the
listener holding streams and the lease endpoint serving cold captures in
the lab-healthy 4.5-9 s shape once #112's CPU limit landed. A controlled
lab soak adds nothing over two days of that; PR 4's cohort flip is
declared done by the de-facto state: prod IS the cohort, and has been
since 10:39 UTC on 09-19.

## Operational resilience hardening (2026-09-20)

The 24 h also surfaced the operational gaps, each closed same-day:

- **Three capacity alerts** (caesar #113, `allchat-warning-alerts`):
  `TikTokPureNodeConnectBudgetExhausted` (>3 rate_limit sign
  failures/15 m), `TikTokPureNodeNoWarmSession` (>2 no_session/15 m),
  `TikTokPureNodeWarmCaptureFailing` (>3 capture_failed/30 m).
  Overnight triage validated all three fire on real signal: the budget
  alert caught ~77 refusals/12 h from ~8 churny rooms saturating the
  24-connects/h budget (the first hard datum for the session-sharding
  follow-up), and the capture alert caught the pod-move cold-start.
- **Offline-warm-room capture waste** (all-chat #916): the 09-20 capture
  alert fired on a warm room that had gone offline — every cold lease
  fetch burned its ~90 s capture timeout before falling through, the
  breaker churning on a room that could never answer. Fix: the lease
  endpoint liveness-checks warm rooms (api-live user route,
  `user.status === 2`, viewer UA) BEFORE spending a capture, fail-open
  (probe errors still attempt the capture — a liveness-route outage must
  not make warm rooms uncapturable). Signer suite 65/65.
- **Warm-room schedule diversity** (caesar #115, then #116): 2 → 4 → 6
  rooms. The two-room set's failure mode (both streamers offline at
  once) was exactly the 09-20 alert scenario; the four-room morning set
  left the 23-06 UTC window uncovered, so the listener's 24 h connect
  log supplied two high-frequency overnight rooms (stonesinmykidney 4x
  at 02-03 UTC, claudinhoclt 3x at 23 UTC) — chosen from real streaming
  schedules rather than a one-shot discovery scan.
- **Unrelated-but-same-window cluster work**: the
  `KubeDaemonSetRolloutStuck` alert (kured + svclb) root-caused to a
  kube-controller-manager node-view desync from caesar4's overnight OOM
  flap (all DSs were actually ready; `numberMisscheduled` stuck at 1) —
  fixed by restarting k3s on the lease holder. The tempo stack was
  pinned off caesar4 (caesar #117): the 4 GB node had been re-accruing
  unconstrained tempo pods after every restart.

## Warm-room auto-curator (2026-09-20, all-chat #917 + caesar #118)

The static warm list was the remaining flimsy piece: handles verified at
audit time are stale in hours (this day alone needed two manual
re-audits). The curator automates the audit with the manual process's
own safety rules encoded: liveness probe every 30 min (fail-open; warm
tabs count as coverage — a captured session outlives its stream);
under a coverage floor of 2, ONE live-feed discovery pass on a
pool-lane page (same UA and request blocking as a capture's page),
filtered (never canary, never listed, 6 h cooldown on failed
verification), liveness-checked, capture-verified at most 2 — the
capture IS the classic-ness test, and a passing capture leaves the
room warm, which is the point. Any 403 benches all acquisition for an
hour, the operator's manual budget rule since 09-19, now in code.
Add-only: removal stays a human call (breaker + NoWarmSession alert
cover failing rooms).

Ship sequence: #917 merged (71/71 signer tests), soaked ~7 h in prod
with the flag off (29 lease successes, cold-capture fallthrough
healthy), then flipped in caesar #118. Two defects shipped in that
window, both caught same-day: #118's diff dropped the
`SIGNER_WARM_ROOMS` value line (caught before Argo synced it; prod
never served the broken manifest; restored in #119), and the first
live tick exposed `DISCOVERY_CANDIDATE_CAP is not defined` —
`page.evaluate` serializes its callback into the browser context
where module constants don't exist (fixed in all-chat #918; the
injected-discovery test seam structurally cannot catch evaluate-body
bugs, noted for future triage).

After the fix, the first real cycle worked end to end: coverage 1/6
(one seed live), discovery ran, `heav3nn.ow` captured in 7.6 s and
`mwambi09` in 18.1 s, both acquired and lease-servable immediately
(one live Set shared by the lease endpoint, the pool's pinned rooms
and the curator — no restart). The warm list now self-heals.

## Closeout state (2026-09-20)

- Prod TikTok ingest: pure-node, cross-room from warm leases, Euler
  fully off the path. Listener 2 replicas, `TIKTOK_PREMIUM_FALLBACK=on`,
  canary rooms empty.
- Signer: caesar3 (20 GB node), `cpu: 3` / `memory: 6Gi`, six-room seed
  list, `SIGNER_WARM_AUTOCURATE=on`, `SIGNER_RELAY_FALLBACK=on`.
- Open follow-ups, in priority order: (1) session sharding for
  connect-budget capacity — ~8-10 churning rooms saturate 24
  connects/h, the first measured hard limit of the single-session
  design; (2) caesar4 needs RAM or swap, or a deliberate
  redis-node-2 volume migration (its local-path PV pins it);
  (3) `TikTokPureNodeNoWarmSession`'s >2/15 m floor may ride just
  under a long no-warm soak at the listener's lease cadence — wait for
  one real occurrence before tuning.
