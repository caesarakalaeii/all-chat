# Phase 30: Avatar Frame & Flair System - Research

**Researched:** 2026-03-16
**Domain:** Full-stack cosmetics catalog: PostgreSQL catalog tables, Go REST handlers, React admin page, React settings card, reusable avatar composite component, message processor enricher extension
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Catalog browsing UX**
- 4-column thumbnail grid for both frames and flairs — each item shows image + name below, active item gets a highlighted border
- Frames and flairs are two sections within a single card (no tabs), one below the other: "Avatar Frame" section then "Avatar Flair" section below it, with a single Save button at the bottom of the card
- First item in each grid is "None" (shows empty circle / no decoration) — the only way to clear a selection
- Explicit Save button — live preview updates immediately on click, but PATCH fires only on Save (consistent with gradient editor in Phase 29)

**Admin catalog management**
- New dedicated page at `/admin/cosmetics` with two tabs: Frames and Flairs
- Follows the existing admin page pattern (`/admin/overlays`, `/admin/sources`, `/admin/users`, `/admin/viewers`)
- Add entry form fields: Name, Image URL, is_premium toggle — no sort_order field in v1.4
- Image URL field shows a 64×64 live preview thumbnail on blur (catches broken URLs before saving)
- Entry list: name, thumbnail, premium badge, delete (×) button — delete-and-re-add only, no inline editing
- Admin nav needs a "Cosmetics" entry added alongside existing admin links

**Overlay rendering**
- Frame and flair render on: website overlay (`/overlay/[id]`), browser extension chat overlay, and settings page live preview — all three surfaces
- Create a reusable `<UserAvatar>` component accepting `avatarUrl`, `frameUrl`, `flairUrl`, `size` — used in all three surfaces for consistency
- Composite stacking: frame overflows the avatar container (`overflow: visible`); frame absolutely positioned centered via `translate(-50%, -50%) + top/left 50%`; flair absolutely positioned at bottom-right; all layers use `pointer-events-none`
- Image formats: transparent PNG and transparent GIF supported — admin is responsible for providing correctly prepared assets; component renders as-is with no special CSS blending

**Non-premium locked experience**
- Premium-only catalog items show at reduced opacity with a small lock icon overlay — not clickable for non-premium users
- No section-level lock — only per-item lock icons on premium items; "None" (free) remains selectable by everyone
- Downgrade enforcement: when `is_premium` flips to `false`, server immediately clears `avatar_frame_id` and `avatar_flair_id` in `viewer_cosmetics` — frame/flair stops rendering immediately on next message enrichment cache miss

### Claude's Discretion
- Exact DB migration numbering (next after Phase 29 migrations)
- Specific Tailwind classes for locked item appearance (opacity, cursor, lock icon size/position)
- Error handling if Save fails (PATCH network error or 403)
- `<UserAvatar>` file location within frontend component tree
- Admin nav link placement and icon choice for "Cosmetics"

### Deferred Ideas (OUT OF SCOPE)
- None raised — discussion stayed within phase scope

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| PREM-03 | Premium viewer can select an avatar frame (decorative PNG ring overlaid on their avatar) | DB columns, PATCH handler extension, catalog API, UserAvatar component |
| PREM-04 | Premium viewer can select an avatar flair (small corner icon pinned to bottom-right of avatar) | Same as PREM-03 — both fields travel together through the stack |
| PREM-05 | Frame and flair catalog is managed by admins (add/remove items, mark as premium-only) | `cosmetic_frames` / `cosmetic_flairs` tables, admin handler, admin page |
| WEB-03 | Premium users can browse and select avatar frame from the frame catalog | Settings page cosmetics card, GET catalog endpoint |
| WEB-04 | Premium users can browse and select avatar flair from the flair catalog | Same card, same endpoint pattern |

</phase_requirements>

---

## Summary

Phase 30 extends an already-mature cosmetics stack (Phase 29 delivered gradient storage, enrichment, cache invalidation, and premium gating). The work follows four clear concerns:

