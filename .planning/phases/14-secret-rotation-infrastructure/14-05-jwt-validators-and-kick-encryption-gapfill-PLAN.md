---
phase: 14-secret-rotation-infrastructure
plan: 05
type: execute
wave: 2
depends_on:
  - "14-02"
  - "14-03"
  - "14-04"
files_modified:
  - services/api-gateway/middleware/auth.go
  - services/api-gateway/middleware/viewer_auth.go
  - services/api-gateway/handlers/websocket.go
  - services/api-gateway/handlers/websocket_viewer.go
  - services/api-gateway/cmd/main.go
  - services/share-service/cmd/main.go
  - services/share-service/handlers/shares.go
  - services/overlay-manager/cmd/main.go
  - services/source-manager/cmd/main.go
  - services/auth-service/cmd/main.go
  - services/auth-service/handlers/auth_handler.go
  - services/auth-service/handlers/viewer_auth.go
  - shared/middleware/auth.go
  - shared/middleware/service_auth.go
  - services/kick-listener/cmd/main.go
  - services/kick-listener/channels/manager.go
  - services/overlay-manager/handlers/sources.go
autonomous: true
decisions_addressed:
  - D-07
  - D-08
  - D-09
  - D-10
  - D-16
  - D-17
must_haves:
  truths:
    - "All JWT issuance call sites set kid header to KeyChain.LatestKid() before signing"
    - "All JWT validation call sites use KeyChain.KeyFunc — middlewares accept *KeyChain instead of string secrets"
    - "share-service generates Service JWTs with the SERVICE_JWT_SECRET chain (was JWT_SECRET — bugfix)"
    - "api-gateway internal route service JWT validation uses SERVICE_JWT_SECRET chain (was JWT_SECRET — bugfix)"
    - "kick-listener encrypts access_token/refresh_token on write (encryption_version=1) and decrypts on read"
    - "overlay-manager Kick source handlers encrypt access_token/refresh_token on write and decrypt on read"
    - "User and Service JWT chains never validate each other's tokens (D-10 isolation enforced at every callsite)"
  artifacts:
    - path: "shared/middleware/auth.go"
      provides: "JWTAuth(kc *auth.KeyChain) middleware factory replacing JWTAuth(secret string)"
    - path: "shared/middleware/service_auth.go"
      provides: "ServiceJWTAuth(kc *auth.KeyChain, allowedServices ...string) middleware factory"
    - path: "services/share-service/handlers/shares.go"
      provides: "Service JWT generation uses serviceKeyChain.LatestSecret()/LatestKid() — fixes Pitfall 4"
    - path: "services/api-gateway/cmd/main.go"
      provides: "Internal route group uses serviceKeyChain (not jwtSecret) — fixes parallel bug"
    - path: "services/kick-listener/channels/manager.go"
      provides: "Read path selects encryption_version, decrypts via *MultiKeyEncryptor when >=1"
    - path: "services/overlay-manager/handlers/sources.go"
      provides: "Read path decrypts; write path encrypts and sets encryption_version=1"
  key_links:
    - from: "services/api-gateway/cmd/main.go"
      to: "services/api-gateway/middleware/auth.go (JWTAuth) + shared/middleware/service_auth.go (ServiceJWTAuth)"
      via: "userKeyChain wired into protected/admin route groups; serviceKeyChain wired into /internal route group"
      pattern: "JWTAuth\\(userKeyChain\\)|ServiceJWTAuth\\(serviceKeyChain"
    - from: "services/share-service/handlers/shares.go"
      to: "shared/auth.GenerateServiceJWTWithKid"
      via: "share-service generates service JWTs with SERVICE_JWT_SECRET chain (was JWT_SECRET)"
      pattern: "GenerateServiceJWTWithKid.*serviceKeyChain"
    - from: "services/kick-listener/channels/manager.go"
      to: "shared/encryption.MultiKeyEncryptor.DecryptString"
      via: "decrypt-on-read gated by encryption_version >= 1 (matching youtube-listener pattern)"
      pattern: "encryption_version|encryptor.DecryptString"
---

<objective>
Two intertwined workstreams in one plan:

1. **JWT validator migration (D-07/D-08/D-09/D-10):** every JWT validation call site (api-gateway, share-service, overlay-manager, source-manager, shared/middleware) accepts a `*auth.KeyChain` instead of a raw secret string. Issuers (auth-service, share-service for Service JWTs) sign with `LatestKid()`/`LatestSecret()`. Critically, this plan fixes TWO existing bugs:
   - **share-service Pitfall 4 (RESEARCH.md §4):** `GenerateServiceJWT(..., h.jwtSecret, ...)` is changed to use the SERVICE_JWT_SECRET chain.
   - **api-gateway parallel bug (newly discovered, see RESEARCH §4 cross-validation):** `internal.Use(sharedmiddleware.ServiceJWTAuth(jwtSecret, ...))` at `services/api-gateway/cmd/main.go:563` is changed to use `serviceKeyChain` (currently it incorrectly validates service JWTs against `JWT_SECRET`).

