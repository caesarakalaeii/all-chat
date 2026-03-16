---
phase: 31-all-chat-platform-badges
plan: "01"
subsystem: message-processor/enricher
tags: [badges, enricher, migration, tdd, viewer-identity]
dependency_graph:
  requires:
    - Phase 30 viewer_cosmetics schema (cosmetic_frames, cosmetic_flairs, viewer_cosmetics)
    - Phase 28 viewer_platform_identities, viewer_sessions tables
  provides:
    - badge_definitions catalog table (DDL + seed)
    - ViewerBadgeEnricher with is_admin/is_premium badge injection
    - viewerIdentityCache with IsAdmin/IsPremium fields (Redis round-trip safe)
  affects:
    - services/message-processor/enricher/viewer_badge_enricher.go
    - services/message-processor/enricher/viewer_badge_enricher_test.go
tech_stack:
  added: []
  patterns:
    - LATERAL subquery for deduplication-safe viewer_sessions join
    - Prepend-last wins for badge ordering (premium first, then allchat pushes to index 0)
key_files:
  created:
    - migrations/038_badge_definitions.sql
  modified:
    - services/message-processor/enricher/viewer_badge_enricher.go
    - services/message-processor/enricher/viewer_badge_enricher_test.go
decisions:
  - Prepend premium first, allchat second — allchat ends up at index 0 in final slice
  - LATERAL + LIMIT 1 pattern prevents duplicate rows when viewer has multiple sessions
  - COALESCE(u.is_admin, false) handles no-session viewers without nullable scan
  - fakeViewerDB queryFn extended to 8-return signature; noGradientDB helper updated
metrics:
  duration: ~5min
  completed_date: "2026-03-16"
  tasks_completed: 2
  files_changed: 3
---

# Phase 31 Plan 01: Badge Definitions Migration + Enricher Extension Summary

**One-liner:** badge_definitions catalog table + ViewerBadgeEnricher injects allchat/premium badges via LATERAL viewer_sessions JOIN on is_admin/is_premium

## What Was Built

1. **migrations/038_badge_definitions.sql** — CREATE TABLE badge_definitions (name PK, icon_url_1x, icon_url_2x, created_at) with idempotent INSERT for 'allchat' and 'premium' seed rows.

2. **ViewerBadgeEnricher extended** — DB query extended to LEFT JOIN LATERAL viewer_sessions + users, reading COALESCE(u.is_admin, false) and COALESCE(u.is_premium, false) as 6th and 7th scan destinations. Both cache-hit and DB-fetch paths now inject badges.

3. **viewerIdentityCache struct** — Two new fields: `IsAdmin bool` and `IsPremium bool` (both `omitempty`) persist across Redis round-trips.

4. **Badge order guarantee** — Prepend premium first, then allchat last, so final slice is `[allchat, premium, ...platform badges]`.

5. **fakeViewerDB test double** — Extended to 8-return queryFn signature. All existing tests updated to pass false, false for new params. Four new Phase 31 tests added.

## Tasks

| Task | Description | Commit |
|------|-------------|--------|
| 1 | DB migration 038 + enricher extension | 0583fe8 |
| 2 | fakeViewerDB extension + new badge tests | 01f3ed6 |

## Test Results

All 21 enricher tests pass:
- `TestEnrich_AdminBadge` — allchat badge at index 0
- `TestEnrich_PremiumBadge` — premium badge at index 0
- `TestEnrich_AdminAndPremiumBadge` — [allchat, premium] order
- `TestEnrich_NoBadgesForNonRegisteredViewer` — no badges on ErrNoRows
- All 17 prior tests unchanged

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check: PASSED

All created files exist. Both task commits verified.
