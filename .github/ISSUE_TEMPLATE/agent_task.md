---
name: Agent task
about: A task written so an autonomous agent (Caterpillar) can pick it up, implement it, and be graded without a human in the loop
title: "<service or area>: <what changes>"
labels: ""
assignees: ""
---

<!--
=============================================================================
 HOW THIS TEMPLATE WORKS  (delete this comment block before submitting)
=============================================================================

Caterpillar polls for issues labelled `agent` ("Caterpillar may claim this")
and turns each one into a task. Labels it manages afterwards:

  agent         you added it: the issue is claimable
  agent-wip     a runner holds the task (Caterpillar adds/removes this)
  needs-human   the agent parked on a question and is waiting on you

DO NOT add the `agent` label until the issue is finished. Intake runs on a
short poll, so a half-written issue that carries the label can be claimed
before you finish editing it. Write the body first, label last.

An issue WITHOUT the `agent` block below is rejected with a comment, because
a task with no machine-checkable acceptance criteria can never be marked done.

Definition of done is three independent gates, and the agent controls none of
them. It can only *claim* completion:

  1. every `acceptance` command exits 0  (run by the supervisor, not the agent)
  2. a PR is open and CI is green
  3. a three-lens review council reads the diff; any blocking objection sends
     the work back, and the task parks after 3 rounds

So the prose is not a wish list. It is the brief that decides whether the
change that comes back is the change you wanted.
=============================================================================
-->

## Context

<!--
Why this exists, grounded in things the agent can go and read. This section
carries the weight: an agent that starts from a claim it can verify writes a
different change than one starting from a vague complaint.

- Cite `path/to/file.go:123` and quote the few lines that matter.
- Include the log line, the alert expression, the metric, the user report.
- VERIFY every reference before submitting. A stale line number sends the
  agent to the wrong place, and a refuted premise makes it implement a fix
  for a problem that does not exist.
- If part of the original report turns out to be WRONG, say so here in as
  many words. "Refuted: the buffer is Redis-backed, so there is no per-pod
  replay gap" saves an entire wasted session.
- If some of it is ALREADY FIXED, say that too, and name the commit.
-->

## Proposal

<!--
What to change, as checkboxes. Be specific about the things the agent cannot
infer and would otherwise invent:

- Pin names that are a contract between two places (a metric name, a JSON
  key, a settings key, a close code). If the producer and the consumer must
  agree on a string, write the string here rather than leaving it to chance.
- State defaults, and say what an absent value must resolve to.
- Say where new code goes, and prefer a pure module with its own test over
  logic buried in a component or a handler.
- Recommend the technique when one approach degrades better than another,
  and say WHY, so the agent does not trade it for a plausible-looking
  alternative that breaks under a case you already thought about.
-->

- [ ] 

## Out of scope

<!--
The adjacent work you do NOT want touched, and any tempting-but-wrong fix.
Without this the agent widens the diff to be helpful and the review council
sends it back for being unfocused.
-->

## Notes for the agent

<!--
Environment facts, not requirements. Everything here exists because it has
bitten a run before.

- `repos[0]` is the working directory and the repo whose CI is gate 2. Every
  other entry in `repos` is checked out beneath it at `repos/<name>`, so
  paths into a sibling repo are prefixed that way (in the prose AND in the
  acceptance commands).
- Acceptance commands run in the runner's container, from `repos[0]`'s root,
  with a 15-minute timeout each. A missing interpreter makes the gate
  UNSATISFIABLE rather than failed: nothing the agent does inside a session
  can fix `env: 'python3': No such file or directory`. If a command needs a
  toolchain that the target repo's flake does not provide, do not gate on it
  — assert the artefact exists instead, and run the real thing yourself.
- Known `requires` capabilities: linux, k8s, net, gpu, usb, human-present,
  nix. These are MACHINE properties, not languages. A typo parks the task
  forever, because no runner will ever match it.

Repo-specific traps that have already cost a run:

- **Never `npm test` in `frontend/`.** It maps to bare `vitest`, which is
  watch mode and never returns without a tty. Use
  `npx vitest --project unit --run`.
- **Playwright is not runnable at gate time.** Its Chromium closure is ~2.3 GB
  and deliberately absent from the dev shell, so gate only on the spec FILE
  existing (`test -f frontend/tests/e2e/foo.spec.ts`) and let CI run the suite.
- **`npm ci` must be its own first command** when later commands rely on the
  `node_modules` it writes.
- **Quote `[id]` path segments** in shell gates. Unquoted, `[id]` is a glob
  character class and silently matches nothing.
- **Go builds:** `GOTOOLCHAIN=go1.25.7`. The default go1.26.5 `nodwarf5`
  toolchain mis-loads modules and reports bogus "undefined" errors.
- **Migrations re-run on every pod start**, so a non-idempotent migration
  crash-loops every fresh pod.
-->

## Related

<!-- Prior issues/PRs/commits/ADRs, and one line on how each bears on this. -->

<!--
=============================================================================
 THE AGENT BLOCK  (required)
=============================================================================

Caterpillar strips this block out of the goal it hands the agent — it is
configuration, not instruction, and the acceptance commands are not the
agent's to reinterpret. So never write "see the block above" in the prose:
the agent cannot see it.

Keys:

  repos       Required here (all-chat issues would default to this repo, but
              be explicit). First entry is the working directory. Add a
              sibling repo only if the change genuinely spans both, since
              every extra repo widens the token scope.
  acceptance  Required, at least one. Commands that must exit 0.
  requires    Optional. Capabilities a runner must have to claim this.
  toolchain   Optional. `{mode: nix|inherit, packages: [...]}`. Usually
              omitted: a repo with its own flake.nix is detected already.
              An explicit `mode: nix` implies `requires: [nix]`.

Write acceptance commands that FAIL on today's code. A gate that already
passes proves nothing, and a suite-wide `go test ./...` tells you the repo
still builds, not that the task was done. Mix:

  - the real gates:  build, vet, typecheck, lint, the targeted test file
  - existence gates: `test -f` for a file the change must add
  - contract gates:  `grep -q` for a pinned name that two places must share

Quote every command. YAML will otherwise coerce something and intake rejects
a non-string entry rather than silently shrinking your gate.
=============================================================================
-->

```agent
repos:
  - caesarakalaeii/all-chat
acceptance:
  - "the command that must exit 0"
```
