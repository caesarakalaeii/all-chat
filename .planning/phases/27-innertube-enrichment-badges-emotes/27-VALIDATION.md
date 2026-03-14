---
phase: 27
slug: innertube-enrichment-badges-emotes
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-14
---

# Phase 27 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (stdlib) + testify v1.11.1 + miniredis v2.37.0 |
| **Config file** | none — `go test ./...` |
| **Quick run command** | `cd services/youtube-listener-innertube && go test ./innertube/... && cd ../../services/message-processor && go test ./normalizer/...` |
| **Full suite command** | `cd services/youtube-listener-innertube && go test ./... && cd ../../services/message-processor && go test ./...` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd services/youtube-listener-innertube && go test ./innertube/... && cd ../../services/message-processor && go test ./normalizer/...`
- **After every plan wave:** Run full suite (both services)
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 27-01-01 | 01 | 0 | YTBADGE-01, YTBADGE-02 | unit stub | `go test ./innertube/... -run TestExtractBadgesRich` | ❌ W0 | ⬜ pending |
| 27-01-02 | 01 | 0 | YTEMOTE-01, YTEMOTE-02, YTEMOTE-03, YTEMOTE-04 | unit stub | `go test ./innertube/... -run TestExtractMessageText` | ❌ W0 | ⬜ pending |
| 27-01-03 | 01 | 0 | YTEMOTE-05 | unit stub (miniredis) | `go test ./... -run TestCacheYTEmotes` | ❌ W0 | ⬜ pending |
| 27-01-04 | 01 | 0 | YTBADGE-03, YTBADGE-04 | unit stub | `go test ./normalizer/... -run TestYouTubeNormalizer_ExtractBadges` | ❌ W0 | ⬜ pending |
| 27-02-01 | 02 | 1 | YTBADGE-01, YTBADGE-02 | unit | `go test ./innertube/... -run TestExtractBadgesRich` | ❌ W0 | ⬜ pending |
| 27-02-02 | 02 | 1 | YTEMOTE-01, YTEMOTE-02, YTEMOTE-03, YTEMOTE-04 | unit | `go test ./innertube/... -run TestExtractMessageText` | ❌ W0 | ⬜ pending |
| 27-02-03 | 02 | 1 | YTEMOTE-05 | unit (miniredis) | `go test ./... -run TestCacheYTEmotes` | ❌ W0 | ⬜ pending |
| 27-03-01 | 03 | 2 | YTBADGE-03, YTBADGE-04 | unit | `go test ./normalizer/... -run TestYouTubeNormalizer_ExtractBadges` | ❌ W0 | ⬜ pending |
| 27-03-02 | 03 | 2 | YTEMOTE-01, YTEMOTE-02, YTEMOTE-03 | unit | `go test ./normalizer/... -run TestYouTubeNormalizer_EmoteData` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `services/youtube-listener-innertube/innertube/parser_badge_test.go` — stubs for YTBADGE-01, YTBADGE-02
- [ ] `services/youtube-listener-innertube/innertube/parser_emote_test.go` — stubs for YTEMOTE-01, YTEMOTE-02, YTEMOTE-03, YTEMOTE-04
- [ ] `services/message-processor/normalizer/youtube_normalizer_badges_test.go` — stubs for YTBADGE-03, YTBADGE-04
- [ ] `services/message-processor/normalizer/youtube_normalizer_emotes_test.go` — stubs for YTEMOTE-01, YTEMOTE-02, YTEMOTE-03 in normalizer
- [ ] `services/youtube-listener-innertube/yt_emote_cache/cache_test.go` — stubs for YTEMOTE-05 using miniredis; add `github.com/alicebob/miniredis/v2` to innertube `go.mod`

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Real channel membership badge image renders in overlay | YTBADGE-01 | Requires live YouTube stream with members | Monitor a live stream overlay, verify membership badge shows channel image not generic SVG |
| Unicode emoji renders as text only (no image regression) | YTEMOTE-03 | Requires live chat with Unicode emoji | Send 🎉 in chat, verify it appears as text not broken image in overlay |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
