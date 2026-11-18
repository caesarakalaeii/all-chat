# ADR-0007: Leadership Rebalancing for Auto-Scaling

**Date**: 2026-03-28
**Status**: Accepted
**Revised**: 2026-11-03

## Context

The `LeadershipCoordinator` splits streams across pods with per-stream Redis locks (`SET NX EX`). The first pod to claim a lock holds it via heartbeat renewal, so pods added by auto-scaling get zero work; nothing lets a pod shed excess locks. The tiktok-listener port of the scheme illustrated the failure mode: on 2026-09-14 one pod held 43 of ~48 leases and every channel on it went deaf. Affected: twitch-listener, kick-listener, youtube-listener-innertube, and tiktok-listener (TypeScript port since 2026-09-15, `src/coordination/leadership.ts`).

## Decision

Peer-aware rebalancing in the shared `LeadershipCoordinator`:

- Each pod registers in Redis via source-manager `POST /leadership/peers/register` (`services/source-manager/cmd/main.go`, handler in `services/source-manager/election/leader.go`): a sorted set `peers:{platform}` with one member per pod scored by expiry timestamp, refreshed per registration, returning the active peer count.
- `Stabilized Rebalance` releases the pod's excess leases only after the peer count has been stable for `DefaultRebalanceStabilizationPeriod` (30s, `shared/sourcemanager/coordinator.go`), preventing oscillation while pods scale. Go callers keep a rebalanced-out exclusion for one demand cycle so the pod does not immediately re-claim the streams it released (`services/twitch-listener/channels/manager.go`).
- The tiktok-listener TS port (`src/coordination/leadership.ts`, `rebalance()`) carries the same gate and exclusion, is driven from its 25s demand safety-net poll, and takes `maxPerPod` (`TIKTOK_MAX_STREAMS_PER_POD`, default 20): a pod must never hold more leases than it can connect, since a leased-but-unconnectable stream is deaf everywhere.

Alternatives rejected: consistent hashing (all pods must agree on ring state); central assignment by source-manager (assignment queries and migration events); lock stealing (message gaps during forced handoff).

## Consequences

- Scaling from 2 to 3 pods distributes streams evenly within about 30s, without forced takeover.
- Released streams are unmonitored for roughly 30s until another pod claims them; acceptable for chat. Alphabetical release order always sheds the same streams first; acceptable because streams are equivalent.
