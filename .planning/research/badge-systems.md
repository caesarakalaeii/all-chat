# Badge Systems

**Domain:** Badge rendering patterns for multi-platform chat overlays
**Researched:** 2026-03-14
**Overall confidence:** HIGH (based on existing codebase, Twitch API docs, and InnerTube reverse engineering)

## Sources

- Existing codebase: `services/message-processor/enricher/badge_enricher.go`
- Existing codebase: `services/message-processor/normalizer/youtube_normalizer.go`
- Twitch Helix Badges API: https://dev.twitch.tv/docs/api/reference/#get-global-chat-badges
- YouTube InnerTube badge structure: see `youtube-innertube-badges-emotes.md`
- Open-source chat overlay: https://github.com/Ciremun/chat-overlay

---

## Existing Badge Pipeline

The current badge pipeline for All-Chat:

```
Platform listener
  → tags["badges"] or flags (is_moderator, is_sponsor, etc.)
  → YouTube/Twitch normalizer builds []Badge{Name, Version, IconURL}
  → BadgeEnricher (Twitch-only) replaces IconURL with Helix API URL
  → Published to overlay via Redis Pub/Sub
  → Extension/frontend renders <img> per badge
```

### Current Badge struct:

```go
type Badge struct {
    Name    string `json:"name"`     // "subscriber", "moderator", "member"
    Version string `json:"version"`  // badge tier/version identifier
    IconURL string `json:"icon_url"` // URL to badge image (or SVG data URI)
}
```

This struct is correct and extensible. No structural changes needed.

---

## Badge Sources by Platform

### Twitch

| Badge | Source | Pipeline |
|-------|--------|----------|
| Global (mod, staff, broadcaster) | Twitch Helix `/chat/badges/global` | BadgeEnricher fetches, caches 24h in Redis |
| Channel (subscriber tiers, bits) | Twitch Helix `/chat/badges?broadcaster_id=X` | BadgeEnricher fetches, caches 24h |
| IRC Wire | IRC tags `badges=subscriber/12,moderator/1` | TwitchNormalizer builds Badge list from IRC |

The Twitch normalizer creates stub Badge objects from IRC tag info. The `BadgeEnricher` then fills in real `IconURL` values from the Helix API. This two-phase approach is correct.

### YouTube (InnerTube)

| Badge | Source | Current Status | Needed Change |
|-------|--------|---------------|---------------|
| Moderator | `icon.iconType = "MODERATOR"` | Hardcoded SVG | Keep SVG fallback (no image URL from InnerTube) |
| Owner/Broadcaster | `icon.iconType = "OWNER"` | Hardcoded SVG | Keep SVG |
| Verified | `icon.iconType = "VERIFIED"` | Hardcoded SVG | Keep SVG |
| Membership | `customThumbnail.thumbnails[0].URL` | SVG, image dropped | Extract real URL from InnerTube |

For YouTube, the InnerTube response contains the actual membership badge image URL directly in `liveChatAuthorBadgeRenderer.customThumbnail.thumbnails`. This URL is channel-and-tier-specific. There is no separate badge API needed — the URL is in the message itself.

**The innertube parser must extract membership badge URLs and pass them via `tags`.**

### Kick

Kick badges are parsed from Pusher WebSocket events. The Kick listener currently handles these. Badge URLs follow the Kick CDN pattern.

### TikTok

TikTok listener handles role badges from the unofficial library. Limited badge variety.

---

## All-Chat Platform Badges (New Feature)

Beyond platform badges, v1.4 introduces All-Chat-specific badges visible to viewers using the browser extension. These are badges assigned by the streamer or earned by the viewer.

### Badge types to introduce:

| Badge | Trigger | Visual |
|-------|---------|--------|
| Premium viewer | Viewer has premium subscription | Gold star or crown |
| All-Chat user | Viewer is authenticated in extension | Small All-Chat logo |
| Streamer-granted VIP | Streamer marks viewer as VIP | Custom VIP icon |
| Regular | Viewer has visited N streams | Progress-based |

### Injection point:

All-Chat badges must be injected AFTER platform normalization and AFTER platform badge enrichment. A new `ViewerBadgeEnricher` (or extending the existing enricher) queries viewer data and prepends All-Chat badges to `msg.User.Badges`.

**Recommended enricher order:**
1. Platform normalization (creates initial `[]Badge`)
2. `BadgeEnricher` (fills Twitch badge IconURLs from Helix API)
3. `YouTubeBadgeEnricher` (fills YouTube membership badge IconURLs from InnerTube tags) — NEW
4. `ViewerBadgeEnricher` (prepends All-Chat platform badges for authenticated viewers) — NEW

### Badge ordering convention:

Badges should be rendered left-to-right with highest-priority first:
```
[All-Chat badge] [Platform-role badge] [Subscriber/Member badge]
```

The `BadgeOrder` concept already exists in the extension (`src/lib/badgeOrder.ts`). The backend should produce badges in priority order.

### ViewerBadgeEnricher data flow:

```
Message arrives with user.ID and platform
  → Look up viewer_id = cache["viewer_platform:{platform}:{user_id}"]
  → If found, look up viewer_badges = cache["viewer_badges:{viewer_id}"]
  → Prepend viewer's All-Chat badges to msg.User.Badges
  → If not found, skip (user has no All-Chat account linked)
```

