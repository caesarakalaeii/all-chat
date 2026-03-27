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

// IMPORTANT: Tracing must be initialized before any other imports for auto-instrumentation to work
import { initTracing, shutdownTracing } from './tracing.js';
initTracing();

import { TikTokLiveConnection, WebcastEvent } from 'tiktok-live-connector';
import { createClient, RedisClientType } from 'redis';
import { Pool } from 'pg';
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

// Import coordinator modules (Phase 6)
import { CoordinatorClient } from './coordination/client.js';
import { MigrationSubscriber } from './coordination/subscriber.js';
import { MigrationEvent, Assignment } from './coordination/models.js';

// Import demand subscriber (Phase 5)
import { DemandSubscriber, DemandSource } from './demand/subscriber.js';

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
const DEMAND_SAFETY_INTERVAL_MS = parseInt(process.env.DEMAND_SAFETY_INTERVAL_MS || '60000'); // 60 seconds

// Coordinator configuration (Phase 6)
const COORDINATOR_URL = process.env.COORDINATOR_URL || 'http://source-manager:8088';
const SERVICE_JWT_SECRET = process.env.SERVICE_JWT_SECRET || '';
const POD_NAME = process.env.HOSTNAME || 'tiktok-listener-unknown';
const HEARTBEAT_INTERVAL_MS = parseInt(process.env.HEARTBEAT_INTERVAL_MS || '10000'); // 10 seconds

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
  event_type?: string; // "gift", "follow", "like_aggregate", "share", etc.
  event_data?: Record<string, any>; // Event-specific payload
}

// Like aggregation window for tracking likes over 30-second periods
interface LikeAggregation {
  aggregation_id: string;
  username: string; // Channel username
  overlay_id: string;
  user_id: string;
  user_nickname: string;
  like_count: number;
  window_start: Date;
  last_published: Date;
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

  // Like aggregation tracking
  private likeAggregations: Map<string, LikeAggregation> = new Map();
  private likeAggregationTimer?: NodeJS.Timeout;
  private readonly LIKE_AGGREGATION_WINDOW_MS = 30000; // 30 seconds
  private readonly LIKE_PUBLISH_INTERVAL_MS = 5000; // Publish updates every 5 seconds

  // Connection state tracking to prevent concurrent connections
  private connectingStreams: Set<string> = new Set();

  // Coordinator integration (Phase 6)
  private coordinatorClient?: CoordinatorClient;
  private migrationSubscriber?: MigrationSubscriber;
  private assignedSourceIDs: Set<string> = new Set(); // source_id set for demand filtering
  private filteredAssignmentCount: number = 0; // Number of assigned sources that have database channels
  private heartbeatTimer?: NodeJS.Timeout;
  private firstMessageCallbacks: Map<string, () => void> = new Map(); // username -> callback

