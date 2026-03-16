---
phase: 29-viewer-color-gradient-editor
plan: "01"
subsystem: viewer-identity
tags: [gradient, premium, jwt, enricher, typescript, migration, tdd]
dependency_graph:
  requires:
    - "28-06: handlePatchCosmeticsLogic, cosmeticsUpsertRepo interface, ViewerIdentityRepository"
    - "28-01: viewer_cosmetics table, viewers table, JWT ViewerClaims"
  provides:
    - "migrations/036_viewer_gradient.sql: is_premium + name_gradient DB columns"
    - "shared/auth.ViewerClaims.IsPremium: JWT field for premium flag"
    - "shared/middleware: is_premium propagated to gin context"
    - "cosmeticsUpsertRepo: extended with nameGradient []byte param"
    - "PATCH /viewer/cosmetics: gradient acceptance, premium gate, mutual exclusion"
    - "ViewerBadgeEnricher: name_gradient injected into UnifiedChatMessage.User"
    - "frontend NameGradient interface + buildGradientCSS utility"
  affects:
    - "29-02: gradient UI (reads NameGradient type)"
    - "29-03: overlay render (reads msg.User.NameGradient)"
tech_stack:
  added: []
  patterns:
    - "Premium gate via gin context is_premium set by JWT middleware"
    - "Mutual exclusion enforced in handler before DB write"
    - "JSONB stored as []byte, propagated as raw JSON string"
key_files:
  created:
    - migrations/036_viewer_gradient.sql
    - frontend/src/lib/utils/gradient.ts
  modified:
    - shared/auth/jwt.go
    - shared/middleware/auth.go
    - services/auth-service/handlers/viewer_auth.go
    - services/auth-service/handlers/viewer_cosmetics.go
    - services/auth-service/handlers/viewer_cosmetics_test.go
    - services/auth-service/repository/viewer_identity_repository.go
    - services/message-processor/enricher/viewer_badge_enricher.go
    - services/message-processor/enricher/viewer_badge_enricher_test.go
    - services/message-processor/models/message.go
    - frontend/src/lib/types/message.ts
decisions:
  - "Gradient stored as JSONB bytes and propagated as raw JSON string (string([]byte)) — avoids double-parse in enricher hot path"
  - "Mutual exclusion enforced in handler (gradient presence zeroes nameColor before DB write) — DB stores both columns but only one is ever non-null at a time"
  - "is_premium read from gin context (set by JWT middleware) not re-queried from DB in handler — consistent with established middleware pattern"
  - "GetViewerIsPremium added to ViewerIdentityRepository for generateViewerJWT; soft-fail defaults to false to avoid blocking auth on DB hiccup"
  - "fakeViewerDB in enricher tests extended to 3-column return to match new SELECT — noGradientDB helper wraps old 2-return callers for backward compatibility"
metrics:
  duration_seconds: 451
  tasks_completed: 2
  tasks_total: 2
  files_created: 2
  files_modified: 10
  completed_date: "2026-03-15"
---

# Phase 29 Plan 01: DB Migration, Go Type Extensions, and TypeScript NameGradient Type Summary

**One-liner:** DB schema additions (is_premium + name_gradient JSONB), JWT/middleware premium propagation, PATCH handler gradient validation with premium gate, enricher gradient injection, and TypeScript NameGradient contract.

## Tasks Completed

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | DB migration 036 + Go type extensions | c24954a | migrations/036_viewer_gradient.sql, shared/auth/jwt.go, shared/middleware/auth.go, viewer_auth.go, viewer_cosmetics.go, viewer_identity_repository.go, viewer_cosmetics_test.go |
| 2 | Enricher gradient propagation + TypeScript NameGradient type | a141e0e | viewer_badge_enricher.go, models/message.go, viewer_badge_enricher_test.go, message.ts, gradient.ts |

## What Was Built

### Task 1: DB migration 036 + Go type extensions

**Migration** (`migrations/036_viewer_gradient.sql`): Adds `is_premium BOOLEAN NOT NULL DEFAULT FALSE` to `viewers` and `name_gradient JSONB` (nullable) to `viewer_cosmetics`.

**JWT** (`shared/auth/jwt.go`): `ViewerClaims` gains `IsPremium bool \`json:"is_premium"\``.

