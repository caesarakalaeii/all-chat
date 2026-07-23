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
import { PreviewBackdropPicker } from '../PreviewBackdropPicker'

afterEach(() => {
  cleanup()
})

describe('PreviewBackdropPicker', () => {
  it('renders the preset swatches and a custom color input', () => {
    render(<PreviewBackdropPicker value={null} onChange={() => {}} />)
    expect(screen.getByRole('button', { name: 'Preview on app background' })).toBeDefined()
    expect(screen.getByRole('button', { name: 'Preview on light background' })).toBeDefined()
    expect(screen.getByRole('button', { name: 'Preview on chroma green' })).toBeDefined()
    expect(screen.getByLabelText('Custom preview background color')).toBeDefined()
  })

  it('clicking a preset reports its color', () => {
    const onChange = vi.fn()
    render(<PreviewBackdropPicker value={null} onChange={onChange} />)
    fireEvent.click(screen.getByRole('button', { name: 'Preview on chroma green' }))
    expect(onChange).toHaveBeenCalledWith('#00b140')
  })

  it('clicking the default preset resets to null', () => {
    const onChange = vi.fn()
    render(<PreviewBackdropPicker value="#00b140" onChange={onChange} />)
    fireEvent.click(screen.getByRole('button', { name: 'Preview on app background' }))
    expect(onChange).toHaveBeenCalledWith(null)
  })

  it('the custom color input reports arbitrary colors', () => {
    const onChange = vi.fn()
    render(<PreviewBackdropPicker value={null} onChange={onChange} />)
    fireEvent.change(screen.getByLabelText('Custom preview background color'), {
      target: { value: '#123456' },
    })
    expect(onChange).toHaveBeenCalledWith('#123456')
  })

  it('marks the active preset with aria-pressed', () => {
    render(<PreviewBackdropPicker value="#00b140" onChange={() => {}} />)
    expect(
      screen.getByRole('button', { name: 'Preview on chroma green' }).getAttribute('aria-pressed')
    ).toBe('true')
    expect(
      screen
        .getByRole('button', { name: 'Preview on app background' })
        .getAttribute('aria-pressed')
    ).toBe('false')
  })
})
