---
phase: 02-support-bot-operational-awareness
verified: 2026-03-26T10:30:00Z
status: passed
score: 15/15 must-haves verified
re_verification: false
---

# Phase 2: Support Bot Operational Awareness Verification Report

**Phase Goal:** Give the support bot access to Grafana logs and Kubernetes cluster state to detect ongoing infrastructure errors (missing secrets, crashed pods, OOMKills) so it can distinguish code bugs from operational issues. Bot must NEVER leak raw logs to Discord users. Recognize lead developer and ping them when infrastructure issues are detected.
**Verified:** 2026-03-26T10:30:00Z
**Status:** passed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | queryCodebase passes --mcp-config JSON with grafana-caesar MCP server when GRAFANA_URL and GRAFANA_SERVICE_ACCOUNT_TOKEN are set | VERIFIED | `agent.ts` lines 49-68: builds mcpConfigArg conditionally; test `includes --mcp-config with grafana-caesar server when Grafana env vars are set` asserts exact JSON shape and env values |
| 2 | queryCodebase includes Bash(kubectl:*) and Grafana MCP tools in --allowedTools | VERIFIED | `agent.ts` lines 70-82: `baseTools = ['Read','Glob','Grep','Bash(kubectl:*)']` always included; grafana tools spread in conditionally; two tests verify both with and without Grafana env vars |
| 3 | queryCodebase parses INFRA_VERDICT:type\|\|\|summary from Claude output into infraVerdict field | VERIFIED | `agent.ts` lines 106-117: indexOf + split('|||') parsing; tests for both 'infrastructure' and 'code' type values pass |
| 4 | queryCodebase strips INFRA_VERDICT: marker from the answer text returned to caller | VERIFIED | `agent.ts` lines 119-122: slices answer to verdictIndex and trimEnd(); test `strips INFRA_VERDICT marker from result.answer` asserts `result.answer` equals 'The auth service is having issues.' |
| 5 | queryCodebase omits --mcp-config when Grafana env vars are absent | VERIFIED | `agent.ts` lines 53-68: guarded by `hasGrafana`; two tests for GRAFANA_URL missing and GRAFANA_SERVICE_ACCOUNT_TOKEN missing both assert mcpConfigIndex === -1 |
| 6 | System prompt contains leak prevention guardrails instructing Claude to never include raw logs, secrets, or internal hostnames | VERIFIED | `agent.ts` lines 19-25: prompt includes 'NEVER include raw log lines', 'NEVER include environment variable values, secret names with their values, or internal hostnames'; test `system prompt contains "NEVER include raw log lines"` passes |
| 7 | Subprocess timeout is 180_000ms | VERIFIED | `agent.ts` line 97: `timeout: 180_000`; test `uses timeout of 180_000ms (not 120_000ms)` passes |
| 8 | Bot prepends @mention to response when infraVerdict.type is 'infrastructure' | VERIFIED | `bot.ts` lines 43-49: `shouldPingLeadDev = result.infraVerdict?.type === 'infrastructure' || result.issueProposal !== null`; prepends `<@${config.leadDeveloperDiscordId}>`; test `prepends lead dev @mention when infraVerdict.type is infrastructure` passes |
| 9 | Bot prepends @mention to response when issueProposal is not null | VERIFIED | Same bot.ts shouldPingLeadDev logic; test `prepends lead dev @mention when issueProposal is not null and infraVerdict is null` passes |
| 10 | Bot does NOT prepend @mention when infraVerdict is null and issueProposal is null | VERIFIED | test `does NOT prepend lead dev @mention when infraVerdict is null and issueProposal is null` passes |
| 11 | Bot does NOT prepend @mention when infraVerdict.type is 'code' and no issue created | VERIFIED | test `does NOT prepend lead dev @mention when infraVerdict.type is code and issueProposal is null` passes |
| 12 | validateEnv exits with 1 when LEAD_DEVELOPER_DISCORD_ID, GRAFANA_URL, or GRAFANA_SERVICE_ACCOUNT_TOKEN is missing | VERIFIED | `index.ts` lines 13-21: all three in the `required` Record; three tests in index.test.ts each delete one and assert exit(1) with the correct var name in error message; all pass |
| 13 | Dockerfile installs kubectl v1.33.5 and mcp-grafana v0.11.3 before USER node | VERIFIED | `Dockerfile` lines 8-19: kubectl RUN block at line 9, mcp-grafana RUN block at line 14; USER node at line 32 — binaries installed before privilege drop |
| 14 | Deployment has LEAD_DEVELOPER_DISCORD_ID, GRAFANA_URL, GRAFANA_SERVICE_ACCOUNT_TOKEN env vars | VERIFIED | `support-bot-deployment.yaml` lines 99-107: all three present; GRAFANA_SERVICE_ACCOUNT_TOKEN uses secretKeyRef to allchat-secrets/support-bot-grafana-token |
| 15 | RBAC Role grants read-only access to pods, events, deployments, replicasets, and metrics; RoleBinding binds it to support-bot ServiceAccount | VERIFIED | `support-bot-rbac.yaml` lines 33-60: Role `support-bot-cluster-reader` with pods/events (get,list,watch), deployments/replicasets (get,list), metrics.k8s.io/pods (get,list); RoleBinding at lines 48-60 binds to ServiceAccount support-bot |

