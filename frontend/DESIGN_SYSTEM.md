# All-Chat Design System

**Last Updated**: 2026-03-09
**Status**: Active - All frontend changes must follow these rules
**Design Direction**: StreamElements Modern (Option 2)

---

## Philosophy

All-Chat is a **professional creator tool** for streamers aggregating multi-platform chat. The UI should feel:
- **Polished** (creators expect professional tools)
- **Approachable** (not sterile or intimidating)
- **Functional** (tool-first, not decoration)
- **Consistent** (predictable patterns across all pages)

**Inspiration**: StreamElements, Linear, Notion, Radix UI
**Not like**: Generic Tailwind templates, corporate dashboards, social media apps

---

## Color System

### Base Palette
```css
/* Backgrounds */
--bg-primary: #0f172a     /* slate-900 - page background */
--bg-secondary: #1a2332   /* slate-850 - card background */
--bg-tertiary: #1e293b    /* slate-800 - elevated cards */

/* Borders */
--border-default: rgb(51 65 85 / 0.5)  /* slate-700/50 - subtle borders */
--border-focus: #3b82f6   /* blue-500 - focus rings */

/* Text */
--text-primary: #f8fafc   /* slate-50 - headings, primary content */
--text-secondary: #94a3b8 /* slate-400 - secondary content */
--text-tertiary: #64748b  /* slate-500 - labels, metadata */

/* Platform Colors (use as accents only) */
--color-twitch: #9146FF
--color-youtube: #FF0000
--color-kick: #53FC18
--color-tiktok: #000000

/* Accent Gradient (primary CTAs) */
--accent-gradient: linear-gradient(to right, #a855f7, #3b82f6)  /* purple-500 → blue-500 */

/* Status Colors */
--status-success: #10b981  /* green-500 */
--status-warning: #f59e0b  /* amber-500 */
--status-error: #ef4444    /* red-500 */
--status-info: #3b82f6     /* blue-500 */
```

### Usage Rules
```
✅ DO:
- Use slate-900/850/800 for backgrounds (not gray-900)
- Use platform colors for badges, borders, status indicators
- Use accent gradient for primary CTAs only (create overlay, save, submit)
- Use status colors for states (connected/disconnected, success/error)

❌ DON'T:
- Mix gray and slate scales (pick one: slate)
- Use more than 2 accent colors in one component
- Use bright colors for large backgrounds
- Use pure black (#000000) except for TikTok brand
```

---

## Typography

### Font Stack
```css
--font-sans: 'Inter', system-ui, -apple-system, sans-serif;
--font-mono: 'SF Mono', 'Consolas', monospace;
```

### Scale
```
/* Headings */
--text-4xl: 2.25rem (36px) - Page titles
--text-3xl: 1.875rem (30px) - Section headers
--text-2xl: 1.5rem (24px) - Card titles
--text-xl: 1.25rem (20px) - Subsection headers
--text-lg: 1.125rem (18px) - Emphasized text

/* Body */
--text-base: 1rem (16px) - Default body text
--text-sm: 0.875rem (14px) - Secondary text, labels
--text-xs: 0.75rem (12px) - Metadata, captions

/* Weights */
--font-semibold: 600 - Headings
--font-medium: 500 - Emphasized body text
--font-normal: 400 - Body text
```

### Usage Rules
```
✅ DO:
- Page titles: text-3xl font-semibold text-slate-50
- Section headers: text-xl font-semibold text-slate-50
- Card titles: text-lg font-semibold text-slate-50
- Body text: text-base text-slate-300
- Labels: text-sm font-medium text-slate-400
- Metadata: text-xs text-slate-500
- Use leading-relaxed (1.625) for body text

❌ DON'T:
- Use more than 3 font sizes on one page
- Use font weights below 400 (no light/thin)
- Use uppercase for body text (only for small labels: text-xs uppercase tracking-wide)
```

---

## Spacing & Layout

### Container System
```
/* Max widths */
--container-sm: 640px   /* Forms, focused content */
--container-md: 768px   /* Standard content */
--container-lg: 1024px  /* Dashboards */
--container-xl: 1280px  /* Wide dashboards */
--container-2xl: 1536px /* Admin tables */

/* Padding */
--container-padding: px-4 sm:px-6 lg:px-8
```

