# ADR-0049: Personal Access Tokens

**Date**: 2026-08-18
**Status**: Accepted, recorded as ADR-0051
**Revised**: 2026-11-18
**Deciders**: caesarakalaeii

> **Read this first.** The decision described here is real, is built, and is
> live. Its canonical record is **ADR-0051, "Personal Access Tokens for
> Non-Browser Clients"**, and that is the number to cite in code comments and
> commit messages: a growing minority of non-documentation files say "ADR-0049"
> when they really mean the PAT, so readers need this note to know which of the
> two they landed on. This file exists because the work was planned under the
> number 0049 while 0049, 0050 and 0051 were landing in parallel. Where the two
> ever disagree, ADR-0051 is right and this file is stale.
>
> Note also that **0049 already names a different credential**:
> [ADR-0049, "Stream Deck and Desktop Control Surfaces via Paired Device
> Tokens"](./0049-desktop-control-surfaces-via-paired-device-tokens.md) is the
> PKCE loopback *pairing* flow. ADR numbering is shared with the
> `caesar-deployment` repository (see the [README](./README.md)), which is how
> the collision arose. Paired device tokens and personal access tokens solve
> adjacent halves of the same problem, and confusing them is the most likely way
> to misread either document.

## Context

Every authenticated caller at this point was a browser: the dashboard carries a
cookie, and the browser extension is handed a JWT by `postMessage` from an
allowlisted platform origin. A Stream Deck plugin, a stream-side CLI or a
headless capture box is none of those things, and the session JWT is unusable
for them on two counts: there is no browser context to obtain one, and it lives
24 hours (`shared/auth/jwt.go`). The actions themselves needed no new backend;
poll, prediction and send endpoints already existed and were proxied. The whole
problem was credentials.

## Decision

**A long-lived personal access token, presented as
`Authorization: Bearer allchat_pat_<secret>`, as an additional authentication
path that does not replace the pairing flow.** See [ADR-0051](./0051-personal-access-tokens-for-non-browser-clients.md)
for the frozen contract (prefix, `api_tokens` table, endpoints, scopes), the
refusal list, and the consequences. Two properties of the design are worth
restating here because they hold for every future credential type:

- **Stored as a hash, shown exactly once.** `api_tokens.token_hash` (migration
  086) holds a SHA-256 digest in `BYTEA`, mirroring
  `overlay_moderators.invite_token_hash` (migration 080): the same hash, the
  same column type, the same shown-once contract. "Show me that token again"
  is impossible rather than unimplemented, and the remedy for a lost or exposed
  token is revoke and re-mint.
- **Registered in three services, deliberately.** The resolver (`SetAPITokenResolver`)
  is called at startup in auth-service, api-gateway **and** engagement-service,
  because the gateway's `copyHeaders` forwards `Authorization` verbatim and each
  backend re-validates independently. Gateway-only wiring would leave
  engagement-service rejecting every PAT. The general rule: an authentication
  mechanism must be registered in every service that terminates authentication
  for a route it needs to reach; the forgotten wire manifests as a confusing
  401, not a wiring error.

A resolved PAT populates the same request-context keys a valid JWT does, so no
handler, ownership check or feature gate can tell the two apart: token
management, admin surfaces, account deletion and the PII export all refuse a
PAT outright, and `RequireAPITokenScope` sits beside the existing
authorization middleware, never in place of it.

## Related Decisions

- [ADR-0051](./0051-personal-access-tokens-for-non-browser-clients.md) — the
  canonical record; cite this one.
- [ADR-0049 (paired device tokens)](./0049-desktop-control-surfaces-via-paired-device-tokens.md)
  — the PKCE loopback pairing flow; a PAT is the hand-issued credential its
  spike step calls for and the only thing that works where loopback cannot.
- ADR-0048 — the source of the `invite_token_hash BYTEA` precedent.
- ADR-0012 (least privilege), ADR-0008 (gates a PAT must not bypass).
- [Stream Deck user guide](../guides/streamdeck.md) — how one is minted and used.
