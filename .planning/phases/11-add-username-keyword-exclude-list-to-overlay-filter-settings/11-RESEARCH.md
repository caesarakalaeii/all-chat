# Phase 11: Add username/keyword exclude list to overlay filter settings - Research

**Researched:** 2026-04-12
**Domain:** React/Next.js frontend — overlay config persistence, client-side message filtering, tag-style UI components
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** Client-side filtering only — filter messages in the overlay page JavaScript before rendering. Message-processor stays stateless.
- **D-02:** The overlay page (`frontend/src/app/overlay/[id]/page.tsx`) applies FilterSettings from the config to incoming WebSocket messages before adding them to the render queue.
- **D-03:** Keywords (`banned_words`) support regex patterns. Plain strings work as-is (valid regex). Client-side only so performance and regex complexity are non-concerns.
- **D-04:** Username matching (`banned_users`) is exact match, case-insensitive. Banning "nightbot" blocks "Nightbot" and "NIGHTBOT" but not "nightbot_fan".
- **D-05:** `hide_commands` suppresses messages starting with `!`.
- **D-06:** `min_message_length` filters messages shorter than the threshold (0 = disabled).
- **D-07:** The overlay editor preview applies filters in real-time (WYSIWYG). Filtered messages do not appear in the preview.
- **D-08:** "Add common bots" quick-add button populates `banned_users` with Nightbot, StreamElements, Moobot, Fossabot, SoundAlerts, and other widely-used bots. Users can remove any after adding.

### Claude's Discretion

- Exact UI layout and spacing of the Filters section within AppearancePanel
- Tag/chip input component implementation details
- Order of filter checks (username → keyword → commands → length, or any order)
- Error handling for invalid regex patterns (inline validation or silent skip)
- The specific bot names included in the preset list

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.

</user_constraints>

---

## Summary

Phase 11 adds a client-side message filtering UI to the overlay editor and wires the stored `FilterSettings` into the live overlay WebSocket pipeline. The backend model and API layer already support `filter_settings` as a JSONB column and a first-class field in the config PUT endpoint. The TypeScript `FilterSettings` interface already defines all four fields (`banned_words`, `banned_users`, `min_message_length`, `hide_commands`).

The work is predominantly frontend: a new `FilterGroup.tsx` component for the editor's panel, integration into the editor's state and save path, and filter application logic in the live overlay page. There is **one backend gap**: the public config endpoint (`HandleGetPublicConfig`) currently omits `filter_settings` from its response, but the live overlay page uses this unauthenticated endpoint. Adding `filter_settings` to the public response is required for D-01/D-02 to work in production.

**Primary recommendation:** Build `FilterGroup.tsx` following the existing `*Group.tsx` pattern, add `filter_settings` to the public config response, wire filter application into the `ws.onmessage` handler in the overlay page, and apply the same logic to the preview channel in the editor.

---

## Standard Stack

### Core (all already installed)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| React | 19+ | UI component model | Project standard |
| Next.js | 16+ App Router | Page routing | Project standard |
| TypeScript | 5+ | Type safety | Project standard |
| Tailwind CSS | v3 | Utility classes | Project standard |

### Supporting (already installed)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| @base-ui/react/collapsible | installed | Collapsible section primitives | Used by CollapsibleSection |
| lucide-react | installed | Icons (X for tag removal, Plus for add) | Used throughout UI |
| vitest | installed | Unit tests | All component tests |
| @testing-library/react | installed | Component render/interaction testing | Component tests |

**No new packages required.** [VERIFIED: codebase grep — all dependencies already in frontend/package.json]

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Hand-rolled tag input | react-tag-input, react-select creatable | Adds dependency; hand-rolled tag input is ~30 lines and fits design system exactly |
| Inline regex validation | Third-party validator | Overkill; `new RegExp(pattern)` in a try/catch is sufficient |

---

## Architecture Patterns

### Established Pattern: `*Group.tsx` Component

Every AppearancePanel section is a standalone `*Group.tsx` file. [VERIFIED: codebase — TypographyGroup, ColorsGroup, BackgroundGroup, VisibilityGroup, SizingGroup, PlatformColorsGroup, EventsGroup all follow this structure]

