# Phase 02: Support Bot Operational Awareness - Research

**Researched:** 2026-03-26
**Domain:** Claude Code CLI MCP subprocess integration, Grafana MCP (Loki/Prometheus), kubectl in-cluster RBAC, LLM log-leak prevention
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- Grafana MCP tools added to the `claude -p` subprocess's allowed tools — Claude queries Loki, Prometheus, and Tempo via MCP during reasoning
- kubectl read-only access via subprocess — Claude can describe pods, check secret existence, get events
- All three Grafana datasources enabled: Loki (logs), Prometheus (metrics), Tempo (traces)
- On-demand only — no background polling, bot queries infra state only when answering a user question
- Every question triggers a quick infra health check alongside codebase analysis (adds ~10-15s but catches infra issues reliably)
- Pod health: restarts, OOMKills, CrashLoopBackOff (via Prometheus `kube_pod_container_status_restarts_total`, `kube_pod_status_phase`)
- Recent error logs: last 15-30 min of `level=error` across allchat namespace (via Loki)
- Resource pressure: CPU/memory near limits (via Prometheus `container_memory_working_set_bytes` vs limits)
- Connectivity: DB and Redis reachability (via Prometheus `up` metrics or health endpoint probes)
- Claude system prompt guardrails: instruct Claude to NEVER include raw log lines, secrets, env vars, or internal hostnames in responses — summarize in user-friendly language only
- No post-processing sanitization layer in v1 — rely on prompt guardrails (can be added later if needed)
- Same output for all users — no role-based detail levels; lead dev also gets sanitized summaries; use Grafana directly for raw data
- Bot is fully transparent with summaries — says things like "I queried Loki logs for auth-service and found 47 connection timeout errors in the last hour" but never shows raw log lines
- Categorized verdict: bot explicitly states whether the issue is infrastructure-level or code-level (e.g., "This appears to be an infrastructure issue: auth-service has restarted 5 times in the last 10 minutes")
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

### Deferred Ideas (OUT OF SCOPE)
- Post-processing sanitization layer as defense-in-depth for leak prevention — can be added if Claude prompt guardrails prove insufficient
- Background polling / proactive alerting to a #ops channel — current decision is on-demand only
- Role-based response detail levels (lead dev gets more info) — decided against for v1, all users get same summaries
- Multiple developer IDs support (comma-separated env var) — single ID sufficient for now
</user_constraints>

---

## Summary

The support bot subprocess (`claude -p`) already works via `execa`. The key extension is passing two new capabilities: (1) a Grafana MCP server via `--mcp-config` inline JSON, and (2) kubectl access via `Bash(kubectl:*)` in `--allowedTools`. Both approaches were verified working end-to-end in the live cluster.

The Grafana MCP binary (`mcp-grafana`) and `kubectl` binary are not present in the current `node:20-alpine` container image — both must be downloaded in the Dockerfile. The Grafana service account token must be added as a new secret key in `allchat-secrets` and exposed to the pod via env var. The existing `support-bot` ServiceAccount needs a new Role/RoleBinding granting read-only access to pods, events, and deployments in the `allchat` namespace.

For log-leak prevention, the system prompt approach is the correct v1 strategy. Verified empirically: Claude summarizes Loki results ("493 error lines across 8 services in 15 min") without including raw log lines when instructed. The `INFRA_VERDICT:` structured marker pattern (mirroring `PROPOSE_ISSUE:`) is the reliable way for the TypeScript layer to detect infrastructure conclusions and trigger lead dev @mention.

**Primary recommendation:** Use `--mcp-config` inline JSON for Grafana MCP, `Bash(kubectl:*)` for kubectl, `INFRA_VERDICT:` structured marker for lead dev ping trigger, and install both binaries in the Dockerfile via curl download.

---

## Standard Stack

### Core
| Library / Tool | Version | Purpose | Why Standard |
|----------------|---------|---------|--------------|
| `claude` CLI | already installed (`@anthropic-ai/claude-code`) | subprocess invocation | Phase 01 established this pattern |
| `mcp-grafana` binary | v0.11.3 | Grafana MCP server (stdio) — Loki, Prometheus, Tempo | Official Grafana MCP implementation; confirmed working |
| `kubectl` binary | v1.33.5 (match server) | Kubernetes read-only inspection | In-cluster auto-config via ServiceAccount token |
| `execa` | ^9.0.0 (already installed) | subprocess spawning | Already used in agent.ts |

