# Phase 36: Events Styling + Theme Import - Research

**Researched:** 2026-03-18
**Domain:** React/TypeScript frontend — appearance controls, CSS custom property parsing, theme import UX
**Confidence:** HIGH

## Summary

Phase 36 is a pure frontend phase that extends the existing appearance panel system built in phases 33–35. It adds a final control group (`EventsGroup`), a new utility (`theme-css-parser.ts`), and wires theme loading to pre-populate controls. All infrastructure is in place: the `VisualSettings` type, `PROPERTY_MAP`, `visualSettingsToCss`, `AppearancePanel`, `handleVisualSettingsChange`, `sendCssToIframe`, and the embed page's `VISUAL_CSS_UPDATE` postMessage listener.

The critical discovery is that `VisualSettings` already has all five event visibility fields (`showSuperChat`, `showSubscriptions`, `showRaids`, `showBits`, `showMembershipGift`) and four of the five size modifier fields. Only `membershipGiftSizeModifier` is missing — it must be added to both the type and `PROPERTY_MAP`. The `visual-settings-to-css.test.ts` full-settings test counts 49 properties, so adding one more means the test must be updated to expect 50.

The overlay editor (`/overlays/[id]/page.tsx`) already has `visualSettings` state, `handleVisualSettingsChange`, `sendCssToIframe`, `parsedThemeSettings` (not yet — must add), and the `ThemeMarketplaceModal` `onApplyTheme` callback wired. The modal's `onApplyTheme` prop is typed `(css: string) => void` — it must be extended to `(css: string, parsedSettings: Partial<VisualSettings>) => void`. Theme parsing will happen in the editor page, not in the modal (modal stays dumb).

The events.css file already uses `.event-message` with `transform: scale(1.05)` in the `@layer marketplace-themes` block. The visual-customizer layer sits above marketplace-themes in the cascade, so `--chat-*-size-modifier` vars emitted there will correctly override the baseline scale. New per-event-type CSS rules using `scale(var(--chat-super-chat-size-modifier, 1))` (etc.) must be added to events.css inside a `@layer visual-customizer` block.

**Primary recommendation:** Build incrementally in three plans: EventsGroup component + CSS wiring → theme-css-parser utility with tests → theme import integration + Reset button.

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Events scope:**
- All 5 event types: SuperChat, Subscriptions, Raids, Bits, MembershipGift
- Each gets both a show/hide toggle and a size modifier slider
- `membershipGiftSizeModifier` field must be added to `VisualSettings` and `PROPERTY_MAP` — CSS var: `--chat-membership-gift-size-modifier`
- Section label: "Events"
- Order: SuperChat → Subscriptions → Raids → Bits → Membership Gift

**Size modifier behavior:**
- CSS `transform: scale(X)` applied to `.event-message` element via CSS var
- Range: 0.5–3.0×, step 0.1
- Per-event-type sliders (separate field per type)
- When field is undefined: no CSS var emitted → cascade default applies (existing `transform: scale(1.05)` stays)
- Slider display: value shown as `1.5×` (number with × suffix)

**Theme import merge strategy:**
- `onApplyTheme(css: string)` extended to `onApplyTheme(css: string, parsedSettings: Partial<VisualSettings>)`
- Same atomic action sets both `customCss` and `visualSettings`
- If visualSettings is non-empty: show confirm dialog before replacing
  - Dialog: "Loading this theme will reset your visual customizations. Continue?"
  - Confirm: replace visualSettings with parsedSettings; store as `parsedThemeSettings`
  - Cancel: do nothing — neither CSS nor settings applied
- If visualSettings is empty: apply immediately without prompt
- Unknown CSS vars: `console.warn` in dev, silently ignored in behavior
- Opacity pairs parsed independently as separate fields

