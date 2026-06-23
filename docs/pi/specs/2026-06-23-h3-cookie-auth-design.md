# H3 — Streamer/Admin Cookie-Based Auth Migration

**Date:** 2026-06-23
**Status:** Approved (pending reviewer pass)
**Audit ref:** SECURITY_AUDIT_REPORT.md H3
**Branch target:** `security/audit-hardening`

---

## Problem

H3 (audit): JWT + refresh token stored in browser `localStorage`. Any XSS reads the admin JWT + impersonation tokens + refresh token silently → account takeover. H3 **minimum** was applied (refresh token moved to in-memory, access JWT left in localStorage). This spec covers the **full** fix: httpOnly; Secure; SameSite cookies set by the backend, with the frontend no longer attaching `Authorization` headers.

## Goal

Eliminate JS-readable long-lived streamer/admin auth tokens. Access + refresh JWTs live in httpOnly cookies the browser sends automatically to same-origin requests. XSS can no longer exfiltrate tokens (no API to read them; refresh token never leaves the cookie scope).

## Non-goals (out of scope)

- **Viewer token** migration. Cross-origin overlay embeds (`/overlay/[id]` iframed by OBS/external) need viewer-JWT, which as a third-party cookie is blocked by Safari ITP + Chrome's third-party-cookie phaseout. Viewer token stays as-is (in-memory). Documented in `RESIDUALS.md`.
- **WebSocket auth.** Stays on `Sec-WebSocket-Protocol: bearer.<token>` (H5, already done). Gateway cookie path for WS not added (subprotocol is cross-origin-safe, sufficient).
- **Signing (L2)** wiring into prod. Separate deferral.

## Architecture

The **api-gateway is the cookie boundary.** It reads the httpOnly cookie, validates the JWT, and sets `Authorization: Bearer <jwt>` on the proxied request. Backends are unchanged.

```
Browser
  │ Cookie: access_token (httpOnly, Secure, SameSite=Lax, Path=/)
  │ Cookie: refresh_token (httpOnly, Secure, SameSite=Lax, Path=/api/v1/auth/refresh)
  ▼
api-gateway JWTAuth
  ├─ read cookie (fallback: Authorization: Bearer — backward compat)
  ├─ validate + blacklist check (JWTAuthWithRevocation pattern)
  ├─ normalize → c.Request.Header["Authorization"] = "Bearer <jwt>"
  └─ proxy.ForwardRequest
      └─ L17 strip (Cookie/Referer/Origin dropped before forwarding) ──▶ backend
                                                                          └─ JWTAuthWithRevocation (reads Authorization, unchanged)
```

**Why gateway as boundary:** auth-service and the 4 user-facing backends already read `Authorization: Bearer`. Putting cookie-reading in the gateway preserves L17 (proxy strips `Cookie`/`Referer`/`Origin` — added in the prior commit) so backends never receive the raw cookie, and lets all backends keep their existing `JWTAuthWithRevocation` middleware unchanged. Single chokepoint.

## Cookies

Issued by auth-service via `Set-Cookie` (response headers pass through the gateway proxy unchanged). Both cookies:

- `HttpOnly: true` — JS cannot read.
- `Secure: true` — HTTPS only (TLS ingress exists per M14).
- `SameSite=Lax` — sent on same-site navigations + same-origin fetch; blocks cross-site POST/PUT/DELETE in modern browsers.

| Cookie | Path | Lifetime | Notes |
|---|---|---|---|
| `access_token` | `/` | JWT exp (hours, env `JWT_EXPIRY_HOURS`) | Sent to every same-origin request. |
| `refresh_token` | `/api/v1/auth/refresh` | 14d (matches Redis refresh-token TTL) | Path-scoped: only sent to the refresh endpoint, minimizing exposure surface. |

Cookie names (`access_token`, `refresh_token`) are defined as constants in `shared/auth/cookie.go` (new file) so auth-service (issuer) and gateway (reader) share one source of truth.

