# Phase 1: Discord Support Bot - Research

**Researched:** 2026-03-24
**Domain:** Discord bot (Node.js/TypeScript), Claude Agent SDK, GitHub Issues API
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- New standalone service: `services/support-bot` (not extending existing discord-bot)
- Language: Node.js TypeScript — consistent with existing discord-bot patterns; discord.js + @anthropic-ai/sdk are mature in Node
- Deployed always-on in Kubernetes alongside other services
- Topics handled: setup & configuration, architecture & how it works, bug triage & reporting
- Deployment/Kubernetes help is out of scope for v1
- Invocation: @mention or slash command only — bot activates only when explicitly called
- When a code change is needed: bot opens a GitHub issue with the proposed change and context (not a Discord reply, not a verbal-only response)
- Full repo clone with file reading (most powerful, most current)
- Both repos accessible: `all-chat` repo (mounted as Kubernetes volume) and `all-chat-extension` repo (mounted as second Kubernetes volume)
- File selection: Claude decides which files to read based on the question (tool-use / ReAct loop)
- Use `@anthropic-ai/sdk` with Claude Code OAuth tokens to reuse existing Claude.ai subscription
- Token provisioning: run `claude auth login` locally once, export the OAuth token/credentials file, store as a Kubernetes secret, mount into the bot pod
- Model: `claude-sonnet-4-6` — best balance of reasoning depth and response speed for Discord

### Claude's Discretion
- Conversation threading (per-user, per-channel, or fresh context each message)
- Exact GitHub issue template format
- Rate limiting and concurrency handling
- Error message wording

### Deferred Ideas (OUT OF SCOPE)
- Deployment/Kubernetes help topic — out of scope for v1, could be added in a follow-up phase
- Watching a #support channel automatically (bot monitors all messages) — current decision is @mention/slash only; could revisit
</user_constraints>

---

## CRITICAL FINDING: Claude Code OAuth is Blocked

> **This invalidates the locked authentication decision and must be surfaced to the planner.**

The user's locked decision states "Use `@anthropic-ai/sdk` with Claude Code OAuth tokens to reuse existing Claude.ai subscription." **This is no longer feasible.**

On 2026-01-09, Anthropic deployed server-side enforcement blocking all third-party use of Claude Code OAuth tokens (`sk-ant-oat01-*`). Attempting to use them against the Messages API returns:

```
"OAuth authentication is currently not supported."
```

Anthropic's official documentation now explicitly states: *"Unless previously approved, Anthropic does not allow third party developers to offer claude.ai login or rate limits for their products, including agents built on the Claude Agent SDK. Please use the API key authentication methods described in this document instead."*

**Required substitution:** Use `ANTHROPIC_API_KEY` (standard API key from platform.claude.com). The user will need a separate API key. However, the total cost for a Discord support bot with moderate usage is very low (a few questions per day at $3/MTok for claude-sonnet-4-6 = cents per day).

**Recommended SDK:** `@anthropic-ai/claude-agent-sdk` (formerly `@anthropic-ai/claude-code`) — this is the exact SDK that powers Claude Code. It natively supports the read-only file exploration loop the user wants, requires only `ANTHROPIC_API_KEY`, and includes built-in `Read`, `Glob`, `Grep` tools with a managed agentic loop.

