# Onlook pilot for the beta frontend

Status: pilot proposal, not adopted. Try it on ONE page before making it a
workflow. Verify the caveat below first — it is the go/no-go criterion.

## What it is

Onlook (https://github.com/onlook-dev/onlook, local-first, open source) is a
Figma-style visual editor that edits the *actual* Next.js + Tailwind code in
this repo: drag-adjust a spacing, get a real diff on a real branch. For the
beta's "make the chrome look professional" work — many small visual nudges
across dashboard, settings, editor — that is the missing piece between
"agent writes code blind" and "agent screenshots the result": a human or
agent can adjust visually and the commit stays reviewable.

Deliberate fit constraints, checked against this repo:

- Fits: Next.js + Tailwind, local-first, agent-compatible (works from a
  branch in a worktree).
- The drag-to-code fidelity was built against Tailwind v3; v4 support has
  been its active work item. **This repo is Tailwind v4** (`tailwindcss
  ^4.1.18` in `frontend/package.json`). The pilot's first task is to verify
  drag edits emit correct v4 utilities on one real page.

## Pilot protocol

1. Install Onlook locally (it is an app, not a dependency — nothing lands
   in `package.json`).
2. Open ONLY the `/settings` page in it. Settings is chrome (not broadcast
   art), moderately complex, and already pixel-baselined — the baseline
   gives an objective read on what the drag edits did.
3. Make three drag edits (a padding, a gap, a font size).
4. Check the diffs Onlook produced:
   - only token utilities, no raw palette values (ADR-0056 rule),
   - `cn()`/composition conventions respected, no second pattern beside
     existing ones,
   - no `!important` or inline-style escapes.
5. Run `npm run test:visual` locally and dispatch the baseline update if
   the changes are keepers. The pixel gate is the pilot's acceptance test —
   if Onlook's edits produce a clean baseline diff, it can stay; if they
   produce token violations or broken utilities, it goes.

## Relationship to the rest of the tooling

- The pixel gate (`frontend-visual.yml`) judges Onlook's output the same way
  it judges an agent's or a human's. No tool is exempt from the gate.
- The shadcn caveat: Onlook edits the rendered markup whatever primitive
  produced it, so the beta's move off shadcn does not interact with the
  pilot.
- The ui-review ritual (`AGENT_UI_VERIFICATION.md`) still applies on top:
  Onlook output gets screenshotted and read like any other change.

## Decision

Adopt for chrome work if the pilot passes; keep to code-only otherwise. The
decision is the operator's after the pilot, not a session's.
