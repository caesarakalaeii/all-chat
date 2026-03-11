# Phase 25: Page Migration & Split-view Preview - Context

**Gathered:** 2026-03-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Migrate all application pages to the Phase 23/24 design system (design tokens, shadcn/ui components, typography, platform colors). Pages in scope: landing, dashboard, overlay editor, settings, admin pages, new overlay form. Overlay preview CSS is explicitly preserved unchanged (marketplace compatibility). This phase also adds the split-view live preview feature to the overlay editor (FEAT-01–04).

No new backend endpoints or data models. Frontend-only changes.

</domain>

<decisions>
## Implementation Decisions

### Landing Page Hero
- **Background:** Near-black `#07070a` with the magnetic glow stat cards system (from `homepage-reference.html`) as the hero visual. The 4 platform stat cards are used as the primary hero section — cursor-tracked glow, idle Lissajous animation, noise layer.
- **Login buttons:** Platform-colored buttons styled with design tokens and PLATFORM_COLORS map (Twitch purple, YouTube red, Kick green). Keep YouTube Design Guidelines in mind — YouTube button must use the approved YouTube branding (white play icon on red background per prior Phase 23 decisions).
- **No live demo section** — landing page focused on conversion. The overlay preview is available post-login.
- **Feature grid below CTA** uses the same magnetic glow card system as the hero stat cards.
- Logo + animated infinity snake (established in Phase 23) is the primary brand mark.

### Split-view Layout (FEAT-01–04)
- **Layout:** Config panel LEFT (~40%), preview panel RIGHT (~60%). Standard editor pattern.
- **Preview implementation:** Embed `/overlays/[id]/preview` page in an `<iframe>` inside the editor. Reuses all existing WebSocket logic, overlay CSS, and events.css. Marketplace themes work automatically with zero code duplication.
- **Divider:** Draggable — user can resize config/preview proportions. Need minimum widths for both panels (prevent collapsing entirely).
- **Mobile (< 768px):** Stack vertically — config panel on top, preview below.
- **Responsive trigger:** Below 768px → vertical stack. Above 768px → side-by-side split.

### Overlay Card Multi-source Color (Dashboard)
- **Single source:** 3px top border using that platform's color from PLATFORM_COLORS map (established pattern from Phase 23).
- **Multiple sources:** Segmented top border gradient — each platform color occupies an equal segment (~45px each), with a narrow ~10px blending zone between adjacent colors. The blending zone is intentionally small to minimize muddy color mixing. Order follows the order sources were added.
- **No sources (empty overlay):** Neutral `--color-border` token for the top border. Visual signal that setup is needed.
- Example CSS pattern for 2-source border: `background: linear-gradient(90deg, #9146FF 0%, #9146FF calc(50% - 5px), #FF4444 calc(50% + 5px), #FF4444 100%)` — adjust proportions per source count.

### Empty States (PAGE-10)
- **Style:** Large Lucide icon as illustration + short message + gradient CTA button. Subtle platform badge strip below the icon (using PlatformBadge components from Phase 24) as accent.
- **Dashboard empty state:** `MonitorPlay` or `LayoutGrid` Lucide icon. Message: "No overlays yet". CTA: "Create your first overlay" (gradient button).
- **Tone:** Utility-focused, not playful. Streamers are professionals using a tool.

### Loading States (PAGE-09)
- **Pattern:** Skeleton cards everywhere — no spinners. Replace all spinner instances with Skeleton component cards that mirror the actual loaded layout.
- **Dashboard loading:** Render 3 skeleton overlay cards in the grid (same proportions as real cards) while fetching.
- **Form loading:** Disable form controls + skeleton on submit button while submitting.

### Error States
- **Transient errors** (API failures, delete failures, OAuth errors): Toast component. Errors stay until manually dismissed per Phase 24 Toast spec (4s auto-dismiss for success/info only).
- **Form validation errors:** Inline below the field. Remove `alert()` usage entirely.
- **Notification state pattern:** Replace inline `notification` state with Toast calls throughout all pages.

### Admin Pages
- **Visual consistency:** Same nav, same card/table patterns as dashboard and settings. Use Card component for content sections.
- **Claude's Discretion:** Exact layout of admin tables, pagination treatment, action button placement.

