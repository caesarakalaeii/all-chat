# Discord Quota Monitor Bot

A Discord bot that monitors YouTube API quota usage and posts real-time updates and alerts to a Discord channel.

## Features

- 📊 **Real-time Alerts**: Subscribes to Redis pub/sub for instant quota event notifications
- 🕐 **Periodic Updates**: Posts quota status summaries at configurable intervals (default: 1 hour)
- 🎨 **Rich Embeds**: Beautiful Discord embeds with:
  - Current quota state (HEALTHY, DEGRADED, CRITICAL, EXHAUSTED, DEPLETED)
  - Usage percentage with visual progress bar
  - Remaining quota units
  - Quota reset countdown
  - Top consuming channels
  - Polling speed multiplier
- 🚨 **Smart Alerts**: Different severity levels (info, warning, error, critical) with color coding
- 🔔 **Event Types**:
  - State transitions (healthy → degraded → critical, etc.)
  - Threshold crossings (70%, 85%, 95%, 100%)
  - Quota exhausted/depleted warnings
  - Quota recovery notifications
  - Per-channel quota exceeded alerts

## Prerequisites

1. **Discord Bot**:
   - Create a bot at [Discord Developer Portal](https://discord.com/developers/applications)
   - Enable necessary intents (just need basic guild access)
   - Get the bot token
   - Invite bot to your server with permission to send messages

2. **Discord Channel**:
   - Create a dedicated channel for quota monitoring
   - Get the channel ID (enable Developer Mode in Discord settings, right-click channel → Copy ID)

3. **Infrastructure**:
   - Redis server (for receiving quota alerts)
   - YouTube Listener service running (provides quota status API)

## Setup

### 1. Install Dependencies

```bash
cd services/discord-bot
npm install
```

### 2. Configure Environment

Copy the example environment file and fill in your values:

```bash
cp .env.example .env
```

Edit `.env`:

```env
# Required: Your Discord bot token from Discord Developer Portal
DISCORD_BOT_TOKEN=YOUR_DISCORD_BOT_TOKEN_HERE

# Required: Discord channel ID where bot will post updates
DISCORD_CHANNEL_ID=YOUR_CHANNEL_ID_HERE

# Redis connection (adjust if not running locally)
REDIS_HOST=localhost
REDIS_PORT=6379

# YouTube Listener API endpoint
YOUTUBE_LISTENER_URL=http://localhost:8086

# How often to post status updates (in milliseconds)
# Default: 3600000 (1 hour)
STATUS_UPDATE_INTERVAL=3600000
```

### 3. Run the Bot

**Development (with auto-restart):**
```bash
npm run dev
```

**Production:**
```bash
npm start
```

### 4. Docker Deployment

**Build the image:**
```bash
docker build -t all-chat-discord-bot .
```

**Run the container:**
```bash
docker run -d \
  --name discord-quota-bot \
  --env-file .env \
  --restart unless-stopped \
  all-chat-discord-bot
```

**With docker-compose:**

Add to your `docker-compose.yml`:

```yaml
discord-bot:
  build: ./services/discord-bot
  environment:
    DISCORD_BOT_TOKEN: ${DISCORD_BOT_TOKEN}
    DISCORD_CHANNEL_ID: ${DISCORD_CHANNEL_ID}
    REDIS_HOST: redis
    REDIS_PORT: 6379
    YOUTUBE_LISTENER_URL: http://youtube-listener:8086
    STATUS_UPDATE_INTERVAL: 3600000
  depends_on:
    - redis
    - youtube-listener
  restart: unless-stopped
```

## How It Works

### Real-time Alerts (Redis Pub/Sub)

The bot subscribes to the `quota:alerts` Redis channel. The YouTube Listener service publishes quota events to this channel:

1. **State Changes**: When quota state transitions (e.g., HEALTHY → DEGRADED)
2. **Threshold Crossings**: When usage crosses 70%, 85%, 95%, or 100%
3. **Critical Events**: Quota exhausted, depleted, or recovered
4. **Channel Alerts**: When individual channels exceed their allocation

### Periodic Status Updates

Every hour (configurable), the bot fetches the current quota status from the YouTube Listener API (`/quota/status`) and posts a comprehensive summary embed showing:

- Current state and usage percentage
- Visual progress bar
- Remaining quota units
- Time until quota resets (midnight PST)
- Polling speed multiplier
- Top 5 quota-consuming channels

### Single Message per Kind (Rollout Cleanup)

The bot keeps only one status message (and one alert message) visible in the channel instead of posting a new one on every deploy. During a process lifetime it edits its existing messages in place. On startup — after a rollout or restart, which clears the in-memory message IDs — it scans recent channel history for its own quota messages, reuses the most recent one (editing it in place, so no new notification is sent), and deletes the older stale duplicates. Only the bot's own messages are touched, so no extra Discord permissions are required.

## Discord Embed Examples

### Quota Status Embed (Healthy)

```
📊 YouTube API Quota Status
━━━━━━━━━━━━━━━━━━━━━━━
Current State: 🟢 HEALTHY

📈 Usage
2,450 / 10,000 units (24.50%)

Progress
[████░░░░░░░░░░░░░░░░] 24.5%

⏱️ Remaining          🔄 Resets          ⚡ Polling Speed
7,550 units          in 8 hours         1.00x

🔝 Top Consuming Channels
1. UCxxxxxxxxxxxxx - 450 units (18.4%)
2. UCyyyyyyyyyyyyyy - 380 units (15.5%)
3. UCzzzzzzzzzzzzz - 290 units (11.8%)
```

### Quota Alert Embed (Critical State)

```
🔄 Quota Alert: State Changed
━━━━━━━━━━━━━━━━━━━━━━━
Quota state changed from DEGRADED to CRITICAL (87.50% used)

State          Usage          Remaining
🟠 CRITICAL    87.50%         1,250 units

Severity: ERROR
```

### Quota Depleted Alert

```
❌ Quota Alert: Quota Depleted
━━━━━━━━━━━━━━━━━━━━━━━
Quota depleted: 10,000/10,000 units used, all API requests blocked

State          Usage          Remaining
⛔ DEPLETED    100.00%        0 units

Severity: CRITICAL
```

## Configuration Options

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `DISCORD_BOT_TOKEN` | (required) | Discord bot token from Developer Portal |
| `DISCORD_CHANNEL_ID` | (required) | Discord channel ID where messages are posted |
| `REDIS_HOST` | `localhost` | Redis server hostname |
| `REDIS_PORT` | `6379` | Redis server port |
| `YOUTUBE_LISTENER_URL` | `http://localhost:8086` | YouTube Listener API base URL |
| `STATUS_UPDATE_INTERVAL` | `3600000` | Interval between status posts (milliseconds) |

## Troubleshooting

**Bot not posting messages:**
- Verify bot has permissions to send messages in the channel
- Check that `DISCORD_CHANNEL_ID` is correct
- Ensure bot is in the same server as the channel

**No quota alerts appearing:**
- Verify Redis connection is working
- Check that YouTube Listener has quota notifications enabled
- Ensure bot is subscribed to `quota:alerts` channel (check logs)

**Periodic updates not working:**
- Verify YouTube Listener API is accessible at `YOUTUBE_LISTENER_URL`
- Check `STATUS_UPDATE_INTERVAL` is a valid number in milliseconds
- Review bot logs for API fetch errors

## Logs

The bot logs important events to stdout:

```
✅ Connected to Redis
✅ Subscribed to quota:alerts channel
✅ Discord bot logged in
✅ Discord bot ready as QuotaMonitor#1234
Posting initial quota status...
Posted quota status to Discord
⏰ Scheduled periodic updates every 60 minutes
Received quota alert: state_changed
Posted quota event to Discord: state_changed
```

## License

MIT
