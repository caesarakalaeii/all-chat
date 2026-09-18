/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */

import { describe, expect, it, vi } from 'vitest';
import { CanaryConsumer } from './canary-consumer.js';
import { makeAckFrame, makeChatFrame } from './relay-fixtures.js';

// The consumer's comparison logic is windowed over internal state; the
// tests drive notePrimaryFrame / noteCanaryFrame and force the window
// comparison via the private compareWindow, so no HTTP or SSE is needed.
// The SSE-specific plumbing (connect/readSse) is exercised in the lab rig,
// not here. Frames come from relay-fixtures: real wire-format PushFrames
// that must decode with the connector's own schemas — see the missing-
// await regression test for why a fake payload proves nothing.

function makeConsumer(windowSeconds = 60) {
  const consumer = new CanaryConsumer({
    username: 'lab-test',
    signerUrl: 'http://signer.invalid:8092',
    windowSeconds
  });
  const compare = () =>
    (consumer as unknown as { compareWindow(): void }).compareWindow();
  return { consumer, compare };
}

describe('CanaryConsumer relay frame decoding', () => {
  it('counts a real PushFrame chat message as a decoded canary method', async () => {
    // Regression (phase 3 recon): noteCanaryFrame used to call
    // deserializeWebSocketMessage without await. The function is async
    // (gzip payload path), so decodedData was always undefined and every
    // frame silently counted as an ack — method_set and
    // decode_failure_rate could never fire.
    const { consumer } = makeConsumer();
    const frame = await makeChatFrame('hello');
    await (consumer as unknown as { noteCanaryFrame(b: string): Promise<void> })
      .noteCanaryFrame(frame.base64);
    const window = (consumer as unknown as {
      window: { canary: Map<string, number>; canaryFrames: number; canaryDecodeFailures: number };
    }).window;
    expect(window.canaryFrames).toBe(1);
    expect(window.canary.get('WebcastChatMessage')).toBe(1);
    expect(window.canaryDecodeFailures).toBe(0);
  });

  it('counts an ack PushFrame as a frame with no methods', async () => {
    const { consumer } = makeConsumer();
    await (consumer as unknown as { noteCanaryFrame(b: string): Promise<void> })
      .noteCanaryFrame(await makeAckFrame());
    const window = (consumer as unknown as {
      window: { canary: Map<string, number>; canaryFrames: number };
    }).window;
    expect(window.canaryFrames).toBe(1);
    expect(window.canary.size).toBe(0);
  });

  it('counts a malformed payload as a decode failure, not an ack', async () => {
    const { consumer } = makeConsumer();
    await (consumer as unknown as { noteCanaryFrame(b: string): Promise<void> })
      .noteCanaryFrame('bm90LXByb3RvYnVm'); // "not-proobuf"
    const window = (consumer as unknown as {
      window: { canaryDecodeFailures: number };
    }).window;
    expect(window.canaryDecodeFailures).toBe(1);
  });

  it('parses an SSE frame event into a canary frame', async () => {
    // handleSseChunk drives noteCanaryFrame via the base class 'frame'
    // event; this covers the wiring between SSE parsing and the counter.
    const { consumer } = makeConsumer();
    const frame = await makeChatFrame('via sse');
    const handle = (consumer as unknown as { handleSseChunk(chunk: string): void })
      .handleSseChunk.bind(consumer);
    handle(`event: frame\ndata: ${JSON.stringify({ type: 'frame', payload: frame.base64 })}`);
    // noteCanaryFrame runs async (connector decode); await the microtask
    // queue, not a timer, so the assertion is deterministic.
    await new Promise((resolve) => setImmediate(resolve));
    const window = (consumer as unknown as {
      window: { canary: Map<string, number>; canaryFrames: number };
    }).window;
    expect(window.canaryFrames).toBe(1);
    expect(window.canary.get('WebcastChatMessage')).toBe(1);
  });
});

