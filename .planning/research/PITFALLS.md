# Domain Pitfalls

**Domain:** Discord Gateway listener + relay integration into Go microservices platform (v1.5)
**Researched:** 2026-03-15
**Confidence:** HIGH

---

## Critical Pitfalls

Mistakes that cause rewrites, data loss, or bot bans.

---

### Pitfall 1: Missing MESSAGE_CONTENT Privileged Intent Causes Silent Empty Messages

**What goes wrong:** The bot receives `MESSAGE_CREATE` events with an empty `content` field. Events arrive, Prometheus counters increment, Redis Streams fill with messages — but every message has blank text. Discord does not error. It silently omits the field.

**Why it happens:** Since April 2022, `MESSAGE_CONTENT` is a privileged gateway intent. Bots in fewer than 100 servers receive it automatically during development. Verified bots in 100+ servers must declare the intent in the `IDENTIFY` payload's `intents` bitmask AND enable it in the Discord Developer Portal under Bot > Privileged Gateway Intents. Missing either half means silent content omission.

**Consequences:** The listener appears functional. Messages are published to Redis Streams with `text: ""`. They flow through message-processor normalization (which passes empty text through), into overlay pub/sub, and render as blank messages on overlays. Silent data corruption that is easy to miss in integration testing if tests use bot-posted messages from a small test server (where the intent is auto-granted).

**Prevention:**
- Declare intents bitmask `33281` in the Gateway `IDENTIFY` payload: `(1 << 0) | (1 << 9) | (1 << 15)` = `GUILDS | GUILD_MESSAGES | MESSAGE_CONTENT`.
- Enable MESSAGE_CONTENT in the Discord Developer Portal before running any integration tests.
- Add a startup assertion: on first `READY` event, log the resolved intents value. Assert the `MESSAGE_CONTENT` bit is set. Fail fast if not.
- Write a contract test: post a message via REST API in a test guild > 100 servers (or a bot that has been through verification flow), verify `content` is non-empty in the received event.
- Add a Prometheus counter `discord_message_content_empty_total`. Any non-zero value after confirmed message delivery indicates missing intent.

**Detection:** `discord_message_content_empty_total > 0` after verified message delivery. Log a `WARN` with `intent_value` on every `READY` event.

**Phase:** Must be validated in Phase 1 (Gateway connection), before any integration tests are written. The startup assertion should exist before the first demo.

---

### Pitfall 2: Relay Echo Loop — Discord Messages Re-relayed Back to Discord

**What goes wrong:** A Discord message arrives in channel A. It flows through: discord-listener → Redis Streams → message-processor (normalizes, sets `platform="discord"`) → Redis Pub/Sub (`overlay:{overlay_id}`) → relay consumer. The relay reads the pub/sub message, sees a new message to forward, and posts it back to Discord (channel A or the configured outbound channel). That post triggers a new `MESSAGE_CREATE` event from the bot itself. The listener ingests it. The cycle repeats. Infinite loop.

**Why it happens:** The relay subscribes to `overlay:{overlay_id}` pub/sub, which carries all normalized messages — including Discord-sourced ones. Without an explicit platform-origin guard, the relay cannot distinguish "came from Discord" from "came from Twitch." This is a new architectural pattern with no precedent in the existing codebase (all other listeners are receive-only).

**Consequences:** Unthrottled message storm. Discord rate-limits the bot (429s), then throttles all REST calls, then potentially bans the application token. All overlay messages for affected overlays stop. Recovery requires bot token rotation, application reconfiguration in the Discord Developer Portal, and a Kubernetes rollout. Downtime measured in hours.

**Prevention:**
- The `RawChatMessage` struct already carries a `Platform` field. Ensure `platform = "discord"` is set on every message published from the discord-listener.
- In the relay consumer goroutine, filter unconditionally: only forward messages where `platform != "discord"`. This is the loop-safe filter described in the PROJECT.md milestone requirements.
- Secondary guard: compare the relay's configured outbound `channel_id` against the inbound message's source channel ID. If identical, drop regardless of platform (handles edge cases where platform field is malformed).
- Write an integration test that injects a Discord-platform message directly into the overlay pub/sub channel and asserts the relay does NOT call the Discord REST endpoint. This test must exist before the relay is ever connected to a live bot.
- The filter must be present before the relay is merged into any branch connected to a live Discord application.

