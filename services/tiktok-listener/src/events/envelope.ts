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
 * Classification of TikTok's ENVELOPE message down to the coin chest (the "treasure box").
 *
 * Lives outside index.ts because that module starts the service on import and so cannot be
 * pulled into a test. These predicates decide whether a streamer sees a chest at all, and every
 * branch of the handler is a silent return, so they are exactly the logic that needs tests.
 */

import type { AvatarImageModel } from '../avatar.js';

// Localized display string attached to a message. `key` is a stable template id
// (e.g. `pm_mt_ttlive_superfanbox_join`) — the connector itself probes it to tell
// the envelope products apart; see isTikTokCoinChest.
export interface TikTokDisplayText {
  key?: string;
}

export interface TikTokEnvelopeCommon {
  msgId?: string;
  createTime?: string;
  displayText?: TikTokDisplayText;
}

// The chest ("red envelope") a viewer drops into the stream. Note the sender is
// inlined here as flat `sendUser*` fields rather than a nested `user`, and the
// avatar is a single image model rather than the thumb/medium/large trio.
export interface TikTokEnvelopeInfo {
  envelopeId?: string; // stable per-chest id, identical on every frame for that chest
  businessType?: number; // which envelope product this is; see isTikTokCoinChest
  diamondCount?: number; // coins in the chest
  peopleCount?: number; // how many viewers may claim it
  sendUserId?: string; // numeric sender id
  sendUserName?: string; // sender display name (this payload carries no @handle)
  sendUserAvatar?: AvatarImageModel;
}

export interface TikTokEnvelopeData {
  common?: TikTokEnvelopeCommon;
  envelopeInfo?: TikTokEnvelopeInfo;
  display?: number; // `EnvelopeDisplay`: add vs. remove the chest; see isTikTokEnvelopeDrop
}

// `EnvelopeBusinessType` values. TikTok multiplexes several products onto the
// ENVELOPE message and the connector forwards every one of them — its dispatcher
// emits SUPER_FAN_BOX and then falls through to ENVELOPE for the *same* frame —
// so the coin chest has to be picked out by business type. Only the two
// diamond-bearing variants are chests; the rest (portals, merch drops, shells,
// fan-club boxes, Super Fan Box) must not surface as one.
export const TIKTOK_ENVELOPE_BUSINESS_TYPE_UNKNOWN = 0;
export const TIKTOK_ENVELOPE_COIN_BUSINESS_TYPES = new Set<number>([
  1, // BUSINESS_TYPE_USER_DIAMOND — a viewer drops a chest
  2 // BUSINESS_TYPE_PLATFORM_DIAMOND — TikTok drops one into the room
]);

const TIKTOK_SUPER_FAN_BOX_DISPLAY_KEY_MARKER = 'ttlive_superfanbox';

/**
 * Emits one line per classified frame when TIKTOK_ENVELOPE_TRACE is set, so a
 * "my chest never appeared" report can be answered from the logs instead of a
 * redeploy-and-guess loop. Read from the environment on every call rather than
 * captured at import, so enabling it needs no rebuild of the module graph — and
 * so it stays completely silent, per frame, while unset (the default).
 */
function traceCoinChestDecision(
  businessType: number,
  displayKey: string | undefined,
  result: boolean
): void {
  if (!process.env.TIKTOK_ENVELOPE_TRACE) {
    return;
  }

  console.log(
    JSON.stringify({
      timestamp: new Date().toISOString(),
      level: 'debug',
      service: 'tiktok-listener',
      message: 'Classified ENVELOPE frame',
      business_type: businessType,
      display_text_key: displayKey ?? null,
      is_coin_chest: result
    })
  );
}

/**
 * Tells a coin chest apart from the other envelope products.
 *
 * `businessType` is the authoritative signal and it decides on its own whenever
 * TikTok sends it: a diamond-bearing type is a chest even if the frame also
 * carries a Super Fan Box display key, because the connector's dispatcher emits
 * SUPER_FAN_BOX and then falls through to ENVELOPE for the *same* frame, so that
 * key rides along on genuine chests. Only when businessType is absent — it
 * decodes to UNKNOWN then, and treating UNKNOWN as "not a chest" would silently
 * emit nothing for the whole feature — does the display-text probe the connector
 * uses to spot a Super Fan Box get to rule the frame out.
 */
export function isTikTokCoinChest(data: TikTokEnvelopeData): boolean {
  const displayKey = data.common?.displayText?.key;
  const businessType = data.envelopeInfo?.businessType ?? TIKTOK_ENVELOPE_BUSINESS_TYPE_UNKNOWN;

  let result: boolean;
  if (businessType !== TIKTOK_ENVELOPE_BUSINESS_TYPE_UNKNOWN) {
    result = TIKTOK_ENVELOPE_COIN_BUSINESS_TYPES.has(businessType);
  } else {
    result = !displayKey?.toLowerCase().includes(TIKTOK_SUPER_FAN_BOX_DISPLAY_KEY_MARKER);
  }

  traceCoinChestDecision(businessType, displayKey, result);
  return result;
}

// `EnvelopeDisplay` values. The ENVELOPE message doubles as a remove
// instruction: once a chest expires or has been claimed by everyone it may hold,
// TikTok re-sends it with display=HIDE so the client takes it off screen. That
// frame repeats the whole envelopeInfo under a fresh msgId, and it can land long
// after the dedup TTL has forgotten the drop, so this flag is what keeps one
// chest from being published twice.
const TIKTOK_ENVELOPE_DISPLAY_UNKNOWN = 0;
const TIKTOK_ENVELOPE_DISPLAY_NEW = 1;

/**
 * Tells a chest being dropped apart from one being taken off screen.
 *
 * As with businessType, `display` decodes to UNKNOWN when TikTok omits it on the
 * wire; treating UNKNOWN as "not a drop" would silently emit nothing for the
 * whole feature, so only an explicit non-NEW display is dropped.
 */
export function isTikTokEnvelopeDrop(data: TikTokEnvelopeData): boolean {
  const display = data.display ?? TIKTOK_ENVELOPE_DISPLAY_UNKNOWN;
  return display === TIKTOK_ENVELOPE_DISPLAY_UNKNOWN || display === TIKTOK_ENVELOPE_DISPLAY_NEW;
}

/** An envelope frame that carries a real chest: sender fields may still be absent, coins are not. */
export interface TikTokChestFrame extends TikTokEnvelopeData {
  envelopeInfo: TikTokEnvelopeInfo & { diamondCount: number };
}

/**
 * Whether the frame actually carries a chest, as opposed to announcing one that is not there.
 *
 * TikTok also sends ENVELOPE frames with no envelopeInfo at all — upstream issue #27 captured a
 * treasure-box payload and an empty one side by side. Because the two predicates above
 * deliberately read a missing businessType/display as "probably a chest" rather than discard a
 * real drop, an empty frame passes both, and would render a phantom "Sent a coin chest" worth 0
 * coins from Anonymous. A real chest always carries its coin count, so that is what separates a
 * drop from an empty announcement.
 */
export function hasTikTokChestPayload(data: TikTokEnvelopeData): data is TikTokChestFrame {
  return !!data.envelopeInfo && (data.envelopeInfo.diamondCount ?? 0) > 0;
}
