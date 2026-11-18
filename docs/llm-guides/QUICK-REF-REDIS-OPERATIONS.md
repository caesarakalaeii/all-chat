# Quick Reference: Redis Operations

Goal: inspect and debug Redis Streams, Pub/Sub and cache state.

Access: `redis-cli` locally, or `kubectl exec -n allchat deployment/redis -- redis-cli` in Kubernetes.

## Streams (`chat:raw`)

```bash
XINFO STREAM chat:raw              # length, groups
XINFO GROUPS chat:raw              # consumer groups: message-processor, alert-processors
XPENDING chat:raw message-processor            # lag: pending count per group
XPENDING chat:raw message-processor - + 10     # individual pending messages
XINFO CONSUMERS chat:raw message-processor     # per-consumer pending + idle time
XAUTOCLAIM chat:raw message-processor <free-consumer> 300000 0-0 COUNT 100   # claim idle messages
```

Consumer lag above ~10,000 (the alert threshold in the message-processor README) means the processors are behind: check consumer idle times, then scale message-processor/consumer code at `services/message-processor/`, `services/alert-processor/consumer/consumer.go`.

## Pub/Sub (`overlay:{overlay_id}`)

Per-overlay channels after processing:

```bash
PUBSUB CHANNELS overlay:*
PUBSUB NUMSUB overlay:<id>
SUBSCRIBE overlay:<id>    # watch live; Ctrl+C to stop
```

API-Gateway presence keys: `overlay:connected:{id}`.

## Cache

Emote cache keys are `emote:*` with TTLs. `KEYS emote:*` is fine for spot checks but use `SCAN` on a cache big enough to matter. `redis-cli --bigkeys` finds large keys; `INFO memory` shows usage; `SLOWLOG GET 10` shows slow commands.

## Common issues

- Stream filling quickly: publishers cap the stream length (`MAXLEN ~`); if it outgrows the cap, scale message-processors rather than raising the cap blindly.
- High memory: `redis-cli --bigkeys`, `MEMORY DOCTOR`.
- Lag stuck: claim idle pending messages with `XAUTOCLAIM` (see above).

References: [docs/adr/0002-redis-streams-pubsub.md](../adr/0002-redis-streams-pubsub.md), [docs/architecture/01-DATA-FLOW.md](../architecture/01-DATA-FLOW.md).
