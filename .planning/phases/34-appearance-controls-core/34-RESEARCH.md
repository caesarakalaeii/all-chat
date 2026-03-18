# Phase 34: Appearance Controls — Core - Research

**Researched:** 2026-03-18
**Domain:** React UI controls (color picker, combobox, collapsible, sliders) + postMessage iframe communication
**Confidence:** HIGH

## Summary

Phase 34 implements the first three visual control groups (Typography, Colors, Background & Bubbles) inside an `AppearancePanel` component, wired to the preview iframe via postMessage. The foundation (VisualSettings type, `visualSettingsToCss` utility, cascade layer) was built in Phase 33 and is confirmed complete in the codebase.

The key external addition is `react-colorful` — it is NOT currently a direct project dependency (only a transitive dep from Storybook). It must be explicitly installed. All other needed building blocks are already present: `@base-ui/react` (v1.2.0) provides Collapsible and Combobox primitives; `SplitView` needs an `onIframeReady` callback wired; the embed page needs a postMessage listener.

The VisualSettings flat interface is confirmed complete in `visual-settings.ts` except for two fields called out in CONTEXT: `usernameFontFamily` and `timestampFontFamily` — these must be added to both the type AND the `PROPERTY_MAP` in `visual-settings-to-css.ts`. The `OverlayConfig` type in `overlay.ts` currently types `visual_settings` as `Record<string, unknown>` — it should be updated to import and use `Partial<VisualSettings>` for type safety.

**Primary recommendation:** Install `react-colorful`, add the two missing VisualSettings fields, then implement CollapsibleSection → AppearancePanel → TypographyGroup → ColorsGroup → BackgroundGroup in sequence across three plans.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Live Preview Wiring**
- postMessage from the editor to the iframe — `iframeRef.current.contentWindow.postMessage({ type: 'VISUAL_CSS_UPDATE', css }, '*')`
- No debounce — send on every control change (CSS generation is effectively free, instant feedback is better)
- Embed page (`/overlays/[id]/preview/embed/page.tsx`) adds a `window.addEventListener('message', ...)` listener that upserts a `<style id="visual-customizer-style">` tag with the received CSS
- SplitView gets an `onIframeReady?: (ref: HTMLIFrameElement) => void` prop; it calls the callback with the iframe element after mount
- `visualSettings` state lives in the overlay editor page component (`useState<VisualSettings>({})`), co-located with `customCss` and other editor state — passed as props down to `AppearancePanel` and groups
- On page load, `visual_settings` from the API response populates this state (same as `custom_css` is loaded today)

**Font Family Picker**
- Both system fonts + curated Google Fonts in a single grouped list
- Searchable combobox UI — type to filter by font name
- Separate per-element font pickers: body text (`fontFamily`), username (`usernameFontFamily`), timestamp (`timestampFontFamily`)
  - Note: check current VisualSettings type; `usernameFontFamily` and `timestampFontFamily` fields may need to be added
- System fonts: Inter, Arial, Helvetica, Georgia, Courier New, Impact, Trebuchet MS, Verdana, Tahoma
- Curated Google Fonts (streaming-style selection): Bebas Neue, Oswald, Rajdhani, Barlow Condensed, Exo 2, Nunito, Poppins, Roboto, Open Sans, Montserrat
- Google Fonts loaded via `<link>` in the embed page when a web font is selected (dynamic injection, not preloaded)
- Font options displayed: label shows font name rendered in that font (via `font-family` inline style on the option)

**Color Picker**
- Library: `react-colorful` — `HexColorPicker` for solid colors, small bundle, accessible, zero deps
- Text colors (message body, username, timestamp): solid hex only — no opacity slider
- Color + opacity controls (overlay background, bubble background): inline row — [color swatch button → opens `HexColorPicker` popover] + [opacity % slider (0–100)]
  - Swatch shows current color as a small colored square button
  - Popover opens/closes on click (not hover); closes on click-outside
  - Opacity stored as separate `overlayBgOpacity` / `bubbleBgOpacity` string fields (e.g. `"0.7"`)
- Color values stored as hex strings (e.g. `"#1a1a2e"`); opacity as decimal string (e.g. `"0.85"`)

