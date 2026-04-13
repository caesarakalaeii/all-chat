# Phase 12: Notification sound on incoming messages - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-12
**Phase:** 12-notification-sound-on-incoming-messages-with-premium-custom-
**Areas discussed:** Audio playback strategy, Sound assets & presets, Trigger behavior, Premium custom sound gating

---

## Audio Playback Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Pooled HTMLAudioElement | Pre-create pool of <audio> elements, pick and play. Simple, well-supported. | ✓ |
| Web Audio API | More control but more complex. Overkill for simple notifications. | |
| Web Audio API oscillators | Generate sounds programmatically. No files but basic synthetic tones only. | |

**User's choice:** Pooled HTMLAudioElement (Recommended)
**Notes:** None

---

## Sound Assets & Presets

| Option | Description | Selected |
|--------|-------------|----------|
| Static files in /public | Ship MP3/OGG in /public/sounds/. Simple, cacheable, ~10-50KB. | ✓ |
| Base64 embedded in JS | Inline as data URIs. Zero requests but increases bundle. | |
| CDN-hosted | External CDN. Clean repo but adds dependency and CORS issues. | |

**User's choice:** Static files in /public (Recommended)
**Notes:** None

---

## Trigger Behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Cooldown timer | Wait N ms between sounds (default 500ms). Configurable slider. | ✓ |
| Play every message | No throttling. Works for low-traffic but overwhelming in busy chats. | |
| Smart debounce | Batch rapid messages, play once per burst. Sophisticated but complex. | |

**User's choice:** Cooldown timer (Recommended)
**Notes:** None

---

## Premium Custom Sound Gating

| Option | Description | Selected |
|--------|-------------|----------|
| Frontend-only gate | Disabled input + PremiumBadge upsell. Matches viewer cosmetics pattern. | ✓ |
| Frontend + backend validation | Frontend gate plus backend rejects non-premium custom URLs. | |
| Feature gate controlled | Use admin feature gate infrastructure. Most flexible but overhead. | |

**User's choice:** Frontend-only gate (Recommended)
**Notes:** None

---

## Claude's Discretion

- Audio file formats
- Audio pool size
- Sound preview button in editor
- Error handling for failed custom URLs
- Cooldown slider label format

## Deferred Ideas

None — discussion stayed within phase scope
