# Phase 9: Alejo Pronouns Integration — Research

**Researched:** 2026-04-04
**Domain:** API enrichment (Go), Redis caching, frontend overlay rendering, TypeScript UI controls
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** Pronouns render as a badge-style pill (small colored pill/tag like badges, e.g. [she/her] with background color)
- **D-02:** Pill position is user-configurable per overlay: "before username" or "after username" (stored in display_settings as `pronoun_position`)
- **D-03:** Pill color is configurable per overlay (stored in display_settings as `pronoun_color`)
- **D-04:** Pronoun lookups cached in Redis with 24-hour TTL (matching avatar enricher pattern)
- **D-05:** When Alejo API is unreachable or returns an error, silently skip pronouns — message renders without pronoun pill, no visible error
- **D-06:** Cache key prefix pattern: `pronoun:{twitch_username}` (lowercase)
- **D-07:** `show_pronouns` enabled by default for new overlays
- **D-08:** Toggle, position selector, and color picker live in the existing VisibilityGroup component (frontend/src/components/appearance/VisibilityGroup.tsx), next to show_badges/show_avatars/show_platform_badge
- **D-09:** New display_settings fields: `show_pronouns` (bool), `pronoun_position` ("before" | "after"), `pronoun_color` (hex string)
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

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

---

## Summary

This phase adds Alejo pronouns (pr.alejo.io) to the message enrichment pipeline and renders them as a pill next to usernames in the chat overlay. The work spans three layers: (1) a new `PronounEnricher` in `services/message-processor/enricher/` following the exact `AvatarEnricher` pattern, (2) a `UserInfo.Pronouns` field propagated from Go through the WebSocket to the frontend, and (3) overlay rendering + appearance panel UI controls.

The Alejo API is live at `https://api.pronouns.alejo.io/v1/`. A GET to `/v1/users/{twitch_login}` returns a single JSON object (`channel_id`, `channel_login`, `pronoun_id`, `alt_pronoun_id`). A GET to `/v1/pronouns` returns the full map of pronoun IDs to display strings. The API returns HTTP 404 when a user has no pronouns set. The server sets `Cache-Control: max-age=3600` and `Access-Control-Allow-Origin: *`. No authentication is required.

The cross-platform pronoun lookup (D-11/D-12) is the trickiest design point: `viewer_platform_identities` stores numeric Twitch IDs, but the Alejo API requires Twitch login names (e.g. `pajlada`, not `11148817`). The Twitch login is available in `viewer_sessions.username` (confirmed: auth-service stores `platform_user_id = numeric ID` and `username = login name`). The `ViewerBadgeEnricher` DB query must be extended with a lateral join to also fetch the linked Twitch login in the same query, caching it in `viewerIdentityCache.TwitchUsername`. The `PronounEnricher` then reads `msg.User.TwitchUsername` (a new field on `UserInfo`, omitted from JSON output to clients) rather than issuing its own DB query.

**Primary recommendation:** Implement in three sequential work units — (1) Go enricher + model field, (2) frontend type extensions + overlay rendering, (3) VisibilityGroup UI controls.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `net/http` (stdlib) | Go 1.25 | HTTP client for Alejo API | Already used by AvatarEnricher |
| `github.com/redis/go-redis/v9` | v9.18.0 | Redis cache for pronoun TTL | Already in go.mod |
| `github.com/alicebob/miniredis/v2` | v2.37.0 | In-process Redis for enricher tests | Already in go.mod as direct dep |

### Frontend

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| React (existing) | 19+ | Pronoun pill rendering | Project standard |
| Tailwind CSS (existing) | existing | Pill styling (`rounded-full`, `text-[11px]`) | Project standard |
| `ColorPickerControl` (existing) | local | Color picker for `pronoun_color` | Already used in ColorsGroup |
| `ToggleSwitch` (existing) | local | `show_pronouns` toggle | Already used in VisibilityGroup |

No new dependencies required for this phase.

---

## Architecture Patterns

### Recommended Project Structure

