# Project Research Summary

**Project:** InnerTube YouTube Listener (Quota-Free Alternative)
**Domain:** Live Chat Ingestion Microservice
**Researched:** 2026-02-21
**Confidence:** MEDIUM

## Executive Summary

This research addresses building a quota-free YouTube live chat listener using Google's undocumented InnerTube API as a drop-in replacement for the existing official API-based youtube-listener service. The recommended approach is a Node.js microservice using the masterchat library (@stu43005/masterchat v1.5.0), integrated via HTTP REST with existing Go services and publishing to Redis Streams using the identical RawChatMessage contract.

The core trade-off is language heterogeneity (adding Node.js to a Go ecosystem) versus ecosystem maturity. No mature Go InnerTube libraries exist for live chat, while the JavaScript ecosystem has 3+ years of production-proven libraries. This heterogeneity is acceptable because All-Chat already uses multiple runtimes (Go services + React/Next.js frontend) and service-to-service HTTP integration is standard microservices practice.

The primary risk is schema drift: InnerTube is an undocumented internal API that can change without notice, breaking message normalization. Mitigation requires contract testing, schema validation, graceful degradation, and canary deployments. Secondary risks include deletion event semantic mismatches and stream discovery edge cases. Success depends on maintaining behavioral equivalence with the official listener despite swapping the data source.

## Key Findings

### Recommended Stack

**Recommendation:** Node.js 20 LTS service with TypeScript, masterchat library, ioredis for Redis integration, and Express for HTTP control plane.

**Rationale:** The InnerTube ecosystem is strongest in JavaScript (masterchat, YouTube.js) and Python (pytchat - archived). Masterchat provides 20+ action types, active maintenance (latest update April 2025), and production use by HolodexNet. The 3-year maturity advantage outweighs language consistency concerns. Go-Node.js HTTP integration matches existing All-Chat patterns (API Gateway WebSocket, service mesh).

**Core technologies:**
- **Node.js 20 LTS + TypeScript 5.x**: Runtime with type safety, matches masterchat implementation, active LTS until 2026-10
- **@stu43005/masterchat 1.5.0**: Most mature InnerTube live chat library, purpose-built for chat (not general YouTube API wrapper), handles 20+ action types including deletions
- **ioredis 5.x**: Industry-standard Redis client with Streams + Pub/Sub support, superior TypeScript types vs node-redis
- **Express 4.x**: HTTP server for health checks and control endpoints, simpler than Fastify, adequate for low-traffic control plane

**Critical version note:** Use @stu43005/masterchat fork (npm published, April 2025 update) over original sigvt/masterchat (June 2022, stale) and HolodexNet fork (maintenance status unclear).

### Expected Features

**Must have (table stakes):**
- **Live chat ingestion**: Real-time message polling via masterchat async iterator API
- **Message normalization**: Transform InnerTube responses to RawChatMessage JSON schema (exact match with official listener)
- **Super Chat/membership detection**: Extract amounts, milestones, tier data from InnerTube actions
- **Health checks**: K8s readiness/liveness probes (/health/live, /health/ready)
- **Graceful shutdown**: Clean up masterchat connections, flush Redis buffer on SIGTERM
- **Stream discovery**: Channel-to-video ID resolution (critical prerequisite for all features)
- **Reconnection on error**: Retry with exponential backoff for network failures

**Should have (competitive advantages):**
- **No quota limits**: Unlimited polling (vs official API 1M units/day cap)
- **No OAuth required**: Simpler setup, no token refresh complexity
- **Message deletion events**: Detect moderator deletions (markChatItemAsDeletedAction)
- **Ban/timeout detection**: Detect user bans (markChatItemsByAuthorAsDeletedAction)
- **Lower latency**: Configurable polling (1-5s) vs quota-constrained official API

**Defer (explicitly avoid):**
- **Quota tracking database**: InnerTube has no quotas - remove quota_usage table, state machine, adaptive polling
- **OAuth token storage**: InnerTube works unauthenticated - remove youtube_oauth_tokens table, refresh logic
- **Cross-service quota coordination**: Remove /quota/record endpoint and YouTubeQuotaClient from overlay-manager integration

### Architecture Approach

