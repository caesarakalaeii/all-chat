# ADR-0007: Leadership Rebalancing for Auto-Scaling

**Date**: 2026-03-28
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation)

---

## Context and Problem Statement

The `LeadershipCoordinator` uses per-stream Redis locks (`SET NX EX`) to split channels across pods. The first pod to claim a lock holds it indefinitely via heartbeat renewal. When new pods join via auto-scaling, they get 0 work because all locks are already held by existing pods. There is no mechanism for pods to voluntarily release excess locks.

This affects twitch-listener, kick-listener, and youtube-listener-innertube.

> **Update 2026-09-15**: tiktok-listener now ports this scheme to its TypeScript
> coordinator (`src/coordination/leadership.ts`, `rebalance()`), driven from its
> 25s demand safety-net poll — below the 30s peer TTL, so a pod's peer
> registration does not expire between polls and the stabilization gate can
> actually open. The original omission mattered: on 2026-09-14 one tiktok pod
> held 43 of ~48 leases, overran the Euler free tier's concurrent WebSocket
> proxy cap, and every channel on it went silently deaf (ADR-0052 context). The
> TS port adds the same 30s peer-count stabilization gate as the Go
> coordinator, keeps the Go callers' rebalanced-out exclusion (released
> streams are skipped for one demand cycle so this pod does not immediately
> re-claim them), and takes a `maxPerPod` parameter the service sets to
> `TIKTOK_MAX_STREAMS_PER_POD` (default 20): a pod must never hold more
> leases than it can connect, because a leased-but-unconnectable stream is
> deaf everywhere — no other pod may claim it.

> **Addendum**: the shared Go coordinator later gained a peer-count
> stabilization gate (`shared/sourcemanager/coordinator.go`,
> `DefaultRebalanceStabilizationPeriod` = 30s): `Rebalance` registers the peer
> and returns without releasing anything until the peer count has been stable
> for one period, preventing oscillation while pods scale. The TS port in
> tiktok-listener ports this same gate.

## Decision

Implement peer-aware rebalancing in the shared `LeadershipCoordinator`:

1. **Peer Registry**: Each pod registers itself in Redis via the source-manager
   `POST /leadership/peers/register` endpoint (`services/source-manager/cmd/main.go`,
   handler in `services/source-manager/election/leader.go::RegisterPeer`). A platform
   key (`peers:{platform}`) holds a Redis sorted set with one member per pod, scored
   by its expiry timestamp; registration refreshes the caller's expiry and returns
   the current active peer count (O(1) via ZCARD after pruning expired members).
   Stabilized `Rebalance` releases a pod's excess leases only after the peer count
   has been stable for `DefaultRebalanceStabilizationPeriod` (30s). Go callers keep
   a rebalanced-out exclusion for one demand cycle so a pod does not immediately
   re-claim the streams it released (`services/twitch-listener/channels/manager.go:598`).

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
