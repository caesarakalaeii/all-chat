---
phase: 28
slug: inbound-listener-core
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-15
---

# Phase 28 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | testify v1.11.1 + go test |
| **Config file** | none (standard go test) |
| **Quick run command** | `cd services/discord-listener && go test ./gateway/... -v -run TestMessageCreate` |
| **Full suite command** | `cd services/discord-listener && go test ./... && cd ../message-processor && go test ./normalizer/... -run TestDiscord` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./gateway/... ./publisher/...` (discord-listener) or `go test ./normalizer/... -run TestDiscord` (message-processor)
- **After every plan wave:** Run `cd services/discord-listener && go test ./... && cd ../message-processor && go test ./normalizer/... -run TestDiscord`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 28-01-01 | 01 | 0 | INBD-01 | unit | `go test ./gateway/... -run TestHandleMessageCreate_BotFiltered` | ❌ W0 | ⬜ pending |
| 28-01-02 | 01 | 0 | INBD-01 | unit | `go test ./publisher/... -run TestPublish_HappyPath` | ❌ W0 | ⬜ pending |
| 28-01-03 | 01 | 0 | INBD-02 | unit | `go test ./normalizer/... -run TestDiscordNormalizer` | ❌ W0 | ⬜ pending |
| 28-01-04 | 01 | 1 | INBD-01 | unit | `go test ./gateway/... -run TestHandleMessageCreate_BotFiltered` | ❌ W0 | ⬜ pending |
| 28-01-05 | 01 | 1 | INBD-01 | unit | `go test ./gateway/... -run TestHandleMessageCreate_UnknownChannel` | ❌ W0 | ⬜ pending |
| 28-01-06 | 01 | 1 | INBD-01 | unit | `go test ./gateway/... -run TestHandleMessageCreate_EmptyContent` | ❌ W0 | ⬜ pending |
| 28-02-01 | 02 | 1 | INBD-02 | unit | `go test ./normalizer/... -run TestDiscordNormalizer` | ❌ W0 | ⬜ pending |
| 28-02-02 | 02 | 1 | INBD-02 | unit | `go test ./normalizer/... -run TestDiscordNormalizer_NickFallback` | ❌ W0 | ⬜ pending |
| 28-02-03 | 02 | 1 | INBD-02 | unit | `go test ./normalizer/... -run TestDiscordNormalizer_BlackColor` | ❌ W0 | ⬜ pending |
| 28-02-04 | 02 | 1 | INBD-02 | unit | `go test ./normalizer/... -run TestDiscordNormalizer_WrongPlatform` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `services/discord-listener/gateway/message_create_test.go` — stubs for INBD-01 filtering + halt behavior
- [ ] `services/discord-listener/publisher/stream_publisher_test.go` — stubs for publish happy path
- [ ] `services/message-processor/normalizer/discord_normalizer_test.go` — stubs for INBD-02 normalization

*All existing test infrastructure (testify, go test) is in place; only new test files needed for new code.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Discord message appears in overlay within 1 second | INBD-01 | Requires live Discord Gateway connection and running overlay | Send message in configured channel, verify overlay receives it within 1s |
| Channel registry invalidation triggers reload | INBD-01 | Requires live Redis Pub/Sub and overlay-manager coordination | Create/delete Discord source in overlay-manager, verify discord-listener picks up change |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
