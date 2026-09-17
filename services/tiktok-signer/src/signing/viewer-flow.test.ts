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
      navLog.push('reload');
      emitResponses();
      return undefined;
    },
    evaluate: async () => {
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
    lanes: Map<string, { host: string; prewarmed: unknown[] }>;
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
});