**Detection:** `relay_discord_suppressed_total` counter that increments on every suppressed Discord-sourced message. Alert if this counter is zero after 24 hours with active Discord sources — may indicate the filter is broken and passing everything through.

**Phase:** Architecture decision in Phase 1. Filter implementation in Phase 2 (relay). Integration test in Phase 3. The filter logic must exist before the relay connects to any live bot.

---

### Pitfall 3: Gateway Heartbeat Miss Causes Zombie Connection and Message Loss

**What goes wrong:** The Discord Gateway requires a `Heartbeat` (opcode 1) every `heartbeat_interval` milliseconds (provided in the `HELLO` payload, typically ~41.25 seconds). If the bot sends a heartbeat but does not receive an ACK (opcode 11) before the next heartbeat is due, Discord considers the connection a zombie and closes it with close code 1008. The session may not be resumable depending on how long the zombie persisted.

**Why it happens:** Common Go implementation mistakes:
- Using `time.Sleep` instead of `time.NewTicker`, causing drift under load.
- Not tracking whether the previous heartbeat was ACK'd before sending the next one.
- Sharing the WebSocket write path between the heartbeat goroutine and the event-dispatch goroutine without a mutex, causing concurrent-write panics that crash the connection goroutine silently.
- Not storing `session_id` and `resume_gateway_url` from `READY`, so a reconnect requires full re-IDENTIFY (counts against the identify rate limit).

**Consequences:** Connection drops without warning. Pod may not detect the drop for up to one heartbeat interval (~41 seconds). All messages from monitored Discord channels during that window are lost. If session cannot be resumed, a full re-IDENTIFY is required, which costs 5+ seconds and counts against the global identify rate limit (1 per 5 seconds per token).

**Prevention:**
- Implement heartbeat as a dedicated goroutine with `time.NewTicker(heartbeat_interval)`.
- Track a boolean `heartbeatACKed`. Before sending each heartbeat: check `heartbeatACKed`. If false, the connection is a zombie — close with code 1000 and initiate session resume.
- Use a single write channel (`chan []byte`) for all WebSocket writes. Both the heartbeat goroutine and event dispatcher send to this channel. A dedicated writer goroutine drains it and calls `conn.WriteMessage`. This eliminates all concurrent-write races.
- On receiving `READY`: store `session_id` and `resume_gateway_url` in Redis, keyed by pod ID. On reconnect, always attempt `RESUME` (opcode 6) before `IDENTIFY` (opcode 2). Successful RESUME does not count against the identify rate limit.

**Detection:** `discord_heartbeat_ack_missed_total` counter. Alert on any value > 0 in a 5-minute window.

**Phase:** Core implementation concern for Phase 1. Session resume storage in Redis must be implemented alongside the initial connection — not deferred to "later."

---

### Pitfall 4: Gateway Identify Rate Limit During HPA Scale-Up or Rolling Deploy

**What goes wrong:** On startup, multiple discord-listener pods each open a Gateway connection and send `IDENTIFY`. Discord enforces a global identify rate: 1 per 5 seconds per bot token. If 3 pods start simultaneously (HPA scale-up event or Kubernetes rolling deploy), all 3 send `IDENTIFY` within milliseconds of each other. Discord closes 2 connections with opcode 9 (Invalid Session, not resumable). Those pods retry immediately and hit the rate limit again. Cascading reconnect storm.

**Why it happens:** The existing load balancing system applies startup jitter (0-30 seconds random delay) across Twitch/YouTube/Kick/TikTok listeners specifically to prevent the thundering herd pattern (see `PROJECT.md` Key Decisions). This jitter must be applied to the discord-listener. Additionally, the Discord identify rate limit is stricter and more consequential than IRC reconnects — a failed identify cannot simply be retried immediately.

**Consequences:** Pods stuck in perpetual identify-fail-wait cycles. No Discord messages ingested during the identify storm. If triggered by a rolling deploy, the entire Discord listener fleet may be offline for the duration of the deploy (potentially minutes).

