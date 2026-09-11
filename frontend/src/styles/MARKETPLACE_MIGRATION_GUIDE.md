# Overlay Marketplace Migration Guide — v1.3

**Companion to:** [EVENTS_CSS_API.md](./EVENTS_CSS_API.md)
**Updated:** 2026-03-14

This guide is for overlay theme authors who publish CSS themes to the All-Chat marketplace. It documents what changed in v1.3 (Frontend Redesign) and how to update your themes.

---

## What Changed in v1.3

### CSS Cascade Layer Architecture (New)

The most important change in v1.3 is the introduction of CSS cascade layers. This **eliminates the need for `!important` in your theme CSS**.

The overlay's layer order is:

```
@layer base, design-system, marketplace-themes, user-overrides, visual-customizer;
```

Theme CSS is injected into `@layer marketplace-themes`. The precedence
hierarchy, strongest first:

| Rank | Origin                                  | How it is expressed                    |
| ---- | --------------------------------------- | -------------------------------------- |
| 1    | Manual custom CSS (the user's editor)   | **Unlayered** — beats every layer      |
| 2    | GUI visual settings (appearance panels) | `@layer visual-customizer` (top layer) |
| 3    | Your theme                              | `@layer marketplace-themes`            |
| 4    | App defaults, design system, Tailwind   | their own layers                       |

**Action required:** write plain, unlayered CSS. `!important` is stripped from
bundled-theme CSS at injection, and `@layer user-overrides` ranks BELOW the
GUI layer — neither mechanism helps you.

**Before (v1.2 and earlier):**

```css
.event-message {
  border-radius: 0 !important;
}
```

**Now:**

```css
.event-message {
  border-radius: 0;
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

/* Plain unlayered rules — the overlay injects your theme into
   @layer marketplace-themes, below the user's GUI settings and manual CSS */
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
```

---

## What Changed in v1.4 — Event Tokens

Event chrome is now expressed as `--event-*` custom properties. **No class name
changed and no default look changed**, so existing themes keep working; the
tokens are an easier way in.

| Change                                       | Impact on your theme                                                                                                                                                                       |
| -------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `--event-*` tokens added                     | Optional. Set tokens instead of re-declaring rules.                                                                                                                                        |
| `.event-user` element removed                | Rules targeting it are now no-ops. The chatter's name is the row header's `.chat-username` on events, same as on chat.                                                                     |
| Events excluded from the forced bubble rules | Your `.event-message` border/padding/radius rules now actually apply. Previously an `!important` rule in `@layer visual-customizer` silently overrode them.                                |
| Tier borders now render                      | Cosmetic: the `.event-tier-*` border you were promised was being erased by the rule above.                                                                                                 |
| Event size/colour/indent left Tailwind       | `text-4xl`, `text-yellow-300`, `text-slate-200`, `ml-14` are gone from the markup; the equivalents are `--event-icon-size`, `--event-value-color`, `--event-text-color`, `--event-indent`. |

Full token reference: **[docs/overlay-themes/AUTHORING-EVENTS.md](../../../docs/overlay-themes/AUTHORING-EVENTS.md)**.

---

## Migration Checklist

- [ ] Replace `!important` rules with plain, unlayered declarations
- [ ] Verify your theme loads AFTER the platform CSS (the overlay page handles this automatically)
- [ ] Test all event types: subscription, raid, Super Chat, bits, follow
- [ ] Test all platforms: Twitch, YouTube, Kick, TikTok
- [ ] No action needed for frozen class names — they are unchanged

---

## Questions and Support

- **Class name freeze policy:** See [EVENTS_CSS_API.md](./EVENTS_CSS_API.md) — Change Policy section
- **Report a breaking change:** Open a GitHub issue with the `breaking-css-api` label
- **CSS cascade layers reference:** [MDN @layer](https://developer.mozilla.org/en-US/docs/Web/CSS/@layer)
