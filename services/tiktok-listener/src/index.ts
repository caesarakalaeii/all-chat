/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */

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

// Import coordination modules (leadership-based)
import { SourceManagerClient } from './coordination/client.js';
import { LeadershipCoordinator } from './coordination/leadership.js';

// Import demand subscriber (Phase 5)
import { DemandSubscriber, DemandSource } from './demand/subscriber.js';
import { pickAvatarUrl, tiktokAvatarUrl } from './avatar.js';
import {
  hasTikTokChestPayload,
  isTikTokCoinChest,
  isTikTokEnvelopeDrop,
  type TikTokEnvelopeData
} from './envelope.js';

// Environment variables
const LOG_LEVEL = process.env.LOG_LEVEL || 'info';
const LOG_FORMAT = process.env.LOG_FORMAT || 'json'; // 'json' or 'simple'
const REDIS_HOST = process.env.REDIS_HOST || 'localhost';
const REDIS_PORT = parseInt(process.env.REDIS_PORT || '6379');
// Cap for Redis (re)connect backoff. node-redis retries up to this delay so a
// transient outage (e.g. the redis pod restarting during a node reboot) is
// ridden out instead of crashing the pod into CrashLoopBackOff.
const REDIS_RECONNECT_MAX_DELAY_MS = 5000;
const DATABASE_HOST = process.env.DATABASE_HOST || 'localhost';
const DATABASE_PORT = parseInt(process.env.DATABASE_PORT || '5432');
const DATABASE_USER = process.env.DATABASE_USER || 'allchat';
const DATABASE_PASSWORD = process.env.DATABASE_PASSWORD || '';
if (!DATABASE_PASSWORD) {
  console.error('DATABASE_PASSWORD must be set');
  process.exit(1);
}
const DATABASE_NAME = process.env.DATABASE_NAME || 'allchat';
const HTTP_PORT = parseInt(process.env.PORT || '8089');
const DEMAND_SAFETY_INTERVAL_MS = parseInt(process.env.DEMAND_SAFETY_INTERVAL_MS || '60000'); // 60 seconds

// Coordinator configuration (Phase 6)
const COORDINATOR_URL = process.env.COORDINATOR_URL || 'http://source-manager:8088';
const SERVICE_JWT_SECRET = process.env.SERVICE_JWT_SECRET || '';
const POD_NAME = process.env.HOSTNAME || 'tiktok-listener-unknown';

// New TikTok live detection configuration
const TIKTOK_STATUS_CHECK_CACHE_TTL_MS = parseInt(process.env.TIKTOK_STATUS_CHECK_CACHE_TTL_MS || '10000');
const TIKTOK_POLLER_INTERVAL_MS = parseInt(process.env.TIKTOK_POLLER_INTERVAL_MS || '30000');
const TIKTOK_BASE_OFFLINE_BACKOFF_MS = parseInt(process.env.TIKTOK_BASE_OFFLINE_BACKOFF_MS || '60000');
const TIKTOK_MAX_OFFLINE_BACKOFF_MS = parseInt(process.env.TIKTOK_MAX_OFFLINE_BACKOFF_MS || '600000');
const TIKTOK_ERROR_BACKOFF_MS = parseInt(process.env.TIKTOK_ERROR_BACKOFF_MS || '2000');
const TIKTOK_MAX_ERROR_BACKOFF_MS = parseInt(process.env.TIKTOK_MAX_ERROR_BACKOFF_MS || '300000');
const TIKTOK_HEARTBEAT_INTERVAL_MS = parseInt(process.env.TIKTOK_HEARTBEAT_INTERVAL_MS || '30000');
const TIKTOK_HEARTBEAT_TIMEOUT_MS = parseInt(process.env.TIKTOK_HEARTBEAT_TIMEOUT_MS || '90000');

// Hard cap on internal admin HTTP request bodies (/api/retry, /api/reset-backoff).
// These accept small JSON payloads; capping prevents an oversized POST from spiking
// process memory and risking an OOMKill.
const MAX_REQUEST_BODY_BYTES = parseInt(process.env.MAX_REQUEST_BODY_BYTES || '65536'); // 64 KB

