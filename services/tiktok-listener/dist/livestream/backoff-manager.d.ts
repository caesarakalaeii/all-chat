/**
 * TikTok Backoff Manager
 *
 * Manages exponential backoff for offline checks and connection errors.
 * Implements separate backoff strategies for:
 * - Offline checks: User not streaming (1m → 10m)
 * - Connection errors: Network/API failures (2s → 5m)
 */
import winston from 'winston';
/**
 * Backoff state for a single username
 */
export interface BackoffState {
    username: string;
    consecutiveOfflineChecks: number;
    consecutiveErrors: number;
    lastCheckTime: number;
    nextCheckTime: number;
    currentBackoffMs: number;
    lastSeenLive?: number;
}
/**
 * Configuration for backoff behavior
 */
export interface BackoffConfig {
    baseOfflineBackoffMs: number;
    maxOfflineBackoffMs: number;
    errorBackoffMs: number;
    maxErrorBackoffMs: number;
}
/**
 * BackoffManager handles exponential backoff for TikTok live detection
 */
export declare class BackoffManager {
    private logger;
    private backoffStates;
    private readonly BASE_OFFLINE_BACKOFF_MS;
    private readonly MAX_OFFLINE_BACKOFF_MS;
    private readonly ERROR_BACKOFF_MS;
    private readonly MAX_ERROR_BACKOFF_MS;
    /**
     * @param logger Winston logger instance
     * @param config Optional backoff configuration
     */
    constructor(logger: winston.Logger, config?: Partial<BackoffConfig>);
    /**
     * Record an offline check (user is not streaming)
     * Implements exponential backoff: 1m → 2m → 4m → 8m → 10m (max)
     *
     * @param username TikTok username
     */
    recordOfflineCheck(username: string): void;
    /**
     * Record a connection error (network error, API error, etc.)
     * Uses faster backoff than offline checks: 2s → 4s → 8s → 16s → ... → 5m (max)
     *
     * @param username TikTok username
     * @param error Error that occurred
     */
    recordConnectionError(username: string, error: Error): void;
    /**
     * Record successful connection (stream detected and connected)
     * Resets all backoff counters
     *
     * @param username TikTok username
     */
    recordSuccessfulConnection(username: string): void;
    /**
     * Record disconnection (user went offline during stream)
     * Resets to base interval for quick re-detection
     *
     * @param username TikTok username
     */
    recordDisconnection(username: string): void;
    /**
     * Check if we should attempt to check this username now
     *
     * @param username TikTok username
     * @returns true if enough time has passed
     */
    shouldCheckNow(username: string): boolean;
    /**
     * Get time until next check (in milliseconds)
     *
     * @param username TikTok username
     * @returns milliseconds until next check, or 0 if should check now
     */
    getTimeUntilNextCheck(username: string): number;
    /**
     * Get current backoff state for a username
     *
     * @param username TikTok username
     * @returns BackoffState or undefined if not found
     */
    getState(username: string): BackoffState | undefined;
    /**
     * Remove backoff state when channel is no longer active
     *
     * @param username TikTok username
     */
    removeState(username: string): void;
    /**
     * Get all usernames currently tracked
     *
     * @returns Array of usernames
     */
    getAllUsernames(): string[];
    /**
     * Get statistics about current backoff states
     */
    getStats(): {
        totalTracked: number;
        withOfflineBackoff: number;
        withErrorBackoff: number;
        seenLiveBefore: number;
        avgOfflineChecks: number;
        avgErrorCount: number;
    };
    /**
     * Create or retrieve backoff state for a username
     *
     * @private
     */
    private getOrCreateState;
}
//# sourceMappingURL=backoff-manager.d.ts.map