# Phase 9: Alejo Pronouns Integration - Context

**Gathered:** 2026-04-03
**Status:** Ready for planning

<domain>
## Phase Boundary

Integrate the Alejo pronouns API (pr.alejo.io) into the message processing pipeline so that pronouns display as a badge-style pill next to usernames in chat overlays. Per-overlay toggle and position/color settings control visibility. Cached pronoun lookups with 24h TTL. Cross-platform support via viewer identity resolution (linked Twitch accounts).

</domain>

<decisions>
## Implementation Decisions

### Pronoun Display Style
- **D-01:** Pronouns render as a badge-style pill (small colored pill/tag like badges, e.g. [she/her] with background color)
- **D-02:** Pill position is user-configurable per overlay: "before username" or "after username" (stored in display_settings as `pronoun_position`)
- **D-03:** Pill color is configurable per overlay (stored in display_settings as `pronoun_color`)

### Caching Strategy
- **D-04:** Pronoun lookups cached in Redis with 24-hour TTL (matching avatar enricher pattern)
- **D-05:** When Alejo API is unreachable or returns an error, silently skip pronouns — message renders without pronoun pill, no visible error
- **D-06:** Cache key prefix pattern: `pronoun:{twitch_username}` (lowercase)

### Per-Overlay Toggle UX
- **D-07:** `show_pronouns` enabled by default for new overlays
- **D-08:** Toggle, position selector, and color picker live in the existing VisibilityGroup component (frontend/src/components/appearance/VisibilityGroup.tsx), next to show_badges/show_avatars/show_platform_badge
- **D-09:** New display_settings fields: `show_pronouns` (bool), `pronoun_position` ("before" | "after"), `pronoun_color` (hex string)

### Platform Scope
- **D-10:** Alejo API lookups use Twitch usernames. For Twitch messages, use the message username directly.
- **D-11:** For non-Twitch messages (YouTube, Kick, TikTok, Discord): piggyback on the existing viewer identity resolution from viewer_badge_enricher. If a registered viewer has a linked Twitch account, use that Twitch username for the Alejo lookup. If no Twitch link exists, skip pronoun lookup.
- **D-12:** No extra DB queries for cross-platform resolution — reuse the viewer identity data already fetched by viewer_badge_enricher in the enrichment pipeline.

### Claude's Discretion
- Enricher ordering in the pipeline (where pronoun enricher runs relative to other enrichers)
- Redis cache key format details beyond the prefix
- Alejo API HTTP client configuration (timeouts, retries)
- Pronoun pill CSS styling specifics (border radius, font size, padding)
- Default pronoun pill color value
- How to pass resolved Twitch username from viewer_badge_enricher to pronoun_enricher (shared context, field on message, etc.)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Message Processing Pipeline
- `services/message-processor/enricher/avatar_enricher.go` — Enricher pattern: HTTP fetch + Redis cache + TTL, reference implementation for pronoun enricher
- `services/message-processor/enricher/viewer_badge_enricher.go` — Cross-platform viewer identity resolution, pronoun enricher piggybacks on this
- `services/message-processor/enricher/badge_enricher.go` — Badge enricher pattern
- `services/message-processor/models/message.go` — UnifiedChatMessage and UserInfo structs, where `pronouns` field will be added
- `services/message-processor/cmd/main.go` — Enricher pipeline wiring

### Overlay Configuration
- `services/overlay-manager/models/config.go` — OverlayConfig model with DisplaySettings map
- `services/overlay-manager/handlers/config.go` — Config CRUD endpoints
- `services/overlay-manager/repository/config_repo.go` — Config persistence

### Frontend Types & Rendering
- `frontend/src/lib/types/message.ts` — ChatMessage, UserInfo, Badge types
- `frontend/src/lib/types/overlay.ts` — DisplaySettings interface with existing toggle fields
- `frontend/src/app/overlay/[id]/page.tsx` — OBS overlay page that renders messages with username/badges/color
- `frontend/src/components/appearance/VisibilityGroup.tsx` — Where pronoun toggle/settings will be added

### Architecture Decisions
- `docs/adr/README.md` — ADR index for any new architectural decisions

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `AvatarEnricher` (avatar_enricher.go): HTTP client + Redis cache pattern with TTL — directly reusable for pronoun enricher structure
- `ViewerBadgeEnricher` (viewer_badge_enricher.go): Cross-platform viewer identity resolution with `viewerIdentityCache` struct — pronoun enricher can read resolved Twitch username from this
- `VisibilityGroup.tsx`: Existing component with show_badges, show_avatars, show_platform_badge toggles — pronoun toggle slots in naturally
- `DisplaySettings` interface: Already has boolean toggles and string config fields — extend with pronoun fields

### Established Patterns
- Enrichers are constructed in cmd/main.go and called sequentially in the message handler
- Redis cache uses prefix:key pattern with TTL (e.g. `avatar:{user_id}`, `viewer:identity:{platform}:{user_id}`)
- Frontend DisplaySettings fields are optional with sensible defaults
- Overlay config is a JSONB column — no migration needed for new display_settings fields

### Integration Points
- message-processor cmd/main.go: Wire new PronounEnricher into enrichment pipeline after ViewerBadgeEnricher
- UserInfo struct: Add `Pronouns string` field
- Frontend ChatMessage UserInfo: Add `pronouns?: string` field
- Overlay page renderer: Add pronoun pill rendering next to username
- VisibilityGroup: Add toggle + position + color controls

</code_context>

<specifics>
## Specific Ideas

- Alejo API endpoint: `https://pr.alejo.io/v1/pronoun` (exact endpoint to be confirmed during research)
- Pronoun pill should look like existing badge pills but with configurable background color
- Cross-platform resolution: viewer_badge_enricher already queries `viewer_platform_links` table — pronoun enricher should consume this data, not re-query

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 09-add-optional-support-for-alejo-pronouns-pr-alejo-io-integrate-the-pronouns-api-add-per-overlay-toggle-to-enable-disable-pronoun-display-cache-pronoun-lookups-and-render-pronouns-next-to-usernames-in-the-chat-overlay*
*Context gathered: 2026-04-03*
