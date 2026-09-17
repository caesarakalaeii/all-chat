import { describe, expect, it, vi } from 'vitest';
import { ViewerPool, VIEWER_UA } from './viewer.js';

/**
 * Page double for exercising the capture flow without Chromium. Responses are
 * emitted synchronously on navigation (goto/reload/SPA evaluate); the
 * player-root check is time-driven via livePlayerFoundAt; navError makes the
 * navigation itself fail like a dead proxy.
 */
function fakePage(behavior: {
  responses?: Array<{ url: string; status: number; body: Buffer }>;
  livePlayerFoundAt?: number;
  navDelayMs?: number;
  navError?: boolean;
}) {
  const listeners = new Set<Function>();
  let closed = false;
  const started = Date.now();
  const navLog: string[] = [];
  const page = {
    isClosed: () => closed,
    on: (_event: string, fn: Function) => {
      listeners.add(fn);
    },
    off: (_event: string, fn: Function) => {
      listeners.delete(fn);
    },
    setUserAgent: async () => undefined,
    setRequestInterception: async () => undefined,
    authenticate: async () => undefined,
    cookies: async () => [],
    goto: async (url: string) => {
      navLog.push(`goto:${url}`);
      if (behavior.navDelayMs) {
        await new Promise((r) => setTimeout(r, behavior.navDelayMs));
      }
      if (behavior.navError) {
        throw new Error('net::ERR_CONNECTION_RESET');
      }
      emitResponses();
      return undefined;
    },
    reload: async () => {
      if (behavior.navError) {
        throw new Error('net::ERR_CONNECTION_RESET');
      }
      navLog.push('reload');
      emitResponses();
      return undefined;
    },
    evaluate: async () => {
      if (behavior.navError) {
        throw new Error('net::ERR_CONNECTION_RESET');
      }
      // SPA anchor click: player bootstrap replays without a new document.
      navLog.push('spa');
      emitResponses();
      return true;
    },
    waitForNavigation: async () => undefined,
    $$: async () => {
      const found =
        behavior.livePlayerFoundAt === undefined ||
        Date.now() - started >= behavior.livePlayerFoundAt;
      return found ? [{}] : [];
    },
    close: async () => {
      closed = true;
    }
  };
  function emitResponses() {
    for (const r of behavior.responses ?? []) {
      for (const fn of listeners) {
        fn({
          url: () => r.url,
          status: () => r.status,
          buffer: async () => r.body
        });
      }
    }
  }
  return { page, navLog };
}

const GOOD_BODY = Buffer.alloc(1500, 'x');

function poolInternals(pool: ViewerPool) {
  return pool as unknown as {
    lanes: Map<string, {
      host: string;
      prewarmed: unknown[];
      prewarmRefilling: boolean;
      waiters: Array<() => void>;
      slots: number;
    }>;
    tabs: Map<string, { page: unknown; lastUsed: number }>;
    roomLane: Map<string, string>;
  };
}