1. **Database**: Two new catalog tables (`cosmetic_frames`, `cosmetic_flairs`) + two nullable FK columns on `viewer_cosmetics` (`avatar_frame_id`, `avatar_flair_id`) via a single new migration (037). Downgrade enforcement is a SQL UPDATE in the existing cosmetics PATCH handler.

2. **Backend (auth-service)**: A new `AdminCosmeticsHandler` (GET/POST/DELETE for frames and flairs under `/admin/cosmetics/*`), an extension of `UpsertViewerCosmetics` to accept two additional UUID-nullable parameters, and a new `GetViewerAvatarURLs` helper that resolves FKs to image URLs for the JWT-issued cosmetics and the enricher's DB query.

3. **Frontend (website)**: A new `AvatarCosmeticsCard` component on `/settings/viewer` (below the existing `ColorGradientCard`), a new `/admin/cosmetics` page following the viewers-page pattern (tabs for Frames/Flairs, add-form with URL preview on blur, entry list with delete), the `<UserAvatar>` composite component, and an extension of the live-preview row to include the new component.

4. **Message processor enricher**: `viewerIdentityCache` and the DB query in `viewer_badge_enricher.go` extend with `AvatarFrameURL` and `AvatarFlairURL`; the Go `UserInfo` model gets two new fields; the TypeScript `UserInfo` interface in both the monorepo and extension repos gets the same two optional fields.

**Primary recommendation:** Build in this order — DB migration first (unblocks all Go changes), then Go catalog handler + cosmetics PATCH extension, then TypeScript type updates, then `<UserAvatar>` component, then settings card, then admin page, then enricher extension. Each step is independently testable.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| pgx/v5 | current | PostgreSQL access for catalog tables | Already used throughout services |
| gin | current | HTTP routing for new admin endpoints | Already used in auth-service |
| go-redis/v9 | current | Cache invalidation after catalog/cosmetics write | Already used in ViewerCosmeticsHandler |
| google/uuid | current | UUID PK for catalog entries | Already used for viewer_id, overlay_id |
| Next.js App Router | 14+ | Admin and settings pages | Already the frontend framework |
| Tailwind CSS | current | Component styling, lock icon overlays, grid | Already the styling system |
| shadcn/ui Card, Button, Input, Badge | current | Admin form and catalog grid primitives | Already available; used in viewers page |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| date-fns | current | `created_at` display in admin catalog list | Already imported in viewers page |
| toastManager | internal | Success/error feedback in admin page | Already used in all admin pages |
| apiClient | internal | Authenticated fetch wrapper in admin pages | Already used in admin pages |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| UUID PK for catalog | SERIAL integer | UUID chosen for consistency with all other entity IDs in the schema |
| Storing resolved URLs in Redis cache | Storing only IDs in cache | URLs are stored to avoid a second DB join on every message enrichment |

**Installation:** No new dependencies required. Everything is already installed.

---

## Architecture Patterns

### Recommended Project Structure

New files to create:

```
services/auth-service/
├── handlers/admin_cosmetics.go           # GET/POST/DELETE for frames + flairs
├── handlers/admin_cosmetics_test.go      # unit tests via mock DB interface
└── repository/cosmetics_catalog_repo.go  # DB operations for catalog tables

services/message-processor/
└── enricher/viewer_badge_enricher.go     # extend existing file (new fields)

frontend/src/
├── components/UserAvatar.tsx             # reusable composite avatar component
├── app/settings/viewer/page.tsx          # add AvatarCosmeticsCard below ColorGradientCard
└── app/admin/cosmetics/page.tsx          # new admin page

migrations/
└── 037_viewer_cosmetics_catalog.sql      # new tables + viewer_cosmetics columns

all-chat-extension/src/lib/types/message.ts  # add avatar_frame_url?, avatar_flair_url? to UserInfo
all-chat-extension/src/ui/components/ChatContainer.tsx  # use UserAvatar for avatar rendering
```

### Pattern 1: Catalog Table Shape

The two catalog tables are structurally identical. Migration 037 follows the pattern established in 035 and 036:

```sql
-- Source: direct inspection of migrations/035_viewer_identity.sql and 036_viewer_gradient.sql
CREATE TABLE IF NOT EXISTS cosmetic_frames (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(100) NOT NULL,
    image_url  TEXT NOT NULL,
    is_premium BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS cosmetic_flairs (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(100) NOT NULL,
    image_url  TEXT NOT NULL,
    is_premium BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE viewer_cosmetics
    ADD COLUMN IF NOT EXISTS avatar_frame_id UUID REFERENCES cosmetic_frames(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS avatar_flair_id UUID REFERENCES cosmetic_flairs(id) ON DELETE SET NULL;
```

`ON DELETE SET NULL` is critical: if an admin deletes a catalog entry, the viewer's selection is cleared gracefully without breaking referential integrity.

### Pattern 2: Admin Catalog Handler

Follows `AdminViewerHandler` exactly — constructor accepts `*pgxpool.Pool` (or a thin repo), handler methods are named `HandleList*`, `HandleCreate*`, `HandleDelete*`:

```go
// Source: direct inspection of services/auth-service/handlers/admin_viewers.go
type AdminCosmeticsHandler struct {
    log    *zap.Logger
    db     *pgxpool.Pool  // or cosmetics catalog repo interface
}

func (h *AdminCosmeticsHandler) HandleListFrames(c *gin.Context) { ... }
func (h *AdminCosmeticsHandler) HandleCreateFrame(c *gin.Context) { ... }
func (h *AdminCosmeticsHandler) HandleDeleteFrame(c *gin.Context) { ... }
// Mirror for flairs
```

Routes register under the existing `admin` group in `main.go`:

```go
// Source: direct inspection of services/auth-service/cmd/main.go lines 298–319
admin.GET("/cosmetics/frames", adminCosmeticsHandler.HandleListFrames)
admin.POST("/cosmetics/frames", adminCosmeticsHandler.HandleCreateFrame)
admin.DELETE("/cosmetics/frames/:id", adminCosmeticsHandler.HandleDeleteFrame)
admin.GET("/cosmetics/flairs", adminCosmeticsHandler.HandleListFlairs)
admin.POST("/cosmetics/flairs", adminCosmeticsHandler.HandleCreateFlair)
admin.DELETE("/cosmetics/flairs/:id", adminCosmeticsHandler.HandleDeleteFlair)
```

### Pattern 3: Extending PATCH /viewer/cosmetics

The existing `patchCosmeticsRequest` and `UpsertViewerCosmetics` must grow two nullable UUID fields. The cosmetics handler checks premium status before accepting non-nil frame/flair IDs. Downgrade enforcement (stripping frame/flair when `is_premium` becomes false) happens in the same PATCH handler: when the incoming request sets `is_premium = false` or is called from a non-premium JWT, clear both fields.

```go
// Source: direct inspection of services/auth-service/handlers/viewer_cosmetics.go
// Extend patchCosmeticsRequest:
type patchCosmeticsRequest struct {
    NameColor     *string          `json:"name_color"`
    NameGradient  *NameGradientReq `json:"name_gradient"`
    AvatarFrameID *uuid.UUID       `json:"avatar_frame_id"` // null clears selection
    AvatarFlairID *uuid.UUID       `json:"avatar_flair_id"`
}
// Premium gate: if !is_premium && (AvatarFrameID != nil || AvatarFlairID != nil) → 403
// Downgrade enforcement: applied server-side when is_premium changes to false
//   UPDATE viewer_cosmetics SET avatar_frame_id = NULL, avatar_flair_id = NULL WHERE viewer_id = $1
```

**Important:** The frontend sends `avatar_frame_id: null` to clear, and omits the field when not changing it. The handler must distinguish null (clear) from absent (no change). Use `*uuid.UUID` pointer with explicit null in JSON.

### Pattern 4: ViewerBadgeEnricher Extension

Extend `viewerIdentityCache` struct and the DB query to carry two additional URL fields:

```go
// Source: direct inspection of services/message-processor/enricher/viewer_badge_enricher.go
type viewerIdentityCache struct {
    ViewerID       string  `json:"viewer_id"`
    NameColor      *string `json:"name_color"`
    NameGradient   []byte  `json:"name_gradient,omitempty"`
    AvatarFrameURL string  `json:"avatar_frame_url,omitempty"` // Phase 30
    AvatarFlairURL string  `json:"avatar_flair_url,omitempty"` // Phase 30
}

// DB query extension — join cosmetic_frames and cosmetic_flairs:
row := e.db.QueryRow(ctx, `
    SELECT vpi.viewer_id::text, vc.name_color, vc.name_gradient,
           COALESCE(cf.image_url, '') AS avatar_frame_url,
           COALESCE(cfl.image_url, '') AS avatar_flair_url
    FROM viewer_platform_identities vpi
    LEFT JOIN viewer_cosmetics vc ON vc.viewer_id = vpi.viewer_id
    LEFT JOIN cosmetic_frames cf ON cf.id = vc.avatar_frame_id
    LEFT JOIN cosmetic_flairs cfl ON cfl.id = vc.avatar_flair_id
    WHERE vpi.platform = $1 AND vpi.platform_user_id = $2
`, msg.Platform, msg.User.ID)
```

After DB scan, inject into `msg.User.AvatarFrameURL` and `msg.User.AvatarFlairURL`.

Cache invalidation: the existing cache key `viewer:identity:{platform}:{platform_user_id}` is already invalidated in `HandlePatchCosmetics` on HTTP 200. No additional invalidation needed.

### Pattern 5: UserAvatar React Component

Three rendering surfaces all use the same structure. The component is purely presentational (no data fetching):

```tsx
// Source: derived from CONTEXT.md composite stacking spec
interface UserAvatarProps {
  avatarUrl?: string
  frameUrl?: string
  flairUrl?: string
  size: number          // px — e.g. 40 for overlay, 32 for extension, 48 for settings preview
  displayName?: string  // for fallback initials
}

// Structural outline (Tailwind classes are Claude's discretion):
// - outer div: relative, overflow-visible, w-[size]px h-[size]px
// - avatar: w-full h-full rounded-full object-cover (Image or fallback div)
// - frame img: absolute, top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2,
//              w-[1.4×size]px h-[1.4×size]px, pointer-events-none
// - flair img: absolute, bottom-0 right-0,
//              w-[0.4×size]px h-[0.4×size]px, pointer-events-none
```

