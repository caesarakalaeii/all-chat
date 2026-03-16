# Phase 29: Viewer Color & Gradient Editor - Research

**Researched:** 2026-03-15
**Domain:** Frontend UI (Next.js/React), Go backend (auth-service), PostgreSQL JSONB, Redis caching, CSS gradient text rendering
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Color picker UX**
- Autosave on color change (fires PATCH immediately on input) — consistent with extension popup from Phase 28
- Native `<input type="color">` swatch + editable hex input field only — no additional sliders or wheels
- Save feedback: subtle inline "Saved ✓" text briefly appears next to the picker on success — no toast, no modal
- Color section and gradient section share a single card with two tabs: "Solid Color" and "Gradient"

**Gradient editor UI**
- Stacked color swatches with +/− buttons: a list of 2–4 color rows, each with a native color swatch + hex field; Add stop / Remove (×) buttons
- Angle: range slider (0–360°) + numeric input field showing exact degrees side-by-side
- Explicit "Save gradient" button (not autosave) — gradient editor accumulates multiple changes before a PATCH is sent
- Saving a gradient clears `name_color` (set to null) — one mode active at a time. Overlay logic: if `name_gradient` non-null → render gradient; else if `name_color` non-null → render flat color; else platform default

**Live preview**
- Simulated chat message row: small avatar circle + viewer's actual display name (from JWT claims) styled with the current color/gradient + static sample message text "Hello world!"
- Preview positioned below the controls within the same card tab
- Preview updates live as the user adjusts stops or angle (before saving)

**Premium gate**
- The "Gradient" tab is visible to all authenticated users but disabled (not clickable) for non-premium users, with a lock icon or "Premium" badge on the tab label
- JWT re-validation on gradient save: before each PATCH for gradient, re-fetch the viewer JWT to confirm it is still valid and premium; return 403 if not premium (server-side enforcement always applies regardless)
- Gradient rendering scope: both website overlay (`frontend/src/app/overlay/[id]/page.tsx`) and browser extension chat overlay — both receive the `name_gradient` field via the enriched `UserInfo` and apply it

### Claude's Discretion
- Exact DB migration numbering (next after the Phase 28 migrations)
- Error state presentation if gradient save fails (PATCH 403 or network error)
- Specific Tailwind classes for the locked tab appearance
- Sample avatar appearance in the live preview (initials-based fallback acceptable)

### Deferred Ideas (OUT OF SCOPE)
- None raised — discussion stayed within phase scope
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| VID-01 | Viewer can set a fallback name color (hex) as a global preference | Autosave PATCH to existing `/api/v1/auth/viewer/cosmetics` endpoint; no schema change needed for color-only |
| VID-02 | Viewer's fallback color is applied in all overlays where platform sends no color | ViewerBadgeEnricher already injects `name_color` into `UserInfo.Color`; extend to also propagate `name_gradient` |
| PREM-01 | Premium viewer can set a multi-stop gradient (2–4 colors, angle) as their name color | New `name_gradient` JSONB column in `viewer_cosmetics`; extend PATCH endpoint and enricher |
| PREM-02 | Gradient name renders in overlay using CSS `background-clip: text` — no JavaScript required | `bg-clip-text text-transparent` + inline `backgroundImage` style; verified Tailwind pattern |
| WEB-01 | Settings page has a "Viewer Identity" section for all authenticated users (color picker, platform linking) | Replace Phase 28 stub card with tabbed card; existing page structure retained |
| WEB-02 | Premium users see a "Premium Cosmetics" section with gradient editor (multi-stop, angle control) | Gradient tab shown to all but disabled for non-premium; reads `viewer.is_premium` from JWT |
| WEB-05 | Live preview of name color, gradient displayed on the settings page | Inline preview row in the card tab using current editor state |
</phase_requirements>

---

## Summary

Phase 29 is a full-stack extension of the Phase 28 viewer identity foundation. The backend work is narrow and mechanical: add a `name_gradient JSONB` column to `viewer_cosmetics`, add `is_premium` to the `viewers` table, extend the PATCH cosmetics endpoint to accept and validate gradient JSON, extend the ViewerBadgeEnricher Redis cache shape and DB query, and add `IsPremium` to `ViewerClaims` JWT so the frontend can gate the UI without an extra round-trip.

