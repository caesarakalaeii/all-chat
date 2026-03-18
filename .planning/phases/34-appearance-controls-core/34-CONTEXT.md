# Phase 34: Appearance Controls — Core - Context

**Gathered:** 2026-03-18
**Status:** Ready for planning

<domain>
## Phase Boundary

Implement the Typography, Colors, and Background & Bubbles control groups in the overlay editor panel. All three groups are hosted by `AppearancePanel` with a `CollapsibleSection` wrapper for each. Wired to live preview via postMessage. Visibility, Sizing, Platform Colors, and Events controls are Phase 35–36.

</domain>

<decisions>
## Implementation Decisions

### Live Preview Wiring
- **postMessage** from the editor to the iframe — `iframeRef.current.contentWindow.postMessage({ type: 'VISUAL_CSS_UPDATE', css }, '*')`
- **No debounce** — send on every control change (CSS generation is effectively free, instant feedback is better)
- **Embed page** (`/overlays/[id]/preview/embed/page.tsx`) adds a `window.addEventListener('message', ...)` listener that upserts a `<style id="visual-customizer-style">` tag with the received CSS
- **SplitView** gets an `onIframeReady?: (ref: HTMLIFrameElement) => void` prop; it calls the callback with the iframe element after mount
- **`visualSettings` state** lives in the overlay editor page component (`useState<VisualSettings>({})`), co-located with `customCss` and other editor state — passed as props down to `AppearancePanel` and groups
- On page load, `visual_settings` from the API response populates this state (same as `custom_css` is loaded today)

### Font Family Picker
- **Both system fonts + curated Google Fonts** in a single grouped list
- **Searchable combobox** UI — type to filter by font name
- **Separate per-element font pickers**: body text (`fontFamily`), username (`usernameFontFamily` — add this field), timestamp (`timestampFontFamily` — add this field if missing from VisualSettings)
  - Note: check current VisualSettings type; `usernameFontFamily` and `timestampFontFamily` fields may need to be added
- **System fonts**: Inter, Arial, Helvetica, Georgia, Courier New, Impact, Trebuchet MS, Verdana, Tahoma
- **Curated Google Fonts** (streaming-style selection): Bebas Neue, Oswald, Rajdhani, Barlow Condensed, Exo 2, Nunito, Poppins, Roboto, Open Sans, Montserrat
- Google Fonts loaded via `<link>` in the embed page when a web font is selected (dynamic injection, not preloaded)
- Font options displayed: label shows font name rendered in that font (via `font-family` inline style on the option)

### Color Picker
- **Library: `react-colorful`** — `HexColorPicker` for solid colors, small bundle, accessible, zero deps
- **Text colors** (message body, username, timestamp): solid hex only — no opacity slider; simpler and transparent text is almost never wanted
- **Color + opacity controls** (overlay background, bubble background): inline row — [color swatch button → opens `HexColorPicker` popover] + [opacity % slider (0–100)]
  - Swatch shows current color as a small colored square button
  - Popover opens/closes on click (not hover); closes on click-outside
  - Opacity stored as separate `overlayBgOpacity` / `bubbleBgOpacity` string fields (e.g. `"0.7"`)
- Color values stored as hex strings (e.g. `"#1a1a2e"`); opacity as decimal string (e.g. `"0.85"`)

### CollapsibleSection Component
- **Built on `@base-ui/react` Collapsible** primitive (handles `aria-expanded`, keyboard, animation hooks)
- **All groups start collapsed by default** — user expands what they want
- **State persisted in localStorage** — key: `appearance-panel-sections-v1`, stores object of `{ [groupId]: boolean }`
- Smooth open/close animation via CSS (height transition using `@base-ui/react` `Collapsible.Panel` data attributes)
- `CollapsibleSection` props: `id: string`, `title: string`, `children: React.ReactNode`

### AppearancePanel Structure
- `AppearancePanel` component hosts all three groups in Phase 34 (more added in Phases 35–36)
- Each group wrapped in `CollapsibleSection`
- Order: Typography → Colors → Background & Bubbles
- Props: `visualSettings: VisualSettings`, `onChange: (patch: Partial<VisualSettings>) => void`
  - Parent merges patch into full state: `setVisualSettings(prev => ({ ...prev, ...patch }))`

### TypographyGroup Controls
- Font family picker (combobox) × 3: body text, username, timestamp
- Font weight: dropdown select (100 Thin → 900 Black, common weights)
- Font size: number input + px label (range 10–32px) for body text
- Username font size, timestamp font size: separate number inputs
- Line height: slider (1.0–2.5, step 0.1)
- Letter spacing: slider (-2px to 8px, step 0.5px)

### ColorsGroup Controls
- Message body color: `HexColorPicker` (solid, no opacity)
- Username color: `HexColorPicker` (solid, no opacity)
- Timestamp color: `HexColorPicker` (solid, no opacity)

### BackgroundGroup Controls
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

</decisions>

<specifics>
## Specific Ideas

- "Separate per-element font pickers" — body, username, timestamp each get their own font family combobox
- Font option labels should visually render in their own font (so users can see what it looks like)
- Google Fonts loaded dynamically only when selected (no preloading all 10 fonts upfront)

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `frontend/src/lib/utils/visual-settings-to-css.ts`: `visualSettingsToCss(settings)` — ready to call on every control change
- `frontend/src/lib/types/visual-settings.ts`: `VisualSettings` flat interface — check if `usernameFontFamily` and `timestampFontFamily` fields exist; add if missing
- `frontend/src/components/SplitView.tsx`: add `onIframeReady?: (ref: HTMLIFrameElement) => void` prop; call after mount
- `frontend/src/app/overlays/[id]/preview/embed/page.tsx`: add `window.addEventListener('message', handler)` for `VISUAL_CSS_UPDATE` messages
- `frontend/src/components/ui/`: existing Button, Card, Input — available for control layout

### Established Patterns
- Editor page uses `useState` for all settings (`customCss`, `fontSize`, `maxMessages`, etc.) — same pattern for `visualSettings`
- API load pattern: `useEffect(() => { overlaysApi.getConfig(id).then(...) }, [id, token])` in embed page — add `visual_settings` load here too
- `@base-ui/react` already installed; Collapsible component available at `@base-ui/react/collapsible`
- Tailwind v4 with design tokens — use `bg-surface`, `border-border`, `text-muted` etc. for control styling

### Integration Points
- `SplitView` component: parent (overlay editor page) gets iframe ref via `onIframeReady` callback
- Overlay editor page `handleSave()`: already saves `custom_css`; extend to include `visual_settings` in the PUT payload
- `visual_settings` API field already passes through Go backend from Phase 33 — no backend changes needed
- Embed page already has a `<style>` injection pattern for `custom_css` — add a second `<style id="visual-customizer-style">` managed by postMessage

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 34-appearance-controls-core*
*Context gathered: 2026-03-18*