**theme-css-parser.ts:**
- Location: `frontend/src/lib/utils/theme-css-parser.ts`
- Signature: `parseCssToVisualSettings(css: string): Partial<VisualSettings>`
- Algorithm: regex/string scan for `--chat-*` and `--platform-*` declarations
- Map known vars using reverse of `PROPERTY_MAP`
- Skip unknowns with `console.warn`
- Returns only fields found (never sets field to undefined explicitly)
- Unit tests: full theme CSS → all matched, unknown ignored + warned

**Reset to theme defaults:**
- Button location: near theme selector, below/adjacent to "Open Theme Marketplace" button
- Active theme: clicking Reset sets `visualSettings = parsedThemeSettings`
- No theme: clicking Reset sets `visualSettings = {}`
- `parsedThemeSettings` stored in editor page state, initialized `{}`, updated only on theme apply

**APPR-10 verification:**
- Every EventsGroup control change must trigger postMessage to iframe
- Same postMessage pattern as existing groups — no new wiring
- No special casing needed since all vars go through `visualSettingsToCss`

### Claude's Discretion

- Exact dialog component used for confirm prompt (reuse existing Dialog from @base-ui/react)
- Regex pattern for CSS var extraction in `theme-css-parser.ts`
- EventsGroup row layout (match VisibilityGroup rows for consistency)
- Whether "Reset to theme" renders as a button or a text link

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| APPR-09 | User can customize special event styling: show/hide, size modifier for Super Chat, subscriptions, raids | EventsGroup component reusing ToggleSwitch + SliderControl; 5 event types; CSS vars already defined in PROPERTY_MAP (except membershipGiftSizeModifier) |
| APPR-10 | All visual control changes update live overlay preview in real-time without save | postMessage pattern via `sendCssToIframe` already wired in editor page; embed page listener handles all CSS vars generically; EventsGroup follows the same onChange pattern |
| VISM-02 | Loading a marketplace theme pre-populates visual controls with that theme's CSS variable values | `parseCssToVisualSettings` utility parses CSS string → `Partial<VisualSettings>`; editor page extends `onApplyTheme` handler to parse and merge |
| VISM-04 | Resetting visual customizations restores theme defaults (or system defaults if no theme loaded) | `parsedThemeSettings` state snapshot; Reset button sets `visualSettings = parsedThemeSettings` or `{}` |
</phase_requirements>

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| React | 18+ | Component rendering | Project standard |
| TypeScript | 5+ | Type safety | Project-wide, no `any` |
| Vitest | detected in vitest.config.ts | Unit testing | Already configured for unit + Storybook projects |
| @testing-library/react | detected in existing tests | Component test rendering | Used in all existing appearance group tests |
| @base-ui/react | v1.2.0+ | Dialog component | Used in project's `ui/dialog.tsx` wrapper |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| lucide-react | project standard | Icons (RotateCcw for Reset button) | If "Reset to theme" rendered as button with icon |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Regex CSS parsing | postcss / css-tree | Full parsers are correct but heavy; regex is sufficient for `--chat-*: value;` declarations which are simple property:value pairs |

**Installation:** No new packages required. All dependencies are already present.

---

## Architecture Patterns

### Recommended Project Structure

```
frontend/src/
├── lib/
│   ├── types/visual-settings.ts          # Add membershipGiftSizeModifier field
│   └── utils/
│       ├── visual-settings-to-css.ts     # Add membershipGiftSizeModifier to PROPERTY_MAP
│       ├── theme-css-parser.ts           # NEW: parseCssToVisualSettings utility
│       └── __tests__/
│           ├── visual-settings-to-css.test.ts  # Update property count (49→50)
│           └── theme-css-parser.test.ts         # NEW: unit tests
├── components/appearance/
│   ├── EventsGroup.tsx                   # NEW: 5 event types × (toggle + slider)
│   ├── AppearancePanel.tsx               # Add EventsGroup as last section
│   └── __tests__/
│       └── EventsGroup.test.tsx          # NEW: component tests
├── styles/
│   └── events.css                        # Add visual-customizer layer rules for size modifiers
└── app/overlays/[id]/
    └── page.tsx                          # Add parsedThemeSettings state, extend onApplyTheme, add Reset button
```