**CollapsibleSection Component**
- Built on `@base-ui/react` Collapsible primitive
- All groups start collapsed by default — user expands what they want
- State persisted in localStorage — key: `appearance-panel-sections-v1`, stores object of `{ [groupId]: boolean }`
- Smooth open/close animation via CSS (height transition using `@base-ui/react` `Collapsible.Panel` data attributes)
- `CollapsibleSection` props: `id: string`, `title: string`, `children: React.ReactNode`

**AppearancePanel Structure**
- Hosts all three groups in Phase 34
- Each group wrapped in CollapsibleSection
- Order: Typography → Colors → Background & Bubbles
- Props: `visualSettings: VisualSettings`, `onChange: (patch: Partial<VisualSettings>) => void`
  - Parent merges patch: `setVisualSettings(prev => ({ ...prev, ...patch }))`

**TypographyGroup Controls**
- Font family picker (combobox) × 3: body text, username, timestamp
- Font weight: dropdown select (100 Thin → 900 Black, common weights)
- Font size: number input + px label (range 10–32px) for body text
- Username font size, timestamp font size: separate number inputs
- Line height: slider (1.0–2.5, step 0.1)
- Letter spacing: slider (-2px to 8px, step 0.5px)

**ColorsGroup Controls**
- Message body color: `HexColorPicker` (solid, no opacity)
- Username color: `HexColorPicker` (solid, no opacity)
- Timestamp color: `HexColorPicker` (solid, no opacity)

**BackgroundGroup Controls**
- Overlay background: color swatch + opacity slider
- Bubble background: color swatch + opacity slider
- Bubble border radius: slider (0–24px, step 1)
- Bubble border width: slider (0–8px, step 1)
- Bubble border color: `HexColorPicker` (solid)
- Bubble padding: slider (0–32px, step 2px)
- Message gap: slider (0–24px, step 2px)
- Backdrop blur: slider (0–20px, step 1px)

### Claude's Discretion
- Exact slider thumb and track styling (within streaming dark aesthetic)
- Popover positioning (above/below color swatch) and z-index management
- Whether to use a `useColorPicker` hook or inline the popover open/close state per control
- Exact localStorage serialization/deserialization for section state

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `@base-ui/react` | ^1.2.0 | Collapsible, Combobox, Select, Button, Input primitives | Already installed, used throughout project |
| `react-colorful` | ^5.6.1 | `HexColorPicker` color picker component | Locked decision; zero deps, small bundle (~3KB gzip) |
| Tailwind CSS v4 | ^4.1.18 | Styling with design tokens | Project standard |
| `lucide-react` | ^0.563.0 | Chevron icons for collapsible trigger | Already installed |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `clsx` | ^2.1.1 | Conditional class names | Available, already used everywhere |
| `class-variance-authority` | ^0.7.1 | Component variants | Available, used in Button/Input |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| react-colorful | Native `<input type="color">` | Native is simpler but no hex text input, no popover control, poor styling |
| @base-ui/react Combobox | Native `<select>` | Native can't render font previews in option labels, no search |
| @base-ui/react Collapsible | Custom accordion | Primitive handles a11y (aria-expanded, keyboard) automatically |

**Installation:**
```bash
npm install react-colorful
```
(All other deps already in package.json)

## Architecture Patterns

### Recommended Project Structure
```
frontend/src/
├── components/
│   ├── appearance/                   # NEW — all Phase 34 components
│   │   ├── AppearancePanel.tsx       # Host component, owns no state
│   │   ├── CollapsibleSection.tsx    # Reusable collapsible wrapper
│   │   ├── TypographyGroup.tsx       # Typography controls
│   │   ├── ColorsGroup.tsx           # Color picker controls
│   │   ├── BackgroundGroup.tsx       # Background & bubble controls
│   │   ├── FontFamilyCombobox.tsx    # Reusable font picker (used ×3 in Typography)
│   │   ├── ColorPickerControl.tsx    # Reusable hex color + optional opacity row
│   │   └── SliderControl.tsx         # Reusable labeled slider row
├── lib/types/
│   └── visual-settings.ts            # EXTEND: add usernameFontFamily, timestampFontFamily
├── lib/utils/
│   └── visual-settings-to-css.ts    # EXTEND: add those two fields to PROPERTY_MAP
├── lib/types/
│   └── overlay.ts                    # UPDATE: visual_settings?: Partial<VisualSettings>
├── components/
│   └── SplitView.tsx                 # EXTEND: add onIframeReady callback prop
└── app/overlays/[id]/
    ├── page.tsx                      # EXTEND: add visualSettings state, AppearancePanel, save wiring
    └── preview/embed/page.tsx        # EXTEND: add postMessage listener
```

