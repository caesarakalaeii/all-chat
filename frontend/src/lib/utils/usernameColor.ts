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
 * User info subset needed to resolve a username color.
 */
export interface UsernameColorUser {
  /**
   * The authoritative color, resolved server-side: the viewer's manually chosen
   * All-Chat color, else the platform-native color (Twitch/Kick/Discord).
   * Absent when the chatter has neither.
   */
  color?: string
  /**
   * Deterministic palette fallback computed by message-processor, stable per
   * viewer (and shared across their linked platforms).
   */
  auto_color?: string
}

/**
 * Resolves the CSS color for a username, implementing the ADR-0047 priority chain:
 *
 *   1. the viewer's manual All-Chat color   (server-resolved into `color`)
 *   2. the platform-native color            (server-resolved into `color`)
 *   3. the streamer's "Username color" overlay setting
 *   4. the deterministic auto palette
 *   5. white
 *
 * Steps 1 and 2 are already collapsed into `color` by the message-processor, so
 * a non-empty `color` short-circuits. Steps 3-5 are expressed with CSS custom
 * property fallback syntax rather than resolved here: `--chat-username-color` is
 * only emitted when the streamer actually picks one (see visual-settings-to-css),
 * so an unset setting falls through to the auto color at paint time.
 *
 * Note the ordering constraint this encodes: the auto color must NOT be folded
 * into `color` upstream, or the streamer's setting could never apply.
 */
export function resolveUsernameColor(user: UsernameColorUser | undefined): string {
  if (user?.color) {
    return user.color
  }
  return `var(--chat-username-color, ${user?.auto_color || '#FFFFFF'})`
}
