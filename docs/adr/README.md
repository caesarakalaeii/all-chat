# Architecture Decision Records (ADRs)

This directory contains Architecture Decision Records (ADRs) documenting significant architectural decisions made for All-Chat.

---

## What are ADRs?

An Architecture Decision Record (ADR) captures a **single architectural decision** along with its context, alternatives considered, and consequences. ADRs are immutable once accepted - if a decision changes, a new ADR supersedes the old one.

**Benefits**:
- **Historical context**: Understand WHY decisions were made
- **Onboarding**: New team members learn architectural rationale
- **Avoid rework**: Prevent revisiting already-rejected alternatives
- **Accountability**: Clear decision ownership and timeline

---

## When to Create an ADR

Create an ADR for decisions that:
- ✅ Are **architecturally significant** (affect multiple services, infrastructure, or patterns)
- ✅ Have **multiple viable alternatives** (trade-offs between approaches)
- ✅ Are **hard to reverse** (database choice, deployment platform, message queue)
- ✅ Will be **questioned in the future** ("Why did we choose X over Y?")

**Do NOT create ADRs for**:
- ❌ Implementation details (variable naming, file organization)
- ❌ Tactical decisions (which library for HTTP parsing)
- ❌ Obvious choices (using Kubernetes ConfigMaps for configuration)

---

## ADR Format (MADR Template)

All ADRs follow the **Markdown Any Decision Records (MADR)** template:

```markdown
# ADR-XXXX: [Title in Imperative Form]

**Date**: YYYY-MM-DD
**Status**: Proposed | Accepted | Superseded by ADR-YYYY
**Deciders**: [Team/Individual]

## Context and Problem Statement

[2-3 sentences describing the problem to solve]

## Decision Drivers

- [Key factor influencing the decision]
- [Another key factor]

## Considered Options

1. **[Option A]** - Brief description
   - ✅ Pros: [Advantages]
   - ❌ Cons: [Disadvantages]

2. **[Option B]** - Brief description
   - ✅ Pros: [Advantages]
   - ❌ Cons: [Disadvantages]

3. **[Option C]** - Brief description
   - ✅ Pros: [Advantages]
   - ❌ Cons: [Disadvantages]

## Decision Outcome

**Chosen**: [Option X]

**Rationale**: [Why this option was selected, addressing decision drivers]

## Consequences

### Positive
- [Measurable benefit 1]
- [Measurable benefit 2]

### Negative
- [Trade-off or technical debt incurred]
- [Limitation introduced]

## Implementation

- **Files**: [Specific file paths affected]
- **Migration**: [If applicable, migration steps]
- **Configuration**: [Environment variables, settings]
- **Timeline**: [When implemented]

## Related Decisions

- Links to other ADRs, architecture docs, issues
```

---

## ADR Index

### Status Legend
- ✅ **Accepted** - Active decision, currently implemented
- 🔄 **Superseded** - Replaced by newer ADR
- 📝 **Proposed** - Under discussion, not yet implemented

---

### ADR-0001: Standard Go Layout (Not Hexagonal Architecture)

**Status**: ✅ Accepted
**Date**: 2025-11-11
**Problem**: Need consistent service structure optimized for LLM code generation
**Decision**: Use Standard Go project layout, reject ports/adapters hexagonal architecture
**Impact**: 60% less boilerplate, LLM-friendly patterns
**→ Read**: [0001-standard-go-layout.md](./0001-standard-go-layout.md)

---

### ADR-0002: Redis Streams + Pub/Sub Hybrid

**Status**: ✅ Accepted
**Date**: 2025-11-11
**Problem**: Need durable message queue + low-latency broadcast
**Decision**: Redis Streams (chat:raw) for durability + Redis Pub/Sub (overlay:*) for broadcast
**Impact**: 100-500ms latency, simpler than Kafka, single Redis instance (Phase 1)
**→ Read**: [0002-redis-streams-pubsub.md](./0002-redis-streams-pubsub.md)

---

### ADR-0003: CloudNativePG for PostgreSQL

**Status**: ✅ Accepted
**Date**: 2025-11-11
**Problem**: PostgreSQL high availability is complex (replication, failover, backups)
**Decision**: Use CloudNativePG operator for automated PostgreSQL management
**Impact**: Automated failover (<30s RTO), PITR, team experience with CNPG
**→ Read**: [0003-cloudnative-postgres.md](./0003-cloudnative-postgres.md)

---

### ADR-0004: No Hexagonal Architecture

**Status**: ✅ Accepted
**Date**: 2025-11-11
**Problem**: Initial plan had hexagonal architecture with ports/adapters
**Decision**: Remove ports/adapters layer, use direct handler → service calls
**Impact**: Removed ~8,000 lines of interface code, simpler for LLMs
**→ Read**: [0004-no-hexagonal-architecture.md](./0004-no-hexagonal-architecture.md)

---

### ADR-0005: React + Next.js App Router

**Status**: ✅ Accepted
**Date**: 2025-11-11
**Problem**: Minimize manual frontend coding, maximize LLM code generation
**Decision**: Next.js 16+ with App Router and Server Components
**Impact**: LLMs generate 90%+ of frontend code, SSR for SEO, streaming overlays
**→ Read**: [0005-react-nextjs-frontend.md](./0005-react-nextjs-frontend.md)

---

### ADR-0006: YouTube Quota Reserve-Confirm-Rollback

**Status**: ✅ Accepted
**Date**: 2025-11-15
**Problem**: Simple quota counter had ±500 units/day drift (5% error)
**Decision**: Atomic database reservations before API calls (reserve → confirm/rollback)
**Impact**: 99.95%+ accuracy, 9,000+ units/day waste eliminated (90% reduction)
**→ Read**: [0006-youtube-quota-tracking.md](./0006-youtube-quota-tracking.md)

---

### ADR-0007: Leadership Rebalancing for Auto-Scaling

**Status**: Accepted
**Date**: 2026-03-28
**Problem**: Leadership locks held indefinitely — new pods from auto-scaling get 0 work
**Decision**: Peer-aware rebalancing: pods register in Redis, shed excess leases based on peer count
**Impact**: Even channel distribution across N pods within ~30s of scale event
**Read**: [0007-leadership-rebalancing.md](./0007-leadership-rebalancing.md)

---

### ADR-0008: Feature Gate Infrastructure

