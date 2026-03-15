---
gsd_state_version: 1.0
milestone: v1.5
milestone_name: Discord Listener
status: ready_to_plan
stopped_at: Roadmap created 2026-03-15 — 6 phases (27-32), 19/19 requirements mapped, ready to plan Phase 27
last_updated: "2026-03-15T00:00:00.000Z"
last_activity: 2026-03-15 — v1.5 roadmap created
progress:
  total_phases: 6
  completed_phases: 0
  total_plans: 0
  completed_plans: 0
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

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Key decisions relevant to v1.5:

- **Single discord-listener service**: Handles both inbound Gateway and outbound relay in two goroutine groups — loop prevention filter requires shared in-memory channel registry
- **Bot Token model**: Static `DISCORD_BOT_TOKEN` Kubernetes sealed-secret; NOT routed through token-refresh-service; OAuth2 callback captures guild_id only (no per-user token)
- **No new DB tables**: Discord sources use existing `overlay_chat_sources` with `config` JSONB for Discord-specific fields (`guild_id`, `inbound_channel_id`, `relay_channel_id`, `relay_enabled`)
- **Snowflake IDs as strings**: All Discord Snowflake IDs stored and transmitted as strings to avoid JS safe-integer truncation above 2^53
- **Single shard (num_shards=1)**: Correct at v1.5 scale (far below 2,500-guild per-shard limit); shard ownership via source-manager leader election

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

Last session: 2026-03-15
Stopped at: Roadmap created — ROADMAP.md, STATE.md written, REQUIREMENTS.md traceability updated
Resume file: None

**Next action:** `/gsd:plan-phase 27` to plan Phase 27 (Auth and Bot Token Foundation)