### Pattern 1: CollapsibleSection with localStorage persistence
**What:** A wrapper that reads/writes open state to localStorage and delegates animation to @base-ui/react Collapsible.Panel data attributes.
**When to use:** Every appearance sub-group in AppearancePanel.
**Example:**
```typescript
// @base-ui/react Collapsible API (confirmed from node_modules type definitions)
// Root: defaultOpen, open, onOpenChange, disabled
// Trigger: renders <button>, inherits aria-expanded automatically
// Panel: keepMounted prop; exposes data-open / data-closed for CSS animation

import { Collapsible } from '@base-ui/react/collapsible'

function CollapsibleSection({ id, title, children }: {
  id: string
  title: string
  children: React.ReactNode
}) {
  const STORAGE_KEY = 'appearance-panel-sections-v1'
  const stored = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '{}') as Record<string, boolean>
  const [open, setOpen] = useState(stored[id] ?? false)

  const handleOpenChange = (next: boolean) => {
    setOpen(next)
    const current = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '{}') as Record<string, boolean>
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ ...current, [id]: next }))
  }

  return (
    <Collapsible.Root open={open} onOpenChange={handleOpenChange}>
      <Collapsible.Trigger className="flex w-full items-center justify-between px-4 py-3 text-sm font-medium text-text hover:text-text-sub">
        {title}
        <ChevronDown className="size-4 transition-transform data-[open]:rotate-180" />
      </Collapsible.Trigger>
      <Collapsible.Panel
        keepMounted
        className="overflow-hidden transition-[height] duration-200 data-[open]:animate-none"
      >
        <div className="px-4 pb-4">{children}</div>
      </Collapsible.Panel>
    </Collapsible.Root>
  )
}
```

### Pattern 2: postMessage CSS injection (iframe → editor)
**What:** Editor page sends generated CSS via postMessage; embed page upserts a dedicated `<style>` tag.
**When to use:** On every `visualSettings` change in the editor.
**Example:**
```typescript
// Editor page: send on every change
const sendCssToIframe = (settings: Partial<VisualSettings>) => {
  const css = visualSettingsToCss(settings)
  iframeRef.current?.contentWindow?.postMessage(
    { type: 'VISUAL_CSS_UPDATE', css },
    '*'
  )
}

// Embed page: receive and upsert style tag
useEffect(() => {
  const handler = (event: MessageEvent) => {
    if (event.data?.type !== 'VISUAL_CSS_UPDATE') return
    const css = event.data.css as string
    let styleEl = document.getElementById('visual-customizer-style') as HTMLStyleElement | null
    if (!styleEl) {
      styleEl = document.createElement('style')
      styleEl.id = 'visual-customizer-style'
      document.head.appendChild(styleEl)
    }
    styleEl.textContent = css
  }
  window.addEventListener('message', handler)
  return () => window.removeEventListener('message', handler)
}, [])
```

### Pattern 3: FontFamilyCombobox using @base-ui/react Combobox
**What:** Searchable grouped combobox with font name rendered in its own font for option preview.
**When to use:** Three instances in TypographyGroup (body, username, timestamp).
**Example:**
```typescript
// @base-ui/react Combobox API (confirmed from node_modules)
// Parts: Root, Input, Trigger, Popup, Positioner, List, Item, ItemIndicator, Group, GroupLabel, Empty
import { Combobox } from '@base-ui/react/combobox'

// Font option label rendered in its own font:
<Combobox.Item value={font.value} key={font.value}>
  <span style={{ fontFamily: font.value }}>{font.label}</span>
</Combobox.Item>

// Groups (System / Google Fonts):
<Combobox.Group>
  <Combobox.GroupLabel>System Fonts</Combobox.GroupLabel>
  {SYSTEM_FONTS.map(...)}
</Combobox.Group>
<Combobox.Group>
  <Combobox.GroupLabel>Google Fonts</Combobox.GroupLabel>
  {GOOGLE_FONTS.map(...)}
</Combobox.Group>
```

