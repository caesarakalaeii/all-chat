# All-Chat Design System

**Status**: Active. Enforced by CI, not by convention.
**Authority**: `src/app/globals.css` is the source of truth for tokens. This
document explains it; where they disagree, the CSS wins and this file is a bug.
**Rationale**: [ADR-0056](../docs/adr/0056-shadcn-token-vocabulary-as-the-design-system-contract.md)

> Working as an agent? Start with `.claude/skills/shadcn-ui/SKILL.md` — it is the
> procedure. This file is the reference behind it.

---

## The one rule

**Never invent a class name.** Tailwind has no unknown-class error: write
`bg-primary` when no `--color-primary` token exists and Tailwind emits nothing.
The build passes, types pass, lint passes, and the element renders unstyled.

This is not hypothetical — it is how the app shipped a `<Button>` with no
background and error text that was never red (ADR-0056). Everything below exists
to make that impossible.

`npx vitest run --project unit src/__tests__/design-tokens.test.ts` compiles the
real stylesheet and fails on any utility that emits no CSS. Run it when you
touch classes.

---

## Layers

Work top-down. Stop at the first layer that answers the question.

| Layer            | Where                             | Use it for                                            |
| ---------------- | --------------------------------- | ----------------------------------------------------- |
| 1. Primitives    | `src/components/ui/*`             | Buttons, inputs, dialogs, tabs — anything with chrome |
| 2. Tokens        | `@theme` in `src/app/globals.css` | Colour, spacing, radius, type scale                   |
| 3. Raw utilities | Tailwind built-ins                | Layout only: flex, grid, gap, position                |

If a need is not met at layer 1, add a primitive — do not hand-roll at layer 3.

---

## Layer 1: primitives

### What exists

`button` · `input` · `textarea` · `label` · `field` · `select` · `switch` ·
`toggle` · `toggle-group` · `tabs` · `card` · `badge` · `dialog` ·
`alert-dialog` · `popover` · `separator` · `skeleton` · `toast` ·
`visually-hidden`

Check `ls src/components/ui/` — this list ages.

### Adding one

```bash
cd frontend
npx shadcn@4 search @shadcn -q "combobox"   # find it
npx shadcn@4 view @shadcn/combobox          # read it before installing
npx shadcn@4 add @shadcn/combobox           # install
```

Then: add the AGPL header (every source file has one), run
`npx prettier --write src/components/ui/`, and run the design-token test.

The registry's `base-nova` style targets Base UI, which is what this project
already uses, and its components are written against the token vocabulary
aliased in `globals.css`. **They drop in unedited. That is the contract.**

### Keep registry files verbatim

Do not restyle a primitive in place. `npx shadcn@4 diff` is only useful while
the file matches upstream, and a local edit turns every future upstream fix into
a manual merge.

Local styling goes in the **call site's** `className`:

```tsx
<Button variant="outline" size="sm" className="w-full justify-start">
```

If a change genuinely belongs in the primitive — a project-wide behaviour, like
`aria-invalid` styling on `Input` — make it and **write down why in the file**.
That note is what tells the next person the diff is intentional.

### Choosing a variant

| Intent                        | Variant       | Notes                                                                      |
| ----------------------------- | ------------- | -------------------------------------------------------------------------- |
| The main action               | `default`     | One per view. Filled brand purple                                          |
| Marketing / hero CTA          | `gradient`    | Landing surfaces only                                                      |
| Supporting action             | `outline`     | The common case                                                            |
| Filled but secondary          | `secondary`   | Tinted surface                                                             |
| Low emphasis, icons, toolbars | `ghost`       | No chrome until hover                                                      |
| Destructive                   | `destructive` | Tinted, not solid red — solid red for a routine "Remove" reads as an alarm |
| Inline text action            | `link`        | Add `className="h-auto p-0"` when it sits in a sentence                    |

Sizes: `xs` `sm` `default` `lg`, and `icon-xs` `icon-sm` `icon` `icon-lg` for
square icon-only buttons. Icon-only buttons need an `aria-label`.

### Links that look like buttons

A control that **navigates** must stay an `<a>` — middle-click, copy-link and
open-in-new-tab all break on a `<button>`. Share the styling instead of the
element, with `buttonVariants`:

```tsx
import Link from 'next/link'
import { buttonVariants } from '@/components/ui/button'
import { cn } from '@/lib/utils'

<Link href="/dashboard" className={cn(buttonVariants({ variant: 'ghost', size: 'lg' }), 'w-full justify-start')}>
```

