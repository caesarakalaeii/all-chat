# Phase 24: Component Library Setup & Customization - Context

**Gathered:** 2026-03-10
**Status:** Ready for planning

<domain>
## Phase Boundary

Install and customize the remaining shadcn/ui core primitives (Card, Input, Badge, Dialog, Toast) on top of `@base-ui/react`, themed with Phase 23 design tokens. Button is already done — this phase completes the component library and adds platform-coded variants, micro-interactions, loading skeletons, and Storybook documentation. No page redesigns — components only.

</domain>

<decisions>
## Implementation Decisions

### Gradient CTA (COMP-05)
- Primary gradient = Twitch purple → TikTok teal: `linear-gradient(90deg, #9146FF, #69C9D0)`
- Same gradient already used for nav active underline in Phase 23 — reuse exactly, no new gradient definition
- Applied to Button's `gradient` variant (new variant alongside existing default/outline/ghost etc.)
- This is the "platforms are the theme" identity — no generic purple/blue gradients

### Platform Badge Design (COMP-06)
- **Content:** Glow dot + short label in DM Mono (TWITCH / YOUTUBE / KICK / TIKTOK)
- **Dot:** Platform-colored `box-shadow` glow — same pattern as chat message dots from Phase 23
- **Background:** `--color-badge-bg` (7% white opacity, neutral) — platform color only on dot + label text
- **Sizes:** Two CVA variants — `default` (standalone contexts, source lists) and `sm` (inline, table rows)
- Platform color applied via the static `PLATFORM_COLORS` map established in Phase 23

### Loading Skeletons (COMP-07)
- **Animation:** Pulse (opacity fade) — `tw-animate-css` `animate-pulse`. Calmer, non-distracting for a tool used while streaming
- **Color:** `--color-surface-2` for skeleton blocks — neutral, no platform tinting
- Skeleton component wraps arbitrary shapes (use `className` for width/height/radius)

### Dialog
- **Backdrop:** Frosted glass — `backdrop-filter: blur(8px)` + `rgba(0,0,0,0.6)` overlay
- Consistent with frosted nav from Phase 23
- Dialogs only appear in dashboard/editor context — never in overlay pages (no OBS performance concern)

### Toast
- **Position:** Bottom-right
- **Auto-dismiss:** 4 seconds for success/info; errors stay until manually dismissed
- Stacking behavior: new toasts push up from bottom-right

### Micro-interactions (COMP-04)
- Hover: subtle scale (`scale-[1.02]`) + shadow lift on interactive cards
- Transitions via `tw-animate-css` — no Framer Motion (decided in Phase 23)
- Buttons already have `transition-all` from the existing Button component

### Component Variants (COMP-03)
- CVA for all components — same pattern as existing Button
- All components use `cn()` from `@/lib/utils`
- Variants use design tokens, not raw Tailwind color classes

### Claude's Discretion
- Exact CVA variant names beyond what's specified (e.g., Card size variants if needed)
- Input focus ring treatment (use `focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50` pattern from Button)
- Toast enter/exit animation specifics (tw-animate-css provides options)
- Whether Dialog needs size variants (sm/default/lg) — planner decides based on use cases found in codebase

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `frontend/src/components/ui/button.tsx` — Button done, built on `@base-ui/react/button` + CVA. All new components follow this exact pattern.
- `frontend/src/lib/utils.ts` — `cn()` helper, import in every component
- `frontend/src/app/globals.css` — all tokens available: `--color-badge-bg`, `--color-surface-2`, `--color-border`, `--shadow-glow-twitch` etc.
- `frontend/src/stories/Button.stories.ts` — existing Storybook story as template for new stories

### Established Patterns
- `@base-ui/react` primitive wrapping: import primitive → wrap in function with CVA className → export
- `tw-animate-css` already installed — use `animate-pulse` for skeletons, transition utilities for hover states
- Static `PLATFORM_COLORS` map established in Phase 23 — use for badge dot colors, don't reconstruct dynamically
- `@storybook/addon-a11y` already in `.storybook/main.ts` — a11y checking is already wired up

### Integration Points
- All components land in `frontend/src/components/ui/` (alongside existing `button.tsx`)
- Storybook picks up `**/*.stories.@(ts|tsx)` from `src/` — stories go alongside components or in `src/stories/`
- `frontend/src/stories/` has existing default stories (Button, Header, Page) — new stories can go here or co-located
- `shadcn` CLI (`shadcn@4.0.2`) already in package.json — use it to scaffold component shells, then customize for `@base-ui/react`

</code_context>

<specifics>
## Specific Ideas

- The "platforms are the theme" principle governs ALL color decisions: gradient CTA reuses the nav underline gradient (purple→teal), badge dots use platform colors with glow, no new brand colors introduced
- Phase 23 reference mockups in `.planning/phases/23-design-token-system-foundation/` show the full visual language — planner should consult `homepage-reference.html` for component context
- Chat message dots (glowing dot = platform color) are the established pattern for platform indicators — badges extend this same language

</specifics>

<deferred>
## Deferred Ideas

- None — discussion stayed within phase scope

</deferred>

---

*Phase: 24-component-library-setup-customization*
*Context gathered: 2026-03-10*
