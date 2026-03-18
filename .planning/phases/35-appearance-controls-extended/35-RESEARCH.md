# Phase 35: Appearance Controls — Extended - Research

**Researched:** 2026-03-18
**Domain:** React component authoring — visibility toggles, sizing sliders, platform color overrides within an existing `AppearancePanel`
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **Toggle switch** control per row — label on left, on/off switch on right
- New `ToggleSwitch` primitive built inline (~20 lines, no library); reuses streaming dark aesthetic
- Labels use verb form: "Show avatars", "Show badges", "Show timestamps", "Show platform badge", "Show emotes", "Show username"
- Checked state maps `'inline'`/`'block'` → ON; `'none'` → OFF
- **Query iframe computed CSS** once on `onIframeReady` — call `getComputedStyle(iframe.contentDocument.documentElement).getPropertyValue('--chat-show-*')` for all 6 properties
- Store queried values as local component defaults (used only when `visualSettings` field is `undefined`)
- Query happens once; result stored in component state (not re-queried on re-render)
- If `visualSettings` has a value, that wins over queried default
- **Avatar size**: 16–64px, default 32px, step 2px — `avatarSize` field, emits `${v}px`
- **Badge size**: 12–32px, default 18px, step 2px — `badgeSize` field, emits `${v}px`
- **Emote scale**: 0.5–3.0×, default 1.0×, step 0.1 — `emoteScale` field, emits `${v}` (unitless multiplier)
- All use existing `SliderControl` component
- Each platform row initializes the `ColorPickerControl` to the platform's brand default when no override is set:
  - Twitch: `#9147FF`, YouTube: `#FF0000`, Kick: `#53FC18`, TikTok: `#000000`, Discord: `#5865F2`
- Small reset icon button (`↺`) per row — clicking sets the field to `undefined` (removes override, restores cascade default)
- When field is `undefined`, the picker displays the brand default hex as its visual value (but does not write it to `visualSettings`)
- Uses existing `ColorPickerControl` (solid hex only, no opacity)
- Add all three groups after the existing Phase 34 groups
- Order: Typography → Colors → Background & Bubbles → Visibility → Sizing → Platform Colors
- All sections start collapsed (consistent with Phase 34)

### Claude's Discretion

- Exact ToggleSwitch visual styling (track color, thumb size, transition duration)
- Reset button icon choice and hover state
- Whether to use `contentDocument` or `contentWindow.document` for iframe style query
- Fallback behavior if iframe is not yet ready when Visibility section is first opened

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| APPR-05 | User can toggle component visibility individually: avatars, badges, timestamps, platform badge, emotes, username | VisibilityGroup with ToggleSwitch primitive; `--chat-show-*` CSS vars already mapped in `visual-settings-to-css.ts` |
| APPR-06 | User can adjust component sizing: avatar size, badge size, emote scale | SizingGroup using existing `SliderControl`; fields already in `VisualSettings` interface |
| APPR-07 | User can override per-platform accent colors (Twitch, YouTube, Kick, TikTok, Discord) | PlatformColorsGroup using existing `ColorPickerControl`; `--platform-*-accent` vars already in CSS map |
</phase_requirements>

---

## Summary

Phase 35 adds three new control groups to the existing `AppearancePanel`. All infrastructure is already in place from Phase 33/34: `VisualSettings` type has all required fields, `visual-settings-to-css.ts` already maps all CSS properties, `SliderControl` and `ColorPickerControl` exist and are proven, `CollapsibleSection` wraps groups, and `AppearancePanel` knows how to mount groups. The live preview postMessage pipeline (`handleVisualSettingsChange` → `sendCssToIframe`) is already wired in the overlay editor page and requires no modification.

The only net-new primitive is `ToggleSwitch`, an inline (~20 line) React component. `VisibilityGroup` adds a complexity layer: it must read iframe computed CSS once on `onIframeReady` to seed defaults, requiring the parent overlay-editor page to pass an `iframeRef` or the queried defaults down as props. The CONTEXT.md decision is to hoist the query to the parent (overlay editor), store queried defaults in component state there, and pass them as props to `VisibilityGroup`.

`PlatformColorsGroup` needs a per-row reset button (`↺`) that sets the field to `undefined` in `visualSettings`, while the color picker continues to display the platform brand default visually when the field is absent.