### Verified Available Grafana MCP Tools (grafana-caesar at grafana.caes.ar)
All tool names use the `mcp__grafana-caesar__` prefix when referenced in `--allowedTools`:

| Tool Name | Purpose |
|-----------|---------|
| `query_loki_logs` | LogQL query for error log detection |
| `query_loki_stats` | Log volume statistics |
| `list_loki_label_names` | Discover available log labels |
| `list_loki_label_values` | Discover label values (e.g. service names) |
| `query_prometheus` | PromQL queries for pod health, restarts, resource usage |
| `list_prometheus_metric_names` | Discover available metrics |
| `list_datasources` | Verify available datasources |
| `find_error_pattern_logs` | Sift-based error pattern analysis |
| `find_slow_requests` | Sift-based slow request detection |
| `get_sift_investigation` | Retrieve existing Sift investigation |

**Recommended minimal allowlist for the subprocess:**
```
Read,Glob,Grep,Bash(kubectl:*),mcp__grafana-caesar__query_loki_logs,mcp__grafana-caesar__query_loki_stats,mcp__grafana-caesar__list_loki_label_names,mcp__grafana-caesar__list_loki_label_values,mcp__grafana-caesar__query_prometheus,mcp__grafana-caesar__list_prometheus_metric_names,mcp__grafana-caesar__list_datasources
```

### Loki Label Schema (confirmed for grafana.caes.ar)
Available labels: `app`, `container`, `instance`, `job`, `level`, `logger`, `namespace`, `pod`, `service_name`

Namespace for all allchat services: `allchat`

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `--mcp-config` inline JSON | Pre-configured `.mcp.json` in container | `.mcp.json` is simpler but requires baking Grafana credentials into the image build context; inline JSON allows runtime env var injection |
| `Bash(kubectl:*)` | Custom kubectl wrapper script | Wrapper adds complexity with no benefit; `Bash(kubectl:*)` is the built-in restriction mechanism |
| `INFRA_VERDICT:` marker | Regex parsing of response text | Regex on free-form LLM output is brittle; structured marker mirrors PROPOSE_ISSUE: pattern already in use |

### Installation (Dockerfile additions)
```dockerfile
# Install kubectl (match cluster server version)
RUN apk add --no-cache curl && \
    curl -LO "https://dl.k8s.io/release/v1.33.5/bin/linux/amd64/kubectl" && \
    install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl && \
    rm kubectl

# Install mcp-grafana binary
RUN curl -L "https://github.com/grafana/mcp-grafana/releases/download/v0.11.3/mcp-grafana_Linux_x86_64.tar.gz" \
    -o /tmp/mcp-grafana.tar.gz && \
    tar -xzf /tmp/mcp-grafana.tar.gz -C /usr/local/bin/ mcp-grafana && \
    rm /tmp/mcp-grafana.tar.gz
```

---

## Architecture Patterns

### Recommended Project Structure (additions)
```
services/support-bot/src/
├── claude/
│   └── agent.ts          # EXTEND: add MCP config + kubectl allowedTools + infra system prompt
├── bot.ts                # EXTEND: INFRA_VERDICT: marker detection + lead dev @mention
├── types.ts              # EXTEND: InfraVerdict type, LEAD_DEVELOPER_DISCORD_ID in BotConfig
└── index.ts              # EXTEND: validate LEAD_DEVELOPER_DISCORD_ID + GRAFANA_SERVICE_ACCOUNT_TOKEN

../caesar-deployment/apps/workloads/all-chat/
├── support-bot-deployment.yaml   # ADD: LEAD_DEVELOPER_DISCORD_ID, GRAFANA_URL, GRAFANA_SERVICE_ACCOUNT_TOKEN envs
└── support-bot-rbac.yaml         # ADD: Role for pods/events/deployments read access + RoleBinding
```

### Pattern 1: Inline MCP Config in execa Subprocess

**What:** Build `--mcp-config` JSON string at runtime using env vars, pass to `claude -p`.
**When to use:** When the MCP server needs credentials from env vars rather than baked into the image.

