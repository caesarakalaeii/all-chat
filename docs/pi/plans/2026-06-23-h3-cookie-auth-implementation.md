# H3 Streamer/Admin Cookie Auth Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate streamer/admin JWTs from localStorage to httpOnly; Secure; SameSite=Lax cookies, with the api-gateway as the cookie boundary (backends unchanged) and SameSite=Lax + stateless Origin-check CSRF defense.

**Architecture:** auth-service issues/clears cookies; gateway runs two new middlewares — `CookieToBearer` (protected routes, copies access cookie → `Authorization` before the shared `JWTAuthWithRevocation`) and `AuthCookieForward` (auth routes, forwards cookies as `X-Access-Token`/`X-Refresh-Token` headers that bypass the L17 proxy `Cookie` strip). Frontend fetch wrappers stop attaching `Authorization` and gain a 401→refresh→retry interceptor; `auth-store` derives login state from `/auth/me`. Impersonation moves server-side (`/impersonate` + `/stop-impersonation` + Redis-stashed admin identity).

**Tech Stack:** Go 1.25 / Gin / go-redis / jwt-v5 (backend); Next.js / React 19 / TypeScript / Zustand (frontend).

**Spec:** `docs/pi/specs/2026-06-23-h3-cookie-auth-design.md`

**Branch:** `security/audit-hardening` (continue on it)

---

## File Structure

### Backend — Go
| File | Responsibility | Action |
|---|---|---|
| `shared/auth/cookie.go` | Cookie name constants + helpers (`CookieAccessToken`, `CookieRefreshToken`, `SetAuthCookies`, `ClearAuthCookies`). | Create |
| `shared/auth/jwt.go` | Add `expiry time.Duration` param to `GenerateImpersonationJWTWithKid`; generate + set `jti` (`RegisteredClaims.ID`). | Modify |
| `shared/middleware/origin_check.go` | Stateless CSRF `OriginCheck(allowed []string)` middleware (POST/PUT/DELETE/PATCH). | Create |
| `shared/middleware/origin_check_test.go` | Tests for allow/absent/deny. | Create |
| `services/auth-service/handlers/auth_handler.go` | `HandleStreamerTokenExchange` sets cookies (no tokens in body); `HandleRefresh` reads `X-Refresh-Token` + rotates cookies; `HandleLogout` clears cookies + revokes refresh Redis key. | Modify |
| `services/auth-service/handlers/admin.go` | `ImpersonateUser` sets impersonated-user cookie + Redis-stashed admin identity (jti-bound). | Modify |
| `services/auth-service/handlers/admin_impersonation.go` | New `HandleStopImpersonation` (reads `X-Access-Token`, restores admin cookie). | Create |
| `services/auth-service/cmd/main.go` | Register `POST /stop-impersonation` on the protected group. | Modify |
| `services/api-gateway/middleware/cookie_to_bearer.go` | `CookieToBearer()` — copies access cookie → `Authorization` (no-op if header already present). | Create |
| `services/api-gateway/middleware/cookie_to_bearer_test.go` | Tests. | Create |
| `services/api-gateway/middleware/auth_cookie_forward.go` | `AuthCookieForward()` — sets `X-Access-Token`/`X-Refresh-Token` headers from cookies. | Create |
| `services/api-gateway/middleware/auth_cookie_forward_test.go` | Tests. | Create |
| `services/api-gateway/middleware/cors.go` | Export the parsed `CORS_ORIGIN` origins (refactor `parseOrigins` to a shared `loadHTTPAllowedOrigins`). | Modify |
| `services/api-gateway/cmd/main.go` | Wire `CookieToBearer` before `JWTAuthWithRevocation` on protected/admin/metrics; switch those to `JWTAuthWithRevocation`; wire `AuthCookieForward` on `/auth/refresh`,`/auth/logout`,`/auth/stop-impersonation`; wire `OriginCheck`; register `protectedAPI.POST("/auth/stop-impersonation", proxyHandler.ForwardRequest)`. | Modify |

### Frontend — TypeScript
| File | Responsibility | Action |
|---|---|---|
| `frontend/src/lib/api/client.ts` | Remove `localStorage`/`Authorization` attach; add 401→refresh→retry interceptor. | Modify |
| `frontend/src/lib/api/overlays.ts` | Remove `inMemoryTokens`/localStorage token reads + `Authorization` attach. | Modify |
| `frontend/src/lib/api/chat.ts` | Inherits `apiClient` — verify no direct token reads. | Verify |
| `frontend/src/lib/api/moderation.ts` | Verify uses `apiClient` (no direct token reads). | Verify |
| `frontend/src/lib/stores/auth-store.ts` | Drop `token` state + `jwt_token`/`admin_token`/`impersonating`/`impersonated_user` localStorage; `init` calls `/auth/me`; `startImpersonation`/`stopImpersonation` call the new endpoints. | Modify |
| `frontend/src/lib/auth/in-memory-store.ts` | Remove streamer/admin fields (`accessToken`,`refreshToken`,`adminToken`,`impersonating`,`impersonatedUsername`); KEEP `viewerAccessToken`. | Modify |
| `frontend/src/app/auth/callback/page.tsx` | After `POST /exchange` (cookie-setting), read user + `redirect_to` from body only. | Modify |
| `frontend/src/lib/api/auth.ts` | Add `stopImpersonation()` wrapper if needed (or fold into store). | Modify |

---

## Domain Ordering

Three disjoint domains for parallel execution. Domain ordering shows dependencies; within each, tasks are sequential (TDD).

- **Domain A (shared + auth-service):** Tasks 1–7. No cross-domain deps.
- **Domain B (gateway):** Tasks 8–13. Depends on Task 1 (`shared/auth/cookie.go` constants) + Task 4 (`shared/middleware/origin_check.go`). Run B after A, or at least after Task 1+4.
- **Domain C (frontend):** Tasks 14–20. Depends on A+B being deployable (backend contract), but TypeScript changes can be written against the documented contract. Run C in parallel with B; integrate last.

---

## Domain A — shared + auth-service

### Task 1: Create `shared/auth/cookie.go`

**Files:**
- Create: `shared/auth/cookie.go`

- [ ] **Step 1: Write the failing test**

`shared/auth/cookie_test.go`:

```go
package auth

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestSetAuthCookies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	SetAuthCookies(c, "access-jwt", "refresh-jwt", time.Hour, 14*24*time.Hour)

	cookies := w.Result().Cookies()
	var gotAccess, gotRefresh bool
	for _, ck := range cookies {
		switch ck.Name {
		case CookieAccessToken:
			gotAccess = true
			if ck.Value != "access-jwt" { t.Errorf("access value=%s", ck.Value) }
			if !ck.HttpOnly { t.Error("access not HttpOnly") }
			if !ck.Secure { t.Error("access not Secure") }
			if ck.SameSite != http.SameSiteLaxMode { t.Error("access not Lax") }
			if ck.Path != "/" { t.Errorf("access path=%s", ck.Path) }
		case CookieRefreshToken:
			gotRefresh = true
			if ck.Value != "refresh-jwt" { t.Errorf("refresh value=%s", ck.Value) }
			if ck.Path != "/api/v1/auth/" { t.Errorf("refresh path=%s", ck.Path) }
		}
	}
	if !gotAccess { t.Error("access cookie missing") }
	if !gotRefresh { t.Error("refresh cookie missing") }
}

func TestClearAuthCookies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	ClearAuthCookies(c)

	cookies := w.Result().Cookies()
	if len(cookies) != 2 { t.Fatalf("want 2 cleared cookies, got %d", len(cookies)) }
	for _, ck := range cookies {
		if ck.MaxAge != -1 && ck.Value != "" { t.Errorf("cookie %s not cleared", ck.Name) }
		if ck.Name == CookieRefreshToken && ck.Path != "/api/v1/auth/" {
			t.Errorf("refresh clear path=%s", ck.Path)
		}
	}
}
```

Add `"net/http"` import to the test file.

- [ ] **Step 2: Run test to verify it fails**

```
cd shared/auth && go test -run TestSetAuthCookies -v
```
Expected: FAIL — `SetAuthCookies`/`ClearAuthCookies`/`CookieAccessToken` undefined.

