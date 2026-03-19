# Phase 37: Editor UX Rework - Research

**Researched:** 2026-03-19
**Domain:** React component restructuring, localStorage persistence, collapsible UI sections
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Theme section format**
- Inline embedded content — `CollapsibleSection` titled "Theme" expands to show theme cards directly in the editor panel (no modal)
- Reuse existing `ThemeMarketplaceModal` content: extract the theme list/grid and render it inline, compact sizing
- "Reset to theme defaults" button (from Phase 36) lives inside the Theme section alongside the theme list
- Theme section starts expanded by default — it's the primary entry point for the v1.6 customizer
- The existing `ThemeMarketplaceModal` component and "Browse Themes" button in the CSS editor row are removed

**Section open/close defaults**
- Theme: expanded by default
- Sources: expanded by default
- Appearance: collapsed by default
- Behavior: collapsed by default
- Expert: collapsed by default (roadmap requirement — CSS editor hidden by default)
- State persisted in localStorage using a new key: `editor-panel-sections-v1` (separate from `appearance-panel-sections-v1` used by AppearancePanel sub-groups)

**Sources section content**
- Inline add/remove controls — full source management in the editor panel, no navigation needed
- Shows list of configured sources with platform icon + channel name + ✕ remove button per row
- "Add Source" expands an inline form within the section: platform selector + channel input fields
- Removing a source is immediate (no confirmation prompt)

**Behavior section scope**
- In Behavior: Max Messages, Message Duration, Disable Message Fade, Platform Badge settings (show/hide + position + style), Emote Providers (7TV, BTTV, FFZ)
- Moved to Appearance (Typography sub-group): Font Size slider (was a standalone control)
- Moved to Expert: Mock Messages (inject messages, sample chat/events, clear)
- Stats (message count, connection status): stays in Behavior section

**Expert section contents**
- CSS editor (MonacoCSSEditor) with Enable/Disable toggle
- Mock Messages testing tools (inject, sample chat, sample events, clear)

**Save button placement**
- Sticky footer — pinned at the bottom of the editor panel sidebar, always visible regardless of which sections are open

**Font Size placement**
- Merged into the existing Typography sub-group inside AppearancePanel — consistent with other typography properties (font family, weight, line height, letter spacing)
- The legacy standalone `fontSize` state in the editor page maps to `visualSettings.typography.fontSize`

**CollapsibleSection reuse**
- Use the existing `CollapsibleSection` component from `frontend/src/components/appearance/CollapsibleSection.tsx`
- Top-level section IDs: `"theme"`, `"appearance"`, `"sources"`, `"behavior"`, `"expert"`
- Override default (closed) behavior per-section: pass `defaultOpen` prop or handle in the component via the `editor-panel-sections-v1` key

### Claude's Discretion
(None specified — discussion stayed within phase scope)

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| EDUX-01 | Theme Marketplace is the first visible element in the editor panel | Inline ThemeContent component extracted from ThemeMarketplaceModal, placed in first CollapsibleSection (defaultOpen=true) |
| EDUX-02 | Editor panel uses collapsible sections: Theme, Appearance (with sub-groups), Sources, Behavior, Expert | CollapsibleSection with configurable storageKey prop; localStorage key `editor-panel-sections-v1` |
| EDUX-03 | CSS editor is hidden by default inside the collapsible "Expert" section | Expert CollapsibleSection has defaultOpen=false; MonacoCSSEditor rendered inside it |
| EDUX-04 | Visual customizer sub-groups: Typography, Colors, Background & Bubbles, Visibility, Sizing, Platform Colors, Events | All 7 sub-groups already exist in AppearancePanel; fontSize migration from standalone state to visualSettings.typography.fontSize completes this |
</phase_requirements>

## Summary

Phase 37 restructures the overlay editor panel (`frontend/src/app/overlays/[id]/page.tsx`) from a flat vertical scroll into 5 labeled collapsible sections: Theme, Appearance, Sources, Behavior, Expert. This is a pure frontend reorganization — no backend changes, no new libraries, no new API calls.