// Message deduplication configuration
const TIKTOK_DEDUP_TTL_MS = parseInt(process.env.TIKTOK_DEDUP_TTL_MS || '300000'); // 5 minutes
const TIKTOK_DEDUP_CLEANUP_INTERVAL_MS = parseInt(process.env.TIKTOK_DEDUP_CLEANUP_INTERVAL_MS || '60000'); // 1 minute
const TIKTOK_DEDUP_MAX_CACHE_SIZE = parseInt(process.env.TIKTOK_DEDUP_MAX_CACHE_SIZE || '10000');

// Import logger interface
import { Logger } from './types/logger.js';
import { validateTikTokUsername } from './types/validation.js';

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
  event_type?: string; // "gift", "follow", "like_aggregate", "share", "treasure_chest", etc.
  event_data?: Record<string, unknown>; // Event-specific payload
}

// Minimal typed views of the tiktok-live-connector v2.4+ event payloads.
//
// The library sources these message types from `tiktok-live-proto/v3` (a
// transitive dependency we intentionally do not import directly). We only
// declare the subset of fields this service consumes. Field names below reflect
// the current TikTok wire protocol; several were renamed relative to the older
// pre-v3 proto (noted inline) — that rename is why chat text silently vanished
// on the pinned 2.1.0 build.
interface TikTokImageModel {
  urlList?: string[];
}

interface TikTokUser {
  id?: string; // numeric user id (was `userId`)
  nickname?: string; // display name
  displayId?: string; // @handle (was `uniqueId`)
  // Profile picture image models, smallest → largest (was `profilePictureUrl`).
  // TikTok populates these inconsistently per message, so avatar selection
  // falls through thumb → medium → large; see tiktokAvatarUrl in ./avatar.
  avatarThumb?: TikTokImageModel;
  avatarMedium?: TikTokImageModel;
  avatarLarge?: TikTokImageModel;
}

// Per-message relationship flags relative to the broadcasting anchor.
interface TikTokUserIdentity {
  isFollowerOfAnchor?: boolean;
  isSubscriberOfAnchor?: boolean;
  isModeratorOfAnchor?: boolean;
}

// Localized display string attached to a message. `key` is a stable template id
// (e.g. `pm_mt_ttlive_superfanbox_join`) — the connector itself probes it to tell
// the envelope products apart; see isTikTokCoinChest.
interface TikTokDisplayText {
  key?: string;
}

interface TikTokCommon {
  msgId?: string;
  createTime?: string;
  displayText?: TikTokDisplayText;
}

interface TikTokChatData {
  common?: TikTokCommon;
  user?: TikTokUser;
  userIdentity?: TikTokUserIdentity;
  content?: string; // message text (was `comment`)
}

interface TikTokGift {
  id?: string;
  name?: string;
  type?: number; // gift type: 1 = streakable (combo), other = non-streakable (was top-level `giftType`)
  diamondCount?: number;
}

interface TikTokGiftData {
  common?: TikTokCommon;
  user?: TikTokUser;
  giftId?: string; // now top-level (was `gift.giftId`)
  repeatCount?: number;
  repeatEnd?: number; // 1 once a streakable gift's combo has finished; intermediate frames are 0
  gift?: TikTokGift;
}

interface TikTokLikeData {
  common?: TikTokCommon;
  user?: TikTokUser;
  count?: number; // like count (was `likeCount`)
}

interface TikTokSocialData {
  common?: TikTokCommon;
  user?: TikTokUser;
}

