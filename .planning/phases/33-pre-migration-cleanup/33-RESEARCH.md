# Phase 33: Pre-Migration Cleanup - Research

**Researched:** 2026-03-17
**Domain:** Go refactoring — source ID normalization and function signature canonicalization across twitch-listener and kick-listener
**Confidence:** HIGH

## Summary

Phase 33 is a surgical cleanup of two concrete, code-level inconsistencies between the existing twitch-listener and kick-listener services. Both issues were discovered during the SDK design phase (v1.6 planning) and must be resolved before any SDK code is written, because the SDK's `ChannelManager` interface assumes a uniform contract.

The first inconsistency is in how each listener strips (or does not strip) the `:platform` suffix from coordinator `Assignment.SourceID` values during assignment refresh. Twitch-listener strips the suffix in its refresh loop using `strings.LastIndexByte`; kick-listener does not strip at all. This means the `assignedSourceIDs` map inside each manager is keyed differently. The fix is to normalize both to raw UUIDs (stripped form) at the point they are inserted into `assignedSourceIDs`.

The second inconsistency is the return type of `HandleMigrationEvent`. Both twitch-listener and kick-listener currently declare `func (m *Manager) HandleMigrationEvent(event *coordination.MigrationEvent)` with no return value. The target canonical signature is `func(event *coordination.MigrationEvent) error`. The `shared/coordination/MigrationSubscriber` currently stores the handler as `func(*MigrationEvent)` and calls it without capturing a return value. Changing the signature requires updating: the two channel managers, the `MigrationSubscriber` handler field type and call site, and all wiring in the two `cmd/main.go` files.

**Primary recommendation:** Fix source ID stripping in kick-listener's assignment refresh loop (mirror what twitch-listener already does); then change `HandleMigrationEvent` to return `error` in both managers and update `MigrationSubscriber` to log or ignore the returned error.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| PREP-01 | Source ID suffix handling normalized — Twitch and Kick agree on whether `:platform` suffix is present or stripped before SDK extraction begins | Source ID divergence fully documented in §Source ID Problem; fix path is clear |
| PREP-02 | `HandleMigrationEvent` signature canonicalized to `func(event *coordination.MigrationEvent) error` in both Twitch and Kick channel managers, deployed before SDK extraction | All call sites identified; migration subscriber update path documented in §HandleMigrationEvent Problem |
</phase_requirements>

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/caesar/all-chat/shared/coordination` | local | `MigrationEvent`, `MigrationSubscriber`, `CoordinatorClient` | Already in use in both listeners |
| `go.uber.org/zap` | existing | Error logging inside updated migration subscriber | Already project standard |
| `strings` stdlib | stdlib | `strings.LastIndexByte` for suffix stripping | Already used in twitch-listener |

No new dependencies are required for this phase.

---

## Architecture Patterns

### Recommended Project Structure

No structural changes. All edits are in-place modifications to existing files:

```
services/twitch-listener/
├── channels/manager.go        # Change HandleMigrationEvent signature
└── cmd/main.go                # No change needed (handler type inferred)

services/kick-listener/
├── channels/manager.go        # Change HandleMigrationEvent signature
└── cmd/main.go                # Add suffix stripping in assignment refresh loop

shared/coordination/
└── migration_subscriber.go    # Change handler field type + call site
```

### Pattern 1: Source ID Normalization (Strip-at-intake)

**What:** Strip the `:platform` suffix from `Assignment.SourceID` immediately when building `assignedSourceIDs`, not at any later lookup site. This is the "strip-at-intake" pattern: the map always contains bare UUIDs.

**When to use:** Every listener that processes `Assignment` slices from `QueryAssignments`.

**Current state — twitch-listener `cmd/main.go` lines 261-269 (already correct):**
```go
// Existing code in twitch-listener assignment refresh — already strips suffix
newAssignedIDs := make(map[string]bool)
for _, a := range newAssignments {
    sourceID := a.SourceID
    // Strip platform suffix if present (e.g., "abc123:twitch" → "abc123")
    if colonIdx := strings.LastIndexByte(sourceID, ':'); colonIdx != -1 {
        sourceID = sourceID[:colonIdx]
    }
    newAssignedIDs[sourceID] = true
}
```

**Missing — kick-listener `cmd/main.go` lines 260-263 (does NOT strip):**
```go
// Existing code in kick-listener assignment refresh — MISSING suffix stripping
newAssignedIDs := make(map[string]bool)
for _, a := range newAssignments {
    newAssignedIDs[a.SourceID] = true   // BUG: keeps ":kick" suffix
}
```

**Fix for kick-listener (mirror twitch pattern):**
```go
newAssignedIDs := make(map[string]bool)
for _, a := range newAssignments {
    sourceID := a.SourceID
    if colonIdx := strings.LastIndexByte(sourceID, ':'); colonIdx != -1 {
        sourceID = sourceID[:colonIdx]
    }
    newAssignedIDs[sourceID] = true
}
```

Also note: twitch-listener's **initial** assignment extraction at startup (lines 149-152) does NOT strip suffixes — only the refresh loop does. Both the initial extraction and the refresh extraction must be consistent. The fix must normalize both paths in each listener.

### Pattern 2: Error-returning Handler via Updated MigrationSubscriber

**What:** Change `MigrationSubscriber.handler` field from `func(*MigrationEvent)` to `func(*MigrationEvent) error`, update the call site to capture and log the error, and update both channel managers to return `error` from `HandleMigrationEvent`.

**Current `migration_subscriber.go` handler field and call site:**
```go
// shared/coordination/migration_subscriber.go — current (no error return)
type MigrationSubscriber struct {
    redisClient *redis.Client
    logger      *zap.Logger
    handler     func(*MigrationEvent)       // Current: no error return
}

