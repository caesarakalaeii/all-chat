# Phase 07: Dynamic Rebalancing & HPA Integration - Context

**Gathered:** 2026-02-20
**Status:** Ready for planning

<domain>
## Phase Boundary

Automatic load-aware channel redistribution system that monitors pod workload (message rate and channel count) and redistributes channels when imbalance is detected. Includes safeguards against thrashing and coordination with Kubernetes HPA scale events. Phase 6 migration infrastructure is already complete - this phase adds the intelligence layer that decides WHEN and WHAT to migrate.

</domain>

<decisions>
## Implementation Decisions

### Rebalancing Triggers
- Monitor pod load every 30 seconds (per requirements)
- Trigger rebalancing when BOTH conditions met:
  - Imbalance ratio (max_load / avg_load) exceeds 0.5
  - Busiest pod exceeds 100 msg/sec minimum threshold
- Use weighted combination of message rate AND channel count when calculating load
- Rationale: Avoid unnecessary rebalancing when system is mostly idle, but remain responsive under load

### Channel Selection Strategy
- Use proportional redistribution approach: move channels to equalize total load across pods (not just hot channels)
- Apply 20% limit per pod: each overloaded pod can migrate up to 20% of its channels per operation
- Prefer moving many low-traffic channels over few high-traffic channels (reduces migration risk)
- Select destination pods using round-robin across all underutilized pods (below average load)
- **RESEARCH REQUIRED**: Profile listener resource usage to determine connection overhead vs message processing cost - informs whether channel count or message rate is the dominant load factor

### Safeguard Behavior
- Enforce 5-minute cooldown between rebalancing operations (per requirements)
- Use escalation override: allow earlier rebalancing if imbalance increases significantly (e.g., ratio jumps from 0.6 to 1.0)
- Abort rebalancing operation if target pod becomes unhealthy or migration confirmations fail
- Thrashing detection (>3 rebalances in 15min) response: Claude's discretion based on research
- Incomplete rebalancing handling (when 20% isn't enough): Claude's discretion

### HPA Coordination
- Staggered pod startup: each new pod waits random delay (0-30s) before querying coordinator (prevents thundering herd)
- Scale-up interaction: abort current rebalancing when HPA triggers scale-up, let scale complete, then reassess
- Scale-down handling: proactively migrate channels away from to-be-terminated pod before Kubernetes kills it
- Lock coordination: use Redis distributed locks - only one operation (rebalance or scale event) can modify assignments at a time

### Claude's Discretion
- Exact weighted formula for combining message rate and channel count into composite load score
- Thrashing response strategy (pause, increase thresholds, or alert-only)
- Handling incomplete rebalancing when 20% limit isn't sufficient
- Specific escalation thresholds for overriding cooldown
- Implementation details of preemptive migration timing during scale-down

</decisions>

<specifics>
## Specific Ideas

- Connection overhead vs message processing: User questioned whether "tiny channels" (few messages) actually provide minimal load - if connection overhead (TCP sockets, heartbeats, buffers) is significant compared to message processing, then many idle channels could be more expensive than few active channels. This affects whether to balance by channel count or message rate primarily.

- Escalation override: Allow breaking cooldown when imbalance significantly worsens (not just persists) - indicates system state change that needs immediate response.

</specifics>

<deferred>
## Deferred Ideas

None - discussion stayed within phase scope.

</deferred>

---

*Phase: 07-dynamic-rebalancing-hpa-integration*
*Context gathered: 2026-02-20*