### Pattern 1: EventsGroup Component Structure

**What:** One row per event type containing a ToggleSwitch and a SliderControl, grouped under a section label.

**When to use:** When an event type has both a boolean (show/hide) and a numeric (size modifier) control.

**Example — based on VisibilityGroup + SizingGroup established patterns:**

```typescript
// EventsGroup.tsx structure
'use client'

import React from 'react'
import type { VisualSettings } from '@/lib/types/visual-settings'
import { ToggleSwitch } from './ToggleSwitch'
import { SliderControl } from './SliderControl'

// Each event type maps a showField and a sizeField
const EVENT_ROWS: Array<{
  label: string
  showField: keyof VisualSettings
  sizeField: keyof VisualSettings
}> = [
  { label: 'Super Chat',      showField: 'showSuperChat',       sizeField: 'superChatSizeModifier' },
  { label: 'Subscriptions',   showField: 'showSubscriptions',   sizeField: 'subscriptionSizeModifier' },
  { label: 'Raids',           showField: 'showRaids',           sizeField: 'raidSizeModifier' },
  { label: 'Bits',            showField: 'showBits',            sizeField: 'bitsSizeModifier' },
  { label: 'Membership Gift', showField: 'showMembershipGift',  sizeField: 'membershipGiftSizeModifier' },
]
```

Key details:
- Toggle `checked`: `(value !== 'none')`, defaulting to `true` when undefined
- Toggle `onChange`: emit `{ [showField]: checked ? 'block' : 'none' }`
- Slider `value`: `parseFloat(visualSettings[sizeField] ?? '1')`
- Slider `onChange`: emit `{ [sizeField]: \`${v}\` }` (unitless string, no px)
- Slider props: `min={0.5}`, `max={3.0}`, `step={0.1}`, `unit="×"`

### Pattern 2: CSS var extraction in theme-css-parser.ts

**What:** Scan a CSS string for `--chat-*` and `--platform-*` custom property declarations, reverse-map to VisualSettings fields.

**When to use:** When a marketplace theme CSS is loaded and must pre-populate controls.

**Algorithm:**

```typescript
// Source: derived from PROPERTY_MAP in visual-settings-to-css.ts (reversed)
const REVERSE_MAP: ReadonlyMap<string, keyof VisualSettings> = new Map(
  PROPERTY_MAP.map(([field, cssVar]) => [cssVar, field])
)

// Regex: match `--chat-xxx: value;` or `--platform-xxx: value;`
// Handles optional whitespace around colon and semicolon
const CSS_VAR_REGEX = /(--(chat|platform)-[\w-]+)\s*:\s*([^;}\n]+?)\s*;/g

export function parseCssToVisualSettings(css: string): Partial<VisualSettings> {
  const result: Partial<VisualSettings> = {}
  let match: RegExpExecArray | null
  while ((match = CSS_VAR_REGEX.exec(css)) !== null) {
    const cssVar = match[1]
    const value = match[3].trim()
    const field = REVERSE_MAP.get(cssVar)
    if (field !== undefined) {
      // Type-safe assignment: value is always string; VisualSettings fields are string | 'inline' | 'block' | 'none'
      // Caller gets Partial<VisualSettings> — no runtime validation needed for planner's intent
      ;(result as Record<string, string>)[field] = value
    } else {
      if (process.env.NODE_ENV !== 'production') {
        console.warn(`[theme-css-parser] Unknown CSS variable: ${cssVar}`)
      }
    }
  }
  return result
}
```

Note on regex: The `g` flag with `exec()` mutates `lastIndex` — reset regex before each call or use a new instance per call. Safest: define regex inside the function or reset `lastIndex = 0`.