- [ ] **Step 3: Write minimal implementation**

`shared/auth/cookie.go`:

```go
package auth

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Cookie names shared by auth-service (issuer) and the api-gateway (reader).
const (
	CookieAccessToken  = "access_token"
	CookieRefreshToken = "refresh_token"
)

// SetAuthCookies issues httpOnly; Secure; SameSite=Lax cookies for the access
// and refresh JWTs. The access cookie is Path=/ (every same-origin request);
// the refresh cookie is Path=/api/v1/auth/ (only auth routes receive it).
// See docs/pi/specs/2026-06-23-h3-cookie-auth-design.md.
func SetAuthCookies(c *gin.Context, accessToken, refreshToken string, accessTTL, refreshTTL time.Duration) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     CookieAccessToken,
		Value:    accessToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(accessTTL.Seconds()),
	})
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     CookieRefreshToken,
		Value:    refreshToken,
		Path:     "/api/v1/auth/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(refreshTTL.Seconds()),
	})
}

// ClearAuthCookies expires both auth cookies (Max-Age=0) so the browser
// deletes them. The refresh cookie Path must match the one used at issue time.
func ClearAuthCookies(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: CookieAccessToken, Value: "", Path: "/", HttpOnly: true, Secure: true,
		SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
	http.SetCookie(c.Writer, &http.Cookie{
		Name: CookieRefreshToken, Value: "", Path: "/api/v1/auth/", HttpOnly: true, Secure: true,
		SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

```
cd shared/auth && go test -run 'TestSetAuthCookies|TestClearAuthCookies' -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add shared/auth/cookie.go shared/auth/cookie_test.go
git commit -m "✨ feat(auth): add httpOnly cookie helpers for H3 cookie-auth migration"
```

---

### Task 2: Add configurable expiry + jti to `GenerateImpersonationJWTWithKid`

**Files:**
- Modify: `shared/auth/jwt.go` (the `GenerateImpersonationJWTWithKid` function, ~line 408)
- Modify: `services/auth-service/handlers/admin.go:179` (the one caller)
- Test: `shared/auth/jwt_test.go`

- [ ] **Step 1: Write the failing test**

Append to `shared/auth/jwt_test.go`:

```go
func TestGenerateImpersonationJWTWithKid_ConfigurableExpiryAndJTI(t *testing.T) {
	kid := "v1"
	secret := []byte("0123456789abcdef0123456789abcdef")
	tok, err := auth.GenerateImpersonationJWTWithKidExpiry(kid, "admin-1", "admin", "user-2", "user", "twitch-9", secret, 30*time.Minute)
	if err != nil { t.Fatalf("sign: %v", err) }

	claims, err := auth.ValidateJWTWithKeyChain(tok, &auth.KeyChain{ByKid: map[string][]byte{kid: secret}, LatestKid: kid})
	if err != nil { t.Fatalf("validate: %v", err) }

	if claims.ImpersonatedBy != "admin-1" { t.Errorf("impersonated_by=%s", claims.ImpersonatedBy) }
	if claims.ID == "" { t.Error("jti not set") }
	if !claims.ExpiresAt.Time.After(time.Now().Add(29 * time.Minute)) {
		t.Errorf("expiry not respected: %v", claims.ExpiresAt.Time)
	}
}
```

> NOTE: confirm the exact `ValidateJWTWithKeyChain` signature + `KeyChain` field names (`ByKid`, `LatestKid`) in `shared/auth/jwt.go` before finalizing the test. Adapt the test to the real signature. The assertion intent: the new function respects the passed `expiry` (30m, not the old hardcoded 2h), sets `ImpersonatedBy`, and sets a non-empty `jti` (`claims.ID`).

- [ ] **Step 2: Run test to verify it fails**

```
cd shared/auth && go test -run TestGenerateImpersonationJWTWithKid_ConfigurableExpiryAndJTI -v
```
Expected: FAIL — function undefined.

- [ ] **Step 3: Write minimal implementation**

In `shared/auth/jwt.go`, rename `GenerateImpersonationJWTWithKid` → `GenerateImpersonationJWTWithKidExpiry` and add the `expiry` param + jti:

```go
// GenerateImpersonationJWTWithKidExpiry signs an impersonation JWT with a
// configurable expiry (replaces the legacy hardcoded 2h). Sets a jti so
// callers can bind server-side state (e.g. impersonation: Redis stash) to
// the token. (audit H3)
func GenerateImpersonationJWTWithKidExpiry(kid, adminUserID, adminUsername, targetUserID, targetUsername, targetTwitchID string, secret []byte, expiry time.Duration) (string, error) {
	roles := []string{"user", "admin"}
	jti, err := generateJTI()
	if err != nil {
		return "", fmt.Errorf("generate jti: %w", err)
	}
	claims := Claims{
		UserID:           targetUserID,
		TwitchID:         targetTwitchID,
		Username:         targetUsername,
		Roles:            roles,
		ImpersonatedBy:   adminUserID,
		ImpersonatedUser: targetUserID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			Issuer:    "all-chat-admin",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["kid"] = kid
	return token.SignedString(secret)
}

// generateJTI returns a URL-safe random token ID.
func generateJTI() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
```

Add imports `"crypto/rand"`, `"encoding/hex"` to `jwt.go` if not present.

Keep a backward-compat shim so the existing caller compiles during the transition — OR update the single caller in Task 3. Simplest: update the caller now (Task 2 Step 3b):

- [ ] **Step 3b: Update the single caller**

In `services/auth-service/handlers/admin.go` (~line 179), change:

```go
	token, err := auth.GenerateImpersonationJWTWithKid(
		h.userKeyChain.LatestKid(),
		adminUserID.(string),
		adminUsername.(string),
		targetUser.ID,
		targetUser.Username,
		targetTwitchID,
		string(h.userKeyChain.LatestSecret()),
	)
```
to:
```go
	token, err := auth.GenerateImpersonationJWTWithKidExpiry(
		h.userKeyChain.LatestKid(),
		adminUserID.(string),
		adminUsername.(string),
		targetUser.ID,
		targetUser.Username,
		targetTwitchID,
		h.userKeyChain.LatestSecret(),
		h.jwtExpiry,
	)
```

> NOTE: confirm `h.jwtExpiry` exists on `AdminHandler` (it may be on `AuthHandler`). If `AdminHandler` lacks it, either add a field or read `JWT_EXPIRY_HOURS` via the existing config path. Check `services/auth-service/handlers/admin.go` constructor + `cmd/main.go` wiring. Adapt to the real field name.

- [ ] **Step 4: Run tests to verify they pass**

```
cd shared/auth && go test ./...
cd services/auth-service && go build ./... && go vet ./...
```
Expected: PASS + clean build.

- [ ] **Step 5: Commit**

```bash
git add shared/auth/jwt.go shared/auth/jwt_test.go services/auth-service/handlers/admin.go
git commit -m "✨ feat(auth): configurable expiry + jti on impersonation JWT (H3)"
```

---

### Task 3: `HandleStreamerTokenExchange` sets cookies (no tokens in body)

**Files:**
- Modify: `services/auth-service/handlers/auth_handler.go` — `HandleStreamerTokenExchange` (~line 444)

- [ ] **Step 1: Write the failing test**

In `services/auth-service/handlers/auth_handler_test.go` (create if absent):

```go
func TestHandleStreamerTokenExchange_SetsCookies_OmitsTokensFromBody(t *testing.T) {
	// miniredis + store a streamer_auth_code:<uuid> payload
	rdb := miniredis.Run(t)
	defer rdb.Close()
	h := newTestAuthHandler(t, rdb)
	payload := StreamerAuthPayload{AccessToken: "acc", RefreshToken: "ref", ExpiresIn: 3600, TokenType: "Bearer", RedirectTo: "/dashboard"}
	data, _ := json.Marshal(payload)
	code := "code-123"
	rdb.Set("streamer_auth_code:"+code, string(data))

	router := gin.New()
	router.POST("/exchange", h.HandleStreamerTokenExchange)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/exchange", strings.NewReader(`{"code":"`+code+`"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != 200 { t.Fatalf("status=%d body=%s", w.Code, w.Body.String()) }

	// cookies
	var gotAccess, gotRefresh bool
	for _, c := range w.Result().Cookies() {
		if c.Name == "access_token" && c.Value == "acc" { gotAccess = true }
		if c.Name == "refresh_token" && c.Value == "ref" { gotRefresh = true }
	}
	if !gotAccess { t.Error("access cookie not set") }
	if !gotRefresh { t.Error("refresh cookie not set") }

	// body must NOT contain tokens
	body := w.Body.String()
	if strings.Contains(body, "acc") || strings.Contains(body, "ref") {
		t.Errorf("tokens leaked in body: %s", body)
	}
	if strings.Contains(body, "redirect_to") == false { t.Error("redirect_to missing from body") }
}
```

> NOTE: confirm the test-helper constructor name (`newTestAuthHandler`) + `StreamerAuthPayload` field names in `viewer_auth.go` (~line 126). Adapt the test to the real constructor. The assertions: cookies set with the right values; JSON body omits `access_token`/`refresh_token` but keeps `redirect_to`/`expires_in`.

- [ ] **Step 2: Run test to verify it fails**

```
cd services/auth-service && go test -run TestHandleStreamerTokenExchange_SetsCookies -v
```
Expected: FAIL — current handler returns the full payload (tokens in body), no cookies.

- [ ] **Step 3: Write minimal implementation**

In `HandleStreamerTokenExchange`, after `json.Unmarshal` of the payload, replace the final `c.JSON(http.StatusOK, payload)` with cookie-set + redacted body:

```go
	// Issue httpOnly cookies (audit H3). Tokens live in cookies, not the body.
	auth.SetAuthCookies(c, payload.AccessToken, payload.RefreshToken,
		time.Duration(payload.ExpiresIn)*time.Second, 14*24*time.Hour)

	// Body carries only non-secret data for the UI.
	c.JSON(http.StatusOK, gin.H{
		"expires_in":          payload.ExpiresIn,
		"token_type":          payload.TokenType,
		"redirect_to":         payload.RedirectTo,
		"source_added":        payload.SourceAdded,
		"moderation_enabled":  payload.ModerationEnabled,
	})
```

Add imports: `"github.com/caesar/all-chat/shared/auth"` (if not already), `"time"` (likely already imported).

- [ ] **Step 4: Run test to verify it passes**

```
cd services/auth-service && go test -run TestHandleStreamerTokenExchange -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/auth-service/handlers/auth_handler.go services/auth-service/handlers/auth_handler_test.go
git commit -m "✨ feat(auth): /exchange sets httpOnly cookies, omits tokens from body (H3)"
```

---

### Task 4: Create `shared/middleware/origin_check.go` (CSRF)

**Files:**
- Create: `shared/middleware/origin_check.go`
- Create: `shared/middleware/origin_check_test.go`

- [ ] **Step 1: Write the failing test**

`shared/middleware/origin_check_test.go`:

```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestOriginCheck_AllowsAllowedOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(OriginCheck([]string{"https://allch.at"}))
	r.POST("/x", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/x", nil)
	req.Header.Set("Origin", "https://allch.at")
	r.ServeHTTP(w, req)
	if w.Code != 200 { t.Errorf("want 200 got %d", w.Code) }
}

func TestOriginCheck_RejectsDisallowedOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(OriginCheck([]string{"https://allch.at"}))
	r.POST("/x", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/x", nil)
	req.Header.Set("Origin", "https://evil.example")
	r.ServeHTTP(w, req)
	if w.Code != 403 { t.Errorf("want 403 got %d", w.Code) }
}

func TestOriginCheck_AllowsAbsentOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(OriginCheck([]string{"https://allch.at"}))
	r.POST("/x", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/x", nil) // no Origin/Referer
	r.ServeHTTP(w, req)
	if w.Code != 200 { t.Errorf("absent Origin should be allowed (non-browser), got %d", w.Code) }
}

func TestOriginCheck_SkipsGet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(OriginCheck([]string{"https://allch.at"}))
	r.GET("/x", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Origin", "https://evil.example")
	r.ServeHTTP(w, req)
	if w.Code != 200 { t.Errorf("GET should be allowed regardless of Origin, got %d", w.Code) }
}
```

- [ ] **Step 2: Run test to verify it fails**

```
cd shared/middleware && go test -run TestOriginCheck -v
```
Expected: FAIL — `OriginCheck` undefined.

- [ ] **Step 3: Write minimal implementation**

`shared/middleware/origin_check.go`:

```go
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// OriginCheck is a stateless CSRF defense paired with SameSite=Lax cookies.
// On state-changing methods (POST/PUT/DELETE/PATCH), if the request carries
// an Origin (or Referer fallback), it must be in the allowed list. Absent
// Origin/Referer is allowed (non-browser API clients authenticate via
// Authorization header). Safe methods (GET/HEAD/OPTIONS) are not checked.
// See docs/pi/specs/2026-06-23-h3-cookie-auth-design.md.
func OriginCheck(allowed []string) gin.HandlerFunc {
	allowedSet := make(map[string]bool, len(allowed))
	for _, o := range allowed {
		allowedSet[o] = true
	}
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
		default:
			c.Next()
			return
		}
		origin := c.GetHeader("Origin")
		if origin == "" {
			origin = c.GetHeader("Referer")
			// Referer is a full URL; strip to origin (scheme://host).
			if origin != "" {
				if u, err := url.Parse(origin); err == nil && u.Host != "" {
					origin = u.Scheme + "://" + u.Host
				}
			}
		}
		if origin == "" {
			// Non-browser client (no Origin/Referer) — allowed; relies on
			// Authorization header for auth.
			c.Next()
			return
		}
		if !allowedSet[origin] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "origin not allowed"})
			return
		}
		c.Next()
	}
}
```

Add `"net/url"` import.

- [ ] **Step 4: Run test to verify it passes**

```
cd shared/middleware && go test -run TestOriginCheck -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add shared/middleware/origin_check.go shared/middleware/origin_check_test.go
git commit -m "✨ feat(middleware): stateless Origin-check CSRF defense (H3)"
```

---

### Task 5: `HandleRefresh` reads `X-Refresh-Token` + rotates cookies

**Files:**
- Modify: `services/auth-service/handlers/auth_handler.go` — `HandleRefresh` (~line 485)

- [ ] **Step 1: Write the failing test**

Append to `auth_handler_test.go`:

```go
func TestHandleRefresh_ReadsHeaderAndSetsCookies(t *testing.T) {
	// miniredis: pre-seed refresh_token:<hash> for "old-rt"
	rdb := miniredis.Run(t)
	defer rdb.Close()
	hash := refreshTokenHash("old-rt")
	rdb.Set("refresh_token:"+hash, "user-1", 0)

	// stub the twitchOAuth.RefreshToken to return new tokens (use the existing
	// mock pattern in this test file — confirm the mock name).
	h := newTestAuthHandlerWithMockOAuth(t, rdb, &oauthRefreshMock{
		accessToken:  "new-twitch-access",
		refreshToken: "new-rt",
		expiry:       time.Now().Add(24 * time.Hour),
		userID:      "twitch-1",
	})
	// also seed a user in the mock userRepo
	h.userRepo.(*mockUserRepo).users["twitch-1"] = &models.User{ID: "user-1", TwitchID: strPtr("twitch-1"), Username: "u"}

	router := gin.New()
	router.POST("/refresh", h.HandleRefresh)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/refresh", nil)
	req.Header.Set("X-Refresh-Token", "old-rt")
	router.ServeHTTP(w, req)

	if w.Code != 200 { t.Fatalf("status=%d body=%s", w.Code, w.Body.String()) }

	// old-rt must be consumed (reuse detection)
	if rdb.Exists("refresh_token:" + hash) { t.Error("old refresh token not consumed") }

	// cookies rotated
	var gotAccess bool
	for _, c := range w.Result().Cookies() {
		if c.Name == "access_token" && c.Value != "" { gotAccess = true }
	}
	if !gotAccess { t.Error("access cookie not set on refresh") }

	// body must NOT contain access_token / refresh_token
	body := w.Body.String()
	if strings.Contains(body, "new-twitch-access") || strings.Contains(body, "new-rt") {
		t.Errorf("tokens leaked in body: %s", body)
	}
}
```

> NOTE: this test references mock helpers that may not exist. Adapt to the existing mock patterns in `auth_handler_test.go` (check what's already used for `twitchOAuth` / `userRepo` mocks — e.g. the existing `TestHandleRefresh_*` tests if any). The assertions: `X-Refresh-Token` header read; old Redis key consumed; new cookies set; body has no tokens.

- [ ] **Step 2: Run test to verify it fails**

```
cd services/auth-service && go test -run TestHandleRefresh_ReadsHeaderAndSetsCookies -v
```
Expected: FAIL — current `HandleRefresh` reads JSON body, no cookie/header read, returns tokens in body.

- [ ] **Step 3: Write minimal implementation**

Rewrite the token-read at the top of `HandleRefresh`:

```go
func (h *AuthHandler) HandleRefresh(c *gin.Context) {
	// Read refresh token from the X-Refresh-Token header (forwarded by the
	// gateway AuthCookieForward middleware; the raw Cookie is stripped by L17
	// before reaching auth-service). Fallback to JSON body for backward compat
	// during rollout (deprecated).
	refreshToken := c.GetHeader("X-Refresh-Token")
	if refreshToken == "" {
		var req struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := c.ShouldBindJSON(&req); err == nil {
			refreshToken = req.RefreshToken
		}
	}
	if refreshToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refresh token required"})
		return
	}

	// Reuse detection (audit M2) — unchanged.
	tokenHash := refreshTokenHash(refreshToken)
	rtKey := "refresh_token:" + tokenHash
	_, err := h.redis.GetDel(c.Request.Context(), rtKey).Result()
	if err != nil {
		h.logger.Warn("Refresh token reuse detected — token not in active set",
			zap.String("refresh_token_hash", tokenHash[:16]),
			zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Refresh token already used or invalid"})
		return
	}

	token, err := h.twitchOAuth.RefreshToken(c.Request.Context(), refreshToken)
	// ... (rest unchanged: GetUserInfoTwitch, GetByTwitchID, UpdateTokens, set new RT Redis key)
```

At the end, replace the `c.JSON(http.StatusOK, models.TokenResponse{...})` with cookie-set + redacted body:

```go
	// Issue new access JWT.
	jwtToken, err := auth.GenerateTokenWithKid(h.userKeyChain.LatestKid(), user.ID, user.Username, string(h.userKeyChain.LatestSecret()), h.jwtExpiry, user.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Rotate cookies (audit H3).
	auth.SetAuthCookies(c, jwtToken, token.RefreshToken, h.jwtExpiry, 14*24*time.Hour)

	c.JSON(http.StatusOK, gin.H{
		"expires_in":  int(h.jwtExpiry.Seconds()),
		"token_type":  "Bearer",
		"user":        gin.H{"id": user.ID, "username": user.Username, "is_admin": user.IsAdmin},
	})
```

Add the `auth` import if not present.

- [ ] **Step 4: Run test to verify it passes**

```
cd services/auth-service && go test -run TestHandleRefresh -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/auth-service/handlers/auth_handler.go services/auth-service/handlers/auth_handler_test.go
git commit -m "✨ feat(auth): /refresh reads X-Refresh-Token, rotates cookies (H3)"
```

---

### Task 6: `HandleLogout` clears cookies + revokes refresh Redis key

**Files:**
- Modify: `services/auth-service/handlers/auth_handler.go` — `HandleLogout` (~line 585)

- [ ] **Step 1: Write the failing test**

```go
func TestHandleLogout_ClearsCookiesAndRevokesRefresh(t *testing.T) {
	rdb := miniredis.Run(t)
	defer rdb.Close()
	// pre-seed a blacklist-empty state; pre-seed a refresh_token:<hash>
	hash := refreshTokenHash("some-rt")
	rdb.Set("refresh_token:"+hash, "user-1", 0)

	h := newTestAuthHandler(t, rdb)
	router := gin.New()
	// gateway would set X-Access-Token + X-Refresh-Token; simulate
	router.POST("/logout", h.HandleLogout)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/logout", nil)
	req.Header.Set("X-Access-Token", "jwt-to-blacklist")
	req.Header.Set("X-Refresh-Token", "some-rt")
	router.ServeHTTP(w, req)

	if w.Code != 200 { t.Fatalf("status=%d body=%s", w.Code, w.Body.String()) }

	// access token blacklisted
	if !rdb.Exists("blacklist:jwt-to-blacklist") { t.Error("access token not blacklisted") }
	// refresh token Redis key deleted
	if rdb.Exists("refresh_token:" + hash) { t.Error("refresh token not revoked") }

	// cookies cleared
	var cleared int
	for _, c := range w.Result().Cookies() {
		if c.Value == "" || c.MaxAge == -1 { cleared++ }
	}
	if cleared < 2 { t.Errorf("want 2 cleared cookies, got %d", cleared) }
}
```

> NOTE: `HandleLogout` currently reads `Authorization: Bearer`. This test sends `X-Access-Token` (the post-cookie path). Adapt the test helper to match `HandleLogout`'s real constructor.

- [ ] **Step 2: Run test to verify it fails**

```
cd services/auth-service && go test -run TestHandleLogout_ClearsCookiesAndRevokesRefresh -v
```
Expected: FAIL — current logout reads `Authorization`, doesn't clear cookies, doesn't revoke refresh.

- [ ] **Step 3: Write minimal implementation**

Rewrite the token-read at the top of `HandleLogout`:

```go
func (h *AuthHandler) HandleLogout(c *gin.Context) {
	// Read token from X-Access-Token (gateway AuthCookieForward) or
	// Authorization (backward compat).
	token := c.GetHeader("X-Access-Token")
	if token == "" {
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Blacklist the access JWT (expires after JWT expiry).
	if err := h.redis.Set(c.Request.Context(), "blacklist:"+token, "1", h.jwtExpiry).Err(); err != nil {
		h.logger.Error("Failed to blacklist token", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
		return
	}

	// Revoke the refresh token family entry (audit H3).
	if rt := c.GetHeader("X-Refresh-Token"); rt != "" {
		rtKey := "refresh_token:" + refreshTokenHash(rt)
		if err := h.redis.Del(c.Request.Context(), rtKey).Err(); err != nil {
			h.logger.Warn("Failed to revoke refresh token on logout", zap.Error(err))
		}
	}

	// Clear the auth cookies.
	auth.ClearAuthCookies(c)

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}
```

- [ ] **Step 4: Run test to verify it passes**

```
cd services/auth-service && go test -run TestHandleLogout -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/auth-service/handlers/auth_handler.go services/auth-service/handlers/auth_handler_test.go
git commit -m "✨ feat(auth): /logout clears cookies + revokes refresh token (H3)"
```

---

### Task 7: Impersonation cookie issuance + `HandleStopImpersonation`

**Files:**
- Modify: `services/auth-service/handlers/admin.go` — `ImpersonateUser` (~line 144)
- Create: `services/auth-service/handlers/admin_impersonation.go`
- Modify: `services/auth-service/cmd/main.go:409` — register `/stop-impersonation`

- [ ] **Step 1: Write the failing test**

In `services/auth-service/handlers/admin_impersonation_test.go`:

```go
func TestImpersonateUser_SetsCookieAndStashesAdmin(t *testing.T) {
	// seed admin + target user in mock repo; admin context set via middleware
	// ... use existing admin handler test fixtures (see admin_test.go)
	h := newTestAdminHandler(t)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("user_id", "admin-1"); c.Set("username", "admin"); c.Next() })
	router.POST("/users/:id/impersonate", h.ImpersonateUser)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/users/user-2/impersonate", nil)
	router.ServeHTTP(w, req)
	if w.Code != 200 { t.Fatalf("status=%d body=%s", w.Code, w.Body.String()) }

	// impersonated access cookie set
	var got bool
	for _, c := range w.Result().Cookies() {
		if c.Name == "access_token" && c.Value != "" { got = true }
	}
	if !got { t.Error("impersonation access cookie not set") }

	// body must NOT contain the token
	if strings.Contains(w.Body.String(), "impersonation_token") {
		t.Error("token leaked in body")
	}
}

func TestStopImpersonation_RestoresAdminCookie(t *testing.T) {
	// seed an impersonated JWT (ImpersonatedBy=admin-1) + Redis stash
	// impersonation:<jti> → {admin_user_id, admin_username}
	// ... call POST /stop-impersonation with X-Access-Token=<impersonated-jwt>
	// assert: response has a fresh access_token cookie; Redis stash deleted
}
```

> NOTE: confirm the admin handler test fixtures (`newTestAdminHandler`, mock repo setup) in `admin_test.go` + the `KeyChain`/`jwtExpiry` fields on `AdminHandler`. Adapt the test to real fixtures. The assertions: impersonation sets the cookie (no token in body); stop-impersonation restores an admin cookie + deletes the stash.

- [ ] **Step 2: Run test to verify it fails**

```
cd services/auth-service && go test -run 'TestImpersonateUser_SetsCookieAndStashesAdmin|TestStopImpersonation_RestoresAdminCookie' -v
```
Expected: FAIL.

- [ ] **Step 3a: Modify `ImpersonateUser` to set cookie + stash**

In `services/auth-service/handlers/admin.go`, after generating the token (currently returns `gin.H{"impersonation_token": token}`), replace the response tail:

```go
	// Stash the admin identity in Redis keyed by the new JWT's jti, so
	// /stop-impersonation can restore the admin session. TTL = jwtExpiry.
	impClaims, _ := auth.ValidateJWTWithKeyChain(token, h.userKeyChain)
	if impClaims == nil || impClaims.ID == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read impersonation jti"})
		return
	}
	stashKey := "impersonation:" + impClaims.ID
	stash := map[string]string{
		"admin_user_id":  adminUserID.(string),
		"admin_username": adminUsername.(string),
	}
	stashJSON, _ := json.Marshal(stash)
	if err := h.redis.Set(c.Request.Context(), stashKey, string(stashJSON), h.jwtExpiry).Err(); err != nil {
		h.logger.Error("Failed to stash admin identity for impersonation", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Set the impersonated access cookie (replaces the admin's cookie).
	http.SetCookie(c.Writer, &http.Cookie{
		Name: auth.CookieAccessToken, Value: token, Path: "/",
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
		MaxAge: int(h.jwtExpiry.Seconds()),
	})

	c.JSON(http.StatusOK, gin.H{
		"user":          gin.H{"id": targetUser.ID, "username": targetUser.Username, "display_name": targetUser.DisplayName},
		"impersonating": true,
	})
```

> NOTE: `AdminHandler` needs a `redis` client + `jwtExpiry` field. Confirm both exist; if not, add them to the struct + constructor wiring in `cmd/main.go`. Check `services/auth-service/handlers/admin.go` struct def + `cmd/main.go` adminHandler construction.

- [ ] **Step 3b: Create `HandleStopImpersonation`**

`services/auth-service/handlers/admin_impersonation.go`:

```go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/caesar/all-chat/shared/auth"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// HandleStopImpersonation restores the admin session after an impersonation.
// POST /auth/stop-impersonation (protected; requires an impersonated JWT).
// Reads the current access token from X-Access-Token (forwarded by the
// gateway). If it carries an impersonated_by claim, looks up the stashed
// admin identity (impersonation:<jti>), issues a fresh admin access JWT, and
// sets it as the access cookie. (audit H3)
func (h *AuthHandler) HandleStopImpersonation(c *gin.Context) {
	token := c.GetHeader("X-Access-Token")
	if token == "" {
		authHeader := c.GetHeader("Authorization")
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			token = authHeader[7:]
		}
	}
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	claims, err := auth.ValidateJWTWithKeyChain(token, h.userKeyChain)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	if claims.ImpersonatedBy == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "not currently impersonating"})
		return
	}

	// Look up the stashed admin identity.
	stashKey := "impersonation:" + claims.ID
	data, err := h.redis.GetDel(c.Request.Context(), stashKey).Result()
	if err != nil {
		h.logger.Warn("Impersonation stash missing on stop", zap.String("jti", claims.ID), zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "impersonation session expired"})
		return
	}
	var stash struct {
		AdminUserID  string `json:"admin_user_id"`
		AdminUsername string `json:"admin_username"`
	}
	if err := json.Unmarshal([]byte(data), &stash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Fetch the admin user (for is_admin) + issue a fresh admin JWT.
	adminUser, err := h.userRepo.GetUserByID(c.Request.Context(), stash.AdminUserID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "admin user not found"})
		return
	}
	jwtToken, err := auth.GenerateTokenWithKid(h.userKeyChain.LatestKid(), adminUser.ID, adminUser.Username, string(h.userKeyChain.LatestSecret()), h.jwtExpiry, adminUser.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name: auth.CookieAccessToken, Value: jwtToken, Path: "/",
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
		MaxAge: int(h.jwtExpiry.Seconds()),
	})

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{"id": adminUser.ID, "username": adminUser.Username, "is_admin": adminUser.IsAdmin},
	})
}
```

> NOTE: `HandleStopImpersonation` is on `AuthHandler` (it needs `userRepo` + `userKeyChain` + `redis` + `jwtExpiry` + `logger` — all present on `AuthHandler`). If `AuthHandler` lacks `GetUserByID` on its repo interface, adapt to the actual method name. Confirm `ValidateJWTWithKeyChain` signature + that `claims.ID` is populated (Task 2 sets jti only on impersonation JWTs; admin restoration uses `GenerateTokenWithKid` which does NOT set jti — that's fine, the stash is only for impersonation).

- [ ] **Step 3c: Register the route**

In `services/auth-service/cmd/main.go`, in the protected group (next to `protected.POST("/logout", ...)`):

```go
		protected.POST("/stop-impersonation", legacyAuthHandler.HandleStopImpersonation)