```
services/message-processor/
├── enricher/
│   ├── pronoun_enricher.go          # NEW
│   └── pronoun_enricher_test.go     # NEW
├── models/
│   └── message.go                   # MODIFY: add Pronouns + TwitchUsername to UserInfo
└── cmd/main.go                      # MODIFY: wire PronounEnricher after ViewerBadgeEnricher

frontend/src/
├── lib/types/
│   ├── message.ts                   # MODIFY: pronouns? field on UserInfo
│   ├── overlay.ts                   # MODIFY: pronoun_* fields on DisplaySettings
│   └── visual-settings.ts           # MODIFY: showPronouns, pronounPosition, pronounColor
├── app/overlay/[id]/
│   └── page.tsx                     # MODIFY: render pill, load pronoun config
└── components/appearance/
    └── VisibilityGroup.tsx          # MODIFY: pronoun toggle + position + color picker
```

### Pattern 1: AvatarEnricher Structure (reference implementation)

The `PronounEnricher` struct should mirror `AvatarEnricher` exactly:

```go
// Source: services/message-processor/enricher/avatar_enricher.go
type PronounEnricher struct {
    httpClient  *http.Client   // Timeout: 3s (faster than avatar; smaller payload)
    redisClient *redis.Client
    logger      *zap.Logger
}

func NewPronounEnricher(redisClient *redis.Client, logger *zap.Logger) *PronounEnricher {
    return &PronounEnricher{
        httpClient:  &http.Client{Timeout: 3 * time.Second},
        redisClient: redisClient,
        logger:      logger,
    }
}

func (e *PronounEnricher) Enrich(ctx context.Context, msg *models.UnifiedChatMessage) error {
    // 1. Determine twitch username to look up
    twitchUsername := resolveTwitchUsername(msg) // returns "" if unavailable
    if twitchUsername == "" {
        return nil
    }

    // 2. Cache check
    cacheKey := fmt.Sprintf("pronoun:%s", strings.ToLower(twitchUsername))
    cached, err := e.redisClient.Get(ctx, cacheKey).Result()
    if err == nil {
        // "" sentinel means no pronouns set; skip
        if cached != "" {
            msg.User.Pronouns = cached
        }
        return nil
    }

    // 3. Fetch from API
    pronounText, err := e.fetchPronoun(ctx, twitchUsername)
    if err != nil {
        e.logger.Warn("PronounEnricher: API fetch failed, skipping",
            zap.String("username", twitchUsername),
            zap.Error(err),
        )
        return nil  // D-05: silent skip
    }

    // 4. Cache result (including empty sentinel for "no pronouns")
    e.redisClient.Set(ctx, cacheKey, pronounText, 24*time.Hour)
    if pronounText != "" {
        msg.User.Pronouns = pronounText
    }
    return nil
}
```

### Pattern 2: Alejo API Response Structure

**Verified via live API call on 2026-04-04:**

```
GET https://api.pronouns.alejo.io/v1/users/{twitch_login}
→ 200: {"channel_id":"11148817","channel_login":"pajlada","pronoun_id":"hehim","alt_pronoun_id":null}
→ 404: user has no pronouns set

GET https://api.pronouns.alejo.io/v1/pronouns
→ 200: {"hehim":{"name":"hehim","subject":"He","object":"Him","singular":false}, ...}
```

The display string to show in the pill should be constructed from `pronoun_id` and optional `alt_pronoun_id`:
- `pronoun_id = "sheher"` → look up in pronouns map → `"she/her"` (subject + "/" + object, all lowercase)
- `alt_pronoun_id = "theythem"` → display as `"she/they"` (primary subject + "/" + secondary subject)
- When `alt_pronoun_id` is null: display primary only → `"she/her"`

The pronouns map construction: `subject.ToLower() + "/" + object.ToLower()`. This matches what the pr.alejo.io browser extension uses.

**Alternative base URL confirmed:** `https://pronouns.alejo.io/api/users/{login}` also works (older endpoint, returns array format). Use `api.pronouns.alejo.io/v1/` — it is the canonical current API and returns the richer object format.

### Pattern 3: Cross-Platform Twitch Username Resolution

The `viewer_badge_enricher.go` DB query must be extended once to add a `LEFT JOIN` that retrieves the linked Twitch login for non-Twitch viewers. This satisfies D-12 (no extra queries):

