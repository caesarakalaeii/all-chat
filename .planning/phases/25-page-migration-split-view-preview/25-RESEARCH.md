# Phase 25: Page Migration & Split-view Preview - Research

**Researched:** 2026-03-11
**Domain:** React/Next.js page redesign, CSS split-view layout, WCAG 2.1 AA accessibility
**Confidence:** HIGH

## Summary

Phase 25 migrates all six application pages to the Phase 23/24 design system and adds a split-view live preview feature to the overlay editor. The infrastructure is fully in place: design tokens in `globals.css`, component library (`Button`, `Card`, `Input`, `Badge`, `Dialog`, `Skeleton`, `Toast`) all built and Storybook-tested. This phase is purely about applying those components to the existing pages — replacing raw Tailwind utility soup with the proper components and introducing the iframe-based split view.

The existing pages contain significant technical debt: `window.confirm()` and `alert()` calls throughout dashboard and settings/editor, inline `notification` state managed manually (should be replaced by `toastManager`), raw gray-scale classes (`bg-gray-900`, `border-gray-700`) instead of design tokens, and no loading/empty state polish. The admin pages uniquely still use light-mode classes (`bg-white`, `text-gray-900`, `bg-gray-50`) and have a completely separate nav style — they require the most structural work.

The split-view feature is architecturally straightforward: embed `/overlays/[id]/preview` in an `<iframe>` beside the editor config panel with a draggable CSS `resize`-or-`pointermove` divider. The preview page already handles its own WebSocket connection, so no new backend logic is needed.

**Primary recommendation:** Migrate pages in dependency order — landing first (standalone), then shared nav pattern (dashboard, settings, admin), then overlay editor with split-view last (most complex). For each page: swap raw classes → tokens, replace `confirm()`/`alert()` → Dialog/Toast, replace spinner → Skeleton, wire empty states with Lucide icons + gradient CTA.

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Landing Page Hero**
- Background: Near-black `#07070a` with magnetic glow stat cards system (from `homepage-reference.html`) as the hero visual. The 4 platform stat cards are used as the primary hero section — cursor-tracked glow, idle Lissajous animation, noise layer.
- Login buttons: Platform-colored buttons styled with design tokens and PLATFORM_COLORS map (Twitch purple, YouTube red, Kick green). YouTube button must use approved branding (white play icon on red background).
- No live demo section — landing page focused on conversion.
- Feature grid below CTA uses the same magnetic glow card system as the hero stat cards.
- Logo + animated infinity snake (established in Phase 23) is the primary brand mark.

**Split-view Layout (FEAT-01–04)**
- Layout: Config panel LEFT (~40%), preview panel RIGHT (~60%). Standard editor pattern.
- Preview implementation: Embed `/overlays/[id]/preview` page in an `<iframe>` inside the editor. Reuses all existing WebSocket logic, overlay CSS, and events.css. Marketplace themes work automatically with zero code duplication.
- Divider: Draggable — user can resize config/preview proportions. Need minimum widths for both panels (prevent collapsing entirely).
- Mobile (< 768px): Stack vertically — config panel on top, preview below.
- Responsive trigger: Below 768px → vertical stack. Above 768px → side-by-side split.

**Overlay Card Multi-source Color (Dashboard)**
- Single source: 3px top border using that platform's color from PLATFORM_COLORS map.
- Multiple sources: Segmented top border gradient — each platform color occupies an equal segment (~45px each), with a narrow ~10px blending zone between adjacent colors.
- No sources (empty overlay): Neutral `--color-border` token for the top border.
- Example CSS pattern for 2-source border: `background: linear-gradient(90deg, #9146FF 0%, #9146FF calc(50% - 5px), #FF4444 calc(50% + 5px), #FF4444 100%)`

**Empty States (PAGE-10)**
- Style: Large Lucide icon as illustration + short message + gradient CTA button. Subtle platform badge strip below the icon (using PlatformBadge components from Phase 24) as accent.
- Dashboard empty state: `MonitorPlay` or `LayoutGrid` Lucide icon. Message: "No overlays yet". CTA: "Create your first overlay" (gradient button).
- Tone: Utility-focused, not playful. Streamers are professionals using a tool.

**Loading States (PAGE-09)**
- Pattern: Skeleton cards everywhere — no spinners. Replace all spinner instances with Skeleton component cards that mirror the actual loaded layout.
- Dashboard loading: Render 3 skeleton overlay cards in the grid (same proportions as real cards) while fetching.
- Form loading: Disable form controls + skeleton on submit button while submitting.

**Error States**
- Transient errors (API failures, delete failures, OAuth errors): Toast component. Errors stay until manually dismissed per Phase 24 Toast spec (4s auto-dismiss for success/info only).
- Form validation errors: Inline below the field. Remove `alert()` usage entirely.
- Notification state pattern: Replace inline `notification` state with Toast calls throughout all pages.

