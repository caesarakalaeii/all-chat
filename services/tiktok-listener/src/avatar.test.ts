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
import { pickAvatarUrl, tiktokAvatarUrl } from './avatar.js';

// Representative TikTok avatar url shapes (paths shortened, tokens elided).
const SHRINK = 'https://p16-sign.tiktokcdn-us.com/tos/avt/72x72/user-a.jpeg?shrink=1';
const WEBP_100 = 'https://p16-sign-va.tiktokcdn.com/tos/100x100/avt/user-a.webp?x-expires=1';
const JPEG_100 = 'https://p16-sign-va.tiktokcdn.com/tos/100x100/avt/user-a.jpeg?x-expires=1';
const JPEG_300 = 'https://p16-sign-va.tiktokcdn.com/tos/300x300/avt/user-a.jpeg?x-expires=1';

describe('pickAvatarUrl', () => {
  it('returns empty string for missing image / urlList / empty list', () => {
    expect(pickAvatarUrl(undefined)).toBe('');
    expect(pickAvatarUrl({})).toBe('');
    expect(pickAvatarUrl({ urlList: [] })).toBe('');
  });

  it('prefers a 100x100 webp over other formats', () => {
    expect(pickAvatarUrl({ urlList: [SHRINK, JPEG_300, JPEG_100, WEBP_100] })).toBe(WEBP_100);
  });

  it('prefers a 100x100 jpeg when no 100x100 webp exists', () => {
    expect(pickAvatarUrl({ urlList: [SHRINK, JPEG_300, JPEG_100] })).toBe(JPEG_100);
  });

  it('avoids the shrink placeholder when a real URL exists', () => {
    expect(pickAvatarUrl({ urlList: [SHRINK, JPEG_300] })).toBe(JPEG_300);
  });

  it('returns empty (not the shared placeholder) when every URL is a shrink placeholder', () => {
    // Returning '' lets tiktokAvatarUrl fall through to a larger model / the
    // initial, instead of emitting the account-shared placeholder.
    expect(pickAvatarUrl({ urlList: ['a-shrink-1', 'b-shrink-2'] })).toBe('');
  });

  it('ignores empty / non-string entries', () => {
    // Empty strings must never be chosen (they would suppress the initial fallback).
    expect(pickAvatarUrl({ urlList: ['', JPEG_300] })).toBe(JPEG_300);
    expect(pickAvatarUrl({ urlList: ['', ''] })).toBe('');
  });
});

describe('tiktokAvatarUrl', () => {
  it('returns empty string when the user or all image models are absent', () => {
    expect(tiktokAvatarUrl(undefined)).toBe('');
    expect(tiktokAvatarUrl({})).toBe('');
    expect(tiktokAvatarUrl({ avatarThumb: { urlList: [] } })).toBe('');
  });

  it('reads the thumb image model when present', () => {
    expect(tiktokAvatarUrl({ avatarThumb: { urlList: [WEBP_100] } })).toBe(WEBP_100);
  });

  it('falls through to medium then large when smaller models are empty', () => {
    expect(
      tiktokAvatarUrl({ avatarThumb: { urlList: [] }, avatarMedium: { urlList: [JPEG_100] } })
    ).toBe(JPEG_100);
    expect(
      tiktokAvatarUrl({
        avatarThumb: {},
        avatarMedium: { urlList: [] },
        avatarLarge: { urlList: [JPEG_300] },
      })
    ).toBe(JPEG_300);
  });

  it('falls through when the thumb model carries only shrink placeholders', () => {
    expect(
      tiktokAvatarUrl({
        avatarThumb: { urlList: ['x-shrink-1', 'y-shrink-2'] },
        avatarMedium: { urlList: [JPEG_100] },
      })
    ).toBe(JPEG_100);
  });

  it('gives distinct URLs for distinct users (regression: all-identical avatars)', () => {
    const userA = { avatarThumb: { urlList: [SHRINK, 'https://cdn/tos/100x100/avt/A.webp'] } };
    const userB = { avatarThumb: { urlList: [SHRINK, 'https://cdn/tos/100x100/avt/B.webp'] } };
    expect(tiktokAvatarUrl(userA)).not.toBe(tiktokAvatarUrl(userB));
  });
});
