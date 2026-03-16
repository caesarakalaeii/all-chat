# Phase 30: Avatar Frame & Flair System - Context

**Gathered:** 2026-03-15
**Status:** Ready for planning

<domain>
## Phase Boundary

Premium viewers can select an avatar frame (decorative ring) and avatar flair (corner icon) from an admin-curated catalog on `/settings/viewer`. Admin can add/remove catalog entries and mark them premium-only via `/admin/cosmetics`. Frame and flair selections persist in `viewer_cosmetics` and are injected as URLs by the message processor for rendering in overlays and the browser extension. Badge system (Phase 31) and animated gradient (v2) are separate phases.

</domain>

<decisions>
## Implementation Decisions

### Catalog browsing UX
- 4-column thumbnail grid for both frames and flairs — each item shows image + name below, active item gets a highlighted border
- Frames and flairs are two sections within a single card (no tabs), one below the other: "Avatar Frame" section then "Avatar Flair" section below it, with a single Save button at the bottom of the card
- First item in each grid is "None" (shows empty circle / no decoration) — the only way to clear a selection
- Explicit Save button — live preview updates immediately on click, but PATCH fires only on Save (consistent with gradient editor in Phase 29)

### Admin catalog management
- New dedicated page at `/admin/cosmetics` with two tabs: Frames and Flairs
- Follows the existing admin page pattern (`/admin/overlays`, `/admin/sources`, `/admin/users`, `/admin/viewers`)
- Add entry form fields: Name, Image URL, is_premium toggle — no sort_order field in v1.4
- Image URL field shows a 64×64 live preview thumbnail on blur (catches broken URLs before saving)
- Entry list: name, thumbnail, premium badge, delete (×) button — delete-and-re-add only, no inline editing
- Admin nav needs a "Cosmetics" entry added alongside existing admin links

### Overlay rendering
- Frame and flair render on: website overlay (`/overlay/[id]`), browser extension chat overlay, and settings page live preview — all three surfaces
- Create a reusable `<UserAvatar>` component accepting `avatarUrl`, `frameUrl`, `flairUrl`, `size` — used in all three surfaces for consistency
- Composite stacking: frame overflows the avatar container (`overflow: visible`); frame absolutely positioned centered via `translate(-50%, -50%) + top/left 50%`; flair absolutely positioned at bottom-right; all layers use `pointer-events-none`
- Image formats: transparent PNG and transparent GIF supported — admin is responsible for providing correctly prepared assets; component renders as-is with no special CSS blending

### Non-premium locked experience
- Premium-only catalog items show at reduced opacity with a small lock icon overlay — not clickable for non-premium users
- No section-level lock — only per-item lock icons on premium items; "None" (free) remains selectable by everyone
- Downgrade enforcement: when `is_premium` flips to `false`, server immediately clears `avatar_frame_id` and `avatar_flair_id` in `viewer_cosmetics` — frame/flair stops rendering immediately on next message enrichment cache miss

### Claude's Discretion
- Exact DB migration numbering (next after Phase 29 migrations)
- Specific Tailwind classes for locked item appearance (opacity, cursor, lock icon size/position)
- Error handling if Save fails (PATCH network error or 403)
- `<UserAvatar>` file location within frontend component tree
- Admin nav link placement and icon choice for "Cosmetics"

</decisions>

<specifics>
## Specific Ideas

- The "None" thumbnail being first in the grid is the standard pattern — users expect to find the remove option at the start, not as a separate action
- Frames/flairs extend visually beyond the avatar circle (1.4× per roadmap success criteria) — parent container must allow overflow visible; chat message layout may need slight padding adjustment to prevent frame clipping
- Transparent GIF support was explicitly requested — component should not assume static images (important for animated frame assets in the future)
- Strip-on-downgrade approach was chosen deliberately (not graceful carry-over) — enforces access control immediately

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `frontend/src/app/overlay/[id]/page.tsx` lines 612–630: existing avatar rendering block (Image + fallback div) — `<UserAvatar>` component replaces this block; same block needs replacing in extension overlay
- `frontend/src/app/settings/viewer/page.tsx`: Phase 29 card with live preview row — Phase 30 adds a new "Avatar Cosmetics" card below the color/gradient card using the same shadcn/ui Card pattern
- `frontend/src/components/ui/card.tsx`, `button.tsx`, `input.tsx`, `badge.tsx`: all already available — use for admin form and catalog grid
- `frontend/src/lib/types/message.ts` `UserInfo` interface (line ~76): Phase 30 adds `avatar_frame_url?: string` and `avatar_flair_url?: string` alongside existing `avatar_url`, `color`, `name_gradient`
- `frontend/src/app/admin/` pattern: existing admin pages (`/admin/overlays`, `/admin/users`, `/admin/viewers`) — `/admin/cosmetics` follows same layout/structure
- `frontend/src/app/admin/layout.tsx`: admin nav sidebar — add "Cosmetics" link here

### Established Patterns
- Phase 29 live preview: simulated chat row with avatar + display name below controls — Phase 30 extends same preview to show frame/flair on the avatar circle
- PATCH `/api/v1/auth/viewer/cosmetics` with explicit Save (gradient pattern) — extend with `avatar_frame_id` and `avatar_flair_id` fields
- Redis cache key `viewer:identity:{platform}:{platform_user_id}` — value currently `{ viewer_id, name_color, name_gradient }`; Phase 30 adds `avatar_frame_url` and `avatar_flair_url` to cached value
- Premium gate pattern from Phase 29: visible but disabled with lock icon (Gradient tab) — same visual treatment applied per-item in catalog grid
- Admin pages use client components with `fetch()` calls to `/api/admin/*` endpoints — follow same fetch/state pattern

### Integration Points
- `viewer_cosmetics` table: add `avatar_frame_id UUID FK` and `avatar_flair_id UUID FK` columns via new migration
- New tables: `cosmetic_frames (id UUID PK, name VARCHAR, image_url TEXT, is_premium BOOLEAN, created_at TIMESTAMPTZ)` and `cosmetic_flairs (id UUID PK, name VARCHAR, image_url TEXT, is_premium BOOLEAN, created_at TIMESTAMPTZ)`
- Message-processor enricher: extend cache value shape and `UserInfo` injection to include `avatar_frame_url` and `avatar_flair_url` resolved from catalog lookup
- New admin API endpoints: `GET/POST /api/admin/cosmetics/frames`, `DELETE /api/admin/cosmetics/frames/:id`, same for flairs
- `is_premium` downgrade hook: when premium status changes to false in auth-service, clear frame/flair in `viewer_cosmetics` (or check on enricher cache-miss with is_premium=false guard)

</code_context>

<deferred>
## Deferred Ideas

- None raised — discussion stayed within phase scope

</deferred>

---

*Phase: 30-avatar-frame-flair-system*
*Context gathered: 2026-03-15*
