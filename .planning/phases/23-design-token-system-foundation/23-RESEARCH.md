# Phase 23: Design Token System & Foundation - Research

**Researched:** 2026-03-10
**Domain:** Tailwind CSS v4 @theme, CSS cascade layers, design tokens, oklch color space
**Confidence:** HIGH

## Summary

Phase 23 is a CSS infrastructure phase with zero component or page changes. It replaces the current conflicting `globals.css` (two `@theme` blocks, two `@layer base` blocks, one bare `:root` oklch block, one bare `:root` HSL block) with a single authoritative dark-only token set using the three-layer hierarchy (base → semantic → component) and four explicitly ordered cascade layers.

The work is well-contained: rewrite `globals.css`, migrate four files from `bg-gradient-to-*` to `bg-linear-to-*`, replace `getPlatformColor()` string-based switch with a static map, define `EVENTS_CSS_API.md` as the stability contract, and create `frontend/src/styles/EVENTS_CSS_API.md`. The codebase already runs Tailwind v4.1.18 — no version upgrade is needed. All decisions are locked in CONTEXT.md; this research verifies the technical patterns that will be used.

**Primary recommendation:** Use `@theme` for base and semantic tokens, `@theme inline` for tokens that reference other CSS variables, and `@layer` (CSS standard) for the four-layer cascade ordering — all in one `globals.css` file.

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Visual Identity**
- Platform colors ARE the brand — no single brand color. The four platform colors are the entire visual language.
- Background: near-black `#07070a`
- Surface: `#0d0d12`
- Dark-only — light theme explicitly deferred to v2. All tokens defined for dark mode only.
- StreamElements Modern aesthetic as reference — professional, streaming-focused, not consumer-generic

**Platform Colors (exact values, used throughout)**
- Twitch: `#9146FF` / RGB `145,70,255`
- YouTube: `#FF4444` / RGB `255,68,68`
- Kick: `#53FC18` / RGB `83,252,24`
- TikTok: `#69C9D0` / RGB `105,201,208`

**Typography**
- Primary font: Barlow (Google Fonts), weights 400/500/600/700/800
- Monospace: DM Mono — used for labels, badges, timestamps, platform tags
- No fallback to system fonts for brand-facing UI

**Token Structure**
- All tokens live in `globals.css` under a single `@theme` block — no split files
- Three-layer hierarchy: Base (raw palette) → Semantic (purpose-named) → Component (specific use)
- Clean slate rewrite of `globals.css` — both conflicting `:root` blocks (HSL + oklch) removed, replaced with single authoritative dark-mode token set in oklch
- oklch color space for all semantic tokens

**Glow & Effects System (stat cards only)**
- Magnetic glow: global cursor projected into card-local space (not clamped)
- Directional border: 4 inset box-shadows per card, brightness = dot product of edge normal with cursor-to-card-centre vector
- Intensity falloff: quadratic, MAX_DIST = 520px
- Idle animation: Lissajous paths with breathing opacity after 1800ms no movement
- Noise layer: SVG feTurbulence fractalNoise at opacity 0.055, mix-blend-mode: overlay
- Flashy mode toggle: user preference (persisted)

**Component Glow Rules**
- Stat cards: full magnetic glow system
- Overlay cards: NO glow — plain flat, 3px colored top border (primary platform only)
- Chat messages: glowing dot indicator (platform color, box-shadow glow) — static

**Logo**
- Shape: Heroicons solid `chat-bubble-left` SVG as bubble, Lucide `infinity` path as mark
- Infinity animation: 4 solid-colour path layers (one per platform), stroke-dasharray segments, constant loop 6000ms
- Bubble style: filled, rgba(255,255,255,0.07) fill, rgba(255,255,255,0.1) stroke

**Nav**
- Frosted glass: `backdrop-filter: blur(20px) saturate(1.5)`, `background: rgba(7,7,10,0.8)`
- Active nav underline: `linear-gradient(90deg, rgb(145,70,255), rgb(105,201,208))`
- LIVE badge: green pulsing dot, `rgba(34,197,94,0.08)` background
- CTA button: `rgba(255,255,255,0.07)` background, subtle border

**CSS Cascade Layers**
- `@layer base` — reset, html/body defaults
- `@layer design-system` — all component styles using tokens
- `@layer marketplace-themes` — overlay theme overrides (replaces `!important` in events.css)
- `@layer user-overrides` — user-specific CSS injected via Monaco editor