**Status**: Accepted
**Date**: 2026-03-29
**Problem**: Adding a new premium-gated feature requires code changes and a deploy
**Decision**: DB + in-memory cache for capability-level premium toggling; Redis Pub/Sub invalidation + 60s TTL fallback
**Impact**: Zero-downtime feature toggles; unknown gates default to premium (safe); no external dependencies
**Read**: [0008-feature-gate-infrastructure.md](./0008-feature-gate-infrastructure.md)

---

### ADR-0009: Ring Buffer Publisher for Listener XADD Resilience

**Status**: Accepted
**Date**: 2026-03-29
**Problem**: All 6 Go listeners silently drop messages when Redis XADD fails during temporary unavailability
**Decision**: Opt-in RingBufferPublisher in shared/listener SDK — mutex-protected circular buffer (1000 messages) with 500ms retry goroutine
**Impact**: No silent message drops during Redis blips; bounded memory; single shared implementation for all listeners
**Read**: [0009-ring-buffer-publisher.md](./0009-ring-buffer-publisher.md)

---

### ADR-0010: Pronoun Enricher via Alejo API

**Status**: Accepted
**Date**: 2026-04-04
**Problem**: Pronoun display for chat users requires an external API dependency on api.pronouns.alejo.io — the only publicly available opt-in pronoun data source for Twitch users
**Decision**: Integrate Alejo API as a new enricher in the message-processor pipeline with 24h Redis cache and silent-fail on API errors; cross-platform lookup via linked Twitch accounts resolved in ViewerBadgeEnricher query
**Impact**: Pronouns for opted-in users with no user action required; 24h cache reduces API load; graceful degradation preserves message delivery on API failure
**Read**: [0010-pronoun-enricher-alejo-api.md](./0010-pronoun-enricher-alejo-api.md)

---

### ADR-0011: Zombie Listener Detection via Received-vs-Published Drift

**Status**: Accepted
**Date**: 2026-04-08
**Problem**: Twitch-listener pods appeared alive (IRC connected, liveness probe passing) but were not delivering messages — outages lasted 30–85 minutes requiring manual restart
**Decision**: Add two atomic counters (messagesReceived, messagesPublished) to the liveness probe; if received advances but published stalls for 5 minutes, return HTTP 503 so Kubernetes restarts the pod automatically
**Impact**: Zombie outages auto-recover in ~5.5 minutes; false positives on offline channels prevented via both-zero check; configurable stall window via ZOMBIE_STALL_WINDOW_MINUTES
**Read**: [0011-zombie-listener-detection.md](./0011-zombie-listener-detection.md)

---

### ADR-0012: OAuth Scope Minimisation Post Extension v1.6.0

**Status**: ✅ Accepted
**Date**: 2026-04-29
**Problem**: Viewer OAuth flows requested send-capable scopes (`user:write:chat`, `youtube.force-ssl`, `chat:write`) even though chat sending moved client-side in extension v1.6.0
**Decision**: Drop send scopes from viewer OAuth requests — the backend no longer needs to send chat on behalf of viewers
**Impact**: Reduced consent friction, smaller attack surface, simpler review for app verification
**→ Read**: [0012-oauth-scope-minimisation.md](./0012-oauth-scope-minimisation.md)

---

### ADR-0013: Public Overlay Observability View + Shared `useOverlayStream` Hook

**Status**: ✅ Accepted
**Date**: 2026-05-31
**Problem**: The OBS render route is the only way to watch an overlay's chat, but it is transparent/fading/theme-styled and unreadable as a monitor
**Decision**: Extract the realtime connection logic into a shared `useOverlayStream` hook (both the overlay and a new public `/overlay/[id]/view` consume it); the view is a readable, theme-agnostic, light/dark dashboard with resizable Chat + Activity panels
**Impact**: One implementation of reconnect/replay/dedup; overlay behavior preserved; streamers get a second-monitor observability dashboard with moderation logging
**→ Read**: [0013-overlay-observability-view.md](./0013-overlay-observability-view.md)

---

### ADR-0014: Linger Upstream Capture Demand Symmetric with the Downstream Pub/Sub Linger

**Status**: ✅ Accepted
**Date**: 2026-06-02
**Problem**: On overlay disconnect the `overlay:connected` key was deleted after ~60s, tearing down upstream chat capture, while the downstream pub/sub subscriber lingers 5 min and replays — so reconnect gaps beyond the grace period lost chat permanently
**Decision**: On disconnect, linger the `overlay:connected` key for `PUBSUB_LINGER_SECONDS` (default 5 min) instead of deleting it, and let source-manager release demand via the periodic reconcile when the key expires (no eager drop)
**Impact**: Brief overlay reconnects no longer lose chat; capture and replay are symmetric end to end; bounded idle cost; `PUBSUB_LINGER_SECONDS=0` reverts to immediate teardown
**→ Read**: [0014-demand-linger-symmetric-with-pubsub-linger.md](./0014-demand-linger-symmetric-with-pubsub-linger.md)

---

### ADR-0015: Dynamic EventSub Chat-Ownership Claim for the IRC↔EventSub Partition

**Status**: ✅ Accepted
**Date**: 2026-06-03
**Problem**: The IRC↔EventSub chat split used a static scope predicate, so a scope-eligible channel was dropped by IRC even when EventSub was not actually delivering (revocation, partial scope, verification failure, demand/leader gaps) — chat lost with no fallback
**Decision**: Make the partition dynamic — EventSub writes a per-channel `eventsub:chat:owner:{login}` claim (TTL, refreshed on delivered chat); IRC excludes only channels with a live claim and serves everything else; message-processor dedupes the handoff overlap on the native Twitch message id
**Impact**: IRC is the universal fallback — every gap where EventSub is not delivering is covered with no silent loss; cap relief preserved for active channels; self-healing within the claim TTL
**→ Read**: [0015-eventsub-chat-ownership-claim.md](./0015-eventsub-chat-ownership-claim.md)

---

### ADR-0016: Per-Link Twitch Credentials for Non-Twitch-Login Accounts

