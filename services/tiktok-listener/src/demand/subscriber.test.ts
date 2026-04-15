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
 * DemandSubscriber unit tests
 *
 * Tests that the DemandSubscriber correctly filters by platform
 * and calls the handler with the correct Map.
 */

import { describe, it, expect, vi, beforeEach, type MockInstance } from 'vitest';
import { DemandSubscriber, DemandSource, DemandUpdate, DemandHandler } from './subscriber.js';
import { Logger } from '../types/logger.js';

// Create a minimal mock logger
const mockLogger: Logger = {
  error: vi.fn(),
  warn: vi.fn(),
  info: vi.fn(),
  debug: vi.fn(),
};

// Mock Redis subscriber client returned by duplicate()
function makeMockSubscriberClient() {
  const subscriptions: Map<string, (message: string) => void | Promise<void>> = new Map();

  return {
    connect: vi.fn().mockResolvedValue(undefined),
    disconnect: vi.fn().mockResolvedValue(undefined),
    unsubscribe: vi.fn().mockResolvedValue(undefined),
    subscribe: vi.fn().mockImplementation(
      (channel: string, callback: (message: string) => void | Promise<void>) => {
        subscriptions.set(channel, callback);
        return Promise.resolve();
      }
    ),
    // Helper for tests to publish messages to the subscriber
    _publish: (channel: string, message: string) => {
      const cb = subscriptions.get(channel);
      if (cb) return cb(message);
    },
  };
}

// Create a mock Redis client with duplicate() returning a fresh subscriber mock
function makeMockRedisClient() {
  const subscriberClient = makeMockSubscriberClient();
  return {
    duplicate: vi.fn().mockReturnValue(subscriberClient),
    _subscriberClient: subscriberClient,
  };
}

describe('DemandSubscriber', () => {
  let mockRedis: ReturnType<typeof makeMockRedisClient>;
  let handler: MockInstance & DemandHandler;

  beforeEach(() => {
    mockRedis = makeMockRedisClient();
    handler = vi.fn().mockResolvedValue(undefined) as unknown as MockInstance & DemandHandler;
    vi.clearAllMocks();
  });

  it('subscribes to source:demand channel after subscribe() is called', async () => {
    const subscriber = new DemandSubscriber(
      mockRedis as any,
      handler,
      mockLogger
    );

    await subscriber.subscribe();

    expect(mockRedis.duplicate).toHaveBeenCalledOnce();
    expect(mockRedis._subscriberClient.connect).toHaveBeenCalledOnce();
    expect(mockRedis._subscriberClient.subscribe).toHaveBeenCalledWith(
      'source:demand',
      expect.any(Function)
    );
    expect(subscriber.getIsSubscribed()).toBe(true);
  });

  it('filters by tiktok platform and calls handler with correct Map', async () => {
    const subscriber = new DemandSubscriber(
      mockRedis as any,
      handler,
      mockLogger
    );

    await subscriber.subscribe();

    const update: DemandUpdate = {
      type: 'demand_update',
      timestamp: new Date().toISOString(),
      sources: [
        { source_id: 'src-1', channel_id: 'tiktokuser1', platform: 'tiktok', overlay_id: 'overlay-a' },
        { source_id: 'src-2', channel_id: 'twitchuser1', platform: 'twitch', overlay_id: 'overlay-b' },
        { source_id: 'src-3', channel_id: 'tiktokuser2', platform: 'tiktok', overlay_id: 'overlay-c' },
      ],
    };

    await mockRedis._subscriberClient._publish('source:demand', JSON.stringify(update));

    expect(handler).toHaveBeenCalledOnce();
    const demanded: Map<string, DemandSource> = handler.mock.calls[0][0];
    expect(demanded.size).toBe(2);
    expect(demanded.has('tiktokuser1')).toBe(true);
    expect(demanded.has('tiktokuser2')).toBe(true);
    expect(demanded.has('twitchuser1')).toBe(false);
  });

  it('calls handler with empty Map when sources array is empty', async () => {
    const subscriber = new DemandSubscriber(
      mockRedis as any,
      handler,
      mockLogger
    );

    await subscriber.subscribe();

    const update: DemandUpdate = {
      type: 'demand_update',
      timestamp: new Date().toISOString(),
      sources: [],
    };

    await mockRedis._subscriberClient._publish('source:demand', JSON.stringify(update));

    expect(handler).toHaveBeenCalledOnce();
    const demanded: Map<string, DemandSource> = handler.mock.calls[0][0];
    expect(demanded.size).toBe(0);
  });

  it('filters out non-tiktok platforms completely', async () => {
    const subscriber = new DemandSubscriber(
      mockRedis as any,
      handler,
      mockLogger
    );

    await subscriber.subscribe();

    const update: DemandUpdate = {
      type: 'demand_update',
      timestamp: new Date().toISOString(),
      sources: [
        { source_id: 'src-x', channel_id: 'youtuber', platform: 'youtube', overlay_id: 'overlay-y' },
        { source_id: 'src-y', channel_id: 'kicker', platform: 'kick', overlay_id: 'overlay-k' },
      ],
    };

    await mockRedis._subscriberClient._publish('source:demand', JSON.stringify(update));

    expect(handler).toHaveBeenCalledOnce();
    const demanded: Map<string, DemandSource> = handler.mock.calls[0][0];
    expect(demanded.size).toBe(0);
  });

  it('accepts all tiktok sources without filtering (leadership handles distribution)', async () => {
    const subscriber = new DemandSubscriber(
      mockRedis as any,
      handler,
      mockLogger,
    );

    await subscriber.subscribe();

    const update: DemandUpdate = {
      type: 'demand_update',
      timestamp: new Date().toISOString(),
      sources: [
        { source_id: 'src-1', channel_id: 'tiktokuser1', platform: 'tiktok', overlay_id: 'overlay-a' },
        { source_id: 'src-2', channel_id: 'tiktokuser2', platform: 'tiktok', overlay_id: 'overlay-b' },
      ],
    };

    await mockRedis._subscriberClient._publish('source:demand', JSON.stringify(update));

    expect(handler).toHaveBeenCalledOnce();
    const demanded: Map<string, DemandSource> = handler.mock.calls[0][0];
    expect(demanded.size).toBe(2);
    expect(demanded.has('tiktokuser1')).toBe(true);
    expect(demanded.has('tiktokuser2')).toBe(true);
  });

  it('ignores malformed JSON messages and logs a warning', async () => {
    const subscriber = new DemandSubscriber(
      mockRedis as any,
      handler,
      mockLogger
    );

    await subscriber.subscribe();

    await mockRedis._subscriberClient._publish('source:demand', 'not-valid-json');

    expect(handler).not.toHaveBeenCalled();
    expect(mockLogger.warn).toHaveBeenCalled();
  });
});
