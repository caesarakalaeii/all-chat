# ADR-0011: Zombie Listener Detection via Received-vs-Published Drift

**Date**: 2026-04-08
**Status**: Accepted

---

## Context and Problem Statement

Recurring production outages (April 7, March 29, 2026) were caused by twitch-listener pods that appeared alive from a Kubernetes perspective but were not delivering messages to Redis. The liveness probe returned 200 (IRC connected, process healthy) while overlays received no new messages.

Post-mortems identified a "zombie" pattern: the IRC connection manager showed activity (PING/PONG flowing) but the stream publisher had stalled — ring buffer enqueue calls were failing silently or XADD operations to Redis were permanently failing. The existing IRC staleness check only detects zombie IRC connections (no PING received for 10 minutes), not zombie publisher paths.

The net effect: pods remained running for 30–85 minutes before a human operator manually restarted them. Kubernetes liveness probes could have auto-recovered these pods if they returned 503.

## Decision

Add received-vs-published message drift detection to the twitch-listener liveness probe using two atomic counters in a new `zombie.Detector` struct:

- **`messagesReceived`**: incremented on each IRC PRIVMSG (in `handlePrivateMessage`).
- **`messagesPublished`**: incremented after each successful ring buffer accept (in `handlePrivateMessage`, after `publisher.Publish` returns nil).

If `received` advances but `published` has not advanced for `stallWindow` (default 5 minutes), `IsZombie()` returns `true` and the liveness probe returns HTTP 503.

Kubernetes then restarts the pod within the next probe cycle, automatically recovering the outage.

## Rationale for 5-Minute Stall Window

Balances detection speed against false positive risk:

- **Shorter** (e.g. 1 minute): Higher risk of false positives during transient Redis blips. JitteredBackoff cap is 30s, so a reconnect takes at most ~10 retry cycles (~5 min total). Killing the pod at 1 minute would interrupt an in-progress recovery.
- **Longer** (e.g. 10 minutes): Matches the existing IRC staleness threshold — defeats the purpose of adding a separate publish-stall detector.
- **5 minutes**: Gives the Redis reconnect backoff time to recover (30s cap × ~10 cycles = ~5 minutes), while keeping outage duration to a 5-minute detection window + Kubernetes restart (~30s) = ~5.5 minutes maximum. Significantly better than the 30–85 minute manual recovery observed in production.

Configurable via `ZOMBIE_STALL_WINDOW_MINUTES` environment variable.

## False Positive Avoidance via Both-Zero Check (D-10)

When a streamer is offline, both `received` and `published` are 0. No drift is present, so `IsZombie()` returns false.

This is critical because source-manager keeps sources "active" in its registry even when streamers are offline — a pod with assigned channels but no active streams has zero messages flowing through it and must not be treated as a zombie.

## Snapshot Rotation

`IsZombie()` uses a two-snapshot approach rather than continuous polling:

1. A snapshot of `{received, published}` is taken every `stallWindow/2` (2.5 minutes by default).
2. On each call, deltas since the last snapshot are computed.
3. If not enough time has elapsed since the last snapshot (`< stallWindow/2`), `false` is returned — insufficient data.
4. If enough time has elapsed, the delta is evaluated: `receivedDelta > 0 && publishedDelta == 0` → zombie.
5. The snapshot is rotated after evaluation so the next window starts fresh.

This gives a detection latency of `stallWindow/2` (2.5 minutes) with a maximum of one full `stallWindow` (5 minutes) in the worst case.

## Alternatives Considered

1. **Heartbeat-based detection**: A background goroutine sends a test message to Redis every N seconds and checks if it arrives back. Rejected — adds network round-trip complexity, requires a separate Redis channel, and doesn't directly measure the actual publish path.

2. **External monitoring only (Grafana alerts + PagerDuty)**: Monitor `ring_buffer_depth` and alert when buffer fills. Rejected — alerting latency (5+ minutes to page, human response 5+ minutes) gives 10+ minute outages vs. 5.5-minute auto-recovery. Also requires human action for a mechanical failure.

3. **Readiness probe instead of liveness**: Failing readiness removes the pod from the load balancer service mesh but does not restart it. Rejected — the zombie pod is consuming IRC channel leadership locks that prevent other pods from taking over. A restart (liveness failure) releases those locks immediately, enabling fast recovery via peer takeover.

4. **Counter on XADD success (not ring buffer accept)**: Counting only confirmed XADD successes would miss the ring buffer buffering case — a message buffered for retry would not increment the counter, potentially triggering false positives during transient Redis blips. Counting ring buffer accept (the call to `publisher.Publish` returning nil) is safer: the ring buffer absorbs blips, so `publishedDelta == 0` only when the entire publish pipeline is stalled.

## Consequences

### Positive

- Zombie pods are automatically restarted within ~5.5 minutes (detection window + Kubernetes restart).
- No manual operator intervention required for publish-stall outages.
- Leadership locks on zombie pod's channels are released on pod restart, enabling the healthy peer pod to claim them within one sync cycle (~30s).
- False positives on offline channels prevented by the both-zero check.

### Negative

- Pods may be restarted during genuine extended Redis outages (> 5 minutes). Acceptable — if Redis is unavailable for 5+ minutes, restarting the pod is a reasonable recovery action. The pod will restart, IRC will reconnect, and publishing will resume when Redis recovers.
- The snapshot approach means a stall must persist for `stallWindow/2` before detection. This is intentional — shorter would increase false positive risk.

## Implementation

- **Files**: `services/twitch-listener/zombie/detector.go`, `services/twitch-listener/zombie/detector_test.go`
- **Wiring**: `services/twitch-listener/irc/connection.go` (RecordReceived, RecordPublished), `services/twitch-listener/handlers/health.go` (IsZombie check), `services/twitch-listener/cmd/main.go` (construction and injection)
- **Configuration**: `ZOMBIE_STALL_WINDOW_MINUTES` env var (default: 5)
- **Implemented**: Phase 10, Plan 01 (2026-04-08)

## Related Decisions

- ADR-0009: Ring Buffer Publisher — the publish counter is incremented on ring buffer accept, not XADD success, deliberately consistent with the ring buffer's buffering semantics.
- ADR-0007: Leadership Rebalancing — zombie pod restart releases leadership locks, enabling automatic rebalancing to the healthy peer pod.
