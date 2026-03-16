# Phase 31: All-Chat Platform Badges - Research

**Researched:** 2026-03-16
**Domain:** Badge enrichment (Go), inline SVG React components, badge sort ordering
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Auto-assignment mechanism**
- JOIN at enrichment time — extend ViewerBadgeEnricher DB query with:
  `LEFT JOIN viewer_sessions vs ON vs.viewer_id = vpi.viewer_id LEFT JOIN users u ON u.id = vs.user_id`
  then SELECT `u.is_admin, u.is_premium` alongside existing cosmetics columns
- Result cached in existing Redis identity cache (viewerIdentityCache struct extended with `IsAdmin bool` and `IsPremium bool`)
- Enricher prepends `{Name: "allchat", Version: "1", IconURL: ""}` badge if `is_admin=true`, and `{Name: "premium", Version: "1", IconURL: ""}` if `is_premium=true`
- No `viewer_badges` table — fully derived from user flags
- Manual grant/revoke = existing is_admin / is_premium toggles on admin users page (no new badge-specific UI)

**badge_definitions catalog**
- New `badge_definitions` table with columns: `name VARCHAR PK` (e.g. "allchat", "premium"), `icon_url_1x TEXT`, `icon_url_2x TEXT`, `created_at TIMESTAMPTZ`
- Seeded with two rows: `("allchat", "", "")` and `("premium", "", "")`
- icon_url fields are empty strings — both badges render as inline React components, not CDN images
- Table satisfies BADGE-04 requirement (catalog exists and is the authoritative source for badge metadata)

**Admin badge rendering**
- `<AllChatBadge size={18} />` — reuse `<InfinityLogo size={18} />` component (animated 4-colour infinity snake, existing component)
- Render path: overlay and extension check `badge.name === 'allchat'` → render `<AllChatBadge />` instead of `<img>`
- No CDN URL needed (component is already in the bundle)
- Component renders in overlay (`/overlay/[id]/page.tsx`), extension ChatContainer, and settings page live preview

**Premium badge rendering**
- `<PremiumBadge size={18} />` — new inline SVG component, gem/star shape, design is Claude's discretion
- Same render path as allchat: `badge.name === 'premium'` → render `<PremiumBadge />` instead of `<img>`
- Placed in `frontend/src/components/PremiumBadge.tsx` (and mirrored in extension)

**Badge sort order**
- Add to ROLE_PRIORITIES in `frontend/src/lib/badgeOrder.ts` and `all-chat-extension/src/lib/badgeOrder.ts`:
  ```
  allchat: -2,
  premium: -1,
  ```
- This ensures All-Chat badges sort before moderator (0), vip (1), broadcaster (2)
- Both frontend monorepo and extension need this change

**Badge render size**
- Update ALL badge rendering to `h-[1em] w-auto` responsive sizing (was `w-4 h-4`)
- Applies to overlay (`/overlay/[id]/page.tsx` badge img tags) and extension ChatContainer
- All-Chat badge components rendered at `size={18}` (matches 1em at typical overlay font size)
- Add `title={badge.name}` attribute for tooltip on all badges (both img and component-rendered)

**No new admin UI**
- Existing `is_premium` toggle on admin users page (`/admin/users`) is sufficient for granting/revoking premium badge
- No new badge-specific UI, section, or dialog needed for Phase 31
- `is_admin` management already handled via existing admin tooling

### Claude's Discretion

- `<PremiumBadge size={18} />` gem/star SVG design

### Deferred Ideas (OUT OF SCOPE)

- None raised — discussion stayed within phase scope