**Prevention:**
- Apply the same startup jitter (0-30s random delay) already present in other listeners.
- Add Discord-specific stagger: derive `pod_index * 6 seconds` additional delay from the pod hostname (parse the StatefulSet ordinal or use a Redis-based pod registration sequence). Six seconds safely exceeds the 5-second identify rate limit.
- On receiving opcode 9 (Invalid Session), wait 1-5 seconds (random) before re-identifying. Never retry opcode 9 immediately.
- Attempt RESUME before IDENTIFY on every reconnect. Successful RESUME bypasses the identify rate limit entirely.
- Only pods that have been assigned at least one Discord source by the coordinator should open a Gateway connection. Pods with no assigned Discord sources must not connect.

**Detection:** `discord_invalid_session_total` counter. Alert on any value > 0 — this should never happen in a healthy deployment.

**Phase:** Phase 1 (startup behavior). The jitter and RESUME logic must be present before any load testing or production deployment.

---

### Pitfall 5: REST Rate Limit Mismanagement During Relay Bursts

**What goes wrong:** The relay posts messages to Discord via REST (`POST /channels/{channel_id}/messages`). Discord enforces two rate limit layers: per-route buckets (typically 5 requests per second per channel for message posting) and a global bucket (50 requests per second across all routes for the bot token). During a high-traffic overlay event such as a Twitch raid, dozens of messages per second flow through. The relay attempts to forward all of them, exhausts the per-channel bucket, and receives 429 responses. A naive retry-immediately loop amplifies toward the global bucket. If the global limit is hit, ALL Discord REST calls are blocked, including calls needed for source management.

**Why it happens:** No existing service in the codebase makes outbound REST calls under load. The existing `shared/ratelimit/` module handles inbound API Gateway rate limiting only. There is no outbound REST client rate limiter in the shared package. This is a genuinely new pattern.

**Consequences:** 429 errors cascade. Relay queue backs up. If the queue is unbounded, memory grows until the pod OOMs. If the global rate limit is hit, Gateway reconnect calls are also blocked, potentially causing the bot to disconnect.

**Prevention:**
- Parse `X-RateLimit-Remaining`, `X-RateLimit-Reset-After`, and `X-RateLimit-Bucket` response headers on every Discord REST call. Build a per-bucket leaky bucket that respects these headers rather than relying on a fixed rate.
- Implement a configurable relay rate cap: max 2 messages per second per outbound channel as the default (well below the 5/second Discord limit, leaving headroom for other REST calls).
- On 429 response: extract `Retry-After` header. Sleep exactly that duration before retrying. Do not retry sooner.
- Use a bounded buffered Go channel as the relay queue per outbound channel (suggested capacity: 50 messages). If the queue is full, drop the oldest message (not the newest — prefer recency for live chat relay). Log drops to `discord_relay_dropped_total`.
- Maintain a separate global rate limit bucket. Count all Discord REST calls against it regardless of per-route bucket.

**Detection:** `discord_relay_429_total` labeled by `{bucket_id, channel_id}`. `discord_relay_dropped_total` for queue overflow. Alert on sustained 429 rate above 1 per minute.

**Phase:** Phase 2 (relay implementation). Must be designed from the start. Adding rate limiting as a fix after hitting limits in production requires a relay rewrite.

---

### Pitfall 6: Shard Mismatch Causes Silent Event Blackout for Affected Guilds

**What goes wrong:** The Discord Gateway sharding model routes each guild to exactly one shard using `guild_id % shard_count`. If a pod connects with `shard_id=0, num_shards=2` but an assigned guild belongs to shard 1, that pod receives zero events for that guild — with no error, no log message, no indication from Discord that anything is wrong. The channel configured as a source simply never produces messages.

**Why it happens:** At current scale (small bot, few guilds), all guilds fit on a single shard (`shard_id=0, num_shards=1`). This works in development and early production. When the bot grows past approximately 2,500 guilds, Discord requires multiple shards. If someone adds shards by setting `num_shards=N` per pod (matching replica count) without coordinating shard ID assignment, each pod may connect to the wrong shard for its assigned guilds.

