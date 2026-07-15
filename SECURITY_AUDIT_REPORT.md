# All-Chat Security Audit Report

> **Historical status (as of 2026-07-15):** This is a point-in-time snapshot (audited 2026-06-23, re-checked 2026-07-15), not a live status tracker. Every finding below has since been remediated in code and this document is retained as a historical record. For the current residual-risk register, see [RESIDUALS.md](./RESIDUALS.md).

**Date:** 2026-06-23
**Method:** 8 parallel `reviewer` subagents (auth, crypto, config/infra, injection, gateway, listeners, deps, frontend) + manual `govulncheck` on 3 critical modules.
**Scope:** `services/*` (20 svcs), `shared/*` (19 pkgs), `frontend/`, `deployments/k8s/`, Dockerfiles, docker-compose.

---

## Executive Summary

Auth/crypto core is solid: JWT is HMAC-pinned with multi-key rotation + chain isolation; AES-GCM encryption-at-rest uses fresh random nonces + verified tags + correct versioned rotation; pgx parameterization is consistent (no SQLi/command/path/template injection); OAuth tokens encrypted at rest; EventSub webhook HMAC is constant-time with skew + dedup; chat rendering is XSS-safe (React escaping, image allowlist, no eval). `govulncheck` on api-gateway/auth-service/shared → **0 reachable vulns**.

The dominant residual risk is **infra/config hardening** (zero k8s `securityContext`, no NetworkPolicy, open unauth Redis, mutable `:latest` tags, no TLS ingress) plus a few **auth/session gaps** (logout blacklist never checked, JWT+refresh-token in localStorage, viewer-JWT leaked via wildcard `postMessage`, JWT in WS URL query logged in access logs) and one **gateway DoS** (WS `SetReadLimit` never called).

---

## CRITICAL / HIGH (fix first)

### H1 — WebSocket ReadLimit never enforced → OOM DoS
- **Loc:** `services/api-gateway/websocket/connection.go:31-42` (const defined) vs `readPump` `connection.go:172-194`
- **Evidence:** `MaxMessageSize` defined but `c.conn.SetReadLimit(...)` never called anywhere in `services/api-gateway/` (grep 0 hits). Any WS client (incl. anonymous viewer `/ws/chat/:streamer`) can buffer unbounded messages.
- **Fix:** Add `c.conn.SetReadLimit(MaxMessageSize)` at top of `readPump`.

### H2 — Logout blacklist never checked → tokens unrevocable for 24h
- **Loc:** `services/auth-service/handlers/auth_handler.go:488,531` (write only); `shared/middleware/auth.go:46-96` + `services/api-gateway/middleware/auth.go` (no read)
- **Evidence:** `HandleLogout` writes `blacklist:<token>` to Redis; no middleware ever reads it (grep `blacklist` → only 2 write sites).
- **Fix:** Add `EXISTS blacklist:<token>` check in `JWTAuth` before accepting token.

### H3 — JWT + refresh token in localStorage → XSS = account takeover
- **Loc:** `frontend/src/lib/stores/auth-store.ts:62,74,119-121` (`jwt_token`,`admin_token`,`impersonating`); `frontend/src/app/auth/callback/page.tsx:74` (`refresh_token`); `frontend/src/lib/stores/viewer-auth-store.ts:51,68`
- **Evidence:** Bearer tokens read from `localStorage` per-request (`lib/api/overlays.ts:306`, `lib/api/viewer.ts:45`). Any XSS reads admin JWT + impersonation tokens + refresh token silently.
- **Fix:** Move JWT/refresh to `httpOnly; Secure; SameSite=Strict` cookies set by backend; drop manual `Authorization` header from fetch wrappers. At minimum move `refresh_token` out of localStorage.

### H4 — Viewer JWT leaked via `postMessage(..., '*')`
- **Loc:** `frontend/src/app/chat/auth-success/page.tsx:61,98`
- **Evidence:** `window.opener.postMessage({type:'ALLCHAT_AUTH_SUCCESS', token, streamer}, '*')` — any page that opened the viewer-OAuth popup receives the fresh viewer JWT. No origin/allowlist check.
- **Fix:** Replace `'*'` with explicit `targetOrigin` (All-Chat origin; extension must handshake origin). Verify `event.origin` on inbound listeners too.

