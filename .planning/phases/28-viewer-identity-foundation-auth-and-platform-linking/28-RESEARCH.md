# Phase 28: Viewer Identity Foundation — Auth & Platform Linking - Research

**Researched:** 2026-03-14
**Domain:** Chrome Extension OAuth, Go auth-service refactor, DB schema migration, message-processor enricher
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### Viewer Identity Schema
- Unified model: `viewer_sessions` IS the viewer record — add `viewer_id` UUID FK to a new `viewers` table (id + created_at)
- New `viewer_platform_identities` table: maps (platform, platform_user_id) → viewer_id. One row per platform link.
- New `viewer_cosmetics` table: stores `name_color VARCHAR(7)` per viewer_id
- No account merging in Phase 28 — each sign-in via a platform creates its own viewer_id (or reuses existing if already linked). Multi-platform linking via multiple `viewer_platform_identities` rows is the future path (VID-04).
- Viewer JWT carries `viewer_id` (not session_id) as the durable cross-platform identity claim. JWT claims: viewer_id + platform + platform_user_id.

#### Extension OAuth Flow
- Use `chrome.identity.launchWebAuthFlow` — Chrome's built-in OAuth API, no popup window or tab navigation needed
- Redirect URI is the extension's `chrome-extension://[id]/...` URL
- After getting the auth code, extension calls a backend code-exchange endpoint
- Auth-service callback handler refactored to handle both redirect (GET) and code-exchange (POST) modes — single endpoint, two modes
- Supported platforms in Phase 28: Twitch, YouTube, Kick

#### Extension Popup — Platform Detection
- Content script detects current URL and writes platform to `chrome.storage.session` (twitch.tv → 'twitch', youtube.com → 'youtube', kick.com → 'kick')
- Popup reads `chrome.storage.session` on open to determine current platform
- On a streaming platform page: show only that platform's sign-in button (single, full-width, platform-colored)
- On a non-streaming tab: show all three platform sign-in buttons stacked vertically

#### Extension Popup — Signed-in State Layout
- Top: viewer avatar + display name (from viewer JWT)
- Middle: "Name Color" label + inline `<input type="color">` swatch
- Bottom: "Open Settings" button (navigates to `/settings/viewer` on website) + "Sign Out" link
- Color save feedback: subtle inline "Saved ✓" text briefly appears next to the picker on successful PATCH — no toast, no modal

#### Extension Popup — Color Save
- Color change triggers immediate PATCH `/api/viewer/cosmetics` with `{ name_color: "#rrggbb" }`
- Also saves to `chrome.storage.local` for offline access
- No explicit Save button — autosave on color input

#### ViewerBadgeEnricher
- Enricher always overrides `UserInfo.Color` with viewer's stored `name_color` (if the viewer has one set) — viewer's All-Chat preference wins over platform color
- If viewer has no stored color (null/unset), pass through without modification
- Redis cache key: `viewer:identity:{platform}:{platform_user_id}` — value: JSON `{ viewer_id, name_color }` or `null` sentinel
- Cache TTL: 5 minutes
- Cache miss behavior: query `viewer_platform_identities` JOIN `viewer_cosmetics` from DB, populate cache, inject color if found
- Null sentinel cached to avoid DB thundering herd on unknown viewers
- Message-processor gets a PostgreSQL connection (shared DB with auth-service) for viewer lookup on cache miss

### Claude's Discretion
- Exact DB migration numbering (next after 034)
- Extension manifest.json permissions list (chrome.identity, storage, tabs, activeTab)
- Error handling for OAuth failure in extension (show error state in popup)
- `/settings/viewer` page content in Phase 28 (minimal stub — full editor is Phase 29)

