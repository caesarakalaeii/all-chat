---
phase: 27-innertube-enrichment-badges-emotes
plan: 03
subsystem: api
tags: [youtube, innertube, badges, emotes, normalizer, message-processor]

# Dependency graph
requires:
  - phase: 27-innertube-enrichment-badges-emotes
    provides: "Plan 01 TDD stubs for normalizer badges and emotes tests; Plan 02 InnerTube parser setting badge_member_url, badge_member_tooltip, emote_data tags"
provides:
  - "YouTubeNormalizer.extractBadges reads badge_member_url tag for real channel-specific member badge images"
  - "YouTubeNormalizer.extractYTEmotes parses emote_data JSON tag into Message.Emotes with Provider='youtube'"
  - "findAllPositions helper calculates byte-offset Positions for each emote code in message text"
  - "Backward compatibility maintained: old youtube-listener messages (is_sponsor without badge_member_url) still produce member badge with SVG fallback"
affects:
  - overlay-frontend
  - message-processor

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "badge_member_url tag checked first, SVG fallback only when absent (InnerTube-first enrichment pattern)"
    - "emote_data JSON tag parsed in normalizer into []models.Emote with Positions via findAllPositions"
    - "ytEmoteEntry struct duplicated in normalizer to avoid cross-service coupling"

key-files:
  created: []
  modified:
    - services/message-processor/normalizer/youtube_normalizer.go

key-decisions:
  - "badge_member_url presence triggers member badge; is_sponsor remains as fallback for old youtube-listener compatibility"
  - "extractYTEmotes added as separate method on YouTubeNormalizer to keep Normalize() clean"
  - "findAllPositions returns nil (not empty slice) when substr is empty, consistent with Go nil slice idiom"
  - "NormalizeEvent Message.Emotes left as []models.Emote{} — events (Super Chat, membership) intentionally have no emote enrichment"

patterns-established:
  - "Tag-based enrichment: normalizer reads structured tags set by listener, no direct API calls from normalizer"
  - "Graceful JSON degradation: invalid emote_data returns empty Emotes, no error propagated to caller"

requirements-completed:
  - YTBADGE-01
  - YTBADGE-02
  - YTBADGE-03
  - YTBADGE-04
  - YTEMOTE-01
  - YTEMOTE-02
  - YTEMOTE-03

# Metrics
duration: 12min
completed: 2026-03-14
---

# Phase 27 Plan 03: YouTube Normalizer Badge URL + Emote Data Parsing Summary

**YouTubeNormalizer updated to consume InnerTube badge_member_url and emote_data tags, completing the end-to-end enrichment pipeline from InnerTube parser through to overlay-ready unified messages**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-03-14T13:10:00Z
- **Completed:** 2026-03-14T13:22:00Z
- **Tasks:** 3
- **Files modified:** 1

## Accomplishments
- extractBadges now checks badge_member_url first, using real channel-specific image URLs from InnerTube with tooltip as badge Version; falls back to SVG for old listener messages
- extractYTEmotes parses the emote_data JSON array tag into []models.Emote entries with Provider="youtube" and byte-offset Positions
- All 9 Phase 27 requirements (YTBADGE-01 through YTBADGE-04, YTEMOTE-01 through YTEMOTE-03 + cache tests from Plan 02) are traceable to GREEN tests
- Full test suites pass for message-processor; youtube-listener-innertube passes all tests except a pre-existing poller Redis connectivity test unrelated to Phase 27

## Task Commits

1. **Task 1: Update extractBadges to use badge_member_url with SVG fallback** - `652ed45a4` (feat)
2. **Task 2: Parse emote_data tag into Message.Emotes** - `08ea543ea` (feat)
3. **Task 3: Full test suite validation** - no code changes (verification only)

## Files Created/Modified
- `services/message-processor/normalizer/youtube_normalizer.go` - Added badge_member_url logic in extractBadges, added ytEmoteEntry struct, findAllPositions helper, extractYTEmotes method

## Decisions Made
- badge_member_url presence alone triggers member badge without requiring is_sponsor (InnerTube listener never sets is_sponsor)
- is_sponsor remains as a fallback path for backward compatibility with old quota-based youtube-listener
- ytEmoteEntry struct duplicated in normalizer package (not imported from yt_emote_cache) to avoid cross-service module coupling — consistent with Plan 02 decision
- NormalizeEvent.Message.Emotes intentionally left as empty slice: events (Super Chat, membership events) carry no custom emotes

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

Pre-existing failure in `TestPoller_SuccessfulPolling` in youtube-listener-innertube: test attempts to connect to Redis at localhost:6379 which is not running in the development environment. This failure predates Phase 27 and is unrelated to normalizer changes. All tests for packages modified in Phase 27 (innertube, yt_emote_cache, normalizer) pass.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Complete end-to-end enrichment pipeline is operational: InnerTube parser (Plan 02) sets tags, normalizer (Plan 03) consumes tags
- YouTube chat overlays will display real channel-specific member badge images instead of generic SVG icons
- YouTube custom emotes from InnerTube appear in Message.Emotes with correct Provider, URL, and Positions
- Phase 27 complete: all requirements YTBADGE-01 through YTEMOTE-03 satisfied

---
*Phase: 27-innertube-enrichment-badges-emotes*
*Completed: 2026-03-14*
