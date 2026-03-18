# Phase 36: Events Styling + Theme Import - Context

**Gathered:** 2026-03-18
**Status:** Ready for planning

<domain>
## Phase Boundary

Add `EventsGroup` to the `AppearancePanel` with show/hide + size modifier controls for all special event types. Build `theme-css-parser.ts` utility that maps `--chat-*` / `--platform-*` CSS variables to `VisualSettings` fields. Wire theme loading in the overlay editor to pre-populate all visual controls. Implement "Reset to theme defaults" (VISM-04). Verify APPR-10 end-to-end (live preview on every change). No backend changes needed.

</domain>

<decisions>
## Implementation Decisions

### Events scope
- All 5 event types get controls: SuperChat, Subscriptions, Raids, Bits, MembershipGift
- Each event type gets both a **show/hide toggle** and a **size modifier slider** — consistent controls for all
- `membershipGiftSizeModifier` field must be **added** to `VisualSettings` type (and `PROPERTY_MAP` in `visual-settings-to-css.ts`) — currently missing
  - CSS var: `--chat-membership-gift-size-modifier`
- Section label: **"Events"** (short, consistent with other AppearancePanel section labels)
- Order in EventsGroup: SuperChat → Subscriptions → Raids → Bits → Membership Gift

### Size modifier behavior
- Modifier applies **CSS `transform: scale(X)`** to the whole `.event-message` element
  - CSS: `--chat-super-chat-size-modifier: 1.5` → `.event-message.super-chat { transform: scale(var(--chat-super-chat-size-modifier, 1)) }`
- Range: **0.5–3.0×**, step 0.1
- Per-event-type sliders — separate field and slider for each event type (already typed as separate fields)
- When field is **undefined**: no CSS var emitted → events.css cascade default applies (existing `transform: scale(1.05)` stays)
- Slider display: show value as `1.5×` (number with × suffix)

### Theme import merge strategy
- `onApplyTheme(css: string)` callback is extended to accept parsed settings: `onApplyTheme(css: string, parsedSettings: Partial<VisualSettings>)`
- In the **same atomic action**, the editor page sets both `customCss` and `visualSettings`
- **If user has visual customizations** (visualSettings is non-empty): show a **confirm dialog** before replacing
  - Dialog: "Loading this theme will reset your visual customizations. Continue?"
  - On confirm: replace visualSettings with parsedSettings; also store parsedSettings as `parsedThemeSettings`
  - On cancel: do nothing (theme CSS is NOT applied either — atomic cancel)
- **If visualSettings is empty**: apply immediately without prompt
- Unknown CSS vars in the theme: **`console.warn`** in dev, silently ignored in behavior
- Opacity pairs parsed independently: `--chat-overlay-bg-color` → `overlayBgColor`, `--chat-overlay-bg-opacity` → `overlayBgOpacity` as separate fields

### theme-css-parser.ts
- New utility: `frontend/src/lib/utils/theme-css-parser.ts`
- Signature: `parseCssToVisualSettings(css: string): Partial<VisualSettings>`
- Algorithm: scan CSS string for `--chat-*` and `--platform-*` custom property declarations
  - Use regex or simple string scanning (not a full CSS parser)
  - Map known vars using a reverse of `PROPERTY_MAP` from `visual-settings-to-css.ts`
  - Skip vars not in the map (with `console.warn` for unknowns)
- Returns only the fields that were found in the CSS (never sets a field to `undefined` explicitly)
- Unit tests: full theme CSS → all matched properties extracted, unknown properties ignored + warned

### Reset to theme defaults
- **Button location**: near the theme selector button — small "Reset to theme" text button/link below/adjacent to "Open Theme Marketplace" button
- **Active theme loaded**: clicking Reset sets `visualSettings = parsedThemeSettings` (the snapshot stored at load time)
  - All controls update immediately to reflect the snapshot
