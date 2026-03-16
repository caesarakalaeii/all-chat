---
gsd_state_version: 1.0
milestone: v1.5
milestone_name: Discord Listener
status: planning
stopped_at: Completed 31-load-balancing-01-PLAN.md
last_updated: "2026-03-16T09:24:16.607Z"
last_activity: 2026-03-15 — v1.5 roadmap created, 6 phases (27-32), 19 requirements mapped
progress:
  total_phases: 18
  completed_phases: 10
  total_plans: 41
  completed_plans: 40
  percent: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-15)

**Core value:** Streamers can aggregate chat from all platforms they stream to, with reliable message delivery even during high-traffic events through intelligent load balancing, auto-scaling, and unlimited YouTube chat access.
**Current focus:** v1.5 Discord Listener — Phase 27 (Auth and Bot Token Foundation)

## Current Position

Phase: 27 of 32 (Auth and Bot Token Foundation)
Plan: — (not yet planned)
Status: Ready to plan
Last activity: 2026-03-15 — v1.5 roadmap created, 6 phases (27-32), 19 requirements mapped

Progress: [░░░░░░░░░░] 0% (v1.5 — 0 plans complete)

## Performance Metrics

**Velocity (prior milestones):**
- v1.0: 11 plans (3 phases)
- v1.1: 21 plans (7 phases)
- v1.2: 21 plans (12 phases)
- v1.3: 20 plans (4 phases)

**By Milestone:**

| Milestone | Phases | Plans | Status |
|-----------|--------|-------|--------|
| v1.0 Message Deletion | 1-3 | 11 | Complete |
| v1.1 Load Balancing | 4-10 | 21 | Complete |
| v1.2 InnerTube Listener | 11-22 | 21 | Complete |
| v1.3 Frontend Redesign | 23-26 | 20 | Complete |
| v1.5 Discord Listener | 27-32 | TBD | Not started |

*Updated: 2026-03-15 after roadmap creation*
| Phase 27 P01 | 2 | 2 tasks | 3 files |
| Phase 27 P02 | 6 | 2 tasks | 8 files |
| Phase 27 P03 | 12 | 2 tasks | 6 files |
| Phase 27 P04 | 9 | 3 tasks | 3 files |
| Phase 28 P02 | 108s | 2 tasks | 3 files |
| Phase 28 P01 | 8m | 3 tasks | 8 files |
| Phase 29 P01 | 142s | 2 tasks | 3 files |
| Phase 29 P02 | 286s | 2 tasks | 7 files |
| Phase 30-outbound-relay P01 | 129 | 2 tasks | 7 files |
| Phase 30-outbound-relay P02 | 5 | 1 tasks | 3 files |
| Phase 31-load-balancing P03 | 2min | 2 tasks | 4 files |
| Phase 31-load-balancing P01 | 117 | 2 tasks | 3 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Key decisions relevant to v1.5:

- **Single discord-listener service**: Handles both inbound Gateway and outbound relay in two goroutine groups — loop prevention filter requires shared in-memory channel registry
- **Bot Token model**: Static `DISCORD_BOT_TOKEN` Kubernetes sealed-secret; NOT routed through token-refresh-service; OAuth2 callback captures guild_id only (no per-user token)
- **No new DB tables**: Discord sources use existing `overlay_chat_sources` with `config` JSONB for Discord-specific fields (`guild_id`, `inbound_channel_id`, `relay_channel_id`, `relay_enabled`)
- **Snowflake IDs as strings**: All Discord Snowflake IDs stored and transmitted as strings to avoid JS safe-integer truncation above 2^53
- **Single shard (num_shards=1)**: Correct at v1.5 scale (far below 2,500-guild per-shard limit); shard ownership via source-manager leader election
- [Phase 27]: guild_id stored as VARCHAR(30) not BIGINT — Discord Snowflake IDs exceed JS safe-integer range
- [Phase 27]: discord platform registered in overlay-manager validPlatforms to unblock Plan 03 and discord-listener
- [Phase 27]: SessionStore interface in gateway/client.go isolates Redis for unit testability
- [Phase 27]: WARN log on READY event reminds operator to enable MESSAGE_CONTENT privileged intent in Discord Developer Portal
- [Phase 27]: Port 8086 for discord-listener HTTP health server (avoids collision with existing services)
- [Phase 27]: ComputeMissingPermissions exported for testability — avoids HTTP mocking in permission bit logic tests
- [Phase 27]: GetUserInfo returns error stub — Discord bot auth has no user identity; handlers bypass this method
- [Phase 27]: stateStorer interface in discord.go (not _test.go) enables memStateStore injection in tests without _test.go visibility limitation
- [Phase 27]: discordAPIBase overridable string field on DiscordHandler allows httptest.Server injection without extracting HTTP client interface
- [Phase 27]: Discord routes conditionally registered — graceful degradation via WARN log when env vars absent, consistent with YouTube/Kick pattern
- [Phase 28]: firstNonEmpty helper reused from kick_normalizer.go (same package) — no duplication
- [Phase 28]: gateway.MessagePublisher uses interface{} payload to prevent circular import; publisherAdapter bridges via JSON re-marshal in cmd/main.go
- [Phase 28]: Pure Redis GET approach for ChannelRegistry — no in-memory set needed at v1.5 scale
- [Phase 28]: HandleMessageCreate exported for direct unit testing without exposing Connect()
- [Phase 29]: HandleMessageDeleteBulk delegates to HandleMessageDelete per ID — consistent channel filter, no duplicated registry lookup
- [Phase 29]: Deletion event message_id uses snowflake+':del' suffix to prevent Redis Stream key collision with create event
- [Phase 29]: capturePayloadPublisher defined separately in message_delete_test.go — deletion tests need payload inspection, create tests count calls only
- [Phase 29]: ResolveMentions exported as package-level function (not method) for unit testability without GatewayClient construction
- [Phase 29]: GuildCache nil-guard in HandleMessageCreate enables graceful degradation — mentions pass through unchanged if cache not wired
- [Phase 29]: discord:guild:channels: and discord:guild:roles: key prefixes distinct from discord:channels: channel registry to avoid collision
- [Phase 30-outbound-relay]: httpPoster.baseURL overridable for httptest.Server injection — no HTTP client interface extraction needed
- [Phase 30-outbound-relay]: HandleMessage exported on relay.Manager for synchronous unit test injection, avoiding Redis Pub/Sub mocking
- [Phase 30-outbound-relay]: doPost(isRetry bool) helper enforces single-retry contract — public Post always passes false, recursive retry passes true
- [Phase 30-outbound-relay]: relay.NewHTTPPoster takes logger as third param — actual signature differs from plan spec; called with all three args
- [Phase 30-outbound-relay]: Shutdown order: gwClient.Close() -> relayMgr.Stop() -> srv.Shutdown() ensures relay goroutines drain before HTTP server closes
- [Phase 31-load-balancing]: [Phase 31-03]: maxReplicas=3 for discord-listener HPA — single-shard model; extra pods provide fault tolerance standby only
- [Phase 31-load-balancing]: [Phase 31-03]: scaleUp type=Pods value=1 periodSeconds=30 — prevents Redis shard ownership lock race during scale-up
- [Phase 31-load-balancing]: [Phase 31-03]: discord-listener-secrets separate Secret for DISCORD_BOT_TOKEN — independent rotation from allchat-secrets
- [Phase 31-load-balancing]: [Phase 31]: Gateway RESUME protocol — RESUME/IDENTIFY branch in Connect() after OpHello; d=false clears Redis session keys, d=true preserves them

### Pending Todos

None yet.

### Blockers/Concerns

**Phase 27 (Research flags):**
- Gateway connection ownership model: Redis lock approach (`discord:gateway:shard:0:holder`) must be validated against source-manager coordinator API before writing startup connection gating logic
- MESSAGE_CONTENT privileged intent: Must be enabled in Discord Developer Portal before any integration testing — silent empty messages if missing

**Phase 30 (Research flags):**
- Discord REST rate limit bucket values: Verify `X-RateLimit-Bucket` and `X-RateLimit-Limit` headers against live bot before finalizing token bucket configuration

**Phase 31 (Research flags):**
- HPA metric selection: Define specific scaling metric (Gateway throughput vs. relay queue depth vs. guild count) after Prometheus surface is built in Phases 28-30

## Session Continuity

Last session: 2026-03-16T09:24:16.604Z
Stopped at: Completed 31-load-balancing-01-PLAN.md
Resume file: None

**Next action:** `/gsd:plan-phase 27` to plan Phase 27 (Auth and Bot Token Foundation)
