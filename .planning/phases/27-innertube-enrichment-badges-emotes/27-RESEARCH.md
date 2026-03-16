# Phase 27: InnerTube Enrichment — Badges & Emotes - Research

**Researched:** 2026-03-14
**Domain:** YouTube InnerTube chat enrichment (badges + emotes) — surgical changes to existing Go services
**Confidence:** HIGH

## Summary

This phase adds two distinct enrichments to the existing InnerTube → Redis Streams → message-processor pipeline. Both changes are surgical: no new services, no new external APIs, no schema-breaking changes.

**Badges:** The `AuthorBadge` struct in `innertube/types.go` already carries `CustomThumbnail.Thumbnails` for membership badges and `Icon.IconType`+`Tooltip` for system badges. The current `extractBadges()` function in `innertube/parser.go` only extracts tooltip strings into a `tags["badges"]` comma-joined string. It needs to be extended to also emit `tags["badge_member_url"]` and `tags["badge_member_tooltip"]` for membership badges (those with `CustomThumbnail` rather than an `Icon`). The YouTube normalizer in `message-processor` then reads these tags to populate real `Badge.IconURL` on the `member` badge instead of the hardcoded SVG.

**Emotes:** The `EmojiData` struct is missing an `IsCustomEmoji bool` field — InnerTube sends `"isCustomEmoji": true` for channel membership and global emotes. The `extractMessageText()` function currently drops emoji image data entirely, only preserving shortcut text. It must be updated to (a) add `IsCustomEmoji` to `EmojiData`, (b) emit `Emote{}` entries (populated into `tags["emote_data"]` as JSON) for custom emojis, and (c) continue rendering shortcuts as text placeholders in the text field so the message makes sense without the image. The message-processor normalizer reads the emote data from tags and merges them with third-party emotes from the emote service.

**Primary recommendation:** Extend the existing `extractBadges()` and `extractMessageText()` functions with minimal targeted changes, pass enrichment through `tags`, and handle it in the YouTube normalizer. No new services needed.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| YTBADGE-01 | Membership badge renders with real channel-specific image from InnerTube (`customThumbnail.thumbnails[1].URL` at 32px) | `LiveChatAuthorBadgeRenderer.CustomThumbnail` already in `types.go`; pass URL via `tags["badge_member_url"]` |
| YTBADGE-02 | Membership badge tooltip carries tier name (e.g. "3-Month Member") from InnerTube `tooltip` field | `LiveChatAuthorBadgeRenderer.Tooltip` already populated; pass via `tags["badge_member_tooltip"]` |
| YTBADGE-03 | Moderator, owner, and verified badges continue to render (static SVG fallback acceptable) | These use `Icon.IconType` not `CustomThumbnail`; existing normalizer SVG logic handles them via `tags["is_owner"]`, `tags["is_moderator"]`, `tags["is_verified"]` |
| YTBADGE-04 | Backward compatibility: old youtube-listener (quota-based) unaffected | New tags are additive; old listener simply never sets them; normalizer falls back to SVG when absent |
| YTEMOTE-01 | Channel membership emotes (`isCustomEmoji: true`, `emojiId` starts with `UC`) render as inline images | Add `IsCustomEmoji` to `EmojiData`; emit emote entries for these |
| YTEMOTE-02 | Global YouTube emotes (`isCustomEmoji: true`, `emojiId` starts with `_`) render as inline images | Same path as YTEMOTE-01 |
| YTEMOTE-03 | Standard Unicode emoji continue to render as text — no regression | Unicode emoji have `isCustomEmoji: false` (or absent); existing shortcut text path preserved |
| YTEMOTE-04 | Emote images served at 48px (larger InnerTube thumbnail) | `EmojiData.Image.Thumbnails` already contains multiple sizes; pick index 1 (48px) or largest |
| YTEMOTE-05 | Emotes accumulate in per-channel Redis cache keyed by `emojiId` (`yt:emote:{channel_id}:{emoji_id}`, TTL 24h) | New Redis SET operations in innertube publisher or a new helper; message-processor reads and merges |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/redis/go-redis/v9` | v9.18.0 | Redis SET/GET for emote cache | Already used in both services |
| `encoding/json` | stdlib | Encode emote entries in `tags["emote_data"]` | Already used throughout |
| `github.com/stretchr/testify` | v1.11.1 | Unit test assertions | Already project standard |
| `github.com/alicebob/miniredis/v2` | v2.37.0 | In-memory Redis for tests in message-processor | Already in message-processor go.mod |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `strings` | stdlib | Prefix matching for `emojiId` (`UC`, `_`) | Classify emote type |
| `strconv` | stdlib | Integer parsing (thumbnails index) | Already used in parser |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `tags["emote_data"]` JSON blob | New top-level field on `RawChatMessage` | New field would require schema migration across all consumers; tags avoids this |
| Per-message Redis HSET for emotes | Inline in tags | HSET would require message-processor to know channel_id at cache-write time; the innertube service already has channelID |
| 48px thumbnail (index 1) | 32px (index 0) | Requirement YTEMOTE-04 explicitly specifies 48px |

**Installation:** No new dependencies needed for either service.

## Architecture Patterns

### Recommended Project Structure

No new directories needed. Changes are surgical additions to existing files:

```
services/youtube-listener-innertube/
├── innertube/
│   ├── types.go          # Add IsCustomEmoji bool to EmojiData
│   └── parser.go         # Extend extractBadges(), extractMessageText()