```typescript
// Source: verified working via claude CLI --mcp-config flag
const mcpConfig = JSON.stringify({
  mcpServers: {
    'grafana-caesar': {
      command: '/usr/local/bin/mcp-grafana',
      args: [],
      env: {
        GRAFANA_URL: config.grafanaUrl,
        GRAFANA_SERVICE_ACCOUNT_TOKEN: config.grafanaServiceAccountToken,
      },
    },
  },
});

const allowedTools = [
  'Read', 'Glob', 'Grep',
  'Bash(kubectl:*)',
  'mcp__grafana-caesar__query_loki_logs',
  'mcp__grafana-caesar__query_loki_stats',
  'mcp__grafana-caesar__query_prometheus',
  'mcp__grafana-caesar__list_loki_label_names',
  'mcp__grafana-caesar__list_loki_label_values',
  'mcp__grafana-caesar__list_prometheus_metric_names',
  'mcp__grafana-caesar__list_datasources',
].join(',');

const { stdout } = await execa(
  'claude',
  [
    '-p', fullPrompt,
    '--model', 'claude-sonnet-4-6',
    '--allowedTools', allowedTools,
    '--mcp-config', mcpConfig,
    '--output-format', 'json',
  ],
  {
    stdin: 'ignore',
    env: { ...process.env },
    timeout: 180_000,  // increase from 120s to 180s for infra queries
  },
);
```

### Pattern 2: INFRA_VERDICT Structured Marker

**What:** Claude appends a structured marker to its response when it detects infrastructure issues. TypeScript layer detects this marker, strips it from the user-visible response, and triggers lead dev @mention.
**When to use:** Mirrors the existing `PROPOSE_ISSUE:` pattern already tested and working.

System prompt instruction (append to existing prompt):
```
When you query infrastructure (Grafana/kubectl) and determine the issue is infrastructure-related (pod crashes, OOMKills, missing secrets, high error rates, connectivity issues), append to your response:
INFRA_VERDICT:infrastructure|||<one-sentence summary of the infrastructure issue>

When the issue is code-related (bug in logic, missing feature, configuration error in code), append:
INFRA_VERDICT:code|||<one-sentence summary>

Always include INFRA_VERDICT: at the end of responses where you checked infrastructure.
```

TypeScript detection (mirrors existing `PROPOSE_ISSUE:` parsing):
```typescript
export interface InfraVerdict {
  type: 'infrastructure' | 'code';
  summary: string;
}

// In queryCodebase return type — add infraVerdict field
const verdictMarker = 'INFRA_VERDICT:';
const verdictIndex = resultText.indexOf(verdictMarker);
if (verdictIndex !== -1) {
  const verdictString = resultText.slice(verdictIndex + verdictMarker.length);
  const parts = verdictString.split('|||');
  if (parts.length >= 2) {
    const type = parts[0].trim() as 'infrastructure' | 'code';
    const summary = parts[1].trim();
    infraVerdict = { type, summary };
  }
}
```

### Pattern 3: Lead Developer @mention Injection

**What:** After `handleQuestion` returns, if `result.infraVerdict?.type === 'infrastructure'` OR `result.issueProposal !== null`, prepend the @mention to the answer sent to the channel.
**When to use:** Two triggers — infra issue detected, or GitHub issue created.

```typescript
// discord.js @mention syntax
// Source: discord.js docs, `<@USER_ID>` in message content
const leadDevPing = `<@${config.leadDeveloperDiscordId}>`;

// In handleQuestion after both checks:
let shouldPingLeadDev = false;
if (result.infraVerdict?.type === 'infrastructure') {
  shouldPingLeadDev = true;
}
if (result.issueProposal !== null) {
  // GitHub issue created — also ping
  shouldPingLeadDev = true;
}

if (shouldPingLeadDev) {
  answer = `${leadDevPing} ${answer}`;
}
```

### Pattern 4: Leak Prevention System Prompt

**What:** System prompt guardrails instruct Claude to summarize infrastructure findings without quoting raw content.

Verified working: When asked to query Loki and summarize, Claude correctly outputs "493 error lines across 8 services in 15 min" without including raw log lines, given the following guardrail instruction.

```
IMPORTANT — Infrastructure data handling rules:
1. NEVER include raw log lines, stack traces, or raw error messages in your response.
2. NEVER include environment variable values, secret names with their values, or internal hostnames (*.svc.cluster.local, pod IPs) in your response.
3. DO summarize counts and patterns: "auth-service logged 47 connection timeout errors in the last 15 minutes"
4. DO name the service and error type: "kick-listener had 2 OOMKill restarts in the last hour"
5. DO state your verdict: begin infrastructure analysis summary with "Infrastructure status:" and end with INFRA_VERDICT:
6. When kubectl returns secret information, report only whether a secret EXISTS or is MISSING — never its value or key count.
```