Component location (Claude's discretion): `frontend/src/components/UserAvatar.tsx`

The overlay page (`/overlay/[id]/page.tsx` lines 612–630) replaces its current avatar block with `<UserAvatar avatarUrl={message.user.avatar_url} frameUrl={message.user.avatar_frame_url} flairUrl={message.user.avatar_flair_url} size={40} displayName={message.user.display_name} />`.

The extension's `ChatContainer.tsx` gets the same substitution. Since the extension is a separate repo, `UserAvatar` must be duplicated or inlined there (no shared import path).

### Pattern 6: Admin Page (cosmetics)

Follows `/admin/viewers/page.tsx` exactly:

```tsx
// Source: direct inspection of frontend/src/app/admin/viewers/page.tsx
'use client'
// imports: useEffect, useState, apiClient, toastManager, Card, Button, Badge, Input
// Two tabs (Frames / Flairs) with local activeTab state
// Each tab:
//   1. Existing entries list: name + 64×64 thumbnail + is_premium badge + delete button
//   2. Add-form below: Name input, Image URL input (onBlur triggers thumbnail preview),
//      is_premium toggle, Submit button
// DELETE handler calls apiClient.delete, refetches list
// POST handler calls apiClient.post, refetches list
```

AdminNav in `frontend/src/components/AdminNav.tsx` gets a new entry in `ADMIN_LINKS`:
```ts
{ href: '/admin/cosmetics', label: 'Cosmetics' }
```

### Pattern 7: AvatarCosmeticsCard (settings page)

Added as a new exported function component below `ColorGradientCard` in `/settings/viewer/page.tsx`. Card structure:

- `<Card className="p-6">` — consistent with Phase 29
- H2: "Avatar Cosmetics" with premium badge (matches gradient tab lock pattern)
- Section "Avatar Frame": 4-column grid, items fetched from `GET /api/v1/auth/viewer/catalog/frames`
- Section "Avatar Flair": same, from `GET /api/v1/auth/viewer/catalog/flairs`
- None item first (circle with no image, always free)
- Premium-only items: `opacity-50 cursor-not-allowed` with a small lock icon (SVG or lucide)
- Single Save button at bottom fires `PATCH /api/v1/auth/viewer/cosmetics` with `{ avatar_frame_id, avatar_flair_id }`
- Live preview row shows `<UserAvatar>` immediately on item click (local state update); PATCH fires only on Save

**New public catalog endpoints** (no auth required for browsing — admin controls supply):
```
GET /api/v1/auth/viewer/catalog/frames   → [{id, name, image_url, is_premium}]
GET /api/v1/auth/viewer/catalog/flairs   → same shape
```
These are read-only, require no JWT, served from auth-service under the unauthenticated group.

### Anti-Patterns to Avoid

- **Storing image URLs in viewer_cosmetics directly** — use FK to catalog table; admin delete then cascades gracefully via `ON DELETE SET NULL`.
- **Fetching catalog in the enricher hot path per-message** — resolve frame/flair URL once in the `ViewerBadgeEnricher` DB join and cache it with the rest of the identity cache.
- **Invalidating the enricher Redis cache in the admin catalog delete handler** — not needed: the enricher cache stores resolved URLs; when the catalog entry is deleted, `ON DELETE SET NULL` clears the FK; on the next enricher cache miss the DB join returns empty string; the TTL (5 minutes) is acceptable.
- **Using `next/image` in the `<UserAvatar>` component for frame/flair** — frame and flair images are admin-supplied external URLs with unknown domains; use `<img>` (with `// eslint-disable-next-line`) consistent with how the Phase 29 profile avatar is handled in the settings page.
- **Tabs in the settings cosmetics card** — the decision is one card, two sections, no tabs.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| UUID generation for catalog IDs | Custom ID generation | `gen_random_uuid()` in migration DEFAULT | Already the schema pattern for all PK columns |
| Auth check on admin endpoints | Custom JWT parsing | Existing `middleware.AdminOnly()` gin middleware already on admin group | Phase 28/29 pattern; routes inherit it automatically |
| Cache invalidation | Custom cache watch/subscribe | Existing `redis.Del(cacheKey)` call in `HandlePatchCosmetics` | Same pattern already handles name_color/gradient |
| Frontend fetch with auth header | Custom fetch wrapper | `apiClient` from `@/lib/api/client` | Already used in all admin pages |

---

## Common Pitfalls

### Pitfall 1: Overflow clipping on avatar container in chat rows

**What goes wrong:** Frame is 1.4× avatar size; if the parent `<div>` has `overflow: hidden` or a fixed size, the frame gets cropped.

**Why it happens:** The existing chat message layout (`flex items-start gap-3`) contains a `flex-shrink-0` avatar wrapper. The wrapper dimensions match the avatar (40×40px), so any overflow is clipped by the parent flex item.

**How to avoid:** The `<UserAvatar>` outer div uses `overflow: visible` and the parent flex item in the chat row may need `overflow: visible` added explicitly. The chat row itself may need `gap-3` slightly increased to prevent frame from overlapping adjacent content. Test with a frame that extends to the full 1.4× and check all four sides.

**Warning signs:** Frame appears cropped at the left or top edge during testing.

### Pitfall 2: JSON null vs. absent in PATCH body

**What goes wrong:** The frontend sends `avatar_frame_id: someUUID` to set, and `avatar_frame_id: null` to clear. If the handler uses Go's `json:"avatar_frame_id,omitempty"` on a `*uuid.UUID`, the omitempty tag suppresses null and the handler cannot distinguish "clear" from "not sent".

**Why it happens:** `omitempty` on a pointer type omits the field when nil, so `null` and absent become indistinguishable after decode.

**How to avoid:** Use `*uuid.UUID` without `omitempty` for fields that must support explicit null. The existing pattern in `patchCosmeticsRequest` uses `*string` without omitempty for `name_color` (line 85 in viewer_cosmetics.go) — follow the same pattern.

### Pitfall 3: Image URL validation in admin form

**What goes wrong:** Admin saves a frame with a broken image URL; the enricher injects it; overlays show broken images for all viewers until the admin deletes and re-adds.

**Why it happens:** The URL field is free text; there is no server-side fetch validation.

**How to avoid:** The 64×64 on-blur thumbnail preview in the admin form is the primary guard (user sees broken image before saving). This is sufficient for v1.4; no server-side validation required.

### Pitfall 4: Extension repo type drift

**What goes wrong:** `UserInfo` in `all-chat-extension/src/lib/types/message.ts` has `avatar_frame_url?` and `avatar_flair_url?` added but `ChatContainer.tsx` is not updated to pass them to `<UserAvatar>`, so frames/flairs never render in the extension.

**Why it happens:** The extension is a separate repo; changes must be made in two places and are easy to miss.

**How to avoid:** The extension `UserInfo` interface (currently missing `name_gradient` already, only has `color` not gradient) must be updated. Track it as an explicit task. The extension `ChatContainer.tsx` currently shows no avatar image at all — it shows username-only with color. Adding the `<UserAvatar>` component there requires duplicating or inlining the component since there is no shared import path between the extension and the monorepo.

### Pitfall 5: Catalog GET endpoint — unauthenticated vs. viewer-authenticated

**What goes wrong:** If the catalog GET endpoints are placed behind `viewerProtected` middleware, the settings page cannot pre-load the catalog before the viewer JWT is confirmed valid (during the three-state hydration guard `undefined` phase).

**Why it happens:** The settings page reads the JWT from localStorage in a `useEffect`; during SSR and initial render the token is unknown.

**How to avoid:** Place `GET /viewer/catalog/frames` and `GET /viewer/catalog/flairs` in the unauthenticated router group. The response includes `is_premium` on each item; the client uses the JWT claims to determine which items to lock. Catalog is not sensitive data.

---

## Code Examples

### Querying catalog entries (Go)

```go
// Source: derived from existing pgxpool.Query pattern in auth-service repository files
rows, err := db.Query(ctx, `
    SELECT id, name, image_url, is_premium, created_at
    FROM cosmetic_frames
    ORDER BY created_at ASC
`)
// rows.Scan(&entry.ID, &entry.Name, &entry.ImageURL, &entry.IsPremium, &entry.CreatedAt)
```

### Extending UpsertViewerCosmetics signature

```go
// Source: derived from existing UpsertViewerCosmetics in viewer_identity_repository.go
func (r *ViewerIdentityRepository) UpsertViewerCosmetics(
    ctx context.Context,
    viewerID uuid.UUID,
    nameColor *string,
    nameGradient []byte,
    avatarFrameID *uuid.UUID,  // Phase 30: nil = keep existing, *uuid.Nil = clear
    avatarFlairID *uuid.UUID,  // Phase 30
) error {
    _, err := r.db.Exec(ctx, `
        INSERT INTO viewer_cosmetics (viewer_id, name_color, name_gradient, avatar_frame_id, avatar_flair_id, updated_at)
        VALUES ($1, $2, $3, $4, $5, NOW())
        ON CONFLICT (viewer_id) DO UPDATE SET
            name_color      = EXCLUDED.name_color,
            name_gradient   = EXCLUDED.name_gradient,
            avatar_frame_id = EXCLUDED.avatar_frame_id,
            avatar_flair_id = EXCLUDED.avatar_flair_id,
            updated_at      = NOW()
    `, viewerID, nameColor, nameGradient, avatarFrameID, avatarFlairID)
    return err
}
```

**Note:** The signature change is a breaking change to the `cosmeticsUpsertRepo` interface. The mock in `viewer_cosmetics_test.go` must be updated to match.

### UserInfo Go model extension

```go
// Source: derived from existing UserInfo in services/message-processor/models/message.go line 44
type UserInfo struct {
    ID             string  `json:"id"`
    Username       string  `json:"username"`
    DisplayName    string  `json:"display_name"`
    AvatarURL      string  `json:"avatar_url,omitempty"`
    Badges         []Badge `json:"badges"`
    Color          string  `json:"color,omitempty"`
    NameGradient   string  `json:"name_gradient,omitempty"`
    SourceBadges   []Badge `json:"source_badges,omitempty"`
    SourceUserID   string  `json:"source_user_id,omitempty"`
    AvatarFrameURL string  `json:"avatar_frame_url,omitempty"` // Phase 30
    AvatarFlairURL string  `json:"avatar_flair_url,omitempty"` // Phase 30
}
```

### TypeScript UserInfo extension (monorepo)

```typescript
// Source: derived from frontend/src/lib/types/message.ts line 77
export interface UserInfo {
  id: string;
  username: string;
  display_name: string;
  avatar_url?: string;
  badges: Badge[];
  color?: string;
  name_gradient?: NameGradient;
  source_badges?: Badge[];
  source_user_id?: string;
  avatar_frame_url?: string;  // Phase 30
  avatar_flair_url?: string;  // Phase 30
}
```

### TypeScript UserInfo extension (extension repo)

```typescript
// Source: derived from all-chat-extension/src/lib/types/message.ts line 20
export interface UserInfo {
  id: string;
  username: string;
  display_name: string;
  avatar_url?: string;
  badges: Badge[];
  color?: string;
  name_gradient?: string;     // add if not already present
  avatar_frame_url?: string;  // Phase 30
  avatar_flair_url?: string;  // Phase 30
}
```

### Downgrade enforcement SQL

```sql
-- Called when is_premium flips to false (from PATCH cosmetics handler or admin premium toggle)
UPDATE viewer_cosmetics
SET avatar_frame_id = NULL,
    avatar_flair_id = NULL,
    updated_at = NOW()
WHERE viewer_id = $1;
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| No viewer cosmetics | Phase 28: name_color column in viewer_cosmetics | Phase 28 | Foundation for all cosmetics |
| name_color only | Phase 29: name_gradient JSONB added, is_premium on viewers table | Phase 29 | Premium gate pattern established |
| No catalog entities | Phase 30: cosmetic_frames, cosmetic_flairs, FK columns in viewer_cosmetics | Phase 30 | Enables per-item locking and admin control |

**The enricher has a stable extension pattern:** Phase 29 added `name_gradient` to `viewerIdentityCache` and the SELECT query with a one-line addition. Phase 30 adds two more string fields following identical pattern — the cache TTL and invalidation are unchanged.

---

## Open Questions

1. **Downgrade enforcement trigger**
   - What we know: CONTEXT.md says "when `is_premium` flips to false, server immediately clears frame/flair"
   - What's unclear: Where does the premium flip happen? The `viewers.is_premium` column is written by the admin (no viewer-facing toggle exists yet). The cosmetics PATCH handler checks `is_premium` from the gin context (JWT claim). There is no webhook or trigger for premium status changes.
   - Recommendation: Add the downgrade clear as a SQL step in the cosmetics PATCH handler: when the handler receives a request and `is_premium = false`, execute the clear UPDATE as a side-effect. This is consistent with how the gradient is rejected (403) but goes further to actually clear the stored values. The Redis cache invalidation already happens on PATCH success — this is sufficient.

2. **Catalog GET endpoint: public vs. viewer-token-required**
   - What we know: The decision was public browsing (non-premium users see items with lock icons)
   - What's unclear: Whether the catalog endpoints should accept an optional viewer JWT to pre-populate the selected item IDs in the response
   - Recommendation: Keep catalog endpoints unauthenticated; the selected `avatar_frame_id` and `avatar_flair_id` are loaded separately via `GET /viewer/me` or from the JWT claims. No need to conflate catalog listing with selection state.

3. **Extension UserAvatar duplication**
   - What we know: The extension is a separate repo with no import path to the monorepo
   - What's unclear: Whether to inline `UserAvatar` in `ChatContainer.tsx` or create a separate file in the extension
   - Recommendation: Create `all-chat-extension/src/ui/components/UserAvatar.tsx` as a standalone file — keeps `ChatContainer.tsx` clean and keeps the component reusable in the extension if used elsewhere.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` package + `net/http/httptest` (backend); Vitest (frontend) |
| Config file | `vitest.config.ts` in frontend root |
| Quick run command (backend) | `cd /home/moersener/Hobby/all-chat/services/auth-service && go test ./handlers/... -run TestAdminCosmetics -v` |
| Full suite command (backend) | `cd /home/moersener/Hobby/all-chat/services/auth-service && go test ./...` |
| Full suite command (frontend) | `cd /home/moersener/Hobby/all-chat/frontend && npm run test` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PREM-03 | PATCH cosmetics with avatar_frame_id saves and invalidates cache | unit (Go) | `go test ./handlers/... -run TestPatchCosmetics` | Wave 0 — extend existing test |
| PREM-04 | PATCH cosmetics with avatar_flair_id saves and invalidates cache | unit (Go) | `go test ./handlers/... -run TestPatchCosmetics` | Wave 0 — extend existing test |
| PREM-05 | Admin can list/create/delete frames and flairs | unit (Go) | `go test ./handlers/... -run TestAdminCosmetics` | ❌ Wave 0 |
| WEB-03 | Frame catalog renders with None first, locked premium items, save fires PATCH | manual visual + unit | `cd frontend && npx vitest run --reporter=verbose` | ❌ Wave 0 |
| WEB-04 | Flair catalog renders; selection reflects in live preview | manual visual + unit | `cd frontend && npx vitest run --reporter=verbose` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go test ./handlers/... -count=1` (auth-service) — fast, < 5 seconds
- **Per wave merge:** `go test ./...` in auth-service + message-processor
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `services/auth-service/handlers/admin_cosmetics_test.go` — covers PREM-05 (list/create/delete catalog items via mock DB)
- [ ] `services/auth-service/handlers/viewer_cosmetics_test.go` — extend existing mock to accept two new UUID parameters (covers PREM-03, PREM-04 regression)
- [ ] `services/message-processor/enricher/viewer_badge_enricher_test.go` — extend existing enricher test fixture for `avatar_frame_url` / `avatar_flair_url` injection

---

## Sources

### Primary (HIGH confidence)

- Direct file inspection: `services/auth-service/handlers/viewer_cosmetics.go` — PATCH cosmetics handler pattern
- Direct file inspection: `services/auth-service/handlers/admin_viewers.go` — admin handler pattern
- Direct file inspection: `services/auth-service/repository/viewer_identity_repository.go` — UpsertViewerCosmetics and query patterns
- Direct file inspection: `services/message-processor/enricher/viewer_badge_enricher.go` — viewerIdentityCache, DB query, cache invalidation
- Direct file inspection: `migrations/035_viewer_identity.sql`, `migrations/036_viewer_gradient.sql` — migration numbering and schema pattern
- Direct file inspection: `frontend/src/app/admin/viewers/page.tsx` — admin page structure, apiClient, toastManager usage
- Direct file inspection: `frontend/src/components/AdminNav.tsx` — ADMIN_LINKS array to extend
- Direct file inspection: `frontend/src/app/settings/viewer/page.tsx` — ColorGradientCard pattern, live preview row
- Direct file inspection: `frontend/src/lib/types/message.ts` — UserInfo interface
- Direct file inspection: `all-chat-extension/src/ui/components/ChatContainer.tsx` — extension avatar rendering gap
- Direct file inspection: `all-chat-extension/src/lib/types/message.ts` — extension UserInfo (missing name_gradient already)
- Direct file inspection: `.planning/phases/30-avatar-frame-flair-system/30-CONTEXT.md` — all locked decisions

### Secondary (MEDIUM confidence)

- None needed — all findings verified directly against codebase

### Tertiary (LOW confidence)

- None

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries already in codebase; no new dependencies
- Architecture: HIGH — directly derived from Phase 28/29 patterns verified against live code
- Pitfalls: HIGH — pitfalls derived from observed code patterns (pointer semantics, overflow, repo separation)

**Research date:** 2026-03-16
**Valid until:** 2026-04-16 (stable; no fast-moving external dependencies)