```
frontend/src/components/appearance/
├── FilterGroup.tsx          ← NEW: implements Filters section
├── AppearancePanel.tsx      ← EDIT: import + render FilterGroup
├── CollapsibleSection.tsx   ← reuse (no changes)
├── ToggleSwitch.tsx         ← reuse (no changes)
└── SliderControl.tsx        ← reuse (no changes)
```

### Pattern 1: FilterGroup Component Signature

The `AppearancePanel` currently takes `(visualSettings, onChange)` — both typed as `VisualSettings`. `FilterSettings` lives in `OverlayConfig`, not `VisualSettings`, so `FilterGroup` needs separate props:

```typescript
// Source: codebase analysis of AppearancePanel.tsx and overlay.ts types
export interface FilterGroupProps {
  filterSettings: FilterSettings
  onChange: (patch: Partial<FilterSettings>) => void
}
```

`AppearancePanel` must accept an additional `filterSettings` + `onFilterChange` pair, or `FilterGroup` can be rendered directly in the editor page outside `AppearancePanel`, or `AppearancePanel`'s props can be extended. **Recommended:** extend `AppearancePanelProps` with optional `filterSettings` and `onFilterChange` — matches existing `visibilityDefaults` opt-in pattern.

### Pattern 2: Tag Input (hand-rolled, design-system-compatible)

No existing tag/chip component exists in the codebase. The pattern used throughout for text inputs is a native `<input>` with Tailwind styling. A tag input follows the same convention:

```typescript
// Source: [ASSUMED] — standard pattern for tag inputs in React
// Render existing tags as dismissible chips, Enter/comma adds new tag
function TagInput({ tags, onAdd, onRemove, placeholder }: TagInputProps) {
  const [draft, setDraft] = useState('')

  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if ((e.key === 'Enter' || e.key === ',') && draft.trim()) {
      e.preventDefault()
      onAdd(draft.trim().toLowerCase())  // usernames lowercased at entry
      setDraft('')
    }
    if (e.key === 'Backspace' && draft === '' && tags.length > 0) {
      onRemove(tags[tags.length - 1])
    }
  }

  return (
    <div className="flex flex-wrap gap-1 rounded-lg border border-border bg-surface p-2">
      {tags.map(tag => (
        <span key={tag} className="flex items-center gap-1 rounded bg-surface-alt px-2 py-0.5 text-xs text-text">
          {tag}
          <button type="button" onClick={() => onRemove(tag)} aria-label={`Remove ${tag}`}>
            <X className="h-3 w-3" />
          </button>
        </span>
      ))}
      <input
        className="flex-1 min-w-[120px] bg-transparent text-xs text-text outline-none placeholder:text-text-dim"
        value={draft}
        onChange={e => setDraft(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder={placeholder}
      />
    </div>
  )
}
```

### Pattern 3: Filter Logic in Overlay Page

The `ws.onmessage` handler processes `chat_message` envelopes at line 322-333 of `overlay/[id]/page.tsx`. Filter logic inserts between message parsing and `setMessages`:

```typescript
// Source: codebase analysis of overlay/[id]/page.tsx lines 322-333
// Suggested insertion point:

if (envelope.type === 'chat_message' && envelope.data) {
  let message: ChatMessage = envelope.data
  message = await resolveTwitchBadgeIcons(message)
  message = sortMessageBadges(message)
  // ... name_gradient parsing ...

  // Phase 11: apply filter settings BEFORE adding to state
  if (shouldFilterMessage(message, filterSettingsRef.current)) return

  setMessages(prev => [...prev, message].slice(-maxMessagesRef.current))
}
```

Using a `ref` (like `maxMessagesRef`) ensures the closure always sees the latest filter settings without adding `filterSettings` to the WebSocket effect dependency array (which would reconnect on every settings change).

### Pattern 4: Filter Settings Loading in Overlay Page

The overlay page uses the **public** endpoint `/api/v1/overlays/public/${id}/config`. The `HandleGetPublicConfig` handler currently returns only `display_settings`, `custom_css`, `visual_settings`, and `sources`. [VERIFIED: services/overlay-manager/handlers/config.go lines 165-170]

**The filter_settings field must be added to the public config response.** This is a one-line addition to `HandleGetPublicConfig`:

```go
// Source: services/overlay-manager/handlers/config.go line 165
c.JSON(http.StatusOK, gin.H{
    "display_settings":  config.DisplaySettings,
    "filter_settings":   config.FilterSettings,   // ADD THIS
    "custom_css":        config.CustomCSS,
    "visual_settings":   config.VisualSettings,
    "sources":           sourceStatus,
})
```

