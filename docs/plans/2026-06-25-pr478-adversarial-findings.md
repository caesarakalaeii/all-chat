# PR #478 Adversarial Review — Confirmed Findings (full detail)

Summary: {"total_findings": 31, "confirmed": 30, "refuted": 1, "by_severity": {"BLOCKING": 1, "HIGH": 2, "MEDIUM": 12, "LOW": 15}}

---

## ✅ Resolution status (implemented 2026-06-25)

All 30 confirmed findings fixed on `security/audit-hardening` (working tree). The 3
originally-deferred items (#12/#13/#18) were pre-existing bugs in files PR #478 didn't
touch; they were implemented in a second pass on request and are noted as such below.

| ID | Sev | Status | What landed |
|----|-----|--------|-------------|
| #1 | BLOCKING | ✅ | `client.ts` no longer self-redirects on the `/api/v1/auth/me` probe → no more logged-out reload loop |
| #2,#5,#6,#21 | HIGH/MED/LOW | ✅ | `HandleRefresh` rewritten: resolve user up front from the reuse-key, provider-aware refresh dispatch (Twitch/YouTube/Kick — fixes YouTube/Kick force-logout), pre-seed new key + rotate cookie before fallible DB write, transient Redis/DB → 503 (not logout), impersonation token → 409 instead of silent admin revert. New tests + nil-pool guard on `GetByID`. |
| #3 | HIGH | ✅ | CSP `script-src`+`frame-src` allow the Twitch Embed SDK/player (credits route); `frame-src 'self'` keeps the editor preview iframe working |
| #4 | MED | ✅ | `middleware.SetLogger(log)` wired in auth/overlay/moderation/share/payment services |
| #7 | MED | ✅ | `ImpersonateUser` rejects chained impersonation (audit-trail laundering) |
| #8 | MED | ✅ | Cookie-auth owner WS requires strict first-party Origin (FRONTEND_URL), not the extension-wildcard WS list |
| #9,#22 | MED/LOW | ✅ | PKCE verifier TTL 10m→30m (matches state); diagnostic log on non-PKCE fallback |
| #10 | MED | ✅ | auth-success popup posts the viewer JWT to an explicit opener-origin allowlist (restores extension login) |
| #11 | MED | ✅ | Theme Google-Fonts `@import` rewritten to the same-origin `/api/fonts/css` proxy (multi-family support + allowlist expanded); no CSP/DSGVO regression |
| #14 | MED | ✅ | Auth limiter is per-endpoint (`MiddlewareScoped`) + default raised 5→20 |
| #15 | MED | ✅ | Auth limiter fails CLOSED on Redis error (`FailClosed`, env-overridable) |
| #16 | LOW | ✅ | `OriginAllowed` wildcard match is host-boundary-safe (`https://allch.at*` no longer matches `…​.evil.com`) |
| #17 | LOW | ✅ | `AuthCookieForward` clears client-supplied `X-Access-Token`/`X-Refresh-Token` before setting from cookies |
| #19 | LOW | ✅ | Owner/viewer WS consults the logout blacklist before accepting a token |
| #20 | LOW | ✅ | Dedicated `IMPERSONATION_EXPIRY_HOURS` (default 2h), not the 24h session TTL |
| #23 | LOW | ✅ | Editor `EMBED_READY` receiver validates `event.origin` |
| #24,#25 | LOW | ✅ | Shared `RedactQuery` scrubs token/tts_token/access_token/refresh_token/code/state on ALL routes, both logging middlewares |
| #26,#27 | LOW | ✅ | HMAC signing uses fixed-width per-field framing (no `\|` cross-field collision); replay-window wording corrected |
| #28 | LOW | ✅ | Base `allow-monitoring-scrape` NetworkPolicy for the monitoring namespace |
| #29 | LOW | ✅ | support-bot main container `runAsNonRoot: true` (init containers documented as root) |
| #30 | LOW | ✅ | `resolver.go` reads capped with `io.LimitReader(…, 10<<20)` |
| #12 | MED | ✅ (pre-existing) | key-rotator: byte-only fast-path replaced — new `MultiKeyEncryptor.DecryptStringWithKid` reports the authenticating kid, so "already current" requires a real current-kid decrypt (1/256 collision blobs now re-encrypted). Deterministic regression test added. |
| #13 | MED | ✅ (pre-existing) | key-rotator: YouTube v0 plaintext now encrypted directly (mirrors kick) + per-row `encryption_version`; DB-backed test passes against Postgres |
| #18 | LOW | ✅ (pre-existing) | Viewer logout blacklists the viewer JWT (handler reads X-Access-Token/Authorization; gateway adds AuthCookieForward; AppNav calls `viewerApi.logout()` before clearing local state) |

**Refuted (correctly, no action):** OriginCheck/CORS 403 on `www.allch.at` — prod serves the apex only, the premise didn't hold.

**Verification gate (2026-06-25):** `go build`+`go vet` clean across shared, shared/signing, shared/ratelimit, api-gateway, auth-service (incl. cmd/key-rotator), overlay-manager, moderation/share/payment; `gofmt -l` clean; Go tests green (auth-service handlers incl. new refresh/impersonation/transient-Redis tests, key-rotator incl. new #12 colliding-first-byte + #13 YouTube-v0 DB tests passing against a real Postgres testcontainer, gateway handlers/middleware incl. new WS #8 test, shared middleware/signing/ratelimit/encryption incl. new collision + host-boundary tests); `npx tsc --noEmit` + `npm run build` clean; `kubectl kustomize deployments/k8s/base` renders 42 objects.

---


## #1 [BLOCKING] client.ts redirects to '/' on every non-reauth 401, causing an infinite reload loop on the public landing page for all logged-out users
- dimension: frontend-auth | verdict: confirmed | kind: regression-from-fix
- location: frontend/src/lib/api/client.ts:131-135 (redirect) driven by auth-store.ts:86 (unconditional getMe) and HomeClient.tsx:181-183 (init on landing); gateway 401 body at services/api-gateway/middleware/auth.go:34

**Scenario:** A logged-out user (the majority of visitors) opens https://allch.at/. page.tsx renders HomeClient, whose useEffect calls useAuthStore().init(). init() calls authApi.getMe() → apiClient.get('/api/v1/auth/me') → this.fetch('/api/v1/auth/me'). No access cookie exists → 401, error !== 'reauth_required'. tryRefresh() POSTs /api/v1/auth/refresh with no refresh cookie → 401 → returns false. client.ts then runs `window.location.href = '/'`. The browser fully reloads '/', re-mounts HomeClient, re-runs init(), gets 401 again, redirects again. The landing page never renders its login buttons; the user is stuck in a reload loop and can never sign in. The same funnel hits ProtectedRoute pages (e.g. /dashboard): logged-out → getMe() 401 → href='/' → loop on '/'.

**Fix:** Stop client.ts from self-redirecting on the auth-probe path; let init()/ProtectedRoute own navigation. Concrete, minimal implementation:

In frontend/src/lib/api/client.ts, gate the redirect so the /auth/me probe never triggers a navigation. Replace the block at lines 131-135:

        // refresh failed (or retry exhausted) — bounce to login
        if (typeof window !== 'undefined') {
          window.location.href = '/'
        }
        throw new ApiError(401, errorValue || 'Unauthorized', { error: errorValue || 'Unauthorized' })

with:

        // refresh failed (or retry exhausted). Do NOT navigate for the auth probe
        // (/api/v1/auth/me) — init() runs it on EVERY page (incl. the public
        // landing page) and a self-redirect to '/' creates an infinite reload
        // loop for logged-out visitors. Let the caller (init() catch -> user:null,
        // ProtectedRoute -> router.push('/')) drive navigation instead.
        if (typeof window !== 'undefined' && !endpoint.startsWith('/api/v1/auth/me')) {
          window.location.href = '/'
        }
        throw new ApiError(401, errorValue || 'Unauthorized', { error: errorValue || 'Unauthorized' })

init() at auth-store.ts:95-97 already catches and sets user:null cleanly, and ProtectedRoute.tsx:60-62 already does router.push('/') when user is null, so authenticated-but-expired users on protected pages still get bounced correctly via the SPA router (no full reload loop). 

Also fix/replace the stale landing E2E (frontend/tests/e2e/landing.spec.ts) to mock GET /api/v1/auth/me -> 401 {"error":"missing authorization header"} and POST /api/v1/auth/refresh -> 401, then assert the page renders the login buttons and does NOT navigate/reload (e.g. assert "Sign in with Twitch" is visible and page.url() stays '/').

(An equally valid alternative is to remove the window.location.href='/' from client.ts entirely and rely solely on ProtectedRoute/logout for navigation; the endpoint-gated version above is the smaller, lower-risk change.)

**Reasoning:** Traced the full logged-out flow end-to-end at HEAD and it holds exactly as claimed.

CONTROL FLOW (current code):
1. HomeClient.tsx:181-183 — useEffect calls init() on mount of '/'. page.tsx renders HomeClient unconditionally (no auth gate), so EVERY visitor to https://allch.at/ runs init().
2. auth-store.ts:80-98 — init() now UNCONDITIONALLY calls authApi.getMe() (the diff removed the pre-PR `const token = localStorage.getItem('jwt_token'); if (!token) { set({loading:false}); return }` early return). getMe() -> apiClient.get('/api/v1/auth/me') -> this.fetch('/api/v1/auth/me').
3. Gateway: for a logged-out visitor there is no access_token cookie. CookieToBearer (cookie_to_bearer.go:31-35) finds no cookie -> sets no Authorization header. JWTAuth (auth.go:33-37) returns 401 with body {"error":"missing authorization header"}.
4. client.ts:99-116 — status===401 and endpoint is not /refresh or /login, so it parses errorValue="missing authorization header", which is !== 'reauth_required'.
5. client.ts:118 — tryRefresh() POSTs /api/v1/auth/refresh; logged-out visitor has no refresh cookie -> gateway 401 -> r.ok false -> returns false.
6. client.ts:131-133 — refresh failed, so `window.location.href = '/'` executes. Assigning window.location.href to the current URL triggers a full-page navigation/reload (this is real browser behavior for a non-hash URL, even when identical).
7. Reload re-mounts HomeClient -> init() -> getMe() -> 401 -> redirect again => unbounded full-page reload loop. The login buttons (rendered in the `!isLoggedIn` branch, HomeClient.tsx:323-389) never get a chance to be used; the page reloads before the user can click.

This is a genuine regression introduced by the H3 cookie migration: pre-PR, init() short-circuited for logged-out users and never reached the client.ts redirect path. The same funnel hits every ProtectedRoute page (ProtectedRoute.tsx:49-53 calls init()), so /dashboard etc. also loop on '/'.

NOT MITIGATED: the reauth_required carve-out does not apply (the gateway returns "missing authorization header", not "reauth_required"). The retried-guard doesn't help (it gates the retry, not the redirect). The landing E2E test (frontend/tests/e2e/landing.spec.ts) is stale — it asserts h1=="All-Chat" and "Currently in beta", text that does not exist in the current HomeClient, so it isn't even exercising this page and would not catch the loop. No test mocks /auth/me for the logged-out landing path.

Severity BLOCKING stands: the public landing page is unusable for logged-out users (the majority of traffic) and they cannot sign in at all.

--------------------------------------------------------------------------------

## #2 [HIGH] HandleRefresh: transient failure AFTER Twitch rotation permanently locks the user out (L2 fix is incomplete — only the upstream-refresh step is protected)
- dimension: refresh-rotation-reuse | verdict: confirmed | kind: incomplete-fix
- location: services/auth-service/handlers/auth_handler.go:607-612 (GetUserInfoTwitch 500), :615-621 (GetByTwitchID ClearAuthCookies+401), :624-629 (UpdateTokens 500) — all return after the OLD key was GetDel-consumed at :565 and before the NEW key seed at :633 / cookie rotation at :648

**Scenario:** User's access JWT expires; frontend client.ts fires POST /auth/refresh. auth-service GetDel-consumes refresh_token:<oldhash> (line 565), then calls Twitch RefreshToken which succeeds and returns a fresh refresh token. At that instant Twitch's userinfo endpoint returns a 503 (or the Postgres primary is mid-failover and GetByTwitchID/UpdateTokens errors). Handler returns 500 (or 401+ClearAuthCookies for the DB-lookup case). The new refresh token Twitch issued is discarded; the client's cookie still has the old (now-dead, reuse-key-deleted) token. On the user's next /refresh, GetDel misses -> 401 'Refresh token already used or invalid' + ClearAuthCookies -> the user is logged out and cannot recover without a full re-login. During a Twitch userinfo outage or a CNPG failover this hits every streamer whose JWT happens to expire in that window.

**Fix:** In services/auth-service/handlers/auth_handler.go HandleRefresh: seed the NEW refresh token's reuse key AND rotate the cookie immediately after the successful Twitch RefreshToken call (right after line 604), BEFORE the userinfo/DB/JWT steps, so the client always holds a token whose reuse key exists and any downstream failure is retryable.

Concretely, insert after line 604 (the closing `}` of the `if err != nil` block following RefreshToken):

	// Seed the NEW refresh token's reuse key and rotate the cookie up front so a
	// transient downstream failure (userinfo 5xx, DB blip, JWT) leaves the client
	// holding a token that is still valid for retry, not a dead/keyless one.
	newRTKey := "refresh_token:" + refreshTokenHash(token.RefreshToken)
	if err := h.redis.Set(c.Request.Context(), newRTKey, "1", 14*24*time.Hour).Err(); err != nil {
		h.logger.Warn("Failed to pre-seed new refresh-token reuse key", zap.Error(err))
	}
	// Note: do NOT ClearAuthCookies on the GetByTwitchID branch for a generic DB error.

Then:
  1. Remove the now-duplicate reuse-key seed at lines 633-636 (it is superseded by the pre-seed above; or keep one and drop the other — must not seed twice with conflicting values, currently it Sets user.ID, the pre-seed should match: use `payload`/user.ID once user is known, or just "1").
  2. At line 615-621 (GetByTwitchID error) do NOT call ClearAuthCookies + 401 on a generic DB error — return 503 retryable instead, reserving the 401/clear path for a genuine sql.ErrNoRows ("user gone"). Distinguish with `errors.Is(err, pgx.ErrNoRows)`.
  3. Rotating the cookie (SetAuthCookies with token.RefreshToken) before the JWT step requires the JWT to already exist; simplest correct ordering: seed newRTKey immediately after 604 (above), keep SetAuthCookies at the end (648), and make the 607/624/639 error branches restore by writing newRTKey (done by the pre-seed) so the cookie's existing old token isn't the problem — but since the cookie still holds the OLD token on those 5xx paths, also call auth.SetAuthCookies(c, <old-or-current-jwt>, token.RefreshToken, ...) on those error branches, OR (cleaner) move SetAuthCookies(c, ..., token.RefreshToken, ...) to right after JWT generation and ensure newRTKey is seeded first. Net rule: by the time any 5xx is returned, the refresh token in the client's cookie MUST have a live reuse key.

**Reasoning:** Read the current HandleRefresh at HEAD (services/auth-service/handlers/auth_handler.go:540-656). The flow is exactly as the reviewer describes. Line 565 `GetDel(rtKey)` atomically consumes the OLD refresh token's reuse key. The L2 fix (lines 577-604) ONLY restores that key when the Twitch `RefreshToken` call itself fails transiently (the `else` branch at 594-603 does `redis.Set(rtKey,...)` and returns 503). Once `h.twitchOAuth.RefreshToken` SUCCEEDS (line 576), Twitch (a confidential client per oauth/twitch.go:54 using config.TokenSource) has issued a new token; the new refresh token lives only in the local `token` var. The four subsequent steps then fail-without-recovery:
  - 607 GetUserInfoTwitch err -> 500, no key restore, no new-key seed (633 unreached), no SetAuthCookies (648 unreached). Cookie still holds OLD token whose reuse key is now gone.
  - 615 GetByTwitchID err -> ClearAuthCookies + 401. Terminal regardless.
  - 624 UpdateTokens err -> 500, same as 607: old token kept in cookie, key gone, new-key seed at 633 unreached.
  - 639 GenerateTokenWithKid err -> 500, SetAuthCookies at 648 unreached.
The new-token reuse key is seeded only at line 633-636 and the cookie rotated only at line 648, both AFTER all four failure points. So on any post-rotation transient failure the client's cookie keeps a refresh token with NO Redis reuse key. The very next POST /auth/refresh hits GetDel(565) -> miss -> classified as theft -> 401 + ClearAuthCookies(570) -> forced re-login. This is a genuine REGRESSION introduced by this PR: merge-base HandleRefresh (verified via git show) had no GetDel/reuse key at all, so a transient post-rotation 5xx was simply retryable. The L2 fix is therefore incomplete — it guards only the upstream-refresh step, leaving the four downstream steps able to brick the session, which is precisely the lockout L2 was meant to prevent. Scenario is narrow (streamer's JWT must expire during a Twitch-userinfo outage or CNPG failover window) hence HIGH, not BLOCKING. Confirmed.

--------------------------------------------------------------------------------

## #3 [HIGH] New CSP blocks Twitch Embed SDK + player iframes on the credits/clips overlay route
- dimension: frontend-xss-redirect-csp | verdict: confirmed | kind: regression-from-fix
- location: frontend/next.config.js:62 (script-src, missing embed.twitch.tv) and lines 60-69 (cspBase has no frame-src), applied to the credits route via the /overlay/:id* entry at frontend/next.config.js:101-104; feature usage at frontend/src/app/overlay/[id]/credits/page.tsx:250 (Script src) and :119 (window.Twitch.Embed)

**Scenario:** Streamer enables credits/clips on an overlay and adds /overlay/<id>/credits as an OBS browser source (OBS CEF enforces response CSP headers). The route matches the second headers() entry source:'/overlay/:id*'. The browser blocks the <Script src="https://embed.twitch.tv/embed/v1.js"> load because embed.twitch.tv is not in script-src; onLoad never fires, setTwitchReady(true) is never called, new window.Twitch.Embed(...) throws/never runs, and no clips ever play. Even if the script were allowed, the Twitch player <iframe> it injects (player.twitch.tv / embed.twitch.tv) is blocked because there is no frame-src and default-src is 'self'. The credits feature silently dies in production.

**Fix:** In frontend/next.config.js, extend cspBase so the Twitch Embed SDK and its player iframe are allowed. Change line 62 to add embed.twitch.tv to script-src, and add a frame-src directive permitting Twitch player domains. Concretely, edit the cspBase array (lines 60-69):\n  "script-src 'self' 'unsafe-inline' https://embed.twitch.tv",\nand add a new entry:\n  "frame-src https://embed.twitch.tv https://player.twitch.tv https://www.twitch.tv https://clips.twitch.tv",\nKeep these in the shared cspBase so BOTH the /:path* (line 92) and /overlay/:id* (line 104) entries inherit them (the credits route matches /overlay/:id*). connect-src already allows https: so no change needed there. After editing, load /overlay/<id>/credits in a browser/OBS and confirm the SDK loads and a clip plays end-to-end under the CSP. Note: keeping the directives in cspBase widens script-src/frame-src for all routes; that is acceptable since Twitch embed is first-party-trusted and the alternative (a third headers() entry scoped to /overlay/:id*/credits) is more complex — but if minimizing surface is desired, add a dedicated headers() entry for source:'/overlay/:id*/credits' carrying the Twitch-extended CSP and leave cspBase unchanged.

**Reasoning:** The PR (M10) introduces a per-route CSP in frontend/next.config.js. cspBase (line 60-69) = "default-src 'self'; ...; script-src 'self' 'unsafe-inline'; ..." with NO embed.twitch.tv in script-src and NO frame-src/child-src directive (so iframes fall back to default-src 'self'). The /overlay/:id* entry (line 101-106) applies cspBase + frame-ancestors 'none'. I confirmed there is no other CSP source: no middleware.ts, no http-equiv meta tag, no per-page header — next.config.js is the only emitter (grep found only the two lines 92 and 104).\n\nThe credits route is real and pre-existing: frontend/src/app/overlay/[id]/credits/page.tsx existed at merge-base c1cfd7f8 (git cat-file -e confirmed EXISTED AT MERGE-BASE), so the PR did NOT add it — this is a regression of the new CSP against a shipping feature. Line 250 loads <Script src=\"https://embed.twitch.tv/embed/v1.js\" onLoad={() => setTwitchReady(true)} strategy=\"afterInteractive\" />; line 119 calls new window.Twitch.Embed(...) which injects a player.twitch.tv/embed.twitch.tv iframe.\n\nRoute matching: Next.js source:'/overlay/:id*' is a catch-all that matches /overlay/<id>/credits. Next applies all matching headers() entries; for the duplicate Content-Security-Policy key the later (overlay-specific) entry wins, so the strict overlay CSP governs the credits route.\n\nFailure: under this CSP the browser blocks the embed.twitch.tv script (absent from script-src), so onLoad/setTwitchReady(true) never fires and window.Twitch is undefined -> Embed() never runs -> no clips. Even if the script loaded, the injected Twitch player iframe is blocked (no frame-src; default-src 'self'). OBS CEF enforces response CSP headers, so this breaks in production. connect-src already has https: so XHR/WS to Twitch is fine — the gaps are precisely script-src and frame-src, exactly as the reviewer states. Severity HIGH (fully breaks the credits/clips feature for any streamer using it) rather than BLOCKING (not the auth/login path).

--------------------------------------------------------------------------------

## #4 [MEDIUM] L1 fix incomplete: blacklist Redis-error logging is wired only in api-gateway; 5 backend services still drop it via the nop logger
- dimension: jwt-revocation | verdict: confirmed | kind: incomplete-fix
- location: shared/middleware/auth.go:37 (NewNop default) + :105 (revocationLog().Warn); missing callers at services/auth-service/cmd/main.go:364, services/overlay-manager/cmd/main.go:282, services/moderation-service/cmd/main.go:238, services/share-service/cmd/main.go:185, services/payment-service/cmd/main.go:219 (only services/api-gateway/cmd/main.go:66 calls SetLogger)

**Scenario:** During a Redis disruption (the codebase memory documents that the single-replica allchat/redis is a SPOF that crashloops during nightly kured reboots), every backend that runs its own JWTAuthWithRevocation fails the blacklist `Exists` check. The middleware fails OPEN (revoked/logged-out tokens are accepted) which is the documented trade-off — but for 5 of the 6 services there is also NO log line emitted, so on-call has zero signal that revocation enforcement was bypassed cluster-wide for the duration of the outage. Only the gateway emits the warning.

**Fix:** In each of the 5 services that wire JWTAuthWithRevocation with a non-nil Redis client, add `sharedmiddleware.SetLogger(log)` (or the local middleware alias) once at startup, immediately after the service logger is constructed and before the routes are registered, mirroring services/api-gateway/cmd/main.go:66. Concretely: services/auth-service/cmd/main.go (after `log` is built, before line 364), services/overlay-manager/cmd/main.go (before line 282), services/moderation-service/cmd/main.go (before line 238), services/share-service/cmd/main.go (before line 185), services/payment-service/cmd/main.go (before line 219). Verify each file already imports the middleware package under the same alias it uses for JWTAuthWithRevocation (e.g. `middleware`); api-gateway uses alias `sharedmiddleware`, the others use `middleware` — match the local alias. Optional hardening: at startup, log once that revocation logging is wired, so a future missing call is visible.

**Reasoning:** Verified at HEAD. shared/middleware/auth.go:37 defines `revocationLogger = zap.NewNop()` as a process-level global; line 105 logs the fail-open blacklist Redis error via `revocationLog()`, which returns that no-op logger unless `SetLogger` (line 43) is called in the running process. A full-tree grep (`grep -rn SetLogger services/...`) returns exactly ONE caller: services/api-gateway/cmd/main.go:66. The five other services each wire `JWTAuthWithRevocation(userKeyChain, redisClient)` with a real, non-nil go-redis client (verified each redisClient is constructed via sharedredis.NewClientWithRetry or redis.NewClient, not nil): auth-service/cmd/main.go:364 (+404,417), overlay-manager/cmd/main.go:282 (+341), moderation-service/cmd/main.go:238, share-service/cmd/main.go:185, payment-service/cmd/main.go:219 — and NONE of them call SetLogger anywhere in their tree. Each is a separate `package main` binary, so the gateway's SetLogger only mutates the gateway process's global and cannot reach the other 5 processes. Therefore in those 5 services the blacklist Redis-error path logs to the no-op logger and is silently swallowed — the exact L1 symptom. Severity is MEDIUM, not higher: the fail-open behavior (revoked tokens accepted during a Redis outage) is the documented trade-off and is unchanged by this; the only defect is the missing on-call signal in 5 of 6 services during a Redis disruption.

--------------------------------------------------------------------------------

## #5 [MEDIUM] HandleRefresh: transient DB error in GetByTwitchID force-logs-out a valid user (treats 'DB unavailable' as 'auth failed')
- dimension: refresh-rotation-reuse | verdict: confirmed | kind: incomplete-fix
- location: services/auth-service/handlers/auth_handler.go:615-621

**Scenario:** Streamer's JWT expires during a CNPG primary failover (the project's MEMORY notes nightly kured reboots + DB churn). /auth/refresh consumes the old reuse key, Twitch refresh succeeds, then GetByTwitchID hits a transient 'connection refused'/'context deadline'. Handler runs auth.ClearAuthCookies(c) and returns 401 'User not found'. The streamer is logged out mid-stream despite a fully valid account and refresh chain.