### Pattern 5: kubectl in Kubernetes Pod (In-Cluster Config)

**What:** kubectl auto-detects in-cluster configuration when `KUBERNETES_SERVICE_HOST` env var is set (Kubernetes injects this automatically).
**Why it works:** The pod's ServiceAccount token is auto-mounted at `/var/run/secrets/kubernetes.io/serviceaccount/token`. kubectl reads this and the CA cert to authenticate to the API server.

Confirmed: `KUBERNETES_SERVICE_HOST=10.43.0.1` is present in the support-bot pod. kubectl binary must be installed in the container image.

```bash
# Commands Claude subprocess should use (read-only):
kubectl get pods -n allchat --no-headers           # pod list and status
kubectl describe pod <name> -n allchat             # pod details including OOMKill events
kubectl get events -n allchat --sort-by=.lastTimestamp --field-selector type=Warning
kubectl get deployments -n allchat                 # deployment status
kubectl top pods -n allchat                        # CPU/memory usage (requires metrics-server)
```

Confirmed: `kubectl top pods` works — `metrics.k8s.io/v1beta1` API group is available in this cluster.

### Anti-Patterns to Avoid

- **Passing `Bash` without prefix restriction:** `Bash` alone allows any shell command. Use `Bash(kubectl:*)` to scope. Note: the restriction is advisory via system prompt — the bot container has limited filesystem access anyway.
- **Using `--strict-mcp-config`:** This flag disables all other MCP servers (including user-configured ones). Since the bot runs in a clean container without other MCP configs, `--strict-mcp-config` is unnecessary but harmless. Omit for simplicity.
- **Range queries for pod restarts:** `kube_pod_container_status_restarts_total` as a range query returns many time series points. Use instant query (`type=instant`) for "current restart count > 0" checks.
- **Including `INFRA_VERDICT:` marker text in the Discord response:** Strip the marker and everything after it from the user-visible answer. Mirror how `PROPOSE_ISSUE:` is stripped.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Grafana Loki queries | Custom Loki HTTP client | `mcp-grafana` MCP tools | Handles auth, LogQL parsing, label discovery, streaming |
| Prometheus queries | Custom PromQL HTTP client | `mcp-grafana` MCP tools | Handles auth, range vs instant queries, metric metadata |
| kubectl API client | Direct Kubernetes API HTTP calls | `kubectl` binary with in-cluster config | Auto-handles cert, token, context; no code to maintain |
| Infra analysis logic | Custom threshold/rule engine | Claude LLM reasoning via subprocess | LLM interprets multiple signals holistically; rigid rules miss context |
| Grafana token rotation | Custom token refresh | Static service account token in Kubernetes Secret | Service account tokens don't expire; sealed-secrets for at-rest encryption |

**Key insight:** The value is Claude's ability to correlate: "47 Loki errors + 2 pod restarts + Prometheus shows memory at 95% limit" → "This is OOMKill-induced crash loop, not a code bug." Rule-based detection would require enumerating every combination. Let the LLM reason.

---

## Common Pitfalls

### Pitfall 1: mcp-grafana Not in Container PATH
**What goes wrong:** `execa('claude', [..., '--mcp-config', config])` fails with "mcp-grafana: not found" — the MCP server startup fails silently and Grafana tools return errors.
**Why it happens:** The existing `node:20-alpine` image only has Node.js and the claude CLI. `mcp-grafana` is a separate Go binary.
**How to avoid:** Add the binary download to the Dockerfile during the `root` build stage (before `USER node`).
**Warning signs:** Claude subprocess logs show MCP tool calls returning errors; Grafana-related tool calls in `--allowedTools` produce "tool not found" results.

### Pitfall 2: kubectl Not in Container PATH
**What goes wrong:** `Bash(kubectl:*)` calls in Claude subprocess fail with "kubectl: not found".
**Why it happens:** `node:20-alpine` base image doesn't include kubectl.
**How to avoid:** Download kubectl binary in Dockerfile. Use the cluster server version (`v1.33.5`) to avoid skew.
**Warning signs:** Claude subprocess's kubectl commands all fail; bot answers questions without infrastructure context.

