# ADR-0036: Admin Master-Detail with URL-Addressable Selection

**Date**: 2026-07-17
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation)
**Deciders**: caesarakalaeii

## Context

The admin dashboard is a set of list+detail pages (users, overlays, sources, viewers, cosmetics, feature gates). Each page kept its selected entity and filter state in opaque React state. Two problems followed: entities could not be linked to each other (a source row knew its `overlay_id`, but there was no way to navigate from an overlay to its owning user or back — the single most common admin usability complaint, "overlays are not linked to their users"), and nothing was shareable (an admin investigating a ticket could not paste a link that reopened the dashboard on the exact entity).

One page already pointed the way: the Overlays page read `?overlay=<id>` on load and auto-selected, and the Sources page linked into it. The pattern just needed to become the convention.

## Decision

**URL query parameters are the single source of truth for admin selection and filters.** Each list+detail page reads selection (`?user=<id>`, `?overlay=<id>`, …) and filters (`?filter=`, `?connected=`, `?platform=`, `?q=`) from the query string on mount and auto-selects the referenced entity; cross-links are plain `href`s carrying the owning page's params (source → `/admin/overlays?overlay=<id>`, overlay → `/admin/users?user=<owner_id>`, user → `/admin/sources?user=<id>`). Reloading or sharing a URL reopens the same entity.

Master-detail is the standard layout: a scrollable list next to a sticky detail panel, the URL selecting what the panel shows. No new `[id]` routes — selection stays in query params, which keeps list and detail co-resident and avoids a Next.js routing migration.

Alternatives considered: real nested dynamic routes (`/admin/users/[id]`, …) — canonical URLs and breadcrumbs, but a routing migration across every page for a dashboard that wants list and panel on screen together (the documented next step if per-entity URLs are ever needed); cross-links in memory only (still not shareable, does not solve the complaint).

## Consequences

- Entities are cross-navigable with plain links; a support hand-off is a URL; future admin pages get deep-linking by following the convention.
- **Caveat:** pages only *read* params on mount. In-page selection changes are not written back into the address bar, so a shared URL reflects the view at the time the link was copied, not later navigation. Write-back was in the original scope but never shipped; deep links in and cross-links out remain accurate.
- Each page carries a small amount of param-read boilerplate.

## Implementation

- `frontend/src/app/admin/{users,overlays,sources,viewers,...}/page.tsx` — mount-time param reads, master-detail layout, cross-link `href`s. No routing change.

## Related

- [ADR-0034](./0034-admin-viewer-identity-model.md) — the viewer admin view, one of the standardized pages.
- [ADR-0035](./0035-admin-global-entity-search.md) — global search deep-links into these views.
