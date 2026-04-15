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
 * Heartbeat Monitor
 *
 * Monitors TikTok WebSocket connections for silent failures by tracking
 * the last message received timestamp combined with the library's internal
 * connection state.
 *
 * Only forces reconnection when BOTH conditions are true:
 * 1. Library reports connection is established (isConnected && upgradedToWebsocket)
 * 2. No messages received within timeout period (indicating silent failure)
 *
 * This prevents unnecessary reconnections that trigger TikTok's anti-bot
 * measures while still catching genuine silent connection failures.
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
      // Check library's internal connection state before forcing reconnection
      let libraryState: any;
      try {
        // Note: getState() exists but isn't in TypeScript definitions, so we cast to any
        libraryState = (state.connection as any).getState();
      } catch (error) {
        this.logger.warn('Failed to get connection state from library', {
          username,
          error: error instanceof Error ? error.message : String(error)
        });
        // If we can't get state, assume connection is healthy and skip forced reconnection
        return;
      }

      // Only force reconnection if library thinks it's connected but we know it's not receiving data
      // This prevents unnecessary reconnections during normal disconnections or connection attempts
      if (!libraryState.isConnected || !libraryState.upgradedToWebsocket) {
        this.logger.debug('Heartbeat timeout but library reports connection not established - skipping forced reconnection', {
          username,
          silence_duration_ms: silenceDuration,
          library_is_connected: libraryState.isConnected,
          library_upgraded_to_websocket: libraryState.upgradedToWebsocket
        });
        return;
      }

      // Library thinks connection is healthy, but no messages received = silent failure
      this.logger.warn('Silent connection failure detected - forcing reconnection', {
        username,
        silence_duration_ms: silenceDuration,
        silence_duration_seconds: Math.round(silenceDuration / 1000),
        last_message_time: new Date(state.lastMessageTime).toISOString(),
        timeout_threshold_ms: this.HEARTBEAT_TIMEOUT,
        library_state: {
          is_connected: libraryState.isConnected,
          upgraded_to_websocket: libraryState.upgradedToWebsocket,
          room_id: libraryState.roomId
        }
      });

      // Record timeout in metrics
      this.metrics.recordHeartbeatTimeout(username);

      // Force disconnection to trigger reconnection flow
      try {
        state.connection.disconnect();
        this.logger.info('Connection disconnected due to silent failure', { username });
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
