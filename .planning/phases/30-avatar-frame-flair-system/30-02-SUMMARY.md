---
phase: 30-avatar-frame-flair-system
plan: "02"
subsystem: auth-service
tags: [cosmetics, admin-api, premium-gate, handler, tdd]
dependency_graph:
  requires:
    - "30-01"  # DB schema + UpsertViewerCosmetics signature
  provides:
    - admin_cosmetics_crud_api
    - public_catalog_read_api
    - extended_patch_cosmetics_handler
  affects:
    - auth-service
tech_stack:
  added: []
  patterns:
    - cosmeticsCatalogDB interface for testable handler without concrete pgxpool
    - pgxPoolAdapter wraps *pgxpool.Pool to satisfy interface
    - Premium gate using is_premium from gin context (set by JWT middleware)
    - Downgrade enforcement sentinel: &uuid.Nil passed to UPSERT clears DB column
key_files:
  created:
    - services/auth-service/handlers/admin_cosmetics.go
    - services/auth-service/handlers/admin_cosmetics_test.go
  modified:
    - services/auth-service/handlers/viewer_cosmetics.go
    - services/auth-service/handlers/viewer_cosmetics_test.go
    - services/auth-service/cmd/main.go
decisions:
  - cosmeticsCatalogDB interface + pgxPoolAdapter pattern mirrors viewerDB/pgxPoolAdapter from Phase 28 — consistent testability approach across handlers
  - downgrade enforcement passes &uuid.Nil (not nil pointer) so UPSERT writes NULL explicitly; nil pointer would also work in this UPSERT but sentinel is self-documenting
  - public catalog endpoints reuse HandleListFrames/HandleListFlairs methods — list is read-only and catalog data is not sensitive
  - pre-existing handler test failures (Redis unavailable, nil repo in viewer_exchange_test) are out of scope infrastructure issues unchanged by this plan
metrics:
  duration: "~5min"
  completed_date: "2026-03-16"
  tasks: 2
  files_changed: 5
---

# Phase 30 Plan 02: Auth-Service Backend for Avatar Frame/Flair System Summary

Admin catalog CRUD API and extended viewer PATCH handler with premium gating for avatar frames and flairs.

## What Was Built

### Task 1: AdminCosmeticsHandler + Test Scaffold

Created `admin_cosmetics.go` implementing a full catalog management handler with six methods:
- `HandleListFrames` / `HandleListFlairs` — returns `[]CosmeticCatalogEntry` (empty array, never null), ordered by `created_at ASC`
- `HandleCreateFrame` / `HandleCreateFlair` — validates name and image_url required, returns 201 with created entry
- `HandleDeleteFrame` / `HandleDeleteFlair` — UUID path param, returns 204 on success, 404 when not found

The handler accepts a `cosmeticsCatalogDB` interface (not a concrete `*pgxpool.Pool`) for testability. A `pgxPoolAdapter` wraps the pool at construction. A `newAdminCosmeticsHandlerWithDB` factory allows tests to inject a mock.

`admin_cosmetics_test.go` provides 12 unit tests covering all six endpoints with a `mockCosmeticsCatalogDB` that implements `pgx.Rows` and `pgx.Row` interfaces directly — no real DB required.

Fixed `viewer_cosmetics_test.go` mock: `UpsertViewerCosmetics` had the old 4-param signature from before Plan 01 expanded it to 6 params (added `avatarFrameID`, `avatarFlairID`).

### Task 2: Extended PATCH Handler + Route Wiring

Extended `patchCosmeticsRequest` with:
```go
AvatarFrameID *uuid.UUID `json:"avatar_frame_id"`
AvatarFlairID *uuid.UUID `json:"avatar_flair_id"`
```
(No `omitempty` — null and absent must be distinguishable.)

Added premium gate logic in `handlePatchCosmeticsLogic` (Step 5b):
- Non-premium viewer sending a non-zero UUID for frame or flair → 403
- Non-premium viewer (any body) → downgrade enforcement: passes `&uuid.Nil` to UPSERT to write NULL to DB
- Premium viewer → passes request values through directly

Response now includes `avatar_frame_id` and `avatar_flair_id` fields.

Wired routes in `cmd/main.go`:
- Public: `GET /viewer/catalog/frames` and `GET /viewer/catalog/flairs` (no JWT)
- Admin: `GET/POST/DELETE /admin/cosmetics/frames` and `/admin/cosmetics/flairs`

Added 5 new test cases covering premium acceptance, non-premium gate (frame), non-premium gate (flair), downgrade clear sentinel, and response field inclusion.

## Verification Results

```
TestAdminCosmeticsFrames_List_Empty         PASS
TestAdminCosmeticsFrames_List_WithEntries   PASS
TestAdminCosmeticsFrames_Create_Valid       PASS
TestAdminCosmeticsFrames_Create_MissingName PASS
TestAdminCosmeticsFrames_Create_MissingImageURL PASS
TestAdminCosmeticsFrames_Delete_Exists      PASS
TestAdminCosmeticsFrames_Delete_NotFound    PASS
TestAdminCosmeticsFlairs_List_Empty         PASS
TestAdminCosmeticsFlairs_Create_Valid       PASS
TestAdminCosmeticsFlairs_Create_MissingName PASS
TestAdminCosmeticsFlairs_Delete_Exists      PASS
TestAdminCosmeticsFlairs_Delete_NotFound    PASS
TestPatchCosmetics_ValidColor               PASS
TestPatchCosmetics_NullColor                PASS
TestPatchCosmetics_InvalidHex               PASS
TestPatchCosmetics_MissingViewerID          PASS
TestPatchCosmetics_InvalidViewerIDFormat    PASS
TestPatchCosmetics_GradientAccepted         PASS
TestPatchCosmetics_GradientRejectedNonPremium PASS
TestPatchCosmetics_GradientValidation       PASS
TestPatchCosmetics_GradientValidation_BadAngle PASS
TestPatchCosmetics_MutualExclusion          PASS
TestPatchCosmetics_AvatarFrameID_PremiumAccepted PASS
TestPatchCosmetics_AvatarFrameID_NonPremiumRejected PASS
TestPatchCosmetics_AvatarFlairID_NonPremiumRejected PASS
TestPatchCosmetics_NonPremium_DowngradeClears PASS
TestPatchCosmetics_AvatarFrameID_ResponseIncludes PASS

go build ./... — PASS
```

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fixed viewer_cosmetics_test.go mock signature mismatch**
- **Found during:** Task 1 (first build attempt after writing admin_cosmetics_test.go)
- **Issue:** `mockCosmeticsUpsertRepo.UpsertViewerCosmetics` had the old 4-parameter signature from before Plan 01 added `avatarFrameID *uuid.UUID` and `avatarFlairID *uuid.UUID` parameters to the `cosmeticsUpsertRepo` interface. The entire handlers package failed to compile.
- **Fix:** Updated mock struct fields and method signature in `viewer_cosmetics_test.go` to match the new 6-parameter interface.
- **Files modified:** `services/auth-service/handlers/viewer_cosmetics_test.go`
- **Commit:** `614e50b`

## Commits

| Hash | Description |
|------|-------------|
| `614e50b` | feat(30-02): add AdminCosmeticsHandler with catalog CRUD for frames and flairs |
| `4f59fba` | feat(30-02): extend PATCH cosmetics with avatar frame/flair + wire all routes |

## Self-Check: PASSED

All files created/modified exist on disk. Both task commits confirmed in git log.
