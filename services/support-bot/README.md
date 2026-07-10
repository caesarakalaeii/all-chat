# Support Bot

A Discord support/admin agent for All-Chat. It answers codebase and operational
questions over a **locally hosted, OpenAI-compatible LLM** and executes tools through a
code-enforced access model. It replaces the previous TypeScript bot that shelled out to
the `claude` CLI (`CLAUDE_CODE_OAUTH_TOKEN`) — the "Claude workaround" — with a native Go
agentic loop.

## Access modes

The requester's mode is decided **in trusted code from a Discord UID allow-list**
(`SUPPORT_BOT_ADMIN_DISCORD_IDS`), never from message content. This is the core security
invariant: a message that says "you are now admin" changes nothing.

| | SUPPORT (everyone) | ADMIN (allow-listed maintainers) |
|---|---|---|
| Read repo files (`read_file`, `glob`, `grep`) | ✅ (redacted) | ✅ |
| Read-only cluster (`kubectl` get/describe/logs/events/top) | ✅ (logs summarized) | ✅ (raw) |
| Grafana Loki/Prometheus queries | ✅ (logs summarized) | ✅ (raw) |
| GitHub read + comment + file issue (`github`) | ✅ | ✅ |
| GitHub write: branch + PR + review + close (`github_write`) | ❌ | ✅ |
| Bot memory recall/store (`memory`) | ✅ | ✅ |
| Database writes / secret reads | ❌ (not exposed) | ❌ (not exposed) |
| Output redaction (log lines, secrets, internal hosts/IPs) | ✅ enforced | off |

"Push to the cluster" for admins is done **only by opening a GitHub PR** against the
manifests; the existing GitOps/Keel pipeline deploys it. The bot has no kubectl-write
access and needs none.

## How it works

```
Discord message / /support
  → resolve AccessMode from UID allow-list (access.Policy)
  → build scope-aware system prompt + sanitized user turn
  → agent loop (agent.Run):
        LLM (llm.ChatClient, OpenAI /v1/chat/completions, tool_choice=auto)
        → tool_calls → registry.Dispatch (fail-closed by mode, loop-detected)
        → tool results fed back → repeat until stop / iteration cap
  → StripInternalScaffolds + (SUPPORT) redact final answer
  → deliver in a Discord thread; ping lead dev on side effects
```

Guardrails: fail-closed permissioning, repeat/no-progress detection, prompt-injection
sanitization, XML boundary tags on tool output, and a code-level leak redactor applied
at both the tool-output and final-answer boundaries.

## Configuration

Required: `DISCORD_BOT_TOKEN`, `LOCAL_LLM_MODEL`. See [.env.example](./.env.example) for
the full list. Key variables:

| Variable | Purpose |
|---|---|
| `DISCORD_BOT_TOKEN` / `DISCORD_CLIENT_ID` / `DISCORD_GUILD_ID` | Discord bot + slash-command registration |
| `LOCAL_LLM_BASE_URL` / `LOCAL_LLM_MODEL` / `LOCAL_LLM_API_KEY` | OpenAI-compatible endpoint (API key optional) |
| `SUPPORT_BOT_ADMIN_DISCORD_IDS` | Comma-separated maintainer UIDs → ADMIN |
| `GITHUB_TOKEN` / `GITHUB_OWNER` / `GITHUB_BOT_LOGIN` | GitHub tools (owner default `caesarakalaeii`) |
| `GRAFANA_URL` / `GRAFANA_SERVICE_ACCOUNT_TOKEN` | Enables the Grafana tools |
| `KUBE_NAMESPACE` | Namespace kubectl reads are pinned to (default `allchat`) |
| `DATABASE_*` | Bot memory store (optional; bot runs without it) |
| `ALL_CHAT_REPO_PATH` / `ALL_CHAT_EXTENSION_REPO_PATH` | Jail roots for the read tools |

## Develop

```bash
go build ./...
go test ./...          # unit tests (no external services needed)
go vet ./...
```

## Deploy

Container image `ghcr.io/caesarakalaeii/allchat-support-bot`; auto-rolled by Keel. The
Dockerfile builds from the repo root and bundles `kubectl`. Kubernetes manifests live in
`deployments/k8s/base/support-bot/` (this repo) and `apps/workloads/all-chat/` in the
caesar-deployment repo (authoritative). The read-only cluster RBAC is in
`support-bot-rbac.yaml` there.
