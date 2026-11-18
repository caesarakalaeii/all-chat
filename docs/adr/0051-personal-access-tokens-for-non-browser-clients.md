# ADR-0051: Personal Access Tokens for Non-Browser Clients

**Date**: 2026-08-16
**Status**: Accepted
**Revised**: 2026-11-18
**Deciders**: caesarakalaeii

## Context

ADR-0049 prescribed the plugin work order: spike first against a hand-issued
token, before any pairing backend. Three in-flight deliverables (a
StreamController plugin, an Elgato plugin, a frontend token page) need one
long-lived bearer credential authenticating a desktop process as an All-Chat
user, revocable per device, with its format and endpoints frozen as a contract
rather than left to whichever consumer lands first. Constraints: it must work
headless, across machines, behind the gateway's verbatim `Authorization`
forwarding; it must not bypass any gate (the premium gate on starting a
poll/prediction is the most likely accidental "fix"); least privilege; digest
storage per the `overlay_moderators.invite_token_hash` precedent (migration
080).

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

**So are whole-account surfaces.** `DELETE /me` (account deletion, which cascades every overlay) and `GET /me/data-export` (a full PII dump) both refuse a PAT via `RefuseAPIToken`. These are the surfaces where scopes are the *wrong* instrument: there is no scope that should unlock irreversible account destruction, so the answer is refusal at any scope rather than a new `account:delete` scope nobody should ever mint. Without this, the least-privilege claim at the top of this ADR — that a chat-send credential does not carry overlay deletion — would be false, since deletion cascades overlays. The guard runs *before* any mutation or read, so a refusal changes nothing and discloses nothing.

The general rule: **scopes bound what a PAT may do on surfaces it should reach; `RefuseAPIToken` marks the surfaces no PAT should reach at all.** Both are strictly additive to the existing authorization checks — neither ever replaces a premium gate or an ownership check.

## Consequences

Any non-browser client now has a supported credential with no loopback
requirement; ADR-0049's spike was unblocked, and device tokens later reuse the
same resolver seam. Digest-only storage means no dump or log can yield a
usable token (the list projection never selects `token_hash`). Risks: the
pasted secret on camera (bounded, not eliminated — ADR-0049's loopback flow
remains the target for published plugins); no per-overlay binding (a PAT
reaches every overlay its owner owns, ownership checks per overlay still
apply); `expires_at` defaults to NULL so a forced expiry cannot break a live
stream, at the cost that an abandoned token lives until someone notices it;
release requirements (premium toggle, onboarding extras, Patreon post) belong
to the ADR-0049 feature release, not this credential — token *creation* is
deliberately ungated because three consumers were being built against the
endpoints in the same change.

## Implementation Notes## Implementation Notes

- Migration `086_api_tokens.sql` (+ `_down`), following migration 080's hashing and column conventions.
- `shared/middleware/apitoken.go`: prefix test, SHA-256 hashing, `crypto/rand` generation, `SetAPITokenResolver`, the pgx-backed resolver, and `RequireAPITokenScope`. The JWT path in `auth.go` is unchanged for anything without the prefix.
- Startup wiring in `services/{auth-service,api-gateway,engagement-service}/cmd/main.go`, each from its own pool.
- Tests to keep: valid PAT authenticates; revoked rejected; expired rejected; JWT unchanged (with a resolver wired); missing scope rejected; and a **non-premium owner's correctly scoped PAT is still 403'd by `RequirePremium`**.
- Prior art followed rather than reinvented: `SetLogger` for the setter shape, `shared/middleware/premium.go` for gate enforcement shape, and migration 080 plus `moderation-service/repository/grants.go` for "hash it, project it never".

(ADR numbering is shared with caesar-deployment, so this is 0051.)