**Pattern:** Async iterator-based stream handlers with state machine lifecycle management. Each video ID gets dedicated StreamHandler with masterchat connection, message normalizer, and Redis publisher. HTTP API layer receives start/stop commands from Go services (overlay-manager). Stream Manager orchestrates multiple concurrent handlers with health monitoring.

**Major components:**
1. **HTTP API Layer (Express)**: Control plane endpoints (POST /streams/monitor, DELETE /streams/:id, GET /status) called by overlay-manager
2. **Stream Manager**: Orchestrates Map<videoId, StreamHandler>, lifecycle management (start/stop/reconnect), health monitoring
3. **Stream Handler**: Per-stream state machine (INITIALIZING → RUNNING → STOPPED → ERROR), manages masterchat connection, error recovery with exponential backoff
4. **Masterchat Client**: Async iterator consuming InnerTube API (mc.iter()), filters 20+ action types
5. **Message Normalizer**: Maps masterchat actions to RawChatMessage schema, generates UUIDs, formats timestamps, extracts tags
6. **Redis Publisher (ioredis)**: XADD to chat:raw stream, connection pooling, retry logic

**Integration contract:** Output to Redis Streams chat:raw must be byte-for-byte identical to official youtube-listener for drop-in replacement. Message-processor consumes without code changes.

### Critical Pitfalls

1. **RawChatMessage schema drift** - InnerTube field names/types differ from official API (authorExternalChannelId vs authorDetails.channelId, nested structures, optional fields). Silent data loss: blank usernames, missing badges, type assertion panics. **Mitigation:** Contract tests with golden files (Phase 2), JSON schema validation built-in (Phase 1), cross-listener comparison (Phase 3).

2. **Deletion event semantic mismatch** - InnerTube deletion IDs may not match YouTube API message IDs stored in message-processor registry. Race condition: deletions arrive faster than original messages (InnerTube WebSocket-like vs API polling). Orphaned messages remain visible, batch deletions send 50 individual events overwhelming processor. **Mitigation:** Store InnerTube itemId → messageId mapping in Redis (Phase 1), deletion buffer for race conditions (Phase 1), batch deletion detection (Phase 2).

3. **InnerTube API instability** - Undocumented internal API changes without warning. Field renames, nested structure changes, continuation token format changes cause parser failures mid-stream. No changelog, no versioning, discovered via user reports. **Mitigation:** Schema version detection (Phase 1), graceful degradation on missing fields (Phase 1), snapshot tests (Phase 2), canary deployment 10% rollout (Phase 3), community monitoring (ongoing).

4. **Stream discovery edge cases** - Multiple concurrent streams, premieres vs live streams, unlisted streams. InnerTube may return different results than official API search.list. Listener connects to wrong stream, premiere countdowns mistaken for live chat. **Mitigation:** Stream filter contract matching official API behavior (Phase 1), premiere detection via upcomingEventData (Phase 1), multi-stream handling with viewer count sorting (Phase 1), cross-validation tests (Phase 2).

5. **Rate limiting differences** - InnerTube uses IP-based rate limiting (not quota). Aggressive polling triggers 429/403, IP blocks. Default 1000ms continuation vs official API 2000-5000ms. **Mitigation:** Respect timeoutMs field from InnerTube (Phase 1), exponential backoff on rate limit errors (Phase 1), configurable MIN_POLLING_INTERVAL_MS override (Phase 1).

## Implications for Roadmap

Based on research, suggested phase structure:

### Phase 1: Proof of Concept - Core Ingestion
**Rationale:** Validate InnerTube viability before investing in production hardening. Manual video ID input bypasses stream discovery complexity. Focus on message flow: InnerTube → Redis → message-processor compatibility.

**Delivers:**
- Node.js service with masterchat polling
- Message normalization to RawChatMessage format
- Redis Streams publishing (XADD chat:raw)
- Basic health checks (/health/live, /health/ready)
- Manual video ID configuration (hardcoded or env var)

**Addresses (from FEATURES.md):**
- Live chat ingestion (core purpose)
- Message normalization (table stakes)
- Super Chat/membership detection (must have)

**Avoids (from PITFALLS.md):**
- Schema drift: Build JSON schema validation into normalizer from day 1
- Rate limiting: Respect masterchat timeoutMs, implement exponential backoff
- Deletion semantics: Defer deletion events to Phase 2 (simplify MVP)