**events.css Stability Contract**
- Public API = all class names in events.css: `event-message`, `event-tier-high`, `event-tier-medium`, `event-tier-low`, `event-title`, `event-value`, plus platform indicator classes
- These class names are FROZEN — never rename or remove without a deprecation notice
- `!important` in events.css replaced by cascade layer specificity
- Contract document (`EVENTS_CSS_API.md`) to be created in `frontend/src/styles/`

### Claude's Discretion
- Exact oklch values for neutral scale (audit against StreamElements reference)
- Specific spacing/radius token values
- Exact `SEG_FRAC` for infinity snake (currently 0.55 — adjust for visual balance at nav size)

### Deferred Ideas (OUT OF SCOPE)
- Light theme — explicitly v2, not in v1.3 scope
- Flashy mode toggle UI — the preference itself is decided (it exists), the settings page UI is Phase 25
- Framer Motion / advanced animations — deferred (tw-animate-css covers micro-interactions)
- Logo as actual SVG asset / favicon — Phase 25 when pages are built
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| FOUND-01 | Design token system established with Tailwind v4 @theme directive (colors, spacing, typography, shadows) | @theme directive documented: namespace-based token generation confirmed. Three layers all go inside single @theme block in globals.css. |
| FOUND-02 | Three-layer token hierarchy implemented (base → semantic → component) | Pattern verified from Tailwind v4 docs + maviklabs guide: base primitives → semantic intent → component specifics, all within @theme. Semantic tokens reference base via CSS var(). |
| FOUND-03 | Static platform color mapping object created (no dynamic class construction) | JIT-safe pattern: export const PLATFORM_COLORS = { twitch: 'text-[#9146FF]' } or CSS var references. No string concatenation. |
| FOUND-04 | Overlay CSS stability contract documented (events.css classes as public API) | Documentation pattern established; EVENTS_CSS_API.md format defined in this research. |
| FOUND-05 | Tailwind v4 gradient codemod executed (bg-gradient-to-* → bg-linear-to-*) | 4 files confirmed with specific line numbers. npx @tailwindcss/upgrade or manual find-replace is the approach. |
| FOUND-06 | CSS cascade layers defined (@layer base, design-system, marketplace-themes, user-overrides) | Verified: Tailwind v4 only reserves base/components/utilities — custom layer names pass through unmodified. Order declaration at top of globals.css controls cascade priority. |
</phase_requirements>

---

## Standard Stack

### Core (already installed — no new installs needed)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| tailwindcss | ^4.1.18 | Utility CSS framework with @theme | Already installed, v4 CSS-first config |
| @tailwindcss/postcss | ^4.1.18 | PostCSS integration | Already installed |
| tw-animate-css | ^1.4.0 | CSS animation utilities | Already installed for micro-interactions |
| class-variance-authority | ^0.7.1 | Component variant patterns | Already installed |
| tailwind-merge | ^3.5.0 | Merge conflicting Tailwind classes | Already installed |

### Supporting (design-time references only)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| @tailwindcss/upgrade | npx, no install | Codemod for bg-gradient-to-* → bg-linear-to-* | One-time migration run |
| Google Fonts (Barlow + DM Mono) | n/a | Typography assets | Load via Next.js `next/font/google` or CSS `@import` |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| oklch in @theme | HSL CSS variables | oklch is perceptually uniform; already partially used in globals.css dark block; consistent with Tailwind v4's own defaults |
| Single globals.css | Split token files | CONTEXT.md locked: single file. Simpler import chain for Next.js layout.tsx |
| npx @tailwindcss/upgrade | Manual find-replace | Codemod handles 90% automatically, but project already on v4 — codemod may skip already-upgraded patterns. Manual for 4 known files is safer and verifiable. |

**Installation:**
No new packages required. All dependencies are already installed.

---

## Architecture Patterns

### Recommended globals.css Structure

```
globals.css
├── @import statements (tailwindcss, tw-animate-css)
├── @layer order declaration (base, design-system, marketplace-themes, user-overrides)
├── @theme block — base tokens (platform colors, neutral scale, raw palette)
├── @theme block — semantic tokens (background, surface, text, border — referencing base)
├── @theme block — component tokens (stat-card-glow, badge-bg, etc.)
├── @theme inline block — font references (var(--font-barlow) bridge)
└── @layer base — html/body resets, font-face or next/font variable hookup
```

