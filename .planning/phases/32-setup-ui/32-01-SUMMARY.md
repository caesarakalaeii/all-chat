---
phase: 32-setup-ui
plan: "01"
subsystem: overlay-manager, frontend-types, frontend-api
tags: [discord, patch-endpoint, type-system, design-tokens, api-client]
dependency_graph:
  requires: []
  provides:
    - PATCH /api/v1/overlays/:id/sources/:source_id endpoint
    - DiscordSourceConfig type
    - discord entry in PLATFORM_COLORS
    - --color-discord CSS token
    - apiClient.patch method
    - discord.ts API module (getGuilds, getGuildChannels, disconnectGuild, startDiscordOAuth, updateSourceConfig)
  affects:
    - Plans 32-02 and 32-03 (depend on PATCH endpoint, types, and discord.ts)
tech_stack:
  added: []
  patterns:
    - TDD red-green-refactor for Go handler and frontend modules
    - Interface-first design for SourceRepository (UpdateConfig added)
    - Literal class strings in PLATFORM_COLORS for Tailwind JIT safety
key_files:
  created:
    - services/overlay-manager/handlers/sources_patch_test.go
    - frontend/src/lib/__tests__/types.test.ts
    - frontend/src/lib/api/discord.ts
    - frontend/src/lib/api/__tests__/discord.test.ts
  modified:
    - services/overlay-manager/repository/source_repo.go
    - services/overlay-manager/handlers/sources.go
    - services/overlay-manager/handlers/sources_shared_overlay_test.go
    - services/overlay-manager/handlers/overlay_test.go
    - services/overlay-manager/cmd/main.go
    - frontend/next.config.js
    - frontend/src/lib/api/client.ts
    - frontend/src/lib/types/overlay.ts
    - frontend/src/lib/platform-colors.ts
    - frontend/src/app/globals.css
    - frontend/src/lib/__tests__/platform-colors.test.ts
decisions:
  - "HandleUpdateSourceConfig returns 403 (not 404) on ownership mismatch — consistent with plan spec; distinguishes auth failure from resource absence"
  - "mockSourceRepository in sources_shared_overlay_test.go updated with UpdateConfig stub to satisfy expanded SourceRepository interface"
  - "mockOverlayRepository in overlay_test.go updated with UnsetAllPublicForUser stub to fix pre-existing compilation error"
metrics:
  duration: "338s"
  completed_date: "2026-03-16"
  tasks_completed: 3
  files_modified: 11
  files_created: 4
---

# Phase 32 Plan 01: Foundation — Backend PATCH Endpoint, Discord Types, and API Module Summary

**One-liner:** PATCH source config endpoint in overlay-manager + Discord platform type system + discord.ts API module with 5 exported functions, all test-driven.

## What Was Built

### Task 1: PATCH Source Config Endpoint (overlay-manager)

`UpdateConfig(ctx, id, config)` method added to `SourceRepository` and its concrete implementation in `source_repo.go`. The SQL is:

```sql
UPDATE overlay_chat_sources SET config = $2, updated_at = NOW() WHERE id = $1
```

`HandleUpdateSourceConfig` handler in `handlers/sources.go`:
- Extracts `user_id` from JWT context
- Verifies overlay ownership via `overlayRepo.GetByIDAndUserID` (403 if not found)
- Parses `{ "config": map[string]interface{} }` body (400 if config missing)
- Calls `sourceRepo.UpdateConfig` (500 on error)
- Returns 200 `{ "message": "config updated" }`

Route registered: `protected.PATCH("/:id/sources/:source_id", sourcesHandler.HandleUpdateSourceConfig)`

### Task 2a: Type System and Design Tokens (frontend)

- `--color-discord: #5865F2` added to `globals.css` `@theme` block
- `PLATFORM_COLORS.discord = { text: 'text-discord', bg: 'bg-discord' }` added as literal strings
- `ChatSource.platform` and `AddSourceRequest.platform` unions extended with `'discord'`
- `DiscordSourceConfig` interface exported from `types/overlay.ts`
- `apiClient.patch<T>` method added to `ApiClient` class
- `cdn.discordapp.com` added to `next.config.js` `images.domains`

### Task 2b: discord.ts API Module (frontend)

New file `src/lib/api/discord.ts` exports:
- `getGuilds()` → GET `/api/v1/auth/guilds`
- `getGuildChannels(guildId)` → GET `/api/v1/auth/guilds/{id}/channels`
- `disconnectGuild(guildId)` → DELETE `/api/v1/auth/guilds/{id}`
- `startDiscordOAuth()` → GET `/api/v1/auth/discord/connect` + redirect
- `updateSourceConfig(overlayId, sourceId, config)` → PATCH `/api/v1/overlays/{id}/sources/{sourceId}`

## Verification Results

```
overlay-manager: go build ./... ✓
overlay-manager: go test ./...  ✓ (handlers, models, repository, creditroll)
frontend: npx tsc --noEmit      ✓ (no errors)
frontend: vitest unit (34 tests) ✓ (platform-colors: 13, types: 2, discord: 4, ui: 15)
```

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added UnsetAllPublicForUser stub to mockOverlayRepository**
- **Found during:** Task 1 (RED phase compilation)
- **Issue:** `mockOverlayRepository` in `overlay_test.go` did not implement `OverlayRepository.UnsetAllPublicForUser`, causing all handler tests to fail compilation
- **Fix:** Added no-op stub `func (m *mockOverlayRepository) UnsetAllPublicForUser(...)` to `overlay_test.go`
- **Files modified:** `services/overlay-manager/handlers/overlay_test.go`
- **Commit:** 471b7a8

**2. [Rule 3 - Blocking] Added UpdateConfig stub to mockSourceRepository**
- **Found during:** Task 1 (GREEN phase — interface expansion)
- **Issue:** `mockSourceRepository` in `sources_shared_overlay_test.go` needed `UpdateConfig` after the `SourceRepository` interface was extended
- **Fix:** Added `UpdateConfig` no-op stub to `sources_shared_overlay_test.go`
- **Files modified:** `services/overlay-manager/handlers/sources_shared_overlay_test.go`
- **Commit:** 471b7a8

## Self-Check: PASSED

All 4 created files exist on disk. All 3 task commits found in git log.

| Item | Status |
|------|--------|
| sources_patch_test.go | FOUND |
| types.test.ts | FOUND |
| discord.ts | FOUND |
| discord.test.ts | FOUND |
| 32-01-SUMMARY.md | FOUND |
| commit 471b7a8 (Task 1) | FOUND |
| commit f7e72d8 (Task 2a) | FOUND |
| commit 2145a04 (Task 2b) | FOUND |