**Consequences:** Users report "Discord chat not showing up in overlay" for affected guilds. No errors in logs. Extremely difficult to diagnose without understanding the shard routing formula.

**Prevention:**
- For v1.5 (current scale, < 2,500 guilds): hardcode `shard_id=0, num_shards=1` in all pods. Document this limit explicitly in the service README and an ADR.
- Query `GET /gateway/bot` on startup to obtain Discord's recommended shard count. Log the value. If recommended count differs from configured count, log a WARN.
- Design the future shard assignment protocol before hitting scale: the source-manager coordinator assigns guild+channel sources to pods; the Discord shard for each guild is `guild_id % total_shards`. Source assignment and shard assignment are separate concerns — do not conflate them.
- On `READY`, log `unavailable_guilds` count. If non-zero after 60 seconds, a guild has not connected — may indicate a shard routing problem.

**Detection:** Log `unavailable_guilds` count from `READY` payload. Alert if count remains > 0 after 60 seconds. Alert if `GET /gateway/bot` returns `shards > 1` and configured `num_shards == 1`.

**Phase:** Phase 1 (document single-shard assumption and scale threshold). Phase 3 (integration with load balancer — do not conflate shard assignment with channel assignment).

---

## Moderate Pitfalls

---

### Pitfall 7: Discord Bot OAuth Flow Confused with User OAuth Flow

**What goes wrong:** The existing auth-service handles Twitch and YouTube with the standard user OAuth2 flow: authorization code grant, user grants scopes, per-user access token stored encrypted in PostgreSQL, token-refresh-service refreshes it periodically. Discord bot authorization is different: the bot token (from the Discord Developer Portal) is a static application credential that does not expire and does not need refreshing. The user-facing "Add to Server" flow (`scope=bot`) results in the bot being added to a guild — it does not issue a user access token at all.

**Prevention:**
- Store the Discord bot token as an application environment variable (`DISCORD_BOT_TOKEN`), not in the OAuth tokens table. It is not a per-user credential.
- Do NOT route the Discord bot token through token-refresh-service. The service will attempt unnecessary refresh calls and likely fail or corrupt the stored value.
- The "Add to Server" OAuth2 flow (user-facing setup UI) uses `scope=bot+applications.commands` with the `authorization_code` grant. The result is guild membership for the bot, not a token to store. Record guild membership in a new PostgreSQL table: `(user_id, guild_id, authorized_at, inbound_channel_id, outbound_channel_id)`.
- Model this as the existing systems model bot vs. user: Twitch uses a bot OAuth token for IRC (static credential) separate from the user's Twitch OAuth token. Follow the same separation.

**Phase:** Phase 1 (auth design and database migration). Wrong direction here is expensive to undo.

---

### Pitfall 8: Source-Manager Assignment Key Must Include Guild ID

**What goes wrong:** The existing load balancer hashes on `source_id` (an opaque string) to assign sources to pods. Two Discord channels from different guilds may have similar or identical numeric channel IDs if the system only stores the channel ID without the guild ID. Hash collisions cause incorrect pod assignments. More critically, the `MESSAGE_CREATE` event payload includes `guild_id` — if the source record does not store it, the listener cannot verify that an incoming event matches an active source.

**Prevention:**
- Define the Discord source key as `discord:{guild_id}:{channel_id}` for consistent hashing. This matches the existing platform-prefixed pattern used by other listeners.
- Validate that `guild_id` is present on every `MESSAGE_CREATE` event before publishing to Redis Streams. Direct messages (DMs) do not have `guild_id` — the listener must drop DM events immediately, as they are not a supported source type.
- The source-manager coordinator, heartbeat, and migration logic all operate on opaque source IDs. No changes to coordinator internals are needed — only to how the discord-listener registers its sources.

**Phase:** Phase 1 (data model and source registration). Must be correct before any load balancing work.

---

### Pitfall 9: Multiple Pods Opening Gateway Connections on the Same Shard

**What goes wrong:** All discord-listener pods share the same `DISCORD_BOT_TOKEN`. At single-shard scale, if two pods both open a Gateway connection, the second `IDENTIFY` from the same bot token causes Discord to invalidate the first connection (opcode 7 Reconnect or opcode 9 Invalid Session). The first pod reconnects. Discord invalidates it again. Neither pod maintains a stable connection.