</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| BADGE-01 | Admin users automatically receive an All-Chat logo badge shown in overlays | Enricher DB query join on `users.is_admin`; prepend Badge{Name:"allchat"} to msg.User.Badges |
| BADGE-02 | Premium users automatically receive a gem/star icon badge shown in overlays | Enricher DB query join on `users.is_premium`; prepend Badge{Name:"premium"} to msg.User.Badges |
| BADGE-03 | All-Chat badges are prepended to the badge list (rendered before platform badges) | ROLE_PRIORITIES allchat:-2 / premium:-1 guarantees sort order; enricher prepends at injection time |
| BADGE-04 | Badge icon images are served from CDN and specified per badge type in a badge definitions catalog | `badge_definitions` table migration 038 with name PK, icon_url_1x, icon_url_2x columns; seeded with two rows |

</phase_requirements>

---

## Summary

Phase 31 adds two All-Chat-specific badges (admin logo, premium gem) that are prepended to the badge list of any registered viewer who has `users.is_admin=true` or `users.is_premium=true`. All implementation builds on the Phase 28–30 enricher extension pattern: extend the DB query JOIN chain, extend `viewerIdentityCache`, inject badge structs at message enrichment time. No new table for badge assignments — badges are derived flags, not stored records.

The frontend side has two render-site changes (overlay `page.tsx` has two badge render blocks — before and after username — both need updating) and one new component (`PremiumBadge`). The `AllChatBadge` is a thin wrapper around the existing `InfinityLogo` component with a fixed `size=18` default. Both monorepo and extension require identical changes to `badgeOrder.ts` and to the badge render blocks.

The `badge_definitions` table is minimal: `name VARCHAR PRIMARY KEY`, `icon_url_1x TEXT`, `icon_url_2x TEXT`, `created_at TIMESTAMPTZ`. It seeds two rows with empty URL strings. This exists purely to satisfy BADGE-04 (catalog-as-source-of-truth) while icon rendering remains component-based.

**Primary recommendation:** Follow the Phase 30 enricher extension pattern exactly. The work is low-risk and highly patterned — the only novel piece is the DB JOIN extension to reach `users.is_admin/is_premium` via the viewer session link.

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| pgx/v5 | existing | DB query extension | Already used in enricher QueryRow pattern |
| go-redis/v9 | existing | Redis identity cache | viewerIdentityCache already serialized here |
| React 18 / Next.js 14 | existing | AllChatBadge / PremiumBadge components | App Router, inline SVG, no CDN dependency |
| TypeScript | existing | Type-safe badge name guards | Codebase convention |

### No New Dependencies

All work uses existing dependencies. No new npm packages or Go modules needed.

---

## Architecture Patterns

### Pattern 1: Enricher Extension (Phase 28–30 Established)

**What:** Each phase extends `viewer_badge_enricher.go` by (1) adding fields to `viewerIdentityCache`, (2) adding columns to the DB query SELECT and JOIN, (3) adding scan variables, (4) adding injection logic after the scan.

**When to use:** Any time viewer cosmetic data from the DB needs to appear on messages.

