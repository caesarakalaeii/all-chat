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
type, and persist a *semantic diff* — only the declarations the user changed — so
bundled theme fixes keep reaching every rule the user did not touch.**

- **Preload.** Applying a theme (`applyThemeImmediately`) and loading an overlay
  with no saved override both set the editor content to the bundled theme's CSS
  (`getBundledTheme(theme_id).css`), kept alongside a `pristineThemeCss` baseline.
  The editor now shows real, editable CSS instead of a blank box.
- **Live preview.** Editing debounce-pushes the editor content to the preview
  iframe (`CUSTOM_CSS_UPDATE`, 300 ms), guarded so CSS with unbalanced braces is
  held back (a mid-edit unclosed rule can't blank the preview). The embed rewrites
  Google-Fonts `@import`s through the same-origin font proxy on this path too, so
  theme fonts load under CSP during live editing.
- **Diff-based persistence (`theme-css-diff.ts`, postcss, unit-tested).** At save
  time we diff the editor against the pristine bundled theme and persist one of
  three modes (encoded as a leading marker comment in `custom_css`; `theme_id` is
  always retained and the render is uniform — bundled theme first, then `custom_css`
  last with the delta's `!important` winning):
  - **linked** — editor equals the theme → `custom_css` empty → overlay fully tracks
    the bundle.
  - **diff** — user only *changed values / added rules* → store **only those
    declarations** (each forced `!important` to beat the theme). Every untouched
    rule stays owned by the theme and updates on deploy — this is the whole point.
  - **fork** — user *deleted* a theme declaration/rule. CSS layering cannot "un-set"
    an earlier rule, so we can't express a deletion as an additive override. That
    overlay stores a **full copy** and stops auto-updating (the honest, exact
    choice). Deletion is the only trigger; changing/adding never forks.
  On reload, a stored diff is **merged back onto the current theme** so the editor
  always shows the latest theme plus the user's changes — a theme fix we shipped
  even appears in their editor.
- **Reset to theme.** Restores the editor to `pristineThemeCss` (re-linking the
  overlay to the bundle, mode `linked`); with no theme, the button is "Clear".
- **Validation tips.** `MonacoCSSEditor` forwards Monaco's markers via `onValidate`;
  the editor surfaces "✓ No CSS problems detected" or an error/warning list with
  line numbers. Syntax highlighting is Monaco's built-in CSS mode.

The diff/merge logic lives in `frontend/src/lib/utils/theme-css-diff.ts`
(`computeThemeCssDiff`, `reconstructEditorCss`) and the "is it customised?" pill
check in `custom-css.ts` (`isCustomCssForked`, `markerSeverityToLevel`), so the
decisions are pure and unit-tested rather than buried in the 3.6k-line editor page.

## Considered Alternatives

- **Full copy on any edit ("fork-on-edit").** The first iteration stored the whole
  edited theme whenever it diverged. Simple, but *any* customisation — even changing
  one colour — permanently detached the overlay from theme fixes. Superseded by the
  semantic diff, which keeps updates flowing for everything the user didn't touch.
- **Read-only theme reference + separate overrides box.** Preserves theme links but
  the user cannot edit theme rules directly, which is what was asked for. Rejected in
  favour of the "edit the CSS you see" model; the diff gives the same
  updates-keep-flowing property without a separate box.
- **Diff with `revert` for deletions (always diff-based).** Would keep *every* overlay
  updating, but a deleted declaration would fall back to the browser default rather
  than the overlay's own base look — surprising. Rejected in favour of auto-forking
  only the overlays that actually delete theme rules (exact fidelity where it matters,
  updates everywhere else).
- **Gate the live push on Monaco error markers.** The marker state lags the keystroke,
  making the gate racy. Rejected in favour of the brace-balance guard plus advisory
  diagnostics (invalid declarations are harmlessly dropped by the browser anyway).

## Consequences

- Streamers see and can directly edit the CSS behind their theme, with the preview
  updating as they type — the reported blockers are removed.
- **Theme fixes keep reaching customised overlays.** Changing values / adding rules
  stores only the delta, so any rule the user didn't touch still updates on deploy.
  Only overlays that *delete* theme rules detach (full copy), and only those.
- Legacy `custom_css` saved before this ADR (no marker) is shown verbatim and treated
  as a full copy until the next save re-diffs it — no migration needed.
- `postcss` (already a dependency) is used to parse/diff/merge; it is imported only by
  the editor route, never by the public overlay render (which still injects strings).
- Custom CSS remains owner-authored CSS injected on a public overlay; the existing
  CSP `style-src` + preview `scopeCustomCss` constraints (ADR-0040, M10/M11) are
  unchanged and still the blast-radius control.
- The Advanced editor is an existing, non-gated tool; this ADR enhances it and adds
  no new premium gate.
