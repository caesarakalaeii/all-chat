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
  type TikTokEnvelopeData
} from './envelope.js';
import { isTikTokSuperFanBox } from './superfan.js';
import {
  buildQuestionEvent,
  buildSubscriptionEvent,
  buildSuperFanBoxEvent,
  parseSubMonth
} from './publish-builders.js';

const CLOCK = { fallbackNowIso: '2026-10-15T00:00:00.000Z' };

describe('isTikTokSuperFanBox', () => {
  // A box frame. The connector emits superFanBox AND falls through to envelope
  // for the same frame, so both registrations must agree on this predicate.
  const boxFrame = (overrides: Partial<TikTokEnvelopeData> = {}): TikTokEnvelopeData => ({
    common: { msgId: 'm1', createTime: '1786600000000' },
    display: 1,
    envelopeInfo: {
      envelopeId: 'env-box-1',
      businessType: 19,
      peopleCount: 1,
      sendUserId: '6800000000000000002',
      sendUserName: 'someviewer'
    },
    ...overrides
  });

  it('accepts a Super Fan Box by business type', () => {
    expect(isTikTokSuperFanBox(boxFrame())).toBe(true);
  });

  it('accepts a Super Fan Box riding only the display-text key', () => {
    expect(
      isTikTokSuperFanBox(
        boxFrame({ common: { msgId: 'm1', displayText: { key: 'pm_mt_ttlive_superfanbox_join' } } })
      )
    ).toBe(true);
  });

  it('rejects a coin chest frame', () => {
    expect(
      isTikTokSuperFanBox({ ...boxFrame(), envelopeInfo: { businessType: 1, diamondCount: 20 } })
    ).toBe(false);
  });

  it('rejects a frame with neither marker', () => {
    expect(isTikTokSuperFanBox({ common: { msgId: 'm2' } })).toBe(false);
  });

  it('still excludes the box when the coin chest classifier sees it', () => {
    const frame = boxFrame();
    expect(isTikTokSuperFanBox(frame)).toBe(true);
    expect(isTikTokCoinChest(frame)).toBe(false);
  });

  it('coin chest classifier still accepts a chest with no superfanbox marker', () => {
    expect(
      hasTikTokChestPayload({ envelopeInfo: { businessType: 1, diamondCount: 20 } })
    ).toBe(true);
  });
});

describe('parseSubMonth', () => {
  it('parses an int64 month string', () => {
    expect(parseSubMonth('3')).toBe(3);
    expect(parseSubMonth('12')).toBe(12);
  });

  it('defaults to 0 for absent, empty or non-numeric values', () => {
    expect(parseSubMonth(undefined)).toBe(0);
    expect(parseSubMonth('')).toBe(0);
    expect(parseSubMonth('abc')).toBe(0);
    expect(parseSubMonth('1a')).toBe(0);
  });
});

describe('buildSubscriptionEvent', () => {
  it('carries cumulative months and no gift fields for a bought sub', () => {
    const event = buildSubscriptionEvent('streamer', 'overlay-1', 'handle', 'Viewer', {
      ...CLOCK,
      msgId: 'm',
      createTime: '1786600000000',
      monthText: '5',
      isSend: false,
      giftSource: 0
    });
    expect(event.event_type).toBe('tiktok_subscription');
    expect(event.event_data).toEqual({ sub_month: 5 });
    expect(event.text).toBe('');
    expect(event.username).toBe('Viewer');
    expect(event.timestamp).toBe(new Date(1786600000000).toISOString());
  });

  it('marks the community-gifted pair (isSend + giftSource PLATFORM) as a gift', () => {
    const event = buildSubscriptionEvent('streamer', 'overlay-1', 'handle', 'Viewer', {
      ...CLOCK,
      msgId: 'm',
      monthText: '1',
      isSend: true,
      giftSource: 1
    });
    expect(event.event_data).toEqual({ sub_month: 1, is_gift: true, gift_source: 'platform' });
  });

  it('does not mark isSend alone as a gift without the PLATFORM gift source', () => {
    const event = buildSubscriptionEvent('streamer', 'overlay-1', 'handle', 'Viewer', {
      ...CLOCK,
      msgId: 'm',
      monthText: '1',
      isSend: true,
      giftSource: 2
    });
    expect(event.event_data).toEqual({ sub_month: 1 });
  });

  it('never reports an unknown month as undefined', () => {
    const event = buildSubscriptionEvent('streamer', 'overlay-1', 'h', 'V', {
      ...CLOCK,
      msgId: 'm',
      monthText: undefined
    });
    expect(event.event_data.sub_month).toBe(0);
  });
});

describe('buildSuperFanBoxEvent', () => {
  it('matches the treasure chest shape with its own text and no gifter fields', () => {
    const event = buildSuperFanBoxEvent('streamer', 'overlay-1', {
      ...CLOCK,
      msgId: 'm',
      createTime: '1786600000000',
      coins: 30,
      canOpen: 2,
      senderId: '6800',
      senderName: 'someviewer',
      senderAvatar: { urlList: ['https://img/100x100.webp'] }
    });
    expect(event.event_type).toBe('super_fan_box');
    expect(event.text).toBe('Sent a Super Fan chest');
    expect(event.user_id).toBe('6800');
    expect(event.username).toBe('someviewer');
    expect(event.event_data).toEqual({ coins: 30, can_open: 2 });
    expect(event.tags.profile_picture_url).toBe('https://img/100x100.webp');
  });

  it('publishes as Anonymous with zero coins when the sender fields and coins are absent', () => {
    const event = buildSuperFanBoxEvent('streamer', 'overlay-1', { ...CLOCK });
    expect(event.username).toBe('Anonymous');
    expect(event.user_id).toBe('unknown');
    expect(event.event_data).toEqual({ coins: 0, can_open: 0 });
  });
});

describe('buildQuestionEvent', () => {
  it('puts the question text in text and the asker in the user fields', () => {
    const event = buildQuestionEvent('streamer', 'overlay-1', {
      ...CLOCK,
      msgId: 'm',
      createTime: '1786600000000',
      questionId: 'q-1',
      questionText: 'Favorite boss so far?',
      askerHandle: 'asker',
      askerName: 'Asker Display',
      askerAvatarUrl: 'https://img/a.webp'
    });
    expect(event.event_type).toBe('question');
    expect(event.text).toBe('Favorite boss so far?');
    expect(event.user_id).toBe('asker');
    expect(event.event_data).toEqual({ question_id: 'q-1' });
  });

  it('still builds with a missing asker rather than throwing', () => {
    const event = buildQuestionEvent('streamer', 'overlay-1', {
      ...CLOCK,
      msgId: 'm',
      questionId: 'q-1',
      questionText: 'hi'
    });
    expect(event.user_id).toBe('');
    expect(event.username).toBe('Anonymous');
  });

  it('falls back to the handler clock when the question block has no createTime', () => {
    const event = buildQuestionEvent('s', 'o', { ...CLOCK, questionId: 'q', questionText: 'x' });
    expect(event.timestamp).toBe(CLOCK.fallbackNowIso);
  });
});

describe('superfan.ts test env', () => {
  it('keeps the envelope trace quiet by default', () => {
    const log = vi.spyOn(console, 'log').mockImplementation(() => {});
    isTikTokSuperFanBox({ envelopeInfo: { businessType: 19 } });
    expect(log).not.toHaveBeenCalled();
    afterEach(() => {
      vi.restoreAllMocks();
    });
  });
});
