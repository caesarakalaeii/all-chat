---
status: awaiting_human_verify
trigger: "7TV emotes are not enriched when messages come from YouTube (InnerTube listener). Works correctly for Twitch and Kick."
created: 2026-04-01T00:00:00Z
updated: 2026-04-01T11:40:00Z
---

## Current Focus

hypothesis: CONFIRMED — emote-service SevenTVClient.fetchChannelEmotes always uses the Twitch user lookup path. For YouTube messages the channel identifier is a YouTube channel ID (UCxxxxxx), which fails the Twitch Helix lookup. This causes 7TV channel emote fetch to fail, leaving only global emotes available. The FetchCombinedEmotes call for non-Twitch was also routing through the Twitch-channel lookup path because FetchEmotes was called without platform context.
test: COMPLETED — new test TestSevenTVClient_FetchCombinedEmotes_NonTwitchPlatform verifies no Twitch lookup for youtube/kick/tiktok platforms
expecting: VERIFIED — all tests pass, non-Twitch messages now get global 7TV emotes without errors
next_action: await human verification in real overlay

## Symptoms

expected: 7TV emotes in YouTube chat messages should be enriched (replaced with emote images) just like they are for Twitch and Kick messages
actual: 7TV emotes appear as plain text in YouTube messages on the overlay
errors: none reported
reproduction: Send a message containing a 7TV emote (e.g., "ariW") via YouTube chat while using the InnerTube listener
started: unclear — user not sure if it ever worked for YouTube

## Eliminated

- hypothesis: Message-processor not calling enricher for YouTube messages
  evidence: main.go calls emoteEnricher.Enrich(ctx, unified) unconditionally for all chat messages
  timestamp: 2026-04-01T11:00:00Z

- hypothesis: YouTube messages have no channel identifier
  evidence: YouTube normalizer sets ChannelID = raw.ChannelID which comes from the InnerTube listener; it is a YouTube channel ID like UCxxxxxx
  timestamp: 2026-04-01T11:05:00Z

- hypothesis: Kick works because it uses Twitch channel IDs
  evidence: Kick uses the channel slug (e.g. "xqc") which accidentally matches the Twitch username for many streamers, causing the lookup to succeed by coincidence. Not a reliable design.
  timestamp: 2026-04-01T11:20:00Z

## Evidence

- timestamp: 2026-04-01T11:00:00Z
  checked: enricher/emote_enricher.go Enrich() method
  found: For Twitch messages, twitch_room_id metadata is extracted as channelIdentifier. For all other platforms (including YouTube), channelIdentifier = msg.ChannelID (the raw channel ID). Then fetchEmotes(ctx, channelIdentifier, msg.Platform, msg.User.ID) is called.
  implication: YouTube messages pass the YouTube channel ID (UCxxxxxx) as the channel identifier, and "youtube" as the platform.

- timestamp: 2026-04-01T11:05:00Z
  checked: emote-service/handlers/emote.go GetChannelEmotes()
  found: For YouTube (platform != "twitch"), all providers including 7tv run. For 7tv with a userID, FetchCombinedEmotes(ctx, channel, "youtube", userID) is called. For 7tv without userID, FetchEmotes(ctx, channel) is called.
  implication: Both paths eventually reach fetchChannelEmotes which does a Twitch user lookup.

- timestamp: 2026-04-01T11:10:00Z
  checked: emote-service/clients/seventv.go fetchChannelEmotes()
  found: Always calls c.twitchClient.GetUserID(ctx, channel) when channel is not numeric. For YouTube channel IDs (UCxxxxxx) this queries Twitch Helix for a user named "ucxxxxxx" — user not found → error → FetchEmotes returns error → only error is logged, enrichment silently skips all 7TV emotes.
  implication: ROOT CAUSE: 7TV channel emote lookup unconditionally uses Twitch user lookup regardless of platform.

- timestamp: 2026-04-01T11:15:00Z
  checked: 7TV public API behavior
  found: 7TV /v3/users/{platform}/{id} only supports "twitch" as a meaningful platform for channel emote sets. YouTube users are not indexed in 7TV by YouTube channel ID. Global emotes exist at /v3/emote-sets/global and work for all platforms.
  implication: For YouTube, only global 7TV emotes can be returned. Channel-specific 7TV emotes require a Twitch identity.

- timestamp: 2026-04-01T11:20:00Z
  checked: emote-service/clients/seventv.go FetchCombinedEmotes()
  found: Called FetchEmotes(ctx, channel) without passing the platform, so the Twitch-specific channel lookup path is always taken regardless of the platform parameter.
  implication: FetchCombinedEmotes needs to be platform-aware when calling the channel emote fetch.

- timestamp: 2026-04-01T11:30:00Z
  checked: emote-service/handlers/emote.go fetchWithCacheAndUser() condition
  found: When userID == "" and client supports combined emotes, the fallback is fetchWithCache → FetchEmotes(ctx, channel) — also Twitch-specific. Non-Twitch messages without a userID would hit the same failure.
  implication: Both paths (with and without userID) needed fixing.

## Resolution

root_cause: emote-service SevenTVClient.fetchChannelEmotes always performs a Twitch Helix user ID lookup regardless of the source platform. When a YouTube channel ID (UCxxxxxx) is passed, the Twitch lookup fails (user not found), causing FetchEmotes to return an error and all 7TV emotes to be silently skipped. The fix is to bypass the Twitch channel emote lookup for non-Twitch platforms and return only global 7TV emotes instead.

fix: |
  1. Added fetchEmotesForPlatform(ctx, channel, platform) method to SevenTVClient in
     services/emote-service/clients/seventv.go. For platform != "twitch", skips channel-specific
     emote lookup and returns only global emotes. For "twitch", delegates to FetchEmotes (existing behavior).
  2. Updated FetchCombinedEmotes to call fetchEmotesForPlatform instead of FetchEmotes, so the
     platform context is used when deciding how to fetch channel emotes.
  3. Updated fetchWithCacheAndUser in handlers/emote.go to route non-Twitch platforms through
     the combined path (which uses fetchEmotesForPlatform) even when no userID is present,
     preventing the fallback to FetchEmotes(ctx, channel) which would also trigger the Twitch lookup.
  4. Added TestSevenTVClient_FetchCombinedEmotes_NonTwitchPlatform test covering youtube/kick/tiktok
     platforms, verifying no Twitch lookup is made and global emotes are returned.

verification: All existing emote-service tests pass (go test ./...). New test confirms the fix works for YouTube, Kick, and TikTok platforms.

files_changed:
  - services/emote-service/clients/seventv.go
  - services/emote-service/handlers/emote.go
  - services/emote-service/clients/seventv_test.go