- **No theme loaded**: clicking Reset sets `visualSettings = {}` (clear all — overlay renders from marketplace-themes + base layers only)
- **`parsedThemeSettings` state**: stored in overlay editor page state alongside `visualSettings`; initialized as `{}` on page load; updated only when a theme is applied

### APPR-10 verification
- End-to-end: every control change in every group (including new EventsGroup) must trigger postMessage to iframe and update live preview
- Verification is integrated into the EventsGroup implementation — same postMessage pattern, no new wiring needed
- Confirm the embed page's message listener handles EventsGroup vars (no special casing needed since all vars go through `visualSettingsToCss`)

### Claude's Discretion
- Exact dialog component used for confirm prompt (reuse existing Dialog from @base-ui/react component library)
- Regex pattern for CSS var extraction in `theme-css-parser.ts`
- EventsGroup row layout (toggle on left, slider on right, or stacked — match VisibilityGroup rows for consistency)
- Whether "Reset to theme" renders as a button or a text link

</decisions>

<specifics>
## Specific Ideas

- Confirm dialog for theme import: "Loading this theme will reset your visual customizations. Continue?" — atomic cancel (neither CSS nor settings applied on cancel)
- `parsedThemeSettings` snapshot approach: one state var, updated only on theme apply, always available for Reset

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `frontend/src/components/appearance/ToggleSwitch.tsx`: show/hide toggle — use directly for event visibility rows
- `frontend/src/components/appearance/SliderControl.tsx`: labeled slider with unit — use for size modifier rows (unit: `×`)
- `frontend/src/components/appearance/CollapsibleSection.tsx`: wrap EventsGroup in CollapsibleSection with `id="events"` and `title="Events"`
- `frontend/src/components/appearance/AppearancePanel.tsx`: add `EventsGroup` as last section (before Phase 37 reorder)
- `frontend/src/lib/utils/visual-settings-to-css.ts`: `PROPERTY_MAP` — add `membershipGiftSizeModifier` entry; also used as reference for reverse-mapping in parser
- `frontend/src/lib/types/visual-settings.ts`: add `membershipGiftSizeModifier?: string` field
- `frontend/src/components/theme-marketplace/ThemeMarketplaceModal.tsx`: `onApplyTheme(css: string)` prop — extend signature to `(css: string, parsedSettings: Partial<VisualSettings>)`
- `frontend/src/app/overlays/[id]/preview/page.tsx`: `onApplyTheme` usage at line ~1596 — update handler to receive parsedSettings, store both `customCss` and `parsedThemeSettings`

### Established Patterns
- Group props: `visualSettings: Partial<VisualSettings>`, `onChange: (patch: Partial<VisualSettings>) => void`
- `ToggleSwitch` usage: checked state maps `'block'` → ON, `'none'` → OFF
- `SliderControl` usage: `value={parseFloat(visualSettings.superChatSizeModifier ?? '1')}`, `onChange={(v) => onChange({ superChatSizeModifier: \`${v}\` })}`
- Test files: `frontend/src/components/appearance/__tests__/EventsGroup.test.tsx`
- Utility tests: `frontend/src/lib/utils/__tests__/theme-css-parser.test.ts`

### Integration Points
- `AppearancePanel.tsx`: import `EventsGroup`, wrap in `CollapsibleSection id="events" title="Events"`, append after PlatformColorsGroup
- Overlay editor page: add `parsedThemeSettings` state (`useState<Partial<VisualSettings>>({})`); extend `onApplyTheme` handler to parse CSS, handle confirm dialog, set both states
- `ThemeMarketplaceModal`: update `onApplyTheme` prop type; parse CSS in editor page (keep modal dumb — pass raw CSS string out)
- `frontend/src/lib/utils/visual-settings-to-css.ts`: add `membershipGiftSizeModifier` → `--chat-membership-gift-size-modifier` to PROPERTY_MAP
- `frontend/src/styles/events.css`: update `.event-message` rules to consume `--chat-*-size-modifier` CSS vars with `scale(var(..., 1))`

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 36-events-styling-theme-import*
*Context gathered: 2026-03-18*