2. **Encryption gap-fill code-side (D-16/D-17):** kick-listener `channels/manager.go` and overlay-manager `handlers/sources.go` start encrypting on write (setting `encryption_version=1`) and decrypting on read (gated by the column added in Plan 14-03's migration 050). Mirrors the youtube-listener `oauth/store.go` pattern.

Output:
- Middlewares accept `*auth.KeyChain`.
- All issuers use `*WithKid` variants.
- Two cross-chain bugs fixed (share-service service-JWT issuance + api-gateway service-JWT validation).
- kick_oauth_tokens read/write paths encrypt/decrypt round-trip (round-trip test added).
- TikTok code-side: explicitly NOT migrated here (Node.js scope, see Plan 14-03 deferral note).
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.planning/phases/14-secret-rotation-infrastructure/14-CONTEXT.md
@.planning/phases/14-secret-rotation-infrastructure/14-RESEARCH.md
@.planning/phases/14-secret-rotation-infrastructure/14-PATTERNS.md
@.planning/phases/14-secret-rotation-infrastructure/14-02-SUMMARY.md
@.planning/phases/14-secret-rotation-infrastructure/14-03-SUMMARY.md
@.planning/phases/14-secret-rotation-infrastructure/14-04-SUMMARY.md
@shared/auth/jwt.go
@shared/middleware/auth.go
@shared/middleware/service_auth.go
@services/api-gateway/middleware/auth.go
@services/api-gateway/middleware/viewer_auth.go
@services/api-gateway/handlers/websocket.go
@services/api-gateway/handlers/websocket_viewer.go
@services/api-gateway/cmd/main.go
@services/share-service/cmd/main.go
@services/share-service/handlers/shares.go
@services/overlay-manager/cmd/main.go
@services/source-manager/cmd/main.go
@services/auth-service/cmd/main.go
@services/auth-service/handlers/auth_handler.go
@services/auth-service/handlers/viewer_auth.go
@services/kick-listener/cmd/main.go
@services/kick-listener/channels/manager.go
@services/overlay-manager/handlers/sources.go
@services/youtube-listener/oauth/store.go

<interfaces>
<!-- The KeyChain API from Plan 14-02 — wire these throughout. -->

From shared/auth/jwt.go (Plan 14-02):
```go
type KeyChain struct { /* unexported fields */ }
func NewKeyChainFromEnv(prefix string) (*KeyChain, error)
func NewKeyChain(byKid map[string][]byte, legacy []byte, latestKid string) *KeyChain
func (kc *KeyChain) KeyFunc(token *jwt.Token) (interface{}, error)
func (kc *KeyChain) LatestKid() string
func (kc *KeyChain) LatestSecret() []byte
func GenerateJWTWithKid(kid, userID, twitchID, username, secret string, isAdmin bool) (string, error)
func GenerateTokenWithKid(kid, userID, username, secret string, expiry time.Duration, isAdmin bool) (string, error)
func GenerateImpersonationJWTWithKid(kid, adminUserID, adminUsername, targetUserID, targetUsername, targetTwitchID, secret string) (string, error)
func GenerateServiceJWTWithKid(kid, serviceName, secret string, expiry time.Duration) (string, error)
func GenerateViewerJWTWithKid(kid string, claims ViewerClaims, secret string) (string, error)
func ValidateJWTWithKeyChain(tokenString string, kc *KeyChain) (*Claims, error)
func ValidateViewerJWTWithKeyChain(tokenString string, kc *KeyChain) (*ViewerClaims, error)
func ValidateServiceJWTWithKeyChain(tokenString string, kc *KeyChain) (*ServiceClaims, error)
```

Existing middleware to change (current signatures):
```go
// services/api-gateway/middleware/auth.go
func JWTAuth(jwtSecret string) gin.HandlerFunc
// services/api-gateway/middleware/viewer_auth.go
func ViewerJWTAuth(jwtSecret string) gin.HandlerFunc
// shared/middleware/auth.go
func JWTAuth(secret string) gin.HandlerFunc       // (also has ViewerJWTAuth path inside)
// shared/middleware/service_auth.go
func ServiceJWTAuth(secret string, allowedServices ...string) gin.HandlerFunc
```

The current bug in services/api-gateway/cmd/main.go (line 563):
```go
internal.Use(sharedmiddleware.ServiceJWTAuth(jwtSecret, "share-service", "overlay-manager", "auth-service"))
//                                            ^^^^^^^^^ BUG: should be serviceKeyChain (SERVICE_JWT_SECRET chain)
```

The current bug in services/share-service/handlers/shares.go (lines 395, 651):
```go
serviceToken, err := auth.GenerateServiceJWT("share-service", h.jwtSecret, 30*time.Second)
//                                                            ^^^^^^^^^^^^ BUG: should be serviceKeyChain.LatestSecret()
```
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Migrate shared middleware (auth + service_auth) and api-gateway middleware to KeyChain</name>
  <files>shared/middleware/auth.go, shared/middleware/service_auth.go, services/api-gateway/middleware/auth.go, services/api-gateway/middleware/viewer_auth.go</files>
  <read_first>
    - shared/middleware/auth.go (full)
    - shared/middleware/service_auth.go (full)
    - services/api-gateway/middleware/auth.go (full — already shown above)
    - services/api-gateway/middleware/viewer_auth.go (full)
    - shared/auth/jwt.go (post-Plan-14-02 with KeyChain present)
    - .planning/phases/14-secret-rotation-infrastructure/14-RESEARCH.md §4 + Pitfall 4 (lines 919–923)
    - .planning/phases/14-secret-rotation-infrastructure/14-PATTERNS.md "services/api-gateway/middleware/auth.go" + "shared/middleware/service_auth.go" sections (lines 345–395)
  </read_first>
  <behavior>
    - Existing middleware tests in `shared/middleware/auth_test.go` and `shared/middleware/service_auth_test.go` are updated to construct test KeyChains via `auth.NewKeyChain(map[string][]byte{"v1": []byte("test-secret")}, []byte("legacy-secret"), "v1")` and assert that:
      - Valid kid'd tokens are accepted.
      - Legacy (kid-less) tokens are accepted via fallback.
      - Tokens signed with a different secret are rejected (401).
      - Service JWT middleware rejects tokens whose claimed service is not in `allowedServices`.
    - `auth_test.go::TestJWTAuth_KidValidation` covers the new path.
    - `service_auth_test.go::TestServiceJWTAuth_ChainIsolation` (NEW) signs a token with the user-chain secret and asserts ServiceJWTAuth rejects it (D-10 cross-chain isolation at the middleware boundary).
  </behavior>
  <action>
    Step 1 — `shared/middleware/auth.go`:

    a) Change function signature from `JWTAuth(secret string)` to `JWTAuth(kc *auth.KeyChain)`. Replace the call inside the closure from `auth.ValidateJWT(tokenString, secret)` to `auth.ValidateJWTWithKeyChain(tokenString, kc)`.

    b) Same for any `ViewerJWTAuth` in this file: change to take `*auth.KeyChain`, call `auth.ValidateViewerJWTWithKeyChain`.

    Step 2 — `shared/middleware/service_auth.go`:

    a) Change `ServiceJWTAuth(secret string, allowedServices ...string)` to `ServiceJWTAuth(kc *auth.KeyChain, allowedServices ...string)`.

    b) Replace `auth.ValidateServiceJWT(tokenString, secret)` with `auth.ValidateServiceJWTWithKeyChain(tokenString, kc)`.

    Step 3 — `services/api-gateway/middleware/auth.go`:

    a) Same change: `JWTAuth(jwtSecret string)` → `JWTAuth(kc *auth.KeyChain)`. Replace `auth.ValidateJWT(token, jwtSecret)` with `auth.ValidateJWTWithKeyChain(token, kc)`.

    Step 4 — `services/api-gateway/middleware/viewer_auth.go`:

    a) Same change: `ViewerJWTAuth(jwtSecret string)` → `ViewerJWTAuth(kc *auth.KeyChain)`.

    Step 5 — Update all middleware tests:
    - `shared/middleware/auth_test.go`
    - `shared/middleware/service_auth_test.go`
    - `services/api-gateway/middleware/*_test.go` (if any exist)

    Construct KeyChains in tests:
    ```go
    kc := auth.NewKeyChain(
        map[string][]byte{"v1": []byte("test-secret-v1")},
        []byte("test-secret-legacy"),
        "v1",
    )
    middleware.JWTAuth(kc)
    ```

    Add the new `TestServiceJWTAuth_ChainIsolation` test (in service_auth_test.go):
    ```go
    func TestServiceJWTAuth_ChainIsolation(t *testing.T) {
        // Token signed with the USER chain secret
        userToken, _ := auth.GenerateServiceJWTWithKid("v1", "share-service", "user-chain-secret", 30*time.Second)
        // Service chain has DIFFERENT secret
        serviceKC := auth.NewKeyChain(map[string][]byte{"v1": []byte("service-chain-secret")}, nil, "v1")
        // Build a request that ServiceJWTAuth handles
        req := httptest.NewRequest("GET", "/internal/foo", nil)
        req.Header.Set("Authorization", "Bearer "+userToken)
        w := httptest.NewRecorder()
        ctx, _ := gin.CreateTestContext(w)
        ctx.Request = req
        h := middleware.ServiceJWTAuth(serviceKC, "share-service")
        h(ctx)
        require.Equal(t, http.StatusUnauthorized, w.Code, "user-chain token must NOT validate against service chain (D-10)")
    }
    ```

    Step 6 — Compile (do NOT yet update callers in cmd/main.go — Task 2 handles that):
    ```
    cd /home/moersener/Hobby/all-chat && go build ./shared/middleware/... ./services/api-gateway/middleware/...
    ```
    This WILL break callers in cmd/main.go files; that breakage is Task 2's surface. To allow incremental verification, accept that compile errors in `services/api-gateway/cmd/main.go`, `services/share-service/cmd/main.go`, `services/source-manager/cmd/main.go`, `services/overlay-manager/cmd/main.go` are EXPECTED at this checkpoint and are fixed in Task 2.

    Step 7 — Run middleware unit tests in isolation:
    ```
    cd /home/moersener/Hobby/all-chat && go test ./shared/middleware/... -count=1
    ```
    This must pass after Step 5 updates the tests.
  </action>
  <verify>
    <automated>cd /home/moersener/Hobby/all-chat && go test ./shared/middleware/... -count=1</automated>
  </verify>
  <acceptance_criteria>
    - `grep -q "func JWTAuth(kc \*auth.KeyChain)" shared/middleware/auth.go`
    - `grep -q "func ServiceJWTAuth(kc \*auth.KeyChain" shared/middleware/service_auth.go`
    - `grep -q "func JWTAuth(kc \*auth.KeyChain)" services/api-gateway/middleware/auth.go`
    - `grep -q "func ViewerJWTAuth(kc \*auth.KeyChain)" services/api-gateway/middleware/viewer_auth.go`
    - `grep -q "ValidateJWTWithKeyChain\|ValidateServiceJWTWithKeyChain\|ValidateViewerJWTWithKeyChain" shared/middleware/auth.go shared/middleware/service_auth.go services/api-gateway/middleware/auth.go services/api-gateway/middleware/viewer_auth.go`
    - `grep -q "TestServiceJWTAuth_ChainIsolation\|ChainIsolation" shared/middleware/service_auth_test.go`
    - `cd /home/moersener/Hobby/all-chat && go test ./shared/middleware/... -count=1` exits 0
    - `cd /home/moersener/Hobby/all-chat && go build ./shared/middleware/... ./services/api-gateway/middleware/...` exits 0
  </acceptance_criteria>
  <done>Middlewares accept *KeyChain. Cross-chain isolation test (D-10) added and passing. cmd/main.go callers will be broken until Task 2 — that is intentional sequencing.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Wire user + service KeyChains in api-gateway, share-service, overlay-manager, source-manager, auth-service main.go (FIX cross-chain bugs)</name>
  <files>services/api-gateway/cmd/main.go, services/api-gateway/handlers/websocket.go, services/api-gateway/handlers/websocket_viewer.go, services/share-service/cmd/main.go, services/share-service/handlers/shares.go, services/overlay-manager/cmd/main.go, services/source-manager/cmd/main.go, services/auth-service/cmd/main.go, services/auth-service/handlers/auth_handler.go, services/auth-service/handlers/viewer_auth.go</files>
  <read_first>
    - services/api-gateway/cmd/main.go (skim — focus on JWTAuth, ServiceJWTAuth, jwtSecret usage)
    - services/api-gateway/handlers/websocket.go line 111 (`auth.ValidateJWT(token, h.jwtSecret)` — needs KeyChain)
    - services/api-gateway/handlers/websocket_viewer.go lines 99, 156 (similar — both viewer)
    - services/share-service/cmd/main.go (skim — JWTSecret loading + handler construction)
    - services/share-service/handlers/shares.go lines 47–67 (handler struct + constructor) and lines 390–400, 645–655 (the BUG: GenerateServiceJWT uses h.jwtSecret)
    - services/source-manager/cmd/main.go line 154 (`middleware.ServiceJWTAuth(serviceAuthSecret)`)
    - services/overlay-manager/cmd/main.go lines 264, 320 (`middleware.JWTAuth(config.JWTSecret)`)
    - services/auth-service/handlers/auth_handler.go lines 227, 366, 438 (`auth.GenerateToken(... h.jwtSecret ...)`)
    - services/auth-service/handlers/viewer_auth.go line 535 (inline JWT signing with h.jwtSecret)
    - .planning/phases/14-secret-rotation-infrastructure/14-RESEARCH.md §4 "Validators and What They Validate" + Pitfall 4
  </read_first>
  <behavior>
    - Every service that previously held a `jwtSecret` string now holds a `*auth.KeyChain` (often plus a separate `serviceKeyChain *auth.KeyChain` for service-JWT validation/issuance).
    - All issuance call sites use `*WithKid` variants signing with `kc.LatestKid()` and `kc.LatestSecret()`.
    - `services/api-gateway/cmd/main.go:563` switches from `jwtSecret` to `serviceKeyChain` for ServiceJWTAuth (BUGFIX).
    - `services/share-service/handlers/shares.go:395, :651` switches to `auth.GenerateServiceJWTWithKid(serviceKeyChain.LatestKid(), "share-service", string(serviceKeyChain.LatestSecret()), 30*time.Second)` (BUGFIX).
    - All `cmd/main.go` files load TWO KeyChains: `userKeyChain` from `JWT_SECRET` prefix; `serviceKeyChain` from `SERVICE_JWT_SECRET` prefix (where applicable). Services that don't issue/validate Service JWTs only load `userKeyChain`.
  </behavior>
  <action>
    Step 1 — auth-service `cmd/main.go`:
    a) Replace `jwtSecret := os.Getenv("JWT_SECRET")` block (lines 102, 126–128) with:
       ```go
       userKeyChain, err := auth.NewKeyChainFromEnv("JWT_SECRET")
       if err != nil { log.Fatal("JWT key chain init failed", zap.Error(err)) }
       log.Info("JWT key chain initialized", zap.String("latest_kid", userKeyChain.LatestKid()))
       ```
    b) Pass `userKeyChain` (and `jwtExpiryHours`) to `NewAuthHandler` and `NewViewerAuthHandler` instead of the raw secret string.

    Step 2 — auth-service `handlers/auth_handler.go`:
    a) Change struct field `jwtSecret string` → `userKeyChain *auth.KeyChain`.
    b) Change constructor parameter and update assignment.
    c) Replace each `auth.GenerateToken(user.ID, user.Username, h.jwtSecret, h.jwtExpiry, user.IsAdmin)` (lines 227, 366, 438) with:
       ```go
       auth.GenerateTokenWithKid(h.userKeyChain.LatestKid(), user.ID, user.Username, string(h.userKeyChain.LatestSecret()), h.jwtExpiry, user.IsAdmin)
       ```

    Step 3 — auth-service `handlers/viewer_auth.go`:
    a) Change struct field `jwtSecret string` → `userKeyChain *auth.KeyChain`.
    b) Constructor parameter type matches.
    c) Replace the inline JWT construction at line 535 (`token.SignedString([]byte(h.jwtSecret))`) with a call to `auth.GenerateViewerJWTWithKid(h.userKeyChain.LatestKid(), claims, string(h.userKeyChain.LatestSecret()))`.
    d) The local `StringEncryptor` interface is preserved (Plan 14-04 already verified `*MultiKeyEncryptor` satisfies it).

    Step 4 — api-gateway `cmd/main.go`:
    a) Add at startup:
       ```go
       userKeyChain, err := auth.NewKeyChainFromEnv("JWT_SECRET")
       if err != nil { log.Fatal("JWT key chain init failed", zap.Error(err)) }
       serviceKeyChain, err := auth.NewKeyChainFromEnv("SERVICE_JWT_SECRET")
       if err != nil { log.Fatal("service JWT key chain init failed", zap.Error(err)) }
       ```
    b) Replace `protectedAPI.Use(sharedmiddleware.JWTAuth(jwtSecret))` (lines 437, 514) with `sharedmiddleware.JWTAuth(userKeyChain)`.
    c) **CRITICAL BUGFIX (line 563):** Replace `internal.Use(sharedmiddleware.ServiceJWTAuth(jwtSecret, ...))` with:
       ```go
       internal.Use(sharedmiddleware.ServiceJWTAuth(serviceKeyChain, "share-service", "overlay-manager", "auth-service"))
       ```
    d) Pass `userKeyChain` to handlers that need it (websocket handlers).

    Step 5 — api-gateway `handlers/websocket.go` and `websocket_viewer.go`:
    a) Change `h.jwtSecret string` field → `h.userKeyChain *auth.KeyChain`.
    b) Replace `auth.ValidateJWT(token, h.jwtSecret)` (websocket.go:111) with `auth.ValidateJWTWithKeyChain(token, h.userKeyChain)`.
    c) Replace `auth.ValidateViewerJWT(token, h.jwtSecret)` (websocket_viewer.go:99, 156) with `auth.ValidateViewerJWTWithKeyChain(token, h.userKeyChain)`.

    Step 6 — share-service `cmd/main.go`:
    a) Add `userKeyChain` AND `serviceKeyChain` constructors at startup.
    b) Replace `api.Use(middleware.JWTAuth(jwtSecret))` with `middleware.JWTAuth(userKeyChain)`.
    c) Pass `serviceKeyChain` into the share handler constructor (new field).

    Step 7 — share-service `handlers/shares.go`:
    a) Add new struct field `serviceKeyChain *auth.KeyChain` alongside the existing `jwtSecret` field.
    b) Update constructor signature to accept `serviceKeyChain *auth.KeyChain`.
    c) **CRITICAL BUGFIX (lines 395, 651):** Replace:
       ```go
       serviceToken, err := auth.GenerateServiceJWT("share-service", h.jwtSecret, 30*time.Second)
       ```
       with:
       ```go
       serviceToken, err := auth.GenerateServiceJWTWithKid(
           h.serviceKeyChain.LatestKid(),
           "share-service",
           string(h.serviceKeyChain.LatestSecret()),
           30*time.Second,
       )
       ```
    d) Keep `h.jwtSecret` for any spots where it's used for USER JWT operations (those are correct as `JWT_SECRET`-derived). Most likely `h.jwtSecret` can be replaced wholesale by `h.userKeyChain *auth.KeyChain` — audit each usage. If it is unused after the GenerateServiceJWT migration, delete the field.

    Step 8 — overlay-manager `cmd/main.go`:
    a) Add `userKeyChain` constructor at startup.
    b) Replace `middleware.JWTAuth(config.JWTSecret)` (lines 264, 320) with `middleware.JWTAuth(userKeyChain)`.

    Step 9 — source-manager `cmd/main.go`:
    a) Add `serviceKeyChain` constructor at startup (uses `SERVICE_JWT_SECRET` prefix per D-10).
    b) Replace `middleware.ServiceJWTAuth(serviceAuthSecret)` (line 154) with `middleware.ServiceJWTAuth(serviceKeyChain)`.

    Step 10 — Compile every touched service:
    ```
    cd /home/moersener/Hobby/all-chat && go build ./services/api-gateway/... ./services/share-service/... ./services/overlay-manager/... ./services/source-manager/... ./services/auth-service/...
    ```

    Step 11 — Run tests:
    ```
    cd /home/moersener/Hobby/all-chat && go test ./services/api-gateway/... ./services/share-service/... ./services/overlay-manager/... ./services/source-manager/... ./services/auth-service/... -count=1
    ```
    Update test fixtures as needed: any test that constructs a handler with a string JWT secret needs to switch to `auth.NewKeyChain(...)` with a one-entry test chain.

    Step 12 — Add a regression test for the share-service bugfix (Pitfall 4): in `services/share-service/handlers/shares_test.go`, add `TestShares_GenerateServiceJWT_UsesServiceChain` that:
    - Constructs handler with userKeyChain=secret-A, serviceKeyChain=secret-B.
    - Calls the relevant share-link generation method.
    - Captures the issued service token, validates it against secret-B chain (succeeds), validates it against secret-A chain (fails with ErrInvalidToken).

    Step 13 — Add a regression test for the api-gateway bugfix in a new `services/api-gateway/cmd/main_routing_test.go` or expand existing routing tests: assert that `/internal/ws/notify` route requires a token signed with the SERVICE_JWT_SECRET chain, NOT the JWT_SECRET chain.
  </action>
  <verify>
    <automated>cd /home/moersener/Hobby/all-chat && go build ./... && go test ./services/api-gateway/... ./services/share-service/... ./services/overlay-manager/... ./services/source-manager/... ./services/auth-service/... -count=1</automated>
  </verify>
  <acceptance_criteria>
    - `grep -q "userKeyChain, err := auth.NewKeyChainFromEnv(\"JWT_SECRET\")" services/api-gateway/cmd/main.go`
    - `grep -q "serviceKeyChain, err := auth.NewKeyChainFromEnv(\"SERVICE_JWT_SECRET\")" services/api-gateway/cmd/main.go`
    - `grep -q "ServiceJWTAuth(serviceKeyChain" services/api-gateway/cmd/main.go` (BUGFIX line 563)
    - `! grep -q "ServiceJWTAuth(jwtSecret\b" services/api-gateway/cmd/main.go` (the bug is gone)
    - `grep -q "GenerateServiceJWTWithKid" services/share-service/handlers/shares.go`
    - `! grep -E "GenerateServiceJWT\(\"share-service\", h.jwtSecret" services/share-service/handlers/shares.go` (BUGFIX confirmed)
    - `grep -q "h.serviceKeyChain.LatestSecret\|h.serviceKeyChain.LatestKid" services/share-service/handlers/shares.go`
    - `grep -q "auth.GenerateTokenWithKid\|auth.GenerateJWTWithKid" services/auth-service/handlers/auth_handler.go`
    - `grep -q "auth.GenerateViewerJWTWithKid" services/auth-service/handlers/viewer_auth.go`
    - `grep -q "auth.ValidateJWTWithKeyChain" services/api-gateway/handlers/websocket.go`
    - `grep -q "auth.ValidateViewerJWTWithKeyChain" services/api-gateway/handlers/websocket_viewer.go`
    - `grep -q "userKeyChain\|serviceKeyChain" services/source-manager/cmd/main.go services/overlay-manager/cmd/main.go services/share-service/cmd/main.go`
    - `grep -q "TestShares_GenerateServiceJWT_UsesServiceChain\|UsesServiceChain" services/share-service/handlers/`
    - `cd /home/moersener/Hobby/all-chat && go build ./...` exits 0
    - `cd /home/moersener/Hobby/all-chat && go test ./services/api-gateway/... ./services/share-service/... ./services/overlay-manager/... ./services/source-manager/... ./services/auth-service/... -count=1` exits 0
  </acceptance_criteria>
  <done>All five services use *auth.KeyChain at issuance and validation call sites. Two cross-chain bugs (share-service service-JWT issuance, api-gateway service-JWT validation) are fixed and have regression tests. Build is green; tests pass.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: Encrypt kick_oauth_tokens on write/read in kick-listener and overlay-manager handlers</name>
  <files>services/kick-listener/cmd/main.go, services/kick-listener/channels/manager.go, services/overlay-manager/handlers/sources.go</files>
  <read_first>
    - services/kick-listener/channels/manager.go lines 960–985 (current plaintext SELECT access_token)
    - services/kick-listener/cmd/main.go (current — does NOT yet construct an encryptor)
    - services/overlay-manager/handlers/sources.go lines 200–260 (Kick token CRUD — currently plaintext)
    - services/youtube-listener/oauth/store.go lines 38–80 + 144–170 (the EXACT pattern to replicate: encrypt-on-write, encryption_version gate on read)
    - .planning/phases/14-secret-rotation-infrastructure/14-RESEARCH.md §8 "Read and Write Path Inventory" (lines 709–722)
    - .planning/phases/14-secret-rotation-infrastructure/14-PATTERNS.md "services/kick-listener/channels/manager.go" + "services/overlay-manager/handlers/sources.go" sections (lines 433–462)
  </read_first>
  <behavior>
    - Test 1 (TestKickManager_DecryptVersioned, in services/kick-listener/channels/manager_test.go or a new file): insert a row in a test DB (or mocked pgxpool) with `encryption_version=1` and a versioned ciphertext for "test-access-token"; call the manager's read path; assert the returned access_token is "test-access-token".
    - Test 2 (TestKickManager_PlaintextLegacy): insert a row with `encryption_version=0` and plaintext "test-access-token"; assert the manager returns "test-access-token" verbatim (no decrypt attempted).
    - Test 3 (TestOverlayManagerSources_RoundTrip): unit test in `services/overlay-manager/handlers/sources_test.go` covering the upsert flow: write encrypted with `encryption_version=1`, read back, assert plaintext matches.
  </behavior>
  <action>
    Step 1 — kick-listener `cmd/main.go`:
    a) Add an encryptor construction at startup:
       ```go
       cipher, err := encryption.NewMultiKeyEncryptorFromEnv()
       if err != nil { log.Fatal("encryption init failed", zap.Error(err)) }
       ```
    b) Pass `cipher` to `channels.NewManager(...)`.

    Step 2 — kick-listener `channels/manager.go`:
    a) Add field to `Manager` struct: `cipher *encryption.MultiKeyEncryptor`.
    b) Update `NewManager` constructor to accept and store the cipher.
    c) Modify the SELECT at line 968 to additionally select `encryption_version`:
       ```go
       var accessToken string
       var encryptionVersion int16
       query := `SELECT access_token, encryption_version FROM kick_oauth_tokens WHERE channel_id = $1 AND expiry > NOW() ORDER BY expiry DESC LIMIT 1`
       err := pool.QueryRow(m.ctx, query, channelSlug).Scan(&accessToken, &encryptionVersion)
       if err != nil { return "", err }
       if encryptionVersion >= 1 {
           plaintext, err := m.cipher.DecryptString(accessToken)
           if err != nil { return "", fmt.Errorf("decrypt kick access_token: %w", err) }
           return plaintext, nil
       }
       return accessToken, nil // legacy plaintext row
       ```

    Step 3 — overlay-manager `handlers/sources.go`:
    a) Add field `cipher *encryption.MultiKeyEncryptor` to the relevant handler struct (or accept it from `cmd/main.go`'s existing `tokenCipher` from Plan 14-04).
    b) On reads (line 216 area), add `encryption_version` to the SELECT, and decrypt when `>=1`. Pattern same as Step 2.
    c) On writes (line 235 area), encrypt the tokens BEFORE the INSERT and include `encryption_version=1`:
       ```go
       encryptedAccess, err := h.cipher.EncryptString(token.AccessToken)
       if err != nil { return fmt.Errorf("encrypt access_token: %w", err) }
       var encryptedRefresh sql.NullString
       if token.RefreshToken != "" {
           er, err := h.cipher.EncryptString(token.RefreshToken)
           if err != nil { return fmt.Errorf("encrypt refresh_token: %w", err) }
           encryptedRefresh = sql.NullString{String: er, Valid: true}
       }
       _, err = h.db.Exec(ctx, insertQuery, adminUserID, channelID, encryptedAccess, encryptedRefresh, /* ... */, 1 /* encryption_version */)
       ```
       (Adjust insert SQL to include the `encryption_version` column.)

    Step 4 — Update insert SQL to include `encryption_version`:
    ```sql
    INSERT INTO kick_oauth_tokens (user_id, channel_id, access_token, refresh_token, ..., encryption_version)
    VALUES ($1, $2, $3, $4, ..., $N)
    ON CONFLICT (...) DO UPDATE SET access_token = EXCLUDED.access_token, refresh_token = EXCLUDED.refresh_token, ..., encryption_version = EXCLUDED.encryption_version, updated_at = NOW();
    ```

    Step 5 — Add the unit tests described in `<behavior>`. Use `pgxmock` if already in the codebase; otherwise use a real test DB via the existing `make test-integration` style.

    Step 6 — Compile and run tests:
    ```
    cd /home/moersener/Hobby/all-chat && go build ./services/kick-listener/... ./services/overlay-manager/... && go test ./services/kick-listener/... ./services/overlay-manager/... -count=1
    ```

    Step 7 — Confirm tiktok-listener is NOT touched. The Node.js service is explicitly out of scope for this Go-side plan; migration 051 ships the column, and a future phase wires the Node.js write/read paths.
  </action>
  <verify>
    <automated>cd /home/moersener/Hobby/all-chat && go test ./services/kick-listener/... ./services/overlay-manager/... -count=1 -run 'TestKickManager_Decrypt|TestKickManager_Plaintext|TestOverlayManagerSources_RoundTrip|TestKick'</automated>
  </verify>
  <acceptance_criteria>
    - `grep -q "encryption.NewMultiKeyEncryptorFromEnv" services/kick-listener/cmd/main.go`
    - `grep -q "cipher \*encryption.MultiKeyEncryptor" services/kick-listener/channels/manager.go`
    - `grep -q "encryption_version" services/kick-listener/channels/manager.go` (read path now reads the column)
    - `grep -q "cipher.DecryptString\|m.cipher.DecryptString" services/kick-listener/channels/manager.go`
    - `grep -q "encryption_version" services/overlay-manager/handlers/sources.go` (write path now sets the column)
    - `grep -q "EncryptString" services/overlay-manager/handlers/sources.go` (writes encrypt)
    - `grep -q "TestKickManager_DecryptVersioned\|TestKick.*Decrypt" services/kick-listener/channels/`
    - `cd /home/moersener/Hobby/all-chat && go build ./services/kick-listener/... ./services/overlay-manager/...` exits 0
    - `cd /home/moersener/Hobby/all-chat && go test ./services/kick-listener/... ./services/overlay-manager/... -count=1` exits 0
  </acceptance_criteria>
  <done>kick_oauth_tokens read/write paths encrypt/decrypt with the versioned scheme. New writes set encryption_version=1; existing v0 rows continue to read as plaintext. tiktok_oauth_tokens is explicitly NOT migrated (Node.js scope deferral acknowledged).</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Client → API JWT | Bearer tokens flow through KeyChain.KeyFunc; algorithm-confusion rejected |
