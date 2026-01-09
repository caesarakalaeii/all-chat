import { Client, GatewayIntentBits, EmbedBuilder } from 'discord.js';
import { createClient } from 'redis';
import axios from 'axios';

// Configuration from environment
const DISCORD_TOKEN = process.env.DISCORD_BOT_TOKEN;
const DISCORD_CHANNEL_ID = process.env.DISCORD_CHANNEL_ID;
const REDIS_HOST = process.env.REDIS_HOST || 'localhost';
const REDIS_PORT = process.env.REDIS_PORT || 6379;
const YOUTUBE_LISTENER_URL = process.env.YOUTUBE_LISTENER_URL || 'http://localhost:8086';
const STATUS_UPDATE_INTERVAL = parseInt(process.env.STATUS_UPDATE_INTERVAL || '3600000'); // 1 hour default

// Validate required env vars
if (!DISCORD_TOKEN || !DISCORD_CHANNEL_ID) {
  console.error('Missing required environment variables: DISCORD_BOT_TOKEN, DISCORD_CHANNEL_ID');
  process.exit(1);
}

// Initialize Discord client
const discordClient = new Client({
  intents: [GatewayIntentBits.Guilds]
});

// Initialize Redis clients (separate for pub/sub and regular commands)
const redisSubscriber = createClient({
  socket: {
    host: REDIS_HOST,
    port: REDIS_PORT
  }
});

const redisClient = createClient({
  socket: {
    host: REDIS_HOST,
    port: REDIS_PORT
  }
});

// Error handlers
redisSubscriber.on('error', (err) => console.error('Redis Subscriber Error:', err));
redisClient.on('error', (err) => console.error('Redis Client Error:', err));

/**
 * Creates a Discord embed for quota status
 */
function createQuotaEmbed(data) {
  const { global, channels = [] } = data;
  const { state, used, limit, remaining, percentage, resets_at, polling_multiplier } = global;

  // Determine color based on state
  const colors = {
    HEALTHY: 0x00ff00,    // Green
    DEGRADED: 0xffff00,   // Yellow
    CRITICAL: 0xff9900,   // Orange
    EXHAUSTED: 0xff0000,  // Red
    DEPLETED: 0x8b0000    // Dark Red
  };
  const color = colors[state] || 0x808080;

  // Create progress bar
  const progressBar = createProgressBar(percentage);

  // Format resets at time
  const resetsAt = new Date(resets_at);
  const resetsAtFormatted = `<t:${Math.floor(resetsAt.getTime() / 1000)}:R>`;

  const embed = new EmbedBuilder()
    .setTitle('📊 YouTube API Quota Status')
    .setColor(color)
    .setDescription(`**Current State:** ${getStateEmoji(state)} ${state}`)
    .addFields(
      {
        name: '📈 Usage',
        value: `\`\`\`${used.toLocaleString()} / ${limit.toLocaleString()} units (${percentage.toFixed(2)}%)\`\`\``,
        inline: false
      },
      {
        name: 'Progress',
        value: progressBar,
        inline: false
      },
      {
        name: '⏱️ Remaining',
        value: `${remaining.toLocaleString()} units`,
        inline: true
      },
      {
        name: '🔄 Resets',
        value: resetsAtFormatted,
        inline: true
      },
      {
        name: '⚡ Polling Speed',
        value: `${polling_multiplier.toFixed(2)}x`,
        inline: true
      }
    )
    .setTimestamp()
    .setFooter({ text: 'All-Chat Quota Monitor' });

  // Add top consuming channels if available
  if (channels && channels.length > 0) {
    const topChannels = channels
      .slice(0, 5)
      .map((ch, idx) => `${idx + 1}. \`${ch.channel_id}\` - ${ch.used} units (${ch.percentage.toFixed(1)}%)`)
      .join('\n');

    embed.addFields({
      name: '🔝 Top Consuming Channels',
      value: topChannels || 'No channels tracked',
      inline: false
    });
  }

  return embed;
}

/**
 * Creates a Discord embed for quota events (alerts)
 */
function createQuotaEventEmbed(event) {
  const {
    type,
    global_state,
    usage_percentage,
    units_used,
    units_limit,
    units_remaining,
    message,
    severity,
    affected_channels = []
  } = event;

  // Determine color based on severity
  const colors = {
    info: 0x0099ff,      // Blue
    warning: 0xffff00,   // Yellow
    error: 0xff0000,     // Red
    critical: 0x8b0000   // Dark Red
  };
  const color = colors[severity] || 0x808080;

  // Determine emoji based on event type
  const eventEmojis = {
    state_changed: '🔄',
    threshold_crossed: '⚠️',
    quota_exhausted: '🚨',
    quota_depleted: '❌',
    quota_recovered: '✅',
    channel_quota_exceeded: '📢'
  };
  const emoji = eventEmojis[type] || '📊';

  const embed = new EmbedBuilder()
    .setTitle(`${emoji} Quota Alert: ${formatEventType(type)}`)
    .setColor(color)
    .setDescription(message)
    .addFields(
      {
        name: 'State',
        value: `${getStateEmoji(global_state)} ${global_state}`,
        inline: true
      },
      {
        name: 'Usage',
        value: `${usage_percentage.toFixed(2)}%`,
        inline: true
      },
      {
        name: 'Remaining',
        value: `${units_remaining.toLocaleString()} units`,
        inline: true
      }
    )
    .setTimestamp()
    .setFooter({ text: `Severity: ${severity.toUpperCase()}` });

  // Add affected channels for channel-specific events
  if (affected_channels.length > 0) {
    embed.addFields({
      name: '📺 Affected Channels',
      value: affected_channels.slice(0, 10).map(ch => `• ${ch}`).join('\n'),
      inline: false
    });
  }

  return embed;
}

