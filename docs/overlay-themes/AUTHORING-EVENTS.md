# Styling events in a theme

Events — subs, gift bombs, raids, bits, channel-point redemptions, Super Chats,
follows — are chat rows with an `.event-message` wrapper instead of
`.chat-message`. If your theme styles only `.chat-message` (or only the chat
bubble), events fall back to the **platform default**: a scaled-up card with a
gold or purple gradient, a glowing border, a bouncing entrance and a 36px emoji.
In a minimal or retro theme that reads as a different product dropping into the
overlay.

Every theme shipped in `docs/overlay-themes/` styles its events. If you add one,
style yours too — the `bundled-themes-events` test fails otherwise.

---

## The short version

Set tokens, not rules. All of the default event chrome reads `--event-*` custom
properties, so a theme re-skins events from one block:

```css
.event-message {
  --event-scale: 1; /* no scale-up */
  --event-min-height: 0; /* no 100px floor */
  --event-animation: none; /* no bounce/glow loop */
  --event-icon-animation: none; /* no pulsing emoji */
  --event-decor: none; /* no raid sweep overlay */

  --event-title-color: #a5b4fc; /* your palette */
  --event-accent-subscription: #818cf8;
}
```

Because theme CSS is unlayered and the defaults live in `@layer
marketplace-themes`, your token values win **without** `!important`.

### Flatten events into ordinary chat rows

For minimal/inline themes, the whole card can be removed in eight declarations:

```css
.event-message {
  --event-bg: transparent;
  --event-border-width: 0;
  --event-accent-width: 0;
  --event-glow: none;
  --event-animation: none;
  --event-icon-animation: none;
  --event-decor: none;
  --event-scale: 1;
  --event-min-height: 0;
}
```

See `minimal-theme.css` for the full inline treatment (title and value rendered
on the same line as the username).

---

## Token reference

Structure — these default to the same `--chat-*` values the visual customizer
applies to chat bubbles, so events follow the overlay's look on their own:

| Token                   | Default                              | What it does                            |
| ----------------------- | ------------------------------------ | --------------------------------------- |
| `--event-scale`         | `1.05`                               | `transform: scale()` on the row          |
| `--event-min-height`    | `100px`                              | Minimum row height                       |
| `--event-padding`       | `var(--chat-bubble-padding, .75rem)` | Inner padding                            |
| `--event-radius`        | `var(--chat-bubble-border-radius)`   | Corner radius                            |
| `--event-backdrop-blur` | `var(--chat-backdrop-blur, 4px)`     | Backdrop blur radius                     |
| `--event-border-width`  | `3px`                                | Box border width                         |
| `--event-border-style`  | `solid`                              | Box border style                         |
| `--event-border-color`  | tier colour                          | Box border colour                        |
| `--event-bg`            | tier background                      | Row background                           |
| `--event-glow`          | tier shadow                          | `box-shadow`                             |
| `--event-animation`     | `event-bounce-in …`                  | Entrance (+ idle loop on high tier)      |
| `--event-accent-width`  | `8px`                                | Left accent bar width                    |
| `--event-accent-color`  | type colour                          | Left accent bar colour                   |
| `--event-decor`         | `''`                                 | `none` removes decorative pseudo-elements |

Tier — set these to keep the low/medium/high hierarchy in your palette. Setting
`--event-bg` / `--event-border-color` / `--event-glow` directly overrides all
three tiers at once:

`--event-tier-high-bg`, `--event-tier-high-border`, `--event-tier-high-glow`,
`--event-tier-high-animation`, `--event-tier-medium-bg`,
`--event-tier-medium-border`, `--event-tier-medium-glow`,
`--event-tier-low-bg`, `--event-tier-low-border`,
`--event-tier-low-border-width`, `--event-tier-low-glow`

Per-type accent colours (the left bar):

`--event-accent-subscription`, `--event-accent-super-chat`,
`--event-accent-raid`, `--event-accent-bits`, `--event-accent-channel-points`,
`--event-accent-gift`, `--event-accent-follow`, `--event-accent-like`,
`--event-accent-member`

Per-type backgrounds: `--event-bg-super-chat`, `--event-bg-super-chat-high`,
`--event-bg-raid`, `--event-bg-bits`

Typography and inner content:

| Token                                                                                     | Default                     |
| ----------------------------------------------------------------------------------------- | --------------------------- |
| `--event-icon-display` / `-size` / `-filter` / `-animation`                                 | `inline-block` / `2.25rem` / drop-shadow / pulse |
| `--event-title-display` / `-font` / `-size` / `-weight` / `-transform` / `-spacing` / `-color` / `-shadow` | `block` / chat font / `1.125rem` / `700` / `uppercase` / `.05em` / `#fff` / shadow |
| `--event-value-display` / `-size` / `-weight` / `-color` / `-shadow`                         | `block` / `1.5rem` / `800` / `#fde047` / gold glow |
| `--event-text-size` / `-color`                                                              | `.875rem` / `inherit`       |
| `--event-metadata-display` / `--event-meta-size` / `--event-meta-color`                      | `block` / `.75rem` / `#94a3b8` |
| `--event-indent`                                                                            | `3.5rem` (text/metadata left margin) |

---

## Class hooks

`events.css` is the frozen public API (`frontend/src/styles/EVENTS_CSS_API.md`).
The elements inside an event row are:

```
.event-message .event-tier-{low|medium|high} .event-type-{type}   ← the row
  └ .chat-username                                                ← the chatter (row header, same as chat)
  └ .event-content
      ├ .event-icon      the emoji
      ├ .event-title     "GIFT SUBSCRIPTION!"
      ├ .event-value     "x5" / "$20.00"
      ├ .event-message-text        the chatter's own message, when the event carries one
      ├ .event-message-attachments GIFs/uploads on that message
      ├ .event-warning-message     system notices only
      └ .event-metadata  "5 gifts", "1,234 viewers"
```

There is **no** `.event-user` element. An event row carries the same header as a
chat row, so the chatter's name comes from `.chat-username` — style it once and
it covers both. (`.event-user` used to duplicate it, could not render a premium
gradient name, and forced themes to style two username elements.)

---

## Rules

- **Never recolour `.chat-username`.** Name colours belong to the viewer. Size,
  weight, font and outline are fair game; `color` is not.
- **System notices keep their own colours.** `.event-type-token_expiration_warning`,
  `.event-type-source_permission_error` and
  `.event-type-listener_deprecation_notice` are the overlay telling the streamer
  something is broken. They honour the structural tokens but keep their red/amber
  chrome in every theme — don't repaint them.
- **Watch your specificity.** If your theme styles rows via
  `.space-y-3 > div` or `:nth-child(...)`, those selectors also match event rows
  and outrank a plain `.event-message { … }`. Use `.space-y-3 > div.event-message`
  when you need to win.
- **No commas needed in `:is()`** — don't use `:is()` at all. The preview scoper
  splits selector lists on commas and does not parse `:is(...)`, so a rule using
  it renders differently in the marketplace preview than in OBS.

---

## Checking your work

```bash
cd frontend
npm run generate:themes   # bundle docs/overlay-themes/*.css into the app
npm run dev               # then open http://localhost:3000/dev/theme-contrast
```

The dev harness renders every bundled theme with the sample messages **and a
sample event**, which is the fastest way to see a mismatch.

```bash
npm run test:themes                       # message-text contrast (Playwright)
npm test -- bundled-themes-events         # every theme styles its events
```
