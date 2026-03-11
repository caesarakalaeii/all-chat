# Feature Research

**Domain:** Chat Overlay Sharing for Streaming Platforms
**Researched:** 2026-03-08
**Confidence:** MEDIUM

## Feature Landscape

### Table Stakes (Users Expect These)

Features users assume exist. Missing these = product feels incomplete.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Send share request to another user | Standard collaboration pattern in all SaaS products; users expect to initiate sharing | LOW | Search by platform username (Twitch, YouTube, etc.) — follows existing Twitch Stream Together model |
| Accept/decline share requests | Basic permission model — users must consent to collaboration | LOW | Dashboard view of pending requests with clear accept/decline actions |
| Immediate add-on-acceptance | Users expect shared resources to be usable immediately after acceptance | MEDIUM | Both parties must be able to add the shared overlay to their own overlays as sources (no additional approval step) |
| Revoke access at any time | Standard security/privacy expectation in all collaboration tools | LOW | Either party should be able to end the share at any time, following SharePoint/Slack/AWS patterns |
| Visual distinction for shared sources | Users need to differentiate between platform sources and shared overlay sources | LOW | UI indicator showing source type (platform vs shared overlay) in source list |
| Display settings isolation | Users expect their own overlay styling to apply, not the source's styling | MEDIUM | CSS, animations, display settings come from displaying overlay — only message content from source overlay |

### Differentiators (Competitive Advantage)

Features that set the product apart. Not required, but valuable.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Bidirectional sharing (mutual access) | Unlike Twitch Shared Chat (unidirectional), both users share overlays with each other | MEDIUM | Accept workflow includes "share back" selection — creates mutual collaboration, not just guest access |
| Flexible expiry options | Fine-grained control over share lifetime aligns with streaming use cases | MEDIUM | "This stream only", time-based (hours/days), unlimited — most competitors offer only session-based or unlimited |
| Stream lifecycle awareness | Automatic expiry when stream ends without manual cleanup | HIGH | Requires stream detection for all platforms (Twitch already tracked, YouTube/TikTok via InnerTube, Kick needs research) |
| Inactive marking (not deletion) | Preserves historical configuration when shares expire/revoke | LOW | Microsoft 365/SharePoint pattern — shows user what was configured, audit trail, easier to renew |
| Premium feature positioning | First premium feature for All-Chat establishes monetization foundation | LOW | Freemium model follows Discord, Slack, StreamElements pattern — collaboration as premium tier differentiator |
| Admin testing overrides | Enables dogfooding and validation before broad premium rollout | LOW | Admin flag bypasses premium check — critical for testing collaborative features internally |
| Multi-source overlay sharing | Share an overlay that aggregates multiple platform sources (Twitch + YouTube + Kick), not just single platform | LOW | Leverages existing multi-source architecture — more powerful than single-platform sharing (Twitch Shared Chat limitation) |

### Anti-Features (Commonly Requested, Often Problematic)

Features that seem good but create problems.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Public overlay directory/marketplace | "Let anyone discover and use my overlay" | Creates moderation burden, copyright issues with emotes/content, unexpected load from viral overlays, DMCA risk | Manual username search + explicit consent model (invite-based) |
| Automatic share acceptance | "Skip the accept step for trusted users" | Violates consent model, security risk (compromised accounts), no opportunity to select expiry/overlay | Always require explicit acceptance with overlay selection + expiry choice |
| Share settings inheritance | "Use the source overlay's display settings" | Breaks user control over their own stream appearance, CSS conflicts, unexpected visual changes | Always apply destination overlay's display settings (CSS, events) — only content flows |
| Unlimited free sharing | "Make sharing free for everyone" | Eliminates monetization path, reduces value perception, enables abuse/spam | Premium-gate sharing feature, admin override for testing — follows freemium SaaS pattern |
| Cross-platform relay (A shares to B, B auto-shares to C) | "Chain shares for maximum reach" | Creates permission complexity (did A consent to C?), amplifies load unpredictably, audit trail confusion | Direct sharing only — C must request from A explicitly |
| Share analytics/metrics | "Show me who's viewing my shared overlay" | Privacy concerns, creates performance tracking burden, unclear value for collaborative streaming | Focus on collaboration quality, not surveillance — trust-based model |

## Feature Dependencies

