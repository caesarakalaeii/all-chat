# All-Chat

## What This Is

All-Chat is a cloud-native platform that aggregates live chat messages from multiple streaming platforms (Twitch, YouTube, Kick, TikTok) and displays them in unified overlays for streamers. The platform features intelligent load distribution with hybrid hash-based sharding, automatic rebalancing, and quota-free YouTube ingestion via InnerTube API.

## Core Value

Streamers can aggregate chat from all platforms they stream to, with reliable message delivery even during high-traffic events through intelligent load balancing, auto-scaling, and unlimited YouTube chat access.

## Current Milestone: v1.5 Discord Listener

**Goal:** Add Discord as a bidirectional chat source — read from Discord channels into overlays, and relay overlay messages back to Discord with loop-safe filtering.

**Target features:**
- Discord-to-overlay ingestion: managed OAuth2 bot, channel-level source model
- Overlay-to-Discord relay: configurable per-source outbound channel, Discord messages filtered to prevent loops
- OAuth2 "Add to Server" bot authorization (consistent with Twitch/YouTube auth UX)
- Comprehensive setup UI (server connect, channel picker, relay config, overlay editor integration)
- Full load balancing with hash-based sharding + HPA (consistent with other listeners)

## Current State (v1.3 shipped 2026-03-14)

Viewer identity system shipped: OAuth from browser extension, platform linking, global name color/gradient editor, avatar frame/flair cosmetics, All-Chat platform badges, and YouTube InnerTube badge/emote enrichment. All 33 requirements satisfied across 6 phases.

## Requirements

### Validated

<!-- Shipped and confirmed valuable -->

**Infrastructure (existing):**
- ✓ Multi-platform chat aggregation (Twitch, YouTube, Kick, TikTok) — existing
- ✓ Real-time message delivery via WebSocket to overlays — existing
- ✓ Message normalization across platforms (unified schema) — existing
- ✓ Emote enrichment (7TV, BTTV, FFZ, platform-native) — existing
- ✓ Overlay configuration with multi-source support — existing
- ✓ OAuth authentication for platform access — existing
- ✓ Redis Streams for durable message queuing — existing
- ✓ Redis Pub/Sub for real-time broadcast to overlays — existing
- ✓ Microservices architecture (Standard Go Layout) — existing
- ✓ Kubernetes-deployable with health checks and graceful shutdown — existing

**Message Deletion (v1.0):**
- ✓ Message deletion support (Twitch, YouTube, Kick) — v1.0
- ✓ Message ID tracking and registry — v1.0
- ✓ Deletion event propagation pipeline — v1.0
- ✓ WebSocket reconnection replay buffer — v1.0

**Load Balancing (v1.1):**
- ✓ Hash-based channel sharding (consistent hashing across pods) — v1.1
- ✓ Per-pod load metrics collection (messages/sec, channel count) — v1.1
- ✓ Load imbalance detection (threshold-based triggers) — v1.1
- ✓ Automatic hot channel identification (high message rate detection) — v1.1
- ✓ Graceful channel migration (zero message loss during rebalancing) — v1.1
- ✓ Channel assignment registry (Redis-based, pod → channels mapping) — v1.1
- ✓ Rebalancing coordinator (leader-elected, triggers migrations) — v1.1
- ✓ Load balancing across all 4 listener types (Twitch, YouTube, Kick, TikTok) — v1.1
- ✓ Kubernetes HPA integration (scale based on aggregate load) — v1.1
- ✓ Load balancing observability (metrics, dashboards, alerts) — v1.1

**InnerTube YouTube Listener (v1.2):**
- ✓ InnerTube API integration (quota-free YouTube chat ingestion) — v1.2
- ✓ Drop-in replacement architecture (RawChatMessage schema compatibility) — v1.2
- ✓ All event types (messages, Super Chat, Super Sticker, memberships, deletions) — v1.2
- ✓ Batch deletion detection (5+ deletions in 100ms time window) — v1.2
- ✓ Deletion event buffering (500ms delay for race condition handling) — v1.2
- ✓ Dynamic stream management (discovery, lifecycle, offline detection) — v1.2
- ✓ Source-manager integration (leadership coordination) — v1.2
- ✓ Contract validation infrastructure (golden files, dual-listener test) — v1.2
- ✓ Production rollout infrastructure (Argo Rollouts, canary deployment) — v1.2
- ✓ Advanced metrics (per-channel message rate, network error classification) — v1.2

**Frontend Redesign (v1.3):**
- ✓ Design token system (Tailwind v4 @theme, three-tier hierarchy, cascade layers) — v1.3
- ✓ Static platform color map (no dynamic class construction) — v1.3
- ✓ Overlay CSS stability contract (events.css public API, EVENTS_CSS_API.md) — v1.3
- ✓ @base-ui/react + shadcn/ui component library (Button, Card, Input, Badge, Dialog, Toast, Skeleton) with CVA — v1.3
- ✓ All 6 pages redesigned in streaming dark aesthetic — v1.3
- ✓ Draggable SplitView component for live overlay preview — v1.3
- ✓ ESLint v10 flat config + Prettier with Tailwind class ordering (enforced) — v1.3
- ✓ Husky pre-commit hooks (lint-staged + tsc --noEmit) — v1.3
- ✓ 7-gate CI pipeline with Chromatic visual regression and 45/45 Storybook tests — v1.3
- ✓ Marketplace CSS migration guide (cascade layer upgrade path) — v1.3

