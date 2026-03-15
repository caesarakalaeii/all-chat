# Project Research Summary

**Project:** All-Chat v1.5 — Discord Listener + Relay
**Domain:** Bidirectional Discord chat source integration for Go microservices streaming overlay platform
**Researched:** 2026-03-15
**Confidence:** HIGH

## Executive Summary

All-Chat v1.5 adds Discord as a fifth chat source platform — the first platform with a bidirectional relay capability. Unlike the existing Twitch, YouTube, Kick, and TikTok listeners (receive-only), the Discord integration introduces an outbound relay path: normalized overlay messages are posted back to a user-configured Discord channel. This bidirectional topology creates a relay echo loop risk that does not exist in any current service and is the single most important architectural correctness requirement to establish from day one.

The recommended approach is a single new `discord-listener` Go microservice handling both directions via two goroutine groups. The inbound group connects to the Discord Gateway WebSocket using `bwmarrin/discordgo` (the only full-featured Go Discord library), receives `MESSAGE_CREATE` events, and publishes to the existing `chat:raw` Redis Stream. The outbound group subscribes to `overlay:{overlay_id}` Redis Pub/Sub — exactly as the API Gateway does — and posts non-Discord messages to Discord REST. Both groups share the same bot token, channel registry, and in-process loop prevention state, which is why a single service is strongly preferred over a split architecture. Five existing services require minimal extension: source-manager (add "discord" platform), auth-service (add OAuth2 "Add to Server" endpoints), overlay-manager (add "discord" validation), message-processor (add Discord normalizer), and frontend (add Discord source UI).

The top risks are: (1) the `MESSAGE_CONTENT` privileged Gateway Intent, which must be enabled in the Discord Developer Portal — without it, all messages arrive with silent empty content; (2) the relay echo loop, which if triggered causes a bot rate-limit cascade and potential application token ban; (3) Gateway heartbeat implementation, which requires an ACK-before-next-heartbeat pattern plus a single write channel to prevent concurrent-write panics; and (4) the Gateway identify rate limit (1 per 5 seconds per token), which requires startup jitter and RESUME-before-IDENTIFY logic to survive rolling deploys safely. All four risks have deterministic prevention strategies documented in the research.

## Key Findings

### Recommended Stack

The entire implementation can be built using dependencies already present in the codebase. The only net-new dependency is `bwmarrin/discordgo` (expected v0.28.1+), which must be verified with `go get github.com/bwmarrin/discordgo@latest` before pinning. All other required packages — `gin-gonic/gin`, `redis/go-redis/v9`, `jackc/pgx/v5`, `go.uber.org/zap`, `prometheus/client_golang`, `go.opentelemetry.io/otel`, `golang.org/x/time/rate`, `google/uuid`, and `stretchr/testify` — are already versioned in existing go.mod files and must be pinned to those exact versions.

Discord API v10 (Gateway v10) is the current stable version and the default in discordgo v0.28.x. The bot authenticates with a static Bot Token stored as a Kubernetes sealed-secret `DISCORD_BOT_TOKEN`. This is NOT a per-user OAuth token; it never expires and is never routed through token-refresh-service. The OAuth2 "Add to Server" flow captures `guild_id` only — no per-user access token or refresh token is issued.

**Core technologies:**
- `bwmarrin/discordgo` v0.28.1+: Discord Gateway WebSocket client + REST API — the only full-featured Go Discord library. Handles heartbeat, session resume, reconnect, and per-route rate limiting automatically. No meaningful alternative in the Go ecosystem for this use case.
- `golang.org/x/time/rate` (already present via twitch-listener): Per-channel token bucket for relay REST rate limiting — reuse of existing dependency, no new import.
- `redis/go-redis/v9` v9.18.0: Redis Streams publish (inbound) and Pub/Sub subscribe (relay) — identical patterns to existing listeners.
- Discord API v10 / Gateway v10: Default in discordgo v0.28.x. Do not override the API version constant.
- Bot Token model: Static application credential stored as Kubernetes sealed-secret. Not managed by token-refresh-service.