### Pattern 4: ColorPickerControl (hex + optional opacity)
**What:** Reusable row: colored swatch button → opens HexColorPicker popover + optional opacity slider.
**When to use:** All color fields in ColorsGroup and BackgroundGroup.
**Example:**
```typescript
import { HexColorPicker } from 'react-colorful'

function ColorPickerControl({
  label,
  value,
  onChange,
  showOpacity = false,
  opacity,
  onOpacityChange,
}: {
  label: string
  value: string
  onChange: (hex: string) => void
  showOpacity?: boolean
  opacity?: string
  onOpacityChange?: (opacity: string) => void
}) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  // Click-outside to close
  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open])

  return (
    <div className="flex items-center gap-3">
      <label className="text-sm text-text-sub">{label}</label>
      <div ref={ref} className="relative">
        <button
          onClick={() => setOpen(o => !o)}
          className="h-7 w-10 rounded border border-border"
          style={{ backgroundColor: value }}
        />
        {open && (
          <div className="absolute top-full left-0 z-50 mt-1 rounded-lg border border-border bg-surface p-2 shadow-lg">
            <HexColorPicker color={value} onChange={onChange} />
          </div>
        )}
      </div>
      {showOpacity && opacity !== undefined && onOpacityChange && (
        <input
          type="range"
          min={0}
          max={100}
          value={Math.round(parseFloat(opacity) * 100)}
          onChange={e => onOpacityChange(String(Number(e.target.value) / 100))}
          className="w-24"
        />
      )}
    </div>
  )
}
```

### Pattern 5: SliderControl (reusable labeled slider)
**What:** Row with label, range input, and current value display.
**When to use:** Line height, letter spacing, border radius, border width, padding, gap, blur.
```typescript
function SliderControl({
  label,
  value,
  min,
  max,
  step,
  unit = '',
  onChange,
}: {
  label: string
  value: number
  min: number
  max: number
  step: number
  unit?: string
  onChange: (v: number) => void
}) {
  return (
    <div className="flex items-center gap-3">
      <label className="w-28 shrink-0 text-sm text-text-sub">{label}</label>
      <input
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={e => onChange(Number(e.target.value))}
        className="flex-1"
      />
      <span className="w-12 text-right text-xs text-text-dim">
        {value}{unit}
      </span>
    </div>
  )
}
```

