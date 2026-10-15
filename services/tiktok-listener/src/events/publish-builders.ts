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

/**
 * RawChatMessage builders for the TikTok audience events the pipeline consumes
 * as contract event types: subscription, super-fan join, super-fan box and
 * Q&A question. Pure by design — the handlers in index.ts supply the decoded
 * payload pieces and the Redis publish stays behind an injected callback — so
 * the field mapping, not the I/O, is what the unit tests pin.
 */

import { pickAvatarUrl } from '../avatar.js';

/** The fields of the service's RawChatMessage these events fill in. */
export interface TikTokEventRawMessage {
  message_id: string;
  platform: string;
  channel_id: string;
  user_id: string;
  username: string;
  text: string;
  timestamp: string;
  tags: Record<string, string>;
  event_type: string;
  event_data: Record<string, unknown>;
}

/** Ticks the handler captured before dispatch, so every branch of one frame stamps the same time. */
export interface HandlerClock {
  fallbackNowIso: string;
}

/**
 * Parses TikTok's int64-as-string month count, or 0 when it does not parse —
 * 0 is the contract's "unknown", and never undefined, so the normalizer always
 * gets a number.
 */
export function parseSubMonth(monthText: string | undefined): number {
  if (!monthText || !/^\d+$/.test(monthText)) {
    return 0;
  }
  // int64 wire field; far above any real month count, but parseInt is lossy up
  // there, so clamp rather than trust it.
  const parsed = Number.parseInt(monthText, 10);
  return Number.isSafeInteger(parsed) ? parsed : 0;
}

/** WebcastSubNotifyMessage decoded fields. Only the subset the mapping reads. */
export interface SubNotifyPayloadPieces extends HandlerClock {
  msgId?: string;
  createTime?: string;
  monthText?: string;
  isSend?: boolean;
  giftSource?: number;
  userAvatarUrl?: string;
}

/**
 * tiktok_subscription: cumulative months, plus gift metadata only when the
 * payload actually flags a gifted sub: `isSend` + giftSource=PLATFORM is the
 * gifted-by-platform pair TikTok sets on community-gifted subs; a bought sub
 * carries no unambiguous marker, so it stays a plain sub. The envelope carries
 * no sender identity for community gifts, and the message itself has no
 * gifter user, so no gifter field can be honestly filled.
 */
export function buildSubscriptionEvent(
  channelId: string,
  overlayId: string,
  userId: string,
  nickname: string,
  pieces: SubNotifyPayloadPieces
): TikTokEventRawMessage {
  const isGiftedPlatformSub = pieces.isSend === true && pieces.giftSource === 1;
  return {
    message_id: pieces.msgId || '',
    platform: 'tiktok',
    channel_id: channelId,
    user_id: userId,
    username: nickname,
    text: '',
    timestamp: timestampFromCreateTime(pieces.createTime, pieces),
    tags: {
      overlay_id: overlayId,
      user_unique_id: userId,
      profile_picture_url: pieces.userAvatarUrl ?? ''
    },
    event_type: 'tiktok_subscription',
    event_data: {
      sub_month: parseSubMonth(pieces.monthText),
      ...(isGiftedPlatformSub ? { is_gift: true, gift_source: 'platform' } : {})
    }
  };
}

/**
 * Converts a TikTok Unix-milliseconds createTime string to ISO 8601, falling
 * back to the handler's clock when absent or unparseable.
 */
export function timestampFromCreateTime(createTime: string | undefined, clock: HandlerClock): string {
  if (!createTime) return clock.fallbackNowIso;
  const ms = Number.parseInt(createTime, 10);
  if (!Number.isFinite(ms)) return clock.fallbackNowIso;
  return new Date(ms).toISOString();
}

/** super_fan_join carries identity only; the connector's barrage has no monetary payload. */
export interface SuperFanJoinPayloadPieces extends HandlerClock {
  msgId?: string;
  createTime?: string;
  avatarUrl?: string;
}

/**
 * super_fan_join: nothing but identity — the connector's barrage gives the
 * user but no meaningful monetary payload.
 */
export function buildSuperFanJoinEvent(
  channelId: string,
  overlayId: string,
  userId: string,
  nickname: string,
  pieces: SuperFanJoinPayloadPieces
): TikTokEventRawMessage {
  return {
    message_id: pieces.msgId || '',
    platform: 'tiktok',
    channel_id: channelId,
    user_id: userId,
    username: nickname,
    text: '',
    timestamp: timestampFromCreateTime(pieces.createTime, pieces),
    tags: {
      overlay_id: overlayId,
      user_unique_id: userId,
      profile_picture_url: pieces.avatarUrl ?? ''
    },
    event_type: 'super_fan_join',
    event_data: {}
  };
}

/** Super Fan Box decoded fields, mirroring the coin chest's envelopeInfo. */
export interface SuperFanBoxPayloadPieces extends HandlerClock {
  msgId?: string;
  createTime?: string;
  coins?: number;
  canOpen?: number;
  senderId?: string;
  senderName?: string;
  senderAvatar?: { urlList?: string[] };
}

/**
 * super_fan_box, same shape as treasure_chest: the credit roll treats the two
 * as one category, and the frontend styles them off the same event card path.
 */
export function buildSuperFanBoxEvent(
  channelId: string,
  overlayId: string,
  pieces: SuperFanBoxPayloadPieces
): TikTokEventRawMessage {
  const senderId = pieces.senderId || 'unknown';
  const coins = pieces.coins ?? 0;
  return {
    message_id: pieces.msgId || '',
    platform: 'tiktok',
    channel_id: channelId,
    user_id: senderId,
    username: pieces.senderName || 'Anonymous',
    text: 'Sent a Super Fan chest',
    timestamp: timestampFromCreateTime(pieces.createTime, pieces),
    tags: {
      overlay_id: overlayId,
      user_unique_id: '',
      profile_picture_url: pickAvatarUrl(pieces.senderAvatar)
    },
    event_type: 'super_fan_box',
    event_data: {
      coins,
      can_open: pieces.canOpen ?? 0
    }
  };
}

/** Q&A question decoded fields. `data.user` carries the asker; it can be absent. */
export interface QuestionPayloadPieces extends HandlerClock {
  msgId?: string;
  createTime?: string;
  questionId?: string;
  questionText?: string;
  askerHandle?: string;
  askerName?: string;
  askerAvatarUrl?: string;
}

/**
 * question. The question text is the `text`, so the overlay card renders the
 * asked sentence the way it renders a chat bubble; the asker shows as the user.
 */
export function buildQuestionEvent(
  channelId: string,
  overlayId: string,
  pieces: QuestionPayloadPieces
): TikTokEventRawMessage {
  return {
    message_id: pieces.msgId || '',
    platform: 'tiktok',
    channel_id: channelId,
    user_id: pieces.askerHandle || '',
    username: pieces.askerName || 'Anonymous',
    text: pieces.questionText || '',
    timestamp: timestampFromCreateTime(pieces.createTime, pieces),
    tags: {
      overlay_id: overlayId,
      user_unique_id: pieces.askerHandle || '',
      profile_picture_url: pieces.askerAvatarUrl ?? ''
    },
    event_type: 'question',
    event_data: {
      question_id: pieces.questionId ?? ''
    }
  };
}
