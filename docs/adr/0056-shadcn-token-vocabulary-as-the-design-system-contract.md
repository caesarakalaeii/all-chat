# ADR-0056: The shadcn token vocabulary is the design-system contract

**Date**: 2026-08-31
**Status**: Accepted
**Deciders**: caesarakalaeii

(ADR numbering is shared with the caesar-deployment repo, so this is 0056. The
slug `shadcn-token-vocabulary-as-the-design-system-contract` is the stable
identifier; if the number has to move, the slug does not.)

## Context and Problem Statement

`frontend/` is configured for shadcn — `components.json` pins the `base-nova`
style, `shadcn@4` is a devDependency, `@base-ui/react` is a dependency, and
`src/components/ui/` holds primitives copied from the registry. The design
system was documented in `frontend/DESIGN_SYSTEM.md`.

None of it was connected.

**The primitives were half-installed.** shadcn registry components are written
against a fixed token vocabulary: `bg-primary`, `text-muted-foreground`,
`border-input`, `ring-ring`, `bg-destructive`. `src/app/globals.css` defines a
different vocabulary — `--color-bg`, `--color-surface`, `--color-text`,
`--color-text-sub` — and defined **none** of the shadcn names. The components
were pulled from the registry; the CSS variable block they depend on was never
merged in.

Tailwind has no unknown-class error. A utility whose token does not exist is
simply not emitted. So the failure was completely silent: the build passed, the
types passed, ESLint passed, Prettier happily sorted the dead classes, and code
review saw class names that read exactly like working ones. Compiling
`globals.css` with the real Tailwind compiler and grepping the output showed
**119 utility usages across 23 files that produced no CSS at all**:

| Symptom                           | Reality                                                                                                                                         |
| --------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| `ui/button.tsx` — 27 dead classes | The default `<Button>` had **no background**. `variant="destructive"` had no colour. The focus ring (`focus-visible:ring-ring`) did not render. |
| `text-destructive` — 35 usages    | Error copy rendered in ordinary body colour on admin and settings pages.                                                                        |
| `bg-surface-alt`, `bg-subtle`     | Tokens that never existed in any vocabulary. Every hover state using them was inert.                                                            |

**So nobody used the primitives.** With a `<Button>` that looked broken, the
codebase grew its own: **146 raw `<button>` elements against 33 files importing
`Button`**, and **79 raw `<input>` elements against 10 importing `Input`**.
Counted by hand-written class string, that was **66 distinct button styles for
66 buttons** and **24 distinct input styles**, spread across four background
tokens, five different focus treatments and four padding scales. Components grew
private mini design systems — `EngagementControls.tsx` declared its own
`primaryButtonClass` / `secondaryButtonClass` / `inputClass` constants. With no
status tokens defined, **329 raw Tailwind palette classes** (`text-red-400`,
`text-amber-300`, …) stood in across 14 different hues, none contrast-checked.

**And the documentation pointed the wrong way.** `DESIGN_SYSTEM.md` described a
slate-based palette on `#0f172a` from an abandoned direction; `globals.css` had
long since moved to an oklch neutral scale on `#020204`. The ESLint rule meant
to enforce the system told authors to _"use `slate-*` instead of `gray-*`"_ —
advice for a palette that is not in the theme either. An agent reading the repo
got three mutually contradicting answers about what colour to use.

The common cause is not carelessness. It is that **no mechanism could tell a
real token from an imaginary one**, so every guardrail in the repo — types,
lint, review, formatter — was blind to the one mistake that mattered.

## Decision Drivers

- A design token that does not exist must fail loudly, in CI, at the moment it
  is written.
- An unmodified component from the shadcn registry must render correctly. If
  every `shadcn add` needs a manual translation pass, the registry is worthless
  and the CLI, the MCP server and `shadcn diff` are all dead weight.
- One palette. Adding a compatibility layer must not fork the colour system.
- Overlay surfaces rendered in OBS are broadcast art with their own rules and
  must not be dragged into the chrome's contract.

## Considered Options

1. **Translate the registry to the project vocabulary.** Rewrite each primitive
   to use `bg-surface` / `text-text-sub` on the way in.
2. **Replace the project vocabulary with shadcn's.** Rename `--color-surface` to
   `--color-card` everywhere, delete the old names.
3. **Alias the shadcn names onto the existing tokens.** Keep `--color-bg` and
   friends as the source of truth; add the shadcn names as `var()` aliases.

## Decision

**Option 3.** `src/app/globals.css` gains a _shadcn compatibility layer_: a
block of `--color-*` aliases mapping the registry's vocabulary onto the tokens
this project already defines.