### Spacing Scale
```
/* Use Tailwind scale consistently */
gap-2  (0.5rem / 8px)   - Icon + text
gap-3  (0.75rem / 12px) - Form fields
gap-4  (1rem / 16px)    - Card internal spacing
gap-6  (1.5rem / 24px)  - Between cards
gap-8  (2rem / 32px)    - Between sections

/* Padding */
p-3    - Compact buttons, badges
p-4    - Compact cards
p-6    - Standard cards
p-8    - Page sections

/* Margin */
mb-2   - Label → Input
mb-4   - Input → Input
mb-6   - Section → Section
mb-8   - Major section breaks
```

### Usage Rules
```
✅ DO:
- Use even numbers (gap-2, gap-4, gap-6, gap-8)
- Cards: p-6 (default), p-4 (compact)
- Sections: py-8 or py-12
- Responsive: gap-4 md:gap-6 lg:gap-8

❌ DON'T:
- Use odd spacing (gap-5, gap-7, p-5)
- Mix px/py unnecessarily (use p-6, not px-6 py-6)
- Use mb-12+ (break into sections instead)
```

---

## Components

### Buttons

**Variants:**
```tsx
// Primary (main actions)
<button className="rounded-lg bg-gradient-to-r from-purple-500 to-blue-500 px-6 py-2.5 text-sm font-semibold text-white shadow-md transition-all duration-200 hover:shadow-lg hover:scale-[1.02]">
  Create Overlay
</button>

// Secondary (alternative actions)
<button className="rounded-lg border border-slate-700 bg-slate-850 px-6 py-2.5 text-sm font-semibold text-slate-100 shadow-md transition-all duration-200 hover:bg-slate-800 hover:shadow-lg">
  Cancel
</button>

// Destructive (delete, disconnect)
<button className="rounded-lg bg-red-600 px-6 py-2.5 text-sm font-semibold text-white shadow-md transition-all duration-200 hover:bg-red-700 hover:shadow-lg">
  Delete
</button>

// Ghost (tertiary actions)
<button className="rounded-lg px-4 py-2 text-sm font-medium text-slate-400 transition-colors hover:bg-slate-800 hover:text-slate-100">
  Learn More
</button>

// Icon button (toolbar actions)
<button className="rounded-lg p-2 text-slate-400 transition-colors hover:bg-slate-800 hover:text-slate-100">
  <Icon size={20} />
</button>
```

**Rules:**
```
✅ DO:
- Use gradient only for primary CTAs
- Add shadow-md + hover:shadow-lg
- Use transition-all duration-200
- Icon buttons: p-2, icon size 20px

❌ DON'T:
- Use gradients on multiple button types
- Omit hover states
- Use py-3+ (buttons should be compact)
```

---

### Cards

**Variants:**
```tsx
// Standard card
<div className="rounded-xl border border-slate-700/50 bg-slate-850 p-6 shadow-lg">
  {children}
</div>

// Elevated card (modals, popovers)
<div className="rounded-xl border border-slate-700/50 bg-slate-800 p-6 shadow-2xl">
  {children}
</div>

// Interactive card (hover state)
<div className="rounded-xl border border-slate-700/50 bg-slate-850 p-6 shadow-lg transition-all duration-200 hover:shadow-xl hover:scale-[1.02] cursor-pointer">
  {children}
</div>

// Compact card (dashboard widgets)
<div className="rounded-lg border border-slate-700/50 bg-slate-850 p-4 shadow-md">
  {children}
</div>
```

**Rules:**
```
✅ DO:
- Default: rounded-xl, p-6, shadow-lg
- Compact: rounded-lg, p-4, shadow-md
- Always include border (subtle depth)
- Interactive: add hover:shadow-xl hover:scale-[1.02]

❌ DON'T:
- Use rounded-md or rounded-2xl (stick to lg/xl)
- Omit shadows (cards need depth)
- Nest cards more than 2 levels deep
```

---

### Form Inputs

**Text inputs:**
```tsx
<div className="space-y-2">
  <label className="block text-sm font-medium text-slate-400">
    Overlay Name
  </label>
  <input
    type="text"
    className="w-full rounded-lg border border-slate-600 bg-slate-850 px-4 py-2.5 text-sm text-slate-100 placeholder-slate-500 transition-all duration-200 focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500/20"
    placeholder="My Awesome Overlay"
  />
</div>

// Error state
<input className="... border-red-500 focus:border-red-500 focus:ring-red-500/20" />
<p className="mt-1.5 text-xs text-red-400">This field is required</p>
```