### Pattern 3: Confirm Dialog for theme import

**What:** Controlled dialog using the project's existing `Dialog` component from `@/components/ui/dialog`.

**When to use:** When visualSettings is non-empty and user clicks a theme's Apply button.

**Example pattern — how Dialog is used in the project:**

```typescript
// From /overlays/[id]/page.tsx (existing Dialog usage)
import { Dialog } from '@/components/ui/dialog'

// Controlled by boolean state
const [showThemeConfirm, setShowThemeConfirm] = useState(false)
const [pendingTheme, setPendingTheme] = useState<{ css: string; parsed: Partial<VisualSettings> } | null>(null)

// onApplyTheme handler
function handleApplyTheme(css: string, parsedSettings: Partial<VisualSettings>) {
  const hasExisting = Object.keys(visualSettings).length > 0
  if (hasExisting) {
    setPendingTheme({ css, parsed: parsedSettings })
    setShowThemeConfirm(true)
  } else {
    applyThemeImmediately(css, parsedSettings)
  }
}

// In JSX:
<Dialog.Root open={showThemeConfirm} onOpenChange={setShowThemeConfirm}>
  <Dialog.Content size="sm" showCloseButton={false}>
    <Dialog.Title>Apply theme?</Dialog.Title>
    <Dialog.Description>
      Loading this theme will reset your visual customizations. Continue?
    </Dialog.Description>
    {/* Confirm / Cancel buttons */}
  </Dialog.Content>
</Dialog.Root>
```

### Pattern 4: parsedThemeSettings state and Reset button

**What:** A second state variable in the editor page that shadows the last parsed theme settings.

```typescript
// In overlay editor page state initialization
const [parsedThemeSettings, setParsedThemeSettings] = useState<Partial<VisualSettings>>({})

// On theme apply (confirmed):
function applyThemeImmediately(css: string, parsed: Partial<VisualSettings>) {
  setCustomCss(css)
  setUseCustomCss(true)
  setVisualSettings(parsed)
  setParsedThemeSettings(parsed)
  sendCssToIframe(parsed)
  setShowThemeMarketplace(false)
}

// Reset handler:
function handleResetToTheme() {
  setVisualSettings(parsedThemeSettings)  // {} if no theme loaded
  sendCssToIframe(parsedThemeSettings)
}
```

Reset button placement: near "Browse Themes" button in the Custom CSS card section (section 7 of the editor panel).

### Pattern 5: events.css — visual-customizer layer rules for size modifiers

**What:** Add per-event-type rules in a `@layer visual-customizer` block to apply scale transforms from CSS vars.

```css
/* Add to frontend/src/styles/events.css */
@layer visual-customizer {
  .event-message[class*='super_chat'] {
    transform: scale(var(--chat-super-chat-size-modifier, 1.05));
  }
  .event-message[class*='subscription'],
  .event-message[class*='gift_subscription'] {
    transform: scale(var(--chat-subscription-size-modifier, 1.05));
  }
  .event-message[class*='raid'] {
    transform: scale(var(--chat-raid-size-modifier, 1.05));
  }
  .event-message[class*='bits'] {
    transform: scale(var(--chat-bits-size-modifier, 1.05));
  }
  .event-message[class*='member'],
  .event-message[class*='gift'] {
    transform: scale(var(--chat-membership-gift-size-modifier, 1.05));
  }
}
```

The `@layer visual-customizer` block above marketplace-themes in the cascade (cascade order declared at top of events.css: `base, design-system, marketplace-themes, visual-customizer, user-overrides`). This means the visual-customizer scale vars override the baseline `transform: scale(1.05)` set in `@layer marketplace-themes`. When no var is set, the `1.05` fallback matches existing behavior.

**Caution:** The base `.event-message` rule in `@layer marketplace-themes` already sets `transform: scale(1.05)`. If the visual-customizer layer is added without a matching selector for all `.event-message`, the base rule still applies for event types that don't match the new selectors. The per-type selectors above are correct: they match events by their class attribute content (attribute selector `[class*='type']`).