// Reads the TikTok @handle, falling back to the numeric id then a placeholder.
function tiktokUserHandle(user: TikTokUser | undefined): string {
  return user?.displayId || user?.id || 'unknown';
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

  // Latest demanded usernames from the most recent DemandUpdate (the authoritative
  // snapshot). connectToStream() re-checks this after its async live-status pre-check
  // so a stream that loses demand mid-connection bails out instead of becoming an
  // orphan (a live WebSocket + leadership lease with no downstream consumer).
  private demandedUsernames: Set<string> = new Set();

  // Leadership coordination
  private sourceManagerClient?: SourceManagerClient;
  private leadershipCoordinator?: LeadershipCoordinator;

  // Demand subscriber (Phase 5)
  private demandSubscriber: DemandSubscriber | null = null;
  private demandSafetyInterval: ReturnType<typeof setInterval> | null = null;
  private livePollerRunning: boolean = false;

  constructor() {
    // Initialize Redis client.
    // reconnectStrategy makes node-redis keep retrying (with capped backoff)
    // instead of giving up, so a transient Redis outage — e.g. the
    // single-replica redis pod going down during a node reboot — is ridden out
    // rather than crashing the pod.
    this.redis = createClient({
      socket: {
        host: REDIS_HOST,
        port: REDIS_PORT,
        reconnectStrategy: (retries: number): number =>
          Math.min((retries + 1) * 500, REDIS_RECONNECT_MAX_DELAY_MS)
      }
    });

    // node-redis emits socket errors as 'error' events; an EventEmitter with no
    // 'error' listener throws, which would crash the whole process on any Redis
    // blip (this was the cause of the overnight TargetDown crashloop). Logging
    // it here lets the reconnectStrategy reconnect instead.
    this.redis.on('error', (err: Error) => {
      logger.error('Redis client error', { error: err.message });
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

    // Start the HTTP server FIRST so the liveness/readiness/metrics endpoints
    // are available immediately. This keeps the Prometheus scrape target up (so
    // a dependency blip no longer surfaces as TargetDown) and keeps the pod
    // alive while Redis is briefly unreachable. Readiness gates on
    // this.redis.isReady, so traffic is still held off until Redis is connected.
    this.startHttpServer();

    try {
      // Connect to Redis, retrying instead of crashing if it is briefly down.
      await this.connectRedisWithRetry();
      logger.info('Connected to Redis', { host: REDIS_HOST, port: REDIS_PORT });

      // Test database connection
      await this.db.query('SELECT NOW()');
      logger.info('Connected to PostgreSQL', { host: DATABASE_HOST, port: DATABASE_PORT });

      // Initialize leadership-based coordination
      if (SERVICE_JWT_SECRET) {
        logger.info('Leadership coordination enabled', {
          source_manager_url: COORDINATOR_URL,
          pod_name: POD_NAME
        });

        this.sourceManagerClient = new SourceManagerClient(COORDINATOR_URL, SERVICE_JWT_SECRET, logger);
        this.leadershipCoordinator = new LeadershipCoordinator('tiktok', this.sourceManagerClient, logger);

        // Register as active peer for rebalancing
        const peerCount = await this.sourceManagerClient.registerPeer(
          'tiktok', this.leadershipCoordinator.getCallerID()
        );
        logger.info('Registered as peer', { peer_count: peerCount });
      } else {
        logger.warn('Leadership coordination disabled (SERVICE_JWT_SECRET not set)');
      }

      // Wire Redis client to livePoller for lifecycle event publishing (EXPIRY-06)
      this.livePoller.setRedisClient(this.redis);

      // Start message deduplicator cleanup
      this.messageDeduplicator.start();
      logger.info('Message deduplicator started');

      // Start like aggregation publisher
      this.startLikeAggregationPublisher();

      // Initialize demand subscriber (Phase 5)
      // Replaces old pollActiveStreams / startDatabaseListener approach.
      // source-manager publishes full-snapshot DemandUpdates to "source:demand".
      // No assignment-based filtering — tiktok-listener is leadership-based,
      // so all demanded tiktok sources are accepted. Pods claim streams
      // independently via live status checks.
      this.demandSubscriber = new DemandSubscriber(
        this.redis,
        (demanded) => this.handleDemandUpdate(demanded),
        logger
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

  /**
   * Connect to Redis, retrying with capped backoff instead of throwing.
   *
   * node-redis's reconnectStrategy only governs reconnections AFTER an initial
   * successful connect — the first connect() still rejects if Redis is
   * unreachable. Previously that rejection propagated to process.exit(1),
   * turning a transient Redis outage (e.g. the single-replica redis pod going
   * down during a node reboot) into a CrashLoopBackOff. We retry until Redis is
   * reachable; the HTTP server is already up, so the pod stays alive (liveness)
   * and reports not-ready (readiness) until the connection succeeds.
   */
  private async connectRedisWithRetry(): Promise<void> {
    let attempt = 0;
    while (true) {
      try {
        await this.redis.connect();
        return;
      } catch (error) {
        if (this.isShuttingDown) {
          throw error;
        }
        attempt++;
        const delayMs = Math.min(attempt * 500, REDIS_RECONNECT_MAX_DELAY_MS);
        logger.warn('Redis connection attempt failed, retrying', {
          attempt,
          retry_in_ms: delayMs,
          error: error instanceof Error ? error.message : String(error)
        });
        await new Promise<void>((resolve) => setTimeout(resolve, delayMs));
      }
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

        res.writeHead(isReady ? 200 : 503, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({
          status: isReady ? 'ready' : 'not ready',
          active_streams: this.activeStreams.size,
          leadership_leases: this.leadershipCoordinator?.getLeaseCount() ?? 0,
          coordination_enabled: !!this.leadershipCoordinator,
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
        this.readJsonBody(req, res, (body) => {
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
        this.readJsonBody(req, res, (body) => {
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
   * Read a request body as a string with a hard size cap.
   *
   * Bodies larger than MAX_REQUEST_BODY_BYTES are rejected with 413 and the socket
   * is destroyed; onComplete only fires for requests that stay within the limit.
   * The `exceeded` guard ensures we send the 413 response exactly once even if more
   * 'data' chunks arrive before the socket finishes tearing down.
   *
   * @param req Incoming request
   * @param res Response (used to send 413 if the cap is exceeded)
   * @param onComplete Called with the full body once received within the limit
   */
  private readJsonBody(
    req: http.IncomingMessage,
    res: http.ServerResponse,
    onComplete: (body: string) => void
  ): void {
    let body = '';
    let exceeded = false;

    req.on('data', (chunk) => {
      if (exceeded) return;
      body += chunk.toString();
      if (body.length > MAX_REQUEST_BODY_BYTES) {
        exceeded = true;
        res.writeHead(413, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ error: 'Request body too large' }));
        req.destroy();
      }
    });

    req.on('end', () => {
      if (!exceeded) {
        onComplete(body);
      }
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

    // Record the authoritative demand snapshot first, so any connection attempt
    // already in flight (suspended on its async live-status pre-check) sees the
    // updated set when it resumes and aborts if it lost demand.
    this.demandedUsernames = new Set(demanded.keys());

    // Prune every tracked stream that lost demand. A stream can be tracked in
    // three places: active (connected), parked in the poller (offline, awaiting
    // re-check), or mid-connection in connectingStreams. The previous code only
    // iterated activeStreams, so a stream sitting in the poller — which may still
    // hold a leadership lease claimed before it was found offline — was never
    // released, and a stream mid-connection could go active with no demand at all.
    // Here we release leadership and drop poller/active state for active and
    // polled streams; connectingStreams entries abort themselves via the
    // demandedUsernames check in connectToStream().
    const trackedUsernames = new Set<string>([
      ...this.activeStreams.keys(),
      ...this.livePoller.getTargets().map(t => t.username),
    ]);
    for (const username of trackedUsernames) {
      if (this.demandedUsernames.has(username)) continue;

      if (this.leadershipCoordinator) {
        await this.leadershipCoordinator.release(username);
      }
      this.livePoller.removeTarget(username);
      if (this.activeStreams.has(username)) {
        await this.disconnectFromStream(username);
      }
    }

    // Go fully idle when there is no demand: stop the poller entirely.
    if (demanded.size === 0) {
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

    // Claim leadership and connect new demanded streams
    for (const [username, source] of demanded.entries()) {
      // Skip if already active, connecting, or waiting in the poller for live status
      if (!this.activeStreams.has(username) && !this.connectingStreams.has(username) && !this.livePoller.isTargetActive(username)) {
        // Try to claim leadership — if another pod holds it, skip
        if (this.leadershipCoordinator) {
          const acquired = await this.leadershipCoordinator.ensureLeadership(
            username,
            () => {
              // Leadership lost callback — disconnect this stream
              logger.warn('Leadership lost, disconnecting stream', { username });
              this.disconnectFromStream(username).catch(err => {
                logger.error('Failed to disconnect after leadership loss', {
                  username, error: String(err),
                });
              });
            },
          );
          if (!acquired) {
            logger.debug('Skipping stream (another pod has leadership)', { username });
            continue;
          }
        }
        await this.connectToStream(username, source.overlay_id);
      }
    }
  }

  /**
   * Safety-net poll that queries source-manager GET /demand endpoint.
   * Runs every 60s to restore correct state after Redis reconnect / missed Pub/Sub events.
   * Only runs when coordinator integration is enabled.
   */
  private async pollDemandFallback(): Promise<void> {
    if (this.isShuttingDown) return;
    if (!this.sourceManagerClient) return;

    try {
      const sources = await this.sourceManagerClient.getDemand('tiktok');
      const demanded = new Map<string, DemandSource>();
      for (const source of sources) {
        demanded.set(source.channel_id, source);
      }
      await this.handleDemandUpdate(demanded);
    } catch (err) {
      logger.error('Demand fallback poll error', { error: String(err) });
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
  private async connectToStream(username: string, overlayId: string): Promise<void> {
    // Reject malformed usernames before any connection attempt (audit L24).
    const usernameCheck = validateTikTokUsername(username);
    if (!usernameCheck.valid) {
      logger.warn('Rejecting connection to invalid TikTok username', {
        username,
        overlay_id: overlayId,
        reason: usernameCheck.reason,
      });
      return;
    }

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

      // Demand may have been removed while we awaited the live-status check above
      // (handleDemandUpdate runs on the same event loop and updates demandedUsernames).
      // Abort before going active or re-parking in the poller so we don't leave an
      // orphaned connection behind. Release any leadership lease we hold (no-op if none).
      if (!this.demandedUsernames.has(username)) {
        logger.info('Demand removed during connection attempt, aborting', { username, overlay_id: overlayId });
        if (this.leadershipCoordinator) {
          await this.leadershipCoordinator.release(username);
        }
        return;
      }

      if (!statusResult.isLive) {
        // Distinguish between "user is offline" and "API/network error"
        if (statusResult.error) {
          logger.info('Live status check failed (API error), adding to poller with error backoff', {
            username,
            overlay_id: overlayId
          });
          // Record as connection error so error backoff is used (faster retry than offline backoff)
          this.backoffManager.recordConnectionError(username, statusResult.error);
        } else {
          logger.info('User not live, adding to poller and skipping connection', {
            username,
            overlay_id: overlayId
          });
          // Record offline check for backoff
          this.backoffManager.recordOfflineCheck(username);
        }

        // Add to polling targets for re-check
        this.livePoller.addTarget(username, overlayId);

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

      connection.on(WebcastEvent.ENVELOPE, (data) => {
        this.handleTreasureBox(username, overlayId, data);
      });

      // Cast to EventEmitter for lifecycle events not in ClientEventMap
      const emitter = connection as unknown as EventEmitter;

      // Record every frame the connector managed to decode, keyed by its protobuf method name,
      // giving a baseline of what TikTok actually sends on a working connection.
      //
      // Note the limit, because it is easy to over-read this metric: `decodedData` fires only for
      // frames that decoded. deserializeMessage skips a method missing from its schema with a bare
      // `continue`, and a method that throws while decoding is dropped too (its decodeError is set,
      // then the caller skips anything without decodedData). So an absent method here means one of
      // three things — TikTok stopped sending it, TikTok renamed it, or it no longer decodes — and
      // this metric alone does not separate them. The only hook that does is the connector's
      // DEBUG_DESERIALIZE_XD env var, which console.logs unknown method names.
      //
      // What it does buy: if WebcastEnvelopeMessage never appears while chests are visibly dropped
      // in the room, the envelope is definitively not reaching us in decodable form, which is the
      // question that took a prod investigation to answer the first time.
      emitter.on('decodedData', (method: string) => {
        this.metrics.recordWireMessage(method);
      });

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

      // Only schedule a retry if the stream is still demanded. If demand was pulled
      // while we were connecting, re-parking it in the poller would re-create the
      // orphan we are trying to avoid — release leadership instead.
      if (this.demandedUsernames.has(username)) {
        this.livePoller.addTarget(username, overlayId);
      } else if (this.leadershipCoordinator) {
        await this.leadershipCoordinator.release(username);
      }

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

  private async handleChatMessage(username: string, overlayId: string, data: TikTokChatData): Promise<void> {
    try {
      // Record message for heartbeat monitoring
      this.heartbeatMonitor.recordMessage(username);

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

      const text = data.content || '';

      // Check for duplicate messages (prevents replay on reconnect)
      if (this.messageDeduplicator.isDuplicate(msgId, username, text)) {
        logger.debug('Skipping duplicate message', {
          msg_id: msgId,
          username,
          text_preview: text.substring(0, 50)
        });
        return; // Skip publishing duplicate
      }

      // Create raw message in standardized format
      const rawMessage: RawChatMessage = {
        message_id: msgId || randomUUID(), // Use TikTok's msgId, fallback to UUID
        platform: 'tiktok',
        channel_id: username,
        stream_id: undefined, // TikTok doesn't provide stream ID via unofficial lib
        user_id: tiktokUserHandle(data.user),
        username: data.user?.nickname || data.user?.displayId || 'Anonymous',
        text,
        timestamp: timestamp, // Use TikTok's native timestamp
        tags: {
          overlay_id: overlayId,
          user_unique_id: data.user?.displayId || '',
          profile_picture_url: tiktokAvatarUrl(data.user),
          is_follower: (data.userIdentity?.isFollowerOfAnchor ?? false).toString(),
          is_subscriber: (data.userIdentity?.isSubscriberOfAnchor ?? false).toString(),
          badge_level: '0', // No per-user badge level in the v3 chat payload
          native_msg_id: msgId || '', // Store native ID for reference
          native_create_time: createTime || '' // Store native timestamp for reference
        }
      };

      // Publish to Redis Stream (MAXLEN ~100000 keeps stream bounded)
      await this.redis.xAdd('chat:raw', '*', {
        data: JSON.stringify(rawMessage)
      }, { TRIM: { strategy: 'MAXLEN', strategyModifier: '~', threshold: 100000 } });

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

  private async handleGift(username: string, overlayId: string, data: TikTokGiftData): Promise<void> {
    try {
      this.heartbeatMonitor.recordMessage(username);

      // Streakable gifts (gift.type === 1) fire GIFT repeatedly while the combo
      // is in progress; only the final frame (repeatEnd) carries the complete
      // repeatCount. Publishing the intermediate frames would show the same gift
      // multiple times (e.g. a single Rose renders twice). Skip until the streak ends.
      if (data.gift?.type === 1 && !data.repeatEnd) {
        return;
      }

      const msgId = data.common?.msgId || randomUUID();
      const createTime = data.common?.createTime;
      const timestamp = createTime ? new Date(parseInt(createTime)).toISOString() : new Date().toISOString();

      const rawMessage: RawChatMessage = {
        message_id: msgId,
        platform: 'tiktok',
        channel_id: username,
        user_id: tiktokUserHandle(data.user),
        username: data.user?.nickname || 'Anonymous',
        text: `Sent ${data.gift?.name || 'a gift'}`,
        timestamp: timestamp,
        tags: {
          overlay_id: overlayId,
          user_unique_id: data.user?.displayId || '',
          profile_picture_url: tiktokAvatarUrl(data.user)
        },
        event_type: 'gift',
        event_data: {
          gift_id: data.giftId,
          gift_name: data.gift?.name,
          gift_type: data.gift?.type, // 1 = normal, 2 = streakable
          gift_count: data.repeatCount || 1,
          diamond_count: (data.gift?.diamondCount || 0) * (data.repeatCount || 1)
        }
      };

      await this.redis.xAdd('chat:raw', '*', { data: JSON.stringify(rawMessage) }, { TRIM: { strategy: 'MAXLEN', strategyModifier: '~', threshold: 100000 } });
      logger.debug('Published TikTok gift event', { username, overlay_id: overlayId, gift: data.gift?.name });
    } catch (error) {
      logger.error('Failed to handle gift event', { username, error });
    }
  }

  private async handleLike(username: string, overlayId: string, data: TikTokLikeData): Promise<void> {
    try {
      this.heartbeatMonitor.recordMessage(username);

      const userId = tiktokUserHandle(data.user);
      const userNickname = data.user?.nickname || 'Anonymous';
      const likeCount = data.count || 1;
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

  private async handleFollow(username: string, overlayId: string, data: TikTokSocialData): Promise<void> {
    try {
      this.heartbeatMonitor.recordMessage(username);

      const msgId = data.common?.msgId || randomUUID();
      const createTime = data.common?.createTime;
      const timestamp = createTime ? new Date(parseInt(createTime)).toISOString() : new Date().toISOString();

      const rawMessage: RawChatMessage = {
        message_id: msgId,
        platform: 'tiktok',
        channel_id: username,
        user_id: tiktokUserHandle(data.user),
        username: data.user?.nickname || 'Anonymous',
        text: 'Followed',
        timestamp: timestamp,
        tags: {
          overlay_id: overlayId,
          user_unique_id: data.user?.displayId || '',
          profile_picture_url: tiktokAvatarUrl(data.user)
        },
        event_type: 'follow',
        event_data: {}
      };

      await this.redis.xAdd('chat:raw', '*', { data: JSON.stringify(rawMessage) }, { TRIM: { strategy: 'MAXLEN', strategyModifier: '~', threshold: 100000 } });
      logger.debug('Published TikTok follow event', { username, overlay_id: overlayId });
    } catch (error) {
      logger.error('Failed to handle follow event', { username, error });
    }
  }

  private async handleShare(username: string, overlayId: string, data: TikTokSocialData): Promise<void> {
    try {
      this.heartbeatMonitor.recordMessage(username);

      const msgId = data.common?.msgId || randomUUID();
      const createTime = data.common?.createTime;
      const timestamp = createTime ? new Date(parseInt(createTime)).toISOString() : new Date().toISOString();

      const rawMessage: RawChatMessage = {
        message_id: msgId,
        platform: 'tiktok',
        channel_id: username,
        user_id: tiktokUserHandle(data.user),
        username: data.user?.nickname || 'Anonymous',
        text: 'Shared the stream',
        timestamp: timestamp,
        tags: {
          overlay_id: overlayId,
          user_unique_id: data.user?.displayId || '',
          profile_picture_url: tiktokAvatarUrl(data.user)
        },
        event_type: 'share',
        event_data: {}
      };

      await this.redis.xAdd('chat:raw', '*', { data: JSON.stringify(rawMessage) }, { TRIM: { strategy: 'MAXLEN', strategyModifier: '~', threshold: 100000 } });
      logger.debug('Published TikTok share event', { username, overlay_id: overlayId });
    } catch (error) {
      logger.error('Failed to handle share event', { username, error });
    }
  }

  private async handleTreasureBox(username: string, overlayId: string, data: TikTokEnvelopeData): Promise<void> {
    try {
      this.heartbeatMonitor.recordMessage(username);

      // Logged at info, unlike the other handlers' per-message debug lines. ENVELOPE frames are
      // rare (a handful per stream at most, against thousands of chats), and a streamer reporting
      // "my chest never appeared" is otherwise unanswerable: every branch below is a silent
      // return, so at the INFO level prod runs at, the whole feature leaves no trace either way.
      logger.info('Received ENVELOPE frame', {
        username,
        overlay_id: overlayId,
        business_type: data.envelopeInfo?.businessType,
        display: data.display,
        has_envelope_info: !!data.envelopeInfo,
        coins: data.envelopeInfo?.diamondCount
      });

      // ENVELOPE also carries Super Fan Box (and other) frames; publishing those
      // would show the streamer a coin chest nobody sent.
      if (!isTikTokCoinChest(data)) {
        this.metrics.recordEnvelopeFrame('super_fan_box');
        logger.debug('Skipping non-chest ENVELOPE event', {
          username,
          business_type: data.envelopeInfo?.businessType
        });
        return;
      }

      // The same chest comes back as a HIDE frame when it expires or is fully
      // claimed; republishing that would render the drop twice in the activity
      // panel and the overlay.
      if (!isTikTokEnvelopeDrop(data)) {
        this.metrics.recordEnvelopeFrame('not_a_drop');
        logger.debug('Skipping non-drop ENVELOPE event', {
          username,
          display: data.display
        });
        return;
      }

      // An ENVELOPE frame can announce no chest at all; publishing that would render a phantom
      // "Sent a coin chest" worth 0 coins from Anonymous. See hasTikTokChestPayload.
      if (!hasTikTokChestPayload(data)) {
        this.metrics.recordEnvelopeFrame('no_chest_payload');
        logger.debug('Skipping ENVELOPE event with no chest payload', {
          username,
          has_envelope_info: !!data.envelopeInfo,
          coins: data.envelopeInfo?.diamondCount
        });
        return;
      }

      const envelope = data.envelopeInfo;
      const coins = envelope.diamondCount;
      // The sender may be absent entirely on some frames; a chest without an
      // identifiable sender must still publish (as Anonymous) rather than throw.
      const senderId = envelope.sendUserId || 'unknown';
      const canOpen = envelope.peopleCount || 0;
      const text = 'Sent a coin chest';

      const msgId = data.common?.msgId;
      const createTime = data.common?.createTime;
      const timestamp = createTime ? new Date(parseInt(createTime)).toISOString() : new Date().toISOString();

      // Key dedup on the envelope's own id rather than the per-frame msgId, so
      // every frame TikTok emits for one chest — a replay after a reconnect, or
      // the same room seen twice across pooled connections — collapses into a
      // single event. The prefix keeps envelope ids out of the msgId namespace
      // the other handlers share. Falls back to msgId, as the chat handler does,
      // when TikTok omits the envelope id.
      const dedupKey = envelope.envelopeId ? `envelope:${envelope.envelopeId}` : msgId;
      if (this.messageDeduplicator.isDuplicate(dedupKey, username, text)) {
        this.metrics.recordEnvelopeFrame('duplicate');
        logger.debug('Skipping duplicate treasure chest event', { msg_id: msgId, username });
        return;
      }

      const rawMessage: RawChatMessage = {
        message_id: msgId || randomUUID(),
        platform: 'tiktok',
        channel_id: username,
        user_id: senderId,
        username: envelope.sendUserName || 'Anonymous',
        text,
        timestamp: timestamp,
        tags: {
          overlay_id: overlayId,
          user_unique_id: '', // no @handle in the envelope payload; the normalizer falls back to user_id
          profile_picture_url: pickAvatarUrl(envelope.sendUserAvatar)
        },
        event_type: 'treasure_chest',
        event_data: {
          coins,
          can_open: canOpen
        }
      };

      await this.redis.xAdd('chat:raw', '*', { data: JSON.stringify(rawMessage) }, { TRIM: { strategy: 'MAXLEN', strategyModifier: '~', threshold: 100000 } });
      // Logs the decoded fields, not just the outcome: `tiktok-live-proto/v3` is a
      // package subpath this tsconfig cannot resolve, so a wire-format rename here
      // compiles clean and only shows up as zeroed values at runtime.
      this.metrics.recordEnvelopeFrame('published');
      logger.info('Published TikTok treasure chest event', {
        username,
        overlay_id: overlayId,
        business_type: envelope.businessType,
        sender: senderId,
        coins,
        can_open: canOpen
      });
    } catch (error) {
      this.metrics.recordEnvelopeFrame('error');
      logger.error('Failed to handle treasure chest event', { username, error });
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
            await this.redis.xAdd('chat:raw', '*', { data: JSON.stringify(rawMessage) }, { TRIM: { strategy: 'MAXLEN', strategyModifier: '~', threshold: 100000 } });

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

    // Release all leadership leases
    if (this.leadershipCoordinator) {
      await this.leadershipCoordinator.stop();
      logger.info('Leadership leases released');
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
