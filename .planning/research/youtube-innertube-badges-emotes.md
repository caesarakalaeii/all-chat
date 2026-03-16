# YouTube InnerTube: Badges and Emotes

**Domain:** YouTube live chat enrichment via InnerTube (quota-free)
**Researched:** 2026-03-14
**Overall confidence:** HIGH (based on verified open-source implementations that successfully parse actual InnerTube responses)

## Sources

- `YTLiveChat` C# models: https://github.com/Agash/YTLiveChat (actual InnerTube deserialization)
- `xenova/chat-downloader` Python: https://github.com/xenova/chat-downloader (production badge/emoji parsing)
- `darkdread/youtube-live-downloader` example membership JSON: https://github.com/darkdread/youtube-live-downloader/blob/main/documentation/example-membership-msg.md
- `abhinavxd/youtube-live-chat-downloader` Go types: https://github.com/abhinavxd/youtube-live-chat-downloader
- `ys-j/YoutubeLiveChatFlusher` TypeScript types: https://github.com/ys-j/YoutubeLiveChatFlusher/blob/master/ytlivechatrenderer.d.ts

---

## Current State in This Codebase

The `youtube-listener-innertube` service already has correct Go type definitions for both badges and emotes:

```
innertube/types.go:
  AuthorBadge.LiveChatAuthorBadgeRenderer.CustomThumbnail (Thumbnails)
  AuthorBadge.LiveChatAuthorBadgeRenderer.Icon (*IconData)  → IconData.IconType string
  AuthorBadge.LiveChatAuthorBadgeRenderer.Tooltip string

  EmojiData.EmojiID string
  EmojiData.Shortcuts []string
  EmojiData.Image Thumbnails
  // MISSING: IsCustomEmoji bool
```

The parser (`innertube/parser.go`) calls `extractBadges()` which returns `[]string` of badge tooltip strings. The badge image URLs (from `customThumbnail.thumbnails`) are NOT extracted — only the `tooltip` text is captured. This means badge images for membership tiers are currently dropped.

The `extractMessageText()` function already uses `run.Emoji.Shortcuts[0]` as the text placeholder, but does not populate emote image data into the message tags.

---

## Badge Structure (InnerTube)

### Full JSON Example (Verified from darkdread/youtube-live-downloader)

This is a `liveChatMembershipItemRenderer` with an author badge:

```json
{
  "authorBadges": [
    {
      "liveChatAuthorBadgeRenderer": {
        "customThumbnail": {
          "thumbnails": [
            {
              "url": "https://yt3.ggpht.com/c2IBANHDzylVvvAoKx331Mn6ca6u0_2PhOVf5QRA8Ls1TiIF73V35nT46VRJaHhFMvqsh7hwnq9HXuM=s16-c-k"
            },
            {
              "url": "https://yt3.ggpht.com/c2IBANHDzylVvvAoKx331Mn6ca6u0_2PhOVf5QRA8Ls1TiIF73V35nT46VRJaHhFMvqsh7hwnq9HXuM=s32-c-k"
            }
          ]
        },
        "tooltip": "New member",
        "accessibility": {
          "accessibilityData": {
            "label": "New member"
          }
        }
      }
    }
  ]
}
```

### Key observations from `customThumbnail` badges (membership):

- `customThumbnail` is present → this is a **membership badge** (channel-specific image)
- `icon` is absent for membership badges
- `tooltip` contains the tier name: `"New member"`, `"Member (1 month)"`, `"Member (6 months)"`, `"Member (1 year)"`, etc.
- URL format: `https://yt3.ggpht.com/{hash}=s{size}-c-k`
  - Size variants in URL: `s16-c-k` (16px), `s32-c-k` (32px)
  - The hash part is channel-specific and tier-specific
- Two thumbnails always present: 16px and 32px

### Key observations from `icon` badges (system roles):

- `icon.iconType` is present → this is a **system badge** (moderator, owner, verified)
- Known `iconType` values (confirmed from xenova/chat-downloader Python source):
  - `"MODERATOR"` — chat moderator (wrench icon in YouTube UI)
  - `"OWNER"` — channel owner (star icon)
  - `"VERIFIED"` — verified channel (checkmark)
