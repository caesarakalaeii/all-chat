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

import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  hasTikTokChestPayload,
  isTikTokCoinChest,
  isTikTokEnvelopeDrop,
  type TikTokEnvelopeData
} from './envelope.js';

/** A viewer-dropped chest as it arrives once decoded: USER_DIAMOND, display=NEW, 20 coins. */
function chestFrame(overrides: Partial<TikTokEnvelopeData> = {}): TikTokEnvelopeData {
  return {
    common: { msgId: '7300000000000000001', createTime: '1786600000000' },
    display: 1,
    envelopeInfo: {
      envelopeId: 'env-1',
      businessType: 1,
      diamondCount: 20,
      peopleCount: 1,
      sendUserId: '6800000000000000002',
      sendUserName: 'someviewer'
    },
    ...overrides
  };
}

describe('isTikTokCoinChest', () => {
  it('accepts a chest dropped by a viewer', () => {
    expect(isTikTokCoinChest(chestFrame())).toBe(true);
  });

  it('accepts a chest dropped into the room by the platform', () => {
    expect(isTikTokCoinChest(chestFrame({ envelopeInfo: { businessType: 2, diamondCount: 100 } }))).toBe(
      true
    );
  });

  // The connector emits SUPER_FAN_BOX and then falls through to ENVELOPE for the same frame, so
  // without this the streamer would be shown a coin chest for every Super Fan Box join.
  it('rejects a Super Fan Box by business type', () => {
    expect(isTikTokCoinChest(chestFrame({ envelopeInfo: { businessType: 19, diamondCount: 20 } }))).toBe(
      false
    );
  });

  // The connector emits SUPER_FAN_BOX first and then falls through to ENVELOPE for the same
  // frame, so the superfanbox display key rides along on genuine chests too. Ruling the frame out
  // on that key dropped real drops, and the viewer never saw them: businessType decides.
  it('accepts a real chest frame carrying the ttlive_superfanbox display key', () => {
    const frame = chestFrame({
      common: { msgId: 'm1', displayText: { key: 'pm_mt_ttlive_superfanbox_join' } },
      envelopeInfo: { businessType: 1, diamondCount: 20 }
    });
    expect(isTikTokCoinChest(frame)).toBe(true);
  });

  it('accepts a plain chest with no display text at all', () => {
    const frame = chestFrame({ common: { msgId: 'm2' } });
    expect(isTikTokCoinChest(frame)).toBe(true);
  });

  it('rejects a Super Fan Box by display-text key even when the business type is absent', () => {
    const frame = chestFrame({
      common: { displayText: { key: 'pm_mt_ttlive_superfanbox_join' } },
      envelopeInfo: { diamondCount: 20 }
    });
    expect(isTikTokCoinChest(frame)).toBe(false);
  });

  it('matches the display-text key case-insensitively', () => {
    const frame = chestFrame({
      common: { displayText: { key: 'PM_MT_TTLIVE_SUPERFANBOX_JOIN' } },
      envelopeInfo: { diamondCount: 20 }
    });
    expect(isTikTokCoinChest(frame)).toBe(false);
  });

  it.each([
    ['a platform shell', 3],
    ['a portal', 4],
    ['a merch drop', 5],
    ['a fan-club box', 7]
  ])('rejects %s', (_label, businessType) => {
    expect(isTikTokCoinChest(chestFrame({ envelopeInfo: { businessType, diamondCount: 20 } }))).toBe(
      false
    );
  });

  // Deliberately permissive: TikTok omits businessType on some frames, and reading that as
  // "not a chest" would silently disable the whole feature rather than lose one event.
  it('treats an omitted business type as a possible chest', () => {
    expect(isTikTokCoinChest(chestFrame({ envelopeInfo: { diamondCount: 20 } }))).toBe(true);
  });

  // A known non-chest business type is authoritative as well, so the missing display key does not
  // rescue it.
  it('rejects a known non-chest business type carrying no display key', () => {
    expect(isTikTokCoinChest(chestFrame({ envelopeInfo: { businessType: 19, diamondCount: 20 } }))).toBe(
      false
    );
  });
});

describe('TIKTOK_ENVELOPE_TRACE', () => {
  afterEach(() => {
    delete process.env.TIKTOK_ENVELOPE_TRACE;
    vi.restoreAllMocks();
  });

  it('logs nothing per frame while unset', () => {
    const log = vi.spyOn(console, 'log').mockImplementation(() => {});
    isTikTokCoinChest(chestFrame());
    expect(log).not.toHaveBeenCalled();
  });

  it('logs the decision inputs and the result once enabled', () => {
    const log = vi.spyOn(console, 'log').mockImplementation(() => {});
    process.env.TIKTOK_ENVELOPE_TRACE = '1';

    const frame = chestFrame({
      common: { msgId: 'm3', displayText: { key: 'pm_mt_ttlive_superfanbox_join' } },
      envelopeInfo: { businessType: 1, diamondCount: 20 }
    });
    expect(isTikTokCoinChest(frame)).toBe(true);

    expect(log).toHaveBeenCalledTimes(1);
    const entry = JSON.parse(log.mock.calls[0][0] as string);
    expect(entry.business_type).toBe(1);
    expect(entry.display_text_key).toBe('pm_mt_ttlive_superfanbox_join');
    expect(entry.is_coin_chest).toBe(true);
  });
});

describe('isTikTokEnvelopeDrop', () => {
  it('accepts a new drop', () => {
    expect(isTikTokEnvelopeDrop(chestFrame())).toBe(true);
  });

  // A chest that expires or is fully claimed comes back with the whole envelopeInfo under a fresh
  // msgId, which can outlive the dedup TTL — so this flag is what stops one chest rendering twice.
  it('rejects the HIDE frame that takes a spent chest off screen', () => {
    expect(isTikTokEnvelopeDrop(chestFrame({ display: 2 }))).toBe(false);
  });

  it('treats an omitted display as a drop', () => {
    expect(isTikTokEnvelopeDrop(chestFrame({ display: undefined }))).toBe(true);
  });

  it('rejects an unrecognised display value', () => {
    expect(isTikTokEnvelopeDrop(chestFrame({ display: -1 }))).toBe(false);
  });
});

describe('hasTikTokChestPayload', () => {
  it('accepts a frame carrying coins', () => {
    expect(hasTikTokChestPayload(chestFrame())).toBe(true);
  });

  // The empty ENVELOPE frame TikTok also sends passes both predicates above, because each treats
  // its missing field as "probably a chest". Without this guard it published a chest worth 0 coins
  // from Anonymous.
  it('rejects a frame with no envelope info at all', () => {
    expect(hasTikTokChestPayload({ common: { msgId: 'm1' }, display: 1 })).toBe(false);
  });

  it('rejects a frame whose envelope info carries no coin count', () => {
    expect(hasTikTokChestPayload(chestFrame({ envelopeInfo: { envelopeId: 'env-2' } }))).toBe(false);
  });

  it('rejects a frame reporting zero coins', () => {
    expect(hasTikTokChestPayload(chestFrame({ envelopeInfo: { diamondCount: 0 } }))).toBe(false);
  });

  it('narrows the coin count so callers need no non-null assertion', () => {
    const frame = chestFrame();
    if (!hasTikTokChestPayload(frame)) throw new Error('expected a chest frame');
    const coins: number = frame.envelopeInfo.diamondCount;
    expect(coins).toBe(20);
  });
});