### Active

<!-- Current scope: v1.5 Discord Listener -->

- [ ] Discord bot reads from a configured channel and pushes messages to overlays as a first-class source
- [ ] Overlay messages (Discord excluded) are relayed to a user-configured Discord outbound channel
- [ ] Loop-safe filtering: Discord-sourced messages are never echoed back to Discord
- [ ] OAuth2 "Add to Server" flow to authorize the bot in a user's Discord server
- [ ] Setup UI: server connection, inbound channel picker, outbound channel picker, relay toggle
- [ ] Discord source integrated in overlay editor alongside Twitch/YouTube/Kick/TikTok
- [ ] discord-listener service with full load balancing (hash-based sharding + HPA)
- [ ] Architecture decision: single service for inbound+outbound vs separate relay service

### Out of Scope

- Cross-region load balancing — Single Kubernetes cluster sufficient for current scale
- Predictive scaling based on stream schedules — Reactive scaling meets needs
- Custom load metrics from user configuration — Message rate provides accurate signal
- Channel affinity/pinning — Rebalancing flexibility more valuable than pinning
- Multi-tenancy isolation — Single-tenant deployment model
- Event suppression (emit 1 batch event vs N events) — Deferred to future (detection working, metadata provided)
- Self-hoster migration guide — Canary deployment guide exists, standalone migration deferred

## Context

**Current State (v1.3 shipped 2026-03-14):**
- 7 core microservices deployed in Kubernetes (api-gateway, auth-service, emote-service, message-processor, overlay-manager, source-manager, token-refresh-service)
- 5 listener services: 4 with load balancing (twitch-listener, kick-listener, tiktok-listener, youtube-listener) + 1 quota-free InnerTube listener (youtube-listener-innertube, ready for canary deployment)
- Frontend: Next.js 14 App Router, Tailwind v4, @base-ui/react, Storybook 10, ~18,833 LOC TypeScript/TSX
- CI/CD: 7-gate GitHub Actions pipeline with Chromatic visual regression baseline established
- ESLint v10 flat config + Prettier enforced via pre-commit hooks and CI

**Load Balancing Implementation (v1.1):**
- **Coordinator:** Kubernetes Lease-based leader election, bounded-load consistent hashing (1.25x average limit)
- **Migration:** Overlap protocol (new pod connects before old disconnects), zero-loss guarantee via first-message confirmation
- **Rebalancing:** Composite load scoring (70% message rate + 30% channel count), automatic hot channel redistribution (>3x average rate)
- **Safeguards:** 5-minute cooldown, 20% per-operation limit, thrashing detection, distributed lock coordination
- **HPA:** Startup jitter (0-30s) prevents thundering herd, filtered assignment count for readiness probes
- **Observability:** 16 distributed tracing spans, Grafana dashboards with Pod×Platform and Pod×Time heatmaps, Prometheus alerts

**InnerTube Listener Architecture (v1.2):**
- **API Integration:** InnerTube HTTP POST client eliminates YouTube API quota constraints (10,000 units/day → unlimited)
- **Schema Compatibility:** RawChatMessage byte-for-byte compatible with official listener, zero downstream changes
- **Event Support:** All chat types (regular messages, Super Chat, Super Sticker, membership events, tickers, deletions)
- **Batch Detection:** Time-windowed aggregation (5+ deletions in 100ms) with reason classification (timeout vs ban)
- **Deletion Buffer:** 500ms delay before emission handles race conditions (deletion arrives before original message)
- **Lifecycle Management:** Stream discovery (HTML parsing), offline detection (empty continuations), auto-resume, graceful shutdown
- **Testing:** 69 automated tests (contract validation, lifecycle behaviors, deletion events), 24-hour dual-listener test infrastructure

**Technical Environment:**
- Go 1.25.6 microservices with Standard Go Layout
- Redis 7 (Streams for queuing, Pub/Sub for broadcast, assignment registry)
- Kubernetes with CloudNativePG (PostgreSQL), HPA, Horizontal Pod Autoscaler
- OpenTelemetry tracing, Prometheus metrics, Grafana dashboards
- Argo Rollouts for canary deployments (InnerTube listener)

