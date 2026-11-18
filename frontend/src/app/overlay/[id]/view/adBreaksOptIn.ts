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
 * Visibility of the monitor's Twitch ad-break opt-in banner (`.../view`).
 *
 * The banner asks the streamer for a re-consent carrying `channel:read:ads`, so
 * `channel.ad_break.begin` (scheduled or manual breaks) can surface a notice in the
 * Activity & Events feed. Same shape as the mod-log opt-in: showing it to somebody
 * who already consented reads as "your grant did not take", which is why the rule
 * lives in its own module with its own test.
 */

export interface AdBreaksOptInState {
  /** Whether the viewer owns the overlay. The consent is the broadcaster's to give. */
  isOwner: boolean
  /** Whether the overlay carries a Twitch source. Only Twitch sends ad-break events. */
  hasTwitchSource: boolean
  /**
   * `ad_breaks_granted` from the capabilities payload. `undefined` when the backend sent
   * no flag — an older service, or one that could not read the credential.
   */
  adBreaksGranted: boolean | undefined
}

/**
 * Whether to show the ad-break opt-in banner.
 *
 * Only an explicit `true` hides it: anything else — `false`, or a payload with no flag
 * at all — is either "not granted" or "could not tell", and both have to leave the CTA
 * on screen (same reasoning as `shouldOfferModLogOptIn`).
 */
export function shouldOfferAdBreaksOptIn({
  isOwner,
  hasTwitchSource,
  adBreaksGranted,
}: AdBreaksOptInState): boolean {
  return isOwner && hasTwitchSource && adBreaksGranted !== true
}