**Admin Pages**
- Visual consistency: Same nav, same card/table patterns as dashboard and settings. Use Card component for content sections.
- Exact layout of admin tables, pagination treatment, action button placement: Claude's Discretion.

**Shared Nav (Authenticated Pages)**
- Frosted glass nav established in Phase 23 applies to all authenticated pages (dashboard, editor, settings, admin).
- Logo with infinity animation in nav (Phase 23 decision).
- Active page underline: purple → teal gradient (Phase 23 decision).

### Claude's Discretion
- Exact responsive breakpoint behavior for the split-view draggable divider
- Admin page table layout specifics (columns, actions per row)
- Settings page section organization (profile, account management, danger zone)
- New overlay creation form layout (modal vs full page — current is full page, can keep)
- Exact Lucide icons chosen for each empty state (MonitorPlay, LayoutGrid, etc.)
- Overlay editor source management card layout details (platform badge + channel name + remove button arrangement)

### Deferred Ideas (OUT OF SCOPE)
- None — discussion stayed within phase scope
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| PAGE-01 | Landing page redesigned (hero with gradient CTAs, platform login buttons, feature cards) | Magnetic glow stat cards from `homepage-reference.html` become hero; PLATFORM_COLORS map drives button colors; existing login logic preserved |
| PAGE-02 | Dashboard redesigned (overlay grid with hover states, create button, empty states) | Card component with `interactive` variant; multi-source top border gradient pattern; MonitorPlay Lucide empty state with gradient CTA |
| PAGE-03 | Overlay editor redesigned (source management cards with platform-color coding) | PlatformBadge + Card component for each source; PLATFORM_COLORS text/bg keys; Dialog replaces `window.confirm()` |
| PAGE-04 | Overlay preview maintained (existing CSS unchanged for marketplace compatibility) | `/overlays/[id]/preview/page.tsx` is locked — do not modify |
| PAGE-05 | Settings page redesigned (account management, profile display) | Existing settings structure is close to design system; replace raw `bg-gray-800/70` with `bg-surface` token; Dialog replaces `window.confirm()` for delete account |
| PAGE-06 | Admin pages redesigned (visual consistency) | Admin layout needs full dark theme conversion; replace `bg-white`/`bg-gray-50`/`text-gray-900` with design tokens; shared frosted nav pattern |
| PAGE-07 | Responsive layouts validated across breakpoints (375px, 768px, 1920px) | Split view breakpoint at 768px; all grids use responsive Tailwind prefixes |
| PAGE-08 | Accessibility compliance achieved (WCAG 2.1 AA, keyboard navigation, focus states) | Storybook a11y addon already in `error` mode; `focus-visible` required on all interactive elements; Dialog accessible (focus trap, Escape key via `@base-ui/react`) |
| PAGE-09 | Loading states implemented for all data-fetching scenarios | Skeleton component (`animate-pulse rounded-md bg-surface-2`) replaces all spinners |
| PAGE-10 | Empty states implemented with illustrations and clear CTAs | Lucide icon + text + Button variant="gradient" pattern |
| FEAT-01 | Split-view layout implemented (editor configuration + live preview side-by-side) | CSS grid/flex with draggable divider; min-width constraints on both panels |
| FEAT-02 | Preview updates in real-time as configuration changes | iframe src points to `/overlays/[id]/preview` which maintains its own WebSocket — real-time is automatic |
| FEAT-03 | Responsive split-view (stacks vertically on mobile, side-by-side on desktop) | `flex-col` below 768px, `flex-row` above; Tailwind `md:` prefix |
| FEAT-04 | Preview window maintains overlay CSS (marketplace theme compatibility) | iframe isolation preserves all overlay CSS; `events.css` already imported in preview page |
</phase_requirements>

---

## Standard Stack

### Core (all already installed, zero new dependencies needed)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `@base-ui/react` | ^1.2.0 | Dialog, Toast, Input, Button primitives | Phase 24 decision — all custom components wrap these |
| `lucide-react` | ^0.563.0 | Lucide icons for empty states, nav, actions | Phase 24 decision — icon system for the design system |
| `tw-animate-css` | ^1.4.0 | `animate-fade-in`, `animate-slide-up` utilities | Phase 23 decision — no Framer Motion |
| `class-variance-authority` | (bundled) | CVA for all component variants | Phase 24 decision |
| `tailwindcss` v4 | (bundled) | Utility classes with `@theme` tokens | Phase 23 decision |

### Phase-supplied components (use these, do not recreate)

