# ADR-0007: Leadership Rebalancing for Auto-Scaling

**Date**: 2026-03-28
**Status**: Accepted

---

## Context and Problem Statement

The `LeadershipCoordinator` uses per-stream Redis locks (`SET NX EX`) to split channels across pods. The first pod to claim a lock holds it indefinitely via heartbeat renewal. When new pods join via auto-scaling, they get 0 work because all locks are already held by existing pods. There is no mechanism for pods to voluntarily release excess locks.

This affects twitch-listener, kick-listener, and youtube-listener-innertube.

## Decision

Implement peer-aware rebalancing in the shared `LeadershipCoordinator`:

1. **Peer Registry**: Each pod registers itself in Redis (`peer:{platform}:{callerID}` with 30s TTL) via a new source-manager endpoint. Registration returns the current peer count.

2. **Rebalance Method**: `Rebalance(ctx, totalStreams)` computes `maxPerPod = ceil(total/peers)` and releases excess leases alphabetically by stream ID.

3. **Integration**: Each service calls `Rebalance` at the top of its sync loop (every ~30s), before `EnsureLeadership` calls. Released streams become immediately claimable by other pods.

## Alternatives Considered

- **Consistent hashing**: Each pod computes which streams it "owns" based on a hash ring. More complex, requires all pods to agree on the ring state.
- **Central coordinator assignment**: Source-manager assigns specific streams to specific pods (like tiktok-listener). More complex, requires migration events and assignment queries.
- **Lock stealing**: New pods forcibly take locks from over-loaded pods. Risky — can cause message gaps during handoff.

## Consequences

- **Positive**: Auto-scaling distributes work evenly. Scale from 2→3 pods: each gets ~1/3 of streams within one sync cycle (~30s).
- **Positive**: No forced takeover — pods only voluntarily shed excess, preventing message gaps.
- **Positive**: Works with existing leadership infrastructure — no new dependencies.
- **Negative**: Alphabetical release order means the same streams are always shed first. Acceptable because streams are equivalent.
- **Negative**: Brief gap (~30s) where released streams are unmonitored until another pod claims them. Acceptable for chat messages.
