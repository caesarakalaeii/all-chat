# ADR-0043: Preloaded, live-preview Custom CSS editor with fork-on-edit

**Date**: 2026-07-24
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

The overlay editor's Advanced → Custom CSS field (`MonacoCSSEditor`, ADR-0040) let
users type raw CSS overrides, but three things made it effectively unusable for
the streamers it is meant for (external feedback, 2026-07):

1. **Blank box, no starting point.** Applying a marketplace theme *cleared* the
   editor (`setCustomCss('')`) and referenced the theme only by `theme_id`. A
   user who wanted to tweak a theme saw an empty editor and had no idea which
   selectors to target — *"I don't understand how I can make my own custom CSS."*
2. **No live feedback.** Raw custom CSS was pushed to the preview iframe only when
   a theme was applied; typing in the editor updated React state but sent nothing
   to the preview. Edits appeared only after **Save + iframe reload** — *"even if
   I follow the guide, nothing happens."* (The WYSIWYG controls, by contrast,
   `postMessage` on every change.)
3. **No guardrails.** Monaco's CSS language service was mounted but its
   diagnostics were never surfaced, and a half-typed rule could reach the preview.

The prior design deliberately kept `custom_css` as *raw overrides only* so that
theme CSS resolved from the build bundle at render (`theme_id`) — this is what
lets a theme fix reach every overlay on the next deploy without a per-overlay
data migration (see `bundled-themes.ts`). Any fix had to preserve that property
for the common "I just picked a theme" case.

## Decision

**Preload the applied theme's full CSS into the editor, live-preview edits as you
type, and only persist a copy when the user actually diverges from the theme
("fork-on-edit").**

- **Preload.** Applying a theme (`applyThemeImmediately`) and loading an overlay
  with no saved override both set the editor content to the bundled theme's CSS
  (`getBundledTheme(theme_id).css`), kept alongside a `pristineThemeCss` baseline.
  The editor now shows real, editable CSS instead of a blank box.
- **Live preview.** Editing debounce-pushes the editor content to the preview
  iframe (`CUSTOM_CSS_UPDATE`, 300 ms), guarded so CSS with unbalanced braces is
  held back (a mid-edit unclosed rule can't blank the preview). The embed rewrites
  Google-Fonts `@import`s through the same-origin font proxy on this path too, so
  theme fonts load under CSP during live editing.
- **Fork-on-edit persistence.** `custom_css` is persisted **only when the editor
  content differs from `pristineThemeCss`** (`persistedCustomCss`, unit-tested).
  An untouched theme saves an empty override and stays linked to `theme_id`, so it
  keeps receiving bundled theme fixes on deploy. Editing detaches this overlay
  onto its own saved copy. `theme_id` is retained (for the status pill and
  "Reset to theme"); on the live overlay the fork's `custom_css` layers last and
  wins, so the frozen copy is what renders.
- **Reset to theme.** Restores the editor to `pristineThemeCss` (re-linking the
  overlay to the bundle); with no theme, the button is "Clear".
- **Validation tips.** `MonacoCSSEditor` forwards Monaco's markers via `onValidate`;
  the editor surfaces "✓ No CSS problems detected" or an error/warning list with
  line numbers. Syntax highlighting is Monaco's built-in CSS mode.

The fork/preload logic lives in `frontend/src/lib/utils/custom-css.ts`
(`isCustomCssForked`, `persistedCustomCss`, `markerSeverityToLevel`) so the
decision is pure and unit-tested rather than buried in the 3.6k-line editor page.

## Considered Alternatives

- **Read-only theme reference + separate overrides box.** Preserves theme links
  perfectly (overrides never copy the theme) but the user cannot edit theme rules
  directly, which is exactly what was asked for. Rejected in favour of the more
  direct "edit the CSS you see" model; the auto-update loss is scoped to overlays
  the user *deliberately* forks.
- **Always copy the theme into `custom_css` on preload.** Simplest, but re-creates
  the per-overlay theme-copy anti-pattern for every overlay that merely opens the
  Advanced tab, permanently detaching them from theme fixes. Rejected — fork only
  on real edits.
- **Gate the live push on Monaco error markers.** The marker state lags the
  keystroke, making the gate racy. Rejected in favour of the brace-balance guard
  plus advisory diagnostics (invalid declarations are harmlessly dropped by the
  browser anyway).

## Consequences

- Streamers see and can directly edit the CSS behind their theme, with the preview
  updating as they type — the reported blockers are removed.
- Untouched themed overlays are unchanged on the wire (`custom_css` stays empty)
  and keep auto-updating; only edited overlays store a copy, and accept that they
  no longer track bundled theme fixes until "Reset to theme".
- Custom CSS remains owner-authored CSS injected on a public overlay; the existing
  CSP `style-src` + preview `scopeCustomCss` constraints (ADR-0040, M10/M11) are
  unchanged and still the blast-radius control.
- The Advanced editor is an existing, non-gated tool; this ADR enhances it and adds
  no new premium gate.