describe('ViewerPool capture flow (fake pages)', () => {
  it('captures from a cold prewarmed tab and registers the warm tab', async () => {
    const pool = new ViewerPool({ userDataDir: '/tmp/x' });
    const internals = poolInternals(pool);
    const { page, navLog } = fakePage({
      responses: [
        { url: 'https://webcast.tiktok.com/webcast/im/fetch/?room_id=42', status: 200, body: GOOD_BODY }
      ]
    });
    internals.lanes.get('')!.prewarmed.push(page);

    const capture = await pool.captureRoom('someuser', { timeoutMs: 10_000 });
    expect(capture.roomId).toBe('42');
    expect(capture.userAgent).toBe(VIEWER_UA);
    expect(capture.protoBase64.length).toBeGreaterThan(0);
    expect(navLog).toContain('goto:https://www.tiktok.com/@someuser/live');
    expect(internals.tabs.has('someuser')).toBe(true);
    await pool.close();
  });

  it('re-uses a warm tab through the SPA path, not a full reload', async () => {
    const pool = new ViewerPool({ userDataDir: '/tmp/x' });
    const internals = poolInternals(pool);
    const { page, navLog } = fakePage({
      responses: [
        { url: 'https://webcast.tiktok.com/webcast/im/fetch/?room_id=7', status: 200, body: GOOD_BODY }
      ]
    });
    internals.tabs.set('warmuser', { page, lastUsed: Date.now() });
    internals.roomLane.set('warmuser', '');

    const capture = await pool.captureRoom('warmuser', { timeoutMs: 10_000 });
    expect(capture.roomId).toBe('7');
    expect(navLog).toContain('spa');
    expect(navLog).not.toContain('reload');
    await pool.close();
  });

  it('fast-fails a zombie page at the heartbeat deadline, not the full timeout', async () => {
    const pool = new ViewerPool({ userDataDir: '/tmp/x' });
    const internals = poolInternals(pool);
    // Player never appears and no responses arrive: a zombie page.
    const { page } = fakePage({ livePlayerFoundAt: Number.MAX_SAFE_INTEGER });
    internals.lanes.get('')!.prewarmed.push(page);

    vi.useFakeTimers();
    const attempt = pool.captureRoom('zombie', { timeoutMs: 60_000 });
    const outcome = expect(attempt).rejects.toThrow(/live player not up/);
    try {
      // Heartbeat deadline (15s) fires well before the 60s capture timeout.
      await vi.advanceTimersByTimeAsync(15_100);
      expect(internals.tabs.has('zombie')).toBe(false);
      await outcome;
    } finally {
      vi.useRealTimers();
    }
    await pool.close();
  });

  it('a race loser does not unregister the winner\'s warm tab', async () => {
    const pool = new ViewerPool({ proxyHosts: ['slow:1', 'fast:2'], userDataDir: '/tmp/x' });
    const internals = poolInternals(pool);
    const fast = fakePage({
      responses: [
        { url: 'https://webcast.tiktok.com/webcast/im/fetch/?room_id=1', status: 200, body: GOOD_BODY }
      ]
    });
    // The slow lane's navigation dies immediately: a failed first attempt.
    const slow = fakePage({ navError: true });
    internals.lanes.get('fast:2')!.prewarmed.push(fast.page);
    internals.lanes.get('slow:1')!.prewarmed.push(slow.page);
    // Pin the room to the slow lane so the race rides the fast one.
    internals.roomLane.set('racer', 'slow:1');

    const capture = await pool.captureRoom('racer', { timeoutMs: 20_000 });
    expect(capture.roomId).toBe('1');
    // The winner's tab is registered; the loser did not touch it.
    expect(internals.tabs.has('racer')).toBe(true);
    expect(internals.roomLane.get('racer')).toBe('fast:2');
    await pool.close();
  });

  it('holds the lane slot for the whole attempt, so the concurrency cap actually caps', async () => {
    const pool = new ViewerPool({ userDataDir: '/tmp/x' });
    const internals = poolInternals(pool);
    // Three parked tabs on the single direct lane; each capture navigates
    // and emits its capture response after a delay, so all three start
    // concurrently before any settles.
    const lanes = internals.lanes.get('')!;
    for (let i = 2; i >= 0; i--) {
      const { page } = fakePage({
        responses: [
          { url: `https://webcast.tiktok.com/webcast/im/fetch/?room_id=${i}`, status: 200, body: GOOD_BODY }
        ],
        navDelayMs: 50
      });
      lanes.prewarmed.push(page);
    }

    const p0 = pool.captureRoom('user0', { timeoutMs: 5_000 });
    // The slot is acquired inside the async IIFE; let it book before the next.
    await new Promise((r) => setTimeout(r, 10));
    expect(lanes.slots).toBe(1);
    const p1 = pool.captureRoom('user1', { timeoutMs: 5_000 });
    const p2 = pool.captureRoom('user2', { timeoutMs: 5_000 });
    await new Promise((r) => setTimeout(r, 10));
    // Cap is 2: the third capture must be queued (slot not stolen).
    expect(lanes.slots).toBe(2);
    expect(lanes.waiters.length).toBe(1);

    const [c0, c1, c2] = await Promise.all([p0, p1, p2]);
    expect(c0.roomId).toBe('0');
    expect(c1.roomId).toBe('1');
    expect(c2.roomId).toBe('2');
    // All settled: every slot returned.
    expect(lanes.slots).toBe(0);
    expect(lanes.waiters.length).toBe(0);
    await pool.close();
  });

  it('releases the lane slot when a capture fails (fast-fail frees the queue)', async () => {
    const pool = new ViewerPool({ userDataDir: '/tmp/x' });
    const internals = poolInternals(pool);
    const lanes = internals.lanes.get('')!;
    // Zombie page: player never appears, no capture response. Pushed last:
    // pop() hands it to the first capture, the good page to the queued one.
    const good = fakePage({
      responses: [
        { url: 'https://webcast.tiktok.com/webcast/im/fetch/?room_id=9', status: 200, body: GOOD_BODY }
      ]
    });
    lanes.prewarmed.push(good.page);
    const { page: zombie } = fakePage({ livePlayerFoundAt: Number.MAX_SAFE_INTEGER });
    lanes.prewarmed.push(zombie);
    vi.useFakeTimers();
    const zombieAttempt = pool.captureRoom('zombie', { timeoutMs: 60_000 });
    const zombieOutcome = expect(zombieAttempt).rejects.toThrow(/live player not up/);
    // Let the attempt book its slot and start navigating (fake timers:
    // microtasks still run between advance calls).
    await vi.advanceTimersByTimeAsync(0);
    expect(lanes.slots).toBe(1);
    // The good capture queues behind the zombie's held slot.
    const goodAttempt = pool.captureRoom('good', { timeoutMs: 60_000 });
    await vi.advanceTimersByTimeAsync(0);
    expect(lanes.slots).toBe(1);
    // Zombie fast-fail at the 15s deadline releases its slot, the good
    // capture takes it and completes.
    await vi.advanceTimersByTimeAsync(15_100);
    await vi.runAllTimersAsync();
    const goodCapture = await goodAttempt;
    expect(goodCapture.roomId).toBe('9');
    await zombieOutcome;
    expect(lanes.slots).toBe(0);
    vi.useRealTimers();
    await pool.close();
  });

  it('a racer does not steal another lane\'s warm tab for the room', async () => {
    const pool = new ViewerPool({ proxyHosts: ['laneA:1', 'laneB:2'], userDataDir: '/tmp/x' });
    const internals = poolInternals(pool);
    // A warm tab for the room lives on lane A.
    // A warm tab for the room lives on lane A. Its SPA navigation is dead
    // (navError), so the first attempt on the pinned lane fails and the
    // race rides lane B.
    const warm = fakePage({ navError: true });
    internals.tabs.set('room', { page: warm.page, lastUsed: Date.now() });
    internals.roomLane.set('room', 'laneA:1');

    // The room is pinned to lane A but its navigation dies: the race rides
    // lane B, whose racer must NOT SPA-navigate lane A's warm tab. The
    // racer takes its own prewarmed tab instead.
    const laneAPage = fakePage({ navError: true });
    internals.lanes.get('laneA:1')!.prewarmed.push(laneAPage.page);
    const laneBPage = fakePage({
      responses: [
        { url: 'https://webcast.tiktok.com/webcast/im/fetch/?room_id=5', status: 200, body: GOOD_BODY }
      ]
    });
    internals.lanes.get('laneB:2')!.prewarmed.push(laneBPage.page);

    const capture = await pool.captureRoom('room', { timeoutMs: 20_000 });
    expect(capture.roomId).toBe('5');
    // The winner registered its own page (lane B's), and the old warm tab
    // was never touched by the racer: no 'spa' nav on it.
    expect(internals.tabs.get('room')?.page).toBe(laneBPage.page);
    expect(internals.roomLane.get('room')).toBe('laneB:2');
    expect(warm.navLog).not.toContain('spa');
    await pool.close();
  });

  it('only the race winner registers its tab: a loser that reaches onResponse does not overwrite it', async () => {
    const pool = new ViewerPool({ proxyHosts: ['pin:1', 'win:2', 'lose:3'], userDataDir: '/tmp/x' });
    const internals = poolInternals(pool);
    // First attempt (pinned lane) dies instantly; the race then rides the
    // other two lanes. Both racers capture, but the winner is whichever
    // the race resolves first; the loser's onResponse must see the race
    // settled against it and close its own page without registering.
    const pin = fakePage({ navError: true });
    internals.lanes.get('pin:1')!.prewarmed.push(pin.page);
    const winner = fakePage({
      responses: [
        { url: 'https://webcast.tiktok.com/webcast/im/fetch/?room_id=11', status: 200, body: GOOD_BODY }
      ]
    });
    internals.lanes.get('win:2')!.prewarmed.push(winner.page);
    const loser = fakePage({
      responses: [
        { url: 'https://webcast.tiktok.com/webcast/im/fetch/?room_id=12', status: 200, body: GOOD_BODY }
      ],
      navDelayMs: 30
    });
    internals.lanes.get('lose:3')!.prewarmed.push(loser.page);
    internals.roomLane.set('roomx', 'pin:1');

    const capture = await pool.captureRoom('roomx', { timeoutMs: 20_000 });
    expect(capture.roomId).toBe('11');
    // The registered tab is the winner's live page, not a closed loser.
    expect(internals.tabs.get('roomx')?.page).toBe(winner.page);
    expect(internals.roomLane.get('roomx')).toBe('win:2');
    await pool.close();
  });
});
