<p align="center">
  <strong>One overlay. Every platform.</strong>
</p>

<p align="center">
  Merge chat from <b>Twitch</b>, <b>YouTube</b>, <b>Kick</b>, <b>TikTok</b>, and <b>Discord</b> into a single feed for OBS — with full <b>7TV</b>, <b>BTTV</b>, and <b>FFZ</b> emote support.
</p>

<p align="center">
  <a href="https://allch.at"><strong>Try it free at allch.at</strong></a> · <a href="https://discord.gg/xCGBSuz39P">Discord</a> · <a href="https://github.com/caesarakalaeii/all-chat/issues/new/choose">Report a Bug</a>
</p>

---

## What is All-Chat?

All-Chat is a **free, open-source chat overlay** that merges Twitch, YouTube, Kick, TikTok, and Discord into a single feed for OBS. Sign in, drop a URL into a Browser Source, go live.

No bots. No IRC tokens. No complicated setup.

### Highlights

- **5 Platforms, One Feed** — Twitch, YouTube, Kick, TikTok, and Discord messages in a single overlay
- **No Setup Required** — Sign in, create an overlay, paste the URL into OBS. That's it.
- **Every Emote Works** — 7TV, BTTV, FFZ, plus native Twitch and YouTube emotes all render correctly
- **16 Themes + Full CSS** — Built-in themes from Win98 retro to cyberpunk neon, or write your own CSS
- **Real-Time Delivery** — Messages arrive via WebSocket — no 5-second delay
- **Smart Polling** — Listeners only run when your overlay is actually visible (see [How it works](#demand-driven-listeners))
- **Browser Extension** — Replace native site chat with All-Chat on Twitch, YouTube, and Kick
- **Open Source** — AGPL 3.0 licensed, fully self-hostable

---

## Quick Start

### 1. Create your overlay

1. Visit **[allch.at](https://allch.at)** and sign in with Twitch
2. Create a new overlay
3. Add chat sources — Twitch channels, YouTube streams, Kick channels, etc.
4. Tweak display settings (font, message duration, animations)

### 2. Add it to OBS

1. In OBS, add a **Browser Source**
2. Set the URL to `https://allch.at/overlay/YOUR_OVERLAY_ID`
3. Set dimensions to something sensible for a chat panel — e.g. **400 × 800** (adjust to taste)
4. **Optional:** Check *"Shutdown source when not visible"* — this tells All-Chat nobody is watching, so the backend stops polling platforms until you make the source visible again. Great for saving resources when you switch scenes.
5. Done!

> **Tip:** The overlay has a transparent background by default, so it layers cleanly over your stream.

### Monitor your chat on a second screen

The OBS overlay is built for compositing on stream, not for reading. For a **readable, dashboard-style view** of everything happening in an overlay, open:

```
https://allch.at/overlay/YOUR_OVERLAY_ID/view
```

A Twitch-dashboard-inspired monitor with a **resizable live Chat panel + Activity feed**, per-platform connection indicators, a config summary, and its own **light/dark mode**. Scrolling up **pauses the chat feed** on what you're reading (a pill shows how many new messages are waiting), and **clicking a username filters chat to that person**: a 1:1 conversation view that keeps up even when chat is flying by. It ignores the overlay's themes and animations — purely for observability — and even keeps moderated messages visible (struck-through) with a moderation log. No login required; it reuses the same public overlay link.

You can also open it from your overlay's settings page (or its preview page) via the **Monitor View** button.

**Your moderators can use it too.** Under **Moderators** in the overlay editor you can invite the
people who already moderate for you (Premium). They act with **their own** platform accounts, so
Twitch, YouTube and Kick re-check their moderator role on every action and it lands in your native
mod log under their name — and they never need a plan of their own.

### 3. Customize the look

All-Chat ships with **16 ready-made themes** you can paste into your OBS Browser Source custom CSS:

| Theme | Vibe |
|-------|------|
| [Win98](./docs/overlay-themes/win98-theme.css) | Nostalgic retro windows |
| [Cyberpunk](./docs/overlay-themes/cyberpunk-theme.css) | Neon-soaked sci-fi |
| [Minecraft](./docs/overlay-themes/minecraft-theme.css) | Blocky pixel style |
| [Neon Glass](./docs/overlay-themes/neon-glass-theme.css) | Frosted glass with glow |
| [Vaporwave](./docs/overlay-themes/vaporwave-theme.css) | A E S T H E T I C |
| [Terminal Hacker](./docs/overlay-themes/terminal-hacker-theme.css) | Green-on-black terminal |
| [Pastel Cute](./docs/overlay-themes/pastel-cute-theme.css) | Soft & friendly |
| [High Contrast](./docs/overlay-themes/high-contrast-theme.css) | Accessibility-first |
| [Modern Dark](./docs/overlay-themes/modern-dark-theme.css) | Clean dark mode |
| [Minimal](./docs/overlay-themes/minimal-theme.css) | Just the text |
| [Minimal Icon](./docs/overlay-themes/minimal-icon-theme.css) | Text + platform icons |
| [Noita Minimal](./docs/overlay-themes/noita-minimal-theme.css) | Inspired by Noita |
| [Comic Speech](./docs/overlay-themes/comic-speech-theme.css) | Pop-art speech balloons with halftone and ink outlines |
| [Neo Brutalist](./docs/overlay-themes/neo-brutalist-theme.css) | Bold cards with thick borders and hard offset shadows |
| [Sticky Notes](./docs/overlay-themes/sticky-notes-theme.css) | Tilted paper notes with washi tape |
| [Trading Card](./docs/overlay-themes/trading-card-theme.css) | Holographic collectible cards |

Pick a theme in the editor's **Advanced → Custom CSS** tab and its full CSS is
**preloaded into an in-app editor you can edit directly** — changes preview live as
you type, with syntax highlighting and inline tips for broken CSS. Only your *changes*
are saved, so fixes we ship to a theme still reach the rules you didn't touch;
**Reset to theme** discards your edits and re-links. Want to build your own from
scratch? See the **[CSS Customization Guide](./docs/CSS_CUSTOMIZATION.md)** or the **[Theme Gallery & Creation Guide](./docs/overlay-themes/README.md)**.

---

## Browser Extension

Don't just watch chat in OBS — **give your viewers unified chat too.**

The [All-Chat Browser Extension](https://github.com/caesarakalaeii/all-chat-extension) replaces native site chat on Twitch, YouTube, and Kick with your All-Chat feed. Your community can read and send messages across platforms without leaving the page they're watching.

**Install:**
- [Chrome / Edge / Brave](https://chromewebstore.google.com/detail/all-chat-extension/ioneembbnocfljgbhgfknbbnpfeadacm)
- [Firefox](https://addons.mozilla.org/en-US/firefox/addon/all-chat-extension/)
- [GitHub Releases](https://github.com/caesarakalaeii/all-chat-extension/releases) (gets updates first)

**Why it's great:**
- Replaces native chat automatically — no configuration needed
- Send messages with your existing platform account
- Full emote autocomplete (7TV, BTTV, FFZ) with `:` prefix
- Auto-reconnects if your connection drops — never miss a message

---

## Platform Support

| Platform | How it works | What you get |
|----------|-------------|--------------|
| **Twitch** | IRC + EventSub webhooks | Chat, emotes (native + 7TV/BTTV/FFZ), chat GIFs, badges, colors, channel points, raids, follows, chat notices (watch streaks, announcements, charity donations) |
| **YouTube** | HTTP polling + InnerTube API | Chat, Super Chat, member badges, multi-stream selection (public, currently live streams only) |
| **Kick** | Pusher WebSocket | Chat, emotes, badges, message deletion |
| **TikTok** | Unofficial live connector library | Chat messages, username-based display |
| **Discord** | Bot gateway + webhook relay | Channel chat relay to overlay |

> YouTube has two listener modes: the official **YouTube Data API** (quota-tracked with reserve-confirm-rollback) and an **InnerTube poller** that costs zero quota. Both are production-ready.
>
> **Note:** Only public, currently live YouTube streams are supported. Unlisted streams, private streams, and scheduled (upcoming) streams do not work. Stream discovery relies on YouTube's public search API (`search.list` with `eventType=live`), which only returns streams that are both public and actively broadcasting.

---

## Demand-Driven Listeners

All-Chat doesn't poll platforms 24/7. The system tracks WebSocket connections from your overlay:

1. **Overlay visible in OBS** → WebSocket connects → backend starts listening to your configured platforms
2. **Overlay hidden / OBS closed** → WebSocket disconnects → after a 60-second grace period, listeners stop polling
3. **Overlay visible again** → listeners spin back up instantly

This means zero wasted API calls when nobody is watching. It's also why *"Shutdown source when not visible"* in OBS works so well — it's not just an OBS optimization, it tells All-Chat to stand down.

---

## For Developers

Everything below is for contributors and self-hosters. If you're a streamer, you don't need any of this — just use [allch.at](https://allch.at).

---

### Architecture

All-Chat is a **Go microservices platform** with a React/Next.js frontend. Services communicate through Redis Streams (raw messages) and Redis Pub/Sub (overlay-specific delivery).

```
Platform Listeners (Twitch IRC, YouTube API, Kick Pusher, TikTok WS, Discord Bot)
       │
       ▼ publish raw messages
  Redis Streams (chat:raw)
       │
       ▼ consume via XREADGROUP
  Message Processor
       ├── Normalize to unified format
       ├── Enrich with emotes (7TV, BTTV, FFZ)
       └── Route to overlay channels
       │
       ▼ Redis Pub/Sub (overlay:{id})
  API Gateway (WebSocket server)
       │
       ▼ broadcast
  Frontend Overlay (React)
```

**Services** (all in `services/`):

| Service | Role |
|---------|------|
| `api-gateway` | WebSocket server for overlays, HTTP routing, OAuth callbacks |
| `auth-service` | Authentication, sessions, OAuth flows (Twitch, YouTube, Kick, Discord) |
| `emote-service` | 7TV, BTTV, FFZ, and native emote resolution |
| `twitch-listener` | Twitch IRC chat ingestion (⚠️ deprecated, ADR-0026 — migrating to `twitch-eventsub-listener`) |
| `twitch-eventsub-listener` | Twitch EventSub webhooks (channel points, moderation, raids) |
| `youtube-listener` | YouTube Data API polling with quota tracking |
| `youtube-listener-innertube` | YouTube InnerTube API polling (zero quota cost) |
| `youtube-quota-monitor` | Reads the shared YouTube quota table; exports the quota metric + publishes `quota:alerts` for the discord-bot (ADR-0023) |
| `kick-listener` | Kick chat via Pusher WebSocket |
| `tiktok-listener` | TikTok live chat via unofficial connector |
| `discord-listener` | Discord channel chat relay |
| `message-processor` | Message normalization, emote enrichment, overlay routing |
| `overlay-manager` | Overlay CRUD, source configuration, settings |
| `source-manager` | Leader election, active source tracking, demand coordination |
| `share-service` | Shareable overlay link generation; admin premium/beta-tester/ambassador role grants + feature gates; public ambassador showcase (ADR-0041) |
| `moderation-service` | Cross-platform chat moderation write-path: delete / timeout / ban (ADR-0017); delegated-moderator grants (ADR-0048) |
| `payment-service` | Patreon premium entitlements: grants `users.is_premium` (streamer, ADR-0018) and `viewers.is_premium` (viewer split, ADR-0019) from subscriptions |
| `token-refresh-service` | OAuth token refresh on schedule |
| `engagement-service` | Polls, predictions, and a per-overlay viewer points economy (#523) |
| `discord-bot` | TypeScript Discord bot for community ops (not a chat listener) |
| `support-bot` | Discord support/admin agent |

### Tech Stack

- **Backend**: Go 1.25+, Gin framework, pgx/v5 (PostgreSQL), go-redis/v9, Zap logging
- **Frontend**: React 19, Next.js 16 (App Router), TypeScript, Tailwind CSS, Zustand
- **Database**: PostgreSQL 16
- **Messaging**: Redis 7 (Streams + Pub/Sub)
- **Infrastructure**: Docker Compose (local dev), Kubernetes + CloudNativePG (production)

### Local Development

```bash
# Full environment
make docker-up         # Start PostgreSQL, Redis, and all services
make test              # Run tests
make migrate           # Apply database migrations

# Frontend-only (minimal backend)
make frontend-dev      # Start only what the frontend needs
make frontend-seed     # Create test overlay with mock sources
make frontend-messages # Generate fake chat messages
cd frontend && npm run dev
```

**First time?** See [GETTING_STARTED.md](./docs/GETTING_STARTED.md) for the full onboarding guide, or [FRONTEND_QUICK_START.md](./docs/frontend/FRONTEND_QUICK_START.md) if you're only touching the frontend.

### Self-Hosting

Want to run your own instance?

```bash
git clone https://github.com/caesarakalaeii/all-chat.git
cd all-chat
cp .env.example .env
# Edit .env with your Twitch/YouTube/etc. credentials
make docker-up
```

Visit `http://localhost:3000` (frontend) — you're live. The API Gateway is on `http://localhost:8080`.

**Prerequisites:**
- Docker & Docker Compose
- Twitch Developer application (for OAuth)
- Optional: YouTube API credentials, Kick OAuth, Discord bot token

For production Kubernetes deployment, see the **[Deployment Guide](./docs/DEPLOYMENT.md)**.

### Key Design Decisions

Architecture decisions are documented as ADRs in [`docs/adr/`](./docs/adr/README.md):

- **ADR-0001**: Standard Go Layout (not hexagonal/ports-adapters)
- **ADR-0002**: Redis Streams + Pub/Sub hybrid messaging
- **ADR-0003**: CloudNativePG for PostgreSQL in Kubernetes
- **ADR-0006**: YouTube quota reserve-confirm-rollback pattern
- **ADR-0007**: Leadership rebalancing for auto-scaling listeners
- **ADR-0008**: Feature gate infrastructure for premium capabilities
- **ADR-0020**: Beta-tester role + early-access feature gates (extends ADR-0008; a beta-tester is premium plus early-access, granted via an admin button — no data migration)
- **ADR-0041**: Ambassador role + public homepage showcase (extends ADR-0020; an ambassador is premium plus early-access AND opts into a public "Featured Ambassadors" homepage card; admin-granted, streamer-consented)
- **ADR-0013**: Public overlay observability view + shared `useOverlayStream` hook
- **ADR-0014**: Linger upstream capture demand symmetric with the downstream pub/sub linger
- **ADR-0015**: Dynamic EventSub chat-ownership claim — IRC is the always-on fallback; EventSub owns a channel only while it is actively delivering chat (no silent loss across the partition)
- **ADR-0026**: Two-phase deprecation of the Twitch IRC listener — `warn` nudges connected sources to re-add their Twitch source (in-overlay notice), `enforce` stops joining channels
- **ADR-0046**: Twitch chat notices via `channel.chat.notification` — watch streaks and announcements carry the chatter's own message on no other subscription, so they were silently dropped; notices are routed by type, skipping the five a dedicated subscription already delivers
- **ADR-0048**: Delegated overlay moderators — a moderator acts with their OWN platform credential (never the streamer's, no fallback), so the platform re-checks their role per call; Discord is the exception (the shared bot always acts, so All-Chat verifies guild permissions live) and premium is keyed on the overlay owner
- **ADR-0027**: Time-limited admin premium grants — an optional expiry on the admin override (streamer + viewer) that reverts on its own; recompute ignores an expired override, and a single-replica payment-service sweep clears lapsed grants

### Documentation

| Topic | Link |
|-------|------|
| Onboarding | [GETTING_STARTED.md](./docs/GETTING_STARTED.md) |
| Frontend dev | [FRONTEND_QUICK_START.md](./docs/frontend/FRONTEND_QUICK_START.md) |
| CSS customization | [CSS_CUSTOMIZATION.md](./docs/CSS_CUSTOMIZATION.md) |
| Deployment | [DEPLOYMENT.md](./docs/DEPLOYMENT.md) |
| Architecture overview | [docs/architecture/](./docs/architecture/00-OVERVIEW.md) |
| Data flow | [01-DATA-FLOW.md](./docs/architecture/01-DATA-FLOW.md) |
| Troubleshooting | [Decision Tree](./docs/troubleshooting/decision-tree.md) |
| LLM quick-ref guides | [docs/llm-guides/](./docs/llm-guides/) |

---

## Contributing

We welcome contributions from everyone!

**Non-developers:**
- Create and share themes (CSS)
- [Report bugs](https://github.com/caesarakalaeii/all-chat/issues/new/choose)
- [Suggest features](https://github.com/caesarakalaeii/all-chat/issues/new/choose)

**Developers:**
1. Fork the repository
2. Create a feature branch
3. Make your changes and write tests
4. Open a Pull Request

See [CONTRIBUTING.md](./CONTRIBUTING.md) for the full process.

---

## Acknowledgments

Built on the shoulders of:
- [gempir/go-twitch-irc](https://github.com/gempir/go-twitch-irc) — Twitch IRC client
- [Gin](https://gin-gonic.com/) — Go HTTP framework
- [Next.js](https://nextjs.org/) — React framework
- [7TV](https://7tv.io/), [BTTV](https://betterttv.com/), [FFZ](https://www.frankerfacez.com/) — Emote providers

---

## License

[AGPL 3.0](./LICENSE)

## Support

- [Back us on Patreon](https://patreon.com/all_chat) — unlock premium (streamer €5 / viewer €2)
- [Discord community](https://discord.gg/xCGBSuz39P)
- [GitHub Issues](https://github.com/caesarakalaeii/all-chat/issues)
- [GitHub Discussions](https://github.com/caesarakalaeii/all-chat/discussions)
- Email: all.chat.support@gmail.com

---

<p align="center"><b>Free. Open source. Built for streamers who refuse to pick just one platform.</b></p>
