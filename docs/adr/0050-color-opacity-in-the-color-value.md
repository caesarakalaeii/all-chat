# ADR-0050: Visual-customizer colors carry their opacity in the value (8-digit hex)

**Date**: 2026-08-16
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

Every color in the overlay editor's appearance panels (message, username,
timestamp, bubble background, overlay background, bubble border, pronoun pill,
five platform accents) is stored in `visual_settings` as a 6-digit hex and
emitted by `visualSettingsToCss` as a `--chat-*` / `--platform-*` custom
property. Only the two background colors had an opacity control, and that
opacity lived in a **sibling field** (`overlayBgOpacity`, `bubbleBgOpacity`,
"0"–"1" strings) which was combined with the hex into an `rgba()` **inline
style** on the overlay container and chat bubble.

That split had two consequences:

1. **No other color could be made translucent at all** — the remaining nine
   pickers had no opacity control, so "make the bubbles transparent" (reported
   by a user running a variant of the Minimal theme) had no answer in the UI and
   had to be solved with hand-written CSS.
2. **The sibling opacity did not reach themes.** A theme rule such as
   `background: var(--chat-bubble-bg-color, rgba(0,0,0,.85)) !important` reads
   only the color variable, and its `!important` beats the inline `rgba()`
   style. `--chat-bubble-bg-opacity` was emitted but consumed by nothing — no
   bundled theme references it. So with any theme applied, the bubble-background
   opacity slider silently did nothing.

## Decision

**Opacity is part of the color value: a non-opaque color is stored as an 8-digit
hex (`#rrggbbaa`). Every color picker gets an opacity slider; no new sibling
opacity fields are introduced.**

- `lib/utils/hex-alpha.ts` owns the format (`normalizeHex`, `alphaFromHex`,
  `stripAlpha`, `hexWithAlpha`, `withLegacyOpacity`). Fully opaque colors stay
  6-digit, so untouched settings and hand-authored theme CSS keep the shape
  everyone already writes.
- `ColorPickerControl` renders the opacity slider unconditionally and writes the
  alpha into the hex; its swatch sits on a checkerboard (`.alpha-checkerboard`)
  so translucency is visible. The `showOpacity` / `opacity` / `onOpacityChange`
  props are gone.
- The alpha therefore travels into **both** consumers: the inline
  bubble/overlay styles (`hexToRgba` now reads a hex alpha channel, which wins
  over the legacy field) and the `--chat-*` custom properties — so theme CSS
  reading `var(--chat-bubble-bg-color)` honours it, `!important` and all.
- **Legacy fields stay readable, and are cleared on write.**
  `overlayBgOpacity` / `bubbleBgOpacity` are marked deprecated; `BackgroundGroup`
  folds a stored value into the slider position (`withLegacyOpacity`) and patches
  the field to `undefined` when the color changes, so a setting is never dimmed
  twice.

Alternatives considered: adding a `*Opacity` field per color (≈10 new keys, and
`visualSettingsToCss` would have to compose `rgba()` anyway to reach themes —
same end state with more state), or storing `rgba()` strings (breaks the hex
text input and every hand-authored theme's expectation of a hex).

## Consequences

- Any color in the customizer can be made translucent or fully transparent,
  including with a theme applied. Bubble background at 0% is now the supported
  answer to "transparent bubbles" — no custom CSS needed.
- Theme and custom CSS authors may receive an 8-digit hex from any `--chat-*`
  color variable. Modern browsers (and OBS's CEF) accept it anywhere a color is
  valid; CSS that string-manipulates a color variable (e.g. splicing it into
  `rgba(...)`) would not — no bundled theme does.
- `visual_settings` is schemaless JSON (`map[string]any` in overlay-manager), so
  there is no backend or migration work; older rows keep working unchanged.
- An overlay page still running a pre-deploy JS bundle would not understand an
  8-digit hex in the inline-style path (it falls back to the Tailwind default
  background) until it reloads. Only affects colors edited during a deploy.
