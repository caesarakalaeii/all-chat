---
status: awaiting_human_verify
trigger: "The public extension overlay randomly changes to a different overlay without any user action"
created: 2026-04-08T00:00:00Z
updated: 2026-04-08T02:00:00Z
---

## Current Focus

hypothesis: CONFIRMED (deletion path) — GetPublicOverlayByUsername uses LIMIT 1 without ORDER BY; deleting any overlay changes which row PostgreSQL returns first, making the "public" overlay appear to flip. Compounded by HandleDeleteOverlay not promoting a replacement when the public overlay itself is deleted.
test: All handler tests pass (20/20), api-gateway builds cleanly
expecting: User confirms extension overlay no longer flips after deleting overlays
next_action: Human verification — delete a non-public overlay and confirm extension doesn't flip; delete the public overlay and confirm a different overlay is auto-promoted

## Symptoms

expected: Each user should have exactly one public extension overlay, and it should stick permanently until explicitly changed
actual: The public extension overlay randomly changes to a different overlay without any user action
errors: None reported — silent behavior change
reproduction: No clear trigger — feels very random, no frontend interaction needed, it just happens on its own
started: Used to work correctly, broke at some point
observation: Visible in the user dashboard — wrong overlay shown as public extension

## Eliminated

- hypothesis: share-service or background jobs flipping the flag
  evidence: share-service never touches is_public_for_viewers; no background jobs found that modify overlay flags
  timestamp: 2026-04-08

- hypothesis: HandleUpdateOverlay incorrectly setting the flag
  evidence: Update handler only sets flag when explicitly passed is_public_for_viewers=true in the request body — correct gated behavior
  timestamp: 2026-04-08

- hypothesis: HandleDeleteOverlay has promotion logic that randomly picks a new public overlay
  evidence: HandleDeleteOverlay is a plain delete — no promotion, no flag manipulation. The flip is not caused by an explicit promotion.
  timestamp: 2026-04-08

- hypothesis: Database trigger on DELETE promotes another overlay
  evidence: Searched all migrations for CREATE TRIGGER / AFTER DELETE / BEFORE DELETE — only triggers are updated_at maintenance, config auto-create on INSERT, and source change notifications. None fire on overlay DELETE.
  timestamp: 2026-04-08

## Evidence

- timestamp: 2026-04-08
  checked: services/overlay-manager/handlers/overlay.go HandleCreateOverlay
  found: Line 65 hardcoded IsPublicForViewers: true for EVERY new overlay, regardless of user's existing overlays. The subsequent UnsetAllPublicForUser call stripped the flag from whatever overlay was previously the extension overlay.
  implication: Every overlay creation silently hijacked the public extension designation from the previously-set overlay.

- timestamp: 2026-04-08
  checked: commit f8ac1cf5 "fix(overlay): enforce single public overlay per user on creation"
  found: That fix added UnsetAllPublicForUser to creation but left IsPublicForViewers: true hardcoded — it enforced uniqueness but didn't stop the flag being stolen on every creation.
  implication: The prior fix was incomplete; it prevented multiple overlays being public simultaneously but didn't prevent new overlays from automatically claiming the public flag.

- timestamp: 2026-04-08
  checked: services/overlay-manager/handlers/overlay_test.go
  found: Existing test "creation unsets other public overlays" asserted is_public_for_viewers=true in the response — it was validating the broken behavior.
  implication: The broken behavior was inadvertently codified in tests.

- timestamp: 2026-04-08
  checked: services/api-gateway/subscription/repository.go GetPublicOverlayByUsername
  found: Query is "SELECT o.id FROM overlays o JOIN users u ... WHERE ... AND o.is_public_for_viewers = true LIMIT 1" — there is NO ORDER BY clause.
  implication: When more than one overlay has is_public_for_viewers=true (possible via migration 046 backfill or old create-bug), PostgreSQL returns an arbitrary row. Deleting any overlay changes the query plan's row ordering, causing a different overlay to surface — the observed "random flip."

- timestamp: 2026-04-08
  checked: services/overlay-manager/handlers/overlay.go HandleDeleteOverlay
  found: Plain delete — fetches overlay for ownership check, deletes, returns 204. No check whether the deleted overlay was the public one. No promotion of a replacement.
  implication: If the user deletes the overlay that currently holds is_public_for_viewers=true, no other overlay is promoted. The extension silently breaks (returns 404 for viewers). The user would have to manually re-designate one.

- timestamp: 2026-04-08
  checked: All migrations for DB-level uniqueness constraint on is_public_for_viewers
  found: No UNIQUE partial index or CHECK constraint enforces "at most one public overlay per user" at the database level. Invariant is application-only (via UnsetAllPublicForUser).
  implication: Any bypass of the application layer (direct SQL, migration backfill, race condition) can result in multiple public overlays, which then non-deterministically rotate via LIMIT 1 without ORDER BY.

- timestamp: 2026-04-08
  checked: migrations/046_fix_public_overlay_default.sql
  found: Set column DEFAULT to true AND ran UPDATE overlays SET is_public_for_viewers=true WHERE is_public_for_viewers=false AND is_active=true — backfilled ALL active overlays to public simultaneously.
  implication: Migration 047 tried to clean this up with a one-time ranked UPDATE, but it only ran once. Any subsequent creation/deletion bypasses it. The column DEFAULT of true persists and would affect any INSERT that omits the column (though the app always passes it explicitly).

- timestamp: 2026-04-08
  checked: services/overlay-manager/handlers/overlay.go HandleDeleteOverlay + OverlayRepository interface
  found: Repository interface exposes Delete(ctx, id) and ListByUserID(ctx, userID) but HandleDeleteOverlay does not use ListByUserID to detect the public-overlay case.
  implication: Fix is entirely in the handler layer — no new repository method needed. Handler must: (1) read the deleted overlay's is_public_for_viewers value before deleting, (2) if true, promote the oldest remaining active overlay after deletion.

## Resolution

root_cause: Two compounding bugs triggered by overlay deletion:
  1. GetPublicOverlayByUsername (api-gateway) uses LIMIT 1 with no ORDER BY. When multiple overlays have is_public_for_viewers=true (caused by migration 046 backfill or the prior create-path bug), deleting any overlay changes the PostgreSQL query plan's row ordering, causing a different overlay to surface — the observed "random flip."
  2. HandleDeleteOverlay has no promotion guard: if the user deletes the overlay that currently holds is_public_for_viewers=true, no other overlay is promoted. The extension silently loses its designated overlay.

fix:
  1. Add ORDER BY o.created_at ASC to GetPublicOverlayByUsername so selection is deterministic even when multiple overlays have the flag.
  2. In HandleDeleteOverlay: read the overlay's is_public_for_viewers value before deleting. If it was the public overlay, after deletion find the next oldest active overlay and promote it by setting is_public_for_viewers=true.
  3. Add a new migration (048) adding a partial unique index to enforce at most one public overlay per user at the DB level, preventing future drift.

verification: 20/20 handler unit tests pass (including 4 new deletion promotion tests). api-gateway builds cleanly. Awaiting human confirmation in real environment.

files_changed:
  - services/overlay-manager/handlers/overlay.go
  - services/overlay-manager/handlers/overlay_test.go
  - services/api-gateway/subscription/repository.go
  - migrations/048_unique_public_overlay.sql
