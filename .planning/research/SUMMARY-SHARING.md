# Project Research Summary

**Project:** All-Chat v1.3: Chat Overlay Sharing
**Domain:** Multi-platform streaming collaboration
**Researched:** 2026-03-08
**Confidence:** HIGH

## Executive Summary

Chat overlay sharing for All-Chat follows a proven microservices pattern: create a new `share-service` for permission management combined with extensions to existing message routing infrastructure. The feature enables bidirectional overlay sharing (both streamers share with each other) with flexible expiry options (time-based, stream lifecycle, unlimited) and premium enforcement. This differentiates from Twitch's unidirectional Shared Chat and Restream's link-based model by providing explicit consent-based collaboration with multi-platform aggregation.

The recommended approach requires minimal stack additions—only two new Go libraries (`nicklaw5/helix/v2` for Twitch stream status, `robfig/cron/v3` for time-based expiry). All other capabilities (premium enforcement, user search, message fan-out, Redis caching) leverage existing infrastructure. The architecture treats shared overlays as virtual platform sources, enabling reuse of source activation patterns and avoiding duplicate code paths.

Key risks center on permission edge cases and timing-sensitive operations. Client-side premium bypass, race conditions between revocation and message delivery, circular share dependencies, and OAuth token revocation without share cleanup are the highest-severity pitfalls. All are preventable through server-side enforcement, Redis cache invalidation, cycle detection, and health monitoring respectively. The critical insight: shares are permission grants first, message routing second—permission verification must happen at every layer.

## Key Findings

### Recommended Stack

The existing All-Chat infrastructure handles 90% of requirements. Only two new libraries are needed for share-specific capabilities:

**New dependencies:**
- **github.com/nicklaw5/helix/v2** — Twitch stream status detection for "this stream only" expiry (official client, 2.5k stars, battle-tested)
- **github.com/robfig/cron/v3** — Time-based expiry job scheduler (13k stars, industry standard, zero external dependencies)

**Extend existing capabilities:**
- **PostgreSQL 16 (pgx/v5)** — Add `shares` table, `users.is_premium` flag (JSONB for premium features, existing transaction patterns)
- **Redis 7 (go-redis/v9)** — Premium status caching (5min TTL), share permission caching (1s TTL), Pub/Sub for cache invalidation
- **YouTube/TikTok lifecycle** — Already tracked by existing listeners, no changes needed
- **Kick stream status** — Research gap: webhook subscription patterns need implementation-phase investigation (unofficial API)

**Explicitly avoid:**
- External feature flag services (LaunchDarkly, Unleash) — overkill for single boolean flag, use database JSONB + Redis cache
- Separate expiry microservice — adds complexity, collocate cron job in share-service
- GraphQL for user search — over-engineered for single-field prefix matching
- Redis Streams for share events — Pub/Sub sufficient for cache invalidation

### Expected Features

Research reveals a clear feature hierarchy aligned with SaaS collaboration patterns:

**Must have (table stakes):**
- User search by platform username (Twitch, YouTube, Kick, TikTok) — discovery mechanism
- Send/accept share requests with explicit consent — follows Slack/AWS invitation patterns
- Bidirectional sharing (mutual overlay access) — differentiates from unidirectional guest models
- Display settings isolation — destination overlay's CSS applies, not source's settings
- Manual revocation by either party — standard security/privacy expectation
- Shared overlay source type — new virtual platform that delivers aggregated chat
- Premium enforcement with admin override — first premium feature for All-Chat

**Should have (competitive):**
- Flexible expiry options (this stream, time-based, unlimited) — most competitors offer session-only or unlimited
- Stream lifecycle awareness — auto-expiry when stream ends (Twitch/YouTube tracked, Kick gap)
- Inactive source marking (not deletion) — preserves configuration history like Microsoft 365
- Multi-source overlay sharing — share overlays aggregating Twitch + YouTube + Kick (not just single platform)

**Defer (v2+):**
- Share request expiration (currently persist indefinitely) — add 7-day expiry in v1.4
- Notification system for share events — improves awareness, defer until core validated
- Share renewal workflow — easier to extend expired share, defer until usage patterns emerge
- Share analytics/metrics — privacy concerns, trust-based model preferred