```

- [ ] **Step 4: Run tests to verify they pass**

```
cd services/auth-service && go test ./handlers/... -v -run 'Impersonate|StopImpersonation'
cd services/auth-service && go build ./... && go vet ./...
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/auth-service/handlers/admin.go services/auth-service/handlers/admin_impersonation.go services/auth-service/handlers/admin_impersonation_test.go services/auth-service/cmd/main.go
git commit -m "✨ feat(auth): cookie-based impersonation + /stop-impersonation (H3)"
```

---

## Domain B — api-gateway

### Task 8: Create `CookieToBearer` middleware

**Files:**
- Create: `services/api-gateway/middleware/cookie_to_bearer.go`
- Create: `services/api-gateway/middleware/cookie_to_bearer_test.go`

- [ ] **Step 1: Write the failing test**

`services/api-gateway/middleware/cookie_to_bearer_test.go`:

```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caesar/all-chat/shared/auth"
	"github.com/gin-gonic/gin"
)

func TestCookieToBearer_SetsAuthorizationFromCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotAuth string
	r := gin.New()
	r.Use(CookieToBearer())
	r.GET("/x", func(c *gin.Context) { gotAuth = c.GetHeader("Authorization"); c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieAccessToken, Value: "jwt-from-cookie"})
	r.ServeHTTP(w, req)
	if gotAuth != "Bearer jwt-from-cookie" { t.Errorf("Authorization=%q", gotAuth) }
}

