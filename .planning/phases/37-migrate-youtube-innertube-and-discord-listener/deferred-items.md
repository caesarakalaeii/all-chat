# Deferred Items — Phase 37

## Pre-existing Test Failures (Out of Scope)

### discord-listener gateway package tests

**Discovered during:** Plan 37-03, Task 2

**Issue:** All gateway package tests fail to compile with:
```
*mockChannelRegistry does not implement gateway.ChannelRegistry (missing method ListConfiguredChannels)
```

The `mockChannelRegistry` in gateway test files does not include `ListConfiguredChannels`, which was added to the `gateway.ChannelRegistry` interface in a prior phase (v1.5 Discord Listener). The production code uses `redisChannelRegistry` which does implement `ListConfiguredChannels`.

**Confirmed pre-existing:** Verified by stashing Plan 37-03 changes and running `go test ./...` — same failures appear with no changes applied.

**Affected files:**
- `services/discord-listener/gateway/guild_cache_test.go`
- `services/discord-listener/gateway/mention_test.go`
- `services/discord-listener/gateway/message_create_test.go`
- `services/discord-listener/gateway/message_delete_test.go`

**Fix needed:** Add `ListConfiguredChannels(_ context.Context) (map[string]string, error)` stub to `mockChannelRegistry` in each affected test file.

**Not fixed in this phase** — out of scope per deviation rules (pre-existing, not caused by current task changes).

---

### youtube-listener-innertube streams and poller test failures

**Discovered during:** Plan 37-02, Task 2

**Issue 1:** `TestManager_OnOverlayConnected_CachedVideoID` in `streams` package panics with nil pointer dereference in `innertube.(*Discovery).GetInitialContinuation(0x0, ...)` — discovery is nil in test setup.

**Issue 2:** `TestPoller_SuccessfulPolling` in `poller` package fails with continuation token mismatch: `continuation = token-1, want token-2 or token-3`.

**Confirmed pre-existing:** Verified by stashing Plan 37-02 changes and running `go test ./streams/... ./poller/...` — same failures appear with no changes applied.

**Not fixed in this phase** — out of scope per deviation rules (pre-existing, not caused by current task changes).