The primary technical challenge is: (1) modifying `CollapsibleSection` to accept a configurable storage key so top-level editor sections use `editor-panel-sections-v1` while AppearancePanel sub-groups continue to use `appearance-panel-sections-v1`; and (2) extracting the inline-renderable theme content from `ThemeMarketplaceModal` into a new component (`ThemeContent` or similar) that can be embedded directly without the modal wrapper.

The `fontSize` state in the editor page is currently a standalone number stored in `display_settings.font_size`. The decision is to migrate it into `visualSettings.fontSize` (string, e.g. `"16px"`) which maps to the `--chat-font-size` CSS variable — this unifies it with the TypographyGroup control already present in AppearancePanel.

**Primary recommendation:** Three-plan wave. Plan 1: extend CollapsibleSection with storageKey prop and add ThemeContent inline component. Plan 2: restructure the editor panel JSX into 5 sections + sticky footer + sources inline management. Plan 3: migrate fontSize state, wire Behavior section, wire Expert section.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `@base-ui/react` (Collapsible) | ^1.0 (project-installed) | Collapsible primitive for all sections | Already used in CollapsibleSection.tsx |
| React `useState` / localStorage | Built-in | Section open/close persistence | Established pattern from CollapsibleSection |
| Tailwind CSS | Project-installed | Sticky footer, layout classes | Project-wide styling system |
| `lucide-react` | Project-installed | Icons (ChevronDown, X for remove source) | Already imported in editor page |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `overlaysApi.addSource` / `deleteSource` | Existing | Inline source management | Sources section add/remove controls |
| `useThemeMarketplace` hook | Existing | Theme list/filter state | Powers inline Theme section content |
| `ThemeCard` | Existing | Renders individual theme cards | Reused in inline Theme section |
| `ThemeFilters` | Existing | Search + tag filter UI | Reused in inline Theme section |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Extending CollapsibleSection with storageKey prop | Creating a separate EditorCollapsibleSection component | Prop extension is simpler; avoids code duplication; existing tests only assert on key `appearance-panel-sections-v1` so no test change needed if prop has a default |

**No installation required** — all dependencies already in the project.

## Architecture Patterns

### Recommended Project Structure
No new files/directories needed. Changes are:
```
frontend/src/
├── components/appearance/
│   └── CollapsibleSection.tsx           # Add storageKey prop
├── components/theme-marketplace/
│   └── ThemeContent.tsx                 # New: extracted inline theme list/grid
└── app/overlays/[id]/
    └── page.tsx                         # Restructured: 5 sections + sticky footer
```

### Pattern 1: CollapsibleSection storageKey Extension

**What:** Add a `storageKey` prop to `CollapsibleSection` that defaults to `'appearance-panel-sections-v1'` to preserve existing behavior, allowing callers to pass `'editor-panel-sections-v1'` for the top-level sections.

**When to use:** Any time two independent sets of collapsible sections must persist their open state independently.

**Current component signature:**
```typescript
// Source: frontend/src/components/appearance/CollapsibleSection.tsx (line 19-23)
export interface CollapsibleSectionProps {
  id: string
  title: string
  children: React.ReactNode
}
```

**Extended signature:**
```typescript
export interface CollapsibleSectionProps {
  id: string
  title: string
  children: React.ReactNode
  storageKey?: string          // defaults to 'appearance-panel-sections-v1'
  defaultOpen?: boolean        // defaults to false (existing behavior)
}
```

The `readStoredSections()` function currently hardcodes `STORAGE_KEY`. With this change, the function must accept the key as a parameter. The `defaultOpen` prop is used as the fallback when no stored value exists — currently the fallback is `false`.

### Pattern 2: ThemeContent Inline Component

**What:** Extract the theme list, loading state, error state, and empty state from `ThemeMarketplaceModal` into a new `ThemeContent` component that renders inline (no fixed overlay, no modal wrapper, no ESC handler, no body scroll lock).

**Props signature:**
```typescript
interface ThemeContentProps {
  onApply: (css: string) => void
}
```

Internally calls `useThemeMarketplace()` directly (same hook ThemeMarketplaceModal uses). Renders `ThemeFilters` and the `ThemeCard` grid compactly for the sidebar context (single column, smaller previews).

**Important:** The `ThemeMarketplaceModal` is still used by the Credit Roll editor (`/overlays/[id]/credits`), so it must NOT be deleted — only the reference in the overlay editor page is replaced.

