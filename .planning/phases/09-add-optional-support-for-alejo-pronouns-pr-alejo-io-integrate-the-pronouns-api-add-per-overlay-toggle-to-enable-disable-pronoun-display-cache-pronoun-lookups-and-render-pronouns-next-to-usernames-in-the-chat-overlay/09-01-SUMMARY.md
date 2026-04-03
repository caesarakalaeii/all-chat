---
phase: 09-add-optional-support-for-alejo-pronouns
plan: "01"
subsystem: message-processor
tags: [pronouns, enricher, alejo-api, redis-cache, tdd, adr]
dependency_graph:
  requires: []
  provides:
    - PronounEnricher with Alejo API integration and Redis cache
    - Pronouns field on UserInfo (json:pronouns,omitempty)
    - TwitchUsername field on UserInfo (json:"-", internal pipeline only)
    - ViewerBadgeEnricher extended query with linked Twitch username resolution
  affects:
    - services/message-processor (pipeline enrichment)
    - docs/adr (ADR-0010 added)
tech_stack:
  added: []
  patterns:
    - TDD red-green per task
    - httptest.NewServer for mock external API
    - miniredis for Redis unit tests
    - Silent-fail enricher pattern (D-05)
    - Empty sentinel caching for 404 API responses
key_files:
  created:
    - services/message-processor/enricher/pronoun_enricher.go
    - services/message-processor/enricher/pronoun_enricher_test.go
    - docs/adr/0010-pronoun-enricher-alejo-api.md
  modified:
    - services/message-processor/models/message.go
    - services/message-processor/enricher/viewer_badge_enricher.go
    - services/message-processor/enricher/viewer_badge_enricher_test.go
    - services/message-processor/cmd/main.go
    - services/message-processor/README.md
    - docs/adr/README.md
decisions:
  - "PronounEnricher uses newPronounEnricherWithURL internal constructor for test injection (not a public option)"
  - "Empty string used as 404 sentinel (not a separate constant) — Redis GET returns empty string, distinguishable from redis.Nil cache miss"
  - "Test server registers /users/{login} and /pronouns without /v1 prefix since baseURL replaces the full production constant"
  - "pronounsMap loaded once at construction via fetchPronounsMap; empty map on failure (graceful degradation)"
metrics:
  duration: 451s
  completed_date: "2026-04-04"
  tasks_completed: 2
  files_changed: 8
---

# Phase 09 Plan 01: Backend Pronoun Enricher Summary

Backend pipeline enrichment for optional pronoun display — PronounEnricher fetches from Alejo API (api.pronouns.alejo.io/v1/), caches with 24h Redis TTL, resolves cross-platform via linked Twitch accounts, and silently skips on all API errors.

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | Extend UserInfo model + ViewerBadgeEnricher for TwitchUsername resolution | 59d0a23 | models/message.go, viewer_badge_enricher.go, viewer_badge_enricher_test.go |
| 2 | Create PronounEnricher + wire into CHAT PATH + ADR + README | 1099e44 | pronoun_enricher.go, pronoun_enricher_test.go, cmd/main.go, ADR-0010, README |

## Verification Results

```
go test ./... -count=1
ok  github.com/caesar/all-chat/services/message-processor/classifier
ok  github.com/caesar/all-chat/services/message-processor/cmd
ok  github.com/caesar/all-chat/services/message-processor/consumer
ok  github.com/caesar/all-chat/services/message-processor/dedup
ok  github.com/caesar/all-chat/services/message-processor/enricher
ok  github.com/caesar/all-chat/services/message-processor/filter
ok  github.com/caesar/all-chat/services/message-processor/handlers
ok  github.com/caesar/all-chat/services/message-processor/integration
ok  github.com/caesar/all-chat/services/message-processor/normalizer
ok  github.com/caesar/all-chat/services/message-processor/publisher
ok  github.com/caesar/all-chat/services/message-processor/registry
ok  github.com/caesar/all-chat/services/message-processor/router
ok  github.com/caesar/all-chat/services/message-processor/sessions
ok  github.com/caesar/all-chat/services/message-processor/seventv

go build -o /tmp/message-processor ./cmd/... → success
test -f docs/adr/0010-pronoun-enricher-alejo-api.md → exists
```

## Decisions Made

1. **`newPronounEnricherWithURL` internal test constructor**: Accepts custom baseURL and pre-loaded pronounsMap for unit tests, avoiding real network calls. Public `NewPronounEnricher` fetches the map at startup.

2. **Empty string as 404 sentinel**: When Alejo API returns 404 (no pronouns set), we store an empty string `""` in Redis with 24h TTL. On retrieval, `err == nil` (key found) with empty value means "no pronouns". This is distinguishable from `redis.Nil` (cache miss).

3. **Test server path scheme**: Mock server registers `/pronouns` and `/users/{login}` (without `/v1`) because the baseURL in tests replaces the full `alejoAPIBaseURL` constant. The enricher appends `/users/{login}` directly to baseURL.

4. **pronounsMap loaded at construction**: Fetched once from `/v1/pronouns` at `NewPronounEnricher` call time. Empty map on failure (graceful degradation) — unknown pronoun IDs fall back to raw ID string.

## Deviations from Plan

**None** — plan executed exactly as written. The test server path scheme (no `/v1` prefix in mock routes) was an implementation detail not specified in the plan, handled correctly during TDD GREEN phase.

## Known Stubs

None — all fields are wired to real data sources:
- `msg.User.Pronouns` populated from live Alejo API response (cached in Redis)
- `msg.User.TwitchUsername` populated from DB query via LEFT JOIN on viewer_platform_identities

## Self-Check: PASSED

All files created, all commits verified, all key content confirmed.

| Check | Result |
|-------|--------|
| pronoun_enricher.go exists | FOUND |
| pronoun_enricher_test.go exists | FOUND |
| ADR-0010 exists | FOUND |
| Commit 59d0a23 (task 1) | FOUND |
| Commit 1099e44 (task 2) | FOUND |
| Pronouns field in UserInfo | FOUND |
| TwitchUsername json:"-" | FOUND |
| twitch_username COALESCE in ViewerBadgeEnricher query | FOUND |
| pronounEnricher construction in cmd/main.go | FOUND |
| pronounEnricher.Enrich call in CHAT PATH | FOUND |
