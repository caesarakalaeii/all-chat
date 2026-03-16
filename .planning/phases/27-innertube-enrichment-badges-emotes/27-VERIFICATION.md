---
phase: 27-innertube-enrichment-badges-emotes
verified: 2026-03-14T13:30:00Z
status: passed
score: 7/7 must-haves verified
re_verification: false
---

# Phase 27: InnerTube Enrichment (Badges + Emotes) Verification Report

**Phase Goal:** Enrich YouTube InnerTube chat messages with real badge URLs and custom emote data, passing them through the message pipeline to the unified message format.
**Verified:** 2026-03-14T13:30:00Z
**Status:** passed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Text messages with membership badges emit `tags["badge_member_url"]` (real image URL) and `tags["badge_member_tooltip"]` (tier name) | VERIFIED | `extractBadgesRich` in parser.go:465, wired in parseTextMessage at parser.go:177-184; TestParseTextMessage_MemberBadgeTagsSet PASS |
| 2 | Text messages with custom YouTube emotes (isCustomEmoji=true) emit `tags["emote_data"]` as a JSON array of EmoteEntry objects | VERIFIED | `extractMessageText` returns `(string, []EmoteEntry)` at parser.go:399; marshalled to tags at parser.go:155-159; TestParseMessages_EmoteDataTag PASS |
| 3 | Unicode emoji (isCustomEmoji=false) still render as text only — emote_data not set | VERIFIED | Branch at parser.go:432-437 handles non-custom emoji as text only; TestExtractMessageText_UnicodeEmoji_NoCustom PASS |
| 4 | Emote image URL is from thumbnails index 1 (48px) with fallback to index 0 when only one thumbnail | VERIFIED | Logic at parser.go:415-422; TestExtractMessageText_EmoteURL_UsesIndex1 PASS; TestExtractMessageText_EmoteURL_FallbackIndex0 PASS |
| 5 | After publishing, each custom emoji is written to Redis at `yt:emote:{channelID}:{emojiID}` with 24h TTL | VERIFIED | CacheYTEmotes in cache.go:24-42 with EmoteTTL=24h; publisher wired at redis_publisher.go:148-155; TestCacheYTEmotes_WritesKey + TestCacheYTEmotes_TTL24h PASS |
| 6 | YouTube membership badge renders with real channel-specific image URL in normalizer (not SVG) when badge_member_url tag is present; backward compat maintained | VERIFIED | youtube_normalizer.go:112-130 checks badge_member_url first, SVG fallback via is_sponsor; TestYouTubeNormalizer_ExtractBadges_RealMemberURL PASS; TestYouTubeNormalizer_ExtractBadges_BackwardCompat PASS |
| 7 | Custom YouTube emotes from emote_data tag appear in Message.Emotes with Provider="youtube", correct URL, code and byte-offset Positions | VERIFIED | extractYTEmotes at youtube_normalizer.go:152-175; findAllPositions at youtube_normalizer.go:22-35; TestYouTubeNormalizer_EmoteData_ChannelEmote PASS; TestYouTubeNormalizer_EmoteData_Positions PASS |

**Score:** 7/7 truths verified

---

## Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `services/youtube-listener-innertube/innertube/types.go` | EmojiData.IsCustomEmoji bool field | VERIFIED | Line 117: `IsCustomEmoji bool \`json:"isCustomEmoji,omitempty"\`` |
| `services/youtube-listener-innertube/innertube/parser.go` | extractBadgesRich, EmoteEntry struct, updated extractMessageText | VERIFIED | extractBadgesRich at line 465; EmoteEntry struct at line 456; extractMessageText returns (string, []EmoteEntry) at line 399 |
| `services/youtube-listener-innertube/yt_emote_cache/cache.go` | CacheYTEmotes function with 24h TTL Redis SET | VERIFIED | CacheYTEmotes at line 24; EmoteTTL=24*time.Hour at line 12; uses SET (not SETNX) |
| `services/youtube-listener-innertube/publisher/redis_publisher.go` | Calls CacheYTEmotes after XADD | VERIFIED | yt_emote_cache import at line 12; emote_data parsing and CacheYTEmotes call at lines 148-155 |
| `services/message-processor/normalizer/youtube_normalizer.go` | badge_member_url handling and extractYTEmotes | VERIFIED | badge_member_url check at line 112; extractYTEmotes method at line 152; findAllPositions at line 22 |
| `services/youtube-listener-innertube/innertube/parser_badge_test.go` | 5 badge tests | VERIFIED | All 5 TestExtractBadgesRich_* and TestParseTextMessage_MemberBadgeTagsSet PASS |
| `services/youtube-listener-innertube/innertube/parser_emote_test.go` | 7 emote tests | VERIFIED | All 7 TestExtractMessageText_* and TestParseMessages_EmoteDataTag PASS |
| `services/youtube-listener-innertube/yt_emote_cache/cache_test.go` | 6 cache tests | VERIFIED | All 6 TestCacheYTEmotes_* PASS |
| `services/message-processor/normalizer/youtube_normalizer_badges_test.go` | 6 badge normalizer tests | VERIFIED | All 6 TestYouTubeNormalizer_ExtractBadges_* PASS |
| `services/message-processor/normalizer/youtube_normalizer_emotes_test.go` | 6 emote normalizer tests | VERIFIED | All 6 TestYouTubeNormalizer_EmoteData_* PASS |

