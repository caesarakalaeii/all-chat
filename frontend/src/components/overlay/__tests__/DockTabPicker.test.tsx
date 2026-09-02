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
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { DockTabPicker } from '@/components/overlay/DockTabPicker'

afterEach(() => cleanup())

describe('DockTabPicker', () => {
  it('marks the selected tab as the only one selected', () => {
    render(<DockTabPicker tab="chat" onChange={vi.fn()} />)
    expect(screen.getByRole('tab', { name: 'Chat' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('tab', { name: 'Activity' })).toHaveAttribute('aria-selected', 'false')
  })

  it('reports the other tab when it is clicked', () => {
    const onChange = vi.fn()
    render(<DockTabPicker tab="chat" onChange={onChange} />)
    fireEvent.click(screen.getByRole('tab', { name: 'Activity' }))
    expect(onChange).toHaveBeenCalledWith('activity')
  })

  it('does not report a change when the selected tab is clicked again', () => {
    const onChange = vi.fn()
    render(<DockTabPicker tab="activity" onChange={onChange} />)
    fireEvent.click(screen.getByRole('tab', { name: 'Activity' }))
    expect(onChange).not.toHaveBeenCalled()
  })
})