func TestCookieToBearer_DoesNotOverwriteExistingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotAuth string
	r := gin.New()
	r.Use(CookieToBearer())
	r.GET("/x", func(c *gin.Context) { gotAuth = c.GetHeader("Authorization"); c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer explicit")
	req.AddCookie(&http.Cookie{Name: auth.CookieAccessToken, Value: "jwt-from-cookie"})
	r.ServeHTTP(w, req)
	if gotAuth != "Bearer explicit" { t.Errorf("Authorization overwritten: %q", gotAuth) }
}

func TestCookieToBearer_NoCookieNoHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotAuth string
	r := gin.New()
	r.Use(CookieToBearer())
	r.GET("/x", func(c *gin.Context) { gotAuth = c.GetHeader("Authorization"); c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	r.ServeHTTP(w, req)
	if gotAuth != "" { t.Errorf("Authorization should be empty, got %q", gotAuth) }
}
```

- [ ] **Step 2: Run test to verify it fails**

```
cd services/api-gateway && go test -run TestCookieToBearer -v
```
Expected: FAIL — `CookieToBearer` undefined.

- [ ] **Step 3: Write minimal implementation**

`services/api-gateway/middleware/cookie_to_bearer.go`:

```go
package middleware

import (
	"github.com/caesar/all-chat/shared/auth"
	"github.com/gin-gonic/gin"
)

// CookieToBearer normalizes the access_token httpOnly cookie into the
// Authorization: Bearer header so the downstream shared.JWTAuth middleware
// (unchanged) validates it. If an Authorization header is already present
// (non-browser clients / old builds), it takes precedence (backward compat).
// This is the cookie-boundary normalization for H3.
func CookieToBearer() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("Authorization") == "" {
			if tok, err := c.Cookie(auth.CookieAccessToken); err == nil && tok != "" {
				c.Request.Header.Set("Authorization", "Bearer "+tok)
			}
		}
		c.Next()
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

```
cd services/api-gateway && go test -run TestCookieToBearer -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/api-gateway/middleware/cookie_to_bearer.go services/api-gateway/middleware/cookie_to_bearer_test.go
git commit -m "✨ feat(api-gateway): CookieToBearer normalization middleware (H3)"
```

---

### Task 9: Create `AuthCookieForward` middleware

**Files:**
- Create: `services/api-gateway/middleware/auth_cookie_forward.go`
- Create: `services/api-gateway/middleware/auth_cookie_forward_test.go`

- [ ] **Step 1: Write the failing test**

```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caesar/all-chat/shared/auth"
	"github.com/gin-gonic/gin"
)

func TestAuthCookieForward_SetsHeadersFromCookies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotAccess, gotRefresh string
	r := gin.New()
	r.Use(AuthCookieForward())
	r.POST("/x", func(c *gin.Context) {
		gotAccess = c.GetHeader("X-Access-Token")
		gotRefresh = c.GetHeader("X-Refresh-Token")
		c.Status(200)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/x", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieAccessToken, Value: "acc"})
	req.AddCookie(&http.Cookie{Name: auth.CookieRefreshToken, Value: "ref"})
	r.ServeHTTP(w, req)

	if gotAccess != "acc" { t.Errorf("X-Access-Token=%q", gotAccess) }
	if gotRefresh != "ref" { t.Errorf("X-Refresh-Token=%q", gotRefresh) }
}

func TestAuthCookieForward_NoCookiesNoHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotAccess, gotRefresh string
	r := gin.New()
	r.Use(AuthCookieForward())
	r.POST("/x", func(c *gin.Context) {
		gotAccess = c.GetHeader("X-Access-Token")
		gotRefresh = c.GetHeader("X-Refresh-Token")
		c.Status(200)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/x", nil)
	r.ServeHTTP(w, req)
	if gotAccess != "" || gotRefresh != "" { t.Errorf("headers should be empty") }
}
```

- [ ] **Step 2: Run test to verify it fails**

```
cd services/api-gateway && go test -run TestAuthCookieForward -v
```
Expected: FAIL — `AuthCookieForward` undefined.

- [ ] **Step 3: Write minimal implementation**

`services/api-gateway/middleware/auth_cookie_forward.go`:

```go
package middleware

import (
	"github.com/caesar/all-chat/shared/auth"
	"github.com/gin-gonic/gin"
)

// AuthCookieForward copies the access/refresh httpOnly cookies into custom
// request headers (X-Access-Token / X-Refresh-Token) before the gateway proxy
// forwards to auth-service. The proxy's L17 hop-header strip removes Cookie
// (and Referer/Origin) from forwarded requests, so auth-service cannot read
// the raw cookie. These custom headers are NOT in the L17 strip list, so they
// pass through. Used on /auth/refresh, /auth/logout, /auth/stop-impersonation.
// (audit H3)
func AuthCookieForward() gin.HandlerFunc {
	return func(c *gin.Context) {
		if tok, err := c.Cookie(auth.CookieAccessToken); err == nil && tok != "" {
			c.Request.Header.Set("X-Access-Token", tok)
		}
		if tok, err := c.Cookie(auth.CookieRefreshToken); err == nil && tok != "" {
			c.Request.Header.Set("X-Refresh-Token", tok)
		}
		c.Next()
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

```
cd services/api-gateway && go test -run TestAuthCookieForward -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/api-gateway/middleware/auth_cookie_forward.go services/api-gateway/middleware/auth_cookie_forward_test.go
git commit -m "✨ feat(api-gateway): AuthCookieForward middleware for auth routes (H3)"
```

---

### Task 10: Export HTTP CORS origins from `cors.go`

**Files:**
- Modify: `services/api-gateway/middleware/cors.go`

- [ ] **Step 1: Read current `cors.go`**

```
cd services/api-gateway && sed -n '1,60p' middleware/cors.go
```
Confirm the `parseOrigins` function + the `CORS_ORIGIN` env read (~line 29). Identify where the parsed origins live (local var in `CORS()`).

- [ ] **Step 2: Refactor to expose origins**

Refactor `parseOrigins` (or add `LoadHTTPAllowedOrigins()`) so the origins list is computed once and reusable by `OriginCheck`. Minimal change: extract the env read into a package-level function:

```go
// LoadHTTPAllowedOrigins reads CORS_ORIGIN (comma-separated) and returns the
// allowlist. Shared by the CORS middleware and the CSRF OriginCheck.
func LoadHTTPAllowedOrigins() []string {
	raw := os.Getenv("CORS_ORIGIN")
	var out []string
	for _, o := range strings.Split(raw, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			out = append(out, o)
		}
	}
	return out
}
```

Keep `CORS()` calling it (replace the inline parse). No behavior change.

- [ ] **Step 3: Verify build + existing CORS tests**

```
cd services/api-gateway && go build ./... && go test ./middleware/...
```
Expected: PASS (no behavior change).

- [ ] **Step 4: Commit**

```bash
git add services/api-gateway/middleware/cors.go
git commit -m "♻️ refactor(api-gateway): expose LoadHTTPAllowedOrigins (H3 CSRF)"
```

---

### Task 11: Wire gateway middleware + route + JWTAuthWithRevocation

**Files:**
- Modify: `services/api-gateway/cmd/main.go`

- [ ] **Step 1: Read current wiring**

```
cd services/api-gateway && sed -n '420,475p' cmd/main.go && sed -n '540,560p' cmd/main.go
```
Confirm `protectedAPI`, `adminAPI`, `metricsGroup` use `sharedmiddleware.JWTAuth(userKeyChain)`. Confirm the public auth routes (`publicAPI.POST("/auth/refresh", ...)`).

- [ ] **Step 2: Apply wiring changes**

In `cmd/main.go`:

(a) Switch protected/admin/metrics to `JWTAuthWithRevocation` + prepend `CookieToBearer`:

```go
	protectedAPI := router.Group("/api/v1")
	protectedAPI.Use(middleware.CookieToBearer(), sharedmiddleware.JWTAuthWithRevocation(userKeyChain, redisClient))
```
Apply the same to `adminAPI` (find its `protectedAPI.Use(...)`/`admin.Use(...)`) and `metricsGroup`.

(b) Wire `AuthCookieForward` + `OriginCheck` on the auth routes that need cookies. For `/auth/refresh` (public), `/auth/logout` (protected), `/auth/stop-impersonation` (protected):

```go
		publicAPI.POST("/auth/refresh", authRateLimiter.Middleware(), middleware.AuthCookieForward(), sharedmiddleware.OriginCheck(middleware.LoadHTTPAllowedOrigins()), proxyHandler.ForwardRequest)
```
For `/auth/logout` (already in protectedAPI group) + `/auth/stop-impersonation` (new), wire `AuthCookieForward` on those specific routes:

```go
		protectedAPI.POST("/auth/logout", middleware.AuthCookieForward(), proxyHandler.ForwardRequest)
		protectedAPI.POST("/auth/stop-impersonation", middleware.AuthCookieForward(), proxyHandler.ForwardRequest)
```

> NOTE: `OriginCheck` applies to mutating methods globally if wired at group level; wiring it per-route on the auth POST routes is sufficient for CSRF coverage of the cookie-auth paths. If you prefer group-level, wire on `protectedAPI` — but then verify it doesn't block legitimate non-browser POSTs (it allows absent Origin, so it won't). Pick the group-level approach for broader coverage:

```go
	protectedAPI.Use(middleware.CookieToBearer(), sharedmiddleware.JWTAuthWithRevocation(userKeyChain, redisClient), sharedmiddleware.OriginCheck(middleware.LoadHTTPAllowedOrigins()))
```
And on `publicAPI` for the refresh route only.

(c) Confirm `redisClient` is in scope at the wiring site (it is — used elsewhere).

- [ ] **Step 3: Build + test**

```
cd services/api-gateway && go build ./... && go vet ./... && go test ./...
```
Expected: PASS. If `JWTAuthWithRevocation` signature mismatch, check `shared/middleware/auth.go` for the exact name (it was added in the prior commit).

- [ ] **Step 4: Commit**

```bash
git add services/api-gateway/cmd/main.go
git commit -m "✨ feat(api-gateway): wire CookieToBearer + AuthCookieForward + OriginCheck + JWTAuthWithRevocation (H3)"
```

---

## Domain C — frontend

### Task 12: Remove token attachment + add 401→refresh→retry interceptor in `client.ts`

**Files:**
- Modify: `frontend/src/lib/api/client.ts`

- [ ] **Step 1: Read current `client.ts`**

```
sed -n '60,115p' frontend/src/lib/api/client.ts
```
Confirm the `fetch` method attaches `Authorization: Bearer` from `localStorage.getItem('jwt_token')` and the 401 handler clears localStorage + redirects.

- [ ] **Step 2: Rewrite `fetch`**

Replace the token-read + header-attach block:

```typescript
  private refreshing = false
  private refreshPromise: Promise<boolean> | null = null

  private async fetch(endpoint: string, options: RequestInit = {}): Promise<Response> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(options.headers as Record<string, string>),
    }

    const url = endpoint.startsWith('http') ? endpoint : `${API_URL}${endpoint}`
    const response = await fetch(url, { ...options, headers, credentials: 'same-origin' })

    if (response.status === 401 && !endpoint.startsWith('/auth/refresh') && !endpoint.startsWith('/auth/login')) {
      // Try one cookie-based refresh, then retry the original request once.
      const ok = await this.tryRefresh()
      if (ok) {
        return fetch(url, { ...options, headers, credentials: 'same-origin' })
      }
      // refresh failed — bounce to login
      if (typeof window !== 'undefined') {
        window.location.href = '/'
      }
      const errorData = await response.json().catch(() => ({ error: 'Unauthorized' }))
      throw new ApiError(401, (errorData as { error?: string }).error || 'Unauthorized', errorData as Record<string, unknown>)
    }

    if (!response.ok) {
      const errorData: Record<string, unknown> = await response.json().catch(() => ({ error: 'Unknown error' }))
      const errorValue = typeof errorData.error === 'string' ? errorData.error : undefined
      throw new ApiError(response.status, errorValue || response.statusText, errorData)
    }

    return response
  }

  private async tryRefresh(): Promise<boolean> {
    if (this.refreshPromise) return this.refreshPromise
    this.refreshPromise = (async () => {
      try {
        const r = await fetch(`${API_URL}/auth/refresh`, {
          method: 'POST',
          credentials: 'same-origin',
          headers: { 'Content-Type': 'application/json' },
        })
        return r.ok
      } catch {
        return false
      } finally {
        this.refreshPromise = null
      }
    })()
    return this.refreshPromise
  }