### Deferred Ideas (OUT OF SCOPE)
- Multi-platform account merging (signing in with Twitch + YouTube linking to same viewer_id) — future phase
- TikTok identity linking (VID-TK-01) — deferred per requirements (unofficial library, user ID stability unclear)
- Full cosmetics editor on `/settings/viewer` (gradient builder, frame/flair) — Phase 29/30
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| VID-03 | Viewer's color preference persists server-side and survives extension reinstall | `viewer_cosmetics` table + PATCH `/api/viewer/cosmetics` handler + `chrome.storage.local` fallback |
| VID-04 | Viewer can link one or more platform identities (Twitch, YouTube, Kick) to their All-Chat account | `viewers` table + `viewer_platform_identities` table — foundation tables for multi-platform identity |
| VID-05 | Viewer can authenticate from the browser extension popup (sign in with Twitch or YouTube) | `chrome.identity.launchWebAuthFlow` + POST code-exchange endpoint on auth-service |
| VID-06 | Extension popup shows current auth status and signed-in display name / avatar | Extension popup reads JWT from `chrome.storage.local`, decodes viewer_id/display_name/avatar |
| EXT-01 | Extension popup shows an inline name color picker with reset-to-default option | `<input type="color">` in extension popup; reset calls PATCH with null or default value |
| EXT-02 | Color change saves immediately to server and local storage (no explicit Save button) | PATCH `/api/viewer/cosmetics` on input event + `chrome.storage.local` write |
| EXT-03 | Extension popup contains an "Open Settings" button navigating to `/settings/viewer` | `chrome.tabs.create({ url: website + '/settings/viewer' })` from popup |
| EXT-04 | Extension content scripts apply viewer's `name_color` to their own messages in the overlay | Content script reads `name_color` from `chrome.storage.local`; CSS injection into overlay iframe/page |
</phase_requirements>

---

## Summary

Phase 28 builds the viewer identity layer from three distinct fronts simultaneously: database schema (3 new tables), backend services (auth-service endpoint refactor + new cosmetics endpoint, message-processor ViewerBadgeEnricher), and the browser extension (OAuth popup flow, color picker, signed-in state).

The most technically novel piece is `chrome.identity.launchWebAuthFlow`, which redirects the OAuth browser pop-up back to a `chrome-extension://` URI rather than a server callback page. The auth-service already has all three OAuth providers (ViewerTwitchOAuth, ViewerYouTubeOAuth, ViewerKickOAuth) and the existing callback GET handlers need a POST "code-exchange" sibling that accepts `{ code, state }` from the extension instead of query parameters.

The `viewers` table is a thin anchor (UUID + created_at) that provides a durable identity across platform sessions. The existing `viewer_sessions` table gets a nullable `viewer_id` FK column added via migration 035. The ViewerBadgeEnricher follows the exact same structural pattern as `AvatarEnricher` and `BadgeEnricher` — it receives Redis + DB connections at construction, calls `Enrich(ctx, *UnifiedChatMessage)`, and returns a non-fatal error.

**Primary recommendation:** Follow the existing enricher interface pattern exactly; add the DB pool to message-processor startup where it already connects to PostgreSQL. The biggest risk is the JWT claim change — `ViewerClaims` in `shared/auth/jwt.go` needs a `ViewerID` field and `generateViewerJWT` in `viewer_auth.go` must populate it while remaining backward-compatible.

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| chrome.identity API | MV3 built-in | `launchWebAuthFlow` for extension OAuth | Only Chrome API that handles extension-originated OAuth without a server redirect |
| chrome.storage.local | MV3 built-in | Persist JWT + name_color in extension | Survives service worker restarts; survives browser restart; sync not needed |
| chrome.storage.session | MV3 built-in | Platform detection state (twitch/youtube/kick) | Cleared when browser session ends — correct lifetime for "which tab am I on" |
| pgx/v5 | Already in project | DB queries in ViewerBadgeEnricher | Matches existing message-processor pgxpool usage |
| go-redis/v9 | Already in project | Redis cache for viewer identity lookup | Same client already wired in message-processor |
| golang-jwt/jwt/v5 | Already in project | Extend ViewerClaims with viewer_id | Consistent with current JWT strategy |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| uuid (Google) | Already in project | viewer_id UUID generation | Consistent with ViewerSession.ID pattern |
| `<input type="color">` | HTML native | Color picker in extension popup | Zero dependencies; supported in all modern Chromium |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `chrome.identity.launchWebAuthFlow` | Open new tab manually, poll | Tab navigation is jarring; polling requires a server-side state server or longer-lived state tokens. `launchWebAuthFlow` is the correct API. |
| `chrome.storage.local` for JWT | `chrome.storage.sync` | Sync would share auth across devices which could be surprising and has a 8KB limit per key. Local is correct. |
| Null sentinel in Redis | No sentinel / always DB on miss | Without sentinel, every message from unknown viewer hits DB cold. Sentinel caps DB load. |

