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

import { describe, it, expect } from 'vitest'
import { resolveUsernameColor } from '../usernameColor'

// ADR-0047 priority chain:
//   viewer's manual All-Chat color > platform-native color
//     > streamer's "Username color" overlay setting > deterministic auto palette
//
// The middle two steps are expressed with CSS var fallback syntax: the streamer's
// setting only emits --chat-username-color when they actually set it, so an unset
// setting falls through to the auto color at paint time.

describe('resolveUsernameColor', () => {
  it("uses the chatter's authoritative color verbatim when present", () => {
    // Twitch-set color (#DAA520 = GoldenRod, a real Twitch palette color)
    expect(resolveUsernameColor({ color: '#DAA520', auto_color: '#5B8DEF' })).toBe('#DAA520')
  })

  it('honors the platform color over the streamer setting and the auto palette', () => {
    const result = resolveUsernameColor({ color: '#FF4500', auto_color: '#1ABC9C' })
    expect(result).not.toContain('--chat-username-color')
    expect(result).not.toContain('#1ABC9C')
  })

  it('falls through to the streamer setting, then auto color, when no platform color', () => {
    // No authoritative color: the streamer's setting wins if set, else auto color.
    expect(resolveUsernameColor({ auto_color: '#5B8DEF' })).toBe(
      'var(--chat-username-color, #5B8DEF)'
    )
  })

  it('treats an empty-string color as absent', () => {
    // Go omitempty drops the field, but defensive: "" must not paint transparent.
    expect(resolveUsernameColor({ color: '', auto_color: '#9B7BFF' })).toBe(
      'var(--chat-username-color, #9B7BFF)'
    )
  })

  it('falls back to white when neither a color nor an auto color is present', () => {
    expect(resolveUsernameColor({})).toBe('var(--chat-username-color, #FFFFFF)')
  })

  it('tolerates an undefined user', () => {
    expect(resolveUsernameColor(undefined)).toBe('var(--chat-username-color, #FFFFFF)')
  })
})