### Pattern 5: Filter Settings Persistence in Editor

The editor's `handleSaveConfiguration` (line 1644) currently does NOT save `filter_settings`. The save call must be extended:

```typescript
// Source: codebase analysis of overlays/[id]/page.tsx line 1648
await overlaysApi.updateConfig(id, {
  display_settings: { ... },
  enable_7tv,
  enable_bttv,
  enable_ffz,
  custom_css: useCustomCss ? customCss : '',
  visual_settings: visualSettings,
  filter_settings: filterSettings,   // ADD THIS
})
```

### Pattern 6: Preview Filtering in Editor

The editor page sends preview messages to the iframe preview, and also renders a local message list for the live preview. The same `shouldFilterMessage` utility must be applied to the local preview channel. The editor's WebSocket (`ws.onmessage` at line 1348) handles `share_revoked` only — the real preview is an embedded iframe showing `overlay/[id]`. The iframe receives filter settings via the public endpoint on its own `loadConfig`. Since the preview iframe is a live embed of the actual overlay page, **it will apply filters automatically once filter_settings is returned by the public endpoint.** No additional preview-specific wiring is needed.

### Pattern 7: `shouldFilterMessage` Pure Utility

Extract filter logic as a pure utility function (following `pronounPill.ts`, `usernameSpan.ts` pattern):

```typescript
// frontend/src/lib/utils/filterMessage.ts
import type { ChatMessage } from '../types/message'
import type { FilterSettings } from '../types/overlay'

export function shouldFilterMessage(
  message: ChatMessage,
  settings: FilterSettings | null | undefined
): boolean {
  if (!settings) return false

  const username = message.user?.username?.toLowerCase() ?? ''
  const displayName = message.user?.display_name?.toLowerCase() ?? ''
  const text = message.message?.text ?? ''

  // D-04: exact case-insensitive username match
  if (settings.banned_users?.some(u => u.toLowerCase() === username || u.toLowerCase() === displayName)) {
    return true
  }

  // D-03: regex keyword match (literal strings are valid regex)
  if (settings.banned_words?.length) {
    for (const pattern of settings.banned_words) {
      try {
        if (new RegExp(pattern, 'i').test(text)) return true
      } catch {
        // invalid regex — skip silently (Claude's discretion)
      }
    }
  }

  // D-05: hide bot commands
  if (settings.hide_commands && text.startsWith('!')) return true

  // D-06: minimum message length
  if (settings.min_message_length && settings.min_message_length > 0) {
    if (text.length < settings.min_message_length) return true
  }

  return false
}
```

### Anti-Patterns to Avoid

- **Adding filter_settings to VisualSettings:** FilterSettings is a separate concern with its own JSONB column. Never merge it into visual_settings.
- **Filtering on the server/message-processor:** D-01 locks this as client-side. Message-processor is stateless by design.
- **Re-creating RegExp objects on every message:** Compile patterns once per settings change, not per message. Cache compiled regexes in a ref alongside the settings ref.
- **Skipping the public config endpoint fix:** Without adding `filter_settings` to `HandleGetPublicConfig`, the live overlay will never receive filter settings (it uses the unauthenticated endpoint).
- **Hardcoding filter logic in the overlay page:** Extract to `shouldFilterMessage` utility so it can be unit tested and reused.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Tag input | Custom state machine | Simple `useState` + `onKeyDown` (Enter/comma/Backspace) | ~30 lines, no dependency needed |
| Regex validation | Custom parser | `try { new RegExp(pattern) } catch {}` | JavaScript built-in; silent skip is correct per discretion |
| Config persistence | Custom save logic | Existing `overlaysApi.updateConfig()` | Already handles partial updates; backend merges correctly |

**Key insight:** The entire feature is frontend-only with one minor backend field addition. No new Go packages, no new API endpoints, no database migrations.

---

## Common Pitfalls

### Pitfall 1: Public Config Endpoint Omits filter_settings
**What goes wrong:** The live overlay loads config from `/api/v1/overlays/public/${id}/config`. This endpoint currently returns `display_settings`, `custom_css`, `visual_settings`, `sources` — no `filter_settings`. Without adding it, filters configured in the editor will silently not apply to the live overlay.
**Why it happens:** The public handler was written conservatively to avoid leaking private data; filter lists were not considered at that time.
**How to avoid:** Add `"filter_settings": config.FilterSettings` to the public handler's JSON response (one line).
**Warning signs:** Filters work in preview but not in OBS.

