/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */

/**
 * DemandSubscriber
 *
 * Redis Pub/Sub subscriber for demand update events.
 * Receives full-snapshot DemandUpdate messages from source-manager on the
 * "source:demand" channel and calls the handler with a filtered Map of
 * TikTok channels.
 *
 * Leadership-based coordination: all demanded tiktok sources are accepted.
 * Per-stream leadership claims handle distribution across pods.
 */

import { RedisClientType } from 'redis';
import { Logger } from '../types/logger.js';

/**
 * DemandSource represents a single source that has active demand.
 */
export interface DemandSource {
  source_id: string;
  channel_id: string;  // TikTok username
  platform: string;    // "tiktok"
  overlay_id: string;
}

/**
 * DemandUpdate is the full-snapshot message published by source-manager.
 */
export interface DemandUpdate {
  type: string;
  sources: DemandSource[];
  timestamp: string;
}

export type DemandHandler = (demanded: Map<string, DemandSource>) => void | Promise<void>;

/**
 * DemandSubscriber subscribes to the "source:demand" Redis Pub/Sub channel.
 * Filters to TikTok-platform sources only and calls the handler with all of them.
 */
export class DemandSubscriber {
  private redisClient: RedisClientType;
  private logger: Logger;
  private handler: DemandHandler;
  private isSubscribed: boolean = false;
  private subscriberClient: RedisClientType | null = null;

  constructor(
    redisClient: RedisClientType,
    handler: DemandHandler,
    logger: Logger,
  ) {
    this.redisClient = redisClient;
    this.handler = handler;
    this.logger = logger;
  }

  async subscribe(): Promise<void> {
    const channel = 'source:demand';
    this.logger.info('Subscribing to demand channel', { channel });

    const subscriber = this.redisClient.duplicate();
    await subscriber.connect();
    this.subscriberClient = subscriber;

    await subscriber.subscribe(channel, async (message: string) => {
      await this.handleMessage(message);
    });

    this.isSubscribed = true;
    this.logger.info('Successfully subscribed to demand channel', { channel });
  }

  async unsubscribe(): Promise<void> {
    if (this.subscriberClient) {
      await this.subscriberClient.unsubscribe('source:demand');
      await this.subscriberClient.disconnect();
      this.subscriberClient = null;
    }
    this.isSubscribed = false;
  }

  private async handleMessage(message: string): Promise<void> {
    try {
      const update: DemandUpdate = JSON.parse(message);

      // Filter to tiktok platform only — no assignment filtering needed,
      // leadership coordination handles pod distribution
      const tiktokSources = update.sources.filter(s => s.platform === 'tiktok');

      const demanded = new Map<string, DemandSource>();
      for (const source of tiktokSources) {
        demanded.set(source.channel_id, source);
      }

      this.logger.debug('Received demand update', {
        total_sources: update.sources.length,
        tiktok_sources: tiktokSources.length,
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

  getIsSubscribed(): boolean {
    return this.isSubscribed;
  }
}