**Primary recommendation:** Build `VisibilityGroup`, `SizingGroup`, and `PlatformColorsGroup` as direct siblings of the existing Phase 34 groups, following the identical `{ visualSettings, onChange }` prop contract. Mount all three in `AppearancePanel` using `CollapsibleSection`.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| React | 18+ | Component rendering | Project standard |
| TypeScript | 5+ | Type safety | Project standard — `any` is prohibited |
| Tailwind CSS | 3+ | Utility styling | Project standard for all UI |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `@base-ui/react` | 1.2.0 | `Collapsible.*` used by `CollapsibleSection` | Already used; no new imports needed for Phase 35 groups |
| `react-colorful` | current | `HexColorPicker` used inside `ColorPickerControl` | Already used; no new dependency |
| `lucide-react` | current | Icon for reset button (`RotateCcw` or similar) | Already a project dependency |
| `vitest` + `@testing-library/react` | current | Unit tests for groups | All tests in `__tests__/` use `// @vitest-environment jsdom` |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Inline `ToggleSwitch` | Headless UI / Radix Switch | Custom inline keeps dependency count low; ~20 lines is trivial |
| `RotateCcw` (lucide) for reset | Unicode `↺` character | Icon is cleaner; `lucide-react` is already present |

**Installation:** No new packages required.

---

## Architecture Patterns

### Recommended Project Structure

```
frontend/src/components/appearance/
├── VisibilityGroup.tsx          # new — APPR-05
├── SizingGroup.tsx              # new — APPR-06
├── PlatformColorsGroup.tsx      # new — APPR-07
├── ToggleSwitch.tsx             # new — primitive used by VisibilityGroup
├── AppearancePanel.tsx          # modified — add 3 new group imports + CollapsibleSection entries
└── __tests__/
    ├── VisibilityGroup.test.tsx # new
    ├── SizingGroup.test.tsx     # new
    └── PlatformColorsGroup.test.tsx  # new
```

### Pattern 1: Group Component

Every group follows the same contract established in Phase 34.

```typescript
// Source: frontend/src/components/appearance/ColorsGroup.tsx (verified)
'use client'

import React from 'react'
import type { VisualSettings } from '@/lib/types/visual-settings'

export interface XxxGroupProps {
  visualSettings: Partial<VisualSettings>
  onChange: (patch: Partial<VisualSettings>) => void
}

export function XxxGroup({ visualSettings, onChange }: XxxGroupProps): React.ReactElement {
  return (
    <div className="space-y-3">
      {/* controls */}
    </div>
  )
}
```

### Pattern 2: SliderControl usage

```typescript
// Source: frontend/src/components/appearance/BackgroundGroup.tsx (verified)
<SliderControl
  label="Border radius"
  value={parseFloat(visualSettings.bubbleBorderRadius ?? '8')}
  min={0}
  max={24}
  step={1}
  unit="px"
  onChange={(v) => onChange({ bubbleBorderRadius: `${v}px` })}
/>
```

For emote scale (unitless multiplier, step 0.1):

```typescript
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

### Pattern 3: ColorPickerControl usage (solid hex, no opacity)

```typescript
// Source: frontend/src/components/appearance/ColorsGroup.tsx (verified)
<ColorPickerControl
  label="Message color"
  value={visualSettings.messageColor ?? '#ffffff'}
  onChange={(hex) => onChange({ messageColor: hex })}
/>
```

For PlatformColorsGroup with reset button:

```typescript
// When field is undefined, display brand default in picker but do NOT write it to visualSettings
const twitchValue = visualSettings.twitchAccent ?? '#9147FF'

<div className="flex items-center gap-1">
  <ColorPickerControl
    label="Twitch"
    value={twitchValue}
    onChange={(hex) => onChange({ twitchAccent: hex })}
  />
  <button
    type="button"
    aria-label="Reset Twitch accent"
    onClick={() => onChange({ twitchAccent: undefined })}
    className="rounded p-1 text-text-dim hover:text-text"
  >
    <RotateCcw className="h-3 w-3" />
  </button>