### Pattern 3: 5-Section Editor Panel Structure

**What:** Replace the flat `space-y-6` content inside `SplitView` with 5 `CollapsibleSection` wrappers, each using `storageKey="editor-panel-sections-v1"`.

```tsx
<SplitView ...>
  <div className="flex flex-col h-full">
    {/* scrollable sections */}
    <div className="flex-1 overflow-y-auto space-y-0 p-4">
      <CollapsibleSection id="theme" title="Theme" storageKey="editor-panel-sections-v1" defaultOpen={true}>
        {/* ThemeContent + Reset to theme defaults button */}
      </CollapsibleSection>
      <CollapsibleSection id="appearance" title="Appearance" storageKey="editor-panel-sections-v1" defaultOpen={false}>
        {/* AppearancePanel */}
      </CollapsibleSection>
      <CollapsibleSection id="sources" title="Sources" storageKey="editor-panel-sections-v1" defaultOpen={true}>
        {/* inline source list + add form */}
      </CollapsibleSection>
      <CollapsibleSection id="behavior" title="Behavior" storageKey="editor-panel-sections-v1" defaultOpen={false}>
        {/* Max Messages, Duration, Fade, InvertOrder, PlatformBadge, Emotes, Stats */}
      </CollapsibleSection>
      <CollapsibleSection id="expert" title="Expert" storageKey="editor-panel-sections-v1" defaultOpen={false}>
        {/* Custom CSS (MonacoCSSEditor) + Mock Messages */}
      </CollapsibleSection>
    </div>
    {/* sticky footer */}
    <div className="flex-shrink-0 border-t border-border p-4">
      <Button onClick={...} ...>Save Configuration</Button>
    </div>
  </div>
</SplitView>
```

### Pattern 4: Inline Sources Section

**What:** Replace the existing `<section aria-label="Chat sources">` with a streamlined version that shows sources as rows with ✕ remove button (no confirmation dialog). "Add Source" expands an inline form.

The existing `SourceCard` component is feature-heavy (relay config, revocation, Discord config) — for the simplified inline Sources section, use a simpler row component. However, the relay panel and revocation confirm modal must still function, so the SourceCard should still be used — the decision says "immediate remove, no confirmation prompt", which means only the `handleRemoveSource` call is wired directly, bypassing the existing revocation dialog for non-shared sources.

**Critical distinction:** Existing `handleRemoveSource` function in the page already handles this. The "no confirmation" decision applies specifically to source removal via the ✕ button in the Sources section. The existing `RevocationConfirmModal` is for shared overlay revocation (different flow) and should remain.

### Pattern 5: Font Size State Migration

**What:** The standalone `fontSize` state (number, in `display_settings.font_size`) should map to `visualSettings.typography.fontSize`. Since TypographyGroup already has a "Body Size" input field (line 103-115 of TypographyGroup.tsx), the standalone slider in the editor page is removed. The `fontSize` state in `handleSaveConfiguration` is updated to save `visualSettings.fontSize` parsed as a number.

**Current save path:**
```typescript
// display_settings.font_size: number
display_settings: { font_size: fontSize, ... }
```

**New save path (after migration):**
```typescript
// fontSize comes from visualSettings.fontSize (string like "16px")
display_settings: { font_size: parseInt(visualSettings.fontSize ?? '16'), ... }
```

This maintains backward compatibility with the backend schema (`display_settings.font_size` is still saved) while the UI control moves into the Typography sub-group.

### Anti-Patterns to Avoid