```

Remove the old `localStorage.getItem('jwt_token')` + `headers['Authorization']` + the inline `localStorage.removeItem`/redirect 401 block.

- [ ] **Step 3: Typecheck**

```
cd frontend && npx tsc --noEmit
```
Expected: PASS (no type errors).

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/api/client.ts
git commit -m "✨ feat(frontend): cookie-based apiClient + 401→refresh→retry (H3)"
```

---

### Task 13: Remove token reads from `overlays.ts` (+ verify `chat.ts`/`moderation.ts`)

**Files:**
- Modify: `frontend/src/lib/api/overlays.ts`
- Verify: `frontend/src/lib/api/chat.ts`, `moderation.ts`

- [ ] **Step 1: Find token reads**

```
cd frontend && rg -n "inMemoryTokens|localStorage.getItem\('jwt_token'\)|Authorization.*Bearer" src/lib/api/overlays.ts src/lib/api/chat.ts src/lib/api/moderation.ts
```

- [ ] **Step 2: Strip token attachment in `overlays.ts`**

In `overlays.ts` (the `testTTSKey` method ~line 309 + any other site), remove:
- `const token = inMemoryTokens.getAccessToken() ?? localStorage.getItem('jwt_token')`
- `if (token) headers['Authorization'] = ...`

Keep `credentials: 'same-origin'` on the fetch (or rely on the default). If `overlays.ts` has its own `fetch` (not via `apiClient`), ensure it uses `credentials: 'same-origin'` and drops the `Authorization` header.

