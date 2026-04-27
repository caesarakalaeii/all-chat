---
phase: 14-secret-rotation-infrastructure
plan: 02
type: execute
wave: 1
depends_on: []
files_modified:
  - shared/auth/jwt.go
  - shared/auth/keychains_test.go
autonomous: true
decisions_addressed:
  - D-07
  - D-08
  - D-09
  - D-10
  - D-11
  - D-12
must_haves:
  truths:
    - "All new JWTs (User, Viewer, Service, Impersonation) carry a kid header"
    - "Validators dispatch on kid header to select per-key secret; unknown/missing kid falls back to legacy JWT_SECRET"
    - "User and Service JWT chains are independent; cross-chain validation fails by construction"
    - "Expired tokens are rejected even when their kid is in the active validator chain"
  artifacts:
    - path: "shared/auth/jwt.go"
      provides: "KeyChain, NewKeyChainFromEnv, KeyFunc, GenerateJWTWithKid, GenerateViewerJWTWithKid, GenerateServiceJWTWithKid, GenerateImpersonationJWTWithKid, ValidateJWTWithKeyChain, ValidateViewerJWTWithKeyChain, ValidateServiceJWTWithKeyChain"
      min_lines: 350
    - path: "shared/auth/keychains_test.go"
      provides: "TestKeyChain_*, TestLegacyFallback, TestChainIsolation, TestExpiredKidStillRejects, TestKeyFunc_*"
      min_lines: 250
  key_links:
    - from: "shared/auth/jwt.go (new ValidateJWTWithKeyChain)"
      to: "shared/auth/jwt.go (existing Claims struct)"
      via: "jwt.ParseWithClaims with KeyChain.KeyFunc as the keyfunc; same Claims/ViewerClaims/ServiceClaims types preserved"
      pattern: "jwt.ParseWithClaims.*KeyFunc"
    - from: "shared/auth/jwt.go (new GenerateJWTWithKid)"
      to: "shared/auth/jwt.go (existing GenerateJWT)"
      via: "same claims construction; adds token.Header[\"kid\"] before SignedString"
      pattern: "token.Header\\[\"kid\"\\]"
---

<objective>
Add `kid` header support and multi-key validation to `shared/auth/jwt.go`. This is the JWT counterpart to Plan 14-01's encryption work — every other plan in Wave 2 depends on `KeyChain` and the new `*WithKid` / `*WithKeyChain` functions existing.

