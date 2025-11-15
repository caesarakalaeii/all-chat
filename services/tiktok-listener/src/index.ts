/**
 * TikTok Listener Service
 *
 * Connects to TikTok LIVE streams using the unofficial TikTok-Live-Connector library
 * and publishes chat messages to Redis Streams for processing.
 *
 * IMPORTANT: This service uses an UNOFFICIAL library based on reverse engineering.
 * When TikTok releases an official Live Chat API, this should be replaced.
 *
 * Features:
 * - Monitors multiple TikTok live streams simultaneously
 * - Publishes raw messages to Redis Streams (chat:raw)
 * - Dynamic stream management (add/remove channels)
 * - Health check HTTP endpoint
 * - Graceful shutdown
 */

import { TikTokLiveConnection, WebcastEvent } from 'tiktok-live-connector';
import { createClient, RedisClientType } from 'redis';
import { Pool } from 'pg';
import winston from 'winston';
import { randomUUID } from 'crypto';
import http from 'http';

// Environment variables
const LOG_LEVEL = process.env.LOG_LEVEL || 'info';
const REDIS_HOST = process.env.REDIS_HOST || 'localhost';
const REDIS_PORT = parseInt(process.env.REDIS_PORT || '6379');
const DATABASE_HOST = process.env.DATABASE_HOST || 'localhost';
const DATABASE_PORT = parseInt(process.env.DATABASE_PORT || '5432');
const DATABASE_USER = process.env.DATABASE_USER || 'allchat';
const DATABASE_PASSWORD = process.env.DATABASE_PASSWORD || 'allchat_dev_password';
const DATABASE_NAME = process.env.DATABASE_NAME || 'allchat';
const HTTP_PORT = parseInt(process.env.PORT || '8089');
const POLL_INTERVAL_MS = parseInt(process.env.POLL_INTERVAL_MS || '30000'); // 30 seconds

// Configure logger
const logger = winston.createLogger({
  level: LOG_LEVEL,
  format: winston.format.combine(
    winston.format.timestamp(),
    winston.format.json()
  ),
  defaultMeta: { service: 'tiktok-listener' },
  transports: [
    new winston.transports.Console({
      format: winston.format.combine(
        winston.format.colorize(),
        winston.format.simple()
      )
    })
  ]
});

// Raw message format (matches YouTube/Twitch format)
interface RawChatMessage {
  message_id: string;
  platform: string;
  channel_id: string;
  stream_id?: string;
  user_id: string;
  username: string;
  text: string;
  timestamp: string; // ISO 8601
  tags: Record<string, string>;
}

// Active stream tracking
interface ActiveStream {
  username: string;
  overlay_id: string;
  connection: TikTokLiveConnection;
  is_connected: boolean;
}

class TikTokListenerService {
  private redis: RedisClientType;
  private db: Pool;
  private activeStreams: Map<string, ActiveStream> = new Map();
  private isShuttingDown = false;
  private pollTimer?: NodeJS.Timeout;
  private httpServer?: http.Server;

  constructor() {
    // Initialize Redis client
    this.redis = createClient({
      socket: {
        host: REDIS_HOST,
        port: REDIS_PORT
      }
    });

    // Initialize PostgreSQL pool
    this.db = new Pool({
      host: DATABASE_HOST,
      port: DATABASE_PORT,
      user: DATABASE_USER,
      password: DATABASE_PASSWORD,
      database: DATABASE_NAME,
      max: 10
    });
  }

  async start(): Promise<void> {
    logger.info('Starting TikTok Listener Service', {
      version: process.env.APP_VERSION || 'dev'
    });

    try {
      // Connect to Redis
      await this.redis.connect();
      logger.info('Connected to Redis', { host: REDIS_HOST, port: REDIS_PORT });

      // Test database connection
      await this.db.query('SELECT NOW()');
      logger.info('Connected to PostgreSQL', { host: DATABASE_HOST, port: DATABASE_PORT });

      // Start HTTP server for health checks
      this.startHttpServer();

      // Start polling for active streams
      this.startPolling();

      logger.info('TikTok Listener Service started successfully');
    } catch (error) {
      logger.error('Failed to start service', { error });
      throw error;
    }
  }

