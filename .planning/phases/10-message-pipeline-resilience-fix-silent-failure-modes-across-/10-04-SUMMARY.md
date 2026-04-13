---
phase: 10-message-pipeline-resilience-fix-silent-failure-modes-across-
plan: "04"
subsystem: source-manager/election
tags: [redis, leader-election, lua, sorted-set, atomicity, performance]
dependency_graph:
  requires: []
  provides: [atomic-leadership-renewal, o1-peer-count]
  affects: [source-manager]
tech_stack:
  added: []
  patterns: [lua-script-atomicity, redis-sorted-set-ttl-by-score]
key_files:
  created: []
  modified:
    - services/source-manager/election/leader.go
    - services/source-manager/election/leader_test.go
decisions:
  - "renewScript defined at package level matching existing ReleaseLeadership pattern — consistent Lua script style throughout election package"
  - "int(m.lockTTL.Seconds()) produces correct integer for Redis EXPIRE command — lockTTL is time.Duration, Seconds() returns float64, int() truncates cleanly for whole-second TTLs"
  - "Sorted set key is peers:{platform} (not peer:{platform}:{callerID}) — single key per platform enables O(1) ZCARD instead of O(N) SCAN"
  - "Score = Unix expiry timestamp — ZRemRangeByScore('-inf', now) removes expired members without needing Redis key-level TTL per member"
  - "peerKey helper removed — no longer needed since individual per-callerID keys are replaced by single sorted set"
  - "Key TTL set to 2x PeerTTL — prevents orphaned sorted sets when all pods stop registering"
metrics:
  duration: "~8 minutes"
  completed_date: "2026-04-08"
  tasks_completed: 2
  files_modified: 2
---

# Phase 10 Plan 04: Source-Manager Leadership Renewal and Peer Registration Fixes Summary

Atomic Lua-based RenewLeadership replacing TOCTOU GET+Expire, plus O(1) sorted-set RegisterPeer replacing O(N) SCAN.

## What Was Built

### Task 1: Atomic RenewLeadership via Lua script (F-11)

**Problem:** `RenewLeadership` used a non-atomic GET followed by EXPIRE. In the window between the two Redis calls, another pod could acquire the key after the GET succeeds but before the EXPIRE fires, creating a dual-leadership window.

**Fix:** Added a package-level `renewScript` variable using `redis.NewScript`, matching the existing `ReleaseLeadership` Lua pattern. The script atomically checks ownership and renews TTL in a single Redis operation:

```lua
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("expire", KEYS[1], ARGV[2])
else
    return 0
end
```

`RenewLeadership` now calls `renewScript.Run(ctx, m.client, []string{key}, callerID, int(m.lockTTL.Seconds()))` — no separate GET or EXPIRE calls.

**Commit:** `788941f`

### Task 2: RegisterPeer sorted set optimization (F-12)

**Problem:** `RegisterPeer` used `m.client.Set(key, "1", PeerTTL)` for each peer followed by `m.client.Scan(0, "peer:platform:*", 0)` to count all peers. Under high pod counts, SCAN blocks Redis and degrades performance linearly.

**Fix:** Replaced with a Redis sorted set at key `peers:{platform}` where:
- Each member = callerID
- Score = Unix expiry timestamp (`time.Now().Add(PeerTTL).Unix()`)
- `ZAdd` adds/updates a peer's expiry in O(log N)
- `ZRemRangeByScore("-inf", now)` removes expired members before counting
- `ZCard` returns active peer count in O(1)

The old individual `peer:platform:callerID` string keys and `peerKey` helper are removed. Old keys expire naturally from Redis.

**Commit:** `6ed6965`

## Tests Added

| Test | Purpose |
|------|---------|
| `TestRenewLeadership_WithExplicitCallerID` | Verifies explicit callerID matched/rejected correctly |
| `TestRenewLeadership_Atomic_NoTOCTOURace` | Verifies renewal works for owner, rejected for non-owner |
| `TestRegisterPeer_UsesSortedSet` | Verifies sorted set key exists, SCAN-style keys absent |
| `TestRegisterPeer_UpdatesExistingPeerExpiry` | Verifies re-registration updates score (not duplicates) |
| `TestRegisterPeer_Expiry` (updated) | Uses score-based expiry simulation for sorted set model |

## Verification

```
ok  github.com/caesar/all-chat/services/source-manager/election   0.012s
BUILD OK
```

- `grep "renewScript.Run("` confirms atomic renewal in place
- `grep "m.client.ZAdd"` / `ZRemRangeByScore` / `ZCard` confirm sorted set implementation
- No `m.client.Scan(` in `RegisterPeer` (remaining Scan is in `GetAllLeadership`, unchanged)

## Deviations from Plan

None — plan executed exactly as written. The `peerKey` helper was removed as a natural cleanup (Rule 1 — no longer needed with sorted set approach, keeping it would be dead code).

## Threat Surface Scan

No new network endpoints, auth paths, file access patterns, or schema changes introduced. Changes are internal to the Redis key structure within source-manager — both threat mitigations (T-10-11, T-10-12) from the plan's threat model are now resolved.

## Self-Check

### Files Exist

- `services/source-manager/election/leader.go` — modified
- `services/source-manager/election/leader_test.go` — modified
- `.planning/phases/10-message-pipeline-resilience-fix-silent-failure-modes-across-/10-04-SUMMARY.md` — created

### Commits Exist

- `788941f` — feat(10-04): atomic RenewLeadership via Lua script (F-11)
- `6ed6965` — feat(10-04): RegisterPeer sorted set optimization (F-12)

## Self-Check: PASSED
