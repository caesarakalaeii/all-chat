# ADR-0010: Pronoun Enricher via Alejo API

**Date**: 2026-04-04
**Status**: Accepted
**Revised**: 2026-11-03
**Deciders**: Enrichment team (Phase 9)

## Context

Overlays are expected to show pronouns next to usernames. The community standard for Twitch pronoun display is the Alejo pronouns API (api.pronouns.alejo.io), data users opt into at pr.alejo.io. Integration must not degrade message delivery when the third-party API fails, and must not hammer it (no SLA).

## Decision

Add a `PronounEnricher` in the message-processor that resolves pronouns from the Alejo API with Redis caching and silent failure: an API failure skips pronouns for that message and never affects delivery.

- Cache `pronoun:{twitch_login}`, TTL 24h for hits and 404s; API failures negative-cache the empty sentinel for 5 minutes so a degraded Alejo API does not collapse throughput.
- Pronoun definitions fetched once at startup (`GET /v1/pronouns`); user lookup `GET /v1/users/{twitch_login}`, 3s HTTP timeout.
- Lookup runs only for overlays with pronoun display on (`ShowPronouns` from `display_settings.show_pronouns` / `visual_settings.showPronouns`, default on; `services/message-processor/router/overlay_router.go`), so overlays with it off send nothing to the API.
- Cross-platform: non-Twitch viewers resolved via linked Twitch account, `TwitchUsername` added by `ViewerBadgeEnricher`'s JOIN; the field is `json:"-"` and never serialized to clients.
- Pronoun changes on pr.alejo.io take up to 24h to propagate (cache TTL; accepted).

Alternatives rejected: storing pronouns in our own database (would need a user-facing settings feature and re-registration away from the established community standard); no pronoun support (missing community-expected feature).

## Implementation

- `services/message-processor/enricher/pronoun_enricher.go` (+ tests: `pronoun_enricher_test.go`, `pronoun_enricher_negcache_test.go`), wired into the chat path in `services/message-processor/cmd/main.go` after `viewerBadgeEnricher`.
- Fields: `Pronouns` (`json:"pronouns,omitempty"`) and `TwitchUsername` (`json:"-"`) on `UserInfo` in `services/message-processor/models/message.go`.

Related: ADR-0002 (enrichers run before the Pub/Sub publish).
