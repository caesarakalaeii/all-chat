# Architecture Documentation

Architecture documentation for All-Chat, organized by topic with numbered prefixes for recommended reading order.

---

## Reading Order (For New Team Members)

**Start here** if you're new to the codebase:

1. **[00-OVERVIEW.md](./00-OVERVIEW.md)**
   - Full-system architecture diagram (every deployed component and the Redis paths between them)
   - Service map with production ports and responsibilities
   - Technology stack
   - Key design decisions

2. **[01-DATA-FLOW.md](./01-DATA-FLOW.md)**
   - End-to-end message flow
   - Redis Streams + Pub/Sub patterns
   - Unified message format
   - Platform-specific normalization
   - Alerts, engagement and moderation flows

3. **[02-DEPLOYMENT.md](./02-DEPLOYMENT.md)**
   - Production on K3s via ArgoCD GitOps, local dev via docker-compose
   - CloudNativePG PostgreSQL cluster and backups
   - Redis HA, MinIO
   - Release flow (`beta` → `main`)

4. **[03-SCALING.md](./03-SCALING.md)**
   - Scalability analysis and capacity estimates
   - Service-specific scaling strategies
   - Infrastructure scaling (PostgreSQL, Redis)
   - Performance bottlenecks

5. **[04-OBSERVABILITY.md](./04-OBSERVABILITY.md)**
   - LGTM stack (Loki, Grafana, Prometheus, Tempo)
   - Metrics, structured logging, tracing
   - Alerting rules and dashboards

6. **[05-SECURITY.md](./05-SECURITY.md)**
   - Security architecture
   - Threat model
   - OAuth flows and tokens
   - Known residual risks

---

## Quick Navigation

### By Topic

| Topic | Document |
|-------|----------|
| **System overview, full diagram** | [00-OVERVIEW.md](./00-OVERVIEW.md) |
| **Message processing pipeline** | [01-DATA-FLOW.md](./01-DATA-FLOW.md) |
| **Kubernetes deployment** | [02-DEPLOYMENT.md](./02-DEPLOYMENT.md) |
| **Performance & scaling** | [03-SCALING.md](./03-SCALING.md) |
| **Metrics & monitoring** | [04-OBSERVABILITY.md](./04-OBSERVABILITY.md) |
| **Security & OAuth** | [05-SECURITY.md](./05-SECURITY.md) |

### By Service

The [service map in 00-OVERVIEW.md](./00-OVERVIEW.md#service-map) lists every service with a link to its README; the authoritative list is the `services/` directory.

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

### Historical documents

The original Phase 1 plans (`APPROVED_ARCHITECTURE.md`, `OBSERVABILITY_MONITORING.md`, `SCALING_PERFORMANCE.md`, `LIMITS_ALERTS_MONITORING.md`, `KUBERNETES_CONTROLLER_ANALYSIS.md`) live in [`docs/phase-reports/`](../phase-reports/README.md). They describe what was planned, not what runs; the documents in this directory supersede them.

---

## Maintenance Guidelines

### When to Update

- **Service added/removed**: Update the 00-OVERVIEW.md diagram and service map
- **Redis stream/channel added or renamed**: Update the 00-OVERVIEW.md diagram and 01-DATA-FLOW.md
- **Architecture pattern changed**: Create new ADR, update relevant docs
- **Deployment manifests changed** (in the GitOps repo): Update 02-DEPLOYMENT.md
- **Performance characteristics changed**: Update 03-SCALING.md capacity estimates
- **New metrics or alerts added**: Update 04-OBSERVABILITY.md
- **Security control or residual risk changed**: Update 05-SECURITY.md

### How to Update

1. **Read the existing document** to understand current state
2. **Make focused changes** (don't rewrite entire sections unless necessary)
3. **Update "Last Updated" date** at top of document
4. **Test all code snippets** (especially kubectl commands, SQL queries)
5. **Update cross-references** if file names or sections changed
6. **Check the diagram still renders** (any Mermaid renderer, e.g. `mermaid-cli`)
7. **Commit with descriptive message** (e.g., "docs: update 03-SCALING.md capacity estimates")

### Documentation Standards

- **Line length**: Soft limit 120 characters (hard limit 150)
- **Code blocks**: Always specify language (```yaml, ```go, ```bash, etc.)
- **Diagrams**: Mermaid, so they render on GitHub and stay diffable
- **Headers**: Use sentence case ("Service scaling" not "Service Scaling")
- **Commands**: Include expected output as comments
- **Metrics**: Use actual Prometheus query syntax (test in Grafana)
- **No volatile counts**: point at the directory instead of "N services"
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

- Search the docs with `git grep "keyword" docs/`
- Check git history: `git log --all --full-history docs/architecture/`
- Ask in team chat or create GitHub issue

---

**Last Updated**: 2026-11-04
