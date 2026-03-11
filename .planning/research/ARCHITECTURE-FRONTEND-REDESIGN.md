# Architecture Research: Design System Integration

**Domain:** Frontend Design System Integration with Next.js 16 + Tailwind v4 + shadcn/ui
**Researched:** 2026-03-09
**Confidence:** HIGH

## Standard Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    Next.js App Router                        │
│                  (Server Components Default)                 │
├─────────────────────────────────────────────────────────────┤
│  ┌───────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │  Layout   │  │  Pages   │  │  Server  │  │  Client  │   │
│  │  (RSC)    │  │  (RSC)   │  │Components│  │Components│   │
│  └─────┬─────┘  └─────┬────┘  └─────┬────┘  └─────┬────┘   │
│        │              │              │             │         │
├────────┴──────────────┴──────────────┴─────────────┴────────┤
│                  Component Library Layer                     │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ shadcn/ui Components (Copy-Paste, not npm package)  │    │
│  │ src/components/ui/* (Button, Card, Input, etc.)     │    │
│  └─────────────────────────────────────────────────────┘    │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ Custom Domain Components (Overlays, Chat, Admin)    │    │
│  │ src/components/* (MonacoCSSEditor, ThemeCard, etc.) │    │
│  └─────────────────────────────────────────────────────┘    │
├─────────────────────────────────────────────────────────────┤
│                   Design Token System                        │
│  ┌──────────────────────────────────────────────────┐       │
│  │ Tailwind v4 @theme Directives (globals.css)      │       │
│  │ - CSS Variables (--color-*, --radius-*, etc.)    │       │
│  │ - Platform Colors (Twitch, YouTube, Kick, etc.)  │       │
│  │ - shadcn/ui Semantic Tokens (primary, muted, etc)│       │
│  └──────────────────────────────────────────────────┘       │
├─────────────────────────────────────────────────────────────┤
│                      State Management                        │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐            │
│  │   Zustand  │  │    React   │  │   API      │            │
│  │   Stores   │  │   Context  │  │  Fetching  │            │
│  └────────────┘  └────────────┘  └────────────┘            │
└─────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Typical Implementation |
|-----------|----------------|------------------------|
| **Next.js App Router** | Routing, SSR/SSG, Server Components | File-based routing in `src/app/` |
| **Server Components (RSC)** | Data fetching, static content, SEO | Default for pages/layouts, no client JS |
| **Client Components** | Interactivity, browser APIs, state | Marked with `"use client"` directive |
| **shadcn/ui Components** | Primitive UI components (Button, Card, Input) | Copy-paste from shadcn CLI, customized |
| **Custom Domain Components** | Business logic components (overlays, admin) | Built on shadcn primitives + domain logic |
| **Design Token System** | Theme configuration, CSS variables | Tailwind v4 `@theme` in `globals.css` |
| **Zustand Stores** | Client-side global state | User session, UI preferences |
| **API Layer** | Backend communication | Fetch wrappers in `src/lib/api/` |

## Recommended Project Structure

```
frontend/src/
├── app/                        # Next.js App Router (file-based routing)
│   ├── layout.tsx              # Root layout (Inter font, metadata, global wrappers)
│   ├── page.tsx                # Landing page (Server Component)
│   ├── globals.css             # Tailwind v4 @theme directives, design tokens
│   ├── dashboard/              # Dashboard pages (protected routes)
│   │   └── page.tsx
│   ├── overlays/[id]/          # Dynamic overlay routes
│   │   ├── page.tsx            # Editor (Client Component - Monaco)
│   │   ├── preview/page.tsx    # Live preview
│   │   └── events/page.tsx     # Event configuration
│   └── admin/                  # Admin pages (protected routes)
│       ├── users/page.tsx
│       ├── overlays/page.tsx
│       └── sources/page.tsx
├── components/                 # React components
│   ├── ui/                     # shadcn/ui primitives (copy-paste from CLI)
│   │   ├── button.tsx          # Base Button component (Base UI + CVA)
│   │   ├── card.tsx            # Card container
│   │   ├── input.tsx           # Form input
│   │   ├── dialog.tsx          # Modal/Dialog
│   │   ├── badge.tsx           # Status badges
│   │   ├── select.tsx          # Dropdown select
│   │   └── toast.tsx           # Notifications
│   ├── theme-marketplace/      # Theme marketplace UI (domain-specific)
│   │   ├── ThemeCard.tsx       # Theme preview card
│   │   ├── ThemeFilters.tsx    # Filtering UI
│   │   └── ThemePreview.tsx    # Live preview
│   ├── admin/                  # Admin-specific components
│   │   ├── BanModal.tsx
│   │   └── ToastProvider.tsx
│   ├── legal/                  # Legal page components
│   │   └── LegalLayout.tsx
│   ├── MonacoCSSEditor.tsx     # Monaco editor wrapper (Client Component)
│   ├── PlatformStatusIndicators.tsx  # Platform connection status
│   ├── ProtectedRoute.tsx      # Auth guard component
│   ├── CookieBanner.tsx        # GDPR cookie consent
│   └── ImpersonationBanner.tsx # Admin impersonation UI
├── lib/                        # Shared utilities
│   ├── api/                    # API client functions
│   │   ├── auth.ts             # Auth endpoints
│   │   ├── overlays.ts         # Overlay CRUD
│   │   └── sources.ts          # Source management
│   ├── utils.ts                # cn() helper (clsx + tailwind-merge)
│   └── hooks/                  # Custom React hooks
├── stores/                     # Zustand state stores
│   ├── authStore.ts            # User authentication state
│   └── uiStore.ts              # UI preferences (theme, sidebar state)
├── styles/                     # Additional CSS (beyond Tailwind)
│   └── events.css              # Overlay event animations
└── types/                      # TypeScript type definitions
    ├── api.ts                  # API response types
    └── models.ts               # Domain models
```

### Structure Rationale

- **`app/`:** File-based routing via Next.js App Router. Each folder with `page.tsx` becomes a route. Server Components by default (optimal performance).
- **`components/ui/`:** shadcn/ui components are **copy-pasted**, not npm installed. This gives full control for customization while maintaining consistency.
- **`components/*`:** Domain-specific components organized by feature (theme-marketplace, admin, legal). This prevents `components/` from becoming a dumping ground.
- **`lib/api/`:** Centralized API layer. Each file corresponds to a backend service (auth-service, overlay-manager, etc.). Prevents scattered fetch() calls.
- **`stores/`:** Zustand stores for client-side global state. Keep minimal (auth, UI preferences only). Server state should live in React Query or similar.
- **`styles/`:** Minimal additional CSS. Most styling via Tailwind utility classes. `events.css` contains overlay-specific animations that are too complex for inline styles.

## Architectural Patterns

### Pattern 1: Tailwind v4 Design Token System

**What:** Design tokens defined via `@theme` directive in CSS, replacing JavaScript `tailwind.config.js`. All tokens become CSS variables accessible at runtime.

**When to use:** Always in 2026 with Tailwind v4. This is the standard approach.

**Trade-offs:**
- **Pros:** CSS-native (no JS config), runtime access to variables, 3.5x faster full builds, 8x faster incremental builds, natural theming with CSS custom properties
- **Cons:** Breaking change from v3 (migration required), cannot use JavaScript logic in config (e.g., plugins that generate classes dynamically)

**Example:**
```css
/* frontend/src/app/globals.css */
@import "tailwindcss";

