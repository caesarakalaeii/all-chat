---
phase: 27-auth-and-bot-token-foundation
plan: 02
subsystem: infra
tags: [discord, websocket, gorilla-websocket, redis, gin, zap, gateway, heartbeat]

# Dependency graph
requires:
  - phase: 27-auth-and-bot-token-foundation
    provides: "Plan 01 — discord platform registered in overlay-manager, discord_guilds migration applied"
provides:
  - "discord-listener Go service scaffold with working Gateway WebSocket connection"
  - "gateway/types.go — op codes, intent constants (RequiredIntents=33281), Redis key schema"
  - "gateway/client.go — BuildIdentifyPayload, HandleReady, GatewayClient with HELLO/heartbeat/READY loop"
  - "handlers/health.go — /health/live and /health/ready Gin handlers with Redis ping"
  - "cmd/main.go — service entry point with graceful shutdown, reconnect loop"
  - "Dockerfile — Alpine-based single binary image on port 8086"
  - "4 unit tests covering intent bitmask, op codes, IDENTIFY payload, READY Redis writes"
affects: [28-discord-gateway-message-dispatch, 31-discord-scaling-shard-ownership]

# Tech tracking
tech-stack:
  added:
    - "github.com/gorilla/websocket v1.5.3 — Discord Gateway WebSocket transport"
    - "github.com/gin-gonic/gin v1.12.0 — HTTP health endpoints"
    - "github.com/redis/go-redis/v9 v9.18.0 — session state persistence"
    - "go.uber.org/zap v1.27.0 — structured logging"
    - "github.com/stretchr/testify v1.11.1 — test assertions"
  patterns:
    - "SessionStore interface for testable Redis abstraction (client.go)"
    - "MockRedis in test file for unit testing without live Redis"
    - "TDD RED/GREEN commit pattern: test scaffold first, implementation second"
    - "gateway goroutine with 5s reconnect backoff in main.go"
    - "Standard health handler pattern matching kick-listener/handlers"

key-files:
  created:
    - services/discord-listener/go.mod
    - services/discord-listener/go.sum
    - services/discord-listener/gateway/types.go
    - services/discord-listener/gateway/client.go
    - services/discord-listener/gateway/client_test.go
    - services/discord-listener/handlers/health.go
    - services/discord-listener/cmd/main.go
    - services/discord-listener/Dockerfile
  modified: []

key-decisions:
  - "SessionStore interface in client.go isolates Redis dependency for unit testability — MockRedis in test satisfies it without live Redis"
  - "RequiredIntents = 33281 hardcoded as named constant — GUILDS(1)|GUILD_MESSAGES(512)|MESSAGE_CONTENT(32768)"
  - "WARN log on READY event reminds operator to enable MESSAGE_CONTENT privileged intent in Developer Portal"
  - "Port 8086 chosen for discord-listener (avoids collision with other services)"
  - "TODO(Phase 31) comment gates shard ownership on source-manager leader election Redis lock"

patterns-established:
  - "SessionStore interface: Set/Get abstraction used in gateway/client.go, implemented by redisSessionStore in main.go and MockRedis in tests"
  - "Gateway reconnect loop: infinite for loop with ctx.Done check and 5s backoff for production resilience"
  - "Privileged intent operator warning: WARN log on READY event documents operator action required for MESSAGE_CONTENT"

requirements-completed: [AUTH-01, AUTH-02, AUTH-03, AUTH-04]

# Metrics
duration: 6min
completed: 2026-03-15
---

# Phase 27 Plan 02: Discord Listener Gateway WebSocket Foundation Summary

**Go service `discord-listener` with Discord Gateway WebSocket connection using intents bitmask 33281, heartbeat loop, READY handler persisting session to Redis, and Gin health endpoints**

## Performance

- **Duration:** 6 min
- **Started:** 2026-03-15T21:00:33Z
- **Completed:** 2026-03-15T21:06:18Z
- **Tasks:** 2 (TDD: RED + GREEN)
- **Files modified:** 8 created

