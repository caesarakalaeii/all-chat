# All-Chat - Multi-Platform Chat Overlays

**GitHub**: [caesarakalaeii/all-chat](https://github.com/caesarakalaeii/all-chat)

**Aggregate chat from Twitch, YouTube, Kick, and TikTok into a single streaming overlay.**

Perfect for streamers who multistream or want unified chat displays across platforms. Supports 7TV, BTTV, and FFZ emotes with full customization.

---

## ✨ Try It Now

**Live Instance**: [allch.at](https://allch.at)

Create your multi-platform chat overlay in minutes - no installation required!

### 🧭 Prefer working from your browser?

Check out the **[All-Chat Browser Extension](https://github.com/caesarakalaeii/all-chat-extension)**. It replaces the native site chat (Twitch/YouTube/Kick/TikTok) with an All-Chat panel so you can talk with everyone else across platforms right from your browser, without running a local server.

**What it solves**
- Native chat swap: automatically swaps the site's chat UI with All-Chat so you can read and send messages across platforms from one place
- Built-in auth: uses browser-native OAuth to sign you into supported platforms without sharing tokens elsewhere
- Automatic source bridging: connects to the public chat sources the streamer has shared through All-Chat—no need to keep platform tabs open
- Zero extra setup: the extension connects to All-Chat out of the box—no local services needed

**Install / setup** (follows the extension README)
1. Download the latest packaged zip from the **Releases** section of the browser extension repo and unzip it.
2. Chrome/Edge: open `chrome://extensions` or `edge://extensions`, enable **Developer mode**, and click **Load unpacked** on the unzipped folder.
3. Firefox: open `about:debugging#/runtime/this-firefox`, click **Load Temporary Add-on**, and choose `manifest.json` inside the unzipped folder.
4. Open a supported site; the native chat will be replaced with All-Chat. Use the extension popup to sign in with Twitch and start chatting across platforms.

## 🎯 What is All-Chat?

All-Chat lets you create beautiful chat overlays for OBS that combine messages from multiple streaming platforms in real-time:

- **🎮 Multi-Platform**: Twitch, YouTube, Kick, and TikTok (Twitch & Kick fully supported, YouTube in beta)
- **🎨 Customizable Themes**: Use CSS to create custom themes (Win98, cyberpunk, minimalist, etc.)
- **💬 Emote Support**: Full support for 7TV, BTTV, and FFZ emotes
- **⚡ Real-Time**: WebSocket-based for instant message delivery
- **🎭 Multi-Source**: One overlay can show chat from multiple platforms simultaneously
- **🖼️ Flexible Display**: Show/hide avatars, badges, platform labels, timestamps

## 🚀 Quick Start (Using the Live Instance)

### 1. Create an Overlay

Visit **[allch.at](https://allch.at)** and:
1. Sign in with Twitch
2. Create a new overlay
3. Add your chat sources (Twitch channels, YouTube streams, etc.)
4. Customize settings (font size, message duration, etc.)

### 2. Add to OBS

1. In OBS, add a **Browser Source**
2. Set the URL to: `https://allch.at/overlay/YOUR_OVERLAY_ID`
3. Set width/height (e.g., 1920x1080)
4. Check "Shutdown source when not visible" for best performance
5. Click OK!

### 3. Customize the Look (Optional)

Want a custom theme? We have comprehensive documentation for CSS customization:

1. Browse available themes (Win98, Cyberpunk, Minimalist, etc.)
2. Copy the CSS from the theme file
3. In OBS Browser Source properties, paste into "Custom CSS"
4. Adjust toggle options to show/hide elements

**Example themes:**
- 🪟 [Windows 98](./docs/overlay-themes/win98-theme.css) - Nostalgic retro style
- 🌈 More themes coming soon!

**📚 Customization Documentation:**
- [**CSS Customization Guide**](./docs/CSS_CUSTOMIZATION.md) - Complete CSS reference and guide
- [Theme Gallery & Creation Guide](./docs/overlay-themes/README.md) - Create your own themes
- [Quick Start Guide](./docs/overlay-themes/QUICK-START.md) - Apply themes quickly

## 🎨 Customization Features

### Theme Options

Every theme can be customized with CSS. Toggle visibility of:
- ✅ User avatars
- ✅ Platform badges (TWITCH, YOUTUBE, etc.)
- ✅ User badges (subscriber, moderator, etc.)
- ✅ Timestamps
- ✅ Message size and colors

**📚 Customization Documentation:**
- [**CSS Customization Guide**](./docs/CSS_CUSTOMIZATION.md) - Complete CSS reference for developers
- [Theme Gallery & Examples](./docs/overlay-themes/README.md) - Browse and create themes

### Overlay Settings

Configure in the dashboard:
- **Max Messages**: How many messages to keep on screen
- **Message Duration**: How long each message stays visible (seconds)
- **Font Size**: Text size for messages
- **Chat Sources**: Which channels/streams to aggregate

## 🌟 Features

### For Streamers

- **Multi-Stream Ready**: Show chat from all your streaming platforms at once
- **Zero Setup**: No bots to configure, no IRC tokens to manage
- **Real-Time Updates**: Messages appear instantly with WebSocket technology
- **OBS Integration**: Simple browser source, works with all streaming software
- **Customizable**: Complete control over appearance with CSS

### For Viewers

- **Platform Identification**: See which platform each message is from
- **Rich Emotes**: Full support for Twitch, 7TV, BTTV, and FFZ emotes
- **User Badges**: Subscriber, moderator, and other badges display correctly
- **Smooth Animations**: Messages slide in smoothly with configurable durations

### Technical Features

- **Cloud-Native**: Scalable microservices architecture
- **High Performance**: Built with Go for speed and efficiency
- **Real-Time**: Redis Streams + Pub/Sub for low-latency messaging
- **Reliable**: Health checks, auto-reconnection, graceful degradation
- **Open Source**: AGPL3.0 licensed, self-hostable

## 📱 Platform Support

| Platform | Status | Features |
|----------|--------|----------|
| **Twitch** | ✅ Working | Chat, emotes (Twitch/7TV/BTTV/FFZ), badges, colors |
| **YouTube** | 🧪 Beta | Chat, Super Chat, member badges (closed beta) |
| **Kick** | ✅ Working | Chat, emotes, badges via Pusher WebSocket |
| **TikTok** | 🚧 Development | OAuth complete, listener in development (closed beta) |

## 🏗️ Architecture

All-Chat uses a modern microservices architecture:

```
┌─────────────────┐
│   Your Stream   │
│  (OBS Browser)  │
└────────┬────────┘
         │ WebSocket overlays
┌────────▼────────┐        ┌────────────────────┐
│  api-gateway    │◄──────►│   auth-service     │
└────────┬────────┘        └────────────────────┘
         │ Redis Pub/Sub
         │
┌────────▼──────────┐        ┌──────────────────┐
│ message-processor │◄──────►│  emote-service   │
└────────┬──────────┘        └──────────────────┘
         │
┌────────▼──────────┐
│  chat listeners   │ ← twitch-listener, youtube-listener,
│ (multi-platform)  │    kick-listener, tiktok-listener
└────────┬──────────┘
         │
┌────────▼──────────┐
│  source-manager   │ ← Active source tracking
└────────┬──────────┘
         │
┌────────▼──────────┐
│ overlay-manager   │ → Feeds overlays to OBS
└───────────────────┘
```

**Key Services (mirrors `services/`):**
- **api-gateway**: WebSocket server for overlays, HTTP routing, OAuth callbacks
- **auth-service**: Manages authentication, sessions, and OAuth flows
- **emote-service**: Fetches platform emotes and enriches chat payloads
- **kick-listener**: Connects to Kick (Pusher) WebSocket and normalizes chat
- **message-processor**: Routes messages via Redis and orchestrates enrichment
- **overlay-manager**: Maintains overlay state and renders scenes for OBS clients
- **source-manager**: Tracks active sources and coordinates Redis leader election
- **tiktok-listener**: Polls TikTok live WebSocket feed and normalizes chat
- **twitch-listener**: Handles IRC ingestion and normalization for Twitch chat
- **youtube-listener**: Polls YouTube Live API, normalizes chat (leader election)

[📖 Read full architecture docs](./CLAUDE.md#architecture-principles)

## 🛠️ Tech Stack

- **Backend**: Go 1.25+ (Gin framework)
- **Frontend**: React 18 + Next.js 14 (App Router)
- **Database**: PostgreSQL 16
- **Cache/Messaging**: Redis 7
- **Deployment**: Docker + Kubernetes
- **Real-Time**: WebSockets + Redis Pub/Sub

## 📖 Documentation

### For Users & Streamers
- **[CSS Customization Guide](./docs/CSS_CUSTOMIZATION.md)** - Complete CSS reference for customizing overlays
- **[Theme Gallery](./docs/overlay-themes/README.md)** - Browse and create custom overlay themes
- **[Win98 Theme Quick Start](./docs/overlay-themes/QUICK-START.md)** - Apply the Win98 theme in minutes

### For Developers & Self-Hosting
- **[Deployment Guide](./docs/DEPLOYMENT.md)** - Self-host your own instance
- **[Developer Guide](./CLAUDE.md)** - Architecture and development
- **[Getting Started Guide](./GETTING_STARTED.md)** - Navigate the codebase

## 🔧 Self-Hosting

Want to run your own instance? See the **[Deployment Guide](./docs/DEPLOYMENT.md)** for:

- Docker Compose setup (easiest)
- Kubernetes deployment (production)
- Environment configuration
- API credentials setup
- Monitoring and scaling

**Prerequisites:**
- Docker & Docker Compose
- Twitch Developer account (for OAuth)
- Optional: YouTube API credentials
- Optional: Kubernetes cluster

```bash
# Quick start with Docker Compose
git clone https://github.com/caesar/all-chat.git
cd all-chat
cp .env.example .env
# Edit .env with your credentials
make docker-up
```

Visit http://localhost:8080

[📖 Full deployment guide](./docs/DEPLOYMENT.md)

## 🤝 Contributing

We welcome contributions! Here's how you can help:

### For Non-Developers

- 🎨 **Create themes** - Share your custom CSS themes
- 📝 **Report bugs** - Use our [bug report template](https://github.com/caesarakalaeii/all-chat/issues/new/choose) to report issues
- 💡 **Suggest features** - Use our [feature request template](https://github.com/caesarakalaeii/all-chat/issues/new/choose) to propose ideas
- 📖 **Improve docs** - Use our [documentation template](https://github.com/caesarakalaeii/all-chat/issues/new/choose) to suggest improvements

### For Developers

- 🔧 **Fix bugs** - Check our [issues](https://github.com/caesar/all-chat/issues)
- ✨ **Add features** - Implement items from the [roadmap](#-roadmap)
- 🧪 **Write tests** - Improve test coverage
- 📊 **Add platform support** - Help integrate Kick or TikTok

**Process:**
1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Write/update tests
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to your branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

[📖 Developer documentation](./CLAUDE.md)

## 🗺️ Roadmap

### Phase 1: Core Platform (✅ Complete)
- [x] Microservices architecture
- [x] Twitch integration (IRC + OAuth)
- [x] YouTube integration (API polling)
- [x] Real-time WebSocket delivery
- [x] Message normalization
- [x] Emote enrichment (7TV, BTTV, FFZ)

### Phase 2: Enhanced Features (✅ Complete)
- [x] React + Next.js frontend
- [x] User dashboard
- [x] Overlay configuration UI
- [ ] Theme marketplace
- [x] Custom CSS themes (Win98 theme available!)
- [ ] Message filtering and moderation

### Phase 3: Platform Expansion (🚧 In Progress)
- [x] Kick integration (OAuth + WebSocket listener complete)
- [🚧] TikTok integration (OAuth complete, listener in development)
- [ ] Rumble integration
- [ ] Custom emote animations
- [ ] Advanced filtering (keywords, users, etc.)

### Phase 4: Advanced Features (💡 Future)
- [ ] Analytics dashboard
- [ ] Chat replay system
- [ ] Multi-language support
- [ ] Mobile app (for overlay management)
- [ ] AI-powered moderation
- [ ] Bits/Super Chat highlighting

## 📊 Project Status

**Current Version**: 1.0.0 (Beta)
**Status**: Phase 2 Complete, Phase 3 in progress

**What's Working:**
- ✅ Twitch chat fully functional
- ✅ Kick chat fully functional
- ✅ YouTube chat (closed beta)
- ✅ React + Next.js frontend
- ✅ User dashboard and overlay management
- ✅ Real-time WebSocket streaming
- ✅ Emote support (7TV, BTTV, FFZ)
- ✅ Custom CSS themes
- ✅ Docker + Kubernetes deployment

**What's Next:**
- 🔄 TikTok listener implementation
- 🔄 Complete YouTube/TikTok beta testing
- 🔄 Theme marketplace
- 🔄 Message filtering and moderation

## 🙏 Acknowledgments

Built with amazing open-source projects:

- [gempir/go-twitch-irc](https://github.com/gempir/go-twitch-irc) - Twitch IRC client
- [Gin Framework](https://gin-gonic.com/) - HTTP web framework
- [Next.js](https://nextjs.org/) - React framework
- [Redis](https://redis.io/) - In-memory data store
- [PostgreSQL](https://www.postgresql.org/) - Database

Special thanks to the Twitch, 7TV, BTTV, and FFZ communities!

## 📝 License

This project is licensed under the AGPL 3.0 - see the [LICENSE](LICENSE) file for details.

## 📮 Support & Contact

- **💬 Discord**: [Join our Discord server](https://discord.gg/xCGBSuz39P) - Get help and chat with the community
- **🐛 Bug Reports**: [GitHub Issues](https://github.com/caesar/all-chat/issues)
- **💬 Questions**: [GitHub Discussions](https://github.com/caesar/all-chat/discussions)
- **📧 Email**: support@allch.at
- **🌐 Website**: [allch.at](https://allch.at)

## ⭐ Star History

If you find this project useful, please consider giving it a star! It helps others discover All-Chat.

---

**Made with ❤️ for the streaming community**