### Pattern 6: Google Fonts dynamic injection
**What:** When a Google Font is selected, inject a `<link>` into the embed page's `<head>` dynamically. Do not preload all fonts.
**When to use:** Inside embed page's postMessage handler or via a dedicated font-loading utility.
```typescript
// Called from the embed page when it receives a VISUAL_CSS_UPDATE with a font setting
const GOOGLE_FONT_NAMES = new Set(['Bebas Neue', 'Oswald', 'Rajdhani', ...])

function ensureGoogleFontLoaded(fontFamily: string) {
  if (!GOOGLE_FONT_NAMES.has(fontFamily)) return // system font, skip
  const id = `gfont-${fontFamily.replace(/\s+/g, '-')}`
  if (document.getElementById(id)) return // already loaded
  const link = document.createElement('link')
  link.id = id
  link.rel = 'stylesheet'
  link.href = `https://fonts.googleapis.com/css2?family=${encodeURIComponent(fontFamily)}:wght@400;600;700&display=swap`
  document.head.appendChild(link)
}
```

### Anti-Patterns to Avoid
- **Importing `VisualSettings` as a nested type:** Phase 33 CONTEXT proposed nested, but the actual shipped code uses a FLAT interface. Use the flat interface from `visual-settings.ts` — do not restructure.
- **Debouncing postMessage:** CONTEXT explicitly says no debounce — CSS generation is free.
- **Preloading all Google Fonts:** Only load a font when the user actually selects it.
- **Global `<style>` element ID collisions:** The visual-customizer style tag must use id `visual-customizer-style` — distinct from the existing `overlay-preview-custom-css` style tag already in the embed page.
- **Modifying `overlay.ts` visual_settings type to `VisualSettings` instead of `Partial<VisualSettings>`:** All fields are optional, so Partial is correct.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Color picker UI | Custom color wheel/hex inputs from scratch | `react-colorful` HexColorPicker | Handles pointer events, touch, keyboard, accessibility |
| Collapsible with a11y | `<details>/<summary>` or CSS-only toggle | `@base-ui/react` Collapsible | aria-expanded, keyboard, animation data attributes |
| Searchable font dropdown | Custom dropdown with filtering | `@base-ui/react` Combobox | Built-in filtering, keyboard nav, accessibility |
| Click-outside detection for popover | Custom global listener | `useEffect` with `document.addEventListener('mousedown', ...)` using a ref | Simple, already an established pattern |
| Font preview in options | `<canvas>` or server-side render | Inline `style={{ fontFamily }}` on option label | Native CSS font rendering |

**Key insight:** All primitives needed are already available — the implementation is wiring known parts together, not building infrastructure.

## Common Pitfalls

### Pitfall 1: VisualSettings type mismatch (flat vs. nested)
**What goes wrong:** Phase 33 CONTEXT described a nested VisualSettings interface, but the actual shipped code (`visual-settings.ts`) uses a flat interface. Using the nested shape in Phase 34 components would cause type errors.
**Why it happens:** The plan was revised during Phase 33 implementation.
**How to avoid:** Always import directly from `@/lib/types/visual-settings` and use the actual flat shape.
**Warning signs:** TypeScript errors when accessing `settings.typography?.fontFamily` — correct form is `settings.fontFamily`.

### Pitfall 2: Missing usernameFontFamily / timestampFontFamily fields
**What goes wrong:** TypographyGroup needs separate font pickers for username and timestamp, but these fields don't exist yet in VisualSettings or PROPERTY_MAP.
**Why it happens:** The existing type covers most fields but was not extended for per-element font families.
**How to avoid:** Plan 34-01 MUST add these two fields to `visual-settings.ts` AND to `PROPERTY_MAP` in `visual-settings-to-css.ts` before implementing TypographyGroup.
**Warning signs:** TypeScript "does not exist on type VisualSettings" errors.

### Pitfall 3: overlay.ts visual_settings type is too loose
**What goes wrong:** `visual_settings?: Record<string, unknown>` in `OverlayConfig` provides no type checking when reading visual settings from API responses.
**Why it happens:** Phase 33 used `Record<string, unknown>` as a safe passthrough.
**How to avoid:** Update `overlay.ts` to import `VisualSettings` and type `visual_settings?: Partial<VisualSettings>`.

### Pitfall 4: iframe ref not available on first render
**What goes wrong:** Editor page tries to postMessage to `iframeRef.current` before SplitView has mounted and provided the ref.
**Why it happens:** SplitView renders the iframe internally; parent doesn't have direct access until after mount.
**How to avoid:** Use the `onIframeReady` callback pattern — only attempt postMessage when the callback fires. Guard all postMessage calls: `iframeRef.current?.contentWindow?.postMessage(...)`.

### Pitfall 5: react-colorful not installed as direct dependency
**What goes wrong:** `import { HexColorPicker } from 'react-colorful'` appears to work (transitive dep from Storybook), but will break in production builds that tree-shake differently, or when Storybook version changes.
**Why it happens:** react-colorful is a devDependency of @storybook/addon-docs, not a direct project dep.
**How to avoid:** Run `npm install react-colorful` before implementing ColorsGroup. Verify it appears in `package.json` `dependencies` (not devDependencies).

### Pitfall 6: Combobox value type for font family
**What goes wrong:** Combobox.Root `value` is typed generically — passing `string | undefined` when the type expects `string | null` causes type errors or silent failures.
**Why it happens:** Base UI Combobox has nullable typed value.
**How to avoid:** Pass `value ?? null` when binding to `visualSettings.fontFamily` (which can be undefined).

### Pitfall 7: CSS animation for Collapsible.Panel height transition
**What goes wrong:** Height transitions from `0` to `auto` don't work with plain CSS `transition: height`. The panel snaps open.
**Why it happens:** CSS cannot transition to `height: auto`.
**How to avoid:** Use the `@base-ui/react` Collapsible.Panel built-in height animation via data attributes. The panel exposes `data-open` and `data-closed` — pair with CSS custom property `--collapsible-panel-height` which the component manages. Or use `max-height` trick as fallback.

### Pitfall 8: localStorage access during SSR
**What goes wrong:** CollapsibleSection reads localStorage during render, causing hydration mismatch or SSR errors.
**Why it happens:** localStorage is not available server-side.
**How to avoid:** Read localStorage inside `useState` initializer (lazy init) or inside a `useEffect`. Since this is a 'use client' component, lazy initializer is safe: `useState(() => JSON.parse(localStorage.getItem(KEY) ?? '{}'))`.

## Code Examples

Verified patterns from existing codebase:

### Existing handleSaveConfiguration pattern (to extend for visual_settings)
```typescript
// Current (from frontend/src/app/overlays/[id]/page.tsx:1260)
await overlaysApi.updateConfig(id, {
  display_settings: { font_size: fontSize, ... },
  custom_css: useCustomCss ? customCss : '',
})

