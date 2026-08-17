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

import { describe, expect, it } from 'vitest';

import { classifySignatureFailure, SignatureFailure } from './signer.js';

/** Build an error carrying one of the connector's typed error names. */
function named(name: string, message = 'something went wrong'): Error {
  const error = new Error(message);
  error.name = name;
  return error;
}

describe('classifySignatureFailure', () => {
  describe("the connector's typed sign errors", () => {
    it('classifies SignatureRateLimitError as rate_limit', () => {
      expect(classifySignatureFailure(named('SignatureRateLimitError'))).toBe('rate_limit');
    });

    it('classifies PremiumFeatureError as paywall', () => {
      expect(classifySignatureFailure(named('PremiumFeatureError'))).toBe('paywall');
    });

    it('classifies SignatureMissingTokensError as signature', () => {
      expect(classifySignatureFailure(named('SignatureMissingTokensError'))).toBe('signature');
    });

    it('prefers the typed name over whatever the message happens to say', () => {
      // The connector puts Euler's own prose into these messages, and that prose is not stable.
      // Keying off the type keeps the metric meaningful across sign-server wording changes.
      expect(
        classifySignatureFailure(named('PremiumFeatureError', 'connection timed out'))
      ).toBe('paywall');
    });
  });

  describe("the free tier's undefined retry-after crash", () => {
    it("classifies the connector's TypeError as rate_limit", () => {
      // Observed on 2026-08-14 and quoted verbatim in #698: twelve concurrent connection
      // attempts exhausted the free-tier sign limit, and the connector threw while reading the
      // 429 response rather than surfacing SignatureRateLimitError.
      //
      // Without this arm the single most important failure mode in the whole issue lands in
      // `unknown`, and the graph that is supposed to show the rate ceiling going away shows
      // nothing at all.
      const error = new TypeError("Cannot read properties of undefined (reading 'retry-after')");

      expect(classifySignatureFailure(error)).toBe('rate_limit');
    });

    it('matches regardless of the surrounding wording', () => {
      expect(
        classifySignatureFailure(
          new TypeError("Cannot read property 'retry-after' of undefined")
        )
      ).toBe('rate_limit');
    });
  });

  describe('message-based fallbacks', () => {
    it('classifies an explicit rate limit message', () => {
      expect(
        classifySignatureFailure(new Error('Too many connections started, try again later.'))
      ).toBe('rate_limit');
    });

    it('classifies the Business plan paywall message', () => {
      // The exact string fetchAvailableGifts() returns today.
      expect(
        classifySignatureFailure(new Error('This endpoint requires a Business plan.'))
      ).toBe('paywall');
    });

    it.each([
      'connect ECONNREFUSED 10.0.0.1:443',
      'connect ETIMEDOUT',
      'getaddrinfo ENOTFOUND api.eulerstream.com',
      'socket hang up',
      'Failed to connect to sign server.'
    ])('classifies %s as network', message => {
      expect(classifySignatureFailure(new Error(message))).toBe('network');
    });

    it('classifies a rejected signature as signature', () => {
      expect(classifySignatureFailure(new Error('Sign Error: invalid X-Bogus'))).toBe('signature');
    });

    it('is case insensitive', () => {
      expect(classifySignatureFailure(new Error('TOO MANY CONNECTIONS'))).toBe('rate_limit');
    });
  });

  describe('inputs that are not well-formed errors', () => {
    it('returns unknown for an unrecognised message', () => {
      expect(classifySignatureFailure(new Error('the sky is falling'))).toBe('unknown');
    });

    it.each([[undefined], [null], [42], [{}]])('does not throw on %s', value => {
      // Anything can be thrown in JavaScript, and a classifier that throws while classifying
      // would turn a signature failure into a crash on the connect path.
      expect(() => classifySignatureFailure(value)).not.toThrow();
      expect(classifySignatureFailure(value)).toBe('unknown');
    });

    it('reads a thrown string', () => {
      expect(classifySignatureFailure('rate limit exceeded')).toBe('rate_limit');
    });

    it('returns a bounded set of values, since these become metric labels', () => {
      const permitted = new Set(['rate_limit', 'paywall', 'signature', 'network', 'unknown']);
      const samples: unknown[] = [
        named('SignatureRateLimitError'),
        named('PremiumFeatureError'),
        named('SignatureMissingTokensError'),
        new TypeError("Cannot read properties of undefined (reading 'retry-after')"),
        new Error('socket hang up'),
        new Error('Sign Error'),
        new Error('???'),
        undefined,
        'nonsense'
      ];

      for (const sample of samples) {
        expect(permitted).toContain(classifySignatureFailure(sample));
      }
    });
  });
});

describe('SignatureFailure', () => {
  it('names the signer in the message so on-call knows which one broke', () => {
    const failure = new SignatureFailure('self', 'TikTok rejected the signature');

    expect(failure.message).toBe('[self] TikTok rejected the signature');
    expect(failure.signer).toBe('self');
  });

  it('preserves the underlying error as the cause', () => {
    const cause = new Error('X-Gnarly mismatch');

    expect(new SignatureFailure('self', 'wrapped', cause).cause).toBe(cause);
  });

  it('is an Error, so existing catch and logging paths handle it', () => {
    const failure = new SignatureFailure('euler', 'nope');

    expect(failure).toBeInstanceOf(Error);
    expect(failure.name).toBe('SignatureFailure');
  });
});