- [ ] **Step 3: Verify `chat.ts` + `moderation.ts`**

Confirm both delegate to `apiClient` (no direct token reads). If they do, no change. If any has a direct `Authorization` attach, strip it the same way.

- [ ] **Step 4: Typecheck + build**

```
cd frontend && npx tsc --noEmit && npm run build
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/api/overlays.ts frontend/src/lib/api/chat.ts frontend/src/lib/api/moderation.ts
git commit -m "✨ feat(frontend): drop Authorization attachment from api wrappers (H3)"
```

---

### Task 14: Rewrite `auth-store.ts` for cookie auth + new impersonation

**Files:**
- Modify: `frontend/src/lib/stores/auth-store.ts`
- Modify: `frontend/src/lib/auth/in-memory-store.ts`
- Modify: `frontend/src/lib/api/auth.ts` (if `stopImpersonation` wrapper needed)

- [ ] **Step 1: Read current store**

```
sed -n '1,170p' frontend/src/lib/stores/auth-store.ts
```

- [ ] **Step 2: Rewrite the store**

Remove `token` state, `setToken`, all `localStorage` `jwt_token`/`admin_token`/`impersonating`/`impersonated_user` reads/writes, and the `inMemoryTokens` streamer/admin calls. New store shape:

```typescript
interface AuthStore {
  user: User | null
  loading: boolean
  isImpersonating: boolean
  impersonatedUsername: string | null
  setUser: (user: User) => void
  logout: () => Promise<void>
  init: () => Promise<void>
  startImpersonation: (targetUserId: string) => Promise<void>
  stopImpersonation: () => Promise<void>
}

export const useAuthStore = create<AuthStore>((set) => ({
  user: null,
  loading: true,
  isImpersonating: false,
  impersonatedUsername: null,

  setUser: (user) => set({ user, loading: false }),

  logout: async () => {
    try {
      await authApi.logout() // POST /auth/logout — cookie cleared server-side
    } catch {
      // ignore — clearing is best-effort
    }
    set({ user: null, loading: false, isImpersonating: false, impersonatedUsername: null })
    if (typeof window !== 'undefined') {
      window.location.href = '/'
    }
  },

  init: async () => {
    if (typeof window === 'undefined') {
      set({ loading: false })
      return
    }
    try {
      const user = await authApi.getMe() // GET /auth/me — succeeds if access cookie valid
      set({ user, loading: false })
    } catch {
      set({ user: null, loading: false })
    }
  },

  startImpersonation: async (targetUserId: string) => {
    const res = await authApi.impersonate(targetUserId) // POST /admin/users/:id/impersonate — sets cookie
    set({ user: res.user, isImpersonating: true, impersonatedUsername: res.user.username })
  },

  stopImpersonation: async () => {
    const res = await authApi.stopImpersonation() // POST /auth/stop-impersonation — restores admin cookie
    set({ user: res.user, isImpersonating: false, impersonatedUsername: null })
  },
}))
```

