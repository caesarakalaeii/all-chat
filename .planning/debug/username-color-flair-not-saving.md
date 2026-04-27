---
status: verifying
trigger: "username color flair not saving — always reverts to purple"
created: 2026-03-30T00:00:00Z
updated: 2026-03-30T00:00:00Z
---

## Current Focus

hypothesis: CONFIRMED — UpsertViewerCosmetics always overwrites all 4 columns. Saving avatar cosmetics sends name_color=null to the UPSERT, which overwrites the stored name_color with NULL in the DB. On reload, GET returns name_color=null, frontend keeps the default #9146ff (purple).
test: traced full call stack from frontend save → backend handler → repository SQL
expecting: fix to be partial-update semantics for name_color/name_gradient when avatar-only PATCH
next_action: implement fix

## Symptoms

expected: When a user sets their username color flair in viewer settings, the chosen color should persist across page loads and sessions.
actual: The color always reverts back to purple regardless of what the user selects.
errors: None reported
reproduction: Go to viewer settings, change username color flair to any color other than purple, save. Reload or revisit — color is back to purple.
started: Unknown — reported by user

## Eliminated

(none)

## Evidence

- timestamp: 2026-03-30T00:00:00Z
  checked: frontend/src/app/settings/viewer/page.tsx — ColorGradientCard + AvatarCosmeticsCard
  found: Two separate Cards share one PATCH /api/v1/auth/viewer/cosmetics endpoint. ColorGradientCard sends {name_color, name_gradient}. AvatarCosmeticsCard sends {avatar_frame_id, avatar_flair_id} with NO name_color/name_gradient fields.
  implication: When AvatarCosmeticsCard.handleSave runs, name_color/name_gradient are absent from the body.

- timestamp: 2026-03-30T00:00:00Z
  checked: services/auth-service/handlers/viewer_cosmetics.go — handlePatchCosmeticsLogic
  found: When name_color and name_gradient are both absent (nil), nameGradientBytes stays nil. Both are passed to UpsertViewerCosmetics as nil.
  implication: The upsert receives nil for both name cosmetics columns regardless of what's in the DB.

- timestamp: 2026-03-30T00:00:00Z
  checked: services/auth-service/repository/viewer_identity_repository.go — UpsertViewerCosmetics
  found: The SQL uses ON CONFLICT DO UPDATE SET name_color=EXCLUDED.name_color, name_gradient=EXCLUDED.name_gradient. When nil is passed, these become NULL in the DB.
  implication: Saving avatar cosmetics (frame/flair) ALWAYS NULLs out the stored name_color and name_gradient. On next GET, name_color=null → frontend falls back to #9146ff (purple).

- timestamp: 2026-03-30T00:00:00Z
  checked: shared/auth/jwt.go — ViewerClaims struct
  found: ViewerClaims does NOT include name_color or name_gradient fields, despite the frontend ViewerJWTClaims interface declaring them.
  implication: claims.name_color is always undefined in the frontend. The initial nameColor state is always '#9146ff'. If the GET fetch fails, color stays purple permanently.

## Resolution

root_cause: UpsertViewerCosmetics always overwrites all 4 columns (name_color, name_gradient, avatar_frame_id, avatar_flair_id). AvatarCosmeticsCard.handleSave sends a PATCH with only avatar_frame_id/avatar_flair_id — no name fields. Go JSON parsing cannot distinguish absent fields from JSON null, so name_color and name_gradient arrive as nil at the DB layer, overwriting the stored name_color with NULL. On reload GET returns name_color=null, frontend falls back to the hardcoded default #9146ff (purple).

fix: |
  Backend (services/auth-service):
  1. Changed patchCosmeticsRaw to use json.RawMessage for each field, enabling absent vs explicit-null detection via field presence flags.
  2. Added UpsertAvatarCosmetics repository method that only updates avatar_frame_id and avatar_flair_id, leaving name_color and name_gradient untouched.
  3. handlePatchCosmeticsLogic now routes to UpsertAvatarCosmetics when name_color and name_gradient are both absent from the body.

  Frontend (frontend/src/app/settings/viewer/page.tsx):
  4. AvatarCosmeticsCard now fetches saved avatar_frame_id and avatar_flair_id from GET /cosmetics on mount (alongside catalog fetch), so the correct selection is highlighted after page reload.

verification: All 16 cosmetics handler unit tests pass. TypeScript type-check clean. Build clean.
files_changed:
  - services/auth-service/handlers/viewer_cosmetics.go
  - services/auth-service/handlers/viewer_cosmetics_test.go
  - services/auth-service/repository/viewer_identity_repository.go
  - services/auth-service/repository/viewer_identity_repository_test.go
  - frontend/src/app/settings/viewer/page.tsx