**Existing query to extend:**
```go
// Current Phase 30 query (viewer_badge_enricher.go line 117)
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

**Phase 31 extension — add to SELECT and JOIN:**
```sql
-- Add to SELECT:
COALESCE(u.is_admin, false) AS is_admin,
COALESCE(u.is_premium, false) AS is_premium
-- Add to JOIN chain (after existing LEFT JOINs):
LEFT JOIN viewer_sessions vs ON vs.viewer_id = vpi.viewer_id
LEFT JOIN users u ON u.id = vs.user_id
```

**Why COALESCE:** `viewer_sessions` may not exist for non-session viewers; COALESCE to `false` avoids NULL scan into `bool`.

**viewerIdentityCache extension:**
```go
// Add to viewerIdentityCache struct:
IsAdmin   bool `json:"is_admin,omitempty"`
IsPremium bool `json:"is_premium,omitempty"`
```

**Scan variable additions:**
```go
var isAdmin bool
var isPremium bool
// Add to row.Scan(...) as 6th and 7th args
```

**Badge injection (after frame/flair injection, step 6 in Enrich):**
```go
// Phase 31: prepend All-Chat badges for resolved viewers
if isAdmin {
    msg.User.Badges = append([]models.Badge{{Name: "allchat", Version: "1", IconURL: ""}}, msg.User.Badges...)
}
if isPremium {
    msg.User.Badges = append([]models.Badge{{Name: "premium", Version: "1", IconURL: ""}}, msg.User.Badges...)
}
```

**Cache injection (in step 3, building identity struct):**
```go
identity := viewerIdentityCache{
    // ... existing fields ...
    IsAdmin:        isAdmin,
    IsPremium:      isPremium,
}
```

**Cache read injection (in step 1, cache hit path):**
```go
if identity.IsAdmin {
    msg.User.Badges = append([]models.Badge{{Name: "allchat", Version: "1", IconURL: ""}}, msg.User.Badges...)
}
if identity.IsPremium {
    msg.User.Badges = append([]models.Badge{{Name: "premium", Version: "1", IconURL: ""}}, msg.User.Badges...)
}
```

### Pattern 2: fakeViewerDB Extension (Test Pattern)

The test file uses `fakeViewerDB.queryFn` with a fixed signature. Phase 30 extended it to 6 return values. Phase 31 extends to 8:

```go
// Phase 31: fakeViewerDB queryFn signature extension
queryFn func(platform, userID string) (viewerID string, nameColor *string, nameGradient []byte, avatarFrameURL string, avatarFlairURL string, isAdmin bool, isPremium bool, err error)
```

The `noGradientDB` helper also needs updating to pass `false, false` for the two new booleans.

The `fakeRow.Scan` method adds two more `dest` arms for positions 5 and 6 (0-indexed):
```go
if len(dest) >= 6 {
    if bp, ok := dest[5].(*bool); ok { *bp = r.result.isAdmin }
}
if len(dest) >= 7 {
    if bp, ok := dest[6].(*bool); ok { *bp = r.result.isPremium }
}
```

### Pattern 3: Badge Render Name-Check (Frontend)

Both badge render blocks in `overlay/[id]/page.tsx` and the ChatContainer in the extension currently render `<Image>` (or `<img>`) when `badge.icon_url` is truthy, and a `<span>` fallback otherwise. The All-Chat badges have empty `icon_url`, so they fall into the span fallback today. The correct pattern is to check name first:

```tsx
// Replace the current ternary pattern with a 3-way check:
badge.name === 'allchat' ? (
  <AllChatBadge key={idx} size={18} title={badge.name} />
) : badge.name === 'premium' ? (
  <PremiumBadge key={idx} size={18} title={badge.name} />
) : badge.icon_url ? (
  <Image
    key={idx}
    src={badge.icon_url}
    alt={badge.name}
    width={18}
    height={18}
    className="h-[1em] w-auto object-contain"
    title={badge.name}
  />
) : (
  <span key={idx} className="text-xs px-1 py-0.5 rounded bg-gray-700 text-gray-300 leading-none" title={badge.name}>
    {badge.name}
  </span>
)
```

**Two render locations in overlay page.tsx:**
- Lines 642–662: `platformBadgePosition === 'before'` block
- Lines 702–722: `platformBadgePosition === 'after'` block

Both must be updated identically.

**Extension ChatContainer** (line ~454): current render only shows badge if `badge.icon_url` is truthy — All-Chat badges would be silently dropped. Needs the same 3-way name-check pattern. The extension does not use Next.js `<Image>`, it uses native `<img>`.

### Pattern 4: AllChatBadge Component

`AllChatBadge` wraps `InfinityLogo` with a fixed size default:

```tsx
// frontend/src/components/AllChatBadge.tsx
import { InfinityLogo } from '@/components/InfinityLogo'

