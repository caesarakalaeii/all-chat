# PR #478 Review Findings — Implementation Worklist

**PR:** #478 "🔒 Security audit hardening + H3 cookie-auth migration"
**Branch:** `security/audit-hardening` · **Base/merge-base:** `5aa0bfca052acd69c920114c90374218ac1cec65` · **Head reviewed:** `69319f8b`
**Author:** caesarakalaeii · **Size:** 190 files, +7,381 / −1,285 · 100% AI-generated
**Reviewed:** 2026-06-24 (14-agent workflow + manual re-verification of the blocking items)

---

## ✅ Resolution Status (worked 2026-06-24)

All BLOCKING + SHOULD-FIX + most LOW items addressed on `security/audit-hardening`
(working tree, uncommitted). Verification gate below is green except where noted.

| ID | Status | What landed |
|-----|--------|-------------|
| **B1** | ✅ fixed | `.dockerignore`: un-excluded `scripts/run-migrations.sh` so the migrations image build finds it. |
| **C1** | ✅ fixed | Gateway: registered missing `POST /api/v1/auth/exchange` (rate-limited + proxy) — streamer/admin login no longer 404s. |
| **C2** | ✅ fixed + tested | `/exchange` now seeds `refresh_token:<hash>` reuse key; first `/refresh` after login no longer force-logs-out. 2 new tests. |
| **B2** | ✅ fixed | Reverted `/metrics` to unauth `gin.WrapH(promhttp.Handler())`; Prometheus scraping restored (ServiceMonitor `allchat-services` + pod annotations confirmed). |
| **C5** | ✅ partial | Added `TestHandleStreamerTokenExchange_SeedsRefreshTokenReuseKey` + `TestHandleRefresh_AfterExchange_DoesNotForceLogout` proving the C1/C2 contract (CI-runnable, miniredis). Full gateway-router E2E BLOCKED: gateway router is inline in `main()` (no extractable helper) + `/refresh` uses concrete `*UserRepository` (needs real DB; `setupTestDB` is testcontainers/Docker-gated + not cross-package reusable). Tracked as follow-up (extract `buildRouter`). RESIDUALS H3 updated. |
| **M1** | ✅ fixed + tested | Backslash guard in fe `redirect-allowlist.ts`, fe callback page (uses shared helper), and be `viewer_auth.go` `sanitizeRedirectPath`. New fe + be tests. |
| **M2** | ✅ fixed | `client.ts` retry routes back through `this.fetch` with one-shot recursion guard. |
| **M3** | ✅ fixed | Gateway `router.SetTrustedProxies(TRUSTED_PROXIES)` (default RFC1918); `ClientIP` no longer spoofable via X-Forwarded-For. ingress-nginx LB confirmed. |
| **M4** | ✅ fixed + tested | Shared `OriginAllowed` helper (exact + `*` + `/*` suffix); CORS, OriginCheck, both WS checkers refactored to use it. 7 new tests. |
| **M5** | ✅ fixed | `redis/secret.yaml` removed from kustomization `resources:` (out-of-band mgmt comment); `apply -k` no longer clobbers live AUTH password. |
| **M6** | ✅ no-op (verified) | k3s/flannel cluster: pods 1/1 Ready with NetworkPolicies applied — node→pod probes work. No code change needed; CNI permits probes. |
| **M7** | ✅ fixed + tested | Twitch IRC reconnect: `attempt=0` reset now guarded on `wasConnected`; extracted `nextBackoff()`; 4 new tests prove escalation to 30s cap. |
| **M8** | ✅ fixed (code+doc) | Dev-mode console warning in `client.ts` (code) + same-origin invariant documented in `frontend/README.md` + `docs/pi/specs/2026-06-23-h3-cookie-auth-design.md` (doc). |
| **M9** | ⏳ deferred | PR split — process recommendation; coordinate with PR author post-fix. (Rebase onto `main` still outstanding — PR is CONFLICTING.) |
| **L1** | ✅ fixed | `shared/middleware` `SetLogger`; blacklist Redis errors now actually logged (were `_ = err`). Gateway wires it at startup. |
| **L2** | ✅ fixed | `/refresh` restores reuse key on transient upstream 5xx (503); keeps terminal 400/401 (ClearAuthCookies + 401). |
| **L3** | ✅ fixed | `DELETE /auth/me` now clears cookies + revokes refresh (parity with `/logout`); gateway route gets `AuthCookieForward`. |
| **L4** | ✅ fixed | Both callback paths log `storeErr` (was stale `err`). |
| **L5** | ✅ fixed + tested | `warnIfWeakKey` now takes a real logger via `NewMultiKeyEncryptorFromEnvWithLogger`; 10 prod callers updated; old fn kept for compat. |
| **L6** | ✅ fixed | `next.config.js` X-Frame-Options deduped to one value per route. |
| **L7** | ✅ fixed | `client.ts` 401-guard prefixes → `/api/v1/auth/refresh`, `/api/v1/auth/login`. |
| **L8** | ✅ fixed + tested | `tts.go`: voiceID validated `^[A-Za-z0-9_-]+$` + `url.PathEscape`; body bounded via `http.MaxBytesReader`. 20 new subtests. |
| **L9** | ✅ fixed | `govulncheck@latest` → `@v1.1.4`. |
| **L10** | ✅ fixed | `generate-vault-from-env.sh`: `CORS_ALLOWED_ORIGINS` → `CORS_ORIGINS`. |
| **L12** | ✅ fixed | `auth-store.ts`: removed `as unknown as User`; fetches full `/auth/me` after swap, `Partial<User>` merge fallback. |
| **L13** | ✅ fixed | `shared/signing/README.md` rewritten to match real API (NewSigner returns error, 6-field payload, VerifySignature). |
| **L11** | ✅ non-issue | All base deployments already use `imagePullPolicy: IfNotPresent` (no `Always` anywhere in `deployments/k8s/`). Finding's premise didn't hold. Digest-pinning remains the only hardening (Keel handles tag updates); tracked TODO. |
| **L14** | ✅ fixed | TLS overlay: removed `configuration-snippet` annotation (disabled by default since ingress-nginx v1.9); replaced with built-in `proxy-read-timeout`/`proxy-send-timeout` annotations. ingress-nginx handles WS upgrade natively. |
| **I1** | ✅ fixed + tested | `checkOrigin` hardened via pure `originAllowedForWS` helper: when the access_token COOKIE is present (browser path), a non-empty allowed Origin is REQUIRED (empty Origin → reject). Non-cookie paths (subprotocol/query token for OBS) still allow empty Origin. 4 new tests. |
| **I2** | ✅ fixed + tested | Viewer OAuth callbacks (Twitch/YouTube/Kick) now use idempotency tombstones (`<stateKey>:used`, 60s TTL) mirroring `platform_auth_v2`. Duplicate callbacks (iOS Safari prefetch, Google multi-code) replay the original redirect instead of hard-failing. 2 new tests. |
| **I3** | ✅ partial fix | `HandleStopImpersonation` now blacklists the impersonation JWT (was valid to expiry). Roles-always-`[user,admin]` scoping left as product decision (admins may need admin role while impersonating to troubleshoot). |
| **I4** | ✅ non-issue | `ALLCHAT_INNERTUBE_KEY` set in NEITHER `deployments/k8s` NOR `caesar-deployment` → both fall back to the same public default key → resolve vs listen consistent. |

