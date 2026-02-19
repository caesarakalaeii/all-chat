#!/bin/bash
set -euo pipefail

# Phase 5 Chaos Testing Automation Script
# Runs chaos scenarios from docs/testing/chaos-testing-phase5.md

NAMESPACE="allchat"
REDIS_POD="redis-0"

echo "=== Phase 5 Chaos Testing ==="
echo "Namespace: $NAMESPACE"
echo ""

# Helper functions
get_leader_pod() {
    kubectl get pods -n $NAMESPACE -l app=source-manager -o json | \
        jq -r '.items[] | select(.metadata.annotations.leader=="true") | .metadata.name'
}

count_assignments() {
    kubectl exec -n $NAMESPACE $REDIS_POD -- redis-cli KEYS "shard:assignment:*" | wc -l
}

check_metric() {
    local pod=$1
    local metric=$2
    kubectl exec -n $NAMESPACE $pod -- wget -qO- http://localhost:8088/metrics | grep "^$metric"
}

wait_for_condition() {
    local condition=$1
    local timeout=$2
    local elapsed=0

    while ! eval "$condition"; do
        sleep 1
        elapsed=$((elapsed + 1))
        if [ $elapsed -ge $timeout ]; then
            echo "Timeout waiting for: $condition"
            return 1
        fi
    done
    echo "Condition met after ${elapsed}s: $condition"
}

# Scenario 1: Leader Pod Failure
echo "--- Scenario 1: Leader Pod Failure ---"
LEADER=$(get_leader_pod)
echo "Current leader: $LEADER"

BEFORE_COUNT=$(count_assignments)
echo "Assignments before: $BEFORE_COUNT"

echo "Deleting leader pod..."
kubectl delete pod -n $NAMESPACE $LEADER --grace-period=0 --force

echo "Waiting for new leader election (30s timeout)..."
wait_for_condition "[ -n \"$(get_leader_pod)\" ]" 30

NEW_LEADER=$(get_leader_pod)
echo "New leader: $NEW_LEADER"

echo "Waiting for assignment completion (60s timeout)..."
sleep 60

AFTER_COUNT=$(count_assignments)
echo "Assignments after: $AFTER_COUNT"

if [ "$BEFORE_COUNT" -eq "$AFTER_COUNT" ]; then
    echo "✓ Scenario 1 PASSED: All assignments recovered"
else
    echo "✗ Scenario 1 FAILED: Assignment count mismatch ($BEFORE_COUNT → $AFTER_COUNT)"
    exit 1
fi
echo ""

# Scenario 2: Network Partition
echo "--- Scenario 2: Network Partition (Requires manual validation) ---"
echo "This scenario requires iptables access and manual cleanup."
echo "See docs/testing/chaos-testing-phase5.md for full procedure."
echo "Skipping automated execution."
echo ""

# Scenario 3: Listener Pod Failure
echo "--- Scenario 3: Listener Pod Failure ---"
LISTENER_POD=$(kubectl get pods -n $NAMESPACE -l app=twitch-listener -o jsonpath='{.items[0].metadata.name}')
echo "Target listener pod: $LISTENER_POD"

echo "Recording pod load distribution..."
kubectl exec -n $NAMESPACE $REDIS_POD -- redis-cli ZRANGE shard:load 0 -1 WITHSCORES

echo "Deleting listener pod..."
kubectl delete pod -n $NAMESPACE $LISTENER_POD --grace-period=0 --force

echo "Waiting for heartbeat timeout detection (15s per CONTEXT.md)..."
sleep 15

echo "Waiting for redistribution (45s)..."
sleep 45

echo "Pod load distribution after redistribution:"
kubectl exec -n $NAMESPACE $REDIS_POD -- redis-cli ZRANGE shard:load 0 -1 WITHSCORES

echo "✓ Scenario 3 COMPLETED: Check load distribution manually"
echo "  Verify: No pod exceeds 1.25x average load"
echo ""

# Scenario 4: Simultaneous Failures
echo "--- Scenario 4: Simultaneous Leader and Listener Failure ---"
LEADER=$(get_leader_pod)
LISTENER_POD=$(kubectl get pods -n $NAMESPACE -l app=twitch-listener -o jsonpath='{.items[0].metadata.name}')

echo "Targets: Leader=$LEADER, Listener=$LISTENER_POD"
BEFORE_COUNT=$(count_assignments)

echo "Deleting both pods simultaneously..."
kubectl delete pod -n $NAMESPACE $LEADER $LISTENER_POD --grace-period=0 --force

echo "Waiting for recovery (60s)..."
sleep 60

AFTER_COUNT=$(count_assignments)
echo "Assignments: $BEFORE_COUNT → $AFTER_COUNT"

if [ "$BEFORE_COUNT" -eq "$AFTER_COUNT" ]; then
    echo "✓ Scenario 4 PASSED: System recovered from cascading failures"
else
    echo "✗ Scenario 4 FAILED: Assignment count mismatch"
    exit 1
fi
echo ""

# Scenario 5: Redis Latency
echo "--- Scenario 5: Redis Latency Spike (Requires tc tool) ---"
echo "This scenario requires tc (traffic control) tool in Redis pod."
echo "See docs/testing/chaos-testing-phase5.md for full procedure."
echo "Skipping automated execution."
echo ""

echo "=== Chaos Testing Summary ==="
echo "✓ Scenario 1: Leader Pod Failure - PASSED"
echo "○ Scenario 2: Network Partition - Manual"
echo "✓ Scenario 3: Listener Pod Failure - COMPLETED"
echo "✓ Scenario 4: Simultaneous Failures - PASSED"
echo "○ Scenario 5: Redis Latency - Manual"
echo ""
echo "Run manual scenarios from docs/testing/chaos-testing-phase5.md"
