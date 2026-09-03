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
import { render, screen, cleanup, fireEvent } from '@testing-library/react'
import type { VisualSettings } from '@/lib/types/visual-settings'
import { buildOutlineShadow } from '@/lib/utils/text-outline'
import { TypographyGroup } from '../TypographyGroup'

afterEach(() => {
  cleanup()
})

describe('TypographyGroup', () => {
  const defaultSettings: Partial<VisualSettings> = {}

  it('renders font family labels for body, username, and timestamp', () => {
    const onChange = vi.fn()
    render(<TypographyGroup visualSettings={defaultSettings} onChange={onChange} />)
    expect(screen.getByText(/body font/i)).toBeDefined()
    expect(screen.getByText(/username font/i)).toBeDefined()
    expect(screen.getByText(/timestamp font/i)).toBeDefined()
  })

  it('renders font weight label', () => {
    const onChange = vi.fn()
    render(<TypographyGroup visualSettings={defaultSettings} onChange={onChange} />)
    expect(screen.getByText(/font weight/i)).toBeDefined()
  })

  it('renders font size labels for body, username, and timestamp', () => {
    const onChange = vi.fn()
    render(<TypographyGroup visualSettings={defaultSettings} onChange={onChange} />)
    expect(screen.getByText(/body size/i)).toBeDefined()
    expect(screen.getByText(/username size/i)).toBeDefined()
    expect(screen.getByText(/timestamp size/i)).toBeDefined()
  })

  it('renders line height and letter spacing labels', () => {
    const onChange = vi.fn()
    render(<TypographyGroup visualSettings={defaultSettings} onChange={onChange} />)
    expect(screen.getByText(/line height/i)).toBeDefined()
    expect(screen.getByText(/letter spacing/i)).toBeDefined()
  })

  it('renders the text shadow presets and reports a patch on change', () => {
    const onChange = vi.fn()
    render(<TypographyGroup visualSettings={defaultSettings} onChange={onChange} />)
    const select = screen.getByLabelText(/text shadow/i) as HTMLSelectElement
    expect(select).toBeDefined()
    fireEvent.change(select, { target: { value: '0 1px 2px rgba(0, 0, 0, 0.6)' } })
    expect(onChange).toHaveBeenCalledWith({ textShadow: '0 1px 2px rgba(0, 0, 0, 0.6)' })
  })

  it('selecting None unsets textShadow (falls back to theme/none)', () => {
    const onChange = vi.fn()
    render(
      <TypographyGroup
        visualSettings={{ textShadow: '0 1px 2px rgba(0, 0, 0, 0.6)' }}
        onChange={onChange}
      />
    )
    fireEvent.change(screen.getByLabelText(/text shadow/i), { target: { value: '' } })
    expect(onChange).toHaveBeenCalledWith({ textShadow: undefined })
  })

  it('shows a Custom option when the saved shadow matches no preset', () => {
    const onChange = vi.fn()
    render(
      <TypographyGroup visualSettings={{ textShadow: '2px 2px 0 #ff00ff' }} onChange={onChange} />
    )
    const select = screen.getByLabelText(/text shadow/i) as HTMLSelectElement
    expect(select.value).toBe('2px 2px 0 #ff00ff')
    expect(screen.getByText('Custom')).toBeDefined()
  })

  it('selecting Outline writes a 2px outline, thicker than the old 1px shape', () => {
    const onChange = vi.fn()
    render(<TypographyGroup visualSettings={defaultSettings} onChange={onChange} />)
    const select = screen.getByLabelText(/text shadow/i) as HTMLSelectElement
    const outline = Array.from(select.options).find((o) => o.text === 'Outline')!
    fireEvent.change(select, { target: { value: outline.value } })
    expect(onChange).toHaveBeenCalledWith({ textShadow: buildOutlineShadow(2) })
  })

  it('hides the outline thickness slider for the non-outline presets', () => {
    const onChange = vi.fn()
    for (const textShadow of [undefined, '0 1px 2px rgba(0, 0, 0, 0.6)']) {
      const { unmount } = render(
        <TypographyGroup visualSettings={{ textShadow }} onChange={onChange} />
      )
      expect(screen.queryByLabelText(/outline thickness/i)).toBeNull()
      unmount()
    }
  })

  it('reports the new width when the outline thickness slider moves', () => {
    const onChange = vi.fn()
    render(
      <TypographyGroup visualSettings={{ textShadow: buildOutlineShadow(2) }} onChange={onChange} />
    )
    const slider = screen.getByLabelText(/outline thickness/i)
    fireEvent.change(slider, { target: { value: '5' } })
    expect(onChange).toHaveBeenCalledWith({ textShadow: buildOutlineShadow(5) })
  })

  it('keeps Outline selected for a stored non-default width', () => {
    const onChange = vi.fn()
    render(
      <TypographyGroup visualSettings={{ textShadow: buildOutlineShadow(3) }} onChange={onChange} />
    )
    const select = screen.getByLabelText(/text shadow/i) as HTMLSelectElement
    expect(select.selectedOptions[0].text).toBe('Outline')
    expect(screen.queryByText('Custom')).toBeNull()
    expect((screen.getByLabelText(/outline thickness/i) as HTMLInputElement).value).toBe('3')
  })

  // An outline stored in the geometry #833 shipped: eight compass points at
  // radius 4, which rendered as detached copies of the text (ADR-0057). The
  // editor has to recognise it as a 4px outline rather than as an opaque
  // "Custom" value, and re-picking Outline has to write the CURRENT geometry
  // instead of handing the old string back to the API unchanged.
  it('adopts an outline stored in the geometry that shipped broken', () => {
    const onChange = vi.fn()
    const ghosted = [
      '4px 0px 0 rgba(0, 0, 0, 0.85)',
      '0px 4px 0 rgba(0, 0, 0, 0.85)',
      '-4px 0px 0 rgba(0, 0, 0, 0.85)',
      '0px -4px 0 rgba(0, 0, 0, 0.85)',
      '4px 4px 0 rgba(0, 0, 0, 0.85)',
      '-4px 4px 0 rgba(0, 0, 0, 0.85)',
      '-4px -4px 0 rgba(0, 0, 0, 0.85)',
      '4px -4px 0 rgba(0, 0, 0, 0.85)',
    ].join(', ')

    render(<TypographyGroup visualSettings={{ textShadow: ghosted }} onChange={onChange} />)

    const select = screen.getByLabelText(/text shadow/i) as HTMLSelectElement
    expect(select.selectedOptions[0].text).toBe('Outline')
    expect(screen.queryByText('Custom')).toBeNull()
    expect((screen.getByLabelText(/outline thickness/i) as HTMLInputElement).value).toBe('4')

    const outline = Array.from(select.options).find((o) => o.text === 'Outline')!
    fireEvent.change(select, { target: { value: outline.value } })
    expect(onChange).toHaveBeenCalledWith({ textShadow: buildOutlineShadow(4) })
  })

  // The font-weight dropdown is portalled, so it must out-stack the overlay
  // customiser's own layers. That is what the z-index utility on the positioner
  // buys, and it is asserted on the rendered class attribute rather than the
  // source because the risk being guarded is an edit that changes the emitted
  // class list (an ESLint disable comment landing inside the className
  // expression, say) without touching the literal.
  it('stacks the portalled font-weight dropdown with z-200', () => {
    const onChange = vi.fn()
    render(<TypographyGroup visualSettings={defaultSettings} onChange={onChange} />)
    fireEvent.click(screen.getByLabelText(/font weight/i))
    const popup = screen.getByRole('listbox')
    const positioner = popup.closest('[class*="z-"]')
    expect(positioner).not.toBeNull()
    expect(positioner!.className.split(/\s+/)).toContain('z-200')
  })

  it.todo('onChange called with fontFamily patch when font selection changes')

  it.todo('onChange called with fontWeight patch on select change')

  it.todo('onChange called with lineHeight patch on slider change')
})