**Known Issues / Technical Debt:**
- Health handler test mock signature needs fix (Phase 9 minor gap, non-blocking)
- Self-hoster migration guide incomplete (canary deployment covered, standalone path deferred)
- Event suppression architecture deferred (emits N events where 5th is tagged 'batch', not 1 batch event)
- Argo Rollouts CRD not installed in cluster yet (prerequisite for InnerTube canary deployment)
- Human verification pending: 24-hour dual-listener test, Grafana dashboard rendering, Kubernetes dry-run validation
- Twitch design token (`#A37BFF`) lightened for WCAG AA contrast — differs from Twitch brand purple (#9146FF)

## Constraints

- **Tech Stack**: Go 1.25.6, existing microservices, Redis 7, Kubernetes — No new infrastructure dependencies
- **Backward Compatibility**: Must not break existing message flow — Single-pod deployments still supported
- **Zero Downtime**: Channel migration must be lossless — Enforced via overlap protocol and confirmation signaling
- **Platform Connection Limits**: Respect platform rate limits and connection quotas — InnerTube bypasses YouTube quota
- **Stateless Services**: Listeners remain stateless — All state in Redis, not in-memory

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Hybrid hash-based + load-aware approach | Predictable under normal load, adapts to real-world usage patterns | ✓ Good — v1.1 delivers both predictability and load awareness |
| Consistent hashing for channel assignment | Minimizes reassignments when pods scale, predictable placement | ✓ Good — CRC32 hash with virtual nodes, bounded-load (1.25x) |
| Redis for assignment registry | Centralized state, atomic updates, survives pod restarts | ✓ Good — O(1) lookups, O(log N) load queries via sorted sets |
| Reuse source-manager for coordination | Already has leader election, knows active sources, avoid duplication | ✓ Good — Extended with coordination logic, no new service needed |
| Kubernetes Lease-based leader election | Built-in fencing via resourceVersion, automatic leader failover | ✓ Good — Prevents split-brain, tested with chaos scenarios |
| Overlap migration protocol | Zero message loss guarantee, confirmation signaling | ✓ Good — First-message confirmation via channel signaling |
| Composite load scoring (70% message rate + 30% channel count) | Message processing dominates CPU, channel count matters for memory | ✓ Good — Validated via Prometheus query analysis |
| 5-minute rebalancing cooldown | Prevents thrashing while allowing timely redistribution | ✓ Good — Thrashing detection (>5 in 30min) as backstop |
| 20% per-operation migration limit | Balance between quick redistribution and system stability | ✓ Good — Minimum 1 channel enforced, proportional strategy |
| Startup jitter (0-30s random delay) | Prevents thundering herd during HPA scale-up | ✓ Good — Applied across all listeners, prevents coordinator overload |
| W3C Trace Context propagation through Redis | Standard propagation format, interoperable with observability tools | ✓ Good — 16 spans instrumented, trace context in Redis Streams |
| **InnerTube API over official YouTube API** | Eliminates quota constraints (10,000 units/day bottleneck), faster deletion detection | ✓ Good — Quota-free unlimited access, drop-in replacement architecture proven |
| **Drop-in replacement strategy (RawChatMessage compatibility)** | Zero downstream changes, maintains contract with message-processor | ✓ Good — Byte-for-byte schema match, contract validation tests pass |
| **Batch deletion time-window aggregation (100ms)** | Balances detection speed with false positive risk | ✓ Good — Detects bans/timeouts reliably, 5+ threshold works well |
| **Deletion buffer with 500ms delay** | Handles race condition (deletion arrives before original message) | ✓ Good — Buffering works, suppression architecture deferred to future |
| **Argo Rollouts for canary deployment** | Automatic promotion/rollback based on Prometheus metrics | ✓ Good — Infrastructure ready, 10%→50%→100% progression with <1% error threshold |
| **HTML parsing for stream discovery** | InnerTube browse endpoint unstable, HTML canonical link reliable | ✓ Good — Premiere filtering works, 15-minute timeout appropriate |
| **Source-manager leadership coordination** | Reuse existing leader election, single source of truth for active streams | ✓ Good — Async discovery, Redis caching, graceful lifecycle management |
| **Tailwind v4 @theme directive for design tokens** | Native CSS custom properties, zero runtime overhead, JIT-safe | ✓ Good — Three-tier hierarchy works cleanly, token naming consistent |
| **@base-ui/react over Radix UI** | Unstyled primitives with first-class CSS custom properties, better Tailwind v4 alignment | ✓ Good — Clean integration, no className prop conflicts |
| **Static PLATFORM_COLORS map over dynamic class construction** | Tailwind JIT requires static string analysis; dynamic `bg-${platform}` breaks JIT | ✓ Good — Required approach for Tailwind v4 compatibility |
| **Cascade layers for events.css** | Eliminates all !important while preserving marketplace theme override priority | ✓ Good — All 14 !important removed, marketplace-themes layer positions correctly |
| **ESLint v10 flat config (eslint.config.mjs)** | v9 .eslintrc.json silently ignored by ESLint v10; flat config required | ✓ Good — 345 pre-existing violations fixed, both linters exit 0 |
| **Husky v9 new-style hooks (no husky.sh sourcing)** | Deprecated v8 sourcing logs deprecation warnings in v9, fails in v10 | ✓ Good — Clean hook execution, verified tsc + lint-staged exit 0 |

---
*Last updated: 2026-03-15 after v1.5 milestone started*