The frontend work is the largest part. The existing Phase 28 stub card (`Name Color` section in `/settings/viewer/page.tsx`) is replaced with a two-tab card ("Solid Color" | "Gradient"). The solid color tab keeps the autosave pattern already in the stub. The gradient tab introduces a new local-state editor (2–4 color stop rows, angle slider, angle number input, "Save gradient" button) with a live preview row below. Both tabs display a simulated chat-message row that updates reactively to the current editor state.

The overlay rendering change is the most surgical: line 676 of `frontend/src/app/overlay/[id]/page.tsx` currently sets `style={{ color: message.user?.color || '#FFFFFF' }}` on the username `<span>`. Phase 29 branches this: if `message.user?.name_gradient` is present, the span switches to `className="bg-clip-text text-transparent"` with `style={{ backgroundImage: 'linear-gradient(...)' }}`; otherwise it falls back to the existing flat-color path. The browser extension `ChatContainer.tsx` gets the same two-branch treatment on its username span.

**Primary recommendation:** Three plans — (1) DB migration + Go backend (JSONB column, `is_premium`, enricher, PATCH schema, JWT claim), (2) Settings page UI (tabbed card, autosave solid color, gradient editor, live preview, premium gate), (3) Overlay render update (website overlay + extension ChatContainer gradient branch).

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Next.js App Router | 14+ | Settings page (`/settings/viewer`) | Project standard; existing page uses it |
| React | 18+ | Component state, controlled inputs | Project standard |
| Tailwind CSS | 3.x | Styling including `bg-clip-text text-transparent` | Project standard; gradient text pattern works out of the box |
| shadcn/ui | existing | `Card`, `Button`, `Input` components | Already imported in the settings page |
| Go + Gin | 1.23+ | PATCH cosmetics endpoint extension | Project standard for auth-service |
| pgx/v5 | existing | JSONB scan/write for `name_gradient` | Project standard; `pgx` handles JSONB as `[]byte` or custom type |
| go-redis/v9 | existing | Redis cache invalidation on PATCH | Already used in `ViewerCosmeticsHandler` |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `encoding/json` | stdlib | Marshal/unmarshal gradient JSON in Go | Used to validate JSONB structure server-side |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Native `<input type="color">` | react-colorful, Spectrum | CONTEXT locked to native input; avoid |
| Inline `backgroundImage` style | CSS custom property + class | Inline is simpler for dynamic values; class would require JIT-unsafe dynamic class strings |

---

## Architecture Patterns

### Recommended Project Structure Changes

```
services/auth-service/
├── handlers/viewer_cosmetics.go   # extend patchCosmeticsRequest, validation, UpsertViewerCosmetics signature
├── repository/viewer_identity_repository.go  # UpsertViewerCosmetics adds nameGradient param
shared/auth/jwt.go                 # add IsPremium bool to ViewerClaims
migrations/
├── 036_viewer_gradient.sql        # ADD COLUMN name_gradient JSONB to viewer_cosmetics; ADD COLUMN is_premium to viewers

services/message-processor/
├── enricher/viewer_badge_enricher.go  # extend viewerIdentityCache struct and DB query

frontend/src/
├── app/settings/viewer/page.tsx   # replace stub Name Color card with tabbed card
├── app/overlay/[id]/page.tsx      # gradient branch on username span (line 676)
├── lib/types/message.ts           # add name_gradient?: NameGradient to UserInfo

all-chat-extension/src/
├── ui/components/ChatContainer.tsx           # gradient branch on username span
├── lib/types/extension.ts                    # add viewer_name_gradient?: string to LocalStorage
├── lib/storage.ts                            # add getNameGradient() helper
├── background/service-worker.ts              # handle SAVE_NAME_GRADIENT message
```

### Pattern 1: JSONB PATCH with mutual exclusion

The cosmetics PATCH endpoint currently accepts `{ name_color: string | null }`. Phase 29 extends it to accept `{ name_color: string | null, name_gradient: NameGradient | null }`. Mutual exclusion is enforced server-side: if `name_gradient` is non-null in the request, `name_color` is forced to null in the DB upsert (and vice versa).

```go
// Source: existing pattern in viewer_cosmetics.go — extend with gradient field
type patchCosmeticsRequest struct {
    NameColor    *string          `json:"name_color"`
    NameGradient *NameGradientReq `json:"name_gradient"`
}

type NameGradientReq struct {
    Type   string   `json:"type"`   // must be "linear"
    Colors []string `json:"colors"` // 2–4 #rrggbb values
    Angle  int      `json:"angle"`  // 0–360
}
```

Validation steps: type must be "linear", colors length 2–4, each color matches `hexColorRegex`, angle 0–360.