  private startHttpServer(): void {
    this.httpServer = http.createServer((req, res) => {
      if (req.url === '/health/live' && req.method === 'GET') {
        // Liveness probe
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ status: 'ok' }));
      } else if (req.url === '/health/ready' && req.method === 'GET') {
        // Readiness probe
        const isReady = this.redis.isReady && !this.isShuttingDown;
        res.writeHead(isReady ? 200 : 503, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({
          status: isReady ? 'ready' : 'not ready',
          active_streams: this.activeStreams.size
        }));
      } else if (req.url === '/status' && req.method === 'GET') {
        // Status endpoint
        const streams = Array.from(this.activeStreams.entries()).map(([key, stream]) => ({
          username: stream.username,
          overlay_id: stream.overlay_id,
          is_connected: stream.is_connected
        }));
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({
          active_streams_count: this.activeStreams.size,
          streams
        }));
      } else {
        res.writeHead(404);
        res.end();
      }
    });

    this.httpServer.listen(HTTP_PORT, () => {
      logger.info('HTTP server listening', { port: HTTP_PORT });
    });
  }

  private startPolling(): void {
    logger.info('Starting active stream polling', { interval_ms: POLL_INTERVAL_MS });

    // Poll immediately
    this.pollActiveStreams();

    // Then poll on interval
    this.pollTimer = setInterval(() => {
      this.pollActiveStreams();
    }, POLL_INTERVAL_MS);
  }

  private async pollActiveStreams(): Promise<void> {
    if (this.isShuttingDown) return;

    try {
      // Query database for active TikTok channels
      // This assumes there's a table tracking which TikTok usernames should be monitored
      const result = await this.db.query(`
        SELECT DISTINCT
          ocs.overlay_id,
          ocs.channel_id as tiktok_username,
          ocs.is_active
        FROM overlay_chat_sources ocs
        WHERE ocs.platform = 'tiktok'
          AND ocs.is_active = true
      `);

      const activeUsernames = new Map<string, string>(); // username -> overlay_id

      for (const row of result.rows) {
        const username = row.tiktok_username;
        const overlayId = row.overlay_id;
        activeUsernames.set(username, overlayId);
      }

      // Connect to new streams
      for (const [username, overlayId] of activeUsernames.entries()) {
        if (!this.activeStreams.has(username)) {
          await this.connectToStream(username, overlayId);
        }
      }

      // Disconnect from streams no longer active
      for (const [username, stream] of this.activeStreams.entries()) {
        if (!activeUsernames.has(username)) {
          await this.disconnectFromStream(username);
        }
      }

      logger.debug('Active streams poll complete', {
        total: activeUsernames.size,
        connected: this.activeStreams.size
      });
    } catch (error) {
      logger.error('Failed to poll active streams', { error });
    }
  }

  private async connectToStream(username: string, overlayId: string): Promise<void> {
    try {
      logger.info('Connecting to TikTok stream', { username, overlay_id: overlayId });

      const connection = new TikTokLiveConnection(username, {
        processInitialData: false, // Don't process historical messages
        enableExtendedGiftInfo: false
      });

      // Set up event handlers
      connection.on(WebcastEvent.CHAT, (data) => {
        this.handleChatMessage(username, overlayId, data);
      });

      connection.on('connected', (state: any) => {
        logger.info('TikTok stream connected', {
          username,
          room_id: state.roomId,
          overlay_id: overlayId
        });
        const stream = this.activeStreams.get(username);
        if (stream) {
          stream.is_connected = true;
        }
      });

      connection.on('disconnected', () => {
        logger.warn('TikTok stream disconnected', { username });
        const stream = this.activeStreams.get(username);
        if (stream) {
          stream.is_connected = false;
        }
      });

      connection.on('error', (err: any) => {
        logger.error('TikTok stream error', { username, error: err });
      });

      // Store before connecting
      this.activeStreams.set(username, {
        username,
        overlay_id: overlayId,
        connection,
        is_connected: false
      });

      // Connect
      await connection.connect();
    } catch (error) {
      logger.error('Failed to connect to TikTok stream', { username, error });
      this.activeStreams.delete(username);
    }
  }

  private async disconnectFromStream(username: string): Promise<void> {
    const stream = this.activeStreams.get(username);
    if (!stream) return;

    try {
      logger.info('Disconnecting from TikTok stream', { username });
      stream.connection.disconnect();
      this.activeStreams.delete(username);
    } catch (error) {
      logger.error('Failed to disconnect from TikTok stream', { username, error });
    }
  }

  private async handleChatMessage(username: string, overlayId: string, data: any): Promise<void> {
    try {
      // Create raw message in standardized format
      const rawMessage: RawChatMessage = {
        message_id: randomUUID(),
        platform: 'tiktok',
        channel_id: username,
        stream_id: undefined, // TikTok doesn't provide stream ID via unofficial lib
        user_id: data.user?.uniqueId || data.user?.userId || 'unknown',
        username: data.user?.nickname || data.user?.uniqueId || 'Anonymous',
        text: data.comment || '',
        timestamp: new Date().toISOString(),
        tags: {
          overlay_id: overlayId,
          user_unique_id: data.user?.uniqueId || '',
          profile_picture_url: data.user?.profilePictureUrl || '',
          is_follower: data.user?.isFollower?.toString() || 'false',
          is_subscriber: data.user?.isSubscriber?.toString() || 'false',
          badge_level: data.user?.badgeLevel?.toString() || '0'
        }
      };

      // Publish to Redis Stream
      await this.redis.xAdd('chat:raw', '*', {
        data: JSON.stringify(rawMessage)
      });

      logger.debug('Published TikTok chat message', {
        username,
        overlay_id: overlayId,
        user: rawMessage.username,
        message_id: rawMessage.message_id
      });
    } catch (error) {
      logger.error('Failed to handle chat message', { username, error });
    }
  }

  async stop(): Promise<void> {
    this.isShuttingDown = true;
    logger.info('Shutting down TikTok Listener Service...');

    // Stop polling
    if (this.pollTimer) {
      clearInterval(this.pollTimer);
    }

    // Disconnect all streams
    for (const [username, _] of this.activeStreams.entries()) {
      await this.disconnectFromStream(username);
    }

    // Close HTTP server
    if (this.httpServer) {
      await new Promise<void>((resolve) => {
        this.httpServer!.close(() => {
          logger.info('HTTP server closed');
          resolve();
        });
      });
    }

    // Close Redis connection
    await this.redis.quit();
    logger.info('Redis connection closed');

    // Close database pool
    await this.db.end();
    logger.info('Database connection closed');

    logger.info('Service shutdown complete');
  }
}

// Main
const service = new TikTokListenerService();

service.start().catch((error) => {
  logger.error('Failed to start service', { error });
  process.exit(1);
});

// Graceful shutdown
process.on('SIGINT', async () => {
  logger.info('Received SIGINT signal');
  await service.stop();
  process.exit(0);
});

process.on('SIGTERM', async () => {
  logger.info('Received SIGTERM signal');
  await service.stop();
  process.exit(0);
});