## CSRF defense: SameSite=Lax + Origin check

`SameSite=Lax` blocks cross-site state-changing requests in modern browsers but is not complete (subdomain cookie injection, legacy browsers). Additional layer:

**Origin-check middleware** (gateway, on POST/PUT/DELETE/PATCH):
- Read `Origin` header (fallback `Referer` if `Origin` absent).
- If present: must be in the `CORS_ORIGINS` allowlist (already loaded by `loadAllowedOrigins`). Else `403 Forbidden`.
- If **both absent**: allow (non-browser API clients authenticate via `Authorization: Bearer` fallback — those carry no `Origin`/`Referer` and rely on the token).
- Stateless, ~40 lines, OWASP-recommended modern pattern. No frontend change.

## Issuance (auth-service)

### `POST /api/v1/auth/exchange` (streamer login completion — M1 code exchange)
Current: returns JSON `{access_token, refresh_token, expires_in, token_type, redirect_to, ...}`.
Change:
- Build access JWT + refresh token as today.
- `Set-Cookie: access_token=...; HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=<jwtexp>`
- `Set-Cookie: refresh_token=...; HttpOnly; Secure; SameSite=Lax; Path=/api/v1/auth/refresh; Max-Age=1209600`
- Store refresh-token hash in Redis (M2 reuse detection — unchanged).
- JSON body: `{expires_in, token_type, redirect_to, user:{id,username,display_name,is_admin}, source_added?, moderation_enabled?}`. **No tokens in body.**

### `POST /api/v1/auth/refresh`
Current: reads `refresh_token` from JSON body; returns `{access_token, refresh_token, expires_in}`.
Change:
- Read refresh token from **cookie** `refresh_token` (fallback: JSON body `{refresh_token}` — backward compat during rollout, deprecated).
- Reuse-detection via `GetDel` on `refresh_token:<hash>` Redis key (unchanged).
- On success: rotate both cookies (new access + new refresh `Set-Cookie`), update Redis hash, return `{expires_in, token_type, user:...}` only.
- On failure: `401`; **clear both cookies** (`Max-Age=0`).

### `POST /api/v1/auth/logout`
Current: reads `Authorization: Bearer`, blacklists token in Redis.
Change:
- Read token from cookie (fallback: Authorization).
- Blacklist access JWT (`blacklist:<token>`, TTL = remaining JWT exp) — unchanged.
- `Set-Cookie: access_token=; Max-Age=0; Path=/` + `Set-Cookie: refresh_token=; Max-Age=0; Path=/api/v1/auth/refresh`.
- Delete refresh-token Redis key (revoke the family entry).

## Gateway (api-gateway)

### `middleware/auth.go` — `JWTAuth`
Current: reads `Authorization: Bearer` only.
Change: token resolution precedence:
1. `access_token` cookie.
2. `Authorization: Bearer` header (backward compat).
3. (No token → anonymous / OBS-mode paths as today.)

On success: set `c.Request.Header.Set("Authorization", "Bearer "+token)` so backends see a normalized Bearer header (unchanged contract). On failure: 401 as today.

### CSRF Origin-check middleware
New `shared/middleware/origin_check.go` (`OriginCheck(allowedOrigins []string)`), wired on the gateway for mutating methods. Pure-stateless. Shared so it can be unit-tested in isolation.

### Cookie passthrough
The gateway proxy already copies response headers (including `Set-Cookie`) back to the client. Verify no existing middleware drops `Set-Cookie` (the L17 strip is on the **request** to backends, not the response — confirm during implementation). The gateway itself does not issue `Set-Cookie`; auth-service does.

## Frontend

### Fetch wrappers — remove token attachment
Files: `src/lib/api/client.ts`, `overlays.ts`, `chat.ts`, `moderation.ts` (uses `apiClient`).
- Remove `Authorization: Bearer` header construction + `localStorage.getItem('jwt_token')` reads.
- Remove the in-memory token store reads (`inMemoryTokens.getAccessToken()` — H3 minimum, now vestigial).
- Same-origin fetch sends the access cookie automatically. `credentials: 'same-origin'` (Next default) sufficient.
- Keep the `401`-handler hook (see interceptor below).