## Accomplishments
- New Go module `services/discord-listener` with all dependencies wired (gorilla/websocket, gin, go-redis, zap)
- Gateway WebSocket client with correct IDENTIFY payload (intents=33281), heartbeat with jitter, and READY persistence to Redis
- SessionStore interface enabling unit test isolation — MockRedis satisfies it without live Redis
- 4 unit tests covering IntentBitmask, OpCodes, IdentifyPayload construction, and ReadyHandler Redis writes
- Gin HTTP server with /health/live and /health/ready (Redis ping), graceful 25s shutdown, port 8086

## Task Commits

Each task was committed atomically:

1. **Task 1: Write Gateway test scaffold (Wave 0)** - `b988ad1` (test)
2. **Task 2: Implement Gateway client and service entry point** - `2b461a8` (feat)

_Note: TDD tasks have two commits — RED (failing tests) then GREEN (implementation)_

## Files Created/Modified
- `services/discord-listener/go.mod` — Go module declaration with dependency versions
- `services/discord-listener/go.sum` — Dependency checksums
- `services/discord-listener/gateway/types.go` — Op codes, intent bitmasks (RequiredIntents=33281), Redis key schema, payload structs
- `services/discord-listener/gateway/client.go` — BuildIdentifyPayload, HandleReady, GatewayClient with full connect/heartbeat/READY loop
- `services/discord-listener/gateway/client_test.go` — 4 unit tests with MockRedis, no live dependencies required
- `services/discord-listener/handlers/health.go` — /health/live and /health/ready Gin handlers
- `services/discord-listener/cmd/main.go` — Service entry point, redisSessionStore adapter, reconnect goroutine, graceful shutdown
- `services/discord-listener/Dockerfile` — Multi-stage Alpine build, port 8086

## Decisions Made
- `SessionStore` interface isolates Redis from unit tests — `redisSessionStore` in main.go wraps `*redis.Client`, `MockRedis` in tests satisfies the same interface
- WARN log on READY event documents that `MESSAGE_CONTENT` privileged intent must be enabled in Discord Developer Portal — silent empty messages result if missing
- Port 8086 for HTTP health server to avoid collision with existing services
- `TODO(Phase 31)` comment in main.go gates gateway goroutine on shard ownership via source-manager leader election Redis lock key `discord:gateway:shard:0:holder`

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Added missing `fmt` import to client_test.go**
- **Found during:** Task 1 (Writing test file)
- **Issue:** Plan's test scaffold used `fmt.Errorf` in MockRedis.Get without importing `"fmt"`, which would cause compile failure
- **Fix:** Added `"fmt"` to test file import block
- **Files modified:** services/discord-listener/gateway/client_test.go
- **Verification:** `go test ./gateway/...` compiles and passes
- **Committed in:** b988ad1 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 - bug in plan's test code)
**Impact on plan:** Minor fix to missing import in plan's test scaffold. No scope creep.

## Issues Encountered
- Missing go.sum entry for testify transitive dependencies — resolved with `go get github.com/stretchr/testify/assert@v1.11.1` before the RED test run

## User Setup Required
None - no external service configuration required for this plan. DISCORD_BOT_TOKEN is required at runtime but is a Kubernetes sealed-secret per architectural decision.

## Next Phase Readiness
- Gateway connection foundation is complete and tested — Phase 28 can add MESSAGE_CREATE dispatch processing
- SessionStore interface is extensible for Phase 31 shard ownership gating
- Health endpoints ready for Kubernetes probes from day one
- Reminder: `MESSAGE_CONTENT` privileged intent must be enabled in Discord Developer Portal before Phase 28 integration testing

## Self-Check: PASSED

All 8 files exist on disk. Both commits (b988ad1, 2b461a8) present in git log.

---
*Phase: 27-auth-and-bot-token-foundation*
*Completed: 2026-03-15*
