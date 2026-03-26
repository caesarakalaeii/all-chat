# GSD Debug Knowledge Base

Resolved debug sessions. Used by `gsd-debugger` to surface known-pattern hypotheses at the start of new investigations.

---

## kick-messages-not-in-overlay — HeartbeatInterval > HeartbeatTimeout causes false pod failures and dropped assignments
- **Date:** 2026-03-26
- **Error patterns:** assigned=0, no pods available, filtered channels assigned=0, kick assignments cleared, source-manager reconciliation, heartbeat stale, failed_pods
- **Root cause:** HeartbeatInterval (30s) exceeded source-manager HeartbeatTimeout (15s). During reconciliation cycles, listener pods appeared as failed when their heartbeat was between 15–30s old. The source-manager wrote no assignments for those pods, causing listeners to unsubscribe from all channels and stop publishing messages.
- **Fix:** Reduced HeartbeatInterval from 30s to 10s in `shared/listener/config.go` DefaultConfig(). HeartbeatInterval must always be strictly less than HeartbeatTimeout.
- **Files changed:** shared/listener/config.go
---
