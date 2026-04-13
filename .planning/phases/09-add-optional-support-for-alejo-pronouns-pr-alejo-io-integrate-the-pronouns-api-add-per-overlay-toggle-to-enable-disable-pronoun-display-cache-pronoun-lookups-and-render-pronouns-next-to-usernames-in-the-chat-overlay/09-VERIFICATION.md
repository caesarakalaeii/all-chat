---
phase: 09-add-optional-support-for-alejo-pronouns
verified: 2026-04-04T16:53:30Z
status: human_needed
score: 12/12 must-haves verified
human_verification:
  - test: "Pronoun pill visual rendering in chat overlay"
    expected: "A colored badge-style pill with pronoun text (e.g. 'she/her') appears next to the username when a message arrives from a user with pronouns set on pr.alejo.io"
    why_human: "Requires live Alejo API lookup over the network with a real Twitch account that has pronouns registered; automated tests mock the API"
  - test: "Color picker visual accuracy in appearance panel"
    expected: "Changing the Pill color via the color picker in the appearance settings immediately updates the pill color in the overlay preview"
    why_human: "CSS rendering and live preview interaction requires visual verification; not testable via automated checks"
  - test: "Pronoun controls disabled state visual appearance"
    expected: "When 'Show pronouns' toggle is turned off, the Position radio and Pill color picker are visually dimmed (opacity-40) and non-interactive"
    why_human: "opacity-40 and pointer-events-none verified by unit test class name, but visual result and pointer interaction require human confirmation"
---

# Phase 9: Alejo Pronouns Integration — Verification Report

**Phase Goal:** Integrate Alejo pronouns API into the message-processor enrichment pipeline, add per-overlay pronoun display toggle with configurable position and color, cache lookups in Redis with 24h TTL, resolve cross-platform pronouns via linked Twitch accounts, and render pronoun pills next to usernames in the chat overlay.
**Verified:** 2026-04-04T16:53:30Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|---------|
| 1 | Pronoun pill renders as a colored badge-style pill next to username (D-01) | ✓ VERIFIED | `rounded-full px-2 py-1 text-[11px] font-semibold leading-none text-white` + `style={{ backgroundColor: pronounColor }}` at lines 793-794, 830-831 of page.tsx |
| 2 | Pill position is controlled by pronoun_position setting before/after username (D-02) | ✓ VERIFIED | Two render sites: `pronounPosition === 'before'` at line 791 and `pronounPosition === 'after'` at line 828 of page.tsx |
| 3 | Pill color is controlled by pronoun_color setting (D-03) | ✓ VERIFIED | `style={{ backgroundColor: pronounColor }}` at both render sites; default `#7B68EE` set in useState |
| 4 | Pronoun lookups cached in Redis with 24h TTL (D-04) | ✓ VERIFIED | `PronounCacheTTL = 24 * time.Hour` constant; `Set(ctx, cacheKey, displayText, PronounCacheTTL)` in Enrich() |
| 5 | API errors result in silent skip — message renders without pronouns (D-05) | ✓ VERIFIED | All error paths in Enrich() return nil with zap.Warn logging; 10 test functions verify silent skip behavior |
| 6 | Cache key prefix pattern: pronoun:{twitch_username} lowercase (D-06) | ✓ VERIFIED | `PronounCacheKeyPrefix = "pronoun:"` + `strings.ToLower(...)` in Enrich() |
| 7 | show_pronouns enabled by default for new overlays (D-07) | ✓ VERIFIED | `useState(true)` for showPronouns in page.tsx line 86 |
| 8 | Toggle, position, and color controls in VisibilityGroup (D-08) | ✓ VERIFIED | `label="Show pronouns"` toggle, PRONOUN_POSITION_OPTIONS RadioGroup, ColorPickerControl all in VisibilityGroup.tsx |
| 9 | New display_settings fields: show_pronouns, pronoun_position, pronoun_color (D-09) | ✓ VERIFIED | All three fields in `DisplaySettings` interface in overlay.ts lines 49-51 |
| 10 | Alejo API lookups use Twitch usernames; Twitch messages use message username directly (D-10) | ✓ VERIFIED | `if msg.Platform == "twitch" { twitchLogin = strings.ToLower(msg.User.Username) }` in pronoun_enricher.go |
| 11 | Non-Twitch with linked Twitch account gets pronouns; without link silently skips (D-11) | ✓ VERIFIED | `else { twitchLogin = strings.ToLower(msg.User.TwitchUsername) }` + `if twitchLogin == "" { return nil }` |
| 12 | No extra DB queries for cross-platform resolution — reuses ViewerBadgeEnricher data (D-12) | ✓ VERIFIED | `TwitchUsername string \`json:"-"\`` on UserInfo; populated by extended LEFT JOIN query in viewer_badge_enricher.go; `pronounEnricher.Enrich` runs after `viewerBadgeEnricher.Enrich` in CHAT PATH |

