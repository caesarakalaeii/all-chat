# All-Chat Overlay Themes

Custom CSS themes for All-Chat chat overlays. Copy a theme into the Custom CSS field of the OBS browser source, or into the in-app editor (dashboard → overlay → **Advanced → Custom CSS**, which also auto-updates bundled themes). Detailed selectors and events: [CSS_CUSTOMIZATION.md](../CSS_CUSTOMIZATION.md) and [AUTHORING-EVENTS.md](./AUTHORING-EVENTS.md).

## Bundled themes

These files live next to this README and ship inside the app (`frontend/scripts/generate-bundled-themes.mjs` bundles every `*.css` here into the theme marketplace; run `npm run generate:themes` after editing):

- `minimal-theme.css`: inline layout, `[icon] [badges] username: message`, no bubbles or avatars, events flattened inline. Requires the platform badge style set to **Icon**. Absorbed the older `minimal-theme-fixed`; overlays configured with that id resolve here (ADR-0053).
- `minimal-icon-theme.css`: clean dark bubbles with icon platform badges before the username. Requires badge position **Before username** and style **Icon**.
- `high-contrast-theme.css`: pure black and white maximum-contrast look (WCAG AAA ratios), bold sizes, excludes the scroll sentinel via `.space-y-3 > div:not(.scroll-anchor)`.
- `noita-minimal-theme.css`: dark fantasy pixel-art, Noita Blackletter usernames (font by viowlet, https://www.reddit.com/r/noita/comments/jp56ub/), transparent background, inline layout.
- `win98-theme.css`: Windows 98 windows with inset input styling, pixelated rendering, optional title bar. See the customization comments in the file itself (uncomment the Hide sections for avatars, platform badges, user badges, timestamps; scale and background colour are one-line edits).

- `comic-speech-theme.css`: speech balloons with tails and halftone fill; paints the balloon from `--chat-bubble-bg-color`.
- `trading-card-theme.css`: card-faced rows with foil borders; per-row variety on the border axis.
- `sticky-notes-theme.css`: coloured note squares, the worked example for per-row bubble colours.
- `neo-brutalist-theme.css`: thick black borders and hard offset shadows.
- `cyberpunk-theme.css`, `minecraft-theme.css`, `neon-glass-theme.css`, `vaporwave-theme.css`, `terminal-hacker-theme.css`, `pastel-cute-theme.css`, `modern-dark-theme.css`: genre looks on the same hook classes.

Every theme here styles its events; the marketplace preview renders all of them. Apply any of them from the in-app theme picker, or by pasting the entire file into OBS → Browser Source → Custom CSS.

## Layout: the two feed preferences

Feed Anchor and Invert Message Order are user preferences, not theme properties. A theme must adapt rather than hardcode one arrangement. The wrapper carries `[data-feed-anchor="top|bottom"]` and `[data-feed-order="newest-last|newest-first"]`; the anchor sets where blank space collects, the order decides which end is newest. Entry animation direction is driven by the inherited `--msg-enter-dir` / `--msg-enter-origin` custom properties that the `.msg-anim-*` presets read. Details and examples are in [CSS_CUSTOMIZATION.md](../CSS_CUSTOMIZATION.md#display-settings). Exclude `.scroll-anchor` from any rule that styles rows; do not override the wrapper's flex layout or `.overlay-live-body`'s `margin-top`.

## Cooperative customizer values

Most `--chat-*` properties are cooperative: read the variable with your own theme value as the fallback and the user's customization wins when they have set it.

```css
.break-words {
  font-size: var(--chat-font-size, var(--theme-font-size, 1rem));
}
```

A value declared as a `--theme-*` variable in your `:root` (`--theme-font-size: 15px`, `--theme-avatar-gap: 16px`) applies only while the user has left the matching dashboard control unset; the app's consumer rules read the theme intent as the middle fallback. Minimal Clean shows the pattern; the theme-CSS parser reads a theme's declarations and `var()` fallbacks back into the editor's fields, so a bare fallback (not nested in a second `var()`) is treated as the user's choice and becomes unbeatable.

Three properties are **not** cooperative. When their dashboard control is set, the app forces the property in the top `visual-customizer` layer regardless of what your theme writes, and emits nothing when unset: Text Shadow (`text-shadow` on `.break-words`, `.chat-username`, `.text-xs.text-slate-500`), Bubble shadow (`box-shadow` on chat rows), and Platform accent (`color`/`fill` on `[data-platform='<p>'] .platform-badge` and its SVG). Never declare or read `--chat-text-shadow`, `--chat-bubble-shadow` or `--platform-*-accent` in a theme: mentioning one makes your default indistinguishable from a user choice. Tests in `frontend/src/lib/theme-marketplace/__tests__/` enforce this for bundled themes.

Bubble color is cooperative too, and themes are expected to read it:

```css
.chat-message {
  background-color: var(--chat-bubble-bg-color, var(--paper, #fff8c4));
}
```

Paint it on whatever element you render as the bubble, not necessarily the row. For per-row color variation, put the per-row color in a **nested** var fallback and gate any extra contrast wash on `--customizer-bubble-bg-set` (which is `1` only when the streamer picked a solid color); Sticky Notes is the worked example. The streamer can additionally force per-platform bubble fills or per-user bubble colors; where set, those replace your fill. Do not key theme rules off `data-bubble-slot` (assigned by arrival order, so it is not positional); use `nth-child` for rhythm.

## Authoring your own theme

Start from an existing theme and the class map in [CSS_CUSTOMIZATION.md](../CSS_CUSTOMIZATION.md#class-reference). Non-negotiable parts of the contract:

- Apply theme rules through plain declarations. Theme CSS is layered above the app defaults; `!important` is stripped at injection.
- Every bundled theme must style its events (see `--event-*` tokens in [AUTHORING-EVENTS.md](./AUTHORING-EVENTS.md)); leaving them on the platform default reads as a different product dropped into your overlay. Tests fail otherwise.
- Never recolor `.chat-username`. Size, weight, font and outlines are yours; `color` belongs to the viewer.
- Don't use `:is()` or comma-split placeholder text: the marketplace preview scopes selectors by splitting lists on commas and does not parse `:is(...)`, so such a rule renders differently in preview and OBS.
- Set theming metadata in a comment header at the top of the file: `Theme Name`, `Description`, `Tags`, `Author`, `Version`, `Updated`.

Check your work:

```bash
cd frontend
npm run generate:themes   # bundle docs/overlay-themes/*.css into the app
npm run dev               # then open http://localhost:3000/dev/theme-contrast
npm run test:themes       # message-text contrast (Playwright)
npm test -- bundled-themes-events
```

To share a theme, open a pull request adding the CSS file to `docs/overlay-themes/` and a section here.
