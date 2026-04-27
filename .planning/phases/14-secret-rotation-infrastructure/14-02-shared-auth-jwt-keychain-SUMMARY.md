---
phase: 14-secret-rotation-infrastructure
plan: 02
subsystem: shared-auth
tags: [jwt, key-rotation, kid-header, multi-key, keychain, hmac, alg-confusion]
dependency_graph:
  requires: [shared/encryption.MultiKeyEncryptor]
  provides: [shared/auth.KeyChain, shared/auth.NewKeyChainFromEnv, shared/auth.KeyFunc, shared/auth.GenerateJWTWithKid, shared/auth.GenerateViewerJWTWithKid, shared/auth.GenerateServiceJWTWithKid, shared/auth.GenerateImpersonationJWTWithKid, shared/auth.GenerateTokenWithKid, shared/auth.ValidateJWTWithKeyChain, shared/auth.ValidateViewerJWTWithKeyChain, shared/auth.ValidateServiceJWTWithKeyChain]
  affects: [services/api-gateway, services/auth-service, services/share-service, services/overlay-manager, services/source-manager]
tech_stack:
  added: []
  patterns: [kid-header JWT issuance, multi-key KeyFunc dispatch, env-prefix chain isolation, alg-confusion rejection]
key_files:
  created:
    - shared/auth/keychains_test.go
  modified:
    - shared/auth/jwt.go
decisions:
  - D-07: kid header set on all five *WithKid issuance variants before SignedString
  - D-08: KeyChain.KeyFunc dispatches by kid; unknown kid falls back to legacy; absent kid uses legacy
  - D-09: ErrExpiredToken surfaced from jwt.ErrTokenExpired regardless of kid validity (golang-jwt/v5 enforces ExpiresAt before KeyFunc result)
  - D-10: NewKeyChainFromEnv prefix arg isolates JWT_SECRET_V<n> from SERVICE_JWT_SECRET_V<n>; cross-chain HMAC mismatch yields ErrInvalidToken (proven by TestValidateServiceJWTWithKeyChain_ChainIsolation)
  - D-11: services/overlay-manager/tts/jwt.go not modified — TTS boundary honored
  - D-12: KeyFunc rejects non-HMAC methods (alg=none, RS256 confusion) with "unexpected signing method" error; proven by TestKeyChain_KeyFunc_RejectsNonHMAC
metrics:
  duration: 17m
  completed: "2026-04-27"
  tasks: 1
  files: 2
---

# Phase 14 Plan 02: Shared Auth JWT KeyChain Summary

Adds `KeyChain` type and all associated `*WithKid` / `*WithKeyChain` functions to `shared/auth/jwt.go` — the JWT rotation counterpart to Plan 14-01's encryption work. Every Wave 2 plan that migrates JWT call sites depends on this API existing.

## What Changed

### Task 1: KeyChain type + all issuance and validation variants (TDD)

**RED commit (`4c835d02`):** `shared/auth/keychains_test.go` — 19 tests covering all PLAN.md behaviors and all VALIDATION.md rows for D-07/D-08/D-09/D-10/D-12. Tests failed to compile (all symbols undefined).

**GREEN commit (`d0032506`):** `shared/auth/jwt.go` — full implementation appended after existing API (no existing signatures removed or changed).

#### New API surface

```go
// Errors
var ErrNoVersionedJWTSecrets = errors.New("no versioned JWT secrets configured: set <PREFIX>_V1")
var ErrUnknownKidNoLegacy    = errors.New("unknown kid and no legacy fallback secret")

// KeyChain type
type KeyChain struct { /* unexported fields */ }

func NewKeyChainFromEnv(prefix string) (*KeyChain, error)
// prefix = "JWT_SECRET" for User+Viewer; "SERVICE_JWT_SECRET" for Service (D-10 isolation)

func NewKeyChain(byKid map[string][]byte, legacy []byte, latestKid string) *KeyChain
// explicit constructor for tests and bootstrappers

func (kc *KeyChain) LatestKid() string    // returns the highest registered "v<n>"
func (kc *KeyChain) LatestSecret() []byte // returns secret bytes for LatestKid

func (kc *KeyChain) KeyFunc(token *jwt.Token) (interface{}, error)
// implements jwt.Keyfunc — rejects non-HMAC, dispatches by kid, falls back to legacy

// Issuance helpers — each mirrors its kid-less twin + sets token.Header["kid"] = kid
func GenerateJWTWithKid(kid, userID, twitchID, username, secret string, isAdmin bool) (string, error)
func GenerateTokenWithKid(kid, userID, username, secret string, expiry time.Duration, isAdmin bool) (string, error)
func GenerateImpersonationJWTWithKid(kid, adminUserID, adminUsername, targetUserID, targetUsername, targetTwitchID, secret string) (string, error)
func GenerateServiceJWTWithKid(kid, serviceName, secret string, expiry time.Duration) (string, error)
func GenerateViewerJWTWithKid(kid string, claims ViewerClaims, secret string) (string, error)

// Validation helpers — each mirrors its kid-less twin using kc.KeyFunc
func ValidateJWTWithKeyChain(tokenString string, kc *KeyChain) (*Claims, error)
func ValidateViewerJWTWithKeyChain(tokenString string, kc *KeyChain) (*ViewerClaims, error)
func ValidateServiceJWTWithKeyChain(tokenString string, kc *KeyChain) (*ServiceClaims, error)
```

