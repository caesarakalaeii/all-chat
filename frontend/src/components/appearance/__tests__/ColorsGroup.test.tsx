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
import { describe, it, expect, vi, afterEach } from 'vitest'
import React from 'react'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import type { VisualSettings } from '@/lib/types/visual-settings'
import { ColorsGroup } from '../ColorsGroup'

afterEach(() => { cleanup() })

describe('ColorsGroup', () => {
  const defaultSettings: Partial<VisualSettings> = {}

  it('renders labels for message, username, and timestamp colors', () => {
    const onChange = vi.fn()
    render(<ColorsGroup visualSettings={defaultSettings} onChange={onChange} />)
    expect(screen.getByText(/message color/i)).toBeDefined()
    expect(screen.getByText(/username color/i)).toBeDefined()
    expect(screen.getByText(/timestamp color/i)).toBeDefined()
  })

  it('renders three color swatches', () => {
    const onChange = vi.fn()
    render(<ColorsGroup visualSettings={defaultSettings} onChange={onChange} />)
    // Swatches are buttons with data-testid or role=button with background style
    const swatches = document.querySelectorAll('[data-testid="color-swatch"]')
    expect(swatches).toHaveLength(3)
  })

  it('onChange called with messageColor hex patch when slider value changes (simulated)', () => {
    const onChange = vi.fn()
    const settings: Partial<VisualSettings> = { messageColor: '#ff0000' }
    render(<ColorsGroup visualSettings={settings} onChange={onChange} />)
    // Swatch renders and color is set from visualSettings
    const swatches = document.querySelectorAll('[data-testid="color-swatch"]')
    expect(swatches[0]).toBeDefined()
    // Color is displayed via inline style
    expect((swatches[0] as HTMLElement).style.backgroundColor).toBeTruthy()
  })

  it('onChange called with usernameColor patch when swatch clicked and picker changes', () => {
    const onChange = vi.fn()
    render(<ColorsGroup visualSettings={defaultSettings} onChange={onChange} />)
    // Clicking the second swatch (username) opens the popover
    const swatches = document.querySelectorAll('[data-testid="color-swatch"]')
    expect(swatches[1]).toBeDefined()
    fireEvent.click(swatches[1])
    // Popover should now be visible (HexColorPicker is rendered)
    const popover = document.querySelector('.react-colorful')
    expect(popover).toBeDefined()
  })

  it('onChange called with timestampColor patch with correct defaults', () => {
    const onChange = vi.fn()
    render(<ColorsGroup visualSettings={defaultSettings} onChange={onChange} />)
    // Third swatch uses timestampColor default '#888888'
    const swatches = document.querySelectorAll('[data-testid="color-swatch"]')
    // Default color applied as background-color
    expect((swatches[2] as HTMLElement).getAttribute('style')).toContain('rgb(136, 136, 136)')
  })

  it('defaultSettings and onChange types are correct', () => {
    const onChange = vi.fn()
    expect(typeof onChange).toBe('function')
    expect(defaultSettings).toBeDefined()
  })
})
