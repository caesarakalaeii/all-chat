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

// @vitest-environment jsdom

/**
 * PlatformBadge tests (TDD).
 *
 * Regression guard: rendering a source whose platform is not a chromatic key
 * (e.g. 'shared_overlay', which has no PLATFORM_COLORS entry and no CSS color
 * var) must NOT throw. Previously `PLATFORM_COLORS[platform].text` dereferenced
 * undefined and crashed the whole admin page.
 */

import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import { PlatformBadge } from '../badge'

describe('PlatformBadge', () => {
  it('renders a known platform with its color class and label', () => {
    const { getByText } = render(<PlatformBadge platform="twitch" />)
    const badge = getByText('TWITCH')
    expect(badge.className).toContain('text-twitch')
    expect(badge.getAttribute('data-platform')).toBe('twitch')
  })

  it('does not throw for an unknown platform and falls back to system styling', () => {
    let rendered: ReturnType<typeof render> | undefined
    expect(() => {
      rendered = render(<PlatformBadge platform="shared_overlay" />)
    }).not.toThrow()
    // Friendly label: underscores become spaces.
    const badge = rendered!.getByText('SHARED OVERLAY')
    // System fallback text color (not a broken/undefined class).
    expect(badge.className).toContain('text-text-sub')
  })

  it('renders discord (in the color map) without throwing', () => {
    expect(() => render(<PlatformBadge platform="discord" />)).not.toThrow()
  })
})