**Fix:** In services/auth-service/handlers/auth_handler.go, replace the blanket error handling at lines 615-621 with a distinction mirroring the L2 split above. Ensure "errors" and the repository package are imported (both already are). Change:

	user, err := h.userRepo.GetByTwitchID(c.Request.Context(), twitchUser.ID)
	if err != nil {
		h.logger.Error("User not found", zap.Error(err))
		auth.ClearAuthCookies(c) // audit H3
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

to:

	user, err := h.userRepo.GetByTwitchID(c.Request.Context(), twitchUser.ID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			// Genuine not-found / deleted account → terminal, clear cookies.
			h.logger.Warn("User not found during refresh", zap.Error(err))
			auth.ClearAuthCookies(c) // audit H3
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			return
		}
		// Transient DB error (CNPG failover, pool exhaustion, ctx deadline). Do NOT
		// clear cookies. The reuse key (line 565) was already consumed and the client
		// still holds the OLD refresh token in its cookie (cookies were not rotated),
		// so restore that key under the original rtKey so the retry isn't read as theft.
		h.logger.Warn("Transient DB error during refresh user lookup — restoring reuse key for retry", zap.Error(err))
		if setErr := h.redis.Set(c.Request.Context(), rtKey, "1", 14*24*time.Hour).Err(); setErr != nil {
			h.logger.Warn("Failed to restore refresh-token reuse key", zap.Error(setErr))
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable, please retry"})
		return
	}

Note rtKey is the already-defined variable from line 564 (the OLD token's key), which is correct to restore because cookies were not rotated, so the client retries with the same refresh token. The new token.RefreshToken is intentionally NOT seeded here since it was never persisted or delivered to the client.

**Reasoning:** Verified at HEAD. auth_handler.go:615-621 calls user, err := h.userRepo.GetByTwitchID(...); on ANY non-nil err it runs auth.ClearAuthCookies(c) + 401 "User not found". The repository (user_repository.go:108-114) returns a DISTINGUISHABLE error: the sentinel repository.ErrUserNotFound only for pgx.ErrNoRows, and a wrapped "failed to get user by Twitch ID: %w" for every transient failure (connection refused, context deadline, pool exhaustion, CNPG failover). The handler ignores this distinction, so a transient DB error is treated identically to a deleted user. By line 615 the path is irreversible-for-this-session: (1) the old reuse key was already consumed by GetDel at line 565; (2) the Twitch RefreshToken call at 576 SUCCEEDED, producing a new valid refresh token that is NOT yet persisted (UpdateTokens is at 624) and NOT seeded into Redis (633); (3) ClearAuthCookies (shared/auth/cookie.go:59-68) deletes both access+refresh cookies with MaxAge:-1. Net effect: a transient Postgres blip at this exact query force-logs-out a streamer with a fully valid account mid-stream. This is precisely the transient-vs-terminal split the fix round (L2) correctly applied to the Twitch RefreshToken call directly above (lines 577-604) but failed to apply to the sibling GetByTwitchID call immediately below — a classic incomplete-fix where one of two adjacent error paths was hardened and the other left fail-closed. Realistic given the project's documented nightly kured reboots / CNPG churn. Calibration to MEDIUM (not higher): narrow timing window (JWT-expiry + refresh-in-flight + DB-down-at-this-query + Twitch-just-succeeded), and impact is a single-session forced re-login that fully recovers on next login — the claim's "unrecoverable" framing is slightly overstated (the account recovers; only that session dies), but the force-logout itself is genuine.

--------------------------------------------------------------------------------

## #6 [MEDIUM] HandleRefresh always calls twitchOAuth.RefreshToken — YouTube-authenticated streamers are force-logged-out on first token expiry
- dimension: refresh-rotation-reuse | verdict: confirmed | kind: new-issue
- location: services/auth-service/handlers/auth_handler.go:576 (unconditional h.twitchOAuth.RefreshToken) and :584-592 (terminal force-logout on 400/401); source of the Google refresh token in the cookie: services/auth-service/handlers/platform_auth_v2.go:861 (RefreshToken: token.RefreshToken on YouTube regular login) via /exchange at auth_handler.go:490

**Scenario:** A streamer signs in with the 'YouTube login' button on the homepage. The /exchange seeds refresh_token:<hash(googleRefreshToken)> and sets the refresh cookie. ~24h later (jwtExpiry) the access JWT expires; the next API call 401s; client.ts calls /auth/refresh; auth-service GetDel-consumes the key, then sends the Google refresh token to Twitch -> 400 invalid_grant -> errors.As matches StatusBadRequest -> terminal path -> ClearAuthCookies + 401. client.ts sees !r.ok, redirects to '/'. The YouTube streamer is logged out and must re-login every single time the JWT expires — they can never maintain a session.

**Fix:** HandleRefresh must dispatch to the correct provider's RefreshToken based on the user's AuthProvider, because the same /refresh endpoint serves Twitch, YouTube, and Kick logins. The handler currently cannot know the provider from the bare refresh token. Cleanest fix: after GetDel-consuming the reuse key (auth_handler.go:565), look up the user/provider before refreshing. Concretely, in services/auth-service/handlers/auth_handler.go around line 576, replace the unconditional `token, err := h.twitchOAuth.RefreshToken(...)` with provider-aware dispatch. Option A (preferred, minimal new state): the C2 reuse key value already stores user.ID (set at auth_handler.go:502 and platform_auth_v2.go via storeStreamerAuthCode). Change the GetDel at line 565 to capture that value (`userID, err := h.redis.GetDel(...).Result()`), then `user, uerr := h.userRepo.GetByID(ctx, userID)` and switch on user.AuthProvider: `"youtube"|"google"` -> h.youtubeOAuth.RefreshToken (oauth/youtube.go:261, which uses google.Endpoint), `"kick"` -> kick provider, default -> h.twitchOAuth.RefreshToken. Then resolve the user by the provider-specific ID (GetByGoogleID for youtube) instead of always GetByTwitchID (auth_handler.go:615), and store the refreshed Google token back via UpdateTokens + reseed the reuse key (633). This requires the seeded reuse key to reliably contain the user ID for ALL login paths — verify storeStreamerAuthCode/exchange always seed with payload.User.ID (auth_handler.go:502 does; confirm the V2 path's reuse-key seeding does too, otherwise add it). Option B if user-ID-in-key is not guaranteed: embed `auth_provider` as a JWT claim and forward the still-valid (or recently-expired) access token as X-Access-Token from the gateway, then read the provider claim in HandleRefresh. Either way, do NOT leave the unconditional Twitch call. If provider-aware refresh is deemed out of scope, the interim must NOT silently force-logout: gate the homepage "Sign in with YouTube" button (HomeClient.tsx:347) off, or make YouTube-streamer sessions explicitly long-lived (skip the auto-refresh terminal-clear for google-auth users by returning 503 instead of clearing cookies). Confirm with the product owner whether YouTube-only streamer login is a supported, session-persisting flow.

**Reasoning:** The bug is real in the current code, though the claim cited the wrong (dead) handler. PRODUCTION FLOW: YouTube streamer login is a real user-facing flow — "Sign in with YouTube" button in frontend/src/app/HomeClient.tsx:347 -> GET /api/v1/auth/youtube/login. The production YouTube callback is wired to platformAuthHandlerV2.HandleCallback(oauth.PlatformYouTube) (cmd/main.go:325), NOT the legacy HandleYouTubeCallback the claim cited (auth_handler.go:350 is route-dead). But the bug is identical in the live path: at platform_auth_v2.go:858-870 the regular-login branch stores `RefreshToken: token.RefreshToken` (a GOOGLE refresh token for a YouTube login) into StreamerAuthPayload. The frontend then calls POST /exchange = legacyAuthHandler.HandleStreamerTokenExchange (cmd/main.go:343), which at auth_handler.go:490 sets the httpOnly refresh cookie to that Google token and seeds the C2 reuse key (501). Later, when the access JWT expires (default JWT_EXPIRY_HOURS=24, cmd/main.go:105), client.ts (frontend/src/lib/api/client.ts:99-135) auto-fires POST /auth/refresh = legacyAuthHandler.HandleRefresh (cmd/main.go:340). HandleRefresh at auth_handler.go:576 UNCONDITIONALLY calls h.twitchOAuth.RefreshToken — no branch on AuthProvider. TwitchOAuth.RefreshToken (oauth/twitch.go:257-268) uses twitch.Endpoint (twitch.go:64) and wraps the error with %w, so the Google token is POSTed to Twitch's token endpoint -> Twitch returns 400 invalid_grant as *oauth2.RetrieveError. errors.As at auth_handler.go:584 matches StatusBadRequest -> terminal branch -> ClearAuthCookies (590) + 401 (591). client.ts sees !r.ok from tryRefresh (158), so `ok` is false at line 119, then window.location.href='/' (133) -> force logout. The reuse GetDel at line 565 already consumed the key, so even a manual retry fails. Net effect: every YouTube-login streamer is force-logged-out at each JWT expiry and can NEVER hold a session beyond ~24h. The merge-base HandleRefresh also only called Twitch, but it returned 401 WITHOUT clearing cookies and there was no client.ts auto-refresh-on-401 cookie flow, so H3 is what makes this fire automatically and permanently — the claim's delta analysis is correct. Severity MEDIUM (not higher): only affects YouTube-login streamers (Twitch is the dominant path), it's a degraded-UX/availability bug not a security hole or auth bypass, and it's recoverable by re-login.

--------------------------------------------------------------------------------

## #7 [MEDIUM] Impersonated session can chain-impersonate and corrupts the DSGVO audit trail (victim recorded as the acting admin)
- dimension: impersonation | verdict: confirmed | kind: new-issue
- location: services/auth-service/handlers/admin.go:167 (adminUserID=c.Get("user_id")) and admin.go:229-232 (audit INSERT) — root cause is the missing guard; supporting: shared/middleware/auth.go:140 (user_id=target) and auth.go:184-191 (AdminOnly roles-only), shared/auth/jwt.go:413 (roles hardcoded user+admin); reachable at api-gateway/cmd/main.go:719 and auth-service/cmd/main.go:422

**Scenario:** 1) Real admin calls POST /api/v1/admin/users/A/impersonate where A is an ordinary non-admin streamer. Gateway issues the impersonation cookie (sub=A, roles=[user,admin], impersonated_by=realAdmin). 2) From that same session, call POST /api/v1/admin/users/B/impersonate. CookieToBearer + JWTAuthWithRevocation set user_id=A, roles=[user,admin]; AdminOnly passes because roles contains admin; OriginCheck passes (same-origin admin UI). 3) ImpersonateUser reads adminUserID = c.Get("user_id") = "A" and inserts into impersonation_audit_log VALUES (A, A's username, B, ...). The accountability record now blames non-admin user A for impersonating B. A real admin can deliberately launder every sensitive impersonation through a chain of victim accounts so the audit log never names them — defeating the stated 'DSGVO Art. 5(2) accountability' purpose of the log. The chained token also has impersonated_by=A, so /stop-impersonation re-fetches user A and (because A.IsAdmin is false) silently degrades the session to a non-admin A.

**Fix:** Block chained impersonation in services/auth-service/handlers/admin.go ImpersonateUser. Immediately after fetching adminUserID at admin.go:167-171, add a guard that rejects requests carrying impersonation provenance: `if c.GetString("impersonated_by") != "" { c.JSON(http.StatusForbidden, gin.H{"error": "cannot impersonate while impersonating"}); return }`. This is the primary, sufficient fix and it preserves audit integrity (the real admin can never be laundered out of the record because a chained call is refused before any token mint or audit INSERT). Defense-in-depth (optional, separate change): also reject impersonation tokens at the auth-service admin group and gateway admin group by adding a small middleware after AdminOnly() that aborts 403 when c.GetString("impersonated_by") != "" — register it in services/auth-service/cmd/main.go:416-418 (admin.Use(...)) and services/api-gateway/cmd/main.go:700-705 (adminAPI.Use(...)). That stops an impersonated non-admin session from reaching ANY /admin/* route, not just /impersonate. Do NOT attempt to "fix" the audit row by reading impersonated_by inside the INSERT — chained impersonation should be forbidden outright, not silently re-attributed.

**Reasoning:** I traced the full chain in the CURRENT code at HEAD and every load-bearing claim holds. (1) jwt.go:413 GenerateImpersonationJWTWithKidExpiry hardcodes roles:=[]string{"user","admin"} and sets UserID=targetUserID (jwt.go:419), Username=targetUsername (jwt.go:421), ImpersonatedBy=adminUserID (jwt.go:423). (2) shared/middleware/auth.go:140-141 sets c user_id = claims.UserID (the TARGET for an impersonation token) and username = claims.Username (the target's username). impersonated_by is only stashed into context (auth.go:147), never consulted by any guard. (3) AdminOnly() (auth.go:184-191) only scans roles for "admin", so an impersonation token of a non-admin victim passes AdminOnly at BOTH the gateway admin group (api-gateway/cmd/main.go:699-705, route :719) and the auth-service admin group (auth-service/cmd/main.go:416-422, both .Use(AdminOnly())). (4) ImpersonateUser reads adminUserID=c.Get("user_id") (admin.go:167) and writes the DSGVO audit row INSERT INTO impersonation_audit_log (admin_user_id, admin_username,...) VALUES (adminUserID, adminUsername,...) at admin.go:229-232 — i.e. for a chained call it records the impersonated victim A (not the real admin) as the acting admin, since user_id/username resolve to A. I grepped the whole tree: the only consumers of impersonated_by/ImpersonatedBy are HandleStopImpersonation (admin_impersonation.go:55, restore path) and /me (auth_handler.go:684, display only). There is NO chained-impersonation/depth guard anywhere (grep for chain/depth/already impersonat returned nothing relevant). The fix commit d577723f added the H3 cookie+Redis stash plumbing but introduced no guard. Consequences are all real: (a) accountability laundering — a malicious/compromised admin chains impersonation through victim accounts so every audit row names an innocent non-admin; (b) audit-log integrity corruption even in benign chained flows; (c) the chained token's ImpersonatedBy=A means /stop-impersonation fetches A, and since A.IsAdmin=false (admin_impersonation.go:79-84) it silently mints a non-admin token. Severity adjusted to MEDIUM, not HIGH: the action requires an actor who is ALREADY a real admin (or an existing admin-issued impersonation session); it is not a privilege-escalation/auth-bypass reachable by an ordinary unprivileged user. The genuine harm is defeat of the stated DSGVO Art.5(2) accountability/integrity of the audit trail by a trusted-but-malicious admin — security-relevant correctness, not external exploitation.

--------------------------------------------------------------------------------

## #8 [MEDIUM] I1 fix incomplete: cookie-authenticated owner WebSocket trusts any browser extension via moz-extension://* / broad wildcard origin
- dimension: gateway-ws-proxy | verdict: partial | kind: incomplete-fix
- location: services/api-gateway/handlers/websocket.go:417 (originAllowedForWS defers non-empty Origin to OriginAllowed) + shared/middleware/origin_check.go:54-57 (moz-extension://* unrestricted prefix wildcard) + caesar-deployment/apps/workloads/all-chat/configmap.yaml:55 (moz-extension://* in WEBSOCKET_ALLOWED_ORIGINS); cookie-auth entry point websocket.go:101-103

**Scenario:** A streamer is logged into https://allch.at (access_token cookie set, HttpOnly Secure SameSite=Lax) and has any Firefox extension installed that holds host permission for allch.at (broad-permission extensions are extremely common, and a compromised/malicious extension is a realistic threat for streamers). The extension's background/content page (Origin: moz-extension://<attacker-uuid>) opens `new WebSocket('wss://allch.at/ws/overlay/<overlayId>')`. The browser attaches allch.at's cookies because the extension has host permission. checkOrigin -> originAllowedForWS: Origin is non-empty so it calls OriginAllowed, which matches the `moz-extension://*` prefix -> allowed. extractWSAuthToken reads the access_token cookie -> ValidateJWTWithKeyChain succeeds with the streamer's identity -> VerifyOverlayOwnership passes -> the socket is established as the OWNER. Impact: (1) the attacker streams the full owner chat feed for that overlay, including non-public overlays (the owner feed is NOT overlay_id-stripped like the viewer feed); replaying `since=0` (default) dumps the entire buffered chat history. (2) Side effect: HandleOverlayConnection calls ActivateSourcesForOverlay (DB write marking sources active) and Subscribe(), which is the trigger that starts/sustains YouTube polling — forcing quota burn and source re-activation the streamer did not initiate, repeatable to amplify cost / cause churn.

**Fix:** Two-part fix. (1) Tighten the cookie-auth WS path so the permissive extension wildcard cannot be used with the ambient cookie. In services/api-gateway/handlers/websocket.go originAllowedForWS (line 406-418), when hasAccessCookie is true require a STRICT first-party exact-match Origin (no `*`/prefix wildcards) instead of the permissive WEBSOCKET_ALLOWED_ORIGINS list. Concretely:

	func originAllowedForWS(allowed []string, firstParty []string, r *http.Request) bool {
		origin := r.Header.Get("Origin")
		hasAccessCookie := false
		if ck, err := r.Cookie(auth.CookieAccessToken); err == nil && ck.Value != "" {
			hasAccessCookie = true
		}
		if hasAccessCookie {
			// Cookie auth is only ever legitimately used by the same-origin
			// monitor view (FRONTEND_URL). Extensions/OBS use bearer.<token>
			// or ?token= and never the cookie. Require strict first-party
			// exact Origin — never the permissive (wildcard) allowlist.
			if origin == "" {
				return false // browser always sends Origin; cookie+empty = CSRF
			}
			for _, fp := range firstParty {
				if origin == fp {
					return true
				}
			}
			return false
		}
		if origin == "" {
			return true // non-browser client (subprotocol/query token)
		}
		return sharedmiddleware.OriginAllowed(allowed, origin)
	}

where firstParty is a new exact-match allowlist derived from FRONTEND_URL / a new WS_COOKIE_ALLOWED_ORIGINS (defaulting to https://allch.at), wired through NewWebSocketHandler and checkOrigin. (2) Independently, pin the Firefox extension to its fixed gecko ID in caesar-deployment/apps/workloads/all-chat/configmap.yaml:55 — replace `moz-extension://*` with the published extension's concrete `moz-extension://<uuid>` (matching how the Chrome entry is already pinned to ioneembbnocfljgbhgfknbbnpfeadacm), in both CORS_ORIGIN and WEBSOCKET_ALLOWED_ORIGINS. Add a regression test in services/api-gateway/handlers/websocket_test.go: Origin moz-extension://<random-uuid> + access_token cookie against a prod-style allowlist must be REJECTED, while Origin https://allch.at + cookie is ACCEPTED, and Origin moz-extension://<uuid> WITHOUT cookie (bearer path) is still accepted.

**Reasoning:** Every code-level fact in the claim is accurate at HEAD. (1) The owner-WS cookie auth path is real: extractWSAuthToken step 3 accepts the httpOnly access_token cookie (websocket.go:101-103), and HandleOverlayConnection validates it via ValidateJWTWithKeyChain -> owner identity -> VerifyOverlayOwnership (websocket.go:181-206), creating a non-viewer connection (NewConnection, isViewer=false at websocket.go:241), so the owner feed is NOT overlay_id-stripped and ActivateSourcesForOverlay + Subscribe fire on connect (websocket.go:244-260). (2) The I1 fix only closed the empty-Origin sub-case: originAllowedForWS returns !hasAccessCookie when Origin=="" but for any non-empty Origin defers unconditionally to sharedmiddleware.OriginAllowed (websocket.go:412-417). (3) OriginAllowed matches `moz-extension://*` as an unrestricted prefix wildcard (origin_check.go:54-57: strings.HasPrefix(origin, "moz-extension://")), so ANY moz-extension://<uuid> passes. (4) Prod config confirms WEBSOCKET_ALLOWED_ORIGINS contains `moz-extension://*` (configmap.yaml:55). (5) /ws/overlay/:overlay_id is registered directly on router (main.go:480); the CORS+OriginCheck stack explicitly skips /ws/ (main.go:438) and OriginCheck is only applied to the /api/v1 groups (main.go:497,595) — so checkOrigin is the SOLE CSRF control on the owner WS. (6) websocket_test.go covers only empty/allowed-exact/disallowed-exact origins (lines 65-112) — no extension-wildcard + cookie case. (7) The prior-round plan doc explicitly flagged this exact wildcard as an OPEN QUESTION (docs/plans/2026-06-24-pr478-review-findings.md:219 "in prod includes wildcard moz-extension://* ... Decide whether to tighten the WS origin allowlist or require a non-cookie token for the owner WS"), and the "fixed" entry (line 45) only hardened the empty-Origin path — it did NOT tighten the allowlist or add a non-cookie requirement. So the fix is genuinely incomplete as the prior round itself acknowledged.

The one factor that bounds the claim below HIGH: the cookie is SameSite=Lax (cookie.go:43). A plain malicious WEBSITE at https://evil.com canNOT exploit this — a script-initiated cross-site WebSocket handshake to wss://allch.at does not carry a SameSite=Lax cookie. The attack only holds for the scenario the claim itself posits: a browser EXTENSION that holds host permission for allch.at, because the browser attaches the host's cookies to extension-originated requests to a granted host regardless of SameSite (this is precisely why the extension entries are in the allowlist). So the realistic threat actor is a malicious/compromised/over-permissioned Firefox extension (any UUID, since moz-extension://* is a blanket wildcard) the streamer has installed — not the open internet. The official Firefox extension has a single fixed gecko ID that could be pinned exactly like the Chrome entry already is, yet the wildcard accepts every extension. Real exploitable gap, narrower precondition than HIGH implies -> MEDIUM.

--------------------------------------------------------------------------------

## #9 [MEDIUM] PKCE verifier TTL (10m) shorter than state TTL (30m) breaks streamer login when consent completes after 10 minutes
- dimension: oauth-pkce-state | verdict: confirmed | kind: regression-from-fix
- location: services/auth-service/handlers/platform_auth_v2.go:544 (Twitch login verifier 10m) and :558 (YouTube login verifier 10m) vs :137 (state 30m); fallback that drops the verifier at :986 (Twitch) and :1002 (YouTube); challenge emitted at services/auth-service/oauth/twitch.go:186 and services/auth-service/oauth/youtube.go:146

**Scenario:** A streamer clicks 'Login with Twitch' (or YouTube). generateAuthURL stores verifier at oauth_verifier:twitch:<csrf> with 10m TTL and the authorize URL carries code_challenge=S256(verifier). The user gets distracted (phone, another tab) and completes the Twitch consent screen 12 minutes later. The browser hits GET /twitch/callback well within the 30m state window, so GetDel(oauth_state:twitch:<csrf>) succeeds and state validation passes. exchangeCodeForToken then does GetDel(oauth_verifier:twitch:<csrf>) which returns redis.Nil (expired at 10m). For Twitch/YouTube the code falls back to the non-PKCE ExchangeCode (lines 985-986, 1001-1002) sending NO code_verifier; since a code_challenge was presented at authorize, the provider rejects the token request per RFC 7636 4.6 -> 'Failed to exchange code' 500, login fails. For Kick the same window already failed pre-PR (line 967 returns an error), but Twitch/YouTube login is a new failure introduced by this PR.

**Fix:** Align the PKCE verifier TTL to the state TTL (30m) at all four store sites in services/auth-service/handlers/platform_auth_v2.go: line 527 (Kick login), line 544 (Twitch login), line 558 (YouTube login), and line 448 (moderation-Kick). Concretely change each `h.redis.Set(ctx, verifierKey, codeVerifier, 10*time.Minute)` to `30*time.Minute` so the verifier and its paired state always co-expire (they are consumed together in exchangeCodeForToken via GetDel). Verifier confidentiality is unaffected: it is random per-login and consumed via GetDel exactly once. Optionally (defensive, secondary), change the Twitch/YouTube fallback branches (lines 982-986 and 998-1002) to hard-error like Kick (line 966-968) when a verifier is expected but absent for the login flow, so a future TTL desync surfaces as a clear error rather than a silent invalid_grant — but the TTL alignment alone fixes the reported failure.

**Reasoning:** Verified the full flow at HEAD (not stale — fix commit d577723f did NOT touch these files; the PKCE-L4 hardening commits 586d63cd/82373b8d are the current state). The git diff against merge-base c1cfd7f8 confirms this PR NEWLY added PKCE to Twitch login (platform_auth_v2.go:540-546) and YouTube login (549-560); merge-base only had Kick + the Twitch chat-scopes flow. Mechanism confirmed end-to-end: (1) generateAuthURL stores the PKCE verifier with a 10m TTL at lines 527 (Kick login), 544 (Twitch login), 558 (YouTube login), and 448 (moderation-Kick); the OAuth state is stored with 30m TTL at lines 137, 322, 456. (2) twitch.go:186 and youtube.go:146 both build the authorize URL with oauth2.S256ChallengeOption(verifier), so code_challenge + code_challenge_method=S256 ARE sent at authorize. (3) On a callback arriving 10-30 min after login initiation, HandleCallback does GetDel(oauth_state) which still succeeds (state alive to 30m), then exchangeCodeForToken does GetDel(oauth_verifier) which returns redis.Nil (expired at 10m); the guard `err == nil && codeVerifier != ""` (lines 982, 998) fails, so it falls back to the non-PKCE ExchangeCode (lines 986, 1002) which sends NO code_verifier. Per RFC 7636 §4.6, when a code_challenge was presented at authorize the token endpoint MUST require/validate code_verifier, so the exchange returns invalid_grant -> HandleCallback returns "Failed to exchange code" 500, login fails. The auth code itself is fresh (its lifetime starts at consent grant, not login initiation), so the verifier expiry — not code expiry — is the determining failure. Wiring confirmed: cmd/main.go:315-325 routes /login, /twitch/login, /youtube/login through HandleLogin->generateAuthURL with withChatScopes=false (PKCE path) and the callbacks through HandleCallback. This is a genuine NEW regression for Twitch/YouTube (pre-PKCE they used GetAuthURL/ExchangeCode and worked for the full 30m state window). For Kick it is a pre-existing latent bug (not a regression) but worth fixing in the same change; note Kick hard-errors on missing verifier (line 966-968) rather than silently falling back, which is actually safer than the Twitch/YouTube fallback.

--------------------------------------------------------------------------------

## #10 [MEDIUM] auth-success postMessage targetOrigin (H4 fix) defaults to window.location.origin, which can silently drop the viewer JWT to a cross-origin extension opener
- dimension: frontend-auth | verdict: confirmed | kind: regression-from-fix
- location: frontend/src/app/chat/auth-success/page.tsx:48-50 (constant) and :114 (success postMessage) / :76 (error postMessage); opener-origin proof in /home/moersener/Hobby/all-chat-extension/src/content-scripts/twitch.ts:644-660 and manifest.json:41-60

**Scenario:** The browser extension (per project memory, /chat/auth-success is a load-bearing extension OAuth catcher) opens the auth-success popup while the opener tab is on twitch.tv. The popup exchanges the code, gets the viewer token, and calls window.opener.postMessage({type:'ALLCHAT_AUTH_SUCCESS', token, streamer}, POST_MESSAGE_TARGET_ORIGIN). With POST_MESSAGE_TARGET_ORIGIN = 'https://allch.at' and the opener on twitch.tv, the message is silently discarded; the extension never receives the token and login appears to hang/fail with no error.

**Fix:** In frontend/src/app/chat/auth-success/page.tsx, the popup must target the OPENER's origin (a platform page), not its own origin. The opener is always one of the four extension content-script origins, so post the message to an explicit allowlist of those origins. The browser delivers only to the window whose origin matches, so iterating the allowlist delivers exactly once to the real opener and to nobody else (no '*').

Replace lines 48-50:
  const POST_MESSAGE_TARGET_ORIGIN =
    process.env.NEXT_PUBLIC_APP_URL ||
    (typeof window !== 'undefined' ? window.location.origin : '*')
with an allowlist of opener origins (the extension content-script hosts plus the app's own origin for the web flow):
  const ALLOWED_OPENER_ORIGINS = [
    'https://www.twitch.tv',
    'https://www.youtube.com',
    'https://studio.youtube.com',
    'https://kick.com',
    ...(typeof window !== 'undefined' ? [window.location.origin] : []),
  ]

Then change BOTH call sites that currently do `window.opener.postMessage(payload, POST_MESSAGE_TARGET_ORIGIN)` (the error case at lines 71-77 and the success case at lines 108-115) to post once per allowed origin, e.g.:
  for (const origin of ALLOWED_OPENER_ORIGINS) {
    window.opener.postMessage(payload, origin)
  }

This keeps the H4 hardening (never '*', JWT only delivered to a known-good origin) while restoring extension login. Optionally also define NEXT_PUBLIC_APP_URL in deployment so the web-app path is explicit, but the allowlist is required regardless because allch.at is NOT the opener origin in the extension flow. Keep the four platform origins in sync with the extension's manifest.json content_scripts matches.

**Reasoning:** The H4 fix is a real regression that breaks all extension viewer login. Verified end-to-end against the actual extension at /home/moersener/Hobby/all-chat-extension.

CURRENT CODE (HEAD): frontend/src/app/chat/auth-success/page.tsx:48-50 defines `POST_MESSAGE_TARGET_ORIGIN = process.env.NEXT_PUBLIC_APP_URL || (typeof window !== 'undefined' ? window.location.origin : '*')`. The PR diff replaced the prior `'*'` targetOrigin at the two `window.opener.postMessage(...)` call sites (line 76 error, line 114 success) with this constant.

NEXT_PUBLIC_APP_URL is undefined: frontend/.env (71 bytes) contains only PUBLIC_API_URL/PUBLIC_WS_URL; grep across all-chat, caesar-deployment, and k8s manifests finds no definition (only the reference in page.tsx and the built .next bundles). So at runtime the constant resolves to window.location.origin = the popup's own origin (https://allch.at).

THE OPENER ORIGIN IS NOT allch.at: The extension's manifest.json injects content scripts on https://www.twitch.tv/*, https://www.youtube.com/*, https://studio.youtube.com/*, https://kick.com/*. The OAuth popup is opened FROM those page contexts via `window.open(resp.data.loginUrl, 'AllChatOAuth', ...)` — see twitch.ts:644, youtube.ts:1860, youtube-studio.ts:294, kick.ts. Therefore the popup's window.opener is a content-script window whose DOCUMENT ORIGIN is https://www.twitch.tv (etc.), and that window holds the inbound `handleAuthMessage` listener (twitch.ts:647-660, youtube.ts:1863-1876).

Browser postMessage rule: the message is delivered only if the recipient window's document origin EXACTLY equals targetOrigin. Here targetOrigin=https://allch.at but opener origin=https://www.twitch.tv, so the browser silently drops ALLCHAT_AUTH_SUCCESS. handleAuthMessage never fires, popup.close() never runs, and the iframe never gets LOGIN_SUCCESS — extension login hangs with no error. This worked under the old '*'.

Why the prior round missed it: the extension's own test (tests/test-postmessage-origin.spec.ts) only asserts the content-script RELAY messages target extensionOrigin (KICK-05b/c); it does not cover the popup→opener direction, and the H4 change lives in the frontend repo, not the extension.

Not a security regression in the bad direction (it tightens, not loosens), but it is a correctness regression that breaks the load-bearing extension viewer-login path across all four platforms. Web-app viewer login (no window.opener) is unaffected, so MEDIUM as claimed is correct (the reviewer's low confidence understated it — this is a definite break, not theoretical).

--------------------------------------------------------------------------------

## #11 [MEDIUM] New CSP blocks @import of Google Fonts in bundled/custom themes — themed overlay fonts silently fall back
- dimension: frontend-xss-redirect-csp | verdict: confirmed | kind: regression-from-fix
- location: frontend/next.config.js:63 (style-src) + :65 (font-src); blocked @import injected at frontend/src/app/overlay/[id]/page.tsx:645 (themeCss resolved at :329-333) and frontend/src/app/overlays/[id]/preview/embed/page.tsx:433-438; source @imports in frontend/src/lib/theme-marketplace/bundled-themes.generated.ts:28 (+8 more)

**Scenario:** A streamer picks the bundled 'Cyberpunk 2077' (or retro/Orbitron/Press Start 2P/etc.) theme. config.theme_id resolves to bundled CSS whose first line is @import url('https://fonts.googleapis.com/css2?family=Rajdhani...'). On the live /overlay/<id> OBS source, themeCss is injected via dangerouslySetInnerHTML (page.tsx:645). The browser blocks the @import because fonts.googleapis.com is not in style-src (CSS @import is governed by style-src, not connect-src), so Rajdhani/Share Tech Mono never load and the overlay falls back to the system default font — a visible, silent regression for every themed overlay. The same break happens in the editor preview embed. Note the existing same-origin /api/fonts/css proxy is only used by ensureGoogleFontLoaded() for visual-settings fontFamily, NOT for theme/custom CSS @import, so the proxy does not cover this path.

**Fix:** Do NOT take reviewer option (a) (whitelisting fonts.googleapis.com/fonts.gstatic.com in next.config.js CSP) — that re-introduces the DSGVO violation the /api/fonts/css proxy was built to prevent (every overlay viewer's IP hits Google; see explicit comments at frontend/src/app/overlay/[id]/page.tsx:63-64 and frontend/src/app/api/fonts/css/route.ts:80-81). Take option (b): rewrite theme @import URLs through the same-origin proxy. Concretely, add a helper in frontend/src/lib/theme-marketplace (or inline) that, before setThemeCss, replaces `@import url('https://fonts.googleapis.com/...')` with `@import url('/api/fonts/css?family=...')` — extract the `family` query params from the googleapis URL and forward them to the proxy (the proxy already rewrites gstatic binaries to /api/fonts/file at route.ts:82-85; verify it forwards arbitrary `family`/`display` params — it currently builds family from a single param, so widen /api/fonts/css/route.ts to pass through the full original family query string). Apply the same rewrite in frontend/src/app/overlays/[id]/preview/embed/page.tsx where themeCss is built (line 433-437). Verify a bundled Cyberpunk overlay renders Rajdhani/Share Tech Mono under the CSP with no console CSP violations and no direct fonts.googleapis.com/fonts.gstatic.com requests in the network panel.

**Reasoning:** Verified end-to-end against HEAD. (1) The CSP is BRAND NEW in this PR — `git show <merge-base>:frontend/next.config.js` has 0 CSP/style-src lines, so this is a genuine regression-from-fix. Current next.config.js line 63 emits `style-src 'self' 'unsafe-inline'` and line 65 `font-src 'self' data:`; neither lists fonts.googleapis.com or fonts.gstatic.com. (2) bundled-themes.generated.ts contains 9 distinct `@import url('https://fonts.googleapis.com/css2?family=...')` URLs (Rajdhani/Share Tech Mono at line 28, plus Orbitron, Press Start 2P, VT323, Inter, Roboto, Nunito, Space Grotesk, Source Code Pro, Monoton — across ~10 bundled themes; claim said 10 distinct, actual distinct is 9, immaterial). (3) On the live overlay, overlay/[id]/page.tsx:329-333 sets themeCss = getBundledTheme(config.theme_id)?.css (the raw bundled CSS, @import as first rule) and line 645 injects it raw via `<style dangerouslySetInnerHTML={{__html: themeCss}}>`. CSS @import is governed by style-src; the browser blocks the googleapis.com stylesheet fetch under the new CSP, and font-src 'self' data: blocks the gstatic binaries. Themed overlays fall back to system fonts. (4) The /api/fonts/css proxy (ensureGoogleFontLoaded, page.tsx:79-89, called only at line 344 for visual_settings fontFamily keys, and only for 10 hardcoded font names in GOOGLE_FONT_NAMES) does NOT process theme @import URLs — confirmed it covers a different path. (5) Same break on the editor embed: overlays/[id]/preview/embed/page.tsx:433-438 resolves the same raw theme css and injects it. SEVERITY: I downgrade HIGH→MEDIUM. This is a real, reproducible functional regression affecting every overlay using any of the ~10 bundled Google-Fonts themes, but it is purely cosmetic (graceful fallback to system font), no security/auth/login/build impact. FIX NUANCE: the reviewer's 'minimal fix (a)' (whitelist googleapis/gstatic in CSP) is WRONG for this codebase — the /api/fonts/css proxy and the explicit comments (page.tsx:63-64, route.ts:80-81) exist specifically to keep viewer IPs away from Google for DSGVO compliance (LG München 'Google Fonts Urteil'). Adding the domains to CSP would re-introduce that exact violation. Option (b) — rewrite theme @import through the same-origin proxy — is the correct fix.

--------------------------------------------------------------------------------

## #12 [MEDIUM] key-rotator fast-path mis-classifies ~1/256 legacy kid-less blobs as already-current and skips them, leaving them un-migrated (data-loss after legacy key retirement)
- dimension: encryption-keyrotation | verdict: partial | kind: new-issue
- location: services/auth-service/cmd/key-rotator/sweeper.go:152 (byte-only fast-path, no AEAD); flaky test at services/auth-service/cmd/key-rotator/sweeper_test.go:254. Both pre-exist PR #478 (introduced commit 776f6501, 2026-05-04; PR diff for sweeper.go is empty).

**Scenario:** Operator runs key-rotator to migrate all token columns to the versioned format so the legacy key (TOKEN_ENCRYPTION_KEY) can be retired. The sweep reports success; ~0.4% of legacy-format user/session/token rows whose nonce[0] coincidentally equals 0x01 are silently classified as 'skipped (already current)' and never gain a kid prefix. The legacy key is later removed from the env (the entire purpose of rotation). Those ~0.4% of rows are now kid-less blobs that can only be decrypted by the now-absent legacy key: DecryptString tries the versioned path (kid byte 0x01 routes to the V-current cipher, AEAD fails because it's actually legacy-key ciphertext) then the legacy chain (empty) -> 'no key in chain authenticated'. Auth-service GetUser/token refresh, youtube-listener oauth/store.go GetToken (returns 'decrypt access token' error -> 5xx), and kick-listener getKickAuthToken all fail for those users: forced logout / broken token refresh / dead listeners, with no recovery path because the plaintext is gone.

**Fix:** In services/auth-service/cmd/key-rotator/sweeper.go, make the fast-path authoritative only when the blob actually authenticates under the current kid. Replace lines 149-154:

    // Fast-path: check if the stored blob is already at the current kid.
    decoded, err := base64.StdEncoding.DecodeString(stored)
    if err == nil && len(decoded) >= 29 && decoded[0] == s.encryptor.CurrentKid() {
        return stored, false, nil // already on current kid
    }

with a decrypt-then-recheck approach (simplest and the rotator is a batch job, so the AEAD cost is negligible) — drop the byte-only fast-path entirely and decide skip-vs-rewrite from the resulting kid:

    // Always decrypt via the chain (versioned + legacy), then re-encrypt only if
    // the blob is not already at the current kid. A byte-only check is unsafe:
    // a kid-less legacy blob whose nonce[0] coincidentally equals CurrentKid()
    // would be mis-classified as already-current and skipped (1/256).
    plaintext, err := s.encryptor.DecryptString(stored)
    if err != nil {
        return "", false, fmt.Errorf("decrypt: %w", err)
    }
    // Determine the stored blob's true kid only after a successful AEAD decrypt.
    decoded, derr := base64.StdEncoding.DecodeString(stored)
    if derr == nil && len(decoded) >= 29 && decoded[0] == s.encryptor.CurrentKid() {
        // Decrypt succeeded AND the kid byte matches the current kid: the versioned
        // path authenticated it (DecryptString tries kid-routed AEAD first), so it
        // is genuinely already current.
        return stored, false, nil
    }
    reencrypted, err := s.encryptor.EncryptString(plaintext)
    if err != nil {
        return "", false, fmt.Errorf("re-encrypt: %w", err)
    }
    return reencrypted, true, nil

(Note: even the post-decrypt kid==CurrentKid recheck is only safe because DecryptString tries the kid-routed AEAD before the legacy chain; if it authenticated under kid 0x01's cipher the blob really is a versioned current-kid blob. A kid-less blob with nonce[0]==0x01 fails that AEAD and falls to the legacy chain, so decoded[0]==0x01 alone is not what gates the skip — the successful current-kid decrypt is.)

Then make sweeper_test.go:241 deterministic: in TestSweeper_EncryptIfNotCurrentKid_LegacyKidless, loop building legacy blobs until nonce[0]==0x01 (mirroring TestMultiKeyEncryptor_FalsePositiveKid) and assert it is STILL re-encrypted (changed==true) and round-trips. This both fixes the flake and guards the regression.

**Reasoning:** The technical bug is REAL, but the claim's "new-issue" classification and HIGH severity are both overstated for this PR.

VERIFIED FACTS:
1. sweeper.go:151-153 (HEAD) does a byte-only fast-path: `if err == nil && len(decoded) >= 29 && decoded[0] == s.encryptor.CurrentKid() { return stored, false, nil }`. There is NO AEAD verification — it trusts decoded[0] alone.
2. The backfill (token-encryption-backfill/main.go) builds a PLAIN AESEncryptor (line 64 NewAESEncryptor) and encryptIfPlaintext (line 231) calls r.cipher.Encrypt → produces a kid-less legacy blob base64(nonce||ct||tag) into users + youtube_oauth_tokens. So legacy kid-less blobs genuinely exist in those tables.
3. The first byte of a kid-less blob is nonce[0], a random byte. In the single-key prod deployment CurrentKid()==0x01, so P(nonce[0]==0x01)==1/256. Those ~0.4% of rows hit the fast-path, return changed=false, are counted RowsSkipped, and are NEVER re-encrypted to the [kid||...] format.
4. Post legacy-key retirement, DecryptString (versioned.go:279-311) routes kid 0x01 to the V1 cipher, strips the byte, AEAD fails (the bytes are really nonce[1:]||ct||tag), then the legacy chain is empty → "no key in chain authenticated". Confirmed un-recoverable forced-logout / broken-refresh for those rows.
5. The test IS flaky: TestSweeper_EncryptIfNotCurrentKid_LegacyKidless (sweeper_test.go:241-262, makeTestEncryptor uses Kid:0x01) builds one random-nonce legacy blob and asserts changed==true; it fails ~1/256 runs when nonce[0]==0x01.

WHY PARTIAL / DOWNGRADE, NOT CONFIRMED-HIGH:
- This is NOT a new issue introduced by PR #478. sweeper.go was added 2026-05-04 in commit 776f6501 (the 14-06 key-rotator sweeper); `git diff c1cfd7f8..HEAD -- sweeper.go` is empty (byte-identical to merge-base), and the PR's only change under cmd/key-rotator/ is a 1-line edit to main.go. The reviewer's kind:"new-issue" is wrong; this is pre-existing latent code the PR does not touch.
- It is latent: it only manifests during a future operator-run key-rotation followed by legacy-key removal. Prod currently runs a single key (kid 0x01) and no rotation has happened, so no data is lost today; nothing in this PR triggers it. It does not break prod auth/login/build now.
- Net: a real correctness/data-loss-on-rotation bug + a flaky unit test, but out of scope for PR #478 and not blocking it. MEDIUM is the honest severity (security-relevant correctness, conditional on a future rotation op).

--------------------------------------------------------------------------------

## #13 [MEDIUM] key-rotator never migrates YouTube v0 plaintext rows despite comments (and token_backfill) claiming it does; rows stay plaintext and inflate the error counter
- dimension: encryption-keyrotation | verdict: confirmed | kind: new-issue
- location: services/auth-service/cmd/key-rotator/sweeper.go:372-373 (encVer scanned, never used) and sweeper.go:378-402 (unconditional encryptIfNotCurrentKid path with no encVer==0 branch); contrast kick at sweeper.go:574. Misleading comment at sweeper.go:360-361. False "supersedes" claim at services/youtube-listener/cmd/token_backfill/main.go:39.

**Scenario:** An operator follows the rotation runbook, sees token_backfill is 'superseded by the sweeper', and runs only key-rotator. Every YouTube row still at encryption_version=0 is decrypt-attempted, fails, and is logged as an error + skipped. Result: (a) YouTube v0 OAuth tokens are left permanently in plaintext in the DB — the security objective (encrypt at rest) is silently not met for those rows; (b) the Errors counter is inflated by the count of all v0 rows, so a real decrypt failure (corrupted/wrong-key ciphertext) is indistinguishable from the expected v0 noise and gets ignored. The data is not corrupted, but the migration silently does not happen and the failure is masked.

**Fix:** Mirror the kick v0 policy in sweepYouTubeOAuthTokens. Add SetEncVersion to the update struct and branch on encVer in the row loop.

1) services/auth-service/cmd/key-rotator/sweeper.go — add field to the struct (currently lines 352-357):
type youtubeTokenUpdate struct {
	UserID        string
	ChannelID     string
	AccessToken   string
	RefreshToken  string
	SetEncVersion int
}

2) Replace the per-row body (currently sweeper.go:378-402, between `s.metrics.RowsScanned["youtube_oauth_tokens"]++` and the batch-flush block) with a branch identical in shape to kick (sweeper.go:574-619):
		if encVer == 0 {
			// v0: plaintext written before encryption rollout — encrypt directly, no Decrypt step.
			newAt, err := s.encryptor.EncryptString(at)
			if err != nil {
				s.logger.Warn("youtube v0 access_token encrypt error",
					zap.String("user_id", userID), zap.String("channel_id", channelID), zap.Error(err))
				s.metrics.Errors["youtube_oauth_tokens"]++
				continue
			}
			newRt, err := s.encryptor.EncryptString(rt)
			if err != nil {
				s.logger.Warn("youtube v0 refresh_token encrypt error",
					zap.String("user_id", userID), zap.String("channel_id", channelID), zap.Error(err))
				s.metrics.Errors["youtube_oauth_tokens"]++
				continue
			}
			batch = append(batch, youtubeTokenUpdate{userID, channelID, newAt, newRt, 1})
		} else {
			newAt, atChanged, err := s.encryptIfNotCurrentKid(at)
			if err != nil { /* existing warn + Errors++ + continue */ }
			newRt, rtChanged, err := s.encryptIfNotCurrentKid(rt)
			if err != nil { /* existing warn + Errors++ + continue */ }
			if !atChanged && !rtChanged {
				s.metrics.RowsSkipped["youtube_oauth_tokens"]++
				continue
			}
			batch = append(batch, youtubeTokenUpdate{userID, channelID, newAt, newRt, encVer})
		}