### Token File Layout

```
frontend/src/
├── app/
│   └── globals.css          # Single source of truth — all tokens + cascade layers
├── styles/
│   ├── events.css            # Overlay marketplace public API (no !important)
│   └── EVENTS_CSS_API.md     # Stability contract document (new file)
```

### Pattern 1: Three-Layer @theme Token Hierarchy

**What:** All tokens in a single `@theme` block, organized by layer tier via CSS comments. Base tokens use raw oklch values. Semantic tokens reference base via `var()`. Component tokens reference semantic.

**When to use:** Always — this is the only pattern for this phase.

```css
/* Source: https://tailwindcss.com/docs/theme + maviklabs.com/blog/design-tokens-tailwind-v4-2026 */

@theme {
  /* ── LAYER 1: BASE TOKENS (raw palette) ── */

  /* Platform brand colors — exact hex converted to oklch */
  --color-twitch:  oklch(0.54 0.28 284);   /* #9146FF */
  --color-youtube: oklch(0.59 0.22 25);    /* #FF4444 */
  --color-kick:    oklch(0.91 0.28 143);   /* #53FC18 */
  --color-tiktok:  oklch(0.78 0.07 198);   /* #69C9D0 */

  /* Neutral scale — near-black to near-white */
  --color-neutral-950: oklch(0.09 0.007 270);  /* #07070a — bg */
  --color-neutral-900: oklch(0.11 0.009 270);  /* #0d0d12 — surface */
  --color-neutral-800: oklch(0.14 0.008 270);  /* #111118 — surface2 */
  --color-neutral-700: oklch(0.22 0.007 270);  /* elevated border */
  --color-neutral-600: oklch(0.35 0.007 270);  /* muted text */
  --color-neutral-400: oklch(0.58 0.007 270);  /* secondary text */
  --color-neutral-100: oklch(0.91 0.003 270);  /* primary text */

  /* ── LAYER 2: SEMANTIC TOKENS (purpose-named) ── */

  --color-bg:          var(--color-neutral-950);
  --color-surface:     var(--color-neutral-900);
  --color-surface-2:   var(--color-neutral-800);
  --color-border:      oklch(from var(--color-neutral-100) l c h / 0.06);
  --color-border-md:   oklch(from var(--color-neutral-100) l c h / 0.10);
  --color-text:        var(--color-neutral-100);
  --color-text-sub:    var(--color-neutral-400);
  --color-text-dim:    var(--color-neutral-600);

  /* ── LAYER 3: COMPONENT TOKENS ── */

  /* Stat card glow system */
  --color-stat-glow-twitch:  oklch(from var(--color-twitch) l c h / 0.25);
  --color-stat-glow-youtube: oklch(from var(--color-youtube) l c h / 0.25);
  --color-stat-glow-kick:    oklch(from var(--color-kick) l c h / 0.25);
  --color-stat-glow-tiktok:  oklch(from var(--color-tiktok) l c h / 0.25);

  /* Badge backgrounds */
  --color-badge-bg: oklch(from var(--color-neutral-100) l c h / 0.07);

  /* Nav */
  --color-nav-bg: oklch(from var(--color-bg) l c h / 0.80);
}
```

**IMPORTANT — @theme inline for font references:**
When a `@theme` variable must reference another CSS variable (e.g. Next.js injects font vars into `:root`), use `@theme inline` to ensure the utility class resolves the variable at use-site, not definition-site:

```css
/* Source: https://tailwindcss.com/docs/theme */
@theme inline {
  --font-sans: var(--font-barlow);
  --font-mono: var(--font-dm-mono);
}
```

### Pattern 2: CSS Cascade Layer Order Declaration

**What:** Declare all layer names at the top of globals.css before any `@layer` usage. This locks precedence order; later-declared layers win over earlier-declared ones regardless of source order.

**When to use:** Once, at the top of globals.css, after @import lines.

```css
/* Source: https://css-tricks.com/using-css-cascade-layers-with-tailwind-utilities/ */
/* Source: https://github.com/tailwindlabs/tailwindcss/discussions/6694 */

/* Tailwind reserves: base, components, utilities */
/* Custom layers pass through unmodified in Tailwind v4 */
/* Precedence: user-overrides wins (last declared) */
@layer base, design-system, marketplace-themes, user-overrides;
```