### Anti-Patterns to Avoid

- **Parsing in the modal:** The modal is kept dumb. Parse CSS in the editor page handler. Changing modal internals is risky and breaks the single-responsibility principle.
- **Setting `membershipGiftSizeModifier: undefined` explicitly in parsedSettings:** The parser must only set fields it finds. Explicit `undefined` would get included in spreads.
- **Mutating CSS var regex with `g` flag across calls:** If `CSS_VAR_REGEX` is a module-level constant with `g` flag, `exec()` will have stale `lastIndex` on second call. Define inside the function or reset `lastIndex` to 0 at start.
- **Using `transform` inside `@layer marketplace-themes` for the new size modifier selectors:** Must go in `@layer visual-customizer` so the cascade override works correctly.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Toggle switch | Custom button/checkbox | `ToggleSwitch` from `@/components/appearance/ToggleSwitch` | Already styled, correct aria, proven in 6+ groups |
| Labeled slider | Custom range | `SliderControl` from `@/components/appearance/SliderControl` | Same pattern used by SizingGroup and others |
| Collapsible section | Custom accordion | `CollapsibleSection` from `@/components/appearance/CollapsibleSection` | Handles localStorage persistence, base-ui Collapsible |
| Confirm dialog | Window.confirm / custom modal | `Dialog` from `@/components/ui/dialog` | Wraps @base-ui/react/dialog with project styling |

**Key insight:** All UI primitives are already implemented and tested. EventsGroup is an assembly of existing atoms.

---

## Common Pitfalls

### Pitfall 1: Forgetting to update the `visual-settings-to-css.test.ts` property count

**What goes wrong:** The full-settings test at line 89 asserts `(result.match(/--chat-|--platform-/g) ?? []).length` equals 49. Adding `membershipGiftSizeModifier` makes it 50. Test fails silently if count check is off.

**Why it happens:** The test hardcodes the expected count as a regression guard.

**How to avoid:** Update the expected count to 50 when adding `membershipGiftSizeModifier` to `PROPERTY_MAP`. Also add the field to the `Required<VisualSettings>` object in that test.

**Warning signs:** Vitest reports `expect(received).toBe(49)` with actual `50`.

### Pitfall 2: CSS var regex `lastIndex` stale state

**What goes wrong:** `parseCssToVisualSettings` called on a second CSS string returns no matches because `CSS_VAR_REGEX.lastIndex` was not reset between calls.

**Why it happens:** Module-level `const CSS_VAR_REGEX = /.../g` — global flag means `exec()` continues from last `lastIndex`. If defined at module scope, it persists across function calls.

**How to avoid:** Either define the regex inside the function body (new RegExp instance per call), or reset `CSS_VAR_REGEX.lastIndex = 0` at the start of each `parseCssToVisualSettings` call. Recommend: define inside the function for clarity.

**Warning signs:** First call to `parseCssToVisualSettings` works; second call returns `{}`.

### Pitfall 3: Theme confirm dialog — applying CSS but not settings (or vice versa)

**What goes wrong:** The confirm dialog is atomic — on cancel, neither `customCss` nor `visualSettings` should change. On confirm, both must change atomically.

**Why it happens:** Async state updates or missing `setParsedThemeSettings` call.

**How to avoid:** All three state updates (`setCustomCss`, `setVisualSettings`, `setParsedThemeSettings`) happen in the same synchronous handler on confirm. Use a `pendingTheme` intermediate state object to hold both values until confirmed.

**Warning signs:** Controls show parsed values but CSS is not applied to the overlay, or vice versa.

### Pitfall 4: Scale transform conflict between marketplace-themes layer and visual-customizer layer

