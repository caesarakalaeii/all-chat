---
phase: 32-integration-wiring-fixes
verified: 2026-03-16T10:32:00Z
status: passed
score: 10/10 must-haves verified
re_verification: false
gaps: []
human_verification:
  - test: "Premium badge appears in live overlay for a viewer with is_premium=true in the viewers table"
    expected: "Gem/star badge rendered in overlay chat, positioned before platform badges"
    why_human: "Requires a live viewer account with is_premium=true and a running overlay — cannot verify DB-to-render path end-to-end programmatically"
  - test: "Gradient username renders without TypeError in browser console when overlay receives a chat_message with name_gradient as JSON string"
    expected: "Colored gradient username shown, no console error, no crash"
    why_human: "Requires live WebSocket message delivery in a browser; vitest tests cover parse guard logic but not the React render path in a real browser"
  - test: "GET /api/v1/auth/viewer/catalog/frames returns 200 (not 404) when services are running"
    expected: "JSON body with frames array, HTTP 200"
    why_human: "Route registration is verified statically; end-to-end 200 requires a running api-gateway + auth-service"
---

# Phase 32: Integration Wiring Fixes Verification Report

**Phase Goal:** Wire all integration gaps — premium badge enricher reads from viewers table, overlay parses name_gradient JSON, API gateway registers all 8 missing cosmetics routes.
**Verified:** 2026-03-16T10:32:00Z
**Status:** PASSED
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| #  | Truth | Status | Evidence |
|----|-------|--------|----------|
| 1  | Premium viewers receive a 'premium' badge injected into UserInfo.Badges | VERIFIED | `isPremium` branch at line 194 injects `premium` badge; `TestEnrich_PremiumBadge` PASS |
| 2  | Non-premium viewers do not receive the premium badge | VERIFIED | `TestEnrich_NoBadgesForNonRegisteredViewer` PASS; guard is `if isPremium` |
| 3  | Enricher SQL reads `viewers.is_premium` not `users.is_premium` | VERIFIED | Line 134: `COALESCE(v.is_premium, false) AS is_premium`; line 142: `LEFT JOIN viewers v ON v.id = vpi.viewer_id` |
| 4  | Overlay ws.onmessage converts name_gradient JSON string before setMessages (chat_message branch) | VERIFIED | Lines 239-241 in page.tsx: typeof guard + JSON.parse before setMessages at line 243 |
| 5  | Overlay ws.onmessage converts name_gradient JSON string before setMessages (message_update branch) | VERIFIED | Lines 254-256 in page.tsx: same guard pattern before setMessages at line 258 |
| 6  | Parse guard unit tests cover string→object, object passthrough, undefined | VERIFIED | `ws-message-parse.test.ts` 3/3 tests PASS |
| 7  | GET /api/v1/auth/viewer/catalog/frames registered in publicAPI (no auth) | VERIFIED | `main.go` line 377: `publicAPI.GET("/auth/viewer/catalog/frames", ...)` |
| 8  | GET /api/v1/auth/viewer/catalog/flairs registered in publicAPI (no auth) | VERIFIED | `main.go` line 378: `publicAPI.GET("/auth/viewer/catalog/flairs", ...)` |
| 9  | 6 admin cosmetics routes registered in protectedAPI (JWT required) | VERIFIED | `main.go` lines 460-465: GET/POST/DELETE frames, GET/POST/DELETE flairs in `protectedAPI` |
| 10 | API gateway builds without errors | VERIFIED | `go build ./...` exits 0, no output |

**Score:** 10/10 truths verified

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `services/message-processor/enricher/viewer_badge_enricher.go` | SQL query reading `viewers.is_premium` via `LEFT JOIN viewers v` | VERIFIED | Line 134 reads `v.is_premium`; line 142 adds `LEFT JOIN viewers v ON v.id = vpi.viewer_id` with explanatory comment |
| `frontend/src/app/overlay/__tests__/ws-message-parse.test.ts` | Unit tests for parse guard — 3 cases | VERIFIED | File exists, 46 lines, 3 test cases, all PASS |
| `frontend/src/app/overlay/[id]/page.tsx` | Parse guard in both ws.onmessage branches | VERIFIED | Lines 239-241 (chat_message) and 254-256 (message_update); `NameGradient` imported at line 25 |
| `services/api-gateway/cmd/main.go` | 8 new proxy route registrations | VERIFIED | 2 public at lines 377-378; 6 protected at lines 460-465 |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `viewer_badge_enricher.go` SQL SELECT | `viewers.is_premium` column | `LEFT JOIN viewers v ON v.id = vpi.viewer_id` | WIRED | Pattern present at line 142; SELECT uses `v.is_premium` at line 134 |
| `page.tsx` ws.onmessage chat_message branch | `buildGradientCSS` (via getUsernameSpanProps) | NameGradient object from parse guard at lines 239-241 | WIRED | Guard fires before setMessages; line 680 calls `buildGradientCSS(message.user.name_gradient)` |
| `page.tsx` ws.onmessage message_update branch | `buildGradientCSS` | Same parse guard at lines 254-256 | WIRED | Guard applied before setMessages at line 258 |
| `publicAPI` Gin group | `proxyHandler.ForwardRequest` | `publicAPI.GET("/auth/viewer/catalog/frames", ...)` | WIRED | Lines 377-378 in correct group (after credit-roll at line 374) |
| `protectedAPI` Gin group | `proxyHandler.ForwardRequest` | `protectedAPI.GET("/admin/cosmetics/frames", ...)` | WIRED | Lines 460-465 in correct group (after admin/sources at line 457) |

