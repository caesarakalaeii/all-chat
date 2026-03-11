# Phase 24: Component Library Setup & Customization - Research

**Researched:** 2026-03-10
**Domain:** @base-ui/react primitives, CVA, Storybook 10, tw-animate-css, CSS cascade layers
**Confidence:** HIGH

## Summary

Phase 24 completes the component library started in Phase 23. Button is already done (`frontend/src/components/ui/button.tsx`) — it wraps `@base-ui/react/button` with CVA variants and `cn()`. The remaining five components (Card, Input, Badge, Dialog, Toast) follow the exact same structural pattern. All infrastructure is already installed: `@base-ui/react@1.2.0`, `class-variance-authority@0.7.1`, `tw-animate-css@1.4.0`, Storybook 10 with `@storybook/addon-a11y`, and Vitest wired to run Storybook stories as browser tests.

The key architectural fact: `@base-ui/react` ships headless primitives for every component needed (Input, Dialog, Toast are all present in v1.2.0). Card and Badge have no `@base-ui/react` primitive — they are pure HTML with CVA styling. The existing Toast system uses `react-hot-toast` in the admin section — Phase 24 builds the new design-system Toast on `@base-ui/react/toast`, which ships a full manager API (`createToastManager`, `useToastManager`, `ToastProvider`, `ToastViewport`, etc.).

Phase 24 also completes COMP-09: removing all `!important` from `events.css` by migrating those rules into `@layer marketplace-themes`. Phase 23 already created the cascade layer ordering and moved many rules — this phase verifies zero remaining `!important` declarations.

**Primary recommendation:** Follow the Button component exactly: import `@base-ui/react` primitive, wrap with CVA, export. For Card/Badge (no primitive), render a plain `<div>` with `data-slot`. For Toast, use `createToastManager()` for the singleton imperative API. A11y testing is already wired — change `test: 'todo'` to `test: 'error'` in `.storybook/preview.ts` after all stories pass.

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Gradient CTA (COMP-05)**
- Primary gradient = Twitch purple to TikTok teal: `linear-gradient(90deg, #9146FF, #69C9D0)`
- Same gradient already used for nav active underline in Phase 23 — reuse exactly, no new gradient definition
- Applied to Button's `gradient` variant (new variant alongside existing default/outline/ghost etc.)
- This is the "platforms are the theme" identity — no generic purple/blue gradients

**Platform Badge Design (COMP-06)**
- Content: Glow dot + short label in DM Mono (TWITCH / YOUTUBE / KICK / TIKTOK)
- Dot: Platform-colored `box-shadow` glow — same pattern as chat message dots from Phase 23
- Background: `--color-badge-bg` (7% white opacity, neutral) — platform color only on dot + label text
- Sizes: Two CVA variants — `default` (standalone contexts, source lists) and `sm` (inline, table rows)
- Platform color applied via the static `PLATFORM_COLORS` map established in Phase 23

**Loading Skeletons (COMP-07)**
- Animation: Pulse (opacity fade) — `tw-animate-css` `animate-pulse`. Calmer, non-distracting for streaming tool
- Color: `--color-surface-2` for skeleton blocks — neutral, no platform tinting
- Skeleton component wraps arbitrary shapes (use `className` for width/height/radius)

**Dialog**
- Backdrop: Frosted glass — `backdrop-filter: blur(8px)` + `rgba(0,0,0,0.6)` overlay
- Consistent with frosted nav from Phase 23
- Dialogs only appear in dashboard/editor context — never in overlay pages (no OBS performance concern)

**Toast**
- Position: Bottom-right
- Auto-dismiss: 4 seconds for success/info; errors stay until manually dismissed
- Stacking: new toasts push up from bottom-right

**Micro-interactions (COMP-04)**
- Hover: subtle scale (`scale-[1.02]`) + shadow lift on interactive cards
- Transitions via `tw-animate-css` — no Framer Motion (decided in Phase 23)
- Buttons already have `transition-all` from the existing Button component

**Component Variants (COMP-03)**
- CVA for all components — same pattern as existing Button
- All components use `cn()` from `@/lib/utils`
- Variants use design tokens, not raw Tailwind color classes