services/message-processor/
├── normalizer/
│   └── youtube_normalizer.go  # Read badge_member_url, merge emote_data into Message.Emotes
└── enricher/
    └── emote_enricher.go      # No change needed (emotes merged at normalization time)
```

The emote Redis cache for YouTube emotes lives in the **innertube service** using go-redis/v9 (already available). The key pattern required is `yt:emote:{channel_id}:{emoji_id}` with 24h TTL, which is a simple SET/GET pattern.

### Pattern 1: Badge URL Extraction (innertube/parser.go)

**What:** Distinguish membership badges (have `CustomThumbnail`) from system badges (have `Icon`). For membership badges emit two new tags.
**When to use:** Inside `extractBadges()`, for each `AuthorBadge` where `LiveChatAuthorBadgeRenderer.CustomThumbnail` has thumbnails.

```go
// In innertube/parser.go — extend extractBadges to return structured data
// Source: existing types.go LiveChatAuthorBadgeRenderer
func extractBadgesRich(badges []AuthorBadge) (badgeTooltips []string, memberURL string, memberTooltip string) {
    for _, badge := range badges {
        r := badge.LiveChatAuthorBadgeRenderer
        if len(r.CustomThumbnail.Thumbnails) > 0 {
            // Membership badge: pick index 1 (32px) if available, else index 0
            idx := 0
            if len(r.CustomThumbnail.Thumbnails) > 1 {
                idx = 1
            }
            memberURL = r.CustomThumbnail.Thumbnails[idx].URL
            memberTooltip = r.Tooltip
        }
        if r.Tooltip != "" {
            badgeTooltips = append(badgeTooltips, r.Tooltip)
        }
    }
    return
}
```

The caller (`parseTextMessage`, `parseMembershipMessage`) then sets:
```go
msg.Tags["badge_member_url"] = memberURL
msg.Tags["badge_member_tooltip"] = memberTooltip
```

### Pattern 2: Emote Extraction with IsCustomEmoji (innertube/types.go + parser.go)

**What:** Add `IsCustomEmoji` to `EmojiData`. In `extractMessageText()`, for custom emoji: append a text placeholder (shortcut or emojiId) AND accumulate emote entries for the caller.
**When to use:** When `run.Emoji != nil && run.Emoji.IsCustomEmoji`.

```go
// types.go addition
type EmojiData struct {
    EmojiID        string     `json:"emojiId,omitempty"`
    Shortcuts      []string   `json:"shortcuts,omitempty"`
    Image          Thumbnails `json:"image,omitempty"`
    IsCustomEmoji  bool       `json:"isCustomEmoji,omitempty"`
}

