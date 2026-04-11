---
phase: 11-add-username-keyword-exclude-list-to-overlay-filter-settings
plan: "01"
subsystem: frontend-filtering
tags: [filtering, overlay, tdd, backend-fix]
dependency_graph:
  requires: []
  provides:
    - shouldFilterMessage utility (frontend/src/lib/utils/filterMessage.ts)
    - filter_settings in public config endpoint
    - filter application in live overlay page
  affects:
    - frontend/src/app/overlay/[id]/page.tsx
    - services/overlay-manager/handlers/config.go
tech_stack:
  added: []
  patterns:
    - TDD (red-green) for pure utility function
    - ref pattern for closure-safe state access in WebSocket onmessage
key_files:
  created:
    - frontend/src/lib/utils/filterMessage.ts
    - frontend/src/lib/utils/__tests__/filterMessage.test.ts
  modified:
    - services/overlay-manager/handlers/config.go
    - frontend/src/app/overlay/[id]/page.tsx
decisions:
  - filterSettingsRef used instead of filterSettings state in ws.onmessage closure to avoid stale closure captures without adding filterSettings to WebSocket effect deps
  - filter_settings added to public config endpoint so unauthenticated overlay page can load it without auth
  - Silent catch around RegExp constructor avoids crashing overlay on invalid user-supplied regex (ReDoS self-only, accepted per T-11-02)
metrics:
  duration: ~3 minutes
  completed_date: "2026-04-11T23:53:11Z"
  tasks_completed: 2
  files_changed: 4
requirements:
  - D-01
  - D-02
  - D-03
  - D-04
  - D-05
  - D-06
---

# Phase 11 Plan 01: shouldFilterMessage Utility + Backend Fix + Overlay Wiring Summary

**One-liner:** Pure shouldFilterMessage utility with TDD (17 tests), public config endpoint fixed to return filter_settings, and live overlay page wired to filter incoming messages before render.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Create shouldFilterMessage utility with TDD | cacfa68 | frontend/src/lib/utils/filterMessage.ts, frontend/src/lib/utils/__tests__/filterMessage.test.ts |
| 2 | Add filter_settings to public config endpoint and wire filtering into overlay page | 2364009 | services/overlay-manager/handlers/config.go, frontend/src/app/overlay/[id]/page.tsx |

## What Was Built

### Task 1: shouldFilterMessage Utility (TDD)

Created `frontend/src/lib/utils/filterMessage.ts` — a pure function with no side effects:

- **D-04**: Exact case-insensitive match of `banned_users` against both `user.username` and `user.display_name`
- **D-03**: Regex keyword matching (case-insensitive `new RegExp(pattern, 'i')`) against message text; invalid patterns silently skipped via `try/catch`
- **D-05**: `hide_commands` flag filters messages starting with `!`
- **D-06**: `min_message_length` filters messages shorter than the threshold (0 = disabled)
- Returns `false` immediately when `settings` is null/undefined/empty

17 unit tests written first (RED), then implementation (GREEN). No refactor needed.

### Task 2: Backend Fix + Frontend Wiring

**Backend** (`services/overlay-manager/handlers/config.go`): Added `"filter_settings": config.FilterSettings` to the `HandleGetPublicConfig` response. Previously the public config endpoint omitted filter_settings entirely, meaning filters configured in the editor would silently never apply in OBS/browser overlays.

**Frontend** (`frontend/src/app/overlay/[id]/page.tsx`):
- Imported `shouldFilterMessage` and `FilterSettings`
- Added `filterSettings` state and `filterSettingsRef` ref (ref pattern for closure safety)
- Added `useEffect` to keep `filterSettingsRef.current` in sync with state
- In `loadConfig`: reads `data.filter_settings` and sets both state and ref immediately
- In `ws.onmessage`: calls `shouldFilterMessage(message, filterSettingsRef.current)` and returns early if the message should be filtered — before `setMessages`

## Verification

```
Tests:  17 passed (17)  — npx vitest run src/lib/utils/__tests__/filterMessage.test.ts
Build:  Go build OK     — cd services/overlay-manager && go build ./...
```

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None. Filter settings are fully wired from backend to frontend.

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| threat_flag: information_disclosure | services/overlay-manager/handlers/config.go | filter_settings (banned usernames/words) now exposed via unauthenticated public config endpoint — accepted per T-11-01 (no credentials, overlay URL is unguessable UUID) |

## Self-Check: PASSED

- `frontend/src/lib/utils/filterMessage.ts` — FOUND
- `frontend/src/lib/utils/__tests__/filterMessage.test.ts` — FOUND
- Commit `cacfa68` — FOUND
- Commit `2364009` — FOUND
