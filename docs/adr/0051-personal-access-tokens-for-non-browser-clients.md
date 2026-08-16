# ADR-0051: Personal Access Tokens for Non-Browser Clients

**Date**: 2026-08-16
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

ADR-0049 established that All-Chat has no credential type for software that is not a browser, and chose a PKCE loopback pairing flow to deliver a scoped device token to a Stream Deck / StreamController plugin. It also prescribed the order of work: **spike first against a hand-issued token**, with two buttons, on the surface the maintainer actually runs, so the button ergonomics are validated by a real stream before any pairing backend is committed to.

That spike needs a credential that exists. This ADR decides what the hand-issued token is, and — because the plugin work and a dashboard page are being built against it concurrently — freezes its format and endpoints as a contract rather than leaving them to whichever consumer lands first.

The narrower problem: three sibling deliverables (a StreamController plugin, an Elgato plugin, a frontend token page) all need one long-lived bearer credential, presented on ordinary HTTP requests, that authenticates a desktop process as an existing All-Chat user and is revocable per device.

## Decision Drivers

- **The spike must be unblocked now, without the pairing backend.** ADR-0049's own implementation order depends on a hand-issued token existing first, and the two plugin tasks plus the dashboard page are in flight against it.
- **The credential must be usable by three independent consumers at once.** So its format, its header and its management endpoints are a fixed contract, not an implementation detail one of them may rename.
- **Authentication only, never authorization.** The premium gate on starting a poll/prediction (`RequirePremium(..., GateEngagement, ...)`) is the single most likely thing to be accidentally "fixed" away when a plugin gets a 403. A non-browser credential must be subject to exactly the gates a browser session is.
- **Downstream services must need no changes.** api-gateway's `copyHeaders` forwards `Authorization` verbatim and every backend re-validates independently, so the resolution must happen in `shared/middleware` where all of them already validate JWTs, and must populate the identical request context.
- **Least privilege (ADR-0012).** A button that sends chat must not carry the ability to open premium polls, and neither should reach billing or overlay deletion.
- **Revocation must be believable, immediate and self-service.** A sold laptop must be cut off from the dashboard without rotating anything else, and without a cache that keeps a revoked token alive.
- **Never store a recoverable secret.** The repo already has the precedent and the shape for this: `overlay_moderators.invite_token_hash BYTEA` (migration 080) stores a SHA-256 of a secret shown exactly once.

## Considered Options

### 1. Personal access token: `Authorization: Bearer allchat_pat_<secret>`, resolved in `shared/middleware`

A user mints a named, scoped token in the dashboard; the plaintext is shown once; only its SHA-256 lands in `api_tokens`. A bearer carrying the `allchat_pat_` prefix is hashed and looked up instead of being parsed as a JWT.

- ✅ Pros: exists after one migration and one middleware branch, so the spike starts immediately. Works headless, across machines, and on a second PC — no loopback, no port binding, no browser on the same host. Prefixed, so it is recognisable to secret scanners and unambiguous in a `switch`. Scoped and individually revocable. Nothing downstream changes, because the resolver populates the same context keys `JWTAuthWithRevocation` sets. Digest-only storage means a database dump contains no usable credential.
- ❌ Cons: **the streamer copies and pastes a secret**, which ADR-0049 rejected for the primary flow, and for a real reason: this population pastes things on camera. It also has no natural per-overlay binding — a PAT is user-scoped, so an overlay restriction has to come from the scope set or a later pairing flow, not from the credential itself.

### 2. Wait for ADR-0049's PKCE loopback pairing flow, build nothing before it

- ✅ Pros: one credential type ever; the good install UX from day one; no pasted secret at any point.
- ❌ Cons: inverts ADR-0049's own recommended order — it commits the whole pairing backend (loopback authorize endpoint, one-time code exchange, PKCE verification, the dedicated redirect validator, the pairing-code fallback, dashboard approve screen) *before* anyone has held a physical button and discovered the button set is wrong. It also blocks three in-flight deliverables on the largest, most security-critical piece of the design, and leaves the spike with no credential at all.

### 3. Hand out ordinary session JWTs with a long expiry

- ✅ Pros: no new table, no new middleware path, no new format.
- ❌ Cons: a JWT is a bearer of the *whole* account — no scope, so a chat-send button also carries overlay deletion and billing. Revocation means either the logout blacklist (Redis-resident, keyed on the raw token, sized for 24 h entries, not for months) or invalidating the user's own session. And a long-lived JWT cannot be listed: there is nothing to show in a "your devices" list because the credential is not recorded anywhere.

