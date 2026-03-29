---
phase: 7
slug: feature-gate-infrastructure
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-29
---

# Phase 7 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` package + `github.com/stretchr/testify` v1.11.1 |
| **Config file** | none (standard `go test ./...`) |
| **Quick run command** | `cd services/share-service && go test ./featuregates/... ./middleware/... ./handlers/... -v -count=1` |
| **Full suite command** | `cd services/share-service && go test ./... -v -count=1` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd services/share-service && go test ./featuregates/... ./middleware/... ./handlers/... -v -count=1`
- **After every plan wave:** Run `cd services/share-service && go test ./... -v -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 07-01-01 | 01 | 1 | D-01 | unit | `go test ./featuregates/... -run TestIsPremium` | ❌ W0 | ⬜ pending |
| 07-01-02 | 01 | 1 | D-01 | unit | `go test ./featuregates/... -run TestIsPremiumFree` | ❌ W0 | ⬜ pending |
| 07-01-03 | 01 | 1 | D-10 | unit | `go test ./featuregates/... -run TestIsPremiumUnknownKey` | ❌ W0 | ⬜ pending |
| 07-02-01 | 02 | 1 | D-11 | unit | `go test ./middleware/... -run TestRequirePremiumGateFree` | ❌ W0 | ⬜ pending |
| 07-02-02 | 02 | 1 | D-11 | unit | `go test ./middleware/... -run TestRequirePremiumGatePremium` | ❌ W0 | ⬜ pending |
| 07-02-03 | 02 | 1 | D-16 | unit | `go test ./middleware/... -run TestRequirePremiumUserPremium` | ❌ W0 | ⬜ pending |
| 07-03-01 | 03 | 2 | D-07 | unit | `go test ./featuregates/... -run TestReload` | ❌ W0 | ⬜ pending |
| 07-03-02 | 03 | 2 | D-08 | unit | `go test ./featuregates/... -run TestInvalidationTrigger` | ❌ W0 | ⬜ pending |
| 07-04-01 | 04 | 3 | D-13 | unit | `go test ./handlers/... -run TestUpdateGate` | ❌ W0 | ⬜ pending |
| 07-04-02 | 04 | 3 | D-13 | unit | `go test ./handlers/... -run TestListGates` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `services/share-service/featuregates/cache_test.go` — stubs for IsPremium, unknown-key default, reload, invalidation trigger
- [ ] `services/share-service/middleware/premium_test.go` — stubs for gate-free, gate-premium + user checks
- [ ] `services/share-service/handlers/admin_featuregates_test.go` — stubs for PATCH and GET handlers

*Existing infrastructure covers framework needs — testify already in go.mod.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Admin UI toggle switches render correctly | D-12 | Visual/browser UI | Open `/admin/features`, verify toggle per gate |
| Real-time propagation across services | D-14 | Requires multi-service env | Toggle gate in admin, verify share-service picks up change within 2s |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
