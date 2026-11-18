# ADR-0093: Support bot runs on Claude and hands confirmed fixes to Caterpillar

**Date**: 2026-11-15
**Status**: Accepted
**Revised**: 2026-11-15
**Deciders**: caesarakalaeii

## Context

The support bot (`services/support-bot`) answered over an OpenAI-compatible inference
gateway. In practice that gateway was too unstable to leave the bot unattended: it refuses
hobby traffic with 429s while production load is high, and swaps or retires the model
behind an alias. The maintainer is about to be away for a week, and the bot has to keep
answering and get confirmed bugs fixed without a human in the loop.

Caterpillar (the caesar-deployment coding agent) already implements issues labelled
`agent` against their acceptance commands, CI and a three-lens review council, and merges
what it approves. The bot could file issues but never labelled them, so nothing it found
was ever fixed automatically.

Constraints: the bot must keep working when the gateway is degraded, and bug reports
that arrive in Discord should turn into fixes without a maintainer (the maintainer is
typically away for stretches); the subscription (OAuth grant) is preferred over per-token
API billing; Anthropic accepts a subscription token only from Claude Code itself, so a
third-party client would have to pose as Claude Code, which this project does not do; and
the agent loop's fail-closed tool registry and redaction boundaries must not change (they
are the bot's security model).

## Considered options

- Claude Code CLI on the subscription: run the unmodified `claude` binary with a
  `claude setup-token` token and serve the bot's tool registry over MCP.
- Claude over the Anthropic Messages API with a Console API key, as a second
  `llm.ChatClient` implementation selected by `ANTHROPIC_API_KEY`.
- Call the Messages API directly with the subscription token (Caterpillar's pi-ai mode
  does): rejected, it works only by posing as Claude Code, which this project does not
  do.
- Point the existing OpenAI-compatible client at Anthropic's OpenAI compatibility
  endpoint: rejected, no prompt caching control, no refusal fallbacks, and Anthropic
  documents it as a compatibility layer, not the production API.

## Decision

Chosen: **option 1 in production, option 2 as the fallback**, plus agent-task filing.
Precedence in `cmd/main.go`: `CLAUDE_CODE_OAUTH_TOKEN`, then `ANTHROPIC_API_KEY`, then the
gateway (`LOCAL_LLM_*`).

**Claude Code** (`agent/claudecode.go`): per question the bot runs `claude -p
--output-format json` with `--system-prompt` (its own), `--tools ""`, `--strict-mcp-config`,
`--allowedTools mcp__allchat`, `--max-turns` = `SUPPORT_BOT_MAX_ITERATIONS`,
`--no-session-persistence`, in an empty cwd with an environment of only `PATH`, `HOME` and
the token. The bot serves its registry on a 127.0.0.1 MCP endpoint (minimal Streamable
HTTP: `initialize`, `tools/list`, `tools/call`, `ping`); each run registers a random
bearer token bound to the requester's `tool.ToolCtx`, so `tools/call` goes through the
same per-mode gate, redaction, repeat guard and effect collection as the in-process loop.
A rate limit maps to `llm.KindBusy` and parks the request; resumption reruns the CLI from
the question. The pinned, checksum-verified binary puts support-bot on `distroless/cc`
(ADR-0092).

**Messages API** (`llm/anthropic.go`, used when only `ANTHROPIC_API_KEY` is set): the
official Go SDK (`anthropic-sdk-go/v2`) behind `llm.ChatClient`, translating the
OpenAI-shaped transcript (system to top-level, tool runs to `tool_result` blocks),
keeping assistant native content so thinking blocks replay unchanged, and set with
`cache_control`, `output_config.effort` (`ANTHROPIC_EFFORT`, default `medium`) and
refusal fallbacks. Defaults `claude-opus-5-5`, 16000 tokens per reply.

**Fixes.** The `github` tool gains `search_issues` and an `agent_task` flag on
`create_issue`; an agent task is refused in code unless its body carries a fenced `agent`
block with `acceptance` commands, and is labelled `agent` + `bot-proposed`. The prompt
tells the bot to understand a report first (up to three clarifying questions), to file an
agent task only when the evidence confirms the defect and the reporter confirmed the
summary in-thread, to search for an existing issue first, and to follow the template.
Questions, user error, platform outages and anything needing secrets, database writes or
a product decision stay answers or plain issues.

## Consequences

- The bot no longer depends on the gateway, and Discord bug reports can become merged
  fixes with no maintainer involved.
- Accepted by the maintainer: Anthropic's terms reserve subscription OAuth for ordinary
  Claude Code use ([legal and compliance](https://code.claude.com/docs/en/legal-and-compliance))
  and may be enforced without notice; if the token stops working, set `ANTHROPIC_API_KEY`
  and drop `CLAUDE_CODE_OAUTH_TOKEN` (option 2 is built). User messages now go to
  Anthropic (US); the privacy policy names it.
- Accepted by the maintainer: any Discord user can steer the bot into filing an agent
  task, which runs under the bot's owner GitHub token and may merge to `main` with only
  acceptance gates, CI and the review council as checks. Narrowing: restrict `agent_task`
  to `access.ModeAdmin` in `tools/github.go`.

## Related

- Caterpillar's own switch back to the Claude subscription: caesar-deployment
  `apps/workloads/caterpillar/README.md` (section 7 records the gateway path).
- [ADR-0092](./0092-distroless-runtime-images.md): support-bot moves from `distroless/static`
  to `distroless/cc` for the Claude Code binary.
