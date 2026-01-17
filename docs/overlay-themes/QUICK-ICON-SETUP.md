# Quick Setup: Icon Badge Layout

Want the clean **[ICON] [BADGES] USERNAME** layout? Follow these 3 simple steps:

## Step 1: Configure Display Settings

Go to: **All-Chat Dashboard → Your Overlay → Preview**

In the **Platform Badge** section:
- **Position**: Select **"Before username"**
- **Style**: Select **"Icon (logo)"**

Click **"Save Configuration"**

## Step 2: Apply Minimal Theme (Optional)

Copy the CSS from `minimal-icon-theme.css` and paste it into the **Custom CSS Editor** on the preview page.

Or use this minimal version:

```css
/* Quick icon badge theme */
.platform-badge-icon {
  background: rgba(255, 255, 255, 0.1) !important;
  padding: 4px !important;
  border-radius: 4px !important;
}

.platform-badge-icon svg {
  width: 18px !important;
  height: 18px !important;
  filter: drop-shadow(0 0 4px rgba(255, 255, 255, 0.3)) !important;
}
```

## Step 3: Test It

Click **"Inject Message"** in the Mock Messages section to see your new layout!

You should see: **[Platform Icon] [Badges] Username**

## Example Layouts

### Default (Text Before)
```
TWITCH [💎] [⚔️] CaesarLP
Hello world!
```

### Icon Before (What You'll Get)
```
[🎮] [💎] [⚔️] CaesarLP
Hello world!
```

### Icon After
Set Position to "After username":
```
CaesarLP [💎] [⚔️] [🎮]
Hello world!
```

### Text After
Set Style to "Text" and Position to "After username":
```
CaesarLP [💎] [⚔️] TWITCH
Hello world!
```

## Platform Icons

The icons use official brand colors:
- 🎮 **Twitch**: Purple (#9146FF)
- 📺 **YouTube**: Red (#FF0000)
- 🎯 **Kick**: Green (#00E701)
- 🎵 **TikTok**: Black (#000000)

## Need Help?

See the full guides:
- [Platform Badge Customization](../PLATFORM_BADGE_CUSTOMIZATION.md)
- [CSS Customization Guide](../CSS_CUSTOMIZATION.md)
- [Theme Gallery](./README.md)