**Installation:** No new dependencies needed — all libraries already present.

---

## Architecture Patterns

### Database Schema

Three new tables plus one column addition to `viewer_sessions`. Next migration number after `034_share_expiry_fields.sql` is **035**.

```sql
-- 035_viewer_identity.sql
CREATE TABLE viewers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMP DEFAULT NOW()
);

ALTER TABLE viewer_sessions
    ADD COLUMN viewer_id UUID REFERENCES viewers(id) ON DELETE SET NULL;

CREATE INDEX idx_viewer_sessions_viewer_id ON viewer_sessions(viewer_id);

CREATE TABLE viewer_platform_identities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    viewer_id UUID NOT NULL REFERENCES viewers(id) ON DELETE CASCADE,
    platform VARCHAR(50) NOT NULL,          -- 'twitch', 'youtube', 'kick'
    platform_user_id VARCHAR(100) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(platform, platform_user_id)
);

CREATE INDEX idx_viewer_platform_identities_viewer_id ON viewer_platform_identities(viewer_id);
CREATE INDEX idx_viewer_platform_identities_lookup ON viewer_platform_identities(platform, platform_user_id);

CREATE TABLE viewer_cosmetics (
    viewer_id UUID PRIMARY KEY REFERENCES viewers(id) ON DELETE CASCADE,
    name_color VARCHAR(7),                  -- hex e.g. '#ff6600', NULL = unset
    updated_at TIMESTAMP DEFAULT NOW()
);
```

### JWT Claims Extension

`shared/auth/jwt.go` — `ViewerClaims` struct needs `ViewerID`:

```go
// Source: shared/auth/jwt.go (existing struct, additive change)
type ViewerClaims struct {
    ViewerID       string `json:"viewer_id"`       // NEW: durable viewer UUID
    SessionID      string `json:"session_id"`
    Platform       string `json:"platform"`
    PlatformUserID string `json:"platform_user_id"`
    Username       string `json:"username"`
    DisplayName    string `json:"display_name"`    // NEW: for extension popup display
    AvatarURL      string `json:"avatar_url"`      // NEW: for extension popup display
    IsViewer       bool   `json:"is_viewer"`
    jwt.RegisteredClaims
}
```

Backward compatible — `jwt.ParseWithClaims` ignores unknown fields. Old tokens simply have `viewer_id == ""`.

### Auth-Service: Dual-Mode Callback

The callback handlers (Twitch, YouTube, Kick) each gain a POST sibling that accepts JSON body instead of query parameters. The GET mode (browser redirect) continues to work unchanged.

