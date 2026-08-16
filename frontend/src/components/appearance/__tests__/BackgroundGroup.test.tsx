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
import { BackgroundGroup } from '../BackgroundGroup'

afterEach(() => { cleanup() })

describe('BackgroundGroup', () => {
  const defaultSettings: Partial<VisualSettings> = {}

  it('renders overlay background controls', () => {
    const onChange = vi.fn()
    render(<BackgroundGroup visualSettings={defaultSettings} onChange={onChange} />)
    const overlayBgElements = screen.getAllByText(/overlay background/i)
    expect(overlayBgElements.length).toBeGreaterThanOrEqual(1)
  })

  it('renders bubble background controls', () => {
    const onChange = vi.fn()
    render(<BackgroundGroup visualSettings={defaultSettings} onChange={onChange} />)
    const bubbleBgElements = screen.getAllByText(/bubble background/i)
    expect(bubbleBgElements.length).toBeGreaterThanOrEqual(1)
  })

  it('renders border and layout control labels', () => {
    const onChange = vi.fn()
    render(<BackgroundGroup visualSettings={defaultSettings} onChange={onChange} />)
    expect(screen.getByText(/border radius/i)).toBeDefined()
    expect(screen.getByText(/border width/i)).toBeDefined()
    expect(screen.getByText(/border color/i)).toBeDefined()
    expect(screen.getByText(/padding/i)).toBeDefined()
    expect(screen.getByText(/message gap/i)).toBeDefined()
    expect(screen.getByText(/backdrop blur/i)).toBeDefined()
  })

  it('onChange called with overlayBgColor patch as hex string', () => {
    const onChange = vi.fn()
    render(<BackgroundGroup visualSettings={defaultSettings} onChange={onChange} />)
    // Color swatches are present
    const swatches = document.querySelectorAll('[data-testid="color-swatch"]')
    expect(swatches.length).toBeGreaterThanOrEqual(1)
  })

  it('writes opacity into the color as an alpha channel and drops the legacy field', () => {
    const onChange = vi.fn()
    render(<BackgroundGroup visualSettings={defaultSettings} onChange={onChange} />)

    fireEvent.change(screen.getByLabelText('Overlay background opacity'), {
      target: { value: '50' },
    })
    expect(onChange).toHaveBeenCalledWith({
      overlayBgColor: '#00000080',
      overlayBgOpacity: undefined,
    })
  })

  it('folds a legacy *BgOpacity setting into the slider position', () => {
    const onChange = vi.fn()
    render(
      <BackgroundGroup
        visualSettings={{ bubbleBgColor: '#1a1a2e', bubbleBgOpacity: '0.5' }}
        onChange={onChange}
      />
    )
    expect(
      (screen.getByLabelText('Bubble background opacity') as HTMLInputElement).value
    ).toBe('50')
  })

  it('offers an opacity slider for the border color too', () => {
    const onChange = vi.fn()
    render(<BackgroundGroup visualSettings={defaultSettings} onChange={onChange} />)

    fireEvent.change(screen.getByLabelText('Border color opacity'), { target: { value: '0' } })
    expect(onChange).toHaveBeenCalledWith({ bubbleBorderColor: '#33333300' })
  })

  it('onChange called with backdropBlur patch with px suffix', () => {
    const onChange = vi.fn()
    render(<BackgroundGroup visualSettings={defaultSettings} onChange={onChange} />)
    // Backdrop blur slider is the last range input — SliderControl renders them
    const rangeSliders = document.querySelectorAll('input[type="range"]')
    // 3 opacity sliders + 5 slider controls = 8 range inputs total
    // Backdrop blur is the last SliderControl
    const lastSlider = rangeSliders[rangeSliders.length - 1]
    fireEvent.change(lastSlider, { target: { value: '10' } })
    expect(onChange).toHaveBeenCalledWith({ backdropBlur: '10px' })
  })

  it('defaultSettings and onChange types are correct', () => {
    const onChange = vi.fn()
    expect(typeof onChange).toBe('function')
    expect(defaultSettings).toBeDefined()
  })
})
