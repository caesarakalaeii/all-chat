# Phase 12: Notification sound on incoming messages with premium custom sound support - Context

**Gathered:** 2026-04-12
**Status:** Ready for planning

<domain>
## Phase Boundary

Add configurable notification sounds that play when new chat messages arrive in the overlay. All users get a selection of preset sounds with volume control. Premium users can additionally supply a custom sound URL. Settings persist via the existing overlay config API. Sound playback happens client-side in the overlay page.

</domain>

<decisions>
## Implementation Decisions

### Audio Playback Strategy
- **D-01:** Pooled HTMLAudioElement approach — pre-create a small pool of `<audio>` elements, pick one and play on each trigger. Simple, well-supported, handles overlapping sounds naturally.
- **D-02:** OBS browser source allows autoplay without user gesture. For standalone browsers, a single user click unlocks audio. No complex AudioContext setup needed.

### Sound Assets & Presets
- **D-03:** Ship 3-5 preset sound files as static assets in `/public/sounds/` (MP3 or OGG). Small footprint (~10-50KB total). Overlay page loads them by URL path.
- **D-04:** Preset names: at minimum "chime", "pop", "ping". Claude's discretion on the exact number and naming of additional presets.

### Trigger Behavior
- **D-05:** Cooldown timer — after playing a sound, enforce a minimum delay before the next trigger. Default ~500ms. Prevents audio spam in high-traffic chats.
- **D-06:** Cooldown duration is configurable via a slider in the Sound settings UI.

### Premium Custom Sound Gating
- **D-07:** Frontend-only gate — custom sound URL input is disabled with a PremiumBadge upsell prompt when `!user.is_premium`. Matches the existing viewer cosmetics pattern. No backend validation needed — overlay page is public, only the owner can configure settings.
- **D-08:** When premium user provides a custom sound URL, it's used instead of the selected preset. Falls back to preset if custom URL fails to load.

### DisplaySettings Extension
- **D-09:** Add to `DisplaySettings` in `frontend/src/lib/types/overlay.ts`:
  - `notification_sound_enabled?: boolean`
  - `notification_sound_preset?: string` (e.g., "chime", "pop", "ping")
  - `notification_sound_url?: string` (premium: custom sound URL)
  - `notification_sound_volume?: number` (0–1)
  - `notification_sound_cooldown?: number` (milliseconds, default 500)

### Claude's Discretion
- Exact audio file formats (MP3 vs OGG vs both with fallback)
- Audio pool size (2-4 elements typically sufficient)
- Sound preview/test button in the editor UI
- Error handling for failed custom URL loads (silent fallback vs toast)
- Whether cooldown slider shows milliseconds or a friendlier label

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Types and Models
- `frontend/src/lib/types/overlay.ts` — DisplaySettings interface to extend (lines 36-52), OverlayConfig
- `services/overlay-manager/models/config.go` — Backend config model, DisplaySettings as map[string]any

### UI Components
- `frontend/src/components/appearance/AppearancePanel.tsx` — Parent panel where Sound section will be added
- `frontend/src/components/appearance/CollapsibleSection.tsx` — Collapsible section wrapper
- `frontend/src/components/appearance/ToggleSwitch.tsx` — For notification_sound_enabled toggle
- `frontend/src/components/appearance/SliderControl.tsx` — For volume and cooldown sliders
- `frontend/src/components/PremiumBadge.tsx` — Premium upsell badge component

### Premium Gating Pattern
- `frontend/src/app/settings/viewer/page.tsx` — Existing is_premium gating pattern (lines 227, 285-295, 539)
- `frontend/src/app/overlays/[id]/page.tsx` — Editor page with user?.is_premium access (line 1517)

### Overlay Pages (playback integration)
- `frontend/src/app/overlay/[id]/page.tsx` — Live overlay page with WebSocket message handler
- `frontend/src/app/overlays/[id]/page.tsx` — Editor page for Sound settings UI
- `frontend/src/app/overlays/[id]/preview/embed/page.tsx` — Embed preview page

### API Layer
- `frontend/src/lib/api/overlays.ts` — updateConfig() for persisting settings
- `services/overlay-manager/handlers/config.go` — Config handler (display_settings already handled)

### Phase 11 Pattern (follow this)
- `frontend/src/components/appearance/FilterGroup.tsx` — Recently added AppearancePanel group, follows the exact pattern this phase should use
- `frontend/src/lib/utils/filterMessage.ts` — Pure utility pattern to follow for sound playback utility

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `CollapsibleSection`: Wrap new Sound section, same as all other AppearancePanel groups
- `ToggleSwitch`: For notification_sound_enabled
- `SliderControl`: For volume and cooldown
- `PremiumBadge`: For custom URL upsell
- `updateConfig()` API: Already handles display_settings persistence — new fields auto-merge
- `FilterGroup.tsx` (Phase 11): Reference implementation for adding a new AppearancePanel group

### Established Patterns
- DisplaySettings stored as JSONB `map[string]any` on backend — new fields require zero migration
- Premium gating is client-side only (disabled UI + PremiumBadge, no backend enforcement)
- Phase 11's postMessage pattern for WYSIWYG preview can be reused for sound preview

### Integration Points
- **AppearancePanel.tsx**: Add new SoundGroup collapsible section
- **Overlay page** (`overlay/[id]/page.tsx`): Play sounds in the WebSocket `onmessage` handler after filtering
- **Editor page** (`overlays/[id]/page.tsx`): Sound settings UI with premium gating
- **`/public/sounds/`**: New directory for static audio assets

</code_context>

<specifics>
## Specific Ideas

- Sound playback should integrate after the filter check — filtered messages should NOT trigger sounds
- The audio pool approach means sounds can overlap slightly (pool size 2-4), creating a natural feel
- Custom URL fallback to preset is important — broken custom URL should not mean silence
- The cooldown timer prevents a "wall of sound" during raids/hype trains

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 12-notification-sound-on-incoming-messages-with-premium-custom-*
*Context gathered: 2026-04-12*