func NewMigrationSubscriber(redisClient *redis.Client, handler func(*MigrationEvent), logger *zap.Logger) *MigrationSubscriber {
    ...
}

// In consumeMessages — current call site (no error capture)
func() {
    defer func() {
        if r := recover(); r != nil {
            s.logger.Error("Migration event handler panicked", ...)
        }
    }()
    s.handler(&event)   // No error return captured
}()
```

**Target state — `migration_subscriber.go`:**
```go
type MigrationSubscriber struct {
    redisClient *redis.Client
    logger      *zap.Logger
    handler     func(*MigrationEvent) error  // Updated: returns error
}

func NewMigrationSubscriber(redisClient *redis.Client, handler func(*MigrationEvent) error, logger *zap.Logger) *MigrationSubscriber {
    ...
}

// In consumeMessages — updated call site
func() {
    defer func() {
        if r := recover(); r != nil {
            s.logger.Error("Migration event handler panicked", ...)
        }
    }()
    if err := s.handler(&event); err != nil {
        s.logger.Error("Migration event handler returned error",
            zap.String("migration_id", event.MigrationID),
            zap.Error(err),
        )
        // Do not return — continue processing subsequent events
    }
}()
```

**Target state — both channel managers:**
```go
// twitch-listener channels/manager.go and kick-listener channels/manager.go
func (m *Manager) HandleMigrationEvent(event *coordination.MigrationEvent) error {
    // ... existing body unchanged ...
    return nil
}
```

**Both `cmd/main.go` wiring sites are already correct** once the manager signatures change — `channelMgr.HandleMigrationEvent` is passed directly as the handler, and Go infers the function type from the method value. No changes needed in the two `main.go` files after the subscriber and manager are updated.

### Anti-Patterns to Avoid

- **Stripping suffix in the manager's filter loop** instead of at intake: The manager receives an `assignedSourceIDs map[string]bool` and should not need to know the coordinator wire format. Strip at intake in `cmd/main.go`.
- **Returning errors from `HandleMigrationEvent` that abort the event loop**: The migration subscriber's goroutine must continue processing events even when one handler invocation fails. Log the error and continue.
- **Adding suffix stripping only to the refresh loop but not the initial extraction**: Both the startup path (`QueryAssignments` result → `assignedSourceIDs`) and the refresh path must be consistent.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| UUID validation after stripping | Custom UUID regex | `strings.LastIndexByte` + existing pattern | The coordinator is the source of truth; if there is no colon, the string is already a bare UUID |
| Error accumulation across migration events | Error channel or slice | Log-and-continue at call site | Each migration event is independent; one failure should not block subsequent events |

---

## Common Pitfalls

### Pitfall 1: Initial Assignment Extraction Missing Normalization

**What goes wrong:** Twitch-listener's refresh loop strips the suffix correctly, but the initial extraction at startup (lines 149-152) stores raw `a.SourceID` without stripping. If the coordinator returns `{uuid}:twitch` format, the initial `assignedSourceIDs` map contains `{uuid}:twitch` while the refresh loop would later replace it with `{uuid}`. This causes a brief inconsistency window at startup.

**Why it happens:** The initial extraction was written before the suffix issue was understood.

**How to avoid:** Apply the same strip logic to BOTH the initial extraction block (in both listeners) AND the refresh loop (in both listeners). The fix in kick-listener should also be applied to twitch-listener's startup path if it is not already normalized there.

**Verification:** After the change, log `assignedSourceIDs` keys at INFO level during startup to confirm they are bare UUIDs.

### Pitfall 2: Kick-Listener's `assignedSourceIDs` Map Is Filtered Without Stripping

**What goes wrong:** In `kick-listener/channels/manager.go`, `syncChannels()` at line 308 checks `m.assignedSourceIDs[ch.SourceID]` where `ch.SourceID` is a bare UUID from the database (`overlay_chat_sources.id`). If the map contains `{uuid}:kick` keys, all lookups return `false` and no channels are ever assigned.

**Why it happens:** The kick-listener main.go builds `assignedSourceIDs` without stripping, so the map keys do not match database UUIDs.

**How to avoid:** Fix the intake point (the loop building `assignedSourceIDs` in `cmd/main.go`). The manager's filter loop does not need to change.

**Warning signs:** All channels showing as unassigned in kick-listener logs even after coordinator returns non-empty assignments.

### Pitfall 3: Coverage Verification in Twitch-Listener Also Strips at Redis Scan

**What goes wrong:** `verifyCoverageComplete` in `twitch-listener/channels/manager.go` lines 406-412 also strips suffixes when scanning `shard:assignment:*` Redis keys. If the coordinator changes to always store bare UUIDs, this double-strip logic would truncate valid UUIDs that happen to contain a colon (unlikely but worth noting).

**Why it happens:** The manager defensively strips during Redis scan because the coordinator's Redis key format was uncertain.

**How to avoid:** After normalizing intake, verify that Redis key format is consistent. The manager-level strip in `verifyCoverageComplete` is a safety net and can stay; it is idempotent when no colon is present.

### Pitfall 4: MigrationSubscriber Panic Recovery Is Preserved

**What goes wrong:** During the signature change, the panic recovery `defer` in `consumeMessages` is accidentally removed or the error-capture `if` block is placed outside the anonymous function, bypassing the recover.

**Why it happens:** Refactoring the call site while also adding error capture.

**How to avoid:** Keep the structure as a named or anonymous function with `defer recover()`. Add `if err := s.handler(&event); err != nil { ... }` inside that same function, after the defer.

---

## Code Examples

### Complete Fix for kick-listener `cmd/main.go` assignment refresh

The assignment refresh loop in kick-listener (lines 259-263) currently:
```go
newAssignedIDs := make(map[string]bool)
for _, a := range newAssignments {
    newAssignedIDs[a.SourceID] = true
}
```

Must become (matching twitch-listener pattern):
```go
newAssignedIDs := make(map[string]bool)
for _, a := range newAssignments {
    sourceID := a.SourceID
    if colonIdx := strings.LastIndexByte(sourceID, ':'); colonIdx != -1 {
        sourceID = sourceID[:colonIdx]
    }
    newAssignedIDs[sourceID] = true
}
```

The `strings` import is already present in `kick-listener/cmd/main.go`.

### Complete `HandleMigrationEvent` Signature Change

**twitch-listener `channels/manager.go` line 600:**
```go
// Before:
func (m *Manager) HandleMigrationEvent(event *coordination.MigrationEvent) {

// After:
func (m *Manager) HandleMigrationEvent(event *coordination.MigrationEvent) error {
```

The return value must be added at the end of the function body:
```go
    // existing body unchanged
    return nil
}
```

**kick-listener `channels/manager.go` line 675:**
```go
// Before:
func (m *Manager) HandleMigrationEvent(event *coordination.MigrationEvent) {

// After:
func (m *Manager) HandleMigrationEvent(event *coordination.MigrationEvent) error {
```

Same treatment: add `return nil` at end of function.

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| No suffix normalization on kick | Strip suffix at intake (same as twitch) | Phase 33 | Kick channels now correctly matched against coordinator assignments |
| `HandleMigrationEvent() void` | `HandleMigrationEvent() error` | Phase 33 | SDK `ChannelManager` interface can express a uniform error-returning contract |

---

## Open Questions

1. **Does the coordinator server return `{uuid}:platform` or bare `{uuid}` in `Assignment.SourceID`?**
   - What we know: Twitch-listener's refresh loop already strips the suffix, indicating the coordinator has historically returned composite keys. The `verifyCoverageComplete` Redis scan also strips them from `shard:assignment:*` keys.
   - What's unclear: Whether new listeners (discord, youtube-innertube) registered after this change will receive composite or bare IDs. The coordinator source code is in `services/source-manager` and was not read for this phase.
   - Recommendation: Implement strip-at-intake defensively in all listeners. A `strings.LastIndexByte` on a bare UUID (no colon) is a no-op, so defensive stripping is safe even if the coordinator changes to return bare IDs.

2. **Should `HandleMigrationEvent` return a meaningful error or always `nil`?**
   - What we know: The current implementations return nothing; they log errors internally and continue. The canonical SDK signature requires `error` for interface uniformity.
   - What's unclear: Whether any caller in Phase 34+ will inspect the returned error to make decisions (e.g., requeue the event).
   - Recommendation: For Phase 33, both managers return `nil` unconditionally. Internal errors continue to be logged inside the method. The subscriber logs errors from the return value. This satisfies the interface contract without behavioral change.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go standard `testing` + `testify/assert` + `testify/require` |
| Config file | none — standard `go test ./...` per service module |
| Quick run command | `cd services/twitch-listener && go test ./channels/... -v` |
| Full suite command | `cd services/twitch-listener && go test ./... && cd ../kick-listener && go test ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PREP-01 | Source IDs inserted into `assignedSourceIDs` are bare UUIDs (no `:platform` suffix) | unit | `cd services/kick-listener && go test ./channels/... -run TestManager_SourceIDNormalization -v` | ❌ Wave 0 |
| PREP-01 | Coverage check in twitch-listener matches bare-UUID keys | unit | `cd services/twitch-listener && go test ./channels/... -v` | ✅ (existing tests pass) |
| PREP-02 | `HandleMigrationEvent` compiles with `error` return in both managers | compile | `cd services/twitch-listener && go build ./... && cd ../kick-listener && go build ./...` | ✅ after code change |
| PREP-02 | `MigrationSubscriber` logs errors without panicking | unit | `cd shared/coordination && go test ./... -run TestMigrationSubscriber -v` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `cd services/twitch-listener && go test ./channels/... -count=1` and `cd services/kick-listener && go test ./channels/... -count=1`
- **Per wave merge:** `cd services/twitch-listener && go test ./... && cd ../kick-listener && go test ./... && cd ../../../shared/coordination && go test ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `services/kick-listener/channels/manager_test.go` — does not exist; need at minimum a test that verifies `syncChannels` only includes channels whose bare SourceID is in `assignedSourceIDs`
- [ ] `shared/coordination/migration_subscriber_test.go` — does not exist; need a test that verifies the subscriber calls a handler that returns an error, logs it, and continues to the next event without panicking
- [ ] New test `TestManager_SourceIDNormalization` for kick-listener — validates that `:kick` suffix is stripped before being stored in `assignedSourceIDs`

---

## Sources

### Primary (HIGH confidence)

- `/home/moersener/Hobby/all-chat/services/twitch-listener/cmd/main.go` — lines 149-159 (initial extraction), 261-279 (refresh loop with stripping)
- `/home/moersener/Hobby/all-chat/services/kick-listener/cmd/main.go` — lines 133-136 (initial extraction, no stripping), 259-263 (refresh loop, no stripping)
- `/home/moersener/Hobby/all-chat/services/twitch-listener/channels/manager.go` — line 600, `HandleMigrationEvent` signature (no error return)
- `/home/moersener/Hobby/all-chat/services/kick-listener/channels/manager.go` — line 675, `HandleMigrationEvent` signature (no error return)
- `/home/moersener/Hobby/all-chat/shared/coordination/migration_subscriber.go` — handler field type `func(*MigrationEvent)`, call site at line 108
- `/home/moersener/Hobby/all-chat/shared/coordination/models.go` — `MigrationEvent` struct
- `/home/moersener/Hobby/all-chat/services/twitch-listener/channels/manager_test.go` — existing tests that must continue to pass

### Secondary (MEDIUM confidence)

- `.planning/REQUIREMENTS.md` — PREP-01, PREP-02 requirement text
- `.planning/ROADMAP.md` — Phase 33 success criteria
- `.planning/STATE.md` — Phase 33 research flags

---

## Metadata

**Confidence breakdown:**
- Source ID inconsistency: HIGH — both divergent code paths read directly from source files
- HandleMigrationEvent signature: HIGH — current signatures and all call sites read directly
- MigrationSubscriber update: HIGH — full file read; all field and call site locations confirmed
- Pitfalls: HIGH — derived from direct code inspection, not inference

**Research date:** 2026-03-17
**Valid until:** Indefinite — this is a static codebase analysis; no external APIs involved