/**
 * Creates a progress bar visualization
 */
function createProgressBar(percentage) {
  const filled = Math.floor(percentage / 5);
  const empty = 20 - filled;
  const bar = '█'.repeat(filled) + '░'.repeat(empty);
  return `[${bar}] ${percentage.toFixed(1)}%`;
}

/**
 * Gets emoji for quota state
 */
function getStateEmoji(state) {
  const emojis = {
    HEALTHY: '🟢',
    DEGRADED: '🟡',
    CRITICAL: '🟠',
    EXHAUSTED: '🔴',
    DEPLETED: '⛔'
  };
  return emojis[state] || '⚪';
}

/**
 * Formats event type for display
 */
function formatEventType(type) {
  return type
    .split('_')
    .map(word => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ');
}

/**
 * Fetches quota status from YouTube Listener API
 */
async function fetchQuotaStatus() {
  try {
    const response = await axios.get(`${YOUTUBE_LISTENER_URL}/quota/status`, {
      timeout: 5000
    });
    return response.data;
  } catch (error) {
    console.error('Failed to fetch quota status:', error.message);
    return null;
  }
}

/**
 * Posts quota status to Discord channel
 */
async function postQuotaStatus() {
  try {
    const channel = await discordClient.channels.fetch(DISCORD_CHANNEL_ID);
    if (!channel) {
      console.error('Discord channel not found');
      return;
    }

    const quotaData = await fetchQuotaStatus();
    if (!quotaData) {
      console.error('Failed to fetch quota data');
      return;
    }

    const embed = createQuotaEmbed(quotaData);
    await channel.send({ embeds: [embed] });
    console.log('Posted quota status to Discord');
  } catch (error) {
    console.error('Error posting quota status:', error);
  }
}

/**
 * Posts quota event (alert) to Discord channel
 */
async function postQuotaEvent(event) {
  try {
    const channel = await discordClient.channels.fetch(DISCORD_CHANNEL_ID);
    if (!channel) {
      console.error('Discord channel not found');
      return;
    }

    const embed = createQuotaEventEmbed(event);
    await channel.send({ embeds: [embed] });
    console.log(`Posted quota event to Discord: ${event.type}`);
  } catch (error) {
    console.error('Error posting quota event:', error);
  }
}

/**
 * Handle Redis quota alert messages
 */
async function handleQuotaAlert(message) {
  try {
    const event = JSON.parse(message);
    console.log('Received quota alert:', event.type);
    await postQuotaEvent(event);
  } catch (error) {
    console.error('Error handling quota alert:', error);
  }
}

/**
 * Start the bot
 */
async function start() {
  try {
    // Connect to Redis
    await redisClient.connect();
    await redisSubscriber.connect();
    console.log('✅ Connected to Redis');

    // Subscribe to quota alerts channel
    await redisSubscriber.subscribe('quota:alerts', handleQuotaAlert);
    console.log('✅ Subscribed to quota:alerts channel');

    // Login to Discord
    await discordClient.login(DISCORD_TOKEN);
    console.log('✅ Discord bot logged in');

    // Wait for Discord to be ready
    discordClient.once('ready', async () => {
      console.log(`✅ Discord bot ready as ${discordClient.user.tag}`);

      // Post initial status
      console.log('Posting initial quota status...');
      await postQuotaStatus();

      // Set up periodic status updates
      setInterval(() => {
        console.log('Posting periodic quota status...');
        postQuotaStatus();
      }, STATUS_UPDATE_INTERVAL);

      console.log(`⏰ Scheduled periodic updates every ${STATUS_UPDATE_INTERVAL / 1000 / 60} minutes`);
    });

  } catch (error) {
    console.error('Failed to start bot:', error);
    process.exit(1);
  }
}

/**
 * Graceful shutdown
 */
async function shutdown() {
  console.log('Shutting down...');

  try {
    await redisSubscriber.unsubscribe('quota:alerts');
    await redisSubscriber.quit();
    await redisClient.quit();
    discordClient.destroy();
    console.log('✅ Graceful shutdown complete');
  } catch (error) {
    console.error('Error during shutdown:', error);
  }

  process.exit(0);
}

// Handle shutdown signals
process.on('SIGINT', shutdown);
process.on('SIGTERM', shutdown);

// Start the bot
start();