3) Make flushYouTubeBatch write the per-row version instead of the hardcoded literal — change the UPDATE at sweeper.go:430 from `encryption_version=1` to `encryption_version=$3` and pass yt.SetEncVersion (re-number the existing $3/$4 to $4/$5), exactly as flushKickBatch does at sweeper.go:649-651.

4) Fix the misleading comment at sweeper.go:360-361 to state the v0 direct-encrypt policy (matching the kick comment at sweeper.go:542-549).

5) Add a TestSweeper_YouTubeV0EncryptsDirect test mirroring TestSweeper_KickV0EncryptsDirect (sweeper_test.go:562) to guard this.

6) Optionally remove the false "The new sweeper from Plan 14-06 supersedes this binary" sentence at services/youtube-listener/cmd/token_backfill/main.go:39 (now that the sweeper actually handles v0, the claim becomes true and the comment can stay, but it should not have been there while the sweeper was broken).

**Reasoning:** I traced the v0 path end-to-end in the CURRENT code at HEAD.

1. sweepYouTubeOAuthTokens (sweeper.go:359-420) SELECTs encryption_version and scans it into encVer (line 372-373) but never branches on it — it unconditionally calls encryptIfNotCurrentKid(at) and encryptIfNotCurrentKid(rt) (lines 378, 388). This is exactly the asymmetry the claim describes: kick (sweeper.go:574 `if encVer == 0`) has a v0 direct-encrypt branch; youtube does not; tiktok (sweeper.go:687) filters v0 out with `WHERE encryption_version >= 1`.