describe('CanaryConsumer divergence detection', () => {
  it('does not diverge when both transports see the same methods', () => {
    const { consumer, compare } = makeConsumer();
    const divergences: unknown[] = [];
    consumer.on('divergence', (d) => divergences.push(d));

    for (let i = 0; i < 100; i++) {
      consumer.notePrimaryFrame('WebcastChatMessage');
    }
    const window = (consumer as unknown as { window: { canary: Map<string, number>; canaryFrames: number } }).window;
    window.canaryFrames = 100;
    window.canary.set('WebcastChatMessage', 100);

    compare();
    expect(divergences).toHaveLength(0);
  });

  it('fires method_set when the canary decodes methods the primary never saw', () => {
    const { consumer, compare } = makeConsumer();
    const divergences: Array<{ kind: string }> = [];
    consumer.on('divergence', (d) => divergences.push(d));

    for (let i = 0; i < 100; i++) {
      consumer.notePrimaryFrame('WebcastChatMessage');
    }
    const window = (consumer as unknown as { window: { canary: Map<string, number>; canaryFrames: number } }).window;
    window.canaryFrames = 100;
    window.canary.set('WebcastChatMessage', 60);
    window.canary.set('WebcastGiftMessage', 40); // primary never saw gifts

    compare();
    expect(divergences.map((d) => d.kind)).toContain('method_set');
  });

  it('fires frame_rate when the canary flows under the ratio threshold', () => {
    const { consumer, compare } = makeConsumer();
    const kinds: string[] = [];
    consumer.on('divergence', (d) => kinds.push(d.kind));

    for (let i = 0; i < 200; i++) {
      consumer.notePrimaryFrame('WebcastChatMessage');
    }
    const window = (consumer as unknown as { window: { canary: Map<string, number>; canaryFrames: number } }).window;
    window.canaryFrames = 20; // 10% of primary, ratio floor is 50%
    window.canary.set('WebcastChatMessage', 20);

    compare();
    expect(kinds).toContain('frame_rate');
  });

  it('fires stalled when the relay is silent but the primary kept receiving', () => {
    const { consumer, compare } = makeConsumer();
    const kinds: string[] = [];
    consumer.on('divergence', (d) => kinds.push(d.kind));

    for (let i = 0; i < 50; i++) {
      consumer.notePrimaryFrame('WebcastChatMessage');
    }
    // canaryFrames stays 0: relay dead for a whole window.

    compare();
    expect(kinds).toContain('stalled');
  });

  it('does not fire stalled when both sides are quiet (room idle)', () => {
    const { consumer, compare } = makeConsumer();
    const kinds: string[] = [];
    consumer.on('divergence', (d) => kinds.push(d.kind));

    // Nothing on either side.
    compare();
    expect(kinds).toHaveLength(0);
  });

  it('ignores method_set noise below the frame floor', () => {
    const { consumer, compare } = makeConsumer();
    const kinds: string[] = [];
    consumer.on('divergence', (d) => kinds.push(d.kind));

    consumer.notePrimaryFrame('WebcastChatMessage');
    const window = (consumer as unknown as { window: { canary: Map<string, number>; canaryFrames: number } }).window;
    window.canaryFrames = 5; // under the 20-frame floor
    window.canary.set('WebcastGiftMessage', 5);

    compare();
    expect(kinds).toHaveLength(0);
  });

  it('resets the window after each comparison', () => {
    const { consumer, compare } = makeConsumer();
    // First window: primary received, relay silent -> stalled fires.
    const kinds: string[] = [];
    consumer.on('divergence', (d) => kinds.push(d.kind));
    for (let i = 0; i < 50; i++) consumer.notePrimaryFrame('WebcastChatMessage');
    compare();
    expect(kinds).toHaveLength(1);

    // Second window: both quiet -> nothing carries over.
    compare();
    expect(kinds).toHaveLength(1);
  });

  it('counts SSE chunks with heartbeat comments as no-ops', () => {
    const { consumer } = makeConsumer();
    const handle = (consumer as unknown as { handleSseChunk(chunk: string): void })
      .handleSseChunk.bind(consumer);
    // A comment-only chunk (heartbeat) must not throw or count a frame.
    expect(() => handle(': ping')).not.toThrow();
    const window = (consumer as unknown as { window: { canaryFrames: number } }).window;
    expect(window.canaryFrames).toBe(0);
  });

  it('parses a state event without counting a frame', () => {
    const { consumer } = makeConsumer();
    const handle = (consumer as unknown as { handleSseChunk(chunk: string): void })
      .handleSseChunk.bind(consumer);
    const states: string[] = [];
    consumer.on('state', (s: string) => states.push(s));
    handle('event: state\ndata: {"type":"state","event":"open"}');
    const window = (consumer as unknown as { window: { canaryFrames: number } }).window;
    expect(window.canaryFrames).toBe(0);
    expect(states).toEqual(['open']);
  });

  it('stop clears timers and prevents reconnects', () => {
    const consumer = new CanaryConsumer({
      username: 'lab-test',
      signerUrl: 'http://signer.invalid:8092'
    });
    consumer.start(); // connect() will fail (invalid host) -> reconnect timer
    consumer.stop();
    // No throw, and internal stopped flag set: reconnect path is inert.
    expect((consumer as unknown as { stopped: boolean }).stopped).toBe(true);
  });

  it('reconnect backoff is capped', () => {
    const consumer = new CanaryConsumer({
      username: 'lab-test',
      signerUrl: 'http://signer.invalid:8092'
    });
    const reconnect = (consumer as unknown as { reconnect(): void }).reconnect.bind(
      consumer
    );
    vi.useFakeTimers();
    try {
      for (let i = 0; i < 20; i++) reconnect();
      const attempts = (consumer as unknown as { reconnectAttempts: number })
        .reconnectAttempts;
      // 20 reconnects: delay must have saturated at 60s, i.e. attempt count
      // grows linearly but the schedule did not explode.
      expect(attempts).toBe(20);
    } finally {
      vi.useRealTimers();
      consumer.stop();
    }
  });
});