### Claude's Discretion
- Exact CVA variant names beyond what's specified (e.g., Card size variants if needed)
- Input focus ring treatment (use `focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50` pattern from Button)
- Toast enter/exit animation specifics (tw-animate-css provides options)
- Whether Dialog needs size variants (sm/default/lg) — planner decides based on use cases found in codebase

### Deferred Ideas (OUT OF SCOPE)
- None — discussion stayed within phase scope
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| COMP-01 | shadcn/ui core primitives installed (Button, Card, Input, Badge, Dialog, Toast) | Button already done. Card, Input, Badge, Dialog, Toast to be built following Button pattern. `shadcn@4.0.2` already in package.json for scaffolding if needed. |
| COMP-02 | Components customized with design tokens (slate scale, not zinc) | All tokens already in `globals.css` under `@theme`. Button demonstrates the pattern: use `--color-*` tokens, not raw Tailwind colors. |
| COMP-03 | Component variant patterns implemented with CVA | `class-variance-authority@0.7.1` installed. Button shows exact pattern: `cva()` base + `variants` object + `defaultVariants`. |
| COMP-04 | Smooth micro-interactions added (hover scale + shadow transitions) | `tw-animate-css@1.4.0` installed. `scale-[1.02]` + `transition-all` pattern for interactive cards. |
| COMP-05 | Gradient CTAs implemented (purple to teal gradient for primary actions) | New `gradient` variant on Button. CSS: `linear-gradient(90deg, #9146FF, #69C9D0)` — same as nav underline. |
| COMP-06 | Platform-color coded components created (badges, borders, status indicators) | Badge component uses `PLATFORM_COLORS` map from `@/lib/platform-colors`. Dot glow via `box-shadow`. |
| COMP-07 | Animated loading states and skeletons implemented | Skeleton component uses `animate-pulse` from `tw-animate-css`. Color: `--color-surface-2`. |
| COMP-08 | Performance budget established (<16ms message render, <100KB bundle increase) | Baseline bundle size measurement before components added, re-measured after. Runtime: React DevTools Profiler or `performance.now()` wrapper. |
| COMP-09 | All !important removed from events.css (replaced with cascade layers) | Phase 23 completed the `@layer marketplace-themes` migration. This phase verifies zero `!important` remain via grep check. |
</phase_requirements>

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `@base-ui/react` | 1.2.0 (installed) | Headless accessible primitives | Provides ARIA semantics, keyboard nav, focus management for free |
| `class-variance-authority` | 0.7.1 (installed) | Variant-based component API | Type-safe variant props, matches Button pattern established in Phase 23 |
| `tw-animate-css` | 1.4.0 (installed) | Animation utilities | `animate-pulse` for skeletons, enter/exit transitions for Dialog/Toast |
| `tailwind-merge` + `clsx` | installed | Class merging via `cn()` | Already in `@/lib/utils`, used by Button |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `@storybook/addon-a11y` | 10.2.17 (installed) | a11y testing in Storybook | Already wired — stories run axe checks automatically |
| `@storybook/addon-vitest` | 10.2.17 (installed) | Storybook stories as Vitest tests | Chromium browser execution for visual/a11y testing |
| `lucide-react` | 0.563.0 (installed) | Icons for component slots | X button in Dialog/Toast, status icons |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `@base-ui/react` Toast | `react-hot-toast` (already installed) | Already used for admin toasts but lacks design system integration; `@base-ui/react/toast` is the architectural choice for consistency |
| Pure div for Dialog | `@base-ui/react/dialog` | Using the primitive is mandatory for correct ARIA role=dialog, focus trap, scroll lock |
| `tw-animate-css` | Framer Motion | Framer Motion explicitly deferred to v2 (Phase 23 decision) |

**Installation:** All dependencies already installed. No new packages required.

---

## Architecture Patterns

