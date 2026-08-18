# ADR-0049: Personal Access Tokens

**Date**: 2026-08-18
**Status**: Accepted — recorded in full as [ADR-0051](./0051-personal-access-tokens-for-non-browser-clients.md)
**Deciders**: caesarakalaeii

> **Read this first.** The decision described below is real, is built, and is
> live. Its canonical record is **ADR-0051, "Personal Access Tokens for
> Non-Browser Clients"**, and that is the number to cite in code comments and
> commit messages — roughly sixteen non-documentation files already do.
> This file exists because the work was planned under the number 0049 while
> 0049, 0050 and 0051 were landing in parallel, and a reader who follows an
> older reference here should find the reasoning rather than a dead end. It
> restates that reasoning rather than deciding anything separately; where the
> two ever disagree, ADR-0051 is right and this file is stale.
>
> Note also that **0049 already names a different credential**:
> [ADR-0049, "Stream Deck and Desktop Control Surfaces via Paired Device
> Tokens"](./0049-desktop-control-surfaces-via-paired-device-tokens.md) is the
> PKCE loopback *pairing* flow. Numbering is shared with the
> `caesar-deployment` repository (see [README](./README.md)), which is how the
> collision arose. Paired device tokens and personal access tokens are two
> credentials that solve adjacent halves of the same problem, and confusing
> them is the most likely way to misread either document.

## Context and Problem Statement

Every authenticated caller All-Chat had until this point was a browser. The
dashboard carries a cookie; the browser extension is handed a JWT by
`postMessage` to an allowlisted platform origin. Both mechanisms assume a user
agent that can be redirected, can hold a cookie jar, and can be sent through an
OAuth dance in a visible window.

A Stream Deck plugin is none of those things. Neither is the Linux
StreamController plugin, nor a stream-side CLI, nor a headless capture box. They
are ordinary desktop processes that need to make an authenticated HTTP call the
moment a physical key is pressed, on a machine the streamer configured once and
would prefer never to think about again. The session JWT is unusable for them on
two independent counts: there is no browser context to obtain one, and it lives
twenty-four hours (`shared/auth/jwt.go`), so even a hand-copied one would break
every morning.

The actions themselves needed no new backend. Poll and prediction endpoints and
the send-to-all-chats endpoint already existed and were already proxied. The
entire problem was credentials, and it reduced to a single question: **what does
a non-browser client present, and what happens to it on the way to a handler?**

## Decision Drivers

- **A desktop client cannot borrow the browser's session.** Cookies and the
  extension's origin-scoped `postMessage` handoff have no meaning in a Node or
  Python process, so a credential that is not a session is required rather than
  merely convenient.
- **Configure once, keep working for months.** A control surface that needs
  re-authorising mid-stream is worse than no control surface, which rules out
  any credential inheriting the JWT's 24-hour life.
- **A leaked credential must be survivable.** This population pastes secrets on
  camera. Blast radius, visibility and one-click revocation matter more than
  making a leak impossible.
- **Least privilege (ADR-0012).** A token minted for a chat-send button must not
  carry account deletion or a PII export.
- **Authentication is not authorization.** Introducing a second way to *prove
  who you are* must not introduce a second way to *be allowed to do something*,
  or every existing gate silently acquires a bypass.
- **The credential must work wherever it is presented.** Whatever the mechanism,
  it has to survive the gateway proxy and be understood by the service that
  ultimately handles the request.

## Considered Options

1. **A long-lived personal access token, minted in the dashboard and pasted into
   the client.**
   - ✅ Pros: works headless, on a second machine, and behind any front end;
     independent of platform OAuth; naturally scoped, listable and revocable;
     buildable immediately.
   - ❌ Cons: the pasted secret is a known leak vector, and the paste step is
     friction at the worst possible moment.

2. **Wait for the PKCE loopback pairing flow of the other ADR-0049 and build
   nothing before it.**
   - ✅ Pros: one credential type rather than two; the best install UX.
   - ❌ Cons: loopback requires the browser and the client on the same host, so
     it cannot serve a headless box, a second PC or a CLI *at all*. Waiting also
     blocks the spike that was supposed to tell us whether the buttons are worth
     building.

3. **Hand out session JWTs with a long expiry.**
   - ✅ Pros: no new code whatsoever.
   - ❌ Cons: a full-power credential with every permission the user holds,
     revocable only by destroying their own session, and lengthening JWT life
     weakens every browser session too.

4. **Per-service API keys validated at the gateway only.**
   - ✅ Pros: one validation site; conceptually tidy.
   - ❌ Cons: factually broken against this architecture, for the reason set out
     under [Registered in three services](#registered-in-three-services-and-why-that-is-not-belt-and-braces).

## Decision Outcome

**Chosen: option 1 — a long-lived personal access token, presented as
`Authorization: Bearer allchat_pat_<secret>` — as an *additional* authentication
path that does not replace the pairing flow.**

The prefix is load-bearing rather than decorative. `shared/middleware/auth.go`
branches on it to decide whether a bearer is a PAT or a JWT before attempting to
parse anything, so the two credential types are distinguishable by inspection,
cannot be confused for one another, and a PAT never reaches JWT-shaped code
paths that would mishandle it.

The pasted-secret objection is not answered here; it is **bounded**. A PAT is
scoped, so a leaked chat token cannot open a premium poll. It is listed with a
last-used timestamp, so a forgotten one is visible. It is revocable in one click
without disturbing the owner's session or any other token. What the pairing ADR
rejected was pasting as the *only* mechanism for a published mass-market plugin,
and that rejection stands unchanged.

### Stored as a hash, shown exactly once

The plaintext token is **never persisted anywhere**. `api_tokens.token_hash`
(migration 086) holds a SHA-256 digest in `BYTEA`, deliberately mirroring
`overlay_moderators.invite_token_hash BYTEA` from
`migrations/080_delegated_moderators.sql`: the same hash, the same column type,
and the same "shown once, then unrecoverable" contract. Reusing the established
shape rather than inventing a second one means there is one story about
secret-at-rest handling in this codebase instead of two.

The consequence that surprises users is intentional. Whoever reads a database
dump gets digests they cannot replay, and there is no code path capable of
leaking a plaintext token from that table because the plaintext is not in it.
"Show me that token again" is therefore *impossible* rather than merely
unimplemented, and the create response is the only place the secret ever
appears. Clients are likewise told never to persist it beyond the plugin's own
settings store. The remedy for a lost or exposed token is not recovery but
replacement: revoke and mint again, which costs seconds and affects nothing
else.

### Registered in three services, and why that is not belt-and-braces

This is the single most surprising fact about the design, and the main reason
this record is worth writing down at all.

The resolver is injected by a package-level setter, `SetAPITokenResolver`, and
called at startup in **three** services: auth-service, api-gateway **and
engagement-service**. A reader's first instinct — that a gateway is a
chokepoint, so validating there covers everything behind it — is exactly wrong
here, and option 4 above fails for the same reason.

`copyHeaders` in `services/api-gateway/handlers/proxy.go` strips hop-by-hop
headers plus `Cookie`, `Referer` and `Origin`, and forwards everything else
**verbatim** — including `Authorization`. It does not exchange the credential
for an internal one, and it does not annotate the request with a verified
identity. Each backend therefore re-validates the incoming bearer
**independently**, on its own terms. Registering the resolver only at the
gateway would leave engagement-service's `JWTAuthWithRevocation` receiving an
`allchat_pat_` string it has no resolver for, failing to parse it as a JWT, and
rejecting it. The observable result would be a gateway that cheerfully accepts
every PAT and an engagement service that **401s every single poll and
prediction action** — that is, precisely the feature the tokens were built for,
broken, while the credential appears to work.

The general rule this encodes: in this architecture, an authentication
mechanism must be registered in *every* service that terminates authentication
for a route it needs to reach. Adding a new backend that plugins will call
means wiring the resolver there too, and the failure mode if that is forgotten
is a confusing 401 rather than a helpful error.

### Authentication only, never an authorization bypass

A resolved PAT populates the same request-context keys a valid JWT does, so no
handler, ownership check or feature gate can tell the two apart. That is the
point: the token answers *who is calling*, and nothing else. Scope enforcement
(`RequireAPITokenScope`) is mounted **beside** the existing authorization
middleware and never in place of it. Scopes only ever *narrow* a token; they
cannot widen one, and a browser session passes through the scope middleware
untouched because it is not scope-limited at all.

The clearest illustration is the deliberate asymmetry in
`services/engagement-service/cmd/main.go`, where the route chain for starting a
round is scope check *then* `requireEngagementPremium`:

- `POST /overlays/:id/polls` and `POST /overlays/:id/predictions` **are**
  premium-gated. Opening a round posts to chat and consumes send quota, so it
  spends a resource the free tier does not include.
- `close`, `lock`, `resolve` and `cancel` are **deliberately ungated**. They
  spend nothing, and a streamer who has already opened a round must always be
  able to finish it — including after a subscription lapses mid-stream. Trapping
  a live poll open because billing changed would be a worse failure than
  refusing to start one.

So a perfectly valid, correctly scoped PAT belonging to a non-premium owner is
still refused by the premium gate, with a 403 rather than a 401. A test pins
this with a failure message that says so explicitly, because "the plugin 403s,
just drop the gate" is the plausible future mistake and it would convert this
authentication feature into an entitlement bypass.

The same principle produces two further refusals. Token management (create,
list, revoke) is session-only, so a leaked token cannot mint more tokens or
lock the owner out of revoking it. Account deletion and the PII data export
refuse a PAT at *any* scope, because these are surfaces where scopes are the
wrong instrument — there is no scope that should unlock irreversible account
destruction. Scopes bound what a PAT may do on surfaces it should reach;
outright refusal marks the surfaces no PAT should reach at all.

## Consequences

### Positive
- Non-browser clients become possible at all, which is the missing piece for
  every desktop, headless and CLI integration, not just the two plugins.
- The plugins are thin HTTP clients over endpoints that already existed, so the
  marginal cost of a new button is close to zero.
- A compromised token exposes its scopes on its owner's overlays, not the
  account: no token management, no deletion, no export, no admin surface.
- Revocation is a live column read on each request, so it takes effect within
  one request with no cache to invalidate.

### Negative
- A new long-lived credential type is new attack surface, and belongs in the
  next security review.
- The paste step remains a real leak vector for a population that streams its
  own screen. Bounded, not eliminated.
- Every service terminating authentication must register the resolver; forget it
  and the symptom is a 401 that looks like a bad token rather than a wiring bug.
- Two credential types (PAT and, later, paired device token) mean two things to
  keep correct behind one resolver seam.

### Neutral
- The resolver seam is shared: a paired device token becomes another row shape
  behind the same interface rather than a second authentication path.
- A PAT is strictly *stricter* than a JWT in one respect — the resolver requires
  the account not be banned on every request, whereas an issued JWT is
  backstopped only by its expiry.

## Related Decisions

- [ADR-0051](./0051-personal-access-tokens-for-non-browser-clients.md) — the
  canonical record of this decision, with the frozen contract, the management
  endpoints and the full refusal list. **Cite this one.**
- [ADR-0049 (paired device tokens)](./0049-desktop-control-surfaces-via-paired-device-tokens.md)
  — the PKCE loopback pairing flow for published plugins. A PAT is the
  hand-issued credential its first implementation step calls for, and the only
  thing that works where loopback cannot reach.
- [ADR-0048](./0048-delegated-overlay-moderators.md) — the source of the
  `invite_token_hash BYTEA` precedent that token storage mirrors.
- [ADR-0012](./0012-oauth-scope-minimisation.md) — the least-privilege stance
  scopes and the refusal list implement.
- [ADR-0008](./0008-feature-gate-infrastructure.md) — the gate infrastructure a
  PAT is forbidden from bypassing.
- [Stream Deck user guide](../guides/streamdeck.md) — the streamer-facing
  instructions for minting and using one.