**Status**: ✅ Accepted
**Date**: 2026-06-13
**Problem**: The EventSub partition predicate only matched chat grants stored on Twitch-login users rows, so YouTube/Kick-login streamers who completed the Twitch add-source consent had their grant silently discarded — their channels were stuck on the IRC listener with no self-service fix
**Decision**: Persist add-source Twitch credentials per link in `twitch_oauth_tokens` (keyed by twitch_login, encrypted, scope-recorded); extend the partition predicate (overlay-manager), the EventSub credential lookup (LATERAL union preferring valid+scoped), and token-refresh (token type `twitch_link`)
**Impact**: "Connect Twitch" works identically for every account type; linked credentials fail back to IRC with ADR-0015 semantics; no double-refresh for Twitch-login accounts
**→ Read**: [0016-linked-twitch-credentials.md](./0016-linked-twitch-credentials.md)

---

### ADR-0017: Chat Moderation Write-Path

**Status**: ✅ Accepted
**Date**: 2026-06-19
**Problem**: All-Chat is read-only end-to-end; streamers want to delete/timeout/ban from the dashboard, requiring the first authenticated write to platform moderation APIs
**Decision**: New `moderation-service` reached via the gateway proxy; broadcaster's-own-token identity; owner-only authz (+ no shared_overlay); reuse the existing `message_deletion` reflect-back pipeline; least-privilege opt-in re-consent minimised to enabled actions; impersonated moderation allowed but attributed to the admin; Twitch full, Kick/Discord new clients, YouTube gated, TikTok unsupported
**Impact**: Isolates a high-blast-radius capability with its own authz/audit/rate-limits; zero new event types; amends ADR-0012 (force-ssl re-added only for opt-in YouTube moderators)
**→ Read**: [0017-chat-moderation-write-path.md](./0017-chat-moderation-write-path.md)

---

### ADR-0018: Premium Entitlements via Patreon

**Status**: ✅ Accepted
**Date**: 2026-06-20
**Problem**: `users.is_premium` is admin-granted only; we want self-serve premium by backing All-Chat's own Patreon, without breaking admin comps or the premium read path
**Decision**: New `payment-service` (Patreon OAuth connect + HMAC-MD5 webhooks + reconcile job); `users.is_premium` becomes a derived column = `(premium_admin_override IS TRUE) OR (override IS NULL AND active subscription)`, recomputed by `shared/premium.RecomputePremium`; identity via Patreon OAuth; status from Patreon's own grace-aware signal; single-replica reconcile backstop
**Impact**: Premium readers unchanged; admin comps survive lapses and payment never clobbers admin decisions; convergent/idempotent write path; Patreon-only (Ko-fi deferred)
**→ Read**: [0018-premium-entitlements-via-patreon.md](./0018-premium-entitlements-via-patreon.md)

---

### ADR-0019: Split Streamer vs Viewer Premium via a Polymorphic Patreon Subject

**Status**: ✅ Accepted
**Date**: 2026-06-20
**Problem**: `users.is_premium` (streamer features) and `viewers.is_premium` (cosmetic badge) are conflated; pure viewers (no `users` account) have no way to buy a cheaper viewer subscription
**Decision**: Generalize the ADR-0018 pipeline to a polymorphic subject (`user` | `viewer`) + tier-driven `product`; viewers connect Patreon via a viewer-JWT flow in payment-service; `viewers.is_premium` becomes a single-writer derived column via `RecomputeViewer` (admin override + active viewer sub OR inherited streamer premium)
**Impact**: One payment stack serves both products; viewer premium gains the convergent/clobber-free guarantees of the user side and fixes the inherited-premium-never-revoked staleness; one Patreon account grants one identity (documented)
**→ Read**: [0019-split-streamer-viewer-premium.md](./0019-split-streamer-viewer-premium.md)

---

### ADR-0020: Beta-Tester Role + Early-Access Feature Gates

**Status**: ✅ Accepted
**Date**: 2026-06-20
**Problem**: Thank the ~5 pre-monetization premium users with a standing role granting all premium features plus early-access ones; there is no role above premium, and a blanket grandfather `UPDATE` would re-run every pod start (009-incident class) and sweep in paid users
**Decision**: `users.is_beta_tester` folded into the `is_premium` derivation (a beta-tester is premium) + a `feature_gates.early_access` dimension enforced by a DB-backed `RequireEarlyAccess` (mirrors `RequirePremium`, fresh on grant); grandfathering is manual via an admin "Grant Beta Tester" button — no data migration. JWT-role gating rejected (stale until re-login)
**Impact**: Reuses ADR-0018 `Recompute`/`Effective` + ADR-0008 gates with no new authz subsystem; beta-testers pass every premium gate transparently; grant/revoke is fresh and convergent
**→ Read**: [0020-beta-tester-role.md](./0020-beta-tester-role.md)

---

### ADR-0023: Decoupled YouTube Quota Monitor

**Status**: ✅ Accepted
**Date**: 2026-06-22
**Problem**: The quota-based `youtube-listener` (the only exporter of `listener_quota_usage_percentage` and publisher of `quota:alerts`) is no longer deployed, so YouTube quota alerting went dark — even though `moderation-service` (bans) and `auth-service` (sends) still spend official quota against the shared `youtube_quota_usage` table; `auth-service` wasn't even counting sends (it failed open against the dead listener)
**Decision**: Extract a canonical `shared/quota` (state machine + `QuotaEvent`/`Notifier` + `Reserver`); switch `auth-service` sends to direct-SQL reserve-confirm-rollback; add a dedicated single-replica `youtube-quota-monitor` that reads the shared table, exports the Prometheus quota gauges, publishes `quota:alerts`, sweeps stale reservations, and serves `/quota/status` for the discord-bot poll
**Impact**: Restores both alert paths (Prometheus + discord-bot) with one owner and no duplicate alerts; sends are now accounted; builds on ADR-0006 (reserve-confirm-rollback) and ADR-0022 (alerting source of truth, in caesar-deployment); single replica is load-bearing for alert dedup only — accounting stays DB-atomic. (No ADR-0022 in this repo: 0022 lives in caesar-deployment, so this is 0023.)
**→ Read**: [0023-decoupled-youtube-quota-monitor.md](./0023-decoupled-youtube-quota-monitor.md)

---

### ADR-0024: Streamer Send-to-All Combined Pill via Pre-Register + Reconcile

