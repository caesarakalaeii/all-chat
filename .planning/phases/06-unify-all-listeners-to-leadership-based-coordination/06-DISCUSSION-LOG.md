# Phase 6: Unify All Listeners to Leadership-Based Coordination - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-27
**Phase:** 06-unify-all-listeners-to-leadership-based-coordination
**Areas discussed:** Coordinator removal scope, Twitch IRC constraints, Migration strategy

---

## Coordinator Removal Scope

### Q1: What should happen to coordinator infrastructure after migration?

| Option | Description | Selected |
|--------|-------------|----------|
| Full removal (Recommended) | Delete CoordinatorClient, heartbeat/assignment loops in ListenerBase, and the coordinator HTTP endpoints in source-manager. Clean break — no dead code. | ✓ |
| Keep as deprecated fallback | Migrate all listeners to leadership but leave the coordinator code intact, marked deprecated. Safety net for rollback. | |
| Remove client, keep server | Remove CoordinatorClient from listeners but keep the source-manager coordinator endpoints. | |

**User's choice:** Full removal
**Notes:** None

### Q2: Should ListenerBase be kept or merged into LeadershipListener?

| Option | Description | Selected |
|--------|-------------|----------|
| Merge into LeadershipListener | Single type: LeadershipListener handles everything (leadership + demand). ListenerBase removed entirely. | ✓ |
| Keep ListenerBase as thin wrapper | ListenerBase stays but only owns demand subscriber loop. Two types but clear separation. | |
| You decide | Claude's discretion. | |

**User's choice:** Merge into LeadershipListener
**Notes:** Single type, simpler mental model

### Q3: Should source-manager consolidate to a single port?

| Option | Description | Selected |
|--------|-------------|----------|
| Consolidate to 8083 | Remove 8088 entirely. All source-manager traffic goes through 8083. | ✓ |
| Keep both ports | Leave 8088 running but empty. | |
| You decide | Claude's discretion on port consolidation. | |

**User's choice:** Consolidate to 8083
**Notes:** None

---

## Twitch IRC Constraints

### Q4: How should leadership interact with Twitch's always-connected IRC model?

| Option | Description | Selected |
|--------|-------------|----------|
| Leadership for source discovery only | LeadershipListener tells Twitch which channels exist, but Twitch stays always-connected. No demand gating. | ✓ |
| Leadership + soft demand gating | Stays connected but stops forwarding messages when no demand. | |
| Full demand gating with rate-limited disconnect | PARTs channels when demand drops with rate-limited batching. | |

**User's choice:** Leadership for source discovery only
**Notes:** Phase 5 excluded Twitch from demand-driven — that exclusion stays

### Q5: Should twitch-eventsub-listener use the same approach?

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, same as Twitch IRC | Leadership provides source discovery. EventSub subscriptions stay always-active. | |
| Demand-gate EventSub too | Unlike IRC, EventSub subscriptions can be created/deleted without rate limits. | ✓ |

**User's choice:** Demand-gate EventSub too
**Notes:** EventSub subscriptions don't have IRC's rate limits, so full demand gating is feasible

---

## Migration Strategy

### Q6: What order should the migration happen?

| Option | Description | Selected |
|--------|-------------|----------|
| SDK first, then listeners, then cleanup (Recommended) | Wave 1: SDK refactor. Wave 2: Migrate listeners. Wave 3: Remove coordinator. Safe rollback between waves. | ✓ |
| All in one wave | Everything at once. Faster but riskier. | |
| One listener at a time | Each listener as its own step. Slowest but safest. | |

**User's choice:** SDK first, then listeners, then cleanup
**Notes:** None

### Q7: Should shared/coordination package be deleted entirely?

| Option | Description | Selected |
|--------|-------------|----------|
| Delete shared/coordination entirely | MigrationSubscriber, CoordinatorClient, models — all go. | ✓ |
| Keep MigrationSubscriber | Migration events might still be useful for leadership-based rebalancing. | |
| You decide | Claude audits what's used, removes only dead code. | |

**User's choice:** Delete shared/coordination entirely
**Notes:** None

---

## Claude's Discretion

- LeadershipListener internal API design
- Demand subscriber loop integration into merged type
- K8s manifest changes for port consolidation
- Test strategy
- Whether kick migration is combined with Twitch or separate

## Deferred Ideas

None