### Verification gate (run 2026-06-24)
- `go build ./...` + `go vet ./...` clean in shared + all 5 changed service modules (api-gateway, auth-service, overlay-manager, twitch-listener, + 10 L5 callers). ✅
- `gofmt -l` clean on all session-changed Go files. ✅
- `cd frontend && npx tsc --noEmit` exit 0; `npm run build` success (32 pages). ✅
- New tests pass: C2 (2), M4 (7), M7 (4), L5 (2), L8 (20 subtests), M1-fe (4), M1-be. ✅
- `go test -short ./handlers/...` (auth-service) green. ✅
- **Still outstanding:** rebase onto `main` (PR is CONFLICTING — in progress); M9 split (process decision — coordinate with PR author); full gateway-router E2E (C5 — blocked on `main()` router-extraction refactor); I3 roles-scoping (product decision).

---

## How to read this doc

Each item has: **ID**, **severity**, **verification status**, **location(s)**, **problem**, **prescribed fix**, **how to verify**.

- `[VERIFIED-MANUAL]` = re-confirmed by hand against the code (high confidence; fix as written).
- `[AGENT-VERIFIED]` = confirmed by an adversarial verifier agent (high confidence; confirm line numbers before editing).
- `[AGENT-REPORTED]` = single-pass finding; **confirm the claim before changing code**.
- `[INVESTIGATE]` = a gap to analyze and decide on — do **not** blind-fix.