### 401 → refresh → retry interceptor
In `apiClient` (the shared fetch wrapper `client.ts`):
- On `401` from a non-`/auth/refresh` request: call `POST /api/v1/auth/refresh` (cookie-based, no body needed), then retry the original request **once**.
- If refresh fails (`401`/`403`): redirect to `/login` (or streamer login entry).
- Guard against infinite loops (refresh-of-refresh).

### `auth-store.ts`
- Stop persisting `jwt_token` to localStorage. Remove `jwt_token` reads.
- Derive login state by calling an authed endpoint (`GET /api/v1/auth/me` if exists, else `/api/v1/auth/verify` — confirm during impl) that succeeds when the access cookie is valid. Store only non-secret user profile (id/username/is_admin) in the store, not the token.
- Remove the in-memory token store module (`src/lib/auth/in-memory-store.ts`) — vestigial after cookies land.

### `viewer-auth-store.ts` + viewer api client
**Unchanged** (viewer token out of scope). The viewer flow keeps its in-memory token.

### OAuth callback (`src/app/auth/callback/page.tsx`)
- After `POST /exchange` (now cookie-setting), read `redirect_to` + user from the **JSON body** (no tokens in body).
- `router.push(redirect_to)` with M12 validation (`startsWith('/')` + `!startsWith('//')`) — unchanged.

## Testing

### Unit
- auth-service: `HandleRefresh` reads from cookie; `HandleExchange` sets cookies; `HandleLogout` clears cookies. Use `httptest.NewRecorder` + assert `Set-Cookie` headers. Mock Redis (miniredis).
- gateway `JWTAuth`: cookie-first resolution; Authorization fallback; both-absent path. Existing tests extended.
- `shared/middleware/origin_check.go`: allow/absent/deny cases.

### Integration
- E2E cookie round-trip: login → cookie set → authed request → 401 → refresh → retry → logout → cookies cleared. (Existing E2E harness if present; else document as manual.)

### Manual smoke
- Browser DevTools: confirm `access_token`/`refresh_token` are httpOnly (Application → Cookies → HttpOnly column checked). Confirm `localStorage` has no tokens. Confirm an XSS probe (`<script>document.cookie</script>` in an overlay) returns empty for auth cookies.

## Rollout

- Backward compat during rollout: gateway accepts `Authorization: Bearer` fallback; auth-service `/refresh` accepts body fallback. Old frontend builds keep working until the new build is deployed.
- After new frontend is live + stable: remove the fallbacks (separate cleanup commit), making cookie the only path.

## Decomposition (implementation plan inputs)

Three file-disjoint domains for parallel agents:

1. **auth-service** — issuance (`/exchange`, `/refresh`), `/logout` cookie clear, reuse-detection unchanged. Files: `services/auth-service/handlers/auth_handler.go`, `viewer_auth.go` (streamer payload helper stays), `cmd/main.go` (no route change).
2. **gateway** — `middleware/auth.go` cookie-first; new `shared/middleware/origin_check.go` + wire on mutating routes; verify `Set-Cookie` passthrough. Files: `services/api-gateway/middleware/auth.go`, `cmd/main.go`, `shared/middleware/origin_check.go` (+test).
3. **frontend** — `client.ts`/`overlays.ts`/`chat.ts`/`moderation.ts` remove token attach; 401-interceptor; `auth-store.ts` cookie-derive + remove in-memory store; callback reads body only. Files under `frontend/src/`.

## Open questions for implementation

- Confirm the authed-session-check endpoint name (`/auth/me` vs `/auth/verify` vs existing) during impl step 3.
- Confirm no existing gateway middleware strips `Set-Cookie` from backend responses during impl step 2.
