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
 * Visibility of the monitor's Twitch moderation-log opt-in banner
 * (`.../view`).
 *
 * The banner asks the streamer for a nine-scope re-consent, one of which is
 * `moderator:manage:automod` on a read-only feature — copy exists on that
 * banner purely to stop the consent screen looking like a mistake. Showing it
 * to somebody who already consented therefore reads as "your grant did not
 * take" and invites them to run that consent again for nothing, which is why
 * the rule lives in its own module with its own test.
 */

export interface ModLogOptInState {
  /** Whether the viewer owns the overlay. The consent is the broadcaster's to give. */
  isOwner: boolean
  /** Whether the overlay carries a Twitch source. Only Twitch produces mod-log events. */
  hasTwitchSource: boolean
  /**
   * `mod_log_granted` from the capabilities payload. `undefined` when the backend sent
   * no flag — an older service, or one that could not read the credential.
   */
  modLogGranted: boolean | undefined
}

/**
 * Whether to show the mod-log opt-in banner.
 *
 * Only an explicit `true` hides it: anything else — `false`, or a payload with no flag
 * at all — is either "not granted" or "could not tell", and both have to leave the CTA
 * on screen. Hiding it on a cannot-tell would leave the streamer no route to the mod log
 * and no explanation of why.
 */
export function shouldOfferModLogOptIn({
  isOwner,
  hasTwitchSource,
  modLogGranted,
}: ModLogOptInState): boolean {
  return isOwner && hasTwitchSource && modLogGranted !== true
}