### H5 — JWT in URL query string → written to access logs
- **Loc:** `services/api-gateway/handlers/websocket.go:76` + `websocket_viewer.go:91` (`c.Query("token")`); `services/api-gateway/middleware/logging.go:30` (`zap.String("query", c.Request.URL.RawQuery)`); logging middleware applied to `/ws/` (not in skip list `cmd/main.go:374-384`). Also `frontend/src/lib/api/websocket.ts:142`, `useNotificationSocket.ts:127`, `useOverlayStream.ts:206`.
- **Evidence:** Both WS endpoints take `?token=<JWT>`; logging writes full JWT to gateway access logs. Also leaks via nginx/proxy logs, browser history, Referer.
- **Fix:** Redact/strip `token` from query before logging (or skip query logging on `/ws/`). Prefer post-connect `auth` message flow. Send token via `Sec-WebSocket-Protocol` or short-lived single-use WS ticket.

### H6 — k8s: zero `securityContext` on any pod
- **Loc:** all manifests under `deployments/k8s/base/` (api-gateway, redis, postgres, all listeners + support-bot)
- **Evidence:** `grep securityContext|runAsNonRoot|readOnlyRootFilesystem|allowPrivilegeEscalation|capabilities deployments/` → no matches. Redis/Postgres images run as root; no read-only FS, no cap drop, no seccomp.
- **Fix:** Pod-level `securityContext:{runAsNonRoot:true, seccompProfile:{type:RuntimeDefault}}` + container `{allowPrivilegeEscalation:false, capabilities:{drop:[ALL]}, readOnlyRootFilesystem:true}`; relax writable dirs via emptyDir.

### H7 — k8s: no NetworkPolicy + Redis unauthenticated
- **Loc:** `deployments/k8s/base/redis/deployment.yaml` (no `--requirepass`); `configmap.yaml` (no `REDIS_PASSWORD`); both docker-compose files (no `--requirepass`). `kind: NetworkPolicy` → 0 matches.
- **Evidence:** Open in-cluster Redis holds streams/pubsub/OAuth session cache. Any pod can read/write all chat + token data. No egress deny to metadata svc `169.254.169.254`.
- **Fix:** `--requirepass` + `REDIS_PASSWORD` secretKeyRef in all envs (enable ACL). Default-deny ingress+egress NetworkPolicy; allow only required svc-to-svc egress; explicit deny to `169.254.169.254`.

### H8 — DB password insecure defaults fail-open across ~16 services
- **Loc:** `services/*/cmd/main.go` `getEnvOrDefault("DATABASE_PASSWORD","allchat_dev_password")` (api-gateway:89, auth-service:142, overlay-manager:403, message-processor:107, payment-service:111, share-service:292, token-refresh-service:107, moderation-service:324, youtube-quota-monitor:58, youtube-listener:121, token_backfill:133, twitch-listener:91, twitch-eventsub-listener:106, kick-listener:85, discord-listener:504); `tiktok-listener/src/index.ts:70`
- **Evidence:** If secret not mounted, service silently connects with weak dev cred. Misconfigured prod → fail-open to known password.
- **Fix:** Remove insecure defaults for required secrets; `log.Fatal` when `DATABASE_PASSWORD`/`JWT_SECRET_V1`/`TOKEN_ENCRYPTION_KEY_V1` unset (mirror KeyChain fail-fast).

### H9 — CORS wildcard `*` with `AllowCredentials:true`
- **Loc:** `services/api-gateway/middleware/cors.go:34-50` (`AllowOriginFunc` returns true when `allowed=="*"`) + `AllowCredentials:true` line 49
- **Evidence:** If operator sets `CORS_ORIGIN=*`, any origin gets credentialed cross-origin access. Triggered by env-var name mismatch (L8) pushing operators to `*`.
- **Fix:** Fail-fast reject `*` when `AllowCredentials:true`; switch to `shared/middleware/CORSFromEnv()` (TODO at `cmd/main.go:395`).

