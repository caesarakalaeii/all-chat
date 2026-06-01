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
import { isDisplayVisible } from '@/lib/utils/displayVisibility'

describe('isDisplayVisible', () => {
  it('hides when the setting is "none" (the bug: OBS showed timestamps despite none)', () => {
    expect(isDisplayVisible('none')).toBe(false)
  })

  it('shows when the setting is "block"', () => {
    expect(isDisplayVisible('block')).toBe(true)
  })

  it('shows when the setting is "inline"', () => {
    expect(isDisplayVisible('inline')).toBe(true)
  })

  it('shows when the setting is "flex"', () => {
    expect(isDisplayVisible('flex')).toBe(true)
  })

  it('defaults to visible when the setting is undefined (never configured)', () => {
    expect(isDisplayVisible(undefined)).toBe(true)
  })
})