### Pitfall 2: WebSocket Closure Staleness
**What goes wrong:** The `ws.onmessage` closure captures filter settings at WebSocket creation time. If settings change after connection, the closure sees stale settings.
**Why it happens:** React closure semantics — the effect captures the value from the render where it ran.
**How to avoid:** Store filter settings in a `ref` (like `maxMessagesRef`) and update the ref whenever state changes. Read from the ref inside the closure.
**Warning signs:** Filters don't update when changed without page reload.

### Pitfall 3: Saving filter_settings Separately From visual_settings
**What goes wrong:** The editor's "Save" button currently saves display/visual settings. If filter_settings is saved in a separate action, users must click save twice. If the save path is missed, filter settings are lost on reload.
**Why it happens:** Two separate concerns (appearance vs. filters) with one save button.
**How to avoid:** Include `filter_settings` in the same `updateConfig` call as all other settings, triggered by the existing Save button.

### Pitfall 4: AppearancePanel Prop Mismatch
**What goes wrong:** `AppearancePanel` currently takes `(visualSettings, onChange)`. FilterSettings is a different type living in a different part of `OverlayConfig`. Passing filter settings via the existing `onChange` would corrupt `VisualSettings`.
**Why it happens:** The panel was designed only for visual settings.
**How to avoid:** Add optional `filterSettings?: FilterSettings` and `onFilterChange?: (patch: Partial<FilterSettings>) => void` props to `AppearancePanelProps`, rendered only when provided.

### Pitfall 5: Username Matching on display_name vs. username
**What goes wrong:** Streamers think in terms of display names (e.g., "Nightbot"), but the message field may be `username` (lowercased). If matching is only against `username`, banning "NightBot" (with capital B) fails.
**Why it happens:** Twitch in particular has separate `username` (all lowercase) and `display_name` (styled) fields.
**How to avoid:** Normalize both the stored banned entry and both `message.user.username` and `message.user.display_name` to lowercase for comparison (D-04).

### Pitfall 6: Regex Catastrophic Backtracking
**What goes wrong:** A user enters a pathological regex like `(a+)+` that causes exponential backtracking on long messages, freezing the tab.
**Why it happens:** JavaScript's `RegExp` engine is vulnerable to ReDoS on malformed patterns.
**How to avoid:** Client-side only + power-user feature (D-03 explicitly accepts this). Mitigate by adding a message-level timeout or note in UI that complex regex is at user's own risk. For MVP, silent catch is sufficient.

---

## Code Examples

### Verified Patterns

#### CollapsibleSection usage (from existing components)
```typescript
// Source: frontend/src/components/appearance/AppearancePanel.tsx
<CollapsibleSection id="filters" title="Filters">
  <FilterGroup filterSettings={filterSettings} onChange={onFilterChange} />
</CollapsibleSection>
```

#### ToggleSwitch usage (from VisibilityGroup.tsx)
```typescript
// Source: frontend/src/components/appearance/VisibilityGroup.tsx
<ToggleSwitch
  label="Hide bot commands (!)"
  checked={filterSettings.hide_commands ?? false}
  onChange={(next) => onChange({ hide_commands: next })}
/>
```

#### SliderControl usage (from existing components)
```typescript
// Source: frontend/src/components/appearance/SliderControl.tsx
<SliderControl
  label="Min length"
  value={filterSettings.min_message_length ?? 0}
  min={0}
  max={200}
  step={1}
  unit=" chars"
  onChange={(v) => onChange({ min_message_length: v })}
/>
```

#### Filter ref pattern (from overlay page's maxMessagesRef)
```typescript
// Source: frontend/src/app/overlay/[id]/page.tsx lines 95-96 (maxMessagesRef pattern)
const filterSettingsRef = useRef<FilterSettings>({})

// Update ref whenever state changes
useEffect(() => {
  filterSettingsRef.current = filterSettings
}, [filterSettings])
```

