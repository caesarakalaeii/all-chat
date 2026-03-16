# Requirements: All-Chat

**Defined:** 2026-03-15
**Core Value:** Streamers can aggregate chat from all platforms they stream to, with reliable message delivery even during high-traffic events through intelligent load balancing, auto-scaling, and unlimited YouTube chat access.

## v1.5 Requirements — Discord Listener

Requirements for the Discord Listener milestone. Each maps to roadmap phases.

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

- [ ] **RELY-01**: Overlay messages from non-Discord sources are relayed to a configured Discord channel, with `platform == "discord"` messages unconditionally filtered to prevent loops
- [ ] **RELY-02**: Each Discord source has a relay_enabled toggle so inbound-only (read-only) mode is supported
- [ ] **RELY-03**: Relay target channel (outbound) is configurable per-source, can be the same or different from the inbound channel
- [ ] **RELY-04**: Relayed messages are posted as plain text `[emoji] username: text`

### LOAD — Load Balancing

- [ ] **LOAD-01**: Gateway shard ownership is coordinated via source-manager leader election — one pod holds each shard's connection
- [ ] **LOAD-02**: discord-listener scales via HPA on Prometheus metrics (events/sec, active guilds)
- [ ] **LOAD-03**: Gateway session state (session_id + resume_gateway_url) is persisted in Redis so pod restarts resume the session instead of full re-IDENTIFY

### UI — Setup UI

- [ ] **UI-01**: Settings page includes a Discord server connect card showing OAuth2 flow and connected server name/icon
- [ ] **UI-02**: Overlay editor allows adding a Discord source with guild selector and inbound channel dropdown (from channel listing API)
- [ ] **UI-03**: Per-source relay configuration panel: toggle relay, pick outbound channel, visual indicator of active filter
- [ ] **UI-04**: Discord source cards in the overlay editor display connection status and relay active/inactive indicator

## Future Requirements

### Extended Discord Features

- **INBD-05**: Support for Discord threads as sources (thread messages ingested separately)
- **INBD-06**: Discord emoji/reaction events surfaced in overlay
- **RELY-05**: Rich embed formatting for relayed messages (avatar, platform color)
- **AUTH-05**: Multi-guild support per user account (connect multiple Discord servers)

## Out of Scope

| Feature | Reason |
|---------|--------|
| Discord slash commands | Out of scope — not a chat aggregation concern |
| Voice channel transcription | High complexity, separate domain |
| Discord DMs / private channels | Privacy concerns, not a streaming use case |
| Per-user Discord identity mapping | Over-engineering — platform username sufficient |
| Discord embeds for relay | Avoid embed rate limits; plain text is sufficient and simpler |
| Reaction/emoji event relay | Not chat messages; different event type |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| AUTH-01 | Phase 27 | Complete |
| AUTH-02 | Phase 27 | Complete |
| AUTH-03 | Phase 27 | Complete |
| AUTH-04 | Phase 27 | Complete |
| INBD-01 | Phase 28 | Complete |
| INBD-02 | Phase 28 | Complete |
| INBD-03 | Phase 29 | Complete |
| INBD-04 | Phase 29 | Complete |
| RELY-01 | Phase 30 | Pending |
| RELY-02 | Phase 30 | Pending |
| RELY-03 | Phase 30 | Pending |
| RELY-04 | Phase 30 | Pending |
| LOAD-01 | Phase 31 | Pending |
| LOAD-02 | Phase 31 | Pending |
| LOAD-03 | Phase 31 | Pending |
| UI-01 | Phase 32 | Pending |
| UI-02 | Phase 32 | Pending |
| UI-03 | Phase 32 | Pending |
| UI-04 | Phase 32 | Pending |

**Coverage:**
- v1.5 requirements: 19 total
- Mapped to phases: 19
- Unmapped: 0 ✓

---
*Requirements defined: 2026-03-15*
*Last updated: 2026-03-15 after roadmap creation — traceability complete*
