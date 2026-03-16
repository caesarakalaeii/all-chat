# Viewer Cosmetics Patterns

**Domain:** Username colors, gradients, avatar frames, viewer identity cosmetics
**Researched:** 2026-03-14
**Overall confidence:** HIGH for CSS techniques; MEDIUM for data model patterns (inferred from industry behavior)

## Sources

- Twitch username color system: https://www.streamscheme.com/change-twitch-name-color/
- CSS gradient text technique: https://www.tutorialpedia.org/blog/css-text-color-gradient-twitch-chat/
- Discord avatar decorations blog: https://discord.com/blog/avatar-decorations-collect-and-keep-the-newest-styles
- React gradient text patterns: https://tailwindcss.com/docs/background-clip
- CSS background-clip spec: https://www.w3schools.com/cssref/css3_pr_background-clip.php

---

## Username Color Systems

### How Twitch Handles Name Colors

Twitch stores a single hex color string per user account (e.g., `#FF4500`). Key design decisions:

- **Free tier**: 15 preset colors only. No custom hex.
- **Turbo/Prime tier**: Any hex color via `/color #RRGGBB` chat command or settings UI.
- **Storage**: Single `color` field in user profile. One color per account globally (not per channel).
- **Default fallback**: When a user has never set a color, a deterministic color is assigned based on username hash so it's stable across sessions.
- **Wire format**: Twitch IRC sends `color=#RRGGBB` in the IRCv3 tags on every message. The color travels with each message, not looked up on the receiving end.

**Implication for All-Chat:** The `UserInfo.Color` field already exists in the unified message model. For Twitch it's populated from IRC tags. For YouTube it's currently empty. For All-Chat viewer cosmetics, the same field should carry the viewer's chosen hex color.

### What "user name color" means in All-Chat context

For All-Chat viewers (people watching via the browser extension):
- They are identified by a viewer account (JWT token in the extension)
- They can optionally set a preferred display name color that appears in the chat overlay when their messages show up
- This is a preference stored server-side, attached to the viewer identity record

**Recommended data model:**
```sql
-- In a viewer_preferences table (or viewer_cosmetics):
viewer_id       UUID NOT NULL REFERENCES viewers(id)
name_color      VARCHAR(7)    -- "#RRGGBB" hex. NULL = use default/platform color
name_gradient   JSONB         -- NULL or {"type":"linear","colors":["#FF0000","#0000FF"],"angle":90}
avatar_frame_id UUID          -- NULL or FK to frames catalog
created_at      TIMESTAMPTZ
updated_at      TIMESTAMPTZ
```

---

## Name Color Rendering in React

### Solid hex color (simple case)

```tsx
// In the chat message component
<span
  className="font-bold"
  style={{ color: user.color || platformDefaultColor }}
>
  {user.display_name}
</span>
```

The `color` CSS property handles solid hex trivially.

### Gradient name color (Twitch-style premium feature)

The standard CSS technique uses `background-clip: text` with a transparent text fill:

```css
.username-gradient {
  background: linear-gradient(90deg, #FF0000, #0000FF);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  /* color: transparent is a fallback for browsers without background-clip:text support */
}
```

**React/Tailwind equivalent:**

Tailwind v4 exposes `bg-clip-text` and `text-transparent`:

```tsx
function GradientUsername({ gradient, name }: { gradient: NameGradient, name: string }) {
  const gradientCSS = `linear-gradient(${gradient.angle}deg, ${gradient.colors.join(', ')})`;
  return (
    <span
      className="font-bold bg-clip-text text-transparent"
      style={{ backgroundImage: gradientCSS }}
    >
      {name}
    </span>
  );
}
```

**Animated gradient (optional premium tier):**

```css
@keyframes shimmer {
  0%   { background-position: 0% 50%; }
  50%  { background-position: 100% 50%; }
  100% { background-position: 0% 50%; }
}

.username-animated-gradient {
  background: linear-gradient(90deg, #FF0000, #0000FF, #FF0000);
  background-size: 200% auto;
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  animation: shimmer 3s linear infinite;
}
```

### Name color rendering decision tree

```
user.name_color == null AND user.name_gradient == null
  → use platform default color (YouTube: white, Twitch: IRC color, Kick: white)

user.name_color == "#RRGGBB"
  → style={{ color: "#RRGGBB" }}

user.name_gradient != null
  → bg-clip-text text-transparent, style={{ backgroundImage: gradientCSS }}
```

### Browser compatibility note

`background-clip: text` is well-supported in all modern browsers. The `-webkit-` prefix is still recommended for Safari compatibility as of 2025. The non-prefixed version is in the CSS spec and supported by Chrome, Firefox, Edge. **No JavaScript required** — pure CSS.

