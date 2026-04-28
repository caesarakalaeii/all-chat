---
phase: 14-secret-rotation-infrastructure
verified: 2026-04-28T08:57:00Z
status: passed
score: 12/12 must-haves verified
overrides_applied: 0
---

# Phase 14: Secret Rotation Infrastructure — Verification Report

**Phase Goal:** Following a secret leak, design and build a rotation mechanism (or per-secret-type set of mechanisms) into the platform — covering DB password (CNPG), JWT signing keys, and the AES-GCM TOKEN\_ENCRYPTION\_KEY used for OAuth access/refresh tokens stored encrypted in the database. Must minimize impact on running services and existing encrypted DB values during rotation, and support repeatable rotation going forward (not a one-shot fix).

**Verified:** 2026-04-28T08:57:00Z
**Status:** PASSED
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| #  | Truth                                                                                       | Status     | Evidence                                                                                                                         |
|----|---------------------------------------------------------------------------------------------|------------|----------------------------------------------------------------------------------------------------------------------------------|
| 1  | TOKEN\_ENCRYPTION\_KEY rotation mechanism: MultiKeyEncryptor + kid-byte format exists       | VERIFIED   | `shared/encryption/versioned.go` — full D-01/D-02 implementation with `[kid(1B)][nonce(12B)][ct][tag(16B)]` wire format          |
| 2  | Legacy ciphertext (no kid) decrypts via TOKEN\_ENCRYPTION\_KEY + YOUTUBE\_TOKEN\_ENCRYPTION\_KEY fallback | VERIFIED   | D-04/D-05 fallback chain in `DecryptString`; `TestMultiKeyEncryptor_LegacyBackcompat`, `TestMultiKeyEncryptor_UnifiedChain` pass |
| 3  | False-positive kid byte handled gracefully (AEAD fail → legacy fallback)                   | VERIFIED   | `TestMultiKeyEncryptor_FalsePositiveKid` passes; code falls through when AEAD rejects the kid path                               |
| 4  | Sweeper re-encrypts all 6 tables; idempotent (2nd run touches 0 rows)                      | VERIFIED   | `TestSweeper_Idempotent` passes: 2nd run shows `rows_re_encrypted=0, rows_skipped=2` on users; all 6 table sweeps confirmed       |
| 5  | Sweeper has dry-run mode; handles BYTEA overlay\_tts\_configs correctly (Pitfall 5)         | VERIFIED   | `TestSweeper_DryRun` passes; `TestSweeper_TTSBytea` passes; `string(b)` / `[]byte(newStored)` BYTEA handling verified in sweeper.go |
| 6  | Sweeper skips tiktok v0 rows; encrypts kick v0 rows directly                               | VERIFIED   | `TestSweeper_SkipsTikTokV0` passes; SQL `WHERE encryption_version >= 1` in `sweepTikTokOAuthTokens`; kick v0 encrypt-direct in `sweepKickOAuthTokens` |
| 7  | JWT KeyChain + kid header on all issued JWTs; multi-key validation                         | VERIFIED   | `shared/auth/jwt.go` has full KeyChain, all 5 `*WithKid` issuers, 3 `*WithKeyChain` validators; all auth tests pass             |
| 8  | Two independent JWT chains; cross-chain validation rejected (D-10)                         | VERIFIED   | `TestValidateServiceJWTWithKeyChain_ChainIsolation` passes; `TestServiceJWTAuth_ChainIsolation` passes; `TestKeyChain_NewFromEnv_PrefixIsolation` passes |
| 9  | alg-confusion rejected (D-12); expired tokens rejected even with valid kid (D-09)          | VERIFIED   | `TestKeyChain_KeyFunc_RejectsNonHMAC` passes; `TestValidateJWTWithKeyChain_ExpiredKidStillRejects` passes                        |
| 10 | All call sites use WithKeyChain validators + WithKid issuers; Pitfall 4 + parallel bug fixed | VERIFIED   | share-service uses `serviceKeyChain.LatestKid()` for ServiceJWT; api-gateway wires `serviceKeyChain` to `/internal` route; all services build cleanly |
| 11 | DB password rotation runbook exists, concrete, uses kubectl patch, no -o yaml              | VERIFIED   | `docs/runbooks/db-password-rotation.md` (379 lines); `docs/runbooks/secret-rotation.md` (660 lines); grep confirms `kubectl patch` present, no `-o yaml` |
| 12 | K8s manifests wire V1 env entries; Pitfall 1 fix (ENCRYPTION\_KEY → TOKEN\_ENCRYPTION\_KEY) | VERIFIED   | 13/19 deployment YAMLs have V1 keys (the 6 without are frontend/redis/emote/message-processor/discord-bot/support-bot which handle no encrypted tokens or JWTs); old `name: ENCRYPTION_KEY` absent from token-refresh-service and twitch-eventsub-listener |