**Validation criteria:**
- Can receive live chat from known stream (hardcoded video ID)
- Message-processor consumes InnerTube messages without code changes
- Side-by-side comparison with official listener shows matching output

### Phase 2: Production Minimum - Dynamic Streams
**Rationale:** Add dynamic stream management and production hardening. Stream discovery is critical path - without it, service requires manual video ID updates. HTTP API enables overlay-manager integration.

**Delivers:**
- HTTP REST API (POST /streams/monitor, DELETE /streams/:id)
- Stream discovery via external resolution (overlay-manager provides video ID)
- Stream Manager with Map<videoId, StreamHandler>
- Reconnection logic with exponential backoff
- Graceful shutdown (SIGTERM handler, connection cleanup)
- Status endpoint (/status showing active streams)
- Structured logging (Winston JSON output, matches Go services)

**Addresses (from FEATURES.md):**
- Stream discovery (critical gap from Phase 1)
- Reconnection on error (table stakes)
- Graceful shutdown (K8s requirement)

**Uses (from STACK.md):**
- Express for HTTP server
- ioredis with connection pooling
- Winston for structured logging

**Implements (from ARCHITECTURE.md):**
- HTTP API Layer
- Stream Manager orchestrator
- Stream Handler state machine (INITIALIZING → RUNNING → STOPPED → ERROR)

**Avoids (from PITFALLS.md):**
- Deletion race conditions: Implement InnerTube itemId → messageId mapping in Redis
- Continuation token lifecycle: Store last continuation in Redis with TTL for fast resume

**Integration points:**
- overlay-manager calls innertube-listener HTTP API
- Docker Compose networking (service name as hostname)
- SERVICE_SECRET authorization (matches existing pattern)

### Phase 3: Contract Validation - Drop-In Testing
**Rationale:** Prove behavioral equivalence with official listener before production rollout. Contract tests catch schema drift early. Golden replay tests ensure byte-for-byte compatibility. Dual-listener validation detects real-time mismatches.

**Delivers:**
- Contract tests with golden files (100 diverse messages)
- Schema snapshot tests (detect InnerTube changes)
- Cross-listener comparison tests (same stream, both listeners)
- Deletion event validation (single + batch)
- Timestamp/badge/amount format validation
- Integration test: innertube-listener → message-processor → UnifiedChatMessage

**Addresses (from PITFALLS.md):**
- Schema drift detection (golden replay tests)
- Deletion semantic validation (comparison with official listener)
- Stream discovery validation (dual-discovery comparison)

**Testing strategy:**
1. Capture 100 messages from official listener (testdata/official_golden/)
2. Capture corresponding InnerTube responses (testdata/innertube_raw/)
3. Golden replay test: Parse InnerTube → RawChatMessage → assert.JSONEq with golden
4. Run both listeners on same live stream, compare output in real-time
5. Measure mismatch rate (target < 0.1%)

**Validation criteria:**
- Contract tests pass for all message types (regular, Super Chat, membership, deletion)
- Dual-listener mismatch rate < 0.1% over 1 week staging test
- Message-processor integration test passes (no code changes)

### Phase 4: Production Rollout - Canary Deployment
**Rationale:** InnerTube instability risk requires gradual rollout with automatic rollback. Canary deployment limits blast radius. Monitoring detects breaking changes early.

**Delivers:**
- Kubernetes deployment manifests (Deployment, Service, HPA)
- Canary deployment config (10% → 50% → 100%)
- Prometheus metrics (messages_total, streams_active, errors_total, parse_errors)
- Sentry error tracking with InnerTube-specific tags
- Automatic rollback on error spike (> 5% error rate)
- Community monitoring alerts (GitHub issues for masterchat/YouTube.js)

**Addresses (from PITFALLS.md):**
- InnerTube breaking changes: Canary limits damage, rollback prevents outage
- Schema changes: Metrics detect parse errors early
- Rate limiting: Monitor 429/403 responses, adjust polling intervals

**Rollout plan:**
1. Week 1: Deploy to 10% of channels, monitor error rates
2. Week 2: If error rate < 1%, scale to 50%
3. Week 3: If error rate < 0.5%, scale to 100%
4. Automatic rollback if error rate > 5% or mismatch rate > 1%

