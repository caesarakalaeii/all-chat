# Phase 1: Discord Support Bot - Context

**Gathered:** 2026-03-24
**Status:** Ready for planning

<domain>
## Phase Boundary

Build a new Discord bot service (`services/support-bot`) that answers user questions about the All-Chat and All-Chat-Extension projects with codebase awareness. The bot proposes code changes (via GitHub issues) but never makes them autonomously. It is deployed always-on in Kubernetes. Creating, modifying, or extending the existing `discord-listener` or `discord-bot` quota-monitor services is out of scope.

</domain>

<decisions>
## Implementation Decisions

### Bot home & runtime
- New standalone service: `services/support-bot` (not extending existing discord-bot)
- Language: Node.js TypeScript — consistent with existing discord-bot patterns; discord.js + @anthropic-ai/sdk are mature in Node
- Deployed always-on in Kubernetes alongside other services

### Question scope
- Topics handled: setup & configuration, architecture & how it works, bug triage & reporting
- Deployment/Kubernetes help is out of scope for v1
- Invocation: @mention or slash command only — bot activates only when explicitly called
- When a code change is needed: bot opens a GitHub issue with the proposed change and context (not a Discord reply, not a verbal-only response)

### Codebase access strategy
- Full repo clone with file reading (most powerful, most current)
- Both repos accessible:
  - `all-chat` repo — mounted as a Kubernetes volume
  - `all-chat-extension` repo — mounted as a second Kubernetes volume (same strategy)
- File selection: Claude decides which files to read based on the question (tool-use / ReAct loop) — not keyword routing, not fixed context injection

### Claude authentication
- Use `claude -p "question"` subprocess approach via `execa` — this reuses the Claude.ai subscription with no separate API billing
- Token provisioning: run `claude setup-token` once locally to generate a ~1-year `CLAUDE_CODE_OAUTH_TOKEN`, store as a Kubernetes secret `CLAUDE_CODE_OAUTH_TOKEN`, pass as env var into the pod
- The `claude` binary must be installed in the Docker image (install via `npm install -g @anthropic-ai/claude-code` in the Dockerfile)
- Model: `claude-sonnet-4-6` — passed via `--model claude-sonnet-4-6` flag to the subprocess
- No `@anthropic-ai/claude-agent-sdk` programmatic SDK — OAuth is blocked for server-side SDK use since 2026-01-09
- Codebase read-only enforcement: pass `--allowedTools Read,Glob,Grep` flag to the `claude -p` subprocess to restrict tool use

### Claude's Discretion
- Conversation threading (per-user, per-channel, or fresh context each message)
- Exact GitHub issue template format
- Rate limiting and concurrency handling
- Error message wording

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing Discord bot patterns (integration reference)
- `services/discord-bot/src/index.js` — Node.js discord.js bot structure, Redis pub/sub, graceful shutdown pattern
- `services/discord-bot/package.json` — dependency versions (discord.js, redis, etc.)

### Codebase documentation the bot will serve
- `CLAUDE.md` — top-level project guidance and service overview
- `services/*/README.md` — per-service documentation the bot should be able to reference
- `docs/architecture/00-OVERVIEW.md` — architecture overview
- `docs/troubleshooting/decision-tree.md` — troubleshooting guide
- `docs/llm-guides/` — task-oriented quick reference guides

### All-Chat-Extension repo
- `/home/moersener/Hobby/all-chat-extension/` — second repo the bot must be able to read; check for its own README/docs

### Claude Code SDK auth
- `@anthropic-ai/claude-code` — Claude Code SDK; researcher must confirm headless OAuth token reuse is supported for server-side use
- `@anthropic-ai/sdk` — standard Anthropic SDK as fallback if Claude Code SDK OAuth is not feasible headlessly

### GitHub integration (for issue creation)
- `gh` CLI is available and authenticated in this environment — researcher should check if GitHub API (octokit/REST) or `gh` CLI subprocess is preferred for issue creation from Node.js

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `services/discord-bot/src/index.js`: Discord client initialization, graceful shutdown (SIGINT/SIGTERM), Redis client pattern — copy as starting structure
- `services/discord-bot/Dockerfile`: Node.js Dockerfile template suitable for new service
- `services/discord-bot/package.json`: discord.js version and Redis client version to match

### Established Patterns
- Node.js bots use ES modules (`import`/`export`) — new service should follow same
- Graceful shutdown: `process.on('SIGINT', shutdown)` + `process.on('SIGTERM', shutdown)` — standard across all services
- Environment config: all config from `process.env`, validated at startup with `process.exit(1)` on missing required vars
- Discord client uses `GatewayIntentBits.Guilds` minimum — support bot will need `GuildMessages` + `MessageContent` intents for @mention detection

### Integration Points
- Kubernetes volume mounts: two PVCs or git-sync sidecars for `all-chat` and `all-chat-extension` repos
- Kubernetes secret: Claude OAuth token file mounted at a known path
- GitHub API: `gh` CLI or Octokit REST for creating issues in the `all-chat` and `all-chat-extension` repos

</code_context>

<specifics>
## Specific Ideas

- Bot must access two repos: `all-chat` (this repo) and `all-chat-extension` (browser extension project)
- Code change proposals go to GitHub issues — not Discord replies, not verbal descriptions
- The bot should NOT be able to modify code — propose-only behavior is a hard requirement
- Claude Code OAuth login is the preferred auth to avoid separate API billing — implemented via `claude -p` subprocess with `CLAUDE_CODE_OAUTH_TOKEN` env var (~1-year token from `claude setup-token`)

</specifics>

<deferred>
## Deferred Ideas

- Deployment/Kubernetes help topic — out of scope for v1, could be added in a follow-up phase
- Watching a #support channel automatically (bot monitors all messages) — current decision is @mention/slash only; could revisit
- Deployment/Kubernetes help (explicitly deferred)

</deferred>

---

*Phase: 01-discord-support-bot*
*Context gathered: 2026-03-24*
