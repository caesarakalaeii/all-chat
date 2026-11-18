# ADR-0049: Stream Deck and Desktop Control Surfaces via Paired Device Tokens

**Date**: 2026-08-14
**Status**: Accepted
**Revised**: 2026-11-18
**Deciders**: caesarakalaeii

## Context

A streamer asked for Stream Deck buttons driving All-Chat: start or close a
poll or prediction, send a canned message to every connected chat. A physical
button fanning a message out to Twitch, YouTube and Kick at once is the
shortest path between intent and three platforms. (All-Chat *reads* five
platforms but posts to three: `POST /api/v1/auth/chat/send` answers 501 for
TikTok and 400 for Discord; the list is pinned by `scripts/check-plugin-parity.py`.)

Targeting Stream Deck hardware costs nothing, and a plugin is ordinary local
software on Windows/macOS. Elgato's app does not run on Linux, so Linux
streamers use **OpenDeck** (loads Elgato-SDK plugins) or **StreamController**
(Python, its own plugin format, needing a separate second plugin). The pairing
mechanism and endpoints are front-end agnostic; only the plugin is not.

The actions need no new backend: poll, prediction and send endpoints already
exist through the gateway. The whole work is authentication: every caller today
is a browser (dashboard cookie or extension JWT handoff), and the session JWT
lives 24 hours (`shared/auth/jwt.go`), far too short for a paired device. A
plugin is published software on machines we do not control, so it is an
untrusted public client and cannot hold a client secret.

## Decision

**All-Chat becomes the authorization server for a scoped, long-lived,
revocable device token** (scope `engagement:write` + `chat:write`), delivered
by a **PKCE loopback redirect** (`http://127.0.0.1` / `[::1]`, any port, one
fixed path — never `localhost`, which resolves through DNS; never routed
through the general user-facing redirect guard). The streamer clicks Approve
once; nothing is typed or pasted. A **pairing code fallback** (8-char
`XXXX-XXXX`, hashed, 10-minute TTL, 5 attempts per request) covers the case
browser and plugin cannot reach each other: a second PC or headless box. Both
paths mint the same token, so this is one credential with two delivery paths.

Scope of the token: bound to one user and one overlay, rejected on any route
outside its scopes, never satisfying the premium gate on its own (a device
cannot route around `requireEngagementPremium`), independently revocable, with
a sliding expiry so an abandoned pairing lapses. On chat send (no overlay
dimension) the scope set is the limit — stated in the middleware, the migration
comment and the approve screen copy rather than implying a stronger guarantee
than exists.

Two plugins ship, because no single format reaches all three apps:
**StreamController** (Python, the primary target, dogfooded daily) and the
Elgato Stream Deck app (TypeScript, published free to the Marketplace); the
licence question (AGPL host loaded into a GPL-3.0 host, no conflict expected)
is a v1 gate before the first submission. The action list (name, endpoint,
payload, failure surface) is the keep-in-sync contract both implement.

## Implementation

Decision built (issue #737):

- `device_tokens` (migration 088): `allchat_dev_` prefix, 256-bit secret,
  SHA-256 at rest, 90-day sliding expiry. Shares the resolver seam with PATs
  (ADR-0051): `IsAPIToken` recognizes both prefixes, one authentication path
  with two row shapes, wired in auth-service, api-gateway and
  engagement-service.
- Loopback redirect validator: `services/auth-service/handlers/loopback_redirect.go`,
  userinfo/query/fragment refused; PKCE is S256 only (plain is refused
  outright since a plain challenge *is* the verifier); `ConsumeAuthCode` is a
  single atomic UPDATE, and replaying a code also revokes the token it minted.
- `device_link_requests` table carries both delivery paths, one approve screen
  (`/link`) and one exchange endpoint. Rate limiting at the gateway on the two
  unauthenticated routes.
- `desktop_control_surfaces` gate (migration 089) on the pairing endpoint,
  seeded `is_premium = FALSE` to match the plugins' advertised price; the
  onboarding entry lives in `OnboardingChecklist.tsx`, not `/upgrade`.
- `/me/devices*` and the loopback callback are session-only surfaces (no
  token-authenticated requests, no impersonating admin), so a compromised
  control surface cannot mint or revoke tokens.

## Consequences

- Every future control surface (Touch Portal, CLI, phone remote) reuses the
  mechanism; All-Chat gains its first non-browser credential type.
- New long-lived credential type, in scope for the next security review (the
  brute-force bound, the loopback validator, the replay window).
- Two linking paths, twice the work per future button, and the rarely used
  fallback rots unless tested deliberately.

## Related Decisions

- ADR-0051: the hand-issued PAT this ADR's original spike step calls for, and
  the credential that works where loopback cannot (CLI, headless box).
- ADR-0008 (feature gates), ADR-0012 (least privilege), ADR-0051 (token
  infrastructure shared), [Stream Deck user guide](../guides/streamdeck.md).
