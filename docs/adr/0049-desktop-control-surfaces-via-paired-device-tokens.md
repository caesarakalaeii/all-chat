# ADR-0049: Stream Deck and Desktop Control Surfaces via Paired Device Tokens

**Date**: 2026-08-14
**Status**: Proposed
**Deciders**: caesarakalaeii

## Context and Problem Statement

A streamer asked for Stream Deck buttons that drive All-Chat directly: start or close a poll, and send a canned message to every connected chat from the monitor view. The reasoning is the one All-Chat exists for. A multistreamer already has both hands busy, and the dashboard is one more window competing for attention on a second monitor. A physical button that fans a message out to Twitch, YouTube, Kick, TikTok and Discord at once is the shortest path between an intent and five platforms.

Two questions had to be answered before this is worth planning, and the first is settled:

**Is the Stream Deck platform open, and does it cost us anything?** No cost. Elgato's SDK is free, plugin development is free, and publishing a free plugin to the Elgato Marketplace is free. A plugin is ordinary local software: a Node.js backend for logic plus an optional Chromium-rendered settings panel, talking to the Stream Deck app over a local WebSocket, on Windows 10+ and macOS 10.15+. Nothing about it requires a commercial relationship with Elgato.

The hardware is not only driven by Elgato's software, and that materially affects reach. Elgato's own app does not run on Linux at all, so a Linux streamer with Stream Deck hardware uses one of two GPL-3.0 third-party apps instead, and they differ in exactly the way that matters here:

- **OpenDeck** (Rust) explicitly "supports plugins made for the original Stream Deck SDK", and runs on Linux, Windows and macOS. A plugin built once against Elgato's SDK is therefore expected to work under OpenDeck as well, including on Linux, at no extra build cost. It additionally offers its own OpenAction API, which we would not need.
- **StreamController** (Python, Linux-first) uses its **own** plugin format and does **not** load Elgato plugins. Supporting it means a second, separate plugin against its Python API (`PluginBase`, `ActionCore`, `BackendBase`), submitted through its own review process.

So "does this support StreamController too?" has a two-part answer, and the split falls exactly along the seam this ADR draws. The pairing mechanism and every endpoint are front-end agnostic and are reused verbatim by both. The plugin is not portable between them.

**The remaining question is authentication, and it is the whole of the work.** The actions themselves need no new backend at all. Every endpoint a first version would call already exists and is already exposed through the API gateway:

| Button | Existing endpoint |
|---|---|
| Start poll | `POST /api/v1/engagement/overlays/:id/polls` |
| Close poll | `POST /api/v1/engagement/overlays/:id/polls/:pollId/close` |
| Prediction lock, resolve, cancel | `POST /api/v1/engagement/overlays/:id/predictions/:pid/{lock,resolve,cancel}` |
| Send message to all chats | `POST /api/v1/auth/chat/send` |

So a plugin is a thin authenticated HTTP client over endpoints we ship today. What we do **not** have is any way for software that is not a browser to hold credentials. Every authenticated caller today is a browser: the dashboard uses a cookie, and the browser extension is handed a JWT by `postMessage` to an allowlisted platform origin (`ALLOWED_OPENER_ORIGINS` in `chat/auth-success`). Neither mechanism has any meaning for a desktop process, and our JWTs live 24 hours (`shared/auth/jwt.go`), which is far too short for a device a streamer pairs once and forgets.

Elgato's own documentation is explicit that plugins must not ship secrets: "It is not recommended to include secrets, for example private API keys, when packaging and distributing your plugin." A plugin is published software installed on machines we do not control, so it is an untrusted public client in the OAuth sense, and it cannot hold a client secret.

## Decision Drivers

- **The plugin is a public, untrusted client.** It is distributed to strangers' machines. It must never hold a client secret, and compromising one installation must not compromise the streamer's platform credentials.
- **Pair once, work for months.** A control surface that needs re-authorising mid-stream, or every 24 hours, is worse than no control surface. This rules out handing over an ordinary JWT.
- **Install must be as close to one click as the platform allows.** This is a convenience feature competing against the streamer's existing habit of using the dashboard, so setup friction is not a minor UX detail, it decides adoption. The extension's one-click login is the bar to match, and anything that asks the streamer to copy, paste or transcribe a value starts below it.
- **Do not re-implement platform OAuth in the plugin.** Our OAuth client relationships (Twitch, Google, Kick) live in auth-service with server-side registered redirect URIs. Dragging a desktop client into those flows means registering loopback redirects with three providers and inheriting each provider's rules about them, for no gain.
- **Least privilege (ADR-0012), extended to devices.** A button that sends chat messages should not carry a token that can also delete the streamer's overlays or read their billing.
- **Revocation must be believable and self-service.** A streamer who sells or loses a laptop must be able to cut it off from the dashboard, without rotating anything else.
- **Reuse the gates we already have.** Poll creation is already premium-gated by `requireEngagementPremium`; chat send already requires the advanced-controls consent grant. A device token must not become a way around either.
- **Generalise beyond Elgato.** OpenDeck, StreamController, Touch Portal, a stream-side CLI and a phone remote are all the same problem. Deciding this per-vendor would mean deciding it repeatedly, and one of those vendors (StreamController) cannot run the Elgato plugin at all, so a vendor-shaped decision would strand it.