export function AllChatBadge({ size = 18, title }: { size?: number; title?: string }) {
  return (
    <span title={title} aria-label="All-Chat badge" className="inline-flex items-center">
      <InfinityLogo size={size} />
    </span>
  )
}
```

The extension already has its own `InfinityLogo.tsx` (identical implementation). Mirror `AllChatBadge` at `all-chat-extension/src/ui/components/AllChatBadge.tsx`.

### Pattern 5: PremiumBadge Component

New inline SVG gem component — design at Claude's discretion. Keep it consistent with the overlay aesthetic (dark background, bright gem). A simple gem/diamond SVG at 18px viewBox:

```tsx
// frontend/src/components/PremiumBadge.tsx
export function PremiumBadge({ size = 18, title }: { size?: number; title?: string }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 18 18"
      fill="none"
      aria-label="Premium badge"
      title={title}
      className="inline-block shrink-0"
    >
      {/* gem shape: top crown + bottom diamond */}
      <polygon points="5,2 13,2 16,7 9,16 2,7" fill="#a855f7" stroke="#7c3aed" strokeWidth="1" />
      <line x1="2" y1="7" x2="16" y2="7" stroke="#7c3aed" strokeWidth="0.8" />
      <line x1="5" y1="2" x2="9" y2="7" stroke="#c084fc" strokeWidth="0.6" />
      <line x1="13" y1="2" x2="9" y2="7" stroke="#c084fc" strokeWidth="0.6" />
    </svg>
  )
}
```

Mirror at `all-chat-extension/src/ui/components/PremiumBadge.tsx`.

### Pattern 6: Badge Sort Order Extension

`ROLE_PRIORITIES` currently has `moderator: 0` as the lowest positive value. Both files use the same sort logic: negative priorities will sort before group 'role' (rank < 0 means rank < moderator's 0).

Wait — the sort logic groups badges into `role | subscriber | other` first via `getBadgeGroup`, then by rank within group. `allchat` and `premium` are NOT currently in `ROLE_PRIORITIES`, so they fall into group `other` with `Number.MAX_SAFE_INTEGER` rank. Adding them to `ROLE_PRIORITIES` with values `-2` and `-1` causes them to be classified as group `role` and sorted before `moderator` (rank 0). This is the correct behavior.

```ts
// Add to ROLE_PRIORITIES in both badgeOrder.ts files:
allchat: -2,
premium: -1,
```

### Pattern 7: Migration Numbering

The last migration is `037_cosmetics_catalog.sql`. Phase 31 creates `038_badge_definitions.sql`.

```sql
-- 038_badge_definitions.sql
-- Phase 31: Add badge_definitions catalog table (BADGE-04)