@theme {
  /* Design tokens as CSS variables */
  --color-primary: oklch(0.87 0.00 0);
  --color-secondary: oklch(0.269 0 0);
  --color-background: oklch(0.145 0 0);
  --radius-lg: var(--radius);
  --radius-xl: calc(var(--radius) * 1.4);

  /* Platform-specific tokens */
  --color-twitch: #9146FF;
  --color-youtube: #FF0000;
  --color-kick: #53FC18;
}

/* Runtime access in custom CSS */
.custom-border {
  border-color: var(--color-primary);
}
```

**Usage in components:**
```tsx
// Utility class (generated by Tailwind)
<div className="bg-primary text-primary-foreground" />

// Runtime variable access
<div style={{ backgroundColor: 'var(--color-twitch)' }} />
```

### Pattern 2: Server-First Component Composition

**What:** Default to Server Components (RSC) for all pages/layouts, push Client Components down the tree to leaf nodes only where interactivity is needed.

**When to use:** Next.js 16 App Router projects. This is the recommended approach for optimal performance.

**Trade-offs:**
- **Pros:** 70% reduction in client JS, faster page loads, better SEO, free data fetching on server
- **Cons:** Cannot use React hooks (useState, useEffect) in Server Components, requires understanding of client/server boundary

**Example:**
```tsx
// app/dashboard/page.tsx (Server Component - default)
import { fetchUserOverlays } from '@/lib/api/overlays';
import { OverlayCard } from '@/components/OverlayCard'; // Client Component

