# Phase 3: Kick Integration + Edge Cases - Research

**Researched:** 2026-02-18
**Domain:** Kick WebSocket deletion events, WebSocket reconnection with deletion replay buffer, batch deletion performance optimization
**Confidence:** MEDIUM-HIGH

## Summary

Phase 3 extends deletion support to Kick platform by implementing `ChatMessageDeletedEvent` WebSocket handler and adds reconnection resilience via Redis-backed deletion event replay buffer. Research confirms that Kick's Pusher WebSocket provides real-time deletion events (verified via third-party library documentation and WebSocket event examples), requiring handler addition to existing client.go event switch. The critical architectural addition is a 1-minute TTL replay buffer persisting deletion events in Redis to handle WebSocket reconnections, addressing the fundamental limitation that Redis Pub/Sub is ephemeral and cannot replay missed events.

**Primary finding:** Phase 1 and 2 infrastructure provides 90% of needed functionality. Kick integration requires single event handler (similar to existing `kickChatMessageEvent` pattern). Reconnection replay is the novel component, requiring Redis-backed persistence since Pub/Sub messages evaporate on disconnect. Load testing for 1,000+ batch deletions validates React 18's automatic batching prevents UI blocking.

**Critical architectural insight:** WebSocket reconnection creates 2-layer problem: (1) API Gateway ↔ Frontend WebSocket can disconnect, losing Pub/Sub messages; (2) Kick Pusher WebSocket can disconnect (already handled by client.go reconnection logic). Solution is deletion-specific replay buffer (separate from Phase 1's race condition buffer) that persists events for 60 seconds, enabling frontend to request replay on reconnect.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| KICK-01 | Listener detects ChatMessageDeletedEvent via WebSocket | Kick WebSocket emits `ChatMessageDeletedEvent` with structure: `{"data":{"deletedMessage":{"id":"msg-uuid","deleted_by":user_id,"chatroom_id":"4910"}}}`. Add handler to client.go switch statement (line 618) alongside existing `kickChatMessageEvent` |
| KICK-02 | Kick event structure validated in production environment | Event structure documented in third-party libraries (KickLib C# .NET, kick_live_ws npm). Format consistent: event name contains "ChatMessageDeleted", data contains deletedMessage with id field. **MEDIUM confidence** - not official Kick documentation, but multiple independent implementations agree |
| KICK-03 | Kick deletion events include message ID for matching | deletedMessage.id field contains UUID of deleted message (Kick's platform message ID). Maps to registry key via `msgid:registry:kick:{channel_id}` hash |
| REL-01 | Deletion events persisted for 1-minute replay window on reconnect | Redis sorted set (ZADD with timestamp score) stores deletion events per overlay. Key: `replay:deletions:{overlay_id}`. TTL: 60 seconds. Frontend reconnect sends last_seen timestamp, gateway replies with ZRANGEBYSCORE |
| REL-02 | WebSocket reconnection triggers deletion event replay | Frontend tracks `lastSeenTimestamp` in localStorage. On reconnect, sends `{"type":"replay_request","since":timestamp}` message. Gateway queries replay buffer, sends missed deletions as batch |
| REL-03 | System handles Redis Pub/Sub message loss gracefully | Pub/Sub is best-effort (acknowledged in design). Replay buffer mitigates loss during disconnect. Permanent subscribers stay connected → no loss. Temporary disconnects covered by 60s buffer. Edge case: >60s disconnect = message loss acceptable per requirements |
| REL-04 | Load testing validates batch deletion performance (1,000+ messages) | Use Artillery or Apache JMeter (both support WebSocket). Simulate 1,000 message send → user ban → measure frontend render time. React 18 auto-batching handles batch via single `setMessages()` call. Target: <100ms render time |
| REL-05 | DOM update optimization prevents UI blocking during large deletions | React 18 automatic batching groups state updates. Single `setMessages(prev => prev.filter(m => m.user.id !== bannedUserId))` triggers ONE render. Verified in Phase 1 (01-VERIFICATION.md). Key optimization: use user ID filter (not message ID array iteration) |

**Confidence Assessment:**
- KICK-01, KICK-03: MEDIUM-HIGH (event structure from multiple third-party sources, no official docs)
- KICK-02: MEDIUM (production validation needed to confirm exact event names/structure)
- REL-01, REL-02, REL-05: HIGH (Redis patterns well-established, React 18 batching verified in Phase 1)
- REL-03: HIGH (Pub/Sub limitations documented, design explicitly accepts best-effort)
- REL-04: MEDIUM (load testing tools confirmed, but performance target needs validation)

</phase_requirements>

## Standard Stack

### Core (Already in Use)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| gorilla/websocket | v1.5.x | Kick Pusher WebSocket client | Already used in kick-listener/websocket/client.go; handles reconnection |
| go-redis/v9 | v9.x | Replay buffer persistence (sorted sets) | Phase 1 registry uses hashes; extend with ZADD/ZRANGEBYSCORE for replay |
| React | 18+ | Frontend state management with automatic batching | Phase 1 verified auto-batching prevents UI blocking; no changes needed |
| Artillery | 2.x | WebSocket load testing | Industry-standard open-source tool with WebSocket support; alternative: Apache JMeter |

### Supporting (Potential Additions)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| Apache JMeter | 5.6+ | Alternative load testing tool | If Artillery insufficient; provides GUI and robust WebSocket sampler |
| k6 (Grafana) | v0.50+ | Modern load testing with WebSocket | If scripting flexibility needed; uses JavaScript for test definitions |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Redis sorted set for replay buffer | Redis Streams (XADD with MAXLEN) | Streams provide message ID tracking, but sorted sets simpler for timestamp-based range queries (ZRANGEBYSCORE). Streams add complexity for minimal benefit in 60s window |
| Artillery for load testing | WebSocket stress-testing with custom Go tool | Custom tool allows precise control but requires development. Artillery provides battle-tested scenarios out-of-box |
| 1-minute replay window | 5-minute or persistent storage | Longer windows increase memory usage (more deletion events retained). 1 minute balances reconnection tolerance with memory footprint. Permanent storage unnecessary (messages ephemeral) |

**Installation:**
```bash
# Artillery (for load testing only, not production dependency)
npm install -g artillery@latest

# No new Go dependencies required - Redis and WebSocket libraries already present
```

## Architecture Patterns

### Recommended Project Structure

Extend existing services, no new service needed:

```
services/kick-listener/
├── websocket/
│   ├── client.go           # MODIFY: Add ChatMessageDeletedEvent handler (line ~620)
│   └── types.go            # ADD: KickMessageDeletedEvent struct
└── cmd/main.go             # MODIFY: Wire deletion event to message handler

services/api-gateway/
├── replay/
│   ├── buffer.go           # NEW: DeletionReplayBuffer interface + Redis impl
│   └── buffer_test.go      # NEW: Unit tests with miniredis
├── handlers/
│   └── websocket.go        # MODIFY: Add replay_request message handler
└── websocket/
    └── connection.go       # MODIFY: Track last_seen timestamp per connection

frontend/src/lib/api/
└── websocket.ts            # MODIFY: Send replay_request on reconnect
```

### Pattern 1: Kick Deletion Event Handler (NEW)

**What:** Extend Kick WebSocket client to handle `ChatMessageDeletedEvent` alongside existing `ChatMessageSentEvent`

**When to use:** Required for KICK-01 - detecting deletion events from Kick platform

**Example:**
```go
// Source: Research recommendation based on existing kick-listener/websocket/client.go pattern
// Extends handleMessage switch statement

// In websocket/types.go - ADD new type
type KickMessageDeletedEvent struct {
	DeletedMessage struct {
		ID          string `json:"id"`           // Kick platform message ID
		DeletedBy   int    `json:"deleted_by"`   // User ID who deleted message
		ChatroomID  string `json:"chatroom_id"`  // Chatroom ID
	} `json:"deletedMessage"`
}

// In websocket/client.go - MODIFY handleMessage() around line 618
const (
	kickChatMessageEvent        = "App\\Events\\ChatMessageSentEvent"
	kickMessageDeletedEvent     = "App\\Events\\ChatMessageDeletedEvent"  // NEW
)

func (c *Client) handleMessage(data []byte) {
	var msg PusherMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		c.logger.Error("Failed to unmarshal Pusher message", zap.Error(err))
		return
	}

	switch msg.Event {
	case kickChatMessageEvent:
		c.handleChatMessage(msg.Channel, msg.Data)

	case kickMessageDeletedEvent:  // NEW
		c.handleMessageDeleted(msg.Channel, msg.Data)

	default:
		c.logger.Debug("Unhandled Pusher event", zap.String("event", msg.Event))
	}
}

func (c *Client) handleMessageDeleted(channel string, data json.RawMessage) {
	var deletedEvent KickMessageDeletedEvent
	if err := json.Unmarshal(data, &deletedEvent); err != nil {
		c.logger.Error("Failed to unmarshal deletion event", zap.Error(err))
		return
	}

	c.logger.Debug("Received message deletion",
		zap.String("channel", channel),
		zap.String("deleted_message_id", deletedEvent.DeletedMessage.ID),
		zap.Int("deleted_by", deletedEvent.DeletedMessage.DeletedBy),
	)

	// Call deletion handler (similar to chat message handler)
	if c.deletionHandler != nil {
		c.deletionHandler(channel, &deletedEvent)
	}
}
```

**Why this works:** Follows existing pattern for `kickChatMessageEvent`. Pusher WebSocket sends both events through same channel subscription. No additional WebSocket connection needed.

### Pattern 2: Deletion Replay Buffer (NEW)

**What:** Redis-backed buffer storing deletion events for 60 seconds to enable replay after WebSocket reconnection

**When to use:** Required for REL-01, REL-02 - handling message loss during disconnect

**Structure:**
```
Key: replay:deletions:{overlay_id}
Type: Redis Sorted Set (ZADD/ZRANGEBYSCORE)
Score: Unix timestamp (milliseconds)
Member: JSON-encoded deletion event

Example:
  ZADD replay:deletions:550e8400-e29b-41d4-a716-446655440000
       1708281660123 '{"type":"single","target_uuid":"abc-123","platform":"kick","timestamp":"2026-02-18T10:01:00.123Z"}'

TTL: 60 seconds (EXPIRE at key level)
```

**Go Implementation:**
```go
// Source: Research recommendation based on Redis sorted set patterns
package replay

type DeletionReplayBuffer interface {
	Add(ctx context.Context, overlayID string, deletion *DeletionEvent) error
	GetSince(ctx context.Context, overlayID string, sinceTimestamp int64) ([]*DeletionEvent, error)
	Prune(ctx context.Context, overlayID string, olderThan int64) error
}

type DeletionEvent struct {
	Type         string    `json:"type"`          // "single", "batch", "clear"
	TargetUUID   string    `json:"target_uuid,omitempty"`
	TargetUserID string    `json:"target_user_id,omitempty"`
	Platform     string    `json:"platform"`
	Timestamp    time.Time `json:"timestamp"`
}

type RedisDeletionReplayBuffer struct {
	client *redis.Client
	ttl    time.Duration // 60 seconds
}

func NewRedisDeletionReplayBuffer(client *redis.Client, ttl time.Duration) *RedisDeletionReplayBuffer {
	return &RedisDeletionReplayBuffer{
		client: client,
		ttl:    ttl,
	}
}

// Add stores deletion event with timestamp score
func (b *RedisDeletionReplayBuffer) Add(ctx context.Context, overlayID string, deletion *DeletionEvent) error {
	key := fmt.Sprintf("replay:deletions:%s", overlayID)

	// Serialize to JSON
	data, err := json.Marshal(deletion)
	if err != nil {
		return fmt.Errorf("failed to marshal deletion event: %w", err)
	}

	// Use millisecond timestamp as score for precise ordering
	score := float64(deletion.Timestamp.UnixMilli())

	// ZADD + EXPIRE in pipeline
	pipe := b.client.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{
		Score:  score,
		Member: string(data),
	})
	pipe.Expire(ctx, key, b.ttl) // Refresh TTL on each add
	_, err = pipe.Exec(ctx)

	return err
}

// GetSince retrieves all deletion events after given timestamp
func (b *RedisDeletionReplayBuffer) GetSince(ctx context.Context, overlayID string, sinceTimestamp int64) ([]*DeletionEvent, error) {
	key := fmt.Sprintf("replay:deletions:%s", overlayID)

	// Query range: (sinceTimestamp, +inf) - exclusive lower bound
	results, err := b.client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min: fmt.Sprintf("(%d", sinceTimestamp), // Exclusive
		Max: "+inf",
	}).Result()

	if err != nil {
		return nil, fmt.Errorf("failed to query replay buffer: %w", err)
	}

	// Deserialize all events
	events := make([]*DeletionEvent, 0, len(results))
	for _, data := range results {
		var event DeletionEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			// Log error but continue processing other events
			continue
		}
		events = append(events, &event)
	}

	return events, nil
}

// Prune removes events older than threshold (automatic via TTL, but useful for cleanup)
func (b *RedisDeletionReplayBuffer) Prune(ctx context.Context, overlayID string, olderThan int64) error {
	key := fmt.Sprintf("replay:deletions:%s", overlayID)
	return b.client.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%d", olderThan)).Err()
}
```

**Why sorted set over alternatives:**
- ZRANGEBYSCORE provides efficient timestamp-based range queries (O(log(N)+M) where M = results)
- Automatic ordering by timestamp (no manual sorting needed)
- TTL at key level (simpler than per-member expiration)
- Alternative (Redis Streams) adds complexity: XADD/XREAD with consumer groups overkill for 60s window

### Pattern 3: WebSocket Reconnection with Replay Request (NEW)

**What:** Frontend tracks last seen timestamp, requests missed deletions on reconnect

**Frontend (TypeScript):**
```typescript
// Source: Research recommendation extending frontend/src/lib/api/websocket.ts
// MODIFY WebSocketClient class

export class WebSocketClient {
  private lastSeenTimestamp: number = 0;

  connect(overlayId: string, token: string) {
    const url = `${WS_URL}/ws/overlay/${overlayId}?token=${token}`;
    this.ws = new WebSocket(url);

    this.ws.onopen = () => {
      console.log('[WebSocket] Connected');
      this.reconnectAttempts = 0;

      // NEW: Request replay of missed deletion events
      if (this.lastSeenTimestamp > 0) {
        const replayRequest = {
          type: 'replay_request',
          since: this.lastSeenTimestamp,
        };
        this.ws?.send(JSON.stringify(replayRequest));
        console.log('[WebSocket] Requested replay since:', new Date(this.lastSeenTimestamp));
      }
    };

    this.ws.onmessage = (event) => {
      const wsMessage: WebSocketMessage = JSON.parse(event.data);

      // NEW: Handle replay response
      if (wsMessage.type === 'replay_response') {
        const deletions = wsMessage.data as DeletionEvent[];
        console.log(`[WebSocket] Replaying ${deletions.length} missed deletions`);
        deletions.forEach(deletion => this.processDeletion(deletion));
        return;
      }

      if (wsMessage.type === 'chat_message') {
        // Update last seen timestamp for deletion events
        if (wsMessage.data.event?.type === 'message_deletion') {
          this.lastSeenTimestamp = Date.now();
          this.processDeletion(wsMessage.data.event.metadata);
        } else {
          // Regular message
          this.messageCallbacks.forEach(cb => cb(wsMessage.data));
        }
      }
    };
  }

  private processDeletion(deletion: DeletionMetadata) {
    // Apply deletion to message list (existing logic from Phase 1)
    // This method called for both real-time AND replayed deletions
  }
}
```

**Backend (Go):**
```go
// Source: Research recommendation for api-gateway/handlers/websocket.go
// MODIFY handleMessage to process replay_request

func (c *Connection) handleMessage(data []byte) {
	var msg models.WSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		c.logger.Warn("Failed to parse WebSocket message", zap.Error(err))
		return
	}

	switch msg.Type {
	case models.WSMessageTypePong:
		// Existing pong handler
		c.logger.Debug("Received pong", zap.String("overlay_id", c.overlayID))

	case "replay_request":  // NEW
		c.handleReplayRequest(msg.Data)

	default:
		c.logger.Debug("Unhandled message type", zap.String("type", msg.Type))
	}
}

func (c *Connection) handleReplayRequest(data json.RawMessage) {
	var request struct {
		Since int64 `json:"since"` // Unix milliseconds
	}
	if err := json.Unmarshal(data, &request); err != nil {
		c.logger.Warn("Invalid replay request", zap.Error(err))
		return
	}

	// Query replay buffer
	deletions, err := c.replayBuffer.GetSince(context.Background(), c.overlayID, request.Since)
	if err != nil {
		c.logger.Error("Failed to retrieve replay buffer",
			zap.String("overlay_id", c.overlayID),
			zap.Error(err),
		)
		return
	}

	if len(deletions) == 0 {
		c.logger.Debug("No missed deletions to replay",
			zap.String("overlay_id", c.overlayID),
			zap.Int64("since", request.Since),
		)
		return
	}

	// Send replay response
	response := models.WSMessage{
		Type: "replay_response",
		Data: deletions,
	}
	responseJSON, _ := json.Marshal(response)
	c.Send(responseJSON)

	c.logger.Info("Replayed missed deletions",
		zap.String("overlay_id", c.overlayID),
		zap.Int("count", len(deletions)),
		zap.Int64("since", request.Since),
	)
}
```

**Why this pattern:**
- Frontend controls replay (not automatic broadcast) → prevents duplicate delivery
- Timestamp-based range query (not last message ID) → simpler for sorted set
- 60-second window balances memory vs reconnection tolerance
- Graceful degradation: >60s disconnect = no replay (acceptable per requirements)

### Pattern 4: Batch Deletion Performance Validation (LOAD TESTING)

**What:** Artillery script simulating 1,000 message batch deletion to measure React rendering performance

**When to use:** Required for REL-04 - validating batch deletion performance

**Artillery Load Test Config:**
```yaml
# Source: Research recommendation using Artillery WebSocket support
# File: tests/load/batch-deletion.yml

config:
  target: "ws://localhost:8080"
  phases:
    - duration: 60
      arrivalRate: 1
      name: "Batch deletion load test"
  processor: "./batch-deletion-scenario.js"

scenarios:
  - name: "Send 1000 messages then ban user"
    engine: ws
    flow:
      # Connect to WebSocket
      - connect:
          url: "/ws/overlay/{{ overlayId }}?token={{ token }}"

      # Wait for connection
      - think: 1

      # Send 1000 messages via API (simulated via HTTP POST)
      - loop:
        - post:
            url: "http://localhost:8080/api/test/send-message"
            json:
              overlay_id: "{{ overlayId }}"
              user_id: "test-user-{{ $uuid }}"
              message: "Test message {{ $loopCount }}"
        count: 1000

      # Wait for messages to arrive via WebSocket
      - think: 5

      # Trigger user ban (batch deletion)
      - post:
          url: "http://localhost:8080/api/test/ban-user"
          json:
            overlay_id: "{{ overlayId }}"
            user_id: "test-user-{{ $uuid }}"

      # Wait for deletion event to propagate
      - think: 2

      # Measure: Frontend should process batch deletion in <100ms
      # (Artillery captures response times automatically)
```

**Alternative: Apache JMeter WebSocket Sampler:**
```xml
<!-- Source: Research finding on JMeter WebSocket support -->
<!-- JMeter provides GUI for test creation, more verbose than Artillery -->
<TestPlan>
  <ThreadGroup num_threads="10" ramp_time="5">
    <WebSocketSampler>
      <url>ws://localhost:8080/ws/overlay/${overlayId}?token=${token}</url>
      <message_pattern>{"type":"chat_message",...}</message_pattern>
      <response_pattern>.*message_deletion.*</response_pattern>
    </WebSocketSampler>
  </ThreadGroup>
</TestPlan>
```

**Performance Target Validation:**
```typescript
// Source: React 18 automatic batching (verified in Phase 1)
// Single state update for batch deletion

setMessages((prev) => {
  // React 18 batches this into ONE render, even for 1,000 removals
  return prev.filter(m => m.user.id !== bannedUserId);
});

// Expected: <100ms for 1,000 message DOM updates
// React reconciliation: O(n) where n = messages
// Modern browsers: 60 FPS = 16.67ms frame budget
// 1,000 messages * ~0.1ms per element check = ~100ms total
```

**Why load testing required:**
- Validates React 18 batching works at scale (Phase 1 tested with small batches)
- Identifies browser rendering bottlenecks (layout thrashing, reflow)
- Confirms 1,000+ message deletion target is achievable
- Artillery/JMeter provide production-grade WebSocket simulation

### Anti-Patterns to Avoid

- **Replaying ALL deletion events on reconnect:** Frontend should request "since timestamp" not "replay everything". Sorted set supports efficient range queries.
- **Persisting replay buffer permanently:** 60-second TTL sufficient; longer retention wastes memory for ephemeral chat messages.
- **Broadcasting replay to all connections:** Replay should be request-based (frontend asks), not automatic broadcast (causes duplicates).
- **Using message ID array for batch deletions:** Filter by user_id (single field check) not message ID list (1,000 comparisons).
- **Manual React batching in React 18+:** `ReactDOM.unstable_batchedUpdates()` not needed; automatic batching handles it.
- **Separate WebSocket connection for replay:** Reuse existing overlay WebSocket connection; replay is message type, not separate channel.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| WebSocket reconnection logic | Custom exponential backoff | Existing frontend/src/lib/api/websocket.ts reconnection | Already implemented with exponential backoff (line 96-104); tested in production |
| Kick WebSocket protocol | Custom Pusher client | Existing gorilla/websocket client in kick-listener | Already handles Pusher protocol 7, reconnection, channel subscriptions |
| Load testing framework | Custom WebSocket stress tool | Artillery or Apache JMeter | Battle-tested tools with WebSocket support, scenario scripting, metrics collection |
| Timestamp-based replay | Custom last message ID tracking | Redis sorted set ZRANGEBYSCORE | Built-in range queries with O(log(N)+M) complexity; handles ordering automatically |
| React batch rendering optimization | Manual requestAnimationFrame | React 18 automatic batching | React 18 batches state updates automatically; no manual optimization needed (verified in Phase 1) |

**Key insight:** Infrastructure from Phase 1 (registry, deletion buffer) and Phase 2 (YouTube integration) provides templates. Kick requires event handler (20 lines), replay buffer (new component but standard Redis pattern), and load testing validation (Artillery script).

## Common Pitfalls

### Pitfall 1: Replay Buffer Memory Growth

**What goes wrong:** Replay buffer grows unbounded as deletion events accumulate across all overlays, causing Redis memory exhaustion.

**Why it happens:** Forgetting to set TTL on replay buffer keys, or setting TTL too long (e.g., 1 hour instead of 60 seconds).

**How to avoid:**
- Set 60-second TTL on every ZADD operation (use pipeline: ZADD + EXPIRE)
- Monitor Redis memory usage: `redis-cli INFO memory | grep used_memory_human`
- Implement automatic pruning: periodically ZREMRANGEBYSCORE to remove entries older than 60s (backup to TTL)

**Warning signs:**
- Redis `used_memory` grows continuously without plateau
- `ZCARD replay:deletions:*` shows increasing set sizes over time
- Slower ZRANGEBYSCORE queries (O(log(N)+M) degrades as N grows)

**Example fix:**
```go
// Always refresh TTL on add
pipe := b.client.Pipeline()
pipe.ZAdd(ctx, key, redis.Z{Score: score, Member: data})
pipe.Expire(ctx, key, 60*time.Second) // Reset to 60s on every add
_, err := pipe.Exec(ctx)
```

### Pitfall 2: Duplicate Deletion Events on Reconnect

**What goes wrong:** Frontend receives same deletion event twice (once real-time, once in replay), causing state inconsistencies or errors (filter returns no messages when already removed).

**Why it happens:** Timestamp tracking uses `Date.now()` before deletion received, so "since" timestamp includes current deletions.

**How to avoid:**
- Update `lastSeenTimestamp` AFTER processing deletion (not before)
- Use exclusive range query in ZRANGEBYSCORE: `(timestamp` not `timestamp` (parenthesis = exclusive)
- Frontend should handle duplicate deletions gracefully (filter returns same array if already removed)

**Example fix:**
```typescript
// Update timestamp AFTER processing, not before
ws.onmessage = (event) => {
  const wsMessage = JSON.parse(event.data);

  if (wsMessage.data.event?.type === 'message_deletion') {
    this.processDeletion(wsMessage.data.event.metadata); // Process first
    this.lastSeenTimestamp = Date.now(); // Update after
  }
};

// Backend: use exclusive range query
Min: fmt.Sprintf("(%d", sinceTimestamp), // Parenthesis = exclusive lower bound
```

**Warning signs:**
- Frontend console shows duplicate deletion logs
- Message state becomes inconsistent (messages flicker on reconnect)
- Replay returns events user already processed

### Pitfall 3: Kick Event Name Mismatch

**What goes wrong:** Kick WebSocket client handler never triggers because event name doesn't match actual Kick API event name.

**Why it happens:** Unofficial Kick API → event names not documented. Third-party libraries show `ChatMessageDeletedEvent` but Kick may use different capitalization, namespace, or format.

**How to avoid:**
- Log ALL unhandled Pusher events during development: `logger.Info("Unhandled event", zap.String("event", msg.Event))`
- Monitor Kick WebSocket traffic via browser DevTools Network tab (filter by WS)
- Start with broad event name match: `strings.Contains(msg.Event, "MessageDeleted")` before narrowing
- Fallback: handle event in `default` case initially, inspect structure, then create specific handler

**Example defensive pattern:**
```go
switch msg.Event {
case "App\\Events\\ChatMessageDeletedEvent":
	// Exact match (preferred)
	c.handleMessageDeleted(msg.Channel, msg.Data)

default:
	// Log everything for debugging
	c.logger.Info("Unhandled Pusher event",
		zap.String("event", msg.Event),
		zap.ByteString("data_preview", msg.Data[:min(100, len(msg.Data))]),
	)

	// Fallback: try to parse as deletion event
	if strings.Contains(msg.Event, "MessageDeleted") || strings.Contains(msg.Event, "message.deleted") {
		c.logger.Warn("Possible deletion event with unexpected name", zap.String("event", msg.Event))
		// Attempt to parse and handle
	}
}
```

**Warning signs:**
- Kick chat messages arrive but deletions don't trigger
- Unhandled event logs show deletion-related event names
- Third-party library documentation conflicts with observed events

### Pitfall 4: Load Test Measures Network Latency, Not UI Render Time

**What goes wrong:** Load test reports "200ms deletion event delivery" but doesn't measure actual React rendering time, missing UI blocking issues.

**Why it happens:** Artillery measures WebSocket message round-trip time, not browser JavaScript execution time.

**How to avoid:**
- Use browser Performance API to measure render time: `performance.mark()` before/after `setMessages()`
- Add custom Artillery processor (batch-deletion-scenario.js) that injects browser-side timing code
- Alternative: Use Lighthouse or Chrome DevTools Performance tab for profiling during manual test
- Supplement Artillery with Playwright/Puppeteer for browser-based performance testing

**Example browser-side measurement:**
```typescript
// In WebSocket onmessage handler
if (deletion.deletion_type === 'batch') {
  performance.mark('deletion-start');

  setMessages(prev => prev.filter(m => m.user.id !== deletion.target_user_id));

  // React 18 schedules render; measure after next paint
  requestAnimationFrame(() => {
    performance.mark('deletion-end');
    const measure = performance.measure('batch-deletion', 'deletion-start', 'deletion-end');
    console.log(`Batch deletion render time: ${measure.duration.toFixed(2)}ms`);

    if (measure.duration > 100) {
      console.warn('UI blocking detected: batch deletion took >100ms');
    }
  });
}
```

**Warning signs:**
- Load test shows fast WebSocket delivery but users report UI freezing
- Browser DevTools shows "Long Task" warnings during batch deletions
- React DevTools Profiler shows >100ms render times

### Pitfall 5: Replay Buffer Not Wired to Message Publisher

**What goes wrong:** Deletion events published to Pub/Sub (frontend receives in real-time) but NOT added to replay buffer, so reconnect replay is empty.

**Why it happens:** Forgetting to add replay buffer integration to Message Processor's deletion event publisher.

**How to avoid:**
- Add replay buffer to API Gateway's Pub/Sub publisher (not Message Processor)
- Wire replay buffer in parallel with Pub/Sub publish: `publishToOverlay()` AND `replayBuffer.Add()`
- Add integration test: publish deletion → disconnect → reconnect → verify replay includes event
- Monitor `replay:deletions:*` keys in Redis during development: `redis-cli KEYS "replay:deletions:*"`

**Example integration point:**
```go
// In api-gateway/subscription/subscriber.go (or similar publisher)
func (p *Publisher) PublishDeletion(ctx context.Context, overlayID string, deletion *models.DeletionEvent) error {
	// Existing: publish to Redis Pub/Sub
	if err := p.pubSubClient.Publish(ctx, fmt.Sprintf("overlay:%s", overlayID), deletion); err != nil {
		return err
	}

	// NEW: also add to replay buffer
	if err := p.replayBuffer.Add(ctx, overlayID, deletion); err != nil {
		p.logger.Error("Failed to add deletion to replay buffer",
			zap.String("overlay_id", overlayID),
			zap.Error(err),
		)
		// Don't fail publish if replay buffer fails (best-effort)
	}

	return nil
}
```

**Warning signs:**
- Real-time deletions work but replay returns empty array
- Redis key `replay:deletions:{overlay_id}` doesn't exist or is empty
- Integration tests show replay buffer is populated but production doesn't

## Code Examples

Verified patterns from official sources and existing codebase:

### Kick WebSocket Deletion Event Handler

```go
// Source: Existing pattern from services/kick-listener/websocket/client.go
// Extends handleMessage switch statement (line 618)

const (
	pusherConnectionEstablished = "pusher:connection_established"
	pusherPing                  = "pusher:ping"
	pusherPong                  = "pusher:pong"
	pusherError                 = "pusher:error"
	kickChatMessageEvent        = "App\\Events\\ChatMessageSentEvent"
	kickMessageDeletedEvent     = "App\\Events\\ChatMessageDeletedEvent" // NEW
)

// In handleMessage()
func (c *Client) handleMessage(data []byte) {
	var msg PusherMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		c.logger.Error("Failed to unmarshal Pusher message", zap.Error(err))
		return
	}

	c.logger.Debug("Received Pusher message",
		zap.String("event", msg.Event),
		zap.String("channel", msg.Channel),
	)

	switch msg.Event {
	case pusherConnectionEstablished:
		c.handleConnectionEstablished(msg.Data)

	case pusherPong:
		c.logger.Debug("Received pong from Pusher")

	case pusherPing:
		c.logger.Debug("Received ping from Pusher")
		if err := c.sendControlEvent(pusherPong); err != nil {
			c.logger.Error("Failed to send pong response", zap.Error(err))
		}

	case kickChatMessageEvent:
		c.handleChatMessage(msg.Channel, msg.Data)

	case kickMessageDeletedEvent: // NEW
		c.handleMessageDeleted(msg.Channel, msg.Data)

	default:
		c.logger.Debug("Unhandled Pusher event", zap.String("event", msg.Event))
	}
}

// NEW handler method
func (c *Client) handleMessageDeleted(channel string, data json.RawMessage) {
	var deletedEvent KickMessageDeletedEvent
	if err := json.Unmarshal(data, &deletedEvent); err != nil {
		c.logger.Error("Failed to unmarshal deletion event", zap.Error(err))
		return
	}

	c.logger.Debug("Received message deletion",
		zap.String("channel", channel),
		zap.String("deleted_message_id", deletedEvent.DeletedMessage.ID),
		zap.Int("deleted_by", deletedEvent.DeletedMessage.DeletedBy),
	)

	// Call deletion handler (wire to main.go message handler)
	if c.deletionHandler != nil {
		c.deletionHandler(channel, &deletedEvent)
	}
}
```

### Redis Sorted Set Replay Buffer Operations

```go
// Source: Redis sorted sets documentation (https://redis.io/docs/latest/develop/data-types/sorted-sets/)
// Official go-redis examples

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"github.com/redis/go-redis/v9"
)

// Add deletion event to replay buffer with timestamp score
func AddToReplayBuffer(ctx context.Context, client *redis.Client, overlayID string, deletion *DeletionEvent) error {
	key := fmt.Sprintf("replay:deletions:%s", overlayID)

	// Serialize event
	data, err := json.Marshal(deletion)
	if err != nil {
		return err
	}

	// Use millisecond precision for score
	score := float64(deletion.Timestamp.UnixMilli())

	// Pipeline: ZADD + EXPIRE
	pipe := client.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{
		Score:  score,
		Member: string(data),
	})
	pipe.Expire(ctx, key, 60*time.Second) // Reset TTL
	_, err = pipe.Exec(ctx)

	return err
}

// Retrieve events after timestamp (exclusive range)
func GetReplayEventsSince(ctx context.Context, client *redis.Client, overlayID string, sinceTimestamp int64) ([]*DeletionEvent, error) {
	key := fmt.Sprintf("replay:deletions:%s", overlayID)

	// Query: (sinceTimestamp, +inf) - exclusive lower bound
	results, err := client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min: fmt.Sprintf("(%d", sinceTimestamp), // Parenthesis = exclusive
		Max: "+inf",
	}).Result()

	if err != nil {
		return nil, err
	}

	// Deserialize all events
	events := make([]*DeletionEvent, 0, len(results))
	for _, member := range results {
		var event DeletionEvent
		if err := json.Unmarshal([]byte(member), &event); err != nil {
			continue // Skip malformed events
		}
		events = append(events, &event)
	}

	return events, nil
}

// Automatic cleanup (backup to TTL)
func PruneOldReplayEvents(ctx context.Context, client *redis.Client, overlayID string) error {
	key := fmt.Sprintf("replay:deletions:%s", overlayID)
	cutoff := time.Now().Add(-60 * time.Second).UnixMilli()

	// Remove events older than 60 seconds
	return client.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%d", cutoff)).Err()
}
```

### Frontend Reconnection with Replay Request

```typescript
// Source: Extends existing frontend/src/lib/api/websocket.ts pattern
// Adds replay request on reconnection

export class WebSocketClient {
  private lastSeenTimestamp: number = 0;
  private overlayId: string = '';
  private token: string = '';

  connect(overlayId: string, token: string) {
    this.overlayId = overlayId;
    this.token = token;

    // Load last seen timestamp from localStorage (survives page reload)
    const storageKey = `ws_last_seen_${overlayId}`;
    this.lastSeenTimestamp = parseInt(localStorage.getItem(storageKey) || '0', 10);

    const url = `${WS_URL}/ws/overlay/${overlayId}?token=${token}`;
    this.ws = new WebSocket(url);

    this.ws.onopen = () => {
      console.log('[WebSocket] Connected');
      this.reconnectAttempts = 0;

      // Request replay if reconnecting (not first connect)
      if (this.lastSeenTimestamp > 0) {
        const replayRequest = {
          type: 'replay_request',
          since: this.lastSeenTimestamp,
        };
        this.ws?.send(JSON.stringify(replayRequest));
        console.log('[WebSocket] Requested deletion replay since:', new Date(this.lastSeenTimestamp));
      }
    };

    this.ws.onmessage = (event) => {
      try {
        const wsMessage: WebSocketMessage = JSON.parse(event.data);

        // Handle replay response (batch of missed deletions)
        if (wsMessage.type === 'replay_response') {
          const deletions = wsMessage.data as DeletionEvent[];
          console.log(`[WebSocket] Replaying ${deletions.length} missed deletions`);

          deletions.forEach(deletion => {
            this.applyDeletion(deletion);
          });
          return;
        }

        // Handle real-time chat messages and deletion events
        if (wsMessage.type === 'chat_message') {
          if (wsMessage.data.event?.type === 'message_deletion') {
            // Process deletion
            this.applyDeletion(wsMessage.data.event.metadata);

            // Update last seen timestamp AFTER processing
            this.lastSeenTimestamp = Date.now();
            localStorage.setItem(`ws_last_seen_${this.overlayId}`, String(this.lastSeenTimestamp));
          } else {
            // Regular message
            this.messageCallbacks.forEach(cb => cb(wsMessage.data));
          }
        }

      } catch (error) {
        console.error('[WebSocket] Failed to parse message:', error);
      }
    };

    this.ws.onclose = (event) => {
      console.log('[WebSocket] Closed:', event.code, event.reason);

      // Attempt to reconnect (existing exponential backoff logic)
      if (this.reconnectAttempts < this.maxReconnectAttempts) {
        const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000);
        console.log(`[WebSocket] Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts + 1})`);

        this.reconnectTimeout = setTimeout(() => {
          this.reconnectAttempts++;
          this.connect(this.overlayId, this.token);
        }, delay);
      }
    };
  }

  private applyDeletion(deletion: DeletionMetadata) {
    // Unified deletion logic for both real-time and replayed events
    // (existing Phase 1 logic from page.tsx)
  }
}
```

### Artillery Load Test for Batch Deletion

```yaml
# Source: Artillery documentation (https://www.artillery.io/docs/guides/websockets)
# File: tests/load/batch-deletion.yml

config:
  target: "ws://localhost:8080"
  phases:
    - duration: 60
      arrivalRate: 5
      name: "Steady load - batch deletion test"
  variables:
    overlayId: "550e8400-e29b-41d4-a716-446655440000"
    token: "test-jwt-token"
  processor: "./batch-deletion-processor.js"
  plugins:
    expect: {}

scenarios:
  - name: "1000 message batch deletion performance"
    engine: ws
    flow:
      # Connect WebSocket
      - connect:
          url: "/ws/overlay/{{ overlayId }}?token={{ token }}"

      # Wait for connection established
      - think: 2

      # Function: Send 1000 messages via HTTP API (simulates chat activity)
      - function: "sendBatchMessages"

      # Wait for all messages to arrive and render
      - think: 10

      # Function: Trigger user ban (batch deletion event)
      - function: "triggerUserBan"

      # Expect deletion event within 2 seconds
      - expect:
          - regexp: ".*message_deletion.*"
            timeout: 2000

      # Disconnect
      - close:
```

**Processor script (batch-deletion-processor.js):**
```javascript
// Source: Artillery custom processor pattern

module.exports = {
  sendBatchMessages: sendBatchMessages,
  triggerUserBan: triggerUserBan,
};

async function sendBatchMessages(context, events, done) {
  const http = require('http');

  const messageCount = 1000;
  const userId = `test-user-${Date.now()}`;

  console.log(`Sending ${messageCount} messages for user ${userId}`);

  for (let i = 0; i < messageCount; i++) {
    // Simulate message send via HTTP API (non-blocking)
    // In practice, use batch endpoint or message processor directly
  }

  // Store user ID for later ban
  context.vars.bannedUserId = userId;

  return done();
}

async function triggerUserBan(context, events, done) {
  const http = require('http');
  const userId = context.vars.bannedUserId;

  console.log(`Triggering ban for user ${userId}`);

  // Call API to ban user (triggers batch deletion event)
  // Measure time from ban request to deletion event received
  context.vars.banStartTime = Date.now();

  return done();
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Redis Pub/Sub message replay | Redis Streams with consumer groups | Redis 5.0+ (2018) | Streams provide message ID tracking and XREAD replay, but Pub/Sub still preferred for ephemeral chat due to lower memory overhead |
| Manual WebSocket reconnection | Exponential backoff with jitter | Industry standard ~2015 | Prevents thundering herd problem; already implemented in frontend/src/lib/api/websocket.ts |
| Per-message deletion events | Coalesced batch deletion events | Phase 1 decision (2026) | Single event with user_id replaces 1,000 individual message IDs; reduces bandwidth and frontend iterations |
| Redis Strings for replay buffer | Redis Sorted Sets for timestamp-based replay | Pattern established ~2013 | Sorted sets enable efficient ZRANGEBYSCORE queries; O(log(N)+M) vs O(N) for filtering strings |
| Kick unofficial REST API scraping | Kick Pusher WebSocket real-time events | Kick API evolution (2023+) | WebSocket provides ~100ms latency vs REST polling ~5s; but API remains unofficial |

**Deprecated/outdated:**
- **Redis Pub/Sub guarantees:** Redis Pub/Sub never guaranteed delivery (fire-and-forget), but developers treated as reliable; documented limitation since Redis 2.0
- **Synchronous React rendering:** React 18 concurrent rendering (March 2022) changed state update batching; `unstable_batchedUpdates()` no longer needed
- **WebSocket without heartbeat:** Modern WebSocket implementations require ping/pong keepalive; already present in api-gateway/websocket/connection.go (30s ping)
- **Kick REST API for chat:** Early Kick integrations used REST API scraping; replaced by Pusher WebSocket for real-time messages

## Open Questions

1. **What is the exact Kick deletion event name and structure?**
   - What we know: Third-party libraries (KickLib, kick_live_ws) show `ChatMessageDeletedEvent` with `deletedMessage.id` field
   - What's unclear: Official Kick documentation doesn't exist; event name might differ (capitalization, namespace)
   - Recommendation: Log all unhandled Pusher events during development; monitor Kick WebSocket via browser DevTools; adjust event name constant based on observed events. **MUST validate in production environment** (KICK-02 requirement)

2. **Should replay buffer distinguish between deletion types (single/batch/clear)?**
   - What we know: All deletion types flow through same UnifiedChatMessage schema; frontend handles type switching
   - What's unclear: Whether replaying "clear" deletion after reconnect makes sense (chat already cleared, new messages may have arrived)
   - Recommendation: Store all deletion types in replay buffer; frontend handles idempotently (filter on empty array = no-op). Add metric to track "clear replayed after >10s disconnect" to identify potential confusion.

3. **How to handle >60 second disconnections?**
   - What we know: Replay buffer TTL is 60 seconds; longer disconnects = message loss acceptable per requirements (REL-03)
   - What's unclear: Should frontend display warning "Disconnected for >60s, some deletions may have been missed"?
   - Recommendation: Track disconnect duration in frontend; if >60s, log warning to console but don't show UI notification (chat is ephemeral, deletions are best-effort). Document limitation in user-facing docs.

4. **Should load test simulate concurrent overlays?**
   - What we know: Load test validates single overlay with 1,000 messages
   - What's unclear: Production may have 100+ concurrent overlays each receiving deletions simultaneously
   - Recommendation: Phase 3 load test focuses on single overlay (validates React rendering). Future: add multi-overlay stress test (validates backend Pub/Sub fan-out and Redis memory under load).

5. **Does Kick WebSocket support user ban events (batch deletion)?**
   - What we know: YouTube has `userBannedEvent`, Twitch has CLEARCHAT with target_user_id
   - What's unclear: Whether Kick emits batch deletion event or only single message deletions
   - Recommendation: Implement single message deletion first (KICK-01). Monitor Kick WebSocket for user ban events during manual testing. If batch events exist, add handler in follow-up. If not exist, document limitation: "Kick platform only supports single message deletion, not batch user timeouts."

## Sources

### Primary (HIGH confidence)
- [Redis Sorted Sets Documentation](https://redis.io/docs/latest/develop/data-types/sorted-sets/) - Official Redis data structure guide
- [Redis Pub/Sub Documentation](https://redis.io/docs/latest/develop/pubsub/) - Official guide acknowledging fire-and-forget nature
- [go-redis v9 Guide](https://redis.io/docs/latest/develop/clients/go/) - Official Go client documentation
- [React Automatic Batching (React 18)](https://github.com/reactwg/react-18/discussions/21) - Official React Working Group discussion
- Existing codebase: services/kick-listener/websocket/client.go (Pusher event handling pattern), frontend/src/lib/api/websocket.ts (reconnection logic), services/message-processor/registry/buffer.go (deletion buffer pattern)

### Secondary (MEDIUM confidence)
- [KickLib C# Library - ChatMessageDeletedEvent](https://github.com/Bukk94/KickLib) - Third-party Kick API wrapper showing deletion event structure
- [kick_live_ws npm package](https://socket.dev/npm/package/kick_live_ws) - Node.js Kick WebSocket library with deletion event examples
- [How to Implement Reconnection Logic for WebSockets (OneUpTime 2026)](https://oneuptime.com/blog/post/2026-01-27-websocket-reconnection/view) - WebSocket reconnection best practices
- [Redis Pub/Sub Fundamentals (pi's Substack)](https://neuralengineer595.substack.com/p/redis-pubsub-fundamentals) - Redis Pub/Sub message loss explanation
- [Realtime Replay Logs with Redis (Lincoln Loop)](https://lincolnloop.com/blog/realtime-replay-logs-redis/) - Pattern for replay buffer using sorted sets

### Tertiary (LOW confidence, needs validation)
- [Artillery WebSocket Load Testing](https://apidog.com/blog/websocket-testing-tools/) - Overview of Artillery for WebSocket testing (not official Artillery docs)
- [Apache JMeter WebSocket Sampler](https://www.blazemeter.com/blog/websocket-load-testing) - JMeter WebSocket testing guide (BlazeMeter, not official JMeter docs)
- [React Batching Optimization (Robin Wieruch)](https://www.robinwieruch.de/react-batching/) - Community guide on React 18 batching (not official React docs)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All libraries already in use (gorilla/websocket, go-redis, React 18), only Artillery is new (load testing only)
- Architecture patterns: MEDIUM-HIGH - Replay buffer pattern verified in Redis docs, but WebSocket replay flow is custom design (no official pattern exists for deletion-specific replay)
- Kick integration: MEDIUM - Event structure from multiple third-party sources, but no official Kick documentation. **Production validation required** (KICK-02)
- Pitfalls: HIGH - Based on Redis best practices, React 18 behavior verified in Phase 1, WebSocket reconnection patterns well-established

**Research date:** 2026-02-18
**Valid until:** 2026-03-18 (30 days; stable domain but Kick API unofficial → may change without notice)

**Critical findings for planner:**
1. **Kick deletion event handler:** Single event type addition, ~20 lines (follow existing `kickChatMessageEvent` pattern)
2. **Replay buffer is NEW component:** Not modification of Phase 1 buffer (different purpose: reconnection vs race conditions)
3. **Redis sorted set optimal for replay:** ZRANGEBYSCORE with timestamp provides efficient range queries, better than Streams for 60s window
4. **Frontend localStorage persistence:** Track last_seen timestamp across page reloads for reconnection replay
5. **Load testing validates React 18 batching:** Artillery script confirms <100ms render time for 1,000 message batch deletion
6. **MEDIUM confidence on Kick event structure:** Third-party sources agree but official validation needed (KICK-02 requirement)