| Component | Import Path | Key Variants/API |
|-----------|-------------|-----------------|
| `Button` | `@/components/ui/button` | `variant`: default, gradient, outline, destructive, ghost; `size`: default, sm, lg, icon |
| `Card` | `@/components/ui/card` | `interactive={true}` for hover scale + shadow |
| `Input` | `@/components/ui/input` | `size`: default, sm |
| `Badge`, `PlatformBadge` | `@/components/ui/badge` | `platform`: twitch, youtube, kick, tiktok, system |
| `Dialog.*` | `@/components/ui/dialog` | `Dialog.Root`, `.Trigger`, `.Content`, `.Title`, `.Description`, `.Close` |
| `ToastProvider`, `ToastList` | `@/components/ui/toast` | Wrap at layout level (already done in `layout.tsx`) |
| `Skeleton` | `@/components/ui/skeleton` | `className` prop for sizing |
| `PLATFORM_COLORS` | `@/lib/platform-colors` | `PLATFORM_COLORS[platform].text`, `.bg` |
| `toastManager` | `@/lib/toast` | `toastManager.add({ title, description, type })` |

**No new packages needed for this phase.**

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| CSS `pointer-events` draggable divider | `react-split-pane` library | Library adds bundle weight; CSS pointer tracking is sufficient and zero-dep |
| `toastManager.add()` calls | Keeping inline `notification` state | Toast is already wired in layout; inline state is technical debt |

---

## Architecture Patterns

### Recommended File Structure (no new files except stories)

All changes are in-place rewrites of existing page files. New stories are the only net-new files.

```
frontend/src/app/
├── page.tsx                          # REWRITE — landing page
├── dashboard/page.tsx                # REWRITE — dashboard
├── overlays/
│   ├── [id]/page.tsx                 # REWRITE — overlay editor + split-view
│   ├── [id]/preview/page.tsx         # FROZEN — do not touch
│   └── new/page.tsx                  # REWRITE — new overlay form
├── settings/page.tsx                 # REWRITE — settings
└── admin/
    ├── layout.tsx                    # REWRITE — admin layout dark conversion
    └── page.tsx                      # REWRITE — admin dashboard
frontend/src/stories/
    ├── LandingPage.stories.tsx       # NEW — a11y validation
    ├── Dashboard.stories.tsx         # NEW
    └── OverlayEditor.stories.tsx     # NEW
```

### Pattern 1: Shared Frosted Glass Nav Component

The same nav appears on dashboard, editor, settings, and admin pages. Extract to a shared component to avoid duplication.

```typescript
// Source: homepage-reference.html nav pattern + Phase 23 tokens
// frontend/src/components/AppNav.tsx
'use client'
import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { useAuthStore } from '@/lib/stores/auth-store'
import { useRouter } from 'next/navigation'

export function AppNav() {
  const pathname = usePathname()
  const { user } = useAuthStore()
  const router = useRouter()

  // Active underline: purple → teal gradient
  const activeStyle = (href: string) =>
    pathname.startsWith(href)
      ? 'text-text relative after:absolute after:bottom-0 after:left-0 after:right-0 after:h-0.5 after:bg-linear-to-r after:from-twitch after:to-tiktok after:rounded-t'
      : 'text-text-sub hover:text-text transition-colors'

  return (
    <nav className="sticky top-0 z-50 flex h-[60px] items-center px-8 bg-nav-bg backdrop-blur-[20px] border-b border-border">
      {/* Logo with infinity ring animation */}
      <Link href="/dashboard" className="flex items-center gap-2.5 mr-10">
        <div className="logo-ring" aria-hidden="true" />
        <span className="text-base font-extrabold tracking-tight">all-chat</span>
      </Link>
      <div className="flex h-full gap-0.5">
        <Link href="/dashboard" className={`flex items-center px-3.5 text-sm ${activeStyle('/dashboard')}`}>Dashboard</Link>
        {user?.is_admin && (
          <Link href="/admin" className={`flex items-center px-3.5 text-sm ${activeStyle('/admin')}`}>Admin</Link>
        )}
        <Link href="/settings" className={`flex items-center px-3.5 text-sm ${activeStyle('/settings')}`}>Settings</Link>
      </div>
    </nav>
  )
}
```

### Pattern 2: Multi-source Top Border (Dashboard Overlay Cards)

```typescript
// Source: CONTEXT.md specifics — segmented gradient with 10px blend zones
function getTopBorderStyle(sources: ChatSource[]): React.CSSProperties {
  // Map from PLATFORM_COLORS raw hex values
  const PLATFORM_HEX: Record<string, string> = {
    twitch: '#A37BFF',
    youtube: '#FF4444',
    kick: '#53FC18',
    tiktok: '#69C9D0',
  }

  if (sources.length === 0) {
    return { background: 'var(--color-border)' }
  }
  if (sources.length === 1) {
    return { background: PLATFORM_HEX[sources[0].platform] ?? 'var(--color-border)' }
  }

  // N sources: divide evenly, 10px blend between each adjacent pair
  const colors = sources.map(s => PLATFORM_HEX[s.platform] ?? '#888')
  const segment = 100 / colors.length
  const blend = 5 // 5% each side of border = 10% total blend zone

  const stops: string[] = []
  colors.forEach((color, i) => {
    const start = i * segment
    const end = (i + 1) * segment
    if (i === 0) {
      stops.push(`${color} 0%`)
      stops.push(`${color} calc(${end}% - ${blend}%)`)
    } else if (i === colors.length - 1) {
      stops.push(`${color} calc(${start}% + ${blend}%)`)
      stops.push(`${color} 100%`)
    } else {
      stops.push(`${color} calc(${start}% + ${blend}%)`)
      stops.push(`${color} calc(${end}% - ${blend}%)`)
    }
  })

  return { background: `linear-gradient(90deg, ${stops.join(', ')})` }
}

// Usage: inline style on a 3px-tall div at top of card
<div style={{ height: '3px', ...getTopBorderStyle(overlay.sources) }} />
```

