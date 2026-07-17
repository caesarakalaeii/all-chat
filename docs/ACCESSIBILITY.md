# Accessibility

The app UI targets **WCAG 2.2 AA**. This is a living reference for the conformance scope, the CI gates that keep it from regressing, and the deliberate design decisions behind a few non-obvious calls. Update it in the same PR as any change that affects these facts.

## Scope

**In scope (app UI):** landing, docs, dashboard, overlay editor, settings (streamer + viewer), admin, upgrade, auth pages, and the viewer participate page.

**Out of scope (broadcast surfaces):** everything rendered inside OBS — `/overlay/[id]` and its view/poll/prediction/credits render pages, and overlay marketplace theme CSS. These are streamer-chosen broadcast art:

- Overlay chat themes have their own deliberate contrast floor of 3.0:1 (see `frontend/tests/e2e/theme-contrast.spec.ts`), below AA on purpose — it exists to catch invisible-text theme bugs, not to police stream aesthetics.
- Viewer-defined chatter name colors are never contrast-adjusted or overridden.
- The axe helper (`frontend/tests/e2e/a11y-helpers.ts`) codifies this exemption once via themed-chat selector exclusions; never widen it into a blanket disable.

## CI gates (`.github/workflows/frontend-a11y.yml`, runs on every frontend PR)

| Job | What it enforces |
|---|---|
| A11y lint + token contrast | `eslint.a11y.config.mjs` (jsx-a11y **strict**, inline disables ignored) with an **empty** suppressions file — any violation fails. Plus the design-token contrast lock (`frontend/src/app/__tests__/token-contrast.test.ts`): text tokens ≥4.5:1 on every surface tier, platform colors ≥3:1, conversion math pinned to the rendered `#020204` background. |
| Storybook axe | Every story runs axe with violations as errors (`.storybook/preview.ts`, `a11y: { test: 'error' }`). New primitives and components need stories. |
| Axe page smoke | `frontend/tests/e2e/a11y.spec.ts` scans the main pages (WCAG 2.2 AA tags) against `a11y-baseline.json` — a **shrink-only** baseline of pre-existing rule ids. New rule ids fail immediately; prune entries in the PR that fixes them (the spec logs prunable entries). |

Both ratchet files may only ever shrink: `frontend/eslint.a11y.suppressions.json` (currently `{}`) and `frontend/tests/e2e/a11y-baseline.json`.

## Contracts for new code

- **Every page `<main>`** gets `id="main-content" tabIndex={-1}` — the root layout's skip link targets it.
- **Forms**: use `ui/field` (auto-wired label/description/error associations) or explicit `htmlFor` via `useId`. Placeholders are hints, never the only label. Hints/units/errors associate via `aria-describedby`; error states set `aria-invalid`.
- **Modals**: always `ui/dialog`; destructive confirmations always `ui/alert-dialog` (no outside-press dismissal, cancel first in DOM order). Never hand-roll `fixed inset-0` modals.
- **Toasts/status**: one system — `toastManager` from `lib/toast`. Loading states get `role="status"` (+ `ui/visually-hidden` label when the state has no visible text); inline error banners get `role="alert"`.
- **Motion**: CSS animation is globally collapsed under `prefers-reduced-motion`; JS-driven motion must check `hooks/useReducedMotion`. Anything auto-advancing needs a visible pause control (2.2.2).
- **Targets**: interactive elements ≥24×24px **element box** — axe measures the element's bounding box, so pseudo-element hit areas do not count. Draw small visuals as inner spans/pseudo-elements inside a 24px control.
- Repo lint additionally enforces `focus-visible:` (never bare `focus:`), no template-literal `className`, slate over gray.

## Deliberate decisions (don't "fix" these without reading why)

- **Split-pane dividers use `role="slider"`, not the APG window-splitter `separator`** (`ResizableSplit`, `SplitView`): aria-query models `separator` as non-interactive structure, so a focusable separator cannot pass the strict lint gate. Slider carries the same value/min/max/orientation and arrow-key semantics and announces the split percentage. Each divider also has ±10% step buttons — WCAG 2.5.7 requires a single-pointer no-drag alternative; keyboard alone does not satisfy it.
- **3.3.8 Accessible Authentication passes by design**: login is OAuth-only (Twitch/YouTube/Kick) — no passwords, no CAPTCHA, no cognitive tests. If a password or CAPTCHA flow is ever added, it must be re-assessed.
- **The Monaco custom-CSS editor captures Tab** (code editors legitimately do): the visible hint next to it documents the exit (Ctrl+M toggles tab capture; Escape then Tab leaves). If that ever proves insufficient for screen-reader users, the fallback is a plain-textarea toggle.
- **`--color-text-dim` and `--color-text-sub`** were raised (0.575/0.60 oklch lightness) so tertiary/secondary text clears 4.5:1 on every surface tier; the token test locks this. Don't lower them.

## Manual test scripts (run per significant UI change)

**Keyboard:** load page → first Tab reveals the skip link → Enter lands focus in main → Tab through everything (visible focus ring, logical order, no traps — Monaco per its documented exit) → every dialog: open, interact, Escape, focus returns to trigger → operate switches (Space), sliders/dividers (arrows + step buttons), comboboxes, color pickers (popover + hex input).

**Screen reader (NVDA/Firefox primary):** landmark and heading navigation are coherent; forms mode announces name/role/state/description for every field; validation errors announce and are re-findable; toasts announce without moving focus.
