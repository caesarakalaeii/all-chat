# ADR-0035: Admin Global Entity Search

**Date**: 2026-07-17
**Status**: Accepted; client-side federation superseded by [ADR-0086](./0086-admin-list-server-side-pagination.md)
**Revised**: 2026-11-03 (brought in line with the current implementation)
**Deciders**: caesarakalaeii

## Context

The admin dashboard had no way to jump straight to an entity by name or id: finding a specific user, overlay, source or viewer meant guessing which page held it, opening that list, and paging or filtering. ADR-0036 made every admin entity URL-addressable; what was missing was a single entry point that takes a free-text query and lands on the right entity regardless of type.

## Decision

**One global admin search** (`frontend/src/app/admin/search/page.tsx`, linked from the admin sidebar, itself seeded from `?q=`) that resolves a query across the four admin entity types and deep-links each hit into the ADR-0036 URL-addressable views:

- users — by `username`, `display_name` or `id` → `/admin/users?user=<id>`;
- overlays — by `name` or `id` → `/admin/overlays?overlay=<id>`;
- sources — by channel name/id/handle (the server-side source query also matches the owning overlay's name and the owner's username) → the owning overlay view;
- viewers — by `username`, `display_name` or `platform_user_id` → `/admin/viewers?q=<username>`.

There is no dedicated search endpoint. The original cut fetched whole lists and matched in the browser; [ADR-0086](./0086-admin-list-server-side-pagination.md) instead made the existing list endpoints searchable server-side (`q` with `limit`/`offset` and a `total` count), and the search page now performs eight bounded server-side matches per group (`limit=8&q=…`), so no full list ever transfers and search covers every row. A dedicated `/api/v1/admin/search` endpoint was anticipated as an escape hatch, never needed, and remains unbuilt.

Alternatives considered: a dedicated server-side endpoint from day one (unwarranted up-front query/authz work at the admin scale of the day, superseded by ADR-0086); relying on per-page filters only (still requires guessing the entity type first).

## Consequences

- One entry point to find any admin entity; search reuses and reinforces the ADR-0036 navigation model rather than introducing a new result surface.
- Search fires four list-endpoint queries per (debounced) keystroke; one batching endpoint would shave that if it ever matters.
- Match quality follows the fields the list endpoints search server-side; richer ranking would need new query work there.

## Related

- [ADR-0036](./0036-admin-url-addressable-selection.md) — the views search deep-links into.
- [ADR-0086](./0086-admin-list-server-side-pagination.md) — moved matching into the list endpoints' SQL.
- [ADR-0034](./0034-admin-viewer-identity-model.md) — defines the viewer entity search resolves.
