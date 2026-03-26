# Phase 2: Support Bot Operational Awareness - Context

**Gathered:** 2026-03-26
**Status:** Ready for planning

<domain>
## Phase Boundary

Extend the existing support-bot service (`services/support-bot`) with Grafana MCP and kubectl read-only access so it can detect ongoing infrastructure errors (pod crashes, missing secrets, OOMKills, connectivity issues) and distinguish them from code bugs when answering user questions. The bot must NEVER leak raw log lines, secrets, or internal hostnames to Discord users. The lead developer (caesarlp) is pinged via @mention when infrastructure issues are detected or GitHub issues are opened. Environment variable `LEAD_DEVELOPER_DISCORD_ID` is added to the deployment in `../caesar-deployment`.

</domain>

<decisions>
## Implementation Decisions

### Log & cluster access strategy
- Grafana MCP tools added to the `claude -p` subprocess's allowed tools — Claude queries Loki, Prometheus, and Tempo via MCP during reasoning
- kubectl read-only access via subprocess — Claude can describe pods, check secret existence, get events
- All three Grafana datasources enabled: Loki (logs), Prometheus (metrics), Tempo (traces)
- On-demand only — no background polling, bot queries infra state only when answering a user question
- Every question triggers a quick infra health check alongside codebase analysis (adds ~10-15s but catches infra issues reliably)

### Infrastructure signals to check
- Pod health: restarts, OOMKills, CrashLoopBackOff (via Prometheus `kube_pod_container_status_restarts_total`, `kube_pod_status_phase`)
- Recent error logs: last 15-30 min of `level=error` across allchat namespace (via Loki)
- Resource pressure: CPU/memory near limits (via Prometheus `container_memory_working_set_bytes` vs limits)
- Connectivity: DB and Redis reachability (via Prometheus `up` metrics or health endpoint probes)

### Leak prevention
- Claude system prompt guardrails: instruct Claude to NEVER include raw log lines, secrets, env vars, or internal hostnames in responses — summarize in user-friendly language only
- No post-processing sanitization layer in v1 — rely on prompt guardrails (can be added later if needed)
- Same output for all users — no role-based detail levels; lead dev also gets sanitized summaries; use Grafana directly for raw data

### Response transparency
- Bot is fully transparent with summaries — says things like "I queried Loki logs for auth-service and found 47 connection timeout errors in the last hour" but never shows raw log lines
- Categorized verdict: bot explicitly states whether the issue is infrastructure-level or code-level (e.g., "This appears to be an infrastructure issue: auth-service has restarted 5 times in the last 10 minutes")

### Lead developer escalation
- `LEAD_DEVELOPER_DISCORD_ID` env var (single ID): `198569499228766208` (caesarlp)
- Bot @mentions lead dev in two scenarios: (1) infrastructure issue detected, (2) GitHub issue opened
- Ping appears as @mention in the existing conversation thread — lead dev can see full context
- Env var added to deployment in `../caesar-deployment/apps/workloads/all-chat/support-bot-deployment.yaml`

### Claude's Discretion
- Exact Grafana MCP tool allowlist (which MCP tools to expose to the subprocess)
- kubectl RBAC scope (which resources the ServiceAccount can read)
- System prompt phrasing for leak prevention guardrails
- Infra health check query specifics (exact PromQL, LogQL queries)
- How to format the categorized verdict in Discord messages

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Support bot service (existing code to extend)
- `services/support-bot/src/claude/agent.ts` — Claude subprocess wrapper; must be extended with Grafana MCP and kubectl tool access
- `services/support-bot/src/bot.ts` — Discord event handlers and response formatting; must integrate lead dev @mention logic
- `services/support-bot/src/types.ts` — TypeScript interfaces; new config types needed for lead dev ID
- `services/support-bot/src/index.ts` — Entry point; new env var validation for LEAD_DEVELOPER_DISCORD_ID

### Kubernetes deployment (to modify)
- `../caesar-deployment/apps/workloads/all-chat/support-bot-deployment.yaml` — Add LEAD_DEVELOPER_DISCORD_ID env var, kubectl RBAC, Grafana MCP config
- `../caesar-deployment/apps/workloads/all-chat/support-bot-rbac.yaml` — Extend RBAC for kubectl read-only access to allchat namespace resources

### Grafana MCP integration
- Grafana MCP server is already available in the environment — tools include `query_loki_logs`, `query_prometheus`, `list_loki_label_names`, etc.
- `../caesar-deployment/apps/workloads/all-chat/configmap.yaml` — Contains service URLs, OpenTelemetry config (Tempo distributor)

### Architecture context
- `docs/architecture/04-OBSERVABILITY.md` — Existing observability setup (metrics, logging, tracing)
- `docs/architecture/02-DEPLOYMENT.md` — Kubernetes deployment architecture

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `services/support-bot/src/claude/agent.ts`: `queryCodebase()` function — needs extension to pass Grafana MCP config and kubectl access to the Claude subprocess
- `services/support-bot/src/bot.ts`: `sendResponse()` function — can be extended with @mention injection for lead dev pings
- `services/support-bot/src/bot.ts`: `PROPOSE_ISSUE:` marker parsing — already triggers GitHub issue creation; lead dev ping hooks into this flow
- `support-bot-rbac.yaml`: Existing RBAC ServiceAccount — extend with read-only cluster access

### Established Patterns
- Environment variable validation at startup with `process.exit(1)` on missing required vars — follow same pattern for `LEAD_DEVELOPER_DISCORD_ID`
- Claude subprocess uses `--allowedTools Read,Glob,Grep` — extend this list with Grafana MCP tool names and kubectl-related tools
- `execa` subprocess with 120s timeout — may need increase for infra-heavy queries
- discord.js `<@USER_ID>` syntax for @mentions in message content

### Integration Points
- Claude subprocess `--allowedTools` flag: add Grafana MCP tool names (query_loki_logs, query_prometheus, etc.)
- System prompt in `agent.ts`: add infra awareness instructions and leak prevention guardrails
- `bot.ts` response flow: inject @mention to lead dev when infra issue detected or issue created
- Kubernetes ServiceAccount: needs ClusterRole/Role binding for read-only pod, event, secret (metadata only) access

</code_context>

<specifics>
## Specific Ideas

- Bot should say things like "I queried Loki logs for auth-service and found 47 connection timeout errors in the last hour" — transparent summaries without raw data
- Lead dev ping on both infra issues AND GitHub issue creation — caesarlp should be immediately aware of both scenarios
- `LEAD_DEVELOPER_DISCORD_ID` as a single ID env var, not comma-separated — keep it simple for now
- The bot checks infra on every question, not selectively — better to spend 10-15s extra than miss an infra issue

</specifics>

<deferred>
## Deferred Ideas

- Post-processing sanitization layer as defense-in-depth for leak prevention — can be added if Claude prompt guardrails prove insufficient
- Background polling / proactive alerting to a #ops channel — current decision is on-demand only
- Role-based response detail levels (lead dev gets more info) — decided against for v1, all users get same summaries
- Multiple developer IDs support (comma-separated env var) — single ID sufficient for now

</deferred>

---

*Phase: 02-support-bot-operational-awareness*
*Context gathered: 2026-03-26*