---

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| BADGE-02 | 32-01-PLAN.md | Premium users automatically receive a gem/star icon badge shown in overlays | SATISFIED | SQL reads `viewers.is_premium`; `TestEnrich_PremiumBadge` PASS; badge injected at lines 194-196 |
| PREM-02 | 32-02-PLAN.md | Gradient name renders in overlay using CSS `background-clip: text` | SATISFIED | Parse guard wired in both ws.onmessage branches; `NameGradient` object passed to `buildGradientCSS` at line 680 |
| PREM-03 | 32-03-PLAN.md | Premium viewer can select an avatar frame | SATISFIED (gateway) | `GET /auth/viewer/catalog/frames` registered at line 377 — 404 removed; selection UI depends on Phase 30 |
| PREM-04 | 32-03-PLAN.md | Premium viewer can select an avatar flair | SATISFIED (gateway) | `GET /auth/viewer/catalog/flairs` registered at line 378 — 404 removed; selection UI depends on Phase 30 |
| PREM-05 | 32-03-PLAN.md | Frame and flair catalog is managed by admins | SATISFIED (gateway) | 6 admin routes registered at lines 460-465 — 404 removed; admin UI depends on Phase 30 |
| WEB-03 | 32-03-PLAN.md | Premium users can browse and select avatar frame from the frame catalog | SATISFIED (gateway) | Public catalog frame route registered; browse/select UI depends on Phase 30 frontend work |
| WEB-04 | 32-03-PLAN.md | Premium users can browse and select avatar flair from the flair catalog | SATISFIED (gateway) | Public catalog flair route registered; browse/select UI depends on Phase 30 frontend work |

**Note on PREM-03, PREM-04, PREM-05, WEB-03, WEB-04:** Phase 32's scope is the gateway wiring only — removing the 404 blocker. The catalog and selection UI were implemented in Phase 30. REQUIREMENTS.md marks all five as "Pending" for Phase 32 because the 404 was the blocking gap; these are now unblocked.

---

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| None detected | — | — | — | — |

No TODOs, FIXMEs, empty implementations, or placeholder returns found in the three modified files.

---

### Human Verification Required

#### 1. Premium badge in live overlay

**Test:** Create or find a viewer account with `viewers.is_premium = true` in the database. Have that viewer send a message in a monitored Twitch/Kick/YouTube channel. Open the overlay in a browser.
**Expected:** The gem/star `premium` badge appears next to the viewer's username in the overlay, positioned before platform badges (e.g., subscriber badge).
**Why human:** Requires live services, a real viewer row with `is_premium=true`, and a browser rendering the overlay. The unit test (`TestEnrich_PremiumBadge`) confirms the logic path but not the full pipeline from DB through Redis cache to WebSocket to React render.

#### 2. Gradient username in live overlay without TypeError

**Test:** Have a registered viewer with a `name_gradient` set (JSON stored in DB) send a chat message. Open the overlay and inspect browser DevTools console.
**Expected:** Username displayed with gradient color, no `TypeError: g.colors.join is not a function` or similar error in the console.
**Why human:** The parse guard is unit-tested in isolation. Confirming no TypeError in a real browser with a real WebSocket message requires live end-to-end flow.

#### 3. Public catalog routes return 200

**Test:** With all services running (`make frontend-dev`), run: `curl http://localhost:8080/api/v1/auth/viewer/catalog/frames` and `curl http://localhost:8080/api/v1/auth/viewer/catalog/flairs`.
**Expected:** HTTP 200 with JSON body (not 404).
**Why human:** Route registration verified statically. Actual 200 depends on auth-service being up and the catalog having been seeded via migration.

---

### Gaps Summary

No gaps. All must-haves verified. All three plan objectives achieved:

1. **BADGE-02 (Plan 01):** Enricher SQL fixed — `LEFT JOIN viewers v` added, `is_premium` reads from `v.is_premium` (viewers table, migration 036), not `u.is_premium` (users/streamer table). All 21 enricher unit tests pass including the four badge-specific tests.

2. **PREM-02 (Plan 02):** Overlay parse guard wired in both `chat_message` and `message_update` ws.onmessage branches before `setMessages`. `NameGradient` type imported. Three-case unit test file created and passes.

3. **PREM-03/04/05, WEB-03/04 (Plan 03):** All 8 proxy routes registered — 2 public catalog routes in `publicAPI` group (no JWT), 6 admin cosmetics routes in `protectedAPI` group (JWT required at gateway, admin role enforced at auth-service). Gateway compiles clean.

---

_Verified: 2026-03-16T10:32:00Z_
_Verifier: Claude (gsd-verifier)_
