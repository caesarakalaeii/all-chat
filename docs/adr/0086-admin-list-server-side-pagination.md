# ADR-0086: Server-side pagination and search for the admin lists

**Date**: 2026-11-02
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation)
**Deciders**: caesarakalaeii

## Context

The admin Users, Overlays and Sources pages fetched their whole table in one request
(`GET /api/v1/admin/users`, `/admin/overlays`, `/admin/sources` returned bare arrays) and
searched, filtered, counted and rendered in the browser; the Users query also computed
per-user overlay/source counts and a platform-ban lookup per row. As the platform grew,
Users and Sources became slow, and global search (ADR-0035) fetched all three full lists
to show eight matches per group, outrunning the full-list load ADR-0035 had accepted.
Constraints: fast page loads at any table size; the ADR-0036 deep links keep working; one
paging contract, the Viewers page's (ADR-0034); search and filters cover every row.

## Considered options

- Server-side paging, search and filters on the existing endpoints with endless scrolling
  (chosen): one bounded page per request, SQL search over every row, global search
  reusing `?q=` with `limit=8`. Cost: the response shape changes from array to envelope,
  so frontend and backend must ship together.
- Keep the full fetch but virtualise rendering: fixes rendering only; the full transfer,
  per-row aggregates and global search stay. Rejected.
- Dedicated `/api/v1/admin/search` endpoint (ADR-0035's escape hatch) alone: repairs
  search but leaves Users and Sources slow and duplicates matching logic. Rejected.

## Decision

`GET /api/v1/admin/users`, `/admin/overlays` and `/admin/sources` take `limit` (default
50, max 100) and `offset` and answer `{ <users|overlays|sources>: [...], total, limit,
offset }`, rows newest first with the id as tiebreak. `q` is a case-insensitive literal
substring search with `%`, `_` and `\` escaped (usernames are full of underscores) over:
users: username, display name, id, twitch_id, google_id, kick_id; overlays: name, id,
owner id, owner username; sources: channel name, id and handle, overlay name, owner
username. Filters move server-side: users take `filter=active|banned|premium|beta|
unconfigured` and return `counts` computed over the same search definitions (a tab's
count and listing cannot disagree); sources take `platform`, `status`, `user`; overlays
take `ids`. The pages load page by page with endless scrolling (`usePagedList` +
`<LoadMore>`, the footer also a real button for keyboard and screen-reader use). The
connected-only Overlays view loads its live-connection-bounded set whole through `ids`
(chunks of 100 per request) then sorts by connection age in the browser. Global search
asks each list endpoint `q` with `limit=8` and shows the returned `total`, superseding
ADR-0035's client-side federation; the dedicated search endpoint it anticipated is not
needed. The Sources page's per-platform cards read `total_sources` from
`/api/v1/admin/stats`.

## Consequences

- Page load is one bounded query of at most 100 rows whatever the table size; one paging
  contract across all four admin lists.
- Frontend and backend must deploy together or the admin lists fail (admin only, one
  deploy window; `beta` breaks the same way until `main` merges into it).
- Users tab counts still aggregate over the search every request (one pass, no per-row
  transfer; the next thing to cache); offset paging can skip or repeat a row on concurrent
  mutations (keyset paging is the upgrade); a mutation reloads Users from page one.

## Implementation

- auth-service: `repository.UserRepository.ListUsers`/`CountUsers` (`UserListFilter`),
  `handlers.AdminHandler.ListUsers`, `pageParams` helper.
- overlay-manager: `OverlayRepository.ListOverlaysWithSourceCount` (`OverlayListFilter`),
  `SourceRepository.ListSourcesWithOverlay` (`SourceListFilter`),
  `handlers.AdminHandler.ListOverlays`/`ListAllSources`.
- frontend: `src/hooks/usePagedList.ts`, `src/components/admin/LoadMore.tsx`, the admin
  list pages under `src/app/admin/`; e2e `frontend/tests/e2e/admin-lists.spec.ts`.
