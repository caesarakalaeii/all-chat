# ADR-0056: The shadcn token vocabulary is the design-system contract

**Date**: 2026-08-31
**Status**: Accepted
**Revised**: 2026-11-18
**Deciders**: caesarakalaeii

## Context

The frontend is configured for shadcn (`components.json`, primitives copied
into `src/components/ui/`), but the registry's fixed token vocabulary
(`bg-primary`, `text-muted-foreground`, `bg-destructive`, ...) was never
merged into `globals.css`, which defines a different vocabulary and none of
the shadcn names. Tailwind has no unknown-class error, so the failure was
silent: 119 utility usages across 23 files emitted no CSS (the default
`<Button>` had no background, `text-destructive` rendered error copy in body
colour), nobody used the primitives (146 raw `<button>` elements against 33
files importing `Button`), status colour was carried by 329 raw palette
classes across 14 hues, and `DESIGN_SYSTEM.md` described a palette from an
abandoned direction. Common cause: no mechanism could tell a real token from
an imaginary one.

## Decision

**Adopt the shadcn vocabulary as the design-system contract via a
compatibility layer in `globals.css`: a block of `--color-*` aliases mapping
the registry's names onto the tokens this project already defines.**

```css
--color-background: var(--color-bg);
--color-card: var(--color-surface);
--color-muted: var(--color-surface-2);
--color-muted-foreground: var(--color-text-sub);
--color-primary: var(--color-twitch);
--color-ring: var(--color-twitch);
/* … */
```

The mapping is one-directional and value-free: aliases never carry a colour of
their own, only a `var()` pointing at a real token. An alias with its own value
forks the palette, which is the failure this ADR ends. Four status tokens
(`--color-destructive`, `--color-success`, `--color-warning`, `--color-info`)
are added as absolute `oklch()` values, because no neutral or platform hue
carries the meaning; they are read as text on all three backdrops, so each
clears 4.5:1 against the lightest of them.

Load-bearing semantics: `--color-popover` maps to `--color-surface` (not
surface-2) so `--color-accent` reads as a visible step on a popover, and
shadcn's *accent* is a hover/selected background, not a brand accent.

### Enforcement

`frontend/src/__tests__/design-tokens.test.ts` compiles `globals.css` with the
real Tailwind compiler and scans `src/` with the real Tailwind scanner, then
asserts every candidate shaped like a token utility emits CSS. There is
deliberately no suppressions baseline: a dead token is always a bug. The
contrast lock in `src/app/__tests__/token-contrast.test.ts` asserts every alias
resolves to the token it claims to alias.

### Scope boundary

The OBS render surfaces, the embed preview and the marketplace theme preview
are broadcast art, not app chrome (the exclusion lists the OBS render routes
individually — written as `src/app/overlay/[id]/page.tsx` a pattern matches
nothing, since square brackets are a glob character class; the patterns use
`*` for the dynamic segment). The palette rule is not enforced there; the
other rules (no template `className`s, `focus-visible:`) still are. Registry
primitives stay byte-identical to upstream so `npx shadcn@4 diff` stays
meaningful; where the registry legitimately uses `focus:`, the ESLint config
exempts `src/components/ui/**`.

## Consequences

- An unmodified registry component renders correctly (verified by adding
  several primitives with zero edits); dead utilities became live CSS; the
  bug class is closed in CI.
- A visible change to the live UI, not a refactor. The alias block is
  indirection (chasing `bg-primary` means following `--color-primary` ->
  `--color-twitch`), the price of not forking the palette.
- Open work: the call-site migration (raw controls to primitives) and the raw
  palette migration. The premium pass is done (`--color-premium`, warmer than
  `--color-warning`, since as raw `amber-400` the two were nearly the same
  paint); the palette rule currently bans only `gray-*`/`slate-*`.

## References

- `frontend/DESIGN_SYSTEM.md` — the design system this ADR makes true
- `.claude/skills/shadcn-ui/SKILL.md` — the agent-facing procedure
- ADR-0008 (feature gates), ADR-0040 (self-hosted Monaco, the earlier silent
  asset failure), ADR-0053 (theme consolidation, the overlay-theme side of the
  scope boundary)