```sql
-- Extension to existing query in viewer_badge_enricher.go
-- Add to the SELECT list:
COALESCE(twitch_vs.username, '') AS twitch_username

-- Add these JOINs after the existing JOINs:
LEFT JOIN viewer_platform_identities twitch_vpi
    ON twitch_vpi.viewer_id = vpi.viewer_id AND twitch_vpi.platform = 'twitch'
LEFT JOIN viewer_sessions twitch_vs
    ON twitch_vs.platform = 'twitch' AND twitch_vs.platform_user_id = twitch_vpi.platform_user_id
```

The `viewerIdentityCache` struct gets a new `TwitchUsername string` field (`json:"twitch_username,omitempty"`).

The `UnifiedChatMessage.UserInfo` struct gets a new `TwitchUsername string` field tagged `json:"-"` — it must NOT be serialized to clients; it is pipeline-internal only.

The `PronounEnricher.resolveTwitchUsername` function:
```go
func resolveTwitchUsername(msg *models.UnifiedChatMessage) string {
    if msg.Platform == "twitch" {
        return strings.ToLower(msg.User.Username)
    }
    return strings.ToLower(msg.User.TwitchUsername) // populated by ViewerBadgeEnricher
}
```

### Pattern 4: Enricher Pipeline Wiring (cmd/main.go)

```go
// After viewerBadgeEnricher construction (line ~174):
pronounEnricher := enricher.NewPronounEnricher(redisClient, log)

// In messageHandler — CHAT PATH, after viewerBadgeEnricher.Enrich():
if err := pronounEnricher.Enrich(ctx, unified); err != nil {
    log.Warn("Failed to enrich pronouns",
        zap.String("message_id", rawMsg.MessageID),
        zap.Error(err),
    )
    // Continue even if enrichment fails — D-05
}
```

Ordering rationale: `PronounEnricher` runs after `ViewerBadgeEnricher` because it reads `msg.User.TwitchUsername` which `ViewerBadgeEnricher` populates. It does NOT run for events — pronoun enrichment is chat messages only (events have user-authored text but no persistent identity for pronoun display purposes).

### Pattern 5: Frontend Config Loading (overlay page)

The overlay page (`page.tsx`) already loads `display_settings` from `/api/v1/overlays/public/${id}/config`. Add three new state variables mirroring existing `platformBadgePosition`/`platformBadgeStyle` pattern:

```typescript
const [showPronouns, setShowPronouns] = useState(true)   // D-07: default on
const [pronounPosition, setPronounPosition] = useState<'before' | 'after'>('after')  // UI-SPEC default
const [pronounColor, setPronounColor] = useState('#7B68EE')  // UI-SPEC default

// In loadConfig():
if (typeof display.show_pronouns === 'boolean') {
  setShowPronouns(display.show_pronouns)
}
if (display.pronoun_position === 'before' || display.pronoun_position === 'after') {
  setPronounPosition(display.pronoun_position)
}
if (typeof display.pronoun_color === 'string' && display.pronoun_color) {
  setPronounColor(display.pronoun_color)
}
```

### Pattern 6: Pronoun Pill Rendering (UI-SPEC confirmed)

From `09-UI-SPEC.md` (approved):

```tsx
{/* Pronoun pill — mirrors platformBadgePosition conditional */}
{showPronouns && msg.user.pronouns && pronounPosition === 'before' && (
  <span
    className="inline-flex items-center rounded-full px-2 py-1 text-[11px] font-semibold leading-none text-white"
    style={{ backgroundColor: pronounColor }}
  >
    {msg.user.pronouns}
  </span>
)}
{/* username block here */}
{showPronouns && msg.user.pronouns && pronounPosition === 'after' && (
  <span
    className="inline-flex items-center rounded-full px-2 py-1 text-[11px] font-semibold leading-none text-white"
    style={{ backgroundColor: pronounColor }}
  >
    {msg.user.pronouns}
  </span>
)}
```

### Pattern 7: VisibilityGroup Extension

The VisibilityGroup component uses `VisualSettings` (camelCase CSS-driven fields), not the raw `DisplaySettings` (snake_case API fields). New fields required in `VisualSettings`:

```typescript
// visual-settings.ts
showPronouns?: 'inline' | 'none'      // mirrors showPlatformBadge pattern
pronounPosition?: 'before' | 'after'  // non-CSS, stored for persistence
pronounColor?: string                  // non-CSS, stored for persistence
```

