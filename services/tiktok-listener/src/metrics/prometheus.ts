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
 * Prometheus Metrics for TikTok Listener
 *
 * Provides metrics for monitoring connection health, message processing,
 * circuit breaker state, and error classification.
 */

import { Counter, Gauge, Histogram, Registry } from 'prom-client';
import { Logger } from '../types/logger.js';

export class PrometheusMetrics {
  private registry: Registry;
  private logger: Logger;

  // Connection health metrics
  private heartbeatTimeouts: Counter<string>;
  private heartbeatLastMessage: Gauge<string>;

  // Message processing metrics
  private messagesQueued: Gauge<string>;
  private messagesDropped: Counter<string>;
  private messageQueueSize: Gauge<string>;

  // Circuit breaker metrics
  private circuitBreakerState: Gauge<string>;
  private circuitBreakerTrips: Counter<string>;

  // Connection pooling metrics
  private pooledConnections: Gauge;
  private connectionSubscribers: Gauge<string>;

  // Error classification metrics
  private errorsByType: Counter<string>;

  // Backoff and detection metrics (NEW)
  private backoffCurrentInterval: Gauge<string>;
  private backoffUsernamesStuck: Gauge;
  private detectionSkippedTotal: Counter<string>;
  private usernamesAtRisk: Gauge<string>;
  private autoRecoveryTotal: Counter<string>;

  constructor(logger: Logger) {
    this.logger = logger;
    this.registry = new Registry();

    // Initialize connection health metrics
    this.heartbeatTimeouts = new Counter({
      name: 'tiktok_heartbeat_timeouts_total',
      help: 'Total number of heartbeat timeouts detected',
      labelNames: ['username'],
      registers: [this.registry]
    });

    this.heartbeatLastMessage = new Gauge({
      name: 'tiktok_heartbeat_last_message_timestamp',
      help: 'Unix timestamp of last message received',
      labelNames: ['username'],
      registers: [this.registry]
    });

    // Initialize message processing metrics
    this.messagesQueued = new Gauge({
      name: 'tiktok_messages_queued',
      help: 'Current number of messages queued for processing',
      labelNames: ['username'],
      registers: [this.registry]
    });

    this.messagesDropped = new Counter({
      name: 'tiktok_messages_dropped_total',
      help: 'Total number of messages dropped',
      labelNames: ['username', 'reason'],
      registers: [this.registry]
    });

    this.messageQueueSize = new Gauge({
      name: 'tiktok_message_queue_size',
      help: 'Current size of message queue per username',
      labelNames: ['username'],
      registers: [this.registry]
    });

    // Initialize circuit breaker metrics
    this.circuitBreakerState = new Gauge({
      name: 'tiktok_circuit_breaker_state',
      help: 'Circuit breaker state (0=closed, 1=open)',
      labelNames: ['username'],
      registers: [this.registry]
    });

    this.circuitBreakerTrips = new Counter({
      name: 'tiktok_circuit_breaker_trips_total',
      help: 'Total number of circuit breaker trips',
      labelNames: ['username'],
      registers: [this.registry]
    });

    // Initialize connection pooling metrics
    this.pooledConnections = new Gauge({
      name: 'tiktok_pooled_connections',
      help: 'Current number of pooled connections',
      registers: [this.registry]
    });

    this.connectionSubscribers = new Gauge({
      name: 'tiktok_connection_subscribers',
      help: 'Number of subscribers per pooled connection',
      labelNames: ['username'],
      registers: [this.registry]
    });

    // Initialize error classification metrics
    this.errorsByType = new Counter({
      name: 'tiktok_errors_by_type_total',
      help: 'Total number of errors by type',
      labelNames: ['username', 'type'],
      registers: [this.registry]
    });

    // Initialize backoff and detection metrics (NEW)
    this.backoffCurrentInterval = new Gauge({
      name: 'tiktok_backoff_current_interval_ms',
      help: 'Current backoff interval per username in milliseconds',
      labelNames: ['username'],
      registers: [this.registry]
    });

    this.backoffUsernamesStuck = new Gauge({
      name: 'tiktok_backoff_usernames_stuck',
      help: 'Number of usernames stuck in backoff >5 minutes',
      registers: [this.registry]
    });

    this.detectionSkippedTotal = new Counter({
      name: 'tiktok_detection_skipped_total',
      help: 'Detections skipped by reason',
      labelNames: ['reason'], // backoff, error, offline
      registers: [this.registry]
    });

    this.usernamesAtRisk = new Gauge({
      name: 'tiktok_usernames_at_risk',
      help: 'Usernames with long backoff (risk level)',
      labelNames: ['risk_level'], // high, medium, low
      registers: [this.registry]
    });

    this.autoRecoveryTotal = new Counter({
      name: 'tiktok_auto_recovery_total',
      help: 'Automatic stuck state recoveries',
      labelNames: ['username', 'reason'], // max_backoff_stuck
      registers: [this.registry]
    });

    this.logger.info('Prometheus metrics initialized');
  }

  // Heartbeat monitoring methods
  recordHeartbeatTimeout(username: string): void {
    this.heartbeatTimeouts.inc({ username });
  }

  recordHeartbeatMessage(username: string, timestamp: number): void {
    this.heartbeatLastMessage.set({ username }, timestamp);
  }

  // Message processing methods
  recordMessageQueued(username: string, queueSize: number): void {
    this.messagesQueued.set({ username }, queueSize);
    this.messageQueueSize.set({ username }, queueSize);
  }

  recordMessageDropped(username: string, reason: string): void {
    this.messagesDropped.inc({ username, reason });
  }

  // Circuit breaker methods
  recordCircuitBreakerState(username: string, state: number): void {
    this.circuitBreakerState.set({ username }, state);
  }

  recordCircuitBreakerTrip(username: string): void {
    this.circuitBreakerTrips.inc({ username });
  }

  // Connection pooling methods
  recordPooledConnections(count: number): void {
    this.pooledConnections.set(count);
  }

  recordConnectionSubscribers(username: string, count: number): void {
    this.connectionSubscribers.set({ username }, count);
  }

  // Error classification methods
  recordError(username: string, errorType: string): void {
    this.errorsByType.inc({ username, type: errorType });
  }

  // Backoff and detection methods (NEW)
  recordBackoffInterval(username: string, intervalMs: number): void {
    this.backoffCurrentInterval.set({ username }, intervalMs);
  }

  setBackoffUsernamesStuck(count: number): void {
    this.backoffUsernamesStuck.set(count);
  }

  recordDetectionSkipped(reason: string): void {
    this.detectionSkippedTotal.inc({ reason });
  }

  setUsernamesAtRisk(riskLevel: string, count: number): void {
    this.usernamesAtRisk.set({ risk_level: riskLevel }, count);
  }

  recordAutoRecovery(username: string, reason: string): void {
    this.autoRecoveryTotal.inc({ username, reason });
  }

  // Cleanup methods
  clearMetricsForUsername(username: string): void {
    // Clear all labeled metrics for a username
    this.heartbeatLastMessage.remove({ username });
    this.messagesQueued.remove({ username });
    this.messageQueueSize.remove({ username });
    this.circuitBreakerState.remove({ username });
    this.connectionSubscribers.remove({ username });
    this.backoffCurrentInterval.remove({ username }); // NEW

    this.logger.debug('Cleared metrics for username', { username });
  }

  // Get metrics for Prometheus scraping
  async getMetrics(): Promise<string> {
    return await this.registry.metrics();
  }

  // Get content type for HTTP response
  getContentType(): string {
    return this.registry.contentType;
  }
}
