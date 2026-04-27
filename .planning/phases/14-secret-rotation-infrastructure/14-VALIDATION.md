---
phase: 14
slug: secret-rotation-infrastructure
status: planned
nyquist_compliant: true
wave_0_complete: planned-inline
created: 2026-04-27
updated: 2026-04-27
---

# Phase 14 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Source: `14-RESEARCH.md ## Validation Architecture` (line 1022).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` package + `github.com/stretchr/testify` (already in use across codebase) |
| **Config file** | none — standard `go test` |
| **Quick run command** | `go test ./shared/encryption/... ./shared/auth/...` |
| **Full suite command** | `make test` (equivalent to `go test ./...` from repo root) |
| **Estimated runtime** | ~30s quick, ~3-4 min full |

---

## Sampling Rate

- **After every task commit:** Run `go test ./shared/encryption/... ./shared/auth/...` (quick)
- **After every plan wave:** Run `make test` (full)
- **Before `/gsd-verify-work`:** Full suite must be green AND `key-rotator --dry-run` smoke check must succeed
- **Max feedback latency:** 30 seconds (quick suite covers the two shared libraries that hold the rotation primitives)

---

## Per-Task Verification Map

> Every plan task maps to a row below via the test command in its `<verify><automated>` block. Status updates as plans complete.

