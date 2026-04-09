<p align="center">
  <strong>One overlay. Every platform.</strong>
</p>

<p align="center">
  Combine chat from <b>Twitch</b>, <b>YouTube</b>, <b>Kick</b>, <b>TikTok</b>, and <b>Discord</b> into a single, beautiful streaming overlay — with full support for <b>7TV</b>, <b>BTTV</b>, and <b>FFZ</b> emotes.
</p>

<p align="center">
  <a href="https://allch.at"><strong>Try it free at allch.at</strong></a> · <a href="https://discord.gg/xCGBSuz39P">Discord</a> · <a href="https://github.com/caesarakalaeii/all-chat/issues/new/choose">Report a Bug</a>
</p>

---

## What is All-Chat?

All-Chat is a **free, open-source overlay** for streamers who broadcast to more than one platform — or just want a cleaner chat experience. Add it as a browser source in OBS, Streamlabs, or any streaming software and see every message from every platform in one place.

No bots. No IRC tokens. No complicated setup. Just sign in, create an overlay, and go live.

### Highlights

- **Multi-Platform** — Twitch, YouTube, Kick, TikTok, and Discord in a single feed
- **Full Emote Support** — 7TV, BTTV, FFZ, plus native Twitch and YouTube emotes render correctly
- **Real-Time** — Messages arrive via WebSocket in under 500 ms
- **Customizable** — 12 built-in themes, full CSS editor, Google Fonts, show/hide badges, avatars, timestamps
- **Shareable** — Give viewers a link so they can follow chat outside of OBS
- **Smart Polling** — Listeners only run when your overlay is actually visible (see [How it works](#-demand-driven-listeners))
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

### 3. Customize the look

All-Chat ships with **12 ready-made themes** you can paste into your OBS Browser Source custom CSS:

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

Want to build your own? See the **[CSS Customization Guide](./docs/CSS_CUSTOMIZATION.md)** or the **[Theme Gallery & Creation Guide](./docs/overlay-themes/README.md)**.

---

## Browser Extension

Don't just watch chat in OBS — **use it from your browser.**

The [All-Chat Browser Extension](https://github.com/caesarakalaeii/all-chat-extension) replaces the native site chat on Twitch, YouTube, Kick, and TikTok with a unified All-Chat panel. Read and send messages across platforms without leaving the page.

**Install:**
- [Chrome / Edge](https://chromewebstore.google.com/detail/all-chat-extension/ioneembbnocfljgbhgfknbbnpfeadacm)
- [Firefox](https://addons.mozilla.org/en-US/firefox/addon/all-chat-extension/)
- [GitHub Releases](https://github.com/caesarakalaeii/all-chat-extension/releases) (gets updates first)

**What it does:**
- Swaps the site's native chat with All-Chat automatically
- Uses browser-native OAuth — no tokens leave your machine
- Connects to shared chat sources the streamer configured
- Zero extra setup — works out of the box

---

## Platform Support

| Platform | How it works | What you get |
|----------|-------------|--------------|
| **Twitch** | IRC + EventSub webhooks | Chat, emotes (native + 7TV/BTTV/FFZ), badges, colors, channel points, raids, follows |
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
| `twitch-listener` | Twitch IRC chat ingestion |
| `twitch-eventsub-listener` | Twitch EventSub webhooks (channel points, moderation, raids) |
| `youtube-listener` | YouTube Data API polling with quota tracking |
| `youtube-listener-innertube` | YouTube InnerTube API polling (zero quota cost) |
| `kick-listener` | Kick chat via Pusher WebSocket |
| `tiktok-listener` | TikTok live chat via unofficial connector |
| `discord-listener` | Discord channel chat relay |
| `message-processor` | Message normalization, emote enrichment, overlay routing |
| `overlay-manager` | Overlay CRUD, source configuration, settings |
| `source-manager` | Leader election, active source tracking, demand coordination |
| `share-service` | Shareable overlay link generation |
| `token-refresh-service` | OAuth token refresh on schedule |

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
make migrate-up        # Apply database migrations

# Frontend-only (minimal backend)
make frontend-dev      # Start only what the frontend needs
make frontend-seed     # Create test overlay with mock sources
make frontend-messages # Generate fake chat messages
cd frontend && npm run dev
```

**First time?** See [GETTING_STARTED.md](./GETTING_STARTED.md) for the full onboarding guide, or [FRONTEND_QUICK_START.md](./FRONTEND_QUICK_START.md) if you're only touching the frontend.

### Self-Hosting

Want to run your own instance?

```bash
git clone https://github.com/caesarakalaeii/all-chat.git
cd all-chat
cp .env.example .env
# Edit .env with your Twitch/YouTube/etc. credentials
make docker-up
```

Visit `http://localhost:8080` — you're live.

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

### Documentation

| Topic | Link |
|-------|------|
| Onboarding | [GETTING_STARTED.md](./GETTING_STARTED.md) |
| Frontend dev | [FRONTEND_QUICK_START.md](./FRONTEND_QUICK_START.md) |
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

- [Discord community](https://discord.gg/xCGBSuz39P)
- [GitHub Issues](https://github.com/caesarakalaeii/all-chat/issues)
- [GitHub Discussions](https://github.com/caesarakalaeii/all-chat/discussions)
- Email: support@allch.at

---

<p align="center"><b>Made with love for the streaming community</b></p>