Sources: [GitHub issue #37205](https://github.com/anthropics/claude-code/issues/37205), [Anthropic Agent SDK docs](https://platform.claude.com/docs/en/agent-sdk/overview), [Claude OAuth Update article](https://daveswift.com/claude-oauth-update/)

---

## Summary

This phase builds `services/support-bot`, a Node.js TypeScript Discord bot that answers questions about the all-chat and all-chat-extension codebases by using the Claude Agent SDK to read repo files autonomously, then creates GitHub issues when code changes are proposed.

The standard stack is: `discord.js` v14 for Discord integration (same version as existing bots), `@anthropic-ai/claude-agent-sdk` for the Claude-powered file reading and reasoning loop, and `@octokit/rest` for GitHub issue creation. Authentication uses a standard `ANTHROPIC_API_KEY` stored as a Kubernetes secret (the user's preferred OAuth approach is not supported by Anthropic for server-side use).

The codebase access pattern leverages the Agent SDK's built-in `Read`, `Glob`, and `Grep` tools with `allowedTools` restricted to read-only operations — the bot can never modify code. Repo directories are bind-mounted into the pod (either as hostPath volumes or git-sync sidecars). Per-question session management is the safest starting point, with optional session continuity as a discretionary enhancement.

**Primary recommendation:** Use `@anthropic-ai/claude-agent-sdk` with `ANTHROPIC_API_KEY`, restrict `allowedTools` to `["Read", "Glob", "Grep"]`, mount both repos as Kubernetes volumes at known paths, and wire the GitHub issue creation via `@octokit/rest` with a GitHub PAT.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `discord.js` | 14.25.1 | Discord gateway, slash commands, @mention handling | Same version as existing discord-bot service; mature, typed |
| `@anthropic-ai/claude-agent-sdk` | 0.2.81 | Agentic loop with built-in file reading tools | Powers Claude Code itself; replaces hand-rolling tool-use loops; uses ANTHROPIC_API_KEY |
| `@octokit/rest` | 22.0.1 | GitHub REST API — create issues | Official GitHub SDK; typed; the standard choice over gh CLI subprocess |
| `typescript` | 5.x | Compile-time type safety | Project convention per CONTEXT.md |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `@anthropic-ai/sdk` | 0.80.0 | Direct Messages API client | If Agent SDK proves insufficient; requires implementing own tool loop |
| `tsx` or `ts-node` | latest | Dev-time TypeScript execution | Local development only |
| `@types/node` | ^20 | Node.js type definitions | Always needed with TypeScript |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `@anthropic-ai/claude-agent-sdk` | `@anthropic-ai/sdk` + hand-rolled tool loop | Agent SDK manages message history, tool execution, and retries; the client SDK requires you to implement the full agentic loop yourself |
| `@octokit/rest` | `gh` CLI subprocess | gh subprocess works but requires gh to be installed in the container image, adds process spawn overhead, and is harder to unit test |
| discord.js slash commands | Discord @mention-only | Slash commands are more discoverable and have built-in argument parsing; @mention still needed as alternative trigger |

**Installation:**
```bash
npm install discord.js @anthropic-ai/claude-agent-sdk @octokit/rest
npm install -D typescript @types/node tsx
```

**Version verification (run before writing package.json):**
```bash
npm view discord.js version          # 14.25.1 verified
npm view @anthropic-ai/claude-agent-sdk version  # 0.2.81 verified
npm view @octokit/rest version        # 22.0.1 verified
```

---

## Architecture Patterns

### Recommended Project Structure

```
services/support-bot/
├── src/
│   ├── index.ts           # Entry point — env validation, client init, start()
│   ├── bot.ts             # Discord client setup, event listeners (messageCreate, interactionCreate)
│   ├── commands/
│   │   ├── deploy.ts      # One-time slash command registration script
│   │   └── support.ts     # /support slash command definition
│   ├── claude/
│   │   └── agent.ts       # Claude Agent SDK wrapper — runQuery(question, repoPath[])
│   ├── github/
│   │   └── issues.ts      # Octokit wrapper — createIssue(repo, title, body)
│   └── types.ts           # Shared TypeScript interfaces
├── package.json           # type: "module", scripts: start/dev/deploy
├── tsconfig.json
└── Dockerfile
```

### Pattern 1: Discord @Mention + Slash Command Handler

**What:** Listen to `messageCreate` for @mentions and `interactionCreate` for slash commands. Both paths converge on the same `handleQuestion(question: string, userId: string, channelId: string)` function.

**When to use:** Required — provides both invocation modes per the locked decision.

**Example:**
```typescript
// Source: discord.js v14 official guide — discordjs.guide/creating-your-bot/slash-commands.html
import { Client, GatewayIntentBits, Events, Message } from 'discord.js';

const client = new Client({
  intents: [
    GatewayIntentBits.Guilds,
    GatewayIntentBits.GuildMessages,
    GatewayIntentBits.MessageContent,  // Required for reading @mention content
  ]
});

client.on(Events.MessageCreate, async (message: Message) => {
  if (!message.mentions.has(client.user!.id)) return;
  if (message.author.bot) return;

  const question = message.content.replace(/<@!?\d+>/g, '').trim();
  await handleQuestion(question, message.author.id, message.channelId, message);
});

client.on(Events.InteractionCreate, async (interaction) => {
  if (!interaction.isChatInputCommand()) return;
  if (interaction.commandName !== 'support') return;

  const question = interaction.options.getString('question', true);
  await interaction.deferReply();  // Required — Claude calls can exceed 3s
  await handleQuestion(question, interaction.user.id, interaction.channelId, interaction);
});
```

**IMPORTANT:** `GatewayIntentBits.MessageContent` requires enabling the "Message Content Intent" in the Discord Developer Portal for the bot application. Without it, message content is empty for messages not addressed directly to the bot in DMs.

### Pattern 2: Claude Agent SDK — Read-Only Codebase Query

**What:** Use `@anthropic-ai/claude-agent-sdk` `query()` with `allowedTools: ['Read', 'Glob', 'Grep']` and a working directory pointing to the mounted repo. The SDK manages the full agentic loop (Claude reads files as needed, then synthesizes an answer).

**When to use:** Every question — this is the core intelligence loop.

**Example:**
```typescript
// Source: https://platform.claude.com/docs/en/agent-sdk/overview
import { query } from '@anthropic-ai/claude-agent-sdk';

export async function queryCodebase(
  question: string,
  repoPaths: string[]  // e.g. ['/repos/all-chat', '/repos/all-chat-extension']
): Promise<string> {
  const systemPrompt = `You are a support bot for the All-Chat project.
You help users with: setup & configuration, architecture questions, and bug triage.
You can read files in: ${repoPaths.join(', ')}.
If a code change is needed, respond with PROPOSE_ISSUE:<title>|||<body> — do NOT make the change.
Do NOT answer questions about Kubernetes deployment (out of scope for v1).`;

  let result = '';
  for await (const message of query({
    prompt: question,
    options: {
      allowedTools: ['Read', 'Glob', 'Grep'],
      // permissionMode: 'default' means no edits/writes — read-only by default with these tools
      systemPrompt,
      // cwd defaults to process.cwd() — set working dir to primary repo for file reads
    }
  })) {
    if ('result' in message) {
      result = message.result ?? '';
    }
  }
  return result;
}
```

**Note on working directory:** The Agent SDK's file tools resolve paths relative to the process working directory. The Kubernetes pod must start with `cwd` pointing to (or the code must pass) the mounted repo path. For multi-repo access, the system prompt should name both paths and Claude will use absolute paths.

### Pattern 3: GitHub Issue Creation via Octokit

**What:** When Claude's response contains a code change proposal, extract it and create a GitHub issue.

**When to use:** When Claude returns a response indicating a code change is needed.

**Example:**
```typescript
// Source: https://github.com/octokit/rest.js
import { Octokit } from '@octokit/rest';

const octokit = new Octokit({ auth: process.env.GITHUB_TOKEN });

export async function createIssue(
  owner: string,
  repo: string,
  title: string,
  body: string
): Promise<string> {
  const response = await octokit.rest.issues.create({
    owner,
    repo,
    title,
    body,
    labels: ['bot-proposed', 'needs-review'],
  });
  return response.data.html_url;
}
```

### Pattern 4: Slash Command Registration (one-time deploy script)

**What:** Slash commands must be registered separately from the bot runtime. A `deploy.ts` script runs once (or on command definition change) to register commands.

**When to use:** Before the bot can respond to `/support` commands. Prefer guild-scoped during development, then switch to global for production.

```typescript
// Source: discordjs.guide/creating-your-bot/command-deployment.html
import { REST, Routes, SlashCommandBuilder } from 'discord.js';

const commands = [
  new SlashCommandBuilder()
    .setName('support')
    .setDescription('Ask a question about All-Chat')
    .addStringOption(option =>
      option.setName('question')
        .setDescription('What would you like to know?')
        .setRequired(true)
    )
    .toJSON()
];

const rest = new REST().setToken(process.env.DISCORD_BOT_TOKEN!);
await rest.put(
  Routes.applicationGuildCommands(CLIENT_ID, GUILD_ID),  // guild for dev, applicationCommands for prod
  { body: commands }
);
```

### Anti-Patterns to Avoid

- **Streaming responses to Discord line by line:** Discord's rate limits (5 messages per 5 seconds per channel) will cause 429 errors. Buffer the full Claude response and send it as a single message (or split into embeds if > 2000 chars).
- **Not deferring slash command replies:** Discord requires a response within 3 seconds. `interaction.deferReply()` extends this to 15 minutes. Always defer before calling Claude.
- **Using `@anthropic-ai/sdk` + hand-rolled tool loop for file reading:** The Agent SDK handles file reads autonomously. Implementing the loop manually duplicates the SDK's core value and adds complexity.
- **Mounting repos as ConfigMaps:** ConfigMaps have a 1MB data limit. Use PersistentVolumeClaims or git-sync sidecars for full repo access.
- **Global slash command registration on every startup:** Global commands propagate to all guilds within an hour. Re-registering on every pod start causes unnecessary API calls and may hit Discord rate limits.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Agentic file reading loop | Custom tool-use message loop with `@anthropic-ai/sdk` | `@anthropic-ai/claude-agent-sdk` `query()` | Agent SDK manages history, retries, tool execution, streaming — ~200 lines of loop logic replaced with 10 |
| GitHub issue creation | `curl`/`fetch` to GitHub REST API | `@octokit/rest` | Handles auth, pagination, error retries, TypeScript types, rate limit headers |
| Slash command registration | Raw REST calls to Discord API | `discord.js` `REST` + `Routes` | Handles token auth, API version, JSON serialization correctly |
| Discord message splitting (> 2000 chars) | Manual string chunking | discord.js embeds or use `SplitOptions` | Discord messages have a 2000-char limit; embeds can display up to 6000 chars total |

**Key insight:** The Claude Agent SDK is the single highest-leverage component — it eliminates the entire tool-use implementation burden and provides the multi-step file reading that makes codebase awareness possible.

---

## Common Pitfalls

### Pitfall 1: MessageContent Intent Not Enabled

**What goes wrong:** Bot receives @mention events but `message.content` is an empty string. The bot appears to respond to nothing.

**Why it happens:** Discord made `MessageContent` a privileged intent in April 2022. It must be enabled both in Gateway intents (code) AND in the Discord Developer Portal (bot settings page) under "Privileged Gateway Intents."

**How to avoid:** In the Discord Developer Portal at discord.com/developers/applications, go to the bot settings, enable "Message Content Intent." In code, include `GatewayIntentBits.MessageContent`.

**Warning signs:** `message.content === ''` when you expect text; works in DMs but not in guilds.

### Pitfall 2: Claude Agent SDK Working Directory for Multi-Repo Access

**What goes wrong:** Claude uses `Read` with a relative path and fails to find files because the cwd is wrong, or it finds files in one repo but not the other.

**Why it happens:** The Agent SDK's `Read`, `Glob`, and `Grep` tools resolve paths relative to the process cwd. When two repos are mounted at `/repos/all-chat` and `/repos/all-chat-extension`, the system prompt must explicitly tell Claude to use absolute paths.

**How to avoid:** Set `process.chdir('/repos/all-chat')` on startup (primary repo as default), and include in the system prompt: "The all-chat repo is at `/repos/all-chat`. The all-chat-extension repo is at `/repos/all-chat-extension`. Use absolute paths when reading from all-chat-extension."

**Warning signs:** Claude reads CLAUDE.md from wrong location, or says "I don't have access to the extension repo files."

### Pitfall 3: Discord 3-Second Slash Command Timeout

**What goes wrong:** Discord shows "This interaction failed" even though the bot eventually answers.

**Why it happens:** Discord requires a response to slash command interactions within 3 seconds. Claude queries take 5-30 seconds.

**How to avoid:** Always call `await interaction.deferReply()` immediately when receiving a `ChatInputCommandInteraction`. Then use `interaction.editReply()` with the final response. `deferReply()` sends a "thinking..." indicator and extends the window to 15 minutes.

**Warning signs:** "This application did not respond" errors in Discord for slash commands.

### Pitfall 4: @mention Content Includes the Bot Mention Tag

**What goes wrong:** Claude receives `<@1234567890> how do I set up YouTube?` as the question and gets confused by the mention syntax.

**Why it happens:** Discord includes the raw mention tag in `message.content`.

**How to avoid:** Strip mention tags before passing to Claude:
```typescript
const question = message.content.replace(/<@!?\d+>/g, '').trim();
```

**Warning signs:** Claude references `<@...>` in its answer or asks "what does `<@1234567890>` mean?"

### Pitfall 5: Kubernetes Volume Freshness

**What goes wrong:** Bot answers questions based on stale repo content (old code paths, removed files).

**Why it happens:** If repos are cloned once at pod startup via an init container, they drift as the actual repos are updated.

**How to avoid:** Use `kubernetes/git-sync` as a sidecar container. It continuously polls the remote repo and atomically updates a shared volume via symlink swap. The bot reads from `REPO_PATH/repo` (the current worktree symlink). Alternative: accept some staleness and have an init container pull on startup — sufficient if the bot doesn't need minute-level freshness.

**Warning signs:** Bot references files or patterns that were removed weeks ago.

### Pitfall 6: Claude Agent SDK and `Bash` Tool

**What goes wrong:** If `Bash` is accidentally included in `allowedTools`, Claude can execute arbitrary commands in the pod.

**Why it happens:** The SDK defaults to requiring explicit `allowedTools` specification, but a copy-paste from a demo might include `Bash`.

**How to avoid:** Explicitly restrict: `allowedTools: ['Read', 'Glob', 'Grep']`. Never include `Write`, `Edit`, or `Bash` in the support bot. This is a **security requirement**.

---

## Code Examples

### Full Entry Point Pattern (index.ts)

```typescript
// Pattern from existing services/discord-bot/src/index.js adapted for TypeScript
import { startBot } from './bot.js';

const REQUIRED_ENV = [
  'DISCORD_BOT_TOKEN',
  'ANTHROPIC_API_KEY',
  'GITHUB_TOKEN',
  'ALL_CHAT_REPO_PATH',
  'ALL_CHAT_EXTENSION_REPO_PATH',
];

for (const key of REQUIRED_ENV) {
  if (!process.env[key]) {
    console.error(`Missing required env var: ${key}`);
    process.exit(1);
  }
}

process.on('SIGINT', shutdown);
process.on('SIGTERM', shutdown);

async function shutdown() {
  console.log('Shutting down support-bot...');
  // destroy Discord client
  process.exit(0);
}

startBot().catch((err) => {
  console.error('Failed to start support-bot:', err);
  process.exit(1);
});
```

### Octokit Issue Body Template

```typescript
// Claude's Discretion — recommended template
function buildIssueBody(
  question: string,
  proposedChange: string,
  discordUser: string,
  channelId: string
): string {
  return `## Context

This issue was automatically created by the All-Chat support bot in response to a user question.

**Discord user:** ${discordUser}
**Channel:** ${channelId}

## Original Question

${question}

## Proposed Change

${proposedChange}

---
*This issue was proposed by the support bot. A human must review and implement any changes.*`;
}
```

---

## Kubernetes Deployment Notes

### Environment Variables Required

```yaml
# ConfigMap keys (non-sensitive)
- name: ALL_CHAT_REPO_PATH
  value: /repos/all-chat
- name: ALL_CHAT_EXTENSION_REPO_PATH
  value: /repos/all-chat-extension
- name: GITHUB_OWNER
  value: moersener  # or org name
- name: GITHUB_REPO_ALLCHAT
  value: all-chat
- name: GITHUB_REPO_EXTENSION
  value: all-chat-extension

# Secret keys (from allchat-secrets)
- name: DISCORD_BOT_TOKEN
  valueFrom:
    secretKeyRef:
      name: allchat-secrets
      key: SUPPORT_BOT_DISCORD_TOKEN
- name: ANTHROPIC_API_KEY
  valueFrom:
    secretKeyRef:
      name: allchat-secrets
      key: ANTHROPIC_API_KEY
- name: GITHUB_TOKEN
  valueFrom:
    secretKeyRef:
      name: allchat-secrets
      key: SUPPORT_BOT_GITHUB_TOKEN
```

### Volume Strategy: init container git clone (simpler)

```yaml
initContainers:
- name: clone-all-chat
  image: alpine/git
  command: ['git', 'clone', '--depth', '1', 'https://$(GITHUB_TOKEN)@github.com/$(GITHUB_OWNER)/all-chat.git', '/repos/all-chat']
  volumeMounts:
  - name: all-chat-repo
    mountPath: /repos/all-chat
volumes:
- name: all-chat-repo
  emptyDir: {}
```

For continuous freshness, replace init containers with `kubernetes/git-sync` sidecar (image: `registry.k8s.io/git-sync/git-sync:v4.x`).

### No Health Check HTTP Server Needed

The existing discord-bot service uses `GatewayIntentBits.Guilds` only — it has no HTTP health endpoints. For the support bot, either:
1. Add a minimal `express` or `http` health endpoint at `/health/live` (matches project convention), or
2. Use Kubernetes `exec` liveness probe checking process health.

The project convention (kick-listener, overlay-manager) strongly prefers HTTP health endpoints. Recommendation: add a minimal HTTP server on a configurable port.

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `@anthropic-ai/claude-code` npm package | `@anthropic-ai/claude-agent-sdk` (renamed) | 2025-2026 | Same SDK, new name — Agent SDK overview docs confirm migration guide exists |
| Claude Code OAuth (`sk-ant-oat01-*`) for server-side use | `ANTHROPIC_API_KEY` required | 2026-01-09 | OAuth blocked by Anthropic policy enforcement; API key is the only supported path |
| `message` event in discord.js | `messageCreate` event | discord.js v13→v14 | Old event name removed; use `Events.MessageCreate` constant |
| `interaction` event | `interactionCreate` event | discord.js v13→v14 | Old event name removed |

**Deprecated/outdated:**
- `@anthropic-ai/claude-code` npm package: now named `@anthropic-ai/claude-agent-sdk` — install the new name
- Claude Code OAuth tokens for server-side bots: blocked January 9, 2026; use ANTHROPIC_API_KEY instead

---

## Open Questions

1. **Session continuity per Discord user**
   - What we know: Agent SDK supports `resume: sessionId` to continue a session across calls
   - What's unclear: Whether per-user session continuity is valuable vs. fresh context per question; sessions consume additional token budget for history
   - Recommendation: Start with fresh context per question (simpler, no state management). Add per-channel threading in a follow-up if users complain about context loss.

2. **Rate limiting for concurrent Claude calls**
   - What we know: Multiple Discord users could trigger Claude queries simultaneously; API rate limits apply
   - What's unclear: The Anthropic API rate limits for claude-sonnet-4-6 on a new API key
   - Recommendation: Add a simple in-memory queue limiting to 1-2 concurrent Claude calls. Surface a "please wait" message when queued.

3. **Repository freshness cadence**
   - What we know: init container clones are stale after first deploy; git-sync sidecars add complexity
   - What's unclear: How often the repos actually change (multiple times per day vs. weekly)
   - Recommendation: Start with init container clone (simple). Add git-sync sidecar only if the bot gives demonstrably stale answers.

4. **GitHub PAT permissions scope**
   - What we know: `@octokit/rest` needs a token to create issues in both repos
   - What's unclear: Whether `all-chat` and `all-chat-extension` are in the same GitHub account — if so, one PAT with `repo` scope covers both
   - Recommendation: Create a fine-grained GitHub PAT with "issues: write" on both repos specifically.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Vitest (TypeScript-native, ES module compatible — aligns with `type: "module"` project pattern) |
| Config file | `vitest.config.ts` — Wave 0 gap |
| Quick run command | `npm test -- --run` |
| Full suite command | `npm test` |

### Phase Requirements to Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| BOT-01 | @mention triggers handleQuestion | unit | `npm test -- --run src/__tests__/bot.test.ts` | Wave 0 gap |
| BOT-02 | Slash command /support triggers handleQuestion | unit | `npm test -- --run src/__tests__/bot.test.ts` | Wave 0 gap |
| BOT-03 | Claude query with read-only tools returns string response | unit (mocked SDK) | `npm test -- --run src/__tests__/agent.test.ts` | Wave 0 gap |
| BOT-04 | PROPOSE_ISSUE response triggers issue creation | unit | `npm test -- --run src/__tests__/github.test.ts` | Wave 0 gap |
| BOT-05 | Missing env vars cause process.exit(1) | unit | `npm test -- --run src/__tests__/index.test.ts` | Wave 0 gap |
| BOT-06 | Graceful shutdown on SIGTERM | unit | `npm test -- --run src/__tests__/index.test.ts` | Wave 0 gap |

### Sampling Rate

- **Per task commit:** `npm test -- --run`
- **Per wave merge:** `npm test`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `src/__tests__/bot.test.ts` — covers BOT-01, BOT-02
- [ ] `src/__tests__/agent.test.ts` — covers BOT-03 (mock `@anthropic-ai/claude-agent-sdk` query)
- [ ] `src/__tests__/github.test.ts` — covers BOT-04 (mock `@octokit/rest`)
- [ ] `src/__tests__/index.test.ts` — covers BOT-05, BOT-06
- [ ] `vitest.config.ts` — no test infrastructure exists yet
- [ ] Framework install: `npm install -D vitest` — needed in package.json

---

## Sources

### Primary (HIGH confidence)

- [Anthropic Agent SDK Overview](https://platform.claude.com/docs/en/agent-sdk/overview) — authentication, allowedTools, query() API, read-only permissions pattern, TypeScript examples
- [GitHub issue #37205 anthropics/claude-code](https://github.com/anthropics/claude-code/issues/37205) — OAuth token blocked for programmatic use; "OAuth authentication is currently not supported"
- [npm: @anthropic-ai/claude-agent-sdk](https://www.npmjs.com/package/@anthropic-ai/claude-agent-sdk) — version 0.2.81 verified
- [npm: discord.js](https://www.npmjs.com/package/discord.js) — version 14.25.1 verified
- [npm: @octokit/rest](https://www.npmjs.com/package/@octokit/rest) — version 22.0.1 verified
- `services/discord-bot/src/index.js` — existing patterns: ES modules, graceful shutdown, env validation
- `services/discord-bot/package.json` — existing discord.js ^14.14.1 version confirmed
- `deployments/k8s/base/kick-listener/deployment.yaml` — established secret/configmap pattern for env vars

### Secondary (MEDIUM confidence)

- [discord.js Guide — slash commands](https://discordjs.guide/creating-your-bot/slash-commands.html) — slash command registration and deployment patterns verified against official guide
- [discord.js Guide — command deployment](https://discordjs.guide/creating-your-bot/command-deployment.html) — guild vs global registration
- [kubernetes/git-sync GitHub](https://github.com/kubernetes/git-sync) — sidecar approach for repo freshness
- [daveswift.com — Claude OAuth Update](https://daveswift.com/claude-oauth-update/) — confirms OAuth infrastructure is being dismantled; API key is the only supported path

### Tertiary (LOW confidence)

- Community reports on Discord rate limits (5 messages/5s per channel) — not from official Discord docs, but widely consistent across sources

---

## Metadata

**Confidence breakdown:**

- Standard stack: HIGH — npm versions verified directly; Agent SDK docs read from official source
- Architecture: HIGH — patterns derived from existing project code + official SDK docs
- OAuth blocking: HIGH — confirmed via GitHub issue + official docs note + multiple news sources
- Pitfalls: MEDIUM — some (MessageContent intent, deferReply) verified from official discord.js guide; rate limits from community consensus
- Kubernetes strategy: MEDIUM — based on existing project deployment patterns; git-sync approach from official kubernetes/git-sync repo

**Research date:** 2026-03-24
**Valid until:** 2026-04-24 (stable stack; OAuth policy is locked but Agent SDK may have minor version updates)