```go
// Pattern for POST code-exchange (extension mode)
// Source: derived from existing HandleTwitchCallback
func (h *ViewerAuthHandler) HandleTwitchExchange(c *gin.Context) {
    var req struct {
        Code  string `json:"code"  binding:"required"`
        State string `json:"state" binding:"required"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Missing code or state"})
        return
    }
    // ... same logic as HandleTwitchCallback but returns JSON {token, viewer_info}
    // instead of HTTP redirect
    c.JSON(http.StatusOK, models.ViewerAuthResponse{Token: jwtToken, ...})
}
```

### Auth-Service: Viewer Identity Create/Link

When a viewer signs in via any platform, the flow becomes:

1. Look up `viewer_platform_identities` for (platform, platform_user_id)
2. If found: retrieve existing `viewer_id` → upsert `viewer_sessions` row with that `viewer_id`
3. If not found: create new `viewers` row → insert `viewer_platform_identities` row → create `viewer_sessions` row
4. Ensure `viewer_cosmetics` row exists (INSERT … ON CONFLICT DO NOTHING)

```go
// Source: derived from existing getOrCreateViewerSession pattern
func (h *ViewerAuthHandler) getOrCreateViewerWithIdentity(
    ctx context.Context,
    platform, platformUserID string,
    // ... user info fields
) (*models.ViewerSession, uuid.UUID, error) {
    // Returns (session, viewerID, error)
    // viewerID used to populate JWT claim
}
```

### Auth-Service: PATCH /viewer/cosmetics

New endpoint on the `viewerProtected` group (already has JWT middleware):

```go
viewerProtected.PATCH("/cosmetics", viewerAuthHandler.HandlePatchCosmetics)
```

Request: `{ "name_color": "#rrggbb" }` (or `null` to clear).
Response: `{ "name_color": "#rrggbb" }`.

Also invalidates Redis cache key `viewer:identity:{platform}:{platform_user_id}` so message-processor picks up the new color within 5 minutes (or immediately on next cache miss).

### API Gateway: Add PATCH Route

```go
// In api-gateway cmd/main.go — protectedAPI group
protectedAPI.PATCH("/auth/viewer/cosmetics", proxyHandler.ForwardRequest)
```

### Message-Processor: ViewerBadgeEnricher

Follows `AvatarEnricher` structural pattern exactly. Constructed with `redisClient` + `*pgxpool.Pool`. The DB pool is already created in `message-processor/cmd/main.go` (line 94–98).

```go
// Source: modeled on avatar_enricher.go
type ViewerBadgeEnricher struct {
    redis  *redis.Client
    db     *pgxpool.Pool
    logger *zap.Logger
}

const ViewerIdentityCacheTTL = 5 * time.Minute
const viewerIdentityCachePrefix = "viewer:identity:"

type viewerIdentityCache struct {
    ViewerID  string  `json:"viewer_id"`
    NameColor *string `json:"name_color"` // nil means no cosmetic set
}

func (e *ViewerBadgeEnricher) Enrich(ctx context.Context, msg *models.UnifiedChatMessage) error {
    if msg.User.ID == "" {
        return nil
    }
    cacheKey := fmt.Sprintf("%s%s:%s", viewerIdentityCachePrefix, msg.Platform, msg.User.ID)
    // 1. Check Redis cache
    // 2. If miss: query DB (viewer_platform_identities JOIN viewer_cosmetics)
    // 3. Cache result (including null sentinel as "null" string)
    // 4. If name_color non-nil: set msg.User.Color = *nameColor
    return nil
}
```

The null sentinel is stored as the JSON string `"null"` in Redis. On read, if the cached value is `"null"` → viewer unknown, skip. This prevents DB queries for every non-registered viewer.

### Extension: chrome.identity.launchWebAuthFlow

```javascript
// Source: Chrome Extension docs (MV3)
// Called from popup.js when user clicks "Sign in with Twitch"
async function signInWithPlatform(platform) {
    // 1. GET /api/v1/auth/viewer/{platform}/login → { auth_url }
    const { auth_url } = await fetchAuthURL(platform);

    // 2. Modify redirect_uri to be the extension's own page
    const redirectUri = chrome.identity.getRedirectURL('oauth');
    const modifiedUrl = auth_url.replace(
        encodeURIComponent(BACKEND_REDIRECT_URI),
        encodeURIComponent(redirectUri)
    );

    // 3. Launch the auth flow — Chrome handles the OAuth popup
    const responseUrl = await new Promise((resolve, reject) => {
        chrome.identity.launchWebAuthFlow(
            { url: modifiedUrl, interactive: true },
            (url) => {
                if (chrome.runtime.lastError) reject(chrome.runtime.lastError);
                else resolve(url);
            }
        );
    });

    // 4. Extract code and state from responseUrl
    const params = new URL(responseUrl).searchParams;

    // 5. POST code + state to backend for exchange
    const { token } = await exchangeCode(platform, params.get('code'), params.get('state'));

    // 6. Store JWT
    await chrome.storage.local.set({ viewer_jwt: token });
}
```

**Critical:** The `redirectUri` passed to `launchWebAuthFlow` must be registered in the OAuth app's allowed redirect URIs. For development, `chrome.identity.getRedirectURL()` returns `https://<extension-id>.chromiumapp.org/oauth`. For production use the same pattern.