**Score:** 12/12 truths verified

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `shared/encryption/versioned.go` | MultiKeyEncryptor with kid-byte AES-GCM | VERIFIED | 293 lines; D-01/D-02/D-04/D-05 fully implemented |
| `shared/auth/jwt.go` | KeyChain + WithKid issuers + WithKeyChain validators | VERIFIED | Full implementation; 5 WithKid issuers, 3 WithKeyChain validators, KeyChain.KeyFunc |
| `services/auth-service/cmd/key-rotator/main.go` | Sweeper binary entry point | VERIFIED | parseFlags + run + idempotent SweepAll; validates DATABASE\_URL env |
| `services/auth-service/cmd/key-rotator/sweeper.go` | 6-table idempotent re-encryption sweeper | VERIFIED | All 6 tables implemented: users, viewer\_sessions, youtube\_oauth\_tokens, overlay\_tts\_configs (BYTEA), kick\_oauth\_tokens (v0 encrypt-direct), tiktok\_oauth\_tokens (v0 SQL-skipped) |
| `services/auth-service/Dockerfile` | Multi-binary build (auth-service + key-rotator) | VERIFIED | Both binaries built via `go build -o /app/key-rotator ./cmd/key-rotator` |
| `migrations/050_kick_token_encryption.sql` | encryption\_version column for kick | VERIFIED | `ADD COLUMN IF NOT EXISTS encryption_version SMALLINT NOT NULL DEFAULT 0` + index |
| `migrations/051_tiktok_token_encryption.sql` | encryption\_version column for tiktok with Node.js deferral documented | VERIFIED | Same schema; SQL comment documents deferral rationale in detail |
| `docs/runbooks/secret-rotation.md` | Rotation runbook (TOKEN\_ENCRYPTION\_KEY, JWT\_SECRET, SERVICE\_JWT\_SECRET) | VERIFIED | 660 lines; SOPS hazard documented; kubectl patch only; no -o yaml; TTL windows documented |
| `docs/runbooks/db-password-rotation.md` | DB password rotation runbook (CNPG fallback + 7-step) | VERIFIED | 379 lines; explains why ManagedRoles is unsuitable; kubectl patch sequence; rollback path |
| `caesar-deployment/.../key-rotator-job.yaml` | K8s Job manifest for manual runs | VERIFIED | kind: Job; not in kustomization (template pattern per design) |
| `caesar-deployment/.../key-rotator-cronjob.yaml` | Weekly CronJob for scheduled sweeps | VERIFIED | kind: CronJob; schedule: Sundays 03:00 UTC; concurrencyPolicy: Forbid |
| `caesar-deployment/.../kustomization.yaml` | CronJob registered | VERIFIED | `key-rotator-cronjob.yaml` present in kustomization |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| auth-service/handlers | MultiKeyEncryptor | `encryption.NewMultiKeyEncryptorFromEnv()` | WIRED | All handlers use `h.cipher.EncryptString/DecryptString`; `userKeyChain.LatestKid()/LatestSecret()` for issuance |
| kick-listener/channels/manager.go | MultiKeyEncryptor | `decryptKickToken(token, encryptionVersion)` | WIRED | `encryption_version >= 1` gate on read; plaintext passthrough for v0 |
| overlay-manager/handlers/sources.go | MultiKeyEncryptor | `h.cipher.EncryptString()` + `encryption_version=1` | WIRED | Writes encrypted with v1; reads decrypt via same cipher; explicit nil-cipher guard |
| api-gateway | service KeyChain | `sharedmiddleware.ServiceJWTAuth(serviceKeyChain, ...)` | WIRED | `/internal` route group uses SERVICE\_JWT\_SECRET chain (Pitfall 1 parallel bug fixed) |
| share-service/handlers | service KeyChain | `auth.GenerateServiceJWTWithKid(h.serviceKeyChain.LatestKid(), ...)` | WIRED | Pitfall 4 fixed — was `h.jwtSecret`, now `h.serviceKeyChain` |
| Sweeper | 6 DB tables | `pgx.Batch` UPDATE per table | WIRED | encryptIfNotCurrentKid helper; batch writes confirmed via TestSweeper\_Telemetry |
| key-rotator-cronjob | auth-service image | `command: ["/app/key-rotator"]` | WIRED | K8s Job overrides CMD with the sweeper binary path |

