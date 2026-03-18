# Phase 35: Appearance Controls — Extended - Context

**Gathered:** 2026-03-18
**Status:** Ready for planning

<domain>
## Phase Boundary

Add `VisibilityGroup`, `SizingGroup`, and `PlatformColorsGroup` to the overlay editor's `AppearancePanel`. Each group is wrapped in a `CollapsibleSection`. All three groups wire to live preview via the same postMessage pattern established in Phase 34. No backend changes needed.

</domain>

<decisions>
## Implementation Decisions

### Visibility toggle UI
- **Toggle switch** control per row — label on left, on/off switch on right
- New `ToggleSwitch` primitive built inline (~20 lines, no library); reuses streaming dark aesthetic
- Labels use verb form: "Show avatars", "Show badges", "Show timestamps", "Show platform badge", "Show emotes", "Show username"
- Checked state maps `'inline'`/`'block'` → ON; `'none'` → OFF

### Visibility defaults
- **Query iframe computed CSS** once on `onIframeReady` — call `getComputedStyle(iframe.contentDocument.documentElement).getPropertyValue('--chat-show-*')` for all 6 properties
- Store queried values as local component defaults (used only when `visualSettings` field is `undefined`)
- Query happens once; result stored in component state (not re-queried on re-render)
- If `visualSettings` has a value, that wins over queried default

### Sizing slider ranges
- **Avatar size**: 16–64px, default 32px, step 2px — `avatarSize` field, emits `${v}px`
- **Badge size**: 12–32px, default 18px, step 2px — `badgeSize` field, emits `${v}px`
- **Emote scale**: 0.5–3.0×, default 1.0×, step 0.1 — `emoteScale` field, emits `${v}` (unitless multiplier)
- All use existing `SliderControl` component

### Platform color defaults & reset
- Each platform row initializes the `ColorPickerControl` to the platform's brand default when no override is set:
  - Twitch: `#9147FF`, YouTube: `#FF0000`, Kick: `#53FC18`, TikTok: `#000000`, Discord: `#5865F2`
- Small reset icon button (`↺`) per row — clicking sets the field to `undefined` (removes override, restores cascade default)
- When field is `undefined`, the picker displays the brand default hex as its visual value (but does not write it to `visualSettings`)
- Uses existing `ColorPickerControl` (solid hex only, no opacity)

### AppearancePanel additions
- Add all three groups after the existing Phase 34 groups
- Order: Typography → Colors → Background & Bubbles → Visibility → Sizing → Platform Colors
- All sections start collapsed (consistent with Phase 34)

### Claude's Discretion
- Exact ToggleSwitch visual styling (track color, thumb size, transition duration)
- Reset button icon choice and hover state
- Whether to use `contentDocument` or `contentWindow.document` for iframe style query
- Fallback behavior if iframe is not yet ready when Visibility section is first opened

</decisions>

<specifics>
## Specific Ideas

- Visibility defaults queried from iframe computed styles once on `onIframeReady` — not hardcoded, not re-queried on every open
- Reset button (↺) per platform row clears the override back to `undefined`, showing brand default in picker without persisting it

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `frontend/src/components/appearance/ColorPickerControl.tsx`: solid hex picker with popover swatch — used directly for PlatformColorsGroup
- `frontend/src/components/appearance/SliderControl.tsx`: labeled slider with unit — used directly for SizingGroup
- `frontend/src/components/appearance/CollapsibleSection.tsx`: collapsible wrapper with localStorage persistence — wrap all three new groups
- `frontend/src/components/appearance/AppearancePanel.tsx`: already exists, add three new group imports + CollapsibleSection entries
- `frontend/src/lib/types/visual-settings.ts`: all Phase 35 fields already defined — `showAvatars`, `showBadges`, `showTimestamps`, `showPlatformBadge`, `showEmotes`, `showUsername`, `avatarSize`, `badgeSize`, `emoteScale`, `twitchAccent`, `youtubeAccent`, `kickAccent`, `tiktokAccent`, `discordAccent`

### Established Patterns
- Group props: `visualSettings: Partial<VisualSettings>`, `onChange: (patch: Partial<VisualSettings>) => void`
- `SliderControl` usage: `value={parseFloat(visualSettings.avatarSize ?? '32')}`, `onChange={(v) => onChange({ avatarSize: \`${v}px\` })}`
- `ColorPickerControl` usage: `value={visualSettings.twitchAccent ?? '#9147FF'}`, `onChange={(hex) => onChange({ twitchAccent: hex })}`
- All groups are client components (`'use client'`), no server-side concerns

### Integration Points
- `AppearancePanel.tsx`: import and mount all three groups with CollapsibleSection wrappers
- `onIframeReady` callback (from Phase 34, wired in overlay editor page): pass iframe ref to VisibilityGroup (or hoist queried defaults to parent)
- `frontend/src/components/appearance/__tests__/`: add test files matching the pattern of ColorsGroup.test.tsx and BackgroundGroup.test.tsx

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 35-appearance-controls-extended*
*Context gathered: 2026-03-18*