#### Common bots preset list
```typescript
// Source: [ASSUMED] — well-known Twitch/YouTube bot accounts
const COMMON_BOTS = [
  'nightbot',
  'streamelements',
  'moobot',
  'fossabot',
  'soundalerts',
  'streamlabs',
  'stay_hydrated_bot',
  'serybot',
  'wizebot',
  'botisimo',
]
```
[ASSUMED: specific bot names — planner/implementer may expand or trim. The list is user-editable so correctness is not critical.]

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| filter_settings stored in DB but not surfaced | Fully wired to UI | Phase 11 | Full streamer control |
| Public config endpoint omits filter_settings | Must add filter_settings to response | Phase 11 | Live overlay gets filters |

**Already done (no changes needed):**
- `FilterSettings` TypeScript interface: defined in `overlay.ts` lines 54-59 [VERIFIED]
- Backend model `models.OverlayConfig.FilterSettings map[string]any`: defined and used [VERIFIED]
- `overlay_configs.filter_settings JSONB`: column exists in initial schema [VERIFIED]
- Config PUT handler: already merges `filter_settings` from request body [VERIFIED: config.go line 101-102]
- Clone handler: already copies `filter_settings` [VERIFIED: overlay.go line 290]
- `updateConfig()` API client: already accepts `Partial<OverlayConfig>` which includes `filter_settings` [VERIFIED]

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Common bots preset list (specific names) | Code Examples | Low — list is user-editable, wrong names just don't match |
| A2 | Tag input implementation details (Enter/comma/Backspace behavior) | Architecture Pattern 2 | Low — standard UX convention; user can correct in review |
| A3 | Silent regex catch is the right error handling strategy | Code Examples | Low — Claude's discretion; alternative is inline warning |

---

## Open Questions

1. **Where does FilterGroup live in the editor — inside AppearancePanel or as a sibling section?**
   - What we know: AppearancePanel takes `visualSettings/onChange` only. FilterSettings is a separate type.
   - What's unclear: Whether to extend AppearancePanelProps or render FilterGroup as a peer CollapsibleSection directly in the editor page.
   - Recommendation: Extend AppearancePanelProps with optional filter props. Keeps the panel self-contained and matches existing pattern. Either approach is valid — planner decides.

2. **Should the filter check order be documented/enforced?**
   - What we know: D-03–D-06 define four independent checks; order is Claude's discretion.
   - Recommendation: username → keyword → commands → length. Most specific first (username) to most general (length). Document in code comment.

---

## Environment Availability

Step 2.6: SKIPPED (no external dependencies — purely frontend code + one backend field addition, no new tools or services required)

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Vitest (with @testing-library/react) |
| Config file | `frontend/vitest.config.ts` |
| Quick run command | `cd frontend && npx vitest run src/components/appearance/__tests__/FilterGroup.test.tsx src/lib/utils/__tests__/filterMessage.test.ts` |
| Full suite command | `cd frontend && npx vitest run` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| F-01 | `shouldFilterMessage` blocks banned usernames (case-insensitive exact match) | unit | `npx vitest run src/lib/utils/__tests__/filterMessage.test.ts` | ❌ Wave 0 |
| F-02 | `shouldFilterMessage` blocks banned keywords via regex | unit | `npx vitest run src/lib/utils/__tests__/filterMessage.test.ts` | ❌ Wave 0 |
| F-03 | `shouldFilterMessage` suppresses `!`-prefixed messages when hide_commands=true | unit | `npx vitest run src/lib/utils/__tests__/filterMessage.test.ts` | ❌ Wave 0 |
| F-04 | `shouldFilterMessage` suppresses messages shorter than min_message_length | unit | `npx vitest run src/lib/utils/__tests__/filterMessage.test.ts` | ❌ Wave 0 |
| F-05 | `shouldFilterMessage` returns false when settings are null/empty | unit | `npx vitest run src/lib/utils/__tests__/filterMessage.test.ts` | ❌ Wave 0 |
| F-06 | `shouldFilterMessage` handles invalid regex without throwing | unit | `npx vitest run src/lib/utils/__tests__/filterMessage.test.ts` | ❌ Wave 0 |
| F-07 | FilterGroup renders banned_users tags and add-input | unit | `npx vitest run src/components/appearance/__tests__/FilterGroup.test.tsx` | ❌ Wave 0 |
| F-08 | FilterGroup renders banned_words tags and add-input | unit | `npx vitest run src/components/appearance/__tests__/FilterGroup.test.tsx` | ❌ Wave 0 |
| F-09 | FilterGroup hide_commands toggle calls onChange | unit | `npx vitest run src/components/appearance/__tests__/FilterGroup.test.tsx` | ❌ Wave 0 |
| F-10 | FilterGroup min_message_length slider calls onChange | unit | `npx vitest run src/components/appearance/__tests__/FilterGroup.test.tsx` | ❌ Wave 0 |
| F-11 | FilterGroup "Add common bots" button populates banned_users | unit | `npx vitest run src/components/appearance/__tests__/FilterGroup.test.tsx` | ❌ Wave 0 |
| F-12 | Public config endpoint returns filter_settings field | manual | Deploy + `curl /api/v1/overlays/public/{id}/config \| jq .filter_settings` | — |
| F-13 | Editor saves filter_settings on Save click and reloads correctly | manual | Open editor, add filter, save, reload, verify persisted | — |