**Middleware** (`shared/middleware/auth.go`): `JWTAuth` now calls `c.Set("is_premium", viewerClaims.IsPremium)` in the viewer token branch. This is load-bearing: `handlePatchCosmeticsLogic` reads `"is_premium"` from gin context for the 403 premium gate.

**Repository** (`viewer_identity_repository.go`):
- `UpsertViewerCosmetics` signature extended to `(ctx, viewerID, nameColor *string, nameGradient []byte) error`; SQL updated with `name_gradient = EXCLUDED.name_gradient`
- `GetViewerIsPremium(ctx, viewerID) (bool, error)` added — `SELECT is_premium FROM viewers WHERE id = $1`

**Auth handler** (`viewer_auth.go`): `generateViewerJWT` now calls `identityRepo.GetViewerIsPremium` when `viewerID != uuid.Nil`, soft-failing to false on DB error.

**Cosmetics handler** (`viewer_cosmetics.go`):
- `NameGradientReq` struct added: `{ Type string; Colors []string; Angle int }`
- `patchCosmeticsRequest` gains `NameGradient *NameGradientReq`
- `cosmeticsUpsertRepo` interface updated to include `nameGradient []byte`
- `handlePatchCosmeticsLogic` validates gradient (type=="linear", 2-4 colors each `#rrggbb`, angle 0-360), enforces mutual exclusion (gradient zeroes nameColor; nameColor zeroes gradient), gates on `is_premium` from context (403 for non-premium), returns `{name_color, name_gradient}` on 200

**Tests** (`viewer_cosmetics_test.go`): 10 tests all GREEN:
- `TestPatchCosmetics_GradientAccepted` — premium viewer + valid gradient → 200
- `TestPatchCosmetics_GradientRejectedNonPremium` — non-premium + gradient → 403
- `TestPatchCosmetics_GradientValidation` — 1 color → 400
- `TestPatchCosmetics_GradientValidation_BadAngle` — angle 400 → 400
- `TestPatchCosmetics_MutualExclusion` — gradient sets name_color nil in DB call + response

### Task 2: Enricher gradient propagation + TypeScript NameGradient type

**Models** (`models/message.go`): `UserInfo.NameGradient string \`json:"name_gradient,omitempty"\`` added.

**Enricher** (`viewer_badge_enricher.go`):
- `viewerIdentityCache` gains `NameGradient []byte \`json:"name_gradient,omitempty"\``
- DB SELECT now includes `vc.name_gradient`; `row.Scan` has 3 destinations
- Cache-hit and DB paths both propagate gradient to `msg.User.NameGradient = string(nameGradientBytes)` with nil guard

**Enricher tests** (`viewer_badge_enricher_test.go`): Extended `fakeViewerDB` to 3-column return; added `noGradientDB` wrapper for existing tests; added `TestEnrich_PropagatesNameGradient` and `TestEnrich_PropagatesNameGradient_FromCache` — both GREEN.

**TypeScript** (`frontend/src/lib/types/message.ts`):
- `export interface NameGradient { type: 'linear'; colors: string[]; angle: number }`
- `name_gradient?: NameGradient` added to `UserInfo`

**Gradient utility** (`frontend/src/lib/utils/gradient.ts`):
- `export function buildGradientCSS(g: NameGradient): string` — returns `linear-gradient(${g.angle}deg, ${g.colors.join(', ')})`

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Compatibility] Updated existing enricher test fakeViewerDB to 3-column signature**
- **Found during:** Task 2
- **Issue:** Existing `fakeViewerDB.queryFn` returned `(string, *string, error)` (2 columns); new SELECT adds `vc.name_gradient` requiring 3 columns in Scan
- **Fix:** Extended `fakeViewerDB.queryFn` signature to `(string, *string, []byte, error)`; added `noGradientDB` helper so all existing callers continue working without change
- **Files modified:** `services/message-processor/enricher/viewer_badge_enricher_test.go`
- **Commit:** a141e0e

None of the other deviations occurred — plan executed closely as written.

## Self-Check: PASSED

- migrations/036_viewer_gradient.sql: FOUND
- frontend/src/lib/utils/gradient.ts: FOUND
- 29-01-SUMMARY.md: FOUND
- Commit c24954a (Task 1): FOUND
- Commit a141e0e (Task 2): FOUND
- All Go builds: PASS
- All enricher tests: PASS (14 tests)
- All cosmetics tests: PASS (10 tests)
- Frontend tsc --noEmit: PASS
