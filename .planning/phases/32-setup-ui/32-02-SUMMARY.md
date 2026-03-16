---
phase: 32-setup-ui
plan: 02
subsystem: ui
tags: [react, nextjs, typescript, discord, settings]

# Dependency graph
requires:
  - phase: 32-setup-ui-01
    provides: "discord.ts API client with getGuilds, disconnectGuild, startDiscordOAuth, DiscordGuild type"
provides:
  - "Settings page Discord section card: connect, view connected guilds, and disconnect with confirmation"
affects: [32-setup-ui-03, settings]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Controlled Dialog pattern via disconnectTarget state — single dialog instance per guild list, driven by guild state"
    - "?discord=connected query param detection with useSearchParams + router.replace to clean URL after OAuth callback"

key-files:
  created: []
  modified:
    - "frontend/src/app/settings/page.tsx"

key-decisions:
  - "Controlled Dialog via disconnectTarget state: one Dialog.Root per guild row with open={disconnectTarget?.guild_id === guild.guild_id} avoids mounting multiple portal roots"
  - "fetchGuilds silent error handling: getGuilds errors are silently swallowed since user may simply not have Discord connected yet"

patterns-established:
  - "OAuth callback param handling: useSearchParams detects ?platform=connected param, shows toast, calls router.replace to clean URL, re-fetches data"
  - "Disconnected/loading/connected state trio: guildsLoading shows Skeleton, guilds.length===0 shows connect CTA, guilds.length>0 shows list"

requirements-completed: [UI-01]

# Metrics
duration: 5min
completed: 2026-03-16
---

# Phase 32 Plan 02: Settings Discord Card Summary

**Discord server connect card in settings page: connect via OAuth, view connected guilds with icon/initial fallback, and disconnect with Dialog confirmation**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-03-16T10:30:00Z
- **Completed:** 2026-03-16T10:35:00Z
- **Tasks:** 1 auto (1 checkpoint pending human verify)
- **Files modified:** 1

## Accomplishments
- Discord section card added to settings page after Data & Privacy, before Danger Zone
- Disconnected state shows "Connect Discord Server" button that calls startDiscordOAuth
- Loading state shows Skeleton row while guilds fetch
- Connected state shows one row per guild: icon (CDN) or initial fallback, server name, Disconnect button
- Disconnect button opens Dialog confirmation before calling disconnectGuild and removing guild from local state
- "Connect another server" ghost button shown below connected guild rows
- ?discord=connected query param triggers success toast, URL cleanup via router.replace, and guild re-fetch

## Task Commits

Each task was committed atomically:

1. **Task 1: Settings page Discord server connect card** - `0c4c54c` (feat)

## Files Created/Modified
- `frontend/src/app/settings/page.tsx` - Added Discord section card with full connect/disconnect UX

## Decisions Made
- Controlled Dialog via `disconnectTarget` state: one `Dialog.Root` per guild row with `open={disconnectTarget?.guild_id === guild.guild_id}` avoids mounting multiple portal roots while keeping the confirmation dialog co-located with each row.
- `fetchGuilds` silently swallows errors since the user may not have Discord connected yet — no red error state shown.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Settings page Discord card is complete and ready for human visual verification (Task 2 checkpoint)
- After checkpoint approval, plan 32-03 can proceed with the Discord source configuration panel in overlay editor

---
*Phase: 32-setup-ui*
*Completed: 2026-03-16*
