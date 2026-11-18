# ADR-0035: Admin Global Entity Search

**Date**: 2026-07-17
**Status**: Accepted; client-side federation superseded by [ADR-0086](./0086-admin-list-server-side-pagination.md)
**Revised**: 2026-11-03 (brought in line with the current implementation)
**Deciders**: caesarakalaeii

## Context and Problem Statement

The admin dashboard had no way to jump straight to an entity by name or id. To find a specific
user, overlay, source, or viewer, an admin had to first *guess which page* held it — is this id a
user? an overlay? a source? a viewer session? — open that page, and then page through the list
(or filter it) to find the row. For a support request that arrives as "here's a username" or
"here's an overlay id," that is several manual hops before any work starts.

ADR-0036 made every admin entity URL-addressable and cross-navigable. What was still missing was a
**single entry point** that takes a free-text query and lands the admin on the right entity in the
right view, regardless of which kind of entity it is.

## Decision Drivers

- One search box that finds anything across the admin domain, so the admin never has to guess the
  entity type first.
- Results must land in the existing URL-addressable views (ADR-0036), not a bespoke result page —
  reuse, don't fork.
- Ship it without blocking on new backend surface if the existing admin endpoints already return the
  data.
- Keep a documented path to a real server-side search if client-side federation stops scaling.

## Considered Options

1. **Ride the existing admin list endpoints, returning typed results that deep-link into the
   ADR-0036 views.**
   - ✅ No new backend endpoint; typed results (user/overlay/source/viewer) link straight into
     the URL-addressable views; simple to ship. Over time this turned into server-side matching
     rather than client federation (see Decision Outcome).
   - ❌ The original cut did the matching client-side and loaded whole lists — fine at the
     admin scale of the day, but not free as data grew. ADR-0086 later removed the limitation.
2. **A dedicated server-side `/api/v1/admin/search` endpoint from day one.**
   - ✅ Efficient (indexed queries, no full-list transfer), scales past client federation.
   - ❌ New endpoint, new query/index work, new authz surface — more up-front cost than the current
     admin scale warrants. Deferred, not rejected: kept as the documented optimization. In the end
     never built; ADR-0086's server-side `q` on the list endpoints covers the need.
3. **Do nothing — rely on per-page filters (`?q=`, `?filter=`).**
   - ✅ Already exists.
   - ❌ Still requires guessing the entity type and opening the right page first; does not solve the
     "find anything" problem. Rejected.

## Decision Outcome

**Chosen**: Option 1, with a fate note: the "documented escape hatch" of a dedicated server
endpoint never shipped — [ADR-0086](./0086-admin-list-server-side-pagination.md) instead made
the existing list endpoints searchable server-side, which solved the scaling problem the
endpoint was held in reserve for.

- **One global admin search** resolves a free-text query across the four admin entity types:
  - **users** — by `username`, `display_name`, or `id`;
  - **overlays** — by `name` or `id`;
  - **sources** — by channel name, channel id, or handle (the server-side source query also
    matches the owning overlay's name and the owner's username, `services/overlay-manager/repository/source_repo.go:383`);
  - **viewers** — by `username`, `display_name`, or `platform_user_id`
    (`services/auth-service/repository/viewer_repository.go:343`).
- **Typed results deep-link into the URL-addressable views** from ADR-0036. Picking a user result
  navigates to `/admin/users?user=<id>`, an overlay to `/admin/overlays?overlay=<id>`, a source
  to its owning overlay view `/admin/overlays?overlay=<overlay_id>`, and a viewer to the viewer
  list filtered by username `/admin/viewers?q=<username>` (`frontend/src/app/admin/search/page.tsx`),
  so search reuses and reinforces the addressable pattern rather than introducing a new result
  surface.
- **The search ships without dedicated backend surface: it asks the existing admin list
  endpoints for matching rows.** Initially this was client-side federation that fetched whole
  lists and matched in the browser; [ADR-0086](./0086-admin-list-server-side-pagination.md)
  then moved search, paging and filters server-side on those endpoints (`q` with `limit`/
  `offset`, a `total` count), and the search page now performs **eight server-side matches
  per group** (`limit=8&q=…`), so no full list ever transfers and search covers every row,
  not only loaded ones. The dedicated `/api/v1/admin/search` endpoint anticipated here was
  never needed and remains unbuilt.

## Consequences

### Positive
- One entry point to find any admin entity by name or id; no more guessing which page holds it.
- Search results reuse and reinforce the ADR-0036 URL-addressable views — one navigation model.
- Ships without a search endpoint of its own: matching runs in the list endpoints' SQL as the
  server-side `q` parameter (ADR-0086), so search sees every row however large the tables grow.

### Negative
- The search fires four list-endpoint queries per keystroke (debounced); one endpoint batching
  all four would shave that if it ever shows up.
- Match quality follows the fields the list endpoints search server-side (ADR-0086); richer
  ranking would come with new query work there.

## Implementation

- **Frontend**: the global admin search page `frontend/src/app/admin/search/page.tsx`, linked
  from the admin sidebar (`frontend/src/components/AdminSidebar.tsx`) and seeded from its own
  `?q=` deep link. Each group fetches `limit=8` server-side `?q=` matches and the `total` from
  the users/overlays/sources/viewers list endpoints and links each hit into its ADR-0036 view.
- **Backend**: no search endpoint of its own; matching is the server-side `q` the admin list
  endpoints gained under ADR-0086.

## Related Decisions

- [ADR-0036](./0036-admin-url-addressable-selection.md) — the URL-addressable views search results
  deep-link into; global search depends on and reinforces this pattern.
- [ADR-0086](./0086-admin-list-server-side-pagination.md) — server-side pagination and search
  for the admin lists; turned this ADR's client-side federation into bounded `?q=`/`limit=8`
  queries.
- [ADR-0034](./0034-admin-viewer-identity-model.md) — defines the viewer entity (raw `viewer_sessions`)
  that the viewer branch of search resolves.
