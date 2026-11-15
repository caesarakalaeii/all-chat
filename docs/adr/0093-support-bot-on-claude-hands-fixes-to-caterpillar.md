# ADR-0093: Support bot runs on Claude and hands confirmed fixes to Caterpillar

**Date**: 2026-11-15
**Status**: Accepted
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
- Anthropic's terms reserve Claude subscription OAuth for Claude Code and Anthropic's own
  apps; a third-party bot must use a Console API key
  ([legal and compliance](https://code.claude.com/docs/en/legal-and-compliance)).
- The agent loop, its fail-closed tool registry and the redaction boundaries must not
  change: they are the bot's security model.

## Considered Options

1. **Claude over the Anthropic Messages API with a Console API key**, as a second
   `llm.ChatClient` implementation selected by `ANTHROPIC_API_KEY`.
2. Point the existing OpenAI-compatible client at Anthropic's OpenAI compatibility
   endpoint. No code, but no prompt caching control, no refusal fallbacks, and Anthropic
   documents it as a compatibility layer rather than the production API.
3. Run the Claude Code CLI with the subscription OAuth token, as the bot did before it was
   rewritten in Go. The bot would answer Discord users from the maintainer's personal
   subscription, which the terms do not allow, and the Go tool registry would have to be
   rebuilt as an MCP server.

## Decision Outcome

Chosen: **option 1**, plus agent-task filing.

**Model.** `llm/anthropic.go` implements `ChatClient` with the official Go SDK
(`anthropic-sdk-go/v2`, beta Messages endpoint). It translates the loop's OpenAI-shaped
transcript: system messages become the top-level `system`, each run of `role: "tool"`
messages becomes one user turn of `tool_result` blocks, and every assistant turn keeps its
native content in `Message.Native` so Claude's thinking blocks are replayed unchanged
(they are bound to the conversation that produced them). Requests set top-level
`cache_control` (the loop resends a growing prefix), `output_config.effort`
(`ANTHROPIC_EFFORT`, default `medium`) and server-side refusal fallbacks
(`fallbacks: "default"`); a refusal still ends the turn with a visible answer. Errors map
onto the existing `llm.APIError` kinds, so a 429 takes the same busy-wait and park path as
the gateway's. Defaults: `claude-opus-5-5`, 16000 tokens per reply. Without
`ANTHROPIC_API_KEY` the bot uses the gateway exactly as before.

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
- Bad: per-token spend on the Anthropic API, and user messages now go to Anthropic (US).
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
- [ADR-0092](./0092-distroless-runtime-images.md): the support-bot image stays distroless; the
  SDK is pure Go.
