# ADR-0086: Server-side pagination and search for the admin lists

**Date**: 2026-11-02
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

The admin Users, Overlays and Sources pages each fetched their whole table in
one request (`GET /api/v1/admin/users`, `/admin/overlays`, `/admin/sources`
returned bare arrays) and then searched, filtered, counted and rendered every
row in the browser. The Users query also computes per-user overlay and source
counts and a platform-ban lookup for every row. As the platform grew, the Users
and Sources pages became slow to load.

Global search (ADR-0035) made it worse: one query fetched all three full lists
just to show eight matches per group. ADR-0035 accepted this as "fine at current
admin scale" and named the remedy for when it stopped scaling. That point has
been reached.

## Decision Drivers

- The pages must load quickly however many users, overlays and sources exist.
- Keep the URL-addressable views and deep links from ADR-0036 working.
- Reuse the paging contract the Viewers page already has (ADR-0034), not a
  second one.
- Search and filters must cover every row, not just the rows loaded so far.

## Considered Options

1. **Server-side paging, search and filters on the existing list endpoints,
   with endless scrolling in the pages.**
   - ✅ Each request is one bounded page; search and filters run in SQL over
     every row; global search reuses the same `?q=` with `limit=8`.
   - ❌ The response shape changes from an array to an envelope, so the
     frontend and the backend have to ship together.
2. **Keep the full fetch but virtualise rendering.**
   - ✅ No API change.
   - ❌ Fixes rendering only. The full transfer and the per-row aggregates for
     every user remain, and global search still loads three full lists.
     Rejected.
3. **A dedicated `/api/v1/admin/search` endpoint (ADR-0035's escape hatch)
   and leave the pages alone.**
   - ✅ Fixes search.
   - ❌ The Users and Sources pages stay slow, and a search endpoint would
     duplicate the per-entity matching the list endpoints need anyway.
     Rejected.

## Decision Outcome

**Chosen**: Option 1.

- `GET /api/v1/admin/users`, `/admin/overlays` and `/admin/sources` take
  `limit` (default 50, at most 100) and `offset`, and answer
  `{ <users|overlays|sources>: [...], total, limit, offset }`, the shape the
  Viewers endpoint already uses. Rows are ordered newest first with the id as a
  tiebreak, so consecutive pages neither repeat nor skip a row.
- `q` is a case-insensitive literal substring search. `%`, `_` and `\` are
  escaped, because usernames are full of underscores. Fields searched:
  - users: username, display name, id, Twitch/Google/Kick id;
  - overlays: name, id, owner id, owner username;
  - sources: channel name, id and handle, overlay name, owner username.
- Filters move to the server: users take `filter=active|banned|premium|beta|unconfigured`
  and return `counts` for every tab, computed over the search in the same
  definitions as the filters, so a tab's count and its listing cannot disagree.
  Sources take `platform`, `status=active|inactive` and `user` (owner). Overlays
  take `ids` (comma-separated) for deep links and the connected-only view.
- The pages load one page at a time and fetch the next as the list scrolls
  (`usePagedList` + `<LoadMore>`). The footer is a real button as well as the
  scroll sentinel, so keyboard and screen-reader users can load more without
  scrolling.
- The connected-only Overlays view is bounded by live connections, not by the
  table. It loads those overlays whole through `ids` (in chunks of 100 per
  request), then filters and sorts by connection age in the browser as before.
- Global search calls each list endpoint with `q` and `limit=8` and shows the
  returned `total`. This supersedes ADR-0035's client-side federation; the
  dedicated search endpoint it anticipated is not needed.
- The Sources page's per-platform cards read `total_sources` from the existing
  `/api/v1/admin/stats` instead of counting loaded rows.

## Consequences

### Positive

- Page load is one bounded query and at most 100 rows whatever the table size.
- Search and filters see every row, not only the loaded ones.
- One paging contract across all four admin lists.

### Negative

- Breaking response shape: an old frontend against a new backend (or the
  reverse) shows the admin lists as failed until both have deployed. Admin
  only, and the window is one deploy. The `beta` frontend breaks the same way
  until `main` is merged into `beta`.
- The Users tab counts still aggregate over every user that matches the search,
  on every page request. That is one pass with no per-row transfer, cheaper
  than the old full listing, but it is the next thing to cache if it shows up.
- Offset paging can skip or repeat a row when one is inserted or deleted while
  the admin scrolls. Acceptable for an operator list; keyset paging is the
  upgrade if it matters.
- After a mutation (ban, role change) the Users list reloads from the first
  page instead of patching the row, because the row may have left the tab.

## Implementation

- auth-service: `repository.UserRepository.ListUsers` / `CountUsers`
  (`UserListFilter`), `handlers.AdminHandler.ListUsers`, and the shared
  `pageParams` (also used by the Viewers handler).
- overlay-manager: `OverlayRepository.ListOverlaysWithSourceCount`
  (`OverlayListFilter`), `SourceRepository.ListSourcesWithOverlay`
  (`SourceListFilter`), `handlers.AdminHandler.ListOverlays` / `ListAllSources`.
- frontend: `src/hooks/usePagedList.ts`, `src/components/admin/LoadMore.tsx`,
  the Users, Overlays, Sources and Search pages under `src/app/admin/`.
- Tests: repository tests against Postgres, handler tests, hook and component
  unit tests, and `frontend/tests/e2e/admin-lists.spec.ts`.

## Related Decisions

- [ADR-0034](./0034-admin-viewer-identity-model.md): the Viewers paging
  contract this reuses.
- [ADR-0035](./0035-admin-global-entity-search.md): its client-side federation
  is superseded here.
- [ADR-0036](./0036-admin-url-addressable-selection.md): the deep links
  (`?user=`, `?overlay=`, `?platform=`, `?filter=`, `?connected=`) keep working
  on top of the paged lists.
