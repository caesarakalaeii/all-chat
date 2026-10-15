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
 * Classification of TikTok's ENVELOPE message down to the Super Fan Box, the
 * coin chest's sibling product on the same multiplexed message.
 */

import type { AvatarImageModel } from '../avatar.js';
import { TIKTOK_ENVELOPE_BUSINESS_TYPE_UNKNOWN, type TikTokEnvelopeData } from './envelope.js';

// `EnvelopeBusinessType.SUPER_FAN_BOX` and the superfanbox display-text key,
// the two ways the same box identifies itself (the connector's dispatcher
// accepts either). See isTikTokCoinChest for why both probes are needed: one
// of the two can be absent on a decodable frame.
export const TIKTOK_SUPER_FAN_BOX_BUSINESS_TYPE = 19;
const TIKTOK_SUPER_FAN_BOX_DISPLAY_KEY_MARKER = 'ttlive_superfanbox';

/**
 * Tells the Super Fan Box apart from the other envelope products.
 *
 * The connector emits `superFanBox` AND falls through to `envelope` for the
 * *same* frame, so the caller must consult one shared predicate rather than
 * trusting which event arrived, or one chest renders twice under two names.
 *
 * UNKNOWN mirrors isTikTokCoinChest: an omitted businessType decodes to 0,
 * and treating UNKNOWN as "not a box" would silently emit nothing whenever
 * TikTok drops the field — there the display-text probe decides instead.
 */
export function isTikTokSuperFanBox(data: TikTokEnvelopeData): boolean {
  const businessType = data.envelopeInfo?.businessType ?? TIKTOK_ENVELOPE_BUSINESS_TYPE_UNKNOWN;
  if (businessType !== TIKTOK_ENVELOPE_BUSINESS_TYPE_UNKNOWN) {
    return businessType === TIKTOK_SUPER_FAN_BOX_BUSINESS_TYPE;
  }
  const displayKey = data.common?.displayText?.key;
  return displayKey?.toLowerCase().includes(TIKTOK_SUPER_FAN_BOX_DISPLAY_KEY_MARKER) ?? false;
}

/**
 * A frame that may carry a Super Fan Box. The envelopeInfo is optional at the
 * type level because TikTok omits it on some frames; a caller publishing one
 * of those would render a phantom box from Anonymous, so callers check at publish
 * time just as the coin chest does with hasTikTokChestPayload.
 */
export type TikTokSuperFanBoxFrame = TikTokEnvelopeData & {
  envelopeInfo: NonNullable<TikTokEnvelopeData['envelopeInfo']>;
};
