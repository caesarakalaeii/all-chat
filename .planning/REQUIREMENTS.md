# Requirements: All-Chat v1.3 Chat Overlay Sharing

**Defined:** 2026-03-08
**Core Value:** Streamers can aggregate chat from all platforms they stream to, with reliable message delivery even during high-traffic events through intelligent load balancing, auto-scaling, and unlimited YouTube chat access.

## v1.3 Requirements

Requirements for chat overlay sharing feature. Each maps to roadmap phases.

### Share Management

- [x] **SHARE-01**: User can search for other users by platform username (Twitch, YouTube, Kick, TikTok)
- [x] **SHARE-02**: User can send share request selecting an overlay to share
- [x] **SHARE-03**: User can view pending incoming share requests in dashboard
- [x] **SHARE-04**: User can accept share request, choosing overlay to share back and expiry option
- [ ] **SHARE-05**: On acceptance, both users can optionally add shared source to an overlay immediately
- [ ] **SHARE-06**: Either user can revoke share at any time
- [ ] **SHARE-07**: Revoked or expired shares are marked as inactive (not deleted from config)
- [x] **SHARE-08**: Share status indicators show active, expired, or revoked state

### Shared Overlay Sources

- [x] **SOURCE-01**: New "Shared Overlays" source type available alongside platform sources
- [x] **SOURCE-02**: User can browse list of available shared overlays when adding source
- [x] **SOURCE-03**: User can add shared overlay as source to any overlay
- [x] **SOURCE-04**: Messages from source overlay's aggregated chat delivered to recipient's overlay
- [ ] **SOURCE-05**: Display settings (CSS, events) from recipient's overlay apply, not source overlay's

### Expiry & Lifecycle

- [ ] **EXPIRY-01**: User can choose expiry option when accepting: "This stream", "n hours", "Unlimited"
- [ ] **EXPIRY-02**: Stream lifecycle detected for Twitch via Helix API
- [ ] **EXPIRY-03**: Share auto-expires when either user's stream ends (if "This stream" selected)
- [ ] **EXPIRY-04**: Time-based expiry checked via background job every 5 minutes
- [ ] **EXPIRY-05**: YouTube and TikTok lifecycle already tracked (reuse existing detection)
- [ ] **EXPIRY-06**: Kick stream lifecycle detection researched (defer implementation if complex)

### Premium & Admin

- [x] **PREMIUM-01**: Non-premium users blocked from creating or accepting shares
- [x] **PREMIUM-02**: Admin can mark specific users as premium for testing purposes

## Future Requirements

Deferred to v1.4 or later.

### Notifications & Lifecycle
- Share request expiration (7-day timeout for pending requests)
- Email/push notifications for share events
- Share renewal workflow (easier to extend expired share)

### Analytics & Insights
- Share usage metrics and analytics
- Popular overlay discovery

## Out of Scope

Explicitly excluded features with reasoning.

| Feature | Reason |
|---------|--------|
| Public overlay directory | Moderation burden, copyright risk, DMCA exposure |
| Automatic share acceptance | Violates consent model, security risk |
| Share settings inheritance | Breaks user control, creates CSS conflicts |
| Unlimited free sharing | Eliminates premium monetization value |
| Cross-platform relay (A→B→C) | Permission complexity, amplifies load issues |
| GraphQL for user search | Over-engineered for simple username lookup |
| Separate expiry microservice | Unnecessary complexity, collocate in share-service |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| SHARE-01 | Phase 14 | Complete |
| SHARE-02 | Phase 14 | Complete |
| SHARE-03 | Phase 14 | Complete |
| SHARE-04 | Phase 15 | Complete |
| SHARE-05 | Phase 15 | Pending |
| SHARE-06 | Phase 18 | Pending |
| SHARE-07 | Phase 18 | Pending |
| SHARE-08 | Phase 15 | Complete |
| SOURCE-01 | Phase 16 | Complete |
| SOURCE-02 | Phase 16 | Complete |
| SOURCE-03 | Phase 16 | Complete |
| SOURCE-04 | Phase 17 | Complete |
| SOURCE-05 | Phase 17 | Pending |
| EXPIRY-01 | Phase 19 | Pending |
| EXPIRY-02 | Phase 19 | Pending |
| EXPIRY-03 | Phase 19 | Pending |
| EXPIRY-04 | Phase 19 | Pending |
| EXPIRY-05 | Phase 19 | Pending |
| EXPIRY-06 | Phase 19 | Pending |
| PREMIUM-01 | Phase 14 | Complete |
| PREMIUM-02 | Phase 14 | Complete |

**Coverage:**
- v1.3 requirements: 21 total
- Mapped to phases: 21/21 (100%)
- Unmapped: None

**Phase Distribution:**
- Phase 14 (Foundation): 5 requirements
- Phase 15 (Share Acceptance): 3 requirements
- Phase 16 (Shared Overlay Sources): 3 requirements
- Phase 17 (Message Routing): 2 requirements
- Phase 18 (Revocation): 2 requirements
- Phase 19 (Lifecycle & Expiry): 6 requirements

---
*Requirements defined: 2026-03-08*
*Last updated: 2026-03-08 after roadmap creation*
