---
phase: 01-foundation-twitch
plan: 01
subsystem: message-processor
tags:
  - infrastructure
  - deletion
  - redis
  - id-mapping
dependency_graph:
  requires: []
  provides:
    - Message ID Registry infrastructure
    - Platform ID to UUID mapping capability
  affects:
    - services/message-processor
tech_stack:
  added:
    - github.com/alicebob/miniredis/v2 (testing)
  patterns:
    - Redis hash for O(1) lookups
    - Pipeline atomicity (HSET + EXPIRE)
    - Unidirectional mapping architecture
key_files:
  created:
    - services/message-processor/registry/registry.go
    - services/message-processor/registry/registry_test.go
  modified:
    - services/message-processor/cmd/main.go
    - services/message-processor/go.mod
    - services/message-processor/go.sum
decisions:
  - decision: "Use unidirectional mapping (platform ID → UUID only)"
    rationale: "Deletion events always provide platform ID, not internal UUID. Bidirectional mapping would add unnecessary complexity and memory overhead."
    alternatives: ["Bidirectional mapping", "Separate lookup tables"]
  - decision: "1-hour TTL per channel"
    rationale: "Balances memory usage with deletion event latency. Most deletions occur within minutes of posting. TTL refreshed on each message add to handle long-lived channels."
    alternatives: ["24-hour TTL", "No expiration"]
  - decision: "Store timestamp with UUID for debugging"
    rationale: "Value format {uuid}|{timestamp} enables debugging without additional Redis operations. Minimal overhead (8 bytes per message)."
    alternatives: ["UUID only", "Separate timestamp field"]
metrics:
  duration: 3
  completed_date: 2026-02-18
  task_count: 2
  test_coverage: 87.5
---

# Phase 01 Plan 01: Message ID Registry Infrastructure Summary

**One-liner:** Created Redis-backed Message ID Registry with O(1) lookups, 1-hour TTL, and unidirectional platform ID → UUID mapping for deletion event matching.

## What Was Built

**Core Infrastructure:**
- `MessageIDRegistry` interface defining Add/Lookup/Remove operations
- `RedisRegistry` implementation using Redis hashes for storage
- Comprehensive unit test suite using miniredis (87.5% coverage)
- Integration into message-processor startup sequence

**Key Characteristics:**
- **Storage model:** Redis hash per platform/channel: `msgid:registry:{platform}:{channelID}`
- **Value format:** `{internalUUID}|{timestamp}` for debugging visibility
- **Atomicity:** Pipeline combines HSET + EXPIRE in single transaction
- **TTL strategy:** 1-hour expiration, refreshed on every message add
- **Performance:** O(1) lookup via Redis HGET operation

## Architecture Decisions

**Unidirectional Mapping Choice:**
Platform deletion events always provide the platform's native message ID, never our internal UUID. Implementing bidirectional mapping would require:
- 2x memory usage (platform ID → UUID + UUID → platform ID)
- 2x write operations per message
- Additional code complexity

Since reverse lookups (UUID → platform ID) are never needed, unidirectional mapping is optimal.

**TTL Strategy:**
- **1-hour TTL** balances memory usage with deletion latency expectations
- Most message deletions occur within seconds/minutes of posting
- TTL **refreshes on every add** to handle long-lived active channels
- After 1 hour of channel inactivity, mappings expire naturally

**Per-Channel Hash Keys:**
Using separate hash keys per platform/channel provides:
- Channel isolation (no key conflicts)
- Efficient expiration (entire channel expires atomically)
- Scalability (Redis can shard different channels)

## Requirements Coverage

| Requirement | Status | Implementation |
|-------------|--------|----------------|
| MSGID-01: Platform IDs preserved | ✅ Complete | Registry stores platform IDs as hash fields |
| MSGID-02: Redis-based registry | ✅ Complete | RedisRegistry with hash structure |
| MSGID-03: 1-hour TTL configured | ✅ Complete | NewRedisRegistry accepts TTL, set to 1 hour |
| MSGID-04: O(1) lookup performance | ✅ Complete | Redis HGET provides O(1) lookups |
| MSGID-05: Infrastructure ready | ✅ Complete | Registry initialized in main.go, ready for use |

