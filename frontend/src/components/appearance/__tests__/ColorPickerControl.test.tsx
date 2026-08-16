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
import { ColorPickerControl } from '../ColorPickerControl'

afterEach(() => {
  cleanup()
})

describe('ColorPickerControl opacity', () => {
  it('renders an opacity slider for every color, not just backgrounds', () => {
    render(<ColorPickerControl label="Message color" value="#ffffff" onChange={vi.fn()} />)
    const slider = screen.getByLabelText('Message color opacity') as HTMLInputElement
    expect(slider.type).toBe('range')
  })

  it('reads the slider position from the color’s alpha channel', () => {
    const { rerender } = render(
      <ColorPickerControl label="Bubble background" value="#1a1a2e" onChange={vi.fn()} />
    )
    expect((screen.getByLabelText('Bubble background opacity') as HTMLInputElement).value).toBe(
      '100'
    )

    rerender(<ColorPickerControl label="Bubble background" value="#1a1a2e80" onChange={vi.fn()} />)
    expect((screen.getByLabelText('Bubble background opacity') as HTMLInputElement).value).toBe(
      '50'
    )
  })

  it('emits the opacity as an alpha channel on the color, keeping the RGB', () => {
    const onChange = vi.fn()
    render(<ColorPickerControl label="Bubble background" value="#1a1a2e" onChange={onChange} />)

    fireEvent.change(screen.getByLabelText('Bubble background opacity'), {
      target: { value: '0' },
    })
    expect(onChange).toHaveBeenCalledWith('#1a1a2e00')
  })

  it('emits a plain 6-digit hex at full opacity', () => {
    const onChange = vi.fn()
    render(<ColorPickerControl label="Border color" value="#33333380" onChange={onChange} />)

    fireEvent.change(screen.getByLabelText('Border color opacity'), { target: { value: '100' } })
    expect(onChange).toHaveBeenCalledWith('#333333')
  })

  it('shows the color over a checkerboard so transparency is visible', () => {
    render(<ColorPickerControl label="Message color" value="#ffffff80" onChange={vi.fn()} />)
    const swatch = screen.getByTestId('color-swatch')
    expect(swatch.parentElement?.className).toContain('alpha-checkerboard')
  })
})