---

## MEDIUM

### M1 — Refresh token in URL fragment
- **Loc:** `services/auth-service/handlers/auth_handler.go:180-184`, `platform_auth_v2.go:~770`
- **Evidence:** `redirectURL := fmt.Sprintf("%s/auth/callback#access_token=...&refresh_token=%s", ...)` — platform OAuth refresh token in URL fragment (accessible to all JS/extensions).
- **Fix:** Use the viewer flow's short-lived auth-code-via-Redis + POST exchange (already implemented for viewer auth) for streamer login too.

### M2 — No refresh-token rotation / reuse detection
- **Loc:** `services/auth-service/handlers/auth_handler.go:341-397` (`HandleRefresh`)
- **Evidence:** Old refresh token not tracked/revoked; no reuse-detection family.
- **Fix:** Track refresh-token family per user; detect reuse → revoke whole family.

### M3 — No JWT secret minimum length enforcement
- **Loc:** `shared/auth/jwt.go:190-209` (`NewKeyChainFromEnv`)
- **Evidence:** No length validation; `.env` has `JWT_SECRET=CHANGE_ME`. Short secret brute-forceable offline.
- **Fix:** Enforce min 32 bytes; fail-fast on short secret.

### M4 — Signing: future-timestamp replay bypass
- **Loc:** `shared/signing/signing.go:141-146`
- **Evidence:** `time.Since(requestTime) > MaxRequestAge` — `time.Since` negative for future ts → future-dated requests accepted indefinitely until they "age in."
- **Fix:** Add `if time.Until(requestTime) > time.Minute { reject }`.

### M5 — Signing: query params + service name not signed
- **Loc:** `shared/signing/signing.go:193-205` (`computeSignature`)
- **Evidence:** Signed msg = `"method|path|timestamp|body_hash"`. `req.URL.RawQuery` excluded; `X-Service-Name` not authenticated.
- **Fix:** Include `req.URL.RawQuery` + `serviceName` in signed message.

### M6 — `/metrics` unauthenticated
- **Loc:** `services/api-gateway/cmd/main.go:411`
- **Evidence:** Public, no rate limit; exposes topology, latencies, overlay counts.
- **Fix:** Protect with admin JWT or bind to internal-only port.

### M7 — No per-endpoint rate limit on auth/login
- **Loc:** `services/api-gateway/cmd/main.go:433-471` (login/refresh/exchange only global 300/min/IP)
- **Evidence:** Brute-force/credential-stuffing exposure.
- **Fix:** Stricter per-endpoint limit (5-10/min/IP) via `ratelimit.CheckLimitForKey` on `/auth/login`,`/auth/refresh`,`*/exchange`.

### M8 — Viewer WS allows ALL origins (anonymous data exfil)
- **Loc:** `services/api-gateway/handlers/websocket_viewer.go:64-69` (`CheckOrigin: func(r *http.Request) bool { return true }`)
- **Evidence:** Any malicious page can open `/ws/chat/<streamer>` via victim browser; silent chat exfil + connection exhaustion.
- **Fix:** Apply owner-WS origin allowlist (`loadAllowedOrigins`).

### M9 — EventSub webhook: unbounded body before signature verify → DoS
- **Loc:** `services/twitch-eventsub-listener/webhooks/handler.go:87` (`io.ReadAll(c.Request.Body)`), public endpoint (`cmd/main.go:541`)
- **Evidence:** Body buffered before HMAC verify; unauthenticated. OOM on large POST.
- **Fix:** `io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))` or `http.MaxBytesReader` middleware.

### M10 — Frontend: no CSP / security headers / frame-ancestors anywhere
- **Loc:** `frontend/next.config.js` (no `headers()`); `nginx.conf`; no `middleware.ts`
- **Evidence:** Overlay `/overlay/[id]` public URL, no `frame-ancestors` → iframable. Missing CSP/HSTS/X-Content-Type-Options/Referrer-Policy.
- **Fix:** Add `headers()` block: strict CSP per route (`default-src 'self'`; overlay `frame-ancestors 'none'`); standard hardening headers.

