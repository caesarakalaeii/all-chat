---
phase: 27
slug: auth-and-bot-token-foundation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-15
---

# Phase 27 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` package + `github.com/stretchr/testify` |
| **Config file** | none — `go test ./...` in each service module |
| **Quick run command** | `cd services/auth-service && go test ./oauth/... ./handlers/... -run TestDiscord -v` |
| **Full suite command** | `cd services/auth-service && go test ./... && cd ../discord-listener && go test ./...` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd services/auth-service && go test ./oauth/... ./handlers/... -run TestDiscord -count=1`
- **After every plan wave:** Run `cd services/auth-service && go test ./... && cd ../discord-listener && go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** ~30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 27-01-01 | 01 | 0 | AUTH-01 | unit | `cd services/auth-service && go test ./oauth/... -run TestDiscordOAuth_GetAuthURL -v` | ❌ W0 | ⬜ pending |
| 27-01-02 | 01 | 0 | AUTH-01 | unit | `cd services/auth-service && go test ./handlers/... -run TestHandleDiscordConnect -v` | ❌ W0 | ⬜ pending |
| 27-01-03 | 01 | 0 | AUTH-02 | unit | `cd services/auth-service && go test ./handlers/... -run TestHandleGetGuildChannels -v` | ❌ W0 | ⬜ pending |
| 27-01-04 | 01 | 0 | AUTH-03 | unit | `cd services/auth-service && go test ./handlers/... -run TestCheckBotPermissions -v` | ❌ W0 | ⬜ pending |
| 27-01-05 | 01 | 0 | AUTH-03 | unit | `cd services/auth-service && go test ./handlers/... -run TestHandleDiscordConnect_MissingPerms -v` | ❌ W0 | ⬜ pending |
| 27-01-06 | 01 | 0 | AUTH-04 | unit | `cd services/auth-service && go test ./handlers/... -run TestHandleDiscordDisconnect_APIFailure -v` | ❌ W0 | ⬜ pending |
| 27-02-01 | 02 | 0 | AUTH-01 | unit | `cd services/discord-listener && go test ./gateway/... -run TestGatewayClient -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `services/auth-service/oauth/discord_test.go` — stubs for AUTH-01 (GetAuthURL, ExchangeCode)
- [ ] `services/auth-service/handlers/discord_test.go` — stubs for AUTH-01 through AUTH-04
- [ ] `services/discord-listener/gateway/client_test.go` — stubs for Gateway IDENTIFY payload and READY handler
- [ ] `services/discord-listener/go.mod` — new module skeleton (discord-listener does not exist yet)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Bot appears in Discord server after OAuth flow | AUTH-01 | Requires live Discord OAuth round-trip and real server | Click "Add to Server" in UI, complete OAuth, verify bot visible in Discord server member list |
| MESSAGE_CONTENT intent is non-empty on READY | AUTH-01 | Requires live Gateway connection with real bot token | Start discord-listener, check startup logs for READY event; assert MESSAGE_CONTENT guild count > 0 |
| Human-readable error shown for missing permissions | AUTH-03 | UI rendering of permission error text | Simulate missing VIEW_CHANNEL, verify error message names the missing permission clearly |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