**Critical detail:** Tailwind v4 only reserves `base`, `components`, and `utilities` as its built-in layer names. Any other name (like `design-system`, `marketplace-themes`, `user-overrides`) is passed through to the browser CSS cascade unmodified. This means all four layers in the decisions work exactly as intended.

**Cascade priority order (lowest → highest):**
1. `base` — html/body resets (lowest)
2. `design-system` — component styles using tokens
3. `marketplace-themes` — events.css overrides (beats design-system without !important)
4. `user-overrides` — Monaco editor CSS injected at runtime (highest)

### Pattern 3: Static Platform Color Mapping Object

**What:** A TypeScript export of a plain object mapping platform names to their design token CSS classes. No string concatenation. No dynamic class names.

**Why:** Tailwind's JIT compiler scans source code at build time. If class names are assembled at runtime via string concatenation (e.g. `"text-" + platform`), the JIT never sees the complete class name and does not emit it.

**When to use:** Replace `getPlatformColor()` switch in `ThemePreview.tsx` and any other file using dynamic platform class construction.

```typescript
// Source: JIT compilation constraint documented at tailwindcss.com/docs/content-configuration
// File: frontend/src/lib/platform-colors.ts

export const PLATFORM_TEXT_COLORS = {
  twitch:  'text-twitch',   // resolved to --color-twitch token
  youtube: 'text-youtube',
  kick:    'text-kick',
  tiktok:  'text-tiktok',
  system:  'text-text-sub',
} as const;

export const PLATFORM_BG_COLORS = {
  twitch:  'bg-twitch',
  youtube: 'bg-youtube',
  kick:    'bg-kick',
  tiktok:  'bg-tiktok',
} as const;

export type Platform = keyof typeof PLATFORM_TEXT_COLORS;
```

All class names appear as complete string literals in source → Tailwind JIT sees them → classes are emitted.

### Pattern 4: events.css Migration to Cascade Layers

**What:** Wrap all existing `events.css` rules in `@layer marketplace-themes { }`. Remove all `!important` declarations. The layer itself now wins over `design-system` layer via cascade order, not specificity hacks.

**When to use:** events.css update in this phase.

```css
/* Before (current) */
.event-message {
  padding: 1.5rem !important;
  border-radius: 16px !important;
}

/* After — cascade layer handles priority, !important removed */
@layer marketplace-themes {
  .event-message {
    padding: 1.5rem;
    border-radius: 16px;
  }
}
```

**Count of `!important` in current events.css:** ~14 declarations. All removed in this phase.

### Pattern 5: Gradient Codemod (Manual — 4 Files)

**What:** Rename `bg-gradient-to-*` → `bg-linear-to-*` in four known files.

**Files and approximate line numbers (confirmed by grep):**
1. `frontend/src/app/overlay/[id]/credits/page.tsx` — lines 180, 191, 204 (3 occurrences)
2. `frontend/src/app/page.tsx` — lines 77, 196 (2 occurrences)
3. `frontend/src/components/legal/LegalLayout.tsx` — line 11 (1 occurrence)
4. `frontend/src/components/theme-marketplace/CreditRollThemePreview.tsx` — line 103 (1 occurrence)

Total: 7 class replacements across 4 files.

**Approach:** Manual find-replace (not `npx @tailwindcss/upgrade`) because:
- The project is already on Tailwind v4; upgrade tool may not re-run against an already-migrated project
- 7 replacements are auditable manually in under 5 minutes
- Visual regression validation needed post-migration regardless

### Anti-Patterns to Avoid