export default async function DashboardPage() {
  // Data fetching on server - no loading state needed
  const overlays = await fetchUserOverlays();

  return (
    <div className="container py-8">
      <h1 className="text-3xl font-semibold">My Overlays</h1>
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mt-6">
        {overlays.map(overlay => (
          // Client Component for interactivity (hover, click)
          <OverlayCard key={overlay.id} overlay={overlay} />
        ))}
      </div>
    </div>
  );
}

// components/OverlayCard.tsx (Client Component - needs interactivity)
"use client"
import { useState } from 'react';
import { Card } from '@/components/ui/card';

export function OverlayCard({ overlay }) {
  const [isHovered, setIsHovered] = useState(false);

  return (
    <Card
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
    >
      {/* Interactive UI */}
    </Card>
  );
}
```

### Pattern 3: shadcn/ui Copy-Paste Component Library

**What:** Components installed via CLI (`npx shadcn@latest add button`) and copied into `src/components/ui/`. Not an npm package - you own the code.

**When to use:** When building a design system on Tailwind + React. This is the standard for 2026.

**Trade-offs:**
- **Pros:** Full customization (you own the code), no version lock-in, tree-shakeable (only install what you need), matches your design tokens automatically, accessible by default (Radix UI primitives)
- **Cons:** No automatic updates (must re-install manually), can diverge from upstream if heavily customized

**Example:**
```bash
# Install shadcn CLI and initialize (one-time setup)
npx shadcn@latest init

# Add components as needed (copies into src/components/ui/)
npx shadcn@latest add button card input dialog badge

# Customize after installation
# Edit src/components/ui/button.tsx to match DESIGN_SYSTEM.md
```

**Customization workflow:**
```tsx
// BEFORE: Default shadcn button (zinc colors, rounded-md)
const buttonVariants = cva(
  "rounded-md bg-zinc-900 text-zinc-50",
  { variants: { ... } }
);

// AFTER: Customized to match All-Chat design system
const buttonVariants = cva(
  "rounded-lg bg-gradient-to-r from-purple-500 to-blue-500 text-white shadow-md transition-all duration-200 hover:shadow-lg hover:scale-[1.02]",
  { variants: { ... } }
);
```

### Pattern 4: Component Slot Pattern for Client/Server Boundaries

**What:** Use `children` prop to create a "slot" in Client Components, allowing Server Components to be passed through without forcing the parent to be client-side.

**When to use:** When a Server Component needs to be rendered inside a Client Component (e.g., modal with server-fetched content).

**Trade-offs:**
- **Pros:** Avoids unnecessary client boundaries, maintains server benefits for children
- **Cons:** Slightly more complex prop passing

**Example:**
```tsx
// components/ui/dialog.tsx (Client Component)
"use client"
export function Dialog({ children, open, onOpenChange }) {
  return (
    <DialogPrimitive open={open} onOpenChange={onOpenChange}>
      {children} {/* Children can be Server Components */}
    </DialogPrimitive>
  );
}

