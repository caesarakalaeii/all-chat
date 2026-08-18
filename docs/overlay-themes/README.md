# All-Chat Overlay Themes

Custom CSS themes for your All-Chat overlays in OBS. These themes allow you to completely transform the look of your chat overlay without touching any code.

---

## 📚 Looking for the Complete CSS Reference?

**For developers and advanced customization**, see these guides:
- **[CSS Customization Guide](../CSS_CUSTOMIZATION.md)** - Complete DOM structure, CSS classes, and advanced techniques
- **[Styling events in a theme](./AUTHORING-EVENTS.md)** - The `--event-*` tokens; **required reading if you add a theme**
- **[Platform Badge Customization](../PLATFORM_BADGE_CUSTOMIZATION.md)** - Platform badge position and style options

**This page** focuses on quick theme application and basic customization.

---

## How to Apply a Theme in OBS

1. **Add Browser Source** (if you haven't already)
   - In OBS, click the `+` button in the Sources panel
   - Select "Browser"
   - Name it (e.g., "All-Chat Overlay")
   - Set the URL to your overlay: `http://your-domain.com/overlay/YOUR_OVERLAY_ID`
   - Set width/height to match your canvas (e.g., 1920x1080)

2. **Apply Custom CSS**
   - Right-click your Browser Source → Properties
   - Scroll down to the "Custom CSS" text box
   - Copy the entire contents of a theme file (e.g., `win98-theme.css`)
   - Paste it into the Custom CSS box
   - Click OK

3. **Customize** (optional)
   - Open the CSS file in a text editor
   - Read the comments for customization options
   - Uncomment sections to hide elements (avatars, badges, platform labels)
   - Adjust colors, sizes, and animations to your preference

## Available Themes

### Minimal Clean (`minimal-theme.css`) ⭐ MOST USED

The flagship minimal look: everything on **one line**, no bubbles, no avatars.

**Features:**
- Inline layout: `[icon] [badges] username: message`
- Colourful usernames with a smooth black readability outline
- Transparent background, no message containers
- Events render inline too, as `username: ⭐ New Subscriber! x5`
- Platform status indicators hidden by default (re-enable with the
  "Platform indicators" toggle in Appearance)

**Required Settings:**
- Platform Badge Style: **"Icon (logo)"** (a text badge is hidden by this theme)

**Preview:**
```
[🎮] [💎] Username: Hello from Twitch chat!
```

> **Consolidated in 2026-08.** This theme absorbed the old
> "Minimal Clean Theme (Fixed Platform & Badges)" (`minimal-theme-fixed`), which
> was a bugfix fork that shipped beside it instead of replacing it. Overlays still
> configured with the old id resolve here automatically — see
> [ADR-0053](../adr/0053-consolidate-minimal-theme-and-retire-theme-ids-via-alias.md).

### Minimal Icon Theme (`minimal-icon-theme.css`) ⭐ NEW

Clean, modern theme showcasing the new **[ICON] [BADGES] USERNAME** layout!

**Features:**
- Platform logos as compact icons (Twitch, YouTube, Kick, TikTok)
- Icon badges appear before username
- Clean dark background with accent border
- Subtle icon backgrounds and glow effects
- Perfect for streamers who want a minimal, uncluttered look

**Required Settings:**
- Platform Badge Position: **"Before username"**
- Platform Badge Style: **"Icon (logo)"**

**Preview:**
```
[🎮] [💎] [⚔️] Username
     Hello from Twitch chat!
     12:34:56 PM
```

### High Contrast Theme (`high-contrast-theme.css`) ⭐ ACCESSIBILITY

Maximum readability theme optimized for WCAG AAA compliance!

**Features:**
- WCAG AAA compliant (21:1 contrast ratios)
- Pure black background (#000) with pure white text (#FFF)
- Bold typography with increased font sizes (usernames: 20px, messages: 22px)
- Multi-layer text shadows for username color legibility
- Enhanced avatar borders (4px) and shadows
- Larger, more distinct platform badges and user badges (24px)
- High-contrast event styling with tier-based glows
- Thick borders (3-6px) for all elements
- Optimized for visibility over any content

**Best For:**
- Streamers prioritizing accessibility
- Viewers with visual impairments
- Maximum readability in all lighting conditions
- Professional/corporate streaming environments
- Streaming over very light or very dark games

**Contrast Ratios (WCAG AAA):**
- White text on black: 21:1 ✓
- Light gray timestamps on black: 17.3:1 ✓
- Yellow accents on black: 19.6:1 ✓

**Customization Options:**
- ✅ Adjust text sizes (compact/large modes)
- ✅ Hide avatars for text-only focus
- ✅ Change accent color (yellow/cyan/magenta)
- ✅ Square vs rounded avatars
- ✅ Border thickness adjustments
- ✅ Disable animations for maximum stability

**Usage:**
1. Copy contents of `high-contrast-theme.css`
2. Open OBS → Browser Source → Custom CSS field
3. Paste and save
4. Verify contrast in OBS preview

**Preview:**
```
┌──────────────────────────────────────────────┐
│ [🎮] [💎] [⚔️] Username (bold, white)        │
│    Hello from Twitch chat!                   │
│    (22px white text, thick black outline)    │
│    [🕐 12:34:56 PM] (14px gray)              │
└──────────────────────────────────────────────┘
(Pure black background, 3px white border, 6px yellow left border)
```

### Noita Minimal Theme (`noita-minimal-theme.css`) ⭐ DARK FANTASY

Dark fantasy pixel-art theme inspired by Noita's mystical underground atmosphere!

**Features:**
- **Authentic Noita Blackletter font** for usernames (custom font by viowlet)
- Pixel-art fonts for message text (Press Start 2P / VT323)
- Magical glowing text effects (purple/gold aura)
- Earthy color palette (browns, golds, dark purples)
- Transparent background (OBS-friendly)
- Inline minimal layout: `[ICON] [BADGES] USERNAME: message`
- Platform-specific muted colors (mystic purple, crimson, mossy green)
- Animated magical pulse effect on usernames
- Optional sparkle particle decorations
- Alchemical event styling with mystical borders

**Color Palette:**
- Background: Fully transparent
- Usernames: `#e8d4a0` (warm parchment) with purple magical glow
- Messages: `#c9b896` (earthy tone) with pixel font
- Platform colors: Muted earthy versions (purple, crimson, green, gray)

**Typography:**
- Username font: `Noita Blackletter` (20px, uppercase, blackletter style)
- Message font: `Press Start 2P` (14px, pixel art)
- Magical text-shadow glow: Multi-layer purple aura

**Customization Options:**
- ✅ Thicker glow effect (intensify magical aura)
- ✅ No animations (disable pulse/fade effects)
- ✅ Monochrome usernames (single gold color)
- ✅ Hide platform badges/icons
- ✅ Hide user badges
- ✅ Compact spacing (tighter layout)
- ✅ Brighter text (increase brightness)
- ✅ Stacked layout (username above message)
- ✅ Particle effects (sparkle decorations)

**Best For:**
- Noita streamers
- Dark fantasy/roguelike games
- Retro/pixel art aesthetics
- Mystical/magical stream themes
- Minimal overlay designs with character

**Font Attribution:**
- Noita Blackletter font created by **viowlet** (Reddit)
- Source: [r/noita community](https://www.reddit.com/r/noita/comments/jp56ub/)

**Accessibility:**
- WCAG AA compliant (gold text ~8:1 contrast ratio)
- Readable 14-16px pixel fonts at 1080p
- Multi-layer outline ensures readability on any background
- Respects `prefers-reduced-motion` for accessibility

**Preview:**
```
[🎮] [💎] USERNAME: Hello from the mystical caves!
     (Glowing blackletter username with magical purple aura)
     (Pixel font message with dark outline)
```

### Windows 98 Theme (`win98-theme.css`)

Transform your chat into a nostalgic Windows 98 experience!

**Features:**
- Classic Win98 gray windows with 3D borders
- Inset avatar frames
- Platform badges styled as Win98 status bars
- Message text in white boxes with inset borders
- Timestamp styled as status indicators
- Pixelated rendering for authentic retro feel
- Optional title bar

**Customization Options:**
- ✅ Show/hide avatars
- ✅ Show/hide platform badges (TWITCH, YOUTUBE, etc.)
- ✅ Show/hide user badges (subscriber, moderator)
- ✅ Show/hide timestamps
- ✅ Adjust message size (scale)
- ✅ Change window background color
- ✅ Add Win98 title bar to messages
- ✅ Adjust transparency
- ✅ Disable animations

**Preview:**
```
┌─────────────────────────────────────────────┐
│ [👤] [TWITCH] Username 💎             │
│      ┌─────────────────────────────┐      │
│      │ Hello from Twitch chat!     │      │
│      └─────────────────────────────┘      │
│      [🕐 12:34:56 PM]                      │
└─────────────────────────────────────────────┘
```

## Creating Your Own Theme

Want to create a custom theme? Here's a quick overview. **For detailed documentation, see the [CSS Customization Guide](../CSS_CUSTOMIZATION.md)**.

### Quick Start

### Key CSS Classes to Target

Based on the current overlay implementation (`frontend/src/app/overlay/[id]/page.tsx`):

#### Message Container
- `.space-y-3 > div` - Individual message wrapper
- `.bg-gray-900/90` - Default dark background
- `.rounded-lg` - Rounded corners (change to 0 for square)
- `.p-3` - Padding
- `.shadow-lg` - Shadow effect

#### Avatar Section
- `.flex-shrink-0` - Avatar container
- `.flex-shrink-0 img` - Avatar image
- `.flex-shrink-0 > div` - Avatar fallback (initials)
- `.w-10.h-10` - Avatar size (40x40px)
- `.rounded-full` - Circular shape

#### Username & Platform
- `.platform-badge` - Platform badge wrapper (both text and icon variants)
- `.platform-badge-text` - Platform text label (TWITCH, YOUTUBE, etc.)
- `.platform-badge-icon` - Platform icon (SVG logo)
- `.font-semibold.text-sm` - Username
- Color classes:
  - `.text-purple-400` - Twitch
  - `.text-red-400` - YouTube
  - `.text-green-400` - Kick
  - `.text-gray-400` - Other
- **Note**: Platform badge position and style can be configured via Display Settings (before/after username, text/icon)

#### Badges
- `.flex.gap-1` - Badge container
- `.flex.gap-1 img` - Individual badge images
- `.w-4.h-4` - Badge size (16x16px)

#### Message Text
- `.text-white.break-words` - Message content
- Inline style: `fontSize` - Text size (default 16px)

#### Timestamp
- `.text-xs.text-gray-500` - Timestamp text

#### Events (subs, raids, redemptions)
- `.event-message` - Event row wrapper (instead of `.chat-message`)
- `.event-tier-{low|medium|high}`, `.event-type-{type}` - Importance and event type
- `.event-icon`, `.event-title`, `.event-value`, `.event-metadata` - Inner content
- `--event-*` custom properties - How you actually restyle them

**A theme that styles only chat leaves events in the platform default** (a
scaled-up gold gradient card with a glow and a bounce), which looks like a
different product dropped into your overlay. See
**[AUTHORING-EVENTS.md](./AUTHORING-EVENTS.md)** for the token table and a
copy-paste recipe; a test fails if a bundled theme has no event styling.

### Theme Template

```css
/* Reset default styles */
body {
  background: transparent !important;
  font-family: 'Your Font', sans-serif !important;
}

/* Message container */
.space-y-3 > div {
  background: #YOUR_COLOR !important;
  border: YOUR_BORDER !important;
  border-radius: YOUR_RADIUS !important;
  padding: YOUR_PADDING !important;
}

/* Avatar */
.flex-shrink-0 img,
.flex-shrink-0 > div {
  /* Your avatar styles */
}

/* Platform badge (text or icon) */
.platform-badge {
  /* Styles for both text and icon variants */
}

.platform-badge-text {
  /* Text badge styles (TWITCH, YOUTUBE) */
}

.platform-badge-icon svg {
  /* Icon badge styles (platform logos) */
}

/* Username */
.font-semibold.text-sm {
  /* Your username styles */
}

/* Message text */
.text-white.break-words {
  /* Your message styles */
}

/* Timestamp */
.text-xs.text-gray-500 {
  /* Your timestamp styles */
}

/* Hide elements (optional) */
.flex-shrink-0 { display: none !important; } /* Hide avatars */
.text-xs.font-semibold.uppercase { display: none !important; } /* Hide platform */
.flex.gap-1 { display: none !important; } /* Hide badges */
```

## Tips & Best Practices

1. **Always use `!important`** - OBS can be finicky with CSS specificity
2. **Test in OBS** - Styles may look different in OBS vs browser
3. **Use web-safe fonts** - Or include @font-face declarations
4. **Keep it simple** - Too many animations can impact performance
5. **Transparent backgrounds** - Use `transparent` or `rgba()` for overlay backgrounds
6. **Consider readability** - Ensure text is readable over your game/content

## Troubleshooting

**CSS not applying?**
- Make sure you're using `!important` on all rules
- Check for syntax errors (missing semicolons, brackets)
- Try refreshing the Browser Source in OBS

**Fonts not working?**
- Use web-safe fonts or include font files
- System fonts may not be available in OBS

**Performance issues?**
- Disable complex animations
- Reduce the number of messages displayed
- Simplify box-shadow and backdrop-filter effects

**Elements not hiding?**
- Ensure you're using the correct class selector
- Use browser DevTools to inspect and verify class names
- Add `display: none !important;` to force hiding

## Contributing Themes

Want to share your theme with the community?

1. Create your CSS file in `docs/overlay-themes/your-theme-name.css`
2. Add clear comments and customization instructions
3. Include a preview/screenshot
4. Update this README with your theme details
5. Submit a pull request!

## Example Theme Ideas

Here are some ideas for future themes:

- **Cyberpunk/Neon** - Glowing borders, neon colors, futuristic fonts
- **Minimalist** - Clean, simple, monochrome design
- **Retro Terminal** - Green/amber text on black, monospace font
- **Material Design** - Google's Material Design principles
- **Glassmorphism** - Frosted glass effect with blur
- **Neumorphism** - Soft 3D raised/pressed elements
- **8-bit/Pixel Art** - Pixelated borders, retro game aesthetic
- **macOS Aqua** - Classic macOS X style (circa 2001)
- **Comic Book** - Bold outlines, speech bubble design
- **Paper/Card** - Physical cards with shadows and texture

## Additional Resources

- **[CSS Customization Guide](../CSS_CUSTOMIZATION.md)** - Complete developer documentation
- **[Quick Start Guide](./QUICK-START.md)** - Apply themes in minutes
- **[Main Repository](../../README.md)** - All-Chat project homepage

## Credits

Created for the All-Chat project - a cloud-native microservices platform for aggregating chat from multiple streaming platforms.

## License

These themes are provided as examples and can be freely modified and distributed.