---

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|-------------------|--------|
| `sweeper.go / sweepUsers` | users.access\_token, refresh\_token | PostgreSQL `users` table via pgx.Pool | Yes — real pgx query + batch UPDATE | FLOWING |
| `sweeper.go / sweepKickOAuthTokens` | kick\_oauth\_tokens.access\_token | PostgreSQL `kick_oauth_tokens` with encryption\_version gate | Yes — v0 encrypt-direct; v1+ via encryptIfNotCurrentKid | FLOWING |
| `sweeper.go / sweepOverlayTTSConfigs` | overlay\_tts\_configs.encrypted\_api\_key | PostgreSQL BYTEA column | Yes — BYTEA→string→encryptIfNotCurrentKid→[]byte round-trip | FLOWING |
| `shared/auth/jwt.go / KeyChain.KeyFunc` | kid header | JWT token header map | Yes — `token.Header["kid"].(string)` dispatch to byKid map | FLOWING |

---

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Encryption tests pass (D-01/D-02/D-04/D-05/D-12 golden) | `go test ./encryption/... -count=1 -run 'TestMultiKey\|...'` | ok 0.003s | PASS |
| Auth tests pass (D-07/D-08/D-09/D-10/D-12) | `go test ./auth/... -count=1 -run 'TestKeyChain\|...'` | ok 0.004s | PASS |
| Sweeper integration tests pass (real Postgres via testcontainers) | `go test ./cmd/key-rotator/... -count=1` | ok 20.689s | PASS |
| Sweeper idempotent (2nd run 0 rows) | `TestSweeper_Idempotent` | PASS (run2: rows\_re\_encrypted=0) | PASS |
| Sweeper dry-run (no mutation) | `TestSweeper_DryRun` | PASS | PASS |
| Sweeper skips TikTok v0 rows | `TestSweeper_SkipsTikTokV0` | PASS | PASS |
| Sweeper handles BYTEA overlay\_tts\_configs | `TestSweeper_TTSBytea` | PASS | PASS |
| Middleware chain isolation (D-10) | `TestServiceJWTAuth_ChainIsolation` | PASS | PASS |
| alg-confusion rejected (D-12) | `TestKeyChain_KeyFunc_RejectsNonHMAC` | PASS | PASS |
| Expired token rejected even with valid kid (D-09) | `TestValidateJWTWithKeyChain_ExpiredKidStillRejects` | PASS | PASS |
| Migrations have `ADD COLUMN IF NOT EXISTS encryption_version` | grep | PASS | PASS |
| Dockerfile builds key-rotator binary | grep | PASS | PASS |
| TOKEN\_ENCRYPTION\_KEY\_V1 present in token-refresh-service deployment | grep count=1 | 1 | PASS |
| Old `name: ENCRYPTION_KEY` absent from Pitfall 1 deployments | grep | not found | PASS |
| shared/crypto deleted | `test ! -d shared/crypto` | PASS | PASS |
| All deployment YAMLs parse as valid YAML | python3 yaml.safe\_load\_all | PASS | PASS |
| CronJob registered in kustomization | grep | PASS | PASS |
| kubectl patch in runbooks, no -o yaml | grep | PASS | PASS |
| All relevant services build cleanly | go build per service | Exit 0 for all 8 services | PASS |

---

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| D-01 (kid wire format) | 14-01 | `[v(1B)][nonce(12B)][ct][tag(16B)]` format | SATISFIED | versioned.go; TestMultiKeyEncryptor\_GoldenV1 |
| D-02 (multi-key env) | 14-01, 14-04 | TOKEN\_ENCRYPTION\_KEY\_V1..Vn env chain | SATISFIED | NewMultiKeyEncryptorFromEnv; all services use it |
| D-03 (lazy + sweeper) | 14-06 | Scheduled background re-encryption; idempotent | SATISFIED | sweeper.go; TestSweeper\_Idempotent |
| D-04 (unified chain) | 14-01 | YOUTUBE\_TOKEN\_ENCRYPTION\_KEY as legacy fallback | SATISFIED | legacyKeys second entry; TestMultiKeyEncryptor\_UnifiedChain |
| D-05 (legacy backcompat) | 14-01 | Kid-less ciphertext decrypts via TOKEN\_ENCRYPTION\_KEY | SATISFIED | DecryptString fallback; TestMultiKeyEncryptor\_LegacyBackcompat |
| D-06 (sweeper as cmd) | 14-06, 14-07 | key-rotator binary; Job + CronJob manifests | SATISFIED | cmd/key-rotator; Dockerfile; key-rotator-*.yaml; kustomization |
| D-07 (kid header issuance) | 14-02, 14-05 | kid header on all issued JWTs | SATISFIED | 5 WithKid functions; all auth-service handlers use them |
| D-08 (multi-key validation) | 14-02, 14-05 | Validators accept JWT\_SECRET\_V1..Vn + legacy | SATISFIED | KeyChain.KeyFunc; all middlewares migrated |
| D-09 (expired rejection) | 14-02 | Expired JWT rejected even with valid kid | SATISFIED | TestValidateJWTWithKeyChain\_ExpiredKidStillRejects |
| D-10 (chain isolation) | 14-02, 14-05 | SERVICE\_JWT\_SECRET chain separate from JWT\_SECRET | SATISFIED | prefix-based NewKeyChainFromEnv; TestValidateServiceJWTWithKeyChain\_ChainIsolation |
| D-11 (TTS JWT untouched) | 14-CONTEXT | overlay-manager/tts/jwt.go not modified | SATISFIED | Confirmed: tts/jwt.go has no KeyChain references |
| D-12 (alg-confusion) | 14-02 | Non-HMAC alg rejected in KeyFunc | SATISFIED | TestKeyChain\_KeyFunc\_RejectsNonHMAC |
| D-13/D-14 (CNPG fallback) | 14-08 | Manual ALTER ROLE runbook (ManagedRoles unsuitable) | SATISFIED | db-password-rotation.md explains failure criteria |
| D-15 (kubectl patch only) | 14-08 | No sops set; kubectl patch for K8s Secret edits | SATISFIED | Both runbooks document this explicitly; grep confirms no -o yaml |
| D-16 (kick encryption) | 14-03, 14-05 | kick\_oauth\_tokens encrypted on write, decrypted on read | SATISFIED | migration 050; overlay-manager/handlers/sources.go; kick-listener/channels/manager.go |
| D-17 (tiktok partial) | 14-03, 14-06 | schema column shipped; Node.js code deferred; sweeper skips v0 | SATISFIED (intentional partial) | migration 051 with Node.js deferral documented; sweeper SQL filter `encryption_version >= 1` |
| D-18/D-19 (first rotation is mechanism run) | 14-08 | Mechanism ships; first post-Phase-14 run IS the remediation | SATISFIED | Runbooks document this explicitly; keys in K8s Secret are operator responsibility |

