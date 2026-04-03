# ADR-0010: Pronoun Enricher via Alejo API

**Date**: 2026-04-04
**Status**: Accepted
**Deciders**: Phase 9 — Add optional support for Alejo pronouns

---

## Context and Problem Statement

All-Chat displays chat messages from multiple streaming platforms. Streamers and viewers increasingly expect pronoun display alongside usernames in chat overlays. The only publicly available pronoun data source for Twitch users is the Alejo pronouns API (api.pronouns.alejo.io), which users opt into at pr.alejo.io. Adding pronoun display requires integrating this external API dependency into the message-processor enrichment pipeline.

---

## Decision Drivers

- Pronoun display must be optional — viewers only see pronouns if they have opted in at pr.alejo.io
- API failures must not degrade message delivery (D-05: silent skip on errors)
- Cache is required to avoid hammering a third-party API with no SLA
- Non-Twitch users with a linked Twitch account should still get pronouns (cross-platform)
- The `TwitchUsername` field used for cross-platform lookup must never be sent to clients

---

## Considered Options

1. **Alejo API (api.pronouns.alejo.io/v1/)** — Opt-in pronoun database for Twitch users
   - ✅ Pros: Free, widely adopted by streaming community, accurate opt-in data
   - ❌ Cons: External dependency, no SLA, third-party outage silences pronouns

2. **Store pronouns in All-Chat's own database** — Users set pronouns in profile settings
   - ✅ Pros: No external dependency, works for all platforms
   - ❌ Cons: Creates user-facing feature gap (separate from established Twitch community standard), requires UI, users must re-register pronouns they already set elsewhere

3. **No pronoun support** — Skip pronouns entirely
   - ✅ Pros: No complexity
   - ❌ Cons: Missing community-expected feature for inclusive chat display

---

## Decision Outcome

**Chosen**: Option 1 — Alejo API with Redis caching and silent fail

**Rationale**: Alejo pronouns is the established community standard for Twitch pronoun display. The graceful degradation design (silent skip on failure, 24h cache) ensures pronoun availability does not affect message delivery. Cross-platform resolution via linked Twitch accounts is achievable without an extra DB roundtrip by extending the existing `ViewerBadgeEnricher` query.

---

## Consequences

### Positive

- Pronouns populated for all users who have opted in at pr.alejo.io — zero extra user action required on All-Chat
- 24h Redis cache drastically reduces API call volume (one API call per unique viewer per day at most)
- Empty sentinel caching for 404 responses avoids re-fetching for users with no pronouns
- Cross-platform pronoun resolution works for YouTube/Kick/TikTok/Discord users with linked Twitch accounts

### Negative

- External API dependency with no SLA — Alejo API outage silences pronouns for all viewers until cache expires
- Pronouns only available for Twitch users; non-Twitch users without a linked account never get pronouns
- Cache key `pronoun:{twitch_login}` — pronoun changes on pr.alejo.io take up to 24h to propagate (acceptable trade-off)

---

## Implementation

- **Files**:
  - `services/message-processor/enricher/pronoun_enricher.go` — PronounEnricher with Alejo API integration and Redis cache
  - `services/message-processor/enricher/pronoun_enricher_test.go` — Unit tests (10 test functions)
  - `services/message-processor/models/message.go` — `Pronouns` (json:pronouns,omitempty) and `TwitchUsername` (json:"-") fields on UserInfo
  - `services/message-processor/enricher/viewer_badge_enricher.go` — Extended DB query with LEFT JOINs to resolve linked Twitch username
  - `services/message-processor/cmd/main.go` — Wired `pronounEnricher.Enrich` after `viewerBadgeEnricher.Enrich` in CHAT PATH only

- **Cache**:
  - Key pattern: `pronoun:{twitch_login}`
  - TTL: 24 hours
  - Sentinel: empty string `""` cached for users with no pronouns (404 response)

- **API**:
  - Pronoun definitions: `GET https://api.pronouns.alejo.io/v1/pronouns` — fetched once at startup
  - User lookup: `GET https://api.pronouns.alejo.io/v1/users/{twitch_login}`
  - HTTP timeout: 3 seconds

- **Cross-platform resolution**: Non-Twitch messages use `msg.User.TwitchUsername` (set by ViewerBadgeEnricher via new `COALESCE(twitch_vs.username, '') AS twitch_username` JOIN). This field is tagged `json:"-"` and never serialized to clients.

- **Timeline**: Phase 9, Plan 01 — 2026-04-04

---

## Related Decisions

- **ADR-0002**: Redis Streams + Pub/Sub — enrichers operate in the message-processor pipeline before Pub/Sub publish
- **ADR-0004**: No hexagonal architecture — enrichers are direct domain logic without ports/adapters
- **Phase 9 CONTEXT.md**: D-05 (silent fail), D-12 (TwitchUsername json:"-"), pronoun display requirements