// app/dashboard/page.tsx (Server Component)
import { Dialog } from '@/components/ui/dialog';
import { fetchOverlayDetails } from '@/lib/api/overlays';

export default async function DashboardPage() {
  const details = await fetchOverlayDetails(); // Server fetch

  return (
    <Dialog open={isOpen}>
      {/* This content stays on server despite Dialog being client */}
      <div>
        <h2>Overlay: {details.name}</h2>
        <p>{details.description}</p>
      </div>
    </Dialog>
  );
}
```

### Pattern 5: CSS Class Migration Strategy for Marketplace Compatibility

**What:** Gradual migration from old class names to design system classes while maintaining backward compatibility for user-generated CSS (overlay marketplace themes).

**When to use:** When redesigning an existing app with user-created themes/styles that reference specific class names.

**Trade-offs:**
- **Pros:** Zero downtime for users, graceful migration, time to communicate changes
- **Cons:** Temporary duplication (old + new classes), requires deprecation timeline

**Example:**
```tsx
// Phase 1: Dual class names (old + new)
<button className="bg-purple-600 bg-primary hover:bg-purple-700 hover:bg-primary/80">
  {/* Both old (purple-600) and new (primary) classes present */}
</button>

// Phase 2: Deprecation warning in CSS editor
// Add linter rule to warn about deprecated classes
// Display migration guide in theme marketplace UI

// Phase 3: Remove old classes after 6-month grace period
<button className="bg-primary hover:bg-primary/80">
  {/* Only new design system classes */}
</button>
```

**Migration guide document:**
```markdown
# Overlay CSS Migration Guide

## Deprecated Classes → New Classes

| Old Class | New Class | Reason |
|-----------|-----------|--------|
| bg-gray-800 | bg-slate-850 | Gray → Slate (warmer tone) |
| bg-purple-600 | bg-primary | Use semantic token |
| rounded-md | rounded-lg | Design system standardization |
| shadow | shadow-lg | Consistent depth scale |

## Timeline
- **2026-03**: Dual classes added (backward compatible)
- **2026-09**: Old classes deprecated (warnings shown)
- **2027-03**: Old classes removed (breaking change)
```

## Data Flow

### Request Flow (Client → Server)

```
[User Interaction in Browser]
    ↓
[Client Component (Button click, form submit)]
    ↓
[API Fetch Function (lib/api/overlays.ts)]
    ↓
[Next.js API Route or Direct Backend Call]
    ↓
[Backend Service (overlay-manager, auth-service)]
    ↓
[PostgreSQL / Redis]
    ↓
[Response] ← [Transform] ← [API Response]
    ↓
[Update UI or Zustand Store]
```

### State Management Flow

```
[Zustand Store (authStore, uiStore)]
    ↓ (subscribe via useStore hook)
[Client Components] ←→ [Store Actions] → [Store Mutations] → [Zustand Store]
    ↑
[Persist to localStorage (optional)]
```

### Design Token Flow (Tailwind v4)

```
[DESIGN_SYSTEM.md (Source of truth)]
    ↓
[globals.css @theme directive]
    ↓
[Tailwind v4 Engine (Rust)]
    ↓
[Generated utility classes (bg-primary, text-slate-50, etc.)]
    ↓
[CSS Variables (--color-primary, --color-background, etc.)]
    ↓ (consumed by)