**Status**: ✅ Accepted
**Date**: 2026-06-22
**Problem**: A streamer "send to all" collapses the per-platform echoes into one combined-pill message, but the pill was pre-registered with the *intended* platform set, so a failed platform (e.g. YouTube not live) still showed in the badge
**Decision**: Keep the pre-registration before fan-out (so fast echoes are recognised, no duplicates/loss), then reconcile the dedup group to the *actual* success set — rewrite survivors when ≥2 succeed, delete the group when <2 succeed; per-platform `error_kind` now classified like the single-send path. (ADR-0021/0022 live in caesar-deployment; numbering is shared across both repos, so this is 0024)
**→ Read**: [0024-send-to-all-combined-pill.md](./0024-send-to-all-combined-pill.md)

---

### ADR-0025: InnerTube Listener Caches activeLiveChatId for Streamer Sends

**Status**: ✅ Accepted
**Date**: 2026-06-22
**Problem**: Streamer sends to YouTube from the monitor always failed: auth-service's `youtube:stream:state` live-chat-id cache (strategy 1) was never populated because the deployed InnerTube listener never obtains the official `activeLiveChatId`, forcing the unreliable `search.list` fallback (observed 403 accountDelegationForbidden)
**Decision**: The InnerTube listener resolves `activeLiveChatId` once per stream via Data API `videos.list` (1 quota unit, gated by `YOUTUBE_API_KEY`), publishes the existing `youtube:stream:state` contract, refreshes it on a heartbeat, and deletes on stream end. Ingestion stays quota-free; disabled with no key. Rejected redeploying the decommissioned quota listener
**Impact**: YouTube send-to-all works in the InnerTube-only deployment via the reliable videos.list path; small, intentional break from strict zero-quota (~1 unit/stream). (ADR-0021/0022 in caesar-deployment, so this is 0025)
**→ Read**: [0025-innertube-livechatid-cache.md](./0025-innertube-livechatid-cache.md)

---

### ADR-0026: Two-Phase Deprecation of the Twitch IRC Listener

**Status**: ✅ Accepted
**Date**: 2026-06-20
**Problem**: The IRC listener is being retired for EventSub, but a streamer's source only moves when they re-add their Twitch source — killing IRC silently would lose chat for IRC-only channels with no warning and no obvious fix
**Decision**: A two-phase env-var gate `TWITCH_IRC_DEPRECATION_MODE` (`off`→`warn`→`enforce`): `warn` keeps serving chat and publishes a `listener_deprecation_notice` system event to every connected source every 5m (reusing the system-event pipeline); `enforce` empties the desired set so the listener joins no channels
**Impact**: Reversible, no-code rollout; users warned in-overlay with a re-add CTA before chat stops; notice fan-out is duplicate-free across replicas via leadership-by-join; fail-safe default (unknown value = off)
**→ Read**: [0026-twitch-irc-listener-deprecation.md](./0026-twitch-irc-listener-deprecation.md)

---

### ADR-0027: Time-Limited Admin Premium Overrides

**Status**: ✅ Accepted
**Date**: 2026-07-05
**Problem**: Admin premium grants (`users`/`viewers.premium_admin_override`, ADR-0018/0019) were permanent-until-revoked; we want to grant premium for a limited time (a comp/trial) that reverts on its own — but `is_premium` is materialized and ADR-0018 kept the rule time-free
**Decision**: Add an optional `premium_admin_override_expires_at` to both tables; `Recompute`/`RecomputeViewer` nullify an expired override in SQL against the DB clock (so `Effective` stays an unchanged time-free boolean, subscription half untouched); a single-replica payment-service sweep clears lapsed grants with a guarded atomic clear+recompute (never clobbers a re-grant). Grant length is a server-computed `NOW()+duration`
**Impact**: Time-limited grants for users and viewers; readers + `Effective` unchanged; correct on any recompute, converges within one sweep interval (≤5m) otherwise; narrow scoped amendment to ADR-0018's time-free property (override input only)
**→ Read**: [0027-time-limited-admin-premium-overrides.md](./0027-time-limited-admin-premium-overrides.md)

---

### ADR-0028: Engagement Chat-Command Write-Path

