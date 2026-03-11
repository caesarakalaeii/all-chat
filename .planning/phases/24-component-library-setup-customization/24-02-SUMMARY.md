---
phase: 24-component-library-setup-customization
plan: 02
subsystem: ui
tags: [react, typescript, tailwind, cva, storybook, base-ui, design-tokens]

requires:
  - phase: 24-component-library-setup-customization
    provides: plan 01 story stubs with inline placeholder components for Card, Input, Skeleton
  - phase: 23-design-token-system-foundation
    provides: globals.css with CSS custom properties (--color-surface, --color-surface-2, --color-border, --color-ring) used as Tailwind tokens

provides:
  - Card component with CVA interactive variant (hover:scale-[1.02] + shadow-lg)
  - Input component wrapping @base-ui/react/input with focus-visible ring and size variants
  - Skeleton component with animate-pulse bg-surface-2
  - Button gradient variant applying #9146FF to #69C9D0 linear-gradient
  - All dark: orphan classes removed from button.tsx (dark-only app, never activate)
  - Stories for Card, Input, Skeleton, Button importing from real @/components/ui/* components

affects:
  - 24-03 (badge, dialog, toast implementations follow same CVA + base-ui wrapping pattern)
  - 24-05 (a11y enforcement depends on stories passing, all Card/Input/Skeleton/Button stories now pass)
  - Phase 25 page migration (uses Card, Input, Button components established here)

tech-stack:
  added: []
  patterns:
    - "CVA interactive boolean variant pattern: interactive: { true: 'hover:scale...', false: '' } for optional hover effects"
    - "Omit<Primitive.Props, 'size'> to resolve native HTML size (number) vs CVA size variant (string) type conflict"
    - "Input wraps @base-ui/react/input InputPrimitive with data-slot='input' — same pattern as Button wrapping ButtonPrimitive"

key-files:
  created:
    - frontend/src/components/ui/card.tsx
    - frontend/src/components/ui/input.tsx
    - frontend/src/components/ui/skeleton.tsx
    - frontend/src/components/ui/__tests__/card.test.ts
    - frontend/src/components/ui/__tests__/input.test.ts
    - frontend/src/components/ui/__tests__/skeleton.test.ts
    - frontend/src/components/ui/__tests__/button.test.ts
  modified:
    - frontend/src/components/ui/button.tsx
    - frontend/src/stories/Card.stories.tsx
    - frontend/src/stories/Input.stories.tsx
    - frontend/src/stories/Skeleton.stories.tsx
    - frontend/src/stories/Button.stories.ts

key-decisions:
  - "Use Omit<InputPrimitive.Props, 'size'> to prevent type conflict between native HTML input size (number) and CVA size variant (string union)"
  - "CVA interactive variant uses boolean key (true/false strings) not named strings — enables cleaner prop API: interactive={true} vs interactive='interactive'"

patterns-established:
  - "Primitive wrapping pattern with data-slot: wrap base-ui primitive, add data-slot, merge CVA variants via cn()"
  - "CVA + Omit for HTML attribute conflicts: when CVA variant name clashes with native HTML attribute, use Omit<NativeProps, 'conflicting-attr'>"

requirements-completed: [COMP-01, COMP-02, COMP-03, COMP-04, COMP-05]

duration: 5min
completed: 2026-03-10
---

# Phase 24 Plan 02: Card, Input, Skeleton, and Button Gradient Summary

**Card/Input/Skeleton components plus Button gradient variant — four production-ready CVA components following the @base-ui/react wrapping pattern with design token CSS classes**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-10T21:57:45Z
- **Completed:** 2026-03-10T22:03:00Z
- **Tasks:** 3
- **Files modified:** 12

## Accomplishments

- Built Card (CVA interactive hover scale), Input (@base-ui/react/input with focus ring), Skeleton (animate-pulse) following button.tsx pattern exactly
- Added gradient variant to Button (linear-gradient #9146FF→#69C9D0) and removed all orphan dark: classes
- Updated Card, Input, Skeleton, Button stories to import from real @/components/ui/* — all 30 Storybook tests pass

## Task Commits

Each task was committed atomically:

1. **Task 1 RED: Add failing tests for Card, Input, Skeleton** - `902d8f52f` (test)
2. **Task 1 GREEN: Implement Card, Input, and Skeleton components** - `a88e6c1f5` (feat)
3. **Task 2 RED: Add failing tests for Button gradient and dark: removal** - `e53bdc1fc` (test)
4. **Task 2 GREEN: Add gradient variant and remove dark: classes** - `801ca870b` (feat)
5. **Task 3: Update stories to import real components** - `9d9fecc25` (feat)

## Files Created/Modified

- `frontend/src/components/ui/card.tsx` - Card with CVA interactive boolean variant (hover:scale-[1.02] + shadow-lg)
- `frontend/src/components/ui/input.tsx` - Input wrapping @base-ui/react/input, focus ring matches Button, default/sm sizes
- `frontend/src/components/ui/skeleton.tsx` - Skeleton with animate-pulse bg-surface-2, className passthrough
- `frontend/src/components/ui/button.tsx` - Added gradient variant, removed 5 dark: class instances
- `frontend/src/components/ui/__tests__/card.test.ts` - Unit tests for Card CVA variant behavior
- `frontend/src/components/ui/__tests__/input.test.ts` - Unit tests for Input size variants and focus ring
- `frontend/src/components/ui/__tests__/skeleton.test.ts` - Unit tests for Skeleton export
- `frontend/src/components/ui/__tests__/button.test.ts` - Unit tests for gradient variant and dark: removal
- `frontend/src/stories/Card.stories.tsx` - Imports from @/components/ui/card, adds WithContent story
- `frontend/src/stories/Input.stories.tsx` - Imports from @/components/ui/input, adds Small story
- `frontend/src/stories/Skeleton.stories.tsx` - Imports from @/components/ui/skeleton, uses className for dimensions
- `frontend/src/stories/Button.stories.ts` - Imports from @/components/ui/button, adds Gradient story

## Decisions Made

- Used `Omit<InputPrimitive.Props, 'size'>` to resolve type conflict between native HTML `<input>` size attribute (number) and CVA size variant (string union "default" | "sm"). This is the correct pattern for any future CVA variant names that clash with native HTML attributes.
- CVA boolean variant `interactive` uses string keys `"true"` and `"false"` per CVA convention — in JSX this renders as `interactive={true}` (boolean prop) which CVA maps to the `"true"` key.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed TypeScript type conflict between native HTML size and CVA size variant**
- **Found during:** Task 3 (story updates)
- **Issue:** `InputPrimitive.Props` inherits `size?: number` from HTML input element. Intersecting with `VariantProps<typeof inputVariants>` created conflicting `size` type (number vs "default" | "sm" | null | undefined), causing TS2322 error in Input.stories.tsx
- **Fix:** Changed Input props type from `InputPrimitive.Props & VariantProps<...>` to `Omit<InputPrimitive.Props, 'size'> & VariantProps<...>`
- **Files modified:** frontend/src/components/ui/input.tsx
- **Verification:** `npx tsc --noEmit` returns zero errors
- **Committed in:** 9d9fecc25 (Task 3 commit)

---

**Total deviations:** 1 auto-fixed (1 type conflict bug)
**Impact on plan:** Fix required for TypeScript correctness. No scope creep.

## Issues Encountered

- Playwright browsers not installed for Storybook vitest tests — ran `npx playwright install chromium` to install (one-time setup, not tracked as deviation).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All four components (Card, Input, Skeleton, Button w/ gradient) production-ready with design tokens
- Pattern established for @base-ui/react wrapping with CVA — Plan 03 (Badge, Dialog, Toast) can follow same pattern
- 30 Storybook tests passing (up from 0 in Plan 01)
- Unit test suite now covers all UI component CVA behavior (15 tests)

---
*Phase: 24-component-library-setup-customization*
*Completed: 2026-03-10*