**Why it happens:** The source-manager assigns channels to specific pods, but does not inherently prevent multiple pods from each opening their own Gateway connection. Other listeners (Twitch IRC, Kick Pusher) handle this by each pod independently managing its assigned channels. This model works for those protocols. For Discord's Gateway, a shard connection covers ALL guilds on that shard — only one connection per shard per token is allowed.

**Prevention:**
- Gate Gateway connection on source assignment: a pod must have at least one Discord source assigned by the coordinator before it opens a Gateway connection. Pods with no assigned Discord sources must not connect to the Gateway.
- For single-shard deployments: at most one pod should hold the active Gateway connection. The coordinator (source-manager leader) assigns all Discord sources to the same pod when possible at single-shard scale.
- Alternatively, use a Redis-based Gateway connection lock: a pod acquires a Redis key `discord:gateway:shard:0:holder` (TTL: 2x heartbeat interval) before connecting. Only the lock holder connects. Other pods wait and poll for channel assignments to be delivered via a different mechanism (Redis Pub/Sub commands from the connection-holding pod).
- Store `session_id` and `resume_gateway_url` in Redis immediately after `READY`. The lock holder writes these; if it dies, the next pod to acquire the lock can attempt RESUME with the stored values.

**Phase:** Phase 1 (architecture decision — connection ownership model). This is the most important architectural question for the discord-listener before any code is written.

---

### Pitfall 10: Relay Not Reusing HTTP Client — Port Exhaustion Under Load

**What goes wrong:** Each relay call to Discord REST creates a new `http.Client` instance or new TCP connection. Under relay load (100+ messages per minute), this exhausts ephemeral TCP ports, accumulates TIME_WAIT connections, and adds 3-10ms of TCP handshake latency to every relay call.

**Prevention:**
- Create a single `http.Client` with an explicit transport at discord-listener service startup. Inject it into the relay component.
- Configure `MaxIdleConnsPerHost: 10` and `IdleConnTimeout: 90 * time.Second` on the transport.
- This follows the existing pattern in emote-service HTTP clients. The relay client is not special.

**Phase:** Phase 2 (relay implementation). Easy to get right from the start, expensive to diagnose in production.

---

### Pitfall 11: Relay Logic Inside Message-Processor Breaks the Processing Pipeline Contract

**What goes wrong:** If the relay is implemented as logic inside the message-processor (triggered on the processing path, not the pub/sub consumer path), a relay failure (Discord 429, timeout, network error) causes the message-processor to fail to ACK the Redis Streams message. The message gets reprocessed. Every reprocess triggers another relay attempt. The relay failure propagates backward into the core message pipeline.

**Why it happens:** The relay reads from the same pub/sub as the API Gateway. It may seem natural to add relay logic inside the message-processor's publish step. This conflates the processing pipeline (inbound) with the relay (outbound).

**Prevention:**
- Implement relay as an independent goroutine (or group of goroutines) that subscribes to `overlay:{overlay_id}` pub/sub as a separate consumer, parallel to the API Gateway's subscription. The relay is a peer of the API Gateway, not a component of the message-processor.
- Relay failures must never affect the message-processor's XACK cadence. Relay errors are logged and counted but do not propagate upstream.
- Keep the message-processor contract unchanged: normalize → enrich → route → publish to pub/sub → XACK. The relay is downstream of this contract.

**Phase:** Phase 1 (architecture decision). The PROJECT.md already lists "single service for inbound+outbound vs separate relay service" as an open question. The correct answer is: single discord-listener service containing both a Gateway inbound goroutine and a relay outbound goroutine, both operating independently. The relay goroutine subscribes to pub/sub independently; it does not depend on or modify the message-processor.

---

### Pitfall 12: Graceful Shutdown Does Not Preserve Session for Resume

**What goes wrong:** When a pod receives SIGTERM and begins the 25-second graceful shutdown, the Gateway WebSocket connection is dropped without a clean close. Discord's session resume window is typically a few minutes. If the pod restarts slowly (image pull, migration, slow startup) and the resume window expires, a full re-IDENTIFY is required. Worse: if the close code is 1000 (Normal Closure), Discord intentionally invalidates the session — there is nothing to resume.