### Pattern 3: Split-view with Draggable Divider

```typescript
// Source: CSS pointer tracking pattern, no external library
'use client'
import { useRef, useState, useCallback } from 'react'

export function SplitView({ overlayId, children }: { overlayId: string; children: React.ReactNode }) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [leftPct, setLeftPct] = useState(40)
  const isDragging = useRef(false)

  const MIN_LEFT = 25  // percent — prevent full collapse of config
  const MAX_LEFT = 70  // percent — prevent full collapse of preview

  const onPointerDown = useCallback((e: React.PointerEvent) => {
    e.currentTarget.setPointerCapture(e.pointerId)
    isDragging.current = true
  }, [])

  const onPointerMove = useCallback((e: React.PointerEvent) => {
    if (!isDragging.current || !containerRef.current) return
    const rect = containerRef.current.getBoundingClientRect()
    const pct = ((e.clientX - rect.left) / rect.width) * 100
    setLeftPct(Math.min(MAX_LEFT, Math.max(MIN_LEFT, pct)))
  }, [])

  const onPointerUp = useCallback(() => {
    isDragging.current = false
  }, [])

  return (
    <div
      ref={containerRef}
      className="flex h-[calc(100vh-60px)] overflow-hidden"
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
    >
      {/* Config panel */}
      <div style={{ width: `${leftPct}%` }} className="overflow-y-auto flex-shrink-0">
        {children}
      </div>

      {/* Draggable divider */}
      <div
        className="w-1 flex-shrink-0 bg-border hover:bg-twitch/50 cursor-col-resize transition-colors select-none"
        onPointerDown={onPointerDown}
        role="separator"
        aria-label="Drag to resize panels"
        aria-orientation="vertical"
      />

      {/* Preview panel */}
      <div className="flex-1 overflow-hidden bg-bg">
        <iframe
          src={`/overlays/${overlayId}/preview`}
          className="w-full h-full border-0"
          title="Overlay live preview"
          sandbox="allow-scripts allow-same-origin"
        />
      </div>
    </div>
  )
}
```

**Mobile stacking:** Wrap the whole split-view in `flex-col md:flex-row`. Below `md` breakpoint (768px), divider is hidden and both panels take full width stacked vertically.

### Pattern 4: Dialog Confirmation (Replace window.confirm)

```typescript
// Source: frontend/src/components/ui/dialog.tsx — Dialog namespace
import { Dialog } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { useState } from 'react'

function DeleteConfirmDialog({
  overlayName,
  onConfirm,
  children,
}: {
  overlayName: string
  onConfirm: () => void
  children: React.ReactNode  // trigger element
}) {
  return (
    <Dialog.Root>
      <Dialog.Trigger render={children} />
      <Dialog.Content showCloseButton={false}>
        <Dialog.Title>Delete "{overlayName}"?</Dialog.Title>
        <Dialog.Description>This action cannot be undone.</Dialog.Description>
        <div className="flex gap-3 justify-end mt-6">
          <Dialog.Close render={<Button variant="outline">Cancel</Button>} />
          <Button variant="destructive" onClick={onConfirm}>Delete</Button>
        </div>
      </Dialog.Content>
    </Dialog.Root>
  )
}
```

### Pattern 5: Toast (Replace inline notification state)

```typescript
// Source: frontend/src/lib/toast.ts — toastManager singleton
import { toastManager } from '@/lib/toast'

// Success
toastManager.add({ title: 'Source added', type: 'success' })

// Error (no auto-dismiss per Phase 24 spec — only success/info auto-dismiss at 4s)
toastManager.add({ title: 'Failed to delete overlay', description: 'Please try again.', type: 'error' })
```

**Critical:** Remove all `notification` state and `setTimeout(() => setNotification(null), 5000)` patterns. The ToastProvider in `layout.tsx` already handles this globally.

### Pattern 6: Skeleton Loading States

```typescript
// Source: frontend/src/components/ui/skeleton.tsx
import { Skeleton } from '@/components/ui/skeleton'

// Dashboard loading — 3 skeleton overlay cards
function OverlayGridSkeleton() {
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
      {Array.from({ length: 3 }).map((_, i) => (
        <div key={i} className="rounded-xl border border-border bg-surface p-6 space-y-3">
          <Skeleton className="h-4 w-2/3" />
          <Skeleton className="h-3 w-full" />
          <Skeleton className="h-3 w-1/2" />
          <div className="flex gap-2 mt-4">
            <Skeleton className="h-5 w-16 rounded-full" />
            <Skeleton className="h-5 w-16 rounded-full" />
          </div>
        </div>
      ))}
    </div>
  )
}
```