This is the only correct way to make a link and a button in the same row look
identical. Hand-copying the button's classes onto the link is how they drift.

### When a native element is right

`<Button>` is for things that **look like buttons**. Use a native `<button>`
with tokens and `focus-visible:` for click targets that are not button-shaped:

- list rows and cards that happen to be clickable
- colour swatches and theme thumbnails
- controls carrying `role="radio"` / `role="option"` / `role="switch"`, where
  the ARIA semantics were chosen deliberately

Wrapping those in `<Button>` fights its `inline-flex items-center justify-center`
layout and, for the ARIA cases, changes semantics that tests assert on.

### Forms

`Field` wires label ↔ control ↔ description ↔ error automatically. Prefer it
over hand-plumbed `htmlFor` / `aria-describedby`:

```tsx
<Field.Root>
  <Field.Label>Channel name</Field.Label>
  <Field.Control render={<Input />} />
  <Field.Description>Shown under your messages.</Field.Description>
  <Field.Error />
</Field.Root>
```

`Input` owns its invalid state — set `aria-invalid` and the red border follows.
Do not hand-roll error borders at the call site.

The `render` prop is the Base UI composition idiom, and it is how you put a
primitive's styling on someone else's element:

```tsx
<AlertDialog.Close render={<Button variant="outline" size="sm" />}>Cancel</AlertDialog.Close>
```

---

## Layer 2: tokens

Defined in the `@theme` block of `src/app/globals.css`. Dark-only; there is no
light theme for app chrome.

### Colour

**Surfaces and text** — a three-step depth scale and a three-step text hierarchy:

| Token               | Class              | Use                                     |
| ------------------- | ------------------ | --------------------------------------- |
| `--color-bg`        | `bg-bg`            | Page background                         |
| `--color-surface`   | `bg-surface`       | Cards, dialogs, popovers                |
| `--color-surface-2` | `bg-surface-2`     | Raised or hovered rows within a surface |
| `--color-border`    | `border-border`    | Standard hairline                       |
| `--color-border-md` | `border-border-md` | Emphasised edge, input borders          |
| `--color-text`      | `text-text`        | Primary copy, headings                  |
| `--color-text-sub`  | `text-text-sub`    | Secondary copy, labels                  |
| `--color-text-dim`  | `text-text-dim`    | Captions, timestamps, placeholders      |

**Status** — `text-destructive` `text-success` `text-warning` `text-info`, plus
the `bg-`/`border-` forms. Each clears 4.5:1 as text on all three surfaces.

**Platform** — `twitch` `youtube` `kick` `tiktok` `discord`, with matching
`--shadow-glow-*`. Accents and identity only, never large fills.

### No raw palette colours

`text-red-400`, `bg-amber-500`, `slate-*`, `gray-*` — none of these are the
design system. They are unverified for contrast and drift apart across files.
`gray-*` and `slate-*` are lint errors; the rest are on their way out (ADR-0056).

Reach for the status token instead. `text-red-400` → `text-destructive`.

### Type, spacing, radius

The scale is deliberately small and deliberately not Tailwind's default —
`text-base` is 14px here, not 16px, because this is a dense tool UI.

`text-xs` 11 · `text-sm` 13 · `text-base` 14 · `text-lg` 16 · `text-xl` 20 ·
`text-2xl` 24 · `text-3xl` 32

Spacing: `1 2 3 4 6 8 12 16`. Radius: `sm` 4 · `md` 8 · `lg` 12 · `xl` 16 ·
`2xl` 24 · `full`.

### The shadcn alias block

`globals.css` also defines `--color-primary`, `--color-muted-foreground`,
`--color-input`, `--color-ring` and the rest of the registry's vocabulary as
**aliases** onto the tokens above. That is what lets a registry component drop
in unedited.

**When you write your own markup, prefer the project names** (`bg-surface`,
`text-text-sub`) — they say what the thing is. The aliases exist so vendored
code works, not as a second vocabulary to choose from.

**Never give an alias its own colour value.** An alias carries a `var()` and
nothing else. A value there forks the palette into two, which is the exact
failure ADR-0056 exists to prevent.

Adding a token? Add it to `@theme`, then add its contrast assertion to
`src/app/__tests__/token-contrast.test.ts`. A token with no contrast lock is a
token nobody has checked.

---

## Layer 3: raw utilities

Fine for layout: `flex`, `grid`, `gap-*`, `absolute`, `w-full`, `min-w-0`.