- When `icon` is present, `customThumbnail` is absent
- `tooltip` confirms: `"Moderator"`, `"Owner"`, `"Verified"`

### Badge disambiguation logic (what the parser must do):

```go
// pseudocode
if badge.LiveChatAuthorBadgeRenderer.CustomThumbnail != nil {
    // Membership badge — use first thumbnail URL as icon
    iconURL = badge.LiveChatAuthorBadgeRenderer.CustomThumbnail.Thumbnails[0].URL
    name = "member"
    version = badge.LiveChatAuthorBadgeRenderer.Tooltip  // "New member", "1 month member", etc.
} else if badge.LiveChatAuthorBadgeRenderer.Icon != nil {
    // System badge — map iconType to a known URL or SVG
    switch badge.LiveChatAuthorBadgeRenderer.Icon.IconType {
    case "MODERATOR":  name = "moderator"
    case "OWNER":      name = "owner"
    case "VERIFIED":   name = "verified"
    }
    // System badges have no customThumbnail; use static SVG or hardcoded URLs
}
```

### Required `types.go` additions:

The existing `EmojiData` struct is missing `IsCustomEmoji`. The `LiveChatAuthorBadgeRenderer` already has both `CustomThumbnail` and `Icon` fields — the types are correct, but the parser only reads `Tooltip` and discards everything else.

---

## Emoji/Emote Structure (InnerTube)

### Full Structure (Verified from YTLiveChat C# models and xenova Python parser)

Emojis appear inside `message.runs[]` as an `emoji` object alongside text runs:

```json
{
  "message": {
    "runs": [
      { "text": "Hello " },
      {
        "emoji": {
          "emojiId": "UCxxxxxxxx/custom-emote-id",
          "shortcuts": [":channelEmote:", ":channelEmoteAlias:"],
          "searchTerms": ["channelEmote", "emote"],
          "image": {
            "thumbnails": [
              {
                "url": "https://yt3.ggpht.com/...",
                "width": 24,
                "height": 24
              },
              {
                "url": "https://yt3.ggpht.com/...",
                "width": 48,
                "height": 48
              }
            ],
            "accessibility": {
              "accessibilityData": {
                "label": ":channelEmote:"
              }
            }
          },
          "isCustomEmoji": true,
          "supportsSkinTone": false,
          "variantIds": ["UCxxxxxxxx/custom-emote-id"]
        }
      },
      { "text": " world!" }
    ]
  }
}
```

### Field semantics:

| Field | Present For | Meaning |
|-------|-------------|---------|
| `emojiId` | all emojis | Unique identifier. For custom: `"UCxxxxxxxx/emote-name"` (channel ID prefix). For standard Unicode: emoji character like `"😂"` or `"❤"`. For YouTube global: `"_like"`, `"_heart"`, etc. |
| `shortcuts` | custom emojis, some globals | Text triggers like `":emote:"`. Array. First element is the "canonical" shortcut. May be absent for standard Unicode emoji. |
| `searchTerms` | custom and global | For autocomplete. Similar to shortcuts. |
| `image.thumbnails` | all emojis | Array of thumbnails. Custom: 24x24 and 48x48. Standard Unicode: may be absent (use the Unicode char instead). |
| `isCustomEmoji` | present when true | `true` = channel membership emote or YouTube global emote. `false`/absent = standard Unicode emoji. |
| `supportsSkinTone` | Unicode emoji | Whether the emoji has skin tone variants. |

### Three emoji categories in InnerTube:

1. **Standard Unicode emoji** (`isCustomEmoji` absent/false): Unicode characters like `😂`. No shortcuts. Image thumbnails may or may not be present — render as text character if no image. `emojiId` is the Unicode character itself.

2. **Channel membership emotes** (`isCustomEmoji: true`, `emojiId` starts with `"UC..."`): Custom images uploaded by the channel. Only visible to members. Always have 24x24 and 48x48 thumbnails. Have shortcuts like `":channelEmote:"`.

3. **YouTube global emotes** (`isCustomEmoji: true`, `emojiId` starts with `"_"`): Platform-wide emotes like `"_like"`, `"_heart"`. Introduced 2022. Have image thumbnails and shortcuts.

