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

**Total ADRs**: 14
**Status**: All accepted (✅)
**Coverage**: Core architecture decisions (Go layout, message flow, databases, frontend, quota tracking, feature gates, resilience patterns, pronoun enrichment, zombie detection, OAuth scope minimisation, overlay observability view, demand linger)

**Most Referenced**:
1. ADR-0002 (Redis patterns) - Referenced by all listeners, message processor
2. ADR-0001 (Go layout) - Referenced by all services
3. ADR-0006 (Quota tracking) - Referenced by YouTube listener, overlay manager

**Last Updated**: 2026-06-02
