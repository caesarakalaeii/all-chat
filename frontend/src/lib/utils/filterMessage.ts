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

import type { ChatMessage } from '../types/message'
import type { FilterSettings } from '../types/overlay'

/**
 * shouldFilterMessage returns true if the given message should be hidden from
 * the overlay based on the provided FilterSettings.
 *
 * All filtering is client-side, applied in the WebSocket onmessage handler
 * before the message is added to the render queue.
 *
 * Rules (all short-circuit on first match):
 *   D-04: exact case-insensitive username/display_name match in banned_users
 *   D-03: regex keyword match (case-insensitive) against message text
 *   D-05: hide bot commands — messages starting with "!"
 *   D-06: minimum message length (0 = disabled)
 */
export function shouldFilterMessage(
  message: ChatMessage,
  settings: FilterSettings | null | undefined
): boolean {
  if (!settings) return false

  const username = message.user?.username?.toLowerCase() ?? ''
  const displayName = message.user?.display_name?.toLowerCase() ?? ''
  const text = message.message?.text ?? ''

  // D-04: exact case-insensitive username match
  if (settings.banned_users?.some(u => {
    const lower = u.toLowerCase()
    return lower === username || lower === displayName
  })) {
    return true
  }

  // D-03: regex keyword match (literal strings are valid regex)
  if (settings.banned_words?.length) {
    for (const pattern of settings.banned_words) {
      try {
        if (new RegExp(pattern, 'i').test(text)) return true
      } catch {
        // invalid regex — skip silently
      }
    }
  }

  // D-05: hide bot commands starting with !
  if (settings.hide_commands && text.startsWith('!')) return true

  // D-06: minimum message length (0 = disabled)
  if (settings.min_message_length && settings.min_message_length > 0) {
    if (text.length < settings.min_message_length) return true
  }

  return false
}
