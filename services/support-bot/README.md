# Support Bot

A Discord support/admin agent for All-Chat. It answers codebase and operational
questions over, in order of precedence, the **Claude Code CLI** on a Claude subscription
(`CLAUDE_CODE_OAUTH_TOKEN`, production), **Claude** via the Anthropic Messages API
(`ANTHROPIC_API_KEY`), or an **OpenAI-compatible LLM endpoint** (`LOCAL_LLM_BASE_URL`, an
external inference gateway). Both are disclosed in the privacy policy (Section 2.8). It executes tools
through a code-enforced access model, and hands confirmed bugs to **Caterpillar**, the
autonomous coding agent, as `agent`-labelled issues (ADR-0093). It replaces the previous
TypeScript bot that shelled out to the `claude` CLI with a native Go agentic loop.

## Access modes

The requester's mode is decided **in trusted code from a Discord UID allow-list**
(`SUPPORT_BOT_ADMIN_DISCORD_IDS`), never from message content. This is the core security
invariant: a message that says "you are now admin" changes nothing.

| | SUPPORT (everyone) | ADMIN (allow-listed maintainers) |
|---|---|---|
| Read repo files (`read_file`, `glob`, `grep`) | ✅ (redacted) | ✅ |
| Read-only cluster (`kubectl` get/describe/logs/events/top) | ✅ (logs summarized) | ✅ (raw) |
| Grafana Loki/Prometheus queries | ✅ (logs summarized) | ✅ (raw) |
| GitHub read + search + comment + file issue (`github`) | ✅ | ✅ |
| File an agent task for Caterpillar (`github` `agent_task`) | ✅ | ✅ |
| GitHub write: branch + PR + review + close (`github_write`) | ❌ | ✅ |
| Bot memory recall/store (`memory`) | ✅ | ✅ |
| Attach a UI screenshot (`send_image`) | ✅ | ✅ |
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
        LLM (llm.ChatClient: Claude via llm/anthropic.go, or OpenAI /v1/chat/completions)
        → tool_calls → registry.Dispatch (fail-closed by mode, loop-detected)
        → tool results fed back → repeat until stop / iteration cap
  → StripInternalScaffolds + (SUPPORT) redact final answer
  → deliver in a Discord thread; ping lead dev on side effects
```

Guardrails: fail-closed permissioning, repeat/no-progress detection, prompt-injection
sanitization, XML boundary tags on tool output, and a code-level leak redactor applied
at both the tool-output and final-answer boundaries.

**Claude Code path** (`agent/claudecode.go`): with `CLAUDE_CODE_OAUTH_TOKEN` set, the bot
runs the unmodified `claude` binary (`-p --output-format json`) per question instead of
its own loop. The CLI gets no built-in tools (`--tools ""`), an empty working directory
and an environment holding only `PATH`, `HOME` and the token, so none of the bot's own
secrets reach it. The bot's tool registry is served to it over a localhost MCP endpoint
(`--mcp-config`, `--strict-mcp-config`, `--allowedTools mcp__allchat`); each run gets a
random bearer token bound to the requester's access mode, so the fail-closed per-mode
gate, redaction, the repeat guard and effect reporting work as in the in-process loop.
`--max-turns` is `SUPPORT_BOT_MAX_ITERATIONS`. A usage or rate limit parks the request in
the busy queue; a resumed request starts the CLI again from the question. The image
ships the pinned, checksum-verified binary (`Dockerfile`), which is why it runs on
`distroless/cc` (glibc) rather than `distroless/static`.

**Messages API specifics** (`llm/anthropic.go`): the transcript stays OpenAI-shaped and is
translated per request. Each assistant turn keeps its native content in `Message.Native`
and is replayed verbatim, because Claude's thinking blocks are bound to the conversation
that produced them. Requests use prompt caching, `output_config.effort` and server-side
refusal fallbacks; a refusal still ends with a visible answer.

**Screenshots** (`tools/image.go`): `send_image` attaches one of the frontend's
visual-regression baselines (`frontend/tests/e2e/__screenshots__/visual-regression.spec.ts-snapshots/`,
rendered by CI with demo data, dark and light, 1280 and 375 wide) to the answer. It picks
by screen name from that directory only, never a path; at most three go out per answer,
as a separate message after the text, and they are not side effects (no lead-dev ping).
New screens appear when the visual suite gains a scene and the bot pod restarts (it clones
the repo at start).

**Understanding first**: the system prompt makes the bot gather what only the reporter
knows (what they see vs. expected, which platform and overlay, since when, what they tried)
and, when that is missing or ambiguous, reply with its reading of the problem and at most
three clarifying questions instead of guessing. The user answers in the thread; follow-ups
get the thread history, including the message the thread was started from (it lives in
the parent channel, so `fetchHistory` fetches it separately).

**Fixing things (Caterpillar)**: when the bot confirms a bug or a small, scoped improvement,
and the reporter has confirmed the bot's summary of it in the thread (a maintainer can
waive that by asking explicitly), it searches for an existing issue (`search_issues`), then files one that follows
`.github/ISSUE_TEMPLATE/agent_task.md` with `agent_task=true`. The tool refuses a body
without a fenced `agent` block carrying `acceptance` commands, and labels the issue `agent`
(+ `bot-proposed`). Caterpillar claims it, implements it, and merges the PR once its review
council approves. The bot's GitHub token is the owner's, so Caterpillar treats these as
owner-authored: anyone in Discord can steer the bot into filing one (accepted risk, see
ADR-0093; restrict `agent_task` to admin mode to narrow it).

**Busy (429) handling**: a 429 is capacity, not failure: the gateway pauses
hobby-project traffic while production load is high, and Anthropic rate-limits. Mid-session the loop
waits (`SUPPORT_BOT_BUSY_RETRY_DELAY`, up to `SUPPORT_BOT_BUSY_MAX_WAITS`) and
re-issues the same transcript — tool results and partial answers are preserved
and side effects never replay. Beyond that the request is parked in a queue and
resumed by a background worker until capacity returns or
`SUPPORT_BOT_BUSY_QUEUE_TTL` expires; the user is told their ask is queued.

## Configuration

Required: `DISCORD_BOT_TOKEN`. See [.env.example](./.env.example) for
the full list. Key variables:

| Variable | Purpose |
|---|---|
| `DISCORD_BOT_TOKEN` / `DISCORD_CLIENT_ID` / `DISCORD_GUILD_ID` | Discord bot + slash-command registration |
| `CLAUDE_CODE_OAUTH_TOKEN` | `claude setup-token` token (one year); when set the bot runs the Claude Code CLI and ignores the two rows below |
| `CLAUDE_CODE_MODEL` / `CLAUDE_CODE_BIN` / `CLAUDE_CODE_HOME` | Defaults `opus` / `claude` / `/tmp/claude-code` |
| `ANTHROPIC_API_KEY` | Console API key; when set the bot runs on Claude and ignores `LOCAL_LLM_*` |
| `ANTHROPIC_MODEL` / `ANTHROPIC_EFFORT` / `ANTHROPIC_MAX_TOKENS` | Defaults `claude-opus-5-5` / `medium` / `16000` |
| `LOCAL_LLM_BASE_URL` / `LOCAL_LLM_MODEL` / `LOCAL_LLM_API_KEY` | vLLM endpoint (OpenAI-compatible); key sent as `Authorization: Bearer <key>`. The model id is resolved at boot from `GET <base URL>/v1/models`; `LOCAL_LLM_MODEL` is an optional pin that wins while the gateway still lists it, and is kept when the list endpoint cannot be reached |
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