See `/home/moersener/Hobby/worktree/all-chat/.planning/research/STACK.md` for full dependency table and installation instructions.

### Expected Features

Discord bot integration follows an established managed-bot pattern. Users expect one-click "Add Bot to Server" authorization — not manual token pasting. Discord's two-token model (static bot token + per-server OAuth2 "Add to Server" flow) is fundamentally different from Twitch/YouTube user OAuth: the OAuth callback captures `guild_id` only; no access token or refresh token is stored or needed. The `MESSAGE_CONTENT` privileged intent must be explicitly enabled in the Discord Developer Portal before any integration testing — this is a common early-stage blocker.

**Must have (table stakes):**
- OAuth2 "Add Bot to Server" flow — standard managed-bot pattern; users expect one-click connection
- Inbound channel picker (select which channel to monitor) — raw channel ID configuration is unacceptable UX
- Real-time Discord messages in overlay — the core value proposition
- `author.bot` filtering at Gateway inbound — bot spam would immediately pollute overlays
- Platform label "discord" on all messages — users must distinguish platform origin
- Relay loop prevention (Discord messages not relayed back) — any visible loop makes the feature unusable
- Relay toggle per source (inbound-only is a valid use case) — some users want no relay
- Discord source card in overlay editor — consistent with Twitch/YouTube/Kick/TikTok source cards

**Should have (differentiators):**
- Rich embed relay (platform color, avatar) — polished relay output vs. raw text dumps
- Channel hierarchy display in picker (grouped by Discord category) — flat lists are hard to navigate in large servers
- Separate inbound and outbound channel configuration — flexibility most bridge bots lack
- Multiple Discord servers per user — multistreaming streamers manage multiple communities
- `MESSAGE_DELETE` forwarding — consistent with Twitch/YouTube deletion support already in the platform

**Defer (v2+):**
- Slash commands (`/status`, `/sources`)
- Discord stage channel support
- Role-based message filtering in overlay
- Discord verification for `MESSAGE_CONTENT` at scale (ops/deployment concern at 100+ servers, not a code concern)

See `/home/moersener/Hobby/worktree/all-chat/.planning/research/FEATURES.md` for full feature dependency graph and per-feature complexity ratings.

### Architecture Approach

A single `discord-listener` service with two goroutine groups handles both directions. The inbound goroutine group manages the Discord Gateway WebSocket connection, filters `author.bot == true` events, maps `MESSAGE_CREATE` events to `RawChatMessage`, and publishes to `chat:raw`. The outbound relay goroutine group subscribes to `overlay:{overlay_id}` Redis Pub/Sub (as a peer of the API Gateway), applies the echo loop filter (`platform != "discord"`), and calls Discord REST `POST /channels/{channel_id}/messages` with per-channel token-bucket rate limiting.

