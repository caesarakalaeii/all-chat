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

**Understand WHY decisions were made**:
- [ADR-0001](./adr/0001-standard-go-layout.md) - Standard Go Layout (not hexagonal)
- [ADR-0002](./adr/0002-redis-streams-pubsub.md) - Redis Streams + Pub/Sub hybrid
- [ADR-0003](./adr/0003-cloudnative-postgres.md) - CloudNativePG operator
- [ADR-0004](./adr/0004-no-hexagonal-architecture.md) - No ports/adapters
- [ADR-0005](./adr/0005-react-nextjs-frontend.md) - React + Next.js
- [ADR-0006](./adr/0006-youtube-quota-tracking.md) - YouTube quota tracking

**→ ADR Index**: [adr/README.md](./adr/README.md)

---

### For Users & Streamers

**Overlay Customization**:
- [CSS Customization Guide](./user-guides/CSS_CUSTOMIZATION.md) - Complete CSS reference
- [Overlay Themes Gallery](./user-guides/overlay-themes/README.md) - Pre-built themes

---

### For Operators & DevOps

**Operations**:
- [DEPLOYMENT.md](./operations/DEPLOYMENT.md) - Self-hosting guide
- [PRODUCTION_DEPLOYMENT.md](./operations/PRODUCTION_DEPLOYMENT.md) - Production checklist
- [OBSERVABILITY_DEPLOYMENT_GUIDE.md](./operations/OBSERVABILITY_DEPLOYMENT_GUIDE.md) - Deploy LGTM stack

**Runbooks**:
- [scale-api-gateway.md](./operations/runbooks/scale-api-gateway.md) - Scale WebSocket capacity
- [recover-redis-outage.md](./operations/runbooks/recover-redis-outage.md) - Redis recovery
- [youtube-quota-recovery.md](./operations/runbooks/youtube-quota-recovery.md) - Quota exhaustion

---

### Development

- [TESTING_COMPREHENSIVE.md](./development/TESTING_COMPREHENSIVE.md) - Testing strategy
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

**Total Documentation**: ~10,000 lines across 50+ files

**Organization**:
- 📖 8 quick reference cards (task-oriented, <200 lines each)
- 📖 6 ADRs (design decisions with context)
- 📖 6 architecture docs (numbered 00-05, ~3,700 lines total)
- 📖 13 service READMEs (100% coverage)
- 📖 6 troubleshooting guides (diagnostic workflows)
- 📖 3 operational runbooks (incident response)

**For LLM Agents**: Documentation refactored for 75-86% reduction in reading for common tasks.

**Last Updated**: 2026-01-28
