# Discord Quota Monitor - Quick Start Guide

Get your Discord bot up and running in 5 minutes!

## Step 1: Create Discord Bot

1. Go to [Discord Developer Portal](https://discord.com/developers/applications)
2. Click **"New Application"**
3. Give it a name (e.g., "All-Chat Quota Monitor")
4. Go to **"Bot"** tab on the left
5. Click **"Add Bot"**
6. Copy the **Bot Token** (you'll need this!)
7. Under **"Privileged Gateway Intents"**, you don't need any special intents enabled

## Step 2: Invite Bot to Your Server

1. Go to **"OAuth2"** → **"URL Generator"**
2. Select scopes:
   - ✅ `bot`
3. Select bot permissions:
   - ✅ `Send Messages`
   - ✅ `Embed Links`
   - ✅ `Read Message History`
4. Copy the generated URL and open it in your browser
5. Select your server and authorize the bot

## Step 3: Get Channel ID

1. In Discord, go to **User Settings** → **Advanced**
2. Enable **"Developer Mode"**
3. Right-click the channel where you want quota updates
4. Click **"Copy Channel ID"**

## Step 4: Configure Environment

Add these to your `.env` file:

```bash
DISCORD_BOT_TOKEN=YOUR_DISCORD_BOT_TOKEN_HERE
DISCORD_CHANNEL_ID=YOUR_CHANNEL_ID_HERE
STATUS_UPDATE_INTERVAL=3600000  # 1 hour (optional)
```

## Step 5: Run the Bot

### Local Development

```bash
cd services/discord-bot
npm install
npm start
```

### Docker Compose

```bash
# From project root
docker-compose up -d discord-bot
```

## Step 6: Verify It's Working

You should see:
1. ✅ Bot appears online in your server
2. 📊 Initial quota status message posted to your channel
3. 🔔 Real-time alerts when quota state changes

### Logs to Check

```bash
# Docker
docker logs allchat-discord-bot

# Local
# Check your terminal

# Look for:
✅ Connected to Redis
✅ Subscribed to quota:alerts channel
✅ Discord bot logged in
✅ Discord bot ready as QuotaMonitor#1234
Posted quota status to Discord
⏰ Scheduled periodic updates every 60 minutes
```

## Troubleshooting

### Bot is offline
- ❌ Check `DISCORD_BOT_TOKEN` is correct
- ❌ Verify bot wasn't deleted in Discord Developer Portal

### No messages appear
- ❌ Check `DISCORD_CHANNEL_ID` is correct
- ❌ Verify bot has permission to send messages in that channel
- ❌ Make sure bot was invited to the server

### No quota alerts
- ❌ Check Redis connection (logs will show "Connected to Redis")
- ❌ Verify YouTube Listener has quota notifications enabled
- ❌ Ensure YouTube Listener is running and processing quota events

### Can't find Channel ID
- ❌ Enable Developer Mode: Settings → Advanced → Developer Mode
- ❌ Right-click channel name (not in chat area) → Copy Channel ID

## What to Expect

### First Message (Immediate)
After starting, bot posts current quota status

### Periodic Updates (Every Hour)
Scheduled status summaries with:
- Current usage percentage
- Remaining quota
- Time until reset
- Top consuming channels

### Real-time Alerts (As They Happen)
Instant notifications for:
- 🟡 Quota state changes (HEALTHY → DEGRADED)
- 🔴 Threshold crossings (70%, 85%, 95%, 100%)
- 🚨 Quota exhausted/depleted
- ✅ Quota recovered

## Testing

### Trigger a Test Alert

Manually publish a test event to Redis:

```bash
redis-cli PUBLISH quota:alerts '{
  "type": "threshold_crossed",
  "timestamp": "2025-01-09T12:00:00Z",
  "global_state": "DEGRADED",
  "usage_percentage": 72.5,
  "units_used": 7250,
  "units_limit": 10000,
  "units_remaining": 2750,
  "message": "Test alert: Quota crossed 70% threshold",
  "severity": "warning"
}'
```

You should see a warning alert appear in Discord!

## Customization

### Change Update Frequency

In `.env`:
```bash
# Every 30 minutes
STATUS_UPDATE_INTERVAL=1800000

# Every 2 hours
STATUS_UPDATE_INTERVAL=7200000

# Every 6 hours
STATUS_UPDATE_INTERVAL=21600000
```

### Multiple Discord Channels

To post to multiple channels, run multiple instances with different `DISCORD_CHANNEL_ID` values.

## Next Steps

- ✅ Set up monitoring alerts for your team
- ✅ Create a dedicated #quota-alerts channel
- ✅ Adjust `STATUS_UPDATE_INTERVAL` based on your needs
- ✅ Monitor quota usage patterns and optimize API calls

## Support

If you run into issues:
1. Check the [full README](./README.md) for detailed documentation
2. Review bot logs for error messages
3. Verify all environment variables are set correctly
4. Test Redis connection: `redis-cli PING` should return `PONG`
