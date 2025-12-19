/**
 * Prometheus Metrics for TikTok Listener
 *
 * Provides metrics for monitoring connection health, message processing,
 * circuit breaker state, and error classification.
 */

import { Counter, Gauge, Histogram, Registry } from 'prom-client';
import winston from 'winston';

export class PrometheusMetrics {
  private registry: Registry;
  private logger: winston.Logger;

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

  constructor(logger: winston.Logger) {
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

  // Cleanup methods
  clearMetricsForUsername(username: string): void {
    // Clear all labeled metrics for a username
    this.heartbeatLastMessage.remove({ username });
    this.messagesQueued.remove({ username });
    this.messageQueueSize.remove({ username });
    this.circuitBreakerState.remove({ username });
    this.connectionSubscribers.remove({ username });

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