Not fine for colour, border, radius, shadow or focus. If you are writing
`rounded-lg border border-border bg-surface px-3 py-1.5`, you are rebuilding
`<Button variant="outline" size="sm">` by hand — that is how the codebase
reached 66 distinct button styles.

### `cn()`, never template literals

```tsx
import { cn } from '@/lib/utils'

;<div className={cn('rounded-lg p-4', isActive && 'bg-surface-2')} />
```

Template literals in `className` are a lint error. `cn()` merges Tailwind
conflicts correctly (`tailwind-merge`); string concatenation produces
`"px-2 px-4"` and lets the loser win at random.

### Focus

`focus-visible:`, never `focus:` — bare `focus:` fires on mouse click and puts
a ring on a button someone just clicked. This is a lint error outside
`src/components/ui/**`, which is exempt because the registry uses `focus:` in
the two places it is correct (listbox items, `focus:z-10` stacking).

Primitives already carry the right ring. Do not add your own.

---

## Overlay surfaces are a different scope

`src/app/overlay/**`, the embed preview and `ThemePreview` are **broadcast art**
rendered inside OBS: user-themable, composited over gameplay footage, held to a
separate contrast floor (`tests/e2e/theme-contrast.spec.ts`). The chrome's
neutral tokens are the wrong vocabulary there and the palette rule is not
enforced.

Everything else — dashboard, editor, settings, admin, marketing, legal — is
chrome and follows this document.

---

## What CI enforces

| Check            | Command                                                                                          | Fails on                                                                       |
| ---------------- | ------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------ |
| Dead tokens      | `npx vitest run --project unit src/__tests__/design-tokens.test.ts`                              | Any utility that emits no CSS                                                  |
| Token contrast   | `npx vitest run --project unit src/app/__tests__/token-contrast.test.ts`                         | A token or alias below its WCAG floor, or an alias pointing at a renamed token |
| Broken utilities | `npx vitest run --project unit src/__tests__/no-broken-tailwind-utilities.test.ts`               | Class names Tailwind v4 silently drops                                         |
| Design rules     | `npx eslint .`                                                                                   | `gray-*`/`slate-*`, template `className`s, bare `focus:`                       |
| Accessibility    | `npx eslint -c eslint.a11y.config.mjs --suppressions-location eslint.a11y.suppressions.json src` | New a11y violations (shrink-only ratchet)                                      |
| Types            | `npx tsc --noEmit`                                                                               |                                                                                |
| Format           | `npx prettier --check .`                                                                         | Also sorts Tailwind classes                                                    |

Two suites are **red on `main`** and not yours to fix: the `api-tokens`,
`LegalThemeToggle`, `OnboardingChecklist` and `useCreditRollThemeMarketplace`
tests fail in the node-environment `unit` project (missing jsdom globals), and
`auth-service ./repository/`. Scope around them; do not gate work on them.

---

## Adding a UI feature: checklist

1. Does a primitive exist? Use it.
2. Does the registry have one? `npx shadcn@4 add` it, unedited.
3. Only then hand-roll — with tokens, `cn()`, and `focus-visible:`.
4. Run the design-token test and `npx tsc --noEmit`.
5. Add a Storybook story (`src/stories/`) — the a11y CI gate runs axe over it.
6. Ship the release steps in `CLAUDE.md` → _Shipping a Feature_: premium gate,
   onboarding tour entry, Patreon post.

## Anti-patterns, with the fix

| Anti-pattern                                                    | Fix                                                |
| --------------------------------------------------------------- | -------------------------------------------------- |
| `<button className="rounded-lg bg-twitch px-4 py-2 …">`         | `<Button size="lg">`                               |
| `<input className="w-full rounded-lg border …">`                | `<Input />`                                        |
| `const primaryButtonClass = '…'` at the top of a component file | A `<Button>` variant                               |
| `text-red-400`                                                  | `text-destructive`                                 |
| `className={`p-2 ${active ? 'bg-x' : ''}`}`                     | `className={cn('p-2', active && 'bg-x')}`          |
| `focus:ring-2`                                                  | `focus-visible:ring-2`, or let the primitive do it |
| Copying a primitive to tweak one colour                         | `className` at the call site                       |
| A test selecting `button.bg-red-500\\/10`                       | `getByRole('button', { name: /…/ })`               |

That last one is not a style nit. A test asserting on a utility class breaks the
moment a control adopts a primitive — for a reason that has nothing to do with
behaviour. Select by role and accessible name.
