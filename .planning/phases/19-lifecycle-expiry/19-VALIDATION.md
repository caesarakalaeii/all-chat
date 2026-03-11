---
phase: 19
slug: lifecycle-expiry
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-11
---

# Phase 19 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test + testify (share-service, twitch-eventsub-listener, youtube-listener) |
| **Config file** | none — standard `go test ./...` |
| **Quick run command** | `cd services/share-service && go test ./... -short` |
| **Full suite command** | `cd services/share-service && go test ./... && cd ../twitch-eventsub-listener && go test ./...` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd services/share-service && go test ./... -short`
- **After every plan wave:** Run `cd services/share-service && go test ./... && cd ../twitch-eventsub-listener && go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 19-01-01 | 01 | 0 | EXPIRY-01,EXPIRY-04 | unit | `cd services/share-service && go test ./... -short` | ❌ W0 | ⬜ pending |
| 19-01-02 | 01 | 1 | EXPIRY-01 | unit | `cd services/share-service && go test ./handlers/ -run TestAcceptShareRequest -short` | ❌ W0 | ⬜ pending |
| 19-01-03 | 01 | 1 | EXPIRY-04 | unit | `cd services/share-service && go test ./jobs/ -run TestExpiryJob_TimedAccepted -short` | ❌ W0 | ⬜ pending |
| 19-02-01 | 02 | 2 | EXPIRY-02 | unit | `cd services/twitch-eventsub-listener && go test ./eventsub/ -run TestSubscribeToStream -short` | ❌ W0 | ⬜ pending |
| 19-02-02 | 02 | 2 | EXPIRY-03 | unit | `cd services/share-service && go test ./jobs/ -run TestLifecycleSubscriber -short` | ❌ W0 | ⬜ pending |
| 19-03-01 | 03 | 3 | EXPIRY-05 | unit | `cd services/youtube-listener-innertube && go test ./... -run TestStreamOfflinePublishesLifecycle -short` | ❌ W0 | ⬜ pending |
| 19-04-01 | 04 | 4 | EXPIRY-06 | manual | see manual verifications | N/A | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `services/share-service/jobs/lifecycle_subscriber_test.go` — stubs for EXPIRY-03
- [ ] `services/share-service/jobs/expiry_test.go` — add TestExpiryJob_TimedAcceptedShares for EXPIRY-04
- [ ] `services/twitch-eventsub-listener/eventsub/subscription_manager_test.go` — covers EXPIRY-02
- [ ] `services/youtube-listener-innertube/streams/lifecycle_test.go` — covers EXPIRY-05 (YouTube publish)
- [ ] Migration `migrations/034_share_expiry_fields.sql` — needed before all tests that touch DB

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Kick gracefully shows "Unlimited" when stream platform is Kick | EXPIRY-06 | No automated webhook path; Kick lifecycle deferred | 1. Create a share where sender has Kick as active platform. 2. Open AcceptModal. 3. Verify "This stream" option is disabled/hidden or shows a note. 4. Confirm "Unlimited" is the default. |
| 60-second debounce prevents phantom Twitch expiry | EXPIRY-02/EXPIRY-03 | Requires simulating rapid stream.offline → stream.online sequence | 1. Trigger stream.offline webhook. 2. Within 60s, trigger stream.online. 3. Verify share was NOT expired after 60s. |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