```css
--color-background: var(--color-bg);
--color-card: var(--color-surface);
--color-muted: var(--color-surface-2);
--color-muted-foreground: var(--color-text-sub);
--color-primary: var(--color-twitch);
--color-ring: var(--color-twitch);
/* … */
```

The mapping is **one-directional and value-free**: the aliases never carry a
colour of their own, only a `var()` pointing at a real token. That is the
property that keeps one palette instead of two. An alias given its own value
forks the design system, which is precisely the failure this ADR exists to end.

Four **status tokens** are added as absolute `oklch()` values, because no
neutral or platform hue carries the meaning: `--color-destructive`,
`--color-success`, `--color-warning`, `--color-info`. They are read as _text_ on
all three backdrops, so each is tuned to clear 4.5:1 against the lightest of
them (`--color-surface-2`), not merely against `--color-bg`. They exist so that
nothing needs to reach for a raw palette colour again.

Two semantic choices are load-bearing and easy to get backwards:

- `--color-popover` maps to `--color-surface`, **not** `--color-surface-2`, so
  that `--color-accent` (surface-2) reads as a visible step _on top of_ a
  popover. Pointing both at surface-2 makes every menu-item hover invisible.
- shadcn's _accent_ is not a brand accent. It is the hover/selected background
  for menu items, list rows and toggles.

### Enforcement

`frontend/src/__tests__/design-tokens.test.ts` compiles `globals.css` with the
**real Tailwind compiler** and scans `src/` with the **real Tailwind scanner**
— not a regex approximation of either — then asserts that every candidate shaped
like a token utility emits CSS. The verdict is exactly what the production build
would emit.

There is deliberately **no suppressions baseline**. The check is at zero and a
dead token is always a bug, never debt worth carrying. This is the opposite
choice from the a11y gate (`eslint.a11y.suppressions.json`), and for a reason:
an a11y violation is a real control that could be improved, while a dead token
is a class that does nothing at all.

The contrast lock in `src/app/__tests__/token-contrast.test.ts` is extended to
assert every alias _resolves to the token it claims to alias_, so a rename on
either side fails there rather than compiling to an empty rule.

### Scope boundary

Everything under `src/app/overlay/**`, the embed preview and the marketplace
theme preview is **broadcast art rendered inside OBS**, not app chrome:
user-themable, composited over arbitrary gameplay footage, and already carved
out of the app's guarantees by the scope note in `token-contrast.test.ts` and
the separate floor in `tests/e2e/theme-contrast.spec.ts`. The chrome's neutral
tokens are the wrong vocabulary for a credits roll that has to read over
someone's stream, so the palette rule is not enforced there. The other rules
(no template `className`s, `focus-visible:`) still are.

### Registry files stay verbatim

Primitives added by the CLI are kept byte-identical to upstream so
`npx shadcn@4 diff` stays meaningful. Local styling belongs in the call site's
`className`. Where the registry legitimately uses `focus:` — listbox items,
which Base UI focuses for pointer highlighting too, and `focus:z-10` stacking
fixes — the _ESLint config_ exempts `src/components/ui/**` rather than the files
carrying disable comments, which would defeat the diff.

## Consequences

**Good.** An unmodified registry component now renders correctly, which makes
`shadcn add`, `shadcn diff` and the shadcn MCP server usable — verified by
adding `tabs`, `select`, `toggle-group`, `textarea`, `label` and `separator`
with zero edits. The 119 dead utilities became live CSS: buttons have
backgrounds, error text is red, focus rings render. The class of bug is closed
in CI rather than documented as a caution.

**Bad / risky.** This is a **visible change to the live UI**, not a refactor:
controls that silently rendered unstyled now render styled. Some of those
surfaces had never been seen correct. The alias block is also indirection — a
reader chasing `bg-primary` must follow `--color-primary` → `--color-twitch`
before reaching a colour, which is the price of not forking the palette.

**Deferred.** The 329 raw palette classes are not migrated. Most are status
colours that now have tokens, but each needs a semantic judgement (is this amber
a warning, a premium marker, or brand decoration?) that a mechanical rewrite
gets wrong. The palette rule currently bans only `gray-*`/`slate-*`, which is at
zero; widening it to the full palette should follow the ratchet pattern the a11y
gate already uses.

## Links

- `frontend/DESIGN_SYSTEM.md` — the design system this ADR makes true
- `.claude/skills/shadcn-ui/SKILL.md` — the agent-facing procedure
- ADR-0008 — feature gates (premium toggle, release step 1)
- ADR-0040 — self-hosting Monaco, the earlier case of a silent CSP/asset failure
- ADR-0053 — theme consolidation, on the overlay-theme side of the scope boundary