**Kick PKCE note:** Kick uses PKCE. The extension must generate its own `code_verifier` and `code_challenge`, include `code_challenge` in the auth URL, and send `code_verifier` in the POST exchange body. The backend's `HandleKickExchange` POST handler must accept `code_verifier`.

### Extension: Popup Architecture

```
all-chat-extension/
├── manifest.json          # MV3 manifest
├── popup/
│   ├── popup.html         # Extension popup shell
│   ├── popup.js           # Popup logic (auth state, color picker, sign-in)
│   └── popup.css          # Popup styles
├── content/
│   └── content.js         # Platform detection → chrome.storage.session
├── background/
│   └── service_worker.js  # (optional) auth token refresh
└── icons/
    └── icon*.png
```

### Extension: Content Script Platform Detection

```javascript
// content.js — runs on twitch.tv, youtube.com, kick.com
const hostname = window.location.hostname;
let platform = null;
if (hostname.includes('twitch.tv')) platform = 'twitch';
else if (hostname.includes('youtube.com')) platform = 'youtube';
else if (hostname.includes('kick.com')) platform = 'kick';

if (platform) {
    chrome.storage.session.set({ current_platform: platform });
}
```

### Extension: EXT-04 Content Script Color Injection

```javascript
// content.js — also injects name_color into the user's own messages
// Reads chrome.storage.local for name_color
// Selects own message elements by matching username (from JWT display name)
// Applies style: element.style.color = nameColor
```

This runs after DOM is ready; must observe mutations for dynamically loaded chat.

### Recommended Project Structure (Extension)

No extension directory exists yet. The extension should live at:
```
all-chat-extension/
```
at the repository root (consistent with `frontend/` location).

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Extension OAuth popup | Custom tab + polling loop | `chrome.identity.launchWebAuthFlow` | Chrome manages the popup, PKCE, and redirect interception natively |
| Color picker UI | Custom color wheel | `<input type="color">` | Native HTML; zero JS; consistent with OS color picker |
| JWT decode in extension | Custom base64 parser | `JSON.parse(atob(token.split('.')[1]))` | One-liner for reading claims; no verification needed in extension (server verifies) |
| DB thundering herd on unknown viewers | No caching | Null sentinel in Redis | Without null sentinel, every Twitch/YouTube/Kick message from a non-registered viewer hits Postgres |

---

## Common Pitfalls

### Pitfall 1: Redirect URI Mismatch in Extension OAuth
**What goes wrong:** `chrome.identity.launchWebAuthFlow` uses `https://<ext-id>.chromiumapp.org/oauth` as the redirect URI, but the OAuth app registration (Twitch/YouTube/Kick) still has the backend URL. The flow succeeds in dev (because backend accepts it) but fails silently in the extension.
**Why it happens:** The extension hijacks the redirect URI, replacing the backend value with its own. Both Twitch and YouTube require the exact redirect URI to match what's registered.
**How to avoid:** Register `https://<ext-id>.chromiumapp.org/oauth` as an additional allowed redirect URI for each OAuth app. For dev builds, Chrome extension IDs are stable if you load an unpacked extension with the same key.
**Warning signs:** `chrome.identity.launchWebAuthFlow` callback receives `undefined` URL or error "The redirect URI in the request did not match a registered redirect URI".