### Pattern 7: Empty State

```typescript
// Source: CONTEXT.md empty state spec
import { MonitorPlay } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { PlatformBadge } from '@/components/ui/badge'

function DashboardEmptyState({ onCreateClick }: { onCreateClick: () => void }) {
  return (
    <div className="flex flex-col items-center py-24 text-center gap-4">
      <MonitorPlay className="size-16 text-text-dim" strokeWidth={1} />
      <h2 className="text-xl font-semibold text-text">No overlays yet</h2>
      <p className="text-text-sub text-sm max-w-sm">
        Create your first overlay to start aggregating chat across platforms.
      </p>
      {/* Subtle platform accent strip */}
      <div className="flex gap-1.5 mt-2">
        {(['twitch', 'youtube', 'kick', 'tiktok'] as const).map(p => (
          <PlatformBadge key={p} platform={p} size="sm" />
        ))}
      </div>
      <Button variant="gradient" size="lg" onClick={onCreateClick} className="mt-4">
        Create your first overlay
      </Button>
    </div>
  )
}
```

### Pattern 8: Magnetic Glow Cards (Landing Page Hero)

The glow is driven by `pointermove` on the container. Each card has a `.mag-glow` blob that tracks cursor proximity. This is a direct React port of the vanilla JS in `homepage-reference.html`.

```typescript
// Critical: use useCallback and requestAnimationFrame to avoid re-render storms
// The glow blob is positioned via direct DOM manipulation (refs), NOT state
// This avoids React re-render on every mousemove

import { useRef, useCallback, useEffect } from 'react'

function useMagneticGlow() {
  const cardRefs = useRef<HTMLDivElement[]>([])
  const glowRefs = useRef<HTMLDivElement[]>([])

  const handlePointerMove = useCallback((e: PointerEvent) => {
    const MAX_DIST = 520
    cardRefs.current.forEach((card, i) => {
      if (!card || !glowRefs.current[i]) return
      const glow = glowRefs.current[i]
      const rect = card.getBoundingClientRect()
      const dx = Math.max(rect.left - e.clientX, 0, e.clientX - rect.right)
      const dy = Math.max(rect.top - e.clientY, 0, e.clientY - rect.bottom)
      const dist = Math.hypot(dx, dy)
      const intensity = Math.pow(Math.max(0, 1 - dist / MAX_DIST), 2)

      if (intensity < 0.003) {
        glow.style.opacity = '0'
        return
      }
      glow.style.left = `${e.clientX - rect.left}px`
      glow.style.top  = `${e.clientY - rect.top}px`
      glow.style.opacity = String(Math.min(intensity * 1.35, 1))
    })
  }, [])

  useEffect(() => {
    window.addEventListener('pointermove', handlePointerMove)
    return () => window.removeEventListener('pointermove', handlePointerMove)
  }, [handlePointerMove])

  return { cardRefs, glowRefs }
}
```

### Anti-Patterns to Avoid

- **Dynamic class construction:** Never `'text-' + platform` — always use `PLATFORM_COLORS[platform].text`
- **Gray-scale classes on app pages:** Never `bg-gray-900`, `border-gray-700`, `text-gray-400` — use `bg-bg`, `border-border`, `text-text-sub`
- **Inline notification state:** Never add `notification` state with `setTimeout` — always `toastManager.add()`
- **`window.confirm()` / `alert()`:** Always `Dialog` for confirmations, `Toast` for informational errors
- **Touching preview page:** `/overlays/[id]/preview/page.tsx` is frozen — do not modify any code in it
- **Modifying events.css:** Frozen stability contract — zero changes
- **Spinners anywhere:** Replace every `animate-spin` loading indicator with `Skeleton` components
- **State updates in pointermove handlers:** Use direct DOM refs for glow animations, not `setState`

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Confirmation dialogs | Custom modal component | `Dialog.*` from `@/components/ui/dialog` | Already built, accessible (focus trap, Escape key, WCAG AA) |
| Toast/notification system | `notification` state + setTimeout | `toastManager.add()` from `@/lib/toast` | Already wired in layout, handles auto-dismiss, ARIA live regions |
| Loading placeholders | `animate-spin` spinners | `Skeleton` from `@/components/ui/skeleton` | Consistent with design system, mirrors actual layout |
| Platform color resolution | `switch(platform)` returning hardcoded colors | `PLATFORM_COLORS` map from `@/lib/platform-colors` | Tailwind JIT-safe, single source of truth |
| Split-view resizable panels | `react-split-pane` or `react-resizable` | CSS `pointer` tracking with direct DOM refs | Zero new dependencies; fits current tw-animate-css animation approach |
| Focus management in dialogs | Manual `tabIndex`, `onKeyDown` | `@base-ui/react/dialog` (already wrapped) | Handles focus trap, Escape, ARIA automatically |

