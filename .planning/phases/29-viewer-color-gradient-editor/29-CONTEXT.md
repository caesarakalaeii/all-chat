# Phase 29: Viewer Color & Gradient Editor - Context

**Gathered:** 2026-03-15
**Status:** Ready for planning

<domain>
## Phase Boundary

Replace the Phase 28 `/settings/viewer` stub with a full color editor (all authenticated users) and gradient editor (premium users). Persist `name_gradient` JSONB in `viewer_cosmetics`. Update overlay render component (website + extension) to apply gradient using `bg-clip-text text-transparent` + inline `backgroundImage`. Non-premium users see the gradient tab locked. Creating/managing premium status, avatar frames, and flairs are separate phases.

</domain>

<decisions>
## Implementation Decisions

### Color picker UX
- Autosave on color change (fires PATCH immediately on input) — consistent with the extension popup decided in Phase 28
- Native `<input type="color">` swatch + editable hex input field only — no additional sliders or wheels
- Save feedback: subtle inline "Saved ✓" text briefly appears next to the picker on success — no toast, no modal
- Color section and gradient section share a single card with two tabs: "Solid Color" and "Gradient"

### Gradient editor UI
- Stacked color swatches with +/− buttons: a list of 2–4 color rows, each with a native color swatch + hex field; Add stop / Remove (×) buttons
- Angle: range slider (0–360°) + numeric input field showing exact degrees side-by-side
- Explicit "Save gradient" button (not autosave) — gradient editor accumulates multiple changes before a PATCH is sent
- Saving a gradient clears `name_color` (set to null) — one mode active at a time. Overlay logic: if `name_gradient` non-null → render gradient; else if `name_color` non-null → render flat color; else platform default

### Live preview
- Simulated chat message row: small avatar circle + viewer's actual display name (from JWT claims) styled with the current color/gradient + static sample message text "Hello world!"
- Preview positioned below the controls within the same card tab
- Preview updates live as the user adjusts stops or angle (before saving)

### Premium gate
- The "Gradient" tab is visible to all authenticated users but disabled (not clickable) for non-premium users, with a lock icon or "Premium" badge on the tab label
- JWT re-validation on gradient save: before each PATCH for gradient, re-fetch the viewer JWT to confirm it is still valid and premium; return 403 if not premium (server-side enforcement always applies regardless)
- Gradient rendering scope: both website overlay (`frontend/src/app/overlay/[id]/page.tsx`) and browser extension chat overlay — both receive the `name_gradient` field via the enriched `UserInfo` and apply it

### Claude's Discretion
- Exact DB migration numbering (next after the Phase 28 migrations)
- Error state presentation if gradient save fails (PATCH 403 or network error)
- Specific Tailwind classes for the locked tab appearance
- Sample avatar appearance in the live preview (initials-based fallback acceptable)

</decisions>

<specifics>
## Specific Ideas

- The tabbed layout means switching from Gradient tab back to Solid Color tab re-enables the flat color picker — saving flat color clears the gradient (one mode at a time, consistent with the gradient-clears-color rule)
- Gradient tab lock icon keeps the feature discoverable for free users without hiding it entirely

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `frontend/src/app/settings/viewer/page.tsx`: Phase 28 stub with `<input type="color">`, `handleColorChange`, `handleSaveColor`, JWT decode, autosave pattern — replace the Name Color card section with the new tabbed card
- `frontend/src/components/ui/card.tsx`, `button.tsx`, `input.tsx`: shadcn/ui components already wired up in the settings page — use for the tab UI and editor controls
- `UserInfo.color?: string` (`frontend/src/lib/types/message.ts` line 76): existing field for flat color — Phase 29 adds `name_gradient?: NameGradient` alongside it
- Overlay render (`frontend/src/app/overlay/[id]/page.tsx` line 676): currently `style={{ color: message.user?.color || '#FFFFFF' }}` — Phase 29 adds gradient branch using `bg-clip-text text-transparent` + inline `backgroundImage: 'linear-gradient(...)'`

### Established Patterns
- ViewerBadgeEnricher already injects `name_color` into `UserInfo.Color` from `viewer:identity:{platform}:{platform_user_id}` Redis cache — Phase 29 extends cache value and enricher to also include `name_gradient` JSONB
- PATCH `/api/v1/auth/viewer/cosmetics` already exists (Phase 28) — Phase 29 adds `name_gradient` to the request/response schema
- JWT claims include `name_color` (Phase 28) — may need `name_gradient` added to claims or fetched separately on the settings page

### Integration Points
- `viewer_cosmetics` table: add `name_gradient JSONB` column via new migration
- Message-processor enricher: extend cache value shape `{ viewer_id, name_color, name_gradient }`, inject into `UserInfo`
- `UserInfo` type in frontend: add `name_gradient?: { type: 'linear'; colors: string[]; angle: number }`
- Browser extension: apply same gradient rendering logic as the website overlay (same `bg-clip-text` pattern)

</code_context>

<deferred>
## Deferred Ideas

- None raised — discussion stayed within phase scope

</deferred>

---

*Phase: 29-viewer-color-gradient-editor*
*Context gathered: 2026-03-15*