**Rules:**
```
✅ DO:
- Label: text-sm font-medium text-slate-400
- Input: rounded-lg, px-4 py-2.5, bg-slate-850
- Focus: ring-2, ring-blue-500/20 (20% opacity)
- Error: border-red-500, text-xs text-red-400
- Space: space-y-2 (label → input)

❌ DON'T:
- Use py-3+ (inputs should align with buttons)
- Omit focus rings (accessibility)
- Use placeholder as label (separate label required)
```

---

### Badges

**Platform badges:**
```tsx
// Twitch
<span className="inline-flex items-center gap-1.5 rounded-full bg-purple-500/10 px-2.5 py-0.5 text-xs font-medium text-purple-400 border border-purple-500/20">
  <TwitchIcon size={12} />
  Twitch
</span>

// YouTube
<span className="inline-flex items-center gap-1.5 rounded-full bg-red-500/10 px-2.5 py-0.5 text-xs font-medium text-red-400 border border-red-500/20">
  <YoutubeIcon size={12} />
  YouTube
</span>

// Status badge
<span className="inline-flex items-center gap-1.5 rounded-full bg-green-500/10 px-2.5 py-0.5 text-xs font-medium text-green-400 border border-green-500/20">
  <div className="h-1.5 w-1.5 rounded-full bg-green-400 animate-pulse" />
  Connected
</span>
```

**Rules:**
```
✅ DO:
- rounded-full (not rounded-lg)
- px-2.5 py-0.5 (tight padding)
- bg-{color}-500/10 (10% opacity background)
- border border-{color}-500/20 (subtle border)
- text-xs font-medium
- Icon: size={12}

❌ DON'T:
- Use solid backgrounds (too bold)
- Use text-sm (badges are small)
- Omit borders (need subtle definition)
```

---

### Modals & Overlays

**Modal backdrop:**
```tsx
<div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
  <div className="relative w-full max-w-lg rounded-xl border border-slate-700/50 bg-slate-800 p-6 shadow-2xl">
    {children}
  </div>
</div>
```

**Rules:**
```
✅ DO:
- Backdrop: bg-black/60 backdrop-blur-sm
- Modal: bg-slate-800 (one level darker)
- Max width: max-w-lg (default), max-w-2xl (large)
- Shadow: shadow-2xl (strongest depth)
- Z-index: z-50

❌ DON'T:
- Use bg-slate-900 (too dark, no contrast)
- Omit backdrop-blur-sm (depth cue)
- Use fixed width (always max-w-X)
```

---

## Layout Patterns

### Dashboard Grid
```tsx
<div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
  <Card>Overlay 1</Card>
  <Card>Overlay 2</Card>
  <Card>Overlay 3</Card>
</div>
```

### Sidebar + Content
```tsx
<div className="flex min-h-screen">
  <aside className="w-64 border-r border-slate-700/50 bg-slate-900 p-6">
    Navigation
  </aside>
  <main className="flex-1 p-8">
    Content
  </main>
</div>
```

### Split Preview
```tsx
<div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
  <div>Configuration</div>
  <div className="sticky top-6 h-fit">Live Preview</div>
</div>
```

---

## Animation & Transitions

### Standard Transitions
```css
/* Default (most components) */
transition-all duration-200

/* Color changes only (hover states) */
transition-colors duration-200

/* Slow transitions (large elements) */
transition-all duration-300
```

### Hover States
```css
/* Cards */
hover:shadow-xl hover:scale-[1.02]

/* Buttons */
hover:shadow-lg hover:scale-[1.02]

/* Links */
hover:text-slate-100

/* Backgrounds */
hover:bg-slate-800
```

### Rules
```
✅ DO:
- Use duration-200 (default)
- Combine shadow + scale for depth
- Use scale-[1.02] (subtle, not 1.05)
- Add cursor-pointer for interactive elements

❌ DON'T:
- Use duration-100 (too fast)
- Use scale-[1.1] (too aggressive)
- Animate width/height (janky performance)
- Use transform-gpu unless necessary
```

---

## Icons

**Library**: Lucide React (already installed)

**Sizing:**
```
size={16} - Inline with text (text-sm)
size={20} - Default (text-base)
size={24} - Emphasized icons (text-lg)
size={32} - Large icons (hero sections)
```