### M11 — Frontend: embed postMessage listener skips origin check
- **Loc:** `frontend/src/app/overlays/[id]/preview/embed/page.tsx:284-388`
- **Evidence:** Handles `VISUAL_CSS_UPDATE`/`CUSTOM_CSS_UPDATE`/`TTS_SETTINGS_UPDATE`/etc. without `event.origin` check; outbound `postMessage(...,'*')` (`:390`). `scopeCustomCss` regex bypassable. Any parent can inject CSS/TTS/filter settings.
- **Fix:** `if (event.origin !== window.location.origin) return` at top; outbound with explicit origin.

### M12 — Frontend: open redirect via unvalidated `redirect_to`
- **Loc:** `frontend/src/app/auth/callback/page.tsx:85-98`
- **Evidence:** `router.push(redirectURL)` from hash param, no validation. (Viewer flow at `chat/auth-success/page.tsx:124` correctly uses `startsWith('/')`.)
- **Fix:** Enforce `startsWith('/')` + reject `//` (protocol-relative).

### M13 — Mutable `:latest`/`:main` image tags, no digest pinning
- **Loc:** all base manifests `image: localhost:5000/allchat-*:latest`; `kustomization.yaml:46-73` → `newTag: main`; `support-bot:latest`, `allchat/tiktok-listener:latest`, `allchat/youtube-listener-innertube:latest`
- **Fix:** Pin to immutable tags or `@sha256:`; set `imagePullPolicy: IfNotPresent`.

### M14 — No TLS termination in k8s manifests
- **Loc:** `deployments/k8s/` — no `Ingress`/cert-manager/tls
- **Evidence:** api-gateway exposed raw `LoadBalancer` port 8080 HTTP; configmap sets `FRONTEND_URL: https://allch.at` but no TLS resource.
- **Fix:** Add Ingress + cert-manager ClusterIssuer + TLS secret (or terminate at cloud LB + document).

### M15 — Real secrets in local `.env` + `CHANGE_ME` JWT
- **Loc:** `.env` (gitignored, not committed) — `TWITCH_CLIENT_SECRET=<redacted>`, `TWITCH_BOT_OAUTH=oauth:<redacted>`, `YOUTUBE_API_KEY=<redacted>`, `JWT_SECRET=CHANGE_ME` (real-value prefixes redacted — do not commit any portion of live secrets, even truncated)
- **Fix:** Rotate leaked Twitch/YouTube creds; purge stale `.env` or move to secret manager; never store `CHANGE_ME`.

### M16 — docker-compose publishes internal ports to host + weak defaults
- **Loc:** `deployments/docker-compose.yml`, `docker-compose.frontend.yml` — ports 5432/6379/all svc to host; `POSTGRES_PASSWORD: allchat_dev_password`; `JWT_SECRET: ${JWT_SECRET:-dev-secret-change-in-production}`; `TOKEN_ENCRYPTION_KEY: ${...:-0123456789abcdef0123456789abcdef}`; `MESSAGE_PROCESSOR_API_KEY: dev-mock-key`; `SOURCE_MANAGER_SECRET: dev-service-secret`; frontend `JWT_SECRET: frontend-dev-secret-12345`; Redis no password.
- **Fix:** Bind internal ports to `127.0.0.1`; drop weak fallbacks; `--requirepass` on Redis.

### M17 — TTS token + spoken text in URL query
- **Loc:** `frontend/src/lib/utils/ttsPlayer.ts:401-404` (`?tts_token=${...}&voice=...&text=...`)
- **Evidence:** JWT + spoken chat content (PII) logged in nginx access logs.
- **Fix:** Move to POST body (`Authorization` header for token, JSON body for text); shorter TTL on per-overlay signing secret.

---

## LOW / DEFENSE-IN-DEPTH