- [ ] **Step 3: Add `impersonate` + `stopImpersonation` to `auth.ts`**

In `frontend/src/lib/api/auth.ts` (the `authApi` object), add:

```typescript
  impersonate: (targetUserId: string) =>
    apiClient.post<{ user: { id: string; username: string; display_name?: string } }>(`/admin/users/${targetUserId}/impersonate`),
  stopImpersonation: () =>
    apiClient.post<{ user: { id: string; username: string; is_admin: boolean } }>('/auth/stop-impersonation'),
```

> NOTE: confirm `apiClient.post` exists + its signature. Adapt to the real client.

- [ ] **Step 4: Scope `in-memory-store.ts` to viewer only**

Remove the streamer/admin fields (`accessToken`, `refreshToken`, `adminToken`, `impersonating`, `impersonatedUsername`) + their getter/setters. Keep `viewerAccessToken` + its getter/setter. Remove now-unused imports.

- [ ] **Step 5: Fix callers of removed store methods**

```
cd frontend && rg -n "setToken|\.token\b|startImpersonation\(|stopImpersonation\(" src/app src/components src/lib
```
For each caller:
- `setToken(token)` callers (e.g. `auth/callback/page.tsx` after exchange) → replace with `setUser(user)` (the exchange response now returns the user, not a token).
- `startImpersonation(token, username)` callers → `startImpersonation(targetUserId)`.
- `stopImpersonation()` callers → unchanged signature (now async; add `await`).
- `useAuthStore(s => s.token)` → remove; components should not read the token.

