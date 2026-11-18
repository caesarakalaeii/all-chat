# Chaos testing: shard coordinator resilience

Factual note: `scripts/chaos-test-phase5.sh` targets a `redis-0` pod; the manifests deploy Redis as a Deployment, so set `REDIS_POD` accordingly or use `kubectl exec -n allchat deploy/redis --` instead.

Validates that the source-manager shard coordinator survives failure modes: leader pod loss, network partition, listener pod loss, simultaneous failure, Redis latency. Scenarios 1, 3 and 4 are automated by `scripts/chaos-test-phase5.sh`:

```bash
./scripts/chaos-test-phase5.sh
```

Prerequisites: source-manager with 2+ replicas, twitch-listener running, Redis and PostgreSQL available, kubectl admin access. Commands use `kubectl exec -n allchat deploy/redis --` to reach Redis.

The coordinator's contract: leader election on a Kubernetes Lease (new leader within 30s), failed-listener detection via heartbeats (15s timeout), redistribution within 60s, bounded load (no pod above 1.25x the average), and version-counter fencing so a stale leader cannot write assignments.

## Scenario 1: leader pod failure

```bash
LEADER=$(kubectl get pods -n allchat -l app=source-manager -o json \
  | jq -r '.items[] | select(.metadata.annotations.leader=="true") | .metadata.name')
kubectl delete pod -n allchat $LEADER --grace-period=0 --force
```

Pass when: a new leader logs "Started leading" within 30s; the assignment count recovers within 60s; each source has exactly one assignment:

```bash
kubectl logs -n allchat -l app=source-manager --tail=50 | grep "Started leading"
kubectl exec -n allchat deploy/redis -- redis-cli KEYS "shard:assignment:*" | wc -l
```

## Scenario 2: network partition (manual)

Isolate the leader from the Kubernetes API with iptables from inside its pod (`iptables -A OUTPUT -d <API_IP> -j DROP`), then note its `<API_IP>` first. Pass when: the lease expires within ~30s, a healthy replica takes over, the stale leader logs "Lost leadership" and its writes are rejected by fencing. Remove the iptables rule afterwards.

## Scenario 3: listener pod failure

```bash
POD=$(kubectl get pods -n allchat -l app=twitch-listener -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n allchat deploy/redis -- redis-cli ZRANGE shard:load 0 -1 WITHSCORES
kubectl delete pod -n allchat $POD --grace-period=0 --force
sleep 60
kubectl exec -n allchat deploy/redis -- redis-cli ZRANGE shard:load 0 -1 WITHSCORES
```

Pass when: the failed pod's sources are redistributed within 60s of deletion, and no remaining pod holds more than 1.25x the average load. Metrics: `shard_failed_pods` +1, `shard_healthy_pods` -1, `shard_imbalance_ratio` below 1.25.

## Scenario 4: simultaneous leader and listener failure

Delete one source-manager and one twitch-listener pod in a single command; pass is the same as scenarios 1 and 3 combined: new leader within 30s, redistribution within 60s, and no assignment gaps.

## Scenario 5: Redis latency spike (manual)

Inside the Redis pod, inject latency (`tc qdisc add dev eth0 root netem delay 2000ms`), then check that no healthy pod gets marked failed (heartbeats retry three times with 2s timeouts, inside the 15s detection window) and that `shard_failed_pods` does not spike. Remove the qdisc afterwards.

## Observing

```bash
kubectl exec -n allchat deploy/redis -- redis-cli KEYS "shard:assignment:*"
kubectl exec -n allchat deploy/redis -- redis-cli ZRANGE shard:heartbeats 0 -1 WITHSCORES
kubectl logs -n allchat -l app=source-manager --tail=50 | grep -E "Started leading|Lost leadership"
curl -s http://source-manager:8088/metrics | grep shard_
```

Coordinator design: [ADR-0007](../adr/0007-leadership-rebalancing.md) (leadership rebalancing).
