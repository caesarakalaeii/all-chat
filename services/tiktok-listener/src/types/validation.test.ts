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

import { describe, it, expect } from 'vitest';
import {
  validateTikTokUsername,
  assertValidTikTokUsername,
  TIKTOK_USERNAME_PATTERN,
  TIKTOK_USERNAME_MIN_LENGTH,
  TIKTOK_USERNAME_MAX_LENGTH,
} from './validation';

// PR #478 review: the username regex is the actual trust boundary, so guard its
// accept/reject set against future permissiveness regressions (TDD).
describe('validateTikTokUsername', () => {
  it('accepts structurally valid usernames (incl. edge underscores/dots)', () => {
    for (const u of ['ab', 'A1', 'user.name', 'ok_user.1', '_lead', 'trail_', 'a'.repeat(24)]) {
      expect(validateTikTokUsername(u).valid, u).toBe(true);
    }
  });

  it('rejects empties, out-of-range lengths and unsafe characters', () => {
    for (const u of ['', 'a', 'a'.repeat(25), 'a/b', 'a b', ' ab', 'ab ', 'a@b', '../etc', 'a\nb', 'a\\b', 'ünïcode']) {
      expect(validateTikTokUsername(u).valid, u).toBe(false);
    }
  });

  it('rejects non-string input', () => {
    // @ts-expect-error exercising the runtime type guard
    expect(validateTikTokUsername(undefined).valid).toBe(false);
  });

  it('derives its length bounds from the exported constants', () => {
    expect(TIKTOK_USERNAME_MIN_LENGTH).toBe(2);
    expect(TIKTOK_USERNAME_MAX_LENGTH).toBe(24);
    expect(TIKTOK_USERNAME_PATTERN.test('a'.repeat(TIKTOK_USERNAME_MIN_LENGTH))).toBe(true);
    expect(TIKTOK_USERNAME_PATTERN.test('a'.repeat(TIKTOK_USERNAME_MIN_LENGTH - 1))).toBe(false);
    expect(TIKTOK_USERNAME_PATTERN.test('a'.repeat(TIKTOK_USERNAME_MAX_LENGTH))).toBe(true);
    expect(TIKTOK_USERNAME_PATTERN.test('a'.repeat(TIKTOK_USERNAME_MAX_LENGTH + 1))).toBe(false);
  });

  it('assertValidTikTokUsername throws only on invalid input', () => {
    expect(() => assertValidTikTokUsername('a/b')).toThrow(/invalid TikTok username/);
    expect(() => assertValidTikTokUsername('ok_user')).not.toThrow();
  });
});