### 4. Per-service API keys checked at the gateway only

- ✅ Pros: one place to wire, one place to reason about.
- ❌ Cons: factually broken here. `copyHeaders` forwards the client's `Authorization` header verbatim and each backend re-validates independently, so a gateway-only credential is rejected by auth-service and engagement-service the moment the request is proxied. Anything that authenticates at the edge only would also mean the edge is the sole authority, which is exactly the property the current design avoids.

## Decision Outcome

**Chosen: option 1, as an additional authentication path and explicitly not as a replacement for ADR-0049's pairing flow.**

`allchat_pat_` tokens are the hand-issued credential ADR-0049's step 1 calls for, generalised just enough to be a real feature: named, scoped, listed, revocable, and hashed at rest. ADR-0049's loopback + pairing-code delivery remains the intended install UX for a published plugin, and when it lands it mints a *device* token through the same resolver seam — the pasted PAT is what makes the spike (and a self-hoster's CLI, and a second-machine setup that loopback cannot reach) possible in the meantime.

The paste-a-secret objection from ADR-0049 stands and is not answered here; it is *bounded* instead. A PAT is scoped, so a leaked chat token cannot open a premium poll; it is listed with a last-used timestamp, so a forgotten one is visible; and it is revocable in one click without touching the streamer's session. What ADR-0049 rejected was pasting as the *only* mechanism for a published, mass-market plugin, and that rejection is unchanged.

### The contract, frozen

Three consumers are being built against this concurrently, so these are fixed names, not preferences:

- Prefix `allchat_pat_`, presented as `Authorization: Bearer allchat_pat_<secret>`. The secret is 256 bits from `crypto/rand`, base64url without padding, so the token is one copy-pasteable word.
- Table `api_tokens` (migration 086): `token_hash BYTEA NOT NULL UNIQUE` holding a SHA-256, plus `name`, `scopes`, `created_at`, `last_used_at`, `expires_at` (NULL = until revoked), `revoked_at`, and `user_id` with `ON DELETE CASCADE`. **The plaintext is never stored**, mirroring `invite_token_hash` in migration 080.
- Management: `POST`/`GET`/`DELETE /api/v1/auth/me/api-tokens[/:id]`. The plaintext appears in the create response and nowhere else, ever; the list is metadata only.
- Scopes: `chat:write`, `engagement:write`.

### Authentication only — the load-bearing constraint

A resolved PAT sets the same context keys a valid JWT sets (`user_id`, `username`, `twitch_id`, `roles`), so no handler, ownership check or gate can tell the difference. Scope enforcement (`RequireAPITokenScope`) is mounted **beside** the existing authorization middleware and never in place of it: on `POST /overlays/:id/polls` the chain is scope-check *then* `RequirePremium`, so a correctly scoped PAT belonging to a non-premium owner is still refused by the premium gate. That is pinned by a test whose failure message says so, because "the plugin 403s, drop the gate" is the plausible future mistake.

A session JWT passes through the scope middleware untouched — it is not scope-limited — so mounting it on an existing route cannot change behaviour for browser clients.

### Registered in three services, deliberately

The resolver is injected by a package-level setter (`SetAPITokenResolver`, mirroring the existing `SetLogger`) and called at startup in **auth-service, api-gateway and engagement-service**. Gateway-only wiring would be inert: `copyHeaders` forwards the header verbatim, so engagement-service's `JWTAuthWithRevocation` would reject every PAT as a malformed JWT and every plugin poll/prediction action would 401.

### A ban cuts a PAT off, unlike a JWT

The resolver requires `users.is_banned = FALSE` on every request. This makes a PAT strictly stricter than a session JWT, deliberately: a ban blocks *login* (migration 015), and an already-issued JWT is backstopped by its 24-hour expiry, but a PAT defaults to no expiry at all. Without the predicate a banned account would keep acting through a token minted before the ban, indefinitely — which would make the ban button a lie for exactly the users most likely to have automated access.

### Token management is a session-only surface

Create, list and revoke all refuse a PAT-authenticated request, and refuse an impersonating admin. A leaked token therefore cannot mint more tokens, cannot revoke the owner's ability to lock it out, and an admin acting as a user cannot walk away with a credential that outlives the impersonation session (which would defeat ADR-0017's attributability).