### Shared Nav (Authenticated Pages)
- **Frosted glass nav** established in Phase 23 applies to all authenticated pages (dashboard, editor, settings, admin).
- Logo with infinity animation in nav (Phase 23 decision).
- Active page underline: purple → teal gradient (Phase 23 decision).

### Claude's Discretion
- Exact responsive breakpoint behavior for the split-view draggable divider
- Admin page table layout specifics (columns, actions per row)
- Settings page section organization (profile, account management, danger zone)
- New overlay creation form layout (modal vs full page — current is full page, can keep)
- Exact Lucide icons chosen for each empty state (MonitorPlay, LayoutGrid, etc.)
- Overlay editor source management card layout details (platform badge + channel name + remove button arrangement)

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `frontend/src/components/ui/button.tsx` — Button with gradient variant, all sizes. Use for all CTAs.
- `frontend/src/components/ui/card.tsx` — Card component. Use for overlay cards, settings sections, admin content.
- `frontend/src/components/ui/badge.tsx` — PlatformBadge with glow dot. Use for source indicators on overlay cards and editor.
- `frontend/src/components/ui/input.tsx` — Input component. Use for all form fields.
- `frontend/src/components/ui/dialog.tsx` — Dialog with frosted glass backdrop. Use for confirmation dialogs (replace `window.confirm()`).
- `frontend/src/components/ui/toast.tsx` — Toast. Use for all transient notifications (replace inline `notification` state).
- `frontend/src/components/ui/skeleton.tsx` — Skeleton component. Use for all loading states.
- `frontend/src/components/ProtectedRoute.tsx` — Already wraps authenticated pages. Keep as-is.
- `.planning/phases/23-design-token-system-foundation/homepage-reference.html` — Full reference mockup with magnetic glow cards.

### Established Patterns
- PLATFORM_COLORS static map (from Phase 23) — import and use for all platform-colored elements, never dynamic class concatenation.
- CVA + `cn()` for all component variants.
- `tw-animate-css` for transitions and animations (no Framer Motion).
- `@base-ui/react` primitives wrapped by custom components.
- Dark-only app — no light mode considerations.

### Integration Points
- All authenticated pages use `useAuthStore` (Zustand) and `ProtectedRoute` wrapper.
- Overlay data comes from `useOverlayStore` and `overlaysApi`.
- Split-view iframe: `/overlays/[id]/preview` is the existing preview URL — use as iframe src.
- Admin pages use their own data-fetching (see existing admin page implementations).
- `window.confirm()` → Dialog component (requires adding Dialog to affected pages).

### Pages to Migrate
1. `frontend/src/app/page.tsx` — Landing (major redesign: magnetic glow cards hero)
2. `frontend/src/app/dashboard/page.tsx` — Dashboard (overlay grid + new Card + Skeleton)
3. `frontend/src/app/overlays/[id]/page.tsx` — Overlay editor (source cards + split-view iframe)
4. `frontend/src/app/settings/page.tsx` — Settings (Card sections + danger zone)
5. `frontend/src/app/admin/page.tsx` + sub-pages — Admin (tables + Card layout)
6. `frontend/src/app/overlays/new/page.tsx` — New overlay form (Input + Button components)

### Preserved (Do Not Touch)
- `frontend/src/app/overlays/[id]/preview/page.tsx` — Overlay preview page (marketplace CSS compatibility)
- `frontend/src/styles/events.css` — Public API, frozen (Phase 23 stability contract)

</code_context>

<specifics>
## Specific Ideas

- Multi-source top border pattern (prevent muddy colors): `linear-gradient(90deg, [color1] 0%, [color1] calc(N% - 5px), [color2] calc(N% + 5px), [color2] 100%)` — only 10px total blending zone per transition. For 3+ platforms, divide evenly with 10px blend zones between each segment.
- Magnetic glow stat cards for hero: reference implementation in `.planning/phases/23-design-token-system-foundation/homepage-reference.html`. The 4 stat cards from that mockup become the landing page hero section.
- Replace all `window.confirm()` usage (found in dashboard delete and settings delete account) with Dialog component.
- Remove all `alert()` usage, replace with Toast.

</specifics>

<deferred>
## Deferred Ideas

- None — discussion stayed within phase scope

</deferred>

---

*Phase: 25-page-migration-split-view-preview*
*Context gathered: 2026-03-11*
