# ADR-0011: Zombie Listener Detection via Received-vs-Published Drift

**Date**: 2026-04-08
**Status**: Accepted
**Revised**: 2026-11-03

## Context

Recurring production outages (April 7 and March 29, 2026) were caused by twitch-listener pods that looked alive to Kubernetes (liveness 200, process healthy) but published nothing to Redis: ping/pong still flowed while the ring buffer silently stalled or XADD failed permanently. Pods ran 30-85 minutes before a human restarted them; a 503 on liveness would have auto-recovered them in minutes.

## Decision

A `zombie.Detector` in the twitch-listener drives the liveness probe with two atomic counters: `messagesReceived` (per IRC PRIVMSG) and `messagesPublished` (per successful ring buffer accept). If `received` advances but `published` has not for `stallWindow` (default 5 minutes, `ZOMBIE_STALL_WINDOW_MINUTES`), the probe returns 503 and Kubernetes restarts the pod, releasing its leadership locks for peers to take over (ADR-0007).

Detection uses snapshot rotation rather than continuous polling: a `{received, published}` snapshot every `stallWindow/2`, deltas evaluated on each call, snapshot rotated after evaluation. Detection latency is `stallWindow/2` (2.5 minutes), at most one full window.

False positives avoided two ways: the both-zero check (streamer offline means both counters are 0, no drift, not a zombie, even though source-manager keeps the source "active"), and counting ring buffer accept rather than confirmed XADD success, so a transient Redis blip absorbed by the ring buffer does not look like a stall (ADR-0009).

The 5-minute window gives the Redis reconnect backoff (30s cap, ~10 cycles, about 5 minutes) time to recover before the pod is killed; a shorter window risks killing an in-progress recovery, a longer one matches the old staleness threshold and adds nothing.

Note: since the Twitch IRC listener entered enforce mode with chat flowing through EventSub instead (ADR-0026), the listener joins no chat channels, so the IRC path this guard was built for no longer carries chat traffic. The detector remains wired and operational as designed.

Alternatives rejected: heartbeat test message through Redis (extra channel and round-trip, does not measure the real publish path); external-only monitoring (10+ minute outages with a human in the loop vs ~5.5 minutes auto-recovery); readiness probe instead of liveness (removes the pod from LB but does not restart it or release the leadership locks other pods need).

## Implementation

`services/twitch-listener/zombie/detector.go` (+ tests); counters wired in `services/twitch-listener/irc/connection.go`, zombie check in `services/twitch-listener/handlers/health.go`, construction in `cmd/main.go`; configured by `ZOMBIE_STALL_WINDOW_MINUTES` (default 5).

Related: ADR-0009 (ring buffer accept semantics), ADR-0007 (rebalancing after restart).
