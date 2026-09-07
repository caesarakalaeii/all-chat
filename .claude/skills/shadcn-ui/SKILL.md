---
name: shadcn-ui
description: >-
  Build or change All-Chat frontend UI so it stays visually consistent: pick the
  right shadcn primitive, use design tokens instead of hand-rolled Tailwind, and
  verify nothing compiles to dead CSS. Use whenever adding or editing a page,
  component, button, form, dialog, table or any styled markup under frontend/,
  when adding a shadcn component, when something "looks off"/inconsistent, or
  when reviewing a frontend diff for design drift. Keywords: shadcn, design
  system, Tailwind, design tokens, component, button, input, form, dialog,
  styling, UI consistency, globals.css, primitives, variant.
---

# Building consistent UI in All-Chat

The reference is `frontend/DESIGN_SYSTEM.md`; the reasoning is
[ADR-0056](../../../docs/adr/0056-shadcn-token-vocabulary-as-the-design-system-contract.md).
This is the procedure.

## Why this skill exists

Tailwind has no unknown-class error. Write `bg-primary` when no
`--color-primary` token exists and Tailwind emits **nothing** — build passes,
types pass, lint passes, element renders unstyled. That silent failure once put
119 dead utilities in this codebase, including a `<Button>` with no background,
which is why the codebase then grew **66 distinct hand-written button styles**.

So: **never invent a class name, and verify before you claim it works.**

---

## Before writing markup

Read `frontend/DESIGN_SYSTEM.md` and check what already exists:

```bash
cd frontend
ls src/components/ui/                          # primitives on hand
grep -oE '^\s*--color-[a-z0-9-]+' src/app/globals.css | sort -u   # real tokens
```

Do not guess a token from memory — this project renamed its palette once
already, and stale names are the failure mode.

---

## The decision, in order

**1. Is there a primitive?** Use it. `<Button>`, `<Input>`, `<Card>`, `<Dialog>`,
`<Tabs>`, `<Select>`, `<Field>`, `<Switch>`, `<Badge>`…

**2. Does the registry have one?**

```bash
npx shadcn@4 search @shadcn -q "combobox"
npx shadcn@4 view @shadcn/combobox      # read before installing
npx shadcn@4 add @shadcn/combobox
```

The `base-nova` style targets Base UI (what this project uses) and is written
against the token vocabulary aliased in `globals.css`, so registry components
**drop in unedited**. After adding: AGPL header, `npx prettier --write`, then
the verification below.

**3. Only then hand-roll** — with tokens, `cn()`, and `focus-visible:`.

If you find yourself writing `rounded-lg border border-border bg-surface px-3
py-1.5`, stop: that is `<Button variant="outline" size="sm">` rebuilt by hand.

---

## Rules that are enforced

| Rule                                           | Why                                                                           |
| ---------------------------------------------- | ----------------------------------------------------------------------------- |
| No invented class names                        | Silently compiles to nothing                                                  |
| No `gray-*` / `slate-*`                        | Not in the theme. Lint error                                                  |
| No raw palette (`text-red-400`)                | Use `text-destructive` / `-success` / `-warning` / `-info` — contrast-checked |
| `cn()`, never template literals in `className` | `tailwind-merge` resolves conflicts; concatenation does not. Lint error       |
| `focus-visible:`, never bare `focus:`          | `focus:` fires on mouse click. Lint error                                     |
| Don't edit `src/components/ui/*` to restyle    | Breaks `shadcn diff`. Style at the call site                                  |
| Don't give a shadcn alias its own colour       | Forks the palette in two                                                      |
| Tests select by role + accessible name         | Selecting on utility classes breaks on any restyle                            |

**Native `<button>` is correct** for click targets that are not button-shaped:
list rows, colour swatches, and controls with a deliberate
`role="radio"|"option"|"switch"`. Use tokens and `focus-visible:` there.

**Overlay surfaces are out of scope.** `src/app/overlay/**`, the embed preview
and `ThemePreview` are broadcast art rendered in OBS with their own contrast
floor. Do not apply the chrome palette there.

---

## Verify (not optional)

```bash
cd frontend
npx vitest run --project unit src/__tests__/design-tokens.test.ts   # dead tokens
npx tsc --noEmit
npx eslint .
npx prettier --write 'src/**/*.tsx'
```

`design-tokens.test.ts` compiles the real stylesheet with the real Tailwind
compiler and fails on any utility that emits no CSS. **Run it whenever you touch
class names** — it is the only check that catches an invented token.

Added a token to `@theme`? Also add its contrast assertion to
`src/app/__tests__/token-contrast.test.ts`. A token with no contrast lock is a
token nobody has checked.

### Known-red on `main` — not yours

`api-tokens`, `LegalThemeToggle`, `OnboardingChecklist` and
`useCreditRollThemeMarketplace` fail in the node-environment `unit` project
(missing jsdom globals), and `auth-service ./repository/` is red. Confirm the
baseline before blaming your change:

```bash
git stash -u && npx vitest run --project unit 2>&1 | grep '^ FAIL'; git stash pop
```

Two ESLint errors for `@next/next/no-location-assign-relative-destination`
(rule-not-found) are also pre-existing.

---

## Seeing it

```bash
cd frontend && npm run dev     # localhost:3000
npm run storybook              # localhost:6006, isolated components
```

New components get a story in `src/stories/` — the a11y CI gate runs axe over
Storybook, so a story is how a component gets accessibility coverage.

---

## Finishing a user-facing feature

Per `CLAUDE.md` → _Shipping a Feature_, it is not done until:

1. It is behind a premium feature gate (ADR-0008).
2. It is in the onboarding tour — `OnboardingChecklist.tsx` **and**
   `app/upgrade/page.tsx`, which must stay in sync.
3. A Patreon post is published (`/announce-feature`).

---

## Auditing existing UI for drift

```bash
cd frontend
# hand-rolled controls that should be primitives
grep -rn --include='*.tsx' '<button' src/ | wc -l
grep -rn --include='*.tsx' '<input' src/ | wc -l
# raw palette colours
grep -rhoE '\b(red|amber|green|blue|violet|purple|orange|yellow)-[0-9]{3}' src/ --include='*.tsx' | sort | uniq -c | sort -rn
```

Count _distinct class strings_ rather than call sites — one bespoke style per
button is the signal that the design system is not being used.

## Optional: the shadcn MCP server

The registry is also available as MCP tools (browse / search / install) instead
of CLI calls. `.mcp.json` is **gitignored**, so this is per-machine — add it
yourself if you want it:

```json
{
  "mcpServers": {
    "shadcn": {
      "command": "npx",
      "args": ["shadcn@4", "mcp"],
      "cwd": "frontend"
    }
  }
}
```

Then restart Claude Code and check `/mcp`. Entirely optional — every step in
this skill works with the CLI, which is always available.
