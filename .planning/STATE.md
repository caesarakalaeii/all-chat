---
gsd_state_version: 1.0
milestone: v1.6
milestone_name: Listener SDK
status: planning
stopped_at: Phase 35 context gathered
last_updated: "2026-03-17T20:56:56.330Z"
last_activity: 2026-03-17 — Roadmap created, phases 33-38 defined
progress:
  total_phases: 24
  completed_phases: 20
  total_plans: 71
  completed_plans: 71
  percent: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-17)

**Core value:** Streamers can aggregate chat from all platforms they stream to, with reliable message delivery even during high-traffic events through intelligent load balancing, auto-scaling, and unlimited YouTube chat access.
**Current focus:** v1.6 Listener SDK — Phase 33 ready to plan

## Current Position

Phase: 33 of 38 (Pre-Migration Cleanup)
Plan: — of — in current phase
Status: Ready to plan
Last activity: 2026-03-17 — Roadmap created, phases 33-38 defined

Progress: [░░░░░░░░░░] 0% (v1.6 — 0 plans complete)

## Performance Metrics

**Velocity (prior milestones):**
- v1.0: 11 plans (3 phases)
- v1.1: 21 plans (7 phases)
- v1.2: 21 plans (12 phases)
- v1.3: 20 plans (4 phases)
- v1.5: 16 plans (6 phases)

**By Milestone:**

| Milestone | Phases | Plans | Status |
|-----------|--------|-------|--------|
| v1.0 Message Deletion | 1-3 | 11 | Complete |
| v1.1 Load Balancing | 4-10 | 21 | Complete |
| v1.2 InnerTube Listener | 11-22 | 21 | Complete |
| v1.3 Frontend Redesign | 23-26 | 20 | Complete |
| v1.5 Discord Listener | 27-32 | 16 | Complete |
| v1.6 Listener SDK | 33-38 | TBD | Not started |
| Phase 33 P01 | 107 | 2 tasks | 3 files |
| Phase 33-pre-migration-cleanup P02 | 4 | 2 tasks | 5 files |
| Phase 34 P01 | 4m | 3 tasks | 8 files |
| Phase 34 P02 | 15m | 3 tasks | 8 files |
| Phase 34 P03 | 10 | 3 tasks | 3 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Key decisions relevant to v1.6:

- SDK lives in `shared/listener/` — four new files in existing shared module; no new Go modules or go.work required
- Two SDK archetypes: `ListenerBase` (assignment-based: Twitch, YouTube, twitch-eventsub) and `LeadershipListener` embeds ListenerBase (per-stream ownership: Kick, InnerTube, Discord)
- Pre-migration cleanup is a hard prerequisite — source ID suffix inconsistency and HandleMigrationEvent signature mismatch must be fixed before SDK code is written
- Migration order: twitch → kick → (innertube + discord in parallel) → (youtube + twitch-eventsub) — simplest first, one listener at a time
- tiktok-listener explicitly excluded — Node.js service cannot use the Go SDK
- `make build-all` CI target (not go.work) chosen for monorepo-wide compile verification — avoids go mod tidy side-effect risks
- [Phase 33]: Strip platform suffix at intake in cmd/main.go (not inside Manager) to keep channels.Manager interface simple; bare-UUID maps passed to NewManager and UpdateAssignedSourceIDs in both kick-listener and twitch-listener
- [Phase 33-pre-migration-cleanup]: HandleMigrationEvent returns nil unconditionally in both managers — error return is a forward-compatible slot for future SDK-defined fatal conditions
- [Phase 33-pre-migration-cleanup]: consumeMessages retains panic recovery defer alongside new error-logging — two separate safety nets for panic vs application error
- [Phase 34]: serviceName passed explicitly by each listener caller — no hostname auto-detection in NewCoordinatorClient
- [Phase 34]: ChannelManager interface defined in shared/listener with 7 methods; compile-time assertions deferred to Phase 35
- [Phase 34]: coordinatorClient is a private interface in base.go — enables mock injection without public API surface
- [Phase 34]: LeadershipListener uses sourcemanager.NewSigningTokenSource (15min TTL) — matches kick-listener production pattern
- [Phase 34]: mockChannelManager defined in test file — satisfies ChannelManager interface without adding public SDK surface

### Pending Todos

None yet.

### Blockers/Concerns

**Phase 33 (Pre-Migration Cleanup — research flags):**
- Source ID normalization strategy: Validate whether coordinator `/assignments` response can be normalized server-side vs. normalizing all listeners client-side before choosing the fix direction
- HandleMigrationEvent error return: Confirm `shared/coordination/migration_subscriber.go` handles the returned error gracefully (logs/ignores) before deploying the canonical signature

**Phase 34 (SDK Definition — research flags):**
- discordgo version: Run `go get github.com/bwmarrin/discordgo@latest` before wiring — expected v0.28.1 or higher; verify before pinning

**Phase 37 (Discord migration — research flag):**
- Integration test shard acquisition and release with SDK-backed LeadershipListener before deploying to confirm Discord Gateway RESUME protocol is unaffected

## Session Continuity

Last session: 2026-03-17T20:56:56.327Z
Stopped at: Phase 35 context gathered
Resume file: .planning/phases/35-migrate-twitch-listener/35-CONTEXT.md

**Next action:** `/gsd:plan-phase 33` to plan Phase 33 (Pre-Migration Cleanup)
