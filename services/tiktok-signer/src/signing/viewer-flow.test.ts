import { describe, expect, it, vi } from 'vitest';
import { ViewerPool } from './viewer.js';
import { VIEWER_UA } from './identity.js';

/**
 * Page double for exercising the capture flow without Chromium. Responses
 * are emitted synchronously on navigation (goto/reload/SPA evaluate) and
 * the wsCreated WebSocket-creation events fire alongside them, mirroring
 * the real page's push socket appearing during the player bootstrap; the
 * player-root check is time-driven via livePlayerFoundAt; navError makes
 * the navigation itself fail like a dead proxy.
 */
function fakePage(behavior: {
  responses?: Array<{ url: string; status: number; body: Buffer }>;
  livePlayerFoundAt?: number;
  navDelayMs?: number;
  navError?: boolean;
  /** Delay before cookies() resolves, to hold the winner mid-registration. */
  cookieDelayMs?: number;
  /** webSocketCreated URLs the CDP double emits on navigation. */
  wsCreated?: string[];
}) {
  const listeners = new Set<Function>();
  const cdpListeners = new Set<(params: unknown) => void>();
  let cdpDetached = false;
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
    createCDPSession: async () => ({
      on: (_event: string, fn: (params: unknown) => void) => {
        cdpListeners.add(fn);
      },
      send: async () => undefined,
      detach: async () => {
        cdpDetached = true;
      }
    }),
    /** Test hooks into the double's CDP state. */
    cdpDetached: () => cdpDetached,
    emitWsCreated: (url: string) => {
      for (const fn of cdpListeners) fn({ url });
    },
    setUserAgent: async () => undefined,
    setRequestInterception: async () => undefined,
    authenticate: async () => undefined,
    cookies: async () => {
      if (behavior.cookieDelayMs) {
        const { promise, resolve } = Promise.withResolvers<void>();
        setTimeout(resolve, behavior.cookieDelayMs);
        await promise;
      }
      return [];
    },
    goto: async (url: string) => {
      navLog.push(`goto:${url}`);
      if (behavior.navDelayMs) {
        const { promise, resolve } = Promise.withResolvers<void>();
        setTimeout(resolve, behavior.navDelayMs);
        await promise;
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
    for (const url of behavior.wsCreated ?? []) {
      for (const fn of cdpListeners) fn({ url });
    }
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
      benchedUntil: number;
      consecutiveFailures: number;
    }>;
    tabs: Map<string, { page: unknown; lastUsed: number; wsUrl: string; roomId: string; capturedAt: number }>;
    roomLane: Map<string, string>;
  };
}