**Status**: ✅ Accepted
**Date**: 2026-07-01
**Problem**: Cross-platform polls/predictions (issue #523) only pay off if the default, non-extension viewer can participate — gating on the low-adoption extension yields empty polls; how does a chat message become a validated vote/wager without regressing the message-processor hot path?
**Decision**: message-processor does a cheap text pre-check + `EXISTS engagement:active:{platform}:{channel}` and forwards candidate commands to a durable `engagement:commands` stream; engagement-service parses/validates/writes off the hot path, resolving `viewer_id` via the existing `GetOrCreateViewerByPlatform`. Earning rides a separate best-effort `engagement:events` Pub/Sub. No platform-chat spam (send is opt-in); feedback via the broadcast tally + pull endpoint
**Impact**: Universal, zero-install participation across all platforms; extension/web page become enhancements; hot path adds only a pre-check for command-shaped messages. (ADR-0021/0022 live in caesar-deployment; numbering is shared, so this is 0027)
**→ Read**: [0028-engagement-chat-command-write-path.md](./0028-engagement-chat-command-write-path.md)

---

### ADR-0029: Viewer Points Economy & Prediction Payout Model

**Status**: ✅ Accepted
**Date**: 2026-07-01
**Problem**: Issue #523 adds All-Chat's first stateful virtual economy (viewer points + points-wagered predictions); its integrity model must be pinned down under concurrency, retries, and multi-replica Pub/Sub fan-out, and reconciled with Twitch's own Channel Points on mirrored predictions
**Decision**: Per-overlay economy; append-only `points_transactions` ledger + materialized balance, idempotent via `UNIQUE dedup_key`; wagers guard `balance >= amt` under `SELECT ... FOR UPDATE`; payout = stake + proportional split of the losers' pool with the integer remainder to the largest stake (conserves points, `math/big`); guarded state transitions + restart-safe auto-lock sweep; **Twitch-native predictions mirror state only and never touch All-Chat points** (they use Twitch Channel Points); private balance delivery is pull-first (broadcast WS would leak it)
**Impact**: Economy is correct-by-construction under retries/concurrency/fan-out; unit-tested payout conservation; points name is streamer-configurable (no hard-coded brand). (ADR numbering shared with caesar-deployment, so this is 0028)
**→ Read**: [0029-viewer-points-economy-and-prediction-payout.md](./0029-viewer-points-economy-and-prediction-payout.md)

---

### ADR-0030: Twitch-Native Poll/Prediction Mirroring

**Status**: ✅ Accepted
**Date**: 2026-07-06
**Problem**: A Twitch streamer can run polls/predictions through Twitch's own UI (settled in Channel Points); those rounds are invisible to All-Chat overlays, and reflecting them must never mix Twitch's currency with All-Chat viewer points or open a write path into a round Twitch owns
**Decision**: Opt-in `channel:read:polls`/`predictions` re-consent (non-fatal scope errors); the EventSub listener normalizes `channel.poll.*`/`channel.prediction.*` to a shared `NativeEngagementEvent` on the **durable** `engagement:twitch-native` stream; engagement-service upserts `source='twitch_native'` rows (per overlay, keyed by migration-070's `(overlay_id, source, external_id)` index) storing aggregate `mirror_*` tallies and re-broadcasts via the existing publisher. **Currency isolation is structural**: native rows never set the chat-active flag, the vote/wager/ledger paths stay `source='allchat'`-only, and public rendering uses separate native-preferring display queries; read tallies are `computed + mirror` (exact because each source populates only one side). Live native rounds 409 the All-Chat create endpoints.
**Impact**: Overlays reflect native Twitch rounds with zero new render/broadcast/type surface (the `source` discriminator does the work); points integrity is enforced by unreachability, not convention; invisible and cost-free on channels that don't opt in. (ADR numbering shared with caesar-deployment, so this is 0029)
**→ Read**: [0030-twitch-native-engagement-mirroring.md](./0030-twitch-native-engagement-mirroring.md)

---

### ADR-0031: Streamer-Keyed Viewer Engagement Participation

**Status**: ✅ Accepted
**Date**: 2026-07-06
**Problem**: The browser extension attaches to a stream by streamer *username* and never learns the overlay id (a viewer-withheld bearer capability), but every viewer participation endpoint is overlay-id-keyed — so the extension could only vote by posting a visible chat command, not through a proper UI
**Decision**: Add streamer-keyed sibling endpoints `/api/v1/engagement/streamers/:username/{active,me,vote,wager,heartbeat}` that resolve username→public overlay server-side (reusing the viewer WebSocket's `GetPublicOverlayByUsername` query) and then call the **unchanged** overlay-keyed vote/wager/balance primitives. The overlay id lives only inside the handler, never serialized to the client; `RecordVote`/`Wager` already reject a poll/prediction from a different overlay, so a resolvable username grants no cross-overlay authority. The chat-command path (ADR-0028) stays the zero-install fallback for surfaces this can't cover
**Impact**: The extension gets a proper no-chat-spam vote/wager/balance UI with only the streamer username + viewer JWT; the "viewers never learn the overlay id" boundary is preserved; no new points/tally logic or write path (a thin username→overlay adapter over ADR-0028/0029). (ADR numbering shared with caesar-deployment, so this is 0031)
**→ Read**: [0031-streamer-keyed-viewer-engagement.md](./0031-streamer-keyed-viewer-engagement.md)

---

### ADR-0032: Source-Liveness Heartbeat Contract and Overlay Self-Heal

**Status**: ✅ Accepted
**Date**: 2026-07-13
**Problem**: The source-manager cleanup job deactivates a source whose `updated_at` is >24h old, assuming every listener heartbeats it (migration 059). YouTube/Kick do; the Twitch IRC listener (now in enforce mode) no longer does and the EventSub listener never did — so a Twitch source on an always-open overlay got deactivated after 24h, the message-processor silently dropped its chat, and the client (transport kept alive by pongs + other sources) never recovered until a manual page refresh. Reproduced in production 2026-07-13 (caesarlp + another overlay the same hour)
**Decision**: (1) The API gateway heartbeats `overlay_chat_sources.updated_at` for every connected overlay's active sources on its 2-min tick — listener-agnostic, so any watched source stays fresh regardless of platform; (2) the EventSub listener self-heartbeats its chat-active sources per sync tick (leader-gated), honoring the migration-059 contract; (3) cleanup stays `updated_at`-based (all heartbeat writes are `updated_at`-only, so the NOTIFY trigger still doesn't fire); (4) the overlay client treats a `platform_status` down→`connected` recovery as a trigger to reconnect with `?since=` and replay the gap (debounced), self-healing without a refresh
**Impact**: The 24h Twitch-chat dropout cannot recur; abandoned overlays are still reaped (quota/load wind down) once the last connection leaves; no demand-refresh storms; overlays self-heal on source recovery. (ADR numbering shared with caesar-deployment, so this is 0032)
**→ Read**: [0032-source-liveness-heartbeat-and-overlay-self-heal.md](./0032-source-liveness-heartbeat-and-overlay-self-heal.md)

---

### ADR-0033: Bounded-Concurrency Enrichment in the Message Processor

**Status**: ✅ Accepted
**Date**: 2026-07-17
**Problem**: The message-processor enriched `chat:raw` messages strictly one-at-a-time on the consume-loop goroutine, with every stage doing blocking I/O (Redis round-trips + emote-service/Twitch/Alejo HTTP + Postgres on cache misses). That capped per-pod throughput at `1/per-message-latency`, with no bulkhead — any upstream latency spike collapsed throughput below the arrival rate and messages backed up in the stream. Production incident 2026-07-17 (`MessageProcessorStreamLagWarning`, 30–60s): input only ~2–3 msg/s, pods idle at ~3% CPU, Redis healthy, but a ~5× emote-service upstream-error spike pushed per-message latency from ~30ms to ~0.5–1s and the serial pipeline could not keep pace
**Decision**: Process each read batch through a semaphore-bounded worker pool (`processBatch`, default 16, `MP_PROCESS_CONCURRENCY`), waiting for the batch to drain before the next `XREADGROUP`; keep per-message retry/DLQ/ACK/dedup/deletion-buffer semantics unchanged; guard the `AvatarEnricher`/`BadgeEnricher` app-token behind mutex accessors (refresh HTTP outside the lock); negative-cache `PronounEnricher` API errors for a short TTL so a degraded pronouns API can't amplify the stall
**Impact**: ~16× per-pod throughput headroom so upstream hiccups no longer produce unbounded lag; no head-of-line blocking within a batch; two latent Twitch app-token data races removed (verified with `-race`). Safe because the consumer group already fans out across pods with no ordering guarantee, message/deletion ordering is handled by the reorder-tolerant deletion buffer, and dedup uses atomic `SETNX`. (ADR numbering shared with caesar-deployment, so this is 0033)
**→ Read**: [0033-message-processor-concurrent-enrichment.md](./0033-message-processor-concurrent-enrichment.md)

---

### ADR-0034: Admin Viewer Identity Model

**Status**: ✅ Accepted
**Date**: 2026-07-17
**Problem**: The admin viewer view was "basically useless" — it listed raw `viewer_sessions` (one per `(platform, platform_user_id)`) with no streamer/usage context and surfaced rate-limit counters as if they were engagement; `viewer_message_history` was write-only (DSGVO export only)
**Decision**: Operate on raw `viewer_sessions` (the unit ban/premium act on), surface the linked streamer `user_id` (migration 040), and add a READ-ONLY per-session activity aggregate over `viewer_message_history` (existing index, joined to `users.username`); stop presenting `message_count_1min/1hour`; do NOT introduce a durable cross-session identity table (dedup deferred)
**Impact**: Viewer view gains streamer/overlay context and real usage counts cheaply; `viewer_message_history` becomes an admin read surface (no new PII); a future durable-identity model can layer on top. (ADR numbering shared with caesar-deployment, so this is 0034)
**→ Read**: [0034-admin-viewer-identity-model.md](./0034-admin-viewer-identity-model.md)

---

### ADR-0035: Admin Global Entity Search

**Status**: ✅ Accepted
**Date**: 2026-07-17
**Problem**: There was no way to jump to an admin entity by name or id — an admin had to guess whether an id was a user/overlay/source/viewer and page through the right list
**Decision**: Add a global admin search resolving a free-text query across users (username/display_name/id), overlays (name/id), sources (channel name/handle/id), and viewers (username/platform_user_id), returning typed results that deep-link into the ADR-0036 URL-addressable views; initial implementation federates over the existing admin list endpoints client-side, with a server-side `/api/v1/admin/search` endpoint noted as the future optimization
**Impact**: One entry point to find anything; reuses and reinforces the URL-addressable pattern; client federation loads full lists (fine at current admin scale) with a server endpoint as the documented escape hatch. (ADR numbering shared with caesar-deployment, so this is 0035)
**→ Read**: [0035-admin-global-entity-search.md](./0035-admin-global-entity-search.md)

---

### ADR-0036: Admin Master-Detail with URL-Addressable Selection

**Status**: ✅ Accepted
**Date**: 2026-07-17
**Problem**: Admin pages kept selection and filters in opaque React state, so entities could not be linked to each other or shared by URL (the #1 complaint: "overlays are not linked to their users"); only the Overlays page read `?overlay=<id>`, and it was never standardized
**Decision**: Standardize every admin list+detail page on URL query params for selection *and* filters (`?user=`, `?overlay=`, `?filter=`, `?connected=`, `?platform=`, `?q=`), with a scrollable-list + sticky-panel master-detail layout; no new dynamic `[id]` routes (query params avoid a routing migration)
**Impact**: Admin views become deep-linkable and cross-navigable (source → user, overlay → owner, user → their sources) and URLs are shareable for support, at the cost of a small per-page mount-time param read; nested routes noted as the future step. (ADR numbering shared with caesar-deployment, so this is 0036)
**→ Read**: [0036-admin-url-addressable-selection.md](./0036-admin-url-addressable-selection.md)

---

### ADR-0037: Twitch Chat GIFs as Media Attachments

**Status**: ✅ Accepted
**Date**: 2026-07-18
**Problem**: Twitch shipped native (Giphy-backed) chat GIFs delivered as a text-replacement token — a bracketed alt caption in the body plus a `gif` EventSub fragment / `gifs` IRC tag naming the GIF that replaces it — and our pipeline ignored it, surfacing bare `[…]` caption text with no image
**Decision**: Reuse the PR #576 media path: both transports converge on the `gifs` tag (eventsub-listener synthesizes it from `gif` fragments like it does `emotes`; the IRC parser already forwards native tags), and the message-processor parses it into `image/gif` `attachments`, strips the caption span from the visible text, and re-anchors first-party emote offsets. Frontend unchanged (`MessageAttachments` is platform-agnostic; GIFs are images, so no CSP change)
**Impact**: Twitch chat GIFs render on all three surfaces with zero frontend change, inheriting #576's a11y (WCAG 2.2.2 hide/show on the monitor) and CSP; one parser serves EventSub + IRC; base feature, not premium-gated (parity with Twitch native chat). (ADR numbering shared with caesar-deployment, so this is 0037)
**→ Read**: [0037-twitch-chat-gifs.md](./0037-twitch-chat-gifs.md)

---

### ADR-0039: Database Connection Budget (bounded pools per service)

**Status**: ✅ Accepted
**Date**: 2026-07-18
**Problem**: The shared DB helper hardcoded `MinConns=5`/`MaxConns=20` per pool; across ~40 service instances that held ~197 permanently-idle connections against the cluster's `max_connections=200`, leaving zero headroom — any mass pod start (node failure, deploy) crashlooped new pods with `SQLSTATE 53300`. Connections were also anonymous (`application_name` empty), so the leak was undiagnosable from the DB
**Decision**: Treat connections as a budget — `Σ(instances × MaxConns)` must stay under `max_connections`. Shrink shared defaults to `MinConns=1`/`MaxConns=10`, make both env-tunable (`DATABASE_MAX_CONNS`/`DATABASE_MIN_CONNS`), set `application_name` (from `DATABASE_APP_NAME`→`OTEL_SERVICE_NAME`→`HOSTNAME`), and raise `allchat-cluster` `max_connections` 200→300 (memory-safe: 815Mi/2Gi at 200 conns, `work_mem` 2MB)
**Impact**: Steady-state DB connections drop ~197→~40 and headroom goes from 0 to ~260; mass restarts stop crashlooping; per-pod attribution restored. PgBouncer (CNPG `Pooler`, transaction mode) noted as the next step if instance count outgrows the additive budget. (ADR numbering shared with caesar-deployment, so this is 0039)
**→ Read**: [0039-database-connection-budget.md](./0039-database-connection-budget.md)

---

### ADR-0040: Self-host the Monaco CSS editor (no third-party CDN)

**Status**: ✅ Accepted
**Date**: 2026-07-19
**Problem**: The overlay editor's Custom CSS field uses `@monaco-editor/react`, whose loader fetches the Monaco engine from `cdn.jsdelivr.net` by default. The app CSP `script-src` enumerates allowed hosts and excludes jsdelivr, so the loader script was blocked and the editor hung forever on "Loading editor…" (reproduced in prod via Playwright: CSP violation → `Monaco initialization: error`)
**Decision**: Self-host instead of punching a CSP hole to a CDN. A build-time script (`scripts/copy-monaco.mjs`, wired via `prebuild`/`predev`) vendors `monaco-editor/min/vs` → `public/monaco/vs` (gitignored, 16 MB; `monaco-editor` stays a lockfile-pinned peer dep to avoid npm-version lockfile churn), and `src/lib/monaco.ts` points the loader at same-origin `/monaco/vs`. Add `worker-src 'self' blob:` to the CSP — Monaco runs its CSS language services in same-origin `blob:` workers regardless of engine host; without it the editor loads but validation/autocomplete silently die. No `script-src` change; no third-party CDN in the runtime path
**Impact**: The CSS editor loads and its CSS language features work under the exact production CSP (verified end-to-end with Playwright); the editor works offline / during a jsdelivr outage and the script-execution surface stays first-party. The only CSP broadening is `worker-src 'self' blob:` (already trusted in `media-src`). (ADR numbering shared with caesar-deployment, so this is 0040)
**→ Read**: [0040-self-host-monaco-editor.md](./0040-self-host-monaco-editor.md)

### ADR-0041: Ambassador Role + Public Homepage Showcase

**Status**: ✅ Accepted
**Date**: 2026-07-21
**Problem**: We want to recognise a curated set of streamers as "ambassadors" who receive premium + beta-tester (early-access) capabilities AND are showcased on the public marketing homepage. The entitlement half maps onto the beta-tester role (ADR-0020), but there is no public endpoint exposing a streamer's avatar/display name, and featuring real people publicly raises a consent question
**Decision**: Model `users.is_ambassador` as an admin-granted boolean folded into `shared/premium.Recompute` (→ premium) and into `RequireEarlyAccess` as `is_beta_tester OR is_ambassador` (→ early access), mirroring beta-tester (fresh DB enforcement, not a JWT role). Keep marketing-card presentation in a separate `ambassador_showcase` table (tagline + sort_order + featured_consent) so the entitlement stays one boolean. Public `GET /api/v1/ambassadors` returns a streamer only when `is_ambassador AND featured_consent` — assigning the role is an admin action, but public display is the streamer's own opt-in. Not a `feature_gates` entry / not in the `/upgrade` funnel (it is a recognition role, not a purchasable feature)
**Impact**: A third admin-granted role reusing the beta-tester machinery; grants take effect immediately; real people are only featured with explicit consent; the homepage section self-hides when nobody has opted in. A generic `user_roles` table stays deferred (reconsider at a fourth role). (ADR numbering shared with caesar-deployment, so this is 0041)
**→ Read**: [0041-ambassador-role.md](./0041-ambassador-role.md)

### ADR-0042: Overlay Editor Settings Navigation (Left Nav Replaces Stacked Drawers)

**Status**: ✅ Accepted
**Date**: 2026-07-23
**Problem**: The overlay editor stacked all settings as nested accordion drawers (7 top-level, "Appearance" nesting 10 more), putting controls 2–3 levels deep with no search and no usage tiering. External usability feedback showed users get lost in the reflowing stack and cannot find settings they can name (e.g. the badge visibility toggle)
**Decision**: Replace the drawer stack with a searchable left nav inside the existing SplitView config column: every section flat and always visible in the nav (grouped Setup / Appearance / Behavior / Advanced), exactly one section panel rendered at a time, a static `sectionRegistry` powering search over control labels + synonyms (jump-to-control with highlight), and low-traffic controls behind a per-section `AdvancedDisclosure`. `AppearancePanel`'s nested groups become first-class sections; the mock message/event injector is promoted from the Expert drawer to a Setup-group "Testing" section. Presentation-only — the save contract, preview postMessage protocol, and backend are untouched
**Impact**: Named settings are findable via search; navigation never reflows the column; the onboarding spotlight forces the active section instead of drawer open-state; the 3,600-line editor page decomposes into per-section components opportunistically. (ADR numbering shared with caesar-deployment, so this is 0042)
**→ Read**: [0042-editor-left-nav-settings.md](./0042-editor-left-nav-settings.md)

### ADR-0043: Preloaded, live-preview Custom CSS editor with fork-on-edit

**Status**: ✅ Accepted
**Date**: 2026-07-24
**Problem**: The Advanced → Custom CSS editor was unusable for streamers — applying a theme cleared it to a blank box (no starting point), raw edits reached the preview only on Save + reload ("nothing happens"), and Monaco's diagnostics were never surfaced. But `custom_css` had to stay "overrides only" so themes resolve from the bundle by `theme_id` and fixes propagate on deploy
**Decision**: Preload the applied theme's full CSS into the editor so it's visible and directly editable; debounce-push edits to the preview as you type (brace-balance guarded, fonts proxied); persist a **semantic diff** (`theme-css-diff.ts`, postcss) so only the user's changed/added declarations are stored and every untouched theme rule keeps updating — deleting a theme rule (which layering can't express) auto-forks *that* overlay to a full copy; reload merges the stored diff back onto the current theme; add "Reset to theme" and surface Monaco CSS diagnostics as line-numbered tips
**Impact**: Streamers can see + edit theme CSS with a live preview; theme fixes we ship still reach every rule a user didn't touch; only overlays that delete theme rules detach. No new premium gate (enhances an existing free tool); postcss stays in the editor route only; CSP + `scopeCustomCss` blast-radius controls unchanged. (ADR numbering shared with caesar-deployment, so this is 0043)
**→ Read**: [0043-preloaded-live-custom-css-editor.md](./0043-preloaded-live-custom-css-editor.md)

### ADR-0044: Overlay chat text outlines use layered text-shadow, not -webkit-text-stroke

**Status**: ✅ Accepted
**Date**: 2026-07-24
**Problem**: Chat text needs a dark edge to stay legible over live video. The minimal/comic-speech/neo-brutalist themes and gradient usernames used `-webkit-text-stroke` + `paint-order: stroke fill`, which centres the stroke on the glyph path and eats into the fill — chunky corners, eroded thin strokes, muddied gradients ("looks very off and not clean")
**Decision**: Standardise on layered `text-shadow` outlines (painted behind glyphs, never eroding the fill; render correctly behind gradient text). Migrate the three stroke themes to equivalent-weight layered outlines and gradient usernames to a clean drop shadow; leave intentional glow themes (cyberpunk/neon-glass/vaporwave) untouched. Regression test asserts no bundled theme's active CSS uses text-stroke/paint-order
**Impact**: Usernames + message text read cleanly at all sizes (verified before/after); new themes must use layered `text-shadow` for legibility outlines (glow effects still fine). (ADR numbering shared with caesar-deployment, so this is 0044)
**→ Read**: [0044-overlay-text-outline-via-layered-text-shadow.md](./0044-overlay-text-outline-via-layered-text-shadow.md)

### ADR-0045: Auto-Provision a First Overlay + Default Source on First Sign-In (Activation Cliff)

**Status**: 📝 Proposed (needs owner sign-off; changes cross-service behavior)
**Date**: 2026-07-27
**Problem**: The 2026-07-27 Umami analytics review found the dominant UX chokepoint: **~68% of users who sign in never copy an OBS URL** (588 `signin_completed` → 188 `obs_url_copied` sessions), i.e. they never get an overlay onto their stream. New users land on an empty dashboard and must manually create an overlay, run an OAuth reflow to add their own channel, then find + copy the OBS URL — re-assembling information the backend already holds after the login OAuth (channel identity + credentials)
**Decision (proposed)**: On the first successful sign-in for a user with zero overlays, the backend auto-provisions one overlay (named from the channel) + a working default source for the just-authed channel (backend-driven because the frontend never receives a ready source `channel_id`), then the client lands the user in the editor with the OBS URL + live preview front-and-centre. Idempotent (guard on zero-overlays + a `first_overlay_provisioned` marker). Fallback: a primed one-click "create my <platform> overlay" empty state
**Impact**: Removes the empty-start friction for the majority path (streamer adding their own channel); first sign-in yields an on-stream-ready overlay. Measurable via the instrumentation shipped alongside the review (`preview_rendered` aha, `obs_url_copied`, `source_add_failed`). Proposed, not accepted — auto-creating user content + auth↔overlay-manager coupling need sign-off first. (ADR numbering shared with caesar-deployment, so this is 0045)
**→ Read**: [0045-activation-auto-provision-first-overlay.md](./0045-activation-auto-provision-first-overlay.md)

---

## How to Create a New ADR

### Step 1: Determine ADR Number

```bash
# Find the next ADR number
ls -1 docs/adr/*.md | grep -E "^[0-9]" | tail -1
# Output: 0006-youtube-quota-tracking.md
# Next number: 0007
```

### Step 2: Create ADR File

```bash
# Use 4-digit number with leading zeros
touch docs/adr/0007-your-decision-title.md
```

### Step 3: Fill in Template

Copy the MADR template from above and fill in all sections. Be specific!

**Good ADR**:
- ✅ Explains **context** (what problem are we solving?)
- ✅ Lists **alternatives** (what other options did we consider?)
- ✅ Provides **rationale** (why this option over others?)
- ✅ Documents **consequences** (what are the trade-offs?)
- ✅ Includes **implementation details** (file paths, config, timeline)

**Bad ADR**:
- ❌ "We chose X because it's better" (no rationale)
- ❌ Only lists chosen option (no alternatives)
- ❌ No consequences section (ignores trade-offs)
- ❌ Vague implementation ("Update code accordingly")

### Step 4: Update This Index

Add entry to ADR Index above with:
- Status (📝 Proposed initially)
- Date
- One-sentence problem
- One-sentence decision
- One-sentence impact
- Link to ADR file

### Step 5: Link from Related Docs

Update cross-references:
- `docs/architecture/00-OVERVIEW.md` (if fundamental decision)
- Service READMEs (if service-specific)
- `CLAUDE.md` (if affects LLM navigation)

---

## Using the /doc-adr Skill

For Claude Code users, use the custom skill to generate ADRs:

```bash
/doc-adr "Your Decision Title"
```

The skill will:
1. Read existing ADRs to understand format
2. Determine next ADR number
3. Interview you about the decision
4. Generate complete ADR using MADR template
5. Update this index automatically

**→ See**: ADR skill in `.claude/skills/doc-adr.md`

---

## ADR Lifecycle

### Status Transitions

```
📝 Proposed
    ↓ (team review + approval)
✅ Accepted
    ↓ (decision changed)
🔄 Superseded by ADR-XXXX
```

### When to Supersede

Create a new ADR if:
- Original decision proven wrong by data/experience
- Technology landscape changed (new options available)
- Requirements changed significantly

**Do NOT**:
- ❌ Edit existing accepted ADRs (immutable history)
- ❌ Delete ADRs (preserve decision trail)

**Instead**:
- ✅ Create new ADR documenting the change
- ✅ Update old ADR status to "Superseded by ADR-XXXX"
- ✅ Link between old and new ADRs

---

## Related Documentation

- **[Architecture Overview](../architecture/00-OVERVIEW.md)** - High-level system design
- **[CLAUDE.md](../../CLAUDE.md)** - Project navigation hub
- **[CONTRIBUTING.md](../../CONTRIBUTING.md)** - Contribution guidelines

---

## Further Reading

- **MADR**: https://adr.github.io/madr/
- **ADR GitHub Organization**: https://adr.github.io/
- **When to Use ADRs**: https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions

---

## Summary

**Total ADRs**: 42 (all-chat repo; ADR numbers are shared with caesar-deployment) — re-verify against `docs/adr/` rather than trusting this count
**Status**: All accepted (✅)
**Coverage**: Core architecture decisions (Go layout, message flow, databases, frontend, quota tracking, feature gates, resilience patterns, pronoun enrichment, zombie detection, OAuth scope minimisation, overlay observability view, demand linger, EventSub chat-ownership partition, linked Twitch credentials, chat moderation write-path, premium entitlements via Patreon, streamer/viewer premium split, engagement economy, source-liveness heartbeat, admin URL-addressable views + viewer identity model + global search)

**Most Referenced**:
1. ADR-0002 (Redis patterns) - Referenced by all listeners, message processor
2. ADR-0001 (Go layout) - Referenced by all services
3. ADR-0006 (Quota tracking) - Referenced by YouTube listener, overlay manager

**Last Updated**: 2026-07-27
