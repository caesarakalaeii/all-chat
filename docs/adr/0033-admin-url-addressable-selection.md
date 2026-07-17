# ADR-0033: Admin Master-Detail with URL-Addressable Selection

**Date**: 2026-07-17
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

The admin dashboard (`frontend/src/app/admin/*`) is a set of list+detail pages — users,
overlays, sources, viewers, cosmetics, feature gates. Each page kept its selected entity and
its filter state in opaque React component state (`useState`). Two problems followed from that:

- **Entities could not be linked to each other.** A source row knew its `overlay_id`, but there
  was no way to navigate from an overlay to its owning user, from a user to their sources, or
  from a source back to the owning user, because selection lived in a component that another page
  could not address. This was the single most common admin usability complaint: *"overlays are
  not linked to their users."*
- **Nothing was shareable.** An admin investigating a support ticket could not paste a link that
  reopened the dashboard on the exact entity in question; the recipient had to be told which page
  to open and which row to click.

One page already pointed the way: the Overlays page read `?overlay=<id>` on load and auto-selected
that overlay (`frontend/src/app/admin/overlays/page.tsx`), and the Sources page linked into it
(`href={`/admin/overlays?overlay=${source.overlay_id}`}`). But the pattern was ad hoc — a single
deep-link on a single page, not a convention the other pages followed.

## Decision Drivers

- Make every admin entity deep-linkable and cross-navigable, resolving the "not linked" complaint.
- Make admin state shareable via URL for support hand-offs.
- Reuse the pattern the Overlays page already proved (`?overlay=<id>`) rather than invent a new one.
- Keep the change small and per-page — no framework/routing migration, no big-bang rewrite.

## Considered Options

1. **Standardize URL query parameters for selection and filters across every admin page.**
   - ✅ Generalizes the existing `?overlay=<id>` pattern; deep-linkable and shareable; each page
     reads params on mount and reflects selection/filter changes back into the URL; no new routes.
   - ❌ Query strings are flatter than nested routes; each page needs a small read-params-on-mount
     change.
2. **Introduce real nested dynamic routes (`/admin/users/[id]`, `/admin/overlays/[id]`, …).**
   - ✅ Canonical URLs, natural breadcrumbs, per-entity server components.
   - ❌ A routing migration across every admin page plus a split of list vs detail layouts; higher
     cost for a dashboard whose master-detail layout wants the list and the panel on screen together.
     Rejected for now on migration cost.
3. **Keep selection in React state, add cross-links as in-memory navigation only.**
   - ✅ No URL work.
   - ❌ Still not shareable, still not restorable on reload, still can't hand a link to another admin.
     Rejected — it does not solve the actual complaint.

## Decision Outcome

**Chosen**: Option 1 — URL query parameters as the single source of truth for admin selection and
filters.

- **Selection is URL-addressable.** Every list+detail admin page reads its selection from the query
  string on load (`?user=<id>`, `?overlay=<id>`, and the like) and writes the current selection back
  into the URL when the admin picks a row. Reloading or sharing the URL reopens the same entity.
- **Filters are URL-addressable too.** List filters live in the query string alongside selection —
  `?filter=`, `?connected=`, `?platform=`, `?q=` — so a filtered view is itself linkable and restored
  on reload.
- **Master-detail is the standard layout.** Each page is a scrollable list next to a sticky side
  detail panel; the URL selects which entity the panel shows. This is the layout the Overlays page
  already used, promoted to the convention for all of them.
- **No new `[id]` routes.** Selection stays in query params rather than nested dynamic segments. This
  keeps list and detail co-resident, avoids a Next.js routing migration, and lets cross-links be plain
  `href` strings into the existing pages (e.g. source → `/admin/overlays?overlay=<id>`,
  overlay → `/admin/users?user=<owner_id>`, user → `/admin/sources?user=<id>`).

## Consequences

### Positive
- Admin entities are cross-navigable: source → owning user, overlay → owner, user → their sources —
  the "overlays are not linked to their users" complaint is resolved with plain links.
- Every admin view is deep-linkable and shareable; a support hand-off is a URL.
- The pattern is uniform, so future admin pages get deep-linking by following the same convention.
- Zero routing/infra migration; changes are localized to each page's mount-time param read and its
  selection/filter handlers.

### Negative
- Query strings are a flatter address space than nested routes; if the admin later needs canonical
  per-entity URLs, breadcrumbs, or per-entity server rendering, the nested-route migration (Option 2)
  remains the documented next step.
- Each page carries a small amount of param-sync boilerplate (read on mount, write on change).

## Implementation

- **Frontend**: `frontend/src/app/admin/{users,overlays,sources,viewers,...}/page.tsx` — read
  selection/filter query params on mount and reflect changes back into the URL; render the
  scrollable-list + sticky-panel master-detail layout. Cross-links are `href`s into the sibling
  admin pages' query params.
- **Prior art**: `frontend/src/app/admin/overlays/page.tsx` already read `?overlay=<id>`;
  `frontend/src/app/admin/sources/page.tsx` already linked into it. This ADR generalizes that.
- **Routing**: unchanged — no new dynamic segments are added.

## Related Decisions

- [ADR-0034](./0034-admin-viewer-identity-model.md) — the viewer admin view, one of the pages
  standardized on this pattern.
- [ADR-0035](./0035-admin-global-entity-search.md) — global admin search deep-links its results
  into the URL-addressable views defined here.
