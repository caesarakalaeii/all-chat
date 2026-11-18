# All-Chat Security Audit Report

Historical snapshot: audited 2026-06-23, re-checked 2026-07-15. Every
finding below was remediated in code; what remains open is tracked in
[RESIDUALS.md](./RESIDUALS.md). `/github/workflows/security-scan.yml`
references this path, so the file is retained as the audit record.

- **Method:** 8 parallel reviewer subagents (auth, crypto, config/infra,
  injection, gateway, listeners, deps, frontend) + manual `govulncheck` on
  api-gateway, auth-service and shared.
- **Scope:** `services/*`, `shared/*`, `frontend/`, `deployments/k8s/`,
  Dockerfiles, docker-compose.
- **Result:** 0 reachable vulnerabilities in the three critical modules at
  audit time; the findings below were all fixed (H=high, M=medium, L=low,
  S=supply chain).

## Findings

### High

- **H1**: WebSocket ReadLimit never called, unbounded message buffer on
  `/ws/*` routes. Fixed by `SetReadLimit` in `readPump`.
- **H2**: logout blacklist written but never read, tokens unrevocable for
  24h. Fixed: `JWTAuthWithRevocation` checks the Redis blacklist everywhere.
- **H3**: JWT and refresh token in `localStorage` (XSS to account takeover).
  Fixed by the cookie migration: access/refresh JWTs live in
  `httpOnly; Secure; SameSite=Lax` cookies (`shared/auth/cookie.go`),
  `/exchange` and `/refresh` set and rotate them, the gateway is the cookie
  boundary (`CookieToBearer`, `AuthCookieForward`, `AuthCookieForward` for
  auth routes).
- **H4**: viewer JWT leaked via `postMessage(..., '*')` on the
  `chat/auth-success` popup. Fixed with explicit target origins.
- **H5**: JWT accepted via `?token=` query parameter and printed into access
  logs. Gateway now reads the token via `Sec-WebSocket-Protocol`; the
  logging middleware redacts the token.
- **H6**: no k8s `securityContext` on any pod. Fixed in
  `deployments/k8s/base/**`.
- **H7**: Redis unauthenticated and no NetworkPolicy. Fixed: Redis secret
  plus default-deny NetworkPolicy.
- **H8**: insecure `DATABASE_PASSWORD` dev fallback in ~16 services,
  `getEnvOrDefault`-style. Fail-fast when the secrets are missing.
- **H9**: CORS wildcard accepted alongside `AllowCredentials`. Fixed via
  `CORSFromEnv` fail-fast.

### Medium

- **M1**: streamer OAuth refresh token in the URL fragment; now short-lived
  `?code=` exchanged via Redis.
- **M2**: refresh-token rotation family tracking; partially landed, rest
  deferred (RESIDUALS).
- **M3**: JWT secret minimum length now enforced (32 bytes).
- **M4/M5**: `shared/signing` future-timestamp replay and unsigned
  query/service-name, fixed in place; the package is still not wired into
  any prod service (see RESIDUALS L2).
- **M6**: `/metrics` unauthenticated.
- **M7**: no per-endpoint rate limit on auth login/refresh/exchange.
- **M8**: viewer WS accepted all origins. Fixed with origin allowlist.
- **M9**: EventSub webhook read unbounded body before HMAC verify; now
  capped.
- **M10**: no CSP/security headers; fixed in `frontend/next.config.js`
  `headers()`, tiered frame-ancestors (locked overlays, open display
  widgets).
- **M11**: embed postMessage listeners skipped the `event.origin` check.
- **M12**: open redirect via unvalidated `redirect_to` in the auth callback.
- **M13**: mutable `:latest`/`:main` image tags.
- **M14**: no TLS termination in k8s manifests.
- **M15**: real secrets in local `.env` (rotated; do not commit any
  portion).
- **M16**: docker-compose binding internal ports with weak dev defaults;
  bound to localhost where possible.
- **M17**: TTS token plus spoken text in the URL query; now `Authorization`
  header plus JSON body.

### Low (defense in depth)

Auth/crypto: empty signing secret accepted (L1); signing not wired into any
prod service (L2, still open); no JWT issuer/audience validation (L3); no
PKCE for Twitch/YouTube OAuth (L4, PKCE landed during hardening); OAuth
state TOCTOU (L5); CSRF token logged (L6); weak/low-entropy key not warned
(L7/L8); dead weak JWT default in overlay-manager (L9). Gateway: badge query
injection (L10), error details in 502 (L11), CORS env var name mismatch
(L12), OBS-mode unauth WS to any active overlay (L13, accepted), internal
route in protected group (L14), public TTS route (L15), no HSTS at gateway
(L16), proxy copies cookies to backend (L17). Injection paths: string-built
table name in admin cosmetics (L18), Sprintf-built JSON from Twitch input
(L19). Listeners: Discord webhook URL logged (L20), no IRC reconnect jitter
(L21), unbounded InnerTube walk (L22), unbounded `io.ReadAll` in
YouTube/InnerTube clients (L23), unvalidated TikTok username (L24), missing
HTTP client timeouts (L25). Config/infra: hardcoded public InnerTube key
(L26), sparse `.dockerignore` (L27), unpinned base images (L28), default
service accounts (L29), weak vault fallbacks (L30). Frontend: gradient CSS
injection (L31), unvalidated `auth_url` redirects (L32),
`dangerouslySetInnerHTML` on owner CSS (L33).

### Supply chain

- **S1**: no `govulncheck`/`npm audit` in CI. `x/net`, `x/crypto`, `x/sys`
  bumps were needed at audit time.
- **S2**: version skew across module pins.
- **S3**: `gorilla/websocket` pseudo-version, blocked on an upstream tag.
- **S4**: stray nested `frontend/frontend/package.json`.

## Verified strengths (kept, for context)

JWT HMAC-pinned KeyFunc (no alg confusion) plus `exp`/`nbf` validation and
multi-key rotation; AES-256-GCM with fresh nonces, verified tag, and
versioned kid rotation; constant-time signature comparisons throughout; pgx
parameterization with no exploitable SQLi; OAuth tokens encrypted at rest;
EventSub webhook HMAC constant-time with skew and Redis dedup; no
`InsecureSkipVerify` anywhere; IDOR guarded by ownership checks in
overlay-manager; admin routes behind `AdminOnly`, `/internal` behind
`ServiceJWTAuth`; gateway has `BodyLimit` and server timeouts; frontend chat
is React-escaped with an image allowlist and no `eval`/`document.write`.

## Fix order used

1 H1, then 2 H6+H7 (infra blast radius), 3 H8, 4 H2, 5 H3+H4+H5 (frontend
auth chain), 6 H9+L12 (CORS), 7 M4+M5+L1 (signing), 8 M9+M6/M7 (DoS), 9
M10-M12 (frontend CSP/postMessage/redirect), 10 S1 plus bumps, 11 remainder.
