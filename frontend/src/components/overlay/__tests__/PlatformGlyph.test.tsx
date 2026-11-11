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
import { render, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'

import { PlatformGlyph } from '@/components/overlay/PlatformGlyph'
import { PLATFORM_COLORS } from '@/lib/platform-colors'

afterEach(() => cleanup())

// Every chat platform. The OBS overlay, the preview embed and the monitor all
// render their platform icon through PlatformGlyph, so a platform missing here
// is a platform with no icon on stream (Discord was, on the overlay).
const PLATFORMS = Object.keys(PLATFORM_COLORS).filter((p) => p !== 'system')

describe('PlatformGlyph', () => {
  it.each(PLATFORMS)('renders an icon for %s', (platform) => {
    const { container } = render(<PlatformGlyph platform={platform} />)
    expect(container.querySelector('svg')).not.toBeNull()
  })

  it('renders nothing for an unknown platform', () => {
    const { container } = render(<PlatformGlyph platform="myspace" />)
    expect(container.querySelector('svg')).toBeNull()
  })
})