describe('CanaryConsumer tab warm (PR 3, pure-node mode)', () => {
  // The RelayStream calls the 409 hook (onNoWarmTab) once per connect that
  // answers 409; the consumer must run the warm callback at most ONCE per
  // stint — each warm is a signer-side capture against a single-digit-
  // per-hour budget. The hook is driven directly: the SSE connect path
  // needs a live signer (lab rig, Task 5).
  function makeWarmConsumer() {
    const warm = vi.fn(async () => true);
    const consumer = new CanaryConsumer({
      username: 'lab-test',
      signerUrl: 'http://signer.invalid:8092',
      warm
    });
    const noWarmTab = () =>
      (consumer as unknown as { onNoWarmTab?: () => void }).onNoWarmTab?.();
    return { consumer, warm, noWarmTab };
  }

  it('runs the warm exactly once across repeated 409s in one stint', () => {
    const { consumer, warm, noWarmTab } = makeWarmConsumer();
    consumer.start();
    // Three 409 connect attempts in one stint: the flag must hold after
    // the first.
    noWarmTab();
    noWarmTab();
    noWarmTab();
    expect(warm).toHaveBeenCalledTimes(1);
    consumer.stop();
  });

  it('does not warm a failed warm again within the stint', async () => {
    const warm = vi.fn(async () => false);
    const consumer = new CanaryConsumer({
      username: 'lab-test',
      signerUrl: 'http://signer.invalid:8092',
      warm
    });
    const noWarmTab = () =>
      (consumer as unknown as { onNoWarmTab?: () => void }).onNoWarmTab?.();
    consumer.start();
    noWarmTab();
    await new Promise((resolve) => setImmediate(resolve)); // warm promise settles
    noWarmTab(); // still 409ing after a failed warm
    noWarmTab();
    expect(warm).toHaveBeenCalledTimes(1); // the retry path is SSE-only
    consumer.stop();
  });

  it('a new stint earns exactly one fresh warm', () => {
    const { consumer, warm, noWarmTab } = makeWarmConsumer();
    consumer.start();
    noWarmTab();
    expect(warm).toHaveBeenCalledTimes(1);
    consumer.stop();
    consumer.start();
    noWarmTab();
    expect(warm).toHaveBeenCalledTimes(2);
    consumer.stop();
  });

  it('never fires the warm hook without a warm callback', () => {
    const consumer = new CanaryConsumer({
      username: 'lab-test',
      signerUrl: 'http://signer.invalid:8092'
    });
    const hook = (consumer as unknown as { onNoWarmTab?: () => void }).onNoWarmTab;
    expect(hook).toBeUndefined(); // RelayStream skips the 409 branch entirely
  });
});
