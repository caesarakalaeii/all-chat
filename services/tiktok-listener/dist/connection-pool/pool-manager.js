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
import { TikTokLiveConnection, WebcastEvent } from 'tiktok-live-connector';
/**
 * ConnectionPoolManager manages shared TikTok connections
 */
export class ConnectionPoolManager {
    logger;
    connections = new Map();
    cleanupTimer;
    IDLE_TIMEOUT_MS;
    CLEANUP_INTERVAL_MS;
    /**
     * @param logger Winston logger instance
     * @param config Optional pool configuration
     */
    constructor(logger, config) {
        this.logger = logger;
        // Apply configuration with defaults
        this.IDLE_TIMEOUT_MS = config?.idleTimeoutMs ?? 300000; // 5 minutes
        this.CLEANUP_INTERVAL_MS = config?.cleanupIntervalMs ?? 60000; // 1 minute
        this.logger.info('ConnectionPoolManager initialized', {
            idle_timeout_ms: this.IDLE_TIMEOUT_MS,
            cleanup_interval_ms: this.CLEANUP_INTERVAL_MS
        });
    }
    /**
     * Start periodic cleanup of idle connections
     */
    start() {
        if (this.cleanupTimer) {
            this.logger.warn('Connection pool cleanup already running');
            return;
        }
        this.cleanupTimer = setInterval(() => {
            this.cleanupIdleConnections();
        }, this.CLEANUP_INTERVAL_MS);
        this.logger.info('Connection pool cleanup started');
    }
    /**
     * Stop periodic cleanup
     */
    stop() {
        if (this.cleanupTimer) {
            clearInterval(this.cleanupTimer);
            this.cleanupTimer = undefined;
            this.logger.info('Connection pool cleanup stopped');
        }
    }
    /**
     * Subscribe to a TikTok username's messages
     *
     * @param username TikTok username
     * @param overlayId Overlay ID subscribing
     * @param subscriber Subscriber callbacks
     * @returns Promise that resolves when connected
     */
    async subscribe(username, overlayId, subscriber) {
        // Get or create pooled connection
        let pooled = this.connections.get(username);
        if (!pooled) {
            // Create new connection
            this.logger.info('Creating new pooled connection', { username });
            pooled = await this.createConnection(username);
            this.connections.set(username, pooled);
        }
        // Add subscriber
        pooled.subscribers.set(overlayId, subscriber);
        pooled.lastActivity = Date.now();
        this.logger.info('Subscriber added to pooled connection', {
            username,
            overlay_id: overlayId,
            total_subscribers: pooled.subscribers.size
        });
        // If already connected, notify subscriber immediately
        if (pooled.isConnected && subscriber.onConnected) {
            subscriber.onConnected({ roomId: undefined });
        }
    }
    /**
     * Unsubscribe from a TikTok username's messages
     *
     * @param username TikTok username
     * @param overlayId Overlay ID unsubscribing
     */
    unsubscribe(username, overlayId) {
        const pooled = this.connections.get(username);
        if (!pooled) {
            this.logger.warn('Cannot unsubscribe from non-existent connection', {
                username,
                overlay_id: overlayId
            });
            return;
        }
        // Remove subscriber
        pooled.subscribers.delete(overlayId);
        pooled.lastActivity = Date.now();
        this.logger.info('Subscriber removed from pooled connection', {
            username,
            overlay_id: overlayId,
            remaining_subscribers: pooled.subscribers.size
        });
        // If no more subscribers, mark for cleanup (but don't close immediately)
        if (pooled.subscribers.size === 0) {
            this.logger.info('No subscribers remaining, connection will be cleaned up if idle', {
                username,
                idle_timeout_ms: this.IDLE_TIMEOUT_MS
            });
        }
    }
    /**
     * Get number of active connections
     */
    getConnectionCount() {
        return this.connections.size;
    }
    /**
     * Get number of subscribers for a username
     */
    getSubscriberCount(username) {
        const pooled = this.connections.get(username);
        return pooled ? pooled.subscribers.size : 0;
    }
    /**
     * Check if a connection exists for a username
     */
    hasConnection(username) {
        return this.connections.has(username);
    }
    /**
     * Get pool statistics
     */
    getStats() {
        const connections = Array.from(this.connections.values());
        const totalSubscribers = connections.reduce((sum, c) => sum + c.subscribers.size, 0);
        const connectedCount = connections.filter(c => c.isConnected).length;
        const connectionDetails = connections.map(c => ({
            username: c.username,
            subscribers: c.subscribers.size,
            isConnected: c.isConnected,
            idleTimeMs: Date.now() - c.lastActivity
        }));
        return {
            totalConnections: this.connections.size,
            connectedCount,
            totalSubscribers,
            avgSubscribersPerConnection: connections.length > 0
                ? Math.round((totalSubscribers / connections.length) * 100) / 100
                : 0,
            connections: connectionDetails
        };
    }
    /**
     * Disconnect all connections (for graceful shutdown)
     */
    async disconnectAll() {
        this.logger.info('Disconnecting all pooled connections', {
            count: this.connections.size
        });
        for (const [username, pooled] of this.connections) {
            try {
                pooled.connection.disconnect();
                this.logger.debug('Disconnected pooled connection', { username });
            }
            catch (error) {
                this.logger.error('Failed to disconnect pooled connection', {
                    username,
                    error: error instanceof Error ? error.message : String(error)
                });
            }
        }
        this.connections.clear();
        this.logger.info('All pooled connections disconnected');
    }
    /**
     * Create a new pooled connection
     *
     * @private
     */
    async createConnection(username) {
        const connection = new TikTokLiveConnection(username, {
            processInitialData: false,
            enableExtendedGiftInfo: false
        });
        const pooled = {
            username,
            connection,
            subscribers: new Map(),
            isConnected: false,
            lastActivity: Date.now()
        };
        // Set up event handlers
        connection.on(WebcastEvent.CHAT, (data) => {
            this.handleMessage(username, data);
        });
        // Cast to EventEmitter for lifecycle events
        const emitter = connection;
        emitter.on('connected', (state) => {
            this.handleConnected(username, state);
        });
        emitter.on('disconnected', () => {
            this.handleDisconnected(username);
        });
        emitter.on('error', (err) => {
            this.handleError(username, err);
        });
        // Connect
        await connection.connect();
        return pooled;
    }
    /**
     * Handle message from pooled connection
     *
     * @private
     */
    handleMessage(username, data) {
        const pooled = this.connections.get(username);
        if (!pooled)
            return;
        pooled.lastActivity = Date.now();
        // Broadcast to all subscribers
        for (const [overlayId, subscriber] of pooled.subscribers) {
            try {
                subscriber.onMessage(data);
            }
            catch (error) {
                this.logger.error('Subscriber message handler failed', {
                    username,
                    overlay_id: overlayId,
                    error: error instanceof Error ? error.message : String(error)
                });
            }
        }
    }
    /**
     * Handle connection established
     *
     * @private
     */
    handleConnected(username, state) {
        const pooled = this.connections.get(username);
        if (!pooled)
            return;
        pooled.isConnected = true;
        pooled.lastActivity = Date.now();
        this.logger.info('Pooled connection established', {
            username,
            room_id: state.roomId,
            subscribers: pooled.subscribers.size
        });
        // Notify all subscribers
        for (const [overlayId, subscriber] of pooled.subscribers) {
            if (subscriber.onConnected) {
                try {
                    subscriber.onConnected(state);
                }
                catch (error) {
                    this.logger.error('Subscriber connected handler failed', {
                        username,
                        overlay_id: overlayId,
                        error: error instanceof Error ? error.message : String(error)
                    });
                }
            }
        }
    }
    /**
     * Handle connection disconnected
     *
     * @private
     */
    handleDisconnected(username) {
        const pooled = this.connections.get(username);
        if (!pooled)
            return;
        pooled.isConnected = false;
        pooled.lastActivity = Date.now();
        this.logger.warn('Pooled connection disconnected', {
            username,
            subscribers: pooled.subscribers.size
        });
        // Notify all subscribers
        for (const [overlayId, subscriber] of pooled.subscribers) {
            if (subscriber.onDisconnected) {
                try {
                    subscriber.onDisconnected();
                }
                catch (error) {
                    this.logger.error('Subscriber disconnected handler failed', {
                        username,
                        overlay_id: overlayId,
                        error: error instanceof Error ? error.message : String(error)
                    });
                }
            }
        }
    }
    /**
     * Handle connection error
     *
     * @private
     */
    handleError(username, err) {
        const pooled = this.connections.get(username);
        if (!pooled)
            return;
        pooled.lastActivity = Date.now();
        this.logger.error('Pooled connection error', {
            username,
            error: err.message,
            subscribers: pooled.subscribers.size
        });
        // Notify all subscribers
        for (const [overlayId, subscriber] of pooled.subscribers) {
            if (subscriber.onError) {
                try {
                    subscriber.onError(err);
                }
                catch (error) {
                    this.logger.error('Subscriber error handler failed', {
                        username,
                        overlay_id: overlayId,
                        error: error instanceof Error ? error.message : String(error)
                    });
                }
            }
        }
    }
    /**
     * Clean up idle connections with no subscribers
     *
     * @private
     */
    cleanupIdleConnections() {
        const now = Date.now();
        let cleaned = 0;
        for (const [username, pooled] of this.connections) {
            // Skip if has active subscribers
            if (pooled.subscribers.size > 0) {
                continue;
            }
            // Check if idle
            const idleTime = now - pooled.lastActivity;
            if (idleTime >= this.IDLE_TIMEOUT_MS) {
                this.logger.info('Cleaning up idle pooled connection', {
                    username,
                    idle_time_ms: idleTime,
                    idle_time_minutes: Math.round(idleTime / 60000)
                });
                try {
                    pooled.connection.disconnect();
                }
                catch (error) {
                    this.logger.error('Failed to disconnect idle connection', {
                        username,
                        error: error instanceof Error ? error.message : String(error)
                    });
                }
                this.connections.delete(username);
                cleaned++;
            }
        }
        if (cleaned > 0) {
            this.logger.info('Cleaned up idle connections', {
                cleaned,
                remaining: this.connections.size
            });
        }
    }
}
//# sourceMappingURL=pool-manager.js.map