| Service → Service JWT | Listener-issued service tokens validated against SERVICE_JWT_SECRET chain — independent from user chain |
| OAuth provider → kick_oauth_tokens | Plaintext access/refresh tokens cross from Kick's OAuth response into PostgreSQL via overlay-manager handlers/sources.go; now wrapped in MultiKeyEncryptor.EncryptString |
| kick_oauth_tokens → process | kick-listener.channels/manager.go reads the row, decrypts when `encryption_version >= 1`, holds the plaintext to authenticate Pusher WebSocket |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-14-05-01 | Spoofing | User JWT accepted on `/internal/ws/notify` (api-gateway line 563 BUG) | mitigate | Task 2 Step 4c switches the middleware to `serviceKeyChain`; regression test in Step 13 asserts a user-chain token is rejected with 401 |
| T-14-05-02 | Spoofing | share-service-issued service tokens forge as user tokens (Pitfall 4) | mitigate | Task 2 Step 7c migrates to `auth.GenerateServiceJWTWithKid` with `serviceKeyChain`; new TestShares_GenerateServiceJWT_UsesServiceChain regression test |
| T-14-05-03 | Spoofing | User chain accidentally accepted on source-manager protected routes (D-10 cross-chain) | mitigate | source-manager `cmd/main.go` Step 9 switches to `serviceKeyChain`; D-10 isolation test in Plan 14-02 + middleware test in Task 1 covers |
| T-14-05-04 | Tampering | Plaintext kick_oauth_tokens rows in DB after Phase 14 deploy (mid-rollout) | accept | encryption_version gate at read paths handles plaintext gracefully (D-16 transition state); sweeper (Plan 14-06) migrates v0 rows to v1 on its first run |
| T-14-05-05 | Information Disclosure | Logging plaintext access_token during error paths in kick-listener | mitigate | All error returns wrap the encryption error with `fmt.Errorf("decrypt kick access_token: %w", err)` — never include the actual token value |
| T-14-05-06 | Repudiation | Existing legacy User JWTs (no kid) silently rejected after middleware swap | mitigate | KeyChain legacy fallback (D-08) accepts no-kid tokens via `os.Getenv("JWT_SECRET")` value loaded into `kc.legacy` |
| T-14-05-07 | Repudiation | Service JWT TTL=15min mid-rotation: tokens issued under old kid still in flight when validator drops the kid | accept | D-09 retire timeline says drop after `T+max(token_TTL)`; Plan 14-07 deployment manifest updates document a 30-minute grace window for service tokens |
| T-14-05-08 | Tampering | TikTok migration column 051 was applied but no code updates Node.js write path → ALL tiktok_oauth_tokens rows remain `encryption_version=0` plaintext | accept | Documented Open Question 1 deferral: Node.js encryption is explicitly out of Phase 14 scope. Sweeper (Plan 14-06) skips v0 rows on tiktok_oauth_tokens until a future phase ships the Node.js encrypt path. The leak protection benefit for Twitch/YouTube/Kick still ships in Phase 14. |
</threat_model>