### Pitfall 2: ViewerClaims JWT Backward Compatibility
**What goes wrong:** Existing viewer JWTs in `chrome.storage.local` (from Phase 24/25 chat-send feature) have `session_id` but no `viewer_id`. After migration, the backend's `HandleMe` and cosmetics handlers try to read `viewer_id` from claims and get empty string.
**Why it happens:** JWT parsing ignores unknown fields but missing fields return zero values. Code that casts `viewer_id` string to `uuid.UUID` will get `uuid.Nil`.
**How to avoid:** Add a guard in handlers: if `viewer_id == ""` in claims, fall back to looking up the session by `session_id` and returning the associated `viewer_id`. Alternatively, the `/viewer/me` endpoint can also return `viewer_id` to allow the extension to refresh its JWT after upgrade.

### Pitfall 3: Kick PKCE in Extension Context
**What goes wrong:** The existing `HandleKickLogin` generates a `code_verifier` server-side, stores it in Redis, and passes `code_challenge` in the auth URL. When the extension calls the endpoint and then later calls POST exchange, the `code_verifier` must still match.
**Why it happens:** The existing flow is designed for browser redirect, where the same Redis state key links the challenge to the verifier. In extension mode the same mechanism works — but the extension must send back the original `state` parameter precisely.
**How to avoid:** The extension should not modify the `state` parameter at all; pass it through from the auth URL to the POST exchange body unchanged. The backend retrieves `code_verifier` from Redis using `state` key exactly as before.

### Pitfall 4: Message-Processor DB Connection for ViewerBadgeEnricher
**What goes wrong:** The `message-processor` already connects to PostgreSQL (line 94 of `cmd/main.go`). Developers may try to pass a new connection instead of the existing pool.
**Why it happens:** The `db` pool variable is already in scope but nothing currently passes it to enrichers (enrichers only use Redis + HTTP).
**How to avoid:** Pass the existing `db *pgxpool.Pool` variable to `enricher.NewViewerBadgeEnricher(redisClient, db, log)`. No new connections needed.

### Pitfall 5: Chrome Storage Session vs Local Confusion
**What goes wrong:** Platform detection data (`current_platform`) is written to `chrome.storage.session` but later popup code tries to read from `chrome.storage.local`.
**Why it happens:** Two different storage buckets; easy to mix up.
**How to avoid:** Use `session` only for `current_platform` (tab-context ephemeral); use `local` only for `viewer_jwt` and `name_color` (persistent). Document clearly which key lives where.

---

## Code Examples

### Enricher Interface Pattern (from existing badge_enricher.go)

```go
// Source: services/message-processor/enricher/badge_enricher.go
// Pattern for ViewerBadgeEnricher.Enrich — same signature
func (e *BadgeEnricher) Enrich(ctx context.Context, msg *models.UnifiedChatMessage) error {
    if msg.Platform != "twitch" { return nil }
    // ... fetch/cache, update msg fields, return nil on soft failure
}
```

### Redis Null Sentinel Pattern

```go
// Source: derived from avatar_enricher.go Redis cache pattern
const nullSentinel = "null"

cached, err := e.redis.Get(ctx, cacheKey).Result()
if err == nil {
    if cached == nullSentinel {
        return nil // known unknown — no viewer record
    }
    var identity viewerIdentityCache
    if json.Unmarshal([]byte(cached), &identity) == nil && identity.NameColor != nil {
        msg.User.Color = *identity.NameColor
    }
    return nil
}
// cache miss — query DB
row := e.db.QueryRow(ctx, `
    SELECT vpi.viewer_id, vc.name_color
    FROM viewer_platform_identities vpi
    LEFT JOIN viewer_cosmetics vc ON vc.viewer_id = vpi.viewer_id
    WHERE vpi.platform = $1 AND vpi.platform_user_id = $2