### No separate catalog endpoint:

There is **no separate InnerTube API endpoint to pre-fetch a channel's emote catalog**. Emotes appear inline within `message.runs[]` when a message contains them. To build an emote map for a channel, you must accumulate them from live chat responses as they arrive (build a per-channel emote cache keyed by `emojiId`).

**Implication for All-Chat:** The emote enrichment approach must work differently from Twitch (which has a static badge/emote API). For YouTube InnerTube emotes:
- Extract `emoji` runs from messages as they are parsed
- Populate the `Emote` struct with `code = shortcuts[0]`, `url = thumbnails[1].URL` (48px), `provider = "youtube"`
- Cache emote images by `emojiId` in Redis for re-rendering
- For `isCustomEmoji: false` and no image thumbnails, substitute the `emojiId` character as text

---

## Current Codebase Gap Analysis

### `innertube/types.go` — Missing field:
```go
// EmojiData is missing:
IsCustomEmoji bool `json:"isCustomEmoji,omitempty"`
```

### `innertube/parser.go` — `extractMessageText()` drops emote images:
Current behavior: converts emoji runs to text using `shortcuts[0]`. This is correct for the raw text field, but the emote image data is thrown away.

Required change: When `run.Emoji != nil`, if `IsCustomEmoji == true` AND image thumbnails exist, the emoji should be tracked separately (like Twitch emotes tracked by position). The message text uses `shortcuts[0]` as the placeholder, and emote positions are recorded by byte offset.

### `innertube/parser.go` — `extractBadges()` drops badge images:
Current behavior: returns `[]string` of tooltip strings. Badge image URLs are not extracted.

Required change: `extractBadges()` should return a richer type that includes `IconURL` (from `customThumbnail.thumbnails[0].URL` for membership badges) and `IconType` (from `icon.iconType` for system badges).

### `message-processor/normalizer/youtube_normalizer.go` — Uses hardcoded SVG badges:
The YouTube normalizer uses inline SVG data URIs for owner/member/moderator/verified badges. These should be replaced by actual image URLs sourced from the InnerTube response via the innertube listener's enriched tags.

---

## Recommended Data Flow Change

To carry badge image URLs from InnerTube to the normalizer, use the existing `tags` map in `RawChatMessage`:

```
tags["badges"] = "member,moderator"   // existing: comma-separated names
tags["badge_member_url"] = "https://yt3.ggpht.com/...=s32-c-k"   // new: badge image URL
tags["badge_member_version"] = "New member"   // new: tier name from tooltip
```

The YouTube normalizer reads these tags to build `[]Badge` with real `IconURL` values instead of SVG data URIs.

---

## YouTube Avatar URL Format

Avatar URLs follow the same `yt4.ggpht.com` pattern:
```
https://yt4.ggpht.com/{hash}=s32-c-k-c0x00ffffff-no-rj   (32px)
https://yt4.ggpht.com/{hash}=s64-c-k-c0x00ffffff-no-rj   (64px)
```
The size suffix `=s{N}` can be changed by rewriting the URL. The base hash before `=` is stable.

---

## Summary for Implementation

| What | Source in InnerTube | Current Status | Change Needed |
|------|--------------------|--------------------|---------------|
| Moderator badge image | `icon.iconType == "MODERATOR"` → no image URL | Static SVG in normalizer | Keep static fallback; it's fine |
| Owner badge image | `icon.iconType == "OWNER"` → no image URL | Static SVG in normalizer | Keep static fallback |
| Membership badge image | `customThumbnail.thumbnails[0].URL` | Dropped in `extractBadges()` | Extract and pass via tags |
| Membership tier name | `tooltip` string | Passed as text in tags["badges"] | Already extractable |
| Custom emote image | `emoji.image.thumbnails[1].URL` (48px) | Dropped in `extractMessageText()` | Extract and emit as Emote[] |
| Custom emote shortcut | `emoji.shortcuts[0]` | Used as text placeholder | Already working for text |
| Unicode emoji | `emoji.emojiId` (the char itself) | Rendered as text | Correct behavior |
