---
phase: 29-inbound-enrichment
verified: 2026-03-16T08:30:00Z
status: passed
score: 10/10 must-haves verified
re_verification: false
---

# Phase 29: Inbound Enrichment Verification Report

**Phase Goal:** Discord messages carry deletion events and resolved mention text through the existing platform pipelines
**Verified:** 2026-03-16T08:30:00Z
**Status:** passed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | When a MESSAGE_DELETE event arrives for a configured inbound channel, a deletion event with EventType=message_deletion is published to chat:raw Redis Stream | VERIFIED | `HandleMessageDelete` in client.go lines 413–443; publishes `event_type: "message_deletion"` with `target_msg_id` and `deletion_type: "single"`. `TestHandleMessageDelete_HappyPath` confirms payload fields. |
| 2 | When a MESSAGE_DELETE_BULK event arrives, one deletion event per message ID is published | VERIFIED | `HandleMessageDeleteBulk` in client.go lines 447–459 iterates IDs and calls `HandleMessageDelete` per entry. `TestHandleMessageDeleteBulk_HappyPath` asserts 3 publishes with distinct `target_msg_id` values. |
| 3 | MESSAGE_DELETE/BULK from channels NOT in ChannelRegistry are silently dropped | VERIFIED | `HandleMessageDelete` returns nil immediately when `found==false` (line 424–426). Tests `TestHandleMessageDelete_UnknownChannel` and `TestHandleMessageDeleteBulk_UnknownChannel` assert 0 publishes. |
| 4 | A deleted Discord message disappears from overlays consistent with Twitch/YouTube deletion behavior (EventType=message_deletion, EventData.deletion_type=single, EventData.target_msg_id=snowflake) | VERIFIED | Schema matches plan spec exactly: `event_type=message_deletion`, `event_data.deletion_type=single`, `event_data.target_msg_id=msg.ID`. `TestHandleMessageDelete_HappyPath` validates all three fields. |
| 5 | A message containing `<@USER_ID>` renders as @alice (resolved global_name or username) | VERIFIED | `ResolveMentions` in client.go lines 542–567 uses regex `<@!?(\d+)>`, looks up `mentionMap[id]`, returns `"@" + GlobalName` (fallback to Username). `TestResolveMentions_UserMention` and `TestResolveMentions_GuildMemberVariant` pass. |
| 6 | A message containing `<#CHANNEL_ID>` renders as #general | VERIFIED | `reChannelMention` regex at line 531; `cache.GetChannelName` called, result prefixed with `#`. `TestResolveMentions_ChannelMention` passes. |
| 7 | A message containing `<@&ROLE_ID>` renders as @Moderators | VERIFIED | `reRoleMention` regex at line 534; `cache.GetRoleName` called, result prefixed with `@`. `TestResolveMentions_RoleMention` passes. |
| 8 | Unresolvable mentions fall back gracefully: user/role → @unknown, channel → #channel | VERIFIED | Lines 563/583/600 return `"@unknown"` for user/role misses and `"#channel"` for channel misses, each logging DEBUG. Tests `TestResolveMentions_UnresolvableUser/Channel/Role` pass. |
| 9 | Channel and role name caches populated from GUILD_CREATE; kept current via CHANNEL_*/GUILD_ROLE_* events | VERIFIED | `HandleGuildCreate` (lines 464–489), `HandleChannelUpdate/Delete` (lines 493–507), `HandleGuildRoleUpdate/Delete` (lines 511–525) all implemented and wired in `Connect()` dispatch loop. 6 tests in `guild_cache_test.go` pass. |
| 10 | Mention resolution applied in HandleMessageCreate before publish | VERIFIED | Lines 373–377 of client.go: `if c.guildCache != nil { text = ResolveMentions(...) }` called before rawMsg map is built; `"text": text` used (not `msg.Content`). `TestHandleMessageCreate_MentionResolved` passes end-to-end. |