**Key insight:** Every UI primitive needed for this phase was built in Phase 24 specifically to avoid hand-rolling. This phase's job is purely to *use* those primitives on real pages.

---

## Common Pitfalls

### Pitfall 1: Forgetting to Import ToastProvider is Already in layout.tsx
**What goes wrong:** Developer adds `ToastProvider` again to a page, causing duplicate providers and broken toast state.
**Why it happens:** `layout.tsx` already wraps everything — it's easy to miss.
**How to avoid:** Never add `ToastProvider` to individual pages. Call `toastManager.add()` anywhere — it works globally.
**Warning signs:** Toasts not showing, or double toasts.

### Pitfall 2: Mixing Old Gray Classes with New Tokens
**What goes wrong:** Page looks inconsistent — some elements use `bg-gray-800` (old), others use `bg-surface` (new token). The colors are visually similar but not identical.
**Why it happens:** Find/replace miss during migration, or copy-pasting from old code.
**How to avoid:** After each page migration, run a search for `gray-[0-9]` and verify all occurrences are intentional (should be zero in migrated pages).
**Warning signs:** Subtle color mismatch visible especially at edge cases.

### Pitfall 3: Split-view iframe Requires Same-Origin
**What goes wrong:** iframe throws `X-Frame-Options` or `Content-Security-Policy` error.
**Why it happens:** `/overlays/[id]/preview` is on the same Next.js origin — this should work. But CSP headers misconfigured in production could block it.
**How to avoid:** Use `sandbox="allow-scripts allow-same-origin"` on the iframe. Test with `make frontend-dev` which runs both on the same origin.
**Warning signs:** Blank iframe, browser console CSP violation.

### Pitfall 4: Magnetic Glow Causing React Re-render Storms
**What goes wrong:** `pointermove` fires ~60fps. If glow position is stored in React state, it triggers constant re-renders degrading performance below the 16ms budget.
**Why it happens:** Natural instinct to use `useState` for cursor position.
**How to avoid:** Use `useRef` to access DOM elements directly. Mutate `glow.style.*` directly in the event handler — no `setState` calls.
**Warning signs:** React DevTools Profiler shows thousands of component renders per second during cursor movement.

### Pitfall 5: Admin Pages Light-Mode Background Leaking
**What goes wrong:** Admin pages still show `bg-white` or `bg-gray-50` because the `admin/layout.tsx` has its own nav/layout that overrides the dark `bg-bg` body default.
**Why it happens:** `admin/layout.tsx` uses `min-h-screen bg-gray-50` and `bg-white shadow-sm` nav — completely separate from the main app theme.
**How to avoid:** Rewrite `admin/layout.tsx` to use `bg-bg`, `bg-surface`, `border-border` tokens like the main app. The `AdminLayoutContent` function needs a full dark conversion.
**Warning signs:** Admin nav appears white/light when all other pages are dark.

### Pitfall 6: Dialog.Trigger Render Prop vs Children
**What goes wrong:** `<Dialog.Trigger>` wraps a child but TypeScript complains or click doesn't work.
**Why it happens:** `@base-ui/react` Dialog.Trigger uses a `render` prop pattern, not `children`.
**How to avoid:** Use `<Dialog.Trigger render={<Button>Delete</Button>}` pattern as shown in Pattern 4. Verify with existing `Dialog.stories.tsx` as reference.

### Pitfall 7: WCAG AA Failures on Platform Color Text
**What goes wrong:** Storybook a11y in `error` mode fails because platform colors don't meet 4.5:1 contrast against dark background.
**Why it happens:** Original Twitch `#9146FF` fails — was already lightened to `#A37BFF` in Phase 24-05. But source cards in the editor might use the old color.
**How to avoid:** Always use `PLATFORM_COLORS[platform].text` (e.g., `text-twitch`) which resolves to `var(--color-twitch)` = `#A37BFF`. Never hardcode `#9146FF`.
**Warning signs:** Storybook a11y addon shows contrast failure for Twitch purple text elements.

---

## Code Examples

### Using toastManager to replace notification state

```typescript
// Source: frontend/src/lib/toast.ts
import { toastManager } from '@/lib/toast'

// BEFORE (pattern to eliminate from all pages):
// const [notification, setNotification] = useState(null)
// setNotification({ type: 'success', message: 'Done' })
// setTimeout(() => setNotification(null), 5000)

// AFTER:
toastManager.add({ title: 'Source added', type: 'success' })
toastManager.add({ title: 'Failed', description: 'Try again.', type: 'error' })
```

### Skeleton card matching overlay card proportions

