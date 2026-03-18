# Requirements: All-Chat

**Defined:** 2026-03-18
**Core Value:** Streamers can aggregate chat from all platforms they stream to, with reliable message delivery even during high-traffic events through intelligent load balancing, auto-scaling, and unlimited YouTube chat access.

## v1.6 Requirements — Visual Overlay Customizer

Requirements for the Visual Overlay Customizer milestone. Each maps to roadmap phases.

### APPR — Appearance Controls

- [ ] **APPR-01**: User can customize typography: font family (picker), weight, line height, letter spacing
- [ ] **APPR-02**: User can customize text colors: message body, username, timestamp
- [ ] **APPR-03**: User can customize overlay background: color + opacity slider
- [ ] **APPR-04**: User can customize message bubble: background color + opacity, border radius, border width/color, inner padding, gap between messages
- [ ] **APPR-05**: User can toggle component visibility individually: avatars, badges, timestamps, platform badge, emotes, username
- [ ] **APPR-06**: User can adjust component sizing: avatar size, badge size, emote scale
- [ ] **APPR-07**: User can override per-platform accent colors (Twitch, YouTube, Kick, TikTok, Discord)
- [ ] **APPR-08**: User can configure backdrop blur (glassmorphism) intensity
- [ ] **APPR-09**: User can customize special event styling: show/hide, size modifier for Super Chat, subscriptions, raids
- [ ] **APPR-10**: All visual control changes update the live overlay preview in real-time without requiring save

### VISM — Visual Customizer Mechanism

- [x] **VISM-01**: Visual customizations are stored as structured JSON in overlay config and persist across sessions
- [ ] **VISM-02**: Loading a marketplace theme pre-populates visual controls with that theme's CSS variable values
- [x] **VISM-03**: Visual customizations generate CSS overrides at a layer above the marketplace theme and below raw user CSS
- [ ] **VISM-04**: Resetting visual customizations restores theme defaults (or system defaults if no theme loaded)

### EDUX — Editor UX Rework

- [ ] **EDUX-01**: Theme Marketplace is the first visible element in the editor panel
- [ ] **EDUX-02**: Editor panel uses collapsible sections: Theme, Appearance (with sub-groups), Sources, Behavior, Expert
- [ ] **EDUX-03**: CSS editor is hidden by default inside the collapsible "Expert" section
- [ ] **EDUX-04**: Visual customizer sub-groups: Typography, Colors, Background & Bubbles, Visibility, Sizing, Platform Colors, Events

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| VISM-01 | Phase 33 | Complete |
| VISM-03 | Phase 33 | Complete |
| APPR-01 | Phase 34 | Not started |
| APPR-02 | Phase 34 | Not started |
| APPR-03 | Phase 34 | Not started |
| APPR-04 | Phase 34 | Not started |
| APPR-08 | Phase 34 | Not started |
| APPR-05 | Phase 35 | Not started |
| APPR-06 | Phase 35 | Not started |
| APPR-07 | Phase 35 | Not started |
| APPR-09 | Phase 36 | Not started |
| APPR-10 | Phase 36 | Not started |
| VISM-02 | Phase 36 | Not started |
| VISM-04 | Phase 36 | Not started |
| EDUX-01 | Phase 37 | Not started |
| EDUX-02 | Phase 37 | Not started |
| EDUX-03 | Phase 37 | Not started |
| EDUX-04 | Phase 37 | Not started |

**Coverage:**
- v1.6 requirements: 18 total
- Mapped to phases: 18
- Unmapped: 0 ✓

---

## v1.5 Requirements — Discord Listener (Complete)

**Archive:** [v1.5-REQUIREMENTS.md](milestones/v1.5-REQUIREMENTS.md)

### AUTH — Bot Setup

- [x] **AUTH-01**: User can connect a Discord server to All-Chat via OAuth2 "Add to Server" flow
- [x] **AUTH-02**: After connecting, user can view a list of readable text channels in the connected server
- [x] **AUTH-03**: Bot permissions are validated on connect (VIEW_CHANNEL, READ_MESSAGE_HISTORY, SEND_MESSAGES) with user-visible errors on failure
- [x] **AUTH-04**: User can disconnect the bot from their server, removing all associated Discord sources

### INBD — Inbound (Discord → Overlay)

- [x] **INBD-01**: Discord channel messages appear in overlays as a first-class chat source
- [x] **INBD-02**: Discord messages are normalized to the unified RawChatMessage schema via a discord normalizer in message-processor
- [x] **INBD-03**: Discord message deletions are propagated through the existing deletion pipeline
- [x] **INBD-04**: Discord @user and #channel mentions are resolved to human-readable names in message text

### RELY — Outbound Relay (Overlay → Discord)

- [x] **RELY-01**: Overlay messages from non-Discord sources are relayed to a configured Discord channel, with `platform == "discord"` messages unconditionally filtered to prevent loops
- [x] **RELY-02**: Each Discord source has a relay_enabled toggle so inbound-only (read-only) mode is supported
- [x] **RELY-03**: Relay target channel (outbound) is configurable per-source, can be the same or different from the inbound channel
- [x] **RELY-04**: Relayed messages are posted as plain text `[emoji] username: text`

### LOAD — Load Balancing

- [x] **LOAD-01**: Gateway shard ownership is coordinated via source-manager leader election — one pod holds each shard's connection
- [x] **LOAD-02**: discord-listener scales via HPA on Prometheus metrics (events/sec, active guilds)
- [x] **LOAD-03**: Gateway session state (session_id + resume_gateway_url) is persisted in Redis so pod restarts resume the session instead of full re-IDENTIFY

### UI — Setup UI

- [x] **UI-01**: Settings page includes a Discord server connect card showing OAuth2 flow and connected server name/icon
- [x] **UI-02**: Overlay editor allows adding a Discord source with guild selector and inbound channel dropdown (from channel listing API)
- [x] **UI-03**: Per-source relay configuration panel: toggle relay, pick outbound channel, visual indicator of active filter
- [x] **UI-04**: Discord source cards in the overlay editor display connection status and relay active/inactive indicator

---
*Requirements defined: 2026-03-18*
*Last updated: 2026-03-18 after v1.6 milestone start — traceability complete*
