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

import { beforeEach, describe, expect, it } from 'vitest';

import { FallbackSigner, ShadowSigner, type SignAttempt, type SignObserver } from './shadow.js';
import type { SignRequest, SignResult, WebcastSigner } from './signer.js';

/** Collects attempts so assertions can read them back. */
class RecordingObserver implements SignObserver {
  readonly attempts: SignAttempt[] = [];

  recordSignAttempt(attempt: SignAttempt): void {
    this.attempts.push(attempt);
  }

  /** Attempts made by one signer, in order. */
  by(signer: string): SignAttempt[] {
    return this.attempts.filter(a => a.signer === signer);
  }
}

/** A signer whose behaviour the test dictates. */
class StubSigner implements WebcastSigner {
  calls: SignRequest[] = [];
  /** Resolved externally, so a test can control the interleaving of two signers. */
  private gate?: { promise: Promise<void>; release: () => void };

  constructor(
    readonly name: string,
    private readonly behaviour: () => SignResult | Promise<SignResult>
  ) {}

  /** Make this signer block until `release()` is called. */
  block(): () => void {
    let release!: () => void;
    const promise = new Promise<void>(resolve => {
      release = resolve;
    });
    this.gate = { promise, release };
    return release;
  }

  async sign(request: SignRequest): Promise<SignResult> {
    this.calls.push(request);
    if (this.gate) await this.gate.promise;
    return this.behaviour();
  }
}

function ok(marker: string): SignResult {
  return { fetchResult: { marker }, fetchResultCookieHeader: `c=${marker}` };
}

function failing(name: string, message: string, errorName?: string): StubSigner {
  return new StubSigner(name, () => {
    const error = new Error(message);
    if (errorName) error.name = errorName;
    throw error;
  });
}

const request: SignRequest = { roomId: '7300000000000000000', userAgent: 'UA/1.0' };

describe('ShadowSigner', () => {
  let observer: RecordingObserver;

  beforeEach(() => {
    observer = new RecordingObserver();
  });

  it('returns the primary result, never the candidate one', async () => {
    // The whole safety property of shadow mode: enabling it cannot change which signature the
    // connection actually uses, so it is safe to leave on in production while gathering data.
    const primary = new StubSigner('euler', () => ok('primary'));
    const candidate = new StubSigner('self', () => ok('candidate'));

    const result = await new ShadowSigner(primary, candidate, observer).sign(request);

    expect(result).toEqual(ok('primary'));
  });

  it('runs the candidate against the same room', async () => {
    const primary = new StubSigner('euler', () => ok('primary'));
    const candidate = new StubSigner('self', () => ok('candidate'));

    await new ShadowSigner(primary, candidate, observer).sign(request);

    // Comparing success rates is only meaningful if both signers saw the same input.
    expect(candidate.calls).toEqual([request]);
    expect(primary.calls).toEqual([request]);
  });

  it('starts the candidate before awaiting the primary, so the two overlap', async () => {
    // Sequencing matters for the measurement itself: TikTok's behaviour varies by room and by
    // minute, so a candidate run after the primary finished is not a like-for-like comparison.
    const primary = new StubSigner('euler', () => ok('primary'));
    const candidate = new StubSigner('self', () => ok('candidate'));
    const releasePrimary = primary.block();

    const pending = new ShadowSigner(primary, candidate, observer).sign(request);
    await Promise.resolve();

    expect(candidate.calls).toHaveLength(1);

    releasePrimary();
    await pending;
  });

  it('records both attempts, marking only the primary as load bearing', async () => {
    const primary = new StubSigner('euler', () => ok('primary'));
    const candidate = new StubSigner('self', () => ok('candidate'));

    await new ShadowSigner(primary, candidate, observer).sign(request);

    expect(observer.by('euler')).toMatchObject([{ outcome: 'success', loadBearing: true }]);
    expect(observer.by('self')).toMatchObject([{ outcome: 'success', loadBearing: false }]);
  });

  it('swallows a candidate failure entirely', async () => {
    const primary = new StubSigner('euler', () => ok('primary'));
    const candidate = failing('self', 'signature rejected by TikTok');

    const result = await new ShadowSigner(primary, candidate, observer).sign(request);

    expect(result).toEqual(ok('primary'));
    expect(observer.by('self')).toMatchObject([
      { outcome: 'failure', reason: 'signature', loadBearing: false }
    ]);
  });

  it('still records the candidate when the primary fails', async () => {
    // This is the most valuable single data point in the whole experiment: Euler failing is
    // exactly when we want to know whether our own signer would have carried the connection.
    const primary = failing('euler', 'Too many connections started, try again later.');
    const candidate = new StubSigner('self', () => ok('candidate'));

    await expect(new ShadowSigner(primary, candidate, observer).sign(request)).rejects.toThrow(
      /Too many connections/
    );

    expect(observer.by('euler')).toMatchObject([{ outcome: 'failure', reason: 'rate_limit' }]);
    expect(observer.by('self')).toMatchObject([{ outcome: 'success', loadBearing: false }]);
  });

  it('waits for a slow candidate rather than dropping its result', async () => {
    const primary = new StubSigner('euler', () => ok('primary'));
    const candidate = new StubSigner('self', () => ok('candidate'));
    const releaseCandidate = candidate.block();

    let settled = false;
    const pending = new ShadowSigner(primary, candidate, observer)
      .sign(request)
      .finally(() => {
        settled = true;
      });

    // Give the primary every chance to finish and resolve the whole call. If the candidate were
    // not awaited, `sign` would already have returned here and its sample would be lost whenever
    // the process moves on to the next connect before the shadow request lands.
    for (let i = 0; i < 50; i++) await Promise.resolve();

    expect(settled).toBe(false);
    expect(observer.by('self')).toHaveLength(0);

    releaseCandidate();
    await pending;

    // Losing shadow samples silently would bias the measured success rate towards whichever
    // outcome happens to be fast, so the candidate is always settled before we return.
    expect(observer.by('self')).toHaveLength(1);
  });

  it('propagates the primary error unchanged', async () => {
    const primary = failing('euler', 'boom', 'SignAPIError');
    const candidate = new StubSigner('self', () => ok('candidate'));

    await expect(new ShadowSigner(primary, candidate, observer).sign(request)).rejects.toMatchObject(
      { name: 'SignAPIError', message: 'boom' }
    );
  });

  it('names both signers so logs say which experiment was running', () => {
    const signer = new ShadowSigner(
      new StubSigner('euler', () => ok('p')),
      new StubSigner('self', () => ok('c')),
      observer
    );

    expect(signer.name).toBe('shadow(euler->self)');
  });
});