- [ ] **Step 6: Typecheck + build**

```
cd frontend && npx tsc --noEmit && npm run build
```
Expected: PASS. Fix remaining type errors from the API surface change.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/lib/stores/auth-store.ts frontend/src/lib/auth/in-memory-store.ts frontend/src/lib/api/auth.ts frontend/src/app frontend/src/components
git commit -m "✨ feat(frontend): cookie-auth auth-store + server-side impersonation (H3)"
```

---

### Task 15: Update `auth/callback/page.tsx` to read user from exchange body

**Files:**
- Modify: `frontend/src/app/auth/callback/page.tsx`

- [ ] **Step 1: Read current callback**

```
sed -n '1,130p' frontend/src/app/auth/callback/page.tsx
```
Confirm it currently calls `POST /exchange` and reads tokens from the response.

- [ ] **Step 2: Rewrite the exchange handler**

After the `POST /auth/exchange` call (which now sets cookies server-side + returns `{expires_in, token_type, redirect_to, user, ...}` — no tokens), update the handler to read the user + redirect_to from the body and call `setUser(user)` (not `setToken`):

```typescript
      const res = await apiClient.post<{ expires_in: number; redirect_to?: string; user: { id: string; username: string; display_name?: string; is_admin?: boolean } }>('/auth/exchange', { code })
      useAuthStore.getState().setUser({
        id: res.user.id,
        username: res.user.username,
        displayName: res.user.display_name,
        isAdmin: res.user.is_admin ?? false,
      })
      const redirect = safeRedirect(res.redirect_to) // existing M12 helper
      router.push(redirect)
```

Remove any `localStorage.setItem('jwt_token'...)` / `inMemoryTokens.setRefreshToken(...)` calls (the cookie is set by the server; the frontend never touches tokens).

- [ ] **Step 3: Typecheck + build**

```
cd frontend && npx tsc --noEmit && npm run build
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/app/auth/callback/page.tsx
git commit -m "✨ feat(frontend): callback reads user from /exchange body, no token handling (H3)"
```

---

### Task 16: Update `ttsPlayer.ts` — remove `Authorization` if present

**Files:**
- Verify/Modify: `frontend/src/lib/utils/ttsPlayer.ts`

- [ ] **Step 1: Check current state**

```
sed -n '395,425p' frontend/src/lib/utils/ttsPlayer.ts
```
The prior M17 commit moved `tts_token` to the `Authorization: Bearer` header. Confirm whether the tts flow still relies on a per-overlay JWT (not the streamer session cookie). If tts_token is a SEPARATE per-overlay token (not the streamer session JWT), leave it as-is (out of H3 scope). If it reads the streamer `jwt_token` from localStorage, remove that.

> NOTE: per the audit, `tts_token` is a per-overlay signing-secret JWT, NOT the streamer session JWT. So this is likely a **no-op** — confirm + leave a comment that tts_token is out of H3 scope (per-overlay, not session).

- [ ] **Step 2: Typecheck + build**

```
cd frontend && npx tsc --noEmit && npm run build
```
Expected: PASS.

- [ ] **Step 3: Commit (only if changed)**

```bash
git add frontend/src/lib/utils/ttsPlayer.ts
git commit -m "📝 docs(frontend): note tts_token is per-overlay, out of H3 scope"
```

---

## Integration + Verification

### Task 17: Full build + test sweep

- [ ] **Step 1: Build all Go modules**

```
for mod in $(find . -name go.mod -not -path '*/node_modules/*' | sort); do
  dir=$(dirname "$mod"); case "$dir" in ./services/auth-service/shared/tracing|./test/youtube-stream-test) continue;; esac
  out=$(cd "$dir" && go build ./... 2>&1); [ -n "$out" ] && { echo "FAIL $dir:"; echo "$out"|head; }
done
echo "build sweep done"
```
Expected: 0 fails.

- [ ] **Step 2: Test all Go modules**

```
for d in shared/auth shared/middleware services/auth-service services/api-gateway; do
  echo "=== $d ==="; (cd "$d" && go test ./... 2>&1 | grep -E '^(FAIL|--- FAIL|panic:|ok)' | head)
done
```
Expected: 0 FAIL/PANIC (except pre-existing `TestAuthHandlerLogout`-style Redis-dependent tests if Redis isn't running — those run with miniredis now).

- [ ] **Step 3: Frontend build**

```
cd frontend && npx tsc --noEmit && npm run build
```
Expected: PASS.

- [ ] **Step 4: gofmt check**

```
gofmt -l $(git diff --name-only -- '*.go')
```
Expected: empty.

- [ ] **Step 5: Commit any formatting fixes**

```bash
git add -u
git commit -m "🔧 chore: gofmt after H3 implementation" || echo "nothing to format"
```

---

### Task 18: Update `RESIDUALS.md`

**Files:**
- Modify: `RESIDUALS.md`

- [ ] **Step 1: Move H3 from deferred → done**

Update the H3 row: "DONE — httpOnly cookie migration implemented (access + refresh cookies; gateway cookie boundary; CSRF Origin-check; server-side impersonation). Viewer token still deferred (third-party-cookie headwinds)."

- [ ] **Step 2: Commit**

```bash
git add RESIDUALS.md
git commit -m "📝 docs: mark H3 cookie-auth complete in RESIDUALS"
```

---

## Backward-compat removal (deferred — separate commit after rollout stable)

Not in this plan. After the new frontend is live + stable, remove:
- `Authorization: Bearer` fallback in gateway `CookieToBearer` (make cookie-only).
- JSON-body refresh fallback in `HandleRefresh`.
- `Authorization` fallback in `HandleLogout` / `HandleStopImpersonation`.

Document this as a follow-up in `RESIDUALS.md`.

---

## Self-Review Notes

- **Spec coverage:** Cookie table → Task 1. Issuance (`/exchange`,`/refresh`) → Tasks 3,5. `/logout` → Task 6. Impersonation → Task 7. Gateway `CookieToBearer` → Task 8. `AuthCookieForward` → Task 9. CSRF `OriginCheck` → Tasks 4,11. Frontend wrappers → Tasks 12,13,16. `auth-store` → Task 14. Callback → Task 15. Viewer out-of-scope → Task 16 note + Task 14 (keep viewerAccessToken). All spec sections covered.
- **Type consistency:** `CookieAccessToken`/`CookieRefreshToken` (Task 1) used consistently in Tasks 8, 9. `GenerateImpersonationJWTWithKidExpiry` (Task 2) used in Task 7. `SetAuthCookies`/`ClearAuthCookies` (Task 1) used in Tasks 3, 5, 6.
- **Placeholders:** test helper names (`newTestAuthHandler`, `newTestAdminHandler`, `oauthRefreshMock`, `mockUserRepo`) are flagged with "adapt to existing mock patterns" — implementer must confirm against the test files. This is the one area requiring runtime judgment; flagged inline.
