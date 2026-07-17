// IMPORTANT: Tracing must be initialized before any other imports for auto-instrumentation to work
import { initTracing } from './tracing.js';
initTracing();

import { Client, GatewayIntentBits, EmbedBuilder } from 'discord.js';
import { createClient } from 'redis';
import axios from 'axios';

// Configuration from environment
const DISCORD_TOKEN = process.env.DISCORD_BOT_TOKEN;
const DISCORD_CHANNEL_ID = process.env.DISCORD_CHANNEL_ID;
const REDIS_HOST = process.env.REDIS_HOST || 'localhost';
const REDIS_PORT = process.env.REDIS_PORT || 6379;
const YOUTUBE_LISTENER_URL = process.env.YOUTUBE_LISTENER_URL || 'http://localhost:8086';
const STATUS_UPDATE_INTERVAL = parseInt(process.env.STATUS_UPDATE_INTERVAL || '600000'); // 10 minutes default
const GRAFANA_PANEL_URL = process.env.GRAFANA_PANEL_URL || null; // Optional Grafana panel embed URL

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

// Store message IDs for editing instead of spamming
let statusMessageId = null;
let alertMessageId = null;

// Embed titles used to identify this bot's own quota messages when reclaiming
// the channel after a rollout, so only a single message of each kind stays
// visible (see reclaimMessage).
const QUOTA_STATUS_TITLE = '📊 YouTube API Quota Status';
const QUOTA_ALERT_TITLE_MARKER = 'Quota Alert:';

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
    .setTitle(QUOTA_STATUS_TITLE)
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

  // Add Grafana dashboard link if configured
  if (GRAFANA_PANEL_URL) {
    embed.setURL(GRAFANA_PANEL_URL);
    embed.addFields({
      name: '📊 Live Dashboard',
      value: `[View Interactive Grafana Dashboard](${GRAFANA_PANEL_URL})`,
      inline: false
    });
  }

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
  // Clamp percentage to 0-100 for display purposes
  const clampedPercentage = Math.max(0, Math.min(100, percentage));
  const filled = Math.floor(clampedPercentage / 5);
  const empty = 20 - filled;
  const bar = '█'.repeat(filled) + '░'.repeat(empty);

  // Show actual percentage (may be > 100%) in text
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
 * Posts or updates quota status to Discord channel
 * Edits existing message if available, otherwise creates new one
 */
async function postQuotaStatus() {
  try {
    console.log(`Fetching Discord channel: ${DISCORD_CHANNEL_ID}`);
    const channel = await discordClient.channels.fetch(DISCORD_CHANNEL_ID);
    if (!channel) {
      console.error('Discord channel not found');
      return;
    }
    console.log(`Channel found: ${channel.name} (type: ${channel.type})`);

    const quotaData = await fetchQuotaStatus();
    if (!quotaData) {
      console.error('Failed to fetch quota data');
      return;
    }
    console.log(`Quota data fetched: ${quotaData.global.used}/${quotaData.global.limit} (${quotaData.global.percentage.toFixed(2)}%)`);

    const embed = createQuotaEmbed(quotaData);

    // Try to edit existing message, or create new one if it doesn't exist
    if (statusMessageId) {
      console.log(`Attempting to edit existing message ID: ${statusMessageId}`);
      try {
        const message = await channel.messages.fetch(statusMessageId);
        await message.edit({ embeds: [embed] });
        console.log(`✅ Updated existing quota status message (ID: ${statusMessageId})`);
      } catch (error) {
        // Message was deleted or not found, create a new one
        console.error('Failed to edit existing message:', error.message, error.code);
        console.log('Status message not found, creating new one');
        try {
          const newMessage = await channel.send({ embeds: [embed] });
          statusMessageId = newMessage.id;
          console.log('Posted new quota status message');
        } catch (sendError) {
          console.error('Failed to send new message:', sendError.message, sendError.code, sendError.stack);
          throw sendError; // Re-throw to be caught by outer catch
        }
      }
    } else {
      // First time, create the message
      const newMessage = await channel.send({ embeds: [embed] });
      statusMessageId = newMessage.id;
      console.log(`Posted initial quota status message (ID: ${statusMessageId})`);
    }
  } catch (error) {
    console.error('❌ Error posting quota status:', error.message, error.code);
    console.error('Full error:', error);
  }
}

/**
 * Posts or updates quota event (alert) to Discord channel
 * Edits existing alert message if available, otherwise creates new one
 */
