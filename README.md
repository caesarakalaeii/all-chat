# All-Chat - Multi-Platform Chat Overlays

**Aggregate chat from Twitch, YouTube, Kick, and TikTok into a single streaming overlay.**

Perfect for streamers who multistream or want unified chat displays across platforms. Supports 7TV, BTTV, and FFZ emotes with full customization.

---

## ✨ Try It Now

**Live Instance**: [allch.at](https://allch.at)

Create your multi-platform chat overlay in minutes - no installation required!

## 🎯 What is All-Chat?

All-Chat lets you create beautiful chat overlays for OBS that combine messages from multiple streaming platforms in real-time:

- **🎮 Multi-Platform**: Twitch, YouTube, Kick, and TikTok (Twitch & YouTube currently supported)
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

Want a custom theme? Check out our [theme gallery](./docs/overlay-themes/):

1. Browse available themes (Win98, Cyberpunk, Minimalist, etc.)
2. Copy the CSS from the theme file
3. In OBS Browser Source properties, paste into "Custom CSS"
4. Adjust toggle options to show/hide elements

**Example themes:**
- 🪟 [Windows 98](./docs/overlay-themes/win98-theme.css) - Nostalgic retro style
- 🌈 More themes coming soon!

[📖 Learn how to create your own theme](./docs/overlay-themes/README.md)

## 🎨 Customization Features

### Theme Options

Every theme can be customized with CSS. Toggle visibility of:
- ✅ User avatars
- ✅ Platform badges (TWITCH, YOUTUBE, etc.)
- ✅ User badges (subscriber, moderator, etc.)
- ✅ Timestamps
- ✅ Message size and colors

[🎨 View Theme Documentation](./docs/overlay-themes/README.md)

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
- **Open Source**: MIT licensed, self-hostable

## 📱 Platform Support

| Platform | Status | Features |
|----------|--------|----------|
| **Twitch** | ✅ Working | Chat, emotes (Twitch/7TV/BTTV/FFZ), badges, colors |
| **YouTube** | 🧪 Testing | Chat, Super Chat, member badges |
| **Kick** | 🔜 Planned | Coming in Phase 2 |
| **TikTok** | 🔜 Planned | Coming in Phase 2 |

## 🏗️ Architecture

All-Chat uses a modern microservices architecture:

```
┌─────────────────┐
│   Your Stream   │
│  (OBS Browser)  │
└────────┬────────┘
         │ WebSocket
┌────────▼────────┐         ┌──────────────────┐
│   API Gateway   │◄────────┤  Platform APIs   │
└────────┬────────┘         │ (Twitch/YouTube) │
         │                  └──────────────────┘
         │ Redis Pub/Sub
         │
┌────────▼──────────┐
│ Message Processor │ ← Enriches messages with emotes
└────────┬──────────┘
         │
┌────────▼──────────┐
│ Chat Listeners    │ ← Connects to chat platforms
│ (Twitch/YouTube)  │
└───────────────────┘
```

**Key Services:**
- **API Gateway**: WebSocket server, HTTP routing
- **Twitch Listener**: IRC connection, message normalization
- **YouTube Listener**: API polling, message normalization
- **Message Processor**: Emote enrichment, message routing
- **Source Manager**: Active source tracking, leader election

[📖 Read full architecture docs](./CLAUDE.md#architecture-principles)

## 🛠️ Tech Stack

- **Backend**: Go 1.23+ (Gin framework)
- **Frontend**: React 18 + Next.js 14 (App Router)
- **Database**: PostgreSQL 16
- **Cache/Messaging**: Redis 7
- **Deployment**: Docker + Kubernetes
- **Real-Time**: WebSockets + Redis Pub/Sub

## 📖 Documentation

- **[Theme Creation Guide](./docs/overlay-themes/README.md)** - Create custom overlay themes
- **[Win98 Theme Quick Start](./docs/overlay-themes/QUICK-START.md)** - Apply the Win98 theme
- **[Deployment Guide](./docs/DEPLOYMENT.md)** - Self-host your own instance
- **[Developer Guide](./CLAUDE.md)** - Architecture and development
- **[Getting Started (Developers)](./GETTING_STARTED.md)** - Navigate the codebase

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
- 📝 **Report bugs** - Open issues for problems you encounter
- 💡 **Suggest features** - Tell us what you'd like to see
- 📖 **Improve docs** - Help make documentation clearer

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

### Phase 2: Enhanced Features (🚧 In Progress)
- [ ] React + Next.js frontend
- [ ] User dashboard
- [ ] Overlay configuration UI
- [ ] Theme marketplace
- [x] Custom CSS themes (Win98 theme available!)
- [ ] Message filtering and moderation

### Phase 3: Platform Expansion (🔜 Planned)
- [ ] Kick integration
- [ ] TikTok integration
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

**Current Version**: 0.9.0 (Beta)
**Status**: Phase 1 Complete (~90%), Phase 2 in progress

**What's Working:**
- ✅ Twitch chat fully functional
- ✅ YouTube chat implemented (testing phase)
- ✅ Real-time WebSocket streaming
- ✅ Emote support (7TV, BTTV, FFZ)
- ✅ Custom CSS themes
- ✅ Docker deployment

**What's Next:**
- 🔄 Frontend dashboard UI
- 🔄 User overlay management
- 🔄 Complete YouTube testing
- 🔄 Theme marketplace

## 🙏 Acknowledgments

Built with amazing open-source projects:

- [gempir/go-twitch-irc](https://github.com/gempir/go-twitch-irc) - Twitch IRC client
- [Gin Framework](https://gin-gonic.com/) - HTTP web framework
- [Next.js](https://nextjs.org/) - React framework
- [Redis](https://redis.io/) - In-memory data store
- [PostgreSQL](https://www.postgresql.org/) - Database

Special thanks to the Twitch, 7TV, BTTV, and FFZ communities!

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 📮 Support & Contact

- **🐛 Bug Reports**: [GitHub Issues](https://github.com/caesar/all-chat/issues)
- **💬 Questions**: [GitHub Discussions](https://github.com/caesar/all-chat/discussions)
- **📧 Email**: support@allch.at
- **🌐 Website**: [allch.at](https://allch.at)

## ⭐ Star History

If you find this project useful, please consider giving it a star! It helps others discover All-Chat.

---

**Made with ❤️ for the streaming community**