**Anti-features (explicitly reject):**
- Public overlay directory — moderation burden, copyright risk, DMCA exposure
- Automatic share acceptance — violates consent model, security risk
- Share settings inheritance — breaks user control, creates CSS conflicts
- Unlimited free sharing — eliminates monetization, reduces value perception
- Cross-platform relay (A→B→C chaining) — permission complexity, amplifies load

### Architecture Approach

Hybrid pattern: new share-service for permission management, extend existing services for message delivery. Treats shares as virtual platform sources to reuse activation logic.

**Major components:**

1. **share-service (NEW)** — Share CRUD, lifecycle management, user search, premium validation, expiry cron job (~2,000 LOC)
   - Endpoints: POST /shares, PUT /shares/:id/accept, DELETE /shares/:id, GET /users/search
   - Cron: Time-based expiry every 5 minutes (robfig/cron/v3)
   - Lifecycle: Subscribe to stream status events (Redis Pub/Sub)

2. **overlay-manager (EXTEND)** — Add share source validation, expose "share" as virtual platform
   - New source type: platform="share", channel_id=share_id
   - Validation: Call share-service to verify share exists and is active before adding source
   - Config: {source_overlay_id, source_user_id, share_id}

3. **message-processor (EXTEND)** — Resolve shares, fan-out messages to consuming overlays
   - Share resolver: Cache active shares in Redis (5min TTL), invalidate on revocation events
   - Modified publisher: Publish to source overlay + all consuming overlays (batch Redis PUBLISH)
   - Permission check: Verify share active BEFORE enrichment (not after)

4. **source-manager (EXTEND)** — Publish stream lifecycle events for expiry detection
   - Pattern: `PUBLISH lifecycle:twitch:12345:offline {timestamp}`
   - Twitch: Poll Helix API every 60s for shares with expires_on_stream_end=true
   - YouTube/TikTok: Already tracked, forward lifecycle events

**Data flow:**
```
Share Request → share-service validates premium → PostgreSQL stores pending
Accept → share-service creates bidirectional entries → overlay-manager allows add as source
Message arrives → message-processor resolves shares → fan-out to consuming overlays
Stream ends → source-manager publishes lifecycle event → share-service expires shares
```

**Database schema:**
- `shares` table: requester_user_id, recipient_user_id, overlay_ids, status (pending/active/expired/revoked), expiry_type, expires_at
- `users.is_premium` flag: Boolean for premium enforcement (JSONB premium_features column for future expansion)
- Foreign keys: ON DELETE RESTRICT (not CASCADE) — soft delete pattern, application-level cascade