</div>
```

### Pattern 4: AppearancePanel mounting

```typescript
// Source: frontend/src/components/appearance/AppearancePanel.tsx (verified)
// Current structure — Phase 35 adds three entries after background:
<CollapsibleSection id="visibility" title="Visibility">
  <VisibilityGroup visualSettings={visualSettings} onChange={onChange} />
</CollapsibleSection>
<CollapsibleSection id="sizing" title="Sizing">
  <SizingGroup visualSettings={visualSettings} onChange={onChange} />
</CollapsibleSection>
<CollapsibleSection id="platform-colors" title="Platform Colors">
  <PlatformColorsGroup visualSettings={visualSettings} onChange={onChange} />
</CollapsibleSection>
```

All `CollapsibleSection` ids are stored in localStorage key `appearance-panel-sections-v1` — the new ids `"visibility"`, `"sizing"`, `"platform-colors"` slot right into the existing mechanism with no change.

### Pattern 5: ToggleSwitch primitive

```typescript
'use client'

import React from 'react'

export interface ToggleSwitchProps {
  label: string
  checked: boolean
  onChange: (checked: boolean) => void
}

export function ToggleSwitch({ label, checked, onChange }: ToggleSwitchProps): React.ReactElement {
  return (
    <div className="flex items-center justify-between">
      <span className="text-sm text-text-sub">{label}</span>
      <button
        type="button"
        role="switch"
        aria-checked={checked}
        onClick={() => onChange(!checked)}
        className={`relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-twitch ${checked ? 'bg-twitch' : 'bg-surface-alt'}`}
      >
        <span
          className={`pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out ${checked ? 'translate-x-4' : 'translate-x-0'}`}
        />
      </button>
    </div>
  )
}
```

### Pattern 6: Visibility defaults from iframe

The CONTEXT.md decision is to query iframe computed styles once on `onIframeReady`. The query happens in the overlay-editor page and the result is passed as a prop to `VisibilityGroup`:

```typescript
// In overlay editor page (frontend/src/app/overlays/[id]/page.tsx)
const [iframeVisibilityDefaults, setIframeVisibilityDefaults] =
  useState<Partial<VisualSettings>>({})

const handleIframeReady = useCallback((iframe: HTMLIFrameElement) => {
  iframeRef.current = iframe
  sendCssToIframe(visualSettings)

  // Query computed visibility defaults once
  const style = getComputedStyle(iframe.contentDocument!.documentElement)
  const fields: Array<[keyof VisualSettings, string]> = [
    ['showAvatars',       '--chat-show-avatars'],
    ['showBadges',        '--chat-show-badges'],
    ['showTimestamps',    '--chat-show-timestamps'],
    ['showPlatformBadge', '--chat-show-platform-badge'],
    ['showEmotes',        '--chat-show-emotes'],
    ['showUsername',      '--chat-show-username'],
  ]
  const defaults: Partial<VisualSettings> = {}
  for (const [field, cssVar] of fields) {
    const v = style.getPropertyValue(cssVar).trim()
    if (v) {
      // Cast: values are either 'inline', 'block', or 'none'
      ;(defaults as Record<string, string>)[field] = v
    }
  }
  setIframeVisibilityDefaults(defaults)
}, [sendCssToIframe, visualSettings])
```

`VisibilityGroup` then receives both `visualSettings` (user overrides) and `visibilityDefaults` (queried from iframe) as props, using the queried value when the field is `undefined` in `visualSettings`.

Alternative: hoist queried defaults directly inside `VisibilityGroup` via an `iframeRef` prop. The CONTEXT.md leaves this as Claude's Discretion but the pattern of passing queried state down from parent is consistent with how the overlay editor already manages `iframeRef` and `visualSettings`.

### Pattern 7: VisibilityGroup field conversion

Visibility fields use union types: `'inline' | 'none'` (for inline elements) or `'block' | 'none'` (for timestamps). The toggle maps:

- checked (ON) → `'inline'` (most fields) or `'block'` (showTimestamps)
- unchecked (OFF) → `'none'`

```typescript
// Convert display value to checked state
function isVisible(val: string | undefined, defaultOn: boolean): boolean {
  if (val === undefined) return defaultOn
  return val !== 'none'
}

