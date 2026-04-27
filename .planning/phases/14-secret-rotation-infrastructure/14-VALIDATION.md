---
phase: 14
slug: secret-rotation-infrastructure
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-27
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

> Filled by planner. Each plan task should map back to one of the rows below. Status updates as plans complete.

| Decision | Behavior | Test Type | Automated Command | Wave | Status |
|----------|----------|-----------|-------------------|------|--------|
| D-01/D-02 | Versioned ciphertext format `[v(1B)] [nonce(12B)] [ct] [tag(16B)]`; new writes prefix kid byte | unit | `go test ./shared/encryption/... -run TestMultiKey` | 1 | ⬜ pending |
| D-04 | Unified TOKEN_ENCRYPTION_KEY chain decrypts both legacy `TOKEN_ENCRYPTION_KEY` and `YOUTUBE_TOKEN_ENCRYPTION_KEY` ciphertext | unit | `go test ./shared/encryption/... -run TestUnifiedChain` | 1 | ⬜ pending |
| D-05 | Legacy kid-less ciphertext decrypts via fallback `TOKEN_ENCRYPTION_KEY` env var | unit | `go test ./shared/encryption/... -run TestLegacyBackcompat` | 1 | ⬜ pending |
| D-05 | Legacy ciphertext where `blob[0]` coincidentally equals a registered kid → AEAD fails → retry with legacy succeeds | unit | `go test ./shared/encryption/... -run TestFalsePositive` | 1 | ⬜ pending |
| D-01 | Golden ciphertexts: fixed test vectors decode correctly forever (regression guard) | unit | `go test ./shared/encryption/... -run TestGolden` | 1 | ⬜ pending |
| D-07/D-08 | JWT kid header present on every issued token; multi-key validation accepts {V<n-1>, V<n>} | unit | `go test ./shared/auth/... -run TestKeyChain` | 1 | ⬜ pending |
| D-08 | Legacy JWT (no kid claim) validates via fallback `JWT_SECRET` env var | unit | `go test ./shared/auth/... -run TestLegacyFallback` | 1 | ⬜ pending |
| D-10 | Service JWT chain (`SERVICE_JWT_SECRET_V<n>`) is independent of user JWT chain (`JWT_SECRET_V<n>`) — cross-chain validation fails | unit | `go test ./shared/auth/... -run TestChainIsolation` | 1 | ⬜ pending |
| D-09 | Expired JWT rejected even when kid still present in active validator chain | unit | `go test ./shared/auth/... -run TestExpiredKidStillRejects` | 1 | ⬜ pending |
| D-16 | Kick token encrypted on write, decrypted on read via versioned scheme | unit | `go test ./services/kick-listener/...` | 2 | ⬜ pending |
| D-17 | Tiktok token encrypted on write, decrypted on read via versioned scheme (Node.js path — see Open Question 1 in research) | unit/integration | TBD by planner — Go test if shim, or `npm test` in tiktok service | 2 | ⬜ pending |
| D-03/D-06 | Sweeper run is idempotent: second run touches 0 rows; resumes after crash | integration | `go test ./services/auth-service/cmd/key-rotator/... -run TestSweeper_Idempotent` | 2 | ⬜ pending |
| D-06 | Sweeper telemetry: `rows_scanned`, `rows_re_encrypted_per_kid`, `errors` emitted as zap logs | integration | `go test ./services/auth-service/cmd/key-rotator/... -run TestSweeper_Telemetry` | 2 | ⬜ pending |
| D-13/D-14 | DB password rotation runbook executes cleanly in dry-run (no destructive ops; `kubectl patch` preview only) | manual | runbook checklist + `key-rotator --dry-run` | 3 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Tests that need scaffolding before any task in Wave 1 can declare its acceptance via the test command above.

- [ ] `shared/encryption/versioned_test.go` — covers D-01, D-02, D-04, D-05; includes golden ciphertext fixtures (`testdata/golden_v0.bin`, `testdata/golden_v1.bin`).
- [ ] `shared/auth/keychains_test.go` — covers D-07, D-08, D-09, D-10; user + service chain isolation.
- [ ] `services/auth-service/cmd/key-rotator/main_test.go` — sweeper idempotency + telemetry.
- [ ] `services/kick-listener/<encrypt-test>.go` — kick encrypt/decrypt round-trip (D-16).

*Framework already installed (Go testing + testify). No `go install` needed in Wave 0.*

---

## Manual-Only Verifications

| Behavior | Decision | Why Manual | Test Instructions |
|----------|----------|------------|-------------------|
| First production rotation of TOKEN_ENCRYPTION_KEY | D-18 | Touches live K8s secret + lazy re-encrypt under load; cannot be reproduced in CI | Follow runbook: `kubectl patch` adds `TOKEN_ENCRYPTION_KEY_V1` next to legacy → rolling deploy of all consumers → confirm new writes prefix kid byte → run sweeper Job → drop legacy env after sweep deadline. Document pod restart counts and any 401s observed. |
| First production rotation of `JWT_SECRET` | D-09 | Requires waiting `T+max(token_TTL)` (24h+) before retiring old kid; cannot be tested in CI | Runbook: add `JWT_SECRET_V<n+1>` → restart auth-service → wait 24h → confirm only new kid in fresh JWTs → drop old kid from validators. |
| First production rotation of DB password (CNPG fallback runbook) | D-14 | Production DB; `ALTER ROLE` is atomic and cannot be replayed safely in CI | 7-step runbook from RESEARCH.md §3.1 — exec into CNPG primary, `ALTER ROLE`, `kubectl patch`, rolling restart of all DB-consuming services, verify zero connection failures, drop old password from secret. |
| `kubectl get secret allchat-secrets` inspection without value leakage | global feedback memory | Per the `kubectl_secret_yaml` memory: never `-o yaml` (leaks via `last-applied-configuration`) | Use `kubectl get secret allchat-secrets -o jsonpath='{.data}' \| jq 'keys'` for key names only. Never include values in commit messages, logs, or screenshots. |

---

## Validation Sign-Off

- [ ] All Wave 1 + 2 tasks have `<automated>` verify commands or are gated on a Wave 0 file in the table above
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify (sweeper integration tests are the only place this risks; planner must check)
- [ ] Wave 0 covers all MISSING references: `versioned_test.go`, `keychains_test.go`, `key-rotator main_test.go`, `kick encrypt-test.go`
- [ ] No watch-mode flags (`-count=1` for cache-busting only when needed)
- [ ] Feedback latency < 30s for quick suite, < 4 min for full suite
- [ ] `nyquist_compliant: true` set in frontmatter once planner confirms every task maps to a row above

**Approval:** pending
