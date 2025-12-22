/**
 * Connection Pool Manager
 *
 * Manages TikTok WebSocket connections by pooling connections for the same username.
 * Multiple overlays monitoring the same TikTok user share a single connection,
 * significantly reducing resource usage.
 *
 * Benefits:
 * - Reduced memory usage (one connection per username vs per overlay)
 * - Reduced network traffic
 * - Better resilience (shared connection management)
 */
import winston from 'winston';
/**
 * Subscriber for a pooled connection
 */
interface ConnectionSubscriber {
    overlayId: string;
    onMessage: (data: any) => void;
    onConnected?: (state: any) => void;
    onDisconnected?: () => void;
    onError?: (error: Error) => void;
}
/**
 * Configuration for connection pool
 */
export interface ConnectionPoolConfig {
    /** Idle timeout before closing unused connections (default: 5 minutes) */
    idleTimeoutMs: number;
    /** How often to check for idle connections (default: 1 minute) */
    cleanupIntervalMs: number;
}
/**
 * ConnectionPoolManager manages shared TikTok connections
 */
export declare class ConnectionPoolManager {
    private logger;
    private connections;
    private cleanupTimer?;
    private readonly IDLE_TIMEOUT_MS;
    private readonly CLEANUP_INTERVAL_MS;
    /**
     * @param logger Winston logger instance
     * @param config Optional pool configuration
     */
    constructor(logger: winston.Logger, config?: Partial<ConnectionPoolConfig>);
    /**
     * Start periodic cleanup of idle connections
     */
    start(): void;
    /**
     * Stop periodic cleanup
     */
    stop(): void;
    /**
     * Subscribe to a TikTok username's messages
     *
     * @param username TikTok username
     * @param overlayId Overlay ID subscribing
     * @param subscriber Subscriber callbacks
     * @returns Promise that resolves when connected
     */
    subscribe(username: string, overlayId: string, subscriber: ConnectionSubscriber): Promise<void>;
    /**
     * Unsubscribe from a TikTok username's messages
     *
     * @param username TikTok username
     * @param overlayId Overlay ID unsubscribing
     */
    unsubscribe(username: string, overlayId: string): void;
    /**
     * Get number of active connections
     */
    getConnectionCount(): number;
    /**
     * Get number of subscribers for a username
     */
    getSubscriberCount(username: string): number;
    /**
     * Check if a connection exists for a username
     */
    hasConnection(username: string): boolean;
    /**
     * Get pool statistics
     */
    getStats(): {
        totalConnections: number;
        connectedCount: number;
        totalSubscribers: number;
        avgSubscribersPerConnection: number;
        connections: {
            username: string;
            subscribers: number;
            isConnected: boolean;
            idleTimeMs: number;
        }[];
    };
    /**
     * Disconnect all connections (for graceful shutdown)
     */
    disconnectAll(): Promise<void>;
    /**
     * Create a new pooled connection
     *
     * @private
     */
    private createConnection;
    /**
     * Handle message from pooled connection
     *
     * @private
     */
    private handleMessage;
    /**
     * Handle connection established
     *
     * @private
     */
    private handleConnected;
    /**
     * Handle connection disconnected
     *
     * @private
     */
    private handleDisconnected;
    /**
     * Handle connection error
     *
     * @private
     */
    private handleError;
    /**
     * Clean up idle connections with no subscribers
     *
     * @private
     */
    private cleanupIdleConnections;
}
export {};
//# sourceMappingURL=pool-manager.d.ts.map