**Observability:**
- Grafana dashboard: message rate, active streams, error breakdown
- Sentry alerts: schema parsing failures, rate limiting events
- PagerDuty integration: automatic rollback triggers

### Phase 5: Feature Parity - Deletion Events & Metrics
**Rationale:** Deletion events and advanced features differentiate InnerTube listener. These are competitive advantages (message deletion not available in official API), not blockers.

**Delivers:**
- Deletion event publishing (markChatItemAsDeletedAction → Redis Pub/Sub)
- Batch deletion detection (5+ deletions in 100ms → synthesized batch event)
- Ban/timeout event publishing (markChatItemsByAuthorAsDeletedAction)
- Prometheus metrics expansion (message_rate gauge, error breakdown by type)
- Super sticker enrichment (full sticker metadata in event_data)
- Membership milestone parsing (extract months/years from message)

**Addresses (from FEATURES.md):**
- Message deletion events (differentiator)
- Ban/timeout detection (differentiator)
- Advanced metrics (production monitoring)

**Phase ordering rationale:**
- Deletion events deferred until schema validation proven (Phase 3)
- Metrics expansion after production rollout (Phase 4 provides baseline)
- These features enhance but don't block core functionality

### Phase Ordering Rationale

- **Dependencies:** Phase 1 validates InnerTube viability before building production features. Phase 2 adds dynamic stream management (prerequisite for real overlay use). Phase 3 proves contract compatibility before production. Phase 4 rolls out safely with monitoring.

- **Risk mitigation:** Gradual rollout limits InnerTube instability blast radius. Contract testing in Phase 3 catches schema drift before production. Canary deployment (Phase 4) enables fast rollback on breaking changes.

- **Architecture:** Phase 1 validates Redis contract (drop-in replacement). Phase 2 builds HTTP integration (matches overlay-manager pattern). Phase 3 tests message-processor compatibility (no code changes). Phase 4 integrates K8s (HPA, leader election via source-manager).

- **Pitfall avoidance:** Schema validation built-in Phase 1 (prevents silent data loss). Deletion ID mapping Phase 2 (prevents orphaned messages). Golden replay tests Phase 3 (catches schema drift). Canary deployment Phase 4 (limits InnerTube breaking change damage).

### Research Flags

Phases likely needing deeper research during planning:

- **Phase 2 (Stream Discovery):** Complex integration - how does overlay-manager resolve channel → video ID? Does it use official API (separate quota pool) or YouTube RSS (no auth, 5-15 min lag)? Need to design async flow to avoid circular dependency (listener needs video ID, overlay-manager needs stream ID from listener).

- **Phase 3 (Contract Testing):** Sparse documentation - InnerTube message deletion semantics not officially documented. Need to capture real deletion events from live streams, reverse-engineer itemId format. May require running test streams with moderator actions.

- **Phase 5 (Deletion Events):** Niche domain - message deletion not documented in official API (no liveChatMessages.retract endpoint). Need to verify message-processor and API Gateway can handle deletion events (may require schema changes).

Phases with standard patterns (skip research-phase):

- **Phase 1 (Core Ingestion):** Well-documented - masterchat library has production use cases (HolodexNet), async iterator pattern established. Node.js → Redis Streams via ioredis is standard pattern.

- **Phase 4 (Production Rollout):** Established patterns - Kubernetes canary deployment well-documented. Prometheus metrics follow existing service patterns (youtube-listener has examples). Sentry integration matches current setup.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | masterchat verified via npm package, GitHub activity, HolodexNet production use. Node.js 20 LTS + TypeScript + ioredis are industry-standard choices. Alternative options (Go native, pytchat Python) thoroughly evaluated and rejected with clear rationale. |
| Features | HIGH | Feature requirements derived from existing youtube-listener codebase (RawChatMessage contract, quota tracking to remove, OAuth to remove). Differentiators confirmed via masterchat action types (deletion/ban events). MVP scope validated against message-processor expectations. |
| Architecture | MEDIUM | Component boundaries follow existing All-Chat patterns (HTTP control plane, Redis Streams data plane, per-stream handlers). Integration points verified via Docker Compose networking docs. Uncertainty: source-manager leader election integration inferred from architecture but not verified in code. |
| Pitfalls | MEDIUM | Schema drift risk confirmed by examining RawChatMessage contract + InnerTube library differences. Deletion semantics verified via official API parser code. InnerTube instability based on community reports (GitHub issues, HN discussion) but no official changelog exists. Stream discovery edge cases inferred from behavior patterns, require validation testing. |

