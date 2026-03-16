---
phase: 27-innertube-enrichment-badges-emotes
plan: "02"
subsystem: youtube-listener-innertube
tags:
  - innertube
  - badges
  - emotes
  - redis-cache
  - tdd-green
dependency_graph:
  requires:
    - 27-01  # RED test stubs created in Plan 01
  provides:
    - innertube badge URL extraction (badge_member_url, badge_member_tooltip tags)
    - innertube custom emote extraction (emote_data tag as JSON array)
    - yt_emote_cache package (CacheYTEmotes, EmoteEntry)
    - publisher calls cache writes after XADD
  affects:
    - services/youtube-listener-innertube/innertube
    - services/youtube-listener-innertube/yt_emote_cache
    - services/youtube-listener-innertube/publisher
tech_stack:
  added: []
  patterns:
    - TDD GREEN (turning Plan 01 RED stubs to passing)
    - Struct duplication for cross-package decoupling (EmoteEntry in innertube + yt_emote_cache)
    - Best-effort cache writes with 500ms timeout context
key_files:
  created:
    - services/youtube-listener-innertube/yt_emote_cache/cache.go
  modified:
    - services/youtube-listener-innertube/innertube/types.go
    - services/youtube-listener-innertube/innertube/parser.go
    - services/youtube-listener-innertube/innertube/parser_test.go
    - services/youtube-listener-innertube/publisher/redis_publisher.go
decisions:
  - "extractBadgesRich return order is (memberURL, memberTooltip, badgeTooltips) — follows test signature from Plan 01, not the plan spec which had (badgeTooltips, memberURL, memberTooltip)"
  - "EmoteEntry struct duplicated in both innertube and yt_emote_cache packages to avoid cross-package coupling"
  - "CacheYTEmotes wraps parent ctx with 500ms timeout to prevent blocking main pipeline"
  - "Cache write errors are logged as Warn and never fail message publishing (best-effort)"
metrics:
  duration_minutes: 4
  completed_date: "2026-03-14"
  tasks_completed: 3
  files_modified: 5
---

# Phase 27 Plan 02: Badge URL Extraction and Custom Emote Caching Summary

**One-liner:** InnerTube parser now extracts membership badge image URLs and custom emoji emote data into message tags, with a new yt_emote_cache package writing emotes to Redis at 24h TTL.

## Tasks Completed

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | Add IsCustomEmoji to EmojiData, define EmoteEntry + extractBadgesRich | 49eda0066 | types.go, parser.go |
| 2 | Update extractMessageText to emit EmoteEntry for custom emoji | 49eda0066 | parser.go, parser_test.go |
| 3 | Create yt_emote_cache package and wire cache writes into publisher | f13a1f8cc | yt_emote_cache/cache.go, publisher/redis_publisher.go |

## What Was Built

### Badge URL Extraction
- `extractBadgesRich(badges []AuthorBadge) (memberURL, memberTooltip string, badgeTooltips []string)` added to parser.go
- For membership badges (CustomThumbnail present): picks index 1 thumbnail (48px) or falls back to index 0
- For system badges (Icon only): memberURL stays empty, tooltip still in badgeTooltips
- `parseTextMessage` now sets `tags["badge_member_url"]` and `tags["badge_member_tooltip"]` when present

### Custom Emote Extraction
- `EmojiData.IsCustomEmoji bool` field added to types.go (json:"isCustomEmoji,omitempty")
- `EmoteEntry struct` added to parser.go: Code (shortcut/fallback), URL (48px image), ID (emojiId)
- `extractMessageText` now returns `(string, []EmoteEntry)` — custom emoji runs produce both text placeholder and EmoteEntry; unicode runs produce text only
- `parseTextMessage` marshals non-empty emote slice to `tags["emote_data"]` as JSON array

### Emote Cache Package
- New `yt_emote_cache/cache.go` with `EmoteEntry` struct and `CacheYTEmotes(ctx, rdb, channelID, emotes)` function
- Writes each emote to Redis key `yt:emote:{channelID}:{emojiID}` using SET (refreshes TTL)
- TTL: 24 hours per emote
- Uses 500ms derived timeout context to prevent blocking
- Returns error on Redis failure; caller (publisher) logs as Warn and continues

### Publisher Integration
- `publishToRedis` now reads `msg.Tags["emote_data"]`, unmarshals to `[]yt_emote_cache.EmoteEntry`, calls `CacheYTEmotes` after successful XADD
- Cache errors logged at Warn level, never propagated (best-effort semantics)

## Test Results

All Plan 01 RED tests turned GREEN:

```
--- PASS: TestExtractBadgesRich_MemberBadgeURL
--- PASS: TestExtractBadgesRich_MemberBadgeSingleThumbnail
--- PASS: TestExtractBadgesRich_SystemBadge
--- PASS: TestExtractBadgesRich_NoBadges
--- PASS: TestParseTextMessage_MemberBadgeTagsSet
--- PASS: TestExtractMessageText_CustomEmoji_ChannelMember
--- PASS: TestExtractMessageText_CustomEmoji_Global
--- PASS: TestParseMessages_EmoteDataTag
--- PASS: TestCacheYTEmotes_WritesKey
--- PASS: TestCacheYTEmotes_TTL24h
--- PASS: TestCacheYTEmotes_MultipleEmotes
--- PASS: TestCacheYTEmotes_EmptyList
--- PASS: TestCacheYTEmotes_RefreshesExistingTTL
--- PASS: TestCacheYTEmotes_TimeoutContext
```

Existing tests continue to pass: `ok github.com/caesar/all-chat/services/youtube-listener-innertube/innertube`

Pre-existing unrelated failure: `TestPoller_SuccessfulPolling` — timing-dependent test in poller package, confirmed pre-existing before this plan's changes.

## Deviations from Plan

### Auto-fixed Issues

None — plan executed exactly as written with one clarification:

**Return order of extractBadgesRich:** The plan spec described `(badgeTooltips []string, memberURL string, memberTooltip string)` but the Plan 01 test file uses `memberURL, memberTooltip, badgeTooltips := extractBadgesRich(badges)` (i.e., memberURL first). The test is the source of truth for TDD, so the implementation follows the test's expected return order: `(memberURL string, memberTooltip string, badgeTooltips []string)`.

## Self-Check: PASSED

All 6 key files found on disk. Both commits (49eda0066, f13a1f8cc) confirmed in git log.
