# Architecture Decision Records (ADRs)

This directory contains Architecture Decision Records (ADRs) documenting significant architectural decisions made for All-Chat.

---

## What are ADRs?

An Architecture Decision Record (ADR) captures a **single architectural decision** along with its context, alternatives considered, and consequences. ADRs describe the **current state**: when the implementation changes what an ADR says, the ADR body is updated in the same change. A decision that is replaced outright gets a new ADR, and the old one is marked superseded.

**Benefits**:

- **Context**: Understand WHY decisions were made and how they are implemented today
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
**Status**: Proposed | Accepted | Superseded by ADR-YYYY | Withdrawn
**Revised**: YYYY-MM-DD (only once the body has been updated after acceptance)
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
- ⛔ **Withdrawn** - Feature removed; kept for the record

---

### ADR-0001: Standard Go Layout (Not Hexagonal Architecture)

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2025-11-11
**Problem**: All-Chat is built with an LLM-first development approach, so the service structure must be one LLMs generate accurately, with minimal boilerplate, while still supporting testability through dependency injection.
**Decision**: Standard Go project layout (cmd/handlers/models/repository plus domain packages) for every service; the hexagonal ports/adapters pattern was never implemented.
**Impact**: Roughly 60% less boilerplate than the hexagonal sketch and higher LLM code-generation accuracy; implemented across the complete services/ directory, validated in production.
**→ Read**: [0001-standard-go-layout.md](./0001-standard-go-layout.md)

---

### ADR-0002: Redis Streams + Pub/Sub Hybrid

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2025-11-11
**Problem**: The pipeline needs durable buffering and backpressure on ingest plus low-latency broadcast to overlays, with a <500ms P95 target and simple operations.
**Decision**: Listeners publish raw messages to Redis Streams (`chat:raw`, consumer groups `message-processor` and `alert-processors`); after normalization/enrichment the message-processor and alert-processor publish to Redis Pub/Sub (`overlay:{id}`, plus `:alerts`) subscribed by the API Gateway.
**Impact**: Measured 425ms P95 end to end, within the 500ms target; single-primary Redis instead of a Kafka cluster, with XPENDING lag monitoring, dead-letter routing and PEL drain at startup so a crashed replica's unacknowledged messages are reprocessed rather than lost.
**→ Read**: [0002-redis-streams-pubsub.md](./0002-redis-streams-pubsub.md)

---

### ADR-0003: CloudNativePG for PostgreSQL

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2025-11-11
**Problem**: PostgreSQL in Kubernetes needs high availability, automated failover and backups, and the team prefers declarative configuration with minimal manual operations.
**Decision**: Use the CloudNativePG operator; the team already had production experience with it. The CNPG `Cluster` manifest (`allchat-cluster`, 3 instances) is owned by the deployment pipeline, not this repo.
**Impact**: Operator-driven failover measured at roughly 22 seconds in early deploys, Prometheus metrics out of the box, and connection budgets via `shared/database` (per ADR-0039) with `max_connections` raised to 300; no S3 `barmanObjectStore` backup is configured yet, so PITR remains an open follow-up.
**→ Read**: [0003-cloudnative-postgres.md](./0003-cloudnative-postgres.md)

---

### ADR-0004: No Hexagonal Architecture

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2025-11-11
**Problem**: The Phase 1 plan existed on paper only; adopting hexagonal ports/adapters would have added roughly 8,000 lines of interface boilerplate across services, with LLMs generating incorrect interface wiring in 40% of trials.
**Decision**: Do not introduce ports/adapters; keep the standard Go layout and direct handler to repository calls with constructor-based dependency injection the services already use.
**Impact**: Around 8,000 lines of interface code avoided and features measured 2.5-3x faster in early trials; less "clean" structure and theoretically harder implementation swaps are accepted trade-offs.
**→ Read**: [0004-no-hexagonal-architecture.md](./0004-no-hexagonal-architecture.md)

---

### ADR-0005: React + Next.js App Router

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2025-11-11
**Problem**: The frontend must support real-time WebSocket chat overlays, SSR/SEO, an emote ecosystem, and an LLM-first development approach where AI generates most of the code.
**Decision**: React 19 + Next.js 16+ with App Router and Server Components; native WebSocket in `src/lib/api/websocket.ts` with the `useOverlayStream` hook on top (reconnect, `?since=` replay, dedup).
**Impact**: LLM-generated frontend code at 90%+ first-try accuracy, statically generated landing page and SSR overlay pages, and shadcn/ui (Base UI primitives) as the component library; App Router learning curve and rare hydration issues as accepted trade-offs.
**→ Read**: [0005-react-nextjs-frontend.md](./0005-react-nextjs-frontend.md)

---

### ADR-0006: YouTube Quota Reserve-Confirm-Rollback

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2025-11-15
**Problem**: The initial simple quota counter drifted about ±500 units/day (5% error) under concurrent updates, risking unexpected exhaustion of the YouTube quota.
**Decision**: Atomic SQL reservations before each official-API call (`reserve_youtube_quota`, then confirm or rollback per migration 008), with a `units_reserved` column on the day's row, 4xx-classified calls rolled back, and stale reservations cleaned up.
**Impact**: Accuracy improve to roughly ±2 units/day at decision time; the mechanisms mainly keep the shared quota table honest across multiple consumers today, since the default chat path runs quota-free on InnerTube (ADR-0065) and official-API units are spent only by the premium listener, moderation bans and streamer sends.
**→ Read**: [0006-youtube-quota-tracking.md](./0006-youtube-quota-tracking.md)

---

### ADR-0007: Leadership Rebalancing for Auto-Scaling

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-03-28
**Problem**: Leadership locks were held indefinitely, so pods added by auto-scaling got no work; one tiktok pod once held 43 of roughly 48 leases, overran a concurrency cap and went deaf everywhere.
**Decision**: Peer-aware rebalancing in the shared LeadershipCoordinator and its tiktok TypeScript port: pods register in Redis (`peers:{platform}` sorted set via source-manager), `Rebalance` sheds excess leases only after a 30-second peer-count stabilization gate, released streams are skipped for one demand cycle, and pods never hold more leases than they can connect (`TIKTOK_MAX_STREAMS_PER_POD`, default 20).
**Impact**: Even channel distribution across pods within about 30 seconds of a scale event with no forced takeover or message gaps; brief unmonitored window for released streams and alphabetical release order are accepted trade-offs.
**→ Read**: [0007-leadership-rebalancing.md](./0007-leadership-rebalancing.md)

---

### ADR-0008: Feature Gate Infrastructure

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-03-29
**Problem**: Premium-gating a new feature required code changes and a deploy; unknown gate keys also needed safe defaults.
**Decision**: `feature_gates` Postgres table plus per-service in-memory cache (`shared/featuregates`), typed gate-key constants, Redis Pub/Sub invalidation with a 60s TTL fallback, and `RequirePremium`/`RequireEarlyAccess` middleware (ADR-0020 adds the `early_access` dimension; unknown keys fail closed on both).
**Impact**: Zero-downtime premium/early-access toggles; a gate flip needs only a migration row and an admin API PATCH, no deploy. Up to 60s staleness if the invalidation message is missed is accepted.
**→ Read**: [0008-feature-gate-infrastructure.md](./0008-feature-gate-infrastructure.md)

---

### ADR-0009: Ring Buffer Publisher for Listener XADD Resilience

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-03-29
**Problem**: Go listeners silently dropped messages when `XADD chat:raw` failed during temporary Redis unavailability, with no retry and no metric.
**Decision**: Opt-in `RingBufferPublisher` in `shared/listener/ring_buffer.go`: mutex-protected circular buffer (1000 messages, drop oldest) with a 500ms retry goroutine, `Stop()` shutdown and Prometheus depth/drop metrics.
**Impact**: No silent drops during Redis blips; near-universal adoption (twitch, kick, youtube-innertube, eventsub, discord, owncast, goodgame, picarto, facebook, altstream, moderation-service), while youtube-listener (official mode) and the TypeScript tiktok-listener still publish without it.
**→ Read**: [0009-ring-buffer-publisher.md](./0009-ring-buffer-publisher.md)

---

### ADR-0010: Pronoun Enricher via Alejo API

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-04-04
**Problem**: Pronoun display needs an external API dependency on api.pronouns.alejo.io, the only public opt-in pronoun source for Twitch users, without degrading message delivery.
**Decision**: Alejo API as a message-processor enricher with 24h Redis caching (5-minute negative cache on failures), silent fail on errors, and cross-platform lookup via linked Twitch accounts resolved in the ViewerBadgeEnricher query; since the 2026-11-02 amendment the lookup runs only for overlays whose pronoun display is on.
**Impact**: Pronouns for opted-in users with no extra user action, at most one API call per unique viewer per day; an Alejo outage silences pronouns until the cache expires but never message delivery.
**→ Read**: [0010-pronoun-enricher-alejo-api.md](./0010-pronoun-enricher-alejo-api.md)

---

### ADR-0011: Zombie Listener Detection via Received-vs-Published Drift

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-04-08
**Problem**: twitch-listener pods looked alive (liveness probe 200) but stopped publishing messages, so outages lasted 30 to 85 minutes until a human restarted them.
**Decision**: Two atomic counters (`messagesReceived`, `messagesPublished` on ring-buffer accept) feed a `zombie.Detector` in the liveness probe; if received advances while published stalls for the stall window (default 5 minutes, `ZOMBIE_STALL_WINDOW_MINUTES`), the probe returns 503 and Kubernetes restarts the pod.
**Impact**: Zombie outages auto-recover in about 5.5 minutes; both-zero check avoids false positives on offline channels. With Twitch IRC in enforce mode (ADR-0026) chat no longer flows through the IRC path the guard was built for, but the detector remains wired and operational.
**→ Read**: [0011-zombie-listener-detection.md](./0011-zombie-listener-detection.md)

---

