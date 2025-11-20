# All-Chat Documentation

Welcome to the All-Chat documentation hub. This directory contains comprehensive guides for users, developers, and operators.

---

## 📚 Documentation Index

### For Users & Streamers

| Document | Description | Audience |
|----------|-------------|----------|
| [**CSS Customization Guide**](./CSS_CUSTOMIZATION.md) | Complete CSS reference for customizing overlay appearance | Streamers, Theme Creators |
| [Overlay Themes Gallery](./overlay-themes/README.md) | Pre-built themes and theme creation guide | Streamers, Designers |
| [Quick Start Guide](./overlay-themes/QUICK-START.md) | Apply themes to your overlay in minutes | Streamers |

### For Developers

| Document | Description | Audience |
|----------|-------------|----------|
| [**Developer Guide (CLAUDE.md)**](../CLAUDE.md) | Architecture, tech stack, development principles | Backend Developers |
| [**Getting Started Guide**](../GETTING_STARTED.md) | Navigate the codebase efficiently | All Developers |
| [Testing Guide](./TESTING_COMPREHENSIVE.md) | Test strategy, coverage, integration tests | QA, Developers |

### For Operators & DevOps

| Document | Description | Audience |
|----------|-------------|----------|
| [**Deployment Guide**](./DEPLOYMENT.md) | Self-hosting with Docker Compose or Kubernetes | DevOps, SRE |
| [Production Deployment](./PRODUCTION_DEPLOYMENT.md) | Production-ready deployment checklist | DevOps, SRE |
| [Critical Architecture Analysis](./CRITICAL_ARCHITECTURE_ANALYSIS.md) | Known issues, security gaps, technical debt | Architects, DevOps |

### Architecture Documentation

Located in `architecture/` subdirectory:

| Document | Description |
|----------|-------------|
| [Data Flow Integration](./architecture/DATA_FLOW_INTEGRATION.md) | Message flow, Redis Streams + Pub/Sub |
| [Deployment Kubernetes](./architecture/DEPLOYMENT_KUBERNETES.md) | K8s manifests, HPA, resource limits |
| [Scaling Performance](./architecture/SCALING_PERFORMANCE.md) | Scalability analysis, bottlenecks |
| [Observability Monitoring](./architecture/OBSERVABILITY_MONITORING.md) | Health checks, metrics, logging |
| [Security Architecture](./architecture/SECURITY_ARCHITECTURE.md) | Auth, secrets, RBAC, NetworkPolicies |

### Phase Completion Reports

Historical documentation of project milestones:

| Document | Phase | Status |
|----------|-------|--------|
| [Phase 2 Complete](./PHASE_2_COMPLETE.md) | Multi-source support | ✅ Complete |
| [Phase 3 Complete](./PHASE_3_COMPLETE.md) | YouTube integration | ✅ Complete |
| [Phase 4 Summary](./PHASE_4_SUMMARY.md) | All core services | ✅ Complete |
| [Phase 4 Implementation Complete](./PHASE_4_IMPLEMENTATION_COMPLETE.md) | Detailed Phase 4 report | ✅ Complete |
| [Phase 5 Frontend Complete](./PHASE_5_FRONTEND_COMPLETE.md) | React + Next.js frontend | ✅ Complete |

---

## 🎯 Quick Links by Task

### I want to...

**Customize my overlay appearance**
→ [CSS Customization Guide](./CSS_CUSTOMIZATION.md) - Complete CSS reference
→ [Overlay Themes](./overlay-themes/README.md) - Pre-built themes

**Deploy All-Chat**
→ [Deployment Guide](./DEPLOYMENT.md) - Docker Compose or Kubernetes
→ [Production Deployment](./PRODUCTION_DEPLOYMENT.md) - Production checklist

**Develop new features**
→ [Developer Guide (CLAUDE.md)](../CLAUDE.md) - Architecture and principles
→ [Getting Started Guide](../GETTING_STARTED.md) - Navigate the codebase
→ [Testing Guide](./TESTING_COMPREHENSIVE.md) - Write tests

**Understand the architecture**
→ [Data Flow Integration](./architecture/DATA_FLOW_INTEGRATION.md) - Message flow
→ [Critical Architecture Analysis](./CRITICAL_ARCHITECTURE_ANALYSIS.md) - Known issues

**Add a new platform (Kick, TikTok, etc.)**
→ [Getting Started Guide](../GETTING_STARTED.md) - Section: "Add Support for a New Platform"
→ [Developer Guide (CLAUDE.md)](../CLAUDE.md) - Service patterns

**Debug issues**
→ [Testing Guide](./TESTING_COMPREHENSIVE.md) - Test coverage and patterns
→ [Troubleshooting sections in README.md](../README.md) - Common issues
→ [CSS Troubleshooting](./CSS_CUSTOMIZATION.md#troubleshooting) - CSS-specific issues

---

## 📖 External Resources

### Platform APIs
- [Twitch IRC Documentation](https://dev.twitch.tv/docs/irc)
- [YouTube Live Chat API](https://developers.google.com/youtube/v3/live/docs)
- [7TV API](https://7tv.io/docs)
- [BTTV API](https://betterttv.com/developers)
- [FFZ API](https://www.frankerfacez.com/developers)

### Technologies Used
- [Go Documentation](https://go.dev/doc/)
- [Next.js Documentation](https://nextjs.org/docs)
- [React Documentation](https://react.dev)
- [PostgreSQL Documentation](https://www.postgresql.org/docs/)
- [Redis Documentation](https://redis.io/docs/)
- [Kubernetes Documentation](https://kubernetes.io/docs/)

---

## 🤝 Contributing Documentation

Found a mistake or want to improve the docs?

1. **For minor fixes** (typos, clarity):
   - Edit the file directly and submit a PR

2. **For new documentation**:
   - Create the document in the appropriate directory
   - Update this README.md index
   - Submit a PR with description

3. **For CSS themes**:
   - Add your theme to `overlay-themes/`
   - Update `overlay-themes/README.md`
   - Include screenshots if possible

---

## 📮 Support

- **🐛 Bug Reports**: [GitHub Issues](https://github.com/caesarakalaeii/all-chat/issues)
- **💬 Questions**: [GitHub Discussions](https://github.com/caesarakalaeii/all-chat/discussions)
- **📧 Email**: support@allch.at

---

**Last Updated**: 2025-11-20