// parser.go: EmoteEntry for transmission
type EmoteEntry struct {
    Code string `json:"code"`   // shortcut or emojiId
    URL  string `json:"url"`    // 48px thumbnail
    ID   string `json:"id"`     // emojiId (for cache keying)
}
```

`extractMessageText()` returns `(text string, emotes []EmoteEntry)`.

The calling function (`parseTextMessage`) marshals emotes to JSON and stores as `tags["emote_data"]`.

### Pattern 3: Emote Redis Cache (innertube service)

**What:** After extracting emotes from a message, upsert each emote into Redis.
**Key format:** `yt:emote:{channelID}:{emojiID}`
**TTL:** 24h
**Where to do it:** In `parseTextMessage` (or a new helper called from there) — the channelID is available.

```go
// Pseudocode — called per message after extracting emotes
func cacheEmotes(ctx context.Context, rdb *redis.Client, channelID string, emotes []EmoteEntry) {
    for _, e := range emotes {
        key := fmt.Sprintf("yt:emote:%s:%s", channelID, e.ID)
        data, _ := json.Marshal(e)
        rdb.Set(ctx, key, data, 24*time.Hour)
    }
}
```

The `parseTextMessage` function currently has no Redis dependency. Two options:
1. **Option A (recommended):** Move cache writes to the publisher/poller layer — after `ParseMessages()` returns, iterate emotes from tags and cache them. The poller/publisher already has a Redis client.
2. **Option B:** Thread Redis client through to the parser. This is invasive and violates the parser's current statelessness.

**Conclusion:** Option A — cache writes in the publisher (or a thin `YTEmoteCache` helper called from the poller loop), not in the parser.

### Pattern 4: YouTube Normalizer Emote Merge (message-processor)

**What:** The YouTube normalizer currently sets `Message.Emotes: []models.Emote{}`. It must parse `tags["emote_data"]` (JSON array of EmoteEntry) and populate `Message.Emotes`.

```go
// In youtube_normalizer.go extractUserInfo or Normalize()
if emoteDataJSON, ok := raw.Tags["emote_data"]; ok && emoteDataJSON != "" {
    var ytEmotes []EmoteEntry  // same struct, or a local equivalent
    if err := json.Unmarshal([]byte(emoteDataJSON), &ytEmotes); err == nil {
        for _, e := range ytEmotes {
            // Find position of placeholder in text
            msg.Message.Emotes = append(msg.Message.Emotes, models.Emote{
                Code:      e.Code,
                Provider:  "youtube",
                URL:       e.URL,
                Positions: findAllPositions(msg.Message.Text, e.Code),
            })
        }
    }
}
```

The `Emote.Positions` pattern (used by Twitch emotes and third-party emote enricher) is `[][]int{{start, end}}`. Since the text placeholder for a custom emoji is its shortcut (e.g., `":UCxxxxx:"`) or emojiId, position detection works the same way as the existing `findWordPosition()` in `emote_enricher.go`.

### Anti-Patterns to Avoid

- **Modifying RawChatMessage struct to add `Emotes` field:** This breaks schema compatibility with the official youtube-listener. Keep emote data in `tags`.
- **Adding Redis to the parser package:** The parser is a pure transformation function. Cache writes belong in the service layer.
- **Fetching emotes from the old emote-service for YouTube:** YouTube custom emotes are not in 7TV/BTTV/FFZ. They accumulate in the `yt:emote:*` cache as messages arrive.
- **Blocking message processing on Redis cache write failures:** Cache writes are best-effort; log and continue.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Thumbnail size selection | Custom size-picking logic | Index 1 if available, else 0 | Already validated: thumbnails[0]=32px, thumbnails[1]=48px per InnerTube docs |
| JSON emote serialization | Custom binary encoding | `encoding/json` | Already used everywhere in this codebase |
| Redis TTL management | Custom expiry loop | `redis.Set(..., 24*time.Hour)` | go-redis handles TTL natively |
| Unicode emoji detection | Unicode range tables | `isCustomEmoji: false` check | InnerTube itself distinguishes custom from Unicode via `isCustomEmoji` field |

**Key insight:** InnerTube already differentiates custom emoji (with image URLs) from Unicode emoji via `isCustomEmoji`. No need for Unicode character table lookups.

## Common Pitfalls

### Pitfall 1: Thumbnail Index Off-By-One
**What goes wrong:** Using `thumbnails[1]` unconditionally causes index out of range panic when only one thumbnail exists.
**Why it happens:** InnerTube occasionally provides only one size for some badge/emote thumbnails.
**How to avoid:** Always check `len(thumbnails) > 1` before accessing index 1; fall back to index 0.
**Warning signs:** Test with a membership badge from a smaller channel (fewer tier images).

### Pitfall 2: Schema Compatibility Break
**What goes wrong:** Adding a new field to `RawChatMessage` in innertube causes the message-processor to fail to decode messages from the old youtube-listener (quota-based) that don't include the field.
**Why it happens:** Go JSON unmarshaling is not strictly additive when types change.
**How to avoid:** Use `tags` map for all new data. `omitempty` on any new struct fields.
**Warning signs:** Integration test with a message that lacks the new tags (simulating old listener output).

### Pitfall 3: Emote Text Placeholder vs. Raw EmojiId
**What goes wrong:** Using `emojiId` directly as the text placeholder (e.g., `UC1234567890/custom_emote`) creates unreadable message text.
**Why it happens:** EmojiId can be a long channel-ID-prefixed string.
**How to avoid:** Prefer `shortcuts[0]` as the placeholder text if available; fall back to a colon-wrapped emojiId.
**Warning signs:** Test with a custom emote that has no shortcuts defined.

### Pitfall 4: Cache Writes Blocking Message Flow
**What goes wrong:** Redis is unavailable during a network hiccup; every message with a custom emoji blocks for the default go-redis timeout.
**Why it happens:** Synchronous XADD + cache write in the same goroutine.
**How to avoid:** Use `context.WithTimeout` for cache writes (500ms), log error, continue. Do not let cache write failure prevent message publication.
**Warning signs:** High publish latency spikes correlating with Redis connection issues.

### Pitfall 5: Normalizer Receives Emote Data but Position Calculation Fails
**What goes wrong:** `findAllPositions` returns empty slice because the placeholder text doesn't appear verbatim in `msg.Message.Text`.
**Why it happens:** If `extractMessageText` joins parts without spaces, the shortcut may be concatenated directly to adjacent text.
**How to avoid:** Verify that `extractMessageText` adds a space before/after emoji placeholders. The existing implementation calls `strings.Join(parts, "")` — ensure emoji shortcut is added as a standalone part with surrounding spaces already present from the message runs.
**Warning signs:** Emotes appear in `Message.Emotes` with empty `Positions`.

## Code Examples

Verified patterns from official sources:

### EmojiData — Current vs. Required (innertube/types.go)
```go
// CURRENT (types.go line 113)
type EmojiData struct {
    EmojiID   string     `json:"emojiId,omitempty"`
    Shortcuts []string   `json:"shortcuts,omitempty"`
    Image     Thumbnails `json:"image,omitempty"`
}

