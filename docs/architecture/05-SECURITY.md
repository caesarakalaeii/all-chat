# Security Architecture

What actually protects traffic and data today, and where each mechanism lives
in code.

## Authentication

- Streamer and viewer login is OAuth 2.0 per platform (Twitch, YouTube, Kick,
  Discord, Facebook, Instagram, Picarto; see `services/auth-service/oauth/`).
  OAuth scopes are minimized per flow (ADR-0012): login requests only
  event-reading scopes (e.g. Twitch: `channel:read:redemptions`,
  `channel:read:subscriptions`, `bits:read`, `moderator:read:followers`);
  chat scopes (`user:read:chat`, `user:bot`, `channel:bot`, and the send scope
  `user:write:chat`) are added only in the add-source/moderation flows.
- Session JWTs: HMAC-SHA256, 24h expiry (`shared/auth/jwt.go`). Middleware is
  `shared/middleware.JWTAuth` plus the revocation-checking variant; keys
  dispatch by the token `kid` (KeyChain) with a legacy-secret fallback.
- Tokens live in httpOnly, Secure, SameSite=Lax cookies issued by auth-service
  (`shared/auth/cookie.go`), not in JSON bodies. Refresh-token reuse is
  detected in Redis and triggers 401 plus cookie clearing.
- Personal API tokens (`api_tokens` table, migration 086) authenticate
  non-browser clients (`shared/middleware/apitoken.go`).
- WebSocket auth takes the token from `Sec-WebSocket-Protocol: bearer.<token>`
  (preferred), then `?token=` (compat), then the access cookie
  (`services/api-gateway/handlers/websocket.go`).

## Authorization

- Ownership checks per service: overlay CRUD against `overlays.user_id`, share
  links against accepted `share_requests`.
- Premium and early-access gating: `feature_gates` table plus
  `RequirePremium`/`RequireEarlyAccess` middleware (ADR-0008).
- Admin surfaces need an admin-claim JWT; impersonation issues a short-lived
  token with rows in `impersonation_audit_log`
  (`services/auth-service/handlers/admin_impersonation.go`).

## Encryption

- OAuth tokens and platform credentials at rest: AES-256-GCM, versioned via
  `shared/encryption/versioned.go` (`MultiKeyEncryptor`: 1-byte kid + 12-byte
  nonce + GCM tag). Keys come from `TOKEN_ENCRYPTION_KEY_V<n>`-style env vars;
  rotation tooling exists in `services/auth-service/cmd/key-rotator/`, and a
  legacy plaintext-to-encrypted backfill job in
  `services/youtube-listener/cmd/token_backfill/`.
- Discord relay webhook URLs use the same scheme on discord-listener
  (ADR-0091) and are never returned by the API.
- In transit: TLS at ingress. Service-to-service traffic inside the cluster is
  plain HTTP; isolation is NetworkPolicies. `shared/signing/` (HMAC request
  signing, replay-protected) exists but no production call path uses it; see
  residual L2 in `docs/security/RESIDUALS.md`.

## Network

- Namespace `allchat`: default-deny all ingress/egress plus explicit allows
  for DNS, internet egress (except 169.254.169.254 metadata), intra-namespace
  traffic, external ingress only to api-gateway, and monitoring-namespace
  scrapes. The full list is `deployments/k8s/base/networkpolicies.yaml`.
- CORS is allowlist-driven via `CORS_ORIGINS`
  (`shared/middleware/cors.go`); a literal `*` is ignored when credentials are
  enabled.

## Data protection

- Retention periods and enforcement live in `docs/security/SECURITY_AUDIT_REPORT.md`
  and the privacy policy (`frontend/src/lib/i18n/messages/en/legal.ts`,
  section 6); the auth-service retention-sweeper
  (`services/auth-service/cmd/retention-sweeper/`, 15-minute CronJob, ADR-0089)
  deletes on schedule. Highlights: viewer message history 1 hour, alert events
  and quota audit logs 30 days, moderation actions 90 days, expired legacy
  tokens 7 days.
- Client IPs are anonymized before logging (`shared/middleware.AnonymizeIP`,
  `shared/middleware/ipanon.go`); gin services use
  `middleware.GinAccessLog()` rather than the stock logger, and the edge access
  log drops client addresses entirely.
- GDPR rights: account data export (`GET /auth/me/data-export`, viewer
  equivalent `GET /auth/viewer/data-export`) and hard delete via
  `DELETE /auth/me` with cascades.

## API hardening

- Rate limiting: Redis-backed per-identity bucket limiter
  (`shared/ratelimit/`), one-minute windows, anonymized-IP keys.
- SQL is parameterized through pgx throughout; handlers validate request
  shapes at the binding step.
- Scanning in CI: govulncheck per Go module (fails on reachable vulns) plus
  npm audit, `.github/workflows/security-scan.yml` (S1 in
  `docs/security/SECURITY_AUDIT_REPORT.md`).

## Threats and known gaps

Audit trail and residual risk live in `docs/security/SECURITY_AUDIT_REPORT.md`
and `docs/security/RESIDUALS.md`; these docs own that content and stay
authoritative. Notable open items: s2s call paths are unsigned (L2), a full
WAF is not deployed, and rate limiting granularity stops at IP/bucket level.