```typescript
// Source: frontend/src/components/ui/skeleton.tsx
<div className="rounded-xl border border-border bg-surface overflow-hidden">
  <div className="h-[3px] w-full bg-surface-2" /> {/* top border placeholder */}
  <div className="p-6 space-y-3">
    <Skeleton className="h-4 w-1/2" />   {/* overlay name */}
    <Skeleton className="h-3 w-3/4" />   {/* description line */}
    <div className="flex gap-1.5 mt-2">
      <Skeleton className="h-4 w-12 rounded-full" /> {/* badge */}
      <Skeleton className="h-4 w-12 rounded-full" /> {/* badge */}
    </div>
    <Skeleton className="h-3 w-1/3 mt-3" /> {/* timestamp */}
  </div>
</div>
```

### Frosted nav background using existing CSS tokens

```typescript
// Source: globals.css --color-nav-bg token
// --color-nav-bg: oklch(from var(--color-neutral-950) l c h / 0.80)
// Tailwind: bg-nav-bg backdrop-blur-[20px]
<nav className="sticky top-0 z-50 h-[60px] flex items-center px-8 bg-nav-bg backdrop-blur-[20px] border-b border-border" />
```

### Overlay card interactive hover with platform glow

```typescript
// Source: homepage-reference.html ov-card pattern + Card component
import { Card } from '@/components/ui/card'

// Card interactive variant adds hover:scale-[1.02] hover:shadow-lg
// Additional platform glow on hover via inline style
<Card
  interactive
  className="overflow-hidden cursor-pointer group"
  onClick={() => router.push(`/overlays/${overlay.id}`)}
  style={{
    '--hover-glow': `var(--shadow-glow-${sources[0]?.platform ?? 'twitch'})`,
  } as React.CSSProperties}
>
  <div style={{ height: '3px', ...getTopBorderStyle(sources) }} />
  <div className="p-6">{/* card content */}</div>
</Card>
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `window.confirm()` / `alert()` | `Dialog` + `Toast` components | Phase 24 built these | Replace all occurrences in editor, dashboard, settings |
| `animate-spin` spinner | `Skeleton` loading cards | Phase 24 built Skeleton | Replace all loading spinners |
| Inline `notification` state | `toastManager.add()` | Phase 24 built Toast | Eliminate all `notification` useState patterns |
| `bg-gray-*` / `text-gray-*` | `bg-bg`, `bg-surface`, `text-text`, `text-text-sub` | Phase 23 tokens | Replace throughout all pages |
| Hard-coded `#9146FF` Twitch color | `var(--color-twitch)` = `#A37BFF` | Phase 24-05 a11y fix | Never hardcode platform hex directly |
| Separate nav in each page | Shared `AppNav` component | Phase 25 (this phase) | Extract frosted glass nav to avoid 5x duplication |
| Navigate-to-preview button | Inline iframe split-view | Phase 25 (this phase) | Config + live preview side-by-side in the editor |

**Deprecated/outdated:**
- `bg-twitch` class in old landing/dashboard pages: was defined as a raw color utility in old config — now resolves to `var(--color-twitch)` via `@theme` tokens. Behavior is preserved but the tokens are the canonical reference.
- `BetaWarning` component: existing component uses `window.confirm`-like UI. Should be migrated to use `Dialog` for visual consistency, but its logic should be preserved.

---

## Open Questions

1. **Admin sub-pages (users, overlays, sources, viewers) scope**
   - What we know: `admin/layout.tsx` and `admin/page.tsx` are in scope. Sub-pages (`admin/users/page.tsx`, `admin/overlays/page.tsx`, `admin/sources/page.tsx`, `admin/viewers/page.tsx`) are listed in requirements as PAGE-06.
   - What's unclear: The planner should determine whether all 4 sub-pages get full migration or only the layout + dashboard page (where most visual regression is concentrated).
   - Recommendation: Migrate `admin/layout.tsx` (dark nav) + `admin/page.tsx` (stats) as required. Include the 3 sub-pages in scope since visual consistency is the requirement.

2. **Infinity snake logo animation implementation for nav**
   - What we know: Phase 23 established the logo-ring as a conic-gradient spinning ring (in `homepage-reference.html`, CSS: `animation: ring-spin 12s linear infinite`). The nav uses it as the brand mark.
   - What's unclear: The animation CSS is in `homepage-reference.html` but not yet in `globals.css` or a React component.
   - Recommendation: Create a `LogoRing` component with the CSS inline (or add `@keyframes ring-spin` to the design-system layer in `globals.css`) during the nav extraction step.

3. **BetaWarning component migration**
   - What we know: `BetaWarning` is used in landing page and overlay editor. It currently renders its own modal-like overlay with raw CSS.
   - What's unclear: Whether to rewrite it using `Dialog` component or leave it as-is (it's not in the pages-to-migrate list but appears in both landing and editor).
   - Recommendation: Migrate `BetaWarning` to use `Dialog` as part of the landing page and overlay editor migration waves — it's rendered on those pages and will be visually inconsistent if left as-is.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Vitest 4.0.18 + `@storybook/addon-vitest` 10.2.17 + `@storybook/addon-a11y` 10.2.17 |