| Decision | Behavior | Test Type | Automated Command | Plan / Task | Wave | Status |
|----------|----------|-----------|-------------------|-------------|------|--------|
| D-01/D-02 | Versioned ciphertext format `[v(1B)] [nonce(12B)] [ct] [tag(16B)]`; new writes prefix kid byte | unit | `go test ./shared/encryption/... -run 'TestMultiKey'` | 14-01 / Task 1 | 1 | ⬜ pending |
| D-04 | Unified TOKEN_ENCRYPTION_KEY chain decrypts blobs from BOTH legacy `TOKEN_ENCRYPTION_KEY` and `YOUTUBE_TOKEN_ENCRYPTION_KEY` | unit | `go test ./shared/encryption/... -run 'TestUnifiedChain\|TestNewMultiKeyEncryptorFromEnv_YouTubeLegacy'` | 14-01 / Task 1 | 1 | ⬜ pending |
| D-05 | Legacy kid-less ciphertext decrypts via fallback `TOKEN_ENCRYPTION_KEY` env var | unit | `go test ./shared/encryption/... -run 'TestLegacyBackcompat'` | 14-01 / Task 1 | 1 | ⬜ pending |
| D-05 | Legacy ciphertext where `blob[0]` coincidentally equals a registered kid → AEAD fails → retry with legacy succeeds | unit | `go test ./shared/encryption/... -run 'TestFalsePositive\|TestMultiKeyEncryptor_FalsePositiveKid'` | 14-01 / Task 1 | 1 | ⬜ pending |
| D-01 | Golden ciphertexts: fixed test vectors decode correctly forever (regression guard) | unit | `go test ./shared/encryption/... -run 'TestGolden\|TestMultiKeyEncryptor_GoldenV1'` | 14-01 / Task 1 | 1 | ⬜ pending |
| (Plan-14-01 cleanup) | shared/crypto deleted; module-wide build green | build | `cd /home/moersener/Hobby/all-chat && go build ./...` | 14-01 / Task 2 | 1 | ⬜ pending |
| D-07/D-08 | JWT kid header present on every issued token; multi-key validation accepts {V<n-1>, V<n>} | unit | `go test ./shared/auth/... -run 'TestKeyChain\|TestGenerateJWTWithKid'` | 14-02 / Task 1 | 1 | ⬜ pending |
| D-08 | Legacy JWT (no kid claim) validates via fallback `JWT_SECRET` env var | unit | `go test ./shared/auth/... -run 'TestKeyChain_KeyFunc_NoKidUsesLegacy\|TestValidateJWTWithKeyChain_LegacyToken'` | 14-02 / Task 1 | 1 | ⬜ pending |
| D-10 | Service JWT chain (`SERVICE_JWT_SECRET_V<n>`) is independent of user JWT chain (`JWT_SECRET_V<n>`); cross-chain validation fails | unit | `go test ./shared/auth/... -run 'TestValidateServiceJWTWithKeyChain_ChainIsolation\|TestChainIsolation\|TestKeyChain_NewFromEnv_PrefixIsolation'` | 14-02 / Task 1 | 1 | ⬜ pending |
| D-09 | Expired JWT rejected even when kid still present in active validator chain | unit | `go test ./shared/auth/... -run 'TestValidateJWTWithKeyChain_ExpiredKidStillRejects\|TestExpired'` | 14-02 / Task 1 | 1 | ⬜ pending |
| D-12 | Algorithm-confusion attack rejected (alg: none / RS256) | unit | `go test ./shared/auth/... -run 'TestKeyChain_KeyFunc_RejectsNonHMAC'` | 14-02 / Task 1 | 1 | ⬜ pending |
| D-16 | Migrations 050 and 051 add encryption_version columns idempotently | grep | `grep -q 'ADD COLUMN IF NOT EXISTS encryption_version' migrations/050_kick_token_encryption.sql migrations/051_tiktok_token_encryption.sql` | 14-03 / Task 1 | 1 | ⬜ pending |
| D-02/D-04/D-05 | All 5 services (auth, overlay, token-refresh, eventsub, youtube) use *MultiKeyEncryptor; build green; existing tests pass | unit+build | `go build ./services/auth-service/... ./services/overlay-manager/... ./services/token-refresh-service/... ./services/twitch-eventsub-listener/... ./services/youtube-listener/...; go test ./services/auth-service/... ./services/overlay-manager/... ./services/token-refresh-service/... ./services/twitch-eventsub-listener/... ./services/youtube-listener/... -count=1` | 14-04 / Task 1 + Task 2 | 2 | ⬜ pending |
| D-07/D-08/D-10 | All JWT middlewares accept *KeyChain; D-10 isolation enforced at middleware boundary | unit | `go test ./shared/middleware/... -count=1 -run 'TestServiceJWTAuth_ChainIsolation\|TestJWTAuth_KidValidation'` | 14-05 / Task 1 | 2 | ⬜ pending |
| (Plan-14-05 bugfix) | share-service generates Service JWTs with serviceKeyChain (was JWT_SECRET — Pitfall 4) | unit | `go test ./services/share-service/... -run 'TestShares_GenerateServiceJWT_UsesServiceChain\|UsesServiceChain' -count=1` | 14-05 / Task 2 | 2 | ⬜ pending |
| (Plan-14-05 bugfix) | api-gateway /internal route uses serviceKeyChain (was JWT_SECRET) | unit | `go test ./services/api-gateway/... -count=1 -run 'TestInternalRoute\|TestInternalServiceAuth'` | 14-05 / Task 2 | 2 | ⬜ pending |
| D-16 | Kick token encrypted on write, decrypted on read via versioned scheme (encryption_version >= 1 gate) | unit | `go test ./services/kick-listener/... ./services/overlay-manager/... -count=1 -run 'TestKickManager_Decrypt\|TestKickManager_Plaintext\|TestOverlayManagerSources_RoundTrip'` | 14-05 / Task 3 | 2 | ⬜ pending |
| D-17 (partial) | Tiktok token Node.js scope deferral documented; sweeper skips tiktok v0 rows | unit + grep | `go test ./services/auth-service/cmd/key-rotator/... -count=1 -run 'TestSweeper_SkipsTikTokV0'; grep -q 'Node.js' migrations/051_tiktok_token_encryption.sql` | 14-03 + 14-06 | 1+2 | ⬜ pending |
| D-03/D-06 | Sweeper run is idempotent: second run touches 0 rows; resumes after crash | unit | `go test ./services/auth-service/cmd/key-rotator/... -run 'TestSweeper_Idempotent\|TestSweeper_SkipsCurrentKid' -count=1` | 14-06 / Task 1 | 2 | ⬜ pending |
| D-06 | Sweeper telemetry: rows_scanned, rows_re_encrypted, rows_skipped, errors emitted as zap logs | unit | `go test ./services/auth-service/cmd/key-rotator/... -run 'TestSweeper_Telemetry' -count=1` | 14-06 / Task 1 | 2 | ⬜ pending |
| D-06 (Pitfall 5) | Sweeper handles overlay_tts_configs.encrypted_api_key BYTEA correctly | unit | `go test ./services/auth-service/cmd/key-rotator/... -run 'TestSweeper_TTSBytea' -count=1` | 14-06 / Task 1 | 2 | ⬜ pending |
| D-06 | --dry-run logs would-update counts without mutating | unit | `go test ./services/auth-service/cmd/key-rotator/... -run 'TestSweeper_DryRun\|TestMain_FlagsParse' -count=1` | 14-06 / Task 1+2 | 2 | ⬜ pending |
| D-06 | key-rotator binary builds and exits non-zero on missing env | unit | `go test ./services/auth-service/cmd/key-rotator/... -run 'TestMain_RequiresDatabaseURL\|TestMain_RequiresEncryptionKey' -count=1` | 14-06 / Task 2 | 2 | ⬜ pending |
| D-02/D-04/D-08/D-10 | All 12 deployment YAMLs have _V1 entries; Pitfall 1 fixed; YAML parses | grep + yaml | `grep -q 'TOKEN_ENCRYPTION_KEY_V1\|JWT_SECRET_V1\|SERVICE_JWT_SECRET_V1' caesar-deployment/apps/workloads/all-chat/*-deployment.yaml; ! grep -q 'name: ENCRYPTION_KEY$' caesar-deployment/apps/workloads/all-chat/token-refresh-service-deployment.yaml caesar-deployment/apps/workloads/all-chat/twitch-eventsub-listener-deployment.yaml; python3 -c "import yaml, glob; [yaml.safe_load(open(f)) for f in glob.glob('caesar-deployment/apps/workloads/all-chat/*-deployment.yaml')]"` | 14-07 / Task 1+2 | 3 | ⬜ pending |
| D-06 | key-rotator Job + CronJob manifests committed; kustomization registers CronJob | grep + yaml | `grep -q 'kind: Job' caesar-deployment/apps/workloads/all-chat/key-rotator-job.yaml; grep -q 'kind: CronJob' caesar-deployment/apps/workloads/all-chat/key-rotator-cronjob.yaml; grep -q 'key-rotator-cronjob.yaml' caesar-deployment/apps/workloads/all-chat/kustomization.yaml` | 14-07 / Task 3 | 3 | ⬜ pending |
| D-13/D-14/D-15/D-18 | Rotation runbook docs exist with required content; no leaky `-o yaml` patterns | grep | `[ $(wc -l < docs/runbooks/secret-rotation.md) -ge 250 ] && [ $(wc -l < docs/runbooks/db-password-rotation.md) -ge 100 ] && grep -q 'kubectl patch' docs/runbooks/*.md && ! grep -q 'kubectl get secret.*-o yaml' docs/runbooks/*.md` | 14-08 / Task 1+2 | 3 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 — Test File Scaffolding (Inline in Plans)