[shadcn/ui Components] + [Custom Components] + [Runtime CSS]
```

### Key Data Flows

1. **Page Load (Server-First):**
   - User navigates → Next.js App Router → Server Component renders → Data fetched on server → HTML sent to client → Client Components hydrate (minimal JS)

2. **Form Submission (Client-Server):**
   - User submits form → Client Component validates → API fetch → Backend service → Database mutation → Success response → Client updates UI

3. **Design Token Changes:**
   - Developer edits `globals.css` @theme → Tailwind v4 rebuilds CSS (8x faster incremental) → New utility classes available → Components reflect changes immediately

4. **Theme Marketplace CSS:**
   - User selects theme → Theme CSS loaded → Applied to overlay preview (iframe) → User can customize via Monaco editor → CSS saved to database → Rendered in production overlay

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| **0-1k users** | Current architecture is perfect. Single Next.js deployment, Tailwind CSS, shadcn/ui components. No changes needed. |
| **1k-100k users** | Add CDN for static assets (Vercel/Cloudflare), optimize images (Next.js Image component), consider React Query for API caching, add Suspense boundaries for better loading states. |
| **100k+ users** | Edge rendering for landing page (Vercel Edge Functions), separate API Gateway for backend (already exists in your architecture), consider component code splitting (React.lazy), optimize bundle size (tree-shaking, dynamic imports). |

### Scaling Priorities

1. **First bottleneck: Overlay CSS rendering**
   - Problem: User-generated CSS in theme marketplace can be large (100KB+)
   - Solution: Lazy-load theme CSS only when preview is opened, cache compiled CSS on server (Redis), minify CSS before serving

2. **Second bottleneck: Monaco Editor bundle size**
   - Problem: Monaco Editor is 2-3MB, slows down overlay editor page
   - Solution: Dynamic import with React.lazy (`const Monaco = lazy(() => import('./MonacoCSSEditor'))`), show loading skeleton while loading

3. **Third bottleneck: Real-time overlay updates**
   - Problem: WebSocket connections can overwhelm server at scale
   - Solution: Already solved (Redis Pub/Sub in backend), no frontend changes needed

## Anti-Patterns

### Anti-Pattern 1: Using Client Components for Everything

**What people do:** Add `"use client"` to every component "just in case" or because they're used to traditional React.

**Why it's wrong:** Defeats the purpose of Next.js App Router. Sends unnecessary JavaScript to client (70% more JS), loses server-side rendering benefits, slower page loads, worse SEO.

**Do this instead:**
- Default to Server Components
- Only add `"use client"` when you need:
  - React hooks (useState, useEffect, useContext)
  - Browser APIs (localStorage, window, document)
  - Event handlers (onClick, onChange)
- Push client components down the tree to leaf nodes

### Anti-Pattern 2: Diverging from Design System

**What people do:** Add one-off styles directly in components (`className="bg-[#9146FF] rounded-[13px]"`) instead of using design tokens.

**Why it's wrong:** Creates inconsistency, harder to theme, arbitrary values don't benefit from design token changes, harder to maintain.

**Do this instead:**
- Use design system tokens: `className="bg-twitch rounded-xl"`
- If a new token is needed, add it to `globals.css` @theme first
- Reference `DESIGN_SYSTEM.md` for approved colors, spacing, shadows

### Anti-Pattern 3: Installing shadcn Components Without Customization

**What people do:** Run `npx shadcn@latest add button` and use it as-is without matching design system.

**Why it's wrong:** shadcn/ui defaults don't match your design (zinc colors, different radii, missing gradients).

**Do this instead:**
1. Install component via CLI
2. Open `src/components/ui/[component].tsx`
3. Customize to match `DESIGN_SYSTEM.md` (colors, spacing, shadows, transitions)
4. Document changes in component file comments

### Anti-Pattern 4: Mixing Tailwind Config Approaches (v3 vs v4)

**What people do:** Try to use `tailwind.config.js` alongside `@theme` directive, or use `extend` syntax from v3.

**Why it's wrong:** Tailwind v4 uses CSS-native configuration. JavaScript config is deprecated. Mixing approaches causes conflicts.

**Do this instead:**
- All theme configuration in `globals.css` via `@theme` directive
- Platform colors, shadows, spacing, etc. all in CSS
- Only use `tailwind.config.js` for `content` paths (required)

### Anti-Pattern 5: Hardcoding Styles in Marketplace Themes

**What people do:** User themes hardcode colors like `background: #0f172a` instead of using CSS variables.

