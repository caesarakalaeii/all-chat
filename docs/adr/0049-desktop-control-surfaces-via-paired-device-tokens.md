# ADR-0049: Stream Deck and Desktop Control Surfaces via Paired Device Tokens

**Date**: 2026-08-14
**Status**: Proposed
**Deciders**: caesarakalaeii

## Context and Problem Statement

A streamer asked for Stream Deck buttons that drive All-Chat directly: start or close a poll, and send a canned message to every connected chat from the monitor view. The reasoning is the one All-Chat exists for. A multistreamer already has both hands busy, and the dashboard is one more window competing for attention on a second monitor. A physical button that fans a message out to Twitch, YouTube, Kick, TikTok and Discord at once is the shortest path between an intent and five platforms.

Two questions had to be answered before this is worth planning, and the first is settled:

**Is the Stream Deck platform open, and does it cost us anything?** No cost. Elgato's SDK is free, plugin development is free, and publishing a free plugin to the Elgato Marketplace is free. A plugin is ordinary local software: a Node.js backend for logic plus an optional Chromium-rendered settings panel, talking to the Stream Deck app over a local WebSocket, on Windows 10+ and macOS 10.15+. Nothing about it requires a commercial relationship with Elgato.

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
- **Do not re-implement platform OAuth in the plugin.** Our OAuth client relationships (Twitch, Google, Kick) live in auth-service with server-side registered redirect URIs. Dragging a desktop client into those flows means registering loopback redirects with three providers and inheriting each provider's rules about them, for no gain.
- **Least privilege (ADR-0012), extended to devices.** A button that sends chat messages should not carry a token that can also delete the streamer's overlays or read their billing.
- **Revocation must be believable and self-service.** A streamer who sells or loses a laptop must be able to cut it off from the dashboard, without rotating anything else.
- **Reuse the gates we already have.** Poll creation is already premium-gated by `requireEngagementPremium`; chat send already requires the advanced-controls consent grant. A device token must not become a way around either.
- **Generalise beyond Elgato.** Touch Portal, a stream-side CLI, and a phone remote are the same problem. Deciding this per-vendor would mean deciding it repeatedly.

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
- ❌ Cons: it is the only option with real backend work: a table, a pairing endpoint pair, a token type in the auth middleware, and dashboard UI to approve and revoke. Pairing codes must be short-lived and rate-limited or they become guessable.

## Decision Outcome

**Chosen: option 5, a device pairing code exchanged for a long-lived scoped device token.** Option 4 is explicitly recommended as the shape of a throwaway spike (see Implementation Notes), not as something to ship.

**Rationale**: the plugin's problem is not "prove which Twitch account this is", which options 2 and 3 both over-solve, but "let a specific device act for a specific streamer, narrowly, for a long time, revocably". A pairing code puts the one step that requires trust (a human confirming, in the dashboard, that this device may act for them) in the one place where we already have a trustworthy session, and leaves the plugin holding a credential that is useless for anything except the buttons it was paired for. Options 1 and 3 both hand out credentials far broader than the task and rely on streamers handling secrets carefully on camera. Option 4 is attractive precisely until the streamer closes the tab, which is the thing they asked to stop worrying about.

The decision is deliberately framed around **device tokens, not Stream Deck**. The Elgato plugin is the first consumer; Touch Portal and a stream-side CLI are the same mechanism with a different front end, and nothing in the backend should mention Elgato.

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

- New long-lived credential type, which is new attack surface and needs to be in scope for the next security review, including the pairing-code brute-force bound.
- Rate limiting matters more than usual: a physical button invites mashing, and chat send in particular fans out to five platforms per press, where per-platform limits are the binding constraint.
- Stream Deck's Node runtime is pinned by the installed Stream Deck version (20 or 24 depending on release), so the plugin's toolchain is not ours to choose.
- Publishing to the Marketplace puts All-Chat's name on software running on machines we cannot debug, so plugin-side errors need to surface clearly rather than failing silently, which is the failure mode option 4 was rejected for.

## Implementation Notes

Suggested order, so that each step is independently useful:

1. **Spike first, decide nothing.** Build the plugin against a hand-issued token and wire two buttons (start poll, send message). This validates the button ergonomics, which is the part most likely to be wrong, before any backend commitment. Option 4's local bridge is an acceptable shortcut for this spike only.
2. **Device pairing backend**: table, `POST` to request a code, `POST` to approve it from an authenticated dashboard session, device-token recognition in the gateway's auth middleware, scope enforcement, revocation endpoint.
3. **Dashboard UI**: approve a code, name a device, list paired devices with last-used, revoke.
4. **Plugin actions** against the real flow, published to the Marketplace as a free plugin.
5. **Release requirements** above.

Prior art to follow rather than reinvent: `shared/middleware/premium.go` for gate enforcement shape, and `tokens/source.go` for how credentials are already resolved per user.