### Auth & crypto
- **L1** Signing empty secret accepted — `shared/signing/signing.go:63-70`; `NewSigner("")` works. Return error if `len(secret) < 32`.
- **L2** Signing not wired into any prod service — grep `shared/signing` in `*.go` → 0 prod imports (only tests). NetworkPolicies sole protection. Wire it in or fix CLAUDE.md to say "implemented-not-deployed."
- **L3** No JWT issuer/audience validation — `shared/auth/jwt.go:142-158`. Add `jwt.WithIssuer("all-chat")` to `ParseWithClaims`.
- **L4** No PKCE for Twitch/YouTube OAuth — `services/auth-service/oauth/twitch.go:47-49`, `platform_auth_v2.go:~465`. (Kick has it.) Add `GenerateVerifier`/`S256Challenge`.
- **L5** OAuth state TOCTOU (non-atomic Get+Del) — `auth_handler.go:133-140`, `viewer_auth.go:133-141`. Use `redis.GetDel` (already used in `HandleTokenExchange`).
- **L6** CSRF token logged — `platform_auth_v2.go:599,606,1332` (`zap.String("csrf_token",...)`). Log truncated hash only.
- **L7** All-zero/weak key not warned — `shared/encryption/encryption.go:42` validates length only. Warn in `NewMultiKeyEncryptorFromEnv` on all-zero/low-entropy keys.
- **L8** `ParseKey` base64-vs-raw ambiguity — `encryption.go:60-73`. Require explicit base64 or add sentinel.
- **L9** Dead weak JWT default in overlay-manager — `overlay-manager/cmd/main.go:407` (`getEnv("JWT_SECRET","default-secret-change-in-production")`); field never read (uses KeyChain at :144). Remove dead field.

### Gateway
- **L10** Twitch badge query param injection — `handlers/twitch_badges.go:130` (`fmt.Sprintf("%s?broadcaster_id=%s", url, roomID)`). Use `url.QueryEscape`.
- **L11** Backend error details in 502 — `handlers/proxy.go:119` (`"backend service unavailable: " + err.Error()`). Return generic; log full server-side.
- **L12** CORS env var name mismatch — gateway reads `CORS_ORIGIN` (`cors.go:29`), `.env.example` documents `CORS_ALLOWED_ORIGINS`, shared reads `CORS_ORIGINS`. Align on one name.
- **L13** OBS-mode allows unauth WS to any active overlay — `websocket.go:79-95`. Acceptable for OBS; consider per-overlay short-lived non-JWT OBS token rotated from dashboard.
- **L14** `/api/v1/internal/overlays/:id/sources/auto` in `protectedAPI` (user JWT) not `/internal` (service JWT) — `cmd/main.go:578-579`. Move to internal group if truly service-to-service.
- **L15** `POST /api/v1/overlays/:id/tts` public (no gateway JWT) — `cmd/main.go:487`; auth downstream via `tts_token` (also URL-logged, see M17). Defense-in-depth gap.
- **L16** No HSTS/CSP at gateway — `shared/middleware/security_headers.go` sets XFO/XCTO/Referrer/Permissions but not HSTS/CSP. Confirm handled at TLS ingress.
- **L17** Proxy copies `Cookie`/`Referer`/`Origin` to backend — `handlers/proxy.go:108`. Unnecessary; strip.

### Injection (defense-in-depth; no exploitable SQLi found)
- **L18** `auth-service/handlers/admin_cosmetics.go:101,145,167` — `"... FROM " + table` string concat; currently hardcoded literals only. Add `map[string]bool` table allow-list (mirrors `user_repository.go:520`).
- **L19** `twitch-eventsub-listener/webhooks/handler.go:531` — JSON built via `fmt.Sprintf` with Twitch-supplied `BroadcasterUserName`. Use `json.Marshal` of a struct.