2. encryptIfNotCurrentKid (sweeper.go:145-166) base64-decodes the stored value for the fast path; for a v0 plaintext token the decode fails so the fast path is skipped, then it calls DecryptString (line 156) and returns ("",false,err) if DecryptString errors (line 157-158).

3. DecryptString (versioned.go:279-311) base64-decodes first (line 280); on failure returns an error immediately. I empirically verified with `base64.StdEncoding.DecodeString` that realistic v0 tokens fail decode: access token "ya29.a0Af..." (contains '.', '-', '_') and refresh token "1//0g..." (contains '-','_') both return decodeErr=true. Even a no-special-char plaintext fails (length not a multiple of 4, and if it were, AEAD auth would fail against every key). So DecryptString ALWAYS errors on v0 plaintext.

Net: every v0 youtube row hits sweeper.go:379/389 (err != nil) → Errors["youtube_oauth_tokens"]++ + continue. The row is counted as an error and NEVER re-encrypted. This directly contradicts the comment at sweeper.go:360-361 ("encryptIfNotCurrentKid handles both plaintext and versioned ciphertext").

Corroborating evidence the bug is live and untested:
- migrations/006_youtube_token_encryption.sql adds encryption_version SMALLINT NOT NULL DEFAULT 0, so v0 is a real historical state.
- sweeper_test.go has TestSweeper_KickV0EncryptsDirect (kick v0) and TestSweeper_SkipsTikTokV0 (tiktok v0) but NO youtube test at all — youtube_oauth_tokens appears only in the CREATE TABLE (line 115). The broken path was never validated.
- docs/runbooks/secret-rotation.md lists youtube_oauth_tokens among the tables the sweeper handles (line 114), makes the sweeper the canonical migration tool (Step 5, lines 209-242), states the legacy-key-retirement gate as "0 re-encryptions on a second dry run" (lines 239-242), and never mentions token_backfill. token_backfill/main.go:39 says the sweeper "supersedes this binary." So an operator following the runbook runs ONLY the sweeper. v0 youtube rows then (a) stay plaintext at rest — the encrypt-at-rest objective silently fails for those rows; and (b) inflate the per-table errors counter, making a genuine wrong-key/corruption decrypt failure indistinguishable from expected v0 noise. The runbook's own example log (line 555) shows youtube re-encryptions with errors:0, reflecting the false expectation this bug creates.

