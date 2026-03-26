/**
 * Migration Subscriber
 *
 * Redis Pub/Sub subscriber for migration events (Phase 6).
 * Mirrors Go shared/coordination/migration_subscriber.go patterns in TypeScript.
 */

import { RedisClientType } from 'redis';
import { Logger } from '../types/logger.js';
import { MigrationEvent } from './models.js';

/**
 * MigrationSubscriber subscribes to Redis Pub/Sub for migration events.
 * Matches Go shared/coordination/migration_subscriber.go MigrationSubscriber behavior.
 */
export class MigrationSubscriber {
  private redisClient: RedisClientType;
  private logger: Logger;
  private handler: (event: MigrationEvent) => void | Promise<void>;
  private isSubscribed: boolean = false;

  /**
   * Creates a new migration event subscriber.
   *
   * @param redisClient - Redis client instance (must support Pub/Sub)
   * @param handler - Callback function to handle migration events
   * @param logger - Logger instance
   */
  constructor(
    redisClient: RedisClientType,
    handler: (event: MigrationEvent) => void | Promise<void>,
    logger: Logger
  ) {
    this.redisClient = redisClient;
    this.handler = handler;
    this.logger = logger;
  }

  /**
   * Subscribe subscribes to the migration:events Redis Pub/Sub channel.
   *
   * Implements MIGRATE-01 (overlap migration pattern notification).
   * Per CONTEXT.md user decision: "Hybrid Redis Pub/Sub approach - Coordinator
   * publishes migration event to Redis Pub/Sub channel (5-20ms latency)"
   *
   * @returns Promise resolving when subscription is confirmed
   */
  async subscribe(): Promise<void> {
    const channel = 'migration:events';

    this.logger.info('Subscribing to migration events channel', {
      channel,
    });

    try {
      // Create a duplicate connection for Pub/Sub
      // (node-redis requires separate connection for Pub/Sub)
      const subscriber = this.redisClient.duplicate();
      await subscriber.connect();

      // Subscribe to channel
      await subscriber.subscribe(channel, async (message: string) => {
        await this.handleMessage(message);
      });

      this.isSubscribed = true;

      this.logger.info('Successfully subscribed to migration events channel', {
        channel,
      });
    } catch (error) {
      this.logger.error('Failed to subscribe to migration events channel', {
        channel,
        error: String(error),
      });

      throw new Error(`Failed to subscribe to ${channel}: ${error}`);
    }
  }

  /**
   * Handle incoming migration event message.
   *
   * @param message - Raw message payload from Redis Pub/Sub
   */
  private async handleMessage(message: string): Promise<void> {
    try {
      // Parse migration event
      const event: MigrationEvent = JSON.parse(message);

      // Log at debug level — the migration:events channel carries events for ALL platforms
      // (twitch, youtube, kick, tiktok). Logging at info here would spam the tiktok-listener
      // with events that are handled by other listener services and immediately discarded here.
      this.logger.debug('Received migration event', {
        migration_id: event.migration_id,
        channel_id: event.channel_id,
        platform: event.platform,
        from_pod: event.from_pod,
        to_pod: event.to_pod,
        reason: event.reason,
      });

      // Call handler with panic protection
      try {
        await this.handler(event);
      } catch (handlerError) {
        this.logger.error('Migration event handler threw error', {
          migration_id: event.migration_id,
          error: String(handlerError),
        });
      }
    } catch (parseError) {
      this.logger.warn('Failed to unmarshal migration event, skipping', {
        payload: message,
        error: String(parseError),
      });
    }
  }

  /**
   * Returns whether subscriber is currently subscribed.
   */
  public getIsSubscribed(): boolean {
    return this.isSubscribed;
  }
}