**What goes wrong:** The `.event-message` base rule in `@layer marketplace-themes` already sets `transform: scale(1.05)`. If the new size modifier rules in `@layer visual-customizer` use element-level selectors that don't match exactly (wrong class name), the base layer wins.

**Why it happens:** Attribute selectors like `[class*='super_chat']` must match what the actual DOM elements have. If event elements use different class names (e.g., `event-type-super_chat` vs `super_chat`), the attribute selector won't match.

**How to avoid:** Check actual event message class names used in the frontend message renderer before writing the attribute selectors. The existing events.css already uses `[class*='super_chat']`, `[class*='subscription']`, `[class*='raid']`, `[class*='bits']`, `[class*='member']`, `[class*='gift']` — use the same selectors.

**Warning signs:** Slider moves but preview doesn't scale the event message.

### Pitfall 5: ThemeMarketplaceModal prop type change not propagated

**What goes wrong:** The modal's `onApplyTheme` prop type changes from `(css: string) => void` to `(css: string, parsedSettings: Partial<VisualSettings>) => void`. The editor page handler must be updated too. If only the modal type changes, TypeScript errors arise at usage site.

**Why it happens:** Prop type changes must be applied at both the definition and all usage sites.

**How to avoid:** Update `ThemeMarketplaceModalProps` interface, then update the `onApplyTheme` callback in `overlays/[id]/page.tsx`. Note: the modal itself does NOT call `parseCssToVisualSettings` — it still only passes the raw CSS string. The parsing happens in the editor page handler, which calls `parseCssToVisualSettings(css)` and passes the result as the second argument.

Wait — re-reading the context: `onApplyTheme(css: string, parsedSettings: Partial<VisualSettings>)` means the editor page's handler signature changes to accept `parsedSettings`. But the modal still only emits a CSS string. The parsing must happen before calling `onApplyTheme`. Since the modal stays dumb, the parsing must happen in the ThemeCard's apply button action, passing both CSS and parsed settings up. OR: the editor page handler receives just `css` (from modal), parses it inline, then stores parsed result. The cleanest interpretation from CONTEXT.md: keep modal dumb, parse in the editor page handler. So the callback can stay `(css: string) => void` and parsing happens inside the editor handler. The CONTEXT.md says "extend signature to `(css: string, parsedSettings: Partial<VisualSettings>)`" — this means the editor page's handler internally receives the CSS, parses it, then does the merge. The ThemeMarketplaceModal prop signature update is optional; the simpler approach is to keep modal prop unchanged and parse in the handler.

**Resolution:** Per CONTEXT.md the modal prop type extension is listed as an integration point. The safest implementation: ThemeCard still calls `onApplyTheme(css)` with only CSS; the editor page handler parses inline: `const parsed = parseCssToVisualSettings(css); handleApplyTheme(css, parsed)`. This avoids touching the modal internals while achieving the goal.

---

## Code Examples

Verified from actual codebase inspection:

### Existing onChange pattern (from SizingGroup)

```typescript
// Source: frontend/src/components/appearance/SizingGroup.tsx
<SliderControl
  label="Emote scale"
  value={parseFloat(visualSettings.emoteScale ?? '1')}
  min={0.5}
  max={3.0}
  step={0.1}
  unit="×"
  onChange={(v) => onChange({ emoteScale: `${v}` })}
/>
```

Size modifier sliders in EventsGroup follow the same pattern with `unit="×"` and unitless string storage.

### Existing postMessage flow (from overlay editor page)

```typescript
// Source: frontend/src/app/overlays/[id]/page.tsx (lines 926-933)
const sendCssToIframe = useCallback((settings: Partial<VisualSettings>) => {
  const css = visualSettingsToCss(settings)
  iframeRef.current?.contentWindow?.postMessage(
    { type: 'VISUAL_CSS_UPDATE', css },
    '*'
  )
}, [])

const handleVisualSettingsChange = useCallback((patch: Partial<VisualSettings>) => {
  setVisualSettings((prev) => {
    const next = { ...prev, ...patch }
    sendCssToIframe(next)
    return next
  })
}, [sendCssToIframe])
```