**Score:** 10/10 truths verified

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `services/discord-listener/gateway/types.go` | MessageDeleteData, MessageDeleteBulkData, GuildCreateData, DiscordChannel, DiscordRole, ChannelUpdateData, GuildRoleUpdateData, GuildRoleDeleteData; Mentions on MessageCreateData | VERIFIED | All 8 struct definitions present at lines 98–149. `Mentions []DiscordUser` on MessageCreateData at line 80. |
| `services/discord-listener/gateway/client.go` | GuildCache interface, HandleMessageDelete, HandleMessageDeleteBulk, HandleGuildCreate, HandleChannelUpdate, HandleChannelDelete, HandleGuildRoleUpdate, HandleGuildRoleDelete, ResolveMentions | VERIFIED | All methods present. GuildCache interface at lines 44–51. ResolveMentions exported function at line 542. `guildCache` field on GatewayClient at line 85. |
| `services/discord-listener/gateway/message_delete_test.go` | 5 tests for MESSAGE_DELETE dispatch | VERIFIED | 5 tests present and passing: UnknownChannel, HappyPath, RegistryError, Bulk_HappyPath, Bulk_UnknownChannel. |
| `services/discord-listener/gateway/guild_cache_test.go` | 6 tests for GuildCache population | VERIFIED | 6 tests present and passing: PopulatesChannelCache, PopulatesRoleCache, ChannelUpdate, ChannelDelete, GuildRoleUpdate, GuildRoleDelete. |
| `services/discord-listener/gateway/mention_test.go` | 9 tests for mention resolution | VERIFIED | 9 tests present and passing: 7 ResolveMentions unit tests + TestHandleMessageCreate_MentionResolved integration test + TestResolveMentions_MultipleInSameMessage. |
| `services/discord-listener/cmd/main.go` | redisGuildCache concrete type with discord:guild:channels: and discord:guild:roles: key prefixes; wired into NewGatewayClient | VERIFIED | `redisGuildCache` defined at lines 66–104 with correct key prefixes. Instantiated at line 142 and passed to `NewGatewayClient` at line 146. |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `Connect()` OpDispatch | `HandleMessageDelete` | `if payload.T == "MESSAGE_DELETE"` | WIRED | client.go lines 202–211 |
| `Connect()` OpDispatch | `HandleMessageDeleteBulk` | `if payload.T == "MESSAGE_DELETE_BULK"` | WIRED | client.go lines 213–222 |
| `HandleMessageDelete` | `publisher.Publish` | `c.publisher.Publish(ctx, deleteEvent)` with `event_type=message_deletion` | WIRED | client.go line 442 |
| `HandleMessageDelete` | `ChannelRegistry.GetOverlayForChannel` | same filter as HandleMessageCreate | WIRED | client.go line 414 |
| `Connect()` OpDispatch | `HandleGuildCreate` | `if payload.T == "GUILD_CREATE"` | WIRED | client.go lines 224–233 |
| `HandleGuildCreate` | `GuildCache.SetChannelName / SetRoleName` | iterate channels and roles arrays | WIRED | client.go lines 469/479 |
| `Connect()` OpDispatch | `HandleChannelUpdate` | `CHANNEL_UPDATE` or `CHANNEL_CREATE` | WIRED | client.go lines 235–244 |
| `Connect()` OpDispatch | `HandleChannelDelete` | `CHANNEL_DELETE` | WIRED | client.go lines 246–255 |
| `Connect()` OpDispatch | `HandleGuildRoleUpdate` | `GUILD_ROLE_UPDATE` or `GUILD_ROLE_CREATE` | WIRED | client.go lines 257–266 |
| `Connect()` OpDispatch | `HandleGuildRoleDelete` | `GUILD_ROLE_DELETE` | WIRED | client.go lines 268–277 |
| `HandleMessageCreate` | `ResolveMentions` | `if c.guildCache != nil { text = ResolveMentions(...) }` before rawMsg build | WIRED | client.go lines 373–377 |
| `HandleMessageCreate` | `GuildCache.GetChannelName / GetRoleName` | via ResolveMentions call with cache | WIRED | ResolveMentions lines 577/595 |
| `cmd/main.go` | `redisGuildCache` | `guildCache := &redisGuildCache{client: rdb}` passed to NewGatewayClient | WIRED | main.go lines 142/146 |
| `redisGuildCache` | Redis keys `discord:guild:channels:` / `discord:guild:roles:` | Set/Get/Del with those prefixes | WIRED | main.go lines 69/73/84/88/92/103; distinct from `discord:channels:` channel registry |

---

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| INBD-03 | 29-01-PLAN.md | Discord message deletions are propagated through the existing deletion pipeline | SATISFIED | `HandleMessageDelete` and `HandleMessageDeleteBulk` implemented; MESSAGE_DELETE and MESSAGE_DELETE_BULK wired in Connect(); 5 tests pass. |
| INBD-04 | 29-02-PLAN.md | Discord @user and #channel mentions are resolved to human-readable names in message text | SATISFIED | `ResolveMentions` function resolves all token types (<@ID>, <@!ID>, <#ID>, <@&ID>); called in HandleMessageCreate before publish; 9 tests pass. |

No orphaned requirements found. REQUIREMENTS.md marks both INBD-03 and INBD-04 as Complete for Phase 29.

---

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `cmd/main.go` | 152 | `TODO(Phase 31): gate on shard ownership via source-manager leader election` | Info | Forward-planning comment for Phase 31 shard ownership; does not affect Phase 29 goal. Pre-existing architectural note. |

No blocker or warning anti-patterns found. The single TODO is a forward-reference for future work, explicitly scoped to Phase 31.

---

### Human Verification Required

None. All phase 29 behaviors are structurally verifiable without UI or real-time testing:
- Deletion event field values are verified by unit tests that inspect the published payload maps.
- Mention resolution output is verified by unit tests asserting exact string outputs.
- Cache population is verified by unit tests asserting in-memory state after handler calls.
- End-to-end wiring (HandleMessageCreate calling ResolveMentions) is verified by the integration test `TestHandleMessageCreate_MentionResolved`.

---

### Test Run Summary

```
ok   github.com/caesar/all-chat/services/discord-listener/gateway   0.003s
?    github.com/caesar/all-chat/services/discord-listener/cmd        [no test files]
ok   github.com/caesar/all-chat/services/discord-listener/publisher  (cached)
```

29 total gateway tests, all passing. Build clean. `go vet` clean.

Verified commits (from summaries):
- `767e18b` — test(29-01): add failing tests for MESSAGE_DELETE handling
- `a2e8077` — feat(29-01): implement MESSAGE_DELETE and MESSAGE_DELETE_BULK dispatch with channel filter
- `232d73b` — test(29-02): add failing tests for GuildCache population and mention resolution
- `99e0776` — feat(29-02): implement GuildCache, GUILD_CREATE/CHANNEL/ROLE dispatch, mention resolution

All 4 commits confirmed in git log.

---

_Verified: 2026-03-16T08:30:00Z_
_Verifier: Claude (gsd-verifier)_