// Convert toggle event to VisualSettings patch
function toDisplayValue(
  field: keyof VisualSettings,
  checked: boolean
): 'inline' | 'block' | 'none' {
  if (!checked) return 'none'
  // showTimestamps is 'block'; all others are 'inline'
  return field === 'showTimestamps' ? 'block' : 'inline'
}
```

### Anti-Patterns to Avoid

- **Re-querying iframe on every render:** Query happens once on `onIframeReady`; result is stored in state.
- **Writing brand default to visualSettings on reset:** Reset sets field to `undefined` to let the CSS cascade provide the default; do NOT write the brand default hex into `visualSettings`.
- **Using `display: none` strings in TypeScript:** The `VisualSettings` fields are typed as literal unions (`'inline' | 'none'`, `'block' | 'none'`); always cast correctly via `toDisplayValue()`.
- **Importing new toggle/switch libraries:** `ToggleSwitch` is built inline per the locked decision.
- **Making ToggleSwitch a server component:** All appearance components are `'use client'` — `ToggleSwitch` must be too.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Labeled slider | Custom range+label wrapper | `SliderControl` | Already handles min/max/step/unit/value display |
| Color picker with popover | Custom popover+picker | `ColorPickerControl` | Already handles click-outside, `HexColorPicker`, swatch button |
| Collapsible section | Custom accordion | `CollapsibleSection` | Already handles localStorage persistence, base-ui animation |
| CSS variable injection to iframe | Direct iframe DOM write | `handleVisualSettingsChange` → `sendCssToIframe` pipeline | Already in place; groups just call `onChange(patch)` |

**Key insight:** The postMessage pipeline is transparent to group components — they only call `onChange(patch: Partial<VisualSettings>)` and the parent handles CSS generation and iframe communication.

---

## Common Pitfalls

### Pitfall 1: Reset button sets `undefined` not brand default

**What goes wrong:** Developer writes `onChange({ twitchAccent: '#9147FF' })` on reset, persisting the brand default into `visualSettings`.
**Why it happens:** The color picker needs a fallback value to display, so it's tempting to store the fallback.
**How to avoid:** Reset always calls `onChange({ twitchAccent: undefined })`. The picker's `value` prop is `visualSettings.twitchAccent ?? '#9147FF'` — the `??` provides the visual fallback without persisting it.
**Warning signs:** `visualSettings` contains brand default hex values when no user customization has been done.

### Pitfall 2: Visibility defaults not queried before section is opened

**What goes wrong:** User opens Visibility section before `onIframeReady` fires; queried defaults are empty so all toggles default to ON regardless of CSS.
**Why it happens:** `onIframeReady` fires asynchronously after iframe load.
**How to avoid:** If `iframeVisibilityDefaults` is empty (query not yet done), fall back to `true` (all shown) as a safe default. The query completes on iframe ready and the component re-renders with accurate defaults.
**Warning signs:** Toggles briefly show wrong state then flip.

### Pitfall 3: TypeScript type error on visibility field assignment

**What goes wrong:** Assigning `getPropertyValue(cssVar).trim()` (type `string`) to a `VisualSettings` field typed as `'inline' | 'none'` causes a TS error.
**Why it happens:** CSS `getPropertyValue` always returns `string`.
**How to avoid:** Cast via `as Record<string, string>` when populating the defaults object, or use a type-safe helper that validates the value before assignment.
**Warning signs:** TypeScript compilation error in the `onIframeReady` handler.

### Pitfall 4: Emote scale emitted with `px` suffix

**What goes wrong:** `SizingGroup` emits `onChange({ emoteScale: '1.5px' })` instead of `'1.5'`.
**Why it happens:** Copy-paste from avatar/badge sliders that use `${v}px`.
**How to avoid:** Per the locked decision, `emoteScale` emits the unitless multiplier: `onChange({ emoteScale: \`${v}\` })`.
**Warning signs:** CSS variable `--chat-emote-scale` has `px` suffix, breaking the emote scaling calculation.

### Pitfall 5: CollapsibleSection id collision

**What goes wrong:** Using an id like `"visibility"` that was already used in a prior phase causes the section to inherit another section's open/closed state from localStorage.
**Why it happens:** All sections share one localStorage key `appearance-panel-sections-v1`.
**How to avoid:** Use distinct ids `"visibility"`, `"sizing"`, `"platform-colors"` — none are used by Phase 34 groups (`"typography"`, `"colors"`, `"background"`).
**Warning signs:** Section open/closed state seems wrong on first render.

---

## Code Examples

Verified patterns from existing source files:

### SliderControl for px-valued field

```typescript
// Source: frontend/src/components/appearance/BackgroundGroup.tsx
<SliderControl
  label="Border radius"
  value={parseFloat(visualSettings.bubbleBorderRadius ?? '8')}
  min={0}
  max={24}
  step={1}
  unit="px"
  onChange={(v) => onChange({ bubbleBorderRadius: `${v}px` })}
/>
```

### ColorPickerControl (solid hex, no opacity)

```typescript
// Source: frontend/src/components/appearance/ColorsGroup.tsx
<ColorPickerControl
  label="Message color"
  value={visualSettings.messageColor ?? '#ffffff'}
  onChange={(hex) => onChange({ messageColor: hex })}
/>
```

### AppearancePanel structure (current — Phase 34 end state)

```typescript
// Source: frontend/src/components/appearance/AppearancePanel.tsx
<div className="flex flex-col gap-0">
  <CollapsibleSection id="typography" title="Typography">
    <TypographyGroup visualSettings={visualSettings} onChange={onChange} />
  </CollapsibleSection>
  <CollapsibleSection id="colors" title="Colors">
    <ColorsGroup visualSettings={visualSettings} onChange={onChange} />
  </CollapsibleSection>
  <CollapsibleSection id="background" title="Background & Bubbles">
    <BackgroundGroup visualSettings={visualSettings} onChange={onChange} />
  </CollapsibleSection>
  {/* Phase 35 adds: visibility, sizing, platform-colors */}
</div>
```

### handleVisualSettingsChange in overlay editor (no change required)

```typescript
// Source: frontend/src/app/overlays/[id]/page.tsx
const handleVisualSettingsChange = useCallback((patch: Partial<VisualSettings>) => {
  setVisualSettings((prev) => {
    const next = { ...prev, ...patch }
    sendCssToIframe(next)
    return next
  })
}, [sendCssToIframe])
```

Groups call `onChange(patch)` — this callback handles CSS generation and iframe injection automatically.

### Test file header pattern

```typescript
// Source: frontend/src/components/appearance/__tests__/ColorsGroup.test.tsx
// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest'
import React from 'react'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import type { VisualSettings } from '@/lib/types/visual-settings'

afterEach(() => { cleanup() })
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| CSS injected via `<style>` tag re-render | `style#visual-customizer-style` managed imperatively via DOM | Phase 34 | Avoids React re-render overhead on every CSS change |
| Visual settings not persisted | Stored as JSONB `visual_settings` in `overlay_configs` table | Phase 33 | Settings survive page reload |
| All CSS custom properties hardcoded in theme | `@layer visual-customizer` overrides theme defaults without touching theme CSS | Phase 33 | Editor overrides stack above marketplace themes |

---

## Open Questions

1. **VisibilityGroup defaults prop contract**
   - What we know: CONTEXT.md says query iframe once on `onIframeReady` and store as component defaults
   - What's unclear: Whether `VisibilityGroup` receives `visibilityDefaults` as an explicit prop from the parent, or whether the parent passes the `iframeRef` and the group queries internally
   - Recommendation: Pass `visibilityDefaults: Partial<VisualSettings>` as an explicit prop from the overlay editor to `VisibilityGroup` (then to `AppearancePanel`). This keeps the iframe-access concern in the parent and keeps `VisibilityGroup` a pure presentational group matching the other groups.

2. **AppearancePanel prop signature extension**
   - What we know: `AppearancePanel` currently takes only `{ visualSettings, onChange }`. `VisibilityGroup` needs defaults.
   - What's unclear: Add `visibilityDefaults?: Partial<VisualSettings>` to `AppearancePanelProps`, or wire it differently.
   - Recommendation: Add optional `visibilityDefaults?: Partial<VisualSettings>` to `AppearancePanelProps` and thread it into `VisibilityGroup`. Existing callers are unaffected (optional prop, defaults to `{}`).

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Vitest (project `unit`) |
| Config file | `frontend/vitest.config.ts` |
| Quick run command | `cd frontend && npx vitest run --project unit src/components/appearance/__tests__/` |
| Full suite command | `cd frontend && npx vitest run --project unit` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| APPR-05 | VisibilityGroup renders 6 labeled toggles | unit | `npx vitest run --project unit src/components/appearance/__tests__/VisibilityGroup.test.tsx` | Wave 0 |
| APPR-05 | Toggle emits correct `showAvatars` patch on click | unit | same | Wave 0 |
| APPR-05 | Toggle reflects `visualSettings` override over queried default | unit | same | Wave 0 |
| APPR-06 | SizingGroup renders 3 sliders with correct ranges | unit | `npx vitest run --project unit src/components/appearance/__tests__/SizingGroup.test.tsx` | Wave 0 |
| APPR-06 | Slider emits `avatarSize` as `${v}px` string | unit | same | Wave 0 |
| APPR-06 | Emote scale emits unitless string | unit | same | Wave 0 |
| APPR-07 | PlatformColorsGroup renders 5 color swatches | unit | `npx vitest run --project unit src/components/appearance/__tests__/PlatformColorsGroup.test.tsx` | Wave 0 |
| APPR-07 | Reset button emits `undefined` for the field | unit | same | Wave 0 |
| APPR-07 | Color picker displays brand default when field is undefined | unit | same | Wave 0 |

### Sampling Rate

- **Per task commit:** `cd frontend && npx vitest run --project unit src/components/appearance/__tests__/`
- **Per wave merge:** `cd frontend && npx vitest run --project unit`
- **Phase gate:** Full unit suite green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `frontend/src/components/appearance/__tests__/VisibilityGroup.test.tsx` — covers APPR-05
- [ ] `frontend/src/components/appearance/__tests__/SizingGroup.test.tsx` — covers APPR-06
- [ ] `frontend/src/components/appearance/__tests__/PlatformColorsGroup.test.tsx` — covers APPR-07

Framework install: none needed — vitest + @testing-library/react already in place.

---

## Sources

### Primary (HIGH confidence)

- `frontend/src/lib/types/visual-settings.ts` — All APPR-05/06/07 fields verified present with correct types
- `frontend/src/lib/utils/visual-settings-to-css.ts` — All CSS variable mappings verified (`--chat-show-*`, `--chat-avatar-size`, `--chat-badge-size`, `--chat-emote-scale`, `--platform-*-accent`)
- `frontend/src/components/appearance/AppearancePanel.tsx` — Current structure verified; mounting pattern confirmed
- `frontend/src/components/appearance/CollapsibleSection.tsx` — localStorage key and open/closed behavior verified
- `frontend/src/components/appearance/ColorPickerControl.tsx` — Props interface and swatch button `data-testid` verified
- `frontend/src/components/appearance/SliderControl.tsx` — Props interface verified
- `frontend/src/components/appearance/ColorsGroup.tsx` — Group prop contract verified
- `frontend/src/components/appearance/BackgroundGroup.tsx` — SliderControl and ColorPickerControl usage patterns verified
- `frontend/src/app/overlays/[id]/page.tsx` — `handleIframeReady`, `handleVisualSettingsChange`, `iframeRef`, `AppearancePanel` mounting all verified
- `frontend/src/components/appearance/__tests__/ColorsGroup.test.tsx` — Test file header pattern and `// @vitest-environment jsdom` directive verified
- `frontend/src/components/appearance/__tests__/BackgroundGroup.test.tsx` — fireEvent and `onChange` assertion patterns verified
- `frontend/vitest.config.ts` — Unit test project config and include glob verified

### Secondary (MEDIUM confidence)

- `lucide-react` — `RotateCcw` icon available; project already uses `lucide-react` (verified via import in `AppearancePanel.tsx` via `CollapsibleSection.tsx`)

---

## Metadata

**Confidence breakdown:**

- Standard stack: HIGH — all libraries verified from source files
- Architecture: HIGH — all patterns verified from Phase 34 implementations
- Pitfalls: HIGH — derived from type definitions and code reading, not speculation
- Test patterns: HIGH — verified from existing test files

**Research date:** 2026-03-18
**Valid until:** 2026-04-18 (stable internal codebase; no external library changes expected)