EventsGroup wires into the exact same `onChange` prop that calls `handleVisualSettingsChange`. No new wiring is needed.

### Existing Dialog usage pattern

```typescript
// Source: frontend/src/components/ui/dialog.tsx exports
// Usage from existing dialog stories and components
<Dialog.Root open={open} onOpenChange={setOpen}>
  <Dialog.Content size="sm" showCloseButton={false}>
    <Dialog.Title>Confirm action</Dialog.Title>
    <Dialog.Description>Are you sure?</Dialog.Description>
    <div className="mt-4 flex justify-end gap-2">
      <Dialog.Close>Cancel</Dialog.Close>
      <button onClick={handleConfirm}>Continue</button>
    </div>
  </Dialog.Content>
</Dialog.Root>
```

### Existing test harness pattern (from VisibilityGroup.test.tsx)

```typescript
// Source: frontend/src/components/appearance/__tests__/VisibilityGroup.test.tsx
// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest'
import React from 'react'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
afterEach(() => { cleanup() })
```

EventsGroup.test.tsx follows this exact header pattern. theme-css-parser.test.ts is a pure unit test — no jsdom environment needed (no DOM).

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Regex across all CSS parsing | Simple regex on custom property declarations only (`--chat-*: value;`) | Phase 36 decision | Correct scope: themes only use custom properties for theming; full CSS parsing not needed |
| CSS variables emitted unconditionally | Only emit set fields (undefined → no emission) | Phase 33 design | Cascade default applies when field is not set |

**Current status of VisualSettings type (discovered):**

The type already has all 5 event visibility fields and 4 size modifier fields. Only `membershipGiftSizeModifier` is absent from both the type definition (line 73 is `bitsSizeModifier`) and `PROPERTY_MAP` (line 65 is `bitsSizeModifier` — no `membershipGiftSizeModifier` entry). This is consistent with the CONTEXT.md finding.

The `visual-settings-to-css.test.ts` full test currently uses 49 properties based on the count assertion at line 89. Adding `membershipGiftSizeModifier` brings the total to 50.

---

## Open Questions

1. **EventsGroup selector specificity in events.css**
   - What we know: existing `[class*='member']` and `[class*='gift']` patterns are used in events.css for border-left coloring
   - What's unclear: does `[class*='member']` also accidentally match non-membership-gift events? The `gift_subscription` class also contains "gift"
   - Recommendation: Use same attribute selectors already in events.css to guarantee consistency; accept that some selectors share CSS vars (membership gift and gift_subscription share the membership-gift size modifier — this is acceptable and consistent with the show/hide field `showMembershipGift` which covers both)

2. **parsedThemeSettings initialization on page load**
   - What we know: initialized as `{}` on fresh page load
   - What's unclear: if overlay config was previously saved with visual_settings AND a theme was applied, should `parsedThemeSettings` be initialized from `visual_settings`?
   - Recommendation: per CONTEXT.md, `parsedThemeSettings` is initialized `{}` on page load and updated only when a theme is applied. If the user saved with visual_settings, Reset clears them (sets to `{}`). This is correct behavior — the snapshot is only from the current session's theme apply.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Vitest (vitest.config.ts — two projects: unit + storybook) |