### Component Location
```
frontend/src/components/ui/
├── button.tsx          # DONE — reference implementation
├── card.tsx            # Phase 24: pure div + CVA (no @base-ui primitive)
├── input.tsx           # Phase 24: @base-ui/react/input wrapper
├── badge.tsx           # Phase 24: pure div + CVA + PLATFORM_COLORS
├── dialog.tsx          # Phase 24: @base-ui/react/dialog wrapper
├── toast.tsx           # Phase 24: @base-ui/react/toast wrapper + manager

frontend/src/stories/
├── Button.stories.ts   # DONE — reference story
├── Card.stories.tsx    # Phase 24: new
├── Input.stories.tsx   # Phase 24: new
├── Badge.stories.tsx   # Phase 24: new
├── Dialog.stories.tsx  # Phase 24: new
├── Toast.stories.tsx   # Phase 24: new
```

### Pattern 1: @base-ui/react Primitive Wrapper (Input, Dialog, Toast)
**What:** Import primitive parts, wrap in CVA-styled function, re-export.
**When to use:** Any component with accessibility requirements (focus management, ARIA roles).
**Example (follows button.tsx exactly):**
```typescript
// Source: frontend/src/components/ui/button.tsx (established pattern)
"use client"

import { Input as InputPrimitive } from "@base-ui/react/input"
import { cva, type VariantProps } from "class-variance-authority"
import { cn } from "@/lib/utils"

const inputVariants = cva(
  "w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text " +
  "placeholder:text-text-dim transition-all outline-none " +
  "focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 " +
  "disabled:pointer-events-none disabled:opacity-50",
  {
    variants: {
      size: {
        default: "h-9",
        sm: "h-7 text-xs",
      },
    },
    defaultVariants: { size: "default" },
  }
)

function Input({ className, size, ...props }: InputPrimitive.Props & VariantProps<typeof inputVariants>) {
  return (
    <InputPrimitive
      data-slot="input"
      className={cn(inputVariants({ size, className }))}
      {...props}
    />
  )
}

export { Input, inputVariants }
```

### Pattern 2: Pure Div Component (Card, Badge)
**What:** No `@base-ui/react` primitive exists for these — use plain HTML element with `data-slot` and CVA.
**When to use:** Presentational containers with no interactive semantics.
**Example:**
```typescript
// Card component — no @base-ui primitive, plain div
"use client"

import { cva, type VariantProps } from "class-variance-authority"
import { cn } from "@/lib/utils"

const cardVariants = cva(
  "rounded-xl border border-border bg-surface text-text transition-all",
  {
    variants: {
      interactive: {
        true: "hover:scale-[1.02] hover:shadow-lg cursor-pointer",
        false: "",
      },
    },
    defaultVariants: { interactive: false },
  }
)

function Card({ className, interactive, ...props }: React.HTMLAttributes<HTMLDivElement> & VariantProps<typeof cardVariants>) {
  return (
    <div
      data-slot="card"
      className={cn(cardVariants({ interactive, className }))}
      {...props}
    />
  )
}
```

### Pattern 3: @base-ui/react Dialog with Frosted Backdrop
**What:** Dialog.Root + Dialog.Backdrop (frosted) + Dialog.Popup.
**Key detail:** `@base-ui/react/dialog` exports `Dialog` namespace with parts: Root, Trigger, Portal, Popup, Backdrop, Title, Description, Close.
**Backdrop approach:**
```typescript
// Backdrop gets the frosted glass treatment
<Dialog.Backdrop className="fixed inset-0 bg-black/60 backdrop-blur-[8px]" />
<Dialog.Popup className="fixed left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 rounded-xl border border-border bg-surface p-6 shadow-xl" />
```

### Pattern 4: @base-ui/react Toast with Singleton Manager
**What:** `createToastManager()` creates an imperative singleton usable outside React. `ToastProvider` + `ToastViewport` in layout. `useToastManager()` in components that need to trigger toasts.
**Architecture:**
```typescript
// frontend/src/lib/toast.ts — singleton manager
import { createToastManager } from "@base-ui/react/toast"
export const toastManager = createToastManager()

// frontend/src/components/ui/toast.tsx — layout provider + toast renderer
// Wraps Toast.Provider with timeout config, Toast.Viewport positioned bottom-right
// Individual Toast.Root renders with tw-animate-css enter/exit

// Usage anywhere:
import { toastManager } from "@/lib/toast"
toastManager.add({ title: "Saved", type: "success", timeout: 4000 })
toastManager.add({ title: "Error", type: "error", timeout: 0 }) // stays until dismissed
```

