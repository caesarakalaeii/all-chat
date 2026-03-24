---
phase: 01-discord-support-bot
plan: 03
subsystem: infra
tags: [docker, kubernetes, node, typescript, discord, claude-code]

# Dependency graph
requires:
  - phase: 01-02
    provides: support-bot TypeScript source code with agent.ts, commands, and discord.js integration
provides:
  - Dockerfile for support-bot using node:20-alpine with globally installed claude CLI
  - Kubernetes Deployment manifest with init containers for repo cloning and secret mounts
  - kustomization.yaml for support-bot k8s resources
affects: [deployment, support-bot, kubernetes]

# Tech tracking
tech-stack:
  added: [node:20-alpine, @anthropic-ai/claude-code (global), alpine/git (init container)]
  patterns: [init-container repo cloning, emptyDir volume for ephemeral repo data, readOnly volume mounts]

key-files:
  created:
    - services/support-bot/Dockerfile
    - deployments/k8s/base/support-bot/deployment.yaml
    - deployments/k8s/base/support-bot/kustomization.yaml
  modified: []

key-decisions:
  - "npm ci (not --production) in Dockerfile — tsx is a devDependency but required at runtime for TypeScript execution"
  - "Init containers use $GITHUB_TOKEN shell variable syntax inside sh -c — NOT $(GITHUB_TOKEN) which is Kubernetes command substitution"
  - "Volume mounts for repo dirs are readOnly: true — bot only needs read access to answer questions"
  - "emptyDir volumes used for cloned repos — ephemeral, populated fresh on each pod start"

patterns-established:
  - "Init container pattern: clone repos into emptyDir volumes, main container reads them as readOnly"
  - "Secrets from allchat-secrets via secretKeyRef for all sensitive values"

requirements-completed: [BOT-07]

# Metrics
duration: 3min
completed: 2026-03-24
---

# Phase 01 Plan 03: Dockerfile and Kubernetes Manifests Summary

**node:20-alpine Dockerfile with globally installed claude CLI, Kubernetes Deployment with init-container repo cloning into readOnly volumes, wired to allchat-secrets**

## Performance

- **Duration:** ~3 min
- **Started:** 2026-03-24T12:29:59Z
- **Completed:** 2026-03-24T12:33:00Z (Task 1 committed; Task 2 awaits human verification)
- **Tasks:** 1/2 automated complete (Task 2 is checkpoint:human-verify)
- **Files modified:** 3

## Accomplishments

- Dockerfile builds successfully on node:20-alpine with claude CLI installed globally via `npm install -g @anthropic-ai/claude-code`
- Kubernetes Deployment has init containers cloning both `all-chat` and `all-chat-extension` repos using SUPPORT_BOT_GITHUB_TOKEN from allchat-secrets
- Volume mounts are readOnly: true for security, using emptyDir for ephemeral clone storage
- All three secrets (SUPPORT_BOT_DISCORD_TOKEN, CLAUDE_CODE_OAUTH_TOKEN, SUPPORT_BOT_GITHUB_TOKEN) wired from allchat-secrets

## Task Commits

Each task was committed atomically:

1. **Task 1: Dockerfile and Kubernetes manifests** - `0336530` (feat)
2. **Task 2: Verify bot works in Discord** - AWAITING HUMAN VERIFICATION

## Files Created/Modified

- `services/support-bot/Dockerfile` - Multi-stage node:20-alpine build with globally installed claude CLI, tsx for runtime TypeScript, USER node for security
- `deployments/k8s/base/support-bot/deployment.yaml` - Kubernetes Deployment with init containers, secrets, readOnly volume mounts, resource limits
- `deployments/k8s/base/support-bot/kustomization.yaml` - Kustomize resource listing

## Decisions Made

- Used `npm ci` (not `--production`) because `tsx` is in devDependencies but required at runtime for `node --import tsx` start command
- Init container commands use `$GITHUB_TOKEN` shell variable syntax inside `sh -c` strings — Kubernetes `$(VAR)` command substitution syntax only works in direct command arrays, not shell strings
- Volume mounts for cloned repos are `readOnly: true` — the bot only reads code to answer questions
- `emptyDir` volumes for cloned repos — repos are cloned fresh on each pod start, no persistent storage needed

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

Before the bot can run in Kubernetes, the following secrets must be added to the `allchat-secrets` Kubernetes secret:

| Secret Key | Source |
|---|---|
| `SUPPORT_BOT_DISCORD_TOKEN` | discord.com/developers/applications -> Bot -> Token |
| `CLAUDE_CODE_OAUTH_TOKEN` | Run `claude setup-token` locally to generate a ~1-year token |
| `SUPPORT_BOT_GITHUB_TOKEN` | github.com -> Settings -> Developer settings -> Fine-grained tokens (Issues: Write on all-chat and all-chat-extension) |

Also required:
- Enable "Message Content Intent" in Discord Developer Portal: discord.com/developers/applications -> Bot -> Privileged Gateway Intents
- Run `npm run deploy-commands` once to register slash commands (requires DISCORD_CLIENT_ID and DISCORD_GUILD_ID)

## Next Phase Readiness

- Dockerfile and Kubernetes manifests ready for deployment once secrets are populated
- Bot must be verified in a real Discord server before being considered production-ready (Task 2 checkpoint)
- After human verification: plan 03 fully complete

## Self-Check: PASSED

- `services/support-bot/Dockerfile` - FOUND
- `deployments/k8s/base/support-bot/deployment.yaml` - FOUND
- `deployments/k8s/base/support-bot/kustomization.yaml` - FOUND
- Docker build: exited 0
- kubectl dry-run: `deployment.apps/support-bot created (dry run)`
- Commit `0336530` exists

---
*Phase: 01-discord-support-bot*
*Completed: 2026-03-24 (awaiting human verification for Task 2)*
