# Architecture Decision Records

ADRs capture single architectural decisions: their context, the decision taken, the alternatives considered, and the consequences. They describe the **current state**: when code changes what an ADR says, fix the ADR body in the same change, fold amendments from later ADRs into it (with a cross-link), and add or bump a `**Revised**: YYYY-MM-DD` line under `**Status**`. A replaced ADR gets `Superseded by ADR-XXXX` and a body that points at its successor; a removed feature gets `Withdrawn`. Never delete an ADR or renumber one.

## Template

```markdown
# ADR-XXXX: Title

**Date**: YYYY-MM-DD
**Status**: Proposed | Accepted | Superseded by ADR-YYYY | Withdrawn
**Revised**: YYYY-MM-DD (what changed, if ever revised)
**Deciders**: who decided

## Context
## Decision
## Consequences
## Alternatives considered (one line each, why rejected)
## Implementation (current file paths, tables, env vars)
```

## Index

| # | Title | Status | Summary |
|---|-------|--------|---------|
| [0001](./0001-standard-go-layout.md) | Standard Go Layout (Not Hexagonal Architecture) | Accepted | All Go services use the standard Go project layout, not ports-and-adapters. |
| [0002](./0002-redis-streams-pubsub.md) | Redis Streams + Pub/Sub Hybrid | Accepted | Listeners publish raw chat to a Redis Stream; the message-processor fans out per-overlay via Pub/Sub. |
| [0003](./0003-cloudnative-postgres.md) | CloudNativePG for PostgreSQL | Accepted | PostgreSQL runs as a CNPG cluster with automated failover, backups and PITR via the Barman Cloud plugin. |
| [0004](./0004-no-hexagonal-architecture.md) | No Hexagonal Architecture | Accepted | No ports/adapters abstraction layer; handlers call domain logic directly. |
| [0005](./0005-react-nextjs-frontend.md) | React + Next.js App Router | Accepted | Frontend is React with Next.js App Router, TypeScript and Tailwind. |
| [0006](./0006-youtube-quota-tracking.md) | YouTube Quota Reserve-Confirm-Rollback | Accepted | YouTube Data API quota use is reserved before a call, confirmed or rolled back after. |
| [0007](./0007-leadership-rebalancing.md) | Leadership Rebalancing for Auto-Scaling | Accepted | Pods voluntarily release excess channel locks so newly scaled pods get work. |
| [0008](./0008-feature-gate-infrastructure.md) | Feature Gate Infrastructure | Accepted | The `feature_gates` table plus `RequirePremium`/`RequireEarlyAccess` middleware gate features without a deploy. |
| [0009](./0009-ring-buffer-publisher.md) | Ring Buffer Publisher for Listener XADD Resilience | Accepted | Listeners buffer `chat:raw` entries in-process and retry instead of dropping them when Redis blips. |
| [0010](./0010-pronoun-enricher-alejo-api.md) | Pronoun Enricher via Alejo API | Accepted | Message-processor optionally fetches pronouns from the Alejo API for opted-in users. |
| [0011](./0011-zombie-listener-detection.md) | Zombie Listener Detection via Received-vs-Published Drift | Accepted | Detects listeners that are probe-healthy but stop publishing, by comparing received vs published counts. |
| [0012](./0012-oauth-scope-minimisation.md) | OAuth Scope Minimisation Post Extension v1.6.0 | Accepted | OAuth flows request only read-capable scopes after chat sending moved out of the backend. |
| [0013](./0013-overlay-observability-view.md) | Public Overlay Observability View + Shared `useOverlayStream` Hook | Accepted | A readable dashboard view of an overlay reuses the realtime pipeline through a shared hook. |
| [0014](./0014-demand-linger-symmetric-with-pubsub-linger.md) | Linger Upstream Capture Demand Symmetric with the Downstream Pub/Sub Linger | Accepted | Demand-gated listeners stop capturing on the same linger timeout the Pub/Sub fan-out uses. |
| [0015](./0015-eventsub-chat-ownership-claim.md) | Dynamic EventSub Chat-Ownership Claim for the Twitch Chat Partition | Accepted | Channels whose owner granted send scopes are read via EventSub; the rest stay on the IRC bot. |
| [0016](./0016-linked-twitch-credentials.md) | Per-Link Twitch Credentials for Non-Twitch-Login Accounts | Accepted | Accounts logging in through another platform store their own per-link Twitch credentials for EventSub reads. |
| [0017](./0017-chat-moderation-write-path.md) | Chat Moderation Write-Path | Accepted; as amended by [ADR-0048](./0048-delegated-overlay-moderators.md) | Streamers moderate chat (delete, timeout, ban) cross-platform from the dashboard using their own OAuth tokens. |
| [0018](./0018-premium-entitlements-via-patreon.md) | Premium Entitlements via Patreon | Accepted | A Patreon subscription drives `users.is_premium` automatically; admin grants still work. |
| [0019](./0019-split-streamer-viewer-premium.md) | Split Streamer vs Viewer Premium via a Polymorphic Patreon Subject | Accepted | Streamer and viewer premium are separate entitlements resolved from a polymorphic Patreon subject. |
| [0020](./0020-beta-tester-role.md) | Beta-Tester Role + Early-Access Feature Gates | Accepted | Beta testers get an `early_access` dimension beside `is_premium` in the gate system. |
| [0023](./0023-decoupled-youtube-quota-monitor.md) | Decoupled YouTube Quota Monitor | Accepted | A separate service reads the shared quota state, exports metrics and raises alerts. |
| [0024](./0024-send-to-all-combined-pill.md) | Streamer Send-to-All Combined Pill via Pre-Register + Reconcile | Accepted | The multi-platform "send to all" pill reconciles which platforms actually received the message. |
| [0025](./0025-innertube-livechatid-cache.md) | InnerTube Listener Caches activeLiveChatId for Streamer Sends | Accepted | The InnerTube listener scrapes the official live chat id into Redis so YouTube sends work. |
| [0026](./0026-twitch-irc-listener-deprecation.md) | Two-Phase Deprecation of the Twitch IRC Listener | Accepted | Twitch chat moves to the EventSub listener; the IRC listener is retired in two phases. |
| [0027](./0027-time-limited-admin-premium-overrides.md) | Time-Limited Admin Premium Overrides | Accepted | Admin premium grants can carry an expiry after which premium reverts on its own. |
| [0028](./0028-engagement-chat-command-write-path.md) | Engagement Chat-Command Write-Path | Accepted | Chat commands become validated poll votes and prediction wagers without touching the message-processor hot path. |
| [0029](./0029-viewer-points-economy-and-prediction-payout.md) | Viewer Points Economy & Prediction Payout Model | Accepted | Per-overlay points currency with an integrity model for balances, concurrency and prediction payouts. |
| [0030](./0030-twitch-native-engagement-mirroring.md) | Twitch-Native Poll/Prediction Mirroring | Accepted | Twitch-native polls and predictions are read and mirrored into overlays without pretending All-Chat ran them. |
| [0031](./0031-streamer-keyed-viewer-engagement.md) | Streamer-Keyed Viewer Engagement Participation | Accepted | Viewer participation surfaces are keyed on the streamer/overlay, not the viewer account. |
| [0032](./0032-source-liveness-heartbeat-and-overlay-self-heal.md) | Source-Liveness Heartbeat Contract and Overlay Self-Heal | Accepted | Sources heartbeat their liveness and overlays re-enable themselves when a source recovers. |
| [0033](./0033-message-processor-concurrent-enrichment.md) | Bounded-Concurrency Enrichment in the Message Processor | Accepted | Per-message enrichment stages run with bounded concurrency instead of serially. |
| [0034](./0034-admin-viewer-identity-model.md) | Admin Viewer Identity Model | Accepted | Admin "viewers" are durable platform accounts; premium and engagement key on the account, not the session. |
| [0035](./0035-admin-global-entity-search.md) | Admin Global Entity Search | Accepted; client-side federation superseded by [ADR-0086](./0086-admin-list-server-side-pagination.md) | The admin dashboard exposes a global search across entities. |
| [0036](./0036-admin-url-addressable-selection.md) | Admin URL-Addressable Selection | Accepted | Admin entity selection is encoded in the URL so views are shareable and refreshable. |
| [0037](./0037-twitch-chat-gifs.md) | Twitch Chat GIFs | Accepted | GIFs posted in Twitch chat are detected, cached and rendered on overlays. |
| [0039](./0039-database-connection-budget.md) | Database Connection Budget | Accepted | Pool sizes are configured per service against a shared connection budget for the Postgres cluster. |
| [0040](./0040-self-host-monaco-editor.md) | Self-host the Monaco CSS editor (no third-party CDN) | Accepted | The Monaco editor is bundled locally instead of loaded from a third-party CDN. |
| [0041](./0041-ambassador-role.md) | Ambassador Role + Public Homepage Showcase | Accepted | Curated streamers get an ambassador role with all premium features and a public homepage showcase. |
| [0042](./0042-editor-left-nav-settings.md) | Overlay Editor Settings Navigation (Left Nav Replaces Stacked Drawers) | Accepted | The overlay editor replaces nested accordion drawers with a left navigation. |
| [0043](./0043-preloaded-live-custom-css-editor.md) | Preloaded, live-preview Custom CSS editor with fork-on-edit | Accepted | The custom CSS editor preloads theme CSS and forks bundled themes on first edit. |
| [0044](./0044-overlay-text-outline-via-layered-text-shadow.md) | Overlay chat text outlines use layered text-shadow, not -webkit-text-stroke | Accepted | Chat text outlines are layered hard text-shadows for legibility on live video. |
| [0045](./0045-activation-auto-provision-first-overlay.md) | Auto-Provision a First Overlay + Default Source on First Sign-In (Activation Cliff) | Proposed | Analytics showed most sign-ins never produce a live overlay; auto-provision a first overlay on sign-in. |
| [0046](./0046-twitch-chat-notices-via-chat-notification.md) | Twitch chat notices (watch streaks, announcements) via channel.chat.notification | Accepted | Twitch notices (streaks, announcements) are ingested through the channel.chat.notification EventSub event. |
| [0047](./0047-username-color-resolution-order.md) | Username colour resolution order | Accepted | Username colours resolve in Twitch's documented order when a viewer has no set colour. |
| [0048](./0048-delegated-overlay-moderators.md) | Delegated Overlay Moderators | Accepted | Streamers can delegate the moderation view to their own volunteer moderators. |
| [0049](./0049-desktop-control-surfaces-via-paired-device-tokens.md) | Stream Deck and Desktop Control Surfaces via Paired Device Tokens | Accepted | Desktop plugins pair via PKCE device tokens to drive polls and send-to-all. |
| [0049](./0049-personal-access-tokens.md) | Personal Access Tokens | Accepted; recorded in full as [ADR-0051](./0051-personal-access-tokens-for-non-browser-clients.md) | Superseded record of the PAT decision; cite ADR-0051 instead. |
| [0050](./0050-color-opacity-in-the-color-value.md) | Visual-customizer colors carry their opacity in the value (8-digit hex) | Accepted | Customizer colors are 8-digit hex so opacity lives in the stored value. |
| [0051](./0051-personal-access-tokens-for-non-browser-clients.md) | Personal Access Tokens for Non-Browser Clients | Accepted | The canonical PAT decision: format and endpoints frozen while plugins were built against it. |
| [0052](./0052-retiring-euler-stream-for-tiktok-signing.md) | Retiring Euler Stream for TikTok webcast signing | Accepted | TikTok webcast signing is self-hosted in `services/tiktok-signer` using TikTok's own SDK. |
| [0053](./0053-consolidate-minimal-theme-and-retire-theme-ids-via-alias.md) | Consolidate the Minimal Clean theme; retire theme ids by alias, not deletion | Accepted | Bundled themes are consolidated and retired theme ids resolve through aliases. |
| [0054](./0054-twitch-moderation-and-automod-event-feed.md) | Twitch moderation and AutoMod event feed | Accepted | Twitch moderation actions and AutoMod events are ingested as an event feed. |
| [0055](./0055-ui-string-catalog-without-locale-routing.md) | A typed UI string catalog, with locale never in the URL | Accepted | UI strings live in a typed catalog; locale never appears in the URL. |
| [0056](./0056-shadcn-token-vocabulary-as-the-design-system-contract.md) | The shadcn token vocabulary is the design-system contract | Accepted | The shadcn `--color-*` token vocabulary is the contract frontend UI must consume. |
| [0057](./0057-outline-thickness-samples-the-disc.md) | A settable text outline samples the whole disc, not the compass | Accepted | Settable outline thickness extends shadows in all directions, not just four compass offsets. |
| [0058](./0058-owncast-instance-url-as-channel-identifier.md) | An Owncast source is the instance URL, not a channel name | Accepted | One websocket per Owncast instance URL; the URL is the channel identifier. |
| [0059](./0059-picarto-unofficial-popout-websocket.md) | Picarto chat via the unofficial pop-out WebSocket feed | Accepted | Picarto chat is read from its unofficial pop-out websocket, defensively parsed. |
| [0060](./0060-facebook-graph-api-and-moderation.md) | Facebook via Graph API polling with a dual-scope page credential and a moderation write path | Accepted | Facebook Live comments poll via Graph API with a dual-scope page credential, plus a moderation path. |
| [0061](./0061-rumble-chat-popup-sse.md) | Rumble chat via the internal chat pop-up SSE endpoint | Withdrawn (2026-10-08) | Withdrawn: no API reads arbitrary channels and Rumble support was removed. |
| [0062](./0062-instagram-live-comments-polling.md) | Instagram Live comments via HTTP polling of `live_comments` behind a Page-linked IG credential | Accepted | Instagram Live comments are polled via Graph API `live_comments`, read-only. |
| [0063](./0063-localization-contribution.md) | Beta-tester localization contribution with admin review and a repo-based catalog pipeline | Accepted | Translators contribute strings through `/translate`; admins review and export them. |
| [0064](./0064-tiktok-transport-tiers-by-entitlement.md) | TikTok transport tiers by entitlement | Accepted | TikTok listeners use a transport tier matched to the account's entitlement. |
| [0065](./0065-premium-official-youtube-api-listener.md) | Unlisted and members-only YouTube streams: pinned links and a premium official-API listener | Accepted | The official-API listener serves opted-in premium channels and pinned links for unlisted streams. |
| [0066](./0066-client-side-ui-locale-selection.md) | German as a second UI language, chosen per browser and applied after hydration | Accepted | The UI locale is chosen in the browser and applied after hydration, never routed. |
| [0067](./0067-desktop-plugins-in-own-repositories.md) | Desktop plugins move to their own repositories under GPL-3.0-or-later | Accepted | The desktop plugins live in their own GPL-3.0 licensed repos. |
| [0068](./0068-event-alerts-platform.md) | Event Alerts Platform | Accepted | Alerts event types route through a second `chat:raw` consumer to alerts, goal and list overlays. |
| [0069](./0069-stream-session-spans-gateway-pods-and-short-drops.md) | A stream session ends only when no gateway pod holds the overlay, after a 15-minute grace | Accepted | Stream sessions stay open through pod restarts and short disconnects with a grace window. |
| [0070](./0070-alert-overlays-carry-chat-sources-and-gateway-resolves-alert-profiles.md) | Alert Overlays Carry Chat Sources; the Gateway Resolves Alert Profiles | Accepted | The gateway resolves stored alert profiles at delivery time. |
| [0071](./0071-alerts-editor-sandboxed-custom-code-and-client-side-gif-search.md) | The Alerts Editor | Accepted | Editing is free with sandboxed custom code and client-side GIF search; uploads stay premium. |
| [0072](./0072-goal-and-list-overlays-polled-goals-and-debounced-leaderboards.md) | Goal and List Overlays | Accepted | Goal overlays poll platform API goals; list overlays rank supporters with debounced pushes. |
| [0073](./0073-klipy-app-key-delivered-at-runtime-from-the-cluster-secret.md) | The KLIPY App Key Is Delivered at Runtime from the Cluster Secret | Accepted | The KLIPY app key ships from a cluster secret at runtime, not baked into the image. |
| [0074](./0074-klipy-app-key-only-for-entitled-sessions.md) | The KLIPY App Key Is Served Only to Signed-In Sessions | Accepted | The KlIPY key endpoint requires authentication so it cannot be scraped anonymously. |
| [0075](./0075-alert-profiles-inherit-an-all-events-base-style.md) | Alert Profiles Inherit an All-Events Base Style | Accepted | Per-event alert profiles inherit from an all-events base style. |
| [0076](./0076-credit-roll-clips-play-the-mp4-from-twitch-web-gql.md) | Credit Roll Clips Play the Clip MP4 from Twitch's Web GQL | Accepted | The credit roll plays Twitch clip MP4s fetched from Web GQL. |
| [0077](./0077-kick-listener-follows-kicks-realtime-provider.md) | Kick Listener Follows Kick's Realtime Provider (Centrifugo or Pusher) | Accepted | Kick chat is read from Centrifugo or Pusher, whichever Kick uses at startup. |
| [0078](./0078-homepage-language-switch-and-german-for-everyone.md) | Homepage Language Switch, and German Opened to Everyone | Accepted | German is available to everyone with a homepage language switch. |
| [0079](./0079-streamer-chat-commands-in-engagement-service.md) | Streamer Chat Commands in engagement-service, With a Sandboxed JS Template Engine | Accepted | Streamer chat commands run in engagement-service with a sandboxed JS template engine. |
| [0080](./0080-chat-command-urlfetch-and-images.md) | Chat Command urlfetch and Images, With a Guarded Outbound HTTP Client | Accepted | `$(urlfetch)` runs through an SSRF-guarded outbound HTTP client. |
| [0081](./0081-chat-command-bot-sender-custom-bots-and-timers.md) | Chat Command Replies From an All-Chat Bot, Premium Custom Bots, and Timers | Accepted | Command replies can come from an All-Chat bot, premium custom bots, or run on timers. |
| [0082](./0082-altstream-public-sse-chat.md) | Altstream chat via the official public SSE chat API | Accepted | Altstream chat is read via the official public SSE API, read-only. |
| [0086](./0086-admin-list-server-side-pagination.md) | Server-side pagination and search for the admin lists | Accepted | Admin lists paginate and search server-side instead of fetching whole tables. |
| [0087](./0087-site-light-theme.md) | A site-wide light theme | Accepted | The site supports a light theme built on the same token system. |
| [0088](./0088-account-and-viewer-data-erasure.md) | Account and viewer data erasure | Accepted | A complete erasure path deletes account and viewer data across the schema. |
| [0089](./0089-retention-sweeper-cronjob.md) | Enforce data retention with a sweeper CronJob | Accepted | The 15-minute retention-sweeper CronJob enforces the privacy program's retention periods. |
| [0090](./0090-disconnecting-a-linked-platform.md) | Disconnecting a linked platform deletes its tokens | Accepted | Disconnecting deletes the linked platform's OAuth tokens. |
| [0091](./0091-discord-relay-webhook-encrypted-and-server-owned.md) | Discord relay webhook URLs are encrypted at rest and server-owned | Accepted | Relay webhook URLs are encrypted at rest and never returned by the API. |
| [0092](./0092-distroless-runtime-images.md) | Distroless runtime images | Accepted | Service images run distroless instead of Alpine. |
| [0093](./0093-support-bot-on-claude-hands-fixes-to-caterpillar.md) | Support bot runs on Claude and hands confirmed fixes to Caterpillar | Accepted | The support bot answered via Claude and hands confirmed fixes onward. |
| [0094](./0094-kick-events-from-realtime-feed-and-official-webhooks.md) | Kick events come from the realtime feed and from Kick's official webhooks | Accepted | Subscriptions, gifted subs and hosts are read from the chat feed; follows and KICKs gifts from signed webhooks kept subscribed per Kick source with an app token. |

ADR numbers are shared with the deployment repository (0021, 0022 and 0085 live there), so gaps in this list are expected.