---

## Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `parser.go extractBadgesRich` | `parseTextMessage tags["badge_member_url"]` | called at parser.go:177, results stored in msg.Tags | WIRED | Line 177: `memberURL, memberTooltip, badgeTooltips := extractBadgesRich(renderer.AuthorBadges)` |
| `parser.go extractMessageText` | `parseTextMessage tags["emote_data"]` | new (string, []EmoteEntry) return marshalled to JSON tag | WIRED | Lines 150-159: `text, emotes := extractMessageText(renderer.Message)` then marshal to tags |
| `publisher/redis_publisher.go publishToRedis` | `yt_emote_cache.CacheYTEmotes` | called after XADD with emote data parsed from tags["emote_data"] | WIRED | Lines 148-155: reads tags["emote_data"], unmarshals, calls CacheYTEmotes |
| `youtube_normalizer.go extractBadges` | `tags["badge_member_url"] + tags["badge_member_tooltip"]` | check badge_member_url first, SVG fallback via is_sponsor | WIRED | Lines 112-130: badge_member_url branch before is_sponsor fallback |
| `youtube_normalizer.go Normalize` | `tags["emote_data"] JSON array` | json.Unmarshal into []ytEmoteEntry via extractYTEmotes | WIRED | Line 72: `Emotes: n.extractYTEmotes(raw)` |

---

## Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
|-------------|-------------|-------------|--------|---------|
| YTBADGE-01 | 27-01, 27-02, 27-03 | YouTube membership badge renders with real channel-specific image from InnerTube | SATISFIED | TestYouTubeNormalizer_ExtractBadges_RealMemberURL PASS; badge_member_url sets Badge.IconURL |
| YTBADGE-02 | 27-01, 27-02, 27-03 | YouTube membership badge tooltip carries tier name from InnerTube tooltip field | SATISFIED | TestYouTubeNormalizer_ExtractBadges_RealMemberURL checks Version==tooltip; badge_member_tooltip sets Badge.Version |
| YTBADGE-03 | 27-01, 27-03 | YouTube moderator, owner, verified badges continue to render (no regression) | SATISFIED | TestYouTubeNormalizer_ExtractBadges_OwnerBadge, _ModeratorBadge PASS |
| YTBADGE-04 | 27-01, 27-03 | Backward compatibility: old youtube-listener still functions without real badge images | SATISFIED | TestYouTubeNormalizer_ExtractBadges_BackwardCompat + _SVGFallback PASS; is_sponsor path preserved |
| YTEMOTE-01 | 27-01, 27-02, 27-03 | YouTube channel membership emotes (isCustomEmoji=true, emojiId starts with UC) render as inline images | SATISFIED | TestYouTubeNormalizer_EmoteData_ChannelEmote PASS; TestExtractMessageText_CustomEmoji_ChannelMember PASS |
| YTEMOTE-02 | 27-01, 27-02, 27-03 | YouTube global emotes (isCustomEmoji=true, emojiId starts with _) render as inline images | SATISFIED | TestYouTubeNormalizer_EmoteData_GlobalEmote PASS; TestExtractMessageText_CustomEmoji_Global PASS |
| YTEMOTE-03 | 27-01, 27-02, 27-03 | Standard Unicode emoji in YouTube messages continue to render as text (no regression) | SATISFIED | TestYouTubeNormalizer_EmoteData_UnicodeNoEmotes PASS; TestExtractMessageText_UnicodeEmoji_NoCustom PASS |
| YTEMOTE-04 | 27-01, 27-02 | Emote images served at 48px (larger InnerTube thumbnail) for retina clarity | SATISFIED | TestExtractMessageText_EmoteURL_UsesIndex1 PASS; index 1 selection at parser.go:415-419 |
| YTEMOTE-05 | 27-01, 27-02 | Emotes accumulate in per-channel Redis cache keyed by emojiId with 24h TTL | SATISFIED | TestCacheYTEmotes_WritesKey + TestCacheYTEmotes_TTL24h PASS; key pattern `yt:emote:{channelID}:{emojiID}` |

All 9 requirements satisfied. No orphaned requirements found for Phase 27.

---

## Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| parser.go | 407, 433 | "placeholder" in comments | Info | Not a code stub — comment describes text placeholder behavior for emoji rendering. No impact. |

No blockers or warnings found in implementation files.

---

## Human Verification Required

None. All phase 27 behaviors are fully covered by automated unit tests using miniredis and table-driven test cases. The enrichment pipeline is purely data transformation (tags in → structured fields out) with no UI or external service behavior to verify manually.

---

## Gaps Summary

No gaps. All 7 observable truths are verified, all 10 artifacts are substantive and wired, all 5 key links are confirmed, all 9 requirements have GREEN test coverage, and both services build without error.

---

_Verified: 2026-03-14T13:30:00Z_
_Verifier: Claude (gsd-verifier)_
