# ADR-0043: Preloaded, Live-Preview Custom CSS Editor with Fork-on-Delete

**Date**: 2026-07-24
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation)
**Deciders**: caesarakalaeii

## Context

The overlay editor's Custom CSS field (`MonacoCSSEditor`, ADR-0040) let users type raw CSS overrides, but was effectively unusable for the streamers it targets (external feedback, 2026-07):

1. **Blank box, no starting point.** Applying a marketplace theme cleared the editor and referenced the theme only by `theme_id`; a user who wanted to tweak a theme saw nothing to edit and no idea which selectors to target.
2. **No live feedback.** Raw custom CSS reached the preview iframe only after Save + reload (the WYSIWYG controls, by contrast, `postMessage` on every change).
3. **No guardrails.** Monaco's diagnostics were mounted but never surfaced.

The prior design deliberately kept `custom_css` as raw overrides so theme CSS resolves from the build bundle at render (`theme_id`), which is what lets a theme fix reach every overlay on the next deploy without a per-overlay migration. Any fix had to preserve that property.

## Decision

**Preload the applied theme's full CSS into the editor, live-preview edits as you type, and persist a semantic diff — only the declarations the user changed — so bundled theme fixes keep reaching every rule the user did not touch.**

- **Preload.** Applying a theme and loading an overlay with no saved override both set the editor content to the bundled theme's CSS, kept alongside a pristine baseline.
- **Live preview.** Editing debounce-pushes the content to the preview iframe (300 ms), guarded so CSS with unbalanced braces is held back (a mid-edit unclosed rule cannot blank the preview). The embed rewrites Google-Fonts `@import`s through the same-origin font proxy on this path too, so theme fonts load under CSP during editing. Gate the push on Monaco markers instead? Rejected: markers lag the keystroke, making the gate racy.
- **Diff-based persistence** (`frontend/src/lib/utils/theme-css-diff.ts`, postcss, unit-tested). At save time the editor is diffed against the pristine theme; `theme_id` is always retained and the render stays uniform (bundled theme first, then `custom_css` last with the delta forced `!important`). Modes, encoded as a leading marker comment:
  - **linked** — editor equals the theme → `custom_css` empty; the overlay fully tracks the bundle.
  - **diff** — user only changed values or added rules → store only those declarations. Untouched rules stay owned by the theme and update on deploy — the whole point.
  - **fork** — user deleted a theme declaration/rule. CSS layering cannot undo an earlier rule, so that overlay stores a full copy and stops auto-updating (the honest, exact choice). Deletion is the only fork trigger; changing or adding never forks.
  A stored "diff always" alternative was rejected: a deleted declaration would fall back to the browser default rather than the overlay's own base look. On reload, a stored diff is merged back onto the *current* theme so the editor always shows the latest theme plus the user's changes; legacy pre-marker `custom_css` is shown verbatim and treated as a full copy until the next save re-diffs it (no migration).
- **Reset to theme** restores the pristine CSS (re-linking, mode `linked`); with no theme the button is "Clear".
- **Validation.** `MonacoCSSEditor` forwards Monaco's markers via `onValidate`; the editor shows "No CSS problems detected" or an error/warning list with line numbers.

Alternatives considered: full copy on any edit (the first iteration; permanently detached even a one-colour change from theme fixes — superseded by the diff); a read-only theme reference plus separate overrides box (users cannot edit theme rules, which is what was asked for).

## Consequences

- Streamers can directly edit the CSS behind their theme and see the preview update as they type; theme fixes keep reaching customised overlays except those that *delete* theme rules.
- `postcss` is used only by the editor route, never the public overlay render (which still injects strings).
- Custom CSS remains owner-authored CSS on a public overlay; the existing `style-src` CSP and the preview's `scopeCustomCss` scoping remain the blast-radius control.
- No new premium gate: this enhances the existing ungated tool.