- **Separate @theme blocks for base/semantic/component:** Use CSS comments to organize tiers within a single `@theme` block. Multiple `@theme` blocks can cause specificity confusion and are harder to audit. CONTEXT.md locks single-block approach.
- **Dynamic Tailwind class construction:** Never `className={\`text-${platform}-400\`}`. Always use static map lookups.
- **`!important` in events.css:** Replace with cascade layer ordering.
- **Retaining the duplicate `:root` blocks:** The current globals.css has an HSL `:root` inside `@layer base` AND a bare oklch `:root` outside any layer AND a `.dark` block. All three conflict. The clean slate rewrite removes all of them.
- **`@theme inline` for base color tokens:** Use plain `@theme` (not inline) for tokens with raw oklch values. Use `@theme inline` only for tokens that reference other CSS variables (font vars injected by Next.js).
- **Registering all four layers with Tailwind's own reserved names:** `components` is Tailwind's reserved layer. Our four custom layers are `base`, `design-system`, `marketplace-themes`, `user-overrides`. The `base` name is shared with Tailwind — that's intentional, resets go in `@layer base`.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Merge conflicting Tailwind classes | Custom cn() implementation | `tailwind-merge` (already installed) | Handles specificity collisions, variant conflicts, and responsive prefix deduplication correctly |
| Component variants (size, color, state) | if/else chains in className | `class-variance-authority` (already installed) | Type-safe slots, compound variants, handles all edge cases |
| Animation utilities | Hand-written @keyframes in globals | `tw-animate-css` (already installed) | Covers hover, fade, slide-up patterns needed |
| oklch value calculation | Manual hex → oklch conversion | Use oklch.com/color-picker or CSS `color()` | OKLCH conversion requires precise math; tools prevent perceptual inconsistencies |
| Cascade layer specificity | `!important` everywhere | `@layer` ordering | CSS spec-compliant; future-proof; already in all modern browsers |

**Key insight:** Phase 23 is pure CSS/token infrastructure. Everything that could be hand-rolled already has a library. The only genuinely new code is the globals.css rewrite, the events.css layer migration, the static color map, and the API contract document.

---

## Common Pitfalls

### Pitfall 1: @theme vs @theme inline Confusion

**What goes wrong:** Semantic tokens that reference other CSS variables are defined with plain `@theme`. The utility class then outputs `font-family: var(--font-barlow)` instead of `font-family: var(--font-sans)` — browser resolves the variable at the wrong scope, and if `--font-barlow` is not defined on `:root` yet, the font falls back to system.

**Why it happens:** Without `inline`, `@theme` embeds the variable name, not the resolved value. Next.js injects font class variables (`--font-barlow`) into the document root after the CSS is parsed.

**How to avoid:** Use `@theme inline` exclusively for tokens that reference `var(--something)`. Use plain `@theme` for tokens with direct values (oklch literals, rem values, etc.).

**Warning signs:** Fonts show as system sans-serif in dev; computed styles show `var(--font-barlow)` not resolving.

### Pitfall 2: Layer Order Not Declared Before First Use

**What goes wrong:** If `@layer marketplace-themes { }` appears in events.css but the order declaration (`@layer base, design-system, marketplace-themes, user-overrides`) is absent from globals.css, browsers determine layer precedence by order of first encounter. This produces non-deterministic cascade behavior depending on CSS load order.

**Why it happens:** CSS cascade layers require an explicit order declaration to be reliable across different import orders.

**How to avoid:** The `@layer base, design-system, marketplace-themes, user-overrides;` declaration must be the first thing after `@import` statements in globals.css — before any `@layer` rule bodies.

**Warning signs:** events.css styles intermittently lose to design-system styles in certain pages; overlay previews look different from marketplace preview.

### Pitfall 3: Tailwind v4 Gradient Direction Mismatch

**What goes wrong:** `bg-gradient-to-br` renamed to `bg-linear-to-br`. Running the upgrade codemod on an already-migrated project may produce no changes (correct) OR may double-migrate partial migrations. Visual regression needed.

**Why it happens:** 4 files still use the old `bg-gradient-to-*` pattern. The project is already on v4 but these were not migrated.

**How to avoid:** Manual replacement per the 4-file list above. After replacing, do a `grep -r "bg-gradient-to-" frontend/src` to confirm zero remaining occurrences.

**Warning signs:** Build succeeds but gradient backgrounds show solid colors (class not recognized).

### Pitfall 4: oklch Color Accuracy for Locked Hex Values

**What goes wrong:** Converting `#9146FF` to oklch manually produces `oklch(0.54 0.28 284)` — but the actual perceptual match depends on the conversion tool used. Different tools may produce slightly different values. If the platform colors drift from exact spec, brand consistency across platforms breaks.

**Why it happens:** oklch is perceptually uniform but the roundtrip hex→oklch→rendered hex must be validated in browser.

**How to avoid:** For platform colors (LOCKED values), use `@theme` with the `#hex` values directly — not oklch. Only use oklch for semantic/neutral scale tokens where perceptual uniformity matters. Platform colors in @theme can stay as hex; they generate `bg-twitch`, `text-twitch` utilities fine.

