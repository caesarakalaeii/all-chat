/**
 * Prometheus Metrics for TikTok Listener
 *
 * Provides metrics for monitoring connection health, message processing,
 * circuit breaker state, and error classification.
 */
import { Counter, Gauge, Registry } from 'prom-client';
export class PrometheusMetrics {
    registry;
    logger;
    // Connection health metrics
    heartbeatTimeouts;
    heartbeatLastMessage;
    // Message processing metrics
    messagesQueued;
    messagesDropped;
    messageQueueSize;
    // Circuit breaker metrics
    circuitBreakerState;
    circuitBreakerTrips;
    // Connection pooling metrics
    pooledConnections;
    connectionSubscribers;
    // Error classification metrics
    errorsByType;
    constructor(logger) {
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
    recordHeartbeatTimeout(username) {
        this.heartbeatTimeouts.inc({ username });
    }
    recordHeartbeatMessage(username, timestamp) {
        this.heartbeatLastMessage.set({ username }, timestamp);
    }
    // Message processing methods
    recordMessageQueued(username, queueSize) {
        this.messagesQueued.set({ username }, queueSize);
        this.messageQueueSize.set({ username }, queueSize);
    }
    recordMessageDropped(username, reason) {
        this.messagesDropped.inc({ username, reason });
    }
    // Circuit breaker methods
    recordCircuitBreakerState(username, state) {
        this.circuitBreakerState.set({ username }, state);
    }
    recordCircuitBreakerTrip(username) {
        this.circuitBreakerTrips.inc({ username });
    }
    // Connection pooling methods
    recordPooledConnections(count) {
        this.pooledConnections.set(count);
    }
    recordConnectionSubscribers(username, count) {
        this.connectionSubscribers.set({ username }, count);
    }
    // Error classification methods
    recordError(username, errorType) {
        this.errorsByType.inc({ username, type: errorType });
    }
    // Cleanup methods
    clearMetricsForUsername(username) {
        // Clear all labeled metrics for a username
        this.heartbeatLastMessage.remove({ username });
        this.messagesQueued.remove({ username });
        this.messageQueueSize.remove({ username });
        this.circuitBreakerState.remove({ username });
        this.connectionSubscribers.remove({ username });
        this.logger.debug('Cleared metrics for username', { username });
    }
    // Get metrics for Prometheus scraping
    async getMetrics() {
        return await this.registry.metrics();
    }
    // Get content type for HTTP response
    getContentType() {
        return this.registry.contentType;
    }
}
//# sourceMappingURL=prometheus.js.map