No new database tables are required. Discord sources fit the existing `overlay_chat_sources` schema using the `config` JSONB column for Discord-specific fields (`guild_id`, `inbound_channel_id`, `relay_channel_id`, `relay_enabled`). The source key format is `discord:{guild_id}:{channel_id}`, consistent with the platform-prefixed pattern used by other listeners. All Discord Snowflake IDs must be stored as strings — never as integers — to avoid frontend JavaScript safe-integer truncation for values above 2^53. At v1.5 scale (far below Discord's 2,500-guild single-shard limit), a single Gateway connection (`num_shards=1`) managed by source-manager leader election is correct and sufficient.

**Major components:**
1. `discord-listener` (NEW) — Gateway WebSocket shards, inbound publish to `chat:raw`, relay subscribe from `overlay:{id}` Pub/Sub, Discord REST posting, per-channel rate limiter, in-memory channel registry
2. `auth-service` (EXTEND) — Add `GET /api/v1/auth/discord/authorize` and `/callback` endpoints; store `guild_id` on successful bot authorization; bot token stored as Kubernetes secret env var, not in `oauth_tokens` table
3. `message-processor` (EXTEND) — Add `normalizer/discord_normalizer.go`; skip 7TV/BTTV/FFZ emote enrichment for `platform="discord"`
4. `source-manager` (EXTEND) — Add `"discord"` to supported platforms; add shard leadership key namespace `leader:discord:shard:{shard_id}`
5. `overlay-manager` (EXTEND) — Add `"discord"` to `validPlatforms`; validate `guild_id` present in config on source create/update

See `/home/moersener/Hobby/worktree/all-chat/.planning/research/ARCHITECTURE.md` for full service directory structure, data flow diagrams, code patterns, and a definitive anti-pattern list.

### Critical Pitfalls

1. **Missing MESSAGE_CONTENT privileged intent — silent empty messages:** Bot receives events, counters increment, Redis fills — but all `content` fields are empty strings with no error from Discord. Must enable in Developer Portal AND declare in `IDENTIFY` intents bitmask `(1<<0)|(1<<9)|(1<<15) = 33281`. Add startup assertion on first `READY` event plus `discord_message_content_empty_total` Prometheus counter. Validate in Phase 1 before writing any integration tests.

2. **Relay echo loop causing bot rate-limit cascade:** Without unconditional `platform != "discord"` filtering in the relay goroutine before any REST call, Discord messages are relayed back to Discord, triggering another `MESSAGE_CREATE`, creating an infinite loop. Unthrottled, this exhausts Discord REST rate limits and can result in bot token ban and hours of downtime. The filter must exist before the relay is ever connected to a live bot. An integration test asserting no REST call is made for a Discord-platform pub/sub message is required before merge.

3. **Gateway heartbeat miss causing zombie connection and message loss:** Common Go mistakes — missing ACK tracking, concurrent WebSocket writes without a mutex — cause Discord to close connections with code 1008. Implement heartbeat with `time.NewTicker`, track a `heartbeatACKed` bool checked before each send, and funnel all WebSocket writes through a single channel to a dedicated writer goroutine. Store `session_id` and `resume_gateway_url` in Redis on every `READY` for session RESUME capability.

4. **Multiple pods connecting on the same shard — session invalidation cascade:** All pods share one bot token. Two concurrent Gateway connections on the same shard cause Discord to invalidate one, triggering a reconnect storm. Gate Gateway connection on source assignment: only pods with at least one Discord source assigned may connect. Use Redis lock `discord:gateway:shard:0:holder` as the connection ownership mechanism.

5. **Gateway identify rate limit during rolling deploys — reconnect storm:** Discord enforces 1 identify per 5 seconds per bot token. HPA scale-up or rolling deploys trigger simultaneous identifies from multiple pods. Apply existing 0-30s startup jitter plus Discord-specific stagger (pod ordinal * 6s). Always attempt RESUME before IDENTIFY — successful RESUME bypasses the rate limit entirely.

See `/home/moersener/Hobby/worktree/all-chat/.planning/research/PITFALLS.md` for all 15 pitfalls with phase-specific warnings, detection metrics, and recovery procedures.

## Implications for Roadmap

Research produced a clear 5-phase build order driven by hard dependency chains: auth must precede inbound (bot must be in guild before Gateway connection is valid), inbound must precede relay (relay loop prevention requires the inbound channel registry), both must precede load balancing hardening (need a correct single-pod service before adding multi-replica complexity), and all backend must precede setup UI (APIs must be stable before frontend calls them). The architecture research file even includes an explicit build order section that aligns with this phase structure.

### Phase 1: Foundation — Auth, Bot Token, and Gateway Connection
**Rationale:** All downstream work depends on a working bot token and a bot that can join servers. The auth credential model and Gateway connection ownership architecture are the highest-risk design decisions in the project. Getting the auth model wrong (treating bot token as per-user OAuth, routing through token-refresh-service) is expensive to undo after DB migrations are written. Getting connection ownership wrong (multiple pods on the same shard) causes the session invalidation cascade that makes the service unstable under any rolling deploy.
**Delivers:** Bot can join Discord servers via "Add to Server" OAuth flow; `DISCORD_BOT_TOKEN` available as Kubernetes sealed-secret; new auth-service Discord OAuth endpoints; guild membership stored in DB; Gateway connection established with correct intents bitmask; session RESUME infrastructure (session_id + resume_gateway_url in Redis); startup jitter and pod-ordinal stagger implemented; `MESSAGE_CONTENT` intent validated at startup with fail-fast assertion.
**Addresses:** Bot authorization (table stakes), `MESSAGE_CONTENT` intent (table stakes), connection ownership model.
**Avoids:** Pitfalls 1 (empty messages), 3 (zombie heartbeat), 4 (multiple pods same shard), 6 (shard mismatch), 7 (bot token treated as user OAuth), 12 (session not preserved on SIGTERM), 13 (Snowflakes stored as integers).

### Phase 2: Inbound Listener — Discord Gateway to Overlay
**Rationale:** The inbound direction is simpler than the relay and proves the full message pipeline integration before adding bidirectional relay complexity. A working inbound path validates discordgo event handling, channel filtering, RawChatMessage mapping, Redis Streams publish, the Discord normalizer in message-processor, and platform registration across source-manager and overlay-manager — all against real Discord data.
**Delivers:** Discord messages appear in overlays in real-time; `platform="discord"` messages flow through the full pipeline; `author.bot` filtering active at the Gateway handler level; channel filtering (only configured inbound channel_id, not all guild channels).
**Uses:** `bwmarrin/discordgo`, existing Redis Streams publisher pattern from twitch-listener, Discord normalizer (new file, minimal — ~6 field mappings, no emote enrichment).
**Implements:** Gateway inbound goroutine group, channel registry (in-memory + DB sync on NOTIFY), Redis Streams publisher, message-processor Discord normalizer, source-manager and overlay-manager platform registration.
**Avoids:** Pitfall 14 (bot receives all guild channels — must filter on channel_id against active source registry).

### Phase 3: Outbound Relay — Overlay to Discord
**Rationale:** Relay depends on the in-memory inbound channel set established in Phase 2 for correct loop prevention. Rate limiting must be designed from the start — adding it as a post-production fix requires a relay component rewrite. The relay is architecturally the riskiest phase because it introduces the first outbound REST call pattern under load in the entire codebase.
**Delivers:** Non-Discord overlay messages relayed to configured Discord channel; echo loop prevented by unconditional platform filter; relay queue bounded (50 messages, drop-oldest) with `discord_relay_dropped_total` counter; Discord 429 responses handled by parsing `Retry-After` header; per-channel token bucket rate limiter (2 msg/s default, burst 5); single shared `http.Client` with configured transport; integration test asserting no REST call on Discord-platform pub/sub message.
**Uses:** Redis Pub/Sub subscriber (api-gateway pattern), `golang.org/x/time/rate` token bucket (existing dependency), single shared `http.Client` with `MaxIdleConnsPerHost: 10`.
**Implements:** Relay goroutine group (relay/worker.go, relay/poster.go, relay/ratelimit.go), loop prevention filter, bounded message queue.
**Avoids:** Pitfalls 2 (echo loop), 5 (REST rate limit cascade), 10 (port exhaustion from per-call HTTP client creation), 11 (relay logic inside message-processor pipeline), 15 (webhooks vs. bot REST).

### Phase 4: Production Hardening — Load Balancing and HPA
**Rationale:** Phases 1-3 produce a correct single-pod service. Phase 4 makes it production-grade by adding multi-replica support with shard ownership via leader election, HPA configuration, full Prometheus metrics surface, and a Grafana dashboard. The shard model is deliberately kept simple at v1.5 scale: `num_shards=1`, single shard leader. The existing source-manager coordinator is reused with a shard-scoped leadership key.
**Delivers:** Multiple discord-listener pods with shard ownership via source-manager leader election; Gateway connection gated on source assignment; failover within 60 seconds; HPA Kubernetes manifest; full Prometheus metrics (6 key counters/gauges documented in ARCHITECTURE.md); Grafana dashboard; `GET /gateway/bot` startup query to log Discord's recommended shard count.
**Implements:** Coordinator integration, shard leadership key namespace in source-manager, HPA config, startup jitter refinement (pod-ordinal * 6s stagger verified under rolling deploy).
**Avoids:** Pitfalls 4 (multiple pods same shard), 6 (shard mismatch), Gateway identify rate limit during HPA scale-up.

### Phase 5: Setup UI — Frontend Discord Source Configuration
**Rationale:** UI is last because it depends on all backend APIs being stable. Building against a moving API surface creates frontend rework. The UI follows established patterns from existing Twitch/YouTube source cards and the current OAuth2 connect button UX — lower technical risk than any of the backend phases.
**Delivers:** Streamer can connect Discord servers (OAuth2 redirect), pick inbound channels (grouped by Discord category), configure optional relay with outbound channel picker, and manage Discord sources from the overlay editor — with full parity to existing Twitch/YouTube UX. Includes "Disconnect server" soft-delete flow.
**Implements:** "Connect Discord Server" OAuth2 redirect button, Discord OAuth2 callback page, guild info display (server name + icon), inbound channel picker (hierarchical dropdown grouped by category), relay toggle + outbound channel picker, Discord source card in overlay editor.
**Avoids:** The hierarchical channel picker (MEDIUM complexity) needs careful implementation — Discord category grouping requires a two-pass channel list transform; a flat list is the fallback if time-constrained.

### Phase Ordering Rationale

- Auth before inbound: the bot must be a guild member before Gateway events for that guild are delivered. Guild membership is established by the Phase 1 "Add to Server" OAuth flow.
- Inbound before relay: the relay's loop prevention filter depends on the in-memory inbound channel registry, which is only populated once the inbound listener is running and syncing from the database.
- Both inbound and relay before hardening: the correct single-pod behavior must be verified end-to-end before introducing multi-replica shard ownership complexity. Bugs that surface only at multi-replica scale are significantly harder to diagnose.
- All backend before UI: the channel picker, relay toggle, and OAuth2 callback page depend on stable API endpoints from Phases 1-3. Frontend-first would require constant API renegotiation.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 1 (Gateway connection ownership model):** The Redis lock approach for shard ownership (`discord:gateway:shard:0:holder`) is the recommended design but has not been validated against the specific source-manager coordinator API. Verify the coordination protocol before writing the startup connection gating logic.
- **Phase 3 (Relay rate limit bucket values):** Discord REST rate limit specifics (exact per-channel limit, per-route bucket semantics, `X-RateLimit-Bucket` header format) are HIGH-confidence from training data but must be verified against live Discord API headers during Phase 3 implementation before finalizing the token bucket configuration.
- **Phase 4 (HPA metric selection):** The specific HPA scaling metric (Gateway message throughput vs. relay queue depth vs. connected shard count) needs definition against the actual Prometheus metric surface built in Phases 2-3.

Phases with standard patterns (research-phase likely skippable):
- **Phase 2 (Inbound listener):** The pattern is near-identical to kick-listener. discordgo handles all WebSocket complexity. The normalizer maps ~6 fields. Well-understood after STACK.md research.
- **Phase 5 (Setup UI):** Follows the established overlay editor source card pattern. The hierarchical channel picker adds MEDIUM complexity but the general approach is clear.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All existing dependencies verified from actual go.mod files in the codebase. discordgo version is MEDIUM (verify latest before pinning). Discord API v10 and bot token model are HIGH — stable and unchanged since 2022. |
| Features | HIGH | Discord API fundamentals (intents, OAuth2 bot scopes, Gateway events, rate limits) are stable and extensively documented. Feature complexity ratings based on direct codebase analysis of existing auth-service and overlay-manager integration patterns. |
| Architecture | HIGH (inbound) / MEDIUM (relay specifics) | Inbound architecture directly mirrors kick-listener patterns verified in codebase. Relay rate limit specifics (exact per-channel burst limits, bucket header semantics) are MEDIUM — based on training data, must be confirmed against live Discord API headers during Phase 3. |
| Pitfalls | HIGH | 15 pitfalls derived from direct codebase inspection (source-manager coordinator, twitch-listener IRC connection, architecture docs) combined with stable Discord API documentation. Echo loop risk and heartbeat correctness are logical derivations from the architecture, not speculative. |

**Overall confidence:** HIGH

### Gaps to Address

- **discordgo version:** Run `go get github.com/bwmarrin/discordgo@latest` before pinning. Expected v0.28.1+; confirm resolved version in `go.sum` before writing go.mod.
- **Discord REST rate limit bucket values:** The relay rate limiter design assumes 5 msg/s per channel and 50 req/s global. Verify by inspecting `X-RateLimit-Bucket` and `X-RateLimit-Limit` response headers from a live bot during Phase 3 implementation before finalizing the token bucket configuration.
- **Discord recommended shard count:** Query `GET /gateway/bot` on first production startup and log the recommended shard count. If it differs from `DISCORD_NUM_SHARDS=1`, log a WARN and document the scale threshold in the service README and a new ADR.
- **MESSAGE_CONTENT privileged intent at scale:** Bots in 100+ servers require Discord verification to retain this intent. Document the scale threshold in the service README before production launch. No code change is needed — purely an ops/growth planning concern.
- **READ_MESSAGE_HISTORY permission requirement:** The minimum bot permission integer documents `READ_MESSAGE_HISTORY` as required. Validate whether Gateway inbound actually requires it or whether `VIEW_CHANNEL` alone suffices before generating the authorization URL.

## Sources

### Primary (HIGH confidence)
- Codebase: `services/twitch-listener/go.mod`, `services/kick-listener/go.mod`, `services/auth-service/go.mod`, `shared/go.mod` — all dependency versions verified
- Codebase: `services/message-processor/models/message.go` — `RawChatMessage` schema verified
- Codebase: `services/twitch-listener/publisher/stream_publisher.go` — Redis Streams publish pattern (`chat:raw`, XADD, MaxLen 1000000) verified
- Codebase: `services/auth-service/oauth/platform.go`, `oauth/twitch.go` — `OAuthProvider` interface pattern verified
- Codebase: `services/overlay-manager/models/chat_source.go` — `validPlatforms` map, `ChatSource.Config` JSONB pattern verified
- Codebase: `services/source-manager/coordination/coordinator.go`, `assigner.go` — leader election and assignment model verified
- Codebase: `services/kick-listener/websocket/client.go` — WebSocket read/write pump pattern verified
- Discord API: Gateway intents (`MESSAGE_CONTENT` privileged since April 2022), bot OAuth2 scope model, heartbeat protocol, rate limit headers — HIGH confidence, stable since 2022

### Secondary (MEDIUM confidence)
- `github.com/bwmarrin/discordgo` — v0.28.1 latest as of training cutoff (August 2025); verify before pinning
- Discord Gateway sharding (2,500 guilds/shard limit) — training knowledge; verify at https://discord.com/developers/docs/topics/gateway#sharding before Phase 4
- Discord REST rate limits (5 msg/s per channel, 50 req/s global) — training knowledge; verify at https://discord.com/developers/docs/topics/rate-limits before Phase 3 rate limiter finalization

### Tertiary (LOW confidence — needs live validation)
- Exact `X-RateLimit-Bucket` header semantics and bucket granularity during relay burst — must be validated against live API during Phase 3
- `READ_MESSAGE_HISTORY` permission requirement for Gateway inbound — verify against live bot test in Phase 1

---
*Research completed: 2026-03-15*
*Ready for roadmap: yes*