If `name_gradient` is non-null → `name_color` set to null in upsert. If `name_color` is non-null → `name_gradient` set to null.

### Pattern 2: CSS gradient text rendering

The locked pattern for gradient text in both the overlay and the extension preview. The `bg-clip-text` and `text-transparent` classes must appear together on the same element; the `backgroundImage` is set via inline style because the value is dynamic.

```tsx
// Source: Tailwind docs + MDN CSS background-clip
// For gradient names in overlay/[id]/page.tsx and ChatContainer.tsx
{message.user?.name_gradient ? (
  <span
    className="font-semibold text-sm bg-clip-text text-transparent"
    style={{
      backgroundImage: buildGradientCSS(message.user.name_gradient),
    }}
  >
    {message.user.display_name || message.user.username}
  </span>
) : (
  <span
    className="font-semibold text-sm"
    style={{ color: message.user?.color || '#FFFFFF' }}
  >
    {message.user.display_name || message.user.username}
  </span>
)}
```

Helper:
```ts
// Pure function — can live in a shared utils file
function buildGradientCSS(g: NameGradient): string {
  return `linear-gradient(${g.angle}deg, ${g.colors.join(', ')})`
}
```

### Pattern 3: Tabbed card with premium gate

The settings page card uses two tabs sharing a single `<Card>`. Tabs are implemented with simple controlled state (`activeTab: 'solid' | 'gradient'`) — no Radix Tabs or separate shadcn tabs component needed given the simplicity. The Gradient tab button has `disabled` and `pointer-events-none` when `!claims.is_premium`.

```tsx
// Controlled tab pattern
const [activeTab, setActiveTab] = useState<'solid' | 'gradient'>('solid')

<div className="flex border-b border-border mb-4">
  <button
    onClick={() => setActiveTab('solid')}
    className={`px-4 py-2 text-sm font-medium transition-colors ${
      activeTab === 'solid' ? 'border-b-2 border-primary text-text' : 'text-text-sub'
    }`}
  >
    Solid Color
  </button>
  <button
    onClick={() => !claims.is_premium ? undefined : setActiveTab('gradient')}
    disabled={!claims.is_premium}
    className={`px-4 py-2 text-sm font-medium transition-colors flex items-center gap-1.5 ${
      activeTab === 'gradient' ? 'border-b-2 border-primary text-text' : 'text-text-sub'
    } ${!claims.is_premium ? 'opacity-50 cursor-not-allowed' : ''}`}
  >
    Gradient
    {!claims.is_premium && <span className="text-xs px-1.5 py-0.5 rounded bg-amber-500/20 text-amber-400">Premium</span>}
  </button>
</div>
```

### Pattern 4: Autosave with debounce on hex input

The solid color tab autosaves on the native `<input type="color">` `onChange` (fires after the color wheel closes on most browsers). The hex text input fires `onChange` too — but hex input changes should debounce to avoid sending incomplete hex strings like `#ff` mid-type.

```tsx
// Debounce for hex text field only; color swatch fires immediately
const debouncedSave = useCallback(
  debounce((color: string) => {
    if (/^#[0-9a-fA-F]{6}$/.test(color)) saveColor(color)
  }, 400),
  []
)
```

A 400 ms debounce on hex input is sufficient. The color swatch saves immediately.

### Pattern 5: Premium validation on gradient PATCH

Before sending the gradient PATCH, the settings page re-fetches the viewer JWT from `localStorage` and re-decodes it to confirm `is_premium` is still true. If the JWT has expired or `is_premium` is false, the save is blocked client-side with an error message. The server always enforces independently (returns 403 if `is_premium` is false on the viewer record, regardless of JWT contents).

### Anti-Patterns to Avoid

- **Dynamic Tailwind classes for gradient:** `bg-gradient-to-r from-[${color1}]` — Tailwind cannot JIT purge these. Always use inline `backgroundImage` style for dynamic gradients.
- **Storing gradient as a string in the DB:** JSONB enables structured validation and future querying; `VARCHAR` would require client-side parsing of a custom format.
- **Embedding `is_premium` check only in the UI:** Server-side enforcement on the PATCH endpoint is mandatory regardless of client-side gating.
- **Invalidating the Redis cache for the viewer identity only on color PATCH:** Must also invalidate on gradient PATCH so the message-processor enricher picks up the new value within the 5-minute cache window.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| CSS gradient from color stops | Custom interpolation | Native CSS `linear-gradient()` | Browser-native, handles all color formats |
| JSONB read/write in Go | Custom marshaling | `pgx/v5` `pgx.Row.Scan` into `[]byte` + `json.Unmarshal` | pgx handles JSONB transparently as `[]byte` |
| Debounce utility | Custom timeout wrapper | Standard `useCallback` + `setTimeout` pattern or tiny inline debounce | Simple 1-function debounce — no library needed |
| Premium check middleware | Per-handler DB query inline | Pattern from `share-service/middleware/premium.go` | Existing pattern queries `viewers.is_premium` directly |

