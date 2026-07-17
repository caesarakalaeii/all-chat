# ADR-0035: Admin Global Entity Search

**Date**: 2026-07-17
**Status**: Accepted
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

1. **Client-side federation over the existing admin list endpoints, returning typed results that
   deep-link into the ADR-0036 views.**
   - ✅ No new backend endpoint; reuses already-admin-accessible list APIs; typed results
     (user/overlay/source/viewer) link straight into the URL-addressable views; simple to ship.
   - ❌ Loads full lists client-side to match against — fine at current admin scale, but not free as
     data grows.
2. **A dedicated server-side `/api/v1/admin/search` endpoint from day one.**
   - ✅ Efficient (indexed queries, no full-list transfer), scales past client federation.
   - ❌ New endpoint, new query/index work, new authz surface — more up-front cost than the current
     admin scale warrants. Deferred, not rejected: kept as the documented optimization.
3. **Do nothing — rely on per-page filters (`?q=`, `?filter=`).**
   - ✅ Already exists.
   - ❌ Still requires guessing the entity type and opening the right page first; does not solve the
     "find anything" problem. Rejected.

## Decision Outcome

**Chosen**: Option 1 — client-side federated global search now, with the server endpoint as a
documented escape hatch.

- **One global admin search** resolves a free-text query across the four admin entity types:
  - **users** — by `username`, `display_name`, or `id`;
  - **overlays** — by `name` or `id`;
  - **sources** — by channel name / handle or `id`;
  - **viewers** — by `username` or `platform_user_id`.
- **Typed results deep-link into the URL-addressable views** from ADR-0036. Picking a user result
  navigates to `/admin/users?user=<id>`, an overlay to `/admin/overlays?overlay=<id>`, and so on, so
  search reuses and reinforces the addressable pattern rather than introducing a new result surface.
- **Initial implementation federates over the already-admin-accessible list endpoints on the
  client.** The search box fetches the admin lists it is already entitled to and matches the query
  across the fields above, grouping matches by type.
- **A dedicated server-side `/api/v1/admin/search` endpoint is noted as a future optimization** for
  if/when client federation stops scaling (indexed, typed, no full-list transfer). It is the explicit
  next step, not part of this initial cut.

## Consequences

### Positive
- One entry point to find any admin entity by name or id; no more guessing which page holds it.
- Search results reuse and reinforce the ADR-0036 URL-addressable views — one navigation model.
- Ships without new backend surface, because it rides the existing admin list endpoints.

### Negative
- Client federation loads full lists to match against. This is acceptable at the current admin scale
  but is the known scaling limit; the server-side `/api/v1/admin/search` endpoint is the documented
  remedy when that limit is reached.
- Match quality is only as good as the list payloads (the fields the list endpoints already return);
  richer ranking would come with the server endpoint.

## Implementation

- **Frontend**: a global admin search component (in `frontend/src/components/admin/` and surfaced from
  `frontend/src/app/admin/layout.tsx`) that federates over the existing admin user/overlay/source/
  viewer list endpoints, produces typed results, and links each result into its ADR-0036 view.
- **Backend**: none in the initial cut (reuses existing admin list endpoints). Future optimization:
  `/api/v1/admin/search` behind the same admin authz.

## Related Decisions

- [ADR-0036](./0036-admin-url-addressable-selection.md) — the URL-addressable views search results
  deep-link into; global search depends on and reinforces this pattern.
- [ADR-0034](./0034-admin-viewer-identity-model.md) — defines the viewer entity (raw `viewer_sessions`)
  that the viewer branch of search resolves.
