# ADR-0093: Support bot runs on Claude and hands confirmed fixes to Caterpillar

**Date**: 2026-11-15
**Status**: Accepted
**Revised**: 2026-11-15
**Deciders**: caesarakalaeii

## Context and Problem Statement

The support bot (`services/support-bot`) answered over an OpenAI-compatible inference
gateway. In practice that gateway was too unstable to leave the bot unattended: it refuses
hobby traffic with 429s while production load is high, and swaps or retires the model
behind an alias. The maintainer is about to be away for a week, and the bot has to keep
answering and get confirmed bugs fixed without a human in the loop.

Caterpillar, the autonomous coding agent deployed from caesar-deployment, already
implements issues labelled `agent` that follow `.github/ISSUE_TEMPLATE/agent_task.md`,
gates them on their acceptance commands, CI and a three-lens review council, and merges
what the council approves. The bot could file issues but never labelled them, so nothing
it found was ever fixed automatically.

## Decision Drivers

- The bot must keep working when the gateway is degraded.
- Bug reports that arrive in Discord should turn into fixes without a maintainer.
- The maintainer wants the bot on the Claude subscription (OAuth grant), not per-token
  API billing.
- Anthropic accepts a subscription token only from Claude Code itself; a third-party
  client can use it only by posing as Claude Code, which this project does not do.
- The agent loop's fail-closed tool registry and the redaction boundaries must not
  change: they are the bot's security model.

## Considered Options

1. **Claude Code CLI on the subscription**: run the unmodified `claude` binary with a
   `claude setup-token` token and serve the bot's tool registry to it over MCP.
2. **Claude over the Anthropic Messages API with a Console API key**, as a second
   `llm.ChatClient` implementation selected by `ANTHROPIC_API_KEY`.
3. Call the Messages API directly with the subscription token, as Caterpillar's pi-ai
   subscription mode does. Rejected: it works only by sending Claude Code's identity
   (its user agent, its beta header and its system prompt) from a client that is not
   Claude Code.
4. Point the existing OpenAI-compatible client at Anthropic's OpenAI compatibility
   endpoint. Rejected: no prompt caching control, no refusal fallbacks, and Anthropic
   documents it as a compatibility layer rather than the production API.

## Decision Outcome

Chosen: **option 1 in production, option 2 as the fallback**, plus agent-task filing.
Precedence in `cmd/main.go`: `CLAUDE_CODE_OAUTH_TOKEN`, then `ANTHROPIC_API_KEY`, then the
gateway (`LOCAL_LLM_*`).

**Claude Code** (`agent/claudecode.go`). Per question the bot runs
`claude -p --output-format json` with `--system-prompt` (the bot's own),
`--tools ""` (no shell, file or web tools), `--strict-mcp-config`,
`--allowedTools mcp__allchat`, `--max-turns` = `SUPPORT_BOT_MAX_ITERATIONS` and
`--no-session-persistence`, in an empty working directory, with an environment of only
`PATH`, `HOME` and the token. The bot serves its registry on a 127.0.0.1 MCP endpoint (a
minimal Streamable HTTP server with JSON responses: `initialize`, `tools/list`,
`tools/call`, `ping`). Each run registers a random bearer token bound to the requester's
`tool.ToolCtx`, so `tools/list` and `tools/call` go through the same per-mode gate,
redaction, repeat guard and effect collection as the in-process loop. A usage or rate
limit maps to `llm.KindBusy` and parks the request; a resumed request reruns the CLI from
the question. The image ships the pinned, checksum-verified binary and therefore runs on
`distroless/cc` ([ADR-0092](./0092-distroless-runtime-images.md)).

**Messages API** (`llm/anthropic.go`), used when only `ANTHROPIC_API_KEY` is set:
the official Go SDK (`anthropic-sdk-go/v2`, beta Messages endpoint) behind
`llm.ChatClient`. It translates the loop's OpenAI-shaped transcript: system messages
become the top-level `system`, each run of `role: "tool"` messages becomes one user turn
of `tool_result` blocks, and every assistant turn keeps its native content in
`Message.Native` so Claude's thinking blocks are replayed unchanged. Requests set
top-level `cache_control`, `output_config.effort` (`ANTHROPIC_EFFORT`, default `medium`)
and server-side refusal fallbacks (`fallbacks: "default"`). Defaults: `claude-opus-5-5`,
16000 tokens per reply.

**Fixes.** The `github` tool gains `search_issues` and an `agent_task` flag on
`create_issue`. An agent task is refused in code unless its body carries a fenced
`agent` block with `acceptance` commands (Caterpillar rejects anything else), and is
labelled `agent` + `bot-proposed` instead of `needs-review`. The system prompt tells both
access modes when to file one (a confirmed defect or a small, scoped improvement a code
or manifest change fixes), to search for an existing issue first, and to follow the
template. Questions, user error, platform-side outages and anything needing secrets,
database writes or a product decision stay answers or plain issues.

### Consequences

- Good: the bot no longer depends on the gateway's availability, and Discord bug reports
  can become merged fixes with no maintainer involved.
- Bad, accepted by the maintainer: Anthropic's terms reserve subscription OAuth for
  ordinary use of Claude Code and do not allow routing other people's requests through a
  personal plan ([legal and compliance](https://code.claude.com/docs/en/legal-and-compliance)),
  and Anthropic may enforce that without notice. If the token stops working, set
  `ANTHROPIC_API_KEY` and drop `CLAUDE_CODE_OAUTH_TOKEN`: option 2 is already built.
- Bad: user messages now go to Anthropic (US).
  The privacy policy (Section 2.8, the recipient list and the transfer list) names it.
- Bad, accepted by the maintainer: any Discord user can steer the bot into filing an
  agent task. The bot's GitHub token is the owner's, so the issue counts as
  owner-authored and Caterpillar runs it: its acceptance commands run as shell on a runner,
  and an approved change merges to `main` and deploys. Acceptance gates, CI and the review
  council are the only checks. To narrow it, restrict `agent_task` to `access.ModeAdmin`
  in `tools/github.go`.

## Related

- Caterpillar's own switch back to the Claude subscription: caesar-deployment
  `apps/workloads/caterpillar/README.md` (section 7 records the gateway path).
- [ADR-0092](./0092-distroless-runtime-images.md): support-bot moves from `distroless/static`
  to `distroless/cc` for the Claude Code binary.