**Score:** 12/12 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `services/message-processor/enricher/pronoun_enricher.go` | PronounEnricher with Alejo API + Redis cache | ✓ VERIFIED | 262 lines; exports NewPronounEnricher, PronounEnricher; contains all required constants and Enrich() method |
| `services/message-processor/enricher/pronoun_enricher_test.go` | Unit tests for all enricher paths | ✓ VERIFIED | 267 lines; 10 test functions covering Twitch, non-Twitch, cache hit, 404 sentinel, network error, alt pronoun, singular pronoun, TTL |
| `services/message-processor/models/message.go` | Pronouns and TwitchUsername fields on UserInfo | ✓ VERIFIED | `Pronouns string \`json:"pronouns,omitempty"\`` and `TwitchUsername string \`json:"-"\`` both present at lines 56-57 |
| `docs/adr/0010-pronoun-enricher-alejo-api.md` | ADR documenting Alejo API external dependency | ✓ VERIFIED | 95 lines; Status: Accepted; documents external dependency, failure mode decisions, consequences |
| `frontend/src/lib/types/message.ts` | pronouns field on UserInfo | ✓ VERIFIED | `pronouns?: string` at line 90 |
| `frontend/src/lib/types/overlay.ts` | pronoun display_settings fields | ✓ VERIFIED | show_pronouns, pronoun_position, pronoun_color at lines 49-51 |
| `frontend/src/lib/types/visual-settings.ts` | pronoun VisualSettings fields | ✓ VERIFIED | showPronouns, pronounPosition, pronounColor at lines 59-61 |
| `frontend/src/app/overlay/[id]/page.tsx` | Pronoun pill rendering and config loading | ✓ VERIFIED | State variables at lines 86-88; config loading at 131-170; two render sites at 791-796, 828-833 |
| `frontend/src/app/overlay/__tests__/pronoun-pill.test.tsx` | Vitest tests for pronoun pill rendering | ✓ VERIFIED | 80 lines; 14 test cases via shouldRenderPronounPill and getPronounPillProps helpers |
| `frontend/src/components/appearance/VisibilityGroup.tsx` | Pronoun toggle, position radio, color picker | ✓ VERIFIED | ColorPickerControl imported; PRONOUN_POSITION_OPTIONS constant; pronounsVisible, pronounPosition, pronounColor derivation; Show pronouns toggle, RadioGroup, ColorPickerControl in JSX |
| `frontend/src/components/appearance/__tests__/VisibilityGroup.test.tsx` | Vitest tests for pronoun controls | ✓ VERIFIED | 177 lines; 17 test cases (9 pronoun-specific covering toggle, position, color, disabled state) |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `cmd/main.go` | `enricher/pronoun_enricher.go` | `pronounEnricher.Enrich(ctx, unified)` in CHAT PATH after viewerBadgeEnricher | ✓ WIRED | Line 177 constructs enricher; line 519 calls Enrich — only 1 occurrence (CHAT PATH only, not EVENT PATH) |
| `pronoun_enricher.go` | `api.pronouns.alejo.io` | HTTP GET /v1/users/{twitch_login} | ✓ WIRED | `alejoAPIBaseURL = "https://api.pronouns.alejo.io/v1"` at line 25; `fmt.Sprintf("%s/users/%s", e.baseURL, twitchLogin)` in Enrich() |
| `viewer_badge_enricher.go` | `models/message.go` | TwitchUsername populated from extended DB query | ✓ WIRED | `COALESCE(twitch_vs.username, '') AS twitch_username` at line 141; LEFT JOINs at 151-155; `msg.User.TwitchUsername = twitchUsername` at line 217 |
| `page.tsx` | `message.ts` | msg.user.pronouns field read during rendering | ✓ WIRED | `message.user?.pronouns` at lines 791 and 828 |
| `page.tsx` | `/api/v1/overlays/public/${id}/config` | loadConfig reads show_pronouns, pronoun_position, pronoun_color from display_settings | ✓ WIRED | `display.show_pronouns` at line 131; `display.pronoun_position` config loading confirmed |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `page.tsx` (pronoun pill) | `message.user.pronouns` | PronounEnricher.Enrich() → Alejo API → Redis cache | Yes — live HTTP fetch from external API, cached in Redis | ✓ FLOWING |
| `pronoun_enricher.go` | `msg.User.Pronouns` | Alejo API GET /v1/users/{login} + Redis cache | Yes — real API response parsed from JSON | ✓ FLOWING |
| `viewer_badge_enricher.go` | `msg.User.TwitchUsername` | PostgreSQL DB query via LEFT JOIN on viewer_platform_identities + viewer_sessions | Yes — DB query produces real linked Twitch username | ✓ FLOWING |
| `VisibilityGroup.tsx` | `showPronouns`, `pronounPosition`, `pronounColor` | visualSettings prop from parent (appearance panel) | Yes — propagated via onChange callback to parent state | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| PronounEnricher unit tests pass | `go test ./enricher/... -run TestPronoun -count=1` | ok enricher 0.007s | ✓ PASS |
| Full message-processor test suite | `go test ./... -count=1` | All 14 packages ok | ✓ PASS |
| Frontend pronoun pill tests pass | `npx vitest run src/app/overlay/__tests__/pronoun-pill.test.tsx` | 14 passed | ✓ PASS |
| VisibilityGroup pronoun controls tests | `npx vitest run src/components/appearance/__tests__/VisibilityGroup.test.tsx` | 17 passed | ✓ PASS |
| TypeScript compiles cleanly | `npx tsc --noEmit` | 0 errors | ✓ PASS |
| message-processor binary builds | `go build -o /tmp/message-processor-test ./cmd/main.go` | 0 errors | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|---------|
| D-01 | 09-02-PLAN.md | Pronouns render as badge-style pill | ✓ SATISFIED | Pill span with rounded-full CSS in page.tsx; 14 Vitest tests confirm rendering logic |
| D-02 | 09-02-PLAN.md | Position configurable before/after username per overlay | ✓ SATISFIED | pronounPosition state + two conditional render sites in page.tsx |
| D-03 | 09-02-PLAN.md | Pill color configurable per overlay | ✓ SATISFIED | pronounColor state + `backgroundColor: pronounColor` inline style |
| D-04 | 09-01-PLAN.md | Pronoun lookups cached in Redis with 24h TTL | ✓ SATISFIED | PronounCacheTTL constant + Set() call in pronoun_enricher.go |
| D-05 | 09-01-PLAN.md | API unreachable/error → silent skip, no visible error | ✓ SATISFIED | All error paths return nil; TestPronounEnricher_NetworkError_SilentSkip verifies |
| D-06 | 09-01-PLAN.md | Cache key: pronoun:{twitch_username} lowercase | ✓ SATISFIED | `PronounCacheKeyPrefix = "pronoun:"` + strings.ToLower() |
| D-07 | 09-02-PLAN.md | show_pronouns enabled by default | ✓ SATISFIED | `useState(true)` for showPronouns in page.tsx |
| D-08 | 09-03-PLAN.md | Controls in VisibilityGroup (toggle + position + color) | ✓ SATISFIED | Show pronouns toggle, Before/After RadioGroup, Pill color ColorPickerControl all present |
| D-09 | 09-02-PLAN.md | display_settings: show_pronouns, pronoun_position, pronoun_color | ✓ SATISFIED | All three fields in DisplaySettings interface in overlay.ts |
| D-10 | 09-01-PLAN.md | Alejo API uses Twitch usernames; Twitch messages use msg username directly | ✓ SATISFIED | `msg.Platform == "twitch"` branch uses `msg.User.Username` |
| D-11 | 09-01-PLAN.md | Non-Twitch with linked Twitch account gets pronouns; without link silently skips | ✓ SATISFIED | `msg.User.TwitchUsername` path; empty login returns nil |
| D-12 | 09-01-PLAN.md | No extra DB queries for cross-platform — reuse ViewerBadgeEnricher data | ✓ SATISFIED | TwitchUsername field on UserInfo (json:"-") set by ViewerBadgeEnricher LEFT JOIN; no DB call in PronounEnricher |

