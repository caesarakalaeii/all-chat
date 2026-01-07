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
import { Pool, Client, Notification } from 'pg';
import { randomUUID } from 'crypto';
import http from 'http';
import { EventEmitter } from 'events';

// Import new live detection modules
import { TikTokStatusChecker } from './livestream/status-checker.js';
import { BackoffManager } from './livestream/backoff-manager.js';
import { LiveStreamPoller } from './livestream/poller.js';
import { PrometheusMetrics } from './metrics/prometheus.js';
import { HeartbeatMonitor } from './reliability/heartbeat-monitor.js';
import { MessageDeduplicator } from './deduplication/message-deduplicator.js';

// Environment variables
const LOG_LEVEL = process.env.LOG_LEVEL || 'info';
const LOG_FORMAT = process.env.LOG_FORMAT || 'json'; // 'json' or 'simple'
const REDIS_HOST = process.env.REDIS_HOST || 'localhost';
const REDIS_PORT = parseInt(process.env.REDIS_PORT || '6379');
const DATABASE_HOST = process.env.DATABASE_HOST || 'localhost';
const DATABASE_PORT = parseInt(process.env.DATABASE_PORT || '5432');
const DATABASE_USER = process.env.DATABASE_USER || 'allchat';
const DATABASE_PASSWORD = process.env.DATABASE_PASSWORD || 'allchat_dev_password';
const DATABASE_NAME = process.env.DATABASE_NAME || 'allchat';
const HTTP_PORT = parseInt(process.env.PORT || '8089');
const POLL_INTERVAL_MS = parseInt(process.env.POLL_INTERVAL_MS || '30000'); // 30 seconds

// New TikTok live detection configuration
const TIKTOK_STATUS_CHECK_CACHE_TTL_MS = parseInt(process.env.TIKTOK_STATUS_CHECK_CACHE_TTL_MS || '10000');
const TIKTOK_POLLER_INTERVAL_MS = parseInt(process.env.TIKTOK_POLLER_INTERVAL_MS || '30000');
const TIKTOK_BASE_OFFLINE_BACKOFF_MS = parseInt(process.env.TIKTOK_BASE_OFFLINE_BACKOFF_MS || '60000');
const TIKTOK_MAX_OFFLINE_BACKOFF_MS = parseInt(process.env.TIKTOK_MAX_OFFLINE_BACKOFF_MS || '600000');
const TIKTOK_ERROR_BACKOFF_MS = parseInt(process.env.TIKTOK_ERROR_BACKOFF_MS || '2000');
const TIKTOK_MAX_ERROR_BACKOFF_MS = parseInt(process.env.TIKTOK_MAX_ERROR_BACKOFF_MS || '300000');
const TIKTOK_HEARTBEAT_INTERVAL_MS = parseInt(process.env.TIKTOK_HEARTBEAT_INTERVAL_MS || '30000');
const TIKTOK_HEARTBEAT_TIMEOUT_MS = parseInt(process.env.TIKTOK_HEARTBEAT_TIMEOUT_MS || '90000');

// Message deduplication configuration
const TIKTOK_DEDUP_TTL_MS = parseInt(process.env.TIKTOK_DEDUP_TTL_MS || '300000'); // 5 minutes
const TIKTOK_DEDUP_CLEANUP_INTERVAL_MS = parseInt(process.env.TIKTOK_DEDUP_CLEANUP_INTERVAL_MS || '60000'); // 1 minute
const TIKTOK_DEDUP_MAX_CACHE_SIZE = parseInt(process.env.TIKTOK_DEDUP_MAX_CACHE_SIZE || '10000');

// Import logger interface
import { Logger } from './types/logger.js';

