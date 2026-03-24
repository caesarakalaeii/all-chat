---
phase: 1
slug: discord-support-bot-that-answers-user-questions-with-codebase-awareness-proposes-code-changes-without-making-them-and-uses-claude-code-login-to-avoid-additional-charges
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-24
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | jest 29.x |
| **Config file** | services/discord-support-bot/jest.config.js (Wave 0 creates) |
| **Quick run command** | `cd services/discord-support-bot && npm test -- --testPathPattern=unit` |
| **Full suite command** | `cd services/discord-support-bot && npm test` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd services/discord-support-bot && npm test -- --testPathPattern=unit`
- **After every plan wave:** Run `cd services/discord-support-bot && npm test`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 1-01-01 | 01 | 0 | setup | scaffold | `ls services/discord-support-bot/package.json` | ❌ W0 | ⬜ pending |
| 1-01-02 | 01 | 1 | bot | unit | `npm test -- --testPathPattern=bot` | ❌ W0 | ⬜ pending |
| 1-01-03 | 01 | 1 | claude | unit | `npm test -- --testPathPattern=claude` | ❌ W0 | ⬜ pending |
| 1-01-04 | 01 | 2 | k8s | manual | see Manual-Only below | — | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `services/discord-support-bot/package.json` — jest + discord.js + anthropic SDK dependencies
- [ ] `services/discord-support-bot/jest.config.js` — jest config
- [ ] `services/discord-support-bot/src/__tests__/bot.test.ts` — stubs for Discord event handling
- [ ] `services/discord-support-bot/src/__tests__/claude.test.ts` — stubs for Claude agent invocation

*Wave 0 installs test framework alongside the initial service scaffold.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Bot responds to @mention in Discord channel | Bot trigger | Requires live Discord token + guild | Mention bot, verify reply within 30s |
| Bot proposes code change as reply (no file writes) | Read-only safety | Requires live Claude API call + real repo | Ask bot to fix a bug, verify no files modified |
| GitHub issue creation from slash command | /issue command | Requires live GitHub PAT + target repo | Run /issue in Discord, verify issue appears on GitHub |
| Kubernetes deployment is healthy | Deployment | Requires cluster access | `kubectl -n allchat get pod -l app=discord-support-bot` shows Running |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
