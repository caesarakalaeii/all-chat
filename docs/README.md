# All-Chat Documentation

Start with [CLAUDE.md](../CLAUDE.md) for the project overview and service list, and [GETTING_STARTED.md](./GETTING_STARTED.md) to run it locally. Service-specific details live in each `services/*/README.md`.

Documentation conventions (no point-in-time docs, update docs in the same change as code) are in CLAUDE.md under "Documentation Conventions".

## Architecture

- [00-OVERVIEW.md](./architecture/00-OVERVIEW.md): services and how they fit together
- [01-DATA-FLOW.md](./architecture/01-DATA-FLOW.md): listeners, Redis streams and channels, processors, gateway
- [02-DEPLOYMENT.md](./architecture/02-DEPLOYMENT.md): Kubernetes, CI images, Keel, migrations
- [03-SCALING.md](./architecture/03-SCALING.md): HPAs, bottlenecks, tuning knobs
- [04-OBSERVABILITY.md](./architecture/04-OBSERVABILITY.md): metrics, logs, tracing, alerts
- [05-SECURITY.md](./architecture/05-SECURITY.md): auth, encryption, network, data protection
- [adr/](./adr/README.md): Architecture Decision Records, the why behind each design

## Developing

- [TESTING.md](./TESTING.md): test targets and CI workflows
- [development/service-template.md](./development/service-template.md): skeleton for a new Go service
- [llm-guides/](./llm-guides/): quick references (add a platform, add an endpoint, migrations, quota, Redis, Kubernetes, scaling, security audit)
- [frontend/FRONTEND_QUICK_START.md](./frontend/FRONTEND_QUICK_START.md): frontend with a minimal backend
- [frontend/I18N.md](./frontend/I18N.md): UI string catalog
- [frontend/AGENT_UI_VERIFICATION.md](./frontend/AGENT_UI_VERIFICATION.md): screenshot and pixel-baseline gate for UI changes
- [ACCESSIBILITY.md](./ACCESSIBILITY.md): WCAG scope and CI gates
- [ADMIN.md](./ADMIN.md): admin roles and endpoints

## Operating

- [troubleshooting/decision-tree.md](./troubleshooting/decision-tree.md): triage, links to the specific guides
- [runbooks/](./runbooks/): Redis outage, YouTube quota, gateway scaling, service fixes, DB password and secret rotation, platform listener rollout
- [testing/chaos-testing-phase5.md](./testing/chaos-testing-phase5.md): chaos scenarios
- [security/](./security/): audit report, accepted residuals, cookie auth design
- [guides/GOOGLE_OAUTH_VERIFICATION.md](./guides/GOOGLE_OAUTH_VERIFICATION.md): Google OAuth app verification checklist

## For streamers

- [CSS_CUSTOMIZATION.md](./CSS_CUSTOMIZATION.md): overlay CSS reference
- [overlay-themes/](./overlay-themes/README.md) and [credit-roll-themes/](./credit-roll-themes/README.md): ready-made themes
- [features/](./features/): message deletion, Twitch moderation events, Twitch ad breaks, passive IRL overlay
- [guides/streamdeck.md](./guides/streamdeck.md): Stream Deck and desktop control surfaces