// Simple, reliable logger using console (Winston was causing issues)
// Outputs JSON format for log aggregation
const createLogger = (): Logger => {
  const _log = (level: string, message: string, meta?: any) => {
    const logEntry = {
      timestamp: new Date().toISOString(),
      level,
      service: 'tiktok-listener',
      version: process.env.APP_VERSION || 'dev',
      message,
      ...(meta || {})
    };

    const output = JSON.stringify(logEntry);
    if (level === 'error') {
      console.error(output);
    } else {
      console.log(output);
    }
  };

  const logLevel = LOG_LEVEL.toLowerCase();

  return {
    error: (message: string, meta?: any) => _log('error', message, meta),
    warn: (message: string, meta?: any) => _log('warn', message, meta),
    info: (message: string, meta?: any) => {
      if (logLevel === 'info' || logLevel === 'debug') {
        _log('info', message, meta);
      }
    },
    debug: (message: string, meta?: any) => {
      if (logLevel === 'debug') {
        _log('debug', message, meta);
      }
    }
  };
};

const logger = createLogger();

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
  private listenClient?: Client; // Dedicated client for PostgreSQL LISTEN
  private activeStreams: Map<string, ActiveStream> = new Map();
  private isShuttingDown = false;
  private pollTimer?: NodeJS.Timeout;
  private httpServer?: http.Server;

  // New live detection modules
  private statusChecker: TikTokStatusChecker;
  private backoffManager: BackoffManager;
  private livePoller: LiveStreamPoller;

  // Prometheus metrics
  private metrics: PrometheusMetrics;

  // Heartbeat monitoring
  private heartbeatMonitor: HeartbeatMonitor;

  // Message deduplication
  private messageDeduplicator: MessageDeduplicator;

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

    // Initialize live detection modules
    this.statusChecker = new TikTokStatusChecker(logger, TIKTOK_STATUS_CHECK_CACHE_TTL_MS);
    this.backoffManager = new BackoffManager(logger, {
      baseOfflineBackoffMs: TIKTOK_BASE_OFFLINE_BACKOFF_MS,
      maxOfflineBackoffMs: TIKTOK_MAX_OFFLINE_BACKOFF_MS,
      errorBackoffMs: TIKTOK_ERROR_BACKOFF_MS,
      maxErrorBackoffMs: TIKTOK_MAX_ERROR_BACKOFF_MS
    });
    this.livePoller = new LiveStreamPoller(
      this.statusChecker,
      this.backoffManager,
      logger,
      { pollIntervalMs: TIKTOK_POLLER_INTERVAL_MS }
    );

    // Set up callback for when poller detects a live stream
    this.livePoller.setOnLiveCallback(async (username: string, overlayId: string) => {
      await this.connectToStream(username, overlayId);
    });

    // Initialize Prometheus metrics
    this.metrics = new PrometheusMetrics(logger);

    // Initialize heartbeat monitor
    this.heartbeatMonitor = new HeartbeatMonitor(
      logger,
      this.metrics,
      TIKTOK_HEARTBEAT_INTERVAL_MS,
      TIKTOK_HEARTBEAT_TIMEOUT_MS
    );

    // Initialize message deduplicator
    this.messageDeduplicator = new MessageDeduplicator(logger, {
      ttlMs: TIKTOK_DEDUP_TTL_MS,
      cleanupIntervalMs: TIKTOK_DEDUP_CLEANUP_INTERVAL_MS,
      maxCacheSize: TIKTOK_DEDUP_MAX_CACHE_SIZE
    });
  }

  async start(): Promise<void> {
    logger.info('Starting TikTok Listener Service', {
      version: process.env.APP_VERSION || 'dev',
      live_detection_enabled: true
    });

    try {
      // Connect to Redis
      await this.redis.connect();
      logger.info('Connected to Redis', { host: REDIS_HOST, port: REDIS_PORT });

      // Start live stream poller
      this.livePoller.start();
      logger.info('Live stream poller started');

      // Start message deduplicator cleanup
      this.messageDeduplicator.start();
      logger.info('Message deduplicator started');

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
          streams,
          deduplication: this.messageDeduplicator.getStats()
        }));
      } else if (req.url === '/metrics' && req.method === 'GET') {
        // Prometheus metrics endpoint
        this.metrics.getMetrics().then((metrics) => {
          res.writeHead(200, { 'Content-Type': this.metrics.getContentType() });
          res.end(metrics);
        }).catch((error) => {
          logger.error('Failed to generate metrics', { error });
          res.writeHead(500, { 'Content-Type': 'text/plain' });
          res.end('Error generating metrics');
        });
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

    // Start PostgreSQL LISTEN for instant notifications
    this.startDatabaseListener();
  }

  private async startDatabaseListener(): Promise<void> {
    try {
      // Create dedicated client for LISTEN (Pool doesn't support notifications)
      this.listenClient = new Client({
        host: DATABASE_HOST,
        port: DATABASE_PORT,
        user: DATABASE_USER,
        password: DATABASE_PASSWORD,
        database: DATABASE_NAME
      });

      await this.listenClient.connect();

      // Listen for chat source changes
      await this.listenClient.query('LISTEN chat_source_changes');

      logger.info('PostgreSQL LISTEN active for instant source updates', {
        channel: 'chat_source_changes'
      });

      // Set up notification handler with proper typing
      this.listenClient.on('notification', (msg: Notification) => {
        if (msg.channel === 'chat_source_changes') {
          logger.info('Source change notification received', {
            payload: msg.payload
          });

          // Trigger immediate sync
          this.pollActiveStreams().catch(err => {
            logger.error('Failed to sync after notification', { error: err });
          });
        }
      });

      // Handle connection errors
      this.listenClient.on('error', (err: Error) => {
        logger.error('PostgreSQL LISTEN connection error', { error: err.message });
      });

      // Handle unexpected disconnection
      this.listenClient.on('end', () => {
        logger.warn('PostgreSQL LISTEN connection ended, will rely on polling');
        this.listenClient = undefined;
      });

    } catch (error) {
      logger.error('Failed to start PostgreSQL LISTEN', { error });
      logger.info('Will rely on periodic polling only');
    }
  }

  private async pollActiveStreams(): Promise<void> {
    if (this.isShuttingDown) return;

    try {
      // Get list of overlays with active WebSocket connections from Redis
      const connectedOverlays = await this.redis.sMembers('overlay:connected');

      if (connectedOverlays.length === 0) {
        logger.debug('No overlays with active connections, skipping poll');

        // Disconnect all streams since no one is watching
        for (const [username, _] of this.activeStreams.entries()) {
          await this.disconnectFromStream(username);
        }

        return;
      }

      logger.debug('Polling for active TikTok streams', {
        connected_overlays_count: connectedOverlays.length
      });

      // Query database for TikTok channels that belong to overlays with active connections
      const result = await this.db.query(`
        SELECT DISTINCT
          ocs.overlay_id,
          ocs.channel_id as tiktok_username,
          ocs.is_active
        FROM overlay_chat_sources ocs
        WHERE ocs.platform = 'tiktok'
          AND ocs.overlay_id = ANY($1::uuid[])
      `, [connectedOverlays]);

      const activeUsernames = new Map<string, string>(); // username -> overlay_id

      for (const row of result.rows) {
        const username = row.tiktok_username;
        const overlayId = row.overlay_id;
        activeUsernames.set(username, overlayId);
      }

      logger.debug('Found TikTok sources for connected overlays', {
        source_count: activeUsernames.size
      });

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

      // Update status for all connected streams to keep updated_at fresh
      // This prevents the 5-minute stale cleanup from marking them inactive
      let statusUpdates = 0;
      for (const [username, stream] of this.activeStreams.entries()) {
        if (stream.is_connected) {
          await this.setSourceActive(username, true);
          statusUpdates++;
        }
      }

      logger.debug('Active streams poll complete', {
        total: activeUsernames.size,
        connected: this.activeStreams.size,
        status_updates: statusUpdates
      });
    } catch (error) {
      logger.error('Failed to poll active streams', { error });
    }
  }

  private async connectToStream(username: string, overlayId: string): Promise<void> {
    try {
      logger.info('Pre-checking live status before connection', { username, overlay_id: overlayId });

      // NEW: Pre-check if user is live before attempting connection
      const statusResult = await this.statusChecker.checkLiveStatus(username);

      if (!statusResult.isLive) {
        logger.info('User not live, adding to poller and skipping connection', {
          username,
          overlay_id: overlayId
        });

        // Add to polling targets
        this.livePoller.addTarget(username, overlayId);

        // Record offline check for backoff
        this.backoffManager.recordOfflineCheck(username);

        // Store in activeStreams as "pending" so we don't retry immediately
        // (but no connection object)
        return;
      }

      logger.info('User is live, proceeding with connection', { username, overlay_id: overlayId });

      const connection = new TikTokLiveConnection(username, {
        processInitialData: false, // Don't process historical messages
        enableExtendedGiftInfo: false
      });

      // Set up event handlers
      connection.on(WebcastEvent.CHAT, (data) => {
        this.handleChatMessage(username, overlayId, data);
      });

      // Cast to EventEmitter for lifecycle events not in ClientEventMap
      const emitter = connection as unknown as EventEmitter;

      emitter.on('connected', (state: { roomId?: string }) => {
        logger.info('TikTok stream connected', {
          username,
          room_id: state.roomId,
          overlay_id: overlayId
        });
        const stream = this.activeStreams.get(username);
        if (stream) {
          stream.is_connected = true;
        }

        // SUCCESS: Reset backoff and remove from poller
        this.backoffManager.recordSuccessfulConnection(username);
        this.livePoller.removeTarget(username);

        // Start heartbeat monitoring
        this.heartbeatMonitor.start(username, connection);

        // Update database: stream is live and source is active
        this.updateStreamHistory(username, true);
        this.setSourceActive(username, true);
      });

      emitter.on('disconnected', () => {
        logger.warn('TikTok stream disconnected', { username });
        const stream = this.activeStreams.get(username);
        if (stream) {
          stream.is_connected = false;
        }

        // Stop heartbeat monitoring
        this.heartbeatMonitor.stop(username);

        // Stream ended - reset to quick re-check
        this.backoffManager.recordDisconnection(username);
        this.livePoller.addTarget(username, overlayId);

        // Update database: stream went offline (but keep source active for other overlays)
        this.updateStreamHistory(username, false);
        // Don't deactivate source - multiple overlays may share this channel
      });

      emitter.on('error', (err: Error) => {
        logger.error('TikTok stream error', { username, error: err });

        // Connection error - record for backoff
        this.backoffManager.recordConnectionError(username, err);
        this.livePoller.addTarget(username, overlayId);
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

      // Record error for backoff
      this.backoffManager.recordConnectionError(username, error as Error);

      // Add to poller for retry
      this.livePoller.addTarget(username, overlayId);

      // Don't delete from activeStreams - keep it for tracking
    }
  }

  private async disconnectFromStream(username: string): Promise<void> {
    const stream = this.activeStreams.get(username);
    if (!stream) return;

    try {
      logger.info('Disconnecting from TikTok stream', { username });
      stream.connection.disconnect();
      this.activeStreams.delete(username);

      // Remove from poller and clear backoff state
      this.livePoller.removeTarget(username);
      this.backoffManager.removeState(username);
      this.statusChecker.clearCache(username);
    } catch (error) {
      logger.error('Failed to disconnect from TikTok stream', { username, error });
    }
  }

  private async updateStreamHistory(username: string, isLive: boolean): Promise<void> {
    try {
      await this.db.query(
        `SELECT update_stream_history_on_detection($1, $2, $3, $4)`,
        ['tiktok', username, username, isLive]
      );
      logger.debug('Updated stream history', { username, is_live: isLive });
    } catch (error) {
      logger.error('Failed to update stream history', { username, error });
    }
  }

  private async setSourceActive(username: string, isActive: boolean): Promise<void> {
    try {
      await this.db.query(
        `UPDATE overlay_chat_sources
         SET is_active = $1, updated_at = NOW()
         WHERE platform = 'tiktok' AND channel_id = $2`,
        [isActive, username]
      );
      logger.debug('Updated source active status', { username, is_active: isActive });
    } catch (error) {
      logger.error('Failed to update source active status', { username, error });
    }
  }

  private async setSourceActiveByOverlay(overlayId: string, username: string, isActive: boolean): Promise<void> {
    try {
      await this.db.query(
        `UPDATE overlay_chat_sources
         SET is_active = $1, updated_at = NOW()
         WHERE platform = 'tiktok' AND overlay_id = $2 AND channel_id = $3`,
        [isActive, overlayId, username]
      );
      logger.debug('Updated overlay-specific source active status', { overlay_id: overlayId, username, is_active: isActive });
    } catch (error) {
      logger.error('Failed to update overlay-specific source active status', { overlay_id: overlayId, username, error });
    }
  }

  private async handleChatMessage(username: string, overlayId: string, data: any): Promise<void> {
    try {
      // Record message for heartbeat monitoring
      this.heartbeatMonitor.recordMessage(username);

      // Extract TikTok's native message ID and timestamp
      const msgId = data.common?.msgId;
      const createTime = data.common?.createTime;
      
      // Convert TikTok timestamp (Unix timestamp in string format, usually in seconds)
      // to ISO 8601 format
      let timestamp: string;
      if (createTime) {
        // TikTok createTime is typically Unix timestamp in seconds (as string)
        const timestampMs = parseInt(createTime) * 1000;
        timestamp = new Date(timestampMs).toISOString();
      } else {
        // Fallback to current time if no timestamp provided
        timestamp = new Date().toISOString();
        logger.warn('Message without createTime, using current time', {
          username,
          msg_id: msgId
        });
      }

      // Check for duplicate messages (prevents replay on reconnect)
      if (this.messageDeduplicator.isDuplicate(msgId, username, data.comment || '')) {
        logger.debug('Skipping duplicate message', {
          msg_id: msgId,
          username,
          text_preview: (data.comment || '').substring(0, 50)
        });
        return; // Skip publishing duplicate
      }

      // Create raw message in standardized format
      const rawMessage: RawChatMessage = {
        message_id: msgId || randomUUID(), // Use TikTok's msgId, fallback to UUID
        platform: 'tiktok',
        channel_id: username,
        stream_id: undefined, // TikTok doesn't provide stream ID via unofficial lib
        user_id: data.user?.uniqueId || data.user?.userId || 'unknown',
        username: data.user?.nickname || data.user?.uniqueId || 'Anonymous',
        text: data.comment || '',
        timestamp: timestamp, // Use TikTok's native timestamp
        tags: {
          overlay_id: overlayId,
          user_unique_id: data.user?.uniqueId || '',
          profile_picture_url: data.user?.profilePictureUrl || '',
          is_follower: data.user?.isFollower?.toString() || 'false',
          is_subscriber: data.user?.isSubscriber?.toString() || 'false',
          badge_level: data.user?.badgeLevel?.toString() || '0',
          native_msg_id: msgId || '', // Store native ID for reference
          native_create_time: createTime || '' // Store native timestamp for reference
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
        message_id: rawMessage.message_id,
        native_timestamp: timestamp
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

    // Stop live stream poller
    this.livePoller.stop();
    logger.info('Live stream poller stopped');

    // Stop all heartbeat monitoring
    this.heartbeatMonitor.stopAll();
    logger.info('Heartbeat monitoring stopped');

    // Stop message deduplicator cleanup
    this.messageDeduplicator.stop();
    logger.info('Message deduplicator stopped');

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

    // Close LISTEN client
    if (this.listenClient) {
      await this.listenClient.end();
      logger.info('PostgreSQL LISTEN connection closed');
    }

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
