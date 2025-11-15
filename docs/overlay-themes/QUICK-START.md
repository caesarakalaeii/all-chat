# Quick Start: Win98 Theme

Get the Windows 98 look in 2 minutes!

## Step 1: Copy the CSS

Open [`win98-theme.css`](./win98-theme.css) and copy everything (Ctrl+A, Ctrl+C)

## Step 2: Paste into OBS

1. Right-click your All-Chat Browser Source in OBS
2. Click "Properties"
3. Scroll to "Custom CSS"
4. Paste (Ctrl+V)
5. Click "OK"

Done! Your chat now has that sweet 90s nostalgia.

## Step 3: Customize (Optional)

Want to hide certain elements? Open `win98-theme.css` in a text editor and find the section titled:

```
OPTIONAL: HIDE ELEMENTS
```

### Hide Avatars

Find this section and **remove the `/*` and `*/`**:

```css
/* --- HIDE AVATARS --- */
/*
.flex-shrink-0 {
  display: none !important;
}
*/
```

Should become:

```css
/* --- HIDE AVATARS --- */
.flex-shrink-0 {
  display: none !important;
}
```

### Hide Platform Labels (TWITCH, YOUTUBE, etc.)

```css
/* --- HIDE PLATFORM BADGES --- */
.text-xs.font-semibold.uppercase {
  display: none !important;
}
```

### Hide User Badges (Subscriber, Mod, etc.)

```css
/* --- HIDE USER BADGES --- */
.flex.gap-1 {
  display: none !important;
}
```

### Hide Timestamps

```css
/* --- HIDE TIMESTAMP --- */
.text-xs.text-gray-500 {
  display: none !important;
}
```

## Mix and Match

You can hide any combination! Want to show only usernames and messages?

**Uncomment all four hide sections** to get a super clean look:
- Hide avatars ✓
- Hide platform badges ✓
- Hide user badges ✓
- Hide timestamps ✓

Result: Just username + message text in Win98 windows!

## Cheat Sheet: Common Customizations

### Make Messages Bigger

Find the "ADDITIONAL CUSTOMIZATION OPTIONS" section and uncomment:

```css
.space-y-3 > div {
  transform: scale(1.2);
  transform-origin: left top;
  margin-bottom: 16px !important;
}
```

Change `1.2` to `1.5` for even bigger messages, or `0.8` for smaller.

### Change the Window Color

```css
.space-y-3 > div {
  background: #008080 !important; /* Teal */
}
```

Try these colors:
- `#c0c0c0` - Win98 Gray (default)
- `#008080` - Teal
- `#800080` - Purple
- `#000080` - Navy Blue
- `#800000` - Maroon

### Add a Title Bar

Uncomment the "ADVANCED: TITLE BAR OPTION" section for authentic Win98 windows with blue title bars!

### Remove Animations

If messages sliding in is too distracting:

```css
.space-y-3 > div {
  animation: none !important;
}
```

## Need Help?

- Check the full [README.md](./README.md) for detailed documentation
- See the main overlay code at `frontend/src/app/overlay/[id]/page.tsx`
- The CSS targets Tailwind classes used in the React component

## Pro Tip: Save Your Presets

OBS lets you save Browser Source properties as "Scene Collections." After customizing your CSS perfectly:

1. File → Scene Collection → Export
2. Save it with a descriptive name (e.g., "Win98-Chat-No-Avatars")
3. You can now switch between different styles instantly!

Enjoy your retro chat overlay! 🪟✨
