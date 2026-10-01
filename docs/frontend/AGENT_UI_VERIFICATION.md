# Agent UI verification for the beta frontend

Status: active. The loop described here is the contract for any agent
session that changes something a user will see.

## Why this exists

The static gates (lint, design tokens, contrast lock, axe) all inspect
source or the accessibility tree. None of them can see a collapsed layout, a
button twice the width of its form, or a nav that stacks into oblivion at
375px — and every one of those shipped to production while all checks were
green. Two additions close that:

1. **The pixel gate** — `frontend/tests/e2e/visual-regression.spec.ts`,
   asserted on every PR by `.github/workflows/frontend-visual.yml`. Fails
   the PR when a chrome route renders differently than its committed
   baseline.
2. **The eyes ritual** — the agent loop below, run on every UI change
   before the completion claim. The gates bite in CI; the ritual catches the
   defect while the agent is still in the room.

## The loop (every UI change)

1. **Implement** the change.
2. **Render it**: `nix develop -c bash -c 'cd frontend && npm run dev'` (or
   attach to an already-running dev server; never kill one you did not
   start). Note the PID if you started it.
3. **Shoot it** at 1280px and 375px, desktop and mobile both. Use
   `skills/ui-review`'s `shoot.py` when available, Playwright MCP
   (`browser_take_screenshot`) when wired (see AGENT_MCP_PLAN.md), or a
   throwaway Playwright script. Mobile width is not optional: the overflow
   nobody scrolled to see lives there.
4. **READ the screenshots.** A screenshot not read is a screenshot not
   taken. Judge against the checklist, not against "looks fine":
   - renders as what it claims to be, not an unstyled wall (broken bundle)?
   - no content overflowing its container or colliding?
   - no horizontal scroll at 375px?
   - empty/error/loading states covered, not just the happy path?
   - spacing consistent with adjacent components (two cards with different
     paddings is a finding)?
   - no console errors under the surface that looks fine?
5. **Fix and re-shoot** with a new label. One fix, one re-shoot — a fix
   without a re-shoot is a claim without evidence.
6. **Update baselines** when the change is deliberate: dispatch
   `frontend-visual.yml` on your branch with `update_snapshots=true`,
   download the artifact, commit the new PNGs in the same PR as the visual
   change that justifies them. A baseline update without its justifying
   change in the diff is a red flag in review, deliberately.
7. **Kill the server you started** (the exact PID), leave the operator's
   alone.

## Scope

- **In scope**: everything except `src/app/overlay/**`, the embed preview
  and `ThemePreview` — those are user-themable broadcast art with their own
  contrast floor (`theme-contrast.spec.ts`) and are not baselined or judged
  against chrome tokens (see `frontend/DESIGN_SYSTEM.md`).
- **Primitive-agnostic**: the loop and the pixel gate do not care whether a
  `<Button>` comes from a registry, Base UI or a hand-rolled component. The
  beta is moving off shadcn; the token vocabulary, `cn()`, the
  focus-visible rule and the no-raw-palette rule stay regardless (ADR-0056
  contract).

## Tool choice

| Need | Tool |
| --- | --- |
| Repeatable interaction/verification in the agent loop | Playwright MCP (see AGENT_MCP_PLAN.md) |
| Debugging: console, network, perf traces | Chrome DevTools MCP (see AGENT_MCP_PLAN.md) |
| Batch pixel gate on PRs | `frontend-visual.yml` / `npm run test:visual` |
| Baseline regeneration | dispatch `frontend-visual.yml` with `update_snapshots=true` |
| Local run on NixOS | `PLAYWRIGHT_CHROMIUM_PATH=<nix chromium> npm run test:visual` |

## What "done" means

A UI change is done when: the screenshots were taken at both widths, read,
judged against the checklist; defects found were fixed and re-shot; the pixel
gate's baselines were updated (via CI dispatch) if the change is deliberate;
and the handback names the final screenshots read.

## Baseline policy

- Baselines are generated on CI only — the committed PNGs must come from
  the browser/font stack CI pins. A dev-host baseline is a baseline CI
  rejects.
- The first PR run on a new branch fails on missing baselines. That is the
  gate biting, not a flake: dispatch with `update_snapshots=true`, commit
  the PNGs.
- Baseline diffs without a visual change in the diff are treated as suspect
  in review: either nondeterminism crept in (fix the spec) or someone is
  sneaking a change past the gate.
