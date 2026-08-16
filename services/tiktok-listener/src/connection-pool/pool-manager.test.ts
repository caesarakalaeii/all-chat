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
 * Fan-out contract of the connection pool.
 *
 * The pool multiplexes ONE upstream TikTok connection onto many overlays, so every
 * subscriber's view of the stream depends on this one loop. What crosses this seam is the
 * RAW CONNECTOR FRAME: the pool hands each subscriber exactly the object the connector
 * emitted, unchanged and un-wrapped. Payload construction (`chat:raw`, chest classification,
 * …) happens on the consumer side, in index.ts — so these tests deliberately assert frame
 * identity and nothing about payload shape.
 *
 * The upstream connector is replaced with an in-memory EventEmitter fake, so the suite runs
 * offline and deterministically: no sign server, no WebSocket, no timers.
 *
 * WHICH CONNECTOR EVENT THE POOL CARRIES — read before adding tests here. The pool registers
 * exactly ONE inbound handler, `WebcastEvent.CHAT` (pool-manager.ts:298). It does NOT listen to
 * `WebcastEvent.ENVELOPE`; the ENVELOPE/treasure-chest path lives on a separate, non-pooled
 * connection in index.ts:893. So the ENVELOPE-shaped frames below are pushed down the CHAT
 * channel on purpose: what is under test is that the pool forwards its inbound frame as opaque
 * cargo, never reading a field, whatever shape that frame happens to have. An envelope-shaped
 * object is simply the most relevant cargo, because the sibling chest-detection work sends
 * frames of that shape through this fan-out once they reach a pooled path.
 *
 * Consequently the inbound event name is HARDCODED as `INBOUND_EVENT` rather than discovered
 * from the emitter's live listener registry. Discovery would make the suite blind to a change
 * in WHICH connector events the pool forwards: swapping CHAT for GIFT, or accidentally
 * registering two handlers and delivering every frame twice, would both stay green.
 * `pins the pool's inbound wiring` below asserts the registered set explicitly.
 */

import { EventEmitter } from 'node:events';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

/** Lifecycle events the pool listens to, as distinct from its inbound message path. */
const LIFECYCLE_EVENTS = ['connected', 'disconnected', 'error'] as const;

/**
 * The connector event the pool forwards to subscribers — `WebcastEvent.CHAT`.
 *
 * Hardcoded deliberately. See the module docblock: resolving this from the emitter would let a
 * change in the pool's wiring slip through unnoticed, which is the exact regression class this
 * suite exists to catch.
 */
const INBOUND_EVENT = 'chat';

/**
 * Stand-in for `TikTokLiveConnection`. Behaves like the real one where the pool touches it
 * (`on`, `connect`, `disconnect`) and records itself so a test can drive frames into the pool.
 */
class FakeTikTokLiveConnection extends EventEmitter {
  static instances: FakeTikTokLiveConnection[] = [];

  connectCalls = 0;
  disconnectCalls = 0;

  constructor(
    public readonly uniqueId: string,
    public readonly options?: unknown
  ) {
    super();
    FakeTikTokLiveConnection.instances.push(this);
  }

  async connect(): Promise<void> {
    this.connectCalls++;
  }

  disconnect(): void {
    this.disconnectCalls++;
  }

  /**
   * Push one frame down the pool's inbound message path, exactly once.
   *
   * Emits on the hardcoded `INBOUND_EVENT` so the assertions downstream can use literal call
   * counts, which in turn makes a duplicate-delivery regression visible.
   */
  pushInboundFrame(frame: unknown): void {
    this.emit(INBOUND_EVENT, frame);
  }
}

// The connector is the only thing in this module that would touch the network.
vi.mock('tiktok-live-connector', () => ({
  TikTokLiveConnection: FakeTikTokLiveConnection,
  // Mirrors the real enum members the pool reads; values are the connector's wire names.
  WebcastEvent: {
    CHAT: 'chat',
    ENVELOPE: 'envelope',
    GIFT: 'gift'
  }
}));

const { ConnectionPoolManager } = await import('./pool-manager.js');

function silentLogger() {
  return {
    error: vi.fn(),
    warn: vi.fn(),
    info: vi.fn(),
    debug: vi.fn()
  };
}

/**
 * An ENVELOPE frame as the connector emits it (a viewer-dropped coin chest). Held here purely
 * as opaque cargo: the pool must forward this object as-is, without reading a single field.
 */
function envelopeFrame(overrides: Record<string, unknown> = {}) {
  return {
    common: { msgId: '7300000000000000001', createTime: '1786600000000' },
    display: 1,
    envelopeInfo: {
      envelopeId: 'env-1',
      businessType: 1,
      diamondCount: 20,
      peopleCount: 1,
      sendUserId: '6800000000000000002',
      sendUserName: 'someviewer'
    },
    ...overrides
  };
}

const USERNAME = 'somestreamer';

let logger: ReturnType<typeof silentLogger>;
let pool: InstanceType<typeof ConnectionPoolManager>;

