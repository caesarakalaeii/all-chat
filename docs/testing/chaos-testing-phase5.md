# Phase 5 Chaos Testing: Shard Coordinator Resilience

**Purpose:** Validate coordinator survives production failure modes (network partitions, pod failures, leader failovers, split-brain scenarios).

**Prerequisites:**
- Kubernetes cluster with allchat namespace
- source-manager deployed with 2+ replicas
- Redis and PostgreSQL available
- kubectl access with admin permissions
- Prometheus metrics endpoint accessible

---

## Scenario 1: Leader Pod Failure

**Objective:** Validate graceful leadership transition when current leader pod crashes.

**Setup:**
1. Deploy source-manager with 2 replicas
2. Create 50 test overlay sources in PostgreSQL
3. Wait for leader election (check Prometheus `shard_coordinator_is_leader=1`)
4. Record current leader pod name: `LEADER=$(kubectl get pods -n allchat -l app=source-manager -o jsonpath='{.items[?(@.metadata.annotations.leader=="true")].metadata.name}')`

**Chaos injection:**
```bash
kubectl delete pod -n allchat $LEADER --grace-period=0 --force
```

**Validation:**
1. Within 30 seconds: New leader elected (check logs for "Started leading")
2. Within 60 seconds: All 50 sources reassigned (query Redis: `DBSIZE shard:assignment:*`)
3. Prometheus metrics:
   - `shard_coordinator_is_leader` = 1 on new leader
   - `shard_reconciliation_cycles_total` incremented on new leader
4. No duplicate assignments (each source has exactly one pod_id)

**Expected outcome:** New leader takes over within 30s, reassigns all channels within 60s, no message loss.

**Failure indicators:**
- Leadership not acquired within 30s → check Lease configuration
- Assignments incomplete after 60s → check reconciliation loop
- Duplicate assignments → check version counter fencing

---

## Scenario 2: Network Partition (Leader Isolated)

**Objective:** Validate split-brain prevention when leader loses connectivity to Kubernetes API.

**Setup:**
1. Identify current leader pod
2. Record initial assignments: `kubectl exec -n allchat $LEADER -- redis-cli KEYS "shard:assignment:*" | wc -l`

**Chaos injection:**
Simulate network partition using iptables (blocks Kubernetes API access):
```bash
kubectl exec -n allchat $LEADER -- iptables -A OUTPUT -d <K8S_API_IP> -j DROP
```

**Validation:**
1. Within 30 seconds: Leader's lease expires (LeaseDuration=30s)
2. Within 45 seconds: New leader elected on healthy replica
3. Stale leader stops reconciliation (logs: "Lost leadership")
4. Stale leader's Redis writes rejected (version counter check fails)
5. Prometheus metrics:
   - `shard_coordinator_is_leader` = 0 on stale leader
   - `shard_coordinator_is_leader` = 1 on new leader only

**Expected outcome:** Kubernetes Lease prevents split-brain, only one coordinator active at a time.

**Failure indicators:**
- Two leaders active simultaneously → Lease fencing not working
- Stale leader continues writing → version counter not implemented
- Assignment conflicts → Redis atomicity broken

---

## Scenario 3: Listener Pod Failure

**Objective:** Validate channel redistribution when listener pod fails (heartbeat timeout).

**Setup:**
1. Deploy twitch-listener with 3 replicas
2. Assign 60 sources (20 per pod average)
3. Record pod assignments: `kubectl exec -n allchat redis-0 -- redis-cli ZRANGE shard:load 0 -1 WITHSCORES`

**Chaos injection:**
Stop one listener pod (simulate crash):
```bash
LISTENER_POD=$(kubectl get pods -n allchat -l app=twitch-listener -o jsonpath='{.items[0].metadata.name}')
kubectl delete pod -n allchat $LISTENER_POD --grace-period=0 --force
```

**Validation:**
1. Within 15 seconds: Coordinator detects missing heartbeat (logs: "Detected failed pods")
2. Within 45 seconds: Failed pod's 20 sources redistributed to remaining 2 pods
3. Remaining pods have ~30 sources each (bounded-load enforced: max 1.25 * avg = 37.5)
4. Prometheus metrics:
   - `shard_failed_pods` increments by 1
   - `shard_healthy_pods` decrements by 1
   - `shard_imbalance_ratio` < 1.25

**Expected outcome:** Fast failure detection (15s per user constraint), redistribution within SLA (60s total), load remains balanced.

**Failure indicators:**
- Heartbeat timeout >15s → check HeartbeatTimeout constant (must be 15s per CONTEXT.md)
- Redistribution >60s → check reconciliation interval
- Load imbalance >1.25x → check bounded-load configuration

---

## Scenario 4: Simultaneous Leader and Listener Failure

**Objective:** Validate coordinator resilience when both leader and listener pods fail simultaneously.