**Why it's wrong:** Themes break when design system changes, users can't customize further, no dark mode support.

**Do this instead:**
- Provide CSS variable reference in theme marketplace docs
- Example: `background: var(--color-background)` instead of hex codes
- Allow users to override variables in their theme CSS

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| **Backend Services (Go)** | HTTP REST API via `fetch()` in `lib/api/` | API Gateway at `localhost:8080`, JWT auth in headers |
| **Monaco Editor** | React wrapper component, dynamic import | Client Component, lazy-loaded, 2-3MB bundle |
| **WebSocket (Overlays)** | Native WebSocket API, reconnection logic | Connected in overlay preview page, listens for chat messages |
| **Tailwind CSS v4** | CSS-native via `@theme` directive in `globals.css` | No JavaScript config, all tokens in CSS |
| **shadcn/ui** | Copy-paste via CLI, not npm package | Components in `src/components/ui/`, customized after install |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| **Server ↔ Client Components** | Props, children slot | Server can pass data to client via props, client can render server children |
| **Page ↔ API Layer** | Direct function calls | Server Components call `lib/api/*` directly, Client Components use hooks |
| **Components ↔ Zustand Stores** | `useStore()` hook | Only in Client Components, minimal state (auth, UI prefs) |
| **Design Tokens ↔ Components** | Tailwind utility classes, CSS variables | Components use `bg-primary`, tokens defined in `@theme` |
| **Overlay Preview ↔ Theme CSS** | Iframe with injected CSS | Isolated from parent, user CSS only affects iframe content |

## New Components vs Modified Components

### Components to Create (New)

Based on the design system spec and current architecture, these components don't exist yet:

| Component | Type | Purpose |
|-----------|------|---------|
| `components/ui/card.tsx` | shadcn/ui | Card container (install via CLI, customize to design system) |
| `components/ui/input.tsx` | shadcn/ui | Form input (install via CLI, customize focus rings, borders) |
| `components/ui/dialog.tsx` | shadcn/ui | Modal/dialog (install via CLI, customize backdrop, shadows) |
| `components/ui/badge.tsx` | shadcn/ui | Status/platform badges (install via CLI, add platform color variants) |
| `components/ui/select.tsx` | shadcn/ui | Dropdown select (install via CLI, customize options styling) |
| `components/ui/toast.tsx` | shadcn/ui | Notifications (install via CLI, replace react-hot-toast) |
| `components/DesignSystemGuard.tsx` | Custom | ESLint-like component that warns in dev mode about design violations |

### Components to Modify (Existing)

| Component | Changes Needed | Reason |
|-----------|---------------|--------|
| `components/ui/button.tsx` | Update variants to match `DESIGN_SYSTEM.md` (gradient primary, slate secondary, proper shadows) | Currently uses Base UI defaults, doesn't match design spec |
| `app/layout.tsx` | Add design system CSS variables to root element, update font loading | Need to expose design tokens to all pages |
| `app/globals.css` | Align @theme values with `DESIGN_SYSTEM.md` (slate not zinc, platform colors, shadows) | Current tokens don't match design system spec |
| `components/MonacoCSSEditor.tsx` | Add design system class warnings, migration guide link | Help users migrate deprecated classes |
| `components/PlatformStatusIndicators.tsx` | Update to use new badge component, platform colors from design system | Currently uses hardcoded colors |
| `components/theme-marketplace/ThemeCard.tsx` | Update to use new card component, platform color accents | Needs consistent styling |

### Build Order (Foundation → Components → Pages)

