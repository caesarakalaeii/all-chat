---
phase: 30-avatar-frame-flair-system
plan: "01"
subsystem: schema-and-types
tags: [migration, repository, go-models, typescript, cosmetics, avatar-frame, avatar-flair]
dependency_graph:
  requires: []
  provides:
    - cosmetic_frames catalog table (migration 037)
    - cosmetic_flairs catalog table (migration 037)
    - avatar_frame_id / avatar_flair_id FK columns on viewer_cosmetics
    - UpsertViewerCosmetics 6-param signature
    - cosmeticsUpsertRepo interface matching new signature
    - AvatarFrameURL / AvatarFlairURL on Go UserInfo
    - avatar_frame_url? / avatar_flair_url? on frontend TypeScript UserInfo
    - name_gradient? / avatar_frame_url? / avatar_flair_url? on extension TypeScript UserInfo
  affects:
    - services/auth-service/handlers/viewer_cosmetics_test.go (mock will fail to compile — Plan 02 fixes)
tech_stack:
  added: []
  patterns:
    - IF NOT EXISTS / ADD COLUMN IF NOT EXISTS idempotent migration
    - ON DELETE SET NULL for catalog FK references
    - Extended repository method signature with nullable *uuid.UUID params
    - Nil-pass-through call site (Plan 02 wires real values)
key_files:
  created:
    - migrations/037_cosmetics_catalog.sql
  modified:
    - services/auth-service/repository/viewer_identity_repository.go
    - services/auth-service/handlers/viewer_cosmetics.go
    - services/message-processor/models/message.go
    - frontend/src/lib/types/message.ts
    - all-chat-extension/src/lib/types/message.ts
decisions:
  - avatar_frame_id and avatar_flair_id use ON DELETE SET NULL so admin catalog deletes gracefully clear viewer selections
  - UpsertViewerCosmetics call site passes nil, nil for new params — Plan 02 expands to pass real values
  - name_gradient added to extension UserInfo in this plan (previously missing from extension)
metrics:
  duration: "~2.5 minutes"
  completed: "2026-03-16"
  tasks_completed: 2
  files_created: 1
  files_modified: 5
---

# Phase 30 Plan 01: DB Schema and Type Contracts for Avatar Frame / Flair Summary

**One-liner:** Migration 037 creates cosmetic_frames and cosmetic_flairs catalog tables with FK columns on viewer_cosmetics, extended UpsertViewerCosmetics to 6-param signature, and added avatar_frame_url/avatar_flair_url to Go and TypeScript type contracts in both repos.

## What Was Built

### Task 1: Migration 037 — catalog tables and FK columns

Created `/home/moersener/Hobby/all-chat/migrations/037_cosmetics_catalog.sql` with:
- `CREATE TABLE IF NOT EXISTS cosmetic_frames` (id UUID PK, name, image_url, is_premium, created_at)
- `CREATE TABLE IF NOT EXISTS cosmetic_flairs` (id UUID PK, name, image_url, is_premium, created_at)
- `ALTER TABLE viewer_cosmetics ADD COLUMN IF NOT EXISTS avatar_frame_id UUID REFERENCES cosmetic_frames(id) ON DELETE SET NULL`
- `ALTER TABLE viewer_cosmetics ADD COLUMN IF NOT EXISTS avatar_flair_id UUID REFERENCES cosmetic_flairs(id) ON DELETE SET NULL`

### Task 2: Extend signatures and type contracts

**services/auth-service/repository/viewer_identity_repository.go:**
- Extended `UpsertViewerCosmetics` to accept `avatarFrameID *uuid.UUID` and `avatarFlairID *uuid.UUID` (6-param signature)
- Updated SQL INSERT/ON CONFLICT SET to include `avatar_frame_id` and `avatar_flair_id` columns

**services/auth-service/handlers/viewer_cosmetics.go:**
- Updated `cosmeticsUpsertRepo` interface to match new 6-param signature
- Updated call site to pass `nil, nil` for the two new params (Plan 02 will wire in real UUID values)

**services/message-processor/models/message.go:**
- Added `AvatarFrameURL string \`json:"avatar_frame_url,omitempty"\`` to `UserInfo`
- Added `AvatarFlairURL string \`json:"avatar_flair_url,omitempty"\`` to `UserInfo`

**frontend/src/lib/types/message.ts:**
- Added `avatar_frame_url?: string` to `UserInfo` interface
- Added `avatar_flair_url?: string` to `UserInfo` interface

**all-chat-extension/src/lib/types/message.ts:**
- Added `name_gradient?: string` (previously missing from extension)
- Added `avatar_frame_url?: string` to `UserInfo` interface
- Added `avatar_flair_url?: string` to `UserInfo` interface

## Decisions Made

1. **ON DELETE SET NULL** for catalog FK references — admin can delete a catalog frame/flair entry and all viewer selections are cleared gracefully without orphan rows or errors.

2. **Nil-pass-through call site** — `handlePatchCosmeticsLogic` passes `nil, nil` for `avatarFrameID` and `avatarFlairID`. Plan 02 will add `AvatarFrameID` / `AvatarFlairID` to `patchCosmeticsRequest` and wire real values through.

3. **Extension name_gradient added here** — the extension `UserInfo` was missing `name_gradient` from Phase 29. This plan catches it up alongside the new frame/flair fields.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1    | 2696c99 | chore(30-01): add migration 037 cosmetics catalog tables and FK columns |
| 2    | cec932d | feat(30-01): extend UpsertViewerCosmetics signature and type contracts for avatar frame/flair |
| 2    | 22f77f9 (extension repo) | feat(30-01): add name_gradient, avatar_frame_url, avatar_flair_url to extension UserInfo |

## Known Follow-On Work

- `services/auth-service/handlers/viewer_cosmetics_test.go` mock will fail to compile after this plan — Plan 02 explicitly fixes the test mock to match the new 6-param interface.

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check: PASSED

- migrations/037_cosmetics_catalog.sql: FOUND
- services/auth-service/repository/viewer_identity_repository.go: FOUND + builds
- services/auth-service/handlers/viewer_cosmetics.go: FOUND + builds
- services/message-processor/models/message.go: FOUND + builds
- frontend/src/lib/types/message.ts: FOUND + contains avatar_frame_url
- all-chat-extension/src/lib/types/message.ts: FOUND + contains avatar_frame_url + name_gradient