### Sampling Rate
- **Per task commit:** `cd frontend && npx vitest run src/lib/utils/__tests__/filterMessage.test.ts src/components/appearance/__tests__/FilterGroup.test.tsx`
- **Per wave merge:** `cd frontend && npx vitest run`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `frontend/src/lib/utils/__tests__/filterMessage.test.ts` — covers F-01 through F-06
- [ ] `frontend/src/components/appearance/__tests__/FilterGroup.test.tsx` — covers F-07 through F-11
- [ ] `frontend/src/lib/utils/filterMessage.ts` — pure utility (created alongside tests)

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | no | Filter config is owned-resource, protected by existing `GetByIDAndUserID` check in config handler |
| V5 Input Validation | yes (limited) | Regex patterns are user-supplied but evaluated only in the user's own browser — no server-side exec risk |
| V6 Cryptography | no | — |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| ReDoS via malicious regex | Denial of Service (self) | Silent catch; client-side only, affects only the overlay owner's browser; acceptable per D-03 |
| filter_settings exposed in public config | Information Disclosure | filter_settings contains banned usernames/words — mildly sensitive but no credentials; acceptable to expose publicly (overlay URL is already unguessable UUID) |

---

## Sources

### Primary (HIGH confidence — verified in codebase)
- `frontend/src/lib/types/overlay.ts` — FilterSettings interface (lines 54-59), OverlayConfig (line 26)
- `services/overlay-manager/models/config.go` — FilterSettings as map[string]any
- `services/overlay-manager/handlers/config.go` — HandleUpdateConfig merges filter_settings (lines 101-102), HandleGetPublicConfig omits filter_settings (lines 165-170)
- `frontend/src/components/appearance/AppearancePanel.tsx` — component structure, imports, props
- `frontend/src/components/appearance/VisibilityGroup.tsx` — ToggleSwitch pattern, CollapsibleSection usage
- `frontend/src/components/appearance/CollapsibleSection.tsx` — @base-ui/react/collapsible, localStorage persistence
- `frontend/src/components/appearance/ToggleSwitch.tsx` — exact prop signature
- `frontend/src/components/appearance/SliderControl.tsx` — exact prop signature
- `frontend/src/lib/api/overlays.ts` — updateConfig(id, Partial<OverlayConfig>) — already handles filter_settings
- `frontend/src/app/overlay/[id]/page.tsx` — ws.onmessage pipeline, public config loading, maxMessagesRef pattern
- `frontend/src/app/overlays/[id]/page.tsx` — handleSaveConfiguration, config loading, AppearancePanel integration
- `migrations/001_initial_schema.sql` — filter_settings JSONB column exists (line 42)
- `frontend/vitest.config.ts` — unit test project config, include pattern
- `frontend/src/components/appearance/__tests__/VisibilityGroup.test.tsx` — test pattern to follow

### Secondary (MEDIUM confidence)
- None required — all critical facts verified directly in codebase.

### Tertiary (LOW confidence)
- Common bots list [ASSUMED] — well-known by convention, not verified against any registry.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — verified in codebase, no new dependencies
- Architecture: HIGH — verified by reading all canonical reference files
- Pitfalls: HIGH — public config gap and closure staleness are verified codebase observations
- Bot preset list: LOW — conventional knowledge

**Research date:** 2026-04-12
**Valid until:** 2026-05-12 (stable codebase, no fast-moving external dependencies)