**Key insight:** CSS `linear-gradient()` already handles multi-stop gradients with angle natively. No JS gradient math library is needed.

---

## Common Pitfalls

### Pitfall 1: `bg-clip-text` invisible on dark backgrounds without `text-transparent`

**What goes wrong:** Adding `bg-clip-text` alone produces no visible change. The text color must also be set to `transparent` to reveal the background clipped to the text shape.
**Why it happens:** `background-clip: text` only clips the background; the text color still paints on top.
**How to avoid:** Always pair `bg-clip-text text-transparent` together. Both classes are required.
**Warning signs:** Gradient appears black or invisible; toggling `text-transparent` makes it visible.

### Pitfall 2: JSONB null vs missing field in Go pgx scan

**What goes wrong:** `pgx.Row.Scan` into `*[]byte` for a JSONB column returns `nil` when the column value is SQL NULL (not set). If code does `json.Unmarshal(nil, &v)` it returns an error.
**Why it happens:** pgx returns nil `[]byte` for NULL JSONB, which is not valid JSON.
**How to avoid:** Check `nameGradientBytes != nil` before calling `json.Unmarshal`.

### Pitfall 3: Mutual exclusion race on rapid tab switching

**What goes wrong:** User autosaves a solid color, immediately switches to gradient tab and saves — both requests in-flight simultaneously. The response order determines which wins in the DB, but the Redis cache key (`viewer:identity:{platform}:{user_id}`) may be invalidated by the first response but then cached again with stale data by the second.
**Why it happens:** Two concurrent PATCH requests both invalidate and then miss the cache at the same time.
**How to avoid:** Each PATCH enforces mutual exclusion in the DB via `ON CONFLICT DO UPDATE SET name_color = ..., name_gradient = ...` — the last write wins atomically in PostgreSQL. This is acceptable; no request queuing needed.

### Pitfall 4: `is_premium` not present in ViewerClaims JWT

**What goes wrong:** The settings page decodes the JWT to read `is_premium` to gate the gradient tab. If `is_premium` is not in the JWT claims, the decode returns `undefined`, which falsily disables the tab for all users.
**Why it happens:** `ViewerClaims` struct in `shared/auth/jwt.go` currently does not have an `is_premium` field. Phase 29 must add it.
**How to avoid:** Add `IsPremium bool \`json:"is_premium"\`` to `ViewerClaims`. When issuing new tokens (in `generateViewerJWT`), look up `is_premium` from the `viewers` table and include it in claims.

### Pitfall 5: Extension `LocalStorage` type doesn't include gradient

**What goes wrong:** Extension ChatContainer reads `viewer_name_color` from `LocalStorage` but there is no `viewer_name_gradient` field. Gradient rendering in the extension is skipped silently.
**Why it happens:** Phase 28 only added the flat color field.
**How to avoid:** Add `viewer_name_gradient?: string` (serialized JSON) to the `LocalStorage` interface in `extension.ts`. Add `getNameGradient()` to `storage.ts`. The service worker's `SAVE_NAME_GRADIENT` message handler serializes the gradient object to JSON before storing.

### Pitfall 6: Missing DB migration — `viewers.is_premium` column

**What goes wrong:** The `viewers` table (created in migration 035) has no `is_premium` column. Any code that joins `viewers` to read premium status will fail with a column-not-found error.
**Why it happens:** Premium was added to the `users` table (streamers) in migration 030 but was never mirrored to the `viewers` table.
**How to avoid:** The Phase 29 migration must `ALTER TABLE viewers ADD COLUMN IF NOT EXISTS is_premium BOOLEAN NOT NULL DEFAULT FALSE`.

---

## Code Examples

Verified patterns from existing codebase:

### Existing PATCH cosmetics handler (to extend)
```go
// Source: services/auth-service/handlers/viewer_cosmetics.go
// Current signature — Phase 29 adds NameGradient field
type patchCosmeticsRequest struct {
    NameColor *string `json:"name_color"`
    // Phase 29: add NameGradient *NameGradientReq `json:"name_gradient"`
}
```