#### LatestKid/LatestSecret accessor pattern

Issuer call sites in Wave 2 use this pattern to auto-select the write kid:

```go
userChain, _ := auth.NewKeyChainFromEnv("JWT_SECRET")
kid := userChain.LatestKid()
secret := string(userChain.LatestSecret())
tok, err := auth.GenerateJWTWithKid(kid, userID, twitchID, username, secret, isAdmin)
```

This ensures that when `JWT_SECRET_V2` is added, issuers automatically flip to `kid="v2"` without code changes, and validators continue accepting `kid="v1"` tokens for the 24h TTL retire window (D-09).

#### Existing API preserved — Wave 2 migrates callers

The following existing functions are unchanged (same signatures, same behavior):

- `GenerateJWT`, `GenerateToken`, `GenerateImpersonationJWT`, `GenerateServiceJWT`
- `ValidateJWT`, `ValidateViewerJWT`, `ValidateServiceJWT`
- `ErrInvalidToken`, `ErrExpiredToken`
- `Claims`, `ViewerClaims`, `ServiceClaims`

Plans 14-04 and 14-05 migrate call sites to `*WithKid` / `*WithKeyChain` variants. Existing call sites continue to compile and function during the transition.

#### Note for Plan 14-05

The share-service bug (uses `JWT_SECRET` instead of `SERVICE_JWT_SECRET` for service JWT issuance) is deliberately NOT fixed here — that fix belongs to Plan 14-05 which updates all JWT validators and issuers.

## Test Coverage

19 tests in `shared/auth/keychains_test.go` covering all PLAN.md behaviors:

| Test | Decision | Status |
|------|----------|--------|
| TestKeyChain_NewFromEnv_LoadsAllVersions | D-08 | PASS |
| TestKeyChain_NewFromEnv_RequiresAtLeastOneVersioned | D-08 | PASS |
| TestKeyChain_NewFromEnv_StopsAtFirstGap | D-07 (DoS guard T-14-02-07) | PASS |
| TestKeyChain_NewFromEnv_PrefixIsolation | D-10 | PASS |
| TestKeyChain_KeyFunc_MatchesByKid | D-08 | PASS |
| TestKeyChain_KeyFunc_NoKidUsesLegacy | D-08 | PASS |
| TestKeyChain_KeyFunc_UnknownKidFallsBackToLegacy | D-08 | PASS |
| TestKeyChain_KeyFunc_NoLegacyAndUnknownKid | T-14-02-02 | PASS |
| TestKeyChain_KeyFunc_RejectsNonHMAC | D-12 / T-14-02-03 | PASS |
| TestGenerateJWTWithKid_HeaderPresent | D-07 | PASS |
| TestValidateJWTWithKeyChain_RoundTrip | D-07/D-08 | PASS |
| TestValidateJWTWithKeyChain_LegacyToken | D-08 | PASS |
| TestValidateJWTWithKeyChain_ExpiredKidStillRejects | D-09 / T-14-02-05 | PASS |
| TestValidateServiceJWTWithKeyChain_ChainIsolation | D-10 / T-14-02-01 | PASS |
| TestValidateViewerJWTWithKeyChain_RoundTrip | D-07/D-08 | PASS |
| TestGenerateServiceJWTWithKid_RoundTrip | D-07 | PASS |
| TestGenerateImpersonationJWTWithKid_HeaderPresent | D-07 | PASS |
| TestGenerateTokenWithKid_HeaderPresent | D-07 | PASS |
| TestKeyChain_LatestKidAndSecret | issuer convenience | PASS |

`go test ./shared/auth/... -count=1 -race` exits 0.

## Deviations from Plan

None — plan executed exactly as specified. The PLAN.md action block described adding `os` and `strconv` imports and a `NewKeyChain` explicit constructor; both were included. All 19 tests from PLAN.md `<behavior>` block implemented and passing.

Note: A pre-existing data race in `shared/listener/ring_buffer_test.go:TestRingBufferRetryUsesBackgroundContext` is visible when running `go test ./... -race` across the whole shared module. This race is unrelated to Plan 14-02 changes (only `shared/auth/jwt.go` was modified). Logged in `deferred-items.md`.

## Known Stubs

None. All new functions are fully implemented. The `KeyChain.KeyFunc` dispatches to real secrets; no mock data flows to callers.

## Threat Flags

No new threat surface introduced. All changes are internal to the `shared/auth` package. No new network endpoints, no schema changes, no new auth paths. The `KeyFunc` method closes the alg-confusion attack surface that existed implicitly in the old single-secret inline closures.

## Self-Check: PASSED

- `shared/auth/jwt.go`: EXISTS, modified
- `shared/auth/keychains_test.go`: EXISTS, created
- Test commit `4c835d02` (RED): EXISTS in git log
- Implementation commit `d0032506` (GREEN): EXISTS in git log
- `go test ./shared/auth/... -count=1 -race`: PASS (19/19 tests)
- `make build-all`: PASS (all listener modules)
- api-gateway, auth-service, share-service, overlay-manager, source-manager `go build ./...`: PASS
- `services/overlay-manager/tts/jwt.go`: NOT modified (D-11 honored)
- `grep -c 'token.Header["kid"] = kid' shared/auth/jwt.go` = 5 (≥ 4 required)
