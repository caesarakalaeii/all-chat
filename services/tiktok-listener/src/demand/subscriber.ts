/**
 * DemandSubscriber
 *
 * Redis Pub/Sub subscriber for demand update events (Phase 5).
 * Receives full-snapshot DemandUpdate messages from source-manager on the
 * "source:demand" channel and calls the handler with a filtered Map of
 * TikTok channels that this pod should be actively connected to.
 *
 * Mirrors the MigrationSubscriber pattern from coordination/subscriber.ts.
 */

import { RedisClientType } from 'redis';
import { Logger } from '../types/logger.js';

/**
 * DemandSource represents a single source that has active demand.
 * Matches the DemandSource interface published by source-manager.
 */
export interface DemandSource {
  source_id: string;
  channel_id: string;  // TikTok username
  platform: string;    // "tiktok"
  overlay_id: string;
}

/**
 * DemandUpdate is the full-snapshot message published by source-manager
 * to the "source:demand" Redis Pub/Sub channel.
 */
export interface DemandUpdate {
  type: string;
  sources: DemandSource[];
  timestamp: string;  // ISO 8601
}

/**
 * DemandHandler receives a Map of TikTok username -> DemandSource
 * representing the current demanded set for this pod.
 * Called on every DemandUpdate received from source-manager.
 */
export type DemandHandler = (demanded: Map<string, DemandSource>) => void | Promise<void>;

/**
 * DemandSubscriber subscribes to the "source:demand" Redis Pub/Sub channel.
 * Filters to TikTok-platform sources only, applies assignedSourceIDs filtering,
 * and calls the handler with the resulting Map<username, DemandSource>.
 */
export class DemandSubscriber {
  private redisClient: RedisClientType;
  private logger: Logger;
  private handler: DemandHandler;
  private assignedSourceIDs: Set<string>;
  private isSubscribed: boolean = false;
  private subscriberClient: RedisClientType | null = null;

  /**
   * @param redisClient - Redis client instance (will be duplicated for Pub/Sub)
   * @param handler - Called with Map<username, DemandSource> on each update
   * @param logger - Logger instance
   * @param assignedSourceIDs - When non-empty, only sources in this set are passed to handler
   */
  constructor(
    redisClient: RedisClientType,
    handler: DemandHandler,
    logger: Logger,
    assignedSourceIDs: Set<string> = new Set()
  ) {
    this.redisClient = redisClient;
    this.handler = handler;
    this.logger = logger;
    this.assignedSourceIDs = assignedSourceIDs;
  }

  /**
   * Update the set of assigned source IDs used for filtering.
   * Call this when the coordinator updates this pod's assignments.
   */
  updateAssignedSourceIDs(ids: Set<string>): void {
    this.assignedSourceIDs = ids;
  }

  /**
   * Subscribe to the "source:demand" Redis Pub/Sub channel.
   * Creates a duplicate connection as required by node-redis for Pub/Sub.
   */
  async subscribe(): Promise<void> {
    const channel = 'source:demand';
    this.logger.info('Subscribing to demand channel', { channel });

    // Duplicate connection for Pub/Sub (node-redis requirement)
    const subscriber = this.redisClient.duplicate();
    await subscriber.connect();
    this.subscriberClient = subscriber;

    await subscriber.subscribe(channel, async (message: string) => {
      await this.handleMessage(message);
    });

    this.isSubscribed = true;
    this.logger.info('Successfully subscribed to demand channel', { channel });
  }

  /**
   * Unsubscribe and disconnect the Pub/Sub connection.
   */
  async unsubscribe(): Promise<void> {
    if (this.subscriberClient) {
      await this.subscriberClient.unsubscribe('source:demand');
      await this.subscriberClient.disconnect();
      this.subscriberClient = null;
    }
    this.isSubscribed = false;
  }

  /**
   * Handle an incoming message from the "source:demand" channel.
   */
  private async handleMessage(message: string): Promise<void> {
    try {
      const update: DemandUpdate = JSON.parse(message);

      // Filter to tiktok platform only
      const tiktokSources = update.sources.filter(s => s.platform === 'tiktok');

      // Build demanded Map, applying assignedSourceIDs filter if active
      const demanded = new Map<string, DemandSource>();
      for (const source of tiktokSources) {
        if (this.assignedSourceIDs.size === 0 || this.assignedSourceIDs.has(source.source_id)) {
          demanded.set(source.channel_id, source);
        }
      }

      this.logger.debug('Received demand update', {
        total_sources: update.sources.length,
        tiktok_sources: tiktokSources.length,
        filtered_sources: demanded.size,
      });

      try {
        await this.handler(demanded);
      } catch (handlerError) {
        this.logger.error('Demand handler threw error', { error: String(handlerError) });
      }
    } catch (parseError) {
      this.logger.warn('Failed to parse demand update', {
        payload: message,
        error: String(parseError),
      });
    }
  }

  /**
   * Returns whether the subscriber is currently active.
   */
  getIsSubscribed(): boolean {
    return this.isSubscribed;
  }
}