Note on PR scope: sweeper.go already existed at the merge-base (c1cfd7f8) and its youtube/kick functions are unchanged by PR #478 — the PR only touched token_backfill/main.go (logger + DATABASE_PASSWORD guard) and key-rotator/main.go. So this is a pre-existing latent bug surfaced during this security PR, not a regression introduced by it. It is still real and live.

Severity MEDIUM (not higher): no data corruption (rows stay plaintext and remain runtime-decryptable via the legacy chain, so YouTube polling keeps working); the correct migration tool (token_backfill) still exists and works; new writes are v1 (user_repository.go:377), so v0 rows are historical-only and may already be drained in prod. But the documented operator path is broken and the error-masking is a real monitoring blind spot — security-relevant correctness, MEDIUM.

--------------------------------------------------------------------------------

## #14 [MEDIUM] Auth rate-limiter is a single per-IP bucket shared across login+exchange+refresh+viewer exchanges, not per-endpoint as documented; 5/min/IP locks out shared-NAT users
- dimension: cors-ratelimit-proxy-config | verdict: partial | kind: incomplete-fix
- location: shared/ratelimit/ratelimit.go:103-111 (getClientKey, no path in key); services/api-gateway/cmd/main.go:156 (default 5), 494-495/497/507/533/536/539/542 (single shared authRateLimiter on all auth routes); comment at main.go:153 mislabels it "per-endpoint"

**Scenario:** A normal single streamer login consumes 2 of the 5 tokens within a minute: GET /auth/login (1) then POST /auth/exchange (2). Behind a shared egress IP (CGNAT, corporate/university NAT, mobile carrier — common for a public consumer streaming product), three users logging in in the same minute total 6 > 5 requests, so the third user's /auth/exchange returns HTTP 429 and their login silently fails. Auth-service has no independent rate limiting, so the gateway limiter is the only layer and its 5/min/shared-key budget is far too small for many users sharing one IP.

**Fix:** Make the auth limiter genuinely per-endpoint AND raise the budget so a shared NAT tolerates concurrent users. Concretely in services/api-gateway/cmd/main.go: replace the single shared authRateLimiter.Middleware() on each route with a path-keyed bucket. Lowest-risk option: add a path component to the key. In shared/ratelimit/ratelimit.go getClientKey (line ~103-111), append c.FullPath() to the returned key, e.g. return fmt.Sprintf("%s:ip:%s:%s", rl.cfg.KeyPrefix, c.ClientIP(), c.FullPath()) (and likewise for the user branch) — but that changes the GLOBAL limiter too, so prefer a scoped variant instead: add a method `func (rl *RateLimiter) MiddlewareScoped(bucket string) gin.HandlerFunc` that builds the key as "<KeyPrefix>:<bucket>:ip:<ip>" and wire each auth route with its own bucket name, e.g. authRateLimiter.MiddlewareScoped("login") on lines 494-495, "exchange" on 507, "refresh" on 497, "viewer_exchange" on 533/536/539/542. Then bump the default at main.go:156 from 5 to ~20 (AUTH_RATE_LIMIT_PER_MINUTE default 20) so a single NAT egress IP can support multiple concurrent logins/refreshes while still blocking brute force per endpoint. At minimum, give /auth/refresh its own (more generous) bucket so automatic token refreshes cannot exhaust the interactive-login budget. Update the comment at main.go:153 to match whichever is implemented (it currently mislabels the limiter as per-endpoint).

**Reasoning:** Verified against HEAD. The structural core of the claim is TRUE: shared/ratelimit/ratelimit.go:103-111 (getClientKey) builds the key as "<KeyPrefix>:ip:<ClientIP>" or "<KeyPrefix>:user:<id>" with NO path/route component. A single authRateLimiter instance (KeyPrefix "api_gateway:auth", default 5/min from AUTH_RATE_LIMIT_PER_MINUTE at main.go:156, no override anywhere in caesar-deployment) is attached via authRateLimiter.Middleware() to GET/POST /auth/login (main.go:494-495), POST /auth/refresh (497), POST /auth/exchange (507), and the four viewer exchange routes (533,536,539,542). They therefore all decrement ONE 5/min counter per source IP — it is per-IP-global, NOT per-endpoint. This contradicts both the in-code comment at main.go:153 ("Stricter per-endpoint rate limiter") and the audit recommendation in SECURITY_AUDIT_REPORT.md:101, which explicitly said to use ratelimit.CheckLimitForKey per endpoint. auth-service has no independent login/refresh/exchange rate limiting (only chat_send per-viewer limits exist), so the gateway bucket is the only layer. SetTrustedProxies IS configured (main.go:419, M3 fix) so c.ClientIP() resolves to the real public client IP — meaning under CGNAT/corporate/carrier NAT many distinct users genuinely share one bucket; the shared-NAT lockout risk is real.

WHERE THE CLAIM IS WRONG (why partial, not confirmed): the per-user token-cost math is incorrect. The real frontend streamer login flow (frontend/src/lib/api/auth.ts:34 -> GET /api/v1/auth/{platform}/login, then GET /auth/{platform}/callback, then frontend/src/app/auth/callback/page.tsx:80 -> POST /api/v1/auth/exchange) uses the PLATFORM-SPECIFIC login/callback routes (main.go:510-528) which are NOT rate-limited. Only POST /auth/exchange is rate-limited in that flow = 1 token per login, not the claimed 2. So the generic GET/POST /auth/login routes the claim points at are not on the real login path (they appear only in docs/ADMIN_RBAC.md examples). The "3 users -> 6 > 5 -> third 429s" scenario is therefore wrong; the real threshold is ~5 logins/min per shared IP. The genuinely concerning shared path is /auth/refresh: it fires automatically (timer/401) for every logged-in streamer/admin on the IP and shares the same 5/min bucket, so a busy NAT can exhaust the budget and 429 both refreshes and logins for everyone on that egress IP, force-logging-out sessions. Real but narrower/lower-frequency than the "every third login fails" framing.

--------------------------------------------------------------------------------

## #15 [MEDIUM] Auth (anti-brute-force) rate limiter fails open when Redis is unavailable — removes the only credential-stuffing defense during the known Redis SPOF outages
- dimension: cors-ratelimit-proxy-config | verdict: confirmed | kind: new-issue
- location: shared/ratelimit/ratelimit.go:69-76 (fail-open error branch) and services/api-gateway/cmd/main.go:157-162 (auth limiter constructed without fail-closed); wired onto auth endpoints at services/api-gateway/cmd/main.go:494-507,533-542

**Scenario:** This deployment runs a single-replica, node-pinned Redis that is a known SPOF and crash-loops listeners during nightly kured node reboots (per project memory). During each such Redis outage window the auth rate limiter silently fails open, leaving POST /auth/login and POST /auth/exchange completely unthrottled. An attacker who times credential-stuffing to a Redis blip (or simply triggers/awaits the recurring nightly window) bypasses the M7 brute-force control entirely. Fail-open is defensible for the general 300/min availability limiter, but not for the security-critical anti-brute-force limiter.

**Fix:** Add a fail-closed option to the rate limiter and enable it for the auth limiter only. (1) In shared/ratelimit/ratelimit.go, add to Config (after line 39): `FailClosed bool // when true, deny on Redis error instead of allowing`. (2) Replace the error branch at ratelimit.go:69-76 so that when rl.cfg.FailClosed is true it aborts with 503 instead of c.Next(): keep the Error log, then `if rl.cfg.FailClosed { c.Header("Retry-After", "5"); c.JSON(http.StatusServiceUnavailable, gin.H{"error": "rate limiter unavailable"}); c.Abort(); return }` else fall through to the existing `c.Next(); return`. (3) In services/api-gateway/cmd/main.go at the authRateLimiter config (line 157-162) add `FailClosed: true,` so login/exchange/refresh fail closed when Redis is down; leave the general rateLimiter (line 143-148) fail-open to preserve availability of non-auth traffic. Optionally gate via env (getEnvAsBoolOrDefault("AUTH_RATE_LIMIT_FAIL_CLOSED", true)) for an operator override.

**Reasoning:** Verified all four load-bearing technical claims against HEAD. (1) shared/ratelimit/ratelimit.go:69-75 fails open: on any Redis error checkLimit returns err, the middleware logs "Rate limit check failed" and calls c.Next(); return (comment at line 73 literally says "On error, allow the request (fail open)"). checkLimit (line 130-133) returns an error whenever pipe.Exec fails, i.e. whenever Redis is unreachable. (2) Same RateLimiter type/Middleware() backs both limiters: the general 300/min limiter (main.go:143) and the auth 5/min limiter (main.go:157) both call ratelimit.NewRateLimiter and .Middleware(), so the auth limiter inherits the identical fail-open path. (3) The auth limiter is the sole brute-force defense: it is wired onto POST/GET /auth/login (main.go:494-495), /auth/refresh (497), /auth/exchange (507), and viewer exchanges (533-542); auth-service's own routes for /login, /refresh, /exchange (cmd/main.go:315-357) carry NO rate-limit middleware. The only RateLimiter in auth-service is a per-viewer-session message-send throttle in chat_send.go (checkRateLimit on ViewerSession), unrelated to login. So during a Redis outage these endpoints have zero throttling. (4) The Redis SPOF window is real: gateway uses a single shared Redis client (main.go:116-134) for both pub/sub and rate limiting, and project memory documents a single-replica node-pinned allchat/redis that crash-loops during nightly kured reboots — a recurring, predictable outage window. The auth limiter is the M7 brute-force control introduced by THIS PR (git diff shows authRateLimiter added at main.go:153-165 and wired at 494-542; ratelimit.go itself predates the merge-base but the security-critical reuse of fail-open for the auth path is new here), so reporting it as a PR-478 gap is fair. Severity MEDIUM (not HIGH): exploitation is gated on coinciding with a Redis outage, and even then primary auth still requires valid credentials/OAuth — what is lost is a defense-in-depth throttle during that window, not a standing auth bypass.

--------------------------------------------------------------------------------

## #16 [LOW] Shared prefix-wildcard matcher used by the CSRF OriginCheck has no host-boundary check: a `https://allch.at/*` or `https://allch.at*` allowlist entry matches https://allch.at.evil.com
- dimension: cookie-core-csrf | verdict: partial | kind: incomplete-fix
- location: shared/middleware/origin_check.go:50-58 (OriginAllowed wildcard branches); shared consumers at services/api-gateway/middleware/cors.go:40, shared/middleware/origin_check.go:93

**Scenario:** An operator adds a subdomain by switching CORS_ORIGIN to the wildcard form `https://allch.at/*` (a natural mistake given the M4 doc explicitly shows `moz-extension:///*` as the CORS-wildcard format). An attacker registers allch.at.evil.com, serves a page that does a credentialed fetch/cross-site form to the gateway. CORS reflects the origin with AllowCredentials and OriginCheck passes, defeating the CSRF defense for cookie-auth'd state-changing requests.

**Fix:** In shared/middleware/origin_check.go OriginAllowed (lines 50-58), make the wildcard prefix-match host-boundary-safe: after a HasPrefix match, require the remainder of origin to be empty or to begin with '/' OR require the stripped prefix to end in "://" (true for the only intended use — extension schemes). Concretely replace the two wildcard blocks with a helper:

    prefixMatch := func(prefix string) bool {
        if !strings.HasPrefix(origin, prefix) {
            return false
        }
        rest := origin[len(prefix):]
        // Safe if the prefix is scheme-only (ends in "://", e.g. moz-extension://)
        // or the matched boundary is end-of-string or a path separator.
        return strings.HasSuffix(prefix, "://") || rest == "" || rest[0] == '/'
    }
    if strings.HasSuffix(a, "/*") {
        if prefixMatch(strings.TrimSuffix(a, "/*")) { return true }
    } else if strings.HasSuffix(a, "*") {
        if prefixMatch(strings.TrimSuffix(a, "*")) { return true }
    }

This keeps moz-extension://* / chrome-extension://* / moz-extension:///* working (prefix ends in "://"), keeps "https://allch.at/*" matching "https://allch.at" and "https://allch.at/path" (rest empty or starts with '/'), but makes "https://allch.at.evil.com" fail (rest[0] == '.'). Add a test case in shared/middleware/origin_check_test.go asserting OriginAllowed([]string{"https://allch.at/*"}, "https://allch.at.evil.com") == false. Optionally also add a startup guard in validateCORSConfig (cors.go) that log.Warns if an http(s):// allowlist entry ends in '*'. Given LOW severity this can be deferred, but the matcher fix is cheap and worth doing as defense-in-depth.

**Reasoning:** The code fact is true: OriginAllowed (shared/middleware/origin_check.go:50-58, at HEAD) implements both the "/*" and "*" suffix wildcards as a bare strings.HasPrefix with no host-boundary/delimiter check, and it is genuinely the single matcher wired into the credentialed CORS AllowOriginFunc (cors.go:40), the CSRF OriginCheck (origin_check.go:93, invoked at gateway main.go:497/595/703 via LoadHTTPAllowedOrigins), and the WS origin checkers (websocket.go:417, websocket_viewer.go:101). So if an operator ever set CORS_ORIGIN to a host-wildcard like "https://allch.at*" or "https://allch.at/*", the prefix strips to "https://allch.at" and HasPrefix("https://allch.at.evil.com", "https://allch.at") returns true — defeating both CORS-credentials and the CSRF boundary. That part is real.

BUT the severity and exploitability are overstated. (1) Production uses the exact form CORS_ORIGIN=https://allch.at (deployments/k8s/base/configmap.yaml:29), which hits the exact-match branch (a == origin) — no wildcard, not exploitable today. The claim itself concedes "latent, not currently exploited." (2) The claim's core premise — that the host-wildcard form is "the documented CORS-wildcard format" an operator would naturally write — is false. The "/*" wildcard is documented and tested ONLY for extension schemes (moz-extension:///*, chrome-extension://*) where the stripped prefix ends in "://" and a host-boundary is structurally irrelevant. There is no configmap, .env.example, README, design doc, or test (origin_check_test.go only exercises extension wildcards + exact HTTP hosts) that shows or steers an operator toward "https://allch.at/*". So triggering the bug requires an operator to invent an undocumented, never-exemplified config. That is a real defense-in-depth sharp edge but not a MEDIUM security bug — it is a latent hardening gap (LOW).

Net: real-but-narrower-than-claimed → partial, downgraded to LOW.

--------------------------------------------------------------------------------

## #17 [LOW] AuthCookieForward + proxy copyHeaders let a client smuggle X-Access-Token / X-Refresh-Token straight to auth-service
- dimension: cookie-core-csrf | verdict: confirmed | kind: new-issue
- location: services/api-gateway/middleware/auth_cookie_forward.go:33-38 (conditional Set, no Del) + services/api-gateway/handlers/proxy.go:144-158 (strip-list omits X-Access-Token/X-Refresh-Token); sink at services/auth-service/handlers/auth_handler.go:697,717

**Scenario:** An attacker who has obtained a victim's raw access+refresh token values (but not a live browser cookie) calls POST /api/v1/auth/logout with their own valid `Authorization: Bearer <attacker>` (passes JWTAuth) plus `X-Access-Token: <victim>` and `X-Refresh-Token: <victim>`. With no cookie on the request AuthCookieForward leaves the attacker headers intact; auth-service blacklists the victim's access token and revokes the victim's refresh-token reuse key — a forced logout/session-kill of the victim performed by an unrelated authenticated session. Same smuggling applies to /auth/refresh (public).

**Fix:** In services/api-gateway/middleware/auth_cookie_forward.go, make the middleware unconditionally authoritative — clear any client-supplied values first, then set only from cookies. Replace lines 32-39 with:

    return func(c *gin.Context) {
        // Always clear client-supplied values: these headers must originate ONLY
        // from the gateway cookie, never from the inbound request (audit H3).
        c.Request.Header.Del("X-Access-Token")
        c.Request.Header.Del("X-Refresh-Token")
        if tok, err := c.Cookie(auth.CookieAccessToken); err == nil && tok != "" {
            c.Request.Header.Set("X-Access-Token", tok)
        }
        if tok, err := c.Cookie(auth.CookieRefreshToken); err == nil && tok != "" {
            c.Request.Header.Set("X-Refresh-Token", tok)
        }
        c.Next()
    }

Do NOT add X-Access-Token/X-Refresh-Token to proxy.go copyHeaders strip-list — copyHeaders runs in the proxy after the middleware, so stripping there would also drop the gateway-set values and break the forward path. The Del-before-Set is the correct and complete fix. Optionally, extend services/api-gateway/middleware/auth_cookie_forward_test.go with a TestAuthCookieForward_StripsClientSuppliedHeaders case that sets X-Access-Token/X-Refresh-Token on the inbound request with no cookie and asserts they are empty downstream.