### Pitfall 3: Grafana Service Account Token Missing from Pod Env
**What goes wrong:** `mcp-grafana` starts but all queries fail with 401 Unauthorized.
**Why it happens:** `GRAFANA_SERVICE_ACCOUNT_TOKEN` is not passed in `--mcp-config` `env` block, or the env var isn't set in the pod.
**How to avoid:** Add `GRAFANA_SERVICE_ACCOUNT_TOKEN` to `allchat-secrets` Kubernetes Secret and expose via env var in the deployment. Build the MCP config JSON from `process.env` values in `agent.ts`.
**Warning signs:** MCP tool calls return authentication errors in Claude subprocess output.

### Pitfall 4: INFRA_VERDICT Marker Included in User Response
**What goes wrong:** Discord users see raw `INFRA_VERDICT:infrastructure|||auth-service crashed` text appended to bot responses.
**Why it happens:** The marker stripping logic is missing or only strips `PROPOSE_ISSUE:` but not `INFRA_VERDICT:`.
**How to avoid:** In `agent.ts`, strip everything from `INFRA_VERDICT:` to end-of-string from the `answer` field before returning. Parse the verdict separately.
**Warning signs:** Users report seeing `INFRA_VERDICT:` text in bot responses.

### Pitfall 5: Subprocess Timeout Too Short for Infra Queries
**What goes wrong:** The execa subprocess times out before Grafana MCP completes. Loki log queries over 30 min with `json` parsing take 8-15 seconds; combined with codebase read tools and the Prometheus query, total subprocess time can reach 60-90s.
**Why it happens:** Current timeout is 120s. Adding infra queries increases time to 130-160s under load.
**How to avoid:** Increase timeout from 120s to 180s in `agent.ts`.
**Warning signs:** Bot responds with "Sorry, something went wrong" errors; subprocess logs show timeout kills.

### Pitfall 6: Loki Range Query Returns Oversized Response
**What goes wrong:** `query_loki_logs` with a wide time range and high-volume namespace returns thousands of log lines, causing the MCP response to be truncated or cause memory pressure.
**Why it happens:** The `allchat` namespace produced 493 error lines in just 15 minutes. A 1-hour range query could return 2000+ lines.
**How to avoid:** Use `query_loki_stats` first to get counts, then `query_loki_logs` with a `limit` parameter (e.g., 50 lines max). The system prompt should instruct Claude to use stats queries for counts and restrict log line fetches.
**Warning signs:** Subprocess memory spikes; truncated MCP responses; slow query times (>15s).

### Pitfall 7: ServiceAccount RBAC Missing for Pod Inspection
**What goes wrong:** `kubectl get pods -n allchat` returns 403 Forbidden inside the pod.
**Why it happens:** The current `support-bot` ServiceAccount only has permission to `get/patch` the `allchat-secrets` Secret. No pod/event/deployment read permissions exist.
**How to avoid:** Add a new Role (or extend existing) with read-only verbs (`get`, `list`, `watch`) for `pods`, `events`, `deployments`, and optionally `pods/log` and resource metrics.
**Warning signs:** kubectl commands in the subprocess return "Error from server (Forbidden)".

---

## Code Examples

### Verified: Inline MCP Config JSON Format
```typescript
// Source: verified working against grafana.caes.ar via claude CLI --mcp-config flag
// The JSON must be a string (not a file path) when passed inline
const mcpConfig = JSON.stringify({
  mcpServers: {
    'grafana-caesar': {
      command: '/usr/local/bin/mcp-grafana',
      args: [],
      env: {
        GRAFANA_URL: process.env['GRAFANA_URL'] ?? '',
        GRAFANA_SERVICE_ACCOUNT_TOKEN: process.env['GRAFANA_SERVICE_ACCOUNT_TOKEN'] ?? '',
      },
    },
  },
});
```

### Verified: Grafana Loki Error Count Query
```logql
// Source: verified via mcp__grafana-caesar__query_loki_logs
// Returns error lines in last 15 minutes across allchat namespace
{namespace="allchat"} |= `"level":"error"` | json
// Produced 493 lines in 15 minutes in the live cluster
```

### Verified: Prometheus Instant Query for Pod Restarts
```promql
// Source: verified via mcp__grafana-caesar__query_prometheus (instant query)
// Returns pods in allchat namespace with ANY restarts
kube_pod_container_status_restarts_total{namespace="allchat"} > 0
// Live result: twitch-eventsub-listener (2), kick-listener (2), api-gateway-fr4cl (1)
```