Purpose: Implements decisions D-07 (kid header on issuance), D-08 (multi-key validation with legacy fallback), D-09 (timeline doesn't change retire logic — that lives in deployment manifests, but the validator must accept multiple kids in parallel), D-10 (separate User vs Service chains via prefix arg), D-11 (TTS overlay JWT scope boundary — see acknowledgement below), D-12 (no denylist — rotation+TTL is the sole revocation).

D-11 honored: `services/overlay-manager/tts/jwt.go` is intentionally NOT in `files_modified` for this plan or for 14-05 — TTS overlay JWTs use Phase 13's per-overlay `tts_signing_secret` regeneration model, untouched by Phase 14. The User/Viewer/Service KeyChain plumbing added here does not extend into TTS-overlay token issuance.

Output:
- `KeyChain` type with `byKid map[string][]byte` and `legacy []byte`.
- `NewKeyChainFromEnv(prefix string)` reading `<PREFIX>_V1`, `<PREFIX>_V2`, ... and `<PREFIX>` (legacy).
- `KeyFunc(*jwt.Token) (interface{}, error)` for use as `jwt.Keyfunc`.
- `GenerateJWTWithKid`, `GenerateViewerJWTWithKid`, `GenerateServiceJWTWithKid`, `GenerateImpersonationJWTWithKid` — set `token.Header["kid"]` before signing.
- `ValidateJWTWithKeyChain`, `ValidateViewerJWTWithKeyChain`, `ValidateServiceJWTWithKeyChain` — use `KeyChain.KeyFunc`.
- Existing `GenerateJWT` / `ValidateJWT` (etc.) signatures preserved — call sites update in Wave 2.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.planning/ROADMAP.md
@.planning/phases/14-secret-rotation-infrastructure/14-CONTEXT.md
@.planning/phases/14-secret-rotation-infrastructure/14-RESEARCH.md
@.planning/phases/14-secret-rotation-infrastructure/14-PATTERNS.md
@.planning/phases/14-secret-rotation-infrastructure/14-VALIDATION.md
@shared/auth/jwt.go

<interfaces>
<!-- Existing types preserved verbatim — KeyChain wraps them. -->

From shared/auth/jwt.go (current):
```go
type Claims struct { UserID, TwitchID, Username string; Roles []string; ImpersonatedBy, ImpersonatedUser string; jwt.RegisteredClaims }
type ViewerClaims struct { ViewerID, SessionID, Platform, PlatformUserID, Username, DisplayName, AvatarURL string; IsViewer, IsPremium, IsAdmin bool; jwt.RegisteredClaims }
type ServiceClaims struct { ServiceName string; Permissions []string; jwt.RegisteredClaims }

func GenerateJWT(userID, twitchID, username, secret string, isAdmin bool) (string, error)
func GenerateToken(userID, username, secret string, expiry time.Duration, isAdmin bool) (string, error)
func GenerateImpersonationJWT(adminUserID, adminUsername, targetUserID, targetUsername, targetTwitchID, secret string) (string, error)
func GenerateServiceJWT(serviceName, secret string, expiry time.Duration) (string, error)
func ValidateJWT(tokenString, secret string) (*Claims, error)
func ValidateViewerJWT(tokenString, secret string) (*ViewerClaims, error)
func ValidateServiceJWT(tokenString, secret string) (*ServiceClaims, error)
var ErrInvalidToken, ErrExpiredToken error
```

External JWT library:
```go
import "github.com/golang-jwt/jwt/v5"
// Keyfunc signature: func(*jwt.Token) (interface{}, error)
// token.Header is a map[string]interface{}; "kid" is an optional string header per RFC7515.
// jwt.SigningMethodHS256 is *jwt.SigningMethodHMAC
```

NOTE: The viewer JWT's `generateViewerJWT` call site lives INSIDE `services/auth-service/handlers/viewer_auth.go` (it inlines `jwt.NewWithClaims`+`SignedString`); this plan adds the helper to `shared/auth/jwt.go` so Plan 14-04 can call `auth.GenerateViewerJWTWithKid`.
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Add KeyChain type, NewKeyChainFromEnv, KeyFunc to shared/auth/jwt.go</name>
  <files>shared/auth/jwt.go, shared/auth/keychains_test.go</files>
  <read_first>
    - shared/auth/jwt.go (full file — every existing function is the structural template)
    - .planning/phases/14-secret-rotation-infrastructure/14-RESEARCH.md §4 "Proposed KeyFunc Shape" (lines 380–425)
    - .planning/phases/14-secret-rotation-infrastructure/14-PATTERNS.md "shared/auth/jwt.go (MODIFY)" section (lines 150–235)
    - .planning/phases/14-secret-rotation-infrastructure/14-CONTEXT.md decisions D-07, D-08, D-10, D-12
    - .planning/phases/14-secret-rotation-infrastructure/14-VALIDATION.md table rows for D-07/D-08, D-08, D-10, D-09 (lines 49–52)
  </read_first>
  <behavior>
    - TestKeyChain_NewFromEnv_LoadsAllVersions: set `JWT_SECRET_V1=a..a`, `JWT_SECRET_V2=b..b`, `JWT_SECRET=c..c`; `NewKeyChainFromEnv("JWT_SECRET")` returns chain with byKid={"v1":a..,"v2":b..} and legacy=c..; latest kid is "v2".
    - TestKeyChain_NewFromEnv_RequiresAtLeastOneVersioned: only `JWT_SECRET=c..c` set, no V1 → returns error matching "no versioned JWT secrets".
    - TestKeyChain_NewFromEnv_StopsAtFirstGap: set V1, V3 but not V2 → constructor stops scanning at V2 (treats V1 as latest); does NOT load V3.
    - TestKeyChain_NewFromEnv_PrefixIsolation: set both `JWT_SECRET_V1=a..a` AND `SERVICE_JWT_SECRET_V1=z..z`; `NewKeyChainFromEnv("JWT_SECRET")` only loads `a..a`, NOT `z..z` (D-10 chain isolation).
    - TestKeyChain_KeyFunc_MatchesByKid: token signed with kid "v1" using secret a..a; chain={"v1":a..}; KeyFunc returns a.. — token validates.
    - TestKeyChain_KeyFunc_NoKidUsesLegacy: token signed with NO kid header (legacy issuer) using legacy secret c..c; chain={byKid:{"v1":a..}, legacy:c..}; KeyFunc returns c.. — token validates.
    - TestKeyChain_KeyFunc_UnknownKidFallsBackToLegacy: token signed with kid "v99" using secret c..c; chain={byKid:{"v1":a..}, legacy:c..}; KeyFunc returns c.. — validates IFF the legacy secret matches.
    - TestKeyChain_KeyFunc_NoLegacyAndUnknownKid: chain={byKid:{"v1":a..}, legacy:nil}; token with kid "v99" → KeyFunc returns error matching "unknown kid".
    - TestKeyChain_KeyFunc_RejectsNonHMAC: hand-craft a token claiming alg=RS256 (use `jwt.SigningMethodNone` or similar); KeyFunc returns error matching "unexpected signing method".
    - TestGenerateJWTWithKid_HeaderPresent: GenerateJWTWithKid("v1", "user-1", "twitch-1", "alice", "secret", false) → parse the resulting token (without verifying); Header["kid"] == "v1".
    - TestValidateJWTWithKeyChain_RoundTrip: chain with kid "v1"→secret-a; GenerateJWTWithKid("v1", ..., "secret-a", false) → ValidateJWTWithKeyChain returns Claims with UserID == "user-1".
    - TestValidateJWTWithKeyChain_LegacyToken: kid-less token from existing GenerateJWT(..., "legacy-secret", ...); chain with legacy="legacy-secret"; ValidateJWTWithKeyChain succeeds.
    - TestValidateJWTWithKeyChain_ExpiredKidStillRejects: GenerateJWTWithKid with `time.Now().Add(-1*time.Hour)` expiry, kid "v1", chain has v1 → ValidateJWTWithKeyChain returns ErrExpiredToken.
    - TestValidateServiceJWTWithKeyChain_ChainIsolation: User chain {v1: secret-user}; Service chain {v1: secret-service}; token signed with secret-user kid v1 → ValidateServiceJWTWithKeyChain(token, serviceChain) returns ErrInvalidToken (D-10 cross-chain validation MUST fail).
    - TestValidateViewerJWTWithKeyChain_RoundTrip: Mirror of User case but for ViewerClaims.
  </behavior>
  <action>
    Step 1 — Append to `shared/auth/jwt.go` (do NOT delete existing functions; existing call sites still use them until Wave 2):

    Add after the existing imports:
    ```go
    import (
        // existing imports preserved
        "os"
        "strconv"
    )
    ```

    Add new errors next to existing var block:
    ```go
    var (
        ErrInvalidToken              = errors.New("invalid token")     // existing
        ErrExpiredToken              = errors.New("token expired")     // existing
        ErrNoVersionedJWTSecrets     = errors.New("no versioned JWT secrets configured: set <PREFIX>_V1")
        ErrUnknownKidNoLegacy        = errors.New("unknown kid and no legacy fallback secret")
    )
    ```

    Add the KeyChain type and constructor:
    ```go
    // KeyChain holds multiple HS256 secrets indexed by string kid ("v1", "v2", ...).
    // The legacy field holds the kid-less <PREFIX> env var value for backwards-compat
    // with tokens issued before kid headers were introduced (D-08).
    type KeyChain struct {
        byKid     map[string][]byte
        legacy    []byte
        latestKid string // highest "v<n>" registered; used by callers for issuance
    }

    // NewKeyChainFromEnv reads <prefix>_V1, <prefix>_V2, ... in sequence until an env
    // var is missing or empty. <prefix> (no version suffix) is loaded as the legacy
    // fallback. prefix examples: "JWT_SECRET", "SERVICE_JWT_SECRET" (D-10 chain isolation).
    //
    // Returns ErrNoVersionedJWTSecrets if no <prefix>_V1 is set; legacy alone is not
    // sufficient because new code MUST issue with kid headers per D-07.
    func NewKeyChainFromEnv(prefix string) (*KeyChain, error) {
        byKid := make(map[string][]byte)
        var latestKid string
        for n := 1; ; n++ {
            envName := prefix + "_V" + strconv.Itoa(n)
            v := os.Getenv(envName)
            if v == "" { break }
            kid := "v" + strconv.Itoa(n)
            byKid[kid] = []byte(v)
            latestKid = kid
        }
        if len(byKid) == 0 {
            return nil, fmt.Errorf("%w (looked for %s_V1)", ErrNoVersionedJWTSecrets, prefix)
        }
        var legacy []byte
        if v := os.Getenv(prefix); v != "" {
            legacy = []byte(v)
        }
        return &KeyChain{byKid: byKid, legacy: legacy, latestKid: latestKid}, nil
    }

    // NewKeyChain constructs from explicit byKid map and legacy bytes (for tests).
    func NewKeyChain(byKid map[string][]byte, legacy []byte, latestKid string) *KeyChain {
        return &KeyChain{byKid: byKid, legacy: legacy, latestKid: latestKid}
    }

    // LatestKid returns the highest "v<n>" registered. Issuers should sign with this kid.
    func (kc *KeyChain) LatestKid() string { return kc.latestKid }

    // LatestSecret returns the bytes for LatestKid (issuer convenience).
    func (kc *KeyChain) LatestSecret() []byte { return kc.byKid[kc.latestKid] }

    // KeyFunc returns the per-token HS256 secret based on token.Header["kid"].
    // Falls back to legacy when kid is absent or unrecognised. Rejects non-HMAC algorithms.
    func (kc *KeyChain) KeyFunc(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        kid, hasKid := token.Header["kid"].(string)
        if !hasKid || kid == "" {
            if kc.legacy == nil {
                return nil, errors.New("token has no kid and no legacy fallback configured")
            }
            return kc.legacy, nil
        }
        if key, ok := kc.byKid[kid]; ok {
            return key, nil
        }
        if kc.legacy == nil {
            return nil, fmt.Errorf("%w: kid=%q", ErrUnknownKidNoLegacy, kid)
        }
        return kc.legacy, nil
    }
    ```

    Add the `*WithKid` issuance variants. Each one mirrors its existing kid-less twin and inserts `token.Header["kid"] = kid` before `SignedString`:
    ```go
    func GenerateJWTWithKid(kid, userID, twitchID, username, secret string, isAdmin bool) (string, error) {
        // identical to GenerateJWT body, except the line below
        // ... build claims ...
        token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
        token.Header["kid"] = kid
        return token.SignedString([]byte(secret))
    }
    func GenerateTokenWithKid(kid, userID, username, secret string, expiry time.Duration, isAdmin bool) (string, error) { /* ditto */ }
    func GenerateImpersonationJWTWithKid(kid, adminUserID, adminUsername, targetUserID, targetUsername, targetTwitchID, secret string) (string, error) { /* ditto */ }
    func GenerateServiceJWTWithKid(kid, serviceName, secret string, expiry time.Duration) (string, error) { /* ditto */ }
    ```

    Also add a viewer issuance helper (since Plan 14-04 needs to migrate `services/auth-service/handlers/viewer_auth.go`'s inline JWT construction to a shared helper):
    ```go
    // GenerateViewerJWTWithKid issues a viewer JWT with kid header. Mirrors the inline
    // construction currently in services/auth-service/handlers/viewer_auth.go.
    func GenerateViewerJWTWithKid(kid string, claims ViewerClaims, secret string) (string, error) {
        token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
        token.Header["kid"] = kid
        return token.SignedString([]byte(secret))
    }
    ```

    Add the `*WithKeyChain` validators. Each mirrors its existing twin but uses `kc.KeyFunc`:
    ```go
    func ValidateJWTWithKeyChain(tokenString string, kc *KeyChain) (*Claims, error) {
        token, err := jwt.ParseWithClaims(tokenString, &Claims{}, kc.KeyFunc)
        // identical error handling as ValidateJWT
        if err != nil {
            if errors.Is(err, jwt.ErrTokenExpired) { return nil, ErrExpiredToken }
            return nil, ErrInvalidToken
        }
        if claims, ok := token.Claims.(*Claims); ok && token.Valid { return claims, nil }
        return nil, ErrInvalidToken
    }
    func ValidateViewerJWTWithKeyChain(tokenString string, kc *KeyChain) (*ViewerClaims, error) { /* ditto, ViewerClaims */ }
    func ValidateServiceJWTWithKeyChain(tokenString string, kc *KeyChain) (*ServiceClaims, error) { /* ditto, ServiceClaims */ }
    ```

    Step 2 — Create `shared/auth/keychains_test.go` covering all 15 listed test behaviors. Use `t.Setenv` for env-driven tests. For TestKeyChain_KeyFunc_RejectsNonHMAC, build a token via `jwt.NewWithClaims(jwt.SigningMethodNone, ...)` and use `jwt.UnsafeAllowNoneSignatureType` per golang-jwt docs.

    Step 3 — Validate that existing call sites still compile (no breaking changes to existing function signatures):
    ```
    cd /home/moersener/Hobby/all-chat && go build ./shared/auth/... ./services/api-gateway/... ./services/auth-service/... ./services/share-service/... ./services/overlay-manager/... ./services/source-manager/...
    ```
  </action>
  <verify>
    <automated>cd /home/moersener/Hobby/all-chat && go test ./shared/auth/... -run 'TestKeyChain|TestGenerateJWTWithKid|TestValidate.*WithKeyChain|TestLegacyFallback|TestChainIsolation|TestExpired' -count=1 -v && go build ./...</automated>
  </verify>
  <acceptance_criteria>
    - `grep -q "type KeyChain struct" shared/auth/jwt.go`
    - `grep -q "func NewKeyChainFromEnv" shared/auth/jwt.go`
    - `grep -q "func (kc \*KeyChain) KeyFunc" shared/auth/jwt.go`
    - `grep -q "token.Header\[\"kid\"\] = kid" shared/auth/jwt.go` (proves issuance helpers set the header)
    - `grep -q "func GenerateJWTWithKid\|func GenerateServiceJWTWithKid\|func GenerateViewerJWTWithKid\|func GenerateImpersonationJWTWithKid" shared/auth/jwt.go` (all four issuance variants present)
    - `grep -q "func ValidateJWTWithKeyChain\|func ValidateViewerJWTWithKeyChain\|func ValidateServiceJWTWithKeyChain" shared/auth/jwt.go`
    - `grep -q "TestKeyChain_KeyFunc_NoKidUsesLegacy\|TestKeyChain_KeyFunc_UnknownKidFallsBack" shared/auth/keychains_test.go`
    - `grep -q "TestValidateServiceJWTWithKeyChain_ChainIsolation\|TestChainIsolation" shared/auth/keychains_test.go` (D-10)
    - `grep -q "TestValidateJWTWithKeyChain_ExpiredKidStillRejects\|TestExpired" shared/auth/keychains_test.go` (D-09)
    - `grep -q "func GenerateJWT(userID, twitchID, username, secret string, isAdmin bool)" shared/auth/jwt.go` (existing API preserved)
    - `cd /home/moersener/Hobby/all-chat && go test ./shared/auth/... -count=1` exits 0
    - `cd /home/moersener/Hobby/all-chat && go build ./...` exits 0
  </acceptance_criteria>
  <done>All 15 test behaviors pass. Existing JWT API preserved (no breaking changes — Wave 2 plans migrate callers). Module-wide build is green.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Client → API JWT validation | Untrusted bearer tokens flow into `KeyChain.KeyFunc` from HTTP Authorization headers |
| Service → Service JWT validation | source-manager and overlay-manager validate listener-issued service JWTs via `KeyChain` |
| K8s Secret → process env | `JWT_SECRET_V<n>` and `SERVICE_JWT_SECRET_V<n>` flow from `allchat-secrets` to `os.Getenv` |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-14-02-01 | Spoofing | User JWT forwarded as Service JWT (cross-chain trust) | mitigate | TestChainIsolation asserts that a token signed with a User-chain secret fails `ValidateServiceJWTWithKeyChain`; D-10 prefix-arg constructor enforces independent env-var namespaces |
| T-14-02-02 | Spoofing | Forged kid pointing to unknown version with no legacy fallback | mitigate | `KeyFunc` returns `ErrUnknownKidNoLegacy` when both byKid lookup misses AND legacy is nil; ParseWithClaims surfaces this as `ErrInvalidToken` |
| T-14-02-03 | Spoofing | Algorithm-confusion attack (token claiming `alg: none` or `alg: RS256`) | mitigate | `KeyFunc` rejects any non-HMAC signing method with `"unexpected signing method"`; TestKeyChain_KeyFunc_RejectsNonHMAC asserts |
| T-14-02-04 | Repudiation | Old User JWT remains valid for entire 24h TTL after rotation | accept | Documented in D-09 and CONTEXT.md; mitigation is timeline (drop legacy from validator at T+24h) — enforced in deployment manifests (Plan 14-07), not in this library |
| T-14-02-05 | Tampering | Expired token whose kid is still in the active chain bypasses TTL | mitigate | TestValidateJWTWithKeyChain_ExpiredKidStillRejects asserts that `jwt.ErrTokenExpired` is surfaced as `ErrExpiredToken` independent of kid validity (golang-jwt/jwt/v5 enforces ExpiresAt before KeyFunc dispatch on Valid) |
| T-14-02-06 | Information Disclosure | Test fixtures leaking real JWT secrets | accept | All test secrets are constants like `"secret-a"`, `"secret-service"` — not production values |
| T-14-02-07 | Denial of Service | `NewKeyChainFromEnv` busy-looping on V<n> if env names are unbounded | mitigate | The `for n := 1; ; n++` loop terminates on the first missing env var; documented as "stops at first gap" in TestKeyChain_NewFromEnv_StopsAtFirstGap |
</threat_model>

<verification>
- `go test ./shared/auth/... -count=1 -race` — all 15 test behaviors green.
- `go build ./...` from repo root — module-wide compile clean (no breaking changes to existing GenerateJWT/ValidateJWT signatures).
- `grep -c "token.Header\[\"kid\"\]" shared/auth/jwt.go` ≥ 4 (one per `*WithKid` issuance variant).
</verification>

<success_criteria>
- `KeyChain` type, `NewKeyChainFromEnv`, `KeyFunc` exist in `shared/auth/jwt.go`.
- All four `*WithKid` issuance variants set `token.Header["kid"]` before signing.
- All three `*WithKeyChain` validators exist and use `kc.KeyFunc`.
- Cross-chain isolation (D-10) is proven by test: a user-chain-signed token fails service-chain validation.
- Existing function signatures (GenerateJWT, ValidateJWT, etc.) unchanged — Wave 2 plans migrate callers.
- `go test ./shared/auth/... -count=1` exits 0; `go build ./...` exits 0.
</success_criteria>

<output>
After completion, create `.planning/phases/14-secret-rotation-infrastructure/14-02-SUMMARY.md` documenting:
- Final API surface (signatures of KeyChain, KeyFunc, all *WithKid and *WithKeyChain functions).
- The `latestKid` accessor pattern (LatestKid()/LatestSecret()) for issuer call-site convenience.
- Confirmation: existing `GenerateJWT`/`ValidateJWT` signatures preserved; Wave 2 will migrate.
- Note for Plan 14-05: the share-service bug (uses `JWT_SECRET` instead of `SERVICE_JWT_SECRET` for service JWT issuance) is fixed in that plan, NOT this one.
</output>
