/**
 * Prometheus Metrics for TikTok Listener
 *
 * Provides metrics for monitoring connection health, message processing,
 * circuit breaker state, and error classification.
 */
import winston from 'winston';
export declare class PrometheusMetrics {
    private registry;
    private logger;
    private heartbeatTimeouts;
    private heartbeatLastMessage;
    private messagesQueued;
    private messagesDropped;
    private messageQueueSize;
    private circuitBreakerState;
    private circuitBreakerTrips;
    private pooledConnections;
    private connectionSubscribers;
    private errorsByType;
    constructor(logger: winston.Logger);
    recordHeartbeatTimeout(username: string): void;
    recordHeartbeatMessage(username: string, timestamp: number): void;
    recordMessageQueued(username: string, queueSize: number): void;
    recordMessageDropped(username: string, reason: string): void;
    recordCircuitBreakerState(username: string, state: number): void;
    recordCircuitBreakerTrip(username: string): void;
    recordPooledConnections(count: number): void;
    recordConnectionSubscribers(username: string, count: number): void;
    recordError(username: string, errorType: string): void;
    clearMetricsForUsername(username: string): void;
    getMetrics(): Promise<string>;
    getContentType(): string;
}
//# sourceMappingURL=prometheus.d.ts.map