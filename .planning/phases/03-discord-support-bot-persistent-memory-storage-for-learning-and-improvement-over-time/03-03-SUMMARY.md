---
phase: 03-discord-support-bot-persistent-memory-storage-for-learning-and-improvement-over-time
plan: "03"
subsystem: infra
tags: [kubernetes, postgresql, cnpg, support-bot, memory, migration]

# Dependency graph
requires:
  - phase: 03-02
    provides: memory repository, agent STORE_MEMORY/UPDATE_MEMORY parsing, tag extraction, pg dependency

provides:
  - DATABASE_URL env var constructed from CNPG cluster credentials in support-bot K8s deployment
  - run-migrations init container with idempotent bot_memories schema migration
  - Support bot can connect to PostgreSQL in production cluster

affects:
  - support-bot deployment in allchat namespace
  - bot_memories table creation on pod start

# Tech tracking
tech-stack:
  added: []
  patterns:
    - K8s variable substitution for DATABASE_URL construction ($(VAR) syntax)
    - postgres:16-alpine init container for migration with ON_ERROR_STOP=0 for idempotency
    - DO $$ block to conditionally CREATE TYPE (PostgreSQL limitation: no IF NOT EXISTS for types)

key-files:
  created: []
  modified:
    - ../caesar-deployment/apps/workloads/all-chat/support-bot-deployment.yaml

key-decisions:
  - "DATABASE_URL constructed via K8s variable substitution from individual DATABASE_HOST/PORT/NAME/USER/PASSWORD vars — avoids hardcoding URL, matches allchat-secrets/database-password key used by other services"
  - "ON_ERROR_STOP=0 in migration init container — allows pod to start even if some idempotent statements fail (safe since CREATE TABLE/INDEX use IF NOT EXISTS)"
  - "DO $$ block for memory_type ENUM — PostgreSQL CREATE TYPE has no IF NOT EXISTS support, requires workaround via pg_type catalog check"
  - "run-migrations placed as FIRST init container — ensures schema exists before code cloning and slash command registration"

patterns-established:
  - "Pattern: postgres:16-alpine init container for schema migration in Node.js services that lack Go migration tooling"

requirements-completed:
  - MEM-09

# Metrics
duration: 1min
completed: 2026-03-26
---

# Phase 03 Plan 03: K8s Deployment — DATABASE_URL and Migration Init Container Summary

**Kubernetes support-bot deployment updated with DATABASE_URL env var and idempotent postgres:16-alpine init container that creates bot_memories table with GIN-indexed tags on every pod start**

## Performance

- **Duration:** ~1 min
- **Started:** 2026-03-26T15:51:15Z
- **Completed:** 2026-03-26T15:52:15Z
- **Tasks:** 1 of 2 (Task 2 is a checkpoint awaiting human verification)
- **Files modified:** 1

## Accomplishments

- Added `run-migrations` init container (postgres:16-alpine) as first init container in support-bot deployment
- Migration SQL is fully idempotent: DO $$ block for memory_type ENUM, IF NOT EXISTS for table and indexes
- Added DATABASE_URL env var to main container using K8s variable substitution pattern
- DATABASE_PASSWORD sourced from `allchat-secrets/database-password` — same secret key used by all other allchat services
- All 92 support-bot unit tests pass

## Task Commits

Each task was committed atomically:

1. **Task 1: Add DATABASE_URL env var and migration init container** - `34f6269` (feat) — committed in caesar-deployment repo

## Files Created/Modified

- `/home/moersener/Hobby/caesar-deployment/apps/workloads/all-chat/support-bot-deployment.yaml` - Added run-migrations init container and DATABASE_URL env vars to support-bot deployment

## Decisions Made

- DATABASE_URL constructed via K8s variable substitution (`$(DATABASE_USER):$(DATABASE_PASSWORD)@$(DATABASE_HOST):$(DATABASE_PORT)/$(DATABASE_NAME)`) — avoids hardcoding credentials and leverages existing secret reference pattern
- ON_ERROR_STOP=0 in migration init container — allows graceful pod start when statements are repeated on restarts (CREATE TABLE IF NOT EXISTS and CREATE INDEX IF NOT EXISTS succeed silently)
- DO $$ block for ENUM type creation — PostgreSQL lacks `CREATE TYPE IF NOT EXISTS`, requires querying pg_type catalog as workaround
- run-migrations placed first among init containers — ensures schema is ready before any code runs

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required. Deployment will apply on next `kubectl apply` or Keel image rollout.

## Next Phase Readiness

- Complete memory layer is implemented across all three plans in Phase 03:
  1. Plan 01: PostgreSQL schema migration (042_support_bot_memories.sql) + MemoryRepository class
  2. Plan 02: STORE_MEMORY/UPDATE_MEMORY marker parsing, memory injection into Claude prompt, tag extraction
  3. Plan 03: K8s deployment with DATABASE_URL and migration init container (this plan)
- Task 2 checkpoint: human review of complete memory implementation before declaring phase done
- After checkpoint approval, bot is ready to deploy and will store/retrieve memories in production

---
*Phase: 03-discord-support-bot-persistent-memory-storage-for-learning-and-improvement-over-time*
*Completed: 2026-03-26*
