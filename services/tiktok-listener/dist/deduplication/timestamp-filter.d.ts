/**
 * Timestamp Filter
 *
 * Filters out messages that are too old (potentially from reconnection replays).
 * This prevents stale messages from being published when TikTok replays
 * historical messages after a connection restart.
 */
import winston from 'winston';
/**
 * Configuration for the timestamp filter
 */
export interface TimestampFilterConfig {
    /** Maximum age of messages to accept in milliseconds (default: 60 seconds) */
    maxAgeMs: number;
}
/**
 * TimestampFilter rejects messages older than a configured threshold
 */
export declare class TimestampFilter {
    private logger;
    private readonly MAX_AGE_MS;
    private droppedCount;
    private acceptedCount;
    /**
     * @param logger Winston logger instance
     * @param config Optional filter configuration
     */
    constructor(logger: winston.Logger, config?: Partial<TimestampFilterConfig>);
    /**
     * Check if a message should be accepted based on its timestamp
     *
     * @param timestamp Message timestamp (ISO string or Unix timestamp in ms)
     * @param context Optional context for logging (e.g., username)
     * @returns true if message should be accepted, false if too old
     */
    shouldAccept(timestamp: string | number, context?: {
        username?: string;
        text?: string;
    }): boolean;
    /**
     * Get filter statistics
     */
    getStats(): {
        acceptedCount: number;
        droppedCount: number;
        totalProcessed: number;
        dropRatePercent: number;
        maxAgeMs: number;
        maxAgeSeconds: number;
    };
    /**
     * Reset statistics
     */
    resetStats(): void;
}
//# sourceMappingURL=timestamp-filter.d.ts.map