---

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `shared/auth/jwt.go` | 89-156 | Legacy `GenerateJWT`, `GenerateToken`, `GenerateServiceJWT`, `GenerateImpersonationJWT` functions still present alongside new WithKid variants | Info | Not a stub — they remain for backwards-compat call sites not yet migrated; all active issuance paths (auth-service handlers) already use WithKid variants; legacy functions compile and are tested |
| `services/twitch-eventsub-listener/cmd/main.go` | 132-137 | Code comment says "deployment manifest currently mounts ENCRYPTION\_KEY; Plan 14-07 renames it to TOKEN\_ENCRYPTION\_KEY" — stale comment if Plan 14-07 already fixed the YAML | Info | YAML is fixed (verified: old name absent); code comment is stale but does not affect behaviour |

Neither is a blocker. The legacy JWT issuance functions are intentional API surface for gradual migration; all active call sites use WithKid.

---

### Human Verification Required

None. All must-haves are mechanically verifiable and all checks passed.

---

### Security Check Results

| Check | Status | Evidence |
|-------|--------|----------|
| alg-confusion (D-12): KeyFunc rejects non-HMAC alg | PASS | `TestKeyChain_KeyFunc_RejectsNonHMAC` passes; code: `if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok { return nil, ... }` |
| D-10 chain isolation at middleware boundary | PASS | `TestServiceJWTAuth_ChainIsolation` passes; api-gateway wires `serviceKeyChain` to /internal; share-service uses `serviceKeyChain` for issuance |
| D-09: expired JWTs rejected even when kid valid | PASS | `TestValidateJWTWithKeyChain_ExpiredKidStillRejects` passes |
| D-05: AEAD failure on kid-prefixed read falls back to legacy | PASS | `TestMultiKeyEncryptor_FalsePositiveKid` passes; code falls through on AEAD error |
| No `sops set` in runbooks | PASS | Explicitly prohibited; grep confirms no `sops set` in either runbook |
| No `kubectl get secret.*-o yaml` in runbooks | PASS | grep: clean |
| No plaintext secret values in code/commits | PASS | Only secretKeyRef references in YAMLs; no hardcoded secrets found |
| D-16: tiktok plaintext not broken by sweeper | PASS | SQL filter `WHERE encryption_version >= 1` protects all running v0 rows |

---

## Gaps Summary

No gaps. All 4 contract elements (TOKEN\_ENCRYPTION\_KEY rotation, JWT rotation, DB password rotation, encryption gap-fill) are fully delivered and verified.

**Intentional partial delivery (documented, not gaps):**
- D-17 (TikTok Node.js code-side encryption): Schema column shipped; Node.js encrypt-on-write deferred to a follow-up phase. Sweeper safely skips v0 rows. Rationale documented in migration 051 SQL comments, STATE.md decisions, and 14-CONTEXT.md.
- D-18/D-19 (actual key rotation): Phase 14 ships the mechanism. First use of that mechanism (running the runbook against production) is the operator's responsibility, intentionally deferred per the context decisions.

---

_Verified: 2026-04-28T08:57:00Z_
_Verifier: Claude (gsd-verifier)_
