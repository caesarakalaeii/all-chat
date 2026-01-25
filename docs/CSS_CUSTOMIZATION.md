# All-Chat Overlay CSS Customization Guide

**Target Audience**: External developers and streamers who want to customize their overlay appearance
**Applies To**: OBS Browser Source CSS override OR in-app CSS editor

---

## Table of Contents

1. [Quick Start](#quick-start)
2. [How to Apply Custom CSS](#how-to-apply-custom-css)
3. [DOM Structure Reference](#dom-structure-reference)
4. [CSS Classes and Selectors](#css-classes-and-selectors)
5. [Common Customizations](#common-customizations)
6. [Platform-Specific Styling](#platform-specific-styling)
7. [Display Settings](#display-settings)
8. [Platform Status Indicators](#platform-status-indicators)
9. [Advanced Techniques](#advanced-techniques)
10. [Event Display Styling](#event-display-styling)
11. [Example Themes](#example-themes)
12. [Troubleshooting](#troubleshooting)

---

## Quick Start

### Minimal Custom CSS Template

```css
/* Change message background color and border */
.space-y-3 > div {
  background: rgba(20, 20, 30, 0.95) !important;
  border: 2px solid #9146FF !important;
  border-radius: 8px !important;
  padding: 12px !important;
}

/* Change username color */
.font-semibold.text-sm {
  color: #FFD700 !important;
}

/* Change message text color */
.text-white.break-words {
  color: #FFFFFF !important;
}
```

**Copy this template, modify colors/sizes, and paste into:**
- OBS → Browser Source → Custom CSS field, OR
- All-Chat dashboard → Overlay Settings → Custom CSS editor

---

## How to Apply Custom CSS

### Method 1: OBS Browser Source (Recommended for Simple Themes)

1. Add a **Browser Source** in OBS
2. Set URL to: `http://your-domain.com/overlay/{your-overlay-id}`
3. Scroll down to **Custom CSS** field
4. Paste your CSS code
5. Click **OK**

**Advantages**: Changes apply instantly without page reload
**Limitations**: No live preview, must test in OBS

### Method 2: In-App CSS Editor (Recommended for Testing)

1. Go to All-Chat dashboard
2. Navigate to **Overlays** → Select your overlay
3. Click **Customize CSS** (or **Preview** tab)
4. Enter CSS in the editor
5. Click **Save**

**Advantages**: Live preview, syntax highlighting, scoped testing
**Limitations**: Requires login, changes must be saved

### Method 3: Direct CSS File (Advanced)

Save CSS to a file (e.g., `mytheme.css`) and import via:

```css
@import url('https://your-server.com/themes/mytheme.css');
```

**Advantages**: Reusable across multiple overlays, version control
**Limitations**: Requires web hosting

---

## DOM Structure Reference

### Complete HTML Structure

Every chat message follows this structure:

```html
<div class="min-h-screen w-full p-4">
  <!-- Your custom CSS is injected here -->
  <style>/* custom_css */</style>

  <!-- Message container -->
  <div class="space-y-3">

    <!-- Individual message card -->
    <div class="bg-gray-900/90 backdrop-blur-sm rounded-lg p-3 shadow-lg">

      <!-- Left: Avatar -->
      <div class="flex-shrink-0">
        <img src="avatar.png" class="w-10 h-10 rounded-full" alt="Avatar" />
        <!-- OR if no avatar: -->
        <div class="w-10 h-10 rounded-full bg-gray-700 flex items-center justify-center text-white text-sm font-semibold">
          AA <!-- User initials -->
        </div>
      </div>

      <!-- Right: Content -->
      <div class="flex-1 min-w-0">

        <!-- Header: Platform + Username + Badges -->
        <div class="flex items-center gap-2">
          <!-- Platform badge -->
          <span class="text-xs font-semibold uppercase text-purple-400">TWITCH</span>

          <!-- Username -->
          <span class="font-semibold text-sm">Username123</span>

          <!-- User badges (moderator, subscriber, etc.) -->
          <div class="flex gap-1">
            <img src="badge1.png" class="w-4 h-4" alt="Badge" />
            <img src="badge2.png" class="w-4 h-4" alt="Badge" />
          </div>
        </div>

        <!-- Message text with inline emotes -->
        <div class="text-white break-words" style="fontSize: 16px">
          Hello world!
          <img src="emote.png" class="inline-block h-[1.4em] w-auto align-text-bottom mx-0.5" alt="Kappa" />
        </div>

        <!-- Timestamp -->
        <div class="text-xs text-gray-500 mt-1">12:34:56</div>
      </div>

    </div>
    <!-- End message card -->

  </div>
  <!-- End message container -->
</div>
```

### Visual Diagram

```
┌─────────────────────────────────────────────────────────┐
│ .min-h-screen (Full screen container)                  │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ .space-y-3 (Message list container)                 │ │
│ │ ┌─────────────────────────────────────────────────┐ │ │
│ │ │ .space-y-3 > div (Message card) ←── MAIN TARGET│ │ │
│ │ │ ┌──────┐ ┌────────────────────────────────────┐ │ │ │
│ │ │ │Avatar│ │ Platform | Username | Badges       │ │ │ │
│ │ │ │      │ │ Message text with emotes           │ │ │ │
│ │ │ │      │ │ Timestamp                          │ │ │ │
│ │ │ └──────┘ └────────────────────────────────────┘ │ │ │
│ │ └─────────────────────────────────────────────────┘ │ │
│ │ ┌─────────────────────────────────────────────────┐ │ │
│ │ │ Next message...                                 │ │ │
│ │ └─────────────────────────────────────────────────┘ │ │
│ └─────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

---

## CSS Classes and Selectors

### Complete Class Reference Table

| Element | CSS Selector | Default Styles | Notes |
|---------|-------------|----------------|-------|
| **Main container** | `.min-h-screen` | Full viewport height, 16px padding | Root element |
| **Message list** | `.space-y-3` | 12px vertical spacing between messages | Container for all messages |
| **Message card** | `.space-y-3 > div` | `bg-gray-900/90 backdrop-blur-sm rounded-lg p-3 shadow-lg` | **PRIMARY CUSTOMIZATION TARGET** |
| **Avatar container** | `.flex-shrink-0` | Flexbox, no shrinking | Contains avatar image or initials |
| **Avatar image** | `.w-10.h-10.rounded-full` | 40x40px, circular | User avatar |
| **Avatar fallback** | `.w-10.h-10.rounded-full.bg-gray-700` | Circular div with initials | Shown when no avatar |
| **Platform badge** | `.platform-badge` | Wraps both text and icon variants | Parent element for platform indicator |
| **Platform badge (text)** | `.platform-badge-text` | 12px, bold, uppercase | Text labels: "TWITCH", "YOUTUBE", etc. |
| **Platform badge (icon)** | `.platform-badge-icon` | 16x16px SVG icon | Platform logos when icon mode enabled |
| **Platform colors** | `.text-purple-400` (Twitch)<br>`.text-red-400` (YouTube)<br>`.text-green-400` (Kick)<br>`.text-gray-400` (TikTok) | Platform-specific accent colors | Used for text platform badges |
| **Username** | `.font-semibold.text-sm` | Bold, 14px | User's display name |
| **Badge container** | `.flex.gap-1` | Horizontal flex, 4px gap | Contains badge images |
| **Badge images** | `.flex.gap-1 img` | 16x16px | Subscriber, moderator, etc. badges |
| **Message text** | `.text-white.break-words` | White text, word wrapping | Actual message content |
| **Emotes** | `.inline-block.h-\[1\.4em\]` | 1.4x line height, inline | Emote images in message |
| **Timestamp** | `.text-xs.text-gray-500.mt-1` | 12px, gray, 4px top margin | Message time |

### Important CSS Selector Notes

**✅ CORRECT - Target direct children only:**
```css
.space-y-3 > div { }
```

**❌ INCORRECT - Too broad, affects nested elements:**
```css
.space-y-3 div { }
```

**Always use `!important`** to override Tailwind defaults:
```css
.space-y-3 > div {
  background: #000 !important;  /* ✅ CORRECT */
  background: #000;              /* ❌ May not work */
}
```

---

## Common Customizations

### 1. Change Message Background

```css
/* Solid color background */
.space-y-3 > div {
  background: #1a1a2e !important;
}

/* Transparent background with blur */
.space-y-3 > div {
  background: rgba(0, 0, 0, 0.7) !important;
  backdrop-filter: blur(10px) !important;
}

/* Gradient background */
.space-y-3 > div {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%) !important;
}
```

### 2. Add/Modify Borders

```css
/* Simple border */
.space-y-3 > div {
  border: 2px solid #9146FF !important;
  border-radius: 12px !important;
}

/* Glowing border effect */
.space-y-3 > div {
  border: 2px solid #9146FF !important;
  box-shadow: 0 0 15px rgba(145, 70, 255, 0.5) !important;
}

/* Retro 3D border (Windows 95 style) */
.space-y-3 > div {
  border-top: 2px solid #dfdfdf !important;
  border-left: 2px solid #dfdfdf !important;
  border-right: 2px solid #0a0a0a !important;
  border-bottom: 2px solid #0a0a0a !important;
  box-shadow: inset 1px 1px 0 #fff, inset -1px -1px 0 #808080 !important;
}
```

### 3. Change Username Color

```css
/* Single color for all usernames */
.font-semibold.text-sm {
  color: #FFD700 !important;
}

/* Rainbow gradient username */
.font-semibold.text-sm {
  background: linear-gradient(90deg, #ff0000, #ff7f00, #ffff00, #00ff00, #0000ff, #4b0082, #9400d3) !important;
  -webkit-background-clip: text !important;
  -webkit-text-fill-color: transparent !important;
  background-clip: text !important;
}
```

### 4. Change Message Text Style

```css
/* Font and color */
.text-white.break-words {
  font-family: 'Comic Sans MS', cursive !important;
  color: #f0f0f0 !important;
  font-size: 18px !important;
  line-height: 1.6 !important;
}

/* Text shadow for better readability */
.text-white.break-words {
  text-shadow: 2px 2px 4px rgba(0, 0, 0, 0.8) !important;
}
```

### 5. Customize Emotes

```css
/* Larger emotes */
.text-white.break-words img {
  height: 2em !important;
  width: auto !important;
}

/* Emote glow effect */
.text-white.break-words img {
  filter: drop-shadow(0 0 5px rgba(255, 255, 255, 0.5)) !important;
}

/* Pixelated emotes (retro style) */
.text-white.break-words img {
  image-rendering: pixelated !important;
  image-rendering: crisp-edges !important;
}
```

### 6. Hide Elements

```css
/* Hide avatars */
.flex-shrink-0 {
  display: none !important;
}

/* Hide platform badges */
.text-xs.font-semibold.uppercase {
  display: none !important;
}

/* Hide user badges (subscriber, mod, etc.) */
.flex.gap-1 {
  display: none !important;
}

/* Hide timestamps */
.text-xs.text-gray-500 {
  display: none !important;
}

/* Hide platform badges entirely */
.platform-badge {
  display: none !important;
}
```

### 7. Customize Platform Badges

```css
/* Style platform text badges */
.platform-badge-text {
  font-size: 10px !important;
  padding: 2px 6px !important;
  border-radius: 4px !important;
  background: rgba(255, 255, 255, 0.1) !important;
}

/* Style platform icon badges */
.platform-badge-icon svg {
  width: 20px !important;
  height: 20px !important;
  filter: drop-shadow(0 0 4px rgba(255, 255, 255, 0.3)) !important;
}

/* Add background to platform icons */
.platform-badge-icon {
  background: rgba(0, 0, 0, 0.5) !important;
  padding: 4px !important;
  border-radius: 4px !important;
}

/* Hide only text platform badges (keep icons) */
.platform-badge-text {
  display: none !important;
}

/* Hide only icon platform badges (keep text) */
.platform-badge-icon {
  display: none !important;
}
```

### 8. Modify Animations

```css
/* Faster animation */
.space-y-3 > div {
  animation-duration: 150ms !important;
}

/* Fade in instead of slide */
@keyframes custom-fade-in {
  from { opacity: 0; }
  to { opacity: 1; }
}

.space-y-3 > div {
  animation: custom-fade-in 300ms ease-out !important;
}

/* Disable animations */
.space-y-3 > div {
  animation: none !important;
  transition: none !important;
}
```

### 9. Change Avatar Style

```css
/* Square avatars */
.w-10.h-10.rounded-full {
  border-radius: 0 !important;
}

/* Avatar border */
.w-10.h-10.rounded-full {
  border: 3px solid #9146FF !important;
}

/* Larger avatars */
.w-10.h-10.rounded-full,
.flex-shrink-0 > div {
  width: 60px !important;
  height: 60px !important;
}
```

---

## Platform-Specific Styling

### Target Messages from Specific Platforms

Use the `:has()` CSS selector to apply styles based on platform:

```css
/* Style all Twitch messages (purple accent) */
.space-y-3 > div:has(.text-purple-400) {
  background: rgba(145, 70, 255, 0.1) !important;
  border-left: 4px solid #9146FF !important;
}

/* Style all YouTube messages (red accent) */
.space-y-3 > div:has(.text-red-400) {
  background: rgba(255, 0, 0, 0.1) !important;
  border-left: 4px solid #FF0000 !important;
}

/* Style all Kick messages (green accent) */
.space-y-3 > div:has(.text-green-400) {
  background: rgba(83, 252, 24, 0.1) !important;
  border-left: 4px solid #53FC18 !important;
}

/* Style all TikTok messages (gray accent) */
.space-y-3 > div:has(.text-gray-400) {
  background: rgba(128, 128, 128, 0.1) !important;
  border-left: 4px solid #808080 !important;
}
```

### Change Platform Badge Colors

```css
/* Override Twitch badge color */
.text-purple-400 {
  color: #A970FF !important; /* Lighter purple */
}

/* Override YouTube badge color */
.text-red-400 {
  color: #FF4444 !important; /* Brighter red */
}

/* Override Kick badge color */
.text-green-400 {
  color: #66FF33 !important; /* Neon green */
}
```

### Hide Specific Platforms

```css
/* Hide all Twitch messages */
.space-y-3 > div:has(.text-purple-400) {
  display: none !important;
}

/* Hide all YouTube messages */
.space-y-3 > div:has(.text-red-400) {
  display: none !important;
}
```

---

## Display Settings

### Configuration Options (Non-CSS)

These settings are configured via the All-Chat dashboard **Display Settings** panel (not CSS):

| Setting | Range | Default | Description |
|---------|-------|---------|-------------|
| **Font Size** | 12-32px | 16px | Base font size for messages |
| **Message Duration** | 5-60s | 15s | How long messages stay visible |
| **Max Messages** | 10-100 | 50 | Maximum messages displayed at once |
| **Font Family** | Any web font | `sans-serif` | Font for message text |
| **Show Badges** | true/false | true | Display user badges (sub, mod, etc.) |
| **Show Avatars** | true/false | true | Display user avatars |
| **Animation** | slide/fade/none | slide | Message entrance animation |
| **Platform Badge Position** | before/after | before | Show platform badge before or after username |
| **Platform Badge Style** | text/icon | text | Display platform as text label or icon |

### Override Display Settings with CSS

While the above settings are preferred, you can override them with CSS:

```css
/* Override font size */
.text-white.break-words {
  font-size: 20px !important;
}

/* Override font family */
.text-white.break-words {
  font-family: 'Roboto', sans-serif !important;
}

/* Force hide badges (overrides show_badges setting) */
.flex.gap-1 {
  display: none !important;
}

/* Force hide avatars (overrides show_avatars setting) */
.flex-shrink-0 {
  display: none !important;
}
```

---

## Platform Status Indicators

### What Are Platform Status Indicators?

Platform status indicators are small icons displayed in the top-right corner of your overlay that show which backend listeners are actively monitoring your chat sources. Each platform (Twitch, YouTube, Kick, TikTok) has its own icon that appears:
- **In color** when the backend listener is actively monitoring that platform
- **In grayscale** when the listener is not active or disconnected

This helps you quickly see at a glance which platforms are being monitored by All-Chat.

### Default Behavior

Platform status indicators are **shown by default** and will automatically update every 30 seconds to reflect the current status of your backend listeners.

### Hide All Platform Indicators

To completely hide the platform status indicators:

```css
/* Hide platform status indicators entirely */
.platform-status-indicators {
  display: none !important;
}
```

### Hide Specific Platform Indicators

To hide specific platforms while keeping others visible:

```css
/* Hide only Twitch indicator */
.platform-indicator-twitch {
  display: none !important;
}

/* Hide only YouTube indicator */
.platform-indicator-youtube {
  display: none !important;
}

/* Hide only Kick indicator */
.platform-indicator-kick {
  display: none !important;
}

/* Hide only TikTok indicator */
.platform-indicator-tiktok {
  display: none !important;
}
```

### Customize Indicator Position

```css
/* Move indicators to bottom-right */
.platform-status-indicators {
  top: auto !important;
  bottom: 16px !important;
  right: 16px !important;
}

/* Move indicators to top-left */
.platform-status-indicators {
  left: 16px !important;
  right: auto !important;
}

/* Move indicators to bottom-left */
.platform-status-indicators {
  top: auto !important;
  bottom: 16px !important;
  left: 16px !important;
  right: auto !important;
}

/* Center indicators at the top */
.platform-status-indicators {
  left: 50% !important;
  right: auto !important;
  transform: translateX(-50%) !important;
}
```

### Customize Indicator Appearance

```css
/* Change indicator size */
.platform-indicator {
  width: 48px !important;
  height: 48px !important;
}

/* Change indicator spacing */
.platform-status-indicators {
  gap: 8px !important;
}

/* Change container background */
.platform-status-indicators {
  background: rgba(0, 0, 0, 0.9) !important;
}

/* Change inactive indicator opacity */
.platform-indicator.grayscale {
  opacity: 0.2 !important;
}

/* Remove background from container */
.platform-status-indicators {
  background: transparent !important;
  backdrop-filter: none !important;
  padding: 0 !important;
}

/* Add custom border to active indicators */
.platform-indicator:not(.grayscale) {
  border: 2px solid white !important;
}
```

### Advanced: Platform-Specific Styling

```css
/* Custom styling for active Twitch indicator */
.platform-indicator-twitch:not(.grayscale) {
  background: rgba(145, 70, 255, 0.3) !important;
  box-shadow: 0 0 10px rgba(145, 70, 255, 0.5) !important;
}

/* Custom styling for active YouTube indicator */
.platform-indicator-youtube:not(.grayscale) {
  background: rgba(255, 0, 0, 0.3) !important;
  box-shadow: 0 0 10px rgba(255, 0, 0, 0.5) !important;
}

/* Custom styling for active Kick indicator */
.platform-indicator-kick:not(.grayscale) {
  background: rgba(83, 252, 24, 0.3) !important;
  box-shadow: 0 0 10px rgba(83, 252, 24, 0.5) !important;
}

/* Custom styling for active TikTok indicator */
.platform-indicator-tiktok:not(.grayscale) {
  background: rgba(6, 217, 210, 0.3) !important;
  box-shadow: 0 0 10px rgba(6, 217, 210, 0.5) !important;
}
```

### Make Indicators More Subtle

```css
/* Reduce size and opacity for minimal look */
.platform-status-indicators {
  opacity: 0.6 !important;
}

.platform-indicator {
  width: 24px !important;
  height: 24px !important;
}

/* Show indicators only on hover */
.platform-status-indicators {
  opacity: 0 !important;
  transition: opacity 0.3s !important;
}

.platform-status-indicators:hover {
  opacity: 1 !important;
}
```

---

## Advanced Techniques

### 1. Custom Fonts (Web Fonts)

```css
/* Import Google Font */
@import url('https://fonts.googleapis.com/css2?family=Roboto:wght@400;700&display=swap');

/* Apply to message text */
.text-white.break-words {
  font-family: 'Roboto', sans-serif !important;
}

/* Apply to usernames */
.font-semibold.text-sm {
  font-family: 'Roboto', sans-serif !important;
  font-weight: 700 !important;
}
```

### 2. Custom Scrollbar

```css
/* Webkit browsers (Chrome, Edge, Safari) */
.min-h-screen::-webkit-scrollbar {
  width: 12px;
}

.min-h-screen::-webkit-scrollbar-track {
  background: #1a1a1a;
}

.min-h-screen::-webkit-scrollbar-thumb {
  background: #9146FF;
  border-radius: 6px;
}

.min-h-screen::-webkit-scrollbar-thumb:hover {
  background: #A970FF;
}

/* Firefox */
.min-h-screen {
  scrollbar-width: thin;
  scrollbar-color: #9146FF #1a1a1a;
}
```

### 3. Pseudo-Elements for Decorations

```css
/* Add title bar above messages */
.space-y-3 > div::before {
  content: '💬 CHAT MESSAGE';
  display: block;
  background: linear-gradient(90deg, #000080, #1084d0);
  color: white;
  padding: 2px 4px;
  font-size: 11px;
  font-weight: bold;
  margin: -12px -12px 8px -12px;
  font-family: 'MS Sans Serif', sans-serif;
}
```

### 4. Conditional Styling with `:nth-child()`

```css
/* Alternate message backgrounds */
.space-y-3 > div:nth-child(odd) {
  background: rgba(20, 20, 30, 0.9) !important;
}

.space-y-3 > div:nth-child(even) {
  background: rgba(30, 30, 40, 0.9) !important;
}
```

### 5. Responsive Sizing (Based on Viewport)

```css
/* Scale messages based on browser width */
@media (max-width: 768px) {
  .space-y-3 > div {
    padding: 8px !important;
    font-size: 14px !important;
  }
}

@media (min-width: 1920px) {
  .space-y-3 > div {
    padding: 16px !important;
    font-size: 20px !important;
  }
}
```

### 6. Custom Animations

```css
/* Define custom animation */
@keyframes bounce-in {
  0% {
    transform: scale(0.3);
    opacity: 0;
  }
  50% {
    transform: scale(1.05);
  }
  70% {
    transform: scale(0.9);
  }
  100% {
    transform: scale(1);
    opacity: 1;
  }
}

/* Apply to messages */
.space-y-3 > div {
  animation: bounce-in 500ms ease-out !important;
}
```

### 7. Badge Customization

```css
/* Larger badges */
.flex.gap-1 img {
  width: 20px !important;
  height: 20px !important;
}

/* Badge glow effect */
.flex.gap-1 img {
  filter: drop-shadow(0 0 3px rgba(255, 215, 0, 0.6)) !important;
}

/* Circular badge background */
.flex.gap-1 img {
  background: rgba(255, 255, 255, 0.1) !important;
  border-radius: 50% !important;
  padding: 2px !important;
}
```

---

## Event Display Styling

All-Chat supports displaying platform events (subscriptions, donations, raids, channel points, Super Chats, gifts, follows, likes) alongside chat messages. These events have special CSS classes for customization.

### What Are Events?

Events are special messages that appear in your overlay when viewers:
- **Twitch**: Subscribe, gift subs, raid, cheer bits, redeem channel points
- **YouTube**: Send Super Chats, buy memberships, celebrate milestones
- **TikTok**: Send gifts, follow, like, share
- **Kick**: Subscribe, send gifts/donations (coming soon)

Events are controlled via the **Event Settings** page at `/overlays/{id}/events`.

### Event Message Structure

Events appear as special message cards with tier-based styling:

```html
<div class="event-message event-tier-high event-type-subscription">
  <!-- Event icon/emoji -->
  <div class="event-icon">🎉</div>

  <!-- Event details -->
  <div class="event-details">
    <div class="event-title">New Subscriber!</div>
    <div class="event-user">Username just subscribed</div>
    <div class="event-metadata">Tier 1 • 3 month streak</div>
  </div>
</div>
```

### Event CSS Classes

| Class | Purpose | Applied To |
|-------|---------|-----------|
| `.event-message` | Base container for all events | All event messages |
| `.event-tier-high` | High-value events (30-60s display) | Subs, large donations, big raids |
| `.event-tier-medium` | Medium-value events (15-20s display) | Follows, milestones, small gifts |
| `.event-tier-low` | Low-value events (5-10s display) | Likes, shares, small bits |
| `.event-type-subscription` | Twitch subscription events | Subs and resubs |
| `.event-type-gift_subscription` | Twitch gift sub events | Gift subs and mystery gifts |
| `.event-type-raid` | Twitch raid events | Incoming raids |
| `.event-type-bits` | Twitch bits events | Bit cheers |
| `.event-type-channel_points` | Twitch channel points | Point redemptions |
| `.event-type-super_chat` | YouTube Super Chat | Paid messages |
| `.event-type-super_sticker` | YouTube Super Stickers | Paid stickers |
| `.event-type-member` | YouTube memberships | New members |
| `.event-type-member_milestone` | YouTube milestones | Anniversary celebrations |
| `.event-type-member_gift` | YouTube member gifts | Gifted memberships |
| `.event-type-gift` | TikTok/Kick gifts | Virtual gifts |
| `.event-type-follow` | TikTok follows | New followers |
| `.event-type-like_aggregate` | TikTok like aggregation | Aggregated likes |
| `.event-type-share` | TikTok shares | Stream shares |

### Event Tier Styling

Events are automatically assigned tiers based on their value/importance:

#### High-Value Events (Tier High)
- Subscriptions and resubscriptions
- Large donations ($10+)
- Big raids (1000+ viewers)
- High-value Super Chats
- Large gift bombs

**Default Style:**
```css
.event-tier-high {
  border: 2px solid gold;
  box-shadow: 0 0 20px rgba(255, 215, 0, 0.5);
  background: linear-gradient(135deg, rgba(255, 215, 0, 0.1), rgba(255, 165, 0, 0.1));
}
```

#### Medium-Value Events (Tier Medium)
- Follows
- Member milestones
- Small gifts
- Medium Super Chats

**Default Style:**
```css
.event-tier-medium {
  border: 2px solid #9146FF;
  box-shadow: 0 0 10px rgba(145, 70, 255, 0.3);
}
```

#### Low-Value Events (Tier Low)
- Likes (aggregated)
- Shares
- Small bits

**Default Style:**
```css
.event-tier-low {
  border: 1px solid #4A90E2;
  opacity: 0.9;
}
```

### Common Event Customizations

#### 1. Hide All Events

```css
/* Hide all event messages (show only chat) */
.event-message {
  display: none !important;
}
```

#### 2. Hide Specific Event Types

```css
/* Hide subscription events */
.event-type-subscription {
  display: none !important;
}

/* Hide TikTok likes */
.event-type-like_aggregate {
  display: none !important;
}

/* Hide all low-tier events */
.event-tier-low {
  display: none !important;
}
```

#### 3. Customize Event Backgrounds by Tier

```css
/* Gold gradient for high-value events */
.event-tier-high {
  background: linear-gradient(135deg, #FFD700 0%, #FFA500 100%) !important;
  color: #000 !important;
}

/* Purple theme for medium events */
.event-tier-medium {
  background: rgba(145, 70, 255, 0.2) !important;
  border-left: 5px solid #9146FF !important;
}

/* Subtle style for low events */
.event-tier-low {
  background: rgba(255, 255, 255, 0.05) !important;
  opacity: 0.7 !important;
}
```

#### 4. Platform-Specific Event Styling

```css
/* Style all Twitch events (subscriptions, raids, etc.) */
.event-message[class*="event-type-subscription"],
.event-message[class*="event-type-raid"],
.event-message[class*="event-type-bits"] {
  border-left: 5px solid #9146FF !important;
  background: rgba(145, 70, 255, 0.1) !important;
}

/* Style YouTube events (Super Chats, memberships) */
.event-message[class*="event-type-super_chat"],
.event-message[class*="event-type-member"] {
  border-left: 5px solid #FF0000 !important;
  background: rgba(255, 0, 0, 0.1) !important;
}

/* Style TikTok events (gifts, follows, likes) */
.event-message[class*="event-type-gift"],
.event-message[class*="event-type-follow"],
.event-message[class*="event-type-like_aggregate"] {
  border-left: 5px solid #00F2EA !important;
  background: rgba(0, 242, 234, 0.1) !important;
}
```

#### 5. Animated Event Effects

```css
/* Pulsing glow for high-value events */
@keyframes pulse-gold {
  0%, 100% {
    box-shadow: 0 0 20px rgba(255, 215, 0, 0.5);
  }
  50% {
    box-shadow: 0 0 40px rgba(255, 215, 0, 0.8);
  }
}

.event-tier-high {
  animation: pulse-gold 2s infinite !important;
}

/* Slide-in animation for raids */
@keyframes raid-sweep {
  0% {
    transform: translateX(-100%);
    opacity: 0;
  }
  100% {
    transform: translateX(0);
    opacity: 1;
  }
}

.event-type-raid {
  animation: raid-sweep 500ms ease-out !important;
}

/* Heartbeat animation for likes */
@keyframes heartbeat {
  0%, 100% {
    transform: scale(1);
  }
  50% {
    transform: scale(1.05);
  }
}

.event-type-like_aggregate {
  animation: heartbeat 1s infinite !important;
}
```

#### 6. Super Chat Value-Based Styling

YouTube Super Chats have different colors based on their value. You can customize these:

```css
/* High-value Super Chats ($50+) - Red */
.event-type-super_chat.event-tier-high {
  background: linear-gradient(90deg, #E62117 0%, #FF6B6B 100%) !important;
  color: white !important;
  font-weight: bold !important;
}

/* Medium Super Chats ($10-$49) - Magenta */
.event-type-super_chat.event-tier-medium {
  background: linear-gradient(90deg, #E91E63 0%, #F48FB1 100%) !important;
}

/* Low Super Chats ($1-$9) - Blue */
.event-type-super_chat.event-tier-low {
  background: linear-gradient(90deg, #1565C0 0%, #64B5F6 100%) !important;
}
```

#### 7. Larger/Smaller Event Messages

```css
/* Make events larger than chat */
.event-message {
  transform: scale(1.1) !important;
  margin: 12px 0 !important;
}

/* Make events smaller and subtle */
.event-message {
  transform: scale(0.9) !important;
  opacity: 0.8 !important;
}
```

#### 8. Event Icon Customization

```css
/* Larger event icons */
.event-icon {
  font-size: 2em !important;
}

/* Animated event icons */
.event-icon {
  animation: bounce 1s infinite !important;
}

@keyframes bounce {
  0%, 100% {
    transform: translateY(0);
  }
  50% {
    transform: translateY(-10px);
  }
}

/* Hide event icons */
.event-icon {
  display: none !important;
}
```

### Event Display Settings

Events can be configured per-overlay at `/overlays/{id}/events`:

**Configurable Options:**
- Enable/disable specific event types (18 toggles)
- TikTok like aggregation window (10-60 seconds)
- Event display duration multiplier (0.1-5.0x)

**Platform-Specific Controls:**
- **Twitch**: Subs, resubs, gift subs, bits, raids, channel points
- **YouTube**: Super Chats, Super Stickers, members, milestones, gifts
- **TikTok**: Gifts, follows, likes, shares
- **Kick**: Coming soon

### Advanced: Event Priority and Stacking

Events are displayed with priority based on their tier:

```css
/* Z-index layering for overlapping events */
.event-tier-high {
  z-index: 100 !important;
}

.event-tier-medium {
  z-index: 50 !important;
}

.event-tier-low {
  z-index: 10 !important;
}

/* Prevent event stacking (show one at a time) */
.event-message + .event-message {
  margin-top: 8px !important;
}
```

### Example: Complete Event Theme

```css
/* Neon Event Theme - Bright, attention-grabbing events */

/* Base event styling */
.event-message {
  background: rgba(0, 0, 0, 0.95) !important;
  border-radius: 12px !important;
  padding: 16px !important;
  border: 2px solid transparent !important;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.5) !important;
}

/* High-value events - Gold neon */
.event-tier-high {
  border-color: #FFD700 !important;
  box-shadow: 0 0 30px rgba(255, 215, 0, 0.6),
              0 4px 20px rgba(0, 0, 0, 0.5) !important;
  animation: neon-pulse 2s infinite !important;
}

/* Medium-value events - Purple neon */
.event-tier-medium {
  border-color: #9146FF !important;
  box-shadow: 0 0 20px rgba(145, 70, 255, 0.5),
              0 4px 20px rgba(0, 0, 0, 0.5) !important;
}

/* Low-value events - Cyan neon */
.event-tier-low {
  border-color: #00FFFF !important;
  box-shadow: 0 0 15px rgba(0, 255, 255, 0.4),
              0 4px 20px rgba(0, 0, 0, 0.5) !important;
}

/* Pulse animation */
@keyframes neon-pulse {
  0%, 100% {
    box-shadow: 0 0 30px rgba(255, 215, 0, 0.6),
                0 4px 20px rgba(0, 0, 0, 0.5);
  }
  50% {
    box-shadow: 0 0 50px rgba(255, 215, 0, 0.9),
                0 4px 20px rgba(0, 0, 0, 0.5);
  }
}

/* Subscription events - Extra special */
.event-type-subscription {
  background: linear-gradient(135deg,
    rgba(255, 215, 0, 0.1),
    rgba(255, 165, 0, 0.1)) !important;
}

/* Raid events - Sweep animation */
.event-type-raid {
  animation: raid-sweep 600ms ease-out, neon-pulse 2s infinite !important;
}

@keyframes raid-sweep {
  0% {
    transform: translateX(-100%);
    opacity: 0;
  }
  100% {
    transform: translateX(0);
    opacity: 1;
  }
}
```

---

## Example Themes

### Theme 1: Minimal Dark

```css
/* Clean, minimal design with focus on readability */
.space-y-3 > div {
  background: rgba(0, 0, 0, 0.85) !important;
  border: none !important;
  border-left: 3px solid #9146FF !important;
  border-radius: 4px !important;
  padding: 10px !important;
}

.font-semibold.text-sm {
  color: #9146FF !important;
}

.text-white.break-words {
  color: #E0E0E0 !important;
  font-size: 16px !important;
}

/* Hide clutter */
.text-xs.font-semibold.uppercase,
.text-xs.text-gray-500 {
  display: none !important;
}
```

### Theme 2: Neon Cyberpunk

```css
/* Bright neon colors with glow effects */
.space-y-3 > div {
  background: rgba(10, 10, 30, 0.9) !important;
  border: 2px solid #00FFFF !important;
  border-radius: 0 !important;
  box-shadow: 0 0 20px rgba(0, 255, 255, 0.5) !important;
}

.font-semibold.text-sm {
  color: #FF00FF !important;
  text-shadow: 0 0 10px rgba(255, 0, 255, 0.8) !important;
}

.text-white.break-words {
  color: #00FFFF !important;
  text-shadow: 0 0 5px rgba(0, 255, 255, 0.5) !important;
}

.text-white.break-words img {
  filter: drop-shadow(0 0 10px rgba(255, 255, 255, 0.8)) !important;
}
```

### Theme 3: Retro Terminal

```css
/* Monospace font, green-on-black terminal style */
@import url('https://fonts.googleapis.com/css2?family=Source+Code+Pro:wght@400;700&display=swap');

.space-y-3 > div {
  background: #000000 !important;
  border: 1px solid #00FF00 !important;
  border-radius: 0 !important;
  padding: 8px !important;
  font-family: 'Source Code Pro', monospace !important;
}

.font-semibold.text-sm {
  color: #00FF00 !important;
}

.text-white.break-words {
  color: #00FF00 !important;
  font-family: 'Source Code Pro', monospace !important;
}

/* Scanline effect */
.min-h-screen::before {
  content: '';
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: repeating-linear-gradient(
    0deg,
    rgba(0, 255, 0, 0.05),
    rgba(0, 255, 0, 0.05) 1px,
    transparent 1px,
    transparent 2px
  );
  pointer-events: none;
}
```

### Theme 4: Windows 98 (See Full Example)

A complete Windows 98-themed overlay is available at:
`docs/overlay-themes/win98-theme.css` (330 lines)

**Features:**
- Classic 3D raised borders
- Inset button effects
- Pixelated rendering
- Custom title bars
- Win98 scrollbar styling

---

## Troubleshooting

### Problem: CSS Not Applying

**Possible Causes:**

1. **Missing `!important`**
   ```css
   /* ❌ Won't work */
   .space-y-3 > div { background: red; }

   /* ✅ Will work */
   .space-y-3 > div { background: red !important; }
   ```

2. **Wrong selector specificity**
   ```css
   /* ❌ Too broad, may conflict */
   div { background: red !important; }

   /* ✅ Specific selector */
   .space-y-3 > div { background: red !important; }
   ```

3. **CSS syntax error** - Check browser console (F12) for errors

4. **OBS caching** - Refresh the browser source:
   - Right-click source → Refresh Browser Source
   - Or toggle "Shutdown source when not visible"

### Problem: Changes Work Locally but Not in OBS

**Solutions:**

1. **Verify URL is correct** in OBS Browser Source
2. **Clear OBS browser cache**:
   - OBS → Settings → Advanced → Delete Cache → Restart OBS
3. **Check browser permissions** (if using localhost):
   - May need to run OBS as administrator
4. **Use absolute URLs** for fonts/images, not relative paths

### Problem: Font Not Loading

**Solutions:**

1. **Use Google Fonts or other CDN**:
   ```css
   @import url('https://fonts.googleapis.com/css2?family=Roboto&display=swap');
   ```

2. **Check font name spelling** (case-sensitive)

3. **Add fallback fonts**:
   ```css
   font-family: 'Roboto', Arial, sans-serif !important;
   ```

### Problem: Performance Issues (Lag/Stuttering)

**Optimization Tips:**

1. **Avoid expensive effects**:
   ```css
   /* ❌ Heavy performance impact */
   backdrop-filter: blur(20px) !important;
   box-shadow: 0 0 50px 50px rgba(0,0,0,0.9) !important;

   /* ✅ Lighter alternatives */
   background: rgba(0, 0, 0, 0.9) !important;
   box-shadow: 0 2px 8px rgba(0,0,0,0.5) !important;
   ```

2. **Use `transform` instead of `top`/`left`** for animations:
   ```css
   /* ✅ GPU-accelerated */
   transform: translateY(10px) !important;

   /* ❌ CPU-bound */
   top: 10px !important;
   ```

3. **Reduce animation duration/complexity**

4. **Lower `max_messages` setting** in Display Settings

### Problem: Emotes Not Displaying

**This is likely a backend issue, not CSS.** Check:

1. Message Processor service is running
2. Emote enrichment is enabled
3. External emote APIs (7TV, BTTV, FFZ) are reachable

**To force emote size via CSS:**
```css
.text-white.break-words img {
  height: 28px !important;
  width: auto !important;
}
```

---

## Best Practices

### ✅ DO

- **Always use `!important`** to override Tailwind styles
- **Test in OBS** after making changes (not just browser)
- **Use specific selectors** (`.space-y-3 > div`)
- **Include fallback fonts** for custom fonts
- **Optimize for performance** (avoid heavy blur/shadow)
- **Comment your CSS** for future reference

### ❌ DON'T

- **Don't use overly broad selectors** (`div { }`)
- **Don't forget `!important`** (styles won't apply)
- **Don't overuse blur effects** (performance killer)
- **Don't rely on JavaScript** (not supported in CSS field)
- **Don't use local file paths** (use CDN URLs)

---

## Additional Resources

### Official Documentation

- **Complete Theme Guide**: `docs/overlay-themes/README.md`
- **Quick Start Tutorial**: `docs/overlay-themes/QUICK-START.md`
- **Example Theme**: `docs/overlay-themes/win98-theme.css`

### Frontend Source Code (Advanced Users)

If you want to understand the underlying HTML structure:

- **Public Overlay Component**: `frontend/src/app/overlay/[id]/page.tsx`
- **Message Rendering Logic**: `frontend/src/lib/renderMessage.tsx`
- **TypeScript Types**: `frontend/src/lib/types/message.ts`

### Helpful CSS Resources

- [MDN Web Docs - CSS](https://developer.mozilla.org/en-US/docs/Web/CSS)
- [Google Fonts](https://fonts.google.com/)
- [Tailwind CSS Color Palette](https://tailwindcss.com/docs/customizing-colors)
- [CSS `:has()` Selector](https://developer.mozilla.org/en-US/docs/Web/CSS/:has)

---

## Support

If you encounter issues not covered in this guide:

1. Check the [GitHub Issues](https://github.com/your-repo/all-chat/issues)
2. Join the community Discord server
3. Submit a bug report with:
   - Your CSS code
   - Expected vs. actual behavior
   - Screenshots
   - Browser/OBS version

---

**Happy customizing! 🎨**

*Last updated: 2025-01-25*