| Config file | `frontend/vitest.config.ts` |
| Quick run command | `cd frontend && npx vitest run --project unit` |
| Full suite command | `cd frontend && npx vitest run --project unit` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| APPR-09 | EventsGroup renders 5 event labels | unit | `cd frontend && npx vitest run --project unit src/components/appearance/__tests__/EventsGroup.test.tsx` | ❌ Wave 0 |
| APPR-09 | Show/hide toggle fires onChange with 'block'/'none' | unit | same | ❌ Wave 0 |
| APPR-09 | Size modifier slider fires onChange with unitless string | unit | same | ❌ Wave 0 |
| APPR-09 | membershipGiftSizeModifier in PROPERTY_MAP emits correct CSS var | unit | `cd frontend && npx vitest run --project unit src/lib/utils/__tests__/visual-settings-to-css.test.ts` | ✅ (update needed) |
| APPR-10 | Every onChange in EventsGroup flows through handleVisualSettingsChange | unit | EventsGroup test with mock onChange | ❌ Wave 0 |
| VISM-02 | parseCssToVisualSettings: full theme CSS → all matched properties extracted | unit | `cd frontend && npx vitest run --project unit src/lib/utils/__tests__/theme-css-parser.test.ts` | ❌ Wave 0 |
| VISM-02 | parseCssToVisualSettings: unknown properties ignored + console.warn called | unit | same | ❌ Wave 0 |
| VISM-04 | Reset button with no theme sets visualSettings to {} | unit (manual verify) | manual-only — tests component behavior, not state transitions in full page | manual-only |

### Sampling Rate

- **Per task commit:** `cd frontend && npx vitest run --project unit`
- **Per wave merge:** `cd frontend && npx vitest run --project unit`
- **Phase gate:** Full unit suite green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `frontend/src/components/appearance/__tests__/EventsGroup.test.tsx` — covers APPR-09, APPR-10
- [ ] `frontend/src/lib/utils/__tests__/theme-css-parser.test.ts` — covers VISM-02
- [ ] `frontend/src/lib/utils/__tests__/visual-settings-to-css.test.ts` — update existing: add `membershipGiftSizeModifier` to Required fixture, update count from 49 to 50

---

## Sources

### Primary (HIGH confidence)

- Direct file inspection: `frontend/src/lib/types/visual-settings.ts` — VisualSettings type, all fields
- Direct file inspection: `frontend/src/lib/utils/visual-settings-to-css.ts` — PROPERTY_MAP, all 49 entries confirmed
- Direct file inspection: `frontend/src/components/appearance/VisibilityGroup.tsx` — toggle pattern
- Direct file inspection: `frontend/src/components/appearance/SizingGroup.tsx` — slider pattern
- Direct file inspection: `frontend/src/components/appearance/AppearancePanel.tsx` — structure, where to append EventsGroup
- Direct file inspection: `frontend/src/components/appearance/ToggleSwitch.tsx` — component API
- Direct file inspection: `frontend/src/components/appearance/SliderControl.tsx` — component API
- Direct file inspection: `frontend/src/components/appearance/CollapsibleSection.tsx` — usage pattern
- Direct file inspection: `frontend/src/components/ui/dialog.tsx` — Dialog API, exports
- Direct file inspection: `frontend/src/app/overlays/[id]/page.tsx` — editor state, postMessage, AppearancePanel integration, ThemeMarketplaceModal usage
- Direct file inspection: `frontend/src/app/overlays/[id]/preview/embed/page.tsx` — VISUAL_CSS_UPDATE listener
- Direct file inspection: `frontend/src/styles/events.css` — existing layer structure, class selectors used
- Direct file inspection: `frontend/src/lib/utils/__tests__/visual-settings-to-css.test.ts` — property count assertion (49)
- Direct file inspection: `frontend/vitest.config.ts` — unit test project config
- Direct file inspection: `.planning/config.json` — nyquist_validation absent (treated as enabled)

### Secondary (MEDIUM confidence)

- CONTEXT.md decisions — all locked decisions from /gsd:discuss-phase session

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — inspected actual source files, no guessing
- Architecture: HIGH — all patterns derived from existing implementation in phases 33–35
- Pitfalls: HIGH — derived from code inspection (e.g., regex lastIndex, cascade layer ordering, test count)

**Research date:** 2026-03-18
**Valid until:** 2026-04-18 (stable frontend codebase, no fast-moving dependencies)