### Existing cache invalidation on cosmetics PATCH
```go
// Source: services/auth-service/handlers/viewer_cosmetics.go — HandlePatchCosmetics
cacheKey := fmt.Sprintf("viewer:identity:%s:%s", platform, platformUserID)
h.redis.Del(context.Background(), cacheKey)
```

### Existing ViewerBadgeEnricher DB query (to extend)
```go
// Source: services/message-processor/enricher/viewer_badge_enricher.go
// Current — Phase 29 adds name_gradient to SELECT and cache struct
row := e.db.QueryRow(ctx, `
    SELECT vpi.viewer_id::text, vc.name_color
    FROM viewer_platform_identities vpi
    LEFT JOIN viewer_cosmetics vc ON vc.viewer_id = vpi.viewer_id
    WHERE vpi.platform = $1 AND vpi.platform_user_id = $2
`, msg.Platform, msg.User.ID)
```

### Existing overlay username render (to branch)
```tsx
// Source: frontend/src/app/overlay/[id]/page.tsx line 674-679
<span
  className="font-semibold text-sm"
  style={{ color: message.user?.color || '#FFFFFF' }}
>
  {message.user?.display_name || message.user?.username}
</span>
```

### Existing extension username render (to branch)
```tsx
// Source: all-chat-extension/src/ui/components/ChatContainer.tsx line 436-445
<span
  className="font-semibold text-sm"
  style={{
    color: (viewerInfo && message.user.username === viewerInfo.username && viewerNameColor)
      ? viewerNameColor
      : (message.user.color || '#fff')
  }}
>
```

### ViewerClaims struct (to extend with IsPremium)
```go
// Source: shared/auth/jwt.go
type ViewerClaims struct {
    ViewerID       string `json:"viewer_id"`
    SessionID      string `json:"session_id"`
    Platform       string `json:"platform"`
    PlatformUserID string `json:"platform_user_id"`
    Username       string `json:"username"`
    DisplayName    string `json:"display_name"`
    AvatarURL      string `json:"avatar_url"`
    IsViewer       bool   `json:"is_viewer"`
    // Phase 29: add IsPremium bool `json:"is_premium"`
    jwt.RegisteredClaims
}
```

