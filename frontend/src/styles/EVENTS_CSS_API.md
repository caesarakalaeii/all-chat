# events.css — Overlay Marketplace Public API

**Status:** STABLE — Class names in this file are frozen. Never rename or remove without a deprecation notice.

---

## Purpose

Marketplace theme authors write CSS targeting the class names defined in `events.css` to customize overlay appearance for their users. This file is the public API for the overlay marketplace — it defines the visual contract between the platform and the community.

Because real-world overlays depend on these class names, breaking changes require a formal deprecation period and a migration guide. Renaming or removing a class without notice will silently break every theme that targets it.

---

## Frozen Class Names

The following class names are part of the public API. They must not be renamed or removed without following the change policy below.

### Base Event Classes

These classes are applied to every event message element:

| Class            | Applied to      | Description                                                 |
| ---------------- | --------------- | ----------------------------------------------------------- |
| `.event-message` | Root element    | Every platform event (subscription, raid, Super Chat, etc.) |
| `.event-content` | Content wrapper | Inner content area of the event                             |
| `.event-icon`    | Icon element    | Event type icon (e.g., heart, star, sword)                  |
| `.event-title`   | Title element   | Bold uppercase event label (e.g., "NEW SUBSCRIBER")         |
| `.event-value`   | Value element   | Monetary or numeric value (e.g., "$50.00", "x1000 bits")    |

### Tier Classes

Applied alongside `.event-message` to indicate visual importance:

| Class                | Tier   | Usage                                            |
| -------------------- | ------ | ------------------------------------------------ |
| `.event-tier-high`   | High   | Large Super Chats, raids, gift bombs — gold glow |
| `.event-tier-medium` | Medium | Subscriptions, medium Super Chats — purple glow  |
| `.event-tier-low`    | Low    | Follows, small events — blue glow                |

### Event Type Classes

Applied alongside `.event-message` to identify the specific event type:

| Class                                  | Event                                               |
| -------------------------------------- | --------------------------------------------------- |
| `.event-type-subscription`             | Twitch subscription                                 |
| `.event-type-gift_subscription`        | Gifted Twitch subscription                          |
| `.event-type-super_chat`               | YouTube Super Chat                                  |
| `.event-type-super_sticker`            | YouTube Super Sticker                               |
| `.event-type-raid`                     | Twitch raid                                         |
| `.event-type-bits`                     | Twitch bits (cheer)                                 |
| `.event-type-channel_points`           | Twitch channel point redemption                     |
| `.event-type-gift`                     | Generic gift event                                  |
| `.event-type-mystery_gift`             | Gift subscription bomb (multiple gifts at once)     |
| `.event-type-follow`                   | New follow                                          |
| `.event-type-like_aggregate`           | TikTok like count milestone                         |
| `.event-type-member`                   | YouTube membership                                  |
| `.event-type-watch_streak`             | Twitch watch streak (carries the viewer's message)  |
| `.event-type-announcement`             | Twitch `/announce` (carries the announcement body)  |
| `.event-type-unraid`                   | Twitch raid cancelled                               |
| `.event-type-modiversary`              | Twitch moderator anniversary                        |
| `.event-type-charity_donation`         | Twitch charity donation                             |
| `.event-type-gift_paid_upgrade`        | Gifted sub continued as paid                        |
| `.event-type-prime_paid_upgrade`       | Prime sub continued as paid                         |
| `.event-type-pay_it_forward`           | Gift recipient gifting onward                       |
| `.event-type-twitch_notice`            | Twitch chat notice with no first-class mapping yet   |
| `.event-type-token_expiration_warning` | Platform OAuth token about to expire (system event) |

### Attribute Selectors

These `data-platform` attribute selectors are also part of the frozen public API:

| Selector                                  | Platform               |
| ----------------------------------------- | ---------------------- |
| `.event-message[data-platform="twitch"]`  | Twitch events          |
| `.event-message[data-platform="youtube"]` | YouTube events         |
| `.event-message[data-platform="kick"]`    | Kick events            |
| `.event-message[data-platform="tiktok"]`  | TikTok events          |
| `.event-message[data-platform="system"]`  | Internal system events |

---

## Cascade Layer Architecture

`events.css` rules live inside `@layer marketplace-themes`. The full cascade layer order is:

```
@layer base, design-system, marketplace-themes, user-overrides;
```

Higher layers in this list win over lower layers at equal specificity:

| Layer                | Priority | Who writes it                    |
| -------------------- | -------- | -------------------------------- |
| `user-overrides`     | Highest  | Theme authors                    |
| `marketplace-themes` | High     | This file (events.css)           |
| `design-system`      | Medium   | Design token system              |
| `base`               | Lowest   | Browser normalization, CSS reset |

This means `events.css` rules already win over `design-system` rules without needing `!important`. Theme authors writing overrides should place their CSS in `@layer user-overrides` for the highest cascade priority — no `!important` needed.

---

## Usage Example

Minimal theme override targeting the frozen public API:

```css
/* In your custom overlay CSS — use @layer user-overrides for highest priority */
@layer user-overrides {
  .event-message {
    border-radius: 0; /* override the default 16px */
  }

  .event-tier-high {
    border-color: hotpink;
  }

  .event-message[data-platform='twitch'] {
    border-left-color: #9146ff;
  }
}
```

Do not use `!important` in custom theme CSS — the cascade layer order handles specificity.

---

## Change Policy

Any breaking change to the frozen class names (rename, removal, semantic meaning change) requires:

1. **GitHub issue** with the `breaking-css-api` label documenting the reason for the change and the migration path.
2. **Minimum 30-day deprecation period** where both the old class name and the new class name are supported simultaneously.
3. **Entry in the Phase 26 Marketplace Migration Guide** so theme authors are notified and given migration steps before the old class name is removed.

Non-breaking additions (new class names, new `data-platform` values) do not require a deprecation period.