Line numbers are from head `69319f8b`; re-grep before editing in case earlier fixes shift them.

**Do NOT "fix" the false positives in the [Non-issues](#non-issues--do-not-change) section.**

---

## 🔴 BLOCKING — must fix before merge

### C1 — Gateway never registers `POST /api/v1/auth/exchange` → streamer/admin login 404s `[VERIFIED-MANUAL]`

- **Where:** `services/api-gateway/cmd/main.go` (publicAPI group, ~L458–461). Frontend caller: `frontend/src/app/auth/callback/page.tsx:79`. Backend route exists: `services/auth-service/cmd/main.go:331` (`router.POST("/exchange", legacyAuthHandler.HandleStreamerTokenExchange)`).
- **Problem:** The frontend OAuth callback POSTs the single-use code to `/api/v1/auth/exchange` (where auth-service sets the httpOnly cookies). The gateway publicAPI group registers `/auth/login`, `/auth/callback`, `/auth/refresh`, and the **viewer** exchanges only — there is **no** `/auth/exchange`, and there is **no** production catch-all/`NoRoute` proxy. The POST hits Gin's 404 before reaching the proxy, so the `Set-Cookie` never reaches the browser. Every fresh streamer/admin login fails.
- **Fix:** Register the route in the publicAPI group, next to `/auth/refresh` (mirror its middleware chain so the Set-Cookie passes back and the body refresh-token path works):
  ```go
  publicAPI.POST("/auth/exchange",
      authRateLimiter.Middleware(),
      proxyHandler.ForwardRequest)
  ```
  (It does **not** need `AuthCookieForward`/`OriginCheck` — at exchange time there is no cookie yet; the client sends a JSON `{code}` body. Just rate-limit + proxy. The proxy already passes Set-Cookie back, see proxy.go copyHeaders.)
- **Verify:** `grep -n 'auth/exchange' services/api-gateway/cmd/main.go` shows the new streamer route. Add the E2E test in C5 which exercises this path.

### C2 — Production login path never seeds the refresh-token reuse set → first `/refresh` force-logs-out every user `[VERIFIED-MANUAL]`

- **Where:** Seeding gap in `services/auth-service/handlers/auth_handler.go` `HandleStreamerTokenExchange` (L456–498). Production login handler `services/auth-service/handlers/platform_auth_v2.go` `HandleCallback` (stores tokens only in `streamer_auth_code:<uuid>` via `storeStreamerAuthCode`, never writes `refresh_token:`). Reuse check that fails: `auth_handler.go:540–553` (`HandleRefresh`).
- **Problem:** `HandleRefresh` does `GetDel("refresh_token:"+sha256(refreshToken))` and treats a **miss as token theft** → `ClearAuthCookies` + 401. But the production login path (`platform_auth_v2.HandleCallback` → `/exchange` → `HandleStreamerTokenExchange`) **never seeds** that key. (The seeding code exists only in the legacy `HandleCallback`/`HandleYouTubeCallback` at L282/L444, which are **not** wired to `/callback`.) Result: the first refresh after any real login is misclassified as reuse and the user is logged out. **This fires even after C1 is fixed.**
- **Fix:** Seed the reuse key when issuing cookies in `HandleStreamerTokenExchange`, right after `auth.SetAuthCookies(...)` (~L487), mirroring the existing pattern at `auth_handler.go:280–286`:
  ```go
  // Track refresh token for reuse detection (audit M2) — seed on initial issue,
  // mirroring HandleCallback. Without this the first /refresh is misread as reuse.
  if payload.RefreshToken != "" {
      rtKey := "refresh_token:" + refreshTokenHash(payload.RefreshToken)
      if err := h.redis.Set(c.Request.Context(), rtKey, payload.User.ID, 14*24*time.Hour).Err(); err != nil {
          h.logger.Warn("Failed to track refresh token for reuse detection", zap.Error(err))
      }
  }
  ```
  (Confirm `payload.User.ID` is the correct field on `StreamerAuthPayload`; if `User` is a different shape, use any stable non-empty value — the value is only informational, the key's existence is what matters.)
- **Verify:** Unit/integration test: simulate exchange (seeds key) → `/refresh` (GetDel hits, rotates, re-seeds at L592). The first refresh must return 200, not 401.

### B1 — New root `.dockerignore` breaks the `migrations` image build (CI currently RED) `[VERIFIED-MANUAL]`

- **Where:** `.dockerignore` (the `scripts/` line, ~L51) vs `migrations/Dockerfile:4` (`COPY scripts/run-migrations.sh /run-migrations.sh`). Built with `context: .` per `.github/workflows/build-and-push.yml` (matrix entry `path: migrations`, L205–207, L243).
- **Problem:** The root `.dockerignore` excludes `scripts/`, but the migrations image copies `scripts/run-migrations.sh` from the root build context → `"/scripts/run-migrations.sh": not found`. Confirmed this is the **only** broken build (all other context-`.` Dockerfiles copy only `shared/`, `services/<svc>/`, `static/`, `migrations/*.sql`, `LICENSE` — none excluded).
- **Fix:** Un-exclude the one needed file. In `.dockerignore`, immediately after the `scripts/` line add:
  ```
  scripts/
  !scripts/run-migrations.sh
  ```
- **Also (recommended):** add a CI smoke-build of the migrations image so this regression can't recur (it currently only triggers when `migrations/**` or `scripts/run-migrations.sh` change, so it's latent on most PRs).
- **Verify:** `docker build -f migrations/Dockerfile -t t .` from repo root succeeds. Re-run CI `build-and-push (migrations,...)`.

### B2 — Admin-gating `/metrics` (M6) breaks Prometheus scraping in prod `[AGENT-VERIFIED]`

- **Where:** `services/api-gateway/cmd/main.go` ~L427–436 (metricsGroup chain `CookieToBearer → JWTAuthWithRevocation → OriginCheck → AdminOnly`). Scrape config in `caesar-deployment`: `apps/workloads/all-chat/servicemonitor.yaml` (ServiceMonitor `allchat-services` includes `api-gateway`, `path: /metrics`, no bearer token) + pod annotations `prometheus.io/scrape` / `prometheus.io/port: 8080`.
- **Problem:** Prometheus presents no JWT, so `/metrics` now returns 401 and api-gateway metrics collection silently stops (dashboards/alerts go blind). Fails closed (no security exposure) but breaks observability. The intended isolation is already network-level (`networkpolicies.yaml` allows the monitoring namespace to reach 8080).
- **Fix (pick one):**
  1. **Preferred:** Drop the admin-JWT chain on `/metrics` and rely on the NetworkPolicy (already present) to restrict scrape access. (Revert metricsGroup to the plain `router.GET("/metrics", gin.WrapH(promhttp.Handler()))` that was on base.)
  2. Or expose `/metrics` on a separate cluster-internal port not routed through the auth chain.
  3. Or, if admin-JWT is kept, update the ServiceMonitor in `caesar-deployment` with `bearerTokenSecret` in the **same** change and verify targets stay UP. (Most fragile — not recommended.)
- **Verify:** After the change, `curl -s localhost:8080/metrics` (no auth) from an allowed network returns 200 metrics; confirm Prometheus targets for api-gateway are UP.

### C5 — Add the missing E2E test + correct the "DONE" status `[VERIFIED-MANUAL]`

- **Where:** `RESIDUALS.md:14` marks H3 "✅ DONE"; `services/auth-service/handlers/auth_handler.go:516–520` carries `TODO(H3-integration): ... not unit-tested`.
- **Problem:** No end-to-end test covers the streamer cookie-login path; unit tests mock at the handler level so they pass while C1/C2 break the real flow. Marking it DONE invites deploying a login-breaking change with green CI.
- **Fix:** Add an integration test (real gateway router + auth-service + miniredis) exercising **callback → /exchange (Set-Cookie) → authed request → access expiry → /refresh (rotate) → /logout**. This is the test that would have caught C1 and C2. Until it exists and passes, downgrade `RESIDUALS.md` H3 status to "pending integration verification."
- **Verify:** Test fails before C1/C2 fixes, passes after.

---

## 🟡 SHOULD-FIX (medium)

### M1 — Redirect-allowlist backslash bypass `[VERIFIED-MANUAL]`
- **Where:** `frontend/src/lib/auth/redirect-allowlist.ts:48–49`; same pattern in `frontend/src/app/auth/callback/page.tsx` (M12 fix) and backend `services/auth-service/.../viewer_auth.go` `sanitizeRedirectPath` (~L1135).
- **Problem:** `if (url.startsWith('/') && !url.startsWith('//')) return true` accepts `/\evil.com` as a "safe relative path", but browsers normalize `\`→`/` and navigate to `evil.com`.
- **Fix:** Reject backslashes before the relative-path shortcut, e.g. `if (/\\/.test(url)) return false;`, **or** always parse via `new URL(url, origin)` and validate `parsed.origin`/`hostname`. Apply the same fix in all three locations.
- **Verify:** `isAllowedExternalRedirect('/\\evil.com')` returns false; add a unit test.

### M2 — 401→refresh→retry returns an unchecked `Response` `[VERIFIED-MANUAL]`
- **Where:** `frontend/src/lib/api/client.ts:91–93`.
- **Problem:** After a successful refresh, the retry does `return fetch(...)` and hands the raw `Response` back to `get/post/...` which call `.json()` directly, bypassing the `!response.ok` / `ApiError` / `reauth_required` handling. A second 401/403/500 or a 204 then throws a raw `SyntaxError` instead of a structured `ApiError`.
- **Fix:** Route the retry back through `this.fetch` with a one-shot guard flag to prevent recursion, **or** replicate the `response.ok`/error-parsing/`ApiError` logic on the retried response before returning it.
- **Verify:** Unit test: post-refresh response of 500 surfaces as `ApiError` with `.status`/`.data`.

### M3 — Auth rate limiter (M7) is bypassable / mis-attributed `[AGENT-REPORTED, high-confidence]`
- **Where:** `services/api-gateway/cmd/main.go:383` (`gin.New()`, no `SetTrustedProxies`); key built from `c.ClientIP()` in `shared/ratelimit/ratelimit.go:~110`.
- **Problem:** Without `SetTrustedProxies`, gin derives `ClientIP` from attacker-controlled `X-Forwarded-For`, defeating the per-IP 5/min brute-force limit (and possibly bucketing all real users as the single ingress IP).
- **Fix:** Call `router.SetTrustedProxies([...])` restricted to the ingress/LB CIDR so `ClientIP` reflects the real edge IP. Confirm what ingress-nginx forwards in this cluster first.
- **Verify:** With a spoofed `X-Forwarded-For` per request, the limiter still trips after N real requests.

### M4 — `OriginCheck` exact-match diverges from CORS wildcard matching `[AGENT-VERIFIED]`
- **Where:** `shared/middleware/origin_check.go` (builds `map[string]bool`, exact lookup) vs `services/api-gateway/middleware/cors.go` (supports `strings.HasSuffix(allowed, "/*")` prefix match). Both consume `LoadHTTPAllowedOrigins()`.
- **Problem:** CORS permits `moz-extension://*` / `chrome-extension://*`; OriginCheck does pure exact match → it will 403 every state-changing request from the Firefox extension once such an origin is configured. Latent today (prod uses literal `https://allch.at`) but a coherence bug.
- **Fix:** Factor the CORS origin-match logic (exact + `/*` suffix prefix) into one shared helper and have CORS, WS `loadAllowedOrigins`, and `OriginCheck` all call it. Add a test with `Origin: moz-extension://<uuid>` against a `moz-extension://*` allowlist.
- **Verify:** Shared matcher returns identical results for CORS and OriginCheck.

### M5 — Empty `redis-auth` Secret can clobber an out-of-band password `[AGENT-VERIFIED]`
- **Where:** `deployments/k8s/base/redis/secret.yaml:32–33` (`stringData: redis-password: ""`) + `deployments/k8s/base/kustomization.yaml:15` (listed under `resources:`).
- **Problem:** Shipping an empty-valued Secret as an apply-able resource means any later `kubectl apply -k` overwrites a live AUTH password back to empty (silently disabling Redis AUTH). Benign only because AUTH is currently default-off.
- **Fix:** Don't ship the empty Secret as a `resources:` entry. Use a kustomize `secretGenerator` with a documented external source, or remove `secret.yaml` from `resources:` and manage redis-auth via sealed-secrets/external-secrets only.
- **Verify:** `apply -k` no longer touches `redis-password`.

### M6 — NetworkPolicy default-deny may block kubelet httpGet probes (CNI-dependent) `[AGENT-REPORTED]`
- **Where:** `deployments/k8s/base/networkpolicies.yaml:33–44` (default-deny-all) and L92–103 (allow-intra-namespace).
- **Problem:** kubelet liveness/readiness probes originate from the node IP, not an allchat pod. For every non-gateway service the probe source matches no ingress allow-rule. Whether this breaks probes depends on the CNI (Calico default permits node→pod; Cilium with host firewall / strict CNIs drop it → NotReady/CrashLoop).
- **Fix:** Verify the cluster CNI permits node→pod probe traffic; if not, add an explicit ingress allow for the node CIDR / kube-system on the health ports. **Roll out to a staging namespace first and confirm all pods reach Ready.**
- **Verify:** Pods Ready in staging with the policies applied.

### M7 — Twitch IRC reconnect "exponential backoff" never escalates `[AGENT-VERIFIED]`
- **Where:** `services/twitch-listener/irc/connection.go:299–339`.
- **Problem:** `attempt++` is immediately overwritten by `attempt = 0` at the bottom of **every** loop iteration, so `JitteredBackoff(attempt)` is always called with `attempt=0` → random sub-1s delay (tighter than the old fixed 5s), undermining the thundering-herd goal. Relevant given known nightly listener crashloops during Redis/kured reboots.
- **Fix:** Only reset `attempt = 0` on a successful/stable connection (move the reset into `handleConnect` or guard it on `wasConnected` becoming true), not unconditionally each iteration.
- **Verify:** Unit-test the backoff sequence escalates across consecutive failures up to the 30s cap.

### M8 — Same-origin frontend+gateway is an undocumented hard requirement `[AGENT-REPORTED]`
- **Where:** `frontend/src/lib/api/client.ts:42,74,117` (`credentials: 'same-origin'`); spec `docs/pi/specs/2026-06-23-h3-cookie-auth-design.md:161`.
- **Problem:** Cookie-auth only works if frontend and gateway share an origin (`SameSite=Lax` + host-only cookie + `credentials:'same-origin'`). The dev default (`:3000` frontend → `:8080` gateway) is cross-origin and would silently fail; a misconfigured `NEXT_PUBLIC_API_URL` breaks login with no obvious error.
- **Fix:** Document the same-origin invariant prominently (README + spec) and add a config note. If cross-origin deploy is ever needed: `credentials:'include'` + `SameSite=None` + strict CORS allowlist + revisit CSRF. (No code change strictly required for prod if ingress already serves both from `allch.at` — confirm that and document it.)
- **Verify:** Documented; optionally a dev-mode console warning when API base origin ≠ window origin.

### M9 — Split the PR (scope/reviewability/rollback) `[AGENT-REPORTED]`
- **Problem:** 190 files mix infra/k8s hardening + shared-lib crypto + a coordinated 3-service auth contract change. Different blast radii; all-or-nothing rollback; impractical human review (the C1 one-line omission hid in the diff).
- **Recommendation:** After the blocking fixes, split into independently reviewable/deployable PRs: **(1)** infra/k8s hardening (no code contracts), **(2)** shared-lib + per-service `JWTAuthWithRevocation`, **(3)** H3 cookie-auth migration with its E2E test (C5). Also rebase — the PR is currently `CONFLICTING` with `main`.
- **Note:** This is a process recommendation; coordinate with the PR author before reorganizing commits.

---

## 🟢 LOW / INFO (fix opportunistically)

- **L1** `[VERIFIED-MANUAL]` Blacklist (revocation) check **fails open on Redis errors and does NOT log** despite the comment claiming it does — `shared/middleware/auth.go:68–81` (`_ = err`). Inject a logger + emit the error (and ideally a metric); fix the misleading comment. Consider fail-closed for admin routes.
- **L2** `[AGENT-REPORTED]` `HandleRefresh` consumes the reuse `GetDel` **before** the Twitch exchange — a transient upstream 5xx then permanently invalidates a valid session. `auth_handler.go:540–563`. Consider only consuming/rotating after the exchange succeeds, or restore the key on non-401 errors.
- **L3** `[AGENT-REPORTED]` `HandleDeleteAccount` (`auth_handler.go:~690–731`) doesn't `ClearAuthCookies` or delete `refresh_token:<hash>` — bring to parity with `HandleLogout`.
- **L4** `[AGENT-REPORTED]` Wrong error var logged: `zap.Error(err)` should be `zap.Error(storeErr)` at `auth_handler.go:274` and `:436`.
- **L5** `[AGENT-REPORTED]` `warnIfWeakKey` is dead code — always called with `zap.NewNop()` in `shared/encryption/versioned.go:165–167,310–325`; only checks all-zero. Pass a real logger or fail-closed on all-zero key.
- **L6** `[AGENT-REPORTED]` Duplicate/conflicting `X-Frame-Options` (DENY + SAMEORIGIN) on overlay routes — `frontend/next.config.js:79–98`. CSP `frame-ancestors 'none'` still covers it; dedupe headers per-route.
- **L7** `[AGENT-REPORTED]` Dead 401-guard prefixes `/auth/refresh`/`/auth/login` never match (real endpoints are `/api/v1/auth/...`) — `frontend/src/lib/api/client.ts:76`. Use real prefixes.
- **L8** `[AGENT-REPORTED]` TTS handler: user-controlled `voiceID` interpolated into upstream URL path (validate `^[A-Za-z0-9]+$` / `url.PathEscape`) and unbounded JSON body read before rate-limit/auth (wrap in `http.MaxBytesReader`) — `services/overlay-manager/handlers/tts.go:658–728`.
- **L9** `[AGENT-REPORTED]` `govulncheck@latest` unpinned in `.github/workflows/security-scan.yml` — pin to a version.
- **L10** `[AGENT-REPORTED]` ansible `generate-vault-from-env.sh:158` still reads removed `CORS_ALLOWED_ORIGINS` (renamed to `CORS_ORIGINS` in `.env.example`) → cors_origin silently defaults. Update the var name.
- **L11** `[AGENT-REPORTED]` `imagePullPolicy: Always → IfNotPresent` on mutable `main`/`latest` tags across `deployments/k8s/base/*/deployment.yaml` — stale-image risk until digests are pinned. Acceptable with Keel; track the digest-pinning TODO.
- **L12** `[AGENT-REPORTED]` `stopImpersonation`/`startImpersonation` store a partial `User` via `as unknown as User` double-cast — `frontend/src/lib/stores/auth-store.ts:105–120`. Type as `Partial<User>` and merge, or trigger `/auth/me` after the swap. (Also violates the repo no-`any`/strict-typing guidance in spirit.)
- **L13** `[AGENT-REPORTED]` `shared/signing/README.md` stale vs the new breaking API (`NewSigner` now returns error) and new signed-payload format. Update docs (library not yet wired to prod, low impact).
- **L14** `[AGENT-REPORTED]` TLS overlay uses nginx `configuration-snippet` annotation (often disabled by default since ingress-nginx v1.9) — `deployments/k8s/overlays/tls/ingress.yaml:38–44`. Confirm `allow-snippet-annotations` or use built-in WebSocket support. (Opt-in overlay, not wired into base.)

---

## 🔎 INVESTIGATE before acting — do NOT blind-fix `[INVESTIGATE]`

These are completeness-critic gaps that need analysis to confirm scope/impact before any change.

- **I1 — Cookie-authenticated owner overlay WebSocket CSRF surface.** `extractWSAuthToken` (`services/api-gateway/handlers/websocket.go`) adds a fallback that authenticates the owner overlay WS via the ambient httpOnly `access_token` cookie. The only CSRF defense on `/ws` is `checkOrigin` against `WEBSOCKET_ALLOWED_ORIGINS`, which in prod includes wildcard `moz-extension://*` (and `checkOrigin` returns true on empty Origin); the gateway's `OriginCheck` middleware covers `/api/v1` but **not** `/ws`. **Check:** can an attacker-controlled extension page open the streamer's owner socket with the victim's cookie, and what privileged actions does that socket allow (send-from-monitor, moderation)? Decide whether to tighten the WS origin allowlist or require a non-cookie token for the owner WS.
- **I2 — Viewer OAuth double-callback resilience after `Get+Del → GetDel` (L5).** `viewer_auth.go` `HandleTwitchCallback`/`HandleYouTubeCallback`/`HandleKickCallback` switched to atomic `GetDel` **without** the idempotency-tombstone replay path that `platform_auth_v2.HandleCallback` has. **Check:** were benign duplicate viewer callbacks (iOS Safari prefetch, Google multi-code) previously tolerated and now hard-fail with "invalid/expired state"? If so, add an equivalent tombstone path. Watch the "Invalid or expired state" error rate after deploy.
- **I3 — Impersonation token/refresh/blacklist lifecycle.** Impersonation tokens always carry roles `[user,admin]` (`shared/auth/jwt.go:411`); `HandleStopImpersonation` issues a fresh admin cookie but does **not** blacklist the impersonation JWT or rotate the refresh cookie. **Check:** behavior when an admin refreshes *while* impersonating; whether a leaked impersonation token stays valid to expiry; whether impersonating a non-admin grants `/admin/*` via the always-admin roles claim. Decide on scoping impersonation tokens to the target's roles and/or blacklisting on stop.
- **I4 — `ALLCHAT_INNERTUBE_KEY` env plumbing consistency.** `client.go`/`resolver.go` now read the key from env at package-init. **Check:** both `youtube-listener-innertube` and `overlay-manager` deployments get the **same** value (or both fall back to the public key) in `deployments/` and `caesar-deployment/`, otherwise resolve vs listen could use different keys.

---

## Non-issues — DO NOT change

- **GitGuardian "Google API Key" alert is a FALSE POSITIVE.** `AIzaSyAO_FJ2SlqU8Q4STEHLGCilw_Y9_11qcW8` is the **public** YouTube InnerTube web-client key, already on `main` in 7+ places. The PR only renamed it to `defaultInnertubeAPIKey` with an env override (`resolver.go`, `innertube/client.go`). Not a secret — do not "rotate" or remove it.
- **Supply chain is clean.** All `go.sum` changes verified against `sum.golang.org`; all upgrades, no downgrades/typosquats. The deleted `frontend/frontend/package.json` was a stray nested duplicate (real `frontend/` untouched). The committed `services/youtube-listener-innertube/youtube-listener-innertube` binary is pre-existing on `main` (hygiene, not introduced here).
- **No malicious code anywhere** — the whole-diff sweep + supply-chain pass found no backdoors, exfiltration, credential logging, or obfuscation. The PR is net-negative on secret exposure.
- **`OriginCheck` fail-open on absent Origin/Referer is intentional and sound** given `SameSite=Lax` cookies — don't "harden" it into fail-closed without understanding the Lax dependency (it would break non-browser API clients).

---

## Suggested fix order

1. **B1** (un-break CI) — trivial, unblocks everything else.
2. **C1 + C2 + C5** (restore + test streamer login) — the core feature is broken without these.
3. **B2** (restore metrics scraping).
4. **M1–M4** (security-relevant correctness).
5. **M5–M8** (infra/robustness; M6 needs staging validation).
6. **L1–L14** opportunistically; **M9** split as a follow-up coordination step.
7. **I1–I4**: investigate, then file/fix as warranted.

## Verification gate before merge
- `go build ./...` + `go vet` clean in edited modules; `gofmt -l` clean.
- `cd frontend && npx tsc --noEmit && npm run build` clean.
- New E2E test (C5) passes; `build-and-push (migrations,...)` green.
- Manual: streamer OAuth login → cookie set → authed request → access expiry → auto-refresh succeeds (no forced logout) → logout clears cookies.
- Prometheus api-gateway targets UP.
- Rebase onto `main` (resolve `CONFLICTING`).
