# Platform Badge Customization Guide

This guide explains the new platform badge display options introduced to give users more control over how platform indicators appear in their overlays.

## Overview

Platform badges identify which streaming platform a chat message came from (Twitch, YouTube, Kick, TikTok). You now have two configurable options:

1. **Position**: Show platform badge before or after the username
2. **Style**: Display as text label or platform icon

## Configuration Options

### 1. Platform Badge Position

**Setting**: `platform_badge_position`
**Values**: `"before"` (default) | `"after"`

Controls where the platform badge appears relative to the username.

#### Before (Default)
```
[TWITCH] [BADGES] Username: Hello world!
```

#### After
```
Username [TWITCH] [BADGES]: Hello world!
```

### 2. Platform Badge Style

**Setting**: `platform_badge_style`
**Values**: `"text"` (default) | `"icon"`

Controls whether the platform is shown as a text label or logo icon.

#### Text (Default)
```
[TWITCH] Username: Hello world!
```

#### Icon
```
[🎮] Username: Hello world!
```
*(Shows official Twitch logo icon)*

## Configuration Examples

### Via Dashboard (Recommended)

Update your overlay's display settings through the All-Chat dashboard:

```json
{
  "display_settings": {
    "font_size": 16,
    "max_messages": 50,
    "platform_badge_position": "before",
    "platform_badge_style": "icon"
  }
}
```

### Via API

**Endpoint**: `PATCH /api/v1/overlays/:id/config`

```bash
curl -X PATCH https://your-domain.com/api/v1/overlays/{overlay_id}/config \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "display_settings": {
      "platform_badge_position": "before",
      "platform_badge_style": "icon"
    }
  }'
```

## CSS Customization

### New CSS Classes

| Class | Description |
|-------|-------------|
| `.platform-badge` | Parent wrapper for both text and icon badges |
| `.platform-badge-text` | Text platform labels (TWITCH, YOUTUBE, etc.) |
| `.platform-badge-icon` | SVG platform icons |

### Examples

#### Style Platform Text Badges
```css
/* Add background and padding to text badges */
.platform-badge-text {
  padding: 2px 8px !important;
  border-radius: 4px !important;
  background: rgba(0, 0, 0, 0.5) !important;
  font-size: 10px !important;
}
```

#### Style Platform Icon Badges
```css
/* Make icons larger with glow effect */
.platform-badge-icon svg {
  width: 24px !important;
  height: 24px !important;
  filter: drop-shadow(0 0 6px rgba(255, 255, 255, 0.4)) !important;
}

/* Add background to icons */
.platform-badge-icon {
  background: rgba(0, 0, 0, 0.3) !important;
  padding: 4px !important;
  border-radius: 6px !important;
}
```

#### Hide Platform Badges Entirely
```css
.platform-badge {
  display: none !important;
}
```

#### Hide Only Text (Keep Icons)
```css
.platform-badge-text {
  display: none !important;
}
```

#### Hide Only Icons (Keep Text)
```css
.platform-badge-icon {
  display: none !important;
}
```

## Platform Icons

The icons use official brand colors per platform guidelines:

- **Twitch**: Purple (`#9146FF`)
- **YouTube**: Red (`#FF0000`)
- **Kick**: Green (`#00E701`)
- **TikTok**: Black (`#000000`)

These are SVG icons rendered inline, so they scale perfectly at any size.

## Layout Examples

### Example 1: Icon Before Username with Badges
```
Config: position="before", style="icon"

[🎮] [SUB] [MOD] Username: Hello world!
```

### Example 2: Text After Username
```
Config: position="after", style="text"

Username [TWITCH]: Hello world!
```

### Example 3: Icon After Username with Badges
```
Config: position="after", style="icon"

Username [SUB] [MOD] [🎮]: Hello world!
```

## Migration Notes

### Default Behavior

- **Position**: `"before"` (platform badge appears first)
- **Style**: `"text"` (shows text labels like "TWITCH")

### Breaking Changes

None! Existing overlays will continue to work with the default settings. The previous text-after-username layout can be restored by setting:

```json
{
  "platform_badge_position": "after",
  "platform_badge_style": "text"
}
```

## Implementation Details

### Frontend Changes

**File**: `frontend/src/app/overlay/[id]/page.tsx`

- Added state variables: `platformBadgePosition`, `platformBadgeStyle`
- Loads settings from `/api/v1/overlays/public/:id/config`
- Renders platform badges conditionally based on settings
- Includes inline SVG components for each platform

### Backend Changes

**No database migration required!** Settings are stored in the existing `overlay_configs.display_settings` JSONB column.

The new fields are:
- `display_settings.platform_badge_position`: `"before"` | `"after"`
- `display_settings.platform_badge_style`: `"text"` | `"icon"`

## Troubleshooting

### Icons Not Showing

**Problem**: Icons appear as empty squares or don't render

**Solution**: Ensure `platform_badge_style` is set to `"icon"` in display settings. Check browser console for errors.

### Wrong Position

**Problem**: Badges appear in unexpected position

**Solution**: Verify `platform_badge_position` is set correctly. Clear OBS browser cache if using OBS.

### CSS Not Applying

**Problem**: Custom CSS doesn't affect platform badges

**Solution**:
1. Use `!important` flag in CSS rules
2. Use the correct selector (`.platform-badge`, `.platform-badge-text`, or `.platform-badge-icon`)
3. Refresh OBS browser source

### Platform Badge Missing

**Problem**: No platform indicator appears at all

**Solution**: Check if you're hiding platform badges via CSS:
```css
/* Remove this if present */
.platform-badge {
  display: none !important;
}
```

## Further Reading

- [Complete CSS Customization Guide](./CSS_CUSTOMIZATION.md)
- [Theme Gallery](./overlay-themes/README.md)
- [Display Settings Documentation](./DISPLAY_SETTINGS.md)

---

**Last Updated**: 2026-01-17
**Feature Added In**: Phase 5 (Frontend Enhancements)