// Extended (Phase 34):
await overlaysApi.updateConfig(id, {
  display_settings: { font_size: fontSize, ... },
  custom_css: useCustomCss ? customCss : '',
  visual_settings: visualSettings,  // add this
})
```

### Existing custom CSS style injection pattern in embed page (reference for visual-customizer style tag)
```typescript
// Current (frontend/src/app/overlays/[id]/preview/embed/page.tsx:354)
{useCustomCss && scopedPreviewCss && (
  <style
    key={scopedPreviewCss}
    id="overlay-preview-custom-css"
    dangerouslySetInnerHTML={{ __html: scopedPreviewCss }}
  />
)}

// New parallel tag (postMessage-managed, id must differ):
// <style id="visual-customizer-style"> — managed imperatively via DOM, not React state
```

### SplitView iframe ref (current SplitView.tsx — iframe is internal)
```typescript
// Current: iframe is rendered inside SplitView with no external ref
<iframe
  src={`/overlays/${overlayId}/preview/embed`}
  className="h-full w-full border-0"
  title="Overlay live preview"
  sandbox="allow-scripts allow-same-origin"
/>

// Phase 34 adds: onIframeReady callback — SplitView calls it with ref after mount
// Parent stores it: const iframeRef = useRef<HTMLIFrameElement | null>(null)
// SplitView: <iframe ref={el => { if (el) onIframeReady?.(el) }} ... />
```

### @base-ui/react Collapsible.Root confirmed props
```typescript
// From node_modules type definitions (HIGH confidence)
// open?: boolean | undefined
// defaultOpen?: boolean | undefined
// onOpenChange?: (open: boolean, eventDetails) => void
// disabled?: boolean | undefined
```

### @base-ui/react Select — available for font weight dropdown
```typescript
// @base-ui/react/select is available (confirmed in node_modules)
// Parts: Root, Trigger, Value, Icon, Portal, Backdrop, Positioner, Popup, List, Item, ItemIndicator, ItemText
// Font weight dropdown is a natural use case: fixed list of 7-8 values, no search needed
import { Select } from '@base-ui/react/select'
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Tailwind v3 `@layer` | Tailwind v4 `@layer` with cascade layer support | Phase 33 | `@layer visual-customizer` is now declared in globals.css |
| `OverlayConfig.visual_settings: undefined` | `visual_settings?: Record<string, unknown>` | Phase 33 | Passthrough exists; needs tightening to `Partial<VisualSettings>` |

**Deprecated/outdated:**
- Phase 33 CONTEXT described a nested VisualSettings interface — this was NOT implemented. Actual code uses flat interface.

## Open Questions

1. **Collapsible.Panel height animation specifics**
   - What we know: @base-ui/react Collapsible.Panel has `keepMounted` prop and exposes data attributes; docs mention CSS custom property `--collapsible-panel-height`
   - What's unclear: Exact CSS property name is not confirmed from local type definitions (only from base-ui.com docs which weren't fetched)
   - Recommendation: Use `max-height` transition as safe fallback; or start with no animation and add it if the data attribute CSS variable approach is discovered to work during implementation