**Score:** 15/15 truths verified

---

## Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `services/support-bot/src/types.ts` | InfraVerdict interface, updated QueryResult with infraVerdict field, updated BotConfig | VERIFIED | All three interfaces present with all required fields; `InfraVerdict { type: 'infrastructure'\|'code'; summary: string }`, `QueryResult.infraVerdict: InfraVerdict\|null`, `BotConfig.leadDeveloperDiscordId/grafanaUrl/grafanaServiceAccountToken: string` |
| `services/support-bot/src/claude/agent.ts` | Extended queryCodebase with MCP config, kubectl tools, infra prompt, INFRA_VERDICT parsing | VERIFIED | File is 141 lines of substantive logic; contains all expected patterns; fully wired — agent.ts imports from types.ts (`import type { QueryResult, IssueProposal, InfraVerdict }`) |
| `services/support-bot/src/__tests__/agent.test.ts` | Unit tests for all new agent.ts behaviors | VERIFIED | 15 tests in describe block; covers INFRA_VERDICT parsing both types, stripping, null case, mcp-config present/absent, allowedTools with/without Grafana, timeout, leak prevention prompt |
| `services/support-bot/src/bot.ts` | Lead dev @mention injection on infra verdict or issue creation | VERIFIED | shouldPingLeadDev logic present; `config.leadDeveloperDiscordId` used; wired to BotConfig from types.ts |
| `services/support-bot/src/index.ts` | Env var validation for LEAD_DEVELOPER_DISCORD_ID, GRAFANA_URL, GRAFANA_SERVICE_ACCOUNT_TOKEN | VERIFIED | All three in required Record; returned in BotConfig; wired to startBot via config object |
| `services/support-bot/src/__tests__/bot.test.ts` | Tests for lead dev @mention behavior | VERIFIED | 5 tests in 'Lead developer @mention' describe block; testConfig includes all new BotConfig fields |
| `services/support-bot/src/__tests__/index.test.ts` | Tests for new env var validation | VERIFIED | 3 new tests (one per new required var); updated BotConfig assertion checks all three new fields |
| `services/support-bot/Dockerfile` | Container image with kubectl and mcp-grafana binaries | VERIFIED | kubectl v1.33.5 at /usr/local/bin/kubectl; mcp-grafana v0.11.3 at /usr/local/bin/mcp-grafana; both installed before USER node |
| `../caesar-deployment/apps/workloads/all-chat/support-bot-deployment.yaml` | Deployment with new env vars | VERIFIED | LEAD_DEVELOPER_DISCORD_ID (plain value), GRAFANA_URL (plain value), GRAFANA_SERVICE_ACCOUNT_TOKEN (secretKeyRef to support-bot-grafana-token) |
| `../caesar-deployment/apps/workloads/all-chat/support-bot-rbac.yaml` | RBAC with read-only cluster access | VERIFIED | support-bot-cluster-reader Role and RoleBinding present; existing support-bot-secret-patcher preserved |

---

## Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `agent.ts` | `types.ts` | `import type { QueryResult, IssueProposal, InfraVerdict }` | VERIFIED | Line 2 of agent.ts: `import type { QueryResult, IssueProposal, InfraVerdict } from '../types.js'` — exact pattern required |
| `bot.ts` | `types.ts` | `config.leadDeveloperDiscordId` | VERIFIED | bot.ts imports `BotConfig` from types.ts; `config.leadDeveloperDiscordId` used in line 47 |
| `bot.ts` | `agent.ts` | `result.infraVerdict` | VERIFIED | bot.ts calls `queryCodebase` and reads `result.infraVerdict?.type` — wired at lines 29, 44 |
| `support-bot-deployment.yaml` | `support-bot-rbac.yaml` | `serviceAccountName: support-bot` | VERIFIED | deployment.yaml line 23: `serviceAccountName: support-bot`; rbac.yaml has ServiceAccount `support-bot` and RoleBinding subjects pointing to it |

