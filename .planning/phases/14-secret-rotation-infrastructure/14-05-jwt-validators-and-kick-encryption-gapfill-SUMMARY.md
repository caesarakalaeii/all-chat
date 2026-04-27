---
phase: 14-secret-rotation-infrastructure
plan: "05"
subsystem: shared-auth, api-gateway, auth-service, share-service, overlay-manager, source-manager, kick-listener
tags: [jwt, keychain, encryption, security, aes-gcm, kid-dispatch, chain-isolation]
dependency_graph:
  requires:
    - "14-02 (KeyChain + *WithKid auth functions)"
    - "14-03 (kick_oauth_tokens schema has encryption_version column)"
    - "14-04 (YouTube/TikTok callsite migration pattern reference)"
  provides:
    - "JWTAuth(kc *auth.KeyChain) shared middleware"
    - "ServiceJWTAuth(kc *auth.KeyChain) shared middleware"
    - "All JWT issuance uses *WithKid variants"
    - "D-10 chain isolation enforced at every callsite"
    - "kick_oauth_tokens encrypted on write, decrypted on read (D-16)"
  affects:
    - "14-06 (key-rotator needs chain isolation to be correct — now enforced)"
    - "14-07 (deployment manifests must set JWT_SECRET_V1 + SERVICE_JWT_SECRET_V1)"
tech_stack:
  added:
    - "encryption.MultiKeyEncryptor cipher injected into channels.Manager and SourcesHandler"
  patterns:
    - "KeyChain dispatcher: kid header → key lookup, legacy fallback for tokens without kid (D-08)"
    - "D-10 chain isolation: JWT_SECRET chain validates user/viewer tokens; SERVICE_JWT_SECRET validates service tokens — never cross-validated"
    - "D-16 write-encrypt/read-decrypt: overlay-manager encrypts kick access_token on copy, kick-listener decrypts before use"
key_files:
  created:
    - services/kick-listener/channels/encryption_test.go
  modified:
    - shared/middleware/auth.go
    - shared/middleware/auth_test.go
    - shared/middleware/service_auth.go
    - shared/middleware/service_auth_test.go
    - services/api-gateway/middleware/auth.go
    - services/api-gateway/middleware/viewer_auth.go
    - services/api-gateway/middleware/auth_test.go
    - services/api-gateway/handlers/websocket.go
    - services/api-gateway/handlers/websocket_viewer.go
    - services/api-gateway/cmd/main.go
    - services/auth-service/cmd/main.go
    - services/auth-service/handlers/auth_handler.go
    - services/auth-service/handlers/viewer_auth.go
    - services/auth-service/handlers/admin.go
    - services/auth-service/handlers/platform_auth.go
    - services/auth-service/handlers/platform_auth_v2.go
    - services/auth-service/handlers/auth_test.go
    - services/auth-service/handlers/viewer_exchange_test.go
    - services/share-service/cmd/main.go
    - services/share-service/handlers/shares.go
    - services/share-service/handlers/shares_test.go
    - services/overlay-manager/cmd/main.go
    - services/overlay-manager/handlers/sources.go
    - services/source-manager/cmd/main.go
    - services/kick-listener/cmd/main.go
    - services/kick-listener/channels/manager.go
decisions:
  - "D-07/D-08: Middleware signature changed from secret string to *auth.KeyChain; kid-dispatch applied at every validation callsite"
  - "D-10 enforced: api-gateway /internal route group now uses serviceKeyChain (was jwtSecret — silent cross-chain acceptance bug)"
  - "D-10 enforced: share-service GenerateServiceJWT now uses serviceKeyChain (was h.jwtSecret — Pitfall 4 security bug)"
  - "D-16: kick_oauth_tokens encrypted on write (encryption_version=1) by overlay-manager; decrypted on read by kick-listener"
  - "cipher is optional in kick-listener (warn+continue when TOKEN_ENCRYPTION_KEY_V1 absent) to avoid blocking startup in envs without encryption keys"
metrics:
  duration: "multi-session (session 1: Tasks 1-2; session 2: Task 2 commit + Task 3)"
  completed: "2026-04-27"
  tasks_completed: 3
  files_changed: 28
---

# Phase 14 Plan 05: JWT Validators and Kick Encryption Gapfill Summary

Migrated all JWT validator middleware from raw `secret string` signatures to `*auth.KeyChain`, fixed two cross-chain security bugs (share-service Pitfall 4 and api-gateway internal route parallel bug), and added AES-GCM encryption for kick_oauth_tokens on the overlay-manager write path and kick-listener read path.

## Tasks Completed

### Task 1: Migrate shared and api-gateway middleware to *auth.KeyChain

**Commit:** `4530ef62`

- `shared/middleware/auth.go`: `JWTAuth(secret string)` → `JWTAuth(kc *auth.KeyChain)`; validation via `ValidateJWTWithKeyChain` and `ValidateViewerJWTWithKeyChain`
- `shared/middleware/service_auth.go`: `ServiceJWTAuth(secret string, ...)` → `ServiceJWTAuth(kc *auth.KeyChain, ...)`; validation via `ValidateServiceJWTWithKeyChain`
- `services/api-gateway/middleware/auth.go`: `JWTAuth(jwtSecret string)` → `JWTAuth(kc *auth.KeyChain)`
- `services/api-gateway/middleware/viewer_auth.go`: `ViewerJWTAuth(jwtSecret string)` → `ViewerJWTAuth(kc *auth.KeyChain)`
- Added `TestServiceJWTAuth_ChainIsolation` (D-10 regression) and `TestJWTAuth_KidValidation` (D-08 regression)