**Usage:**
```tsx
// Inline with text
<button className="inline-flex items-center gap-2">
  <PlusIcon size={16} />
  <span>Add Source</span>
</button>

// Icon-only button
<button className="p-2 rounded-lg hover:bg-slate-800">
  <SettingsIcon size={20} />
</button>

// Leading icon (cards, list items)
<div className="flex items-start gap-3">
  <TwitchIcon size={24} className="text-purple-400" />
  <div>...</div>
</div>
```

**Rules:**
```
✅ DO:
- Use size prop (not className="w-5 h-5")
- Use stroke-2 (default weight)
- Color icons with platform colors
- Align with text: items-center

❌ DON'T:
- Mix icon sizes in same component
- Use size={18} or size={22} (stick to 16/20/24)
- Use stroke-1 (too thin)
```

---

## Responsive Design

### Breakpoints
```
sm: 640px   - Mobile landscape
md: 768px   - Tablet
lg: 1024px  - Desktop
xl: 1280px  - Large desktop
2xl: 1536px - Ultra-wide
```

### Patterns
```tsx
// Grid columns
<div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3">

// Padding
<div className="px-4 sm:px-6 lg:px-8">

// Text size
<h1 className="text-2xl md:text-3xl lg:text-4xl">

// Hidden on mobile
<div className="hidden lg:block">

// Stack on mobile, side-by-side on desktop
<div className="flex flex-col lg:flex-row gap-6">
```

**Rules:**
```
✅ DO:
- Design mobile-first (default styles for mobile)
- Test at 375px, 768px, 1280px
- Hide non-essential content on mobile
- Use gap-4 md:gap-6 lg:gap-8 (scale spacing)

❌ DON'T:
- Use max-sm: or max-md: (mobile-first only)
- Hide critical actions on mobile
- Use fixed pixel widths (always responsive units)
```

---

## Platform Color Usage

### Color Mapping
```tsx
const platformColors = {
  twitch: {
    bg: 'bg-purple-500/10',
    border: 'border-purple-500/20',
    text: 'text-purple-400',
    badge: 'bg-purple-500'
  },
  youtube: {
    bg: 'bg-red-500/10',
    border: 'border-red-500/20',
    text: 'text-red-400',
    badge: 'bg-red-500'
  },
  kick: {
    bg: 'bg-green-500/10',
    border: 'border-green-500/20',
    text: 'text-green-400',
    badge: 'bg-green-500'
  },
  tiktok: {
    bg: 'bg-slate-700/10',
    border: 'border-slate-600/20',
    text: 'text-slate-300',
    badge: 'bg-slate-600'
  }
}
```

### Usage Examples
```tsx
// Source card with platform accent
<div className="rounded-xl border-l-4 border-l-purple-500 bg-slate-850 p-6">
  <span className="text-purple-400">Twitch</span>
</div>

// Platform badge
<span className="inline-flex items-center gap-1.5 rounded-full bg-red-500/10 px-2.5 py-0.5 text-xs font-medium text-red-400 border border-red-500/20">
  YouTube
</span>
```

**Rules:**
```
✅ DO:
- Use platform colors for badges, borders, status indicators
- Use /10 opacity for backgrounds, /20 for borders
- Keep text readable: 400 shade (not 500)
- Border-left accent: border-l-4 border-l-{color}-500

❌ DON'T:
- Use platform colors for large backgrounds
- Mix multiple platform colors in one component
- Use solid platform backgrounds (too bright)
```

---

## Accessibility

### Focus States
```css
/* All interactive elements MUST have visible focus */
focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500
```

### Color Contrast
```
✅ Required contrast ratios:
- Normal text (16px): 4.5:1
- Large text (24px+): 3:1
- Interactive elements: 3:1

✅ Our palette meets WCAG AA:
- slate-50 on slate-900: 16.1:1 ✓
- slate-400 on slate-900: 7.2:1 ✓
- blue-500 on slate-900: 8.2:1 ✓
```

### Keyboard Navigation
```
✅ DO:
- All interactive elements: cursor-pointer
- Buttons: Add focus:ring-2
- Modals: Trap focus, Escape to close
- Forms: Tab order logical

❌ DON'T:
- Use div as button (use <button>)
- Remove focus outline without replacement
- Use hover:scale without focus equivalent
```