2. **Combobox filtering behavior**
   - What we know: @base-ui/react Combobox has `autoHighlight` prop and built-in input filtering
   - What's unclear: Whether filtering is automatic (built-in) or requires providing a filtered items list
   - Recommendation: Test during Plan 34-01 implementation; if not automatic, implement simple string.includes filter in state

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Vitest 4.x |
| Config file | `frontend/vitest.config.ts` |
| Quick run command | `cd frontend && npx vitest run --project unit src/components/appearance` |
| Full suite command | `cd frontend && npx vitest run --project unit` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| APPR-01 | Typography controls render and call onChange with correct VisualSettings patch | unit | `cd frontend && npx vitest run --project unit src/components/appearance/__tests__` | ❌ Wave 0 |
| APPR-02 | Color controls render and call onChange with hex string values | unit | `cd frontend && npx vitest run --project unit src/components/appearance/__tests__` | ❌ Wave 0 |
| APPR-03 | Overlay background color + opacity controls render and emit correct patch | unit | `cd frontend && npx vitest run --project unit src/components/appearance/__tests__` | ❌ Wave 0 |
| APPR-04 | Bubble controls (border radius, width, color, padding, gap) render and emit correct patch | unit | `cd frontend && npx vitest run --project unit src/components/appearance/__tests__` | ❌ Wave 0 |
| APPR-08 | Backdrop blur slider renders and emits correct string value with 'px' unit | unit | `cd frontend && npx vitest run --project unit src/components/appearance/__tests__` | ❌ Wave 0 |
| CollapsibleSection | localStorage state persistence across open/close | unit | `cd frontend && npx vitest run --project unit src/components/appearance/__tests__/CollapsibleSection.test.ts` | ❌ Wave 0 |
| visualSettingsToCss extension | usernameFontFamily and timestampFontFamily emit correct CSS vars | unit | `cd frontend && npx vitest run --project unit src/lib/utils/__tests__/visual-settings-to-css.test.ts` | ✅ (extend existing) |

### Sampling Rate
- **Per task commit:** `cd frontend && npx vitest run --project unit src/lib/utils/__tests__/visual-settings-to-css.test.ts`
- **Per wave merge:** `cd frontend && npx vitest run --project unit`
- **Phase gate:** Full unit suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `frontend/src/components/appearance/__tests__/CollapsibleSection.test.tsx` — covers CollapsibleSection localStorage persistence, open/close behavior
- [ ] `frontend/src/components/appearance/__tests__/TypographyGroup.test.tsx` — covers APPR-01
- [ ] `frontend/src/components/appearance/__tests__/ColorsGroup.test.tsx` — covers APPR-02
- [ ] `frontend/src/components/appearance/__tests__/BackgroundGroup.test.tsx` — covers APPR-03, APPR-04, APPR-08
- [ ] Extend `frontend/src/lib/utils/__tests__/visual-settings-to-css.test.ts` — add cases for `usernameFontFamily`, `timestampFontFamily`

## Sources

### Primary (HIGH confidence)
- Local node_modules `@base-ui/react` type definitions — Collapsible, Combobox, Select APIs confirmed
- `frontend/src/lib/types/visual-settings.ts` — confirmed flat VisualSettings interface, confirmed missing fields
- `frontend/src/lib/utils/visual-settings-to-css.ts` — confirmed PROPERTY_MAP structure
- `frontend/src/components/SplitView.tsx` — confirmed no onIframeReady prop exists yet
- `frontend/src/app/overlays/[id]/preview/embed/page.tsx` — confirmed no postMessage listener exists yet
- `frontend/src/app/overlays/[id]/page.tsx` — confirmed handleSaveConfiguration does not include visual_settings
- `frontend/package.json` — confirmed react-colorful is NOT a direct dependency

### Secondary (MEDIUM confidence)
- `frontend/node_modules/react-colorful/package.json` — not present as direct dep, only transitively via Storybook (verified by filesystem search)

### Tertiary (LOW confidence)
- @base-ui/react Collapsible.Panel CSS custom property for height animation — not verified from local types, only from documentation memory

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries verified in package.json or node_modules
- Architecture: HIGH — patterns derived directly from existing codebase code reading
- Pitfalls: HIGH — all pitfalls identified from direct code inspection (type files, page.tsx, package.json)

**Research date:** 2026-03-18
**Valid until:** 2026-04-18 (stable libraries, no fast-moving parts)