**Reasoning:** Verified against HEAD. (1) services/api-gateway/middleware/auth_cookie_forward.go:33-38 does a conditional Set with no Del: `if tok, err := c.Cookie(...); err == nil && tok != "" { c.Request.Header.Set("X-Access-Token", tok) }` (same for X-Refresh-Token). A client-supplied value is never cleared on the no-cookie path. (2) services/api-gateway/handlers/proxy.go:144-158 copyHeaders strip-list contains Cookie/Referer/Origin + hop-by-hop only — X-Access-Token / X-Refresh-Token are NOT stripped, so they reach the backend. (3) auth-service trusts them: HandleLogout (auth_handler.go:697 blacklist:<X-Access-Token>, :717-721 Del refresh_token:<hash(X-Refresh-Token)>), HandleDeleteAccount (:761, :780), and admin_impersonation.go:38 all read these headers. I traced the /auth/logout route end-to-end (cmd/main.go:601, protectedAPI group): CookieToBearer (cookie_to_bearer.go:31 leaves a present Authorization untouched), JWTAuthWithRevocation (passes on attacker's own valid bearer), OriginCheck (origin_check.go:87-91 — absent Origin/Referer is ALLOWED for non-browser clients), then AuthCookieForward (no cookie → attacker's X-* headers survive), then proxy (forwards them). So an attacker with their own valid streamer/admin JWT plus the victim's raw access+refresh token values can force-logout / revoke the victim's session via X-Access-Token:<victim> + X-Refresh-Token:<victim>. The scenario genuinely holds. Severity is correctly LOW: exploitation requires already possessing the victim's RAW tokens (near-total compromise already), so it is an invariant/defense-in-depth breach, not privilege escalation — the reviewer's own framing is accurate. Scenario B (/auth/refresh, public group) is real but adds NO capability beyond possessing the raw refresh token, since the documented JSON-body fallback (auth_handler.go:547-552) already lets a raw-token holder consume the reuse key directly. The existing test TestAuthCookieForward_NoCookiesNoHeaders (auth_cookie_forward_test.go:53) only covers no-cookie+no-client-header; it does NOT cover the smuggling case, giving a false sense of safety. Note on the proposed fix: adding the X-* headers to proxy copyHeaders strip-list ALONE would be WRONG — copyHeaders runs in the proxy AFTER all middleware, so it would also strip the gateway-set values and break the whole H3 forward path. The authoritative fix is the Del-before-Set in the middleware.

--------------------------------------------------------------------------------

## #18 [LOW] Viewer logout deletes the DB session but never blacklists the viewer JWT — revocation is asymmetric vs. streamer/admin logout
- dimension: jwt-revocation | verdict: partial | kind: new-issue
- location: services/auth-service/handlers/viewer_auth.go:441-466 (HandleLogout, no blacklist write) and services/api-gateway/cmd/main.go:617 (viewer logout route lacks AuthCookieForward); middleware blacklist check at shared/middleware/auth.go:99 does apply to viewer tokens

**Scenario:** A viewer logs out (or an admin force-deletes a viewer session via DB), believing the session is terminated. A copy of the still-unexpired viewer JWT (e.g. captured by a shared-device attacker or a logged token) continues to authenticate against viewer-protected endpoints that trust JWT claims rather than re-fetching the session — e.g. `/auth/viewer/cosmetics` (keyed on viewer_id) and the viewer payment surfaces — for up to 24h. Endpoints that re-fetch by session_id (HandleMe viewer_auth.go:423, chat_send.go:303) do break after logout, which limits but does not eliminate the exposure.

**Fix:** Two-part fix to make viewer logout symmetric with streamer logout. (1) Gateway: add AuthCookieForward to the viewer logout route so the handler can read the access token — services/api-gateway/cmd/main.go:617 change to: protectedAPI.POST("/auth/viewer/logout", localmiddleware.AuthCookieForward(), proxyHandler.ForwardRequest). Note viewer tokens are bearer-in-localStorage, so AuthCookieForward only matters if cookies are ever used; the handler should also fall back to the Authorization Bearer header. (2) Handler: in services/auth-service/handlers/viewer_auth.go HandleLogout (after the sessionID parse, before/around the Delete at L458), read the token (token := c.GetHeader("X-Access-Token"); if empty, strip "Bearer " from Authorization) and, when non-empty and h.redis != nil, write h.redis.Set(c.Request.Context(), "blacklist:"+token, "1", h.jwtExpiry) (best-effort: log a Warn on error, do not fail the logout). This requires ViewerAuthHandler to carry jwtExpiry (NewViewerAuthHandler already receives jwtExpiryHours at cmd/main.go:249 — store it as a time.Duration field). Also have the frontend logout actually call viewerApi.logout() (AppNav.tsx:52 currently calls only the client-only viewerLogout()) so the blacklist is populated on real logouts. The middleware at shared/middleware/auth.go:99 already enforces the blacklist for viewer tokens, so no middleware change is needed.

**Reasoning:** Structural facts in the claim are accurate at HEAD: ViewerAuthHandler.HandleLogout (services/auth-service/handlers/viewer_auth.go:441-466) only calls viewerRepo.Delete(sessionID) and never writes blacklist:<token>; the gateway /auth/viewer/logout route (services/api-gateway/cmd/main.go:617) lacks AuthCookieForward so no X-Access-Token reaches the handler; the middleware viewer branch (shared/middleware/auth.go:116-132) validates viewer tokens on signature/issuer/expiry only, with no DB-session liveness check; and the blacklist check at auth.go:99 IS reached for viewer tokens (it would honor a viewer blacklist entry if one existed). Some viewer endpoints key on viewer_id from claims without re-fetching the session (viewer_cosmetics.go:61 and :202), so they keep working after the DB session is deleted, confirming asymmetry with the streamer/admin logout that DOES blacklist (auth_handler.go:710).

BUT the claim is materially overstated as a new issue at LOW: (1) It is NOT introduced by this PR. git diff c1cfd7..HEAD shows ViewerAuthHandler.HandleLogout was not functionally touched, and the streamer-side blacklist-on-logout already existed before the merge-base (the PR only changed the token source to X-Access-Token and added ClearAuthCookies). The asymmetry is pre-existing. (2) Viewer tokens were deliberately excluded from the H3 cookie migration — they live in localStorage (frontend/src/lib/stores/viewer-auth-store.ts:38-40,64), so the token is JS-readable irrespective of logout; the "stolen token valid until expiry" exposure is the inherent property of any localStorage bearer token, not a hole H3 opened. (3) The real UI logout does not even hit HandleLogout: AppNav.tsx:52 calls the client-only viewerLogout() which clears localStorage/in-memory and never calls viewerApi.logout(), so the backend DB-delete path the claim centers on is largely vestigial. Net: a genuine but pre-existing defense-in-depth gap (no server-side viewer-token revocation; a leaked copy survives up to JWT_EXPIRY_HOURS=24h on claims-keyed endpoints like /auth/viewer/cosmetics), with no attack beyond generic stolen-bearer risk that already exists due to localStorage storage.

--------------------------------------------------------------------------------

## #19 [LOW] WebSocket overlay/viewer auth accepts the access cookie (new in H3) but never consults the revocation blacklist
- dimension: jwt-revocation | verdict: confirmed | kind: new-issue
- location: services/api-gateway/handlers/websocket.go:182 (validation with no blacklist check); cookie source at websocket.go:101; asymmetric HTTP guard at services/api-gateway/cmd/main.go:594 + shared/middleware/auth.go:99; missing redis wiring at services/api-gateway/cmd/main.go:381

**Scenario:** A streamer logs out on a shared/compromised machine; their access token is blacklisted (HTTP routes correctly 401). The same token, replayed over the WS subprotocol/cookie/query param, still passes the WS ownership check and streams the owner's live overlay chat feed until the 24h JWT expiry. Impact is bounded because the WS only exposes the owner's own overlay data and requires VerifyOverlayOwnership, but it defeats the logout-revocation guarantee for the live chat stream.

**Fix:** Wire the gateway's redisClient into both WS handlers and run the same blacklist check used by JWTAuthWithRevocation before ValidateJWTWithKeyChain. Concretely: (1) Add a `rdb redis.UniversalClient` field to WebSocketHandler (services/api-gateway/handlers/websocket.go:108-119) and ViewerWebSocketHandler (websocket_viewer.go:37-47), add it as a constructor param to NewWebSocketHandler and NewViewerWebSocketHandler, and pass redisClient at the two call sites in services/api-gateway/cmd/main.go:381 and :384. (2) In HandleOverlayConnection, immediately after `token != ""` and before `claims, err := auth.ValidateJWTWithKeyChain(token, h.userKeyChain)` at websocket.go:182, add: `if h.rdb != nil { if n, err := h.rdb.Exists(ctx, "blacklist:"+token).Result(); err != nil { h.logger.Warn("Blacklist check failed (fail-open)", zap.Error(err)) } else if n > 0 { c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"}); return } }` — fail-open + log on Redis error to match the HTTP middleware behavior (shared/middleware/auth.go:98-114). (3) Apply the equivalent check in HandleViewerChatConnection at websocket_viewer.go:130-144: on a blacklist hit, fall through to anonymous (do not authenticate the viewer) rather than 401, since viewer auth there is optional — though viewer tokens are not currently in the blacklist write-path, so the owner-WS fix is the load-bearing one.

**Reasoning:** Verified all load-bearing points against HEAD. (1) The H3 change genuinely added the httpOnly access_token cookie as a third WS auth source: extractWSAuthToken at services/api-gateway/handlers/websocket.go:101 returns `r.Cookie(auth.CookieAccessToken)` (CookieAccessToken == "access_token", shared/auth/cookie.go:28); confirmed in the merge-base diff. (2) The owner WS handler validates the token at websocket.go:182 with `auth.ValidateJWTWithKeyChain(token, h.userKeyChain)` and there is NO preceding blacklist:<token> Exists check anywhere in the handler; WebSocketHandler/ViewerWebSocketHandler hold no redis client (struct fields at websocket.go:108-119 and websocket_viewer.go:37-47). (3) The asymmetry is real: HTTP routes are mounted under sharedmiddleware.JWTAuthWithRevocation(userKeyChain, redisClient) (cmd/main.go:594 protected, :702 admin) which DOES run rdb.Exists("blacklist:"+tokenString) (shared/middleware/auth.go:99), but the WS routes /ws/overlay/:overlay_id (main.go:480) and /ws/chat/:streamer_username (main.go:483) are registered directly on router with no middleware, and NewWebSocketHandler/NewViewerWebSocketHandler (main.go:381, 384) are not passed redisClient even though it exists at main.go:121. (4) Token values match: HandleLogout/HandleDeleteAccount write `blacklist:`+token using the exact access-token value (auth_handler.go:710, :769), the same value the WS reads from the cookie — so a blacklisted token is still accepted by the WS path. JWT_EXPIRY_HOURS defaults to 24 (auth-service/cmd/main.go:105), so the gap persists up to 24h after logout. Severity stays LOW (as claimed): exploitation requires the attacker to already hold the raw token (httpOnly cookie defeats XSS theft — needs a compromised/shared machine or MITM), and the only data exposed is the owner's own overlay chat feed, gated by VerifyOverlayOwnership (websocket.go:192), which is essentially the chat the streamer already broadcasts. It is a genuine-but-narrow defeat of the logout-revocation guarantee for the live chat stream, not a broad auth bypass. Viewer-token logout is not in the blacklist write-path, so the owner overlay WS is the real surface, exactly as the claim narrows it.

--------------------------------------------------------------------------------

## #20 [LOW] Impersonation token expiry regressed from 2h to 24h (jwtExpiry default)
- dimension: impersonation | verdict: confirmed | kind: regression-from-fix
- location: services/auth-service/handlers/admin.go:208 (expiry passed to GenerateImpersonationJWTWithKidExpiry is h.jwtExpiry); reinforced by admin.go:254 (stash TTL) and admin.go:268 (cookie MaxAge); h.jwtExpiry sourced from cmd/main.go:105 + cmd/main.go:260 (JWT_EXPIRY_HOURS default 24)

**Scenario:** Admin starts impersonating a user. The impersonation access token is captured (e.g. via a logging mishap, intermediary proxy, or transient exposure during the flow). Pre-H3 the stolen token expired in 2h; now it remains valid for the full 24h JWT_EXPIRY_HOURS unless the admin explicitly clicks Stop (which blacklists it). The longer window meaningfully increases the value of a stolen impersonation token.

**Fix:** Introduce a dedicated short impersonation TTL instead of reusing the 24h session jwtExpiry. In services/auth-service/cmd/main.go after line 105 add: `impersonationExpiryHours := getEnvAsIntOrDefault("IMPERSONATION_EXPIRY_HOURS", 2)` and pass `time.Duration(impersonationExpiryHours)*time.Hour` to NewAdminHandler at line 260 as a new parameter (alongside or replacing the jwtExpiry arg used for impersonation). In services/auth-service/handlers/admin.go, add a field `impersonationExpiry time.Duration` to AdminHandler (line ~54), set it in NewAdminHandler (line 58/65), and use it at admin.go:208 (GenerateImpersonationJWTWithKidExpiry expiry arg), admin.go:254 (Redis stash TTL), and admin.go:268 (cookie MaxAge) instead of h.jwtExpiry — keeping all three aligned to the shorter value. The /stop-impersonation handler at admin_impersonation.go:84/97/106 should keep using h.jwtExpiry for the restored ADMIN cookie and the blacklist TTL (the blacklist only needs to outlive the impersonation token, so 2h there would also suffice but jwtExpiry is harmless). Net: impersonation token, its stash, and its cookie expire in 2h by default (configurable), restoring the pre-PR security posture.

**Reasoning:** The regression is real in the CURRENT code. Pre-PR (c1cfd7f8), services/auth-service/handlers/admin.go called auth.GenerateImpersonationJWTWithKid(...), and that function hardcoded ExpiresAt = time.Now().Add(2 * time.Hour) (verified via `git show c1cfd7f8:shared/auth/jwt.go` line 399 in the WithKid variant, plus line 149 "Shorter expiry for security" in the base variant). So impersonation tokens lived 2h. At HEAD, admin.go:200-209 now calls GenerateImpersonationJWTWithKidExpiry(..., h.jwtExpiry), and jwt.go:412-434 sets ExpiresAt = time.Now().Add(expiry) with no clamp. h.jwtExpiry is wired from JWT_EXPIRY_HOURS (cmd/main.go:105 getEnvAsIntOrDefault("JWT_EXPIRY_HOURS", 24)) and passed as time.Duration(jwtExpiryHours)*time.Hour into NewAdminHandler (cmd/main.go:260), stored at admin.go:65. The same 24h value is reused for the Redis stash TTL (admin.go:254) and the access-cookie MaxAge (admin.go:268). So the default impersonation token lifetime went from 2h to 24h. The I3 blacklist (admin_impersonation.go:100-111) only revokes the token on an explicit POST /stop-impersonation; if the admin never clicks Stop, a leaked impersonation token stays valid for the full 24h. The PR's own comment at jwt.go:408-411 ("replaces the legacy hardcoded 2h") confirms the change was incidental, not a deliberate decision to lengthen the security-sensitive impersonation window — and the backward-compat wrapper GenerateImpersonationJWTWithKid (jwt.go:439-441) still defaults to 2h, showing 2h was the intended value. No IMPERSONATION_EXPIRY config exists (grep returned nothing). Severity LOW is correct: exploitation requires the impersonation token to leak (the cookie is httpOnly+Secure+SameSite=Lax, so not JS-readable and not sent cross-site) AND the admin to not stop; it widens an already-narrow window rather than opening a new bypass.

--------------------------------------------------------------------------------

## #21 [LOW] 401/refresh during impersonation silently reverts to the admin's own session while UI still shows impersonating
- dimension: impersonation | verdict: confirmed | kind: new-issue
- location: services/auth-service/handlers/auth_handler.go:545 (reads only X-Refresh-Token) and :639 (issues plain admin JWT, no impersonated_by); services/auth-service/handlers/admin.go:261-269 (only access cookie swapped on impersonation start); services/api-gateway/middleware/auth_cookie_forward.go:33-38 (forwards both tokens); frontend/src/lib/api/client.ts:99-125 (silent refresh→retry); frontend/src/lib/stores/auth-store.ts:86-94 (isImpersonating only re-derived on init())

**Scenario:** Admin impersonates user A and is working in the impersonated session. The impersonation access token expires or a transient 401 occurs. The client calls /auth/refresh, which uses the admin's still-present refresh cookie and issues a fresh ADMIN access token (sub=admin, no impersonated_by). The retried request now runs as the admin, but the ImpersonationBanner / isImpersonating state still claims the admin is impersonating A. The admin keeps acting believing they are 'as A' while actually acting as themselves, leading to actions taken under the wrong identity; the orphaned impersonation: stash is also never consumed and just TTLs out.

**Fix:** Make /refresh impersonation-aware (the data is already forwarded — X-Access-Token reaches the route via AuthCookieForward). In services/auth-service/handlers/auth_handler.go HandleRefresh (around line 545, before/after reading the refresh token), inspect the access token: `if at := c.GetHeader("X-Access-Token"); at != "" { if claims, vErr := auth.ValidateJWTWithKeyChain(at, h.userKeyChain); vErr == nil && claims.ImpersonatedBy != "" { c.JSON(http.StatusConflict, gin.H{"error":"impersonation_session_ended","impersonation_ended":true}); return } }`. This refuses to silently rotate an impersonation session into an admin session and returns a distinct status. Then in frontend/src/lib/api/client.ts tryRefresh()/the 401 handler (lines 116-135): treat a non-ok refresh that carries impersonation_ended (or simply any refresh failure during impersonation) by calling useAuthStore.getState().init() (or clearing isImpersonating) so the banner state can no longer go stale. Minimal alternative if backend changes are undesirable: in client.ts after a successful tryRefresh() (line 119), re-derive impersonation state by invoking the auth-store init()/getMe() so isImpersonating/impersonatedUsername reflect the post-refresh identity instead of remaining stale until page reload. Either path closes the identity-confusion window; the backend ValidateJWTWithKeyChain + Claims.ImpersonatedBy already exist (used in admin_impersonation.go:50-55, shared/auth/jwt.go:56).