```
[Share Request/Accept Workflow]
    └──requires──> [User Search by Platform Username]
                       └──requires──> [User Platform Connection Registry]

[Bidirectional Sharing]
    └──requires──> [Share Request/Accept Workflow]
    └──requires──> [Overlay Selection on Accept]

[Stream-Based Expiry ("this stream only")]
    └──requires──> [Stream Lifecycle Detection]
                       └──requires──> [Platform Stream Status APIs]
                            ├──requires──> [Twitch Stream Status] (existing)
                            ├──requires──> [YouTube Stream Status] (InnerTube API, existing)
                            ├──requires──> [TikTok Stream Status] (InnerTube-like, existing)
                            └──requires──> [Kick Stream Status] (needs research)

[Time-Based Expiry]
    └──requires──> [Background Job Scheduler]
    └──requires──> [Expiry Timestamp Storage]

[Inactive Source Marking]
    └──requires──> [Source State Management (active/inactive)]
    └──requires──> [Revocation Event Handling]

[Premium Feature Enforcement]
    └──requires──> [User Premium Status Flag]
    └──requires──> [Admin Override Flag]

[Shared Overlay Source]
    └──requires──> [Source Registry Extension (new source type)]
    └──requires──> [Message Routing from Source Overlay's Redis Pub/Sub]
```

### Dependency Notes

- **Share Request requires User Search:** Users must be able to find each other by platform username (e.g., Twitch handle) before initiating share. Depends on existing user-to-platform connection mapping.
- **Stream-Based Expiry requires Stream Lifecycle Detection:** "This stream only" expiry option depends on platform APIs to detect when user's stream ends. Twitch, YouTube, TikTok already tracked; Kick needs investigation.
- **Bidirectional Sharing requires Overlay Selection:** On accept, user selects which overlay to share back — this creates mutual access pattern that differentiates from unidirectional guest models.
- **Inactive Marking requires Source State:** Instead of deleting expired/revoked shares, mark sources as "inactive" in database — preserves configuration history and enables audit trail (follows Microsoft 365 pattern).
- **Premium Enforcement requires Flags:** Both user-level premium status and admin-level testing override flags needed to gate feature and enable internal validation.
- **Shared Overlay Source requires Message Routing:** New source type that subscribes to another user's overlay's Redis Pub/Sub channel (overlay:{overlay_id}) and forwards messages through existing message processing pipeline.

## MVP Definition

### Launch With (v1.3)

Minimum viable product — what's needed to validate the concept.

- [x] User search by platform username — Core discovery mechanism, enables finding collaboration partners
- [x] Send share request (select overlay to share) — Initiates sharing workflow with explicit overlay selection
- [x] View pending requests dashboard — Visibility into incoming/outgoing requests with clear status
- [x] Accept request (select overlay to share back, choose expiry) — Bidirectional consent with configuration
- [x] Immediate add-on-acceptance (both users can add as source) — No additional approval, instant usability
- [x] Shared overlay source type — New source type that delivers all messages from source overlay's chat sources
- [x] Display settings isolation — Destination overlay's CSS/events apply, not source's settings
- [x] Flexible expiry (this stream, time-based, unlimited) — Covers primary use cases for temporary/permanent collaboration
- [x] Stream lifecycle detection (Twitch, YouTube, TikTok existing; Kick to research) — Enables "this stream only" expiry
- [x] Manual revocation (either party) — Basic security/privacy requirement
- [x] Inactive source marking (not deletion) — Preserves configuration history, enables audit trail
- [x] Premium enforcement (blocks non-premium users) — Establishes monetization path, first premium feature
- [x] Admin testing override — Enables internal validation before broad rollout

### Add After Validation (v1.4+)

Features to add once core is working.

- [ ] Share request expiration (e.g., 7 days) — Prevents stale pending requests, follows Slack/SailPoint pattern (currently requests persist indefinitely)
- [ ] Notification system for share events (request received, accepted, revoked, expired) — Improves awareness, reduces need to check dashboard manually
- [ ] Share renewal workflow — Easier to extend expired share than create new request (especially for recurring collaborations)
- [ ] Usage metrics for premium upsell — Track share attempts by non-premium users, inform conversion funnel optimization
- [ ] Batch expiry cleanup — Scheduled job to mark inactive sources from expired shares (currently immediate on stream end detection)

### Future Consideration (v2+)

Features to defer until product-market fit is established.