// REQUIRED — add IsCustomEmoji
type EmojiData struct {
    EmojiID       string     `json:"emojiId,omitempty"`
    Shortcuts     []string   `json:"shortcuts,omitempty"`
    Image         Thumbnails `json:"image,omitempty"`
    IsCustomEmoji bool       `json:"isCustomEmoji,omitempty"`
}
```

### Badge URL Tag Setting (innertube/parser.go)
```go
// In parseTextMessage and parseMembershipMessage:
// Source: existing extractBadges pattern, extended
for _, badge := range renderer.AuthorBadges {
    r := badge.LiveChatAuthorBadgeRenderer
    if len(r.CustomThumbnail.Thumbnails) > 0 {
        idx := 0
        if len(r.CustomThumbnail.Thumbnails) > 1 {
            idx = 1 // prefer 32px (index 1 per InnerTube ordering)
        }
        msg.Tags["badge_member_url"] = r.CustomThumbnail.Thumbnails[idx].URL
        msg.Tags["badge_member_tooltip"] = r.Tooltip
    }
}
```

### YouTube Normalizer Badge Enhancement (youtube_normalizer.go)
```go
// CURRENT (lines 82-89 of youtube_normalizer.go):
if tags["is_sponsor"] == "true" {
    badges = append(badges, models.Badge{
        Name:    "member",
        Version: "1",
        IconURL: "data:image/svg+xml,...", // hardcoded SVG
    })
}

