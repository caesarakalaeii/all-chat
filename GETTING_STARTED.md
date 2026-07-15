# Getting Started with All-Chat

**This file has been reorganized for better LLM efficiency.**

---

## Quick Navigation

**For LLM agents and developers**, see the comprehensive navigation guide:

→ **[docs/llm-guides/NAVIGATION.md](./docs/llm-guides/NAVIGATION.md)** (~400 lines)

This guide includes:
- Service-by-service navigation (every service in `services/`)
- Common tasks and where to find relevant code
- Quick reference card links
- Troubleshooting guides
- Database schema overview
- Useful commands

---

## Essential Starting Points

1. **[CLAUDE.md](./CLAUDE.md)** - Project overview, architecture, tech stack
2. **[docs/llm-guides/NAVIGATION.md](./docs/llm-guides/NAVIGATION.md)** - Detailed service navigation
3. **[docs/architecture/00-OVERVIEW.md](./docs/architecture/00-OVERVIEW.md)** - System architecture

**For specific tasks**, see quick reference guides:
- [QUICK-REF-ADD-PLATFORM.md](./docs/llm-guides/QUICK-REF-ADD-PLATFORM.md) - Add new platform support
- [QUICK-REF-DEBUG-QUOTA.md](./docs/llm-guides/QUICK-REF-DEBUG-QUOTA.md) - YouTube quota debugging
- [More quick refs...](./docs/llm-guides/)

**For troubleshooting**, start with:
- [Troubleshooting Decision Tree](./docs/troubleshooting/decision-tree.md) - High-level triage

---

## Quick Start (Development)

```bash
# Start local environment
make docker-up         # Postgres, Redis, all services

# Run tests
make test

# Apply database migrations
make migrate

# Access services
# - API Gateway: http://localhost:8080
# - Overlays: http://localhost:3000 (frontend)
```

---

## Documentation Organization

All documentation has been reorganized into focused, LLM-optimized guides:

```
docs/
├── llm-guides/              # Task-oriented quick references (quick-reference guides + NAVIGATION)
├── adr/                     # Architecture Decision Records
├── architecture/            # System architecture (6 docs, numbered 00-05)
├── troubleshooting/         # Diagnostic guides (decision tree + 5 guides)
├── operations/              # Deployment and runbooks
└── phase-reports/           # Historical documents (archived)
```

**Reading time**: Most tasks require <200 lines of reading (vs 1,000+ previously).

---

## Need Help?

1. **First time?** → Read [CLAUDE.md](./CLAUDE.md)
2. **Specific task?** → Check [quick reference cards](./docs/llm-guides/)
3. **Architecture questions?** → Read [architecture docs](./docs/architecture/)
4. **Troubleshooting?** → Use [decision tree](./docs/troubleshooting/decision-tree.md)
5. **Why was X chosen?** → Check [ADRs](./docs/adr/)

**Complete navigation**: [docs/llm-guides/NAVIGATION.md](./docs/llm-guides/NAVIGATION.md)
