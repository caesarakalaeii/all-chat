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
import '@testing-library/jest-dom/vitest'
import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'

import { UnsupportedKindPlaceholder } from '@/components/overlays/UnsupportedKindPlaceholder'

afterEach(() => cleanup())

// The real alerts/goal/list renderers are separate issues; until they land the
// render route must show the streamer something recognizable in OBS instead of
// an empty transparent frame.
describe('UnsupportedKindPlaceholder', () => {
  it('names the kind that is not supported yet', () => {
    render(<UnsupportedKindPlaceholder kind="alerts" />)
    expect(screen.getByText(/Alerts/)).toBeVisible()
    expect(screen.getByText(/not yet/i)).toBeInTheDocument()
  })

  it('renders on a transparent page without app chrome', () => {
    // An OBS browser source must not paint a background: the stream shows
    // through wherever the placeholder is not.
    const { container } = render(<UnsupportedKindPlaceholder kind="goal" />)
    expect(container.firstElementChild).not.toHaveClass(/bg-/)
    expect(screen.queryByRole('navigation')).not.toBeInTheDocument()
  })
})