/** Seed a warm tab the way a real registration writes it. */
function seedTab(
  internals: ReturnType<typeof poolInternals>,
  username: string,
  page: unknown,
  wsUrl = ''
) {
  internals.tabs.set(username, {
    page,
    lastUsed: Date.now(),
    wsUrl,
    roomId: 'seed',
    capturedAt: Date.now()
  });
  internals.roomLane.set(username, '|primary');
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
    internals.lanes.get('|primary')!.prewarmed.push(page);

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
    seedTab(internals, 'warmuser', page);

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
    internals.lanes.get('|primary')!.prewarmed.push(page);

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
    internals.lanes.get('fast:2|primary')!.prewarmed.push(fast.page);
    internals.lanes.get('slow:1|primary')!.prewarmed.push(slow.page);
    // Pin the room to the slow lane so the race rides the fast one.
    internals.roomLane.set('racer', 'slow:1|primary');

    const capture = await pool.captureRoom('racer', { timeoutMs: 20_000 });
    expect(capture.roomId).toBe('1');
    // The winner's tab is registered; the loser did not touch it.
    expect(internals.tabs.has('racer')).toBe(true);
    expect(internals.roomLane.get('racer')).toBe('fast:2|primary');
    await pool.close();
  });

  it('holds the lane slot for the whole attempt, so the concurrency cap actually caps', async () => {
    const pool = new ViewerPool({ userDataDir: '/tmp/x' });
    const internals = poolInternals(pool);
    // Three parked tabs on the single direct lane; each capture navigates
    // and emits its capture response after a delay, so all three start
    // concurrently before any settles.
    const lanes = internals.lanes.get('|primary')!;
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
    const lanes = internals.lanes.get('|primary')!;
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

  it('an attempt on lane B does not steal lane A\'s warm tab for the room', async () => {
    const pool = new ViewerPool({ proxyHosts: ['laneA:1', 'laneB:2'], userDataDir: '/tmp/x' });
    const internals = poolInternals(pool);
    // A live warm tab for the room sits on lane A. Lane A is benched, so
    // pickLane's round-robin hands the first attempt to lane B — with the
    // warm tab still registered. Pre-fix code keyed reuse by username
    // alone and SPA-navigated lane A's page from lane B's attempt,
    // mixing the room's session across profiles; post-fix the attempt
    // takes lane B's own prewarmed tab.
    const warm = fakePage({
      responses: [
        { url: 'https://webcast.tiktok.com/webcast/im/fetch/?room_id=5', status: 200, body: GOOD_BODY }
      ]
    });
    seedTab(internals, 'room', warm.page, 'wss://webcast-ws.example/room-a');
    internals.roomLane.set('room', 'laneA:1|primary');
    internals.lanes.get('laneA:1|primary')!.benchedUntil = Date.now() + 60_000;

    const laneBPage = fakePage({
      responses: [
        { url: 'https://webcast.tiktok.com/webcast/im/fetch/?room_id=5', status: 200, body: GOOD_BODY }
      ]
    });
    internals.lanes.get('laneB:2|primary')!.prewarmed.push(laneBPage.page);

    const capture = await pool.captureRoom('room', { timeoutMs: 20_000 });
    expect(capture.roomId).toBe('5');
    // Same-page guard: lane B's page is not the page the registered wsUrl
    // was captured on, and its own tap saw no webcast socket — so the
    // capture must stand empty rather than borrow lane A's wsUrl. A
    // username-only keep would hand back 'wss://webcast-ws.example/room-a'
    // (this is the round-1 guard regression this pins).
    expect(capture.wsUrl).toBe('');
    // Lane A's warm tab was never touched by lane B's attempt.
    expect(warm.navLog).not.toContain('spa');
    // The capture came from lane B's own page, and the room is now
    // registered with that page.
    expect(internals.tabs.get('room')?.page).toBe(laneBPage.page);
    expect(internals.tabs.get('room')?.wsUrl).toBe('');
    expect(internals.roomLane.get('room')).toBe('laneB:2|primary');
    await pool.close();
  });

  it('only the race winner registers its tab: a loser that reaches onResponse does not overwrite it', async () => {
    const pool = new ViewerPool({ proxyHosts: ['pin:1', 'win:2', 'lose:3'], userDataDir: '/tmp/x' });
    const internals = poolInternals(pool);
    // First attempt (pinned lane) dies instantly; the race then rides the
    // other two lanes. Both racers reach a capture: the winner's cookie
    // read is held open (cookieDelayMs) so the loser's onResponse runs
    // while the winner is still mid-registration — the exact window the
    // claim token exists for. Pre-fix code let the loser's registration
    // win; post-fix the loser sees the claim taken and closes its page.
    const pin = fakePage({ navError: true });
    internals.lanes.get('pin:1|primary')!.prewarmed.push(pin.page);
    const winner = fakePage({
      responses: [
        { url: 'https://webcast.tiktok.com/webcast/im/fetch/?room_id=11', status: 200, body: GOOD_BODY }
      ],
      cookieDelayMs: 40
    });
    internals.lanes.get('win:2|primary')!.prewarmed.push(winner.page);
    const loser = fakePage({
      responses: [
        { url: 'https://webcast.tiktok.com/webcast/im/fetch/?room_id=12', status: 200, body: GOOD_BODY }
      ]
    });
    internals.lanes.get('lose:3|primary')!.prewarmed.push(loser.page);
    internals.roomLane.set('roomx', 'pin:1|primary');

    const capture = await pool.captureRoom('roomx', { timeoutMs: 20_000 });
    expect(capture.roomId).toBe('11');
    // The registered tab is the winner's live page, not the loser's.
    expect(internals.tabs.get('roomx')?.page).toBe(winner.page);
    expect(internals.roomLane.get('roomx')).toBe('win:2|primary');
    // The loser closed its own page without registering or benching, and
    // its lane accrued no failure: a lane that delivered a valid capture
    // a hair slower must not count toward profile rotation.
    expect(loser.navLog).not.toContain('spa');
    expect(internals.lanes.get('lose:3|primary')!.consecutiveFailures).toBe(0);
    await pool.close();
  });

  it('records the webcast WS URL from the CDP tap into the capture and the registered tab', async () => {
    const pool = new ViewerPool({ userDataDir: '/tmp/x' });
    const internals = poolInternals(pool);
    const wsUrl = 'wss://webcast-ws.eu.tiktok.com/webcast/im/ws_proxy/ws_reuse_supplement/?x=1';
    const { page } = fakePage({
      wsCreated: ['wss://im-ws.tiktok.com/analytics', wsUrl],
      responses: [
        { url: 'https://webcast.tiktok.com/webcast/im/fetch/?room_id=42', status: 200, body: GOOD_BODY }
      ]
    });
    internals.lanes.get('|primary')!.prewarmed.push(page);

    const capture = await pool.captureRoom('someuser', { timeoutMs: 10_000 });
    // The webcast filter keeps the push URL, not the analytics socket.
    expect(capture.wsUrl).toBe(wsUrl);
    expect(internals.tabs.get('someuser')?.wsUrl).toBe(wsUrl);
    await pool.close();
  });

  it('resolves with an empty wsUrl when the page never opened a webcast socket', async () => {
    const pool = new ViewerPool({ userDataDir: '/tmp/x' });
    const internals = poolInternals(pool);
    const { page } = fakePage({
      responses: [
        { url: 'https://webcast.tiktok.com/webcast/im/fetch/?room_id=42', status: 200, body: GOOD_BODY }
      ]
    });
    internals.lanes.get('|primary')!.prewarmed.push(page);

    const capture = await pool.captureRoom('someuser', { timeoutMs: 10_000 });
    expect(capture.wsUrl).toBe('');
    await pool.close();
  });

  it('keeps the previous wsUrl and its original capturedAt when a warm re-capture sees no fresh socket', async () => {
    const pool = new ViewerPool({ userDataDir: '/tmp/x' });
    const internals = poolInternals(pool);
    const originalStamp = Date.now() - 60_000;
    // The re-capture page emits an im/fetch response but NO
    // webSocketCreated: registered as the warm tab (as a real capture
    // would have), so the reuse path SPA-navigates it.
    const { page } = fakePage({
      responses: [
        { url: 'https://webcast.tiktok.com/webcast/im/fetch/?room_id=7', status: 200, body: GOOD_BODY }
      ]
    });
    internals.tabs.set('warmuser', {
      page,
      lastUsed: Date.now(),
      wsUrl: 'wss://webcast-ws.example/original',
      roomId: '7',
      capturedAt: originalStamp
    });
    internals.roomLane.set('warmuser', '|primary');

    const capture = await pool.captureRoom('warmuser', { timeoutMs: 10_000 });
    expect(capture.wsUrl).toBe('wss://webcast-ws.example/original');
    const entry = internals.tabs.get('warmuser')!;
    expect(entry.wsUrl).toBe('wss://webcast-ws.example/original');
    // The stamp describes the KEPT URL's capture, not the re-capture.
    expect(entry.capturedAt).toBe(originalStamp);
    await pool.close();
  });

  it('refreshes capturedAt when a re-capture records a new non-empty wsUrl', async () => {
    const pool = new ViewerPool({ userDataDir: '/tmp/x' });
    const internals = poolInternals(pool);
    const originalStamp = Date.now() - 300_000;
    const freshUrl = 'wss://webcast-ws.example/fresh';
    const { page } = fakePage({
      wsCreated: [freshUrl],
      responses: [
        { url: 'https://webcast.tiktok.com/webcast/im/fetch/?room_id=7', status: 200, body: GOOD_BODY }
      ]
    });
    internals.tabs.set('warmuser', {
      page,
      lastUsed: Date.now(),
      wsUrl: 'wss://webcast-ws.example/original',
      roomId: '7',
      capturedAt: originalStamp
    });
    internals.roomLane.set('warmuser', '|primary');

    const capture = await pool.captureRoom('warmuser', { timeoutMs: 10_000 });
    expect(capture.wsUrl).toBe(freshUrl);
    const entry = internals.tabs.get('warmuser')!;
    expect(entry.wsUrl).toBe(freshUrl);
    expect(entry.capturedAt).toBeGreaterThan(originalStamp);
    await pool.close();
  });

  it('closes the CDP session on the giveUp failure path', async () => {
    const pool = new ViewerPool({ userDataDir: '/tmp/x' });
    const internals = poolInternals(pool);
    const { page } = fakePage({ livePlayerFoundAt: Number.MAX_SAFE_INTEGER });
    internals.lanes.get('|primary')!.prewarmed.push(page);

    vi.useFakeTimers();
    const attempt = pool.captureRoom('zombie', { timeoutMs: 60_000 });
    const outcome = expect(attempt).rejects.toThrow(/live player not up/);
    try {
      await vi.advanceTimersByTimeAsync(15_100);
      await outcome;
    } finally {
      vi.useRealTimers();
    }
    expect((page as unknown as { cdpDetached: () => boolean }).cdpDetached()).toBe(true);
    await pool.close();
  });

  it('race candidates exclude the other profile class (registered lane asserted)', async () => {
    // First attempt on a:1 primary dies instantly; the race must ride
    // only b:2 PRIMARY. If the class filter regressed, the healthy canary
    // lane of a:1 would join the race, win it, and register a PRIMARY
    // room inside the canary jar — the isolation failure item 6 exists
    // to prevent. Asserted on the registered lane key, not a filter echo.
    const pool = new ViewerPool({
      proxyHosts: ['a:1', 'b:2'],
      userDataDir: '/tmp/x',
      canaryProfileDir: '/tmp/x-canary',
      canaryRooms: new Set(['canaryroom'])
    });
    const internals = poolInternals(pool);
    internals.roomLane.set('primaryroom', 'a:1|primary');
    const bad = fakePage({ navError: true });
    internals.lanes.get('a:1|primary')!.prewarmed.push(bad.page);
    const good = fakePage({
      responses: [
        { url: 'https://webcast.tiktok.com/webcast/im/fetch/?room_id=5', status: 200, body: GOOD_BODY }
      ]
    });
    internals.lanes.get('b:2|primary')!.prewarmed.push(good.page);
    // A healthy page on the canary lane of a:1: must never be picked up.
    internals.lanes.get('a:1|canary')!.prewarmed.push(good.page);

    const capture = await pool.captureRoom('primaryroom', { timeoutMs: 20_000 });
    expect(capture.roomId).toBe('5');
    expect(internals.roomLane.get('primaryroom')).toBe('b:2|primary');
    await pool.close();
  });
});