The toggle maps to `showPronouns: 'inline' | 'none'` (same as `showBadges`, `showAvatars`). However, the overlay page reads `display_settings` directly (snake_case), not `visual_settings`. The appearance panel saves through `visual_settings` (which maps to `VisualSettings`). The overlay page already handles both — it reads `visual_settings.showPlatformBadge` as override over `display_settings.show_platform_badge`. The pronoun fields should follow the same override pattern.

### Anti-Patterns to Avoid

- **Making the pronouns field non-optional on `UserInfo`:** `Pronouns` must be `omitempty` in Go and `pronouns?: string` in TypeScript — most messages will have no pronouns.
- **Caching "user not found" (404) with full TTL:** Cache 404s with the same 24h TTL using an empty string sentinel (`""` = no pronouns). This prevents hammering the API for non-Twitch-registered users.
- **Skipping the `TwitchUsername` sentinel check:** If `TwitchUsername == ""` after viewer_badge enrichment, it means the viewer is not registered or has no linked Twitch account. Must not issue an Alejo API call in this case.
- **Enriching events with pronouns:** Pronoun enrichment applies to chat messages only. Events (subscriptions, bits, raids) should skip pronoun enrichment — the enricher ordering handles this if pronounEnricher.Enrich is only called in the CHAT PATH of `cmd/main.go`.
- **Sending `TwitchUsername` to clients:** The `TwitchUsername` field on `UserInfo` must be tagged `json:"-"` in Go and must NOT appear in the frontend `UserInfo` type — it is internal pipeline state.
- **Confusing `api.pronouns.alejo.io` with `pr.alejo.io`:** `pr.alejo.io` is the frontend app (returns HTML). `api.pronouns.alejo.io/v1/` is the JSON API (confirmed live).

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Pronoun display text | Custom ID→text map | Live fetch from `/v1/pronouns` + cache | API may add new pronoun IDs; hardcoded map goes stale |
| HTTP retry | Custom retry loop | Simple timeout + silent-skip on error | D-05 mandates silent skip; retry adds latency |
| Cache serialization | Custom binary format | Plain string (pronoun text) + empty sentinel | Pronoun is a single short string, no struct needed |
| Cross-platform lookup | Extra DB query per message | Extend existing ViewerBadgeEnricher query | D-12 explicit — no extra DB roundtrips |

**Key insight:** The pronoun cache value is the final display string (e.g. `"she/her"`), not the raw `pronoun_id`. This means display text construction happens once at enrichment time, not at render time. The cache is simple `string → string` with no JSON serialization overhead.

---

## Runtime State Inventory

> This is a greenfield feature addition with no renames or migrations. No runtime state inventory required.

None — verified. No existing data structures use "pronouns" as a key. The overlay display_settings JSONB column accepts new fields transparently (no migration). The Redis namespace `pronoun:*` is currently empty.

---

## Common Pitfalls

### Pitfall 1: Alejo API Uses Twitch Login, Not Numeric ID

**What goes wrong:** Calling `api.pronouns.alejo.io/v1/users/11148817` (numeric ID) returns 404 even if the user has pronouns set. Correct: `api.pronouns.alejo.io/v1/users/pajlada` (login name, lowercase).

**Why it happens:** `viewer_platform_identities` stores `platform_user_id = numeric Twitch ID` (confirmed from auth-service). The Alejo API is keyed by Twitch login (channel_login).

**How to avoid:** For Twitch messages, use `msg.User.Username` (the chat IRC username — this is already the Twitch login). For cross-platform, retrieve from `viewer_sessions.username` via the extended DB query. Always lowercase before caching and before API call.

**Warning signs:** All lookups returning 404 even for known streamers.

### Pitfall 2: `pronouns.alejo.io` vs `api.pronouns.alejo.io`

**What goes wrong:** Using `pronouns.alejo.io/api/users/{login}` (the legacy endpoint) works but returns an array format `[{"id":...,"login":...,"pronoun_id":...}]` without `alt_pronoun_id`. The new endpoint `api.pronouns.alejo.io/v1/users/{login}` returns the object format with `alt_pronoun_id` support.

