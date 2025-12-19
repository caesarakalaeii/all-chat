/**
 * TikTok Live Stream Poller
 *
 * Background task that periodically checks if inactive TikTok channels went live.
 * Respects backoff intervals and coordinates with status checker and backoff manager.
 */
import winston from 'winston';
import { TikTokStatusChecker } from './status-checker.js';
import { BackoffManager } from './backoff-manager.js';
/**
 * Target to poll for live status
 */
export interface PollingTarget {
    username: string;
    overlayId: string;
}
/**
 * Callback when a target goes live
 */
export type OnLiveCallback = (username: string, overlayId: string) => Promise<void>;
/**
 * Configuration for the poller
 */
export interface PollerConfig {
    pollIntervalMs: number;
}
/**
 * LiveStreamPoller periodically checks if offline users went live
 */
export declare class LiveStreamPoller {
    private logger;
    private statusChecker;
    private backoffManager;
    private pollingTargets;
    private pollingTimer?;
    private readonly POLL_INTERVAL_MS;
    private isRunning;
    private onLiveCallback?;
    /**
     * @param statusChecker TikTokStatusChecker instance
     * @param backoffManager BackoffManager instance
     * @param logger Winston logger instance
     * @param config Optional poller configuration
     */
    constructor(statusChecker: TikTokStatusChecker, backoffManager: BackoffManager, logger: winston.Logger, config?: Partial<PollerConfig>);
    /**
     * Set callback function to be called when a target goes live
     *
     * @param callback Function to call with username and overlayId
     */
    setOnLiveCallback(callback: OnLiveCallback): void;
    /**
     * Start periodic polling
     */
    start(): void;
    /**
     * Stop periodic polling
     */
    stop(): void;
    /**
     * Add a username to polling targets
     *
     * @param username TikTok username
     * @param overlayId Overlay ID that needs this stream
     */
    addTarget(username: string, overlayId: string): void;
    /**
     * Remove a username from polling targets
     *
     * @param username TikTok username
     */
    removeTarget(username: string): void;
    /**
     * Get current polling targets count
     *
     * @returns Number of targets being polled
     */
    getTargetCount(): number;
    /**
     * Get all current targets
     *
     * @returns Array of polling targets
     */
    getTargets(): PollingTarget[];
    /**
     * Check if a username is being polled
     *
     * @param username TikTok username
     * @returns true if currently being polled
     */
    isTargetActive(username: string): boolean;
    /**
     * Get poller status
     */
    getStatus(): {
        isRunning: boolean;
        targetCount: number;
        pollIntervalMs: number;
        backoffStats: {
            totalTracked: number;
            withOfflineBackoff: number;
            withErrorBackoff: number;
            seenLiveBefore: number;
            avgOfflineChecks: number;
            avgErrorCount: number;
        };
        cacheStats: {
            totalEntries: number;
            validEntries: number;
            expiredEntries: number;
            cacheTTLMs: number;
        };
    };
    /**
     * Single polling cycle - checks all targets respecting backoff
     *
     * @private
     */
    private runPollingCycle;
    /**
     * Check a single target, respecting backoff
     *
     * @param target Polling target to check
     * @returns true if live (and callback was called), false otherwise
     * @private
     */
    private checkTarget;
}
//# sourceMappingURL=poller.d.ts.map