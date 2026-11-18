# H3 - Streamer/Admin cookie-based auth

Streamer and admin sessions authenticate with httpOnly cookies instead of tokens the page can read. Before this change the access and refresh JWTs lived in `localStorage`; any XSS read them silently. Now XSS has nothing to steal: the tokens are cookies the JS cannot reach, and the viewer token is the only credential that still lives in memory.

## Design

auth-service issues two cookies via `SetAuthCookies` (`shared/auth/cookie.go`, which also defines the names `access_token` and `refresh_token`, shared with the gateway reader):

| Cookie | Path | Lifetime |
|---|---|---|
| `access_token` | `/` | JWT expiry (`JWT_EXPIRY_HOURS`, default 24h) |
| `refresh_token` | `/api/v1/auth/` | 14 days (matches the Redis refresh-token TTL) |

Both are `HttpOnly: true`, `Secure: true`, `SameSite=Lax`. The access cookie rides every same-origin request; the refresh cookie is path-scoped so only the auth routes (`/refresh`, `/logout`, `/stop-impersonation`, `/auth/me` deletion) receive it.

The **api-gateway is the cookie boundary**. Backends never see the raw cookie: the gateway proxy strips `Cookie`, `Referer` and `Origin` from every proxied request (L17, `services/api-gateway/handlers/proxy.go`). Instead, gateway middleware reconstructs the Bearer header the backends already consume:

1. `CookieToBearer()` (`services/api-gateway/middleware/cookie_to_bearer.go`) copies the `access_token` cookie into `Authorization: Bearer <jwt>` when no header is present. It runs on the protected and admin route groups before `JWTAuthWithRevocation` (`services/api-gateway/cmd/main.go`), which validates the JWT and checks the revocation blacklist unchanged, and on the public source-add/bot-connect routes that need the caller identity without group auth. Non-browser clients that still send `Authorization: Bearer` keep working through the same path.
2. `AuthCookieForward()` (`services/api-gateway/middleware/auth_cookie_forward.go`) runs on the auth routes that need the raw tokens: the gateway strips `Cookie`, so this middleware copies the cookie values into `X-Access-Token` / `X-Refresh-Token` headers for `/auth/refresh`, `/auth/logout`, `/auth/stop-impersonation` and `/auth/me` deletion. auth-service reads those headers first and falls back to `Authorization: Bearer`.
3. `Set-Cookie` responses pass through the proxy back to the browser unchanged; the gateway itself never sets cookies.

### CSRF

`SameSite=Lax` blocks cross-site state-changing requests in modern browsers but is not complete. `shared/middleware/origin_check.go` (`OriginCheck`) additionally runs on the gateway's mutating routes, including the group-level middleware on the protected and admin groups: `Origin` (falling back to `Referer`) must be in the HTTP CORS allowlist from `CORS_ORIGIN` (`LoadHTTPAllowedOrigins` in `services/api-gateway/middleware/cors.go`; the separate `WEBSOCKET_ALLOWED_ORIGINS` is not used here). Requests with neither header are allowed, so non-browser API clients authenticating with a Bearer header are unaffected.

## Same-origin invariant

The cookies are host-only and `SameSite=Lax`, and the frontend fetches with `credentials: 'same-origin'` (`frontend/src/lib/api/client.ts`). Cookie auth only works when the frontend and the API gateway share an origin: the standard deployment serves the Next.js app and proxies `/api/*` from one ingress, with `NEXT_PUBLIC_API_URL` resolving to `window.location.origin`. Cross-origin deployments (frontend `:3000`, gateway `:8080` behind separate hosts) silently break login, which is why `client.ts` logs a dev-mode console warning when the API base origin differs from the window origin. The default dev setup proxies `/api` through Next.js so it stays same-origin.

If same-origin were ever dropped, cookie auth would need `credentials: 'include'`, `SameSite=None; Secure`, a strict CORS allowlist and a reworked CSRF strategy. None of that exists or is wanted.

## Auth flows

- **Login/exchange**: `POST /api/v1/auth/exchange` (the OAuth callback's single-use code) sets both cookies and returns only non-secret JSON (`user`, `redirect_to`). The callback page reads `redirect_to` from the body and validates it against the allowlist in `frontend/src/lib/auth/redirect-allowlist.ts` before navigating.
- **Refresh**: `POST /api/v1/auth/refresh` takes the refresh token from `X-Refresh-Token`, reuses the existing Redis reuse-detection (`GetDel` on `refresh_token:<hash>`; a miss is treated as theft and `ClearAuthCookies` clears both cookies). A serializer lock (`refresh_lock:<hash>`) turns concurrent refresh attempts into a retryable `409` instead of spurious reuse-theft 401s. On success both cookies rotate; on terminal provider rejection the cookies are cleared.
- **Logout**: `POST /api/v1/auth/logout` blacklists the access JWT (from `X-Access-Token`), deletes the Redis refresh entry and clears both cookies.
- **WebSocket**: the gateway WebSocket handshake accepts `Sec-WebSocket-Protocol: bearer.<token>` (viewer/overlay clients) as before, plus the ambient `access_token` cookie for same-tab owner connections (decoded in `services/api-gateway/handlers/websocket.go`). No cookie path exists for cross-origin overlay embeds, which is out of scope with the viewer client below.

## Impersonation

Because the page cannot read or swap cookies, admin-to-user impersonation is server-driven:

- `POST /api/v1/admin/users/:id/impersonate` (auth-service, admin-only, `services/auth-service/handlers/admin.go`): verifies the caller is a genuine admin (chained impersonation is rejected), issues a short-lived impersonation JWT (`IMPERSONATION_EXPIRY_HOURS`, default 2h, `shared/auth/jwt.go` impersonation claims), stashes the admin identity in Redis under `impersonation:<jti>` (TTL = the impersonation expiry), writes the DSGVO audit row, and sets the impersonated JWT as the `access_token` cookie. The admin's refresh cookie is untouched: refresh stays bound to the admin, so impersonation is access-only.
- `POST /api/v1/auth/stop-impersonation` (`services/auth-service/handlers/admin_impersonation.go`): reads the access token from `X-Access-Token`, requires an `impersonated_by` claim, consumes the stash with `GetDel` (single-use, so a stop is not replayable), blacklists the impersonation JWT, issues a fresh admin access JWT and sets it as the cookie.

The frontend (`frontend/src/lib/stores/auth-store.ts`) carries `isImpersonating` / `impersonatedUsername` UI state only; `startImpersonation` and `stopImpersonation` call the endpoints and refresh the user via `GET /auth/me` instead of token swapping. If an impersonated access JWT expires while impersonating, the refresh interceptor rotates back to a genuine admin token; the stale flag is reset on detection.

## Frontend

`frontend/src/lib/api/client.ts` sends only the ambient cookie (`credentials: 'same-origin'`). It attaches no `Authorization` header. On a 401 (other than the application-level `reauth_required` signal) it runs one cookie-based refresh, then retries the original request through the same client exactly once; a `409` from refresh means an in-flight refresh is rotating the cookie, so it waits briefly and retries. Failed refresh redirects to `/` with a full page reload rather than in-app navigation, so stores, caches and in-flight requests of the dead session are all discarded.

`frontend/src/lib/stores/viewer-auth-store.ts` and the viewer client keep the in-memory viewer token. The viewer overlay runs cross-origin inside OBS or another site, so a viewer cookie would be third-party and blocked by Safari ITP and Chrome's third-party-cookie deprecation. The viewer token is explicitly out of scope for cookie auth and remains small, short-lived and JS-readable; see `docs/security/RESIDUALS.md` for the residual risk.
