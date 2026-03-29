---
status: resolved
trigger: "hesplayingroblox Twitch messages not appearing on overlay 5d968bcf-5d01-4885-b80f-56c5c41c9626"
created: 2026-03-29T19:40:00Z
updated: 2026-03-29T20:15:00Z
symptoms_prefilled: true
---

## Current Focus

hypothesis: RESOLVED — Pipeline working end-to-end. Original issue was same race condition as CaesarLP bug (already fixed). Two secondary issues found: stale PEL entries (fixed operationally) and TikTok duplicate sources (separate bug, tracked separately).
test: Verified all pipeline stages with live data
expecting: Confirm pipeline works
next_action: Archive session, file TikTok coordinator bug separately

## Symptoms

expected: Twitch chat messages from channel hesplayingroblox should appear on the overlay at allch.at
actual: Messages are not reaching the overlay
errors: Unknown at time of report
reproduction: Send a message in hesplayingroblox's Twitch chat and check overlay 5d968bcf-5d01-4885-b80f-56c5c41c9626
started: Recent report, timing unclear

## Eliminated

- hypothesis: hesplayingroblox channel not joined on IRC listener
  evidence: Channel IS present in active_channels on both sets of pods (confirmed twice - before and after redeployment)
  timestamp: 2026-03-29T19:45:00Z

- hypothesis: Messages not entering chat:raw stream
  evidence: Recent Twitch messages from hesplayingroblox confirmed in chat:raw (message 11 seconds old during check, 90 seconds after redeployment)
  timestamp: 2026-03-29T19:46:00Z

- hypothesis: Message-processor not routing to correct overlay
  evidence: Metrics show 443+ Twitch messages published to overlay 5d968bcf; routing query verified correct in DB
  timestamp: 2026-03-29T19:47:00Z

- hypothesis: API gateway not delivering messages to WebSocket
  evidence: When overlay was connected (19:44-19:47), 25+22+39=86 messages delivered across all 3 gateway pods
  timestamp: 2026-03-29T19:48:00Z

- hypothesis: Overlay sources not configured correctly
  evidence: DB shows overlay has both twitch:hesplayingroblox and tiktok:hesplayingroblox sources, both is_active=true, created 2026-02-08
  timestamp: 2026-03-29T19:49:00Z

- hypothesis: Same race condition as CaesarLP bug
  evidence: Fix already deployed (onConnect callback calls ClearActiveChannels); confirmed in connection.go line 35 and cmd/main.go line 159
  timestamp: 2026-03-29T19:50:00Z

- hypothesis: Stale pending messages blocking processor
  evidence: 44,632 stale PEL entries existed but lag=0 (processor keeping up with new messages); cleaned up operationally
  timestamp: 2026-03-29T20:05:00Z

## Evidence

- timestamp: 2026-03-29T19:44:00Z
  checked: twitch-listener pod status endpoint
  found: hesplayingroblox present in active_channels, IRC connected=true
  implication: Channel is properly joined

- timestamp: 2026-03-29T19:44:30Z
  checked: Redis chat:raw stream for recent hesplayingroblox messages
  found: Multiple Twitch messages, most recent 11 seconds old
  implication: Twitch listener is actively publishing messages

- timestamp: 2026-03-29T19:45:00Z
  checked: Redis consumer group for chat:raw message-processor group
  found: 44,632 pending messages from December 2025 through March 2026; lag=0 for new messages
  implication: Stale PEL entries from past pod crashes, cleaned up with XAUTOCLAIM+XACK

- timestamp: 2026-03-29T19:46:00Z
  checked: message-processor metrics
  found: 2178 Twitch messages consumed, 2203 enriched, 443 published to overlay 5d968bcf
  implication: Message-processor pipeline is working correctly

- timestamp: 2026-03-29T19:47:00Z
  checked: API gateway logs for overlay 5d968bcf
  found: Connection established at 19:44:10, subscribed to pub/sub, 25 messages sent, disconnected at 19:47:00 with close 1005
  implication: Messages ARE delivered when overlay is connected; disconnect was normal browser close

- timestamp: 2026-03-29T19:49:00Z
  checked: TikTok messages in chat:raw
  found: 16/200 TikTok messages are exact duplicates (8% duplication rate); TikTok listener logs show 33%+ duplicate rate
  implication: Both TikTok pods connect to same streams (no coordinator assignment), dedup in message-processor handles this downstream

- timestamp: 2026-03-29T20:05:00Z
  checked: Redis XPENDING after cleanup
  found: 0 pending messages for processor-1; cleanup-worker consumer removed
  implication: Stale PEL entries cleaned up; stream consumer group is healthy

- timestamp: 2026-03-29T20:10:00Z
  checked: New twitch-listener pods (after CI redeployment)
  found: hesplayingroblox on pod s4mh5, Twitch messages still flowing (90s old), new message-processor publishing 3 messages to overlay
  implication: Pipeline fully operational after redeployment

## Resolution

root_cause: Pipeline was working when investigated. The original issue was likely the same CaesarLP race condition (channel joined on dead IRC client during reconnect backoff) that was already fixed by the onConnect callback deployment. The channel may have been affected during a previous pod restart and the fix resolved it.

Two secondary issues discovered:
1. 44,632 stale pending messages in chat:raw Redis Stream PEL from past pod crashes (messages 3+ months old) — FIXED: cleaned up with XAUTOCLAIM + XACK
2. TikTok listener missing SERVICE_JWT_SECRET to enable coordinator-based assignment — both pods connect to same streams causing duplicate messages in chat:raw; handled downstream by dedup but wastes storage/CPU — SEPARATE BUG: needs coordinator env var added to TikTok listener deployment

fix: Operational cleanup of stale Redis PEL entries. No code changes required for the Twitch messages pipeline - it's working correctly. Separate bug ticket needed for TikTok coordinator assignment.

verification: Live verification - Twitch messages from hesplayingroblox confirmed in chat:raw (11s and 90s old during checks); 443+ messages published to overlay; 86 messages delivered to WebSocket during 3-minute test connection.

files_changed: []