**Phase 1: Design Token Foundation (Week 1)**
1. Update `app/globals.css` @theme directive to match `DESIGN_SYSTEM.md` exactly
2. Verify Tailwind utility classes generate correctly (`bg-primary`, `text-slate-50`, etc.)
3. Document CSS variable mapping for marketplace themes

**Phase 2: Primitive Components (Week 2-3)**
1. Install shadcn/ui components via CLI: `button`, `card`, `input`, `dialog`, `badge`, `select`, `toast`
2. Customize each component to match design system (colors, shadows, transitions)
3. Update existing `button.tsx` to match spec
4. Test all variants (primary, secondary, destructive, ghost)

**Phase 3: Domain Components (Week 4-5)**
1. Update `PlatformStatusIndicators.tsx` to use new badge component
2. Update theme marketplace components (`ThemeCard`, `ThemeFilters`, `ThemePreview`)
3. Update admin components (`BanModal`, `ToastProvider`)
4. Create `DesignSystemGuard.tsx` for development warnings

**Phase 4: Page Redesigns (Week 6-10)**
1. Landing page (hero section, platform login buttons)
2. Dashboard (overlay grid, create button)
3. Overlay editor (sidebar, Monaco editor wrapper, preview)
4. Settings page (forms, account management)
5. Admin pages (tables, filters, modals)

**Phase 5: Enforcement & Migration (Week 11-12)**
1. Add ESLint rules for design system compliance (no `bg-gray-*`, no arbitrary values without @theme)
2. Create pre-commit hook to validate design system usage
3. Write marketplace CSS migration guide (old classes → new classes)
4. Add deprecation warnings in Monaco editor for old classes

## CSS Migration Strategy for Marketplace Compatibility

### Challenge

User-generated CSS in the theme marketplace references class names and styles that may change during the redesign. Breaking these without warning creates a poor user experience.

### Solution: Dual-Class Transition Period

**Timeline:**
- **Phase 1 (Months 1-3):** Add new design system classes alongside old classes (backward compatible)
- **Phase 2 (Months 4-9):** Deprecation warnings in Monaco editor, migration guide published
- **Phase 3 (Month 10+):** Remove old classes (breaking change, announced 6 months in advance)

**Implementation:**

1. **Dual Classes in Core Components:**
```tsx
// Button component during transition (Phase 1)
<button className="
  bg-purple-600 bg-primary          /* Old + New */
  hover:bg-purple-700 hover:bg-primary/80
  rounded-md rounded-lg             /* Old + New */
  shadow shadow-md                  /* Old + New */
">
```

2. **Migration Guide in Theme Marketplace:**
```tsx
// components/theme-marketplace/MigrationBanner.tsx
export function MigrationBanner() {
  return (
    <div className="rounded-lg border border-yellow-500/20 bg-yellow-500/10 p-4">
      <h3 className="text-sm font-semibold text-yellow-400">
        Design System Update
      </h3>
      <p className="mt-1 text-xs text-yellow-300">
        We're updating our design system. Some CSS classes will be deprecated in 6 months.
        <a href="/docs/css-migration" className="underline">View migration guide</a>
      </p>
    </div>
  );
}
```

3. **Linter Rules in Monaco Editor:**
```typescript
// lib/cssLinter.ts
const deprecatedClasses = [
  { old: 'bg-gray-800', new: 'bg-slate-850', deprecated: '2026-09-01' },
  { old: 'bg-purple-600', new: 'bg-primary', deprecated: '2026-09-01' },
  { old: 'rounded-md', new: 'rounded-lg', deprecated: '2026-09-01' },
];

export function lintCSS(code: string): Warning[] {
  const warnings = [];
  for (const rule of deprecatedClasses) {
    if (code.includes(rule.old)) {
      warnings.push({
        message: `'${rule.old}' is deprecated. Use '${rule.new}' instead.`,
        severity: 'warning',
        deprecationDate: rule.deprecated,
      });
    }
  }
  return warnings;
}
```

