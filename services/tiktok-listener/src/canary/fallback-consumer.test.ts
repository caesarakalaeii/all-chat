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
import { EventEmitter } from 'node:events';
import { FallbackConsumer } from './fallback-consumer.js';
import { makeAckFrame, makeChatFrame } from './relay-fixtures.js';

// The fallback's contract is DELIVERY: a relay frame must arrive at the
// connection as a decoded message that the connector's own pipeline can
// emit chat events from. The connection stand-in records the
// ProtoMessageFetchResult it was handed; the frames come from
// relay-fixtures so decode is the real connector path, not a mock echo.

interface RecordedCall {
  messages: Array<{ method?: string }>;
}

/** Stand-in for a DISCONNECTED TikTokLiveConnection: records replays. */
function makeConnection(): { emitter: EventEmitter; calls: RecordedCall[] } {
  const emitter = new EventEmitter();
  const calls: RecordedCall[] = [];
  (emitter as unknown as {
    processProtoMessageFetchResult(fr: { messages?: Array<{ method?: string }> }): Promise<void>;
  }).processProtoMessageFetchResult = async (fr) => {
    calls.push({ messages: fr.messages ?? [] });
  };
  return { emitter, calls };
}

function makeConsumer(emitter: EventEmitter) {
  const consumer = new FallbackConsumer({
    username: 'lab-test',
    signerUrl: 'http://signer.invalid:8092',
    connection: emitter as never,
    maxDurationMs: 60_000
  });
  const deliver = (base64: string) =>
    (consumer as unknown as { deliverFrame(b: string): Promise<void> }).deliverFrame(base64);
  return { consumer, deliver };
}

describe('FallbackConsumer delivery', () => {
  it('replays a real chat PushFrame into the connection', async () => {
    const { emitter, calls } = makeConnection();
    const { consumer, deliver } = makeConsumer(emitter);
    const outcomes: string[] = [];
    consumer.on('delivered', (o: string) => outcomes.push(o));

    const frame = await makeChatFrame('hello from the relay');
    await deliver(frame.base64);

    expect(outcomes).toEqual(['delivered']);
    expect(calls).toHaveLength(1);
    expect(calls[0].messages[0].method).toBe('WebcastChatMessage');
  });

  it('reports ack frames as no_messages without replaying', async () => {
    const { emitter, calls } = makeConnection();
    const { consumer, deliver } = makeConsumer(emitter);
    const outcomes: string[] = [];
    consumer.on('delivered', (o: string) => outcomes.push(o));

    await deliver(await makeAckFrame());

    expect(outcomes).toEqual(['no_messages']);
    expect(calls).toHaveLength(0);
  });

  it('counts a malformed payload as decode_failure and keeps consuming', async () => {
    const { emitter, calls } = makeConnection();
    const { consumer, deliver } = makeConsumer(emitter);
    const outcomes: string[] = [];
    consumer.on('delivered', (o: string) => outcomes.push(o));

    await deliver('bm90LXByb3RvYnVm'); // "not-proobuf"
    const frame = await makeChatFrame('still delivering');
    await deliver(frame.base64);

    expect(outcomes).toEqual(['decode_failure', 'delivered']);
    expect(calls).toHaveLength(1);
  });

  it('survives a replay error and keeps consuming', async () => {
    const emitter = new EventEmitter();
    (emitter as unknown as {
      processProtoMessageFetchResult(): Promise<void>;
    }).processProtoMessageFetchResult = async () => {
      throw new Error('room state stale');
    };
    const { consumer, deliver } = makeConsumer(emitter);
    const outcomes: string[] = [];
    consumer.on('delivered', (o: string) => outcomes.push(o));

    const frame = await makeChatFrame('after the error');
    await deliver(frame.base64);

    expect(outcomes).toEqual(['decode_failure']);
  });

  it('ends the stint on relay ws_closed and emits ended exactly once', () => {
    const { emitter } = makeConnection();
    const { consumer } = makeConsumer(emitter);
    const ended: string[] = [];
    consumer.on('ended', (reason: string) => ended.push(reason));

    (consumer as unknown as { handleSseChunk(c: string): void })
      .handleSseChunk('event: state\ndata: {"type":"state","event":"ws_closed"}');

    expect(ended).toEqual(['ws_closed']);
    expect((consumer as unknown as { stopped: boolean }).stopped).toBe(true);
  });

  it('ends the stint at max duration', () => {
    vi.useFakeTimers();
    try {
      const { emitter } = makeConnection();
      const consumer = new FallbackConsumer({
        username: 'lab-test',
        signerUrl: 'http://signer.invalid:8092',
        connection: emitter as never,
        maxDurationMs: 1000
      });
      const ended: string[] = [];
      consumer.on('ended', (reason: string) => ended.push(reason));
      consumer.start();
      vi.advanceTimersByTime(1001);
      expect(ended).toEqual(['max_duration']);
    } finally {
      vi.useRealTimers();
    }
  });

  it('emits ended rejected when the signer answers 404 for the room', () => {
    const { emitter } = makeConnection();
    const { consumer } = makeConsumer(emitter);
    const ended: string[] = [];
    consumer.on('ended', (reason: string) => ended.push(reason));

    // The RelayStream base emits 'rejected' after a 404; simulate it the
    // way the service would observe it.
    (consumer as unknown as { emit(e: string, ...a: unknown[]): void }).emit('rejected');

    expect(ended).toEqual(['rejected']);
  });

  it('ignores frames delivered after the stint ended', async () => {
    const { emitter, calls } = makeConnection();
    const { consumer, deliver } = makeConsumer(emitter);
    (consumer as unknown as { handleSseChunk(c: string): void })
      .handleSseChunk('event: state\ndata: {"type":"state","event":"ws_closed"}');

    const frame = await makeChatFrame('too late');
    await deliver(frame.base64);

    expect(calls).toHaveLength(0);
  });
});