### JWT `is_premium` decode on settings page
```ts
// Source: frontend/src/app/settings/viewer/page.tsx — ViewerJWTClaims interface
// Phase 29 adds is_premium to the interface
interface ViewerJWTClaims {
  viewer_id?: string
  display_name?: string
  // ... existing fields ...
  is_premium?: boolean  // Phase 29: read from JWT for gradient tab gate
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Inline `color` style on username span | Inline `backgroundImage` for gradient, `color` for flat | Phase 29 | Gradient text without JS |
| `viewer_cosmetics` table has only `name_color VARCHAR(7)` | Add `name_gradient JSONB` alongside | Phase 29 migration | Structured gradient storage |
| `viewers` table has no premium flag | Add `is_premium BOOLEAN DEFAULT FALSE` | Phase 29 migration | Viewer premium gating |
| `ViewerClaims` JWT has no premium claim | Add `is_premium bool` to claims | Phase 29 | Settings page can gate without extra API call |
| ViewerBadgeEnricher caches only `name_color` | Cache also includes `name_gradient` | Phase 29 | Overlay renders gradient without extra DB query |

---

## Open Questions

1. **JWT re-issuance for `is_premium`**
   - What we know: Phase 29 adds `is_premium` to `ViewerClaims` and includes it in new tokens. Existing Phase 28 tokens do not have the field.
   - What's unclear: Should a viewer who authenticates before Phase 29 deploys and tries to access `/settings/viewer` see the gradient tab as locked because their JWT lacks `is_premium`? Or should the page fall back to a DB check?
   - Recommendation: Treat absent/undefined `is_premium` in JWT as `false` (safe default). No fallback DB check needed — the server-side PATCH enforcement catches any bypass attempt. Viewers will get a fresh token on next login.

2. **Extension gradient storage format**
   - What we know: `viewer_name_color` is stored as a plain string in `LocalStorage`. The gradient object `{ type, colors, angle }` needs to be stored as well.
   - What's unclear: Whether to store it as a serialized JSON string (`viewer_name_gradient: string`) or as a structured object.
   - Recommendation: Store as a JSON string (`JSON.stringify`) in `LocalStorage` under key `viewer_name_gradient`. Deserialize in `getNameGradient()`. This matches the `viewer_name_color` pattern (plain storage, no schema validation at the storage layer).

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Vitest (unit), Go `testing` package |
| Config file | `frontend/vitest.config.ts` (unit project); standard `go test` for Go |
| Quick run command (frontend) | `cd /home/moersener/Hobby/all-chat/frontend && npx vitest run --project unit` |
| Quick run command (Go) | `cd /home/moersener/Hobby/all-chat/services/auth-service && go test ./handlers/...` |
| Full suite command | `cd /home/moersener/Hobby/all-chat/frontend && npx vitest run --project unit && cd ../services/auth-service && go test ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| VID-01 | Solid color autosave fires PATCH on color change | unit | `npx vitest run --project unit src/app/settings/viewer/__tests__` | ❌ Wave 0 |
| VID-02 | ViewerBadgeEnricher injects name_gradient when present | unit | `cd services/message-processor && go test ./enricher/... -run TestViewerBadgeEnricher` | ❌ Wave 0 |
| PREM-01 | Gradient PATCH accepted with valid JSON; rejected for non-premium | unit | `cd services/auth-service && go test ./handlers/... -run TestPatchCosmetics` | ✅ (extend existing) |
| PREM-02 | Gradient CSS string renders with bg-clip-text pattern | unit | `npx vitest run --project unit src/app/overlay/__tests__` | ❌ Wave 0 |
| WEB-01 | Settings page renders Viewer Identity section when authenticated | unit | `npx vitest run --project unit src/app/settings/viewer/__tests__` | ❌ Wave 0 |
| WEB-02 | Gradient tab disabled for non-premium users | unit | `npx vitest run --project unit src/app/settings/viewer/__tests__` | ❌ Wave 0 |
| WEB-05 | Live preview updates on stop/angle change | unit | `npx vitest run --project unit src/app/settings/viewer/__tests__` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `cd /home/moersener/Hobby/all-chat/services/auth-service && go test ./handlers/... -run TestPatchCosmetics`
- **Per wave merge:** Full suite command above
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `frontend/src/app/settings/viewer/__tests__/page.test.tsx` — covers VID-01, WEB-01, WEB-02, WEB-05
- [ ] `frontend/src/app/overlay/__tests__/gradient-render.test.tsx` — covers PREM-02
- [ ] `services/message-processor/enricher/viewer_badge_enricher_gradient_test.go` — covers VID-02

---

## Sources

### Primary (HIGH confidence)
- Direct code inspection: `services/auth-service/handlers/viewer_cosmetics.go` — existing PATCH endpoint pattern
- Direct code inspection: `services/auth-service/repository/viewer_identity_repository.go` — UpsertViewerCosmetics signature
- Direct code inspection: `services/message-processor/enricher/viewer_badge_enricher.go` — cache shape and DB query
- Direct code inspection: `shared/auth/jwt.go` — ViewerClaims struct
- Direct code inspection: `shared/middleware/auth.go` — JWTAuth middleware sets `viewer_id`, `platform`, `platform_user_id` in context
- Direct code inspection: `migrations/035_viewer_identity.sql` — `viewer_cosmetics` table schema
- Direct code inspection: `frontend/src/app/settings/viewer/page.tsx` — Phase 28 stub to replace
- Direct code inspection: `frontend/src/app/overlay/[id]/page.tsx` line 674-679 — username span to branch
- Direct code inspection: `all-chat-extension/src/ui/components/ChatContainer.tsx` line 436-445 — extension username span
- Direct code inspection: `all-chat-extension/src/lib/types/extension.ts` — LocalStorage type
- Direct code inspection: `services/share-service/middleware/premium.go` — existing premium middleware pattern

### Secondary (MEDIUM confidence)
- MDN: `background-clip: text` + `color: transparent` — standard cross-browser CSS gradient text technique
- Tailwind CSS docs: `bg-clip-text text-transparent` classes — confirmed utilities exist in Tailwind v3

### Tertiary (LOW confidence)
- None

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries already in use in this project
- Architecture: HIGH — patterns directly observed in existing Phase 28 code
- Pitfalls: HIGH — several identified from direct inspection of gaps between existing code and Phase 29 requirements
- CSS gradient text: HIGH — standard web platform feature, no library dependency

**Research date:** 2026-03-15
**Valid until:** 2026-04-15 (stable libraries and patterns)