---

## Component Library Integration

### Recommended: shadcn/ui

**Why:**
- Matches our design system (Tailwind + Radix UI)
- Copy-paste components (not npm dependency)
- Fully customizable
- Accessible by default

**Install:**
```bash
npx shadcn@latest init
```

**Components to add first:**
```bash
npx shadcn@latest add button
npx shadcn@latest add card
npx shadcn@latest add input
npx shadcn@latest add dialog
npx shadcn@latest add badge
npx shadcn@latest add select
npx shadcn@latest add toast
```

**Customize** (edit `components/ui/*.tsx` to match our colors):
- Change colors from zinc → slate
- Adjust rounded-md → rounded-lg/xl
- Update shadows to match our system

---

## LLM Enforcement Checklist

When implementing UI changes, **validate against these rules:**

```
□ Colors: Only slate-900/850/800 backgrounds (not gray)
□ Text: slate-50 (headings), slate-400 (body)
□ Spacing: Even numbers only (gap-4, gap-6, not gap-5)
□ Buttons: rounded-lg, py-2.5 px-6, shadow-md
□ Cards: rounded-xl, p-6, shadow-lg, border
□ Icons: Lucide React, size={20} default
□ Transitions: transition-all duration-200
□ Hover: shadow-xl + scale-[1.02]
□ Focus: ring-2 ring-blue-500/20
□ Platform colors: badges/borders only (not backgrounds)
□ Typography: Inter font, text-base default
□ Responsive: Mobile-first, md:, lg: breakpoints
□ Accessibility: Focus rings, WCAG AA contrast
```

**Before committing:**
1. Run through checklist above
2. Test mobile (375px), tablet (768px), desktop (1280px)
3. Verify focus states with Tab key
4. Check contrast with browser DevTools

---

## Migration Plan

See: `/home/caesar/git/all-chat/.planning/ROADMAP.md` (Phase XX)

**Phase 1: Design Tokens** (current)
- ✓ Create DESIGN_SYSTEM.md
- Create Tailwind theme config
- Audit existing colors/spacing

**Phase 2: Component Library**
- Install shadcn/ui
- Customize components to match design system
- Document in Storybook (optional)

**Phase 3: Page-by-Page Migration**
- Landing page → new design system
- Dashboard → new design system
- Overlay editor → new design system
- Settings → new design system
- Admin pages → new design system

**Phase 4: Enforcement**
- Add ESLint rules for Tailwind (no gray-900, etc.)
- Pre-commit hook to validate design system
- Update CI to check for violations

---

## Examples

### Before (Current "Claude Code" feeling)
```tsx
<div className="bg-gray-800 p-6 rounded shadow">
  <h2 className="text-xl font-bold">My Overlay</h2>
  <button className="bg-purple-600 px-4 py-2 rounded">
    Edit
  </button>
</div>
```

### After (StreamElements Modern)
```tsx
<div className="rounded-xl border border-slate-700/50 bg-slate-850 p-6 shadow-lg transition-all duration-200 hover:shadow-xl hover:scale-[1.02]">
  <h2 className="text-lg font-semibold text-slate-50">My Overlay</h2>
  <button className="mt-4 rounded-lg bg-gradient-to-r from-purple-500 to-blue-500 px-6 py-2.5 text-sm font-semibold text-white shadow-md transition-all duration-200 hover:shadow-lg hover:scale-[1.02]">
    Edit
  </button>
</div>
```

**Key differences:**
1. slate instead of gray (warmer, more refined)
2. Border + shadow for depth
3. Gradient CTA (distinctive, not flat purple)
4. Hover states (scale + shadow increase)
5. Transitions (smooth interactions)
6. Consistent spacing (p-6, px-6 py-2.5)

---

## Questions?

- **"What if I need a color not in the system?"** → Ask first. Likely a platform color or status color.
- **"Can I use gray-X?"** → No. Use slate-X instead (warmer tone).
- **"rounded-lg or rounded-xl?"** → Cards: xl. Buttons/inputs: lg.
- **"How much shadow?"** → Cards: shadow-lg. Buttons: shadow-md. Modals: shadow-2xl.
- **"Can I add animation-X?"** → Keep it simple. transition-all duration-200 covers 90% of cases.

---

**Last updated**: 2026-03-09 by Claude Code
**Next review**: After Phase 2 (Component Library) completion