**Setup:**
1. Deploy source-manager (2 replicas) and twitch-listener (3 replicas)
2. Assign 60 sources
3. Identify current leader and one listener pod

**Chaos injection:**
Delete both pods simultaneously:
```bash
kubectl delete pod -n allchat $LEADER $LISTENER_POD --grace-period=0 --force
```

**Validation:**
1. Within 30 seconds: New coordinator leader elected
2. Within 60 seconds: Failed listener's sources redistributed
3. No assignment gaps (all 60 sources still assigned)
4. Prometheus metrics:
   - `shard_coordinator_is_leader` = 1 on new leader
   - `shard_healthy_pods` = 2 (remaining listeners)
   - All assignments accounted for

**Expected outcome:** System recovers from cascading failures, maintains assignment consistency.

**Failure indicators:**
- Assignment gaps → reconciliation not comprehensive
- Leader election blocked → check RetryPeriod configuration
- Assignment conflicts → fencing not working

---

## Scenario 5: Redis Latency Spike

**Objective:** Validate coordinator behavior during Redis latency (heartbeat false positives).

**Setup:**
1. Deploy all components
2. Monitor baseline Prometheus `redis_command_duration_seconds`

**Chaos injection:**
Simulate Redis latency using tc (traffic control):
```bash
kubectl exec -n allchat redis-0 -- tc qdisc add dev eth0 root netem delay 2000ms
```

**Validation:**
1. Heartbeat publishes continue (listener retries 3x with 2s timeout each per RESEARCH.md Pitfall 5)
2. Coordinator tolerates temporary latency (no false failure detection)
3. Prometheus metrics:
   - `shard_heartbeat_errors_total` may increment slightly but recovers
   - `shard_failed_pods` does NOT spike (no false positives)
4. After latency removed: system returns to normal within 30s

**Expected outcome:** Heartbeat retry logic prevents false positives, coordinator stable during transient latency.

**Failure indicators:**
- Pods marked as failed during latency → heartbeat timeout too aggressive (should be 15s per CONTEXT.md, enough for 3x2s retries)
- Cascading failures → listener retry logic missing

---

## Validation Checklist

After running all scenarios:

- [ ] Leader election happens within 30 seconds of leader failure
- [ ] Channel redistribution completes within 60 seconds of listener failure
- [ ] Split-brain never occurs (only one coordinator leader at a time)
- [ ] Version counter prevents stale writes
- [ ] Bounded-load maintained (max load ≤ 1.25 * avg) during redistribution
- [ ] Heartbeat false positives prevented during Redis latency
- [ ] Prometheus metrics accurately reflect system state
- [ ] No assignment gaps or conflicts in any scenario

**Tools:**
- Query assignments: `kubectl exec -n allchat redis-0 -- redis-cli KEYS "shard:assignment:*"`
- Query load distribution: `kubectl exec -n allchat redis-0 -- redis-cli ZRANGE shard:load 0 -1 WITHSCORES`
- Query heartbeats: `kubectl exec -n allchat redis-0 -- redis-cli ZRANGE shard:heartbeats 0 -1 WITHSCORES`
- Check leader: `kubectl logs -n allchat -l app=source-manager --tail=50 | grep "Started leading"`
- Prometheus query: `curl http://source-manager:8088/metrics | grep shard_`

---

## References

**RESEARCH.md Pitfalls:**
- Pitfall 1: Split-brain (two coordinators active) → Validated in Scenario 2 via Kubernetes Lease fencing
- Pitfall 2: Thundering herd during leader election → Monitored via RetryPeriod and jitter in leader election
- Pitfall 3: Stale assignments from old leader → Validated in Scenario 2 via version counter checks
- Pitfall 4: Load imbalance during redistribution → Validated in Scenario 3 via bounded-load constraint (1.25x)
- Pitfall 5: Heartbeat false positives → Validated in Scenario 5 via retry logic and 15s timeout

**Success Criteria:**
- Success Criterion #5: "Coordinator survives chaos testing (network partitions, pod failures, leader changes)"

**User Constraints:**
- Heartbeat timeout: 15s (not 30s) per CONTEXT.md for fast failure recovery
- Channel redistribution SLA: 60s total (15s detection + 45s redistribution)

**Phase 5 Requirements Validated:**
- SHARD-01: Consistent hashing → Validated in all scenarios (minimal reassignments)
- SHARD-02: Bounded-load balancing → Validated in Scenario 3 (load imbalance <1.25x)
- SHARD-04: Leader election with split-brain prevention → Validated in Scenarios 1, 2, 4
- SHARD-05: Assignment API with version counter → Validated in Scenario 2 (stale write rejection)
- SHARD-06: Heartbeat monitoring → Validated in Scenarios 3, 5
- SHARD-08: Prometheus metrics → Validated in all scenarios
- REBAL-08: Fast failure detection → Validated in Scenario 3 (15s timeout)
