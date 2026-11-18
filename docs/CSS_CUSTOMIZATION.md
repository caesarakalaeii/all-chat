# Overlay CSS Customization

How to restyle a chat overlay page of All-Chat with custom CSS.

**Contents**: applying CSS, the DOM structure and hook classes, display settings, platform status indicators, event styling, troubleshooting.

To apply CSS: either the **OBS browser source** Custom CSS field (OBS → Browser Source → Properties → Custom CSS), or the in-app editor at **dashboard → your overlay → Advanced → Custom CSS**. The in-app editor preloads an applied theme's CSS, previews live as you type, and saves only the diff against the theme, so fixes we ship afterward still reach the rules you did not touch. If you delete theme rules, that stores a full copy and pauses theme updates for that overlay; **Reset to theme** discards your edits and re-links.

Theme files, the theme-intent mechanics (`--theme-*`) and the `--event-*` token table live in [docs/overlay-themes/README.md](./overlay-themes/README.md) and [docs/overlay-themes/AUTHORING-EVENTS.md](./overlay-themes/AUTHORING-EVENTS.md).

## How custom CSS applies

Custom CSS sits outside the app's cascade layers (Tailwind utilities, themes, visual customizer), so it outranks all of them: you **do not need `!important`**. Write plain declarations:

```css
.space-y-3 > div {
  background: #000;
}
```

One difference from a design system: **target direct children**. `.space-y-3 > div` styles message rows; the descendant variant `.space-y-3 div` also matches inside them.

## DOM structure

The overlay renderer (`frontend/src/app/overlay/[id]/page.tsx`) emits this per message:

```html
<div class="min-h-screen w-full bg-transparent p-4" data-feed-anchor="bottom" data-feed-order="newest-last">
  <div class="overlay-live-body space-y-3">
    <div class="scroll-anchor"></div>                <!-- keep this zero-box -->

    <div class="chat-message bg-slate-900/90 rounded-lg p-3 shadow-lg backdrop-blur-sm"
         data-platform="twitch" data-username="user123">
      <div class="flex items-start gap-3">
        <div class="flex-shrink-0">            <!-- avatar, when enabled -->
          <img src="avatar.png" class="w-10 h-10 rounded-full" />
        </div>
        <div class="min-w-0 flex-1">
          <div class="mb-1 flex flex-wrap items-center gap-2">
            <span class="platform-badge platform-badge-text text-xs font-semibold uppercase">TWITCH</span>
            <span class="chat-username text-sm font-semibold" style="color:#9146FF">Username</span>
            <div class="flex items-center gap-1">     <!-- user badges, when enabled -->
              <img src="badge.png" class="h-[1em] w-auto object-contain" />
            </div>
          </div>
          <div class="break-words text-white">
            Hello world!
            <img src="emote.png" class="inline-block h-[1.4em] w-auto align-text-bottom" />
          </div>
          <div class="mt-1 text-xs text-slate-500">12:34:56</div>
        </div>
      </div>
    </div>
  </div>
</div>
```