**How to avoid:** Always use `api.pronouns.alejo.io/v1/` as the base URL.

### Pitfall 3: Empty String vs Null in Cache

**What goes wrong:** If the Redis GET returns `redis.Nil` (cache miss), skip the cache entirely. If it returns `""` (empty string sentinel), it means "user exists but has no pronouns" — don't make an API call. If it returns a non-empty string, that's the display text.

**How to avoid:** Three-way check: `redis.Nil` → fetch, `""` → skip, non-empty → use.

### Pitfall 4: TwitchUsername in JSON Output

**What goes wrong:** If `TwitchUsername` is accidentally serialized into the WebSocket `UnifiedChatMessage`, clients receive internal DB data they shouldn't see.

**How to avoid:** Tag the field `json:"-"` in the Go struct. Do not add it to the frontend `UserInfo` TypeScript type.

### Pitfall 5: VisualSettings vs DisplaySettings Confusion

**What goes wrong:** The appearance panel uses `VisualSettings` (camelCase, goes to `visual_settings` JSONB). The overlay page reads `display_settings` for backward compatibility. Both must be handled.

**How to avoid:** Follow the existing `showPlatformBadge` / `show_platform_badge` dual-path pattern in `page.tsx` — check `visual_settings.showPronouns` as override, fall back to `display_settings.show_pronouns`. The appearance panel should save via `visualSettings.showPronouns`, which goes through the VisualSettings path.

### Pitfall 6: Pronouns Enrichment on Events

**What goes wrong:** If `pronounEnricher.Enrich` is accidentally called in the EVENT PATH (before `goto publish`), it may set pronouns on event messages like "300 bits" or raids, where the username row may not render with a pill at all.

**How to avoid:** Wire pronounEnricher only in the CHAT PATH (the `else` branch in `cmd/main.go`), after `viewerBadgeEnricher.Enrich`. Do not add it to the event enrichment sequence.

### Pitfall 7: fakeViewerDB Test Double Needs Updating

**What goes wrong:** The existing `viewer_badge_enricher_test.go` uses `fakeViewerDB.queryFn` that returns a fixed number of columns. Adding `twitch_username` to the Scan arguments will break existing tests unless the fake is updated.

**How to avoid:** Update `fakeViewerDB.queryResult` and `fakeRow.Scan` to include the new `twitch_username` string column (8th positional arg). All existing tests can return `""` for this field.

---

## Code Examples

### Alejo API — Live Verified Response Shapes

```json
// GET https://api.pronouns.alejo.io/v1/users/pajlada
// Source: live probe 2026-04-04
{
  "channel_id": "11148817",
  "channel_login": "pajlada",
  "pronoun_id": "hehim",
  "alt_pronoun_id": null
}

// GET https://api.pronouns.alejo.io/v1/pronouns
// Source: live probe 2026-04-04
{
  "hehim":    {"name":"hehim",    "subject":"He",   "object":"Him",  "singular":false},
  "sheher":   {"name":"sheher",   "subject":"She",  "object":"Her",  "singular":true},
  "theythem": {"name":"theythem", "subject":"They", "object":"Them", "singular":false},
  ...
  "any":      {"name":"any",      "subject":"Any",  "object":"Any",  "singular":true},
  "other":    {"name":"other",    "subject":"Other","object":"Other","singular":true}
}

// GET https://api.pronouns.alejo.io/v1/users/{unknown_user}
// → HTTP 404 (empty body or error JSON)
```

### Display Text Construction

```go
// From pronoun_id + alt_pronoun_id:
func buildDisplayText(primaryID, altID string, pronounsMap map[string]PronounDef) string {
    primary, ok := pronounsMap[primaryID]
    if !ok {
        return primaryID // fallback: use ID as-is
    }
    subject := strings.ToLower(primary.Subject)
    object := strings.ToLower(primary.Object)
    if altID != "" && altID != "null" {
        if alt, ok := pronounsMap[altID]; ok {
            return subject + "/" + strings.ToLower(alt.Subject)
        }
    }
    if primary.Singular {
        return subject // "any", "other" display as single word
    }
    return subject + "/" + object
}
// Examples:
// hehim + null       → "he/him"
// sheher + theythem  → "she/they"
// any + null         → "any"
```