The PAT branch is taken *before* the JWT logout blacklist check, because that key is `blacklist:<raw-token>` — routing a PAT through it would write the plaintext secret into a Redis command. PAT revocation is a column read live on every request instead, so it takes effect within one request and there is no cache to invalidate. For the same reason `/logout` refuses a PAT-authenticated request rather than blacklisting one: it would store the secret and accomplish nothing. `CookieToBearer` also refuses to promote an `allchat_pat_` value out of the `access_token` cookie — the cookie is the browser-session channel, and a planted one would make a victim's browser act as the attacker's account for no legitimate gain.

**Admin surfaces are session-only too.** `AdminOnly()` refuses a PAT even when the token belongs to an admin, enforced in that one function rather than per admin route group. A token minted for a Stream Deck button has scopes covering chat and engagement writes; ADR-0049's least-privilege clause says such a credential is "rejected on any route outside" its scope, and user bans, impersonation and feature-gate flips are emphatically outside it.

## Consequences

**Positive**

- ADR-0049's spike is unblocked without committing to the pairing backend, and the two plugins plus the dashboard page have a fixed contract to build against today.
- Any non-browser client — CLI, phone remote, a self-hoster's script — now has a supported credential, on any host, with no loopback requirement.
- Digest-only storage means a database dump, a log, or an errant `SELECT *` cannot yield a usable token; the list projection never selects `token_hash`, so there is no serialisation path to leak one.
- The resolver seam is the same one ADR-0049's device tokens will use, so that work becomes "another row shape behind `APITokenResolver`" rather than a second auth path.

**Negative / risks**

- **A pasted secret, on camera, by exactly this population.** Mitigated (scoped, listed, one-click revocable, `last_used_at` visible) but not eliminated; this is the reason ADR-0049's loopback flow is still the target for the published plugins.
- A user-scoped credential has no per-overlay binding, unlike ADR-0049's device token. Ownership checks still apply per overlay, but a PAT reaches every overlay its owner owns.
- New long-lived credential type, so it is in scope for the next security review: the resolve path, the `last_used_at` write throttle, and the 20-live-token-per-user cap.
- `expires_at` defaults to NULL (until revoked), chosen so a forced expiry cannot break a live stream. The cost is that an abandoned token lives until someone notices it in the list; ADR-0049's sliding expiry is the better answer and is not retrofitted here.
- Two credential types will coexist once pairing lands. They share the resolver and the `api_tokens` shape deliberately, so the risk is naming drift in the UI rather than two divergent auth paths.

### Release requirements, and what is deliberately deferred

CLAUDE.md's three release steps apply to the *user-facing desktop control surfaces* feature that ADR-0049 governs, not to this credential in isolation, and two of the three are deliberately left to that release rather than done here:

1. **Premium toggle** — ADR-0049 already decided where it goes: a `desktop_control_surfaces` gate on the **pairing** endpoint, "keeping enforcement in one place and leaving the existing per-action gates untouched". Token *creation* is the analogous single choke point, but it is deliberately **not** gated in this change: auth-service has no `featuregates` cache wired today, and the two plugin tasks plus the dashboard page are being built against these endpoints concurrently, so introducing a premium refusal on create in the same change that defines the contract would break three in-flight consumers on a product decision this ADR is not the right place to take. The per-action gates that actually protect revenue are untouched and are enforced against a PAT exactly as against a session (`RequirePremium` on round start, and the consent grant on chat send), so nothing is *ungated* in the meantime — what is deferred is gating the act of minting a token.
2. **Onboarding extras tour** — belongs with the user-visible feature (the dashboard token page and the plugins), so it lands with the frontend work, in both `OnboardingChecklist.tsx` and `/upgrade`, which must stay in sync.
3. **Patreon post** — after the plugins ship; a credential with no client to use it is not an announcement.

## Implementation Notes

- Migration `086_api_tokens.sql` (+ `_down`), following migration 080's hashing and column conventions.
- `shared/middleware/apitoken.go`: prefix test, SHA-256 hashing, `crypto/rand` generation, `SetAPITokenResolver`, the pgx-backed resolver, and `RequireAPITokenScope`. The JWT path in `auth.go` is unchanged for anything without the prefix.
- Startup wiring in `services/{auth-service,api-gateway,engagement-service}/cmd/main.go`, each from its own pool.
- Tests to keep: valid PAT authenticates; revoked rejected; expired rejected; JWT unchanged (with a resolver wired); missing scope rejected; and a **non-premium owner's correctly scoped PAT is still 403'd by `RequirePremium`**.
- Prior art followed rather than reinvented: `SetLogger` for the setter shape, `shared/middleware/premium.go` for gate enforcement shape, and migration 080 plus `moderation-service/repository/grants.go` for "hash it, project it never".

(ADR numbering is shared with caesar-deployment, so this is 0051.)