<verification>
- All migration build steps green.
- New regression tests for the two cross-chain bugs (share-service service-JWT issuance, api-gateway service-JWT validation) committed and passing.
- kick_oauth_tokens encrypt/decrypt round-trip proven by unit test.
- TikTok intentionally NOT migrated; tiktok_oauth_tokens table has the column from migration 051 but the Node.js encryption is deferred.
</verification>

<success_criteria>
- Every JWT validation site uses *auth.KeyChain.
- Every JWT issuance site uses GenerateXxxWithKid + LatestKid()/LatestSecret().
- share-service Pitfall 4 fixed; api-gateway parallel bug fixed.
- kick_oauth_tokens read/write encrypts/decrypts with versioned scheme; legacy v0 rows read as plaintext until the sweeper runs.
- D-10 chain isolation enforced at every middleware boundary; cross-chain regression tests passing.
- Module-wide build green; all touched-service tests green.
</success_criteria>

<output>
After completion, create `.planning/phases/14-secret-rotation-infrastructure/14-05-SUMMARY.md` documenting:
- The two bugs fixed (share-service GenerateServiceJWT chain, api-gateway internal route ServiceJWTAuth chain) with regression tests added.
- KeyChain wiring topology: which services hold userKeyChain only vs. both (userKeyChain + serviceKeyChain).
- Confirmation that tiktok_oauth_tokens encryption code is NOT migrated (Node.js scope, deferred).
- Note for Plan 14-06: sweeper will sweep kick_oauth_tokens for v1 → CurrentKid migration; tiktok_oauth_tokens scan is skipped (no rows >= v1 will exist until future Node.js work).
- Note for Plan 14-07: deployment manifest updates must (a) add `JWT_SECRET_V1` and `SERVICE_JWT_SECRET_V1` to every service that holds a KeyChain; (b) standardize `ENCRYPTION_KEY` env var name in token-refresh-service + twitch-eventsub-listener (Pitfall 1).
</output>