### Listeners
- **L20** Discord webhook URL logged with token — `discord-listener/relay/poster.go:119,128` (`zap.String("webhook_url", webhookURL)`). Discord webhook URL embeds token. Log webhook ID only.
- **L21** Twitch IRC reconnect no jitter — `twitch-listener/irc/connection.go:258-322` (pure doubling). Use existing `listener.JitteredBackoff` (imported at :82).
- **L22** InnerTube unbounded recursive JSON walk — `youtube-listener-innertube/innertube/discovery.go:340-385` (`collect`, no depth cap). Add `depth` param, cap ~20.
- **L23** YouTube/InnerTube `io.ReadAll` without `LimitReader` — `youtube-listener/api/client.go:438,562`; `innertube/client.go:155`, `discovery.go:127,310,639`. Wrap with `io.LimitReader(resp.Body, 10MB)`. (Contrast: `streams/livechat.go:94` already does.)
- **L24** TikTok username not validated — `tiktok-listener/src/connection-pool/pool-manager.ts:280`, `status-checker.ts:105`. Validate `^[a-zA-Z0-9_.]{2,24}$`.
- **L25** EventSub subscription manager uses `http.DefaultClient` (no timeout) — `twitch-eventsub-listener/eventsub/subscription_manager.go:81`. Use `&http.Client{Timeout:10s}`.

### Config / infra
- **L26** Hardcoded public InnerTube API key — `youtube-listener-innertube/innertube/client.go:39`, `overlay-manager/youtube/resolver.go:48`. Public web-client key; make configurable for rotation hygiene.
- **L27** `.dockerignore` missing for 16/20 services — only frontend/discord-bot/tiktok-listener/twitch-eventsub-listener have one. Add repo-root `.dockerignore`.
- **L28** Unpinned base images — `discord-listener/Dockerfile:18`, `kick-listener/Dockerfile:20` use `alpine:latest`. Pin (others use `alpine:3.23`).
- **L29** Default ServiceAccount for most pods + token automount — only source-manager sets `serviceAccountName`. Add dedicated SAs + `automountServiceAccountToken: false`.
- **L30** `generate-vault-from-env.sh` weak fallbacks — `deployments/ansible/generate-vault-from-env.sh:115-136` bakes `allchat_dev_password`/`dev-secret-change-in-production`/`0123...` into vault when `.env` unset. Fail loud.

### Frontend
- **L31** Gradient CSS injection — `frontend/src/lib/utils/gradient.ts:28` (`linear-gradient(${g.angle}deg, ${g.colors.join(', ')})`); `parseNameGradientGuard` only JSON-parses. Validate colors `^#[0-9a-f]{3,8}$/i`.
- **L32** `auth_url` redirect without client allowlist — ~12 sites (`HomeClient.tsx:209,224,239`, `settings/viewer/page.tsx:83,105`, `overlays/[id]/page.tsx:770,1747`, `lib/api/payment.ts:38`, `lib/api/viewer.ts:130`, `lib/api/discord.ts:62,75`, `EventSubMigrationBanner.tsx:84`). Add client-side allowlist (twitch.tv/patreon.com/discord.com) as defense-in-depth.
- **L33** `dangerouslySetInnerHTML` on streamer CSS (owner-authored, public) — `overlay/[id]/page.tsx:634,637,638`, `credits/page.tsx:256`, `preview/embed/page.tsx:645,651`. Pair with CSP `style-src` to cap blast radius.

---

## SUPPLY-CHAIN / PROCESS

### S1 — govulncheck + npm audit NOT in CI
- **Manual run result (this audit):** `govulncheck ./...` on api-gateway, auth-service, shared → **0 reachable vulns**. Module-level (uncalled) advisories exist — bump:
  - `golang.org/x/net` v0.52.0 → **v0.55.0** (GO-2026-5026, 4918, 5030, 5029, 5028, 5027, 5025)
  - `golang.org/x/crypto` v0.49.0 → **v0.52.0** (GO-2026-5033, 5023, 5021, 5020, 5019, 5018, 5017)
  - `golang.org/sys` v0.42.0 → **v0.44.0** (GO-2026-5024)
- **Fix:** Wire `govulncheck ./...` per module into CI gate; `npm audit` on frontend + discord-bot + tiktok-listener + support-bot. Other service modules (17) not scanned this audit — run there too.

### S2 — Version skew across modules
- `shared/ratelimit`+`shared/signing` pin `x/crypto v0.48.0`/`x/net v0.51.0`; services on `v0.49/v0.52`; youtube-listener `grpc v1.81.1` vs others `v1.80.0`.
- **Fix:** Unify to highest versions.