### Task 2: Wire KeyChains through all five services; fix cross-chain bugs

**Commit:** `b1c470a8` (recorded as `ee5c618d` at time of commit — both refer to same object)

**Two security bugfixes:**
- **Pitfall 4 (share-service):** `GenerateServiceJWT("share-service", h.jwtSecret, ...)` → `GenerateServiceJWTWithKid(h.serviceKeyChain.LatestKid(), "share-service", string(h.serviceKeyChain.LatestSecret()), ...)`. Previously the service JWT was signed with the user chain secret, making it rejected by the api-gateway /internal route validator (which correctly uses the service chain).
- **api-gateway parallel bug (line 564):** `ServiceJWTAuth(jwtSecret, ...)` → `ServiceJWTAuth(serviceKeyChain, ...)` on the `/internal` route group. Previously user-chain tokens could pass service auth validation.

**Services wired:**
- api-gateway: dual `userKeyChain` + `serviceKeyChain` from env; WebSocket handlers use `ValidateJWTWithKeyChain`
- auth-service: 5 handler files migrated from `jwtSecret string` → `userKeyChain *auth.KeyChain`; all 3x `GenerateToken` → `GenerateTokenWithKid`; impersonation, platform_auth, platform_auth_v2, viewer_auth migrated
- share-service: `serviceKeyChain` from `SERVICE_JWT_SECRET`, `userKeyChain` from `JWT_SECRET`
- overlay-manager: `userKeyChain` from `JWT_SECRET`, passed to both protected route groups
- source-manager: `serviceKeyChain` from `SERVICE_JWT_SECRET`, passed to `ServiceJWTAuth`

**Regression tests:**
- `TestInternalServiceAuth` — service-chain token accepted; user-chain token rejected on /internal
- `TestShares_GenerateServiceJWT_UsesServiceChain` — Pitfall 4 regression

### Task 3: kick_oauth_tokens encryption (D-16)

**Commit:** `7b9b9757`

- `kick-listener/channels/manager.go`: added `cipher *encryption.MultiKeyEncryptor` field; `NewManager` gains `cipher` parameter; `getKickAuthToken` SELECTs `encryption_version`, calls `decryptKickToken`; `decryptKickToken` helper: version=0 → passthrough, version>=1 → AES-GCM decrypt via cipher
- `kick-listener/cmd/main.go`: optional cipher init from `TOKEN_ENCRYPTION_KEY_V1` (warn on absence, not fatal — preserves startup in envs without encryption configured)
- `overlay-manager/handlers/sources.go`: `cipher *encryption.MultiKeyEncryptor` added to `SourcesHandler`; `NewSourcesHandler` gains `cipher` parameter; `copyKickTokenForChannel` decrypts source row if `encryption_version>=1`, re-encrypts destination with `encryption_version=1` when cipher is available
- `overlay-manager/cmd/main.go`: passes existing `tokenCipher` to `NewSourcesHandler`

**Tests added:** `TestDecryptKickToken_VersionedDecrypts`, `TestDecryptKickToken_PlaintextLegacy`, `TestDecryptKickToken_NilCipherVersioned`

## Commits

| Hash | Type | Description |
|------|------|-------------|
| `4530ef62` | refactor | migrate shared/middleware JWT validators to KeyChain (D-07/D-08/D-10/D-12) |
| `ee5c618d` | fix | share-service ServiceJWT issuance + api-gateway internal route use service KeyChain (Pitfall 4) |
| `7b9b9757` | feat | kick-listener decrypts kick_oauth_tokens on read; overlay-manager encrypts on write (D-16) |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] auth-service had 5 handler files using `jwtSecret`, not 2**
- **Found during:** Task 2 implementation
- **Issue:** Plan listed `auth_handler.go` and `viewer_auth.go` as the only files to migrate, but `admin.go`, `platform_auth.go`, and `platform_auth_v2.go` also had `jwtSecret string` fields and `GenerateToken` calls. cmd/main.go passes `userKeyChain` to all handlers, so all 5 needed updating.
- **Fix:** Migrated all 5 files (`auth_handler.go`, `viewer_auth.go`, `admin.go`, `platform_auth.go`, `platform_auth_v2.go`) to `userKeyChain *auth.KeyChain`
- **Files modified:** services/auth-service/handlers/admin.go, platform_auth.go, platform_auth_v2.go
- **Commit:** `ee5c618d`

**2. [Rule 1 - Bug] auth-service test files broke after constructor signature change**
- **Found during:** Task 2 verification
- **Issue:** `auth_test.go` passed raw `"test-jwt-secret"` string to `NewAuthHandler`; `viewer_exchange_test.go` used `jwtSecret:` struct field — both now type errors
- **Fix:** Added `testUserKeyChain(secret string) *auth.KeyChain` helper to `auth_test.go`; updated all 3 constructor calls and struct literal in `viewer_exchange_test.go`
- **Files modified:** services/auth-service/handlers/auth_test.go, viewer_exchange_test.go
- **Commit:** `ee5c618d`

## Known Stubs

None — all kick token paths either encrypt/decrypt or pass through with documented version=0 legacy behavior.

## Threat Flags

None — the changes close existing threat surface (cross-chain token acceptance, plaintext kick OAuth tokens at rest) rather than introducing new surface.

## Self-Check: PASSED

All 9 key files found. All 3 task commits verified in git history.