## Test Coverage

**87.5% coverage across 13 test cases:**

**Core Operations:**
- ✅ Add and lookup message IDs
- ✅ Lookup non-existent message returns ErrMessageNotFound
- ✅ Remove message ID mapping

**TTL Management:**
- ✅ TTL set correctly on add (3600 seconds)
- ✅ TTL refreshes on subsequent adds
- ✅ Pipeline atomicity (HSET + EXPIRE both succeed)

**Isolation:**
- ✅ Multiple channels don't conflict (different hash keys)
- ✅ Same platform message ID works in different channels
- ✅ Multiple platforms don't conflict

**Error Handling:**
- ✅ Empty parameter validation for Add/Lookup/Remove
- ✅ Proper error types returned

**Infrastructure:**
- ✅ Redis key format: `msgid:registry:{platform}:{channelID}`

## Tasks Completed

### Task 1: Create Message ID Registry Package
**Commit:** b810ad4
**Files:** registry.go (115 lines), registry_test.go (314 lines)

Created foundational registry package with:
- `MessageIDRegistry` interface (3 methods: Add, Lookup, Remove)
- `RedisRegistry` implementation with pipeline atomicity
- Custom error type `ErrMessageNotFound` for clean error handling
- Unit tests using miniredis for in-memory testing

**Key implementation details:**
- Value stored as `{uuid}|{timestamp}` for debugging
- Lookup extracts UUID by splitting on pipe character
- Pipeline ensures HSET + EXPIRE execute atomically
- Parameter validation prevents empty values

### Task 2: Integrate Registry into Message Processor
**Commit:** 0be3de5
**Files:** cmd/main.go (8 lines added)

Added registry initialization to message-processor startup:
- Import registry package
- Initialize `RedisRegistry` with 1-hour TTL
- Log initialization with TTL duration
- Add TODO comment for Plan 01-02 usage

**Integration point:** After Redis client connection, before component initialization (line 120).

## Deviations from Plan

None - plan executed exactly as written. All tasks completed without modifications, no bugs discovered, no additional features required.

## What Comes Next

**Plan 01-02:** Store platform message IDs in registry during message processing. The `msgIDRegistry` is initialized but not yet used. Next plan will:
1. Pass registry to message handler
2. Call `Add()` for each normalized message
3. Store mapping: platform message ID → internal UUID
4. Enable deletion event matching in later plans

**Dependencies ready:**
- ✅ Registry infrastructure exists
- ✅ Interface defined and tested
- ✅ Redis client available
- ✅ TTL configured

**Remaining deletion work (Phase 1):**
- Store platform IDs during normalization (Plan 01-02)
- Implement deletion event handlers (Plan 01-03)
- Frontend deletion protocol (Plan 01-04, 01-05)

## Performance Characteristics

**Memory per message:** ~50 bytes
- Redis hash field: platform message ID (~20 chars)
- Value: UUID (36 chars) + pipe + timestamp (10 digits)

**Operations per message:**
- 1 HSET (store mapping)
- 1 EXPIRE (refresh TTL)
- Pipelined into single roundtrip

**Lookup latency:** ~1ms (Redis HGET)

**Expiration:** Automatic after 1 hour of channel inactivity

## Self-Check

Verifying implementation claims:

**Files exist:**
```
✓ services/message-processor/registry/registry.go (115 lines)
✓ services/message-processor/registry/registry_test.go (314 lines)
✓ services/message-processor/cmd/main.go (modified)
```

**Tests pass:**
```
✓ go test ./registry/... passes all 13 tests
✓ Test coverage: 87.5%
```

**Build succeeds:**
```
✓ go build ./cmd/ compiles without errors
```

**Commits exist:**
```
✓ b810ad4: feat(01-01): create Message ID Registry package
✓ 0be3de5: feat(01-01): integrate Message ID Registry into message-processor
```

**Registry initialization logged:**
```
✓ Line 122: log.Info("Initialized Message ID Registry", zap.Duration("ttl", 1*time.Hour))
```

## Self-Check: PASSED

All files exist, tests pass, build succeeds, commits verified, and logging confirms initialization.