## Considered Options

### 1. Streamer copies a JWT from the dashboard into the plugin's settings

- ✅ Pros: zero backend work; ships immediately.
- ❌ Cons: our JWTs expire in 24 hours, so this is broken by design for a paired device and would generate a support queue of "my buttons stopped working" every single day. It also trains streamers to copy full-power session tokens into third-party software, and the token carries every permission the streamer has. Revocation means invalidating their own session.

### 2. Loopback OAuth with PKCE, plugin as an OAuth client against the platforms

- ✅ Pros: the standard answer for native apps; no secret in the plugin thanks to PKCE.
- ❌ Cons: solves the wrong problem. The plugin does not need platform identity, it needs *All-Chat* identity, and All-Chat identity is already derived from a completed platform login. This would require registering `http://127.0.0.1:<port>` redirect URIs with three separate providers, each with its own policy on loopback and dynamic ports, and would put a second OAuth client implementation in software we ship to end users. It buys nothing that option 4 does not.

### 3. Long-lived personal access token, generated and pasted by the streamer

- ✅ Pros: simple to build and to reason about; a familiar pattern; works headless.
- ❌ Cons: the pasted-secret UX is a known source of leaked credentials (streamers paste them on stream, into Discord, into screenshots), and this one would be leaked *on camera* by the exact population using it. There is also no natural moment to scope it, so it tends to become an all-powerful key.

### 4. Local bridge: the plugin drives the already-open dashboard tab

The plugin runs a `127.0.0.1` WebSocket server; the monitor view connects to it and performs the API calls using the session it already has.

- ✅ Pros: no new credential exists anywhere, so nothing can leak or need revoking; genuinely zero backend work.
- ❌ Cons: only works while a dashboard tab is open and focused enough not to be throttled, which is precisely the window the streamer wanted to stop babysitting. Any browser-side hiccup silently kills the buttons, and the failure is invisible from the Stream Deck. It also cannot support anything the dashboard cannot already do, and cannot work on a second machine, which is a common Stream Deck layout.

### 5. Device pairing code, exchanged for a long-lived scoped device token

The plugin requests a pairing code from All-Chat and displays it. The streamer, already logged into the dashboard, enters that code (or follows a deep link) and confirms which overlay the device controls. The backend binds the code to their user and returns a long-lived, narrowly-scoped **device token** which the plugin stores locally.

- ✅ Pros: no secret ships in the plugin; no platform OAuth involvement; the credential is minted for one device with one scope set and one overlay, so it is genuinely least-privilege; it is listable and individually revocable in the dashboard; it works headless, across machines, and for any future control surface; and the human confirmation step happens in a context where we already know who the streamer is.
- ❌ Cons: **the streamer has to type a code.** That is friction on the one step where friction is least affordable, the first thirty seconds of using the thing. It also needs a table, a pairing endpoint pair, a token type in the auth middleware, and dashboard UI. Pairing codes must be short-lived and rate-limited or they become guessable.

### 6. Loopback redirect from All-Chat, PKCE-protected, exchanged for a device token

The install flow the extension has, adapted to a desktop process. The plugin binds an ephemeral port on `127.0.0.1` and opens the system browser at All-Chat with that loopback address as the redirect target. The streamer, normally already logged in, sees one approve screen (which overlay, which actions, name this device), clicks Approve, and All-Chat redirects to the loopback with a one-time code that the plugin exchanges, with its PKCE verifier, for the device token. **Nothing is typed and nothing is pasted.**

Note what is different from option 2: here **All-Chat is the authorization server**, so the loopback redirect is registered with *us*. The platform login (Twitch, Google, Kick) is untouched and happens, if at all, exactly as it does today, in the browser, before the approve screen.

