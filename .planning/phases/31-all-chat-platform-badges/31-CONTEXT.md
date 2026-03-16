# Phase 31: All-Chat Platform Badges - Context

**Gathered:** 2026-03-16
**Status:** Ready for planning

<domain>
## Phase Boundary

Admin and premium viewers receive All-Chat-specific badges (allchat logo, premium gem/star icon) that appear in all overlays and the browser extension, prepended before platform badges. The enricher detects badge eligibility at message-processing time via a DB join on users.is_admin / users.is_premium. Two inline React/SVG badge components render the badges in overlay, extension, and settings preview. Badge ordering is fixed in badgeOrder.ts (both monorepo and extension). No viewer_badges table — grant/revoke = existing is_admin/is_premium toggles on the admin users page.

</domain>

<decisions>
## Implementation Decisions

### Auto-assignment mechanism
- **JOIN at enrichment time** — extend ViewerBadgeEnricher DB query with:
  `LEFT JOIN viewer_sessions vs ON vs.viewer_id = vpi.viewer_id LEFT JOIN users u ON u.id = vs.user_id`
  then SELECT `u.is_admin, u.is_premium` alongside existing cosmetics columns
- Result cached in existing Redis identity cache (viewerIdentityCache struct extended with `IsAdmin bool` and `IsPremium bool`)
- Enricher prepends `{Name: "allchat", Version: "1", IconURL: ""}` badge if `is_admin=true`, and `{Name: "premium", Version: "1", IconURL: ""}` if `is_premium=true`
- No `viewer_badges` table — fully derived from user flags
- Manual grant/revoke = existing is_admin / is_premium toggles on admin users page (no new badge-specific UI)

### badge_definitions catalog
- New `badge_definitions` table with columns: `name VARCHAR PK` (e.g. "allchat", "premium"), `icon_url_1x TEXT`, `icon_url_2x TEXT`, `created_at TIMESTAMPTZ`
- Seeded with two rows: `("allchat", "", "")` and `("premium", "", "")`
- icon_url fields are empty strings — both badges render as inline React components, not CDN images
- Table satisfies BADGE-04 requirement (catalog exists and is the authoritative source for badge metadata)

### Admin badge rendering
- **`<AllChatBadge size={18} />`** — reuse `<InfinityLogo size={18} />` component (animated 4-colour infinity snake, existing component)
- Render path: overlay and extension check `badge.name === 'allchat'` → render `<AllChatBadge />` instead of `<img>`
- No CDN URL needed (component is already in the bundle)
- Component renders in overlay (`/overlay/[id]/page.tsx`), extension ChatContainer, and settings page live preview

### Premium badge rendering
- **`<PremiumBadge size={18} />`** — new inline SVG component, gem/star shape, design is Claude's discretion
- Same render path as allchat: `badge.name === 'premium'` → render `<PremiumBadge />` instead of `<img>`
- Placed in `frontend/src/components/PremiumBadge.tsx` (and mirrored in extension)

### Badge sort order
- Add to ROLE_PRIORITIES in `frontend/src/lib/badgeOrder.ts` and `all-chat-extension/src/lib/badgeOrder.ts`:
  ```
  allchat: -2,
  premium: -1,
  ```
- This ensures All-Chat badges sort before moderator (0), vip (1), broadcaster (2)
- Both frontend monorepo and extension need this change

### Badge render size
- Update ALL badge rendering to `h-[1em] w-auto` responsive sizing (was `w-4 h-4`)
- Applies to overlay (`/overlay/[id]/page.tsx` badge img tags) and extension ChatContainer
- All-Chat badge components rendered at `size={18}` (matches 1em at typical overlay font size)
- Add `title={badge.name}` attribute for tooltip on all badges (both img and component-rendered)

### No new admin UI
- Existing `is_premium` toggle on admin users page (`/admin/users`) is sufficient for granting/revoking premium badge
- No new badge-specific UI, section, or dialog needed for Phase 31
- `is_admin` management already handled via existing admin tooling

</decisions>

<specifics>
## Specific Ideas

- The `InfinityLogo` component already exists at `frontend/src/components/InfinityLogo.tsx` — `AllChatBadge` can simply re-export or wrap it with a fixed default size prop
- badge_definitions table design should be minimal: name is the primary key (unique badge type identifier). No `version` column needed at catalog level — version lives on the Badge struct at enrichment time
- Overlay and extension badge render blocks both need updating: there are two badge render locations in the overlay page (before and after username depending on `platformBadgePosition`)
- The enricher already extends viewerIdentityCache for each phase (Phase 29 added NameGradient, Phase 30 added AvatarFrameURL/FlairURL) — Phase 31 adds IsAdmin and IsPremium using the same pattern

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `frontend/src/components/InfinityLogo.tsx`: animated 4-colour infinity logo — reuse as AllChatBadge (wrap with size=18 default)
- `services/message-processor/enricher/viewer_badge_enricher.go`: existing enricher handles color/gradient/frame/flair — extend with badge injection
- `viewerIdentityCache` struct in viewer_badge_enricher.go: add `IsAdmin bool` and `IsPremium bool` fields (same pattern as prior phases)
- `frontend/src/lib/badgeOrder.ts` and `all-chat-extension/src/lib/badgeOrder.ts`: both have identical ROLE_PRIORITIES map — add allchat: -2, premium: -1
- `frontend/src/lib/types/message.ts` UserInfo interface: already has `badges: Badge[]` — no change needed to type

### Established Patterns
- Phase 28–30 enricher extension pattern: extend DB query JOIN + extend viewerIdentityCache struct + inject into msg.User in Enrich()
- Badge struct: `{Name string, Version string, IconURL string}` — AllChat badges use Version "1", IconURL ""
- `sortMessageBadges` called in overlay after resolveTwitchBadgeIcons — All-Chat badges prepended by enricher, preserved by sort thanks to priority -2/-1
- Admin pages follow `frontend/src/app/admin/` layout pattern — no changes needed here

### Integration Points
- `viewer_badge_enricher.go` Enrich() method: extend existing DB query to LEFT JOIN viewer_sessions + users; add badge injection after frame/flair injection (lines after Phase 30 injection block)
- `frontend/src/app/overlay/[id]/page.tsx` lines 641–660: badge map render block — add name-check conditional before `<Image>` render
- Extension ChatContainer: mirror the same badge name-check render logic
- New migration `038_badge_definitions.sql`: CREATE TABLE badge_definitions + seed two rows
- `viewerIdentityCache` in `viewer_badge_enricher.go`: add `IsAdmin bool json:"is_admin"` and `IsPremium bool json:"is_premium"` (omitempty not needed since bool)

</code_context>

<deferred>
## Deferred Ideas

- None raised — discussion stayed within phase scope

</deferred>

---

*Phase: 31-all-chat-platform-badges*
*Context gathered: 2026-03-16*