  // Demand subscriber (Phase 5)
  private demandSubscriber: DemandSubscriber | null = null;
  private demandSafetyInterval: ReturnType<typeof setInterval> | null = null;
  private livePollerRunning: boolean = false;

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
      live_detection_enabled: true,
      coordination_enabled: !!SERVICE_JWT_SECRET
    });

    try {
      // Connect to Redis
      await this.redis.connect();
      logger.info('Connected to Redis', { host: REDIS_HOST, port: REDIS_PORT });

      // Test database connection
      await this.db.query('SELECT NOW()');
      logger.info('Connected to PostgreSQL', { host: DATABASE_HOST, port: DATABASE_PORT });

      // Initialize coordinator integration (TIKTOK-01)
      if (SERVICE_JWT_SECRET) {
        logger.info('Coordinator integration enabled', {
          coordinator_url: COORDINATOR_URL,
          pod_name: POD_NAME
        });

        this.coordinatorClient = new CoordinatorClient(COORDINATOR_URL, SERVICE_JWT_SECRET, logger);

        // Start heartbeat publisher FIRST so source-manager includes this pod in its
        // next assignment cycle before we query for assignments.
        this.startHeartbeatPublisher();
        logger.info('Heartbeat publisher started', { interval_ms: HEARTBEAT_INTERVAL_MS });

        // Wait for at least one source-manager assignment cycle (~30s) plus jitter to
        // prevent thundering herd. This ensures the pod is in the heartbeat registry
        // when it queries, so the source-manager has already assigned sources to it.
        const jitterMs = 35000 + Math.floor(Math.random() * 15000); // 35-50 seconds
        logger.info('Waiting for source-manager assignment cycle before querying', { jitterMs });
        await new Promise(resolve => setTimeout(resolve, jitterMs));

        // Query assignments from coordinator (blocks indefinitely with backoff)
        const assignments = await this.coordinatorClient.queryAssignments(POD_NAME);

        logger.info('Received assignments from coordinator', {
          count: assignments.length,
          pod_id: POD_NAME
        });

        // Extract assigned source IDs into set for demand filtering (TIKTOK-02)
        for (const assignment of assignments) {
          this.assignedSourceIDs.add(assignment.source_id);
        }

        // Start migration subscriber (TIKTOK-03, TIKTOK-04)
        this.migrationSubscriber = new MigrationSubscriber(
          this.redis,
          (event) => this.handleMigrationEvent(event),
          logger
        );
        await this.migrationSubscriber.subscribe();
        logger.info('Migration subscriber started');
      } else {
        logger.warn('Coordinator integration disabled (SERVICE_JWT_SECRET not set)');
      }

      // Wire Redis client to livePoller for lifecycle event publishing (EXPIRY-06)
      this.livePoller.setRedisClient(this.redis);

      // Start message deduplicator cleanup
      this.messageDeduplicator.start();
      logger.info('Message deduplicator started');

      // Start like aggregation publisher
      this.startLikeAggregationPublisher();

      // Start HTTP server for health checks
      this.startHttpServer();

      // Initialize demand subscriber (Phase 5)
      // Replaces old pollActiveStreams / startDatabaseListener approach.
      // source-manager publishes full-snapshot DemandUpdates to "source:demand".
      this.demandSubscriber = new DemandSubscriber(
        this.redis,
        (demanded) => this.handleDemandUpdate(demanded),
        logger,
        this.assignedSourceIDs
      );
      await this.demandSubscriber.subscribe();
      logger.info('Demand subscriber started');

      // Start 60s safety-net poll to restore state after Redis reconnect / missed events
      this.demandSafetyInterval = setInterval(async () => {
        try {
          await this.pollDemandFallback();
        } catch (err) {
          logger.error('Demand safety-net poll failed', { error: String(err) });
        }
      }, DEMAND_SAFETY_INTERVAL_MS);

      // Start periodic metrics update (every 1 minute)
      setInterval(() => {
        this.updateBackoffMetrics();
      }, 60000);

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
        // Readiness probe (TIKTOK-05)
        // Per CONTEXT.md: "Ready status: Pod reports ready AFTER successfully connecting to all assigned channels"
        let isReady = this.redis.isReady && !this.isShuttingDown;

        // If coordinator integration is enabled, check assignments
        if (this.coordinatorClient) {
          // Pod is ready if Redis is connected and not shutting down.
          // A pod with 0 assignments is still ready — the coordinator simply
          // hasn't distributed work to it yet (normal during rolling updates).
          // Requiring assignments caused rollout deadlocks: the old pod held
          // all sources, the new pod never got any, and the rollout stalled.
          res.writeHead(isReady ? 200 : 503, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({
            status: isReady ? 'ready' : 'not ready',
            active_streams: this.activeStreams.size,
            assigned_sources: this.assignedSourceIDs.size,
            filtered_assignments: this.getFilteredAssignmentCount(),
            coordinator_enabled: true
          }));
        } else {
          // Coordinator disabled - use simple readiness check
          res.writeHead(isReady ? 200 : 503, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({
            status: isReady ? 'ready' : 'not ready',
            active_streams: this.activeStreams.size,
            coordinator_enabled: false
          }));
        }
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
      } else if (req.url?.startsWith('/api/channel') && req.method === 'GET') {
        // Get channel state
        const url = new URL(req.url, `http://${req.headers.host}`);
        const username = url.searchParams.get('username');

        if (!username) {
          res.writeHead(400, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ error: 'username parameter required' }));
          return;
        }

        const state = this.getChannelState(username);
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify(state));
      } else if (req.url === '/api/channels' && req.method === 'GET') {
        // List all channel states
        const states = this.getAllChannelStates();
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({
          channels: states,
          summary: {
            total: states.length,
            active: states.filter(s => s.hasActiveConnection).length,
            in_backoff: states.filter(s => s.backoffState && s.backoffState.currentBackoffMs > 0).length,
          }
        }));
      } else if (req.url === '/api/retry' && req.method === 'POST') {
        // Force retry for a username
        let body = '';
        req.on('data', chunk => { body += chunk.toString(); });
        req.on('end', () => {
          try {
            const data = JSON.parse(body);
            if (!data.username) {
              res.writeHead(400, { 'Content-Type': 'application/json' });
              res.end(JSON.stringify({ error: 'username required in request body' }));
              return;
            }

            this.forceRetry(data.username);
            res.writeHead(200, { 'Content-Type': 'application/json' });
            res.end(JSON.stringify({
              message: 'Retry triggered successfully',
              username: data.username,
              timestamp: new Date().toISOString()
            }));
          } catch (error) {
            res.writeHead(400, { 'Content-Type': 'application/json' });
            res.end(JSON.stringify({ error: 'Invalid JSON body' }));
          }
        });
      } else if (req.url === '/api/reset-backoff' && req.method === 'POST') {
        // Reset backoff for username or all
        let body = '';
        req.on('data', chunk => { body += chunk.toString(); });
        req.on('end', () => {
          try {
            const data = body ? JSON.parse(body) : {};

            if (data.username) {
              // Reset specific username
              this.resetBackoff(data.username);
              res.writeHead(200, { 'Content-Type': 'application/json' });
              res.end(JSON.stringify({
                message: 'Backoff reset successfully',
                username: data.username,
                timestamp: new Date().toISOString()
              }));
            } else {
              // Reset all
              const count = this.resetAllBackoff();
              res.writeHead(200, { 'Content-Type': 'application/json' });
              res.end(JSON.stringify({
                message: 'All backoff reset successfully',
                channels_reset: count,
                timestamp: new Date().toISOString()
              }));
            }
          } catch (error) {
            res.writeHead(400, { 'Content-Type': 'application/json' });
            res.end(JSON.stringify({ error: 'Invalid JSON body' }));
          }
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

  /**
   * Handle a demand update from source-manager (Phase 5).
   * Called on every DemandUpdate received via Redis Pub/Sub "source:demand".
   * Full-replacement snapshot: connects new streams, disconnects removed ones.
   * Goes fully idle (stops LiveStreamPoller) when demand is empty.
   */
  private async handleDemandUpdate(demanded: Map<string, DemandSource>): Promise<void> {
    if (this.isShuttingDown) return;

    logger.info('Demand update received', { demanded_count: demanded.size });

    if (demanded.size === 0) {
      // Full idle: disconnect all streams and stop livePoller
      for (const [username] of this.activeStreams.entries()) {
        await this.disconnectFromStream(username);
      }
      if (this.livePollerRunning) {
        this.livePoller.stop();
        this.livePollerRunning = false;
        logger.info('LiveStreamPoller stopped (zero demand)');
      }
      return;
    }

    // Start livePoller if not running
    if (!this.livePollerRunning) {
      this.livePoller.start();
      this.livePollerRunning = true;
      logger.info('LiveStreamPoller started (demand present)');
    }

    // Disconnect streams that lost demand
    const demandedUsernames = new Set(demanded.keys());
    for (const [username] of this.activeStreams.entries()) {
      if (!demandedUsernames.has(username)) {
        await this.disconnectFromStream(username);
      }
    }

    // Connect new demanded streams
    for (const [username, source] of demanded.entries()) {
      if (!this.activeStreams.has(username) && !this.connectingStreams.has(username)) {
        await this.connectToStream(username, source.overlay_id);
      }
    }

    this.filteredAssignmentCount = demanded.size;
  }

  /**
   * Safety-net poll that queries source-manager GET /demand endpoint.
   * Runs every 60s to restore correct state after Redis reconnect / missed Pub/Sub events.
   * Only runs when coordinator integration is enabled.
   */
  private async pollDemandFallback(): Promise<void> {
    if (this.isShuttingDown) return;
    if (!this.coordinatorClient) return;

    try {
      const sources = await this.coordinatorClient.getDemand('tiktok');
      const demanded = new Map<string, DemandSource>();
      for (const source of sources) {
        if (this.assignedSourceIDs.size === 0 || this.assignedSourceIDs.has(source.source_id)) {
          demanded.set(source.channel_id, source);
        }
      }
      await this.handleDemandUpdate(demanded);
    } catch (err) {
      logger.error('Demand fallback poll error', { error: String(err) });
    }
  }

  /**
   * Start heartbeat publisher to coordinator (Phase 6).
   * Publishes heartbeat every 10 seconds to coordinator /heartbeat endpoint.
   * Implements TIKTOK-01: "TikTok listener publishes heartbeat every 10 seconds to coordinator"
   */
  private startHeartbeatPublisher(): void {
    if (!this.coordinatorClient) {
      logger.warn('Cannot start heartbeat publisher: coordinator client not initialized');
      return;
    }

    // Publish immediately
    this.publishHeartbeat();

    // Then publish on interval
    this.heartbeatTimer = setInterval(() => {
      this.publishHeartbeat();
    }, HEARTBEAT_INTERVAL_MS);
  }

  /**
   * Publish heartbeat to coordinator.
   */
  private async publishHeartbeat(): Promise<void> {
    if (!this.coordinatorClient || this.isShuttingDown) {
      return;
    }

    try {
      await this.coordinatorClient.publishHeartbeat(POD_NAME);
    } catch (error) {
      logger.error('Failed to publish heartbeat', {
        pod_id: POD_NAME,
        error: String(error)
      });
      // Don't throw - heartbeat failures shouldn't crash the service
    }
  }

  /**
   * Handle migration event from coordinator (Phase 6).
   * Implements TIKTOK-03 (new pod) and TIKTOK-04 (old pod).
   *
   * Migration protocol:
   * - New pod (to_pod matches POD_NAME): Connect to channel, wait for first message (30s timeout), confirm
   * - Old pod (from_pod matches POD_NAME): Disconnect from channel after confirmation received
   *
   * Per CONTEXT.md: "Minimal state - just channel assignment list - New pod creates fresh connections"
   * Per CONTEXT.md: "TikTok connection state migration for unofficial library - Cannot transfer connection handles, must disconnect/reconnect"
   */
  private async handleMigrationEvent(event: MigrationEvent): Promise<void> {
    // Only handle TikTok platform events. Log non-TikTok events at debug to avoid spam —
    // the migration:events channel carries events for ALL platforms and non-TikTok events
    // are irrelevant to this service.
    if (event.platform !== 'tiktok') {
      logger.debug('Ignoring non-TikTok migration event', {
        migration_id: event.migration_id,
        platform: event.platform
      });
      return;
    }

    logger.info('Processing TikTok migration event', {
      migration_id: event.migration_id,
      channel_id: event.channel_id,
      platform: event.platform,
      from_pod: event.from_pod,
      to_pod: event.to_pod,
      reason: event.reason,
      is_new_pod: event.to_pod === POD_NAME,
      is_old_pod: event.from_pod === POD_NAME
    });

    // Get username from source_id via database query
    // source_id is a UUID, need to get channel_id (username)
    const result = await this.db.query(
      `SELECT channel_id FROM overlay_chat_sources WHERE id = $1 AND platform = 'tiktok'`,
      [event.channel_id]
    );

    if (result.rows.length === 0) {
      logger.error('Source ID not found in database', {
        migration_id: event.migration_id,
        source_id: event.channel_id
      });
      return;
    }

    const username = result.rows[0].channel_id;

    // New pod: Connect to channel (TIKTOK-03)
    if (event.to_pod === POD_NAME) {
      logger.info('New pod: Connecting to channel for migration', {
        migration_id: event.migration_id,
        username,
        timeout_ms: 30000
      });

      // Add to assigned source IDs and update demand subscriber
      this.assignedSourceIDs.add(event.channel_id);
      if (this.demandSubscriber) {
        this.demandSubscriber.updateAssignedSourceIDs(this.assignedSourceIDs);
      }

      // Set up promise to wait for first message or timeout
      const firstMessagePromise = new Promise<void>((resolve) => {
        this.firstMessageCallbacks.set(username, () => {
          this.firstMessageCallbacks.delete(username);
          resolve();
        });
      });

      // Connect to stream
      // Use placeholder overlay_id since we're connecting via migration
      await this.connectToStream(username, 'migration-' + event.migration_id);

      // Wait for first message or timeout (30s per CONTEXT.md)
      const timeout = setTimeout(() => {
        const callback = this.firstMessageCallbacks.get(username);
        if (callback) {
          this.firstMessageCallbacks.delete(username);
          logger.error('Migration timeout (new pod)', {
            migration_id: event.migration_id,
            username
          });
          this.publishMigrationConfirmation(event.migration_id, 'failed', 0);
        }
      }, 30000);

      firstMessagePromise.then(() => {
        clearTimeout(timeout);
        logger.info('New pod: Successfully connected for migration', {
          migration_id: event.migration_id,
          username
        });
        this.publishMigrationConfirmation(event.migration_id, 'connected', 0);
      });
    }

    // Old pod: Disconnect from channel (TIKTOK-04)
    if (event.from_pod === POD_NAME) {
      logger.info('Old pod: Disconnecting from channel for migration', {
        migration_id: event.migration_id,
        username
      });

      // Remove from assigned source IDs and update demand subscriber
      this.assignedSourceIDs.delete(event.channel_id);
      if (this.demandSubscriber) {
        this.demandSubscriber.updateAssignedSourceIDs(this.assignedSourceIDs);
      }

      // Disconnect from stream
      await this.disconnectFromStream(username);

      logger.info('Old pod: Successfully disconnected for migration', {
        migration_id: event.migration_id,
        username
      });
    }
  }

  /**
   * Publish migration confirmation to Redis Streams for coordinator to detect.
   * Called by new pod after first message received or timeout.
   */
  private async publishMigrationConfirmation(
    migrationId: string,
    status: 'connected' | 'failed',
    sequenceNum: number
  ): Promise<void> {
    try {
      const event: Record<string, string> = {
        migration_id: migrationId,
        status: status,
        pod_id: POD_NAME,
        timestamp: Math.floor(Date.now() / 1000).toString(),
        sequence_number: sequenceNum.toString()
      };

      await this.redis.xAdd('migration:log', '*', event);

      logger.info('Published migration confirmation', {
        migration_id: migrationId,
        status: status
      });
    } catch (error) {
      logger.error('Failed to publish migration confirmation', {
        migration_id: migrationId,
        error
      });
    }
  }

  /**
   * Get filtered assignment count (number of demanded sources).
   * Used for observability in health check endpoint.
   */
  private getFilteredAssignmentCount(): number {
    return this.filteredAssignmentCount;
  }

  private async connectToStream(username: string, overlayId: string): Promise<void> {
    // Mark as connecting to prevent concurrent attempts
    if (this.connectingStreams.has(username)) {
      logger.debug('Already connecting to stream, skipping duplicate attempt', { username, overlay_id: overlayId });
      return;
    }

    this.connectingStreams.add(username);

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

      connection.on(WebcastEvent.GIFT, (data) => {
        this.handleGift(username, overlayId, data);
      });

      connection.on(WebcastEvent.LIKE, (data) => {
        this.handleLike(username, overlayId, data);
      });

      connection.on(WebcastEvent.FOLLOW, (data) => {
        this.handleFollow(username, overlayId, data);
      });

      connection.on(WebcastEvent.SOCIAL, (data) => {
        this.handleShare(username, overlayId, data);
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

        // Publish connected status for overlay status indicators
        this.publishPlatformStatus(username, 'connected');

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

        // Publish lifecycle:stream_end event for share expiry (EXPIRY-06)
        this.livePoller.publishStreamEnd(username);

        // Stream ended - reset to quick re-check
        this.backoffManager.recordDisconnection(username);
        this.livePoller.addTarget(username, overlayId);

        // Publish reconnecting status so overlay indicators show the retry state
        const backoffState = this.backoffManager.getState(username);
        const nextRetryAt = backoffState
          ? new Date(Date.now() + backoffState.currentBackoffMs)
          : new Date(Date.now() + TIKTOK_BASE_OFFLINE_BACKOFF_MS);
        this.publishPlatformStatus(username, 'reconnecting', nextRetryAt);

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
    } finally {
      // Always remove from connecting set when done (success or failure)
      this.connectingStreams.delete(username);
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

      // Publish offline status so overlay indicators reflect the disconnected state
      this.publishPlatformStatus(username, 'offline');
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
      // OPTIMIZATION: Only update if status actually changed to prevent notification spam
      await this.db.query(
        `UPDATE overlay_chat_sources
         SET is_active = $1, updated_at = NOW()
         WHERE platform = 'tiktok' AND channel_id = $2 AND is_active != $1`,
        [isActive, username]
      );
      logger.debug('Updated source active status', { username, is_active: isActive });
    } catch (error) {
      logger.error('Failed to update source active status', { username, error });
    }
  }

  private async setSourceActiveByOverlay(overlayId: string, username: string, isActive: boolean): Promise<void> {
    try {
      // OPTIMIZATION: Only update if status actually changed to prevent notification spam
      await this.db.query(
        `UPDATE overlay_chat_sources
         SET is_active = $1, updated_at = NOW()
         WHERE platform = 'tiktok' AND overlay_id = $2 AND channel_id = $3 AND is_active != $1`,
        [isActive, overlayId, username]
      );
      logger.debug('Updated overlay-specific source active status', { overlay_id: overlayId, username, is_active: isActive });
    } catch (error) {
      logger.error('Failed to update overlay-specific source active status', { overlay_id: overlayId, username, error });
    }
  }

  /**
   * Publish platform connection status to Redis Pub/Sub for overlay status indicators.
   * The API Gateway subscribes to platform:status and forwards to connected WebSocket clients.
   */
  private async publishPlatformStatus(
    username: string,
    statusValue: 'connected' | 'reconnecting' | 'offline',
    nextRetryAt?: Date,
    errorMessage?: string
  ): Promise<void> {
    try {
      const msg: Record<string, string> = {
        platform: 'tiktok',
        channel_id: username,
        status: statusValue,
      };
      if (nextRetryAt) {
        msg.next_retry_at = nextRetryAt.toISOString();
      }
      if (errorMessage) {
        msg.error_message = errorMessage;
      }
      await this.redis.publish('platform:status', JSON.stringify(msg));
      logger.debug('Published platform status', { username, status: statusValue });
    } catch (error) {
      logger.warn('Failed to publish platform status', { username, status: statusValue, error: String(error) });
    }
  }

  private async handleChatMessage(username: string, overlayId: string, data: any): Promise<void> {
    try {
      // Record message for heartbeat monitoring
      this.heartbeatMonitor.recordMessage(username);

      // Signal first message for migration confirmation
      const callback = this.firstMessageCallbacks.get(username);
      if (callback) {
        callback();
      }

      // Extract TikTok's native message ID and timestamp
      const msgId = data.common?.msgId;
      const createTime = data.common?.createTime;
      
      // Convert TikTok timestamp (Unix timestamp in string format, in milliseconds)
      // to ISO 8601 format
      let timestamp: string;
      if (createTime) {
        // TikTok createTime is Unix timestamp in milliseconds (as string)
        const timestampMs = parseInt(createTime);
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

  private async handleGift(username: string, overlayId: string, data: any): Promise<void> {
    try {
      this.heartbeatMonitor.recordMessage(username);

      const msgId = data.common?.msgId || randomUUID();
      const createTime = data.common?.createTime;
      const timestamp = createTime ? new Date(parseInt(createTime)).toISOString() : new Date().toISOString();

      const rawMessage: RawChatMessage = {
        message_id: msgId,
        platform: 'tiktok',
        channel_id: username,
        user_id: data.user?.uniqueId || 'unknown',
        username: data.user?.nickname || 'Anonymous',
        text: `Sent ${data.gift?.name || 'a gift'}`,
        timestamp: timestamp,
        tags: {
          overlay_id: overlayId,
          user_unique_id: data.user?.uniqueId || '',
          profile_picture_url: data.user?.profilePictureUrl || ''
        },
        event_type: 'gift',
        event_data: {
          gift_id: data.gift?.giftId,
          gift_name: data.gift?.name,
          gift_type: data.giftType, // 1 = normal, 2 = special
          gift_count: data.repeatCount || 1,
          diamond_count: (data.gift?.diamondCount || 0) * (data.repeatCount || 1)
        }
      };

      await this.redis.xAdd('chat:raw', '*', { data: JSON.stringify(rawMessage) });
      logger.debug('Published TikTok gift event', { username, overlay_id: overlayId, gift: data.gift?.name });
    } catch (error) {
      logger.error('Failed to handle gift event', { username, error });
    }
  }

  private async handleLike(username: string, overlayId: string, data: any): Promise<void> {
    try {
      this.heartbeatMonitor.recordMessage(username);

      const userId = data.user?.uniqueId || 'unknown';
      const userNickname = data.user?.nickname || 'Anonymous';
      const likeCount = data.likeCount || 1;
      const aggregationKey = `${username}:${userId}`;

      let aggregation = this.likeAggregations.get(aggregationKey);

      if (!aggregation) {
        // Start new 30-second aggregation window
        const aggregationId = randomUUID();
        const windowStart = new Date();

        aggregation = {
          aggregation_id: aggregationId,
          username,
          overlay_id: overlayId,
          user_id: userId,
          user_nickname: userNickname,
          like_count: likeCount,
          window_start: windowStart,
          last_published: new Date(0) // Never published yet
        };

        this.likeAggregations.set(aggregationKey, aggregation);

        logger.debug('Started like aggregation window', {
          username,
          user: userNickname,
          aggregation_id: aggregationId
        });
      } else {
        // Update existing window
        aggregation.like_count += likeCount;
      }
    } catch (error) {
      logger.error('Failed to handle like event', { username, error });
    }
  }

  private async handleFollow(username: string, overlayId: string, data: any): Promise<void> {
    try {
      this.heartbeatMonitor.recordMessage(username);

      const msgId = data.common?.msgId || randomUUID();
      const createTime = data.common?.createTime;
      const timestamp = createTime ? new Date(parseInt(createTime)).toISOString() : new Date().toISOString();

      const rawMessage: RawChatMessage = {
        message_id: msgId,
        platform: 'tiktok',
        channel_id: username,
        user_id: data.user?.uniqueId || 'unknown',
        username: data.user?.nickname || 'Anonymous',
        text: 'Followed',
        timestamp: timestamp,
        tags: {
          overlay_id: overlayId,
          user_unique_id: data.user?.uniqueId || '',
          profile_picture_url: data.user?.profilePictureUrl || ''
        },
        event_type: 'follow',
        event_data: {}
      };

      await this.redis.xAdd('chat:raw', '*', { data: JSON.stringify(rawMessage) });
      logger.debug('Published TikTok follow event', { username, overlay_id: overlayId });
    } catch (error) {
      logger.error('Failed to handle follow event', { username, error });
    }
  }

  private async handleShare(username: string, overlayId: string, data: any): Promise<void> {
    try {
      this.heartbeatMonitor.recordMessage(username);

      const msgId = data.common?.msgId || randomUUID();
      const createTime = data.common?.createTime;
      const timestamp = createTime ? new Date(parseInt(createTime)).toISOString() : new Date().toISOString();

      const rawMessage: RawChatMessage = {
        message_id: msgId,
        platform: 'tiktok',
        channel_id: username,
        user_id: data.user?.uniqueId || 'unknown',
        username: data.user?.nickname || 'Anonymous',
        text: 'Shared the stream',
        timestamp: timestamp,
        tags: {
          overlay_id: overlayId,
          user_unique_id: data.user?.uniqueId || '',
          profile_picture_url: data.user?.profilePictureUrl || ''
        },
        event_type: 'share',
        event_data: {}
      };

      await this.redis.xAdd('chat:raw', '*', { data: JSON.stringify(rawMessage) });
      logger.debug('Published TikTok share event', { username, overlay_id: overlayId });
    } catch (error) {
      logger.error('Failed to handle share event', { username, error });
    }
  }

  private startLikeAggregationPublisher(): void {
    this.likeAggregationTimer = setInterval(async () => {
      const now = new Date();
      const expiredKeys: string[] = [];

      for (const [key, agg] of this.likeAggregations.entries()) {
        const windowAge = now.getTime() - agg.window_start.getTime();
        const timeSinceLastPublish = now.getTime() - agg.last_published.getTime();

        // Publish if:
        // 1. Window is closed (30s elapsed), OR
        // 2. 5 seconds since last publish AND likes accumulated
        const shouldPublish =
          windowAge >= this.LIKE_AGGREGATION_WINDOW_MS || // 30s window closed
          (timeSinceLastPublish >= this.LIKE_PUBLISH_INTERVAL_MS && agg.like_count > 0); // 5s update interval

        if (shouldPublish) {
          const isWindowClosed = windowAge >= this.LIKE_AGGREGATION_WINDOW_MS;
          const isUpdate = agg.last_published.getTime() > 0; // true if not first publish

          const rawMessage: RawChatMessage = {
            message_id: agg.aggregation_id, // Keep same ID for updates
            platform: 'tiktok',
            channel_id: agg.username,
            user_id: agg.user_id,
            username: agg.user_nickname,
            text: `Sent ${agg.like_count} like${agg.like_count !== 1 ? 's' : ''}`,
            timestamp: agg.window_start.toISOString(),
            tags: {
              overlay_id: agg.overlay_id
            },
            event_type: 'like_aggregate',
            event_data: {
              aggregation_id: agg.aggregation_id,
              like_count: agg.like_count,
              window_start: agg.window_start.toISOString(),
              window_end: isWindowClosed ? now.toISOString() : null,
              is_update: isUpdate
            }
          };

          try {
            await this.redis.xAdd('chat:raw', '*', { data: JSON.stringify(rawMessage) });

            logger.debug('Published like aggregation', {
              username: agg.username,
              user: agg.user_nickname,
              like_count: agg.like_count,
              is_update: isUpdate,
              window_closed: isWindowClosed
            });

            agg.last_published = now;

            // Remove from map if window closed
            if (isWindowClosed) {
              expiredKeys.push(key);
            }
          } catch (error) {
            logger.error('Failed to publish like aggregation', { error });
          }
        }
      }

      // Cleanup expired windows
      for (const key of expiredKeys) {
        this.likeAggregations.delete(key);
      }
    }, this.LIKE_PUBLISH_INTERVAL_MS); // Run every 5 seconds

    logger.info('Like aggregation publisher started');
  }

  private stopLikeAggregationPublisher(): void {
    if (this.likeAggregationTimer) {
      clearInterval(this.likeAggregationTimer);
      this.likeAggregationTimer = undefined;
      logger.info('Like aggregation publisher stopped');
    }
  }
  // ============================================================================
  // State Inspection and Control Methods (for API endpoints)
  // ============================================================================

  /**
   * Get channel state for a username
   */
  private getChannelState(username: string) {
    const backoffState = this.backoffManager.getState(username);
    const activeStream = this.activeStreams.get(username);
    const isConnecting = this.connectingStreams.has(username);
    const pollerTargets = this.livePoller.getTargets();
    const isInPoller = pollerTargets.some(t => t.username === username);

    let riskLevel = "low";
    let recommendedAction = "";

    if (backoffState && backoffState.currentBackoffMs > 180000) {
      // Backoff > 3 minutes
      riskLevel = "high";
      recommendedAction = "Consider reset-backoff or force retry";
    } else if (backoffState && backoffState.currentBackoffMs > 60000) {
      // Backoff > 1 minute
      riskLevel = "medium";
      recommendedAction = "Monitor for automatic recovery";
    }

    return {
      username,
      backoffState: backoffState ? {
        consecutiveOfflineChecks: backoffState.consecutiveOfflineChecks,
        consecutiveErrors: backoffState.consecutiveErrors,
        currentBackoffMs: backoffState.currentBackoffMs,
        currentBackoffMinutes: Math.round(backoffState.currentBackoffMs / 60000),
        nextCheckTime: new Date(backoffState.nextCheckTime).toISOString(),
        lastCheckTime: new Date(backoffState.lastCheckTime).toISOString(),
        lastSeenLive: backoffState.lastSeenLive ? new Date(backoffState.lastSeenLive).toISOString() : null,
      } : null,
      hasActiveConnection: activeStream?.is_connected || false,
      isConnecting,
      isInPoller,
      riskLevel,
      recommendedAction,
    };
  }

  /**
   * Get all channel states
   */
  private getAllChannelStates() {
    const usernames = new Set<string>();

    // Collect from all sources
    for (const username of this.activeStreams.keys()) {
      usernames.add(username);
    }
    for (const username of this.backoffManager.getAllUsernames()) {
      usernames.add(username);
    }
    for (const target of this.livePoller.getTargets()) {
      usernames.add(target.username);
    }

    return Array.from(usernames).map(username => this.getChannelState(username));
  }

  /**
   * Force retry for a username (reset backoff and trigger immediate check)
   */
  private forceRetry(username: string): void {
    logger.info("Manual force retry requested", { username, action: "admin_force_retry" });

    // Reset backoff
    this.backoffManager.removeState(username);

    // Add to poller if not already there
    const pollerTargets = this.livePoller.getTargets();
    const inPoller = pollerTargets.some(t => t.username === username);
    if (!inPoller) {
      // Get overlay ID from active streams or use placeholder
      const activeStream = this.activeStreams.get(username);
      const overlayId = activeStream?.overlay_id || "admin-force-retry";
      this.livePoller.addTarget(username, overlayId);
    }

    // Trigger a demand fallback poll to restore state
    this.pollDemandFallback().catch(err => {
      logger.error('Force retry demand poll failed', { error: String(err) });
    });
  }

  /**
   * Reset backoff for a specific username
   */
  private resetBackoff(username: string): void {
    logger.info("Manual backoff reset requested", { username, action: "admin_reset_backoff" });
    this.backoffManager.removeState(username);
  }

  /**
   * Reset backoff for all usernames
   */
  private resetAllBackoff(): number {
    const usernames = this.backoffManager.getAllUsernames();
    logger.warn("Manual reset ALL backoff requested", {
      action: "admin_reset_all_backoff",
      count: usernames.length,
    });

    for (const username of usernames) {
      this.backoffManager.removeState(username);
    }

    // Trigger a demand fallback poll to restore state
    this.pollDemandFallback().catch(err => {
      logger.error('Reset all backoff demand poll failed', { error: String(err) });
    });

    return usernames.length;
  }


  async stop(): Promise<void> {
    this.isShuttingDown = true;
    logger.info('Shutting down TikTok Listener Service...');

    // Stop demand safety-net interval (Phase 5)
    if (this.demandSafetyInterval) {
      clearInterval(this.demandSafetyInterval);
      this.demandSafetyInterval = null;
    }

    // Unsubscribe demand subscriber (Phase 5)
    if (this.demandSubscriber) {
      await this.demandSubscriber.unsubscribe();
      logger.info('Demand subscriber stopped');
    }

    // Stop heartbeat publisher (Phase 6)
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
      logger.info('Heartbeat publisher stopped');
    }

    // Stop live stream poller (only if running)
    if (this.livePollerRunning) {
      this.livePoller.stop();
      this.livePollerRunning = false;
      logger.info('Live stream poller stopped');
    }

    // Stop all heartbeat monitoring
    this.heartbeatMonitor.stopAll();
    logger.info('Heartbeat monitoring stopped');

    // Stop message deduplicator cleanup
    this.messageDeduplicator.stop();
    logger.info('Message deduplicator stopped');

    // Stop like aggregation publisher
    this.stopLikeAggregationPublisher();

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

    // Shutdown OpenTelemetry tracing
    await shutdownTracing();

    logger.info('Service shutdown complete');
  }

  /**
   * Update backoff and detection metrics periodically
   * Runs every minute to emit current backoff states to Prometheus
   */
  private updateBackoffMetrics(): void {
    let stuckCount = 0;

    for (const username of this.backoffManager.getAllUsernames()) {
      const backoffState = this.backoffManager.getState(username);

      if (backoffState) {
        this.metrics.recordBackoffInterval(
          username,
          backoffState.currentBackoffMs
        );

        if (backoffState.currentBackoffMs > 180000) {
          stuckCount++;
        }
      }
    }

    this.metrics.setBackoffUsernamesStuck(stuckCount);
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