- [ ] Share templates (preset expiry + permissions) — Power user feature, defer until usage patterns emerge
- [ ] Whitelabel/branded shared overlays — Enterprise feature, defer until B2B demand validated
- [ ] Share history/audit log UI — Currently database records exist, but no UI — add if compliance/transparency becomes user request
- [ ] Multi-tier premium (different share limits) — Defer until single premium tier validated and conversion optimized
- [ ] API for programmatic sharing — Developer/automation feature, defer until core workflow validated

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| User search by platform username | HIGH | MEDIUM | P1 |
| Send/accept share requests | HIGH | MEDIUM | P1 |
| Bidirectional sharing workflow | HIGH | MEDIUM | P1 |
| Shared overlay source type | HIGH | HIGH | P1 |
| Display settings isolation | HIGH | MEDIUM | P1 |
| Flexible expiry options | HIGH | MEDIUM | P1 |
| Manual revocation | HIGH | LOW | P1 |
| Inactive source marking | MEDIUM | LOW | P1 |
| Premium enforcement + admin override | HIGH | LOW | P1 |
| Stream lifecycle detection (all platforms) | MEDIUM | MEDIUM | P1 |
| Share request expiration | MEDIUM | LOW | P2 |
| Notification system | MEDIUM | MEDIUM | P2 |
| Share renewal workflow | MEDIUM | MEDIUM | P2 |
| Usage metrics for premium upsell | LOW | LOW | P2 |
| Share templates | LOW | MEDIUM | P3 |
| Audit log UI | LOW | MEDIUM | P3 |
| API for programmatic sharing | LOW | HIGH | P3 |

**Priority key:**
- P1: Must have for launch (v1.3)
- P2: Should have, add when possible (v1.4+)
- P3: Nice to have, future consideration (v2+)

## Competitor Feature Analysis

| Feature | Twitch Shared Chat | Restream Pairs | StreamElements/Streamlabs | All-Chat Approach |
|---------|-------------------|----------------|---------------------------|-------------------|
| Discovery mechanism | In-platform (Stream Together invite) | Link-based (share URL) | N/A (no sharing) | Username search (cross-platform: Twitch, YouTube, Kick, TikTok handles) |
| Share direction | Unidirectional (chat flows one way) | Asymmetric (guests broadcast to their channels) | N/A | Bidirectional (mutual overlay sharing) |
| Expiry control | Session-based (ends with Stream Together) | Persistent (manual cleanup) | N/A | Flexible (this stream, time-based, unlimited) |
| Permission model | Stream Together host controls | Link access control (anyone with link) | N/A | Explicit accept/decline with consent |
| Access revocation | End Stream Together session | Unclear | N/A | Either party can revoke at any time |
| Expired access visibility | N/A (session-based) | N/A | N/A | Inactive marking (preserves config history) |
| Monetization | Free (Twitch Platform feature) | Freemium (free up to 20 guests) | Free/subscription (not sharing-specific) | Premium-gated (first premium feature for All-Chat) |
| Multi-platform support | Twitch-only | Multi-platform broadcast | Multi-platform overlays (no sharing) | Multi-platform overlay sharing (Twitch + YouTube + Kick + TikTok aggregated) |
| Display settings | Host's chat styling | Each streamer's own styling | Own styling | Destination overlay's styling (CSS isolation) |

**Key Differentiators:**
1. **Bidirectional vs Unidirectional:** Twitch Shared Chat is one-way (chat merges to host); All-Chat is mutual (both share their aggregated overlays)
2. **Multi-Platform Aggregation:** Competitors share single-platform chat; All-Chat shares overlays that aggregate multiple platforms (Twitch + YouTube + Kick + TikTok)
3. **Flexible Expiry:** Twitch = session-only, Restream = persistent, All-Chat = user choice (session, time, unlimited)
4. **Explicit Consent:** Link-based (Restream) risks unwanted access; username search + accept/decline = privacy-first
5. **Premium Positioning:** Establishes monetization foundation (freemium model) while competitors treat collaboration as free platform feature

## Sources

