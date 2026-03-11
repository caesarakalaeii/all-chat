# Phase 23: Design Token System & Foundation - Context

**Gathered:** 2026-03-10
**Status:** Ready for planning

<domain>
## Phase Boundary

Establish the CSS/token foundation for the v1.3 redesign. This phase delivers:
- Design token system (colors, spacing, typography, shadows) in Tailwind v4 `@theme`
- Three-layer token hierarchy (base → semantic → component)
- Static platform color mapping object
- CSS cascade layers (`@layer base, design-system, marketplace-themes, user-overrides`)
- Tailwind v4 gradient codemod (`bg-gradient-to-*` → `bg-linear-to-*`)
- Overlay marketplace CSS stability contract (events.css as public API)
- Visual identity decisions locked in for all downstream phases

No component or page changes in this phase — infrastructure and tokens only.

</domain>

<decisions>
## Implementation Decisions

### Visual Identity
- **Platform colors ARE the brand** — no single brand color. The four platform colors are the entire visual language.
- **Background:** near-black `#07070a`
- **Surface:** `#0d0d12`
- **Dark-only** — light theme explicitly deferred to v2. All tokens defined for dark mode only.
- **StreamElements Modern** aesthetic as reference — professional, streaming-focused, not consumer-generic

### Platform Colors (exact values, used throughout)
- Twitch: `#9146FF` / RGB `145,70,255`
- YouTube: `#FF4444` / RGB `255,68,68`
- Kick: `#53FC18` / RGB `83,252,24`
- TikTok: `#69C9D0` / RGB `105,201,208`

### Typography
- **Primary font:** Barlow (Google Fonts), weights 400/500/600/700/800
- **Monospace:** DM Mono — used for labels, badges, timestamps, platform tags
- No fallback to system fonts for brand-facing UI

### Token Structure
- All tokens live in `globals.css` under a single `@theme` block — no split files
- Three-layer hierarchy:
  - **Base:** raw palette values (platform colors, neutral scale)
  - **Semantic:** purpose-named (background, surface, text, border)
  - **Component:** specific use (stat-card-glow, badge-bg, etc.)
- Clean slate rewrite of `globals.css` — both conflicting `:root` blocks (HSL + oklch) removed, replaced with single authoritative dark-mode token set in oklch
- oklch color space for all semantic tokens

### Glow & Effects System (stat cards only)
- **Magnetic glow:** global cursor position projected into each card's local space (`mx - rect.left` / `my - rect.top`) — NOT clamped to card bounds. This makes adjacent cards share one light source so the glow sweeps across all 4 cards as one continuous light.
- **Directional border:** 4 inset box-shadows per card, each side's brightness = dot product of that edge's outward normal with cursor-to-card-centre vector × intensity
- **Intensity falloff:** quadratic, `MAX_DIST = 520px`
- **Idle animation:** after 1800ms no movement, cards drift on Lissajous paths with breathing opacity. Starts immediately on page load (no cursor present). Cursor return snaps back to tracking.
- **Mouse leaves window:** fade glows, idle starts after 600ms
- **Noise layer:** SVG `feTurbulence fractalNoise` at `opacity: 0.055`, `mix-blend-mode: overlay` on all glow cards — kills CSS gradient banding
- **Flashy mode toggle:** user preference (persisted), controls whether magnetic glow is active

### Component Glow Rules
- **Stat cards:** full magnetic glow system (cursor-tracked + idle animation + noise layer)
- **Overlay cards:** NO glow — plain flat treatment, 3px colored top border matching primary platform only
- **Chat messages:** glowing dot indicator (platform color, `box-shadow` glow) — static, no interaction

### Logo
- **Shape:** Heroicons solid `chat-bubble-left` SVG as the bubble, Lucide `infinity` path as the mark
- **Infinity animation:** 4 solid-colour path layers (one per platform), each `stroke-dasharray` segment = `total * SEG_FRAC / 4`. Two copies per layer (primary + `-b` offset by `-total`) for seamless wrap. Constant slow loop (`LOOP_MS = 6000ms`). Head = Twitch purple, tail = TikTok teal.
- **Infinity centering:** `translateY(-10%)` to optically centre within bubble body (compensates for tail spike)
- **Bubble style:** filled, `rgba(255,255,255,0.07)` fill, `rgba(255,255,255,0.1)` stroke outline
- **No rotation of the infinity shape** — only the colour segments travel along the path

### Nav
- Frosted glass: `backdrop-filter: blur(20px) saturate(1.5)`, `background: rgba(7,7,10,0.8)`
- Active nav underline: `linear-gradient(90deg, rgb(145,70,255), rgb(105,201,208))` — purple → teal
- LIVE badge: green pulsing dot, `rgba(34,197,94,0.08)` background
- CTA button: `rgba(255,255,255,0.07)` background, subtle border — confident, not loud