async function postQuotaEvent(event) {
  try {
    const channel = await discordClient.channels.fetch(DISCORD_CHANNEL_ID);
    if (!channel) {
      console.error('Discord channel not found');
      return;
    }

    const embed = createQuotaEventEmbed(event);

    // Try to edit existing alert message, or create new one if it doesn't exist
    if (alertMessageId) {
      try {
        const message = await channel.messages.fetch(alertMessageId);
        await message.edit({ embeds: [embed] });
        console.log(`Updated existing alert message with: ${event.type}`);
      } catch (error) {
        // Message was deleted or not found, create a new one
        console.log('Alert message not found, creating new one');
        const newMessage = await channel.send({ embeds: [embed] });
        alertMessageId = newMessage.id;
        console.log(`Posted new alert message: ${event.type}`);
      }
    } else {
      // First time, create the message
      const newMessage = await channel.send({ embeds: [embed] });
      alertMessageId = newMessage.id;
      console.log(`Posted initial alert message: ${event.type}`);
    }
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
 * Reclaims this bot's existing quota messages after a rollout.
 *
 * A rollout restarts the process, which clears the in-memory message IDs. Without
 * this, the bot would post a fresh message on every deploy and orphan the previous
 * one, piling up stale duplicates in the channel. This scans recent channel history
 * for the bot's own messages matching `matchesEmbed`, keeps the most recent one
 * (returned so postQuota* can edit it in place instead of sending a new one), and
 * deletes the rest so only a single message of that kind stays visible.
 *
 * @param {import('discord.js').TextBasedChannel} channel
 * @param {(embed: import('discord.js').Embed) => boolean} matchesEmbed
 * @param {string} label - human-readable name for logging
 * @returns {Promise<string|null>} id of the surviving message, or null if none exists
 */
async function reclaimMessage(channel, matchesEmbed, label) {
  try {
    // Own-message embeds are readable without the MessageContent intent, and a
    // bot may always delete its own messages, so no extra Discord perms needed.
    const recent = await channel.messages.fetch({ limit: 100 });
    const mine = [...recent.values()]
      .filter((msg) => msg.author.id === discordClient.user.id && msg.embeds.some(matchesEmbed))
      .sort((a, b) => b.createdTimestamp - a.createdTimestamp);

    if (mine.length === 0) {
      return null;
    }

    const [survivor, ...stale] = mine;
    for (const msg of stale) {
      try {
        await msg.delete();
        console.log(`🗑️  Removed stale ${label} message (ID: ${msg.id})`);
      } catch (error) {
        console.error(`Failed to remove stale ${label} message ${msg.id}:`, error.message);
      }
    }
    console.log(`♻️  Reusing existing ${label} message (ID: ${survivor.id}); removed ${stale.length} stale`);
    return survivor.id;
  } catch (error) {
    console.error(`Failed to reclaim ${label} messages:`, error.message);
    return null;
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

    // Set up Discord ready event listener BEFORE login to avoid race condition.
    // 'clientReady' is the v14.26+ name for the old 'ready' event (which is
    // deprecated and removed in v15).
    discordClient.once('clientReady', async () => {
      console.log(`✅ Discord bot ready as ${discordClient.user.tag}`);

      // This rollout cleared the in-memory message IDs, so reclaim the messages
      // this bot posted before the restart: keep the most recent status and alert
      // message and delete the rest, so only one of each stays visible instead of
      // a new duplicate accumulating on every deploy.
      try {
        const channel = await discordClient.channels.fetch(DISCORD_CHANNEL_ID);
        if (channel) {
          statusMessageId = await reclaimMessage(
            channel,
            (embed) => embed.title === QUOTA_STATUS_TITLE,
            'quota status'
          );
          alertMessageId = await reclaimMessage(
            channel,
            (embed) => (embed.title || '').includes(QUOTA_ALERT_TITLE_MARKER),
            'quota alert'
          );
        }
      } catch (error) {
        console.error('Failed to reclaim existing quota messages on startup:', error.message);
      }

      // Post initial status (edits the reclaimed message in place when present)
      console.log('Posting initial quota status...');
      await postQuotaStatus();

      // Set up periodic status updates
      setInterval(() => {
        console.log('Posting periodic quota status...');
        postQuotaStatus();
      }, STATUS_UPDATE_INTERVAL);

      console.log(`⏰ Scheduled periodic updates every ${STATUS_UPDATE_INTERVAL / 1000 / 60} minutes`);
    });

    // Login to Discord (ready event will fire after successful login)
    await discordClient.login(DISCORD_TOKEN);
    console.log('✅ Discord bot logged in');

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
