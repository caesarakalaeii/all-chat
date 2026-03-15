# Requirements: All-Chat v1.4

**Defined:** 2026-03-14
**Milestone:** v1.4 Viewer Identity & YouTube Enrichment
**Core Value:** Viewers can personalise how their name appears across all overlays; YouTube chat is enriched with real membership badges and inline emotes from InnerTube.

---

## v1.4 Requirements

### Viewer Identity (all users)

- [ ] **VID-01**: Viewer can set a fallback name color (hex) as a global preference when their platform provides no color
- [ ] **VID-02**: Viewer's fallback color is applied in all overlays where the platform sends no `color` field (YouTube, Kick, TikTok default)
- [x] **VID-03**: Viewer's color preference persists server-side and survives extension reinstall
- [x] **VID-04**: Viewer can link one or more platform identities (Twitch, YouTube, Kick) to their All-Chat account to enable cross-platform cosmetics
- [x] **VID-05**: Viewer can authenticate from the browser extension popup (sign in with Twitch or YouTube)
- [x] **VID-06**: Extension popup shows current auth status and signed-in display name / avatar

### Premium Cosmetics

- [ ] **PREM-01**: Premium viewer can set a multi-stop gradient (2–4 colors, angle) as their name color
- [ ] **PREM-02**: Gradient name renders in overlay using CSS `background-clip: text` — no JavaScript required
- [ ] **PREM-03**: Premium viewer can select an avatar frame (decorative PNG ring overlaid on their avatar)
- [ ] **PREM-04**: Premium viewer can select an avatar flair (small corner icon pinned to bottom-right of avatar)
- [ ] **PREM-05**: Frame and flair catalog is managed by admins (add/remove items, mark as premium-only)

### Platform Badges (All-Chat)

- [ ] **BADGE-01**: Admin users automatically receive an All-Chat logo badge shown in overlays
- [ ] **BADGE-02**: Premium users automatically receive a gem/star icon badge shown in overlays
- [ ] **BADGE-03**: All-Chat badges are prepended to the badge list (rendered before platform badges)
- [ ] **BADGE-04**: Badge icon images are served from CDN and specified per badge type in a badge definitions catalog

### YouTube Badges via InnerTube

- [x] **YTBADGE-01**: YouTube membership badge renders with the real channel-specific image from InnerTube (`customThumbnail.thumbnails[1].URL` at 32px)
- [x] **YTBADGE-02**: YouTube membership badge tooltip carries the tier name (e.g. "3-Month Member") from InnerTube `tooltip` field
- [x] **YTBADGE-03**: YouTube moderator, owner, and verified badges continue to render (static SVG fallback acceptable — no image URL provided by InnerTube for system badges)
- [x] **YTBADGE-04**: Backward compatibility maintained: old youtube-listener (quota-based) still functions without real badge images

### YouTube Emotes via InnerTube

- [x] **YTEMOTE-01**: YouTube channel membership emotes (`isCustomEmoji: true`, `emojiId` starts with `UC`) render as inline images in overlays and extension
- [x] **YTEMOTE-02**: YouTube global emotes (`isCustomEmoji: true`, `emojiId` starts with `_`) render as inline images
- [x] **YTEMOTE-03**: Standard Unicode emoji in YouTube messages continue to render as text characters (no regression)
- [x] **YTEMOTE-04**: Emote images served at 48px (the larger InnerTube thumbnail) for retina clarity
- [x] **YTEMOTE-05**: Emotes accumulate in a per-channel Redis cache keyed by `emojiId` (since no catalog endpoint exists)

### Extension UX

- [x] **EXT-01**: Extension popup shows an inline name color picker (`<input type="color">`) with reset-to-default option
- [x] **EXT-02**: Color change saves immediately to server and local storage (no explicit Save button)
- [x] **EXT-03**: Extension popup contains an "Open Settings" button that navigates to `/settings/viewer` on the website
- [x] **EXT-04**: Extension content scripts apply viewer's `name_color` or `name_gradient` to their own messages rendered in the overlay