**Warning signs:** Twitch purple renders noticeably different from `#9146FF` in browser DevTools color picker.

### Pitfall 5: events.css @layer and Next.js Import Order

**What goes wrong:** `events.css` is loaded in overlay preview pages, NOT in the main app layout. If the layer order declaration only exists in globals.css, overlay preview pages that load events.css separately will not inherit the layer order.

**Why it happens:** CSS cascade layers are document-scoped. A page that loads events.css without globals.css gets its own cascade layer ordering.

**How to avoid:** The layer order declaration (`@layer base, design-system, marketplace-themes, user-overrides;`) must also be at the top of events.css (or in a shared layer-order.css imported by both). Since events.css is the public API file, put the declaration at its top.

**Warning signs:** Overlay previews render events.css styles at wrong specificity; test overlay page differs from marketplace preview page.

---

## Code Examples

Verified patterns from official sources and project context:

### globals.css Top-Level Structure

```css
/* Source: tailwindcss.com/docs/theme, project globals.css existing pattern */

@import "tailwindcss";
@import "tw-animate-css";

/* Cascade layer order — must precede all @layer bodies */
@layer base, design-system, marketplace-themes, user-overrides;

/* ── Font bridge (Next.js injects these vars) ── */
@theme inline {
  --font-sans: var(--font-barlow);
  --font-mono: var(--font-dm-mono);
}

/* ── Design Tokens ── */
@theme {
  /* LAYER 1: BASE — raw palette */
  --color-twitch:  #9146FF;
  --color-youtube: #FF4444;
  --color-kick:    #53FC18;
  --color-tiktok:  #69C9D0;

  /* Neutral scale in oklch */
  --color-neutral-950: oklch(0.09 0.007 270);
  /* ... */

  /* LAYER 2: SEMANTIC — purpose-named */
  --color-bg:       var(--color-neutral-950);
  --color-surface:  var(--color-neutral-900);
  --color-text:     var(--color-neutral-100);
  /* ... */

  /* LAYER 3: COMPONENT — specific use */
  --color-stat-glow-twitch: color-mix(in oklch, var(--color-twitch) 25%, transparent);
  /* ... */

  /* Typography */
  --text-xs: 0.6875rem;
  --text-sm: 0.8125rem;
  --text-base: 0.875rem;
  --text-lg: 1rem;
  --text-xl: 1.25rem;

  /* Spacing scale */
  --spacing-1:  0.25rem;
  --spacing-2:  0.5rem;
  --spacing-3:  0.75rem;
  --spacing-4:  1rem;
  --spacing-6:  1.5rem;
  --spacing-8:  2rem;

  /* Border radius */
  --radius-sm:  4px;
  --radius-md:  8px;
  --radius-lg:  12px;
  --radius-xl:  16px;
  --radius-full: 9999px;

  /* Shadows */
  --shadow-glow-twitch:  0 0 20px color-mix(in oklch, var(--color-twitch) 40%, transparent);
  --shadow-glow-youtube: 0 0 20px color-mix(in oklch, var(--color-youtube) 40%, transparent);
  --shadow-glow-kick:    0 0 20px color-mix(in oklch, var(--color-kick) 40%, transparent);
  --shadow-glow-tiktok:  0 0 20px color-mix(in oklch, var(--color-tiktok) 40%, transparent);
}

/* ── Base layer: resets + html/body ── */
@layer base {
  *, *::before, *::after {
    box-sizing: border-box;
    border-color: var(--color-border);
  }
  html {
    font-family: var(--font-sans);
    background: var(--color-bg);
    color: var(--color-text);
  }
  body {
    background: var(--color-bg);
    color: var(--color-text);
  }
}
```

### Static Platform Color Map

```typescript
// Source: Tailwind JIT constraint — tailwindcss.com/docs/content-configuration
// File: frontend/src/lib/platform-colors.ts

export const PLATFORM_COLORS = {
  twitch:  { text: 'text-twitch',  bg: 'bg-twitch'  },
  youtube: { text: 'text-youtube', bg: 'bg-youtube' },
  kick:    { text: 'text-kick',    bg: 'bg-kick'    },
  tiktok:  { text: 'text-tiktok',  bg: 'bg-tiktok'  },
  system:  { text: 'text-text-sub', bg: 'bg-surface' },
} as const;

export type Platform = keyof typeof PLATFORM_COLORS;

// Usage: PLATFORM_COLORS[platform].text
// Replaces getPlatformColor() in ThemePreview.tsx
```

