# Deferred Items — Phase 33 Pre-Migration Cleanup

## Out-of-Scope Issues Discovered During Execution

### [Pre-existing] TestRepository_GetActiveChannelsHandlesStringChatroomIDs fails in kick-listener

- **Discovered during:** Plan 33-01, Task 2 verification
- **Service:** services/kick-listener/channels
- **File:** services/kick-listener/channels/repository_test.go:29
- **Error:** `expected 2 channels, got 0`
- **Status:** Pre-existing failure — confirmed present before any 33-01 changes by `git stash` verification
- **Action required:** Separate fix outside this plan's scope; address before phase-gate CI check

### [Pre-existing] TestStartStopJWTRefresh panics in shared/coordination

- **Discovered during:** Plan 33-02, Task 2 verification
- **Service:** shared/coordination
- **File:** shared/coordination/client_jwt_test.go:152
- **Error:** `panic: close of closed channel` in `CoordinatorClient.StopJWTRefresh` (client.go:299)
- **Status:** Pre-existing failure — confirmed present before any 33-02 changes by `git stash` verification
- **Action required:** Guard `StopJWTRefresh` against double-close of channel; address before phase-gate CI check
