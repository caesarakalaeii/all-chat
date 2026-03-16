# Phase 28: Viewer Identity Foundation — Auth & Platform Linking - Context

**Gathered:** 2026-03-14
**Status:** Ready for planning

<domain>
## Phase Boundary

Establish the viewer account identity model (new DB tables), OAuth sign-in from the browser extension popup for Twitch + YouTube + Kick, inline name color picker in the popup, and a ViewerBadgeEnricher in message-processor that injects viewer name_color into processed messages. The website `/settings/viewer` route stub is included but the full cosmetics editor is Phase 29.

</domain>

<decisions>
## Implementation Decisions

### Viewer Identity Schema
- Unified model: `viewer_sessions` IS the viewer record — add `viewer_id` UUID FK to a new `viewers` table (id + created_at)
- New `viewer_platform_identities` table: maps (platform, platform_user_id) → viewer_id. One row per platform link.
- New `viewer_cosmetics` table: stores `name_color VARCHAR(7)` per viewer_id
- No account merging in Phase 28 — each sign-in via a platform creates its own viewer_id (or reuses existing if already linked). Multi-platform linking via multiple `viewer_platform_identities` rows is the future path (VID-04).
- Viewer JWT carries `viewer_id` (not session_id) as the durable cross-platform identity claim. JWT claims: viewer_id + platform + platform_user_id.

### Extension OAuth Flow
- Use `chrome.identity.launchWebAuthFlow` — Chrome's built-in OAuth API, no popup window or tab navigation needed
- Redirect URI is the extension's `chrome-extension://[id]/...` URL
- After getting the auth code, extension calls a backend code-exchange endpoint
- Auth-service callback handler refactored to handle both redirect (GET) and code-exchange (POST) modes — single endpoint, two modes
- Supported platforms in Phase 28: Twitch, YouTube, Kick

### Extension Popup — Platform Detection
- Content script detects current URL and writes platform to `chrome.storage.session` (twitch.tv → 'twitch', youtube.com → 'youtube', kick.com → 'kick')
- Popup reads `chrome.storage.session` on open to determine current platform
- On a streaming platform page: show only that platform's sign-in button (single, full-width, platform-colored)
- On a non-streaming tab: show all three platform sign-in buttons stacked vertically

### Extension Popup — Signed-in State Layout
- Top: viewer avatar + display name (from viewer JWT)
- Middle: "Name Color" label + inline `<input type="color">` swatch
- Bottom: "Open Settings" button (navigates to `/settings/viewer` on website) + "Sign Out" link
- Color save feedback: subtle inline "Saved ✓" text briefly appears next to the picker on successful PATCH — no toast, no modal

### Extension Popup — Color Save
- Color change triggers immediate PATCH `/api/viewer/cosmetics` with `{ name_color: "#rrggbb" }`
- Also saves to `chrome.storage.local` for offline access
- No explicit Save button — autosave on color input

### ViewerBadgeEnricher
- Enricher always overrides `UserInfo.Color` with viewer's stored `name_color` (if the viewer has one set) — viewer's All-Chat preference wins over platform color
- If viewer has no stored color (null/unset), pass through without modification
- Redis cache key: `viewer:identity:{platform}:{platform_user_id}` — value: JSON `{ viewer_id, name_color }` or `null` sentinel
- Cache TTL: 5 minutes
- Cache miss behavior: query `viewer_platform_identities` JOIN `viewer_cosmetics` from DB, populate cache, inject color if found
- Null sentinel cached to avoid DB thundering herd on unknown viewers
- Message-processor gets a PostgreSQL connection (shared DB with auth-service) for viewer lookup on cache miss

### Claude's Discretion
- Exact DB migration numbering (next after 013)
- Extension manifest.json permissions list (chrome.identity, storage, tabs, activeTab)
- Error handling for OAuth failure in extension (show error state in popup)
- `/settings/viewer` page content in Phase 28 (minimal stub — full editor is Phase 29)

</decisions>

<specifics>
## Specific Ideas

- Platform detection is context-aware: watching Twitch → show Twitch login only. Clean, single-purpose sign-in per context.
- The `chrome.identity.launchWebAuthFlow` approach keeps OAuth entirely within the extension — no tab navigation, no polling.
- Viewer JWT uses `viewer_id` as the primary identity — durable even if the platform session expires or tokens refresh.

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- `ViewerAuthHandler` (auth-service/handlers/viewer_auth.go): Full OAuth for Twitch/YouTube/Kick already implemented. Phase 28 refactors callback handlers to also accept POST code-exchange mode for the extension.
- `oauth.ViewerTwitchOAuth`, `oauth.ViewerYouTubeOAuth`, `oauth.ViewerKickOAuth`: All three providers exist and are wired in auth-service main.go. Phase 28 reuses them directly.
- `enricher/` package (message-processor): Existing `badge_enricher.go`, `emote_enricher.go`, `avatar_enricher.go` — `ViewerBadgeEnricher` follows same interface pattern.
- PLATFORM_COLORS map established in Phase 23/25 frontend: use same platform colors for extension popup sign-in buttons.
- Existing `viewer_sessions` table (migration 011): Phase 28 adds `viewer_id` FK via new migration.

### Established Patterns
- Redis key prefix pattern: domain:type:identifier (e.g., `viewer:identity:twitch:ucxxx`)
- JWT pattern: `ViewerJWTClaims` struct in models/viewer.go — Phase 28 adds `ViewerID uuid.UUID` claim
- Frontend settings page at `/settings` (frontend/src/app/settings/page.tsx): add `/settings/viewer` as a sub-route in same style

### Integration Points
- Message-processor enricher chain: `ViewerBadgeEnricher` added after badge/emote enrichers. Needs PostgreSQL connection injected at startup.
- API Gateway routes: new PATCH `/api/viewer/cosmetics` route needed (proxies to auth-service)
- Extension manifest: `chrome.identity` permission for `launchWebAuthFlow`, `storage` for JWT persistence, `activeTab`/`tabs` for platform detection

</code_context>

<deferred>
## Deferred Ideas

- Multi-platform account merging (signing in with Twitch + YouTube linking to same viewer_id) — future phase
- TikTok identity linking (VID-TK-01) — deferred per requirements (unofficial library, user ID stability unclear)
- Full cosmetics editor on `/settings/viewer` (gradient builder, frame/flair) — Phase 29/30

</deferred>

---

*Phase: 28-viewer-identity-foundation-auth-and-platform-linking*
*Context gathered: 2026-03-14*
