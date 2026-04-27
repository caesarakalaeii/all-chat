---
status: verifying
trigger: "youtube-emote-shortcodes-unresolved"
created: 2026-03-30T00:00:00Z
updated: 2026-03-30T00:00:00Z
---

## Current Focus

hypothesis: TWO distinct bugs cause shortcodes to appear as raw text:
  1. Standard Unicode emoji (`:face_with_tears_of_joy:`) have no thumbnail images in YouTube API — no EmoteEntry created, shortcode stays as raw text
  2. YouTube gaming emoji (`:face-fuchsia-tongue-out:`) have thumbnails but renderMessage.tsx has an off-by-one bug: findAllPositions returns exclusive end, but the guard `emote.end >= text.length` treats end as an inclusive index — emotes at end-of-message are dropped
test: Confirmed by code trace: findAllPositions returns exclusive end, renderMessage guard fails for end-of-text emotes
expecting: Fix requires (a) resolving unicode shortcodes to actual unicode codepoints, (b) fixing the exclusive-vs-inclusive end mismatch in renderMessage.tsx
next_action: Implement fixes for both bugs

## Symptoms

expected: YouTube emoji shortcodes (e.g. :face_with_tears_of_joy:, :face-fuchsia-tongue-out:) should render as actual emoji images or unicode emoji in the browser overlay
actual: They appear as raw text like ":face_with_tears_of_joy:" in the overlay. Example messages: "ROSHH MSTT AAP:face-fuchsia-tongue-out:" and "@haibara girl wtf kyuuu:face_with_tears_of_joy:face_with_tears_of_joy:"
errors: No errors reported - the shortcodes just pass through unresolved
reproduction: View the overlay in browser when YouTube chat messages contain emoji shortcodes
started: Ongoing issue, likely since YouTube listener implementation

## Eliminated

## Evidence

- timestamp: 2026-03-30T00:00:00Z
  checked: innertube/parser.go extractMessageText function (lines 397-463)
  found: For non-custom emoji (IsCustomEmoji=false), shortcode is appended to text parts regardless of thumbnail presence. EmoteEntry is only created when len(thumbs) > 0 AND EmojiID != "". Standard Unicode emoji (e.g. :face_with_tears_of_joy:) returned by YouTube API without thumbnails will have no EmoteEntry created — the shortcode text stays raw.
  implication: Unicode shortcodes will always appear as raw text since they have no thumbnail data

- timestamp: 2026-03-30T00:00:00Z
  checked: youtube_normalizer.go findAllPositions and renderMessage.tsx rendering guard
  found: findAllPositions returns [start, end] where end = start + len(substr) — EXCLUSIVE end (Python-style slice). renderMessage.tsx line 61: guard `emote.end >= text.length` treats end as an array index (inclusive), not a slice bound. For an emote at end of string (e.g. :face-fuchsia-tongue-out: in "ROSHH MSTT AAP:face-fuchsia-tongue-out:"), end=39 equals text.length=39 → emote is SKIPPED.
  implication: YouTube gaming emoji with valid EmoteEntries are silently dropped by the renderer when they appear at or near end of message

- timestamp: 2026-03-30T00:00:00Z
  checked: renderMessage.tsx lines 72, 89
  found: Additional off-by-one bugs: `text.slice(emote.start, emote.end + 1)` includes one extra char when end is exclusive; `cursor = emote.end + 1` skips one extra char after the emote
  implication: Even when emotes are not at end-of-string, the text splitting has off-by-one errors that would include or exclude wrong characters

## Resolution

root_cause: TWO bugs caused emoji shortcodes to appear as raw text. (1) Standard Unicode emoji like :face_with_tears_of_joy: are returned by YouTube's InnerTube API with EmojiID "emoji_u1f602" (etc.) and NO thumbnail images. The parser's non-custom emoji branch only added a shortcode placeholder to text without resolving it to the actual Unicode character. (2) YouTube gaming emoji like :face-fuchsia-tongue-out: DO have thumbnails and DO get EmoteEntry objects, but findAllPositions returned exclusive-end positions [start, start+len] while the frontend renderMessage.tsx was written for inclusive-end positions (like Twitch IRC). This caused the guard `emote.end >= text.length` to silently drop emotes at end-of-message, and off-by-one errors in text slicing even for mid-message emotes. Additionally, the TypeScript Emote.provider type was missing 'youtube' as a valid value.

fix: (1) Added resolveUnicodeEmojiID() in parser.go — converts "emoji_u{hex}" IDs (including multi-codepoint sequences like "emoji_u1f1e8_1f1f3" for flag emoji) to actual Unicode characters. The text field in the raw message now contains 😂 instead of ":face_with_tears_of_joy:". (2) Fixed findAllPositions() in youtube_normalizer.go to return inclusive end: end = start + len(substr) - 1. Now consistent with Twitch IRC positions. (3) Updated Emote.provider TypeScript type to include 'youtube'.

verification: All tests pass. go test ./... passes for both youtube-listener-innertube (10 packages) and message-processor (15 packages). TypeScript tsc --noEmit passes with no errors. New tests added: TestResolveUnicodeEmojiID (8 cases), TestExtractMessageText_UnicodeEmoji_ResolvedToChar, TestExtractMessageText_MultiCodepointEmoji.

files_changed:
  - services/youtube-listener-innertube/innertube/parser.go
  - services/youtube-listener-innertube/innertube/parser_emote_test.go
  - services/message-processor/normalizer/youtube_normalizer.go
  - services/message-processor/normalizer/youtube_normalizer_emotes_test.go
  - frontend/src/lib/types/message.ts
