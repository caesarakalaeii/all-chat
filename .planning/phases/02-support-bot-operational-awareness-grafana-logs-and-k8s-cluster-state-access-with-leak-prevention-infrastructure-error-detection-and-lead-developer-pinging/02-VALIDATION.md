---
phase: 02
slug: support-bot-operational-awareness
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-26
---

# Phase 02 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Vitest 3.x |
| **Config file** | `services/support-bot/vitest.config.ts` (or default package.json `test` script) |
| **Quick run command** | `cd services/support-bot && npm test` |
| **Full suite command** | `cd services/support-bot && npm test` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd services/support-bot && npm test`
- **After every plan wave:** Run `cd services/support-bot && npm test`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 02-01-01 | 01 | 1 | — | unit | `npm test -- agent` | ✅ extend agent.test.ts | ⬜ pending |
| 02-01-02 | 01 | 1 | — | unit | `npm test -- agent` | ✅ extend agent.test.ts | ⬜ pending |
| 02-01-03 | 01 | 1 | — | unit | `npm test -- agent` | ✅ extend agent.test.ts | ⬜ pending |
| 02-02-01 | 02 | 1 | — | unit | `npm test -- bot` | ✅ extend bot.test.ts | ⬜ pending |
| 02-02-02 | 02 | 1 | — | unit | `npm test -- bot` | ✅ extend bot.test.ts | ⬜ pending |
| 02-02-03 | 02 | 1 | — | unit | `npm test -- index` | ✅ extend index.test.ts | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `agent.test.ts` — add test cases for `--mcp-config` presence/absence, `INFRA_VERDICT:` parsing, allowedTools extension
- [ ] `bot.test.ts` — add test cases for `leadDeveloperDiscordId` @mention injection on infra verdict and issue creation
- [ ] `index.test.ts` — add test cases for `LEAD_DEVELOPER_DISCORD_ID` and `GRAFANA_SERVICE_ACCOUNT_TOKEN` env var validation

*Existing test files cover the surrounding infrastructure; new cases extend them, no new files needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Grafana MCP queries return data in live cluster | Infra detection | Requires live Grafana + Loki/Prometheus | Deploy to staging, ask bot a question, verify it mentions querying logs |
| Lead dev @mention appears in Discord thread | Escalation | Requires live Discord connection | Trigger infra issue or GitHub issue, verify @mention in thread |
| Raw logs never appear in Discord responses | Leak prevention | Requires adversarial prompting | Ask bot about specific errors, verify no raw log lines in response |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
