/**
 * Heartbeat Monitor
 *
 * Monitors TikTok WebSocket connections for silent failures by tracking
 * the last message received timestamp. If no messages are received within
 * the timeout period, the connection is considered dead and disconnected.
 *
 * This addresses the issue where WebSocket connections can become silently
 * dead without triggering the 'disconnected' event, causing messages to be lost.
 */

import { Logger } from '../types/logger.js';
import { TikTokLiveConnection } from 'tiktok-live-connector';
import { PrometheusMetrics } from '../metrics/prometheus.js';

interface MonitorState {
  username: string;
  connection: TikTokLiveConnection;
  lastMessageTime: number;
  monitorInterval?: NodeJS.Timeout;
}

export class HeartbeatMonitor {
  private logger: Logger;
  private metrics: PrometheusMetrics;
  private monitors: Map<string, MonitorState> = new Map();

  private readonly HEARTBEAT_INTERVAL: number;
  private readonly HEARTBEAT_TIMEOUT: number;

  /**
   * @param logger Winston logger instance
   * @param metrics Prometheus metrics instance
   * @param heartbeatIntervalMs How often to check for heartbeat (default: 30s)
   * @param heartbeatTimeoutMs How long to wait before considering connection dead (default: 90s)
   */
  constructor(
    logger: Logger,
    metrics: PrometheusMetrics,
    heartbeatIntervalMs: number = 30000,
    heartbeatTimeoutMs: number = 90000
  ) {
    this.logger = logger;
    this.metrics = metrics;
    this.HEARTBEAT_INTERVAL = heartbeatIntervalMs;
    this.HEARTBEAT_TIMEOUT = heartbeatTimeoutMs;

    this.logger.info('HeartbeatMonitor initialized', {
      check_interval_ms: this.HEARTBEAT_INTERVAL,
      timeout_ms: this.HEARTBEAT_TIMEOUT
    });
  }

  /**
   * Start monitoring a connection
   *
   * @param username TikTok username
   * @param connection TikTokLiveConnection instance
   */
  start(username: string, connection: TikTokLiveConnection): void {
    // Stop any existing monitor for this username
    this.stop(username);

    const now = Date.now();

    const state: MonitorState = {
      username,
      connection,
      lastMessageTime: now
    };

    // Start periodic heartbeat check
    const monitorInterval = setInterval(() => {
      this.checkHeartbeat(username);
    }, this.HEARTBEAT_INTERVAL);

    state.monitorInterval = monitorInterval;
    this.monitors.set(username, state);

    // Record initial timestamp in metrics
    this.metrics.recordHeartbeatMessage(username, now);

    this.logger.info('Heartbeat monitoring started', {
      username,
      check_interval_ms: this.HEARTBEAT_INTERVAL,
      timeout_ms: this.HEARTBEAT_TIMEOUT
    });
  }

  /**
   * Record that a message was received
   *
   * @param username TikTok username
   */
  recordMessage(username: string): void {
    const state = this.monitors.get(username);
    if (!state) {
      // Not monitoring this username (yet), ignore
      return;
    }

    const now = Date.now();
    state.lastMessageTime = now;

    // Update metrics
    this.metrics.recordHeartbeatMessage(username, now);

    this.logger.debug('Heartbeat message recorded', {
      username,
      timestamp: new Date(now).toISOString()
    });
  }

  /**
   * Stop monitoring a connection
   *
   * @param username TikTok username
   */
  stop(username: string): void {
    const state = this.monitors.get(username);
    if (!state) {
      return;
    }

    // Clear the interval
    if (state.monitorInterval) {
      clearInterval(state.monitorInterval);
    }

    // Remove from monitors
    this.monitors.delete(username);

    // Clear metrics for this username
    this.metrics.clearMetricsForUsername(username);

    this.logger.info('Heartbeat monitoring stopped', { username });
  }

  /**
   * Check heartbeat for a specific username
   *
   * @private
   */
  private checkHeartbeat(username: string): void {
    const state = this.monitors.get(username);
    if (!state) {
      this.logger.warn('Heartbeat check called for unknown username', { username });
      return;
    }

    const now = Date.now();
    const silenceDuration = now - state.lastMessageTime;

    if (silenceDuration > this.HEARTBEAT_TIMEOUT) {
      this.logger.warn('Heartbeat timeout detected - forcing reconnection', {
        username,
        silence_duration_ms: silenceDuration,
        silence_duration_seconds: Math.round(silenceDuration / 1000),
        last_message_time: new Date(state.lastMessageTime).toISOString(),
        timeout_threshold_ms: this.HEARTBEAT_TIMEOUT
      });

      // Record timeout in metrics
      this.metrics.recordHeartbeatTimeout(username);

      // Force disconnection to trigger reconnection flow
      try {
        state.connection.disconnect();
        this.logger.info('Connection disconnected due to heartbeat timeout', { username });
      } catch (error) {
        this.logger.error('Failed to disconnect connection on heartbeat timeout', {
          username,
          error: error instanceof Error ? error.message : String(error)
        });
      }

      // Stop monitoring (will be restarted when connection re-establishes)
      this.stop(username);
    } else {
      this.logger.debug('Heartbeat check passed', {
        username,
        silence_duration_ms: silenceDuration,
        last_message_time: new Date(state.lastMessageTime).toISOString()
      });
    }
  }

  /**
   * Get current monitor statistics
   */
  getStats() {
    const monitors = Array.from(this.monitors.values());
    const now = Date.now();

    return {
      total_monitored: monitors.length,
      usernames: monitors.map(m => ({
        username: m.username,
        last_message_seconds_ago: Math.round((now - m.lastMessageTime) / 1000),
        is_healthy: (now - m.lastMessageTime) < this.HEARTBEAT_TIMEOUT
      }))
    };
  }

  /**
   * Stop all monitors (for graceful shutdown)
   */
  stopAll(): void {
    this.logger.info('Stopping all heartbeat monitors', {
      count: this.monitors.size
    });

    for (const username of Array.from(this.monitors.keys())) {
      this.stop(username);
    }
  }
}