### Pattern 5: Platform Badge with Glow Dot
**What:** Two-element badge — glow dot + DM Mono label. Platform color via inline style + `PLATFORM_COLORS` Tailwind classes.
**Key insight:** The glow dot needs a `box-shadow` with the exact platform color hex — Tailwind can't generate arbitrary `box-shadow` colors dynamically. Use inline style for the `box-shadow` glow, PLATFORM_COLORS for the `text-*` class.
```typescript
// Glow dot uses box-shadow with CSS var — safe because CSS vars are defined in @theme
// text color uses PLATFORM_COLORS[platform].text — complete literal class string
const PLATFORM_GLOW = {
  twitch:  { boxShadow: '0 0 6px var(--color-twitch)'  },
  youtube: { boxShadow: '0 0 6px var(--color-youtube)' },
  kick:    { boxShadow: '0 0 6px var(--color-kick)'    },
  tiktok:  { boxShadow: '0 0 6px var(--color-tiktok)'  },
} as const
```

### Pattern 6: Gradient Button Variant
**What:** New `gradient` variant on the existing Button component. Inline style or Tailwind arbitrary value.
**Approach:** Since this is a specific one-off gradient value, use inline `style` prop on the ButtonPrimitive rather than a Tailwind arbitrary value:
```typescript
// In buttonVariants CVA — gradient variant in `variant` key:
gradient: "bg-transparent text-white font-semibold",
// Then in the Button component render, conditionally add style:
// style={variant === 'gradient' ? {background: 'linear-gradient(90deg, #9146FF, #69C9D0)'} : undefined}
// OR use Tailwind arbitrary: bg-[linear-gradient(90deg,#9146FF,#69C9D0)]
```
Both approaches work. Arbitrary Tailwind value is cleaner (no extra logic in component function).

### Pattern 7: Skeleton Component
**What:** Simple `div` with `animate-pulse` and `--color-surface-2` background. No `@base-ui` primitive.
```typescript
function Skeleton({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      data-slot="skeleton"
      className={cn("animate-pulse rounded-md bg-surface-2", className)}
      {...props}
    />
  )
}
```

### Anti-Patterns to Avoid
- **Dynamic Tailwind class construction:** Never `'text-' + platform`. PLATFORM_COLORS already has the complete literal strings.
- **Raw color values in CVA:** Never `bg-[#9146FF]` for semantic use — use `bg-twitch` (token alias).
- **Framer Motion:** Explicitly deferred. Use `tw-animate-css` and Tailwind transitions only.
- **`!important` in component styles:** The entire phase is partly about eliminating these from events.css.
- **Dark mode variant classes:** The app is dark-only. Button.tsx has some `dark:` classes inherited from shadcn scaffolding — audit and remove if they conflict with the single-token system.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Focus trap in Dialog | Custom focus management | `@base-ui/react/dialog` | Portal, focus trap, scroll lock, Escape key all built in |
| Toast queue management | Custom array/state | `@base-ui/react/toast` `createToastManager` | Handles stacking, auto-dismiss timers, promise states, concurrent updates |
| Input ARIA + Field integration | Custom form wiring | `@base-ui/react/input` | Auto-integrates with Field.Control for label association, validity, descriptions |
| Animation timing | Custom CSS keyframes | `tw-animate-css` utilities | `animate-pulse`, `animate-fade-in`, `animate-slide-up` are all available |
| Class merging | Custom string concatenation | `cn()` from `@/lib/utils` | Handles Tailwind class conflicts (e.g., two `rounded-*` classes) |

**Key insight:** `@base-ui/react` does zero styling — every visual property must be specified via `className`. This means the component implementations have full design control but must handle all CSS. The accessibility layer is free.

---

## Common Pitfalls

### Pitfall 1: Toast Provider Placement
**What goes wrong:** `Toast.Provider` placed inside a component that unmounts, causing toasts to disappear or crash.
**Why it happens:** `createToastManager()` is a singleton but `Toast.Provider` must stay mounted.
**How to avoid:** Place `Toast.Provider` + `Toast.Viewport` in the root layout (`frontend/src/app/layout.tsx`), not inside any page component.
**Warning signs:** Toasts vanish when navigating between pages.

