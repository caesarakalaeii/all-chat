# ADR-0074: The KLIPY App Key Is Served Only to Entitled Sessions

**Date**: 2026-10-17
**Status**: Accepted
**Revised**: 2026-11-15 (the alerts editor is free, so every signed-in user gets the key; the premium check and its 403 are gone)
**Deciders**: All-Chat Platform Team

## Context and Problem Statement

ADR-0073 made `GET /klipy-config` return the KLIPY app key to anyone, with no
authentication. One unauthenticated request was enough to scrape it, and
nothing tied that request to an All-Chat user. KLIPY still requires search
calls to come from the user's browser (no proxy without written approval), so
the key must reach the browser of the person using the GIF picker. The
question is which browsers.

## Decision Drivers

- Only users who can open the alerts editor need the key: since 2026-11-15
  that is every signed-in user (the `alerts_editor` gate is free, ADR-0070,
  ADR-0071). Until then it was users with premium or beta access.
- The dashboard authenticates with the `access_token` httpOnly cookie
  (same-origin), which the gateway turns into a bearer token
  (`CookieToBearer`) and validates against the JWT key chain plus revocation.
- The frontend pod holds no JWT secret, and user tokens carry no premium
  claim, so verifying the JWT locally would still need a database lookup.

## Considered Options

1. **Forward the caller's credentials to the gateway's
   `GET /api/v1/auth/me` (chosen).** The gateway validates the token,
   revocation included. (It also returns `is_premium` and
   `is_beta_tester`, which the route checked while the editor was premium.)
2. Verify the JWT in the route. This needs the JWT key chain in the frontend
   pod (a new secret there) and still has no premium state.
3. Ship the key only in pages that already require login. The editor is a
   client component and the key would still be fetchable; it moves the
   problem instead of checking it.

## Decision Outcome

Option 1. `/klipy-config` reads the `Cookie` and `Authorization` request
headers, forwards only those to `${API_GATEWAY_URL}/api/v1/auth/me` (5 s
timeout), and answers:

- `401 {apiKey: ""}` without credentials, when the gateway rejects them, or
  when the check fails for any other reason (fail closed);
- `200 {apiKey}` for any signed-in user, with an empty key when
  `KLIPY_API_KEY` is unset.

The route answered `403` to users without premium or beta access until the
editor became free on 2026-11-15.

Every answer has `Cache-Control: private, no-store` and
`Vary: Cookie, Authorization`, so no CDN or ingress cache replays one user's
response to another. The picker (`frontend/src/lib/klipy.ts`) fetches the
route with `credentials: 'same-origin'`, like the API client; any non-2xx
leaves the editor's "GIF search unavailable".

In the cluster (caesar-deployment), `frontend` sets `API_GATEWAY_URL` to
`api-gateway` and `frontend-beta` to `api-gateway-beta`, and both gateways'
NetworkPolicies admit ingress from the two frontend deployments.

### Consequences

- Anonymous callers cannot read the key from All-Chat. A signed-in user can
  still read it from the search URLs their own browser
  sends; that is inherent to browser-side calls, and KLIPY's partner panel
  limits its use.
- Each picker load costs one `/auth/me` call. The gateway's global rate
  limiter keys that call on the frontend pod's IP, because it runs before
  authentication; at one call per editor load this stays far below the
  300/min default.
- The frontend pods now reach the gateways in-cluster, which NetworkPolicy
  previously did not allow.

## Related Decisions

- ADR-0073: runtime key delivery; this record restricts who receives it.
- ADR-0071: browser-side KLIPY search.
- ADR-0070 / ADR-0008: the `alerts_editor` gate (free since 2026-11-15).