describe('FallbackSigner', () => {
  let observer: RecordingObserver;

  beforeEach(() => {
    observer = new RecordingObserver();
  });

  it('uses the preferred signer and does not touch the fallback when it works', async () => {
    const preferred = new StubSigner('self', () => ok('self'));
    const fallback = new StubSigner('euler', () => ok('euler'));

    const result = await new FallbackSigner(preferred, fallback, observer).sign(request);

    expect(result).toEqual(ok('self'));
    // The point of cutting over is to stop spending Euler's free-tier signs. If the fallback
    // were consulted on the happy path we would have changed nothing about the rate ceiling.
    expect(fallback.calls).toHaveLength(0);
  });

  it('falls back to Euler when our signer fails', async () => {
    const preferred = failing('self', 'signature rejected');
    const fallback = new StubSigner('euler', () => ok('euler'));

    const result = await new FallbackSigner(preferred, fallback, observer).sign(request);

    expect(result).toEqual(ok('euler'));
  });

  it('records the preferred failure even though the fallback rescued the connection', async () => {
    // Without this, a completely broken self-signer is indistinguishable from a healthy one
    // from the outside, and we would only find out when we turned the fallback off.
    const preferred = failing('self', 'signature rejected');
    const fallback = new StubSigner('euler', () => ok('euler'));

    await new FallbackSigner(preferred, fallback, observer).sign(request);

    expect(observer.by('self')).toMatchObject([
      { outcome: 'failure', reason: 'signature', loadBearing: true }
    ]);
    expect(observer.by('euler')).toMatchObject([{ outcome: 'success', loadBearing: true }]);
  });

  it('reports both reasons when neither signer can produce a signature', async () => {
    const preferred = failing('self', 'signature rejected');
    const fallback = failing('euler', 'Too many connections started, try again later.');

    await expect(
      new FallbackSigner(preferred, fallback, observer).sign(request)
    ).rejects.toThrow(/self \(signature\).*euler \(rate_limit\)/);
  });

  it('preserves the fallback error as the cause', async () => {
    const preferred = failing('self', 'signature rejected');
    const underlying = new Error('Too many connections started, try again later.');
    const fallback = new StubSigner('euler', () => {
      throw underlying;
    });

    await expect(
      new FallbackSigner(preferred, fallback, observer).sign(request)
    ).rejects.toMatchObject({ cause: underlying });
  });

  it('passes the same request through to the fallback', async () => {
    const preferred = failing('self', 'nope');
    const fallback = new StubSigner('euler', () => ok('euler'));

    await new FallbackSigner(preferred, fallback, observer).sign(request);

    expect(fallback.calls).toEqual([request]);
  });
});