### ADR-0012: OAuth Scope Minimisation Post Extension v1.6.0

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-04-29
**Problem**: Viewer OAuth requested send-capable scopes (`user:write:chat`, `youtube.force-ssl`, Kick `chat:write`/`chat:read`/`channel:read`) although extension v1.6.0 moved all chat sending client-side.
**Decision**: Hard cutover dropping every scope with no remaining call site per flow (a streamer add-source flow re-requested Kick `channel:read`, and ADR-0017's moderation re-consent re-added the Twitch and YouTube send scopes for streamer moderation only).
**Impact**: Smaller consent screens and reduced token-leak blast radius; v1.5.x users who re-authenticate during the cutover window see send failures until upgrading; dead viewer-side send handlers remain candidates for follow-up removal.
**→ Read**: [0012-oauth-scope-minimisation.md](./0012-oauth-scope-minimisation.md)

---

### ADR-0013: Public Overlay Observability View + Shared `useOverlayStream` Hook

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-05-31
**Problem**: The OBS render route (`/overlay/[id]`) was the only way to watch an overlay's chat, but it is transparent, auto-fading and theme-styled, tuned for compositing rather than reading. Streamers asked for a second-monitor dashboard, and the realtime connection logic (reconnect/backoff, `?since=` replay watermark, dedup, deletion handling, `message_update` aggregation, `platform_status` gating) must not be duplicated.
**Decision**: Extract the connection logic into the shared callback-based `useOverlayStream` hook; refactor the overlay to consume it (behavior-preserving) and build a new public `/overlay/[id]/view` route on top: a readable, theme-agnostic light/dark dashboard with resizable Chat + Activity panels. Public by UUID because it exposes only data already public through the overlay endpoints.
**Impact**: One implementation of reconnect/replay/dedup/watermark/platform-status; OBS overlay behavior preserved; observability extras (filtered messages shown, moderated messages struck-through and tagged, deletions logged in the activity feed); the view ignores overlay themes entirely.
**→ Read**: [0013-overlay-observability-view.md](./0013-overlay-observability-view.md)

---

### ADR-0014: Linger Upstream Capture Demand Symmetric with the Downstream Pub/Sub Linger

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-06-02
**Problem**: On overlay disconnect the `overlay:connected` key was deleted after the ~60s grace period, and source-manager eagerly dropped demand, tearing down upstream chat capture, while the downstream pub/sub subscriber lingers 5 minutes buffering and replaying: reconnect gaps beyond the grace period lost chat permanently (recurred on OBS browser-source reconnects; with ADR-0026's IRC enforce phase there is no fallback Twitch chat path once the gate closes).
**Decision**: On disconnect, api-gateway sets `overlay:connected:{id}` to a TTL equal to the disconnect linger window (read from `PUBSUB_LINGER_SECONDS`, default 5 min, `0` reverts to immediate deletion) instead of deleting it; source-manager's `disconnected` handler no longer drops demand and the periodic reconcile releases it when the key expires. A reconnect within the window refreshes the key to the full connection TTL.
**Impact**: Brief overlay reconnects no longer lose chat and buffer/replay works end to end; demand for a truly-gone overlay falls away within `PUBSUB_LINGER_SECONDS` + one reconcile interval (an intentional bounded trade); one env var governs both windows.
**→ Read**: [0014-demand-linger-symmetric-with-pubsub-linger.md](./0014-demand-linger-symmetric-with-pubsub-linger.md)

---

### ADR-0015: Dynamic EventSub Chat-Ownership Claim for the Twitch Chat Partition

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-06-03
**Problem**: The IRC↔EventSub chat split used a static SQL scope predicate decoupled from whether EventSub was actually delivering, so a scope-eligible channel was dropped by IRC even when EventSub delivered nothing (revocation, partial scope consent, verify failure, demand/leader gaps, outages), losing chat with no fallback. Since ADR-0026's enforce phase IRC serves no channels, but the claim is the mechanism the enforce phase stands on: it is how the system decides, per channel and in real time, that EventSub is delivering its chat.
**Decision**: EventSub writes a per-channel ownership claim (`eventsub:chat:owner:{login}` in a shared `shared/twitchchat` package, TTL default 5m, throttled refresh) on each delivered chat message and releases it on revocation; it lapses on its own when delivery stops for any reason. The IRC listener used the live claim as its sole exclusion signal (historical; enforce mode left it moot), and message-processor dedupes the brief handoff overlap on the native Twitch message id. The live claim set is exposed as the `listener_eventsub_chat_owned` Prometheus gauge.
**Impact**: Removes every identified permanent-loss mode in the as-designed two-path world; since the enforce phase there is no fallback, and an uncovered failure stops a channel's chat until it is re-added or the EventSub listener recovers. The claim plus its gauge remain the live "who is serving this chat" signal; activity-scoped claims still bound the EventSub subscription set to channels that actually talk.
**→ Read**: [0015-eventsub-chat-ownership-claim.md](./0015-eventsub-chat-ownership-claim.md)

---

### ADR-0016: Per-Link Twitch Credentials for Non-Twitch-Login Accounts

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-06-13
**Problem**: The EventSub partition predicate matched chat grants only on Twitch-login users rows, which belong to the login provider; YouTube/Kick-login streamers completing the Twitch add-source consent had nowhere to persist the grant (and the platform-ID link itself was silently dropped). Verified in production: 11 of 46 IRC-partitioned channels had no self-service path to EventSub at all.
**Decision**: Store add-source Twitch credentials per link in `twitch_oauth_tokens` (migration 056), keyed by `(user_id, twitch_login)` with encrypted tokens and `granted_scopes`, written only when the flow is the Twitch add-source from a non-Twitch-login account. Consumers look at both credential sources: overlay-manager's `chat_via_eventsub` predicate, a `LEFT JOIN LATERAL` credential union in twitch-eventsub-listener (valid+scoped first, users row breaks ties), and a `twitch_link` token type in token-refresh-service.
**Impact**: "Connect Twitch" works identically for every account type and lands these accounts on the EventSub listener, which ADR-0026's enforce phase left as the only Twitch chat path; a channel is EventSub-eligible through either source with a deterministic choice. If a linked credential dies, the channel's chat goes quiet until the credential is repaired (re-add with `force_verify`). Since 2026-11-03 linking also records the platform id on the account (migration 121 backfills older Twitch and Kick links), so a login with a linked platform reaches the linking account; it authenticates only and never replaces the login provider's profile, tokens or scopes.
**→ Read**: [0016-linked-twitch-credentials.md](./0016-linked-twitch-credentials.md)

---

### ADR-0017: Chat Moderation Write-Path

**Status**: ✅ Accepted; as amended by ADR-0048 (revised 2026-11-03)
**Date**: 2026-06-19
**Problem**: All-Chat is read-only end-to-end (listeners → `chat:raw` → message-processor → Pub/Sub → gateway → frontend); streamers want to delete, timeout and ban directly from the dashboard, requiring All-Chat's first authenticated write against platform moderation APIs on a streamer's behalf.
**Decision**: A new `moderation-service` reached via the api-gateway proxy, which re-validates the JWT and re-checks authorization. The acting credential is the actor's own (broadcaster id = overlay owner's platform id, moderator id = actor's, reusing the ADR-0016 credential selection; never the owner's credential for a delegated moderator; Discord is a shared-bot write where All-Chat is the sole authorizer). Authorization is owner or active delegated moderator (ADR-0048) plus per-platform enablement, per-action grants, and the owner-reach anchor with the `shared_overlay` exclusion. Successful actions reflect back as existing `message_deletion` events on `chat:raw` (no new event types); scopes are opt-in per platform and action (`granted_scopes` as source of truth); admin impersonation is allowed but always attributed in the audit row. All five platform phases shipped: Twitch full; Kick full incl. single-message delete; Discord full via the bot's guild permissions plus opt-in re-invite; YouTube timeout and ban (delete/unban blocked for want of a usable id); TikTok unsupported. Gated on the premium-seeded `moderation` feature gate with the entitlement check moved inside `authorize()`, keyed on the overlay owner.
**Impact**: Isolates a high-blast-radius capability with its own authz, audit log, rate limits, metrics and NetworkPolicy; reuses the reflect-back pipeline so near-zero frontend rendering changes; amends ADR-0012 (`youtube.force-ssl` re-added only for streamers opting into YouTube moderation).
**→ Read**: [0017-chat-moderation-write-path.md](./0017-chat-moderation-write-path.md)

---

### ADR-0018: Premium Entitlements via Patreon

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-06-20
**Problem**: `users.is_premium` could only be set by an admin; there was no self-serve monetization. We want users to unlock premium by backing All-Chat's own Patreon, an external provider granting and revoking an internal entitlement automatically and reliably, without breaking the admin grant or the premium read path.
**Decision**: A new `payment-service` (Patreon OAuth connect, HMAC-MD5-verified membership webhooks, reconcile backstop) with `users.is_premium` derived: tri-state `premium_admin_override` first (grant→TRUE, revoke→NULL, FALSE reserved for a force-deny), else an active subscription; recomputed transactionally by `shared/premium.RecomputePremium` on any input change. Identity via Patreon OAuth user id, membership against the single campaign, active status from Patreon's own grace-aware signal, single-replica reconcile (the derivation also ORs in `is_beta_tester` since ADR-0020 and counts an expired override as absent since ADR-0027).
**Impact**: Premium readers unchanged; admin comps survive subscription lapses and payment never clobbers admin decisions; the convergent, idempotent write path tolerates webhook loss and races; the new inbound surface is isolated in its own service. Patreon-only (Ko-fi was considered and rejected; it was never built).
**→ Read**: [0018-premium-entitlements-via-patreon.md](./0018-premium-entitlements-via-patreon.md)

---

### ADR-0019: Split Streamer vs Viewer Premium via a Polymorphic Patreon Subject

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-06-20
**Problem**: One "premium" concept conflated streamer features (`users.is_premium`) and the viewer cosmetic badge (`viewers.is_premium`); a cheaper viewer subscription is wanted, but viewers are platform identities usually without a `users` account, so a Patreon pledge cannot key them as the streamer side does.
**Decision**: Generalize the ADR-0018 pipeline to a polymorphic subject (`user` | `viewer`) with a tier-driven `product`: one Patreon account maps to exactly one all-chat identity, a viewer connection activates at the cheaper `PATREON_VIEWER_MIN_TIER_CENTS`, and viewers connect via a viewer-JWT-authenticated flow in payment-service (callback and webhook stay single endpoints). `viewers.is_premium` becomes a single-writer derived column via `RecomputeViewer` (tri-state override, plus an active viewer subscription or inherited streamer premium); migration 064 adds the product/viewer columns beside ADR-0018's tables.
**Impact**: Pure viewers can self-serve while one payment stack serves both products; the viewer flag gains the same convergent, clobber-free guarantees and fixes inherited premium never being revoked on streamer lapse (eventually convergent); both premium read paths untouched. One Patreon account grants one identity (documented limitation).
**→ Read**: [0019-split-streamer-viewer-premium.md](./0019-split-streamer-viewer-premium.md)

---

### ADR-0020: Beta-Tester Role + Early-Access Feature Gates

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-06-20
**Problem**: The ~5 pre-monetization premium users should be thanked with a standing beta-tester status granting all premium features plus early-access ones. No role above premium exists, and a blanket grandfather `UPDATE` would re-run every pod start (the 009-incident class) and sweep in paying users.
**Decision**: A `users.is_beta_tester` flag folded into the `is_premium` derivation (`shared/premium.Recompute` ORs it, so a beta-tester is premium and every `RequirePremium` gate admits them; an admin force-deny still beats it), plus a new orthogonal `feature_gates.early_access` dimension enforced by a DB-backed `RequireEarlyAccess` (eligibility `is_beta_tester OR is_ambassador` per ADR-0041; unknown keys fail closed). `beta_tester` is deliberately not in the JWT, so grants take effect immediately; grandfathering is a manual admin "Grant Beta Tester" action, no data migration (migration 065 writes no values).
**Impact**: Reuses ADR-0018 recompute and ADR-0008 gates with no new authorization subsystem; grant/revoke is fresh and convergent; an admin can graduate an early-access feature by flipping the flag. One extra DB read per early-access-gated request, same cost profile as `RequirePremium`.
**→ Read**: [0020-beta-tester-role.md](./0020-beta-tester-role.md)

---

### ADR-0023: Decoupled YouTube Quota Monitor

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-06-22
**Problem**: The quota-based `youtube-listener` (ADR-0006) was the single owner of YouTube quota observability (metric + `quota:alerts` publisher), but it was not deployed: the listener path had moved to InnerTube, which spends no official quota, while moderation-service (bans) and auth-service (sends) still do. PrometheQL quota alert rules lost their series, the discord-bot subscriber starved, auth-service sends were unaccounted (failed open against the dead listener), and `cleanup_stale_quota_reservations()` lost its owner.
**Decision**: Extract a canonical `shared/quota` package (state machine, `QuotaEvent`/`Notifier` for `quota:alerts`, direct-SQL `Reserver`; moderation-service imports it and drops its local copy) and switch auth-service send accounting to the `Reserver` (same reserve-confirm-rollback semantics, prior fail-open behavior kept). Add a dedicated single-replica `youtube-quota-monitor` reading the shared `get_youtube_quota_with_reserved` view on an interval, exporting the Prometheus quota gauges, publishing deduped `QuotaEvent`s, sweeping stale reservations, and serving `/quota/status` for the discord-bot poll.
**Impact**: Both alert paths restored with one owner and no duplicate alerts; sends are counted so quota numbers reflect real usage; the single replica is load-bearing for alert dedup only, accounting stays DB-atomic.
**→ Read**: [0023-decoupled-youtube-quota-monitor.md](./0023-decoupled-youtube-quota-monitor.md)

---

### ADR-0024: Streamer Send-to-All Combined Pill via Pre-Register + Reconcile

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-06-22
**Problem**: Send-to-all collapses per-platform echoes of a streamer's own message into one combined-pill message, but the pill was pre-registered with the intended platform set, so a failed platform (reported: Twitch sent, YouTube failed on live-chat-ID discovery) still showed in the badge; the pill reflected intent, not delivery.
**Decision**: Keep the pre-registration before fan-out (so any fast echo is recognised and never published standalone, preserving the no-duplicates/no-loss happy path), then reconcile the dedup group to the actual success set: rewrite survivors to the success set when at least 2 platforms succeed, delete the whole group when fewer than 2 do (the lone echo then renders as an ordinary single-platform message). Per-platform `error_kind` is now classified like the single-send path (a YouTube "not live" surfaces as `stream_offline` instead of a blanket `send_failed`); the decision logic is a pure function (`decideSendAllPill`) unit-tested independently of Redis.
**Impact**: The combined pill lists exactly the platforms that received the message; happy-path collapse, dedup and no-loss guarantees unchanged. A narrow residual race remains (a fast echo processed before reconcile while another platform is slowly failing can publish with the stale set), not observed for the common fast-fail paths; reconcile adds a few Redis ops per partial-failure send.
**→ Read**: [0024-send-to-all-combined-pill.md](./0024-send-to-all-combined-pill.md)

---

### ADR-0025: InnerTube Listener Caches activeLiveChatId for Streamer Sends

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-06-22
**Problem**: YouTube send-to-all from the monitor always failed: auth-service's first strategy reads the `youtube:stream:state` Redis cache, but the only deployed listener (`youtube-listener-innertube`, InnerTube polling) never obtains the official `activeLiveChatId`, so with the extension videoID also absent for monitor sends every send fell through to the unreliable `search.list` fallback (observed HTTP 403 `accountDelegationForbidden`).
**Decision**: The InnerTube listener gains an optional `LiveChatResolver` (Data API `videos.list`, gated by `YOUTUBE_API_KEY`): on stream start it resolves `activeLiveChatId` off the hot path and writes the schema-compatible `youtube:stream:state` cache, refreshed by a heartbeat while live and deleted on stream end (the short TTL self-heals missed cleanup). Ingestion stays quota-free; with no key configured the feature is disabled and sends fall back to `search.list`.
**Impact**: YouTube send-to-all works on the default InnerTube chat path via the reliable `videos.list` route with no consumer changes; a small intentional break from strict zero-quota (~1 unit per stream start, not accounted against the shared quota table; negligible). The cache is keyed by channel, so multiple simultaneous streams on one channel resolve to one chat (a limitation of the contract).
**→ Read**: [0025-innertube-livechatid-cache.md](./0025-innertube-livechatid-cache.md)

---

### ADR-0026: Two-Phase Deprecation of the Twitch IRC Listener

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-06-20
**Problem**: The IRC listener is being retired for EventSub, but a streamer's source only moves when they re-add their Twitch source, so killing IRC silently would lose chat for IRC-only channels with no warning and no obvious fix.
**Decision**: A two-phase deprecation gate driven by `TWITCH_IRC_DEPRECATION_MODE`: `off` (default) behaves as before; `warn` keeps serving chat and emits one `listener_deprecation_notice` system event per (overlay, connected channel) onto `chat:raw` every `TWITCH_IRC_DEPRECATION_NOTICE_INTERVAL` (default 5m), rendered in the overlay activity feed with a re-add call-to-action; `enforce` empties the desired-channel set so nothing is joined and IRC serves nobody. Notices ride the existing system-event pipeline and are duplicate-free across replicas via leadership-by-join; unknown values resolve to `off` (fail-safe).
**Impact**: Reversible, no-code rollout (warn, enforce, then remove the listener; env-var changes need a pod restart, phases are deliberate operator actions). Production runs `enforce`, so the EventSub listener is now the only Twitch chat path; users on linked (non-Twitch-login) accounts can still re-add their source and land on EventSub via ADR-0016.
**→ Read**: [0026-twitch-irc-listener-deprecation.md](./0026-twitch-irc-listener-deprecation.md)

---

### ADR-0027: Time-Limited Admin Premium Overrides

**Status**: ✅ Accepted
**Date**: 2026-07-05
**Problem**: Admin premium grants via the tri-state `premium_admin_override` were permanent until revoked, but comps, giveaways and trials should revert on their own. ADR-0018 deliberately kept the entitlement rule time-free, and because `is_premium` is materialized, an expiry passing with no other write would leave the column stale.
**Decision**: Add a nullable `premium_admin_override_expires_at` to `users` and `viewers` (migration 067, `NULL` = permanent); `Recompute`/`RecomputeViewer` nullify an expired override in SQL against the DB clock, so the unchanged `Effective` stays time-free (only the admin-override input gains an expiry; the subscription half still honors Patreon's own grace). Admin endpoints take a server-computed `duration_seconds` deadline; a single-replica payment-service sweeper (`OverrideExpirySweeper`, default 5m) clears lapsed grants with a guarded atomic clear+recompute that never clobbers a concurrent re-grant.
**Impact**: Time-limited grants for users and viewers that revert clobber-free; readers and the `Effective` rule unchanged, correct on any recompute and convergent within one sweep interval (≤5m default) for a subject with no other write; a narrow, documented amendment to ADR-0018's time-free property.
**→ Read**: [0027-time-limited-admin-premium-overrides.md](./0027-time-limited-admin-premium-overrides.md)

---

### ADR-0028: Engagement Chat-Command Write-Path

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-07-01
**Problem**: Cross-platform polls/predictions (issue #523) only pay off if the default, non-extension viewer can participate; gating on the low-adoption extension yields empty polls. The question is how a chat message becomes a validated vote/wager without regressing the message-processor hot path, and how feedback is delivered
**Decision**: message-processor does a cheap in-process pre-check (`looksLikeCommand`) plus one `EXISTS engagement:active:{platform}:{channel}` (a refcounted flag engagement-service maintains while a round is ACTIVE) and forwards candidate commands to the durable `engagement:commands` stream; engagement-service parses/validates/writes off the hot path with per-(poll,viewer)/(prediction,viewer) idempotency, resolving `viewer_id` via `GetOrCreateViewerByPlatform` and resolving every overlay carrying the channel. Event-driven earning rides a separate best-effort `engagement:events` Pub/Sub (a missed earn is an unpaid point, never a corrupted balance). No platform-chat spam (send is opt-in); feedback via the broadcast tally + the pull endpoint. Starting a round is premium-gated (`RequirePremium("engagement")`, seeded in migration 076, flippable via the feature-gate admin endpoint); managing an open round, earning and participation are deliberately not gated
**Impact**: Universal, zero-install participation across all platforms; extension/web page become enhancements, not gates. Hot path adds only the pre-check plus at most one `EXISTS`/`XADD` for command-shaped messages on channels with a live engagement; chat-participation *points* are deferred in v1 (the earn consumer already supports `engagement:chat` but nothing publishes it)
**→ Read**: [0028-engagement-chat-command-write-path.md](./0028-engagement-chat-command-write-path.md)

---

### ADR-0029: Viewer Points Economy & Prediction Payout Model

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-07-01
**Problem**: Issue #523 adds All-Chat's first stateful virtual economy (viewer points + points-wagered predictions); its integrity model must be pinned down under concurrency, retries, and multi-replica Pub/Sub fan-out, and reconciled with Twitch's own Channel Points on mirrored predictions
**Decision**: Per-overlay economy (balance keyed `(viewer_id, overlay_id)`); append-only `points_transactions` ledger + materialized balance updated in the same transaction, idempotent via `UNIQUE dedup_key` (chat wagers keyed on the Redis stream entry id, round-independent and safe across redelivery; earning keys stable across the fan-out; `ON CONFLICT DO NOTHING`); debits guarded (`WHERE balance >= amt`) with `CHECK (balance >= 0)` as backstop; guarded prediction state machine (`CREATED → ACTIVE → LOCKED → RESOLVED|CANCELED`) with wagers `SELECT ... FOR UPDATE`ing the row and a restart-safe auto-lock sweep; payout = stake back + `floor(losersPool * stake / winnersPool)` with the remainder to the largest-stake winner (conserves points, `math/big`, unit-tested); **Twitch-native predictions mirror state only and never touch All-Chat points**, and no points ever move on a poll; private balance delivery is pull-first (broadcast WS would leak it)
**Impact**: Economy is safe under retries/concurrency/fan-out by construction (dedup keys + guarded transitions + conditional debits); new tables in migrations 068–070; points name is streamer-configurable. Watch earning flows through the two heartbeat endpoints, daily-capped; event-driven earning is limited to Twitch, YouTube and TikTok (Kick publishes only chat and deletions, so no Kick event earn yet), and chat-participation earning exists end-to-end but nothing publishes it (`chat_per_minute` flagged "coming soon" in the UI)
**→ Read**: [0029-viewer-points-economy-and-prediction-payout.md](./0029-viewer-points-economy-and-prediction-payout.md)

---

### ADR-0030: Twitch-Native Poll/Prediction Mirroring

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-07-06
**Problem**: A Twitch streamer can run polls/predictions through Twitch's own UI (settled in Channel Points); those rounds are invisible to All-Chat overlays, and reflecting them must never mix Twitch's currency with All-Chat viewer points or open a write path into a round Twitch owns
**Decision**: Opt-in `channel:read:polls`/`predictions` re-consent (`?actions=engagement`, added to `preservableScopes`; non-fatal scope errors); the EventSub listener creates the 7 `channel.poll.*`/`channel.prediction.*` subscriptions, normalizes each event to the shared `NativeEngagementEvent` and `XADD`s it to the **durable** `engagement:twitch-native` stream; engagement-service upserts `source='twitch_native'` rows per overlay (keyed by migration-070's `(overlay_id, source, external_id)` index), storing aggregate `mirror_*` tallies and re-broadcasting through the existing publisher. **Currency isolation is structural**: native rows never set the chat-active flag, the vote/wager/ledger paths stay `source='allchat'`-only (`RecordVote` rejects any non-allchat poll), and public rendering uses separate `GetActiveDisplay*` queries; read tallies are `computed + mirror` (exact because each source populates only one side). Mirror upserts are monotonic under out-of-order/redelivered events (state guards, `GREATEST` tallies, `COALESCE` timestamps), a synthetic stale-sweep cancel can be overridden by a genuine terminal event, and display prefers the All-Chat round while a live native round 409s the create endpoints
**Impact**: Overlays reflect native Twitch rounds with zero new render/broadcast/type surface; points integrity is enforced by unreachability, not convention. Known limitations: a scope grant made after the channel is first tracked activates only at the next channel resync (pre-existing subscription behavior); the 7 extra subscriptions add to the uncosted Twitch subscription budget; in the rare both-live state the native round stays hidden until the All-Chat round ends
**→ Read**: [0030-twitch-native-engagement-mirroring.md](./0030-twitch-native-engagement-mirroring.md)

---

### ADR-0031: Streamer-Keyed Viewer Engagement Participation

**Status**: ✅ Accepted
**Date**: 2026-07-06
**Problem**: The browser extension attaches to a stream by streamer _username_ and never learns the overlay id (a viewer-withheld bearer capability, omitted from streamer-info and snapshots), but every viewer participation endpoint is overlay-id-keyed — so the extension could only vote by posting a visible chat command, not through a proper UI
**Decision**: Add streamer-keyed sibling endpoints `/api/v1/engagement/streamers/:username/{active,me,vote,wager,heartbeat}` that resolve username→public overlay server-side (reusing the viewer WebSocket's `GetPublicOverlayByUsername` query: oldest active, public, non-banned) and then call the **unchanged** overlay-keyed vote/wager/balance/award primitives. The overlay id lives only inside the handler, never serialized to the client; `RecordVote`/`Wager` already reject a poll/prediction from a different overlay (silent `false`/`not_found`, no existence oracle), so a resolvable username grants no cross-overlay authority. The chat-command path (ADR-0028) stays the zero-install fallback for surfaces this can't cover
**Impact**: The extension gets a proper no-chat-spam vote/wager/balance UI with only the streamer username + viewer JWT; the "viewers never learn the overlay id" boundary is preserved; no new points/tally logic or write path (a thin username→overlay adapter over ADR-0028/0029). A multi-overlay streamer resolves to their oldest public overlay, the one the viewer WebSocket already selects
**→ Read**: [0031-streamer-keyed-viewer-engagement.md](./0031-streamer-keyed-viewer-engagement.md)

---

### ADR-0032: Source-Liveness Heartbeat Contract and Overlay Self-Heal

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-07-13
**Problem**: The source-manager cleanup job deactivates a source whose `updated_at` is >24h old, assuming every listener heartbeats it (migration 059). YouTube/Kick do; the Twitch IRC listener (now in enforce mode) no longer does and the EventSub listener never did — so a Twitch source on an always-open overlay got deactivated after 24h, the message-processor silently dropped its chat, and the client (transport kept alive by pongs + other sources) never recovered until a manual page refresh. Reproduced in production 2026-07-13 (caesarlp + another overlay the same hour)
**Decision**: (1) The API gateway heartbeats `overlay_chat_sources.updated_at` for every connected overlay's active sources on its 2-min tick — listener-agnostic, so any watched source stays fresh regardless of platform; (2) the EventSub listener self-heartbeats its chat-active sources per sync tick (leader-gated), honoring the migration-059 contract; (3) cleanup stays `updated_at`-based (all heartbeat writes are `updated_at`-only, so the NOTIFY trigger still doesn't fire); (4) the overlay client treats a `platform_status` transition from a known-down state to `connected` for a configured source as a recovery signal and forces a debounced `?since=` replay reconnect, self-healing without a refresh (a first-ever `connected` is deliberately not a recovery)
**Impact**: The 24h Twitch-chat dropout cannot recur; abandoned overlays are still reaped (quota/load wind down) once the last connection leaves; no demand-refresh storms. The self-heal is a best-effort backfill bounded by the gateway replay buffer and complements, not replaces, the transport-silence watchdog, which stays content-blind by design
**→ Read**: [0032-source-liveness-heartbeat-and-overlay-self-heal.md](./0032-source-liveness-heartbeat-and-overlay-self-heal.md)

---

### ADR-0033: Bounded-Concurrency Enrichment in the Message Processor

**Status**: ✅ Accepted
**Date**: 2026-07-17
**Problem**: The message-processor enriched `chat:raw` messages strictly one-at-a-time on the consume-loop goroutine, with every stage doing blocking I/O (Redis round-trips + emote-service/Twitch/Alejo HTTP + Postgres on cache misses). That capped per-pod throughput at `1/per-message-latency`, with no bulkhead — any upstream latency spike collapsed throughput below the arrival rate and messages backed up in the stream. Production incident 2026-07-17 (`MessageProcessorStreamLagWarning`, 30–60s): input only ~2–3 msg/s, pods idle at ~3% CPU, Redis healthy, but a ~5× emote-service upstream-error spike pushed per-message latency from ~30ms to ~0.5–1s and the serial pipeline could not keep pace. Secondary amplifier: the PronounEnricher cached nothing on Alejo API errors, re-issuing a call for every uncached message
**Decision**: Process each read batch through a semaphore-bounded worker pool (`processBatch`, default 16, `MP_PROCESS_CONCURRENCY`), waiting for the batch to drain before the next `XREADGROUP` (natural backpressure); keep per-message retry/DLQ/ACK/dedup/deletion-buffer semantics unchanged; make the enrichers race-free (mutex-guarded app-token accessors on Avatar/Badge enrichers, refresh HTTP outside the lock; other collaborators audited safe); negative-cache `PronounEnricher` API errors for a short TTL (5m) so a degraded pronouns API can't amplify the stall
**Impact**: ~16× per-pod throughput headroom so upstream hiccups no longer produce unbounded lag; no head-of-line blocking within a batch; two latent Twitch app-token data races removed (verified with `-race`). Safe because the consumer group already fans out across pods with no ordering guarantee, deletion-vs-message ordering is handled by the reorder-tolerant deletion buffer, and dedup uses atomic `SETNX`
**→ Read**: [0033-message-processor-concurrent-enrichment.md](./0033-message-processor-concurrent-enrichment.md)

---

### ADR-0034: Admin Viewer Identity Model

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-07-17
**Problem**: The admin viewer view was "basically useless" — it listed raw `viewer_sessions` (one per `(platform, platform_user_id)`) with no streamer/usage context and surfaced rate-limit counters (`message_count_1min`/`1hour`) as if they were engagement; `viewer_message_history` was write-only (DSGVO export only)
**Decision**: Operate on raw `viewer_sessions` (the unit ban acts on and premium resolves through), surface the linked streamer `user_id` (migration 040) with a link into the streamer's admin record via the ADR-0036 url-addressable pattern, and add a READ-ONLY per-session activity aggregate over `viewer_message_history` (existing index, joined to `users.username`, carrying `overlay_id` for deep links); stop presenting `message_count_1min`/`1hour`; do NOT introduce a durable cross-session identity table (session dedup deliberately deferred)
**Impact**: Viewer view gains streamer/overlay context and real usage counts cheaply (indexed group-by, no schema change, no new scan); `viewer_message_history` becomes an admin read surface instead of a write-only DSGVO artifact; a future durable-identity model can layer on top. Trade-off: one human with several sessions still shows as several rows; message metadata is now surfaced in one more admin place
**→ Read**: [0034-admin-viewer-identity-model.md](./0034-admin-viewer-identity-model.md)

---

### ADR-0035: Admin Global Entity Search

**Status**: ✅ Accepted (revised 2026-11-03; client-side federation superseded by ADR-0086) (revised 2026-11-03)
**Date**: 2026-07-17
**Problem**: There was no way to jump to an admin entity by name or id — an admin had to guess whether an id was a user/overlay/source/viewer and page through the right list
**Decision**: Add a global admin search resolving a free-text query across users (username/display_name/id), overlays (name/id), sources (channel name/id/handle, plus the owning overlay's name and the owner's username) and viewers (username/display_name/platform_user_id), returning typed results that deep-link into the ADR-0036 URL-addressable views. No search endpoint of its own: matching runs server-side as the `q` parameter ADR-0086 added to the existing admin list endpoints, and each group fetches `limit=8` matches plus `total` — the dedicated `/api/v1/admin/search` endpoint anticipated here was never built
**Impact**: One entry point to find anything; reuses and reinforces the URL-addressable pattern; search sees every row however large the tables grow, with no full list ever transferred. Cost: four list-endpoint queries per keystroke (debounced), and match quality follows the fields the list endpoints search server-side
**→ Read**: [0035-admin-global-entity-search.md](./0035-admin-global-entity-search.md)

---

### ADR-0036: Admin Master-Detail with URL-Addressable Selection

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-07-17
**Problem**: Admin pages kept selection and filters in opaque React state, so entities could not be linked to each other or shared by URL (the complaint: "overlays are not linked to their users"); only the Overlays page read `?overlay=<id>`, and ad hoc
**Decision**: Standardize every admin list+detail page on URL query params for selection _and_ filters (`?user=`, `?overlay=`, `?filter=`, `?connected=`, `?platform=`, `?q=`), read on mount with a scrollable-list + sticky-panel master-detail layout; cross-links are plain `href`s into the sibling pages' params; no new dynamic `[id]` routes (query params avoid a routing migration and keep list and detail co-resident)
**Impact**: Admin views become deep-linkable and cross-navigable (source → owning user, overlay → owner, user → their sources) and URLs are shareable for support, at the cost of small per-page param-read boilerplate; nested routes noted as the future step. Caveat recorded in the ADR: pages only _read_ params on mount — in-page selection/filter changes are not written back into the address bar (that write-back was in scope but never shipped), so a shared URL reflects the view at copy time
**→ Read**: [0036-admin-url-addressable-selection.md](./0036-admin-url-addressable-selection.md)

---

### ADR-0037: Twitch Chat GIFs as Media Attachments

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-07-18
**Problem**: Twitch shipped native (Giphy-backed) chat GIFs delivered as a text-replacement token — a bracketed alt caption in the body plus a `gif` EventSub fragment / `gifs` IRC tag naming the GIF that replaces it — and our pipeline ignored it, surfacing bare `[…]` caption text with no image
**Decision**: Reuse the PR #576 media path: both transports converge on the `gifs` tag as the internal wire representation (eventsub-listener synthesizes it from `gif` fragments with accumulated byte offsets, like its `emotes` synthesis; the IRC parser forwards the native tag via its tag allowlist), and the message-processor parses it into `image/gif` `attachments` (caption → `filename`, capped at 4 per message), strips the caption span from the visible text, and re-anchors first-party emote byte offsets (`remapEmotePositions`). Frontend unchanged (`MessageAttachments` is platform-agnostic; the existing `img-src https:` CSP already permits Twitch GIF URLs, no `media-src` change)
**Impact**: Twitch chat GIFs render on all three surfaces with zero frontend change, inheriting #576's a11y (WCAG 2.2.2 hide/show on the monitor) and CSP; one parser serves EventSub + IRC; base feature, not premium-gated (parity with Twitch native chat). Limitations: positions are Go byte offsets (the IRC path inherits the pre-existing multibyte caveat of IRC emotes; the EventSub path is exact); zero-caption GIFs are dropped; the render half of event-path GIFs is closed (ADR-0046 follow-up) but a GIF inside a dedicated-resub message remains unrendered
**→ Read**: [0037-twitch-chat-gifs.md](./0037-twitch-chat-gifs.md)

---

### ADR-0039: Database Connection Budget (bounded pools per service)

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-07-18
**Problem**: The shared DB helper hardcoded `MinConns=5`/`MaxConns=20` per pool; across ~40 service instances that held ~197 permanently-idle connections against the cluster's `max_connections=200`, leaving zero headroom — any mass pod start (node failure, deploy) crashlooped new pods with `SQLSTATE 53300`. Connections were also anonymous (`application_name` empty), making them undiagnosable from the database side
**Decision**: Treat connections as a budget — `Σ(instances × MaxConns)` must stay under `max_connections`. Shrink shared defaults to `MinConns=1`/`MaxConns=10`, make both env-tunable (`DATABASE_MAX_CONNS`/`DATABASE_MIN_CONNS`), set `application_name` (from `DATABASE_APP_NAME`→`OTEL_SERVICE_NAME`→`HOSTNAME`), and raise `allchat-cluster` `max_connections` 200→300 (memory-safe: primary at 815Mi/2Gi with 200 conns, `work_mem` 2MB). Applications connect to `allchat-cluster-rw` directly and no CNPG `Pooler` is deployed, so the per-pod budget is the only control in place
**Impact**: Steady-state DB connections drop ~197→~40 and headroom goes from 0 to ~260; mass restarts stop crashlooping; per-pod attribution restored; sizing tunable via env. Trade-off: a quiet service keeps only one warm connection (first concurrent query pays ~ms); the budget is still additive. PgBouncer (CNPG `Pooler`, transaction mode) noted as the next step if the instance count outgrows it
**→ Read**: [0039-database-connection-budget.md](./0039-database-connection-budget.md)

---

### ADR-0040: Self-host the Monaco CSS editor (no third-party CDN)

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-07-19
**Problem**: The overlay editor's Custom CSS field uses `@monaco-editor/react`, whose loader fetches the Monaco engine from `cdn.jsdelivr.net` by default. The app CSP `script-src` enumerates allowed hosts and excludes jsdelivr, so the loader script was blocked and the editor hung forever on "Loading editor…" (reproduced in prod via Playwright: CSP violation → `Monaco initialization: error`)
**Decision**: Self-host instead of punching a CSP hole to a CDN. A build-time script (`scripts/copy-monaco.mjs`, wired via `prebuild`/`predev`) vendors `monaco-editor/min/vs` → `public/monaco/vs` (gitignored, 16 MB; `monaco-editor` stays a lockfile-pinned peer dep to avoid npm-version lockfile churn), and `src/lib/monaco.ts` points the loader at same-origin `/monaco/vs`. Add `worker-src 'self' blob:` to the CSP — Monaco runs its CSS language services in same-origin `blob:` workers regardless of engine host; without it the editor loads but validation/autocomplete silently die. No `script-src` change; no third-party CDN in the runtime path
**Impact**: The CSS editor loads and its CSS language features work under the exact production CSP (verified end-to-end with Playwright); the editor works offline / during a jsdelivr outage and the script-execution surface stays first-party. The only CSP broadening is `worker-src 'self' blob:`; `script-src` has since shrunk further (`https://embed.twitch.tv` dropped; the credits clip embed runs as an iframe under `frame-src https://clips.twitch.tv`)
**→ Read**: [0040-self-host-monaco-editor.md](./0040-self-host-monaco-editor.md)

### ADR-0041: Ambassador Role + Public Homepage Showcase

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-07-21
**Problem**: Recognise a curated set of streamers as ambassadors who receive all premium features, the beta-tester (early-access) capability, and a showcase on the public marketing homepage. The entitlement half maps onto the beta-tester role (ADR-0020), but there is no public endpoint exposing a streamer's avatar/display name, and featuring real people publicly raises a consent question.
**Decision**: `users.is_ambassador` is an admin-granted boolean folded into `shared/premium.Recompute` (premium) and into `RequireEarlyAccess` as `is_beta_tester OR is_ambassador` (early access), mirroring the beta-tester role: DB-backed so grants take effect immediately, not a JWT role. Marketing-card presentation stays in a separate `ambassador_showcase` table (tagline, sort_order, featured_consent) driving a public `GET /api/v1/ambassadors`, which returns a streamer only when `is_ambassador AND featured_consent` and not banned; consent is written only by the streamer. It is not a `feature_gates` entry and not in the `/upgrade` funnel, because it is a recognition role, not a purchasable feature. Migration 078 writes no entitlement or consent values, so a re-run cannot clobber a live grant.
**Impact**: A third admin-granted role reusing the established machinery; the homepage section self-hides when nobody has opted in. The channel link is derived from the streamer's username and platform (best effort on YouTube: an `@handle` link when one resolves, no link otherwise). A generic `user_roles` table stays deferred for a hypothetical fourth role.
**→ Read**: [0041-ambassador-role.md](./0041-ambassador-role.md)

### ADR-0042: Overlay Editor Settings Navigation (Left Nav Replaces Stacked Drawers)

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-07-23
**Problem**: The overlay editor stacked all settings as nested accordion drawers (seven top-level, with "Appearance" nesting eight to ten more), putting controls two to three levels deep with no search and no usage tiering. External usability feedback showed users get lost in the reflowing stack and cannot find settings they can name, such as the badge visibility toggle.
**Decision**: Replace the drawer stack with a searchable left nav inside the existing SplitView config column: every section flat and always visible in the nav (grouped Setup / Appearance / Behavior / Advanced), exactly one section panel rendered at a time, a static `sectionRegistry` powering search over control labels and synonyms (jump-to-control with highlight), and low-traffic controls behind a per-section `AdvancedDisclosure`. `AppearancePanel`'s nested groups become first-class sections, and the mock message/event injector is promoted from the Expert drawer to a Setup-group "Testing" section. Presentation only: the save contract, preview postMessage protocol, and backend are untouched.
**Impact**: Named settings are findable via search; navigation never reflows the column; the onboarding spotlight targets the active left-nav section instead of drawer open-state. The two old localStorage keys were retired in favour of one last-active-section key, and `CollapsibleSection` was deleted. The section JSX remains inline in the editor page; per-section decomposition is opportunistic, not carried out.
**→ Read**: [0042-editor-left-nav-settings.md](./0042-editor-left-nav-settings.md)

### ADR-0043: Preloaded, live-preview Custom CSS editor with fork-on-edit

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-07-24
**Problem**: The Advanced custom CSS editor was effectively unusable: applying a theme cleared it to a blank box, raw edits reached the preview only after Save plus an iframe reload, and Monaco's CSS diagnostics were never surfaced. But `custom_css` had to stay "overrides only" so themes resolve from the bundle by `theme_id` and fixes propagate on deploy.
**Decision**: Preload the applied theme's full CSS into the editor so it is visible and directly editable, and debounce-push edits to the preview as you type (guarded so CSS with unbalanced braces is held back, with Google Fonts imports proxied via a 300ms `CUSTOM_CSS_UPDATE`). Persistence is a semantic diff (`theme-css-diff.ts`, postcss) in three modes: equal to the theme stores nothing (fully tracking the bundle), changed values or added rules store only those declarations so every untouched theme rule keeps updating, and deleting a theme rule, which layering cannot express, auto-forks that overlay to a full copy. Reload merges the stored diff back onto the current theme; add Reset to theme and surface Monaco diagnostics as line-numbered tips.
**Impact**: Streamers can see and edit their theme's CSS with a live preview, and theme fixes still reach every rule the user did not touch; only overlays that delete theme rules detach. Legacy `custom_css` is shown verbatim and re-diffed on the next save, with no migration. `postcss` stays in the editor route only, and the CSP plus `scopeCustomCss` blast-radius controls are unchanged. No new premium gate.
**→ Read**: [0043-preloaded-live-custom-css-editor.md](./0043-preloaded-live-custom-css-editor.md)

### ADR-0044: Overlay chat text outlines use layered text-shadow, not -webkit-text-stroke

**Status**: ✅ Accepted
**Date**: 2026-07-24
**Problem**: Chat text needs a dark edge to stay legible over live video. The minimal, comic-speech and neo-brutalist themes plus gradient usernames used `-webkit-text-stroke` with `paint-order: stroke fill`, which centres the stroke on the glyph path and eats into the fill, eroding thin strokes and muddying gradients; external feedback called the result "very off and not clean".
**Decision**: Standardise on layered `text-shadow` for legibility outlines, which is painted behind the glyphs and never erodes the fill, and migrate the three stroke themes to equivalent-weight layered outlines plus gradient usernames to a clean layered drop shadow. Intentional glow themes (cyberpunk, neon-glass, vaporwave) are untouched. A regression test asserts no bundled theme's active CSS uses text-stroke or paint-order stroke.
**Impact**: Usernames and message text read cleanly over video at all sizes. New themes must use layered `text-shadow` for legibility outlines; `text-shadow` for deliberate glow and neon effects remains allowed.
**→ Read**: [0044-overlay-text-outline-via-layered-text-shadow.md](./0044-overlay-text-outline-via-layered-text-shadow.md)

### ADR-0045: Auto-Provision a First Overlay + Default Source on First Sign-In (Activation Cliff)

**Status**: 📝 Proposed
**Date**: 2026-07-27
**Problem**: The 2026-07-27 Umami analytics review found the dominant UX chokepoint: roughly 68% of signed-in users never copy an OBS URL, so most users never get an overlay onto their stream. Brand-new users land on an empty dashboard and must manually create an overlay, run an OAuth reflow to add their own channel, and find the OBS URL, re-assembling information the backend already holds after the login OAuth.
**Decision (proposed)**: On the first successful sign-in for a user with zero overlays, the backend auto-provisions one overlay named from the channel plus a working default source for the just-authed channel (backend-driven because the frontend never receives a ready-to-use `channel_id`), then the client lands in the editor with the OBS URL and live preview front and centre. Idempotent via a zero-overlays guard plus a persistent `first_overlay_provisioned` marker. A primed one-click manual creation empty state is the acceptable fallback; analytics instrumentation shipped alongside the review makes the funnel measurable.
**Impact**: Removes the empty-start friction for the majority path and makes first sign-in yield an on-stream-ready overlay. The ADR is Proposed, not Accepted: it changes cross-service behaviour (auth-service and overlay-manager) and auto-creates user content, so it needs the owner's sign-off before implementation.
**→ Read**: [0045-activation-auto-provision-first-overlay.md](./0045-activation-auto-provision-first-overlay.md)

### ADR-0046: Twitch chat notices (watch streaks, announcements) via channel.chat.notification

**Status**: ✅ Accepted
**Date**: 2026-08-01
**Problem**: The EventSub listener subscribed only to `channel.chat.message`, never to `channel.chat.notification`. Several notice types carry the chatter's own message text and arrive on no other subscription — watch streaks and `/announce` bodies above all — so those messages were silently dropped with no error anywhere. Since ADR-0026 put IRC in enforce mode, EventSub is the only Twitch chat path. Seven further event types and every `shared_chat_*` notice variant were missing too. The complication is that five notice types (`sub`, `resub`, `sub_gift`, `community_sub_gift`, `raid`) also arrive on dedicated, richer subscriptions, so naive forwarding would double-render.
**Decision**: Subscribe to `channel.chat.notification` on the chat subscription's lifecycle (same chat condition and scopes, alongside the moderation subscriptions) with an explicit repair pass and revocation cache eviction so a failed or revoked companion subscription recovers. Route by notice type: skip the five types dedicated subscriptions already deliver, map the rest onto the event taxonomy, and emit anything unrecognised as a generic `twitch_notice` off `system_message` so future Twitch notice types degrade to a visible event instead of vanishing. Notice tags are built by delegating to the chat path's own `buildChatTags`, so notice-borne messages render with identical emote, badge, colour and `room-id` enrichment and are deletable via the message-ID registry. The message-processor's native-id dedup is widened to any Twitch message carrying `Tags["id"]` except deletions, so a notice and a same-id chat message can never both render.
**Impact**: Watch-streak and announcement messages appear on overlays at all, with no double-rendering, plus a new per-overlay `enable_twitch_watch_streaks` toggle (migration 079, default TRUE). Because a watch streak is the viewer's chat message, a disabled toggle demotes it to plain chat rather than dropping it, so no setting can delete chat; announcements, charity, modiversary and generic notices are deliberately not toggleable. `bits_badge_tier` gets its own event type rather than IRC's misleading `bits` mapping. `EventContent` now renders event text through the shared chat renderer and surfaces attachments, so event-borne GIFs stop vanishing. Coverage stops at a verbatim-Twitch-payload pipeline test; the twitch-listener IRC parser stays unfixed by design in enforce mode.
**→ Read**: [0046-twitch-chat-notices-via-chat-notification.md](./0046-twitch-chat-notices-via-chat-notification.md)

### ADR-0047: Username colour resolution order

**Status**: ✅ Accepted
**Date**: 2026-08-01
**Problem**: Reported as "not using the Twitch viewer's set colour". Measured on live traffic (219 raw Twitch messages, 2026-08-01), 87% did carry a colour tag and it was already honoured end to end; only 13% carry none. The real defect was that `ViewerBadgeEnricher` folded its deterministic fallback palette into the same `UserInfo.Color` field as authoritative colours, which left `Color` always non-empty, silently dead-lettered the streamer's "Username color" setting, and made it impossible to rank the streamer's setting between the platform colour and the fallback.
**Decision**: Split the field. `Color` becomes authoritative only (viewer's manual All-Chat colour, else platform-native, else empty); a new `AutoColor` (`auto_color`) always carries the deterministic palette fallback, keyed by viewer UUID when registered and `platform:userID` otherwise, and is omitted for gradients. The overlay resolves `user.color`, then the streamer's `--chat-username-color` setting, then `user.auto_color`, then `#FFFFFF` in one shared `resolveUsernameColor` helper, using CSS custom-property fallback syntax for the middle two steps so an unset setting falls through at paint time. Reproducing Twitch's own default palette for the 13% was considered and rejected in favour of All-Chat's palette, which is tuned for dark overlays and stays stable across a viewer's linked platforms.
**Impact**: The "Username color" setting works for the first time, applying exactly to chatters with no colour of their own, and one resolver backs all four render sites. Adds `auto_color` (about 20 bytes) to every chat message on the wire; clients reading only `color` see it empty for the colourless ~13% and must adopt the chain, while the API Gateway needs no change. A 2026-09 amendment added a `staticColor` option so "use username colours" bubble mode resolves every username to the picker or palette colour rather than short-circuiting on the chatter's own colour. The 13% still will not match the colour Twitch's own chat shows them, by choice.
**→ Read**: [0047-username-color-resolution-order.md](./0047-username-color-resolution-order.md)

### ADR-0048: Delegated Overlay Moderators

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-08-06
**Problem**: ADR-0017 scoped the moderation write-path owner-only, with every platform call made using the broadcaster's own stored token. Streamers want to hand the merged multi-platform moderation view to the volunteer moderators who actually do the work. Two questions dominate and are the same question: whose platform credential performs the write, and must All-Chat verify the delegate is really a moderator there. Where the moderator's own token acts, the platform re-checks their role on every call and our check is advisory; where the owner's token or a shared bot acts, the platform validates nothing about the delegate and All-Chat becomes the sole authority. The premium gate must also key on the overlay owner rather than the caller, and it is a new enforcement shape as well as a new authorization shape.
**Decision**: The moderator acts with their own platform credential, never the owner's, with no fallback, so Twitch, Kick and YouTube re-check the moderator role per call (platform-executed authority). Discord is the sole exception (platform-attested authority): no per-user Discord moderation API exists, the shared bot is always the actor, and All-Chat itself reads the moderator's live guild permissions, intersects them with the bot's, enforces role hierarchy for timeout and ban, and fail-closes on any Discord API error with a 60-second cache TTL that is a security bound. Adds the owner-reach anchor rule (a moderator may only act on a channel the overlay owner can prove they control; control only, never a moderation-scope predicate), retrofitted to the owner path as well, a dedicated moderator-credential store that must never be keyed to a channel because two listeners select ingest credentials by channel with no user scoping, per-overlay grants narrowed by three fail-closed filters, its own consent state kind with consent deferred to first use, owner-keyed premium enforced inside `authorize()`, and a shared_overlay-exclusion regression test.
**Impact**: Amends ADR-0017 at several clauses (owner-only authorization, `broadcaster_id == moderator_id`, the add-source-state reuse for consent, `RequirePremium` on the action routes, and Discord's "no credential resolution or scope pre-check" statement) and corrects two stale capability claims, shipped 2026-08-12: Kick single-message delete exists under a second scope, and YouTube delegation is timeout plus ban, since single-message delete and unban are blocked by identifiers All-Chat cannot obtain (InnerTube message ids, un-persisted ban resource ids), not by missing work. A cached "not a moderator" state is telemetry, never authorization. Discord carries the residual risk (no platform backstop; a stale grant is a live grant), bounded by per-guild opt-in, default off, and the live permission check. A pre-existing, delegation-independent cross-tenant hole (Discord source-add performs no guild validation) was surfaced and ships as a hard-fail prerequisite check. A 90-day grant dormancy suspension is planned but not yet running; a soft cap of 10 moderators per overlay applies and no chat-send for moderators in v1.
**→ Read**: [0048-delegated-overlay-moderators.md](./0048-delegated-overlay-moderators.md)

### ADR-0049: Stream Deck and Desktop Control Surfaces via Paired Device Tokens

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-08-14
**Problem**: A streamer asked for Stream Deck buttons that drive All-Chat directly, such as starting or closing a poll and sending a canned message to every connected chat. The actions need no new backend since every endpoint already exists and is gateway-exposed, but the entire problem is authentication: every authenticated caller is a browser (dashboard cookie or the extension's JWT handoff), session JWTs live 24 hours, and Elgato explicitly documents that plugins must not ship secrets, so what is needed is All-Chat's first non-browser credential. A 2026-08-31 correction notes send-to-all reaches Twitch, YouTube and Kick only; TikTok has no posting API and Discord is a one-way relay.
**Decision**: A long-lived, narrowly-scoped device token, delivered by a loopback redirect from All-Chat with PKCE (RFC 8252), falling back to a typed pairing code only where loopback cannot work, such as a second PC or headless capture box. Both paths mint the identical token, so this is one credential with two delivery paths. All-Chat is the authorization server, so the loopback redirect is registered with us and the platform logins are untouched; the redirect gets a dedicated validator (exact host `127.0.0.1` or `[::1]`, never `localhost`, any port, one fixed path), PKCE is S256-only, and a replayed one-time code also revokes the token it minted. The decision is framed around device tokens rather than Stream Deck, so future control surfaces reuse the pairing mechanism. A device token is bound to one user and overlay, carries an explicit action scope, is individually revocable with a use-refreshed sliding expiry, and does not by itself satisfy the premium gate or the advanced-controls consent grant, both still evaluated against the owning streamer at call time.
**Impact**: Two plugins ship in v1 because no single plugin format reaches all three apps: StreamController (Linux, Python, its own format) as the primary development target the maintainer dogfoods, and the Elgato SDK plugin (TypeScript, Marketplace), with OpenDeck free-riding on the original-SDK plugin. The real cost is drift, so one pinned action list governs both plugins, checked by `scripts/check-plugin-parity.py`. Both Linux hosts are GPL-3.0, which makes the licence review a release gate. Built 2026-08-20 (#737): migration 088 `device_tokens`, migration 089 seeding the `desktop_control_surfaces` gate (deliberately free, since three shipped documents state the plugins are free), device-token recognition behind the existing `SetAPITokenResolver` seam shared with PATs, the dedicated loopback validator, the shared `device_link_requests` state machine with a 5-attempts pairing-code bound, and the `state` round-trip as the plugin's own CSRF check. The spike's hand-issued token shipped as ADR-0051's personal access tokens; `api_tokens` remains undeprecated because loopback cannot reach a headless box or CLI. Still open deliberately: Marketplace and store submission, the AGPL-in-GPL sign-off, OpenDeck verification, and the Patreon post.
**→ Read**: [0049-desktop-control-surfaces-via-paired-device-tokens.md](./0049-desktop-control-surfaces-via-paired-device-tokens.md)

### ADR-0049: Personal Access Tokens

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-08-18
**Problem**: Every authenticated caller was a browser (dashboard cookie or the extension's origin-scoped JWT handoff), and neither mechanism has any meaning for a desktop process such as a Stream Deck plugin, a CLI, or a headless capture box, which must make an authenticated HTTP call at the moment a key is pressed on a machine configured once. The session JWT works on two counts: no browser context to obtain one, and a 24-hour life. The actions themselves needed no new backend; the entire problem was credentials.
**Decision**: A long-lived personal access token presented as `Bearer allchat_pat_<secret>` as an additional authentication path that does not replace the ADR-0049 pairing flow. The load-bearing prefix lets the middleware decide a bearer is a PAT before parsing it as a JWT. Only the SHA-256 digest is stored (`api_tokens.token_hash`, migration 086, mirroring the moderator invite-token precedent), so "show me that token again" is impossible and the create response is the only place the plaintext appears. The resolver is registered in every service that terminates authentication (auth-service, api-gateway, engagement-service) because the gateway forwards `Authorization` verbatim, so gateway-only registration would 401 every engagement call. A resolved PAT populates the same request-context keys as a JWT, so it is authentication only and never a bypass: `RequireAPITokenScope` is mounted beside the existing gates, never in place of them, token management is session-only, and account deletion and PII export refuse a PAT outright. The pasted-secret objection is bounded, not answered: scoped, visible via last-used, one-click revocable.
**Impact**: Non-browser clients become possible at all, and revocation is a live column read on each request with no cache to invalidate. The premium gate on round-start routes still applies to a correctly scoped PAT belonging to a non-premium owner (403, pinned by a test), while close, lock, resolve and cancel stay deliberately ungated so a streamer can always finish a round. It is a new long-lived credential type for the next security review, and the paste step remains a real leak vector for a population that streams its own screen. The canonical record is ADR-0051; number 0049 collides with the device-token ADR, which is a different credential solving an adjacent half of the same problem.
**→ Read**: [0049-personal-access-tokens.md](./0049-personal-access-tokens.md)

### ADR-0050: Visual-customizer colors carry their opacity in the value (8-digit hex)

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-08-16
**Problem**: All visual-customizer colors were stored as 6-digit hex, and only the two background colors had an opacity control, living in a sibling field (`overlayBgOpacity` / `bubbleBgOpacity`) combined with the hex into an inline `rgba()` style. So no other color could be made translucent at all, and with any theme applied even those sliders silently did nothing: a theme rule reading the color variable with `!important` outranks the inline style, and the emitted opacity variable is referenced by no bundled theme.
**Decision**: Opacity is part of the color value: a non-opaque color is stored as 8-digit hex (`#rrggbbaa`), and every color picker gets an opacity slider with no new sibling opacity fields. `lib/utils/hex-alpha.ts` owns the format and keeps fully opaque colors 6-digit; the alpha travels into both consumers, the inline bubble/overlay styles and the `--chat-*` custom properties, so theme CSS honours it. The deprecated legacy fields stay readable, folded into the slider position, and are cleared on write so a value is never dimmed twice. Rejected: a per-color opacity field (about ten new keys, and composition would be needed anyway to reach themes) and storing `rgba()` strings (breaks hex input and hand-authored theme expectations).
**Impact**: Any color can be made translucent or fully transparent, with or without a theme; bubble background at 0% is now the supported answer to "transparent bubbles". Theme authors may receive an 8-digit hex from any `--chat-*` variable, fine anywhere a color is valid but not for CSS that string-splices a variable into `rgba()` (no bundled theme does). `visual_settings` is schemaless JSON, so no backend or migration work. One sibling reader remains deliberately unfolded: `bubbleUserColorOpacity` is still a separate decimal-string field consumed by the inline-style path for non-8-digit fills, the same reader-versus-writer split this ADR set out to close, not folded in on this pass.
**→ Read**: [0050-color-opacity-in-the-color-value.md](./0050-color-opacity-in-the-color-value.md)

### ADR-0051: Personal Access Tokens for Non-Browser Clients

**Status**: ✅ Accepted
**Date**: 2026-08-16
**Problem**: ADR-0049's own implementation order starts with a spike "against a hand-issued token", on the surface the maintainer actually runs, but no hand-issued credential existed, and three deliverables (a StreamController plugin, an Elgato plugin, a dashboard token page) are in flight against it simultaneously, so its format and endpoints must be a frozen contract rather than whatever the first consumer picks.
**Decision**: A long-lived personal access token presented as `Authorization: Bearer allchat_pat_<secret>` (256 bits from `crypto/rand`, base64url), resolved in `shared/middleware`: a bearer carrying the prefix is SHA-256'd and looked up in `api_tokens` (migration 086) instead of being parsed as a JWT, anything else falling through to the JWT path unchanged. Only the digest is stored, mirroring `overlay_moderators.invite_token_hash`; the plaintext is returned exactly once by the create endpoint, never again. Additive to ADR-0049, not a replacement: its PKCE loopback pairing delivery remains the intended install UX for the published plugins, and a PAT also covers the cases loopback cannot reach (headless box, second PC, self-hoster's CLI).
**Impact**: Authentication only, never authorization is the load-bearing constraint: a resolved PAT sets the same context keys a valid JWT sets, and `RequireAPITokenScope` (`chat:write`, `engagement:write`) is mounted beside the existing gates, never in place of them, so on `POST /overlays/:id/polls` a correctly scoped PAT of a non-premium owner is still 403'd by `RequirePremium`. The resolver is registered in all three of auth-service, api-gateway and engagement-service; gateway-only wiring would be inert. Token management, `/logout` and `RefuseAPIToken` surfaces (admin, account deletion, data export) are session-only, revocation is a column read live with no cache, and a ban cuts a PAT off on every request. Residual risks: a pasted secret handled on camera, no per-overlay binding, `expires_at` defaulting to NULL.
**→ Read**: [0051-personal-access-tokens-for-non-browser-clients.md](./0051-personal-access-tokens-for-non-browser-clients.md)

### ADR-0052: Retiring Euler Stream for TikTok webcast signing

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-08-17
**Problem**: `tiktok-live-connector` cannot open a TikTok LIVE WebSocket without a signed URL and gets it from Euler Stream by default, putting a third party on the critical path of every TikTok connection. Measured on 2026-08-14 (PR #695): twelve concurrent attempts exhausted the free-tier sign limit, a hard ceiling that does not even fail cleanly (`Cannot read properties of undefined (reading 'retry-after')` reading the 429); `fetchAvailableGifts()` needs a Business plan; and authenticated sessions forward the TikTok session cookie to the sign server.
**Decision**: No fork needed: the connector exposes module-level mutable globals covering every Euler call site; only the WebSocket signature lacks a direct alternative. Two independent levers, each with its own flag: `TIKTOK_DISABLE_EULER_FALLBACKS` (default on, safe) drops Euler's leg of the composites; `TIKTOK_SIGNER_MODE` (default `euler`) walks `euler` → `shadow` → `self` → `pure-node`, with `TIKTOK_SELF_SIGN_FALLBACK` staying on until the measured rate justifies removing it. Step 1 is implemented as `services/tiktok-signer` (X-Bogus from TikTok's own web SDK in a headless browser, X-Gnarly vendored, signing and executing `/webcast/im/fetch/`), and production is fully retired: the listener runs `pure-node` with fallback off. Later updates added a page-viewer mode and TLS impersonation after TikTok began gating webcast endpoints on browser-grade sessions, and per-pod stream ceilings after an Euler proxy-cap incident.
**Impact**: The trade-off stays conscious: signing changes become our on-call problem and TikTok ingest is fully down until fixed, ongoing rather than one-off, which is why shadow measurement preceded cutover. `tiktok_sign_attempts_total` distinguishes load-bearing availability from shadow experiments; the licence obstacle in the issue does not apply to the pinned 2.4.0, which is MIT.
**→ Read**: [0052-retiring-euler-stream-for-tiktok-signing.md](./0052-retiring-euler-stream-for-tiktok-signing.md)

### ADR-0053: Consolidate the Minimal Clean theme; retire theme ids by alias, not deletion

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-08-18
**Problem**: `minimal-theme-fixed` was a bugfix fork of `minimal-theme` that shipped alongside the theme it was meant to replace, and both drifted into defects the other had fixed: the fork lost inline event rendering and carried a visibly blocky mixed-radius outline, while the original hid the "Shared Chat" indicator via an over-broad selector and hard-coded platform status indicators off. Between them they held the two largest theme populations (115 + 107 overlays, ~72% of themed overlays). Separately, a user reported that Line Height did nothing.
**Decision**: One theme: `minimal-theme` absorbs the fork's fixes, `minimal-theme-fixed.css` is deleted, and the retired id resolves through a `THEME_ALIASES` map in `bundled-themes.ts` rather than being removed, because a dangling id renders an overlay with no theme CSS at all. Migration 087 rewrites stored ids; the alias stays after it for rollback safety. The Line Height bug was independent and affected every inline-layout theme: the strut's preflight `line-height` outranked the control, so it could loosen rows but never tighten them.
**Impact**: The dominant spacing cost was bubble padding the theme had deleted: customizer declarations carried the platform default as their `var()` fallback, so Minimal got 12px rows anyway. The fallback is now a theme-intent step (`var(--chat-bubble-padding, var(--theme-bubble-padding, 0.75rem))`), giving customizer setting > theme intent > platform default; on the reporter's settings row pitch went 48px to 24px. Retiring any bundled theme id now requires an alias entry, pinned by tests.
**→ Read**: [0053-consolidate-minimal-theme-and-retire-theme-ids-via-alias.md](./0053-consolidate-minimal-theme-and-retire-theme-ids-via-alias.md)

### ADR-0054: Twitch moderation and AutoMod event feed

**Status**: ✅ Accepted
**Date**: 2026-08-23
**Problem**: All-Chat reacted to Twitch moderation only through its effects: `channel.chat.message_delete`, `channel.chat.clear_user_messages` and `channel.chat.clear` carry no acting moderator, so a streamer could not learn which of their mods timed someone out, for how long, or why. Also, a message AutoMod holds is never delivered as chat at all, so the streamer has to sit in Twitch's own AutoMod queue, defeating the single-pane premise.
**Decision**: Three EventSub subscriptions, not one: `channel.moderate` v2 (the moderator action feed, the only one carrying `moderator_user_login`), `automod.message.hold` v2 and `automod.message.update` v2. Recorded because an earlier design misread `channel.moderate`'s `automod_terms` action as the hold feed. Everything arrives as one event type, `mod_action`, with the action in `EventData["action"]` passed through verbatim so mapEventTypeToColumn needs one `columnAlwaysEnabled` entry. Nine scopes behind one `?actions=modlog` consent, including `moderator:manage:automod`, which Twitch requires for the hold subscription although this iteration only reads; one consent rather than two. Owner-only, not delegated, per ADR-0048.
**Impact**: The load-bearing security property is delivery: held text never reached chat, and the overlay WebSocket accepts anonymous OBS sources, so mod frames go only to owner-verified sockets and are excluded from the chat replay buffer. No migration, no column, no toggle; the feed is live-only, capped in the browser. Approve/Deny is deliberately out of scope and additive later. A `modlog` grant takes effect only on the next channel resync, and a missing grant is silent by design.
**→ Read**: [0054-twitch-moderation-and-automod-event-feed.md](./0054-twitch-moderation-and-automod-event-feed.md)

### ADR-0055: A typed UI string catalog, with locale never in the URL

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-08-24
**Problem**: The frontend had no i18n layer of any kind: every user-facing string a literal at its render site and `<html lang="en">` hardcoded, with the intent on record since `errorMessages.ts` was written. Quieter second problem: 38 non-test call sites use `toLocaleString()` or `Intl.*` with no explicit locale, so dates and numbers followed the viewer's browser locale, e.g. "Expected completion: 4. März, 13:00" under English copy.
**Decision**: A pure, typed catalog at `src/lib/i18n/` (`config.ts`, `messages/en/`, `translate.ts`, `format.ts`, `index.ts`), no new dependency, with `MessageKey` a recursive mapped type that makes a bad key a compile error. Lookup never throws: an unknown key resolves to the key string and a missing param leaves the placeholder; overlay pages render inside OBS on live broadcasts and an exception would black out the overlay. Locale never enters the URL (`/overlay/[id]` is pasted into OBS sources for months) and request-time resolution stays rejected, because reading `cookies()`/`headers()` in the layout would make every static page dynamic. `useTranslations()` is provider free so Storybook keeps rendering components standalone. A shrink-only ESLint i18n gate ratchets the catalog the way the a11y gate does.
**Impact**: Two pilot surfaces (root layout skip link, `MaintenanceBanner`), after which issue #799 migrated the rest and split `messages/en.ts` into per-namespace files. Pinning the formatter is the one intended user-visible change: banner dates read in English on every host. `format.ts` caches its `Intl` constructors asserted by spying on them; the `@ts-expect-error` lines in `translate.test.ts` are a load-bearing gate because `tsconfig.json` type checks tests, so a `MessageKey` degrading to `string` fails `tsc --noEmit`.
**→ Read**: [0055-ui-string-catalog-without-locale-routing.md](./0055-ui-string-catalog-without-locale-routing.md)

### ADR-0056: The shadcn token vocabulary is the design-system contract

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-08-31
**Problem**: `frontend/` was configured for shadcn (components.json on base-nova, shadcn@4, primitives in `src/components/ui/`) but nothing was connected: registry components are written against a fixed token vocabulary (`bg-primary`, `text-muted-foreground`) while `globals.css` defined a different one and none of the shadcn names. Tailwind has no unknown-class error, so the failure was silent: compiling the real stylesheet found 119 dead utilities across 23 files, incl. 27 in `ui/button.tsx` (no background) and `text-destructive` in 35 places (error copy never red). Nobody used primitives that looked broken, so the code grew 146 raw buttons and 79 raw inputs; `DESIGN_SYSTEM.md` documented an abandoned slate palette.
**Decision**: A shadcn compatibility layer in `@theme`: the registry's names as `var()` aliases onto the tokens this project already defines, plus four absolute-`oklch()` status tokens (`destructive`/`success`/`warning`/`info`) tuned to clear 4.5:1 as text on all three surfaces. Aliasing is one-directional and value-free; an alias carrying its own colour forks the palette. Load-bearing mappings: `--color-popover` to `--color-surface` (not surface-2), and shadcn's accent being the hover/selected background. Enforcement is `design-tokens.test.ts`, compiling with the real Tailwind compiler and scanner, at zero suppressions, in `frontend-a11y.yml`. Registry files stay byte-identical to upstream; the OBS render surfaces are out of scope as broadcast art; this is a visible UI change and the call-site migration is deliberately deferred (it collided with the i18n migration, 96 conflict hunks vs 10).
**Impact**: The class of bug is closed in CI rather than documented: unmodified registry components render correctly (verified by adding tabs, select, toggle-group, textarea, label, separator), the 119 dead utilities are live CSS, and the contrast lock asserts every alias resolves to the token it claims. Raw palette classes are migrated in semantic passes, with the premium pass done (its own `--color-premium` token) and warning, admin, dashboard and editor surfaces still open.
**→ Read**: [0056-shadcn-token-vocabulary-as-the-design-system-contract.md](./0056-shadcn-token-vocabulary-as-the-design-system-contract.md)

### ADR-0057: A settable text outline samples the whole disc, not the compass

**Status**: ✅ Accepted
**Date**: 2026-09-03
**Problem**: PR #833 made the chat outline's thickness settable (1-6px) and shipped it broken at every width above 1: ghost copies of the text with background showing through. A zero-blur `text-shadow` layer paints one translated copy of the glyph, so the outline is the union of the copies; #833 sampled the eight compass points at radius `w`, which touch at `w = 1` but from `w = 2` put the diagonals `w * sqrt(2)` out with nothing between. Its unit tests asserted the shape of the string and also never ran.
**Decision**: The offsets are every integer point inside the disc of radius `w`, making the painted region the glyph dilated by that disc. Integer spacing is load-bearing; a half-lattice opens hairline gaps and a bare ring leaves a hole inside 1px features. The cost is accepted rather than engineered away: everything through 4px/48 layers sits at the vsync floor, 5px/80 costs 21.1ms and 6px/112 costs 25.7ms, which is why the range stops at 6. The width lives in the declaration, but `visual-settings-to-css.ts` re-derives it through `canonicalizeTextShadow` on the way out, so overlays saved while #833 was live are fixed by deploy, not by migration. ADR-0044 stands: stroke stays banned.
**Impact**: Two gates, because two independent failures were needed. `text-outline.test.ts` rasterises the offsets over glyph-like probes and asserts reach and solidity, exact for an integer disc, and requires both to fail on the exact string #833 shipped; it is a TypeScript rasterisation rather than a screenshot because Playwright's browsers cannot run on a NixOS dev host. And the frontend unit project now runs on a pull request via `a11y-static`, which it previously did not, so #833's tests had never been executed.
**→ Read**: [0057-outline-thickness-samples-the-disc.md](./0057-outline-thickness-samples-the-disc.md)

### ADR-0058: An Owncast source is the instance URL, not a channel name

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-09-08
**Problem**: Every chat platform All-Chat supports addresses content by channel name or platform id, but Owncast has no channels, so the only identity that picks out the content is the instance's base URL, and an instance also goes offline or upgrades independently of All-Chat, so a listener that treats "unreachable" as a fault will crash-loop against a temporary outage that is correct to tolerate.
**Decision**: The instance base URL (normalized to `scheme://host`) is the stored channel identifier, rejected at add time in any other shape. Display name comes from the instance's own `/api/config`, best-effort: a fetch failure falls back to the hostname and never fails the add. Offline is a normal state: capped backoff reconnects, `platform:status` offline with `next_retry_at` (ADR-0032), `is_active = false`, no error spam. Registration repeats per connection and only `CHAT` events publish.
**Impact**: The only platform whose source picker asks for a URL. No OAuth, no auth-service change. Deployment adds `owncast-listener` (port 8095) and migration 091 (`platform_owncast` gate seed); the frontend still hides the platform outside beta.allch.at.
**→ Read**: [0058-owncast-instance-url-as-channel-identifier.md](./0058-owncast-instance-url-as-channel-identifier.md)

### ADR-0059: Picarto chat via the unofficial pop-out WebSocket feed

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-09-08
**Problem**: Picarto publishes a REST API for channel metadata but no public way to follow chat. What exists is the pop-out chat page's WebSocket, verified live against the production bundle's build constants and an anonymous capture, undocumented and free to change in any deploy without notice.
**Decision**: Read over the unofficial pop-out WebSocket (`wss://chat.picarto.tv/chat/token=<jwt>`, anonymous JWT from Picarto's internal GraphQL, no client hello). Protocol interpretation is isolated in `services/picarto-listener/websocket/`; unknown or malformed frames are log-and-dropped, never a panic. Emotes stay shortcodes and the normalizer strips them. The risk is priced into the gate: `platform_picarto` seeds `is_premium = TRUE` (migration 093) until the feed has been observed stable in production; the frontend still hides Picarto outside beta.allch.at.
**Impact**: A Picarto deploy can break chat with no warning; detection is dropped-message metrics and the reconnect loop, not a streamer-facing error. Source changes take up to 30 s to connect.
**→ Read**: [0059-picarto-unofficial-popout-websocket.md](./0059-picarto-unofficial-popout-websocket.md)

### ADR-0060: Facebook via Graph API polling with a dual-scope page credential and a moderation write path

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-09-08
**Problem**: Facebook Live comments have no public realtime API (webhooks need a public HTTPS endpoint and a verify token), so polling is the only route that fits the cloud-hosted model. Reading needs `pages_read_engagement` and moderating needs `pages_manage_engagement`, both behind App Review; requesting the write scope later would mean a second consent round per streamer and a second review cycle. The credential that works is the non-expiring Page token.
**Decision**: `facebook-listener` polls the live-video comments edge (`order=reverse_chronological`, `since` cursor pinned to the newest `created_time`). No quota subsystem: the Pages rate limit is `4800 * engaged users` per rolling 24 h, policed by reading the `X-Business-Use-Case-Usage` header and warning at 90%. The OAuth callback exchanges all the way to the Page token and stores it encrypted in `facebook_oauth_tokens` (migration 094) with no refresh token and no expiry column; token-refresh-service skips the platform. Moderation maps ADR-0017's write path onto `DELETE /{comment-id}` and the `blocked` edge; hide/unhide and timeout are deliberately not offered, and delegated moderation is refused. Single replica, no leader election; source adds fail closed on the token row.
**Impact**: Facebook costs the standard platform surface minus the quota ledger and minus token refresh. App Review is the deployment bottleneck; dev mode covers the integration window. Migration 094 seeds the `platform_facebook` gate, hidden outside beta.allch.at until review passes.
**→ Read**: [0060-facebook-graph-api-and-moderation.md](./0060-facebook-graph-api-and-moderation.md)

### ADR-0061: Rumble chat via the internal chat pop-up SSE endpoint (withdrawn)

**Status**: ⛔ Withdrawn (2026-10-08) (revised 2026-11-03)
**Date**: 2026-09-08
**Problem**: The product model requires reading chat from arbitrary channels a streamer adds as sources, but Rumble's official Live Stream API reports only the creator's own streams and embeds the stream key in its URL, so it cannot address arbitrary channels and was ruled out by the spike.
**Decision**: Read via Rumble's internal chat API, the same SSE endpoint (`GET .../chat/api/chat/<chat_id>/stream`) the official pop-up page drives, with the numeric `chat_id` as the stored channel identifier and the ToS grey zone accepted knowingly. On 2026-10-08 support was withdrawn: there is no API that can read arbitrary channels, and Rumble's political affiliations are ones the project does not want to support. The listener, its normalizer and migration 095 were removed; migration 099 drops the platform row, gate and sources and is idempotent. Bringing Rumble back would require a new ADR and a new spike.
**Impact**: Rumble sources added by beta users before withdrawal were deleted by migration 099 and their overlay feed is lost (the down migration cannot restore it), accepted as the price of the withdrawal. Rumble is not a supported platform.
**→ Read**: [0061-rumble-chat-popup-sse.md](./0061-rumble-chat-popup-sse.md)

### ADR-0062: Instagram Live comments via HTTP polling of `live_comments` behind a Page-linked IG credential

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-09-11
**Problem**: Instagram Live is the last phase of the platform expansion plan. Comments on an IG live broadcast are only readable via `GET /{ig-user-id}/live_comments` on the streamer's IG professional account (business/creator, linked to a Facebook Page), and only while the media is being broadcast. There is no webhook-free realtime public API; webhooks would need a public HTTPS endpoint and a verify token. IG rate limits are BUC-based like Facebook's, so no quota subsystem is warranted.
**Decision**: A new `instagram-listener` service (port 8100) polling `live_comments` with a `comment_id` id-cursor (comments cannot be filtered by timestamp), resolving the currently-broadcast media via `live_media` where an empty set is the normal not-live state. Separate `instagram_oauth_tokens` table (migration 096) keyed `(user_id, ig_user_id)`; the stored credential is preferably the non-expiring Page token from the same Meta app as Facebook's (ADR-0060 chain) with `expires_at = NULL`, falling back to the 60-day user token with `expires_at` set. Scopes `instagram_basic` + `instagram_manage_comments` (+ `pages_show_list` as the enumeration dependency), folded into the next App Review cycle. Read-only: no moderation write path, no emotes, no quota; single replica, no HPA.
**Impact**: A streamer needs an IG professional account linked to a Page; personal accounts cannot connect. Sources add fail-closed unless a token row exists for the exact `(user, ig_user_id)` pair. token-refresh-service has nothing to do for Instagram.
**→ Read**: [0062-instagram-live-comments-polling.md](./0062-instagram-live-comments-polling.md)
### ADR-0064: TikTok transport tiers by entitlement

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-09-17
**Problem**: The TikTok listener's only transport is its own Node WebSocket, whose connect handshake is exposed to TikTok session scoring — the "Unexpected server response: 200" flap. Fast retries clear most flaps; a room whose flap does not clear parks in escalating error backoff with no delivery at all, while the signer already holds a viewer tab for that same room receiving the identical PushFrames (until now tapped read-only by the phase-2 canary, never delivered).
**Decision**: A premium-only second tier: on flap exhaustion, a room whose streamer is premium (`users.is_premium` via overlay ownership, TTL-cached, fail-closed) switches delivery to the signer's viewer-tab relay — frames decoded with the connector's own schemas and replayed into the room's `TikTokLiveConnection` so all handlers, dedup and heartbeat behave exactly as on the primary. A stint ends on stream end, tab death, signer refusal, or a max-duration ceiling (default 6h) — not on "primary health", which is unobservable while the fallback is delivering. Gated by two env flags, both off by default (`SIGNER_RELAY_FALLBACK` signer-side, `TIKTOK_PREMIUM_FALLBACK` listener-side).
**Impact**: Premium rooms ride out flap walls instead of going dark; the signer pod carries one ~150MB tab per fallback room, bounded by the stint ceiling. Tier-transition duplicates are absorbed by the existing dedup layer. Rebuilding the shared decode path exposed a phase-2 bug: the canary never awaited the async `deserializeWebSocketMessage`, so every relay frame had counted as an ack and two of its four divergence checks could never fire — now regression-tested with real wire-format frames.
**→ Read**: [0064-tiktok-transport-tiers-by-entitlement.md](./0064-tiktok-transport-tiers-by-entitlement.md)

### ADR-0063: Beta-tester localization contribution with admin review and a repo-based catalog pipeline

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-09-15
**Problem**: Community members want to translate the UI, but the string catalog is typed code in the repo (ADR-0055), contributors are not developers, and a service that commits to the repo would need a credential and a blast radius no contributor feature justifies.
**Decision**: A guided translator tool at `/translate` (pick language → pick namespace → translate string by string with placeholder hints), gated by the `localization_contribution` early-access gate (migration 097, share-service routes behind RequireEarlyAccess). Open, request-based locales; one row per (locale, key) that resubmission overwrites and resets to pending; admin review at `/admin/localization` with rejection notes. The key list is derived from the English catalog in the frontend, never stored in the DB. Approved rows export as JSON; `scripts/generate-locale-catalog.mjs` turns the export into `messages/<locale>/` files in the catalog's shape, and the locale wiring stays a reviewed PR edit. Nothing auto-commits.
**Impact**: Beta testers (and ambassadors) gain a translation surface without touching git; the catalog stays the single source of truth; graduating the gate opens contribution to all authenticated users with no deploy.
**→ Read**: [0063-localization-contribution.md](./0063-localization-contribution.md)

### ADR-0065: Unlisted and members-only YouTube streams: pinned links and a premium official-API listener

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-10-10
**Problem**: innertube finds a channel's live stream by browsing the channel's `/streams` tab, which never lists unlisted streams, and it polls anonymously, so members-only chat is unreadable. ADR-0023/0025 had decided against bringing the quota-based Data API listener back for every channel.
**Decision**: A pinned stream link (`config.stream_id`) for every user, tried by innertube on every discovery attempt before the browse, so a stale pin never hides a public stream. For premium streamers, an opt-in `config.official_api` (gate `youtube_official_api`, migration 102) routes an owned channel to the revived `youtube-listener`: ownership proven with `channels.list(mine=true)` (a token row alone is not proof), discovery with `liveBroadcasts.list broadcastStatus=active`, chat over gRPC `streamList`. The listener claims each channel with a 3-minute TTL key (ADR-0015 pattern), only while an opted-in owner's overlay is connected; innertube skips claimed channels and resumes when the claim lapses (premium lost, token revoked, quota Critical, overlay disconnected, listener down).
**Impact**: Unlisted streams work for free; members-only chat works for premium streamers; quota is spent only on opted-in channels while their overlay is open. The listener runs as `youtube-listener-official` (2 replicas, global-sync lease holder claims and serves; the other is a warm standby), deployed via caesar-deployment (2026-10-10).
**→ Read**: [0065-premium-official-youtube-api-listener.md](./0065-premium-official-youtube-api-listener.md)

### ADR-0066: German as a second UI language, chosen per browser and applied after hydration

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-10-15
**Problem**: ADR-0055 deferred how a second locale is chosen. Locale may not enter the URL, a request-time read would make every static page dynamic, and the frontend has no React Context.
**Decision**: The choice is stored in the browser (`localStorage.allchat_locale`) and applied after hydration by `LocaleSync` in the root layout; the active locale is a module singleton that `useTranslations()` subscribes to via `useSyncExternalStore`. Non-English catalogs load on demand and fall back to English per key. German covers every namespace except `legal`, which stays English until a legal review signs it off. A Settings card offers the picker, gated by `ui_languages` (migration 103); ADR-0078 later amends where the picker lives (an ungated homepage switch too) and who the gate admits.
**Impact**: No URL, middleware or static-rendering change. Server-rendered pages stay English, and overlays in OBS stay English unless chosen there. The gate controls who can pick, not what renders; a stored choice keeps applying after a gate flip.
**→ Read**: [0066-client-side-ui-locale-selection.md](./0066-client-side-ui-locale-selection.md)

### ADR-0067: Desktop plugins move to their own repositories under GPL-3.0-or-later

**Status**: ✅ Accepted
**Date**: 2026-10-16
**Problem**: The StreamController store lists repositories, not subdirectories, so the plugin could not be submitted from this monorepo; ADR-0049's AGPL-in-GPL licence gate was still open; and contributing to one plugin meant cloning the whole platform.
**Decision**: `caesarakalaeii/allchat-streamdeck` and `caesarakalaeii/allchat-streamcontroller`, split out with history and relicensed GPL-3.0-or-later (the StreamController host's licence). They share no code, so they get one repository each. `scripts/check-plugin-parity.py` stays here and runs over three sibling checkouts; every repository's parity job checks out a same-named branch from the others when one exists, so a cross-repo change is checked as a set.
**Impact**: Store submission and per-plugin releases become possible; a cross-plugin change is up to three PRs; plugin code is no longer under this repository's licence.
**→ Read**: [0067-desktop-plugins-in-own-repositories.md](./0067-desktop-plugins-in-own-repositories.md)

### ADR-0068: Event alerts platform — alert overlay kinds, a separate chat:raw consumer, and presigned-upload alert media

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-10-01
**Problem**: Event data already flows through `chat:raw` as event-typed messages (Twitch/YouTube/TikTok listeners), but nothing routes it to a non-chat rendering surface, keeps event history, or holds streamer-uploaded alert media — and the chat pipeline must not grow a second job.
**Decision**: Three new overlay kinds (`overlays.overlay_type` 'alerts'/'goal'/'list', default 'chat'); a separate **alert-processor** consuming `chat:raw` with its own consumer group `alert-processors`, normalizing via the message-processor's own normalizers (one-direction import), routing to alert-capable overlays, persisting to `alert_events`, publishing to `overlay:{id}:alerts`, and scoring `list` leaderboards — with idempotency as a first-class contract (deterministic UUIDv5 alert ids + ON CONFLICT DO NOTHING for history, per-alert SET NX claim inside one atomic Lua script for boards, ACK only after persist+publish). Alert media in single-node MinIO (public-read behind `media.allch.at`, presigned direct uploads, unguessable `{user_id}/{uuid}/{filename}` keys, `media_objects` index, 10 MiB cap).
**Impact**: Alert features land without touching listeners or the chat pipeline; a broken alert-processor degrades to "no alerts", never "no chat"; redelivery can animate an alert twice (accepted, cosmetic) but never double-writes history or double-counts a board; single-node MinIO defers distribution/replication as follow-ups.
**→ Read**: [0068-event-alerts-platform.md](./0068-event-alerts-platform.md)

---

### ADR-0069: A stream session ends only when no gateway pod holds the overlay, after a 15-minute grace

**Status**: ✅ Accepted
**Date**: 2026-10-16
**Problem**: Credits showed 2h48 for a 4h18 stream. Each api-gateway pod ended the shared session when its own pool emptied, even while another pod still served the OBS overlay, and a 60-second grace split a stream on any scene switch or short drop. Each split also reset the leaderboards.
**Decision**: Pods register in a `session:holders:{overlay}` sorted set (heartbeat-scored, stale after 10 min); the session is only marked ENDING or ended when no holder remains, after `SESSION_END_GRACE_SECONDS` (default 900), separate from the 60s WebSocket grace. `ending_at` lets the next connect replace a session whose ending pod died. A live session is measured from the earlier of its start and the live Twitch stream start (Helix `GET /streams`); other platforms are not consulted yet.
**Impact**: One session per stream across pods and short drops; streams under 15 minutes apart merge; completed-session credits measure to `ended_at` instead of growing since stream start.
**→ Read**: [0069-stream-session-spans-gateway-pods-and-short-drops.md](./0069-stream-session-spans-gateway-pods-and-short-drops.md)

---

### ADR-0070: Alert overlays carry chat sources; the gateway resolves alert profiles

**Status**: ✅ Accepted
**Date**: 2026-10-16
**Problem**: ADR-0068 routes events by an alert overlay's `overlay_chat_sources`, but overlay-manager refused chat sources on non-chat overlays, so routing never matched (zero `alert_events` in prod); and nothing subscribed `overlay:{id}:alerts` or stored how an alert looks (#952).
**Decision**: Every overlay kind takes ordinary chat sources; the message-processor routes chat only to `overlay_type = 'chat'`. The gateway subscribes `overlay:{id}:alerts` for non-chat overlays (sibling of `:poll`/`:prediction`) and forwards `type: "alert"` frames, the envelope plus the resolved `profile` (saved row in `alert_profiles` or the Go default in `shared/alerts`, placeholders substituted server-side; disabled profiles dropped; never replayed). overlay-manager serves `GET/PUT /overlays/:id/alerts/profiles` (PUT premium via `alerts_editor`, alerts overlays only) and `POST /overlays/:id/alerts/test`. Migration 107.
**Impact**: Alert overlays use the source flow streamers already know; one resolver serves real and test alerts; adding an event type is a code change. Builds on ADR-0068.
**→ Read**: [0070-alert-overlays-carry-chat-sources-and-gateway-resolves-alert-profiles.md](./0070-alert-overlays-carry-chat-sources-and-gateway-resolves-alert-profiles.md)

---

### ADR-0071: The alerts editor — premium saving, sandboxed custom code under the page CSP, client-side KLIPY search

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-10-16
**Problem**: Alerts overlays had stored profiles but no editor (the dashboard showed chat appearance sections), and issue #954 asked for premium custom HTML/CSS/JS, KLIPY GIF search and media uploads. Streamer JS needs isolation that holds against hostile code without weakening the strict overlay CSP, and no browser upload could ever have reached MinIO.
**Decision**: Saving stays premium (`alerts_editor`, ADR-0070); defaults and test alerts are free. Custom code (migration 109, 64 KiB per field) renders in `<iframe sandbox="allow-scripts" srcdoc>` (opaque origin, no other tokens) with a `{type:"alert", profile, event}` postMessage bridge and an injected `onAlert`/`allchat.fill` runtime. It runs under the inherited page CSP, unchanged, through `'unsafe-inline'`; a move to nonces must carry them into the srcdoc. KLIPY search runs in the browser with a key fetched at runtime from `GET /klipy-config` (ADR-0073), restricted to entitled sessions (ADR-0074), and degrades to "unavailable". Media upload was made reachable: the gateway now proxies `/api/v1/media`, media-service presigns for the public host, and `MEDIA_PUBLIC_URL` carries the bucket segment.
**Impact**: Hostile alert code is contained by the browser's origin boundary; no CSP or host changes. Custom code is inline-only (no external scripts outside the CSP's hosts, no `eval`). Search needs `KLIPY_API_KEY` in the frontend deployment; without it the picker is unavailable.
**→ Read**: [0071-alerts-editor-sandboxed-custom-code-and-client-side-gif-search.md](./0071-alerts-editor-sandboxed-custom-code-and-client-side-gif-search.md)

---

### ADR-0072: Goal and list overlays — demand-gated goal polls in the alert-processor, debounced leaderboard pushes

**Status**: ✅ Accepted
**Date**: 2026-10-16
**Problem**: Goal and list overlays (ADR-0068) had no renderer, config or data path. Follower/subscriber counts must be polled from platform APIs (YouTube through the shared quota ledger) without polling idle overlays or polling twice from two replicas, and a live leaderboard must update without a frame per cheer.
**Decision**: The alert-processor polls `followers` (Twitch Helix `channels/followers` `total`, owner token via `moderation-service/tokens`) and `subscribers` (YouTube `channels.list?part=statistics`, 1 unit, reserve/confirm/rollback) every `GOAL_POLL_SECONDS` (60) only for overlays whose `overlay:connected:{id}` demand key exists, behind a per-overlay `SET NX` lock. `bits`/`super_chat` goals are once-per-alert `INCRBYFLOAT` counters. Lists get `list_update` frames debounced to one per board per 2s via a Redis window claim that defers rather than drops. Public `GET /overlays/public/:id/goal` and `GET /overlays/:id/list/top` seed the renderer; the gateway forwards `goal`/`list_update` frames under their own WS types. Creation gated by `goal_overlay` / `list_overlay` (migrations 110/111, seeded premium).
**Impact**: No new service or coordinator; YouTube polls show up in the existing quota gauge. The alert-processor now needs the token cipher and Twitch/YouTube app credentials. YouTube subscriber counts are rounded by YouTube.
**→ Read**: [0072-goal-and-list-overlays-polled-goals-and-debounced-leaderboards.md](./0072-goal-and-list-overlays-polled-goals-and-debounced-leaderboards.md)

---

### ADR-0073: The KLIPY app key is delivered at runtime from the cluster secret

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-10-17
**Problem**: ADR-0071 baked the KLIPY key into the image (`NEXT_PUBLIC_KLIPY_API_KEY`), but the key lives in the `allchat-secrets` Kubernetes secret, which a build-time variable cannot read; KLIPY forbids proxying search through our servers without written approval.
**Decision**: `GET /klipy-config` (a Next.js route outside `/api/`, which the ingress sends to the gateway) returns `process.env.KLIPY_API_KEY` per request; the browser fetches it once and keeps calling `api.klipy.com` directly. The deployment sets the env from `secretKeyRef allchat-secrets/klipy-api-key`. Only the beta frontend carries the key today, so on the production frontend search reports "unavailable". The search placeholder reads "Search KLIPY" in every locale (KLIPY's REQUIRED attribution).
**Impact**: Rotating the key needs no rebuild; the build arg and CI variable are gone. The key stays public, as it always was in the search URLs; ADR-0074 restricts who receives it.
**→ Read**: [0073-klipy-app-key-delivered-at-runtime-from-the-cluster-secret.md](./0073-klipy-app-key-delivered-at-runtime-from-the-cluster-secret.md)

---

### ADR-0074: The KLIPY app key is served only to entitled sessions

**Status**: ✅ Accepted
**Date**: 2026-10-17
**Problem**: ADR-0073's `/klipy-config` returned the key to any unauthenticated caller, so one request scraped it; KLIPY still requires browser-side search, so the key must reach the picker user's browser.
**Decision**: The route forwards only the caller's `Cookie`/`Authorization` to the gateway's `GET /api/v1/auth/me` (`API_GATEWAY_URL`) and answers 401 (no or invalid session, or any check failure), 403 (neither `is_premium` nor `is_beta_tester`) or the key, always `Cache-Control: private, no-store` + `Vary: Cookie, Authorization`. The picker fetches it with `credentials: 'same-origin'`. The frontend pods get `API_GATEWAY_URL` and NetworkPolicy ingress to their gateway.
**Impact**: Anonymous and free users cannot read the key from All-Chat; entitled users still see it in their own search URLs. One `/auth/me` call per picker load.
**→ Read**: [0074-klipy-app-key-only-for-entitled-sessions.md](./0074-klipy-app-key-only-for-entitled-sessions.md)

---

### ADR-0075: Alert Profiles Inherit an All-Events Base Style

**Status**: ✅ Accepted
**Date**: 2026-10-17
**Problem**: A streamer who wanted the same GIF and sound on every alert had to save it once per event type (up to 20 saves), and "Send test alert" ignored unsaved editor changes because the gateway resolves tests from saved profiles, so picked GIFs never showed.
**Decision**: A stored profile with event type `all` became the base style. `alerts.ApplyBase` layers it under each event: an unsaved event inherits the base's media, timing and custom code; a saved event fills only its empty image/sound from the base (custom code as a unit); `enabled` is base AND event, so switching the base off silences every alert. GET/PUT /alerts/profiles return `all` first plus the event types that have a stored row, and the editor opens on "All events". "Send test alert" moved into the editor: it tests the edited event type and, for premium users, saves unsaved changes first. `power_up` (Twitch) and `gift_received` (YouTube) join `eventDefs`.
**Impact**: One GIF for every alert is one field and one save, at the cost of two profile lookups per alert at the gateway. No migration is needed and either side of the gateway/frontend pair can be deployed first; a saved event type cannot yet be reset to full inheritance from the editor.
**→ Read**: [0075-alert-profiles-inherit-an-all-events-base-style.md](./0075-alert-profiles-inherit-an-all-events-base-style.md)

---

### ADR-0076: Credit Roll Clips Play the Clip MP4 from Twitch's Web GQL

**Status**: ✅ Accepted
**Date**: 2026-10-17
**Problem**: Twitch's clip embed, the only official clip player, forces every autoplayed clip to `muted=true`/volume 0 whatever `muted=` says, ignores the Embed SDK's postMessage commands, and resets a manual unmute on the next load, so "Mute Clips Audio" off never produced sound in the credit roll; the Embed SDK takes no clips at all.
**Decision**: For the current clip, the credits page looks up its signed MP4 (`playbackAccessToken` + `videoQualities`) from `gql.twitch.tv/gql` in the browser with Twitch's public web client ID, and plays the best quality's `sourceURL` in its own `<video>` (rendered muted, taking `clips_muted` once data is loaded, advancing on `ended`). A failed lookup or video error plays that clip through the clip embed as before: muted, advanced on a timer of its length plus 3 s. No CSP change was needed.
**Impact**: Clip audio works in OBS (measured with OBS's autoplay policy; an ordinary browser tab plays muted and still plays), clips advance on their real end, and no player chrome or mature-content interstitial is shown. The GQL API is undocumented, so when it changes the clip loses sound but keeps playing; each clip costs one GQL request from the viewer's browser.
**→ Read**: [0076-credit-roll-clips-play-the-mp4-from-twitch-web-gql.md](./0076-credit-roll-clips-play-the-mp4-from-twitch-web-gql.md)

---

### ADR-0077: Kick Listener Follows Kick's Realtime Provider (Centrifugo or Pusher)

**Status**: ✅ Accepted
**Date**: 2026-10-18
**Problem**: Kick revoked kick-listener's hardcoded Pusher app key on 2026-10-17 (about 32.5h of dark Kick chat), and kick.com no longer starts from a fixed Pusher key: it asks `web.kick.com/api/v1/realtime/connection` which provider to use, and answers Centrifugo to any client that accepts both, so Pusher may stop being offered at any time.
**Decision**: At startup the listener asks that endpoint with the providers in `KICK_REALTIME_PROVIDERS` (default `centrifugo,pusher`) and builds the client for the provider Kick returns; a failed lookup starts the Pusher client as before. The new Centrifugo client pools connections at 100 chatrooms each (below Kick's 128-subscription limit), fetches and refreshes guest tokens from `/v1/realtime/auth/connection`, answers pings and reconnects with backoff, resubscribing its own chatrooms. Both clients satisfy one interface, share event decoding and drive the same `kick_listener_socket_state` gauge.
**Impact**: Kick chat keeps working on whichever transport Kick prefers, and `KICK_REALTIME_PROVIDERS=pusher` forces Pusher without a rebuild; nothing downstream changes because the chat payload is the same on both transports. Two transports must be maintained until Kick drops Pusher, and the design rests on undocumented Kick endpoints and limits.
**→ Read**: [0077-kick-listener-follows-kicks-realtime-provider.md](./0077-kick-listener-follows-kicks-realtime-provider.md)

---

### ADR-0078: Homepage Language Switch, and German Opened to Everyone

**Status**: ✅ Accepted
**Date**: 2026-10-19
**Problem**: ADR-0066 put the language picker on `/settings` only, behind the `ui_languages` gate, which signed-out visitors can never pass, so the audience a second language helps most, first-time visitors, could not read the homepage in German.
**Decision**: `components/home/LanguageToggle.tsx` renders one button per entry in `SUPPORTED_LOCALES` at the right end of the homepage lanes nav, calls `chooseLocale()` like the settings card and makes no request; below 480px the nav's "themes" link is hidden to make room. The `ui_languages` gate is flipped to `early_access=FALSE` in the admin UI, so the `/settings` card offers German to every signed-in user too.
**Impact**: Signed-out visitors can switch the homepage body to German, and the homepage switch and `/settings` agree. The switch lists every supported locale and does not check the gate, so a later beta-only language needs the switch filtered first, and re-closing the gate would hide German on `/settings` only. `metadata`, JSON-LD, Server Components and `legal.*` stay English.
**→ Read**: [0078-homepage-language-switch-and-german-for-everyone.md](./0078-homepage-language-switch-and-german-for-everyone.md)

---

### ADR-0079: Streamer Chat Commands in engagement-service, With a Sandboxed JS Template Engine

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-10-21
**Problem**: Streamers want what Nightbot, StreamElements and Fossabot give them in one chat across every merged platform: commands, keyword triggers, templated replies with JavaScript for what variables cannot express, and role limits, without taxing the message-processor hot path, without self-triggering replies, and with a bounded engine for streamer-written JavaScript on a shared service.
**Decision**: Options 1 and 4. Extend the ADR-0028 path: engagement-service keeps the `chatcmd:channels` SET (rebuilt atomically every 30s and after every command or timer write); message-processor holds a copy refreshed every 10s and XADDs every message of a listed channel, with the chatter's roles, as a `CommandJob` to `chatcmd:jobs` from the existing off-hot-path engagement forwarder after native dedup. engagement-service consumes the stream with at-most-once NOACK reads, matches first-match-wins, checks `min_role` and three Redis guards (echo, seen, cooldown), clips via auth-service `/internal/twitch/clip` (`clips:edit`, `clip` re-consent action), and posts replies through `/internal/chat/bot-send`, which since ADR-0081 resolves the sender: the All-Chat Bot by default, else the streamer's own account. Templates use single-brace tokens plus `{js …}` in QuickJS (`modernc.org/quickjs`, no host functions, all blocks of one render sharing 100 ms, 16 MB memory, 256 KB stack); YouTube replies require the owner's `users.is_premium`. The `chat_commands` gate was seeded `early_access=TRUE`; the editor is `/commands`, beta-only.
**Impact**: One new stream, one SET and three short-lived key families; `chat_commands` table and the QuickJS dependency. Replies go only to the streamer's own channels (no send path for TikTok/Discord), roles exist only on Twitch, Kick and YouTube (others count as everyone), and Twitch sending needs the users-row Twitch login. ADR-0081 superseded the original "replies come from the streamer's own account" and "timers left out"; ADR-0080 later added `{urlfetch}` and image_url (migration 114).
**→ Read**: [0079-streamer-chat-commands-in-engagement-service.md](./0079-streamer-chat-commands-in-engagement-service.md)

---

### ADR-0080: Chat Command urlfetch and Images, With a Guarded Outbound HTTP Client

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-10-22
**Problem**: ADR-0079 left out Nightbot's `$(urlfetch)` because a server-side fetch of a user-chosen URL is an SSRF surface in a cluster whose egress is open (any pod can reach private ranges and the metadata endpoint), and streamers also want a command to put an image on stream in chats that only accept text.
**Decision**: `{urlfetch}` runs on the server behind new `shared/safehttp`: its dialer refuses every IP that is not public unicast after DNS resolution (covering DNS rebinding and redirects), the client never uses a proxy and follows at most 3 redirects. The URL must be https on port 443, no credentials, at most 2048 bytes, same at every hop, variables query-escaped, at most 3 fetches per render, 3 s each, 32 KB body; inside `{js}` the body enters as a string literal or `null`, and the JS budget now counts only time in `Eval`. The preview really fetches under the per-user rate limit. A command's `image_url` (migration 114) is rendered with chatter input withheld, becomes an `image_url` override on a `chat_command` alert on the owner's alerts overlays, and fires from any chat in the streamer's overlays. The Owncast listener (its instance URL is user input) uses the client and dialer now.
**Impact**: Our servers fetch third-party URLs for streamers (a fetch-heavy command adds up to about 9 s; the User-Agent identifies the feature and cooldowns plus the preview rate limit bound volume), and images need an alerts overlay, which is premium-gated. Any new code dialing a user-chosen URL must use `shared/safehttp`; a local Owncast on a private address is now refused. ADR-0081 later added the `custom_chat_bot` gate; timers and custom bots arrived separately.
**→ Read**: [0080-chat-command-urlfetch-and-images.md](./0080-chat-command-urlfetch-and-images.md)

---

### ADR-0081: Chat Command Replies From an All-Chat Bot, Premium Custom Bots, and Timers

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-10-23
**Problem**: ADR-0079 posts every reply from the streamer's own account and leaves timers out; streamers ask for a branded All-Chat Bot sender, a premium custom bot name like Wizebot sells, and timers posting every X minutes, every X chat messages, or both, firing exactly once across replicas with no live/offline signal.
**Decision**: New auth-service `POST /internal/chat/bot-send` resolves the sender per platform: the streamer's `chat_bot_settings.sender` choice of `self`, else a premium custom bot, else the All-Chat Bot, falling back to the streamer when a grant or bot row is missing (with `sender: "self"` reported). Platform mechanisms differ: Twitch posts via Helix with an app access token (bot authorizes `user:bot`/`user:write:chat`, broadcaster grants `channel:bot` through a new `bot` re-consent action); Kick posts `type: "bot"` with the streamer's token; YouTube posts with the bot's own Google token. Accounts live in `chat_bot_accounts` (migration 115, auth-service-owned), settings in `chat_bot_settings`; the connect flows reuse each platform's callback, admin page `/admin/chat-bots`. Timers (`chat_timers`, migration 116, at most 20 per streamer) count unique lines per owner in Redis and fire from a 15 s tick on every replica, each due timer claimed by a conditional `UPDATE` so exactly one replica posts; timer YouTube sends read the live chat id only from the Redis stream state, never a Data API lookup. Gate `custom_chat_bot` (premium, early access); timer and sender-setting writes use the `chat_commands` gate.
**Impact**: Operator setup is required (create bot accounts, connect on `/admin/chat-bots`, configure the Kick app bot) and streamers must grant `channel:bot` once; until then Twitch and YouTube replies fall back to the streamer. Time-only timers post into offline Twitch/Kick chats. This supersedes in part ADR-0079's "replies from the streamer's own account" and "timers left out".
**→ Read**: [0081-chat-command-bot-sender-custom-bots-and-timers.md](./0081-chat-command-bot-sender-custom-bots-and-timers.md)

---

### ADR-0082: Altstream chat via the official public SSE chat API

**Status**: ✅ Accepted
**Date**: 2026-10-31
**Problem**: Streamers who multistream to Altstream (six asks on Bluesky in October 2026, the first demand from existing users rather than a market survey) want its chat in their overlay. Altstream is a small, community-funded, queer-friendly, anti-generative-AI platform whose values fit All-Chat, the opposite of the values reason that withdrew Rumble (ADR-0061).
**Decision**: New `altstream-listener` (port 8101) reads the official, documented, unauthenticated developer API: the SSE chat stream per channel plus documented REST for live status and emote catalogs, publishing to `chat:raw` like every listener. v1 is read-only (`message` and `highlight` events, native `delete`/`clear`, other types dropped with a metric; no send or moderation path). The SSE `lastId` parameter is mandatory and undocumented, so the listener always sends a non-empty one, resumes with the last id it saw and drops catch-up events older than the connection. Rolled out as early access (ADR-0020): `platform_altstream` seeded `early_access=TRUE, is_premium=FALSE` (migration 117), Altstream offered on every host but on allch.at only to early-access users, and the listener runs from `:main`.
**Impact**: No credentials to store or auth-service change, and the no-OAuth channel-name SSE shape Rumble proved without its ToS grey zone. Accepted risks: a three-month-old, one-developer platform with frequent October outages; a new API whose docs already lag the server; one SSE connection per watched channel from the cluster's egress IP; no full gap replay; unwatched channels' emotes stay text; no alerts (raid/redemption/tip/subscription dropped).
**→ Read**: [0082-altstream-public-sse-chat.md](./0082-altstream-public-sse-chat.md)

---

### ADR-0086: Server-side pagination and search for the admin lists

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-11-02
**Problem**: The admin Users, Overlays and Sources pages each fetched their whole table in one request and did search, filters and counting in the browser (with per-user aggregate queries), and global search fetched all three full lists to show eight matches per group; the Users and Sources pages became slow as the platform grew, out-running both the old design and ADR-0035's accepted full-list load.
**Decision**: The three list endpoints take `limit` (default 50, at most 100)/`offset` and answer `{ <users|overlays|sources>, total, limit, offset }` in the Viewers paging contract (ADR-0034), ordered newest first with an id tiebreak. `q` is a case-insensitive literal substring search with `%`, `_` and `\` escaped, over defined per-entity fields; filters move server-side (users with `filter` and `counts` computed over the same search definitions, sources with platform/status/user, overlays with `ids`). The pages load one page at a time with endless scrolling (`usePagedList` + `<LoadMore>`, the footer also a real button for screen-reader use); the connected-only Overlays view loads its bounded set whole by `ids`; global search asks each endpoint for `q` with `limit=8`, superseding ADR-0035's client-side federation so the dedicated search endpoint it anticipated is not needed.
**Impact**: One bounded query per page load whatever the table size, and one paging contract across all four admin lists. Breaking response shape: frontend and backend must ship together (admin only; `beta` breaks the same way until `main` is merged). User tab counts still aggregate over the search on every request, offset paging can skip or repeat a row on concurrent inserts, and a mutation reloads the Users list from the first page.
**→ Read**: [0086-admin-list-server-side-pagination.md](./0086-admin-list-server-side-pagination.md)

---

### ADR-0089: Enforce data retention with a sweeper CronJob

**Status**: ✅ Accepted (revised 2026-11-03)
**Date**: 2026-11-02
**Problem**: The privacy policy's retention periods (messages one hour, YouTube quota audit logs 30 days, expired OAuth tokens 7 days) were never enforced: the cleanups were SQL functions scheduled inside `IF EXISTS pg_cron` blocks, pg_cron is not installed, nothing else called them, and the 016 YouTube-audit function can never even run. The token rule, which deleted credentials whose access token expired 7+ days ago, would have deleted working credentials refreshable by token-refresh-service or a reactive refresher.
**Decision**: A `retention-sweeper` binary in the auth-service image, run by a Kubernetes CronJob every 15 minutes (`concurrencyPolicy: Forbid`, `backoffLimit: 0`), deletes in batches of 1000 with `FOR UPDATE SKIP LOCKED` over a single connection, re-checking each predicate under lock so a refresh that moved a row out of the window is kept; one table failing does not stop the others. Rules: viewer messages after 1 hour, YouTube audit log and `alert_events` after 30 days, `moderation_actions` after 90 days, abandoned device-link requests 1 hour after expiry; only OAuth credentials provably dead are swept (legacy TikTok rows, legacy Kick rows without `kick_user_id`, expired Instagram tokens), and the other credential tables are left to account deletion because nothing in them proves death. Migration 118 drops the pg_cron functions and adds the `created_at` indexes the deletes need.
**Impact**: The policy periods hold and are checked by an integration test against the migrated schema in CI, with no new service, image or pooled connections. The admin viewer Activity dialog now covers only the last hour, nothing alerts when the CronJob stops running, and the privacy policy has to say that most tokens go with the account, with the 1-hour message promise holding to between 60 and about 75 minutes.
**→ Read**: [0089-retention-sweeper-cronjob.md](./0089-retention-sweeper-cronjob.md)

---

## How to Create a New ADR

### Step 1: Determine ADR Number

```bash
# Highest ADR number in this repo
ls docs/adr | grep -E '^[0-9]{4}-' | sort | tail -1
```

ADR numbers are shared with caesar-deployment, so check its `docs/adr/` too and take the next number free in both.

### Step 2: Create ADR File

```bash
# Use 4-digit number with leading zeros
touch docs/adr/NNNN-your-decision-title.md
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

- Status (📝 Proposed initially; append `(revised YYYY-MM-DD)` whenever the ADR gets a `**Revised**` line)
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

**Keep ADRs current**:

- ✅ When code changes a fact an ADR states (service, path, env var, table, gate, flow, number), fix the ADR body in the same PR and add or bump its `**Revised**: YYYY-MM-DD` line
- ✅ When a later ADR amends an earlier one, fold the amendment into the earlier ADR's body with a cross-link; keep the `Status` line short
- ✅ When a decision is replaced outright, create a new ADR, set the old one to "Superseded by ADR-XXXX", and reduce its body to a pointer at the successor plus the rationale that still matters
- ✅ Update this index entry whenever an ADR's status or decision changes

**Do NOT**:

- ❌ Delete or renumber ADRs
- ❌ Leave a stale statement in an ADR body because "it was true at the time"

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

**Total ADRs**: see `docs/adr/` (ADR numbers are shared with caesar-deployment). The count was hardcoded here and drifted by twelve before anyone noticed; do not re-add a number.
**Last Updated**: 2026-11-03 (every ADR body checked against the implementation)
**Coverage**: see the ADR Index above; it lists every ADR file in this directory.