**Prevention:**
- On SIGTERM: send WebSocket close frame with code 4000 (Unknown Error — Discord allows session resume after this code) rather than 1000.
- Store `session_id` and `resume_gateway_url` in Redis immediately after every `READY` event (keyed by pod ID). On startup, always attempt RESUME first using the stored values before falling back to IDENTIFY.
- The 25-second graceful shutdown window is sufficient: close Gateway with 4000 (instant), drain in-flight Redis Streams publishes (~1-2 seconds), shutdown HTTP server.
- Note: close code 1000 explicitly invalidates the session. Use 4000 for planned restarts, 1001 (Going Away) for pod eviction.

**Phase:** Phase 1 (connection lifecycle). Session storage in Redis must be present before the service reaches production.

---

## Minor Pitfalls

---

### Pitfall 13: Discord Snowflake IDs Must Be Stored as Strings

**What goes wrong:** Discord entity IDs (guild, channel, message, user) are 64-bit Snowflakes, JSON-encoded as strings. If any part of the pipeline stores them as integers, Go handles int64 correctly, but the frontend (JavaScript/TypeScript) has a 53-bit safe integer limit. Values above 2^53 are truncated silently. Discord message IDs regularly exceed this threshold.

**Prevention:**
- Store all Discord IDs as strings throughout the pipeline. Never convert Snowflakes to integer types.
- The system's internal `MessageID` is a UUID generated by the listener. The Discord Snowflake message ID belongs in `Tags["discord_message_id"]` or a dedicated metadata field — not as the primary `MessageID`.
- The `guild_id` and `channel_id` fields in the source record must be string columns in PostgreSQL (`text`, not `bigint`).

**Phase:** Phase 1 (data model). Cannot be corrected after database migrations are written.

---

### Pitfall 14: Bot Receives Events for All Channels in a Guild, Not Just Subscribed Ones

**What goes wrong:** When the bot is in a guild, it receives `MESSAGE_CREATE` events for every channel it has read permissions for — not just the channels configured as sources. Without explicit channel filtering, messages from non-subscribed channels flow into Redis Streams and appear in overlays.

**Prevention:**
- In the discord-listener event handler, look up active sources for the incoming event's `guild_id`. Check whether `event.channel_id` matches a registered source's `channel_id`. Drop the event if there is no matching source.
- This is identical to the pattern in twitch-listener: the IRC client joins only registered channels. The discord-listener must implement the equivalent channel filter at the event handler level.
- Cache the active sources map in memory (refresh on source assignment changes from coordinator). Do not query PostgreSQL on every `MESSAGE_CREATE` event.

**Phase:** Phase 1 (channel filtering). Must be present from the first working implementation.

---

### Pitfall 15: Bot REST Endpoint vs. Webhook — Using Webhooks for Relay

**What goes wrong:** Discord supports posting messages via incoming webhooks (a separate URL, no bot token required). Using webhooks for relay is tempting because they do not require the `SEND_MESSAGES` permission explicitly — just the webhook URL. However, webhooks cannot post "as" the bot user (they post as a webhook identity), require per-channel webhook creation (an additional setup step and permission), and lack consistent rate limit header semantics across older webhook endpoints.

**Prevention:**
- Use bot REST (`POST /channels/{channel_id}/messages` with `Authorization: Bot {token}`) for relay. The bot token is already available. Requires `SEND_MESSAGES` permission in the target channel, which is part of standard bot authorization.
- Do not implement webhooks for relay in v1.5. The additional setup complexity (webhook URL storage, per-channel creation, permission management) exceeds any benefit at current scale.

