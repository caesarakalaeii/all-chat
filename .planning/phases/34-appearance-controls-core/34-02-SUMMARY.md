---
phase: 34-appearance-controls-core
plan: 02
subsystem: ui
tags: [react, typescript, postmessage, iframe, google-fonts, react-colorful]

# Dependency graph
requires:
  - phase: 34-appearance-controls-core
    plan: 01
    provides: VisualSettings type and visualSettingsToCss utility from Phase 33

provides:
  - react-colorful installed as production dependency
  - SplitView exposes onIframeReady callback (iframe HTMLIFrameElement on mount)
  - Embed page listens for VISUAL_CSS_UPDATE postMessages and upserts style#visual-customizer-style
  - Google Font dynamic loading in embed page via ensureGoogleFontLoaded
  - Editor page has visualSettings state (Partial<VisualSettings>) initialized from API
  - Editor page sends CSS to iframe on every visualSettings change via postMessage
  - handleSaveConfiguration includes visual_settings in the update payload

affects:
  - 34-appearance-controls-core/34-03
  - any phase that adds controls to the AppearancePanel

# Tech tracking
tech-stack:
  added:
    - react-colorful ^5.6.1 (color picker library, production dep)
  patterns:
    - postMessage channel between editor page and embed iframe (VISUAL_CSS_UPDATE)
    - Imperative DOM style tag upsert (visual-customizer-style) to avoid React re-renders
    - Google Font lazy loading via dynamic <link> injection keyed by id="gfont-{slug}"
    - onIframeReady callback pattern for exposing iframe ref from SplitView to parent

key-files:
  created: []
  modified:
    - frontend/src/components/SplitView.tsx
    - frontend/src/app/overlays/[id]/preview/embed/page.tsx
    - frontend/src/app/overlays/[id]/page.tsx
    - frontend/package.json

key-decisions:
  - "Google Font names duplicated in embed page (not imported from FontFamilyCombobox) — embed page and component are in different routing contexts"
  - "style#visual-customizer-style managed imperatively via DOM in useEffect, not React state — avoids re-render overhead on every CSS update"
  - "handleIframeReady sends current visualSettings immediately on iframe mount — ensures initial state is applied before user interaction"

patterns-established:
  - "postMessage VISUAL_CSS_UPDATE pattern: editor sends {type, css}, embed upserts style tag"
  - "sendCssToIframe via useCallback with empty deps — iframeRef.current accessed at call time"
  - "handleIframeReady depends on [sendCssToIframe, visualSettings] — sends snapshot of current settings on iframe connect"

requirements-completed: [APPR-02, APPR-03, APPR-04, APPR-08]

# Metrics
duration: 15min
completed: 2026-03-18
---

# Phase 34 Plan 02: Live Preview Wiring Summary

**react-colorful installed; SplitView iframe ref exposed; embed page wired for VISUAL_CSS_UPDATE postMessages with Google Font lazy loading; editor page has full visualSettings state + sendCssToIframe + save wiring**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-03-18T10:18:00Z
- **Completed:** 2026-03-18T10:33:00Z
- **Tasks:** 3
- **Files modified:** 4

## Accomplishments
- Installed react-colorful ^5.6.1 as production dependency in frontend/package.json
- Added onIframeReady callback to SplitView so the editor page can hold a direct ref to the live preview iframe
- Added VISUAL_CSS_UPDATE postMessage listener to the embed page with imperative style#visual-customizer-style upsert and Google Font dynamic loading
- Wired visualSettings state into the overlay editor page: state, iframeRef, sendCssToIframe, handleVisualSettingsChange, handleIframeReady, config load, save payload

## Task Commits

Each task was committed atomically:

1. **Task 1: Install react-colorful + SplitView onIframeReady prop** - `4cdd0dc` (feat)
2. **Task 2: Embed page postMessage listener + Google Font dynamic loader** - `31d4c9d` (feat)
3. **Task 3: Editor page visualSettings state + sendCssToIframe + save wiring** - `918cb73` (feat)

## Files Created/Modified
- `frontend/src/components/SplitView.tsx` - Added onIframeReady?: (iframe: HTMLIFrameElement) => void prop; ref callback on iframe
- `frontend/src/app/overlays/[id]/preview/embed/page.tsx` - Added GOOGLE_FONT_NAMES set, ensureGoogleFontLoaded function, useEffect postMessage listener with style#visual-customizer-style upsert
- `frontend/src/app/overlays/[id]/page.tsx` - Added useCallback import, VisualSettings/visualSettingsToCss imports, visualSettings state, iframeRef, sendCssToIframe, handleVisualSettingsChange, handleIframeReady, config.visual_settings load, visual_settings in save payload, onIframeReady={handleIframeReady} on SplitView
- `frontend/package.json` - react-colorful ^5.6.1 added to dependencies

## Decisions Made
- Google Font names are duplicated as a local constant in the embed page rather than imported from FontFamilyCombobox — the embed page runs in a separate iframe routing context, and importing from a component file just for a constant would create an unnecessary cross-context dependency.
- The visual-customizer-style tag is upserted imperatively (not via React state) to avoid a full component re-render on every CSS update message.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None — TypeScript check was clean after each task with zero new errors.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Live preview wiring complete: all three control groups (Plans 01 and 03) can now call handleVisualSettingsChange to drive live preview
- Plan 03 can add AppearancePanel to the JSX and pass handleVisualSettingsChange as the onChange prop
- react-colorful is available for the color picker controls in Plan 01/03

---
*Phase: 34-appearance-controls-core*
*Completed: 2026-03-18*

## Self-Check: PASSED

All files exist and all task commits verified:
- FOUND: frontend/src/components/SplitView.tsx
- FOUND: frontend/src/app/overlays/[id]/preview/embed/page.tsx
- FOUND: frontend/src/app/overlays/[id]/page.tsx
- FOUND: commit 4cdd0dc (Task 1)
- FOUND: commit 31d4c9d (Task 2)
- FOUND: commit 918cb73 (Task 3)