beforeEach(() => {
  FakeTikTokLiveConnection.instances = [];
  logger = silentLogger();
  pool = new ConnectionPoolManager(logger);
});

afterEach(() => {
  pool.stop();
  vi.clearAllMocks();
});

/** The single upstream connection the pool opened. */
function upstream(): FakeTikTokLiveConnection {
  expect(FakeTikTokLiveConnection.instances).toHaveLength(1);
  return FakeTikTokLiveConnection.instances[0];
}

describe('ConnectionPoolManager fan-out', () => {
  it("pins the pool's inbound wiring to exactly one connector event", async () => {
    await pool.subscribe(USERNAME, 'overlay-a', { overlayId: 'overlay-a', onMessage: vi.fn() });

    const registered = upstream()
      .eventNames()
      .map(name => String(name));
    const inbound = registered.filter(
      name => !(LIFECYCLE_EVENTS as readonly string[]).includes(name)
    );

    // Exactly one inbound handler: two would fan every frame out twice, zero would drop the
    // stream entirely. Both are silent failures without this assertion.
    expect(inbound).toEqual([INBOUND_EVENT]);
    expect(registered).toEqual([INBOUND_EVENT, ...LIFECYCLE_EVENTS]);

    // The pool carries CHAT only. An ENVELOPE frame arriving at this connection reaches nobody,
    // because the chest path is wired on a separate connection in index.ts:893 — recorded here
    // so the sibling chest-detection work is not misled into thinking the pool covers it.
    const seen = vi.fn();
    await pool.subscribe(USERNAME, 'overlay-b', { overlayId: 'overlay-b', onMessage: seen });
    upstream().emit('envelope', envelopeFrame());
    expect(seen).not.toHaveBeenCalled();
  });

  it('fans out an accepted ENVELOPE frame to subscribers', async () => {
    const overlayA = vi.fn();
    const overlayB = vi.fn();
    const overlayC = vi.fn();

    await pool.subscribe(USERNAME, 'overlay-a', { overlayId: 'overlay-a', onMessage: overlayA });
    await pool.subscribe(USERNAME, 'overlay-b', { overlayId: 'overlay-b', onMessage: overlayB });
    await pool.subscribe(USERNAME, 'overlay-c', { overlayId: 'overlay-c', onMessage: overlayC });

    // One upstream connection for three overlays — that is the point of the pool.
    expect(pool.getConnectionCount()).toBe(1);
    expect(pool.getSubscriberCount(USERNAME)).toBe(3);

    const frame = envelopeFrame();
    upstream().pushInboundFrame(frame);

    for (const onMessage of [overlayA, overlayB, overlayC]) {
      expect(onMessage).toHaveBeenCalledTimes(1);
      // The RAW frame, by identity: the pool forwards, it does not build a payload.
      expect(onMessage.mock.calls[0][0]).toBe(frame);
      expect(onMessage.mock.calls[0][0]).toEqual(envelopeFrame());
    }
  });

  it('keeps delivering to the remaining subscribers when one onMessage throws', async () => {
    const boom = vi.fn(() => {
      throw new Error('overlay blew up');
    });
    const before = vi.fn();
    const after = vi.fn();

    // Registered around the thrower so a swallowed error cannot be hidden by ordering.
    await pool.subscribe(USERNAME, 'overlay-before', {
      overlayId: 'overlay-before',
      onMessage: before
    });
    await pool.subscribe(USERNAME, 'overlay-boom', { overlayId: 'overlay-boom', onMessage: boom });
    await pool.subscribe(USERNAME, 'overlay-after', {
      overlayId: 'overlay-after',
      onMessage: after
    });

    const frame = envelopeFrame();
    expect(() => upstream().pushInboundFrame(frame)).not.toThrow();

    expect(boom).toHaveBeenCalled();
    expect(before).toHaveBeenCalledWith(frame);
    expect(after).toHaveBeenCalledWith(frame);
    expect(logger.error).toHaveBeenCalled();
  });

  it('stops delivering to a subscriber once it has unsubscribed', async () => {
    const staying = vi.fn();
    const leaving = vi.fn();

    await pool.subscribe(USERNAME, 'overlay-staying', {
      overlayId: 'overlay-staying',
      onMessage: staying
    });
    await pool.subscribe(USERNAME, 'overlay-leaving', {
      overlayId: 'overlay-leaving',
      onMessage: leaving
    });

    const first = envelopeFrame();
    upstream().pushInboundFrame(first);
    expect(leaving).toHaveBeenCalledTimes(1);

    pool.unsubscribe(USERNAME, 'overlay-leaving');
    expect(pool.getSubscriberCount(USERNAME)).toBe(1);

    // Distinguishable from the first frame so the "not delivered" assertion below cannot pass
    // by structural coincidence.
    const second = envelopeFrame({ envelopeInfo: { envelopeId: 'env-2', businessType: 1 } });
    upstream().pushInboundFrame(second);

    // The departed overlay saw only the pre-unsubscribe frame; the connection stays up for the rest.
    expect(leaving).toHaveBeenCalledTimes(1);
    expect(leaving).not.toHaveBeenCalledWith(second);
    expect(staying).toHaveBeenCalledTimes(2);
    expect(staying).toHaveBeenLastCalledWith(second);
    expect(upstream().disconnectCalls).toBe(0);
  });

  it('reuses one upstream connection per username and opens a second for another', async () => {
    await pool.subscribe(USERNAME, 'overlay-a', { overlayId: 'overlay-a', onMessage: vi.fn() });
    await pool.subscribe(USERNAME, 'overlay-b', { overlayId: 'overlay-b', onMessage: vi.fn() });
    await pool.subscribe('otherstreamer', 'overlay-c', {
      overlayId: 'overlay-c',
      onMessage: vi.fn()
    });

    expect(FakeTikTokLiveConnection.instances.map(c => c.uniqueId)).toEqual([
      USERNAME,
      'otherstreamer'
    ]);
    expect(pool.getConnectionCount()).toBe(2);
    expect(pool.hasConnection(USERNAME)).toBe(true);
    expect(pool.getSubscriberCount(USERNAME)).toBe(2);
    expect(pool.getSubscriberCount('otherstreamer')).toBe(1);
  });

  it('delivers frames only to subscribers of that username', async () => {
    const mine = vi.fn();
    const theirs = vi.fn();

    await pool.subscribe(USERNAME, 'overlay-mine', { overlayId: 'overlay-mine', onMessage: mine });
    await pool.subscribe('otherstreamer', 'overlay-theirs', {
      overlayId: 'overlay-theirs',
      onMessage: theirs
    });

    const frame = envelopeFrame();
    FakeTikTokLiveConnection.instances[0].pushInboundFrame(frame);

    expect(mine).toHaveBeenCalledWith(frame);
    expect(theirs).not.toHaveBeenCalled();
  });

  it('rejects a malformed username before opening any connection', async () => {
    await expect(
      pool.subscribe('../etc/passwd', 'overlay-a', { overlayId: 'overlay-a', onMessage: vi.fn() })
    ).rejects.toThrow();

    expect(FakeTikTokLiveConnection.instances).toHaveLength(0);
    expect(pool.getConnectionCount()).toBe(0);
  });

  it('notifies every subscriber of connect and disconnect without letting one break the rest', async () => {
    const onConnected = vi.fn();
    const onDisconnected = vi.fn();

    await pool.subscribe(USERNAME, 'overlay-boom', {
      overlayId: 'overlay-boom',
      onMessage: vi.fn(),
      onConnected: () => {
        throw new Error('connected handler blew up');
      },
      onDisconnected: () => {
        throw new Error('disconnected handler blew up');
      }
    });
    await pool.subscribe(USERNAME, 'overlay-ok', {
      overlayId: 'overlay-ok',
      onMessage: vi.fn(),
      onConnected,
      onDisconnected
    });

    const connection = upstream();
    expect(() => connection.emit('connected', { roomId: 'room-1' })).not.toThrow();
    expect(onConnected).toHaveBeenCalledWith({ roomId: 'room-1' });

    expect(() => connection.emit('disconnected')).not.toThrow();
    expect(onDisconnected).toHaveBeenCalledTimes(1);
  });

  it('disconnects every pooled connection on shutdown', async () => {
    await pool.subscribe(USERNAME, 'overlay-a', { overlayId: 'overlay-a', onMessage: vi.fn() });
    await pool.subscribe('otherstreamer', 'overlay-b', {
      overlayId: 'overlay-b',
      onMessage: vi.fn()
    });

    await pool.disconnectAll();

    expect(FakeTikTokLiveConnection.instances.every(c => c.disconnectCalls === 1)).toBe(true);
    expect(pool.getConnectionCount()).toBe(0);
  });

  it('drops an idle connection with no subscribers on the cleanup tick', async () => {
    vi.useFakeTimers();
    try {
      const idlePool = new ConnectionPoolManager(logger, {
        idleTimeoutMs: 1000,
        cleanupIntervalMs: 100
      });
      idlePool.start();

      await idlePool.subscribe(USERNAME, 'overlay-a', {
        overlayId: 'overlay-a',
        onMessage: vi.fn()
      });
      const connection = FakeTikTokLiveConnection.instances[0];

      // Still subscribed: the connection survives however long it idles.
      vi.advanceTimersByTime(5000);
      expect(idlePool.getConnectionCount()).toBe(1);
      expect(connection.disconnectCalls).toBe(0);

      idlePool.unsubscribe(USERNAME, 'overlay-a');
      vi.advanceTimersByTime(5000);

      expect(idlePool.getConnectionCount()).toBe(0);
      expect(connection.disconnectCalls).toBe(1);
      idlePool.stop();
    } finally {
      vi.useRealTimers();
    }
  });
});