- **Deleting ThemeMarketplaceModal:** Credits page (`/overlays/[id]/credits`) still uses it — only remove the dynamic import and usage from the overlay editor page.
- **New localStorage key collision:** The 5 editor-level sections MUST use `editor-panel-sections-v1`, not `appearance-panel-sections-v1`. If the same key is shared, the sub-group states would corrupt the section states.
- **Hardcoded `STORAGE_KEY` in CollapsibleSection:** After adding the `storageKey` prop, ensure the hardcoded constant `STORAGE_KEY = 'appearance-panel-sections-v1'` becomes the default value of the prop (or is removed).
- **Breaking AppearancePanel's internal sections:** AppearancePanel's 7 sub-groups pass `id` to `CollapsibleSection` without specifying `storageKey`, so they correctly use the default `appearance-panel-sections-v1`. Do not change AppearancePanel.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Collapsible accordion UI | Custom open/close toggle | `@base-ui/react` Collapsible (already in CollapsibleSection) | Animation, accessibility, ARIA attributes handled |
| Theme list/filter state | Custom fetch + filter logic | `useThemeMarketplace` hook | Caching, GitHub API, favorites, tag filtering already implemented |
| Sticky footer layout | JS scroll position detection | CSS `flex + flex-1 overflow-y-auto` on content + `flex-shrink-0` on footer | Pure CSS, no JS needed |
| Source removal | Custom confirmation modal | Directly call `overlaysApi.deleteSource` (per CONTEXT.md decision: no confirmation) | Confirmed in phase decisions |

**Key insight:** This phase is almost entirely a reorganization of existing code. The risk is in breaking existing functionality by incorrect state wiring, not in building new logic.

## Common Pitfalls

### Pitfall 1: CollapsibleSection defaultOpen Not Respected After First Visit

**What goes wrong:** User opens the page for the first time; Theme and Sources sections should be open. But localStorage returns a stored value of `false` for those section IDs if the user has previously collapsed them — the stored value correctly overrides `defaultOpen`.

**Why it happens:** `defaultOpen` is a fallback only when no stored value exists. After user interacts with sections, stored values take over. This is the correct behavior — user preferences persist.

**How to avoid:** Correct behavior. No fix needed. The `defaultOpen` prop only controls the initial render before any user interaction.

**Warning signs:** If sections are always showing as closed even on first visit, the `readStoredSections()` call is returning a non-null value for the key. Ensure the `editor-panel-sections-v1` key starts with no value in fresh localStorage.

### Pitfall 2: Two Storage Keys Reading Each Other's Data

**What goes wrong:** Editor-level sections accidentally read from `appearance-panel-sections-v1`, or AppearancePanel sub-groups read from `editor-panel-sections-v1`. Section state corrupts — e.g., "theme" section opens in AppearancePanel if an ID collision occurs.

**Why it happens:** The `id` values for editor sections (`"theme"`, `"appearance"`, etc.) don't overlap with AppearancePanel sub-group IDs (`"typography"`, `"colors"`, `"background"`, `"visibility"`, `"sizing"`, `"platform-colors"`, `"events"`), but both sets write to the same localStorage key unless the `storageKey` prop is correctly threaded.

**How to avoid:** Pass `storageKey="editor-panel-sections-v1"` explicitly to all 5 top-level editor sections. Verify in tests that clicking a top-level section writes to `editor-panel-sections-v1`, not `appearance-panel-sections-v1`.

**Warning signs:** AppearancePanel sub-groups change their open state when top-level editor sections are toggled.

### Pitfall 3: fontSize Migration Breaks Saved Config

**What goes wrong:** After migration, `fontSize` state is removed from the page. But `handleSaveConfiguration` still tries to save `display_settings.font_size: fontSize`. TypeScript catches this if done correctly, but the saved value could become NaN or undefined.

**Why it happens:** `visualSettings.fontSize` is a string like `"16px"`. Parsing it as int requires stripping `"px"` first.

**How to avoid:** Use `parseInt(visualSettings.fontSize ?? '16')` (stripping "px" is handled by `parseInt` which stops at non-numeric characters). Or use `parseFloat(visualSettings.fontSize ?? '16')`.

**Warning signs:** `display_settings.font_size` becomes `NaN` in saved config; overlay renders with default font size.

### Pitfall 4: ThemeContent SSR/Dynamic Import Issues

**What goes wrong:** The new `ThemeContent` component uses `useThemeMarketplace` which fetches from GitHub API on mount. If the component renders server-side, it may error on missing `window`/`localStorage`.

**Why it happens:** `ThemeMarketplaceModal` was previously dynamically imported with `{ ssr: false }`. If `ThemeContent` is rendered directly (not via `dynamic()`), it could trigger SSR issues.

**How to avoid:** Wrap `ThemeContent` in a `dynamic()` call with `{ ssr: false }` at the point of use, OR verify `useThemeMarketplace` guards against SSR (it uses `useEffect` for fetching, so it is SSR-safe as long as `localStorage` access in the cache module is guarded with `typeof window !== 'undefined'`).