`, msg.Platform, msg.User.ID)
// ... scan, cache result or null sentinel
```

### Extension: Read JWT and Decode Claims

```javascript
// popup.js
async function getViewerInfo() {
    const { viewer_jwt } = await chrome.storage.local.get('viewer_jwt');
    if (!viewer_jwt) return null;
    try {
        const payload = JSON.parse(atob(viewer_jwt.split('.')[1]));
        // payload.viewer_id, payload.username, payload.avatar_url, payload.exp
        if (payload.exp * 1000 < Date.now()) return null; // expired
        return payload;
    } catch { return null; }
}
```

### Migration Pattern (from 011_viewer_authentication.sql)

```sql
-- 035_viewer_identity.sql
-- Description: Add viewer identity tables for cross-platform cosmetics

CREATE TABLE viewers ( ... );
ALTER TABLE viewer_sessions ADD COLUMN viewer_id UUID REFERENCES viewers(id) ON DELETE SET NULL;
CREATE TABLE viewer_platform_identities ( ... );
CREATE TABLE viewer_cosmetics ( ... );
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Extension OAuth via new tab + manual redirect | `chrome.identity.launchWebAuthFlow` (MV3) | MV3 mandate ~2023 | No background page; service worker replaces it; `launchWebAuthFlow` is the only stable OAuth path |
| Viewer JWT carries `session_id` | Viewer JWT carries `viewer_id` | Phase 28 | `viewer_id` is durable across platform re-auths; `session_id` changes when tokens refresh |

**Deprecated/outdated:**
- MV2 `chrome.extension.getBackgroundPage()` for shared state: replaced by `chrome.storage` in MV3
- Persistent background page: replaced by service worker (event-driven); all state must go to `chrome.storage`

---

## Open Questions

1. **Extension ID stability across developers**
   - What we know: Chrome extension ID is derived from the extension key in `manifest.json`. Without a fixed key, it changes per developer machine.
   - What's unclear: Is there a shared key checked into the repo, or will each dev have a different extension ID?
   - Recommendation: Generate a stable key and commit it with the extension. For CI/production use the same key. This makes the redirect URI registration deterministic.

2. **Kick OAuth redirect URI registration**
   - What we know: Kick's developer portal may only allow one redirect URI per app (unlike Google/Twitch which allow multiple).
   - What's unclear: Whether `https://<ext-id>.chromiumapp.org/oauth` can be added as a second redirect URI alongside the backend callback.
   - Recommendation: Investigate Kick developer portal limits. If only one redirect URI is allowed, Phase 28 Kick sign-in from extension may need to be a "Phase 28b" item or use the website flow.

3. **EXT-04 overlay message matching**
   - What we know: The content script should apply `name_color` to the viewer's own messages in the overlay. The overlay is a React app in a separate frame/page at `overlay/:id`.
   - What's unclear: The overlay page is on the same domain as the website. Content scripts with `matches` for the website domain can access overlay pages. The exact CSS selector for "own messages" needs to be defined.
   - Recommendation: Match by `data-username` attribute on message elements. The frontend/overlay renders messages — check if it already sets `data-username`. If not, add it in Phase 28.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go test (stdlib) + testify |
