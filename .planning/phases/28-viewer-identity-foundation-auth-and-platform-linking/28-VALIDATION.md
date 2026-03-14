---
phase: 28
slug: viewer-identity-foundation-auth-and-platform-linking
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-14
---

# Phase 28 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go test (stdlib) + testify |
| **Config file** | none — per-package `go test` |
| **Quick run command** | `cd services/auth-service && go test ./... -count=1 -short` |
| **Full suite command** | `cd services/auth-service && go test ./... && cd ../message-processor && go test ./enricher/...` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd services/auth-service && go test ./... -count=1 -short`
- **After every plan wave:** Run `cd services/auth-service && go test ./... && cd ../message-processor && go test ./enricher/...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 28-01-01 | 01 | 1 | VID-03, VID-04 | unit | `go test ./repository/... -run TestViewerIdentityCreate` | ❌ W0 | ⬜ pending |
| 28-01-02 | 01 | 1 | VID-05, VID-06 | unit | `go test ./handlers/... -run TestViewerJWTClaims` | ❌ W0 | ⬜ pending |
| 28-02-01 | 02 | 1 | VID-05 | unit | `go test ./handlers/... -run TestTwitchExchange` | ❌ W0 | ⬜ pending |
| 28-02-02 | 02 | 1 | VID-03 | unit | `go test ./handlers/... -run TestPatchCosmetics` | ❌ W0 | ⬜ pending |
| 28-03-01 | 03 | 2 | EXT-01 | manual | N/A — browser extension UI | manual-only | ⬜ pending |
| 28-03-02 | 03 | 2 | EXT-02 | manual | N/A — browser extension interaction | manual-only | ⬜ pending |
| 28-03-03 | 03 | 2 | EXT-03 | manual | N/A — browser extension interaction | manual-only | ⬜ pending |
| 28-04-01 | 04 | 2 | EXT-04 | manual | N/A — overlay DOM inspection | manual-only | ⬜ pending |
| 28-04-02 | 04 | 2 | VID-04 | unit | `go test ./enricher/... -run TestViewerBadgeEnricher` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `services/auth-service/handlers/viewer_cosmetics_test.go` — stubs for VID-03 PATCH handler
- [ ] `services/auth-service/repository/viewer_identity_test.go` — stubs for VID-04 viewer_platform_identities create/lookup
- [ ] `services/auth-service/handlers/viewer_exchange_test.go` — stubs for VID-05 POST code-exchange (Twitch + YouTube + Kick)
- [ ] `services/message-processor/enricher/viewer_badge_enricher_test.go` — stubs for ViewerBadgeEnricher Redis cache hit, miss, null sentinel, color injection

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Color picker renders in signed-in state | EXT-01 | Requires running browser with installed extension | Load unpacked extension, sign in, verify `<input type="color">` visible |
| Color save POSTs to PATCH endpoint | EXT-02 | Requires browser extension interaction | Change color in picker, verify network request to `/api/viewer/cosmetics` |
| "Open Settings" opens `/settings/viewer` | EXT-03 | Requires browser extension interaction | Click "Open Settings" button, verify tab opens to correct URL |
| Content script applies name_color to own messages | EXT-04 | Requires overlay DOM inspection with live chat | Sign in, send chat message, verify username color applied in overlay |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