**Verification:** Check `frontend/src/lib/theme-marketplace/cache.ts` for `localStorage` guards. The existing `ThemeMarketplaceModal` dynamic import pattern confirms the `dynamic({ ssr: false })` approach is the correct one.

### Pitfall 5: SplitView Layout Conflicts

**What goes wrong:** The sticky footer approach (flex column layout) conflicts with SplitView's own layout expectations.

**Why it happens:** SplitView passes its children into a panel container. If SplitView's panel has `overflow-y-auto` on the outer container, adding `flex flex-col h-full` to the inner div breaks the layout.

**How to avoid:** Inspect `SplitView.tsx` to understand what classes/styles it applies to its children container before implementing the sticky footer. The safe approach is: use `position: sticky` on the footer within the scrollable container, rather than `flex flex-col h-full`. With `position: sticky; bottom: 0`, the footer sticks to the bottom of the scroll area without requiring flex layout changes.

**Warning signs:** Editor panel overflows viewport, or save button is hidden beneath content.

## Code Examples

Verified patterns from existing codebase:

### CollapsibleSection Current API
```typescript
// Source: frontend/src/components/appearance/CollapsibleSection.tsx
const STORAGE_KEY = 'appearance-panel-sections-v1'

export interface CollapsibleSectionProps {
  id: string
  title: string
  children: React.ReactNode
}

export function CollapsibleSection({ id, title, children }: CollapsibleSectionProps) {
  const [open, setOpen] = useState<boolean>(() => {
    const stored = readStoredSections()  // reads STORAGE_KEY
    return stored[id] ?? false           // false is the current default
  })
  // ...
}
```

### Extended CollapsibleSection API (Target)
```typescript
export interface CollapsibleSectionProps {
  id: string
  title: string
  children: React.ReactNode
  storageKey?: string        // defaults to 'appearance-panel-sections-v1'
  defaultOpen?: boolean      // defaults to false
}

export function CollapsibleSection({
  id,
  title,
  children,
  storageKey = 'appearance-panel-sections-v1',
  defaultOpen = false,
}: CollapsibleSectionProps) {
  const [open, setOpen] = useState<boolean>(() => {
    const stored = readStoredSections(storageKey)
    return stored[id] ?? defaultOpen
  })
  // handleOpenChange must use storageKey param too
}
```

### ThemeMarketplaceModal Content Structure (Lines to Extract)
```typescript
// Source: ThemeMarketplaceModal.tsx — the content inside the modal (lines ~196-291):
// - ThemeFilters component (search + tag filters)
// - Theme grid: themes.map(theme => <ThemeCard ... onApply={handleApply} />)
// - Loading/error/empty states
// The modal wrapper (fixed overlay, header, close button) is NOT extracted.
// The ESC key handler and body scroll lock are NOT extracted.
// onClose prop is NOT needed in ThemeContent — onApply is the only callback.
```

### Existing handleRemoveSource (No Change Needed)
```typescript
// Source: page.tsx (existing) — used directly in Sources section ✕ buttons
// The function already calls overlaysApi.deleteSource and refreshes sources state.
// No confirmation dialog needed per CONTEXT.md decision.
```