### Streaming Platform Collaboration Features
- [Livepush Multi Chat Overlay](https://livepush.io/features/multi-chat.html) — Multi-platform chat aggregation patterns
- [Streamlabs Shared Twitch Chat](https://streamlabs.com/content-hub/post/streamlabs-desktop-twitch-shared-chat) — Twitch native sharing implementation
- [Twitch Shared Chat Help](https://help.twitch.tv/s/article/shared-chat?language=en_US) — Official Twitch collaboration model
- [Twitch Shared Chat: How It Works](https://www.streamscheme.com/twitch-shared-chat/) — User-facing Shared Chat guide
- [Social Stream Ninja](https://socialstream.ninja/) — Multi-platform chat integration patterns
- [Restream Chat Guide](https://restream.io/blog/restream-chat-everything-you-need-to-know/) — Chat aggregation for collaborations

### Collaboration & Guest Access Patterns
- [Restream Pairs](https://support.restream.io/en/articles/11726283-what-is-restream-pairs) — Bidirectional guest channel sharing (up to 20 guests)
- [Restream Guest Channels](https://support.restream.io/en/articles/8540565-how-to-add-guest-channels-to-my-studio-stream) — Guest collaboration workflow
- [Restream Guest Capabilities](https://support.restream.io/en/articles/9184240-what-you-can-do-as-a-guest-in-restream-studio) — Guest permissions and features
- [StreamYard Guest Invites](https://support.streamyard.com/hc/en-us/articles/360054866191-Does-My-Link-to-Invite-A-Guest-Expire) — Invitation expiry (links last forever, no session reuse)
- [Twitch Raids Guide](https://streamlabs.com/content-hub/post/twitch-raids-what-they-are-and-how-to-raid) — Viewer sharing patterns
- [Collaborative Multistreaming Software](https://streamyard.com/blog/collaborative-multistreaming-software) — Team collaboration features

### Freemium & Premium Feature Patterns
- [Freemium Paywalls | RevenueCat](https://www.revenuecat.com/docs/playbooks/guides/freemium) — Freemium paywall implementation patterns
- [Freemium vs Premium | Refact](https://refact.co/freemium-vs-premium-comparing-two-paywall-models/) — Comparing paywall models
- [Freemium Business Model | Recurly](https://recurly.com/blog/what-is-freemium-a-guide-for-subscription-businesses/) — Freemium strategy guide
- [StreamElements Setup 2026](https://eathealthy365.com/your-complete-streamelements-setup-walkthrough-for-2026/) — Streaming platform premium tiers
- [Streamlabs vs StreamElements 2026](https://www.streamscheme.com/streamlabs-vs-streamelements/) — Feature comparison (premium vs free)

### SaaS Invitation & Access Management Patterns
- [Slack Pending Invitations](https://slack.com/help/articles/360022158293-Pending-member-invitations) — Invitation expiry (30 days)
- [AWS Resource Share Invitations](https://docs.aws.amazon.com/ram/latest/userguide/working-with-shared-invitations.html) — Accept/reject invitation workflow (7-day expiry)
- [Auth0 User Invitations](https://auth0.com/docs/customize/email/send-email-invitations-for-application-signup) — Email invitation patterns
- [Clerk Organization Invitations](https://clerk.com/docs/guides/organizations/add-members/invitations) — Unique invitation links with email delivery
- [Supersaas Invitation Flow](https://supersaas.dev/docs/teams/invite-flow) — User invitation workflow best practices
- [Microsoft Guest Invitation Expiry](https://learn.microsoft.com/en-us/answers/questions/5551108/what-is-the-validity-time-for-the-invitation-link) — External invitation validity (7-90 days)
- [SharePoint Guest Access Expiration](https://www.sharepointdiary.com/2021/08/guest-user-access-expiration-in-sharepoint-online-onedrive.html) — Guest access thresholds (1-365 days)

### Collaboration UX Patterns & Anti-Patterns
- [Table Stakes in SaaS](https://www.linkedin.com/pulse/table-stake-features-saas-enterprise-products-rohit-pareek) — Expected vs differentiating features
- [Table Stakes Sequencing | Product Teacher](https://www.productteacher.com/articles/sequencing-table-stakes-and-differentiators) — Prioritizing table stakes vs differentiators
- [Real-Time Collaboration 2025 | Medium](https://medium.com/@sachhsoft/building-real-time-collaboration-features-what-saas-teams-need-to-know-in-2025-d61a9b678cf5) — Collaboration as table stakes (was premium 5 years ago)
- [Collaboration Anti-Patterns | Lucid](https://lucid.co/blog/collaboration-anti-patterns) — Common collaboration mistakes (stifled discussion, no clear roles)
- [Improved UX for Sharing | Microsoft](https://learn.microsoft.com/en-us/power-platform/release-plan/2023wave1/power-apps/improved-ux-sharing-records) — Access revocation patterns (Share > Manage access > Remove)
- [UX Patterns for Collaborative Interfaces | Medium](https://medium.com/@space.alpaca/ux-patterns-to-use-in-collaborative-interfaces-cf7182ae6e52) — Permission management, mutual awareness, conflict resolution

### Access Management Best Practices
- [M365 Guest User Management | CoreView](https://www.coreview.com/blog/microsoft-365-guest-user-governance-and-best-sharing-practices-to-protect-your-privacy) — Inactive vs deleted access visibility
- [B2B Governance Best Practices | EasyLife 365](https://www.easylife365.cloud/stories/b2b-goverance-best-practices/) — Deactivating inactive guests (90+ days)
- [SharePoint Sharing Permissions | Microsoft Learn](https://learn.microsoft.com/en-us/sharepoint/modern-experience-sharing-permissions) — External sharing security

---
*Feature research for: Chat Overlay Sharing (All-Chat v1.3)*
*Researched: 2026-03-08*