---

## Requirements Coverage

| Requirement | Source Plan | Description (derived from CONTEXT.md) | Status | Evidence |
|-------------|------------|----------------------------------------|--------|----------|
| OPS-01 | 02-01 | Grafana MCP integration in Claude subprocess (--mcp-config with grafana-caesar) | SATISFIED | agent.ts conditional mcpConfigArg; test coverage |
| OPS-02 | 02-01 | kubectl tool access in subprocess (Bash(kubectl:*) in --allowedTools) | SATISFIED | agent.ts baseTools always includes Bash(kubectl:*) |
| OPS-03 | 02-01 | Leak prevention system prompt guardrails (no raw logs/secrets/hostnames) | SATISFIED | agent.ts systemPrompt lines 19-25; test asserts 'NEVER include raw log lines' present |
| OPS-04 | 02-01 | INFRA_VERDICT:type\|\|\|summary marker parsing and stripping | SATISFIED | agent.ts lines 106-138; full parse + strip + return in infraVerdict field |
| OPS-05 | 02-02 | Lead developer @mention when infrastructure issue detected | SATISFIED | bot.ts shouldPingLeadDev check on infraVerdict.type === 'infrastructure' |
| OPS-06 | 02-02 | Lead developer @mention when GitHub issue created; new env var validation | SATISFIED | bot.ts shouldPingLeadDev check on issueProposal !== null; index.ts validates all three new env vars with exit(1) |
| OPS-07 | 02-03 | kubectl binary installed in container (v1.33.5 at /usr/local/bin/kubectl) | SATISFIED | Dockerfile lines 8-12 |
| OPS-08 | 02-03 | mcp-grafana binary installed in container (v0.11.3 at /usr/local/bin/mcp-grafana) | SATISFIED | Dockerfile lines 14-19 |
| OPS-09 | 02-03 | RBAC for read-only cluster inspection + deployment env vars | SATISFIED | support-bot-rbac.yaml (support-bot-cluster-reader Role + RoleBinding); support-bot-deployment.yaml (3 new env vars) |

**All 9 requirements satisfied.** No orphaned requirements detected.

---

## Anti-Patterns Found

No blockers or warnings found. Scanned all 7 modified TypeScript source files for TODO/FIXME/placeholder patterns, empty returns, and console.log-only implementations.

Notable: `bot.ts` uses `void message.channel.sendTyping()` in the interval (fire-and-forget — intentional pattern for typing indicator) and `void main()` at module level (standard Node.js pattern). Neither is a stub or blocker.

---

## Human Verification Required

### 1. Live Grafana MCP Connectivity

**Test:** Deploy the bot with Grafana env vars set; trigger a question that would cause infra checks; confirm Claude subprocess actually queries Loki/Prometheus successfully.
**Expected:** Bot response includes an infrastructure summary (not just code analysis), and INFRA_VERDICT appears in the parsed result, triggering a lead dev @mention.
**Why human:** Cannot verify MCP tool connectivity from static analysis. The mcp-grafana binary integration and live Grafana service account token can only be confirmed at runtime.

### 2. kubectl In-Container Access

**Test:** After deployment, exec into the support-bot container and run `kubectl -n allchat get pods` as the `node` user.
**Expected:** Command succeeds and lists allchat pods. Access is read-only (no create/delete capability).
**Why human:** RBAC correctness against the live cluster must be verified at deployment time. Static YAML analysis confirms the intent but not the live cluster state.

### 3. Lead Developer Discord Mention Delivery

**Test:** Ask the support bot a question that causes an infrastructure verdict. Confirm that Discord shows the @mention notification to the configured lead developer.
**Expected:** Lead developer receives a Discord notification ping containing the bot response.
**Why human:** Discord mention delivery depends on the live Discord API and the correct snowflake ID being valid in the target guild.

---

## Gaps Summary

No gaps. All 15 must-haves verified across all three plans. The test suite exits 0 with 60/60 tests passing. All source files are substantive (not stubs). All key links are wired. All 9 OPS requirements have clear implementation evidence.

Three items flagged for human verification are operational concerns (runtime connectivity, RBAC against live cluster, Discord notification delivery) — not code correctness issues.

---

_Verified: 2026-03-26T10:30:00Z_
_Verifier: Claude (gsd-verifier)_
