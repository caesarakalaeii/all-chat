---
gsd_state_version: 1.0
milestone: v1.5
milestone_name: Discord Listener
status: planning
stopped_at: Completed 27-04-PLAN.md
last_updated: "2026-03-15T21:32:33.839Z"
last_activity: 2026-03-15 — v1.5 roadmap created, 6 phases (27-32), 19 requirements mapped
progress:
  total_phases: 18
  completed_phases: 7
  total_plans: 32
  completed_plans: 32
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

Last session: 2026-03-15T21:26:35.963Z
Stopped at: Completed 27-04-PLAN.md
Resume file: None

**Next action:** `/gsd:plan-phase 27` to plan Phase 27 (Auth and Bot Token Foundation)