### Pitfall 2: Dialog Backdrop z-index Stacking
**What goes wrong:** Frosted glass backdrop appears behind nav (which has `backdrop-filter` and a high stacking context).
**Why it happens:** Nav uses `backdrop-filter` which creates a new stacking context.
**How to avoid:** Dialog portal renders outside the nav's DOM subtree. `@base-ui/react/dialog` Portal appends to `<body>` by default — verify nav z-index vs dialog z-index (dialog backdrop should be above nav: `z-50` or higher).

### Pitfall 3: `dark:` Classes in Button.tsx
**What goes wrong:** The existing `button.tsx` has several `dark:` variant classes (e.g., `dark:border-input`, `dark:bg-input/30`, `dark:aria-invalid:*`). Since the app is dark-only (no `.dark` class toggle), these classes never activate.
**Why it happens:** `button.tsx` was scaffolded from shadcn which assumes light/dark toggling.
**How to avoid:** When adding the `gradient` variant and doing other Button work, clean up orphaned `dark:` classes.

### Pitfall 4: Storybook stories importing from wrong Button
**What goes wrong:** `Button.stories.ts` imports from `'./Button'` (the Storybook example component at `src/stories/Button.tsx`), not from `@/components/ui/button`.
**Why it happens:** Storybook scaffold created example files in `src/stories/` — these are separate from the actual UI components.
**How to avoid:** New component stories import from `@/components/ui/[component]`, not from `src/stories/`.

### Pitfall 5: a11y `test: 'todo'` vs `test: 'error'`
**What goes wrong:** COMP requirement says a11y shows zero violations in 'error' mode. The current `preview.ts` has `test: 'todo'` — violations show as warnings, not test failures.
**Why it happens:** 'todo' is the safe default for gradual adoption.
**How to avoid:** Change to `test: 'error'` only AFTER all stories are written and pass manually. Make this the final step to avoid blocking story development.

### Pitfall 6: `box-shadow` glow with Kick green
**What goes wrong:** Kick's `#53FC18` neon green glow at 40% alpha looks washed out or harsh.
**Why it happens:** Very high-chroma green in oklch space renders differently than purple/teal.
**How to avoid:** Use the existing `--shadow-glow-kick` CSS variable from `globals.css` which already has the calibrated alpha. Don't recalculate.

### Pitfall 7: COMP-09 — events.css still has !important
**What goes wrong:** Assuming Phase 23 removed all `!important` — it may not have if the migration was partial.
**Why it happens:** Phase 23 PLAN focused on layer ordering; individual `!important` removal may need a final audit.
**How to avoid:** Run `grep -n '!important' frontend/src/styles/events.css` as the first task — count must be zero.

---

## Code Examples

### Storybook Story Pattern for Design System Components
```typescript
// Source: frontend/src/stories/Button.stories.ts (template)
import type { Meta, StoryObj } from '@storybook/nextjs-vite'
import { Card } from '@/components/ui/card'  // import from actual component

const meta = {
  title: 'UI/Card',
  component: Card,
  parameters: { layout: 'centered' },
  tags: ['autodocs'],
} satisfies Meta<typeof Card>

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = { args: { children: 'Card content' } }
export const Interactive: Story = { args: { interactive: true, children: 'Hover me' } }
```

### @base-ui/react Dialog Full Structure
```typescript
// Source: @base-ui/react/dialog/index.d.ts — exports Root, Trigger, Portal, Popup, Backdrop, Title, Description, Close
import { Dialog } from "@base-ui/react/dialog"

function ConfirmDialog({ open, onOpenChange, title, children }) {
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Backdrop className="fixed inset-0 bg-black/60 backdrop-blur-[8px] z-40" />
        <Dialog.Popup className="fixed left-1/2 top-1/2 z-50 -translate-x-1/2 -translate-y-1/2 w-full max-w-md rounded-xl border border-border bg-surface p-6 shadow-xl">
          <Dialog.Title className="text-lg font-semibold text-text">{title}</Dialog.Title>
          <Dialog.Description className="mt-2 text-sm text-text-sub">{children}</Dialog.Description>
          <Dialog.Close className="mt-4">Close</Dialog.Close>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  )
}
```