### events.css Layer Migration Pattern

```css
/* Source: CSS Cascade Layers spec — MDN */
/* events.css — Layer order must be declared here too (standalone page scope) */

@layer base, design-system, marketplace-themes, user-overrides;

@layer marketplace-themes {
  /* All existing rules moved here, !important removed */
  .event-message {
    position: relative;
    overflow: hidden;
    padding: 1.5rem;      /* was: 1.5rem !important */
    min-height: 100px;    /* was: 100px !important */
    border-radius: 16px;  /* was: 16px !important */
  }
  /* ... all other rules migrated the same way */
}
```

### Gradient Migration Example

```tsx
// Before (4 files, 7 total occurrences)
<div className="bg-gradient-to-b from-gray-900 to-black">
<div className="bg-gradient-to-br from-purple-900 via-blue-900 to-indigo-900">

// After
<div className="bg-linear-to-b from-gray-900 to-black">
<div className="bg-linear-to-br from-purple-900 via-blue-900 to-indigo-900">
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `tailwind.config.js` for theme | `@theme` CSS directive | Tailwind v4 (Jan 2025) | No JS needed; tokens inspectable in DevTools |
| `bg-gradient-to-*` | `bg-linear-to-*` | Tailwind v4 | Aligns with CSS `linear-gradient()` naming |
| HSL CSS variables | oklch CSS variables | Tailwind v4 defaults switched to oklch | Perceptually uniform; avoids "dead zones" in color interpolation |
| `@layer` via PostCSS (v3 hijack) | Native CSS `@layer` | Tailwind v4 | Custom layer names work; no conflict with framework internals |
| Multiple `tailwind.config.js` + CSS files | Single `globals.css` with `@theme` | Tailwind v4 | Single source of truth for all design tokens |
| `!important` for overrides | Cascade layer ordering | CSS Cascade Layers (all major browsers 2022+) | Deterministic specificity without `!important` hacks |

**Deprecated/outdated in this codebase:**
- Both `:root` blocks in current `globals.css` (HSL block inside `@layer base` + bare oklch block outside any layer): replace entirely
- `.dark { }` block in globals.css: remove — dark-only, no class toggle needed
- `@theme { hsl(var(--*)) }` indirection pattern: replace with direct oklch values in semantic tokens
- `getPlatformColor()` string switch returning Tailwind class strings: replace with static map

---

## Open Questions

1. **oklch conversion accuracy for locked hex platform colors**
   - What we know: `#9146FF` ≈ `oklch(0.54 0.28 284)` using standard converters
   - What's unclear: Whether to use hex directly or oklch in `@theme` for the four locked platform colors. Hex works in `@theme` — Tailwind v4 accepts hex, oklch, rgb all in `@theme`.
   - Recommendation: Use hex values for the four platform colors (they are brand-exact, perceptual uniformity is irrelevant for locked brand values). Use oklch for the neutral scale where perceptual uniformity of the gradient matters.