- ✅ Pros: the easiest install available short of no install, and the closest desktop analogue to the extension's existing one-click flow; still no secret in the plugin, because PKCE replaces the client secret (RFC 8252, OAuth 2.0 for Native Apps); mints the same scoped, revocable device token as option 5, so it is a delivery mechanism rather than a different credential; the approve screen is a natural place to show scope and overlay, which a typed code has to do somewhere anyway.
- ❌ Cons: **loopback only works when the browser and the plugin are on the same machine**, which is not universal: streamers run a Stream Deck against a second PC or a headless capture box. It also assumes the plugin host permits binding a port. And the redirect validation is security-critical: it needs a strict server-side rule of its own rather than reuse of the user-facing redirect allowlist.

## Decision Outcome

**Chosen: option 6 as the primary flow, falling back to option 5's pairing code when loopback cannot work.** Both mint the identical scoped device token, so this is one credential with two delivery paths, not two designs. Option 4 is explicitly recommended as the shape of a throwaway spike (see Implementation Notes), not as something to ship.

**Rationale**: the plugin's problem is not "prove which Twitch account this is", which options 2 and 3 both over-solve, but "let a specific device act for a specific streamer, narrowly, for a long time, revocably". Options 5 and 6 answer that identically and differ only in how the streamer gets the token into the plugin, which makes install friction the deciding factor. A loopback redirect is the one option where the streamer clicks Approve once and is finished, and it is the same shape of flow they already know from the extension. The pairing code survives as a fallback rather than being discarded, because loopback has a real and not-rare failure mode (a control surface on a different machine from the browser) and a typed code is the only thing that crosses a host boundary. Options 1 and 3 both hand out credentials far broader than the task and rely on streamers handling secrets carefully on camera. Option 4 is attractive precisely until the streamer closes the tab, which is the thing they asked to stop worrying about.

### Redirect validation is its own rule, deliberately

The loopback redirect must be validated by a dedicated server-side check: scheme exactly `http`, host exactly `127.0.0.1` or `[::1]`, any port, one fixed path. Three specifics matter:

- **Never accept `localhost` as the host.** It resolves through DNS and can be pointed elsewhere; the literal addresses cannot.
- **Any port must be allowed**, because the plugin cannot reserve one in advance and a fixed port collides. This is what RFC 8252 expects of the authorization server, and it is only safe because the host is pinned.
- **Do not route this through `isAllowedExternalRedirect`.** That guard exists for user-facing navigation, and it already had to be fixed once for backslash normalisation (audit M1). A redirect that hands over a credential deserves a narrow rule that cannot be widened by an unrelated change to the general one.

The code itself is one-time, short-TTL, and bound to the PKCE challenge, so an intercepted redirect on a shared machine is not replayable.

The decision is deliberately framed around **device tokens, not Stream Deck**. The Elgato plugin is the first consumer; OpenDeck, StreamController, Touch Portal and a stream-side CLI are the same mechanism with a different front end, and nothing in the backend should mention Elgato.

### Which front ends v1 covers

**Two plugins ship in v1**, because no single plugin format reaches all three apps:

- **StreamController** (Linux): a Python plugin (`PluginBase`, `ActionCore`, optional `BackendBase`, with `manifest.json`, `about.json` and `attribution.json`, submitted through its own process). **In v1, and the primary development target.** It is the de-facto default for Linux Stream Deck users, and decisively it is the surface the maintainer actually runs, so it is the only one that gets dogfooded daily rather than tested occasionally. Building against it first means the button ergonomics are validated by real use during a real stream instead of by a checklist.
- **Elgato Stream Deck app** (Windows, macOS): a TypeScript/Node plugin against Elgato's SDK, published free to the Elgato Marketplace. In v1, because it is where most Stream Deck owners are.
- **OpenDeck** (Linux, Windows, macOS): no third plugin. It loads plugins built for the original Stream Deck SDK, so it should free-ride on the Elgato plugin. Verify rather than assume during the spike, since that is a compatibility claim we have not tested against our own plugin, but it costs nothing to check and covers a third app if it holds.

**The real cost of two plugins is drift, not effort.** Each is a thin HTTP client, so neither is large, but the *button set* now exists twice and will diverge unless it is defined once. Treat the action list (name, endpoint, payload, and what the button surfaces on failure) as the single source both implementations follow, with a keep-in-sync comment at both sites, exactly as `OnboardingChecklist.tsx` and `/upgrade` already do for the premium feature list. A button that exists on Linux but not Windows is a support burden that outlives the release.

Because a Python plugin is loaded into a GPL-3.0 process (both StreamController and OpenDeck are GPL-3.0), the licence question is a **v1 gate rather than a later cleanup**. All-Chat is already AGPL-3.0 so no conflict is expected, but this must be settled before the first StreamController submission, not after.

