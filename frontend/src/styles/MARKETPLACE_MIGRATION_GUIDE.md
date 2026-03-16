# Overlay Marketplace Migration Guide — v1.3

**Companion to:** [EVENTS_CSS_API.md](./EVENTS_CSS_API.md)
**Updated:** 2026-03-14

This guide is for overlay theme authors who publish CSS themes to the All-Chat marketplace. It documents what changed in v1.3 (Frontend Redesign) and how to update your themes.

---

## What Changed in v1.3

### CSS Cascade Layer Architecture (New)

The most important change in v1.3 is the introduction of CSS cascade layers. This **eliminates the need for `!important` in your theme CSS**.

The full layer order is:

```
@layer base, design-system, marketplace-themes, user-overrides;
```

| Layer                | Priority | Who writes it                          |
| -------------------- | -------- | -------------------------------------- |
| `user-overrides`     | Highest  | You — theme authors using this layer   |
| `marketplace-themes` | High     | `events.css` (platform default styles) |
| `design-system`      | Medium   | Platform design tokens                 |
| `base`               | Lowest   | Browser normalization, CSS reset       |

**Action required:** Replace `!important` declarations in your themes with `@layer user-overrides { ... }` wrapping.

**Before (v1.2 and earlier):**

```css
.event-message {
  border-radius: 0 !important;
}
```

**After (v1.3):**

```css
@layer user-overrides {
  .event-message {
    border-radius: 0;
  }
}
```

---

## Frozen Class Names (No Changes Required)

All class names in `EVENTS_CSS_API.md` are **unchanged** in v1.3. Your existing selectors continue to work without modification:

| Class / Selector                          | Status             |
| ----------------------------------------- | ------------------ |
| `.event-message`                          | Frozen — unchanged |
| `.event-content`                          | Frozen — unchanged |
| `.event-icon`                             | Frozen — unchanged |
| `.event-title`                            | Frozen — unchanged |
| `.event-value`                            | Frozen — unchanged |
| `.event-tier-high`                        | Frozen — unchanged |
| `.event-tier-medium`                      | Frozen — unchanged |
| `.event-tier-low`                         | Frozen — unchanged |
| `.event-type-subscription`                | Frozen — unchanged |
| `.event-type-gift_subscription`           | Frozen — unchanged |
| `.event-type-super_chat`                  | Frozen — unchanged |
| `.event-type-super_sticker`               | Frozen — unchanged |
| `.event-type-raid`                        | Frozen — unchanged |
| `.event-type-bits`                        | Frozen — unchanged |
| `.event-type-channel_points`              | Frozen — unchanged |
| `.event-type-gift`                        | Frozen — unchanged |
| `.event-type-mystery_gift`                | Frozen — unchanged |
| `.event-type-follow`                      | Frozen — unchanged |
| `.event-type-like_aggregate`              | Frozen — unchanged |
| `.event-type-member`                      | Frozen — unchanged |
| `.event-type-token_expiration_warning`    | Frozen — unchanged |
| `.event-message[data-platform="twitch"]`  | Frozen — unchanged |
| `.event-message[data-platform="youtube"]` | Frozen — unchanged |
| `.event-message[data-platform="kick"]`    | Frozen — unchanged |
| `.event-message[data-platform="tiktok"]`  | Frozen — unchanged |
| `.event-message[data-platform="system"]`  | Frozen — unchanged |

---

## Theme Template for v1.3

Minimal v1.3-compatible theme using the cascade layer architecture:

```css
/* my-overlay-theme.css — v1.3 compatible */

/* Import this AFTER the platform's default CSS */
@layer user-overrides {
  /* Override event message appearance */
  .event-message {
    border-radius: 8px;
    border-width: 2px;
  }

  /* Style high-tier events (large Super Chats, raids) */
  .event-tier-high {
    border-color: gold;
    background-color: rgba(255, 215, 0, 0.1);
  }

  /* Platform-specific styling */
  .event-message[data-platform='twitch'] {
    border-left-color: #9146ff;
  }

  .event-message[data-platform='youtube'] {
    border-left-color: #ff0000;
  }
}
```

---

## Migration Checklist

- [ ] Replace `!important` rules with `@layer user-overrides { ... }` wrapper
- [ ] Verify your theme loads AFTER the platform CSS (the overlay page handles this automatically)
- [ ] Test all event types: subscription, raid, Super Chat, bits, follow
- [ ] Test all platforms: Twitch, YouTube, Kick, TikTok
- [ ] No action needed for frozen class names — they are unchanged

---

## Questions and Support

- **Class name freeze policy:** See [EVENTS_CSS_API.md](./EVENTS_CSS_API.md) — Change Policy section
- **Report a breaking change:** Open a GitHub issue with the `breaking-css-api` label
- **CSS cascade layers reference:** [MDN @layer](https://developer.mozilla.org/en-US/docs/Web/CSS/@layer)
