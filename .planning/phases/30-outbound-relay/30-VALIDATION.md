---
phase: 30
slug: outbound-relay
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-16
---

# Phase 30 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing stdlib + testify v1.11.1 |
| **Config file** | none — `go test ./...` from service root |
| **Quick run command** | `cd services/discord-listener && go test ./relay/... -v` |
| **Full suite command** | `cd services/discord-listener && go test ./... -v` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd services/discord-listener && go test ./relay/... -v`
- **After every plan wave:** Run `cd services/discord-listener && go test ./... -v`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** ~5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 30-01-01 | 01 | 0 | RELY-01 | unit | `go test ./relay/... -run TestRelayManager_DiscordPlatformFiltered` | ❌ W0 | ⬜ pending |
| 30-01-02 | 01 | 0 | RELY-01 | unit | `go test ./relay/... -run TestRelayManager_NonDiscordRelayed` | ❌ W0 | ⬜ pending |
| 30-01-03 | 01 | 0 | RELY-02 | unit | `go test ./relay/... -run TestRepository_OnlyRelayEnabledReturned` | ❌ W0 | ⬜ pending |
| 30-01-04 | 01 | 0 | RELY-03 | unit | `go test ./relay/... -run TestHTTPPoster_UsesCorrectChannelID` | ❌ W0 | ⬜ pending |
| 30-01-05 | 01 | 0 | RELY-04 | unit | `go test ./relay/... -run TestFormatRelayContent` | ❌ W0 | ⬜ pending |
| 30-01-06 | 01 | 0 | RELY-04 | unit | `go test ./relay/... -run TestFormatRelayContent_UnknownPlatform` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `services/discord-listener/relay/manager_test.go` — stubs for RELY-01, RELY-02
- [ ] `services/discord-listener/relay/poster_test.go` — stubs for RELY-03, RELY-04

*Note: Implementation files (manager.go, repository.go, poster.go) are created in Wave 1. Wave 0 creates test stubs that fail until Wave 1 implements the code.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Relay message appears in Discord channel within 2 seconds | RELY-01 | Requires live Discord bot token and channel | Configure relay_enabled=true on a Discord source, send a Twitch message through the overlay, verify Discord channel receives formatted message |
| Toggle relay_enabled off stops relay within 30 seconds | RELY-02 | Requires live DB + Redis + Discord | Set relay_enabled=false on source, send messages, verify no Discord REST calls after ~30s |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