**Phase:** Phase 2 (relay implementation).

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| Phase 1: Gateway connection | Missing MESSAGE_CONTENT intent — silent empty messages | Validate intent bitmask on startup; add `discord_message_content_empty_total` counter; contract test against real bot |
| Phase 1: Gateway connection | Heartbeat goroutine without ACK tracking — zombie connection | Implement `heartbeatACKed` bool; zombie detection before each heartbeat send; single write channel |
| Phase 1: Connection ownership | Multiple pods connecting on same shard — session invalidation cascade | Gate Gateway connection on source assignment; Redis lock per shard; only connection-holder pod connects |
| Phase 1: Auth design | Bot token treated as user OAuth token — routed through token-refresh-service | Separate storage path (env var); new guild membership table; no token-refresh-service involvement |
| Phase 1: Data model | Missing guild_id in source key — incorrect hash assignments | Use `discord:{guild_id}:{channel_id}` as source ID; store all IDs as strings |
| Phase 1: Session lifecycle | Close code 1000 on SIGTERM invalidates session | Use close code 4000; store session_id + resume_gateway_url in Redis on every READY |
| Phase 2: Relay | Echo loop — Discord messages re-relayed back to Discord | Filter `platform == "discord"` unconditionally before relay; integration test required before connecting to live bot |
| Phase 2: Relay | REST 429 during relay burst — cascade to global limit | Per-bucket rate limiter parsing X-RateLimit headers; bounded drop-oldest queue; 2 msg/sec default cap |
| Phase 2: Relay | New HTTP client per call — port exhaustion | Single `http.Client` at startup; configure transport with `MaxIdleConnsPerHost: 10` |
| Phase 2: Relay | Relay logic in message-processor — pipeline contamination | Relay as independent pub/sub consumer; relay errors never propagate to XACK path |
| Phase 3: Load balancing | Shard ID conflated with pod index — event blackout for affected guilds | Document single-shard assumption; shard assignment separate from source assignment |
| Phase 3: Load balancing | Identify rate limit during HPA scale-up — reconnect storm | Pod-index stagger (pod_ordinal * 6s) on top of existing 0-30s jitter; RESUME before IDENTIFY |
| Phase 4: Production | Channel filter missing — non-subscribed channels flow into overlays | Filter on channel_id against active sources; cache source map in memory |

---

## Sources

**Confidence assessment:**

- Discord Gateway protocol (intents, heartbeat, opcodes, sharding, rate limits): HIGH — derived from official Discord developer documentation. Knowledge cutoff August 2025 covers all referenced features: privileged intents mandatory since April 2022, sharding model stable since 2020, rate limit bucket model stable since 2019.
- Integration pitfalls with existing Redis Streams / source-manager / load balancing: HIGH — derived from direct inspection of `services/source-manager/coordination/coordinator.go`, `assigner.go`, `services/twitch-listener/irc/connection.go`, `.planning/codebase/ARCHITECTURE.md`, `.planning/codebase/CONCERNS.md`, `.planning/PROJECT.md`.
- Relay echo loop risk: HIGH — logical derivation from system architecture; the pub/sub consumer model is identical to the API Gateway subscription model. No external source needed.
- REST rate limit bucket model: HIGH — Discord's X-RateLimit header semantics have been stable and are extensively documented.

**Official reference URLs:**
- Discord Gateway: https://discord.com/developers/docs/topics/gateway
- Discord Intents: https://discord.com/developers/docs/topics/gateway#gateway-intents
- Discord Rate Limits: https://discord.com/developers/docs/topics/rate-limits
- Discord Sharding: https://discord.com/developers/docs/topics/gateway#sharding
- Discord OAuth2: https://discord.com/developers/docs/topics/oauth2

**Codebase references:**
- `/home/moersener/Hobby/worktree/all-chat/services/source-manager/coordination/coordinator.go`
- `/home/moersener/Hobby/worktree/all-chat/services/source-manager/coordination/assigner.go`
- `/home/moersener/Hobby/worktree/all-chat/services/twitch-listener/irc/connection.go`
- `/home/moersener/Hobby/worktree/all-chat/.planning/PROJECT.md`
- `/home/moersener/Hobby/worktree/all-chat/.planning/codebase/ARCHITECTURE.md`
- `/home/moersener/Hobby/worktree/all-chat/.planning/codebase/CONCERNS.md`

---

*Pitfalls research for: All-Chat v1.5 Discord Listener + Relay*
*Researched: 2026-03-15*
*Confidence: HIGH*
