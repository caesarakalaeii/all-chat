---
phase: 32
slug: setup-ui
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-16
---

# Phase 32 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Vitest ^4.0.18 |
| **Config file** | `frontend/vitest.config.ts` |
| **Quick run command** | `cd frontend && npm test -- --project=unit --run` |
| **Full suite command** | `cd frontend && npm test -- --run` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd frontend && npm test -- --project=unit --run`
- **After every plan wave:** Run `cd frontend && npm test -- --run`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 32-01-01 | 01 | 0 | UI-01, UI-04 | unit | `cd frontend && npm test -- --project=unit --run src/lib/__tests__/platform-colors.test.ts` | ❌ W0 | ⬜ pending |
| 32-01-02 | 01 | 0 | UI-02 | unit | `cd frontend && npm test -- --project=unit --run src/lib/__tests__/types.test.ts` | ❌ W0 | ⬜ pending |
| 32-01-03 | 01 | 0 | UI-03 | unit | `cd frontend && npm test -- --project=unit --run src/lib/api/__tests__/discord.test.ts` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `frontend/src/lib/__tests__/platform-colors.test.ts` — extend to cover `discord` entries (UI-01, UI-04)
- [ ] `frontend/src/lib/api/__tests__/discord.test.ts` — covers `discordApi.updateSourceConfig` PATCH call (UI-03)
- [ ] `frontend/src/lib/__tests__/types.test.ts` — covers type narrowing for `DiscordSourceConfig` and null `guild_icon` (UI-02)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Discord connect card initiates OAuth redirect | UI-01 | Requires live OAuth flow with Discord API | Click "Connect Discord Server" in Settings, verify redirect to Discord authorization page |
| Guild name and icon appear after connect | UI-01 | Requires real OAuth callback with guild data | Complete OAuth flow, verify card shows guild name and icon |
| Overlay editor add-source Discord flow | UI-02 | Requires UI interaction with channel dropdown | Add Discord source in overlay editor, verify guild selector then channel dropdown populate |
| Source card shows relay active/inactive indicator | UI-03 | Visual indicator state dependent on live connection | Toggle relay on/off, verify badge color changes immediately |
| Relay config panel saves outbound channel | UI-04 | Requires PATCH round-trip | Select outbound channel, save, reload page, verify selection persists |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