All 12 requirements satisfied.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| None found | — | — | — | — |

No TODOs, FIXME, placeholders, empty implementations, or hardcoded empty data found in the phase-produced files. The `pronounEmptySentinel = ""` constant is a legitimate cache sentinel (empty string distinguishable from Redis miss) and not a stub — the Enrich() logic correctly distinguishes `err == nil` (hit) from `redis.Nil` (miss).

### Human Verification Required

#### 1. Pronoun pill visual rendering in chat overlay

**Test:** Open the overlay page in a browser. Send a Twitch chat message as a user who has pronouns set on pr.alejo.io (e.g., a Twitch account with pronouns registered). Observe the rendered message in the overlay.
**Expected:** A colored badge-style pill with pronoun text (e.g. "she/her", "he/him") appears next to the username, positioned according to the pronoun_position setting, with the configured background color.
**Why human:** Requires a live Twitch session with a real account that has pronouns registered at pr.alejo.io. The Alejo API is mocked in unit tests but has not been verified end-to-end in production.

#### 2. Color picker visual accuracy in appearance panel

**Test:** Open the overlay appearance settings page. Toggle "Show pronouns" on. Click the Pill color picker and change the color.
**Expected:** The color picker opens and selecting a color immediately updates the state. When the overlay page is open, the pronoun pill reflects the new color.
**Why human:** CSS rendering and the live preview interaction require visual confirmation. The onChange callback is unit-tested but the visual output is not.

#### 3. Pronoun controls disabled state visual appearance

**Test:** Open the overlay appearance settings page. Toggle "Show pronouns" off.
**Expected:** The Position radio (Before/After username) and Pill color picker become visually dimmed (opacity reduced) and non-interactive (cannot click).
**Why human:** The `opacity-40` and `pointer-events-none` classes are verified by the VisibilityGroup test (checks class names), but the actual visual dimming and pointer interaction blocking need human confirmation in the browser.

### Gaps Summary

No gaps found. All 12 phase requirements are satisfied by substantive, wired, and data-flowing implementations. Three items are routed to human verification for visual/behavioral confirmation that cannot be automated without live external service access.

---

_Verified: 2026-04-04T16:53:30Z_
_Verifier: Claude (gsd-verifier)_