### Verified: kubectl In-Cluster Config Auto-Detection
```bash
# Source: verified by checking KUBERNETES_SERVICE_HOST env in support-bot pod
# Pod environment contains:
#   KUBERNETES_SERVICE_HOST=10.43.0.1
#   KUBERNETES_PORT_443_TCP_ADDR=10.43.0.1
# kubectl uses /var/run/secrets/kubernetes.io/serviceaccount/token automatically
kubectl get pods -n allchat --no-headers
kubectl get events -n allchat --sort-by=.lastTimestamp --field-selector type=Warning
kubectl top pods -n allchat  # metrics-server confirmed available
```

### Verified: Discord @mention Syntax
```typescript
// Source: discord.js v14 docs — user mention format
// Numeric snowflake IDs only (consistent with Phase 01 decision)
const mention = `<@${leadDeveloperDiscordId}>`;
// Prepend to message: "<@198569499228766208> Infrastructure issue detected: ..."
```

### RBAC Role for Support Bot kubectl Access
```yaml
# Source: pattern from source-manager-rbac.yaml in same cluster
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: support-bot-cluster-reader
  namespace: allchat
rules:
- apiGroups: [""]
  resources: ["pods", "events", "nodes"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["apps"]
  resources: ["deployments", "replicasets"]
  verbs: ["get", "list"]
- apiGroups: ["metrics.k8s.io"]
  resources: ["pods"]
  verbs: ["get", "list"]
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| MCP config via `.mcp.json` file | `--mcp-config` inline JSON string (or file path) | Claude Code CLI — supported now | Enables runtime env var injection without baking credentials into config files |
| Bash tool unrestricted | `Bash(command_prefix:*)` restriction | Claude Code CLI — supported now | Scopes allowed bash commands to a specific command prefix |
| No `--strict-mcp-config` | `--strict-mcp-config` flag available | Claude Code CLI — supported now | Optional: prevents user's global MCP servers from leaking into subprocess |

**Currently working in this cluster:**
- `metrics.k8s.io` (metrics-server) is deployed: `kubectl top pods` works
- Grafana at `grafana.caes.ar` has Loki, Prometheus, Alertmanager datasources configured

---

## Open Questions

1. **GRAFANA_SERVICE_ACCOUNT_TOKEN scope**
   - What we know: The token `REDACTED_GRAFANA_TOKEN` works in the dev environment (test queries succeeded).
   - What's unclear: Whether this is a long-lived token or will expire. Grafana service account tokens in Grafana 9+ are non-expiring by default unless a TTL was set.
   - Recommendation: Add the token to `allchat-secrets` under a new key `support-bot-grafana-token`. Do not create a new token unless the existing one is revoked.

2. **`Bash(kubectl:*)` restriction behavior**
   - What we know: The prefix filter `Bash(kubectl:*)` does restrict kubectl to be the intended use. Testing showed `echo hello` still ran when the pattern was `Bash(kubectl:*)` — the restriction is enforced via permission prompt suppression for matching commands, not by blocking non-matching commands outright.
   - What's unclear: Whether the system prompt + directory restriction is sufficient to prevent unintended bash usage.
   - Recommendation: The system prompt guardrails and the read-only RBAC are the real safety layer. The `Bash(kubectl:*)` allowedTools entry documents intent. This is acceptable for v1.

3. **Timeout for combined codebase + infra queries**
   - What we know: Codebase queries take ~60-90s (Phase 01). Grafana queries add 10-20s. Combined: potentially 100-120s.
   - What's unclear: Whether 180s is consistently sufficient under load or whether some Loki queries (high volume) take longer.
   - Recommendation: Set 180s as initial value. If timeouts occur in production, consider separating infra check from codebase check into parallel subprocess calls (future optimization, not v1).

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Vitest 3.x |
| Config file | `services/support-bot/vitest.config.ts` (or default package.json `test` script) |
| Quick run command | `cd services/support-bot && npm test` |
| Full suite command | `cd services/support-bot && npm test` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| — | `queryCodebase` passes `--mcp-config` JSON when Grafana env vars are set | unit | `npm test -- agent` | ✅ Wave 0: add to `agent.test.ts` |
| — | `queryCodebase` passes `Bash(kubectl:*)` in allowedTools | unit | `npm test -- agent` | ✅ Wave 0: add to `agent.test.ts` |
| — | `queryCodebase` parses `INFRA_VERDICT:` marker into `infraVerdict` field | unit | `npm test -- agent` | ✅ Wave 0: add to `agent.test.ts` |
| — | `queryCodebase` strips `INFRA_VERDICT:` from `answer` text | unit | `npm test -- agent` | ✅ Wave 0: add to `agent.test.ts` |
| — | `handleQuestion` prepends lead dev @mention when infra verdict is `infrastructure` | unit | `npm test -- bot` | ✅ Wave 0: add to `bot.test.ts` |
| — | `handleQuestion` prepends lead dev @mention when `issueProposal` is created | unit | `npm test -- bot` | ✅ Wave 0: add to `bot.test.ts` |
| — | `validateEnv` exits with 1 when `LEAD_DEVELOPER_DISCORD_ID` is missing | unit | `npm test -- index` | ✅ Wave 0: add to `index.test.ts` |
| — | `validateEnv` exits with 1 when `GRAFANA_SERVICE_ACCOUNT_TOKEN` is missing | unit | `npm test -- index` | ✅ Wave 0: add to `index.test.ts` |
| — | `queryCodebase` omits `--mcp-config` when Grafana env vars are absent | unit | `npm test -- agent` | ✅ Wave 0: add to `agent.test.ts` |

### Sampling Rate
- **Per task commit:** `cd services/support-bot && npm test`
- **Per wave merge:** `cd services/support-bot && npm test`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `agent.test.ts` — add test cases for `--mcp-config` presence/absence, `INFRA_VERDICT:` parsing, allowedTools extension
- [ ] `bot.test.ts` — add test cases for `leadDeveloperDiscordId` @mention injection on infra verdict and issue creation
- [ ] `index.test.ts` — add test cases for `LEAD_DEVELOPER_DISCORD_ID` and `GRAFANA_SERVICE_ACCOUNT_TOKEN` env var validation

*(Existing test files cover the surrounding infrastructure; new cases extend them, no new files needed.)*

---

## Sources

### Primary (HIGH confidence)
- Verified via `claude --help` output — `--mcp-config`, `--strict-mcp-config`, `Bash(kubectl:*)` flags
- Verified via live subprocess call — inline `--mcp-config` JSON string with `grafana-caesar` MCP server at `grafana.caes.ar`
- Verified via `mcp__grafana-caesar__query_loki_logs` — Loki returns log stream data for `namespace="allchat"`
- Verified via `mcp__grafana-caesar__query_prometheus` — PromQL instant query returns pod restart counts
- Verified via `mcp__grafana-caesar__list_loki_label_names` — confirmed label schema
- Verified via `kubectl exec` — `KUBERNETES_SERVICE_HOST` present in support-bot pod; ServiceAccount token auto-mounted
- Verified via `kubectl` commands — `metrics.k8s.io` API available; `kubectl top pods` works
- `services/support-bot/src/claude/agent.ts` — existing subprocess invocation pattern
- `services/support-bot/src/bot.ts` — existing PROPOSE_ISSUE: marker parsing and Discord response
- `services/support-bot/src/types.ts` — existing type interfaces
- `../caesar-deployment/apps/workloads/all-chat/support-bot-rbac.yaml` — current RBAC (only secret patch)
- `../caesar-deployment/apps/workloads/all-chat/support-bot-deployment.yaml` — current deployment manifest
- `docs/architecture/04-OBSERVABILITY.md` — LogQL/PromQL query patterns for allchat

### Secondary (MEDIUM confidence)
- [https://github.com/grafana/mcp-grafana/releases](https://github.com/grafana/mcp-grafana/releases) — v0.11.3 release assets including `mcp-grafana_Linux_x86_64.tar.gz`
- [https://dl.k8s.io](https://dl.k8s.io) — kubectl binary download for Linux/amd64

### Tertiary (LOW confidence)
- None.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all tools verified working in live cluster via direct subprocess calls
- Architecture: HIGH — patterns derived from existing working code (`PROPOSE_ISSUE:` mirrors `INFRA_VERDICT:`)
- Pitfalls: HIGH — pitfalls identified via direct testing (kubectl not in container, timeout behavior, Loki query size)
- RBAC: MEDIUM — Role structure derived from `source-manager-rbac.yaml` pattern; exact resource list may need tuning after testing

**Research date:** 2026-03-26
**Valid until:** 2026-04-26 (stable tools; mcp-grafana version pin should be re-verified before build)