### Scope of a device token

A device token is not a session. It:

- is bound to one user and, at pairing time, to one overlay;
- carries an explicit action scope, initially `engagement:write` and `chat:send`, and is rejected on any route outside it;
- does **not** satisfy the advanced-controls consent grant or the premium gate on its own. Both are still evaluated against the owning streamer at call time, so a device cannot be used to route around `requireEngagementPremium`, and chat send still requires that the streamer completed the moderation re-consent that issues `user:write:chat`, which is why the dashboard's "Reconnect" must use that flow rather than a plain login;
- is independently revocable, and revoking one device affects nothing else;
- is long-lived but not eternal, with a sliding expiry refreshed by use, so an abandoned pairing lapses on its own.

### Release requirements

Per CLAUDE.md, shipping this is not done until three things are true, and they are part of this decision rather than an afterthought:

1. **Premium toggle.** A `desktop_control_surfaces` feature gate on the *pairing* endpoint, so access can be flipped without a deploy. Gating pairing rather than each action keeps enforcement in one place and leaves the existing per-action gates untouched.
2. **Onboarding extras tour.** Entries in both the "Optional: go further" list in `OnboardingChecklist.tsx` and the premium list in `/upgrade`, which must stay in sync (there are keep-in-sync comments at both sites).
3. **Patreon post** once merged and deployed.

## Consequences

**Positive**

- The plugin becomes a thin client over endpoints that already exist, so the recurring cost of new buttons is near zero.
- Every future control surface (Touch Portal, CLI, phone remote) reuses the same pairing mechanism with no further design work.
- All-Chat gains its first credential type that is not a browser session, which is the missing piece for any non-browser integration.
- Compromising a paired device exposes two actions on one overlay, not a streamer's account.

**Negative / risks**

- New long-lived credential type, which is new attack surface and needs to be in scope for the next security review, covering the pairing-code brute-force bound, the loopback redirect validator, and the one-time-code replay window.
- Two linking paths mean two paths to keep correct. The fallback will be used rarely, which is exactly why it will rot silently unless it is tested deliberately rather than only when someone reports it.
- Loopback binds a listening socket on the streamer's machine, briefly, during linking. It should bind to `127.0.0.1` only (never `0.0.0.0`) and close as soon as the code arrives or a short timeout expires.
- Rate limiting matters more than usual: a physical button invites mashing, and chat send in particular fans out to five platforms per press, where per-platform limits are the binding constraint.
- Stream Deck's Node runtime is pinned by the installed Stream Deck version (20 or 24 depending on release), so the plugin's toolchain is not ours to choose.
- Publishing to the Marketplace puts All-Chat's name on software running on machines we cannot debug, so plugin-side errors need to surface clearly rather than failing silently, which is the failure mode option 4 was rejected for.
- **Two plugins in v1, in two languages, through two review processes.** Python for StreamController and TypeScript for Elgato, each with its own manifest, submission and release cadence. Neither is large, but the button set now exists twice, so it will drift unless a single action list governs both, and every future button is two pieces of work rather than one.
- A GPL-3.0 host process for the Linux plugins makes the licence review a release gate rather than a follow-up.

## Implementation Notes

Suggested order, so that each step is independently useful:

1. **Spike first, decide nothing.** Build a **StreamController** plugin against a hand-issued token and wire two buttons (start poll, send message). StreamController first specifically because it can be dogfooded on a real stream immediately, which is the only cheap way to find out that the button ergonomics are wrong. Option 4's local bridge is an acceptable shortcut for this spike only.
2. **Device linking backend**: table; the loopback authorize endpoint plus the one-time-code exchange with PKCE verification; the dedicated loopback redirect validator described above; the pairing-code fallback pair; device-token recognition in the gateway's auth middleware; scope enforcement; revocation endpoint.
3. **Dashboard UI**: the approve screen (overlay, actions, device name) that both flows land on, plus a paired-devices list with last-used and revoke.
4. **Pin the action list** (name, endpoint, payload, failure surface) as the contract both plugins implement, before the second one is written.
5. **StreamController plugin** against the real pairing flow, submitted with its `manifest.json` / `about.json` / `attribution.json`. Settle the GPL-3.0 question before submitting.
6. **Elgato SDK plugin** implementing the same action list, published free to the Elgato Marketplace, and loaded once in **OpenDeck** to confirm the third app comes free.
7. **Release requirements** above.

Prior art to follow rather than reinvent: `shared/middleware/premium.go` for gate enforcement shape, and `tokens/source.go` for how credentials are already resolved per user.