Wave 0 stubs are NOT a standalone wave for Phase 14 because each test file is created INSIDE the plan that owns it (TDD-mode-OFF style). The scaffolding is created by the FIRST task of the plan that needs it; later tasks of the same plan extend it.

| File | Created by | Coverage |
|------|------------|----------|
| `shared/encryption/versioned_test.go` | 14-01 / Task 1 | D-01, D-02, D-04, D-05 |
| `shared/encryption/testdata/golden_v1.bin` + `golden_legacy.bin` | 14-01 / Task 1 | D-01 (golden regression guard) |
| `shared/auth/keychains_test.go` | 14-02 / Task 1 | D-07, D-08, D-09, D-10, D-12 |
| `services/auth-service/cmd/key-rotator/sweeper_test.go` | 14-06 / Task 1 | D-03, D-06 + Pitfall 5 |
| `services/auth-service/cmd/key-rotator/main_test.go` | 14-06 / Task 2 | D-06 (flag parsing, env preflight) |
| `services/kick-listener/channels/manager_test.go` (encryption tests) | 14-05 / Task 3 | D-16 |
| `services/overlay-manager/handlers/sources_test.go` (encryption round-trip) | 14-05 / Task 3 | D-16 |
| `services/share-service/handlers/shares_test.go` (TestShares_GenerateServiceJWT_UsesServiceChain) | 14-05 / Task 2 | Pitfall 4 regression |
| `shared/middleware/service_auth_test.go` (TestServiceJWTAuth_ChainIsolation) | 14-05 / Task 1 | D-10 enforcement at middleware |

---

## Manual-Only Verifications

| Behavior | Decision | Why Manual | Test Instructions |
|----------|----------|------------|-------------------|
| First production rotation of TOKEN_ENCRYPTION_KEY | D-18 | Touches live K8s secret + lazy re-encrypt under load; cannot be reproduced in CI | Follow `docs/runbooks/secret-rotation.md` §1: kubectl patch adds TOKEN_ENCRYPTION_KEY_V2 → rolling deploy → confirm new writes prefix kid V2 → run sweeper Job → drop legacy env after sweep deadline. Document pod restart counts and any 401s observed. |
| First production rotation of `JWT_SECRET` | D-09 | Requires waiting `T+max(token_TTL)` (24h+) before retiring old kid; cannot be tested in CI | Runbook §2: add `JWT_SECRET_V2` → restart auth-service → wait 24h → confirm only new kid in fresh JWTs → drop old kid from validators. |
| First production rotation of DB password (CNPG fallback runbook) | D-14 | Production DB; `ALTER ROLE` is atomic and cannot be replayed safely in CI | Runbook `db-password-rotation.md` 7-step procedure — exec into CNPG primary, `ALTER ROLE`, `kubectl patch`, rolling restart of all DB-consuming services, verify zero connection failures, drop old password from secret. |
| `kubectl get secret allchat-secrets` inspection without value leakage | global feedback memory | Per the `kubectl_secret_yaml` memory: never `-o yaml` (leaks via `last-applied-configuration`) | Use `kubectl get secret allchat-secrets -o jsonpath='{.data}' \| jq 'keys'` for key names only. Never include values in commit messages, logs, or screenshots. |

---

## Validation Sign-Off

- [x] All Wave 1 + 2 tasks have `<automated>` verify commands or are gated on a Wave 0 test file in the table above
- [x] Sampling continuity: no 3 consecutive tasks without automated verify (sweeper integration tests in Plan 14-06 each have explicit Test* functions)
- [x] Wave 0 covers all MISSING references: versioned_test.go, keychains_test.go, key-rotator main_test.go + sweeper_test.go, kick encrypt-test, share-service regression test, middleware chain-isolation test
- [x] No watch-mode flags (-count=1 used for cache-busting where mutation tests need it)
- [x] Feedback latency < 30s for quick suite, < 4 min for full suite
- [x] `nyquist_compliant: true` set in frontmatter once planner confirms every task maps to a row above

**Approval:** approved-by-planner @ 2026-04-27 (8 plans / 3 waves / all decisions D-01..D-20 mapped).
