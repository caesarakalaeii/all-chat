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
import winston from 'winston';
import { TikTokLiveConnection } from 'tiktok-live-connector';
import { PrometheusMetrics } from '../metrics/prometheus.js';
export declare class HeartbeatMonitor {
    private logger;
    private metrics;
    private monitors;
    private readonly HEARTBEAT_INTERVAL;
    private readonly HEARTBEAT_TIMEOUT;
    /**
     * @param logger Winston logger instance
     * @param metrics Prometheus metrics instance
     * @param heartbeatIntervalMs How often to check for heartbeat (default: 30s)
     * @param heartbeatTimeoutMs How long to wait before considering connection dead (default: 90s)
     */
    constructor(logger: winston.Logger, metrics: PrometheusMetrics, heartbeatIntervalMs?: number, heartbeatTimeoutMs?: number);
    /**
     * Start monitoring a connection
     *
     * @param username TikTok username
     * @param connection TikTokLiveConnection instance
     */
    start(username: string, connection: TikTokLiveConnection): void;
    /**
     * Record that a message was received
     *
     * @param username TikTok username
     */
    recordMessage(username: string): void;
    /**
     * Stop monitoring a connection
     *
     * @param username TikTok username
     */
    stop(username: string): void;
    /**
     * Check heartbeat for a specific username
     *
     * @private
     */
    private checkHeartbeat;
    /**
     * Get current monitor statistics
     */
    getStats(): {
        total_monitored: number;
        usernames: {
            username: string;
            last_message_seconds_ago: number;
            is_healthy: boolean;
        }[];
    };
    /**
     * Stop all monitors (for graceful shutdown)
     */
    stopAll(): void;
}
//# sourceMappingURL=heartbeat-monitor.d.ts.map