# Architecture Documentation

This directory contains comprehensive architecture documentation for All-Chat, organized by topic with numbered prefixes for recommended reading order.

---

## Reading Order (For New Team Members)

**Start here** if you're new to the codebase:

1. **[00-OVERVIEW.md](./00-OVERVIEW.md)** (~487 lines, 15-20 min)
   - High-level system overview
   - Service map with all 13 services
   - Technology stack
   - Key design decisions
   - Links to detailed documentation

2. **[01-DATA-FLOW.md](./01-DATA-FLOW.md)** (~800 lines, 25-30 min)
   - End-to-end message flow
   - Redis Streams + Pub/Sub patterns
   - Unified message format
   - Platform-specific normalization
   - Integration patterns

3. **[02-DEPLOYMENT.md](./02-DEPLOYMENT.md)** (~600 lines, 20-25 min)
   - Kubernetes architecture
   - CloudNativePG PostgreSQL cluster
   - Service deployment manifests
   - HPA autoscaling configuration
   - Production deployment procedures

4. **[03-SCALING.md](./03-SCALING.md)** (~535 lines, 20-25 min)
   - Scalability analysis (honest capacity estimates)
   - Service-specific scaling strategies
   - Infrastructure scaling (PostgreSQL, Redis)
   - Performance bottlenecks
   - Capacity planning by phase

5. **[04-OBSERVABILITY.md](./04-OBSERVABILITY.md)** (~703 lines, 25-30 min)
   - LGTM stack (Loki, Grafana, Prometheus, Tempo)
   - Metrics (all services, 100% coverage)
   - Logging (structured Zap logs)
   - Alerting rules (critical + warning)
   - Dashboards and queries

6. **[05-SECURITY.md](./05-SECURITY.md)** (~400 lines, 15-20 min)
   - Security architecture
   - Threat model
   - OAuth flows
   - Known vulnerabilities
   - Mitigation strategies

**Total Reading Time**: ~2 hours for complete architecture understanding

---

## Quick Navigation

### By Topic

**Need to understand...**

| Topic | Document | Lines |
|-------|----------|-------|
| **System overview** | [00-OVERVIEW.md](./00-OVERVIEW.md) | 487 |
| **Message processing pipeline** | [01-DATA-FLOW.md](./01-DATA-FLOW.md) | 800 |
| **Kubernetes deployment** | [02-DEPLOYMENT.md](./02-DEPLOYMENT.md) | 600 |
| **Performance & scaling** | [03-SCALING.md](./03-SCALING.md) | 535 |
| **Metrics & monitoring** | [04-OBSERVABILITY.md](./04-OBSERVABILITY.md) | 703 |
| **Security & OAuth** | [05-SECURITY.md](./05-SECURITY.md) | 400 |

### By Service

**Looking for service-specific details?** See individual service READMEs:

- [api-gateway/README.md](../../services/api-gateway/README.md) - WebSocket hub, HTTP routing
- [auth-service/README.md](../../services/auth-service/README.md) - OAuth, JWT tokens
- [overlay-manager/README.md](../../services/overlay-manager/README.md) - Overlay CRUD
- [emote-service/README.md](../../services/emote-service/README.md) - 7TV, BTTV, FFZ cache
- [twitch-listener/README.md](../../services/twitch-listener/README.md) - IRC client
- [youtube-listener/README.md](../../services/youtube-listener/README.md) - API polling, quota
- [kick-listener/README.md](../../services/kick-listener/README.md) - Pusher WebSocket
- [tiktok-listener/README.md](../../services/tiktok-listener/README.md) - TikTok Live
- [message-processor/README.md](../../services/message-processor/README.md) - Normalization, enrichment
- [source-manager/README.md](../../services/source-manager/README.md) - Leader election
- [token-refresh-service/README.md](../../services/token-refresh-service/README.md) - OAuth refresh

### By Task

**Need to complete a specific task?** See quick reference guides:

- [QUICK-REF-ADD-PLATFORM.md](../llm-guides/QUICK-REF-ADD-PLATFORM.md) - Add new platform support
- [QUICK-REF-DEBUG-QUOTA.md](../llm-guides/QUICK-REF-DEBUG-QUOTA.md) - Debug YouTube quota issues
- [QUICK-REF-SCALING.md](../llm-guides/QUICK-REF-SCALING.md) - Scale services or infrastructure
- [QUICK-REF-KUBERNETES-DEBUG.md](../llm-guides/QUICK-REF-KUBERNETES-DEBUG.md) - Kubernetes troubleshooting
- [QUICK-REF-REDIS-OPERATIONS.md](../llm-guides/QUICK-REF-REDIS-OPERATIONS.md) - Redis Streams/Pub/Sub

### Design Decisions

**Want to understand WHY architectural decisions were made?** See ADRs:

- [ADR Index](../adr/README.md) - All Architecture Decision Records
- [ADR-0001](../adr/0001-standard-go-layout.md) - Standard Go Layout (not hexagonal)
- [ADR-0002](../adr/0002-redis-streams-pubsub.md) - Redis Streams + Pub/Sub hybrid
- [ADR-0003](../adr/0003-cloudnative-postgres.md) - CloudNativePG operator
- [ADR-0004](../adr/0004-no-hexagonal-architecture.md) - No ports/adapters abstraction
- [ADR-0005](../adr/0005-react-nextjs-frontend.md) - React + Next.js frontend
- [ADR-0006](../adr/0006-youtube-quota-tracking.md) - YouTube quota reserve-confirm-rollback

---

## Document Status

### Production-Ready (✅)

These documents are current, accurate, and reflect production implementation:

- ✅ [00-OVERVIEW.md](./00-OVERVIEW.md) - Updated 2026-01-28
- ✅ [01-DATA-FLOW.md](./01-DATA-FLOW.md) - Current implementation
- ✅ [02-DEPLOYMENT.md](./02-DEPLOYMENT.md) - Phase 4 complete
- ✅ [03-SCALING.md](./03-SCALING.md) - Honest capacity estimates
- ✅ [04-OBSERVABILITY.md](./04-OBSERVABILITY.md) - 100% service coverage
- ✅ [05-SECURITY.md](./05-SECURITY.md) - Known vulnerabilities documented

### Historical/Superseded (📚)

These documents are kept for historical reference but are superseded by consolidated docs:

- 📚 **APPROVED_ARCHITECTURE.md** - Original architecture plan (Phase 1 design)
  - **Superseded by**: 00-OVERVIEW.md (current state)

- 📚 **OBSERVABILITY_MONITORING.md** - Original observability plan
  - **Superseded by**: 04-OBSERVABILITY.md (consolidated)

- 📚 **SCALING_PERFORMANCE.md** - Original scaling plan
  - **Superseded by**: 03-SCALING.md (consolidated with capacity analysis)

- 📚 **LIMITS_ALERTS_MONITORING.md** - Original resource limits
  - **Superseded by**: 04-OBSERVABILITY.md (includes resource limits + alerts)

- 📚 **KUBERNETES_CONTROLLER_ANALYSIS.md** - Analysis of hexagonal architecture
  - **Decision**: Rejected hexagonal, using Standard Go Layout
  - **See**: ADR-0004

**Note**: Historical documents will be moved to `docs/phase-reports/` in Phase 4 cleanup.

---

## Maintenance Guidelines

### When to Update

- **Service added/removed**: Update 00-OVERVIEW.md service map
- **Architecture pattern changed**: Create new ADR, update relevant docs
- **Performance characteristics changed**: Update 03-SCALING.md capacity estimates
- **New metrics added**: Update 04-OBSERVABILITY.md metrics section
- **Security vulnerability found**: Update 05-SECURITY.md threat model

### How to Update

1. **Read the existing document** to understand current state
2. **Make focused changes** (don't rewrite entire sections unless necessary)
3. **Update "Last Updated" date** at top of document
4. **Test all code snippets** (especially kubectl commands, SQL queries)
5. **Update cross-references** if file names or sections changed
6. **Commit with descriptive message** (e.g., "docs: Update 03-SCALING.md with Phase 5 capacity planning")

### Documentation Standards

- **Line length**: Soft limit 120 characters (hard limit 150)
- **Code blocks**: Always specify language (```yaml, ```go, ```bash, etc.)
- **Headers**: Use sentence case ("Service scaling" not "Service Scaling")
- **Tables**: Align columns for readability
- **Commands**: Include expected output as comments
- **Metrics**: Use actual Prometheus query syntax (test in Grafana)
- **Status markers**: ✅ (complete), ⏳ (in progress), 🔴 (critical issue), ⚠️ (warning)

---

## Getting Help

**Can't find what you're looking for?**

1. **Check CLAUDE.md** at repository root - Navigation hub for all documentation
2. **Check service READMEs** in `services/*/README.md` - Service-specific details
3. **Check ADRs** in `docs/adr/` - Design decision rationale
4. **Check troubleshooting** in `docs/troubleshooting/` - Common issues and solutions
5. **Check quick references** in `docs/llm-guides/` - Task-oriented guides

**Still stuck?**

- Search codebase with `grep -r "keyword" docs/`
- Check git history: `git log --all --full-history docs/architecture/`
- Ask in team chat or create GitHub issue

---

## Summary

This directory contains **6 core architecture documents** totaling **~3,725 lines** covering:

- System architecture and service design
- Message flow and data integration
- Kubernetes deployment patterns
- Scalability and performance analysis
- Observability (metrics, logs, traces)
- Security architecture

**Reading time**: ~2 hours for complete understanding

**Last Updated**: 2026-01-28