### CSS Cascade Layers
- `@layer base` — reset, html/body defaults
- `@layer design-system` — all component styles using tokens
- `@layer marketplace-themes` — overlay theme overrides (replaces `!important` in events.css)
- `@layer user-overrides` — user-specific CSS injected via Monaco editor

### events.css Stability Contract
- Public API = all class names in `events.css`: `event-message`, `event-tier-high`, `event-tier-medium`, `event-tier-low`, `event-title`, `event-value`, plus platform indicator classes
- These class names are FROZEN — never rename or remove without a deprecation notice
- `!important` in events.css replaced by cascade layer specificity (`marketplace-themes` layer wins over `design-system`)
- Contract document (`EVENTS_CSS_API.md`) to be created in `frontend/src/styles/`

### Claude's Discretion
- Exact oklch values for neutral scale (researcher to audit against StreamElements reference)
- Specific spacing/radius token values
- Exact `SEG_FRAC` for infinity snake (currently 0.55 — adjust for visual balance at nav size)

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `frontend/src/app/globals.css` — exists but needs clean-slate rewrite (conflicting `:root` blocks)
- `frontend/tailwind.config.js` — already minimal, comments point to `globals.css @theme` (correct)
- `frontend/src/components/ui/button.tsx` — one shadcn component exists, uses `@base-ui/react` + CVA
- `frontend/src/lib/utils.ts` — assumed to have `cn()` helper (standard shadcn pattern)

### Established Patterns
- Tailwind v4 `@theme` directive already in use (platform colors already defined: `--color-twitch`, etc.)
- `@layer base` already used in globals.css — extend to full 4-layer cascade system
- `tw-animate-css` already installed — use for component transitions
- CVA (`class-variance-authority`) already installed — use for component variants

### Integration Points
- `globals.css` → consumed by entire Next.js app via `layout.tsx` import
- `events.css` → loaded by overlay preview pages, consumed by marketplace theme authors
- `tailwind.config.js` → Tailwind v4 uses `@theme` in CSS directly, config stays minimal
- `frontend/src/components/PlatformStatusIndicators.tsx` — currently uses hardcoded hex in SVG fills (correct for brand icons), but `getPlatformColor()` in `ThemePreview.tsx` uses `text-purple-400` etc. — needs migration to static map using design tokens

### Migration Required
- `bg-gradient-to-*` → `bg-linear-to-*` in 4 files:
  - `frontend/src/app/overlay/[id]/credits/page.tsx`
  - `frontend/src/app/page.tsx`
  - `frontend/src/components/legal/LegalLayout.tsx`
  - `frontend/src/components/theme-marketplace/CreditRollThemePreview.tsx`
- `getPlatformColor()` in `ThemePreview.tsx` — replace with static map using `--color-twitch` etc.

</code_context>

<specifics>
## Specific Ideas

- Reference mockups saved in `.planning/phases/23-design-token-system-foundation/`:
  - `homepage-reference.html` — full dashboard mockup with magnetic glow cards + live chat
  - `logo-reference.html` — final logo animation (chat bubble + infinity snake)
  - `magnetic-cards-reference.html` — isolated magnetic glow card system

- "Platforms Are the Brand" — the design identity. Platform colors appear on: stat card glows, platform badges, chat message dots, overlay card top borders, logo infinity snake, nav active underline gradient endpoints.

- Noise layer technique: inline SVG `feTurbulence fractalNoise` data URI, `mix-blend-mode: overlay`, `opacity: 0.055` — kills gradient banding on dark backgrounds without dithering complexity.

- Magnetic glow JS pattern (for future reference when implementing in React):
  ```js
  // Global cursor → card-local space (shared light source)
  glow.style.left = `${e.clientX - rect.left}px`;
  glow.style.top  = `${e.clientY - rect.top}px`;
  // Intensity = quadratic falloff from nearest edge
  const dist = closestEdgeDist(rect, e.clientX, e.clientY);
  const intensity = Math.pow(Math.max(0, 1 - dist / 520), 2);
  ```

</specifics>

<deferred>
## Deferred Ideas

- Light theme — explicitly v2, not in v1.3 scope
- Flashy mode toggle UI — the preference itself is decided (it exists), the settings page UI is Phase 25
- Framer Motion / advanced animations — deferred (tw-animate-css covers micro-interactions sufficiently)
- Logo as actual SVG asset / favicon — Phase 25 when pages are built

</deferred>

---

*Phase: 23-design-token-system-foundation*
*Context gathered: 2026-03-10*
