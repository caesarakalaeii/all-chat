# All-Chat Documentation

Welcome to the All-Chat documentation hub. This directory contains comprehensive guides organized for maximum efficiency.

---

## 📚 Quick Navigation

### For LLM Agents & Developers

**Start Here**:
- [CLAUDE.md](../CLAUDE.md) - Project overview, navigation hub
- [llm-guides/NAVIGATION.md](./llm-guides/NAVIGATION.md) - Service-by-service navigation

**Task-Oriented Quick References** (~100-200 lines each):
- [QUICK-REF-ADD-PLATFORM.md](./llm-guides/QUICK-REF-ADD-PLATFORM.md) - Add new streaming platform
- [QUICK-REF-DEBUG-QUOTA.md](./llm-guides/QUICK-REF-DEBUG-QUOTA.md) - YouTube quota debugging
- [QUICK-REF-ADD-ENDPOINT.md](./llm-guides/QUICK-REF-ADD-ENDPOINT.md) - Add HTTP endpoint
- [QUICK-REF-SCALING.md](./llm-guides/QUICK-REF-SCALING.md) - Scale services
- [More quick refs...](./llm-guides/)

**Troubleshooting**:
- [decision-tree.md](./troubleshooting/decision-tree.md) - Start here for diagnosis
- [Troubleshooting guides](./troubleshooting/) - Specific issue guides

---

### Architecture Documentation

**Read in order** (numbered 00-05, ~2 hours total):
1. [00-OVERVIEW.md](./architecture/00-OVERVIEW.md) - System overview, service map
2. [01-DATA-FLOW.md](./architecture/01-DATA-FLOW.md) - Message processing pipeline
3. [02-DEPLOYMENT.md](./architecture/02-DEPLOYMENT.md) - Kubernetes deployment
4. [03-SCALING.md](./architecture/03-SCALING.md) - Performance and scaling
5. [04-OBSERVABILITY.md](./architecture/04-OBSERVABILITY.md) - Metrics, logs, traces
6. [05-SECURITY.md](./architecture/05-SECURITY.md) - Security architecture

**→ Architecture Index**: [architecture/README.md](./architecture/README.md)

---

### Architecture Decision Records (ADRs)

**Understand WHY decisions were made** (12 ADRs total — see index):
- [ADR-0001](./adr/0001-standard-go-layout.md) - Standard Go Layout (not hexagonal)
- [ADR-0002](./adr/0002-redis-streams-pubsub.md) - Redis Streams + Pub/Sub hybrid
- [ADR-0003](./adr/0003-cloudnative-postgres.md) - CloudNativePG operator
- [ADR-0004](./adr/0004-no-hexagonal-architecture.md) - No ports/adapters
- [ADR-0005](./adr/0005-react-nextjs-frontend.md) - React + Next.js
- [ADR-0006](./adr/0006-youtube-quota-tracking.md) - YouTube quota tracking
- ADR-0007 to ADR-0012 — see index

**→ ADR Index**: [adr/README.md](./adr/README.md)

---

### For Users & Streamers

**Overlay Customization**:
- [CSS Customization Guide](./CSS_CUSTOMIZATION.md) - Complete CSS reference
- [Overlay Themes Gallery](./overlay-themes/README.md) - Pre-built themes

---

### For Operators & DevOps

**Operations**:
- [DEPLOYMENT.md](./DEPLOYMENT.md) - Self-hosting guide
- [PRODUCTION_DEPLOYMENT.md](./PRODUCTION_DEPLOYMENT.md) - Production checklist
- [OBSERVABILITY_DEPLOYMENT_GUIDE.md](./OBSERVABILITY_DEPLOYMENT_GUIDE.md) - Deploy LGTM stack

**Runbooks**:
- [scale-api-gateway.md](./operations/runbooks/scale-api-gateway.md) - Scale WebSocket capacity
- [recover-redis-outage.md](./operations/runbooks/recover-redis-outage.md) - Redis recovery
- [youtube-quota-recovery.md](./operations/runbooks/youtube-quota-recovery.md) - Quota exhaustion

---

### Development

- [TESTING_COMPREHENSIVE.md](./TESTING_COMPREHENSIVE.md) - Testing strategy
- [CONTRIBUTING.md](../CONTRIBUTING.md) - Contribution guidelines

---

## Historical Documents

**Phase Reports & Archived Docs**: [phase-reports/](./phase-reports/)

These are superseded by current documentation but preserved for historical reference.

---

## Documentation Organization Principles

1. **Task-oriented quick references** (<200 lines) for common tasks
2. **Comprehensive architecture docs** (numbered 00-05) for deep dives
3. **ADRs** to explain WHY decisions were made
4. **Service READMEs** for service-specific details
5. **Troubleshooting decision tree** for structured diagnosis

**Result**: Most tasks require <200 lines of reading (vs 1,000+ previously).

---

## Summary

**Organization** (current counts as of 2026-05-27):
- 📖 9 quick reference cards in `llm-guides/` (task-oriented, <200 lines each)
- 📖 12 ADRs (design decisions with context)
- 📖 6 architecture docs (numbered 00-05, ~3,700 lines total)
- 📖 ~14 service READMEs (most services have one; `share-service` and `discord-listener` don't)
- 📖 6 troubleshooting guides + decision tree
- 📖 6 operational runbooks (4 in `operations/runbooks/`, 2 in `runbooks/`)

**For LLM Agents**: Documentation refactored for 75-86% reduction in reading for common tasks.

**Last Updated**: 2026-05-27