| Config file | `frontend/.storybook/main.ts` + `frontend/.storybook/vitest.setup.ts` |
| Quick run command | `cd frontend && npm run storybook` (interactive) |
| a11y validation command | `cd frontend && npx storybook --ci` or via Storybook UI a11y panel |
| Full suite command | `cd frontend && npm test` (Vitest unit tests) |

**a11y mode:** Storybook `preview.ts` has `a11y: { test: 'error' }` — all stories fail CI on a11y violations. This is the primary WCAG 2.1 AA gate for PAGE-08.

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PAGE-01 | Landing page renders with hero, login buttons, feature cards | Storybook visual | `npm run storybook` | ❌ Wave 0 |
| PAGE-02 | Dashboard overlay grid, empty state, skeleton loading | Storybook visual + a11y | `npm run storybook` | ❌ Wave 0 |
| PAGE-03 | Overlay editor source cards render with platform colors | Storybook visual + a11y | `npm run storybook` | ❌ Wave 0 |
| PAGE-04 | Preview page unchanged | No test needed (frozen file) | N/A | N/A |
| PAGE-05 | Settings sections render with tokens | Storybook visual | `npm run storybook` | ❌ Wave 0 |
| PAGE-06 | Admin pages use dark theme | Storybook visual | `npm run storybook` | ❌ Wave 0 |
| PAGE-07 | Responsive at 375/768/1920px | Manual browser resize | Manual | N/A |
| PAGE-08 | WCAG 2.1 AA — no a11y violations in error mode | Storybook a11y (error mode) | `npm run storybook` → a11y panel | ❌ Wave 0 |
| PAGE-09 | Skeleton cards display during loading | Storybook story with loading prop | `npm run storybook` | ❌ Wave 0 |
| PAGE-10 | Empty state with icon + CTA renders | Storybook visual | `npm run storybook` | ❌ Wave 0 |
| FEAT-01 | Split-view layout renders left/right panels | Storybook visual (mocked iframe) | `npm run storybook` | ❌ Wave 0 |
| FEAT-02 | Preview iframe src set to correct overlay URL | Unit test | `npm test` | ❌ Wave 0 |
| FEAT-03 | Below 768px: vertical stack | Manual browser resize | Manual | N/A |
| FEAT-04 | iframe preserves overlay CSS | Manual visual check | Manual | N/A |

### Sampling Rate

- **Per task commit:** Visual inspection in browser (`make frontend-dev`) — check the specific page changed
- **Per wave merge:** Run Storybook and verify a11y panel shows no errors in 'error' mode
- **Phase gate:** Full Storybook suite green + manual breakpoint check at 375px/768px/1920px before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `frontend/src/stories/LandingPage.stories.tsx` — covers PAGE-01, PAGE-08 (a11y for login buttons)
- [ ] `frontend/src/stories/Dashboard.stories.tsx` — covers PAGE-02, PAGE-08, PAGE-09, PAGE-10
- [ ] `frontend/src/stories/OverlayEditor.stories.tsx` — covers PAGE-03, FEAT-01
- [ ] `frontend/src/stories/Settings.stories.tsx` — covers PAGE-05, PAGE-08
- [ ] `frontend/src/stories/AdminLayout.stories.tsx` — covers PAGE-06
- [ ] `frontend/src/components/AppNav.tsx` (shared nav component — prerequisite for multiple stories)

---

## Sources

### Primary (HIGH confidence)
- `/home/caesar/git/all-chat/frontend/src/components/ui/` — All Phase 24 components read directly; API confirmed
- `/home/caesar/git/all-chat/frontend/src/app/globals.css` — Design tokens confirmed; `--color-nav-bg` token exists
- `/home/caesar/git/all-chat/frontend/src/lib/platform-colors.ts` — PLATFORM_COLORS map confirmed with exact keys
- `/home/caesar/git/all-chat/frontend/src/lib/toast.ts` — `toastManager` singleton confirmed
- `/home/caesar/git/all-chat/frontend/.storybook/preview.ts` — a11y `test: 'error'` mode confirmed
- `/home/caesar/git/all-chat/.planning/phases/23-design-token-system-foundation/homepage-reference.html` — Magnetic glow JS read; full implementation available
- All existing page files read directly — current technical debt catalogued

### Secondary (MEDIUM confidence)
- Phase 24 accumulated decisions in `STATE.md` — CVA patterns, `@base-ui/react` API patterns, Toast border-color tokens, a11y fixes

### Tertiary (LOW confidence)
- None

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all components verified by reading source files directly
- Architecture patterns: HIGH — code examples derived from existing working components + homepage-reference.html
- Pitfalls: HIGH — derived from reading actual existing page code (confirmed `window.confirm`, `notification` state, `gray-*` classes, light-mode admin pages)
- Split-view implementation: HIGH — iframe/pointer pattern well-understood, existing preview page confirmed at correct URL

**Research date:** 2026-03-11
**Valid until:** 2026-04-11 (stable; component APIs won't change before this phase completes)
