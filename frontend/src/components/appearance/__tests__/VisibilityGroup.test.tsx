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
import { VisibilityGroup } from '../VisibilityGroup'

afterEach(() => { cleanup() })

describe('VisibilityGroup - Pronoun Controls', () => {
  it('renders "Show pronouns" toggle', () => {
    const onChange = vi.fn()
    render(<VisibilityGroup visualSettings={{}} onChange={onChange} />)
    expect(screen.getByText('Show pronouns')).toBeDefined()
  })

  it('when showPronouns is "inline", toggle is checked', () => {
    const onChange = vi.fn()
    render(<VisibilityGroup visualSettings={{ showPronouns: 'inline' }} onChange={onChange} />)
    const switches = screen.getAllByRole('switch')
    const pronounSwitch = switches[switches.length - 1]
    expect(pronounSwitch.getAttribute('aria-checked')).toBe('true')
  })

  it('when showPronouns is "none", toggle is unchecked', () => {
    const onChange = vi.fn()
    render(<VisibilityGroup visualSettings={{ showPronouns: 'none' }} onChange={onChange} />)
    const switches = screen.getAllByRole('switch')
    const pronounSwitch = switches[switches.length - 1]
    expect(pronounSwitch.getAttribute('aria-checked')).toBe('false')
  })

  it('toggling pronouns OFF calls onChange with { showPronouns: "none" }', () => {
    const onChange = vi.fn()
    render(<VisibilityGroup visualSettings={{ showPronouns: 'inline' }} onChange={onChange} />)
    const switches = screen.getAllByRole('switch')
    fireEvent.click(switches[switches.length - 1])
    expect(onChange).toHaveBeenCalledWith({ showPronouns: 'none' })
  })

  it('toggling pronouns ON calls onChange with { showPronouns: "inline" }', () => {
    const onChange = vi.fn()
    render(<VisibilityGroup visualSettings={{ showPronouns: 'none' }} onChange={onChange} />)
    const switches = screen.getAllByRole('switch')
    fireEvent.click(switches[switches.length - 1])
    expect(onChange).toHaveBeenCalledWith({ showPronouns: 'inline' })
  })

  it('position radio renders with "Before username" and "After username" options', () => {
    const onChange = vi.fn()
    render(<VisibilityGroup visualSettings={{}} onChange={onChange} />)
    // Both platform badge and pronouns share the same position labels; getAllByText is correct
    const beforeLabels = screen.getAllByText('Before username')
    const afterLabels = screen.getAllByText('After username')
    expect(beforeLabels.length).toBeGreaterThanOrEqual(1)
    expect(afterLabels.length).toBeGreaterThanOrEqual(1)
  })

  it('changing position to "before" calls onChange with { pronounPosition: "before" }', () => {
    const onChange = vi.fn()
    render(<VisibilityGroup visualSettings={{}} onChange={onChange} />)
    // There are two radio groups with 'before' value (platform badge + pronouns)
    // Use the pronounPosition named radio group
    const beforeRadios = screen.getAllByDisplayValue('before')
    // The last one belongs to the pronounPosition group
    fireEvent.click(beforeRadios[beforeRadios.length - 1])
    expect(onChange).toHaveBeenCalledWith({ pronounPosition: 'before' })
  })

  it('renders "Pill color" label for the color picker', () => {
    const onChange = vi.fn()
    render(<VisibilityGroup visualSettings={{}} onChange={onChange} />)
    expect(screen.getByText('Pill color')).toBeDefined()
  })

  it('when showPronouns is "none", pronoun sub-controls have opacity-40 class', () => {
    const onChange = vi.fn()
    const { container } = render(
      <VisibilityGroup visualSettings={{ showPronouns: 'none' }} onChange={onChange} />
    )
    const dimmedDiv = container.querySelector('.opacity-40')
    expect(dimmedDiv).toBeTruthy()
  })
})

describe('VisibilityGroup', () => {
  it('renders 6 labels', () => {
    const onChange = vi.fn()
    render(<VisibilityGroup visualSettings={{}} onChange={onChange} />)
    expect(screen.getByText('Show avatars')).toBeDefined()
    expect(screen.getByText('Show badges')).toBeDefined()
    expect(screen.getByText('Show timestamps')).toBeDefined()
    expect(screen.getByText('Show platform badge')).toBeDefined()
    expect(screen.getByText('Show emotes')).toBeDefined()
    expect(screen.getByText('Show username')).toBeDefined()
  })

  it('each row has a button with role="switch" (5 ROWS + platform badge + platform indicators + pronouns = 8)', () => {
    const onChange = vi.fn()
    render(<VisibilityGroup visualSettings={{}} onChange={onChange} />)
    const switches = screen.getAllByRole('switch')
    expect(switches).toHaveLength(8)
  })

  it('clicking an ON switch calls onChange with none (showAvatars ON→OFF)', () => {
    const onChange = vi.fn()
    render(
      <VisibilityGroup
        visualSettings={{ showAvatars: 'inline' }}
        onChange={onChange}
      />
    )
    const switches = screen.getAllByRole('switch')
    fireEvent.click(switches[0])
    expect(onChange).toHaveBeenCalledWith({ showAvatars: 'none' })
  })

  it('clicking an OFF switch calls onChange with inline (showAvatars OFF→ON)', () => {
    const onChange = vi.fn()
    render(
      <VisibilityGroup
        visualSettings={{ showAvatars: 'none' }}
        onChange={onChange}
      />
    )
    const switches = screen.getAllByRole('switch')
    fireEvent.click(switches[0])
    expect(onChange).toHaveBeenCalledWith({ showAvatars: 'inline' })
  })

  it('showTimestamps emits "block" (not "inline") when switching to ON', () => {
    const onChange = vi.fn()
    render(
      <VisibilityGroup
        visualSettings={{ showTimestamps: 'none' }}
        onChange={onChange}
      />
    )
    const switches = screen.getAllByRole('switch')
    // showTimestamps is the 3rd row (index 2)
    fireEvent.click(switches[2])
    expect(onChange).toHaveBeenCalledWith({ showTimestamps: 'block' })
  })

  it('visualSettings.showAvatars="none" overrides visibilityDefaults value of "inline"', () => {
    const onChange = vi.fn()
    render(
      <VisibilityGroup
        visualSettings={{ showAvatars: 'none' }}
        onChange={onChange}
        visibilityDefaults={{ showAvatars: 'inline' }}
      />
    )
    const switches = screen.getAllByRole('switch')
    expect(switches[0].getAttribute('aria-checked')).toBe('false')
  })

  it('when visualSettings.showAvatars is undefined and visibilityDefaults.showAvatars is "none", toggle renders unchecked', () => {
    const onChange = vi.fn()
    render(
      <VisibilityGroup
        visualSettings={{}}
        onChange={onChange}
        visibilityDefaults={{ showAvatars: 'none' }}
      />
    )
    const switches = screen.getAllByRole('switch')
    expect(switches[0].getAttribute('aria-checked')).toBe('false')
  })

  it('when both visualSettings and visibilityDefaults are undefined for a field, toggle defaults to checked', () => {
    const onChange = vi.fn()
    render(<VisibilityGroup visualSettings={{}} onChange={onChange} />)
    const switches = screen.getAllByRole('switch')
    expect(switches[0].getAttribute('aria-checked')).toBe('true')
  })
})