| Config file | none — per-package `go test` |
| Quick run command | `cd services/auth-service && go test ./... -count=1` |
| Full suite command | `cd services/auth-service && go test ./... && cd ../message-processor && go test ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| VID-03 | `viewer_cosmetics` persists name_color on PATCH | unit | `go test ./handlers/... -run TestPatchCosmetics` | Wave 0 |
| VID-04 | `viewer_platform_identities` row created on first sign-in | unit | `go test ./repository/... -run TestViewerIdentityCreate` | Wave 0 |
| VID-05 | POST code-exchange returns JWT with viewer_id | unit | `go test ./handlers/... -run TestTwitchExchange` | Wave 0 |
| VID-06 | JWT claims include display_name + avatar_url | unit | `go test ./handlers/... -run TestViewerJWTClaims` | Wave 0 |
| EXT-01 | Color picker renders in signed-in state | manual | N/A — browser extension UI | manual-only |
| EXT-02 | Color save POSTs to PATCH endpoint | manual | N/A — browser extension interaction | manual-only |
| EXT-03 | "Open Settings" opens `/settings/viewer` | manual | N/A — browser extension interaction | manual-only |
| EXT-04 | Content script applies name_color to own messages | manual | N/A — overlay DOM inspection | manual-only |

EXT-01 through EXT-04 are manual-only because they require a running browser with an installed extension. Go unit tests cover all backend paths.

### Sampling Rate
- **Per task commit:** `cd services/auth-service && go test ./... -count=1 -short`
- **Per wave merge:** `cd services/auth-service && go test ./... && cd ../message-processor && go test ./enricher/...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `services/auth-service/handlers/viewer_cosmetics_test.go` — covers VID-03 PATCH handler
- [ ] `services/auth-service/repository/viewer_identity_test.go` — covers VID-04 viewer_platform_identities create/lookup
- [ ] `services/auth-service/handlers/viewer_exchange_test.go` — covers VID-05 POST code-exchange (Twitch + YouTube + Kick)
- [ ] `services/message-processor/enricher/viewer_badge_enricher_test.go` — covers ViewerBadgeEnricher Redis cache hit, miss, null sentinel, color injection

---

## Sources

### Primary (HIGH confidence)
- Direct source inspection of `services/auth-service/handlers/viewer_auth.go` — full OAuth flow for Twitch/YouTube/Kick including existing GET callback pattern
- Direct source inspection of `services/auth-service/models/viewer.go` — `ViewerSession` and `ViewerJWTClaims` structs
- Direct source inspection of `services/auth-service/repository/viewer_repository.go` — GetByPlatformUserID, Create, Update patterns
- Direct source inspection of `shared/auth/jwt.go` — `ViewerClaims` struct (currently missing viewer_id, display_name, avatar_url)
- Direct source inspection of `services/message-processor/enricher/badge_enricher.go` and `avatar_enricher.go` — enricher interface pattern
- Direct source inspection of `services/message-processor/cmd/main.go` (lines 94–170) — existing enricher wiring; `db` pool already available
- Direct source inspection of `migrations/034_share_expiry_fields.sql` — confirms next migration is 035
- Direct source inspection of `migrations/011_viewer_authentication.sql` — existing `viewer_sessions` schema
- Direct source inspection of `.planning/phases/28-viewer-identity-foundation-auth-and-platform-linking/28-CONTEXT.md` — locked design decisions

### Secondary (MEDIUM confidence)
- Chrome Identity API docs (from training data, current as of 2025) — `chrome.identity.launchWebAuthFlow` signature and redirect URI behavior
- Chrome Storage API docs (training data) — `chrome.storage.local` vs `chrome.storage.session` semantics

### Tertiary (LOW confidence)
- Kick developer portal redirect URI limits — unverified; flagged in Open Questions

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all Go libraries are already in use; Chrome APIs are well-documented and stable
- Architecture: HIGH — directly derived from existing code patterns in this repo
- Pitfalls: HIGH (4/5) / MEDIUM (1/5 — Kick redirect URI limit is unverified)
- Extension patterns: MEDIUM — `chrome.identity.launchWebAuthFlow` behavior verified via training data; exact redirect URI registration workflow for Kick is LOW

**Research date:** 2026-03-14
**Valid until:** 2026-04-14 (stable domain; Chrome MV3 APIs are frozen; Go libraries have pinned versions)