4. **CSS Variable Fallbacks:**
```css
/* For custom user CSS that uses old variables */
:root {
  /* Old variables (deprecated) */
  --color-bg-primary: var(--color-background); /* Fallback to new */
  --color-bg-secondary: var(--color-card); /* Fallback to new */

  /* New variables (design system) */
  --color-background: oklch(0.145 0 0);
  --color-card: oklch(0.205 0 0);
}
```

### Performance Considerations

**Tailwind v4 Performance:**
- Full rebuilds: 3.5x faster than v3 (3.5s → <100ms)
- Incremental builds: 8x faster (measured in single-digit milliseconds)
- Impact: Design token changes reflect instantly in dev mode

**Next.js 16 Performance:**
- Server Components: 70% reduction in client JS
- Turbopack (dev): 10x faster HMR, 4x faster production builds
- Impact: Faster development iteration, better production performance

**Component Library Performance:**
- shadcn/ui: Tree-shakeable (only install what you need), no runtime overhead (compiled to Tailwind)
- Class Variance Authority (CVA): Zero-runtime CSS-in-JS, compiled to static classes
- Impact: No JavaScript overhead for styling

**Optimization Priorities:**
1. **Monaco Editor:** Lazy-load with React.lazy (2-3MB bundle)
2. **Theme CSS:** Cache compiled CSS on server, minify before serving
3. **Images:** Use Next.js Image component (automatic optimization)
4. **WebSocket:** Already optimized (Redis Pub/Sub in backend)

## Sources

### Official Documentation
- [Tailwind CSS v4.0](https://tailwindcss.com/blog/tailwindcss-v4) - Official v4 release announcement
- [Tailwind CSS Theme Variables](https://tailwindcss.com/docs/theme) - @theme directive documentation
- [Next.js Server and Client Components](https://nextjs.org/docs/app/getting-started/server-and-client-components) - App Router patterns
- [shadcn/ui Next.js Installation](https://ui.shadcn.com/docs/installation/next) - Integration guide
- [Tailwind CSS Adding Custom Styles](https://tailwindcss.com/docs/adding-custom-styles) - Customization patterns

### Best Practices & Guides (2026)
- [Design Tokens That Scale in 2026 (Tailwind v4 + CSS Variables)](https://www.maviklabs.com/blog/design-tokens-tailwind-v4-2026) - Design token architecture
- [Tailwind CSS v4: The Complete Guide for 2026](https://devtoolbox.dedyn.io/blog/tailwind-css-v4-complete-guide) - Migration and best practices
- [Next.js App Router: The Patterns That Actually Matter in 2026](https://dev.to/teguh_coding/nextjs-app-router-the-patterns-that-actually-matter-in-2026-146) - Server/Client patterns
- [Build a Dashboard with shadcn/ui: Complete Guide (2026)](https://designrevision.com/blog/shadcn-dashboard-tutorial) - Component organization

### Migration & Component Patterns
- [Tailwind CSS v4 Migration: New Features Guide 2026](https://www.digitalapplied.com/blog/tailwind-css-v4-migration-new-features-guide) - v3 to v4 migration
- [How to Implement a Design System: Migration Path](https://www.designsystemscollective.com/how-to-implement-a-design-system-reasons-approach-and-migration-path-051c41734caf) - Design system implementation
- [Pro Tips for Design System Migration in Large Projects](https://medium.com/@houhoucoop/pro-tips-for-ui-library-migration-in-large-projects-d54f0fbcd083) - Migration strategies
- [The Ultimate Guide to Building a Monorepo in 2026](https://medium.com/@sanjaytomar717/the-ultimate-guide-to-building-a-monorepo-in-2025-sharing-code-like-the-pros-ee4d6d56abaa) - Component library organization

---
*Architecture research for: All-Chat Frontend Design System Integration*
*Researched: 2026-03-09*
*Confidence: HIGH (Tailwind v4, Next.js 16, shadcn/ui patterns verified with official docs and 2026 best practices)*