### S3 — gorilla/websocket pseudo-version
- api-gateway/kick-listener/message-processor require `v1.5.4-0.20250319132907-e064f32e3674` (untagged); discord-listener on `v1.5.3`. Pin a tag when upstream releases.

### S4 — Stray nested `frontend/frontend/package.json`
- Only `@monaco-editor/react`; likely accidental. Remove.

---

## STRENGTHS (verified, no action needed)

- **JWT signing:** HMAC-pinned KeyFunc rejects non-HMAC (alg-confusion/`alg:none` blocked); `exp`/`nbf` validated by jwt/v5; multi-key `kid` rotation + versioned secrets; separate `SERVICE_JWT_SECRET` KeyChain → cross-chain isolation tested.
- **Encryption at rest:** AES-256-GCM, fresh random 12-byte nonce per encrypt (`io.ReadFull(rand.Reader)`), GCM tag verified on decrypt (`gcm.Open`), key length validated {16,24,32}, versioned kid rotation correct (AEAD tag must verify regardless of key path), `ErrNoEncryptionKeys` fail-fast.
- **Signing compare:** `hmac.Equal` (wraps `subtle.ConstantTimeCompare`) everywhere; no `==` on signatures.
- **Injection:** pgx `$N` parameterization consistent; no string-concat SQL with user input; no `exec.Command`/`os/exec`; no path traversal; no `text/html/template`/`template.HTML`; no `fmt.Fprintf` into responses.
- **OAuth tokens** encrypted at rest via `MultiKeyEncryptor` (AES-GCM) in `UserRepository`.
- **EventSub webhook** HMAC-SHA256 constant-time + 10-min skew + Redis dedup (set only after success).
- **No `InsecureSkipVerify`** anywhere (Kick Pusher + Discord gateway use default TLS).
- **IDOR** protected — overlay-manager verifies ownership via `GetByIDAndUserID` on every overlay/source CRUD.
- **Gateway foundations:** `gin.Recovery()`, `SecurityHeaders()` (XFO/XCTO/Referrer/Permissions), `BodyLimit(2MB)`, server timeouts (30/30/60s), admin `AdminOnly` gate, `ServiceJWTAuth` allowlist for `/internal`, proxy `CheckRedirect` blocks SSRF-via-redirect, rate limiter present.
- **Frontend chat:** React auto-escaping, `next/image` with `images.domains` allowlist, no `eval`/`new Function`/`document.write`, Umami sanitises tokens from analytics URLs, Bearer auth (CSRF N/A), non-root Docker user, no browser source maps.
- **Secrets hygiene:** k8s via `secretKeyRef` (no plaintext `stringData`), ansible-vault encrypted secrets (gitignored), `.env` gitignored, no committed private keys, no pprof/expvar/debug endpoints, no hardcoded `AKIA`/`ghp_`/`sk_`/`oauth:` literals.
- **RBAC:** source-manager least-privilege `Role` (leases+pods get/list only).
- **`govulncheck`:** 0 reachable vulns in 3 critical modules.

---

## Recommended Fix Order

1. **H1** (WS ReadLimit) — 1-line fix, kills gateway DoS.
2. **H6 + H7** (securityContext + NetworkPolicy + Redis auth) — biggest blast-radius reduction; infra-only, no code contracts.
3. **H8** (DB-password fail-fast) — systemic, prevents fail-open prod deploy.
4. **H2** (logout blacklist) — token revocation correctness.
5. **H3 + H4 + H5** (frontend auth: localStorage→cookie, postMessage origin, WS token logging) — XSS→ATO chain.
6. **H9 + L12** (CORS hardening) — fail-fast wildcard rejection.
7. **M4 + M5 + L1** (signing replay/service-identity/empty-secret) — service-to-service integrity.
8. **M9** (EventSub body cap) + **M6/M7** (metrics auth + auth rate limit).
9. **M10 + M11 + M12** (frontend CSP/postMessage/open-redirect).
10. **S1** (CI govulncheck + npm audit) + version bumps.
11. Remaining M/L items.

---

*Audit compiled from 8 parallel reviewer subagents + manual govulncheck. See `SECURITY_AUDIT_REPORT.md`.*