CREATE TABLE IF NOT EXISTS badge_definitions (
    name        VARCHAR(50) PRIMARY KEY,
    icon_url_1x TEXT        NOT NULL DEFAULT '',
    icon_url_2x TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed the two All-Chat badge types
INSERT INTO badge_definitions (name, icon_url_1x, icon_url_2x)
VALUES ('allchat', '', ''), ('premium', '', '')
ON CONFLICT (name) DO NOTHING;
```

No down migration file is strictly required but can be added as `038_badge_definitions_down.sql` following the project pattern.

### Anti-Patterns to Avoid

- **Injecting badges via a separate enricher:** Use the existing `ViewerBadgeEnricher.Enrich()`. All cosmetic injection lives in one place.
- **Reading `is_admin` from viewer_cosmetics or viewers table:** `is_admin` lives on `users` (the application-level auth user). The join path is `vpi → viewer_sessions → users`. Do not shortcut.
- **Hardcoding badge order at render time:** Order is controlled by `badgeOrder.ts` sort. The enricher only needs to prepend; sort will place them correctly.
- **`is_admin` on the `viewers` table:** Confirm — `users.is_admin` (migration 009) and `users.is_premium` (migration 030). The `viewers` table has `is_premium` (migration 036) for viewer-native premium. The CONTEXT.md specifies joining `users.is_admin` and `users.is_premium` — these are the application-level user flags, not the viewer-level flag. Stick to this.
- **Missing `title` attribute on component badges:** The `title` prop must be threaded through `AllChatBadge` and `PremiumBadge` so hover tooltips work identically to `<img title={badge.name}>`.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Badge sort ordering | Custom sort in overlay | `sortBadges` in badgeOrder.ts | Already handles group + rank; just add allchat/premium to ROLE_PRIORITIES |
| Animated logo component | New canvas/CSS animation | `InfinityLogo` (existing) | Identical component already in both monorepo and extension |
| DB join chain | Custom viewer lookup handler | Extend existing enricher QueryRow | All cosmetics use this single hot path; splitting creates cache inconsistency |

---

## Common Pitfalls

### Pitfall 1: viewer_sessions JOIN May Return Multiple Rows

**What goes wrong:** `viewer_sessions` can have multiple rows per `viewer_id` (one per platform session). The LEFT JOIN without LIMIT/DISTINCT can return duplicate rows, causing `Scan()` to receive the wrong row.

**Why it happens:** `QueryRow` uses the first row returned. If multiple `viewer_sessions` rows match, the first is arbitrary.

**How to avoid:** Use `DISTINCT ON (vpi.viewer_id)` or add `LIMIT 1` on the session join, or use a subquery. The safest approach:
```sql
LEFT JOIN LATERAL (
    SELECT user_id FROM viewer_sessions
    WHERE viewer_id = vpi.viewer_id
    LIMIT 1
) vs ON true
LEFT JOIN users u ON u.id = vs.user_id
```

**Warning signs:** Tests with multiple viewer sessions returning inconsistent is_admin values.

### Pitfall 2: Both is_admin and is_premium True Simultaneously

**What goes wrong:** A user is both admin and premium. The enricher appends both badges. The sort gives `allchat: -2` priority over `premium: -1`, which is correct. But if the enricher prepends `allchat` then prepends `premium`, the final order after sort is `allchat, premium, platform-badges`. This is the desired outcome — verify sort produces `allchat` before `premium`.

**How to avoid:** Add an explicit test case for dual-badge user.

### Pitfall 3: Cache Invalidation on is_admin/is_premium Change

**What goes wrong:** An admin grants a user premium status. The Redis cache still holds the old entry (IsAdmin: false, IsPremium: false) for up to 5 minutes. The user won't see their badge immediately.

**Why it happens:** Cache TTL is 5 minutes (ViewerIdentityCacheTTL). Cache is not invalidated on user flag changes.

**How to avoid:** This is acceptable behavior (documented trade-off from Phase 28 caching design). No additional work needed for Phase 31. Document it.

### Pitfall 4: Extension Badge Render Drops All-Chat Badges Today

**What goes wrong:** The current extension ChatContainer only renders a badge if `badge.icon_url` is truthy (line ~454). All-Chat badges have empty `icon_url`. Without the 3-way name-check, all All-Chat badges are silently dropped from the extension display.

**How to avoid:** The name-check conditional must be added to the extension `ChatContainer.tsx` badge render block before checking `icon_url`.

### Pitfall 5: Next.js Image Component Requires Configured Domain

**What goes wrong:** `<Image>` in Next.js requires CDN hostnames in `next.config.js` `images.remotePatterns`. All-Chat badges use component rendering (not `<Image>`), but existing badges using `<Image>` for platform badges may have this restriction.

**How to avoid:** `AllChatBadge` and `PremiumBadge` are inline SVG components — they never use `<Image>`, so this pitfall does not apply to them. The existing badge `<Image>` calls are unchanged.

### Pitfall 6: BADGE_GROUP_ORDER Doesn't Have a Negative-Priority Group

**What goes wrong:** `ROLE_PRIORITIES['allchat'] = -2` places allchat in group `'role'` with rank `-2`. The sort within group 'role' is by rank ascending: `-2 < -1 < 0 (moderator)`. This produces `allchat, premium, moderator, vip, broadcaster`. Verified correct by reading the sort code.

**Warning signs:** If sort placed all-chat badges after platform role badges.

---

## Code Examples

### Enricher: Extended DB Query (Phase 31)
```go
// Source: services/message-processor/enricher/viewer_badge_enricher.go (extend Phase 30 query)
var viewerID string
var nameColor *string
var nameGradientBytes []byte
var avatarFrameURL string
var avatarFlairURL string
var isAdmin bool
var isPremium bool

row := e.db.QueryRow(ctx, `
    SELECT vpi.viewer_id::text, vc.name_color, vc.name_gradient,
           COALESCE(cf.image_url, '') AS avatar_frame_url,
           COALESCE(cfl.image_url, '') AS avatar_flair_url,
           COALESCE(u.is_admin, false) AS is_admin,
           COALESCE(u.is_premium, false) AS is_premium
    FROM viewer_platform_identities vpi
    LEFT JOIN viewer_cosmetics vc ON vc.viewer_id = vpi.viewer_id
    LEFT JOIN cosmetic_frames cf ON cf.id = vc.avatar_frame_id
    LEFT JOIN cosmetic_flairs cfl ON cfl.id = vc.avatar_flair_id
    LEFT JOIN LATERAL (
        SELECT user_id FROM viewer_sessions
        WHERE viewer_id = vpi.viewer_id
        LIMIT 1
    ) vs ON true
    LEFT JOIN users u ON u.id = vs.user_id
    WHERE vpi.platform = $1 AND vpi.platform_user_id = $2
`, msg.Platform, msg.User.ID)

if scanErr := row.Scan(&viewerID, &nameColor, &nameGradientBytes,
    &avatarFrameURL, &avatarFlairURL, &isAdmin, &isPremium); scanErr != nil {
```

### Enricher: Badge Injection
```go
// Source: viewer_badge_enricher.go Enrich() — Phase 31 injection block (after frame/flair, step 7)
if isAdmin {
    msg.User.Badges = append([]models.Badge{{Name: "allchat", Version: "1", IconURL: ""}}, msg.User.Badges...)
}
if isPremium {
    msg.User.Badges = append([]models.Badge{{Name: "premium", Version: "1", IconURL: ""}}, msg.User.Badges...)
}
```

### viewerIdentityCache Struct Extension
```go
type viewerIdentityCache struct {
    ViewerID       string  `json:"viewer_id"`
    NameColor      *string `json:"name_color"`
    NameGradient   []byte  `json:"name_gradient,omitempty"`
    AvatarFrameURL string  `json:"avatar_frame_url,omitempty"`
    AvatarFlairURL string  `json:"avatar_flair_url,omitempty"`
    IsAdmin        bool    `json:"is_admin,omitempty"`   // Phase 31
    IsPremium      bool    `json:"is_premium,omitempty"` // Phase 31
}
```

### badgeOrder.ts Extension (both repos)
```ts
const ROLE_PRIORITIES: Record<string, number> = {
  allchat: -2,   // Phase 31: sort before all platform role badges
  premium: -1,   // Phase 31: sort before moderator/vip/broadcaster
  moderator: 0,
  mod: 0,
  vip: 1,
  broadcaster: 2,
  streamer: 2,
  owner: 2,
};
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Fixed `w-4 h-4` badge size | `h-[1em] w-auto` responsive | Phase 31 | Badge height scales with font size |
| `<img>` fallback for unknown badge icon | Name-check conditional → component | Phase 31 | All-Chat badges render visually instead of being dropped |

---

## Open Questions

1. **viewer_sessions JOIN uniqueness**
   - What we know: `viewer_sessions` has no UNIQUE constraint on `viewer_id`; multiple sessions are possible.
   - What's unclear: In practice, does one viewer ever have multiple active sessions? The `LATERAL ... LIMIT 1` approach is safe regardless.
   - Recommendation: Use `LATERAL (SELECT user_id FROM viewer_sessions WHERE viewer_id = vpi.viewer_id LIMIT 1) vs ON true` to guarantee at most one row.

2. **viewers.is_premium vs users.is_premium**
   - What we know: `viewers.is_premium` (migration 036) exists on the viewer table. `users.is_premium` (migration 030) exists on the application user table. CONTEXT.md specifies joining `users.is_premium`.
   - What's unclear: Are they kept in sync? The context decision is to use `users.is_premium` (the admin-managed flag), not `viewers.is_premium`.
   - Recommendation: Join `users.is_premium` exactly as CONTEXT.md specifies. `viewers.is_premium` may be a separate concept used for viewer-native premium (Phase 29 gradient gating); ignore it in this enricher.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go test (stdlib) + miniredis for enricher; Vitest for frontend |
| Config file | None (go test ./...) / vitest.config.ts |
| Quick run command | `cd services/message-processor && go test ./enricher/... -v` |
| Full suite command | `make test` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| BADGE-01 | Admin viewer gets allchat badge prepended | unit | `go test ./enricher/... -run TestEnrich_AdminBadge` | ❌ Wave 0 |
| BADGE-02 | Premium viewer gets premium badge prepended | unit | `go test ./enricher/... -run TestEnrich_PremiumBadge` | ❌ Wave 0 |
| BADGE-03 | All-Chat badges sort before platform badges | unit | `npx vitest run src/lib/badgeOrder` | ✅ (extend existing) |
| BADGE-04 | badge_definitions table exists with two seed rows | manual-only | psql query | N/A — migration DDL |

### Sampling Rate
- **Per task commit:** `cd services/message-processor && go test ./enricher/... -v`
- **Per wave merge:** `make test`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `services/message-processor/enricher/viewer_badge_enricher_test.go` — extend with admin/premium badge test cases (file exists, add new test functions)
- [ ] `frontend/src/lib/badgeOrder.test.ts` — extend with allchat/premium sort order tests (file may not exist; create with new cases)

*(Existing test infrastructure covers the enricher pattern — only new test functions needed, not new files for the enricher.)*

---

## Sources

### Primary (HIGH confidence)
- Direct code inspection: `services/message-processor/enricher/viewer_badge_enricher.go` — Phase 28–30 enricher pattern
- Direct code inspection: `services/message-processor/enricher/viewer_badge_enricher_test.go` — fakeViewerDB pattern
- Direct code inspection: `services/message-processor/models/message.go` — Badge struct, UserInfo struct
- Direct code inspection: `frontend/src/lib/badgeOrder.ts` — ROLE_PRIORITIES sort logic
- Direct code inspection: `all-chat-extension/src/lib/badgeOrder.ts` — identical sort logic in extension
- Direct code inspection: `frontend/src/app/overlay/[id]/page.tsx` lines 641–722 — two badge render blocks
- Direct code inspection: `all-chat-extension/src/ui/components/ChatContainer.tsx` lines 453–464 — extension badge render (drops empty icon_url badges)
- Direct code inspection: `frontend/src/components/InfinityLogo.tsx` — existing AllChatBadge source component
- Direct code inspection: `all-chat-extension/src/ui/components/InfinityLogo.tsx` — extension mirror (identical)
- Direct code inspection: `migrations/037_cosmetics_catalog.sql` — migration numbering (next is 038)
- Direct code inspection: `migrations/035_viewer_identity.sql` — viewer_platform_identities, viewer_sessions schema
- Direct code inspection: `migrations/030_share_requests.sql` — `users.is_premium` confirmed
- Direct code inspection: `migrations/009_add_admin_role.sql` — `users.is_admin` confirmed
- Direct code inspection: `.planning/phases/31-all-chat-platform-badges/31-CONTEXT.md` — all locked decisions

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; all existing
- Architecture: HIGH — verified Phase 28–30 code patterns in source
- Pitfalls: HIGH — identified from actual source code structure (viewer_sessions join, extension badge render gap)

**Research date:** 2026-03-16
**Valid until:** 2026-04-16 (stable codebase, slow-moving patterns)
