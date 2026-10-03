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
import { emote_dataTag, serializeEmoteData } from './emotes.js';

const WAVE_URL = 'https://tt.emote/i/wave.png';
const LAUGHCRY_URL = 'https://tt.emote/i/laughcry.png';

describe('serializeEmoteData', () => {
  it('serializes code, url and id for a single emote', () => {
    // "[laughcry]" starts at index 4 of "hey [laughcry]!".
    const tag = serializeEmoteData('hey [laughcry]!', [
      { index: 4, emote: { emoteId: '7123', image: { urlList: [LAUGHCRY_URL] } } },
    ]);

    // Exact string: this tag is the wire contract with the message-processor
    // TikTok normalizer (same shape as the YouTube listener's emote_data).
    expect(tag).toBe(
      `[{"code":"[laughcry]","url":"${LAUGHCRY_URL}","id":"7123"}]`
    );
  });

  it('serializes every emote of a multi-emote message in payload order', () => {
    // "go [wave] now [laughcry]" — "[wave]" at 3, "[laughcry]" at 14.
    const tag = serializeEmoteData('go [wave] now [laughcry]', [
      { index: 3, emote: { emoteId: '7111', image: { urlList: [WAVE_URL] } } },
      { index: 14, emote: { emoteId: '7222', image: { urlList: [LAUGHCRY_URL] } } },
    ]);

    expect(JSON.parse(tag ?? '[]')).toEqual([
      { code: '[wave]', url: WAVE_URL, id: '7111' },
      { code: '[laughcry]', url: LAUGHCRY_URL, id: '7222' },
    ]);
  });

  it('recovers the token when the emote ends the message', () => {
    // "nice [wave]" — "[wave]" at 5, nothing after the closing bracket.
    const tag = serializeEmoteData('nice [wave]', [
      { index: 5, emote: { emoteId: '7111', image: { urlList: [WAVE_URL] } } },
    ]);

    expect(JSON.parse(tag ?? '[]')).toEqual([
      { code: '[wave]', url: WAVE_URL, id: '7111' },
    ]);
  });

  it('recovers the token whatever unit the emote index counts', () => {
    // "héllo 😀 [wave]" — "[wave]" starts at code point 8, UTF-16 code unit 9,
    // and UTF-8 byte 12. TikTok's emote start offset does not document its
    // unit, so all three interpretations must yield the same token.
    for (const index of [8, 9, 12]) {
      const tag = serializeEmoteData('héllo 😀 [wave]', [
        { index, emote: { emoteId: '7111', image: { urlList: [WAVE_URL] } } },
      ]);

      expect(JSON.parse(tag ?? '[]'), `index ${index}`).toEqual([
        { code: '[wave]', url: WAVE_URL, id: '7111' },
      ]);
    }
  });

  it('accepts an emote at index 0', () => {
    // A leading emote is indistinguishable from a missing index in the wire
    // proto (0 is the default), so 0 must be treated as a real position.
    const tag = serializeEmoteData('[wave] hi', [
      { index: 0, emote: { emoteId: '7111', image: { urlList: [WAVE_URL] } } },
    ]);

    expect(JSON.parse(tag ?? '[]')).toEqual([
      { code: '[wave]', url: WAVE_URL, id: '7111' },
    ]);
  });

  it('skips empty strings when picking the image url', () => {
    const tag = serializeEmoteData('hey [wave]!', [
      { index: 4, emote: { emoteId: '7111', image: { urlList: ['', WAVE_URL] } } },
    ]);

    expect(JSON.parse(tag ?? '[]')).toEqual([
      { code: '[wave]', url: WAVE_URL, id: '7111' },
    ]);
  });

  it('returns undefined when the message carries no emotes', () => {
    expect(serializeEmoteData('hello world', [])).toBeUndefined();
    expect(serializeEmoteData('hello world', undefined)).toBeUndefined();
  });

  it('drops an emote whose token cannot be recovered', () => {
    // Index not at a bracket.
    expect(
      serializeEmoteData('hey [laughcry]', [
        { index: 0, emote: { emoteId: '7123', image: { urlList: [LAUGHCRY_URL] } } },
      ])
    ).toBeUndefined();
    // Unterminated token.
    expect(
      serializeEmoteData('hey [laugh', [
        { index: 4, emote: { emoteId: '7123', image: { urlList: [LAUGHCRY_URL] } } },
      ])
    ).toBeUndefined();
    // Empty token.
    expect(
      serializeEmoteData('hey []', [
        { index: 4, emote: { emoteId: '7123', image: { urlList: [LAUGHCRY_URL] } } },
      ])
    ).toBeUndefined();
  });

  it('drops an emote when the unit interpretations disagree on the token', () => {
    // "日本語 [x] あ [y]": "[x]" starts at UTF-8 byte 10, "[y]" at code
    // point/UTF-16 index 10. An index of 10 therefore names two different
    // tokens depending on the unit, and no span can be trusted — the entry
    // is dropped rather than emitting a wrong token.
    expect(
      serializeEmoteData('日本語 [x] あ [y]', [
        { index: 10, emote: { emoteId: '7111', image: { urlList: [WAVE_URL] } } },
      ])
    ).toBeUndefined();
  });

  it('drops an emote without a usable image url or emote record', () => {
    expect(
      serializeEmoteData('hey [wave]!', [
        { index: 4, emote: { emoteId: '7111', image: { urlList: [] } } },
      ])
    ).toBeUndefined();
    expect(
      serializeEmoteData('hey [wave]!', [{ index: 4, emote: undefined }])
    ).toBeUndefined();
  });
});

describe('emote_dataTag', () => {
  it('lands the emote_data tag on the message tags from the payload emotes', () => {
    // Assembled the way handleChatMessage builds a raw message's tags.
    const tags: Record<string, string> = {
      overlay_id: 'ov1',
      user_unique_id: 'user1',
      ...emote_dataTag('hey [laughcry]!', [
        { index: 4, emote: { emoteId: '7123', image: { urlList: [LAUGHCRY_URL] } } },
      ]),
    };

    // Exact string: this tag is the wire contract with the message-processor
    // TikTok normalizer (same shape as the YouTube listener's emote_data).
    expect(tags.emote_data).toBe(
      `[{"code":"[laughcry]","url":"${LAUGHCRY_URL}","id":"7123"}]`
    );
  });

  it('omits the emote_data tag when the message carries no emotes', () => {
    const tags: Record<string, string> = {
      overlay_id: 'ov1',
      ...emote_dataTag('hello world', []),
    };

    expect(tags).toEqual({ overlay_id: 'ov1' });
  });
});