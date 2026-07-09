# Follow-up: true per-card style variation for overlay themes

## Context
Themes like **Trading Card** and **Sticky Notes** look best when consecutive
messages don't all render identically. Today we can only vary cards by
**position** using CSS `:nth-child()` cycling:

- Trading Card cycles 3 foil palettes via `--foil` on `:nth-child(3n+2)` / `:nth-child(3n)`.
- Sticky Notes alternates tilt + paper colour via `:nth-child(even)` / `:nth-child(3n)`.

This is deterministic by *position in the list*, not by the message — so as
messages scroll, a given message's look changes, and the variety is a fixed
repeating pattern rather than random.

## Gap
There is **no per-message random/stable seed** a CSS-only theme can key off.
The overlay DOM exposes `data-platform`, `data-username`, `data-message-id`,
but nothing a theme can turn into a stable "variant N of M".

## Proposed follow-up
Emit a **stable variant index per message** from the overlay renderer so themes
can vary styling per message, consistently across re-renders:

1. In the live overlay renderer (`frontend/src/app/overlay/[id]/page.tsx`) and the
   marketplace preview (`frontend/src/components/theme-marketplace/ThemePreview.tsx`),
   compute a small hash of `message.id` (e.g. sum char codes mod N) and set
   `data-variant={hash}` on the `.chat-message` element (N ~ 4–6).
2. Document `[data-variant="0..N"]` as a stable theme hook (in `/docs` custom-CSS
   section + the theme authoring notes).
3. Update variety-driven themes to key off `[data-variant="k"]` instead of
   `:nth-child`, e.g. `.chat-message[data-variant="2"] { --foil: ... }`.

Deterministic (same message → same variant), varied across messages, and
CSS-only for theme authors. Keep `:nth-child` as the fallback for themes that
don't opt in.

## Effort
Small: one hashing helper + one attribute in two render paths + docs. No schema
or backend change.