### Website Cosmetics Editor

- [ ] **WEB-01**: Settings page has a "Viewer Identity" section for all authenticated users (color picker, platform linking)
- [ ] **WEB-02**: Premium users see a "Premium Cosmetics" section with gradient editor (multi-stop, angle control)
- [ ] **WEB-03**: Premium users can browse and select avatar frame from the frame catalog
- [ ] **WEB-04**: Premium users can browse and select avatar flair from the flair catalog
- [ ] **WEB-05**: Live preview of name color, gradient, avatar frame, and flair displayed on the settings page

---

## v2 Requirements (Deferred)

### Animated Gradients

- **PREM-ADV-01**: Optional shimmer animation on gradient name (CSS `background-position` keyframe)
- **PREM-ADV-02**: Viewer can control animation speed (slow/medium/fast)

### Streamer-Granted Cosmetics

- **STR-01**: Streamer can assign VIP flair to specific viewers from their dashboard
- **STR-02**: Streamer can assign a custom per-channel badge image to specific viewers

### Additional Platform Identity Linking

- **VID-TK-01**: Viewer can link TikTok identity (deferred — unofficial library, user ID stability unclear)
- **VID-KK-01**: Viewer can link Kick identity

---

## Out of Scope

| Feature | Reason |
|---------|--------|
| Streamer controls over viewer cosmetics (disable all cosmetics on overlay) | Adds overlay config complexity; viewers own their own appearance |
| Per-overlay cosmetic overrides | Global preference is the simpler UX; per-overlay is v2 at earliest |
| Animated avatar frames (video/GIF) | Storage and performance cost; static PNG frames sufficient for v1.4 |
| YouTube emote catalog pre-fetch endpoint | No such endpoint exists in InnerTube; accumulate from live chat only |
| Name color on other viewers' messages (streamer-side tinting) | Cosmetics apply only to the viewer's own messages |
| Custom frame upload by viewers | Moderation overhead; admin-curated catalog only for v1.4 |

---

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| VID-01 | Phase 27 | Pending |
| VID-02 | Phase 27 | Pending |
| VID-03 | Phase 27 | Complete |
| VID-04 | Phase 27 | Complete |
| VID-05 | Phase 28 | Complete |
| VID-06 | Phase 28 | Complete |
| PREM-01 | Phase 29 | Pending |
| PREM-02 | Phase 29 | Pending |
| PREM-03 | Phase 30 | Pending |
| PREM-04 | Phase 30 | Pending |
| PREM-05 | Phase 30 | Pending |
| BADGE-01 | Phase 31 | Pending |
| BADGE-02 | Phase 31 | Pending |
| BADGE-03 | Phase 31 | Pending |
| BADGE-04 | Phase 31 | Pending |
| YTBADGE-01 | Phase 27 | Complete |
| YTBADGE-02 | Phase 27 | Complete |
| YTBADGE-03 | Phase 27 | Complete |
| YTBADGE-04 | Phase 27 | Complete |
| YTEMOTE-01 | Phase 27 | Complete |
| YTEMOTE-02 | Phase 27 | Complete |
| YTEMOTE-03 | Phase 27 | Complete |
| YTEMOTE-04 | Phase 27 | Complete |
| YTEMOTE-05 | Phase 27 | Complete |
| EXT-01 | Phase 28 | Complete |
| EXT-02 | Phase 28 | Complete |
| EXT-03 | Phase 28 | Complete |
| EXT-04 | Phase 28 | Complete |
| WEB-01 | Phase 29 | Pending |
| WEB-02 | Phase 29 | Pending |
| WEB-03 | Phase 30 | Pending |
| WEB-04 | Phase 30 | Pending |
| WEB-05 | Phase 29 | Pending |

**Coverage:**
- v1.4 requirements: 33 total
- Mapped to phases: 33
- Unmapped: 0 ✓

---
*Requirements defined: 2026-03-14*
*Last updated: 2026-03-14 after initial v1.4 milestone definition*