**Critical patterns:**
- **Database-per-service:** share-service owns `shares` table, other services query via HTTP API (no direct joins)
- **Message fan-out at processor:** Publish once to multiple overlays (avoid duplicate enrichment)
- **Cache invalidation:** share-service publishes `share:revoked:{id}` on Redis Pub/Sub, processor invalidates cache
- **No nested shares:** 1-layer depth maximum (shares can't contain other shares) to prevent permission cascading

### Critical Pitfalls

Research identified 10 critical pitfalls with proven mitigation strategies:

1. **Client-side premium bypass** — Most severe revenue risk. Mitigation: Server-side enforcement on every API endpoint, audit logging, automated tests with direct API calls
2. **Permission cascade without verification** — Messages flow through share chains without consent. Mitigation: Prohibit sharing overlays that contain shares (1-layer depth), explicit permission model
3. **Revocation race conditions** — 100-500ms window where messages continue flowing. Mitigation: Redis Pub/Sub cache invalidation, permission checks before enrichment, replay buffer metadata
4. **Stream lifecycle inconsistency** — Works for YouTube, sporadic for Twitch, fails for Kick. Mitigation: Platform capability flags, 24-hour backstop expiry, polling hybrid for Twitch
5. **Soft delete with CASCADE constraints** — Database errors on deactivation. Mitigation: ON DELETE RESTRICT foreign keys, application-level cascade logic
6. **Circular share dependencies** — A shares to B, B shares back to A creates message loops. Mitigation: Cycle detection with DFS before acceptance, message ID deduplication
7. **Timezone DST errors** — ±1 hour expiry errors around daylight saving transitions. Mitigation: UTC everywhere, store timestamps as TIMESTAMPTZ, client sends durations not absolute times
8. **OAuth revocation without share cleanup** — Shares remain active after token invalid. Mitigation: Periodic health checks, listener publishes oauth:revoked events
9. **Replay buffer desynchronization** — Reconnection replays stale permissions. Mitigation: Invalidate buffer on source changes, store source_id with messages
10. **Incomplete permission testing** — Edge cases missed in test suite. Mitigation: State machine testing, property-based tests, edge case matrix (90%+ coverage)

**Phase-critical pitfalls:**
- Phase 1: Premium bypass (#1), soft delete CASCADE (#5) — must be correct from foundation
- Phase 2: Permission cascade (#2), circular dependencies (#6), replay buffer (#9) — design acceptance model to prevent
- Phase 4: Revocation race (#3) — implement cache invalidation before revocation
- Phase 5: Stream lifecycle inconsistency (#4), timezone errors (#7) — platform research and UTC from start

## Implications for Roadmap

Based on research, suggested 6-phase structure aligned with dependencies and risk mitigation:

### Phase 1: Foundation (Database + Share Service Core)
**Rationale:** Premium enforcement and schema must be correct from Day 1 (pitfalls #1, #5 are non-recoverable without migration). Share CRUD is independent of message delivery, can validate in isolation.

**Delivers:**
- Database migration: `shares` table, `users.is_premium` flag
- share-service: CRUD endpoints, premium validation
- User search by platform username
- API Gateway routing

**Features from FEATURES.md:** Send share requests, view pending requests, server-side premium enforcement, admin override

**Avoids pitfalls:** Client-side premium bypass (#1), soft delete CASCADE (#5)

**Stack elements:** PostgreSQL (pgx/v5) for shares table, Redis caching for premium status (5min TTL)

**Research flag:** Standard CRUD patterns, skip deep research

### Phase 2: Share Acceptance (Permission Model)
**Rationale:** Permission model must prevent cascading and cycles before message delivery exists (pitfalls #2, #6). Acceptance logic defines bidirectional relationships, order matters.

**Delivers:**
- Accept share request endpoint
- Bidirectional share creation
- Cycle detection (DFS validation)
- Replay buffer invalidation on source changes

**Features:** Accept/decline share requests, bidirectional sharing, overlay selection on accept

**Avoids pitfalls:** Permission cascade (#2), circular dependencies (#6), replay buffer desync (#9)

**Architecture components:** share-service acceptance logic, overlay-manager validation

**Research flag:** Graph algorithms (cycle detection) — standard pattern, low risk

### Phase 3: Message Routing (Actual Chat Delivery)
**Rationale:** Message delivery depends on permission model (Phase 2). Fan-out pattern is core value delivery, must be optimized early (foundation for performance).

**Delivers:**
- message-processor share resolver (Redis cache)
- Modified publisher with fan-out to consuming overlays
- Share source type in overlay-manager
- WebSocket delivery to consuming overlays

**Features:** Shared overlay source type, display settings isolation, messages appear in both overlays

**Uses stack:** Redis caching (5min TTL for shares), Pub/Sub for cache invalidation

**Implements architecture:** Message fan-out at processor layer, permission checks before enrichment

**Research flag:** Standard message routing patterns, well-documented

### Phase 4: Revocation (Security Operations)
**Rationale:** Revocation must handle cache invalidation and race conditions (pitfall #3). Can't safely launch without revocation (users need escape hatch).

**Delivers:**
- Manual revocation endpoint
- Cache invalidation via Redis Pub/Sub
- Permission verification in message processor
- Inactive source marking (not deletion)

**Features:** Manual revocation by either party, inactive source marking

**Avoids pitfalls:** Revocation race conditions (#3) via cache invalidation

**Research flag:** Redis Pub/Sub patterns — existing infrastructure, low risk

### Phase 5: Lifecycle & Expiry (Automation)
**Rationale:** Expiry automation is premium feature differentiator but depends on platform capabilities (pitfall #4). Time zone handling (pitfall #7) must be correct from start.

**Delivers:**
- Time-based expiry cron job (robfig/cron/v3)
- Stream lifecycle detection (Twitch polling, YouTube existing)
- Platform capability flags (disable "this stream" for Kick)
- UTC timestamp normalization

**Features:** Flexible expiry options (this stream, time-based, unlimited), stream lifecycle awareness

**Stack elements:** robfig/cron/v3 (@every 5m), nicklaw5/helix/v2 (Twitch stream status)

**Avoids pitfalls:** Stream lifecycle inconsistency (#4), timezone DST errors (#7)

**Research flag:** Kick stream status API needs implementation-phase research (unofficial API, MEDIUM confidence)

### Phase 6: Health Monitoring (Production Readiness)
**Rationale:** OAuth revocation cleanup (pitfall #8) is non-critical for MVP but required for production. Health monitoring prevents silent failures.

**Delivers:**
- Periodic share health checks (15-minute interval)
- OAuth token validation for share sources
- Listener failure notifications (oauth:revoked events)
- Share deactivation on token invalid

**Features:** (Non-user-facing) — prevents confusion from silent failures

**Avoids pitfalls:** OAuth revocation without cleanup (#8)

**Research flag:** OAuth token validation patterns — standard, low risk

### Phase Ordering Rationale

- **Phases 1-2 before 3:** Permission model must be correct before message delivery to prevent cascading/cycles (non-recoverable without data migration)
- **Phase 4 before launch:** Revocation is security requirement, can't safely launch without escape hatch
- **Phase 5 before production:** Expiry automation is premium feature differentiator, required for "this stream only" value prop
- **Phase 6 post-MVP:** Health monitoring improves reliability but not blocking for initial validation

**Dependency chain:**
```
Phase 1 (Foundation) → Phase 2 (Acceptance) → Phase 3 (Message Routing)
                                            ↓
Phase 4 (Revocation) ← Phase 3             ↓
                                            ↓
Phase 5 (Expiry) ← Phase 4                 ↓
                                            ↓
Phase 6 (Health) ← Phase 5
```

### Research Flags

**Phases needing deeper research during planning:**
- **Phase 5 (Lifecycle & Expiry):** Kick stream status API — unofficial, webhook subscription patterns need investigation (MEDIUM confidence). Recommend `/gsd:research-phase` before implementation.

**Phases with standard patterns (skip research-phase):**
- **Phase 1 (Foundation):** Standard CRUD, PostgreSQL patterns, premium flag enforcement — well-documented
- **Phase 2 (Acceptance):** Graph cycle detection (DFS) is textbook algorithm, bidirectional relationships are standard
- **Phase 3 (Message Routing):** Redis Pub/Sub fan-out is existing pattern in All-Chat, extend not invent
- **Phase 4 (Revocation):** Cache invalidation via Pub/Sub is established pattern
- **Phase 6 (Health Monitoring):** OAuth token validation is standard, periodic job patterns proven

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| **Stack** | HIGH | Minimal additions (2 libraries), existing infrastructure handles most requirements. nicklaw5/helix and robfig/cron are battle-tested with 13k+ combined stars. |
| **Features** | HIGH | Competitor analysis comprehensive (Twitch Shared Chat, Restream Pairs, StreamElements), SaaS collaboration patterns well-documented (Slack, AWS, Microsoft 365). |
| **Architecture** | HIGH | Extends existing All-Chat patterns (virtual platform sources, Redis Pub/Sub, microservices), no novel patterns required. |
| **Pitfalls** | HIGH | Sourced from 2026 production security research, real-world CVEs (CVE-2026-29061), platform comparisons. All critical pitfalls have proven mitigation strategies. |

**Overall confidence:** HIGH

### Gaps to Address

Areas where research was inconclusive or needs validation during implementation:

- **Kick stream lifecycle detection:** Unofficial API, webhook subscription patterns unclear. Confidence: MEDIUM. **Mitigation:** Research during Phase 5 planning, consider disabling "this stream" expiry for Kick if webhook reliability unknown (fallback: time-based only).

- **Share fan-out amplification limits:** Popular streamer (100K viewers) sharing with 1,000 streamers could approach Redis Pub/Sub capacity (500K msg/s). **Mitigation:** Cap shares per overlay (10 active shares), hot overlay detection (>100 shares flagged for review), rate limiting (5 share requests/hour).

- **OAuth token revocation propagation delay:** Time between Google revokes token and All-Chat detects varies by refresh cycle (current: 15min). **Mitigation:** Phase 6 health checks reduce detection time, acceptable for v1.3.

- **Message deduplication at scale:** Redis SET for seen message IDs works up to ~1M active messages (60s TTL). **Validation needed:** Load testing with >10K concurrent shares to verify deduplication performance.

- **Share request expiration strategy:** Research shows 7-day expiry (AWS, Microsoft) vs 30-day (Slack). **Decision deferred:** v1.3 launches without expiration, v1.4+ adds based on usage patterns (prevent stale request backlog).

## Sources

### Primary (HIGH confidence)

**Stack:**
- [nicklaw5/helix GitHub](https://github.com/nicklaw5/helix) — Twitch Helix API client, 2.5k stars, official Go client
- [robfig/cron GitHub](https://github.com/robfig/cron) — Cron library for Go, 13k stars, industry standard
- [PostgreSQL JSONB documentation](https://www.postgresql.org/docs/16/datatype-json.html) — Premium features JSONB column
- [Redis TTL/EXPIRE commands](https://redis.io/docs/latest/commands/expire/) — Official Redis TTL patterns

**Features:**
- [Twitch Shared Chat Help](https://help.twitch.tv/s/article/shared-chat?language=en_US) — Official Twitch collaboration model
- [Restream Pairs](https://support.restream.io/en/articles/11726283-what-is-restream-pairs) — Bidirectional guest channel sharing
- [SharePoint Guest Access Expiration](https://www.sharepointdiary.com/2021/08/guest-user-access-expiration-in-sharepoint-online-onedrive.html) — Guest access thresholds (1-365 days)
- [AWS Resource Share Invitations](https://docs.aws.amazon.com/ram/latest/userguide/working-with-shared-invitations.html) — Accept/reject workflow (7-day expiry)

**Architecture:**
- [Microservices Pattern: Database per service](https://microservices.io/patterns/data/database-per-service.html) — Service data ownership
- [Microsoft: Data Considerations for Microservices](https://learn.microsoft.com/en-us/azure/architecture/microservices/design/data-considerations) — Cross-service communication

**Pitfalls:**
- [CVE-2026-29061: Gokapi privilege escalation](https://advisories.gitlab.com/pkg/golang/github.com/forceu/gokapi/CVE-2026-29061/) — Permission revocation edge case (2026)
- [OAuth 2.0 Token Revocation (RFC 7009)](https://datatracker.ietf.org/doc/html/rfc7009) — Token revocation cascade
- [CWE-602: Client-Side Enforcement](https://cwe.mitre.org/data/definitions/602.html) — Client-side security anti-pattern
- [OWASP Authorization Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authorization_Cheat_Sheet.html) — Authorization edge cases

### Secondary (MEDIUM confidence)

**Stack:**
- [OneUpTime: Redis Key Expiration](https://oneuptime.com/blog/post/2026-01-25-redis-key-expiration-effectively/view) — Redis TTL patterns (2026 blog)
- [Citus Data: Five Ways to Paginate in Postgres](https://www.citusdata.com/blog/2016/03/30/five-ways-to-paginate/) — Offset vs cursor pagination

**Features:**
- [Streamlabs Shared Twitch Chat](https://streamlabs.com/content-hub/post/streamlabs-desktop-twitch-shared-chat) — Twitch native sharing implementation
- [Freemium Paywalls | RevenueCat](https://www.revenuecat.com/docs/playbooks/guides/freemium) — Freemium paywall patterns

**Pitfalls:**
- [Handling Race Conditions in Real-Time Apps](https://dev.to/mattlewandowski93/handling-race-conditions-in-real-time-apps-49c8) — Event cache pattern (2026)
- [WebSockets: The Complete Guide for 2026](https://devtoolbox.dedyn.io/blog/websocket-complete-guide) — Sub-10ms latency patterns
- [Foreign Keys vs Performance: CASCADE DELETE](https://medium.com/@thyagodoliveiraperez/foreign-keys-vs-performance-part-3-the-cascade-delete-story-aac5cabd843b) — Soft delete performance (2026)

### Tertiary (LOW confidence, needs validation)

**Kick API:**
- [Kick API MCP Integration](https://lobehub.com/mcp/nosytlabs-kickmcp) — Webhook-based stream lifecycle (unofficial, third-party documentation)

**Testing:**
- [Day 13: The Replay Buffer](https://javatsc.substack.com/p/day-13-the-replay-buffer-engineering) — Per-user replay buffer architecture (2026 blog)

---
*Research completed: 2026-03-08*
*Ready for roadmap: yes*
