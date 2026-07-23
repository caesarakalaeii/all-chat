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
import { describe, it, expect, afterEach } from 'vitest'
import React from 'react'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { AdvancedDisclosure } from '../AdvancedDisclosure'

afterEach(() => {
  cleanup()
})

describe('AdvancedDisclosure', () => {
  it('renders collapsed by default with a control count', () => {
    const { container } = render(
      <AdvancedDisclosure count={3}>
        <span>advanced child</span>
      </AdvancedDisclosure>
    )
    const details = container.querySelector('details')
    expect(details).not.toBeNull()
    expect(details!.hasAttribute('open')).toBe(false)
    expect(screen.getByText('Advanced (3)')).toBeDefined()
  })

  it('opens when the summary is clicked', () => {
    const { container } = render(
      <AdvancedDisclosure count={1}>
        <span>advanced child</span>
      </AdvancedDisclosure>
    )
    fireEvent.click(screen.getByText('Advanced (1)'))
    expect(container.querySelector('details')!.hasAttribute('open')).toBe(true)
  })
})