**Reasoning:** Verified the full flow against current HEAD (post-d577723f). (1) StartImpersonation at services/auth-service/handlers/admin.go:261-269 sets ONLY auth.CookieAccessToken to the impersonation JWT; the refresh cookie is never touched, so it stays the admin's own Twitch refresh token. (2) The gateway route POST /api/v1/auth/refresh (services/api-gateway/cmd/main.go:497) runs AuthCookieForward(), which forwards BOTH X-Access-Token and X-Refresh-Token (auth_cookie_forward.go:33-38) — but HandleRefresh (auth_handler.go:540-656) reads only X-Refresh-Token (line 545) and never inspects X-Access-Token. (3) It refreshes the admin's Twitch token (line 576), looks up the user by the admin's Twitch ID (line 615), and issues a PLAIN admin JWT via GenerateTokenWithKid(..., user.IsAdmin) at line 639 with no impersonated_by claim, then writes it to the access cookie via SetAuthCookies (line 648). So a /refresh during impersonation silently rotates the session back to the admin's own non-impersonated identity. (4) Frontend client.ts:99-135 auto-refreshes on any non-reauth_required 401 for non-/api/v1/auth endpoints and retries via this.fetch — the retried request now runs as the admin. (5) The Zustand store only derives isImpersonating from /auth/me on init() (auth-store.ts:86-94, set at 92-93); the refresh→retry path in client.ts never updates the store, so ImpersonationBanner keeps showing isImpersonating=true / impersonatedUsername until a full reload, and the impersonation:<jti> stash is orphaned (only consumed by HandleStopImpersonation GetDel at admin_impersonation.go:62) and just TTLs out. The mechanism is real and present in current code. Severity LOW is correct: this is identity/accountability confusion, not privilege escalation (the admin already holds admin rights and gains nothing they don't have; they actually drop the target identity). Real harm: post-revert actions are taken as the admin while the UI claims impersonation, and the DSGVO impersonation audit log (admin.go:228-238) won't reflect the silent revert. Practical trigger is narrow: the impersonation access JWT lives JWT_EXPIRY_HOURS (default 24h, main.go:105), so expiry-driven reverts are rare; the realistic trigger is a keychain rotation or JWT blacklist invalidating the impersonation token mid-session, producing a 401 that the client silently refreshes.

--------------------------------------------------------------------------------

## #22 [LOW] Twitch/YouTube PKCE exchange silently falls back to non-PKCE when verifier is missing, converting a PKCE downgrade into an opaque 500
- dimension: oauth-pkce-state | verdict: partial | kind: incomplete-fix
- location: services/auth-service/handlers/platform_auth_v2.go:981-986 (Twitch fallback) and :997-1002 (YouTube fallback); TTL mismatch root cause: verifier Set 10m at :448/:527/:544/:558 vs state Set 30m at :137/:322/:456

**Scenario:** Tied to the TTL-mismatch finding: when the verifier has expired, the Twitch-login path silently drops to non-PKCE ExchangeCode and the provider rejects it, producing an opaque 'Failed to exchange code' 500 with no log line indicating the verifier was missing — making the TTL bug hard to diagnose on-call. There is no auth-bypass because the provider enforces PKCE, but the silent fallback removes the signal that PKCE was expected.

**Fix:** Two-part minimal fix, no expectPKCE plumbing (which would break YouTube moderation re-consent):

1. Close the TTL window — align the verifier TTL to the state TTL so the verifier never expires before the state. Change the four verifier Set calls from 10*time.Minute to 30*time.Minute at services/auth-service/handlers/platform_auth_v2.go:448, :527, :544, :558 (and the matching Set in auth_handler.go:103 and :133). This eliminates the only scenario in which a PKCE flow reaches the fallback with an expired verifier.

2. Add a diagnostic log on the silent fallback so a future verifier-miss is visible. In the Twitch branch at platform_auth_v2.go:985 and the YouTube branch at :1001, before falling through to ExchangeCode, emit:
  h.logger.Debug("No PKCE verifier found; using non-PKCE exchange (expected for chat-scopes/moderation re-consent flows)", zap.String("platform", string(platform)))
Keep it Debug (not Error) because the verifier-less path is legitimate for the Twitch chat-scopes add-source flow and the YouTube moderation re-consent flow.

Do NOT introduce expectPKCE=true for all YouTube flows — it would break HandleEnableModeration's YouTube re-consent path which legitimately stores no verifier.

**Reasoning:** CODE BEHAVIOR confirmed: at HEAD, exchangeCodeForToken (services/auth-service/handlers/platform_auth_v2.go) treats any GetDel miss on the verifier as "no PKCE" and silently falls through to the non-PKCE ExchangeCode — Twitch lines 981-986, YouTube lines 997-1002. Neither fallback logs anything; the only surfaced error is the generic "Failed to exchange code" 500 at line 654.

THE TTL MISMATCH IS REAL: verifier keys are stored with a 10-minute TTL (lines 448, 527, 544, 558) while the corresponding oauth_state keys get a 30-minute TTL (lines 137, 322, 456). State validation (GetDel at line 618) is independent of the verifier, so for a login/PKCE flow where the user sits on the consent screen 10-30 minutes, the state still validates, the verifier GetDel returns redis.Nil, the code drops to verifier-less ExchangeCode, the provider rejects it (PKCE was sent at authorize), and the operator sees an opaque 500 with no log naming the missing verifier. That is a genuine (LOW) diagnosability defect.

WHY PARTIAL / NOT CONFIRMED-AS-WRITTEN:
1. No security impact — the claim itself concedes the provider enforces PKCE, so there is no downgrade/bypass. It is purely an observability issue. Severity LOW is appropriate, not higher.
2. The claim's evidence note "YouTube has no non-PKCE login path so the fallback is never legitimate" is FACTUALLY WRONG. HandleEnableModeration (line 346) for YouTube calls GetAuthURLWithScopes (non-PKCE, line 443) and stores NO verifier, yet routes through the same HandleCallback -> exchangeCodeForToken (line 652). So YouTube has a legitimate verifier-less re-consent flow that relies on exactly this fallback.
3. Consequently the proposed fix (pass expectPKCE=true for "all youtube flows") would BREAK the YouTube moderation re-consent flow, which legitimately has no verifier. The fix as proposed is incorrect.

--------------------------------------------------------------------------------

## #23 [LOW] Editor EMBED_READY postMessage receiver does not validate event.origin (M11 partial)
- dimension: frontend-xss-redirect-csp | verdict: partial | kind: incomplete-fix
- location: frontend/src/app/overlays/[id]/page.tsx:1442-1451 (handleEmbedReady, missing event.origin check); outbound '*' at :1229, :1237, :1273, :1282, :1295. Contrast: embed receiver origin check at frontend/src/app/overlays/[id]/preview/embed/page.tsx:294.

**Scenario:** While a streamer has the overlay editor open, any page able to obtain a handle to the editor window (e.g. a window the editor opened, or an attacker page the streamer also has open that window.open()ed the editor) can postMessage({type:'EMBED_READY'}) to it. The editor re-runs sendCssToIframe/sendTtsSettingsToIframe and posts the overlay's CSS/filter/TTS settings (including any TTS display config) to its iframe with targetOrigin '*'. Impact is low: the data only reaches the same-origin iframe document and contains display settings (the ElevenLabs token/endpoint is fetched by the iframe itself, not sent here), and EMBED_READY carries no attacker-controlled fields the handler acts on. Still, it is an unvalidated cross-origin receiver and the '*' targetOrigin would leak settings if the iframe were ever navigated cross-origin.

**Fix:** In frontend/src/app/overlays/[id]/page.tsx, mirror the embed-side M11 hardening on the editor receiver. At line 1443, inside handleEmbedReady, add as the first statement:

  if (event.origin !== window.location.origin) return

(so it reads: `const handleEmbedReady = (event: MessageEvent) => { if (event.origin !== window.location.origin) return; if (event.data?.type !== 'EMBED_READY') return; ... }`)

Optionally (defense-in-depth, not required for correctness since the iframe is always same-origin per components/SplitView.tsx:90), change the outbound targetOrigin from '*' to window.location.origin in the four senders: sendFilterSettingsToIframe (page.tsx:1229), sendSoundSettingsToIframe (:1237), sendTtsSettingsToIframe (:1273), and sendCssToIframe (:1282 and :1295). This keeps the editor symmetric with the embed receiver and removes the only theoretical leak vector if the iframe were ever navigated cross-origin.

**Reasoning:** The factual claims are accurate at HEAD. The M11 fix (commit on this branch) added `if (event.origin !== window.location.origin) return` to the embed-SIDE receiver (frontend/src/app/overlays/[id]/preview/embed/page.tsx:294, confirmed added by the PR diff) and changed its outbound EMBED_READY post to window.location.origin. But the symmetric editor-SIDE receiver `handleEmbedReady` (page.tsx:1442-1451) has NO event.origin check, and all four senders post with targetOrigin '*' (sendFilterSettingsToIframe:1229, sendSoundSettingsToIframe:1237, sendTtsSettingsToIframe:1273, sendCssToIframe:1282 & :1295). So the inconsistency is real and the fix is incomplete.

However, the exploitability is effectively nil, narrower than even the reviewer's already-LOW framing:
1) The handler reads NO attacker-controlled field from the event — it only checks data.type==='EMBED_READY' and then re-sends the streamer's OWN current settings.
2) Its only side effect is posting those settings to iframeRef.current.contentWindow. I verified the iframe src in components/SplitView.tsx:90 is the relative path `/overlays/${overlayId}/preview/embed` — always same-origin. So the '*' targetOrigin reaches only the same-origin preview iframe; there is no cross-origin leak path in current code (the '*'-leak scenario requires the iframe to be navigated cross-origin, which never happens).
3) The TTS payload (1250-1271) deliberately excludes the ElevenLabs token/endpoint/voiceId (comment at 1244 + GET /tts-config), so no secret travels even within same-origin.
4) The trigger requires the attacker to hold a window handle to the editor window (opener/popup relationship), a narrow precondition.

Net: a triggering attacker can at most cause the editor to redundantly re-broadcast the streamer's own already-rendered display settings into the streamer's own same-origin iframe — no data reaches the attacker, no state changes, no cross-origin leak. This is a genuine defense-in-depth gap (asymmetric M11 hardening) but not an exploitable security bug. Confirmed as a real incomplete-fix inconsistency; refuted as having any current security impact. Hence partial, LOW.

--------------------------------------------------------------------------------

## #24 [LOW] H5 log-redaction is /ws/-only: tts_token JWT (no expiry) is logged in plaintext on every TTS request
- dimension: cors-ratelimit-proxy-config | verdict: partial | kind: incomplete-fix
- location: services/api-gateway/middleware/logging.go:40 (redaction gate) and logging.go:62 (query logged); fallback consumer at services/overlay-manager/handlers/tts.go:749

**Scenario:** A streamer adds the TTS browser source to OBS using the deep link ...allch.at/overlay/<id>?tts_token=<JWT>. OBS hits POST /api/v1/overlays/<id>/tts?tts_token=<JWT> for every TTS playback. The gateway access log (shipped to Loki) records the full query string including the JWT. Anyone with log read access (or a log-exfil incident) extracts the never-expiring tts_token and can call the public, no-user-JWT POST /overlays/:id/tts endpoint to inject arbitrary TTS audio into the victim's live overlay and burn their ElevenLabs spend indefinitely. The streamer cannot revoke it without an explicit secret rotation they have no reason to perform.

**Fix:** In services/api-gateway/middleware/logging.go, replace the /ws/-and-"token"-only block (lines 40-47) with a path-agnostic, key-denylist redaction so any known-sensitive query param is scrubbed on every route. Concretely:

  if query != "" {
      if values, err := url.ParseQuery(query); err == nil {
          changed := false
          for _, k := range []string{"token", "tts_token", "access_token", "refresh_token", "code", "state"} {
              if values.Has(k) {
                  values.Set(k, "[REDACTED]")
                  changed = true
              }
          }
          if changed {
              query = values.Encode()
          }
      }
  }

This closes the tts_token fallback leak (tts.go:749) AND the more material /auth/callback ?code=/?state= leak (main.go:496) via the same change. Keep the M17 header-based token transport as the primary control.

**Reasoning:** I verified the full request flow at HEAD. The redaction IS /ws/-only and key-"token"-only (services/api-gateway/middleware/logging.go:40-47), and the full query string IS logged unconditionally (logging.go:62). The TTS proxy route POST /api/v1/overlays/:id/tts is on publicAPI ("/api/v1" group, main.go:488/562) and the global Logging middleware (main.go:428) applies to it, so its query is NOT under /ws/ and would not be redacted. The handler still accepts ?tts_token= as a fallback (tts.go:744-750), and the token has no ExpiresAt (tts/jwt.go:20-22,44-49). So the redaction gap is real.

BUT the claim's central scenario — "tts_token is logged on EVERY TTS request / permanent leak on every playback" — is REFUTED by the M17 fix that is present in current code. frontend/src/lib/utils/ttsPlayer.ts:400-409 sends the token via `Authorization: Bearer` header and voice via JSON body, with ttsEndpoint built as `/api/v1/overlays/${id}/tts` with NO query string (overlay/[id]/page.tsx:468 and preview/embed/page.tsx:549). The test ttsPlayer.test.ts:708 explicitly asserts the URL does not contain `tts_token=`. Additionally, the OBS deep link `/overlay/<id>?tts_token=` (tts.go:216) is served by Next.js, not the Go gateway — there is no /overlay/:id route on the gateway (only /ws/* and /api/* and a few /api/twitch, /api/avatars). So the page-load query never reaches the gateway access log either.

Net: in the normal production path NO tts_token ever appears in the gateway query log. The leak only fires if a request arrives with the legacy `?tts_token=` fallback shape (an old client during rollout, a third-party tool, or a manually constructed URL). That is a genuine residual log-leak hole but is NOT triggered by the default flow, so it is narrower and lower severity than claimed. Note also that the SAME root cause does cause a more material live leak on the proxied /auth/callback route (main.go:496) where OAuth ?code=&state= are logged in full — but that is outside this tts_token claim.

--------------------------------------------------------------------------------

## #25 [LOW] OAuth code/state leaked into gateway access logs on all *_callback GET routes (H5 redaction does not cover them)
- dimension: cors-ratelimit-proxy-config | verdict: partial | kind: incomplete-fix
- location: services/api-gateway/middleware/logging.go:40-47,62 (token-only, /ws/-scoped redaction; full query logged) AND shared/middleware/logging.go:43 (no redaction; used by auth-service). Callbacks: services/api-gateway/cmd/main.go:511,516,521,526,528,532,535,538; query read at services/auth-service/handlers/platform_auth.go:132-133

**Scenario:** On every streamer/viewer OAuth login the gateway logs ...callback?code=<authcode>&state=<csrf>. An attacker (or insider) with read access to gateway logs who can observe and replay within the code's short validity window could exchange the leaked authorization code at the provider before the legitimate exchange completes (or correlate state to defeat the CSRF check). Lower severity than the tts_token leak because OAuth codes are single-use and short-lived, but it is still a credential in plaintext logs that the claimed H5 fix was supposed to prevent.

**Fix:** Redact a sensitive-key denylist on ALL paths in BOTH logging middlewares (the claim missed the auth-service one). In services/api-gateway/middleware/logging.go, replace the `/ws/`-scoped block (lines 40-47) with an unconditional redaction over the parsed query: parse `query` with url.ParseQuery and, for each key in {"token","code","state","access_token","refresh_token","tts_token"} that is present, Set it to "[REDACTED]", then `query = values.Encode()`. Apply the identical change in shared/middleware/logging.go around line 31-43 (it currently does no redaction at all and is used by auth-service/cmd/main.go:290, where the same callback query is logged). Suggested shared helper: a `redactQuery(raw string) string` in shared/middleware so both call sites stay in sync. Note: because the code is already consumed by the time it is logged, this is hygiene/defense-in-depth, not a fix for an active exploit.

**Reasoning:** The mechanical claim is TRUE at HEAD. (1) The gateway's Logging middleware redaction is scoped to `/ws/` paths and the literal key `token` only — services/api-gateway/middleware/logging.go:40-47; the full RawQuery is logged at line 62. (2) All provider OAuth callbacks are GET routes proxied through the gateway (main.go:511/516/521/526/528 and viewer variants 532/535/538), and the provider's RedirectURL points at these gateway paths (auth-service/cmd/main.go:89-103,226-232 build `<base>/api/v1/auth/<platform>/callback`). (3) The handler reads the provider authorization `code` and CSRF `state` from the query string (platform_auth.go:132-133). So on every streamer/viewer login the gateway logs `...callback?code=<authcode>&state=<csrf>` verbatim. The leak is in fact WIDER than the claim states: the auth-service (the real handler behind the proxy) logs the same query string via shared/middleware/logging.go:43, which has NO redaction at all — the claim only cites the gateway.

HOWEVER, the claimed attack scenario (replay the leaked code at the provider) does NOT hold in the normal flow, which drops the severity below MEDIUM. The log entry is written AFTER c.Next() in both middlewares (gateway logging.go:50 c.Next() → line 59 log; shared logging.go:34 c.Next() → 56-61 log). The handler synchronously exchanges/burns the code at the provider (platform_auth.go:181/192) and deletes the CSRF `state` from Redis (line 153) BEFORE control returns to the logging middleware. So by the time `code`/`state` hit the log they are already single-use-consumed and the state is gone — no replay window, no CSRF-defeat window in the success path. The only residue is a 5xx/aborted exchange leaving an unredeemed code, but a failed exchange typically means the code is already invalid. This is a genuine logging-hygiene / defense-in-depth gap that H5 was meant to close (credentials should never reach logs), but it is not practically exploitable — hence LOW, not MEDIUM, and "partial" rather than "confirmed".

--------------------------------------------------------------------------------

## #26 [LOW] M5 fix is incomplete: ambiguous '|' canonicalization allows cross-field collisions (query-param tampering NOT actually prevented)
- dimension: signing | verdict: partial | kind: incomplete-fix
- location: shared/signing/signing.go:221-232 (computeSignature; the Sprintf is line 226). Comment at signing.go:217-220; README.md:124,137,140-141.

**Scenario:** All services share one signing secret (README: 'All services must use the same secret'). computeSignature builds message = fmt.Sprintf("%s|%s|%s|%s|%d|%s", method, path, rawQuery, service, ts, bodyHash). The two requests (method=POST, path="/a", rawQuery="b|x", service="y") and (method=POST, path="/a|b", rawQuery="x", service="y") both serialize to "POST|/a|b|x|y|<ts>|<hash>" and therefore HMAC to the IDENTICAL signature (verified empirically: both = fc68ab20152bdd279861594b94110c1bd05ce5b908ce6c65069990fd348d0ba5). Concretely: a confused-deputy/signing-oracle service that is willing to sign a request whose path/query is partly attacker-controlled (e.g. it signs path="/a" with an attacker-supplied query "b|x") yields a signature that an attacker can replay against the verifier as path="/a|b" with query="x" — a different route/authorization than what was signed. The same boundary-shift works between rawQuery and serviceName, defeating the M5 service-identity binding. When this package is eventually wired into prod (the stated intent in RESIDUALS.md L2), this is an integrity bypass; today it is a latent crypto-canonicalization defect that the prior round's M5 fix introduced and missed.