### handleSaveConfiguration — fontSize Migration Target
```typescript
// Before (current):
display_settings: {
  font_size: fontSize,  // number state
  ...
}

// After (target):
display_settings: {
  font_size: parseInt(visualSettings.fontSize ?? '16'),  // from visualSettings
  ...
}
// Remove standalone: const [fontSize, setFontSize] = useState(16)
// Remove standalone font size slider JSX from Behavior section
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `fontSize` as standalone number state + range slider | `visualSettings.fontSize` string in TypographyGroup | Phase 37 | Unifies all typography under AppearancePanel |
| ThemeMarketplace as modal triggered from CSS row | ThemeMarketplace inline in first section | Phase 37 | Theme is primary entry point; CSS editor demoted to Expert |
| Flat `space-y-6` panel content | 5 CollapsibleSections | Phase 37 | Users see Theme + Sources by default; advanced controls hidden |

## Open Questions

1. **SplitView sticky footer compatibility**
   - What we know: SplitView renders children in a panel; the panel likely has `overflow-y-auto`
   - What's unclear: Whether `position: sticky; bottom: 0` will work within SplitView's panel, or whether flex layout is needed
   - Recommendation: Inspect `SplitView.tsx` at plan-execution time and use `sticky` positioning as first approach; fall back to flex column if sticky fails

2. **ThemeContent compact sizing**
   - What we know: CONTEXT.md says "compact sizing" for theme cards in the sidebar context
   - What's unclear: Whether ThemeCard supports a size variant prop or needs custom className overrides
   - Recommendation: Pass a `compact` prop or `className` override to ThemeCard at execution time; if ThemeCard doesn't accept it, wrap in a CSS scale transform or reduce column count to 1

3. **AddSourceForm in Sources section**
   - What we know: The current editor uses `<AddSourceForm>` component. CONTEXT.md says "inline form within the section: platform selector + channel input fields"
   - What's unclear: Whether to reuse `AddSourceForm` as-is or simplify it for the inline context
   - Recommendation: Reuse `AddSourceForm` as-is (it already renders inline) — adding a toggle to show/hide it is the only new behavior needed

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Vitest (unit project) |
| Config file | `frontend/vitest.config.ts` |
| Quick run command | `cd /home/moersener/Hobby/worktree/all-chat/frontend && npx vitest run --project unit` |
| Full suite command | `cd /home/moersener/Hobby/worktree/all-chat/frontend && npx tsc --noEmit && npx vitest run --project unit` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| EDUX-01 | ThemeContent renders theme cards inline (no modal wrapper) | unit | `npx vitest run --project unit --reporter=verbose` | ❌ Wave 0 |
| EDUX-02 | CollapsibleSection accepts `storageKey` prop; top-level sections use `editor-panel-sections-v1` | unit | `npx vitest run --project unit src/components/appearance/__tests__/CollapsibleSection.test.tsx` | ✅ (needs update) |
| EDUX-03 | Expert section starts collapsed; CSS editor not visible by default | unit | `npx vitest run --project unit` | ❌ Wave 0 |
| EDUX-04 | All 7 AppearancePanel sub-groups render (unchanged from Phase 35) | unit | `npx vitest run --project unit src/components/appearance/__tests__/` | ✅ (existing) |

### Sampling Rate
- **Per task commit:** `cd /home/moersener/Hobby/worktree/all-chat/frontend && npx vitest run --project unit`
- **Per wave merge:** `cd /home/moersener/Hobby/worktree/all-chat/frontend && npx tsc --noEmit && npx vitest run --project unit`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `src/components/appearance/__tests__/CollapsibleSection.test.tsx` — update existing test: add assertion that `storageKey` prop causes writes to `editor-panel-sections-v1` instead of `appearance-panel-sections-v1` (covers EDUX-02)
- [ ] `src/components/theme-marketplace/__tests__/ThemeContent.test.tsx` — new file: covers EDUX-01 (theme cards render inline, no modal overlay element)

*(Existing AppearancePanel sub-group tests cover EDUX-04 with no changes needed)*

## Sources

### Primary (HIGH confidence)
- Direct codebase inspection — `CollapsibleSection.tsx`, `AppearancePanel.tsx`, `TypographyGroup.tsx`, `ThemeMarketplaceModal.tsx`, `page.tsx` (overlays/[id]), `visual-settings.ts`
- `36-03-PLAN.md` — confirmed exact state shape: `visualSettings`, `parsedThemeSettings`, `applyThemeImmediately`, `handleResetToTheme`
- `37-CONTEXT.md` — locked decisions, exact section order, storage key names

### Secondary (MEDIUM confidence)
- `vitest.config.ts` + `__tests__/` directory inspection — test infrastructure verified as Vitest unit + jsdom environment

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries already in project, versions confirmed by existing code
- Architecture: HIGH — patterns derived directly from existing CollapsibleSection, ThemeMarketplaceModal, and page.tsx code
- Pitfalls: HIGH — identified from code structure (STORAGE_KEY hardcoding, SplitView layout, fontSize type mismatch)

**Research date:** 2026-03-19
**Valid until:** 2026-04-18 (stable internal codebase — changes only when editor page is modified)