Event messages swap the row class (`event-message` instead of `chat-message`) and replace the body with `EventContent`; see [Event styling](#event-styling).

## Class reference

| Target | Selector | Notes |
|---|---|---|
| Root wrapper | `.min-h-screen` | Full-screen container; carries `data-feed-anchor`, `data-feed-order` |
| Message list | `.overlay-live-body`, `.space-y-3` | Vertical stack with gaps |
| Message row | `.chat-message`, or `.space-y-3 > div` | Primary target. `.space-y-3 > div` also matches event rows |
| Auto-scroll sentinel | `.scroll-anchor` | Invisible, must stay zero-box. Exclude it: `.space-y-3 > div:not(.scroll-anchor)` |
| Avatar container | `.flex-shrink-0` | Rendered only when avatars are on |
| Username | `.chat-username` | Stable class on every name, chat and event rows alike |
| Platform badge | `.platform-badge`, `.platform-badge-text`, `.platform-badge-icon` | Badge wrapper and its two variants |
| Platform name color | `.text-purple-400` (Twitch), `.text-red-400` (YouTube), `.text-green-400` (Kick), `text-tiktok`, `text-discord`, `text-owncast`, `text-goodgame`, `text-picarto`, `text-facebook`, `text-instagram`, `text-altstream` | See `getPlatformColor` in `app/overlay/[id]/page.tsx`; each token is defined in `frontend/src/app/globals.css` |
| User badge container | `.flex.items-center.gap-1` | Per-user badges (sub, mod, ...) |
| Message text | `.text-white.break-words`, or `.break-words` | Actual message content with emotes |
| Emote | `.inline-block.h-\[1\.4em\]` | Inline emote image |
| Timestamp | `.text-xs.text-slate-500.mt-1` | Only when timestamps are on |
| Attribution | `data-platform="twitch"`, `data-username="…"`, `data-user-bubble` | Row-level attributes for per-platform or per-user styling |

Per-user or per-platform accents are common selectors:

```css
[data-platform="youtube"] .chat-message { border-left-color: #FF0000; }
.space-y-3 > div[data-username="user123"] { background: #101820; }
.space-y-3 > div:has(.text-purple-400) { border-left: 4px solid #9146FF; }
```

## Display settings

Most basic knobs live in the dashboard, not CSS (font size and family, message duration, max messages, show badges/avatars/timestamps, animation, platform badge position and style). Two of them are also readable from CSS via attributes on the wrapper:

| Attribute | Values | Meaning |
|---|---|---|
| `data-feed-anchor` | `top` / `bottom` | Which canvas edge the stack rests on |
| `data-feed-order` | `newest-last` / `newest-first` | Which end holds the newest message (newest-first = Invert Message Order on) |

These two are independent: the anchor moves the whole stack (so `bottom` + newest-last is the Twitch look: newest at the bottom, older pushed up), the order flips only the reading direction. Once the feed is taller than the canvas, the anchor stops mattering because the overlay auto-scrolls to keep the newest message visible in every mode.

Example for fading the far end of the feed, whichever end that is:

```css
[data-feed-anchor="top"]    .overlay-live-body > div:last-child  { opacity: 0.6; }
[data-feed-anchor="bottom"] .overlay-live-body > div:first-child { opacity: 0.6; }
[data-feed-order="newest-first"] .overlay-live-body > div:first-child { font-weight: 600; }
```

A new message lands at whichever end `data-feed-order` names, so vertical entry animations must enter from that side. `data-feed-order` drives two inherited custom properties read by the built-in `.msg-anim-*` presets (defined in `frontend/src/app/globals.css`, one class per animation preset: `msg-anim-fly-left`, `msg-anim-fly-right`, `msg-anim-fly-spring`, `msg-anim-pop`, `msg-anim-bounce`, `msg-anim-flip`, `msg-anim-swoosh`, `msg-anim-soft-focus`):

| Property | `newest-last` (default) | `newest-first` |
|---|---|---|
| `--msg-enter-dir` | `1` (enters from below) | `-1` (enters from above) |
| `--msg-enter-origin` | `100%` (hinges on bottom edge) | `0%` (hinges on top edge) |

Custom keyframes can read them too:

```css
@keyframes my-entry {
  from { opacity: 0; transform: translateY(calc(40px * var(--msg-enter-dir, 1))); }
  to   { opacity: 1; transform: none; }
}
```

Overriding a `.msg-anim-*` rule outright also works: they stay single unlayered class selectors, and these properties do not raise their specificity.

Never set `margin-top`, `justify-content` or `flex-direction` on `min-h-screen` or on `.overlay-live-body` itself. Bottom anchoring is a flex-column wrapper plus `margin-top: auto` on `.overlay-live-body`; overriding either re-pins the feed to an edge the user did not pick. Styling the list's children is fine and unaffected.

### The visual customizer and themes

The Appearance panel emits `--chat-*` custom properties (`--chat-font-size`, `--chat-message-gap`, `--chat-bubble-bg-color`, `--chat-avatar-gap`, ...). A theme cooperates with it instead of overriding: `font-size: var(--chat-font-size, var(--theme-font-size, 1rem))` (see [Think about themes](./overlay-themes/README.md)). The authoritative mapping of dashboard fields to variables is `frontend/src/lib/utils/visual-settings-to-css.ts`.

## Platform status indicators

The little icons in the top corner of the overlay (`components/PlatformStatusIndicators.tsx`) show whether each of your configured sources is being read. A source is colored when active and dimmed when not; per-channel state (reconnecting with a countdown, offline, error) changes the pill behind the icon.

Hide or restyle them:

```css
.platform-status-indicators { display: none; }          /* all */
.platform-indicator { opacity: 0.4; }                   /* quieter */
.platform-status-indicators { background: transparent; padding: 0; }  /* no pill */
.platform-status-indicators:hover { opacity: 1; }       /* show on hover */
```

Per-indicator hooks:

| Hook | Values |
|---|---|
| `.platform-indicator` | All indicators |
| `[data-platform="twitch"]` (etc.) | One source's indicator |
| `.platform-status-indicators` | Container (fixed top-right by default) |

Inactive channels are dimmed with opacity, not a grayscale filter class; if you want a custom inactive look, use `[data-platform="x"] .platform-indicator { … }` and test what the monitor actually renders.

## Event styling

Events (subs, gift bombs, raids, bits, channel-point redemptions, Super Chats, gifts, follows, likes, Super Fans, Q&A) arrive as rows with these classes:

- `.event-message`: the row wrapper, plus `.event-tier-{low|medium|high}` and `.event-type-{type}`
- `.event-content`, `.event-icon`, `.event-title`, `.event-value`, `.event-message-text`, `.event-message-attachments`, `.event-metadata`, `.event-warning-message`
- Same header as a chat row: avatar, `.chat-username`, user badges. There is **no** `.event-user` element (it used to duplicate `.chat-username`; it is gone).

Tiers exist: high (subs, big donations/raids, high-value Super Chats), medium (follows, milestones, medium Super Chats/bits), low (small bits, shares, likes).

Everything visual comes from `frontend/src/styles/events.css`. **Set `--event-*` tokens rather than overriding rules.** The authoritative token list and copy-paste recipes (including flattening events into ordinary chat rows) are in [docs/overlay-themes/AUTHORING-EVENTS.md](./overlay-themes/AUTHORING-EVENTS.md). The most useful defaults:

```css
.event-message {
  --event-scale: 1;           /* no scale-up */
  --event-animation: none;    /* no bounce */
  --event-icon-animation: none;
  --event-title-color: #a5b4fc;
  --event-accent-subscription: #818cf8;
}
```

Common CSS-level overrides that still work as first-class rules:

```css
/* Hide all events */
.event-message { display: none; }

/* Hide specific event types */
.event-type-super_chat { display: none; }          /* YouTube Super Chat */
.event-type-channel_points { display: none; }      /* Twitch points */
.event-type-raid { display: none; }
.event-type-like_aggregate { display: none; }      /* TikTok likes */
.event-tier-low { display: none; }

/* Per-platform accent bars */
.event-type-subscription { --event-accent-subscription: #2b6cb0; }
```

Tier defaults (for completeness; set `--event-bg`, `--event-border-color`, `--event-glow` to change all tiers at once):

```css
.event-tier-high {
  --event-bg: linear-gradient(135deg, rgba(255, 215, 0, 0.25) 0%, rgba(255, 140, 0, 0.15) 100%);
  --event-border-color: #FFD700;
  --event-glow: 0 0 30px rgba(255, 215, 0, 0.5), 0 8px 32px rgba(0, 0, 0, 0.4), inset 0 1px 0 rgba(255, 255, 255, 0.1);
}
.event-tier-medium {
  --event-bg: linear-gradient(135deg, rgba(147, 51, 234, 0.2) 0%, rgba(79, 70, 229, 0.15) 100%);
  --event-border-color: #9333EA;
  --event-glow: 0 0 25px rgba(147, 51, 234, 0.4), 0 6px 24px rgba(0, 0, 0, 0.3);
}
.event-tier-low {
  --event-bg: rgba(59, 130, 246, 0.15);
  --event-border-color: #3B82F6;
  --event-border-width: 2px;   /* note: this global token sets ALL box borders */
  --event-glow: 0 0 15px rgba(59, 130, 246, 0.3);
}
```

Raids carry a decorative sweep overlay driven by `--event-decor`: set `--event-decor: none` on `.event-message` to drop every decorative pseudo-element in one declaration (a `content` of `none` also suppresses the box entirely, but `--event-decor` is the cleaner path).

For a full recipe that makes events look like stream alerts (a fixed overlay, a bottom banner), see [docs/overlay-themes/AUTHORING-EVENTS.md](./overlay-themes/AUTHORING-EVENTS.md).

## Troubleshooting

- **CSS not applying**: check for a typo in the selector, a syntax error (browser console F12), or a too-broad rule like `div { ... }` that the browser resolves differently than you expect. Inspect the live overlay page (right-click a message → Inspect) to see the exact classes.
- **CSS works in a plain browser but not in OBS**: refresh the browser source (right-click → Properties → OK re-applies the CSS). Use absolute URLs for `@import`/images, not relative ones.
- **Fonts not loading**: import from a web CDN: `@import url('https://fonts.googleapis.com/css2?family=Roboto&display=swap');`, then set `font-family` with fallbacks: `font-family: 'Roboto', sans-serif;`.
- **Emotes broken or missing**: that is a backend issue (message-processor or 7TV/BTTV/FFZ reachability), not CSS. You can still force size with `.text-white.break-words img { height: 28px; width: auto; }`.
- **Performance**: prefer `background` colors over heavy `backdrop-filter: blur(...)` and giant `box-shadow`s; animate `transform`, not `top`/`left`; reduce the max-messages display setting.

## Where to go next

- Themes, theme-intent mechanics, the `--theme-*` fallback pattern: [docs/overlay-themes/README.md](./overlay-themes/README.md)
- Events in a theme (`--event-*` tokens, flatten-into-chat recipe): [docs/overlay-themes/AUTHORING-EVENTS.md](./overlay-themes/AUTHORING-EVENTS.md)
- Example theme: [docs/overlay-themes/win98-theme.css](./overlay-themes/win98-theme.css)
- Source of truth for markup: `frontend/src/app/overlay/[id]/page.tsx`, `frontend/src/components/overlay/EventContent.tsx`, `frontend/src/app/globals.css`