**Fix:** In shared/signing/signing.go, make computeSignature (line 221) unambiguous so no inter-field byte-shifting is possible. Cleanest option: feed each field as a fixed-width digest into the HMAC instead of a delimited string. Replace the body of computeSignature (lines 222-238) with: hash each variable-length field independently and write the fixed-width SHA-256 digests in order, e.g.\n\n  h := hmac.New(sha256.New, s.secret)\n  writeField := func(b []byte) { d := sha256.Sum256(b); h.Write(d[:]) }\n  writeField([]byte(method))\n  writeField([]byte(path))\n  writeField([]byte(rawQuery))\n  writeField([]byte(serviceName))\n  var tsBuf [8]byte\n  binary.BigEndian.PutUint64(tsBuf[:], uint64(timestamp)); h.Write(tsBuf[:])\n  bodyHash := sha256.Sum256(body); h.Write(bodyHash[:])\n  return hex.EncodeToString(h.Sum(nil))\n\n(add encoding/binary import). Each field becomes a fixed 32-byte block so the '|' delimiter is gone and no field boundary can shift. Alternatively (smaller change) length-prefix each field: write a binary.BigEndian uint32 length before each field's bytes. Then update the format comment at signing.go:217-220 and README.md:124,137,140-141 to describe the per-field-digest (or length-prefixed) encoding rather than the bare '|'-joined string. Add a unit test asserting sign(\"POST\",\"/a\",\"b|x\",\"y\",...) != sign(\"POST\",\"/a|b\",\"x\",\"y\",...) and that a rawQuery/serviceName boundary shift also diverges. NOTE: this format change is backward-incompatible, but since the package has zero callers in prod today there is no rollout coordination cost — land it before the package is wired per RESIDUALS.md L2.

**Reasoning:** The canonicalization defect is REAL and correctly diagnosed. At HEAD, signing.go:221-232 builds the signed message via fmt.Sprintf("%s|%s|%s|%s|%d|%s", method, path, rawQuery, serviceName, timestamp, bodyHash) with a bare '|' delimiter and no length-prefixing or escaping. path, rawQuery, and serviceName are all caller/attacker-influenceable and can each contain a literal '|', so bytes can be shifted across field boundaries. I empirically reproduced the collision with my own secret: sig("POST","/a","b|x","y",1000,nil) == sig("POST","/a|b","x","y",1000,nil) (both => eee8cd34...620922). The collision is secret-independent, proving it is a genuine canonicalization ambiguity, not a hash coincidence. This contradicts the README (README.md:140-141) and code comment (signing.go:218-220) claiming the scheme prevents query/path/service tampering. The timestamp field is %d (digits only) so it cannot absorb a shift, bounding the ambiguity to the method/path/query/service block — exactly as the claim states. HOWEVER, severity must be downgraded: (1) grep for imports of all-chat/shared/signing across services/ and cmd/ returns ZERO hits outside the package itself — the package is NOT wired into any production verifier (CLAUDE.md and RESIDUALS.md L2 confirm "NOT YET WIRED into any prod service"), so there is no reachable exploit today; it is dead/latent code. (2) Because all services share one secret (README.md:169 "All services must use the same secret"), any legitimate caller already holds the secret and can sign any path/query directly — the cross-field shift only yields a privilege gain under a specific confused-deputy/signing-oracle service that signs a partially attacker-controlled path/query while being trusted to constrain another field. That is a plausible future scenario, not a guaranteed one. So the finding is a real, correctly-diagnosed latent crypto defect with an accurate fix, but it is not exploitable in the current deployment, making MEDIUM too high. I mark it partial at LOW: worth fixing before the package is wired, not an active prod issue.

--------------------------------------------------------------------------------

## #27 [LOW] Replay protection is timestamp-window only — no nonce/jti; a captured signed request is replayable for up to ~6 minutes
- dimension: signing | verdict: confirmed | kind: new-issue
- location: shared/signing/signing.go:43 and :152 (comments); shared/signing/README.md:15 (and :25, :184). No nonce/dedup anywhere in shared/signing/ (grep confirms 0 functional matches; README.md:318 lists it only as a future enhancement).

**Scenario:** Service A signs POST /moderation/ban {user:X}. An on-path observer (or a service with read access to inter-service traffic) captures the three X-Service-* headers + body and re-sends the exact bytes within 5 minutes. VerifyMiddleware recomputes the same HMAC (same method/path/query/service/timestamp/body), the timestamp is still within MaxRequestAge, and the duplicate ban/delete/timeout executes again. For idempotent GETs this is harmless; for state-changing internal POSTs (moderation, payment entitlement writes) the replay is a real integrity issue. Because the package is not yet wired into prod this is latent, hence LOW, but it should not be labeled 'replay prevention'.

**Fix:** Two-part fix. (1) Correct the doc/comment wording so it stops claiming full replay prevention: in shared/signing/signing.go change L43 `// Maximum age of signed requests (prevents replay attacks)` to `// Maximum age of signed requests (bounds the replay window; not full replay prevention — no nonce/dedup)` and L152 `// Check request age (prevent replay attacks, audit M4)` to `// Check request age (bounds replay window — see README "Nonce tracking" gap, audit M4)`; in shared/signing/README.md change L15 `Timestamp-based replay attack prevention (past + future skew)` to `Timestamp-based replay-window bounding (past + future skew; no per-request nonce/dedup — see Future Enhancements)` and similarly soften L25/L184. (2) Add a RESIDUALS.md note tied to the package's prod-rollout checklist: before wiring signing.go into any state-changing internal endpoint (moderation/payment), add an X-Service-Nonce to the signed message (extend computeSignature's message format and SignRequest to emit a random nonce header) and have VerifyMiddleware SETNX the nonce in Redis with TTL = MaxRequestAge+MaxFutureSkew (6min), rejecting duplicates — mirroring the EventSub dedup pattern (SECURITY_AUDIT_REPORT.md L11). The nonce should be folded into the HMAC message so it cannot be stripped/swapped. No prod-blocking change is required now because the package has zero importers.

**Reasoning:** Verified against current HEAD. shared/signing/signing.go:43 reads `// Maximum age of signed requests (prevents replay attacks)` and signing.go:152 reads `// Check request age (prevent replay attacks, audit M4)` — both exact matches. README.md:15 lists "Timestamp-based replay attack prevention" as a feature and README L25/L153/L184 reuse "prevents replay"/"Rejects stale (replayed) requests" wording. grep across shared/signing/ for nonce|jti|setnx|dedup|seen|redis returns ZERO functional matches — the only hit is README.md:318 listing "Nonce tracking" as a *future enhancement*. So the mechanism is genuinely timestamp-window-only: VerifyMiddleware (signing.go:115-214) recomputes HMAC over method|path|query|service|timestamp|body_hash (computeSignature, L221-239) and accepts ANY captured request whose timestamp is within MaxRequestAge (5min, L44/L155) and not more than MaxFutureSkew (1min, L61/L167) in the future. There is no per-request state, so a verbatim replay of all three X-Service-* headers + body re-verifies for the remaining freshness window. The scenario (on-path / privileged observer captures a state-changing internal POST and re-sends it) is technically sound. Severity is correctly LOW: (1) the package is latent — grep for `shared/signing` imports across services/ and cmd/ returns zero matches, confirming README L5 "not yet wired into any prod service", so nothing is exploitable today; (2) replay requires an attacker already on the inter-service path, and the README recommends HTTPS for inter-service comms which defeats passive capture; (3) the actionable core is doc-accuracy: the wording overstates timestamp checks as full "replay prevention" when they only bound the replay window. This is a genuine new issue, not a prior-round (M4 was the future-skew fix, which IS correctly present at L167/L251). Confirmed at the reviewer's own LOW severity.

--------------------------------------------------------------------------------

## #28 [LOW] Base kustomization ships discord-listener ServiceMonitor + default-deny NetworkPolicy but no monitoring-namespace ingress allow — scrape is silently blocked
- dimension: infra-k8s | verdict: confirmed | kind: incomplete-fix
- location: deployments/k8s/base/networkpolicies.yaml:81-113 (missing monitoring-ns ingress allow; only allow-intra-namespace + allow-external-api-gateway exist), with deployments/k8s/base/kustomization.yaml:22 (+ :48 servicemonitor) and deployments/k8s/base/discord-listener/servicemonitor.yaml:1-18 scraping :8086

**Scenario:** An operator bootstraps a fresh cluster from `kubectl apply -k deployments/k8s/base` with kube-prometheus-stack in a `monitoring` namespace. default-deny-all blocks all ingress; the only allows are intra-allchat and api-gateway:8080-from-anywhere. Prometheus pods live in `monitoring`, so the discord-listener ServiceMonitor scrape of :8086 is dropped at the CNI. Metrics silently disappear and a 'target down' alert fires, with no obvious cause because the manifests look complete. api-gateway:8080 happens to be reachable only because allow-external-api-gateway uses ipBlock 0.0.0.0/0 which incidentally matches Prometheus pod IPs.

**Fix:** Make the base layer self-consistent with the ServiceMonitor it ships. Append an ingress allow to deployments/k8s/base/networkpolicies.yaml (after L113) mirroring caesar's prod policy, permitting the monitoring namespace to reach the scraped metrics ports:
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-monitoring-scrape
  namespace: allchat
spec:
  podSelector: {}
  policyTypes:
    - Ingress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: monitoring
      ports:
        - protocol: TCP
          port: 8086   # discord-listener /metrics (matches servicemonitor.yaml)
        - protocol: TCP
          port: 8080   # api-gateway /metrics (B2)
(extend ports to cover any other scraped service metrics ports as needed). Alternatively, since the base is dev-only, drop `- discord-listener/servicemonitor.yaml` from deployments/k8s/base/kustomization.yaml:48 so the base does not ship a scrape target it cannot satisfy.

**Reasoning:** Verified all four cited locations at HEAD. networkpolicies.yaml is brand-new in this PR (git diff shows it added from /dev/null). It installs default-deny-all (Ingress+Egress, podSelector:{}) plus exactly two ingress allows: allow-intra-namespace (from podSelector:{} = same-namespace allchat pods only, L92-93) and allow-external-api-gateway (selects app=api-gateway, port 8080, from ipBlock 0.0.0.0/0, L101-113). The ONLY namespaceSelector in the file is the kube-dns egress rule (L49). There is NO ingress rule permitting a separate monitoring/kube-prometheus namespace to reach any metrics port. kustomization.yaml includes BOTH networkpolicies.yaml (L22) and discord-listener/servicemonitor.yaml (L48). The ServiceMonitor scrapes discord-listener port http = containerPort 8086 /metrics (confirmed in discord-listener/deployment.yaml L42-44). A Prometheus pod in a separate `monitoring` namespace is NOT a same-namespace allchat pod, so allow-intra-namespace does not cover it and default-deny drops the :8086 scrape at the CNI. Confirmed prod is unaffected: caesar-deployment/apps/workloads/all-chat/network-policies.yaml contains 10 `namespaceSelector: kubernetes.io/metadata.name: monitoring` allow rules, and prod deploys from caesar, not this base. The bug is scoped exactly as claimed to `kubectl apply -k deployments/k8s/base` (dev/CI/bootstrap); the only overlay present is overlays/tls which adds no monitoring allow. The api-gateway:8080 happens to remain scrapable only incidentally via the 0.0.0.0/0 ipBlock matching Prometheus pod IPs, as the claim states. This is a genuine self-inconsistency: the base ships a ServiceMonitor whose scrape its own bundled NetworkPolicy blocks. It is not a security hole and not a prod outage — metrics are silently missing only on the dev/bootstrap path — so LOW is correct.

--------------------------------------------------------------------------------

## #29 [LOW] support-bot pod runs as root despite the H6 hardening wave (no runAsNonRoot/runAsUser)
- dimension: infra-k8s | verdict: partial | kind: incomplete-fix
- location: deployments/k8s/base/support-bot/deployment.yaml:19-21 (pod securityContext omits runAsNonRoot — real gap); L69-74 main container has no runAsNonRoot but Dockerfile:33 `USER node` makes it non-root anyway; L26-31 & L47-52 alpine/git initContainers genuinely run as root

**Scenario:** An operator applies the base layer trusting the H6 'all pods run non-root' claim. support-bot's containers run as root, contradicting the stated invariant. There is no functional break, but the security posture the PR advertises (runAsNonRoot everywhere) is not actually met for this pod.

**Fix:** In deployments/k8s/base/support-bot/deployment.yaml: (a) Add `runAsNonRoot: true` to the MAIN container securityContext at L69-74 (the image already runs as `USER node`/uid 1000 per Dockerfile:33, so this is purely an enforcement guard — it will pass and restores the H6 invariant for the secret-reading container). Do NOT add `runAsNonRoot:true` at the pod level, because (b) the two alpine/git initContainers (L26-31, L47-52) have no non-root user in the image and would crash on schedule; for those, either explicitly set `runAsUser: 65534` (nobody) + make the /repos emptyDir volumes group/world-writable, or leave them root and add a comment `# alpine/git initContainers run as root: ephemeral clone into emptyDir, ALL caps dropped, no long02-lived process`. Minimal honest fix: add `runAsNonRoot: true` to the main container at L70 and a one-line comment on the init containers documenting why they remain root.

**Reasoning:** Verified against HEAD. The structural gap is REAL: of the 16 base deployments, 15 had pod-level `runAsNonRoot:true`+`runAsUser:1000` added IN THIS PR (api-gateway L21-24, etc.); support-bot/deployment.yaml L19-21 is the ONLY one whose pod securityContext sets only seccompProfile.RuntimeDefault and omits runAsNonRoot/runAsUser. SECURITY_AUDIT_REPORT.md:45 explicitly scopes "support-bot" into H6 and L47 prescribes pod-level runAsNonRoot:true, so this is a genuine consistency gap in the H6 wave.\n\nBUT the claim's central security argument is FALSE. It asserts the MAIN support-bot container "runs as uid 0 (root) from its base image." It does not: services/support-bot/Dockerfile:33 sets `USER node`, and node:20-alpine's `node` user is uid 1000. The main support-bot container (the one with readOnlyRootFilesystem:true, reading the Discord/Claude/GitHub secrets) runs as NON-ROOT regardless of the missing pod-level setting. The reviewer's evidence/scenario hinge on "container runs as uid 0" — that premise is refuted by the image USER directive.\n\nWhat IS a real residual: the two `alpine/git` initContainers (L23-43, L44-64) run as root — `alpine/git` has no USER directive and the init securityContext omits runAsUser (verified: NO runAsUser anywhere in the file). They run a root-owned `git clone` that reads SUPPORT_BOT_GITHUB_TOKEN into an emptyDir. They do drop ALL caps + disable privilege escalation, so the exposure is small and the data cloned is the bot's own repos, not other pods' secrets. So the real issue is narrower than and different from what the claim describes (init containers, not the main container).\n\nNote on the fix: a naive pod-level `runAsNonRoot:true` would FAIL the alpine/git init containers (no non-root user baked in) unless an explicit runAsUser is set on them, so the fix needs per-container handling, not a blanket pod setting.

--------------------------------------------------------------------------------

## #30 [LOW] resolver.go reads InnerTube/oEmbed response bodies with unbounded io.ReadAll while the sibling innertube service was capped in the same PR
- dimension: tts-ssrf-eventsub-misc | verdict: confirmed | kind: incomplete-fix
- location: services/overlay-manager/youtube/resolver.go:157, :197, :283

**Scenario:** POST /overlay/.../youtube-resolve (handlers/youtube.go:69 -> ResolveToChannelID) triggers an HTTPS call to www.youtube.com (innertube resolve_url / oEmbed / browse). If the upstream (or anything able to MITM/spoof that hop, or a future redirect/proxy) returns a multi-gigabyte body, io.ReadAll buffers the entire response into memory in the overlay-manager pod, risking an OOMKill. The blast radius is limited because the host is a fixed trusted www.youtube.com endpoint over TLS, so it is not directly attacker-controlled — hence LOW — but it is the exact memory-exhaustion vector the PR capped everywhere else.

**Fix:** In services/overlay-manager/youtube/resolver.go, wrap each of the three reads with io.LimitReader using the same 10<<20 cap the sibling service uses: L157 `body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))` (resolveHandleToChannelID), L197 `body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))` (resolveVideoToChannelID/oEmbed), L283 `body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))` (innertubeBrowse). io is already imported, so no import change is needed. Optionally also cap config.go:70 in the innertube service for full parity, but that is outside this finding's scope.

**Reasoning:** Verified all three factual legs at HEAD. (1) The PR diff for the sibling service changes io.ReadAll(resp.Body) -> io.ReadAll(io.LimitReader(resp.Body, 10<<20)) in services/youtube-listener-innertube/innertube/client.go:172 and discovery.go:127/310/644 (4 hunks confirmed in the diff). (2) services/overlay-manager/youtube/resolver.go at HEAD still has unbounded io.ReadAll(resp.Body) at L157 (resolveHandleToChannelID), L197 (resolveVideoToChannelID/oEmbed), and L283 (innertubeBrowse). (3) The PR DID touch resolver.go (added ALLCHAT_INNERTUBE_KEY env plumbing, I4/L26 per the diff), so this file was in scope for the hardening wave yet the same body-size cap was not applied. This is current, not stale — d577723f did not address it. On exploitability: the reviewer's own scoping is accurate. The request handler (handlers/youtube.go:61 ResolveChannel -> :69 ResolveToChannelID) only lets req.Input influence the path/query of a hardcoded https://www.youtube.com/... endpoint over TLS; the host/scheme are never attacker-controlled and there is no abnormal redirect-to-attacker-host. So this is defense-in-depth consistency, not a directly attacker-driven OOM — correctly LOW. One nuance weakening the 'capped everywhere else' framing: services/youtube-listener-innertube/innertube/config.go:70 is also still unbounded, so the cap was not literally applied to every read even in the sibling service. But the three resolver.go reads are the same class of InnerTube/oEmbed upstream call that did get capped, so the incomplete-fix finding stands.

--------------------------------------------------------------------------------