Cache TTL: 5 minutes. Invalidated when viewer changes badges.

### Database model for viewer badge assignments:

```sql
-- Platform-to-viewer identity mapping
CREATE TABLE viewer_platform_identities (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    viewer_id   UUID NOT NULL REFERENCES viewers(id) ON DELETE CASCADE,
    platform    VARCHAR(16) NOT NULL,  -- "twitch", "youtube", "kick"
    platform_user_id VARCHAR(128) NOT NULL,
    platform_username VARCHAR(128),
    linked_at   TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(platform, platform_user_id)
);

-- All-Chat badges earned or granted
CREATE TABLE viewer_badges (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    viewer_id   UUID NOT NULL REFERENCES viewers(id) ON DELETE CASCADE,
    badge_type  VARCHAR(32) NOT NULL,  -- "premium", "allchat", "vip", "regular"
    badge_version VARCHAR(16) DEFAULT '1',
    granted_by  UUID REFERENCES users(id),  -- NULL = system-granted
    granted_at  TIMESTAMPTZ DEFAULT NOW(),
    expires_at  TIMESTAMPTZ,            -- NULL = permanent
    UNIQUE(viewer_id, badge_type)
);

-- Badge icon definitions (static catalog)
CREATE TABLE badge_definitions (
    badge_type  VARCHAR(32) PRIMARY KEY,
    icon_url_1x TEXT NOT NULL,
    icon_url_2x TEXT,
    description TEXT,
    sort_order  INT DEFAULT 0
);
```

---

## Badge Rendering in Frontend/Extension

### Current extension rendering (`renderMessage.tsx`):

The extension renders messages from the unified `ChatMessage` type. Badges are in `message.user.badges[]`. Each badge has `icon_url`. The extension renders:

```tsx
{user.badges.map(badge => (
  badge.icon_url ? (
    <img
      key={badge.name}
      src={badge.icon_url}
      alt={badge.name}
      title={badge.name}
      className="inline-block h-[1em] w-auto align-text-bottom mr-0.5"
    />
  ) : null
))}
```

This pattern is correct. It already handles both image-URL badges and (by skipping null icon_url) invisible badges.

### Badge rendering for SVG data URIs (system badges):

The existing YouTube normalizer uses SVG data URIs for moderator/owner/verified badges. These render correctly because `<img src="data:image/svg+xml,..."` works in all browsers. No change needed here — SVG data URIs are a valid pattern when no remote image URL is available.

### Badge sizing:

Chat badges in streaming overlays are universally rendered at 16-18px height (matching line height). The existing `h-[1em]` class is correct for the extension. The overlay's CSS should use the same.

### Tooltip on hover:

Badge tooltips should use the `title` attribute (already in the pattern above) or a custom CSS tooltip. The `tooltip` field from InnerTube (e.g., `"New member"`) should be surfaced in the `Badge.Name` or as a separate `Badge.Tooltip` field.

**Recommendation:** Add `Tooltip string` to the `Badge` struct to carry the human-readable label. This already exists conceptually (`Badge.Name` is close, but `"member"` is less descriptive than `"Member (3 months)"`).

```go
type Badge struct {
    Name    string `json:"name"`    // machine name: "member", "moderator"
    Version string `json:"version"` // tier: "3", or tooltip text for YouTube
    IconURL string `json:"icon_url"`
    // Future addition for richer tooltips:
    // Label string `json:"label,omitempty"` // human-readable: "3-Month Member"
}
```

---

## Platform Badge Image URL Patterns

| Platform | URL pattern | Resolution |
|----------|-------------|------------|
| Twitch global | `https://static-cdn.jtvnw.net/badges/v1/{id}/1` | 18x18, 36x36, 72x72 |
| Twitch channel | `https://static-cdn.jtvnw.net/badges/v1/{id}/1` | Same |
| YouTube membership | `https://yt3.ggpht.com/{hash}=s16-c-k` | 16px and 32px |
| YouTube system | No remote URL — use SVG or YouTube's static badge icons | — |
| All-Chat | Object storage (S3/R2) via CDN | 18x18, 36x36 |

### URL manipulation for YouTube badge resizing:

The YouTube badge URL `=s16-c-k` suffix can be changed to get larger versions:
```
=s16-c-k  → 16px
=s32-c-k  → 32px
=s64-c-k  → 64px (not guaranteed to exist)
```

Use the 32px version for display to support retina screens. The 16px is standard definition.

---

## Migration from SVG Data URIs to Real YouTube Badge Images

The YouTube normalizer currently uses hardcoded SVG data URIs for membership badges. The migration path:

1. InnerTube parser extracts `customThumbnail.thumbnails[0].URL` and stores in `tags["badge_member_url"]`
2. InnerTube parser stores `tooltip` in `tags["badge_member_tooltip"]`
3. YouTube normalizer reads `tags["badge_member_url"]` (if present) instead of using hardcoded SVG
4. Falls back to hardcoded SVG if `tags["badge_member_url"]` is empty (for messages from the old youtube-listener that uses the public API)

This maintains backward compatibility with the quota-based YouTube listener while enabling real badge images from the InnerTube listener.
