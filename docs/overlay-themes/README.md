# All-Chat Overlay Themes

Custom CSS themes for your All-Chat overlays in OBS. These themes allow you to completely transform the look of your chat overlay without touching any code.

---

## 📚 Looking for the Complete CSS Reference?

**For developers and advanced customization**, see these guides:
- **[CSS Customization Guide](../CSS_CUSTOMIZATION.md)** - Complete DOM structure, CSS classes, and advanced techniques
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