### Extended ViewerBadgeEnricher DB Query

```sql
-- Extended query in viewer_badge_enricher.go (append to existing SELECT + JOINs)
-- Additional SELECT column:
COALESCE(twitch_vs.username, '') AS twitch_username

-- Additional JOINs (after existing LEFT JOIN LATERAL ... ON true):
LEFT JOIN viewer_platform_identities twitch_vpi
    ON twitch_vpi.viewer_id = vpi.viewer_id AND twitch_vpi.platform = 'twitch'
LEFT JOIN viewer_sessions twitch_vs
    ON twitch_vs.platform = 'twitch'
    AND twitch_vs.platform_user_id = twitch_vpi.platform_user_id
```

### UserInfo Struct Extensions (Go)

```go
// services/message-processor/models/message.go — add to UserInfo struct
type UserInfo struct {
    // ... existing fields ...
    Pronouns      string `json:"pronouns,omitempty"`   // display text e.g. "she/her"
    TwitchUsername string `json:"-"`                   // INTERNAL: for cross-platform pronoun lookup
}
```

### viewerIdentityCache Extension

```go
// services/message-processor/enricher/viewer_badge_enricher.go
type viewerIdentityCache struct {
    // ... existing fields ...
    TwitchUsername string `json:"twitch_username,omitempty"` // Phase 9: for pronoun lookup
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Hardcoded pronoun list | Live `/v1/pronouns` API | Alejo API v1 | Supports new pronoun IDs without code changes |
| Single-endpoint API | Dual endpoints (v1 object + legacy array) | ~2022 | Use v1 object format for alt_pronoun_id support |

---

## Open Questions

1. **Pronoun enrichment on events**
   - What we know: CONTEXT.md does not explicitly address events
   - What's unclear: Should Sub/Bits/Raid events show pronouns on the username?
   - Recommendation: Skip for events (only chat path). The event username row is styled differently and pronouns would be visually inconsistent. No explicit decision — use discretion.

2. **Pronouns map caching strategy**
   - What we know: The `/v1/pronouns` endpoint is stable (12 pronoun IDs, rarely changes)
   - What's unclear: Should the pronouns map be fetched once at startup or per-lookup?
   - Recommendation: Fetch once at startup and cache in-memory (package-level map initialized in `NewPronounEnricher`). The map is ~1KB, changes rarely, and a restart picks up changes. Do not cache in Redis — it adds serialization overhead for no benefit on a static reference dataset.

3. **`alt_pronoun_id` display format consensus**
   - What we know: No explicit decision in CONTEXT.md; API returns it
   - What's unclear: Display as `"she/they"` (subject+subject) or `"she/they/her/them"` (verbose)?
   - Recommendation: `"she/they"` format (primary_subject + "/" + alt_subject). This is the format used by the pr.alejo.io browser extension and matches community expectation.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `api.pronouns.alejo.io` | PronounEnricher HTTP client | ✓ | Live (no versioning) | D-05: silent skip on error |
| Redis | Pronoun cache | ✓ | Existing in-cluster | — |
| PostgreSQL | Extended viewer identity query | ✓ | Existing in-cluster | — |

**Missing dependencies with no fallback:** None.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework (Go) | `testing` stdlib + `testify` v1.11.1 + `miniredis` v2.37.0 |
| Framework (Frontend) | Vitest (vitest.config.ts exists) |
| Config file | `services/message-processor/go.mod`, `frontend/vitest.config.ts` |
| Quick run command (Go) | `cd services/message-processor && go test ./enricher/... -run TestPronoun -v` |
| Full suite command (Go) | `cd services/message-processor && go test ./... -v` |

### Phase Requirements → Test Map

| Behavior | Test Type | Automated Command | File Exists? |
|----------|-----------|-------------------|-------------|
| PronounEnricher: Twitch message gets pronouns from API | unit | `go test ./enricher/... -run TestPronounEnricher_Twitch` | Wave 0 |
| PronounEnricher: Non-Twitch message with linked Twitch account gets pronouns | unit | `go test ./enricher/... -run TestPronounEnricher_CrossPlatform` | Wave 0 |
| PronounEnricher: Non-Twitch message with no Twitch link skips quietly | unit | `go test ./enricher/... -run TestPronounEnricher_NoTwitchLink` | Wave 0 |
| PronounEnricher: API 404 returns no pronouns, caches empty sentinel | unit | `go test ./enricher/... -run TestPronounEnricher_NotFound` | Wave 0 |
| PronounEnricher: API error silently skips (D-05) | unit | `go test ./enricher/... -run TestPronounEnricher_APIError` | Wave 0 |
| PronounEnricher: Cache hit returns without API call | unit | `go test ./enricher/... -run TestPronounEnricher_CacheHit` | Wave 0 |
| ViewerBadgeEnricher: TwitchUsername populated for non-Twitch viewer with Twitch link | unit | `go test ./enricher/... -run TestViewerBadgeEnricher_TwitchUsername` | Wave 0 |
| Frontend: pronoun pill renders before/after username per config | unit (Vitest) | `cd frontend && npm test -- --run pronoun` | Wave 0 |

### Sampling Rate

- **Per task commit:** `cd services/message-processor && go test ./enricher/... -v`
- **Per wave merge:** `cd services/message-processor && go test ./... -v`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `services/message-processor/enricher/pronoun_enricher_test.go` — all 6 enricher unit tests above
- [ ] Extend `services/message-processor/enricher/viewer_badge_enricher_test.go` — add TwitchUsername to fakeViewerDB and add test for `TestViewerBadgeEnricher_TwitchUsername`
- [ ] `frontend/src/components/appearance/__tests__/VisibilityGroup.test.tsx` — pronoun toggle/position/color controls (if not already exists; check before creating)

---

## Sources

### Primary (HIGH confidence)

- Live API probe: `https://api.pronouns.alejo.io/v1/users/pajlada` — confirmed JSON response format, HTTP status codes, CORS headers (2026-04-04)
- Live API probe: `https://api.pronouns.alejo.io/v1/pronouns` — confirmed full pronouns map, all 12 IDs (2026-04-04)
- `services/message-processor/enricher/avatar_enricher.go` — reference enricher pattern (HTTP client + Redis cache + TTL + silent skip)
- `services/message-processor/enricher/viewer_badge_enricher.go` — cross-platform viewer identity resolution, `viewerIdentityCache` struct, DB query structure
- `services/message-processor/cmd/main.go` — enricher pipeline wiring, ordering of existing enrichers
- `services/message-processor/models/message.go` — `UserInfo` struct, `UnifiedChatMessage` struct
- `services/overlay-manager/models/config.go` — `OverlayConfig.DisplaySettings map[string]any` (JSONB, no migration needed)
- `frontend/src/lib/types/overlay.ts` — `DisplaySettings` interface pattern
- `frontend/src/lib/types/visual-settings.ts` — `VisualSettings` interface, inline/none toggle pattern
- `frontend/src/components/appearance/VisibilityGroup.tsx` — exact integration point, existing RadioGroup, disabled state pattern
- `frontend/src/app/overlay/[id]/page.tsx` — config loading pattern, state variables, rendering pattern
- `.planning/phases/09-.../09-UI-SPEC.md` — approved UI design contract with exact CSS classes
- `migrations/035_viewer_identity.sql` — `viewer_platform_identities` schema
- `migrations/011_viewer_authentication.sql` — `viewer_sessions.username` column confirmed
- `services/auth-service/handlers/platform_auth_v2.go` line 870 — confirmed `username` = Twitch login, `GetID()` = numeric ID

### Secondary (MEDIUM confidence)

- `pronouns.alejo.io` legacy endpoint also confirmed live; use v1 API instead

---

## Metadata

**Confidence breakdown:**
- Alejo API endpoints and response format: HIGH — verified by live probe
- Standard stack: HIGH — no new deps; all existing libraries
- Architecture (enricher pattern): HIGH — direct copy from AvatarEnricher
- Cross-platform resolution design: HIGH — confirmed DB schema and auth-service patterns
- Frontend patterns: HIGH — traced from VisibilityGroup + overlay page existing code
- Pitfalls: HIGH — derived from actual code examination

**Research date:** 2026-04-04
**Valid until:** 2026-07-04 (stable API; API.pronouns.alejo.io has been stable since ~2022)