**Overall confidence:** MEDIUM

While stack choices and feature requirements are well-validated (HIGH confidence), architecture integration details and InnerTube-specific behavior have moderate uncertainty due to undocumented API and inferred patterns.

### Gaps to Address

- **Stream discovery implementation**: How overlay-manager resolves channel → video ID is unclear. Need to decide between Option A (overlay-manager uses official API), Option B (innertube-listener implements InnerTube browse API), or Option C (YouTube RSS feed). Recommend designing API contract first, implementation can be decided during Phase 2 planning.

- **Source-manager leader election integration**: Architecture assumes per-stream leader election to prevent duplicate ingestion across K8s replicas. Integration pattern inferred from existing youtube-listener but not verified. Need to review source-manager codebase during Phase 2 to confirm Redis lock protocol.

- **Deletion event schema**: Exact mapping from InnerTube markChatItemAsDeletedAction to RawChatMessage event_data schema unknown. Official listener deletion handling not found in codebase search. May need to define new schema or confirm message-processor supports deletion events. Validate during Phase 3 contract testing.

- **InnerTube schema versioning**: No official versioning exists. Recommend creating internal schema version detection based on field presence (e.g., "2024-schema" if continuationContents.liveChatContinuation exists). Document known schema patterns during Phase 1 implementation for future stability tracking.

- **Rate limiting thresholds**: Exact IP-based rate limits unknown (not documented by YouTube). Community reports vary (1000ms polling works, sub-500ms triggers blocks). Recommend starting conservative (2000ms min interval) and A/B testing in Phase 4 canary to find safe threshold.

## Sources

### Primary (HIGH confidence)
- [masterchat npm package](https://www.npmjs.com/package/@stu43005/masterchat) - Version 1.5.0 confirmed, published April 2025
- [masterchat documentation](https://github.com/sigvt/masterchat/blob/master/MANUAL.md) - Action types, async iterator API, error events
- [All-Chat youtube-listener source](file:///home/moersener/Hobby/all-chat/services/youtube-listener/) - RawChatMessage contract, quota tracking, OAuth patterns
- [All-Chat message-processor source](file:///home/moersener/Hobby/all-chat/services/message-processor/) - Normalization expectations, consumer patterns
- [Docker Compose networking docs](https://forums.docker.com/t/cross-container-communication-via-http-post-request/54605) - Service-to-service HTTP patterns

### Secondary (MEDIUM confidence)
- [pytchat GitHub](https://github.com/taizan-hokuto/pytchat) - Repository archived Jan 25, 2022 (3+ years unmaintained)
- [innertube-go pkg.go.dev](https://pkg.go.dev/github.com/nezbut/innertube-go) - No live chat methods listed, v0.0.0 unstable
- [YouTube.js GitHub](https://github.com/LuanRT/YouTube.js) - v16.0.1 active (Oct 2025), live chat support unclear from docs
- [chat-downloader InnerTube implementation](https://github.com/xenova/chat-downloader/blob/master/chat_downloader/sites/youtube.py) - Production-grade parser with deletion handling (Python reference)
- [YouTube IP ban behavior](https://multilogin.com/blog/youtube-ip-ban/) - IP-based rate limiting patterns (2026 guide)
- [Contract testing with Pact](https://medium.com/@mohsenny/stop-breaking-my-api-a-practical-guide-to-contract-testing-with-pact-33858d113386) - Testing strategies applicable to drop-in replacement

### Tertiary (LOW confidence)
- [InnerTube stability discussion](https://news.ycombinator.com/item?id=31021611) - Community reports on YouTube.js reliability (HackerNews, needs validation)
- [YouTube rate limiting discussion](https://github.com/jdepoix/youtube-transcript-api/issues/511) - Community reports of rate limiting (anecdotal)
- [InnerTube transcript API changes](https://medium.com/@aqib-2/extract-youtube-transcripts-using-innertube-api-2025-javascript-guide-dc417b762f49) - Recent breaking changes (Medium article, single source)

---
*Research completed: 2026-02-21*
*Ready for roadmap: yes*
