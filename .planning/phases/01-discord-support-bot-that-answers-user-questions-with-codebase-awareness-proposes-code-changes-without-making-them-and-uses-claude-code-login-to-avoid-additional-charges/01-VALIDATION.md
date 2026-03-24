---
phase: 1
slug: discord-support-bot-that-answers-user-questions-with-codebase-awareness-proposes-code-changes-without-making-them-and-uses-claude-code-login-to-avoid-additional-charges
status: draft
nyquist_compliant: true
wave_0_complete: false
created: 2026-03-24
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | vitest 3.x |
| **Config file** | services/support-bot/vitest.config.ts (Plan 01 Task 1 creates) |
| **Quick run command** | `cd services/support-bot && npx vitest run` |
| **Full suite command** | `cd services/support-bot && npx vitest run` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd services/support-bot && npx vitest run`
- **After every plan wave:** Run `cd services/support-bot && npx vitest run`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 1-01-01 | 01 | 1 | BOT-03,04,05,06 | unit | `cd services/support-bot && npx vitest run` | Plan 01 creates | pending |
| 1-01-02 | 01 | 1 | BOT-06 | unit | `cd services/support-bot && npx vitest run` | Plan 01 creates | pending |
| 1-02-01 | 02 | 2 | BOT-01,02 | unit | `cd services/support-bot && npx vitest run` | Plan 02 creates | pending |
| 1-02-02 | 02 | 2 | BOT-01 | typecheck | `cd services/support-bot && npx tsx --check src/commands/deploy.ts` | Plan 02 creates | pending |
| 1-03-01 | 03 | 3 | BOT-07 | build+validate | `docker build -f services/support-bot/Dockerfile -t support-bot:test . && kubectl apply --dry-run=client -f deployments/k8s/base/support-bot/deployment.yaml` | Plan 03 creates | pending |
| 1-03-02 | 03 | 3 | BOT-07 | manual | see Manual-Only below | — | pending |

*Status: pending / green / red / flaky*

---

## Wave 0 Requirements

Wave 0 is handled by Plan 01 Task 1 which creates:

- [x] `services/support-bot/package.json` — vitest + discord.js + execa dependencies
- [x] `services/support-bot/vitest.config.ts` — vitest config
- [x] `services/support-bot/src/__tests__/agent.test.ts` — Claude subprocess tests
- [x] `services/support-bot/src/__tests__/github.test.ts` — GitHub issue creation tests

*Wave 0 is embedded in Plan 01 Task 1 (scaffold + test together).*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Bot responds to @mention in Discord channel | BOT-01 | Requires live Discord token + guild | Mention bot, verify reply within 30s |
| Bot proposes code change as reply (no file writes) | BOT-04 | Requires live Claude subprocess + real repo | Ask bot to fix a bug, verify no files modified |
| GitHub issue creation from slash command | BOT-05 | Requires live GitHub PAT + target repo | Run /support in Discord, verify issue appears on GitHub |
| Kubernetes deployment is healthy | BOT-07 | Requires cluster access | `kubectl -n allchat get pod -l app=support-bot` shows Running |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
