---
phase: 29
slug: inbound-enrichment
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-16
---

# Phase 29 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testify (github.com/stretchr/testify) |
| **Config file** | none — standard `go test ./...` |
| **Quick run command** | `cd services/discord-listener && go test ./gateway/... ./guild/... -v -count=1` |
| **Full suite command** | `cd services/discord-listener && go test ./... -v -count=1` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd services/discord-listener && go test ./gateway/... ./guild/... -v -count=1`
- **After every plan wave:** Run `cd services/discord-listener && go test ./... -v -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** ~10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 29-01-01 | 01 | 0 | INBD-03 | unit stub | `go test ./gateway/... -run TestHandleMessageDelete` | ❌ Wave 0 | ⬜ pending |
| 29-01-02 | 01 | 0 | INBD-03 | unit stub | `go test ./gateway/... -run TestHandleMessageDeleteBulk` | ❌ Wave 0 | ⬜ pending |
| 29-01-03 | 01 | 0 | INBD-04 | unit stub | `go test ./gateway/... -run TestResolveMentions` | ❌ Wave 0 | ⬜ pending |
| 29-01-04 | 01 | 0 | INBD-04 | unit stub | `go test ./guild/... -run TestRedisGuildCache` | ❌ Wave 0 | ⬜ pending |
| 29-01-05 | 01 | 1 | INBD-03 | unit | `go test ./gateway/... -run TestHandleMessageDelete_HappyPath` | ❌ Wave 0 | ⬜ pending |
| 29-01-06 | 01 | 1 | INBD-03 | unit | `go test ./gateway/... -run TestHandleMessageDelete_UnknownChannel` | ❌ Wave 0 | ⬜ pending |
| 29-01-07 | 01 | 1 | INBD-03 | unit | `go test ./gateway/... -run TestHandleMessageDelete_EventShape` | ❌ Wave 0 | ⬜ pending |
| 29-01-08 | 01 | 1 | INBD-03 | unit | `go test ./gateway/... -run TestHandleMessageDeleteBulk_Expansion` | ❌ Wave 0 | ⬜ pending |
| 29-02-01 | 02 | 1 | INBD-04 | unit | `go test ./gateway/... -run TestResolveMentions_User` | ❌ Wave 0 | ⬜ pending |
| 29-02-02 | 02 | 1 | INBD-04 | unit | `go test ./gateway/... -run TestResolveMentions_UserNickVariant` | ❌ Wave 0 | ⬜ pending |
| 29-02-03 | 02 | 1 | INBD-04 | unit | `go test ./gateway/... -run TestResolveMentions_UserFallback` | ❌ Wave 0 | ⬜ pending |
| 29-02-04 | 02 | 1 | INBD-04 | unit | `go test ./gateway/... -run TestResolveMentions_Channel` | ❌ Wave 0 | ⬜ pending |
| 29-02-05 | 02 | 1 | INBD-04 | unit | `go test ./gateway/... -run TestResolveMentions_Role` | ❌ Wave 0 | ⬜ pending |
| 29-02-06 | 02 | 1 | INBD-04 | unit | `go test ./gateway/... -run TestHandleGuildCreate_CachesPopulated` | ❌ Wave 0 | ⬜ pending |
| 29-02-07 | 02 | 1 | INBD-04 | unit | `go test ./gateway/... -run TestHandleChannelDelete_CacheCleared` | ❌ Wave 0 | ⬜ pending |
| 29-02-08 | 02 | 1 | INBD-04 | unit | `go test ./guild/... -run TestRedisGuildCache` | ❌ Wave 0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `services/discord-listener/gateway/message_delete_test.go` — stubs for INBD-03 deletion events
- [ ] `services/discord-listener/gateway/mentions_test.go` — stubs for INBD-04 mention resolution
- [ ] `services/discord-listener/gateway/guild_create_test.go` — stubs for INBD-04 GUILD_CREATE handler
- [ ] `services/discord-listener/guild/cache_test.go` — stubs for GuildCache interface + Redis implementation

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Deletion event disappears from active overlay | INBD-03 | Requires live Discord webhook + running overlay UI | Send message in configured channel, delete it, verify it disappears in overlay within 2s |
| Mention renders as `@username` in overlay | INBD-04 | Requires live Discord bot + running overlay UI | Send message with @mention, verify overlay shows `@alice` not `<@123...>` |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
