---
phase: 6
slug: unify-all-listeners-to-leadership-based-coordination
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-27
---

# Phase 6 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — each service has its own go.mod |
| **Quick run command** | `make build-all` |
| **Full suite command** | `make build-all && cd shared/listener && go test ./...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `make build-all`
- **After every plan wave:** Run `make build-all && cd shared/listener && go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 06-01-01 | 01 | 1 | D-06,D-07,D-08,D-09 | compile | `make build-all` | ✅ | ⬜ pending |
| 06-02-01 | 02 | 2 | D-10,D-11 | compile | `make build-all` | ✅ | ⬜ pending |
| 06-02-02 | 02 | 2 | D-12,D-13 | compile | `make build-all` | ✅ | ⬜ pending |
| 06-02-03 | 02 | 2 | D-15 | compile | `make build-all` | ✅ | ⬜ pending |
| 06-03-01 | 03 | 3 | D-01,D-02,D-03,D-04,D-05 | compile+test | `make build-all && cd services/source-manager && go test ./...` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

*Existing infrastructure covers all phase requirements.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| K8s port 8088 no longer served | D-05 | Requires live cluster | Deploy, verify source-manager serves only on 8083 |
| Listeners reconnect after rolling deploy | D-17 | Requires live cluster | Rolling restart listeners, verify channel reconnection |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
