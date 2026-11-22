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

import {
  MESSAGE_ANIMATIONS,
  MESSAGE_ANIMATION_CLASS,
  isMessageAnimation,
} from '../visual-settings'

describe('isMessageAnimation', () => {
  it("accepts 'none', the off switch for the entry animation", () => {
    expect(isMessageAnimation('none')).toBe(true)
  })

  it('accepts every listed preset', () => {
    for (const animation of MESSAGE_ANIMATIONS) {
      expect(isMessageAnimation(animation)).toBe(true)
    }
  })

  it("rejects '' so an unset dropdown still means the default fade + slide", () => {
    expect(isMessageAnimation('')).toBe(false)
    expect(isMessageAnimation(undefined)).toBe(false)
    expect(isMessageAnimation('no-such-animation')).toBe(false)
  })
})

describe('MESSAGE_ANIMATION_CLASS', () => {
  it("maps 'none' to the class that cancels the animation", () => {
    expect(MESSAGE_ANIMATION_CLASS.none).toBe('msg-anim-none')
  })

  it('maps every animation to a statically greppable class name', () => {
    for (const animation of MESSAGE_ANIMATIONS) {
      expect(MESSAGE_ANIMATION_CLASS[animation]).toBe(`msg-anim-${animation}`)
    }
  })
})
