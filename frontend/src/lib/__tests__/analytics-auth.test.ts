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

import { describe, it, expect, vi, beforeEach } from 'vitest'

// Node test env has no sessionStorage — stub the minimal surface the helpers
// touch (mirrors the localStorage stub in onboarding-store.test.ts).
const sessionStorageStub = (() => {
  let data: Record<string, string> = {}
  return {
    getItem: (key: string) => data[key] ?? null,
    setItem: (key: string, value: string) => {
      data[key] = value
    },
    removeItem: (key: string) => {
      delete data[key]
    },
    clear: () => {
      data = {}
    },
  }
})()
vi.stubGlobal('sessionStorage', sessionStorageStub)

import {
  resolveSigninPlatform,
  stashSigninPlatform,
  readAndClearSigninPlatform,
  sanitizeViewerPlatform,
  bucketViewerAuthError,
  SIGNIN_PLATFORM_KEY,
} from '../analytics-auth'

beforeEach(() => sessionStorageStub.clear())

describe('resolveSigninPlatform', () => {
  it('prefers a valid stashed platform over everything else', () => {
    // The user has linked Google, but they just clicked "Sign in with Twitch".
    expect(resolveSigninPlatform({ google_id: 'g1' }, 'twitch')).toBe('twitch')
  })

  it('ignores an unknown/tampered stashed value', () => {
    expect(resolveSigninPlatform({ twitch_id: 't1' }, 'evil')).toBe('twitch')
  })

  it('uses auth_provider when it is a known platform', () => {
    expect(resolveSigninPlatform({ auth_provider: 'youtube' }, null)).toBe('youtube')
  })

  it('falls back to *_id inference only when it is unambiguous (exactly one linked id)', () => {
    expect(resolveSigninPlatform({ twitch_id: 't1' }, null)).toBe('twitch')
    expect(resolveSigninPlatform({ google_id: 'g1' }, null)).toBe('youtube')
    expect(resolveSigninPlatform({ kick_id: 'k1' }, null)).toBe('kick')
  })

  it('normalizes a non-known auth_provider (e.g. "google") via *_id inference', () => {
    expect(resolveSigninPlatform({ auth_provider: 'google', google_id: 'g1' }, null)).toBe('youtube')
  })

  it('returns "unknown" for a multi-linked account with no stash/known auth_provider', () => {
    // Ambiguous: don't guess (and skew per-platform stats) — report unknown.
    expect(resolveSigninPlatform({ twitch_id: 't1', google_id: 'g1' }, null)).toBe('unknown')
  })

  it('a valid stash or known auth_provider still resolves a multi-linked account', () => {
    expect(resolveSigninPlatform({ twitch_id: 't1', google_id: 'g1' }, 'kick')).toBe('kick')
    expect(resolveSigninPlatform({ auth_provider: 'youtube', twitch_id: 't1', google_id: 'g1' }, null)).toBe(
      'youtube'
    )
  })

  it('returns "unknown" when nothing is attributable', () => {
    expect(resolveSigninPlatform({}, null)).toBe('unknown')
    expect(resolveSigninPlatform(null, null)).toBe('unknown')
    expect(resolveSigninPlatform(undefined, undefined)).toBe('unknown')
  })
})

describe('sessionStorage stash round-trip', () => {
  it('stashes then reads-and-clears the platform', () => {
    stashSigninPlatform('kick')
    expect(sessionStorageStub.getItem(SIGNIN_PLATFORM_KEY)).toBe('kick')
    expect(readAndClearSigninPlatform()).toBe('kick')
    // Cleared after the first read so a later callback can't reuse a stale value.
    expect(readAndClearSigninPlatform()).toBeNull()
  })

  it('returns null when nothing was stashed', () => {
    expect(readAndClearSigninPlatform()).toBeNull()
  })
})

describe('sanitizeViewerPlatform', () => {
  it('passes through known viewer platforms', () => {
    for (const p of ['twitch', 'youtube', 'kick', 'tiktok', 'discord']) {
      expect(sanitizeViewerPlatform(p)).toBe(p)
    }
  })

  it('collapses unknown / prototype-polluting / empty values to "unknown"', () => {
    expect(sanitizeViewerPlatform('toString')).toBe('unknown')
    expect(sanitizeViewerPlatform('evil')).toBe('unknown')
    expect(sanitizeViewerPlatform('')).toBe('unknown')
    expect(sanitizeViewerPlatform(null)).toBe('unknown')
  })
})

describe('bucketViewerAuthError', () => {
  it('never echoes raw free-form text — unknown shapes bucket to "other"', () => {
    // Privacy: the raw error is backend-authored and could carry detail we must
    // not send to analytics, so an unrecognized message must NOT be returned verbatim.
    const weird = 'unexpected internal detail 12345 user@example.com'
    expect(bucketViewerAuthError(weird)).toBe('other')
  })

  it('maps known error shapes to stable enum slugs', () => {
    expect(bucketViewerAuthError('access_denied')).toBe('access_denied')
    expect(bucketViewerAuthError('The user denied the request')).toBe('access_denied')
    expect(bucketViewerAuthError('The code may have expired')).toBe('code_expired')
    expect(bucketViewerAuthError('No authentication code received')).toBe('no_code')
    expect(bucketViewerAuthError('missing required scope')).toBe('insufficient_scope')
    expect(bucketViewerAuthError('This account is banned')).toBe('banned')
    expect(bucketViewerAuthError('Your account was suspended')).toBe('banned')
    expect(bucketViewerAuthError('Exchange failed: 500')).toBe('exchange_failed')
  })

  it('does not misclassify words that merely contain "ban" as banned', () => {
    // 'ban' matched as a bare substring caught 'abandoned' / 'banner'.
    expect(bucketViewerAuthError('the request was abandoned')).toBe('other')
    expect(bucketViewerAuthError('invalid banner configuration')).toBe('other')
  })

  it('returns "unknown" for empty/nullish input', () => {
    expect(bucketViewerAuthError('')).toBe('unknown')
    expect(bucketViewerAuthError(null)).toBe('unknown')
    expect(bucketViewerAuthError(undefined)).toBe('unknown')
  })
})