// REQUIRED — check for real URL first:
if tags["is_sponsor"] == "true" || tags["badge_member_url"] != "" {
    iconURL := tags["badge_member_url"] // real image from InnerTube
    if iconURL == "" {
        iconURL = "data:image/svg+xml,..." // SVG fallback
    }
    tooltip := tags["badge_member_tooltip"]
    if tooltip == "" {
        tooltip = "Member"
    }
    badges = append(badges, models.Badge{
        Name:    "member",
        Version: tooltip, // tier name in Version field
        IconURL: iconURL,
    })
}
```

Note: The InnerTube service sets `tags["is_sponsor"]` only if `is_sponsor` is present in the original tags from the old listener. The new path triggers on `badge_member_url` being present, which the innertube service now sets.

### Redis Emote Cache Write (in poller/publisher layer)
```go
// Source: existing go-redis/v9 pattern from badge_enricher.go
const YTEmoteTTL = 24 * time.Hour

func cacheYTEmotes(ctx context.Context, rdb *redis.Client, channelID string, emotes []EmoteEntry) {
    cacheCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
    defer cancel()
    for _, e := range emotes {
        key := fmt.Sprintf("yt:emote:%s:%s", channelID, e.ID)
        data, err := json.Marshal(e)
        if err != nil {
            continue
        }
        // Use SET with NX is NOT appropriate here — we want to refresh TTL on each seen emote
        rdb.Set(cacheCtx, key, data, YTEmoteTTL)
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `extractBadges()` returns only tooltip strings | Must return URL + tooltip for membership badges | Phase 27 | Enables real badge images in overlays |
| `extractMessageText()` drops emoji image data | Must emit EmoteEntry alongside text placeholder | Phase 27 | Enables inline YouTube custom emote images |
| YouTube normalizer uses hardcoded SVG for member badge | Must check `tags["badge_member_url"]` first | Phase 27 | Real channel membership badge image shown |
| No YouTube emote cache in Redis | `yt:emote:{channel_id}:{emoji_id}` set per message | Phase 27 | Enables emote catalog accumulation without a catalog API |

**Note about old quota-based listener:** `services/youtube-listener/` never sets `tags["badge_member_url"]` or `tags["emote_data"]`. The normalizer must treat absent tags as "use SVG fallback" / "no emotes". This is already the default behavior since the empty map check is falsy.

## Open Questions

1. **Where to put `EmoteEntry` struct for cross-package use?**
   - What we know: The struct is defined in `innertube` package but the message-processor normalizer needs an equivalent struct for JSON decode.
   - What's unclear: Should `EmoteEntry` be in the `shared` module, duplicated as a local type in the normalizer, or inlined as `[]map[string]string`?
   - Recommendation: Duplicate as a local unexported type in the normalizer (same fields). This avoids coupling services via shared module for a 3-field struct. The planner should decide based on team preference.

2. **Which thumbnail index is 32px vs 48px?**
   - What we know: The requirements say "at 32px" for badges (YTBADGE-01) and "at 48px" for emotes (YTEMOTE-04). InnerTube typically provides thumbnails in ascending size order.
   - What's unclear: Whether thumbnails[0]=32px and thumbnails[1]=48px is guaranteed or channel-specific.
   - Recommendation: For badges use index 1 (assuming 32px is a common second size). For emotes use index 1 (48px). In both cases fall back to index 0 if only one thumbnail exists. Log selected URL in debug output to allow easy validation.

3. **`is_sponsor` tag vs. `badge_member_url` for the "member" badge trigger in the normalizer**
   - What we know: The old listener sets `is_sponsor=true` to signal membership. The innertube parser currently doesn't set `is_sponsor`; it only sets `badges` (tooltip string).
   - What's unclear: The innertube service's `parseTextMessage` does not set `is_sponsor` from InnerTube data — it only sets `tags["badges"]` with the tooltip text.
   - Recommendation: In the normalizer, trigger the member badge on `tags["badge_member_url"] != ""` OR `tags["is_sponsor"] == "true"` (backward compat). The planner should confirm the innertube parser does NOT currently set `is_sponsor`.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) + testify v1.11.1 + miniredis v2.37.0 |
| Config file | none (go test ./...) |
| Quick run command | `cd services/youtube-listener-innertube && go test ./innertube/... -run TestExtract` |
| Full suite command | `cd services/youtube-listener-innertube && go test ./... && cd ../message-processor && go test ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| YTBADGE-01 | `extractBadges` returns `badge_member_url` from `customThumbnail.thumbnails[1]` | unit | `go test ./innertube/... -run TestExtractBadgesRich` | ❌ Wave 0 |
| YTBADGE-02 | `badge_member_tooltip` carries tier name from InnerTube `tooltip` | unit | `go test ./innertube/... -run TestExtractBadgesRich` | ❌ Wave 0 |
| YTBADGE-03 | System badges (mod/owner/verified) still render via SVG fallback | unit | `go test ./normalizer/... -run TestYouTubeNormalizer_ExtractBadges` | ✅ (existing, verify still passes) |
| YTBADGE-04 | Old listener message (no `badge_member_url` tag) → SVG fallback in normalizer | unit | `go test ./normalizer/... -run TestYouTubeNormalizer_ExtractBadges_BackwardCompat` | ❌ Wave 0 |
| YTEMOTE-01 | Channel emote (`isCustomEmoji=true`, id starts `UC`) → `Emote{}` entry in normalized message | unit | `go test ./normalizer/... -run TestYouTubeNormalizer_EmoteData` | ❌ Wave 0 |
| YTEMOTE-02 | Global emote (`isCustomEmoji=true`, id starts `_`) → `Emote{}` entry | unit | `go test ./normalizer/... -run TestYouTubeNormalizer_EmoteData` | ❌ Wave 0 |
| YTEMOTE-03 | Unicode emoji (`isCustomEmoji=false`) → text only, no `Emote{}` entry | unit | `go test ./innertube/... -run TestExtractMessageText_UnicodeEmoji` | ❌ Wave 0 |
| YTEMOTE-04 | Emote URL is from thumbnails index 1 (48px) | unit | `go test ./innertube/... -run TestExtractEmotes_ThumbnailIndex` | ❌ Wave 0 |
| YTEMOTE-05 | Emote Redis cache written with `yt:emote:{cid}:{eid}` key, 24h TTL | unit (miniredis) | `go test ./... -run TestCacheYTEmotes` (in publisher or new yt_emote_cache package) | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `cd services/youtube-listener-innertube && go test ./innertube/... && cd ../../services/message-processor && go test ./normalizer/...`
- **Per wave merge:** Full suite both services
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `services/youtube-listener-innertube/innertube/parser_badge_test.go` — covers YTBADGE-01, YTBADGE-02 (new test file)
- [ ] `services/youtube-listener-innertube/innertube/parser_emote_test.go` — covers YTEMOTE-01, YTEMOTE-02, YTEMOTE-03, YTEMOTE-04
- [ ] `services/message-processor/normalizer/youtube_normalizer_badges_test.go` — covers YTBADGE-03, YTBADGE-04 (extend existing file)
- [ ] `services/message-processor/normalizer/youtube_normalizer_emotes_test.go` — covers YTEMOTE-01, YTEMOTE-02, YTEMOTE-03 in normalizer
- [ ] `services/youtube-listener-innertube/yt_emote_cache/cache_test.go` (or in publisher) — covers YTEMOTE-05 using miniredis; requires adding `miniredis` to innertube go.mod

## Sources

### Primary (HIGH confidence)
- Direct code read: `services/youtube-listener-innertube/innertube/types.go` — EmojiData, AuthorBadge, LiveChatAuthorBadgeRenderer structs
- Direct code read: `services/youtube-listener-innertube/innertube/parser.go` — extractBadges, extractMessageText, parseTextMessage
- Direct code read: `services/message-processor/normalizer/youtube_normalizer.go` — extractBadges, Normalize
- Direct code read: `services/message-processor/models/message.go` — Badge, Emote, MessageInfo structs
- Direct code read: `services/message-processor/cache/emote_cache.go` — Redis cache pattern
- Direct code read: `services/message-processor/enricher/badge_enricher.go` — Redis SET/GET pattern for badges

### Secondary (MEDIUM confidence)
- `.planning/REQUIREMENTS.md` — InnerTube field paths stated as `customThumbnail.thumbnails[1].URL` and `isCustomEmoji` — these match the Go structs exactly

### Tertiary (LOW confidence)
- None needed — all implementation details are directly observable from existing code

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries already present, no new dependencies
- Architecture: HIGH — both change surfaces (parser + normalizer) are small, well-tested, and the data flow is unambiguous
- Pitfalls: HIGH — identified from direct code inspection, not speculation

**Research date:** 2026-03-14
**Valid until:** Stable (no external API changes; entirely internal to the codebase)