### @base-ui/react Toast Full Setup
```typescript
// Source: @base-ui/react/toast — createToastManager, ToastProvider, useToastManager

// 1. Singleton manager (src/lib/toast.ts)
import { createToastManager } from "@base-ui/react/toast"
export const toastManager = createToastManager()

// 2. Root layout (app/layout.tsx) — wrap with provider, render viewport
import { Toast } from "@base-ui/react/toast"
import { toastManager } from "@/lib/toast"
<Toast.Provider toastManager={toastManager} timeout={4000}>
  <Toast.Viewport className="fixed bottom-4 right-4 z-50 flex flex-col-reverse gap-2" />
  {children}
</Toast.Provider>

// 3. Trigger from anywhere
import { toastManager } from "@/lib/toast"
toastManager.add({ title: "Saved", type: "success" })
toastManager.add({ title: "Connection failed", type: "error", timeout: 0 })
```

### COMP-09 Verification Command
```bash
grep -c '!important' frontend/src/styles/events.css
# Expected output: 0
```

### Performance Budget Measurement
```typescript
// Wrap message render with performance.now() to verify <16ms
const t0 = performance.now()
// render chat message
const t1 = performance.now()
console.log(`Render: ${t1 - t0}ms`)
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| shadcn default HSL tokens (`--primary`, `--secondary`) | Custom oklch tokens (`--color-bg`, `--color-surface`, `--color-twitch`) | Phase 23 | All components must use the new token names, not shadcn defaults |
| `react-hot-toast` for toasts | `@base-ui/react/toast` for design system | Phase 24 | Two toast systems coexist; admin uses react-hot-toast, UI components use @base-ui |
| `!important` for event override specificity | `@layer marketplace-themes` cascade priority | Phase 23 | events.css rules in `marketplace-themes` layer naturally beat `design-system` layer |
| shadcn CLI generates complete components | shadcn CLI scaffolds shell only, then customized for @base-ui | Phase 23 decision | Use `shadcn` CLI for initial file creation, then replace implementation |

**Deprecated/outdated in this project:**
- `dark:` variant classes on Button — never activate in dark-only app, to be cleaned up
- `react-hot-toast` `ToastProvider` in admin — not removed this phase (admin migration is Phase 25), but do not add any new `react-hot-toast` usage

---

## Open Questions

1. **Conflict between admin `react-hot-toast` and new `@base-ui/react/toast` in shared layout**
   - What we know: `admin/ToastProvider.tsx` uses `react-hot-toast`; root layout would need `@base-ui/react` Toast.Provider for the new system
   - What's unclear: Whether both can coexist in the same React tree (they should — different DOM portals)
   - Recommendation: Place `@base-ui/react` Toast.Provider in root layout; leave admin `react-hot-toast` Toaster untouched until Phase 25 admin redesign

2. **Storybook globals.css import — design tokens available in stories?**
   - What we know: `.storybook/preview.ts` does not import `globals.css` currently
   - What's unclear: Whether CSS custom properties from `globals.css` are available when running Storybook stories
   - Recommendation: Import `globals.css` in `.storybook/preview.ts` as first task; without it, token-based styles will not render correctly in stories

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Vitest 4.0.18 + `@storybook/addon-vitest` 10.2.17 |
| Config file | `frontend/vitest.config.ts` |
| Quick run command | `cd frontend && npx vitest run --project unit` |
| Full suite command | `cd frontend && npx vitest run` (unit + storybook browser tests) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| COMP-01 | Components exist and render | Storybook story | `cd frontend && npx vitest run --project storybook` | ❌ Wave 0 |
| COMP-02 | Design tokens applied (not raw colors) | Visual / Storybook | `cd frontend && npx vitest run --project storybook` | ❌ Wave 0 |
| COMP-03 | CVA variants type-check | unit (tsc) | `cd frontend && npx tsc --noEmit` | n/a — runs on component creation |
| COMP-04 | Hover transitions defined | Visual / Storybook | `cd frontend && npx vitest run --project storybook` | ❌ Wave 0 |
| COMP-05 | Gradient variant on Button | Storybook story | `cd frontend && npx vitest run --project storybook` | ❌ Wave 0 |
| COMP-06 | Platform badge renders correct colors | Storybook story | `cd frontend && npx vitest run --project storybook` | ❌ Wave 0 |
| COMP-07 | Skeleton animates (animate-pulse class present) | Storybook story | `cd frontend && npx vitest run --project storybook` | ❌ Wave 0 |
| COMP-08 | Bundle size delta < 100KB | manual | `cd frontend && npx next build 2>&1 \| grep "First Load"` | n/a |
| COMP-09 | Zero !important in events.css | Shell check | `grep -c '!important' frontend/src/styles/events.css` | n/a — audit task |
| All | Zero a11y violations in 'error' mode | Storybook a11y | `cd frontend && npx vitest run --project storybook` | ❌ Wave 0 — change preview.ts |

### Sampling Rate
- **Per task commit:** `cd frontend && npx tsc --noEmit` (fast type check)
- **Per wave merge:** `cd frontend && npx vitest run --project storybook` (browser story tests)
- **Phase gate:** Full suite green + `grep -c '!important' frontend/src/styles/events.css` returns 0

### Wave 0 Gaps
- [ ] `frontend/src/stories/Card.stories.tsx` — covers COMP-01, COMP-02, COMP-04
- [ ] `frontend/src/stories/Input.stories.tsx` — covers COMP-01, COMP-02
- [ ] `frontend/src/stories/Badge.stories.tsx` — covers COMP-01, COMP-06
- [ ] `frontend/src/stories/Dialog.stories.tsx` — covers COMP-01, COMP-02
- [ ] `frontend/src/stories/Toast.stories.tsx` — covers COMP-01, COMP-05 (gradient CTA in action button)
- [ ] `frontend/src/stories/Skeleton.stories.tsx` — covers COMP-07
- [ ] `.storybook/preview.ts` — add `import '../src/app/globals.css'` so CSS tokens render in stories
- [ ] `.storybook/preview.ts` — change `test: 'todo'` to `test: 'error'` as FINAL step after all stories pass

---

## Sources

### Primary (HIGH confidence)
- `frontend/node_modules/@base-ui/react/` — Direct inspection of installed v1.2.0: dialog/index.d.ts, toast/index.d.ts, input/Input.d.ts, toast/createToastManager.d.ts, toast/useToastManager.d.ts, CHANGELOG.md
- `frontend/src/components/ui/button.tsx` — Existing reference implementation confirming the exact wrapping pattern
- `frontend/src/app/globals.css` — Confirmed token names available: `--color-badge-bg`, `--color-surface-2`, `--color-border`, `--shadow-glow-twitch/youtube/kick/tiktok`, `--color-twitch/youtube/kick/tiktok`
- `frontend/src/lib/platform-colors.ts` — Confirmed `PLATFORM_COLORS` map shape with full literal class strings
- `frontend/package.json` — Confirmed all dependency versions
- `frontend/.storybook/main.ts` — Confirmed `@storybook/addon-a11y` installed; stories glob: `../src/**/*.stories.@(js|jsx|mjs|ts|tsx)`
- `frontend/.storybook/preview.ts` — Confirmed `test: 'todo'` (needs upgrade to 'error')
- `frontend/vitest.config.ts` — Confirmed two projects: `unit` (node) and `storybook` (chromium browser)

### Secondary (MEDIUM confidence)
- `frontend/src/styles/events.css` — Inspected for remaining `!important` patterns; all visible rules are now inside `@layer marketplace-themes`, no `!important` visible (but grep confirmation is the authoritative check)

### Tertiary (LOW confidence)
- Storybook globals.css import requirement — inferred from Storybook docs pattern; not explicitly verified via docs fetch but standard Storybook Next.js Vite setup requires explicit CSS import in preview.ts

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all packages directly inspected in node_modules
- Architecture: HIGH — Button.tsx is the complete working template; all patterns derived from it
- Pitfalls: HIGH — derived from direct code inspection (dark: classes in button.tsx, 'todo' in preview.ts, wrong story import path)
- Validation: HIGH — vitest.config.ts directly read, both test projects confirmed

**Research date:** 2026-03-10
**Valid until:** 2026-04-10 (stable stack, 30-day window)