---

## Avatar Frame / Flair Systems

### Industry pattern (Discord model)

Discord avatar decorations are **PNG images with transparency layered over the avatar**:

1. Avatar: circular `<img>` with `border-radius: 50%`
2. Frame/decoration: position-absolute PNG image that extends beyond the avatar circle, with transparency cut out to show the avatar in the center
3. The decoration PNG is typically larger than the avatar (e.g., avatar 40px, frame PNG 56px) to allow the frame to extend around the circle

**Implementation:**

```tsx
function Avatar({ avatarUrl, frameUrl, size = 40 }: AvatarProps) {
  return (
    <div className="relative inline-block" style={{ width: size, height: size }}>
      <img
        src={avatarUrl}
        className="rounded-full object-cover"
        style={{ width: size, height: size }}
        alt="Avatar"
      />
      {frameUrl && (
        <img
          src={frameUrl}
          className="absolute pointer-events-none select-none"
          style={{
            top: '50%',
            left: '50%',
            transform: 'translate(-50%, -50%)',
            width: size * 1.4,  // frame is 40% larger than avatar
            height: size * 1.4,
          }}
          alt=""
          aria-hidden="true"
        />
      )}
    </div>
  );
}
```

### For a simpler ring/border approach:

If the frame is just a colored ring (not an image overlay), use CSS `box-shadow` or `outline`:

```css
/* Simpler ring flair - gold/premium ring */
.avatar-premium {
  box-shadow: 0 0 0 2px #FFD700, 0 0 0 4px rgba(255, 215, 0, 0.3);
  border-radius: 50%;
}

/* Or with CSS outline (no layout impact) */
.avatar-frame-gold {
  outline: 2px solid #FFD700;
  outline-offset: 2px;
  border-radius: 50%;
}
```

### Frame image format recommendations:

- PNG with transparency: 80x80px minimum (for a 40px avatar display)
- Two sizes: 1x (80px) and 2x (160px) for retina
- The center 40x40 area must be fully transparent so the user's avatar shows through
- Animate with CSS `@keyframes` on the frame image if desired (spinning, pulsing)
- Store frame images in an object storage bucket (S3/R2), serve via CDN

### Database model for frames catalog:

```sql
CREATE TABLE cosmetic_frames (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(64) NOT NULL,
    image_url   TEXT NOT NULL,   -- 1x PNG
    image_url_2x TEXT,           -- 2x PNG for retina
    is_premium  BOOLEAN DEFAULT true,
    sort_order  INT DEFAULT 0,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);
```

---

## Name Badge Flair (Text-Based)

Some platforms use small colored text badges next to usernames (e.g., StreamElements "VIP" badge, Discord server boosters). These are simpler than image badges:

```tsx
function NameFlair({ flair }: { flair: string }) {
  return (
    <span className="inline-block px-1 py-0 rounded text-xs font-bold bg-purple-600 text-white ml-1">
      {flair}
    </span>
  );
}
```

For All-Chat, a viewer premium badge could be rendered this way without any image loading.

---

## All-Chat Specific Recommendations

### What to store per viewer:

```typescript
interface ViewerCosmetics {
  name_color: string | null;       // "#RRGGBB" or null
  name_gradient: {                 // null = not set
    type: 'linear';
    colors: string[];              // 2-4 hex colors
    angle: number;                 // 0-360 degrees
    animated: boolean;
  } | null;
  avatar_frame_id: string | null;  // null = no frame
  name_flair: string | null;       // null or short text like "VIP", "MOD"
}
```

### What arrives in the unified message to the frontend:

The `UserInfo` struct already has `Color string`. For gradient support, a new field or metadata entry is needed:

```go
// In models/message.go UserInfo:
Color          string  `json:"color,omitempty"`
// New:
ColorGradient  *GradientConfig `json:"color_gradient,omitempty"`
AvatarFrameURL string          `json:"avatar_frame_url,omitempty"`
```

### Where to inject cosmetics:

The cosmetics enrichment step runs in the message processor after normalization. The message processor fetches viewer cosmetics from a Redis cache (TTL: 5 minutes) keyed by `viewer:{platform}:{user_id}`. On cache miss, it queries the database (new `viewer_cosmetics` table).

This is analogous to how the badge enricher works today — a separate enricher that decorates the `UserInfo` after platform normalization.

### Performance note:

Loading cosmetics per-message from Redis is fine at scale. A Redis GET takes ~0.1ms. With 20 messages/sec per channel, cosmetic lookups add negligible overhead. The cache should be populated on first viewer login and invalidated on settings change.
