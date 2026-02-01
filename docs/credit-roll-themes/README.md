# Credit Roll Theme Guide

Create custom themes for All-Chat end-of-stream credit rolls.

## What Are Credit Roll Themes?

Credit roll themes customize the appearance of your end-of-stream credits display, including:
- Leaderboards for subs, bits, raids, super chats, follows, and gifts
- User rankings with avatars and platform badges
- Clip showcases (if enabled)
- Thank you messages

## How They Differ from Overlay Themes

| Feature | Overlay Themes | Credit Roll Themes |
|---------|----------------|-------------------|
| Purpose | Chat message display | End-of-stream leaderboards |
| Content | Messages, emotes, badges | Rankings, avatars, statistics |
| Update Frequency | Real-time (< 500ms) | Static (shown at stream end) |
| Duration | Continuous | 10-300 seconds |

## Key CSS Selectors

### Structure
```css
/* Main container */
.min-h-screen { }

/* Section headers with emoji */
h2 { }

/* Leaderboard containers */
.space-y-4 { }

/* Individual leaderboard entries */
.space-y-4 > div { }

/* Top 3 special styling */
.bg-yellow-500\/20 { }  /* 1st place */
.bg-gray-400\/20 { }    /* 2nd place */
.bg-orange-600\/20 { }  /* 3rd place */

/* Rank numbers */
.text-3xl.font-bold { }

/* Display names */
.text-xl.font-semibold { }

/* Platform labels */
.text-sm.text-gray-400 { }

/* Value/count display */
.text-2xl.font-bold { }
```

### Common Customizations

```css
/* Change font for all text */
body {
  font-family: 'Your Font', sans-serif !important;
}

/* Style headers */
h1, h2 {
  color: #custom-color !important;
  text-shadow: 0 0 20px rgba(255, 255, 255, 0.5) !important;
}

/* Customize leaderboard entries */
.space-y-4 > div {
  background: rgba(your, colors, here, 0.2) !important;
  border: 2px solid rgba(your, border, color, 0.5) !important;
  backdrop-filter: blur(10px) !important;
}

/* Gold for 1st place */
.bg-yellow-500\/20 {
  box-shadow: 0 0 30px rgba(255, 215, 0, 0.5) !important;
}
```

## Theme Metadata

Include metadata at the top of your CSS file:

```css
/**
 * Theme Name: Your Theme Name
 * Description: Brief description of your theme
 * Tags: tag1, tag2, tag3
 * Author: Your Name
 * Version: 1.0.0
 * Updated: 2026-02-01
 */
```

## Example Themes

See the `.css` files in this directory for complete examples:
- `classic-cinematic.css` - Traditional movie credits
- `neon-credits.css` - Vibrant neon with glowing effects
- `minimal-clean.css` - Clean modern design
- `retro-gaming.css` - Pixel art/arcade style
- `elegant-script.css` - Cursive fancy credits

## Contributing Themes

1. Create your theme CSS file with metadata header
2. Test it in the All-Chat credit roll preview
3. Submit a pull request to add it to `docs/credit-roll-themes/`
4. Themes must be family-friendly and work across different leaderboard sizes

## Tips for Theme Creators

1. **Use !important** for Tailwind overrides
2. **Test with different data** - some leaderboards may be empty
3. **Consider animations** - but keep them performant
4. **Use web fonts** - @import from Google Fonts works great
5. **Provide fallbacks** - users may have different configurations
6. **Respect user settings** - background opacity should be adjustable

## Resources

- [All-Chat GitHub](https://github.com/caesarakalaeii/all-chat)
- [Tailwind CSS Docs](https://tailwindcss.com/)
- [Google Fonts](https://fonts.google.com/)
