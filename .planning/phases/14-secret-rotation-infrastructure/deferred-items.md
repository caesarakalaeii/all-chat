## Deferred: Pre-existing data race in shared/listener

**Discovered during:** 14-02 full-suite run with -race
**File:** shared/listener/ring_buffer_test.go:185 + ring_buffer.go:237
**Test:** TestRingBufferRetryUsesBackgroundContext
**Race:** goroutine 43 reads `ring_buffer_test.go:185` after goroutine 46 writes at `ring_buffer.go:237` — concurrent publish path in drainOneTick vs test assertion
**Out of scope:** Pre-existing; not caused by 14-02 changes (only shared/auth/jwt.go modified)
**Auth tests pass clean:** go test ./auth/... -count=1 -race exits 0