2. **Neutral scale oklch values (Claude's Discretion)**
   - What we know: Background = `#07070a`, Surface = `#0d0d12`. Text = `#e8e8f0`, text-sub ≈ `#8a8a9e` (from homepage-reference.html)
   - What's unclear: Full neutral scale between these anchors.
   - Recommendation: Derive the intermediate steps using oklch with constant chroma (~0.007) and hue (270) interpolated between L=0.09 (bg) and L=0.91 (text). Approximately 6-7 steps needed.

3. **color-mix() browser support for glow tokens**
   - What we know: `color-mix(in oklch, var(--color-twitch) 25%, transparent)` is the cleanest way to express glow colors as component tokens.
   - What's unclear: Browser support in OBS browser source (Chromium 103 base). `color-mix()` supported since Chrome 111.
   - Recommendation: Use fallback approach — define glow colors as explicit oklch values with alpha, not color-mix(). Or use `rgba()` for shadow values which are not utility classes. LOW risk for design system CSS; HIGH risk for overlay CSS loaded in OBS.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Vitest 4.0.18 (Storybook integration) + Playwright 1.58.2 (E2E) |
| Config file | `frontend/vitest.config.ts` (Storybook-based), `frontend/playwright.config.ts` |
| Quick run command | `cd frontend && npm test -- --project storybook` (Storybook unit tests) |
| Full suite command | `cd frontend && npm test` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| FOUND-01 | @theme tokens produce expected utility classes | Visual / Storybook smoke | Storybook story renders token swatches | ❌ Wave 0 |
| FOUND-02 | Component tokens resolve through semantic → base chain | Unit | Check CSS var resolution in computed style | ❌ Wave 0 |
| FOUND-03 | PLATFORM_COLORS map returns correct class for each platform | Unit | `vitest` unit test on platform-colors.ts | ❌ Wave 0 |
| FOUND-04 | events.css API contract docs exist; class names unchanged | Manual / file existence check | `test -f frontend/src/styles/EVENTS_CSS_API.md` | ❌ Wave 0 |
| FOUND-05 | No `bg-gradient-to-*` in source | Static analysis | `grep -r "bg-gradient-to-" frontend/src` returns 0 matches | ❌ Wave 0 (grep-based) |
| FOUND-06 | @layer order declaration exists; marketplace-themes beats design-system | E2E / Playwright | Playwright checks overlay preview page specificity | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `cd frontend && grep -r "bg-gradient-to-" src/ && npm run type-check`
- **Per wave merge:** `cd frontend && npm test`
- **Phase gate:** Full suite green + `grep -r "bg-gradient-to-" src/` returns empty before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `frontend/src/lib/__tests__/platform-colors.test.ts` — covers FOUND-03 (static map correctness)
- [ ] `frontend/.storybook/stories/DesignTokens.stories.tsx` — covers FOUND-01 (token visual smoke)
- [ ] `frontend/tests/e2e/design-system.spec.ts` — covers FOUND-06 (cascade layer ordering in overlay preview)

---

## Sources

### Primary (HIGH confidence)
- [tailwindcss.com/docs/theme](https://tailwindcss.com/docs/theme) — @theme directive, namespace list, @theme inline semantics
- [tailwindcss.com/docs/adding-custom-styles](https://tailwindcss.com/docs/adding-custom-styles) — @layer usage, built-in vs custom layers
- [tailwindcss.com/docs/upgrade-guide](https://tailwindcss.com/docs/upgrade-guide) — bg-gradient-to-* → bg-linear-to-* rename, upgrade tool
- Project source: `frontend/src/app/globals.css` — confirmed current state (two :root blocks, conflicting @theme)
- Project source: `frontend/src/styles/events.css` — confirmed 14+ !important declarations, all class names audited
- Project source: `frontend/src/components/theme-marketplace/ThemePreview.tsx` — confirmed getPlatformColor() pattern to migrate
- Project source: `frontend/package.json` — confirmed Tailwind 4.1.18, all dependencies already installed

### Secondary (MEDIUM confidence)
- [css-tricks.com/using-css-cascade-layers-with-tailwind-utilities](https://css-tricks.com/using-css-cascade-layers-with-tailwind-utilities/) — custom named layers in Tailwind v4 pass through unmodified
- [github.com/tailwindlabs/tailwindcss/discussions/6694](https://github.com/tailwindlabs/tailwindcss/discussions/6694) — Tailwind reserves only base/components/utilities
- [maviklabs.com/blog/design-tokens-tailwind-v4-2026](https://www.maviklabs.com/blog/design-tokens-tailwind-v4-2026) — three-layer hierarchy pattern with code examples
- [tailwindcss.com/blog/tailwindcss-v4](https://tailwindcss.com/blog/tailwindcss-v4) — v4 overview, CSS-first config

### Tertiary (LOW confidence — needs validation)
- oklch values for neutral scale (tool-derived, must validate in browser)
- color-mix() in OBS browser source compatibility (Chromium version unknown for project's OBS targets)

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all packages already installed, versions confirmed in package.json
- Architecture patterns: HIGH — all patterns verified against tailwindcss.com official docs
- Gradient migration: HIGH — 4 files confirmed by grep with exact occurrences
- oklch neutral scale values: MEDIUM — derived from reference HTML anchor values, must validate visually
- OBS color-mix() compat: LOW — browser version not known, conservative fallback recommended

**Research date:** 2026-03-10
**Valid until:** 2026-06-10 (Tailwind v4 CSS API is stable; 90 day window before re-verification needed)
